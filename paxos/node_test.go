package paxos

import (
	"context"
	"testing"
	"time"
)

func TestNodeSingleValue(t *testing.T) {
	ids := []int{1, 2, 3}
	transports := NewChannelTransportGroup(ids...)

	nodes := make(map[int]*Node)
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
	for _, node := range nodes {
		node.Start(ctx)
	}
	defer func() {
		for _, node := range nodes {
			node.Stop()
		}
	}()

	// Wait for leader election
	time.Sleep(700 * time.Millisecond)

	// Node 3 (highest ID) is the leader
	if err := nodes[3].Propose(ctx, []byte("hello")); err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	select {
	case entry := <-nodes[3].Committed():
		if entry.Slot != 0 {
			t.Errorf("slot = %d, want 0", entry.Slot)
		}
		if string(entry.Value) != "hello" {
			t.Errorf("value = %q, want %q", string(entry.Value), "hello")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for committed entry")
	}
}

func TestNodeMultiDecree(t *testing.T) {
	ids := []int{1, 2, 3}
	transports := NewChannelTransportGroup(ids...)

	nodes := make(map[int]*Node)
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
	for _, node := range nodes {
		node.Start(ctx)
	}
	defer func() {
		for _, node := range nodes {
			node.Stop()
		}
	}()

	time.Sleep(700 * time.Millisecond)

	// Propose 3 values on the leader (node 3)
	values := []string{"alpha", "beta", "gamma"}
	for _, v := range values {
		if err := nodes[3].Propose(ctx, []byte(v)); err != nil {
			t.Fatalf("Propose(%q) failed: %v", v, err)
		}
	}

	// Collect 3 entries
	decided := make(map[int]string)
	for i := 0; i < 3; i++ {
		select {
		case entry := <-nodes[3].Committed():
			decided[entry.Slot] = string(entry.Value)
		case <-time.After(15 * time.Second):
			t.Fatalf("timed out after %d entries", i)
		}
	}

	expected := map[int]string{0: "alpha", 1: "beta", 2: "gamma"}
	for slot, want := range expected {
		got, ok := decided[slot]
		if !ok {
			t.Errorf("slot %d: not decided", slot)
		} else if got != want {
			t.Errorf("slot %d: got %q, want %q", slot, got, want)
		}
	}
}

func TestNodeLeaderElection(t *testing.T) {
	ids := []int{1, 2}
	transports := NewChannelTransportGroup(ids...)

	node1 := NewNode(1, []int{2}, transports[1])
	node2 := NewNode(2, []int{1}, transports[2])

	ctx := context.Background()
	node1.Start(ctx)
	node2.Start(ctx)
	defer func() {
		node1.Stop()
		node2.Stop()
	}()

	// Wait for election
	time.Sleep(700 * time.Millisecond)

	// Node 2 (higher ID) wins election
	if err := node2.Propose(ctx, []byte("from-leader")); err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	// Both nodes should see the committed value
	for _, nd := range []*Node{node1, node2} {
		select {
		case entry := <-nd.Committed():
			if string(entry.Value) != "from-leader" {
				t.Errorf("node %d: value = %q, want %q", nd.id, string(entry.Value), "from-leader")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("node %d: timed out waiting for committed entry", nd.id)
		}
	}
}
