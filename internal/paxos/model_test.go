package paxos

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func TestWithOneProposer(t *testing.T){
	network := NewPaxosEnvironment(1, 2, 3, 100, 200)
	inputString := "Hello World"

	// Create acceptors
	var acceptorList []*Acceptor
	for id := 1; id <= 3; id++ {
		node := network.GetNodeNetwork(id)
		acceptorList = append(acceptorList, NewAcceptor(id, node, 200))
	}

	// Start acceptors
	for _, acc := range acceptorList {
		go acc.Accept()
	}

	// Clean up acceptors when test ends
	defer func() {
		for _, acc := range acceptorList {
			acc.Stop()
		}
	}()

	// Create Proposer
	proposer := NewProposer(100, inputString, network.GetNodeNetwork(100), 1, 2, 3)
	go proposer.Run()

	// Create learner with timeout
	learner := NewLearner(200, network.GetNodeNetwork(200), 1, 2, 3)

	resultCh := make(chan string, 1)
	go func() {
		resultCh <- learner.Learn()
	}()

	select {
	case learnedValue := <-resultCh:
		slog.Info(fmt.Sprintf("Learner %d picked up value %s", learner.id, learnedValue))
		if learnedValue != inputString {
			t.Errorf("Learner learned %q, want %q", learnedValue, inputString)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("TestWithOneProposer timed out after 10 seconds")
	}
}

func TestWithMultipleProposers(t *testing.T){
	network := NewPaxosEnvironment(1, 2, 3, 100, 101, 200)
	inputString1 := "Hello World"
	inputString2 := "Paxos"

	// Create acceptors
	var acceptorList []*Acceptor
	for id := 1; id <= 3; id++ {
		node := network.GetNodeNetwork(id)
		acceptorList = append(acceptorList, NewAcceptor(id, node, 200))
	}

	// Start acceptors — only once
	for _, acc := range acceptorList {
		go acc.Accept()
	}

	// Clean up acceptors when test ends
	defer func() {
		for _, acc := range acceptorList {
			acc.Stop()
		}
	}()

	// Create and start Proposer 1
	proposer1 := NewProposer(100, inputString1, network.GetNodeNetwork(100), 1, 2, 3)
	go proposer1.Run()

	// Create and start Proposer 2 after a short delay
	proposer2 := NewProposer(101, inputString2, network.GetNodeNetwork(101), 1, 2, 3)
	time.AfterFunc(100*time.Millisecond, func() {
		proposer2.Run()
	})

	// Create learner with timeout
	learner := NewLearner(200, network.GetNodeNetwork(200), 1, 2, 3)

	resultCh := make(chan string, 1)
	go func() {
		resultCh <- learner.Learn()
	}()

	select {
	case learnedValue := <-resultCh:
		slog.Info(fmt.Sprintf("Learner %d picked up value %s", learner.id, learnedValue))
		if learnedValue != inputString1 && learnedValue != inputString2 {
			t.Errorf("Learner learned unexpected value %q; want %q or %q",
				learnedValue, inputString1, inputString2)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("TestWithMultipleProposers timed out after 15 seconds")
	}
}
