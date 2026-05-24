package paxos

import (
	"context"
	"fmt"
	"sync"
	"time"

	paxosv1 "github.com/jvenkit/grpc/gen/go/paxos/v1"
	"github.com/jvenkit/grpc/lib/client"
	"github.com/jvenkit/grpc/lib/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcSendTimeout caps how long a single Deliver RPC waits. Paxos retries on
// timeout, so a dead peer must not block the proposer indefinitely.
const grpcSendTimeout = 2 * time.Second

// GRPCTransport is a Transport implementation that carries Paxos messages over
// gRPC. Each node runs an inbound PaxosService server and N-1 outbound
// PaxosService clients, one per peer.
type GRPCTransport struct {
	paxosv1.UnimplementedPaxosServiceServer

	selfID     int
	listenAddr string
	peerAddrs  map[int]string

	runtime server.Runtime
	conns   map[int]*grpc.ClientConn
	clients map[int]paxosv1.PaxosServiceClient

	inbound  chan Message
	done     chan struct{}
	stopOnce sync.Once
}

// NewGRPCTransport constructs a Transport backed by gRPC. listenAddr is the
// local bind address (use ":0" for an ephemeral port). peerAddrs maps each
// remote peer's node ID to its dial target ("host:port"); it must not contain
// selfID.
func NewGRPCTransport(selfID int, listenAddr string, peerAddrs map[int]string) (*GRPCTransport, error) {
	if _, self := peerAddrs[selfID]; self {
		return nil, fmt.Errorf("grpcTransport: peerAddrs must not contain selfID %d", selfID)
	}

	t := &GRPCTransport{
		selfID:     selfID,
		listenAddr: listenAddr,
		peerAddrs:  peerAddrs,
		conns:      make(map[int]*grpc.ClientConn, len(peerAddrs)),
		clients:    make(map[int]paxosv1.PaxosServiceClient, len(peerAddrs)),
		inbound:    make(chan Message, 1024),
		done:       make(chan struct{}),
	}

	rt, err := server.New(server.WithAddress(listenAddr)).
		RegisterService(func(s *grpc.Server) {
			paxosv1.RegisterPaxosServiceServer(s, t)
		}).
		Build()
	if err != nil {
		return nil, fmt.Errorf("grpcTransport: build runtime: %w", err)
	}
	t.runtime = rt
	return t, nil
}

// Start binds the listener and dials all configured peers. Dials are
// non-blocking — the gRPC client connects lazily, so peers can be started in
// any order.
func (t *GRPCTransport) Start(ctx context.Context) error {
	if err := t.runtime.Start(ctx); err != nil {
		return fmt.Errorf("grpcTransport: start runtime: %w", err)
	}
	for id, addr := range t.peerAddrs {
		conn, err := client.New(
			addr,
			client.WithDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
		).Build(ctx)
		if err != nil {
			return fmt.Errorf("grpcTransport: dial peer %d at %s: %w", id, addr, err)
		}
		t.conns[id] = conn
		t.clients[id] = paxosv1.NewPaxosServiceClient(conn)
	}
	return nil
}

// Addr returns the bound listen address (resolves ":0" after Start).
func (t *GRPCTransport) Addr() string {
	return t.runtime.Addr()
}

// Send implements Transport. Each call uses a bounded context so a dead peer
// cannot stall the proposer.
func (t *GRPCTransport) Send(msg Message) error {
	c, ok := t.clients[msg.To]
	if !ok {
		return fmt.Errorf("grpcTransport: no client for peer %d", msg.To)
	}
	ctx, cancel := context.WithTimeout(context.Background(), grpcSendTimeout)
	defer cancel()
	_, err := c.Deliver(ctx, &paxosv1.DeliverRequest{Message: toProto(msg)})
	return err
}

// Receive implements Transport.
func (t *GRPCTransport) Receive(ctx context.Context) (Message, error) {
	select {
	case m := <-t.inbound:
		return m, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-t.done:
		return Message{}, fmt.Errorf("grpcTransport: stopped")
	}
}

// Stop closes outbound connections and shuts the gRPC server.
func (t *GRPCTransport) Stop(ctx context.Context) error {
	t.stopOnce.Do(func() { close(t.done) })
	for _, c := range t.conns {
		_ = c.Close()
	}
	return t.runtime.Stop(ctx)
}

// Deliver is the inbound PaxosService handler. It enqueues the message onto
// the local inbound channel for consumption via Receive.
func (t *GRPCTransport) Deliver(ctx context.Context, req *paxosv1.DeliverRequest) (*paxosv1.DeliverResponse, error) {
	if req == nil || req.Message == nil {
		return &paxosv1.DeliverResponse{}, nil
	}
	select {
	case t.inbound <- fromProto(req.Message):
		return &paxosv1.DeliverResponse{}, nil
	case <-t.done:
		return &paxosv1.DeliverResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func toProto(m Message) *paxosv1.Message {
	return &paxosv1.Message{
		From:   int32(m.From),
		To:     int32(m.To),
		Type:   int32(m.Type),
		Number: int64(m.Number),
		Value:  append([]byte(nil), m.Value...),
		Slot:   int32(m.Slot),
	}
}

func fromProto(m *paxosv1.Message) Message {
	return Message{
		From:   int(m.From),
		To:     int(m.To),
		Type:   MessageType(m.Type),
		Number: int(m.Number),
		Value:  append([]byte(nil), m.Value...),
		Slot:   int(m.Slot),
	}
}
