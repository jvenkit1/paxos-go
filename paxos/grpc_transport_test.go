package paxos

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// freeAddr returns a 127.0.0.1 address bound to a free TCP port. There's a
// small TOCTOU race between the close here and the eventual rebind, but it's
// acceptable for tests.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("listener close: %v", err)
	}
	return addr
}

// buildGRPCCluster wires up N GRPCTransports with mutual peer knowledge and
// starts each one. Returned in id order matching ids.
func buildGRPCCluster(t *testing.T, ids []int) (map[int]*GRPCTransport, func()) {
	t.Helper()

	addrs := make(map[int]string, len(ids))
	for _, id := range ids {
		addrs[id] = freeAddr(t)
	}

	transports := make(map[int]*GRPCTransport, len(ids))
	for _, id := range ids {
		peers := make(map[int]string, len(ids)-1)
		for _, pid := range ids {
			if pid != id {
				peers[pid] = addrs[pid]
			}
		}
		tr, err := NewGRPCTransport(id, addrs[id], peers)
		if err != nil {
			t.Fatalf("NewGRPCTransport(%d): %v", id, err)
		}
		transports[id] = tr
	}

	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// GRPCTransport.Start now blocks until peer connections reach READY, so each
	// transport must start in its own goroutine — otherwise the first one would
	// deadlock waiting for peers whose Start hasn't been called yet.
	var wg sync.WaitGroup
	errCh := make(chan error, len(transports))
	for id, tr := range transports {
		wg.Add(1)
		go func(id int, tr *GRPCTransport) {
			defer wg.Done()
			if err := tr.Start(startCtx); err != nil {
				errCh <- fmt.Errorf("transport %d Start: %w", id, err)
			}
		}(id, tr)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	cleanup := func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		for _, tr := range transports {
			_ = tr.Stop(stopCtx)
		}
	}
	return transports, cleanup
}

func TestGRPCTransportRoundTrip(t *testing.T) {
	transports, cleanup := buildGRPCCluster(t, []int{1, 2})
	defer cleanup()

	want := Message{
		From:   1,
		To:     2,
		Type:   PrepareMsg,
		Number: 12345,
		Value:  []byte("hello-grpc"),
		Slot:   7,
	}
	if err := transports[1].Send(want); err != nil {
		t.Fatalf("Send: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := transports[2].Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.From != want.From || got.To != want.To || got.Type != want.Type ||
		got.Number != want.Number || got.Slot != want.Slot || string(got.Value) != string(want.Value) {
		t.Errorf("round-trip mismatch:\n  got  = %+v\n  want = %+v", got, want)
	}
}

// TestGRPCTransportStartUnreachablePeer verifies that Start blocks waiting for
// every peer to reach READY and returns the ctx error if any peer is
// unreachable before the deadline.
func TestGRPCTransportStartUnreachablePeer(t *testing.T) {
	// freeAddr returns an unbound 127.0.0.1 port. Nothing is listening on it,
	// so the gRPC client connection will never reach READY.
	unreachable := freeAddr(t)

	tr, err := NewGRPCTransport(1, freeAddr(t), map[int]string{2: unreachable})
	if err != nil {
		t.Fatalf("NewGRPCTransport: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = tr.Stop(stopCtx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := tr.Start(ctx); err == nil {
		t.Fatal("Start with unreachable peer should fail")
	}
}

func TestGRPCTransportSendUnknownPeer(t *testing.T) {
	transports, cleanup := buildGRPCCluster(t, []int{1})
	defer cleanup()

	err := transports[1].Send(Message{To: 99})
	if err == nil {
		t.Fatal("Send to unknown peer should fail")
	}
}

func buildGRPCNodes(t *testing.T, ids []int) (map[int]*Node, func()) {
	t.Helper()
	transports, transportCleanup := buildGRPCCluster(t, ids)

	nodes := make(map[int]*Node, len(ids))
	for _, id := range ids {
		var peerIDs []int
		for _, pid := range ids {
			if pid != id {
				peerIDs = append(peerIDs, pid)
			}
		}
		nodes[id] = NewNode(id, peerIDs, transports[id])
	}

	ctx := context.Background()
	for _, n := range nodes {
		n.Start(ctx)
	}

	cleanup := func() {
		for _, n := range nodes {
			n.Stop()
		}
		transportCleanup()
	}
	return nodes, cleanup
}

func TestNodeGRPCSingleValue(t *testing.T) {
	ids := []int{1, 2, 3}
	nodes, cleanup := buildGRPCNodes(t, ids)
	defer cleanup()

	// Wait for leader election. Election window is 500ms; allow buffer for
	// gRPC connection establishment.
	time.Sleep(900 * time.Millisecond)

	ctx := context.Background()
	if err := nodes[3].Propose(ctx, []byte("hello-grpc")); err != nil {
		t.Fatalf("Propose: %v", err)
	}

	select {
	case entry := <-nodes[3].Committed():
		if entry.Slot != 0 {
			t.Errorf("slot = %d, want 0", entry.Slot)
		}
		if string(entry.Value) != "hello-grpc" {
			t.Errorf("value = %q, want %q", string(entry.Value), "hello-grpc")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for committed entry on leader")
	}
}

func TestNodeGRPCMultiDecree(t *testing.T) {
	ids := []int{1, 2, 3}
	nodes, cleanup := buildGRPCNodes(t, ids)
	defer cleanup()

	time.Sleep(900 * time.Millisecond)

	ctx := context.Background()
	values := []string{"alpha", "beta", "gamma"}
	for _, v := range values {
		if err := nodes[3].Propose(ctx, []byte(v)); err != nil {
			t.Fatalf("Propose(%q): %v", v, err)
		}
	}

	decided := make(map[int]string)
	for i := 0; i < len(values); i++ {
		select {
		case entry := <-nodes[3].Committed():
			decided[entry.Slot] = string(entry.Value)
		case <-time.After(15 * time.Second):
			t.Fatalf("timed out after %d decisions; have: %v", i, decided)
		}
	}

	want := map[int]string{0: "alpha", 1: "beta", 2: "gamma"}
	for slot, expected := range want {
		got, ok := decided[slot]
		if !ok {
			t.Errorf("slot %d not decided", slot)
		} else if got != expected {
			t.Errorf("slot %d: got %q, want %q", slot, got, expected)
		}
	}
}

func TestNodeGRPCLeaderElection(t *testing.T) {
	ids := []int{1, 2}
	nodes, cleanup := buildGRPCNodes(t, ids)
	defer cleanup()

	time.Sleep(900 * time.Millisecond)

	ctx := context.Background()
	if err := nodes[2].Propose(ctx, []byte("from-leader")); err != nil {
		t.Fatalf("Propose: %v", err)
	}

	for _, id := range ids {
		select {
		case entry := <-nodes[id].Committed():
			if string(entry.Value) != "from-leader" {
				t.Errorf("node %d: value = %q, want %q", id, string(entry.Value), "from-leader")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("node %d: timed out waiting for committed entry", id)
		}
	}
}

// Verifies that proto/internal field conversions preserve all fields.
func TestGRPCTransportFieldFidelity(t *testing.T) {
	cases := []Message{
		{From: 100, To: 200, Type: PrepareMsg, Number: 1, Value: []byte("a"), Slot: 0},
		{From: 1, To: 2, Type: AckMsg, Number: 1<<31 - 1, Value: []byte{}, Slot: 999},
		{From: 5, To: 6, Type: HeartbeatMsg, Number: 0, Value: nil, Slot: 0},
	}
	for i, m := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			got := fromProto(toProto(m))
			if got.From != m.From || got.To != m.To || got.Type != m.Type ||
				got.Number != m.Number || got.Slot != m.Slot ||
				string(got.Value) != string(m.Value) {
				t.Errorf("roundtrip:\n  in  = %+v\n  out = %+v", m, got)
			}
		})
	}
}
