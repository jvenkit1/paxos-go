// Command paxos runs a single Paxos node over gRPC. Spin up one process per
// node and point each at the others via -peers.
//
// Example (3-node cluster on localhost):
//
//	paxos -id 1 -listen 127.0.0.1:9001 -peers 2=127.0.0.1:9002,3=127.0.0.1:9003
//	paxos -id 2 -listen 127.0.0.1:9002 -peers 1=127.0.0.1:9001,3=127.0.0.1:9003
//	paxos -id 3 -listen 127.0.0.1:9003 -peers 1=127.0.0.1:9001,2=127.0.0.1:9002
//
// Lines read from stdin are submitted as proposals; decided entries are
// written to stdout as "slot=<n> value=<text>".
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jvenkit1/paxos-go/paxos"
)

func main() {
	id := flag.Int("id", 0, "this node's unique ID (required, > 0)")
	listen := flag.String("listen", "", "bind address for inbound gRPC (e.g. 127.0.0.1:9001)")
	peersFlag := flag.String("peers", "", "comma-separated peer list as id=host:port (e.g. 2=127.0.0.1:9002,3=127.0.0.1:9003)")
	flag.Parse()

	if *id <= 0 || *listen == "" {
		flag.Usage()
		os.Exit(2)
	}

	peerAddrs, err := parsePeers(*peersFlag, *id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -peers: %v\n", err)
		os.Exit(2)
	}

	transport, err := paxos.NewGRPCTransport(*id, *listen, peerAddrs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewGRPCTransport: %v\n", err)
		os.Exit(1)
	}

	startCtx, startCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer startCancel()
	if err := transport.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "transport.Start: %v\n", err)
		os.Exit(1)
	}

	peerIDs := make([]int, 0, len(peerAddrs))
	for pid := range peerAddrs {
		peerIDs = append(peerIDs, pid)
	}
	node := paxos.NewNode(*id, peerIDs, transport)
	node.Start(context.Background())

	fmt.Fprintf(os.Stderr, "paxos node %d listening on %s (peers=%v)\n", *id, transport.Addr(), peerAddrs)
	fmt.Fprintln(os.Stderr, "type values on stdin; decided entries appear on stdout")

	go func() {
		for entry := range node.Committed() {
			fmt.Printf("slot=%d value=%s\n", entry.Slot, string(entry.Value))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if err := node.Propose(context.Background(), []byte(line)); err != nil {
				fmt.Fprintf(os.Stderr, "Propose: %v\n", err)
				return
			}
		}
		sigCh <- syscall.SIGTERM
	}()

	<-sigCh
	fmt.Fprintln(os.Stderr, "shutting down")
	node.Stop()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopCancel()
	_ = transport.Stop(stopCtx)
}

// parsePeers parses "id=host:port,id=host:port,..." and rejects entries that
// collide with selfID.
func parsePeers(s string, selfID int) (map[int]string, error) {
	out := map[int]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out, nil
	}
	for _, raw := range strings.Split(s, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		eq := strings.IndexByte(raw, '=')
		if eq <= 0 || eq == len(raw)-1 {
			return nil, fmt.Errorf("entry %q: expected id=host:port", raw)
		}
		id, err := strconv.Atoi(raw[:eq])
		if err != nil {
			return nil, fmt.Errorf("entry %q: bad id: %w", raw, err)
		}
		if id == selfID {
			return nil, fmt.Errorf("entry %q: peer id collides with -id", raw)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("entry %q: duplicate peer id %d", raw, id)
		}
		out[id] = strings.TrimSpace(raw[eq+1:])
	}
	return out, nil
}
