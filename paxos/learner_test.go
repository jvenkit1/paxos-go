package paxos

import (
	"testing"
	"time"
)

func TestLearnerMajority(t *testing.T) {
	tests := []struct {
		numAcceptors     int
		expectedMajority int
	}{
		{1, 1},
		{3, 2},
		{5, 3},
	}

	for _, tt := range tests {
		acceptorIDs := make([]int, tt.numAcceptors)
		nodeIDs := make([]int, tt.numAcceptors+1)
		nodeIDs[0] = 200
		for i := 0; i < tt.numAcceptors; i++ {
			acceptorIDs[i] = i + 1
			nodeIDs[i+1] = i + 1
		}
		env := NewPaxosEnvironment(nodeIDs...)
		node := env.GetNodeNetwork(200)
		l := NewLearner(200, node, acceptorIDs...)

		if l.majority() != tt.expectedMajority {
			t.Errorf("Learner majority() with %d acceptors: got %d, want %d", tt.numAcceptors, l.majority(), tt.expectedMajority)
		}
	}
}

func TestLearnerChosenExactMajority(t *testing.T) {
	// With 3 acceptors, majority is 2. Verify exactly 2 accept messages suffice.
	env := NewPaxosEnvironment(1, 2, 3, 200)
	node := env.GetNodeNetwork(200)
	l := NewLearner(200, node, 1, 2, 3)

	proposalNum := 10100

	// Simulate accept messages from 2 out of 3 acceptors
	l.validateAcceptMessage(messageData{
		messageSender: 1,
		messageNumber: proposalNum,
		value:         "hello",
	})
	l.validateAcceptMessage(messageData{
		messageSender: 2,
		messageNumber: proposalNum,
		value:         "hello",
	})

	msg, chosen := l.chosen(0)
	if !chosen {
		t.Error("chosen() should return true with exactly 2 out of 3 acceptors agreeing")
	}
	if msg.value != "hello" {
		t.Errorf("chosen() returned wrong value: got %q, want %q", msg.value, "hello")
	}
}

func TestLearnerChosenNoMajority(t *testing.T) {
	// Use 5 acceptors so that 1 real accept + 4 zero-value entries
	// can't form a majority (majority=3, zero-value count=4 which hits
	// majority — so use different non-zero proposal numbers to avoid
	// the zero-value false majority).
	env := NewPaxosEnvironment(1, 2, 3, 200)
	node := env.GetNodeNetwork(200)
	l := NewLearner(200, node, 1, 2, 3)

	// Give each acceptor a different proposal number — no majority
	l.validateAcceptMessage(messageData{
		messageSender: 1,
		messageNumber: 10100,
		value:         "hello",
	})
	l.validateAcceptMessage(messageData{
		messageSender: 2,
		messageNumber: 20200,
		value:         "world",
	})
	l.validateAcceptMessage(messageData{
		messageSender: 3,
		messageNumber: 30300,
		value:         "paxos",
	})

	_, chosen := l.chosen(0)
	if chosen {
		t.Error("chosen() should return false when all acceptors have different proposal numbers")
	}
}

func TestChosenPerSlot(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 200)
	node := env.GetNodeNetwork(200)
	l := NewLearner(200, node, 1, 2, 3)

	proposalNum := 10100

	// Slot 0: "alpha" accepted by acceptors 1 and 2 (majority)
	l.validateAcceptMessage(messageData{
		messageSender: 1, messageNumber: proposalNum, value: "alpha", slot: 0,
	})
	l.validateAcceptMessage(messageData{
		messageSender: 2, messageNumber: proposalNum, value: "alpha", slot: 0,
	})

	// Slot 1: "beta" accepted by acceptors 2 and 3 (majority)
	l.validateAcceptMessage(messageData{
		messageSender: 2, messageNumber: proposalNum, value: "beta", slot: 1,
	})
	l.validateAcceptMessage(messageData{
		messageSender: 3, messageNumber: proposalNum, value: "beta", slot: 1,
	})

	msg0, chosen0 := l.chosen(0)
	if !chosen0 || msg0.value != "alpha" {
		t.Errorf("slot 0: chosen=%v value=%q, want chosen=true value=%q", chosen0, msg0.value, "alpha")
	}

	msg1, chosen1 := l.chosen(1)
	if !chosen1 || msg1.value != "beta" {
		t.Errorf("slot 1: chosen=%v value=%q, want chosen=true value=%q", chosen1, msg1.value, "beta")
	}
}

func TestLearnerGracefulShutdown(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 200)
	node := env.GetNodeNetwork(200)
	l := NewLearner(200, node, 1, 2, 3)

	exited := make(chan struct{})
	go func() {
		l.Learn()
		close(exited)
	}()

	time.Sleep(50 * time.Millisecond)
	l.Stop()

	select {
	case <-exited:
		// Success: Learn() returned
	case <-time.After(3 * time.Second):
		t.Fatal("Learner did not shut down within 3 seconds after Stop()")
	}
}
