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

func TestMultiDecree(t *testing.T) {
	network := NewPaxosEnvironment(1, 2, 3, 100, 200)

	// Create acceptors
	var acceptorList []*Acceptor
	for id := 1; id <= 3; id++ {
		node := network.GetNodeNetwork(id)
		acceptorList = append(acceptorList, NewAcceptor(id, node, 200))
	}
	for _, acc := range acceptorList {
		go acc.Accept()
	}
	defer func() {
		for _, acc := range acceptorList {
			acc.Stop()
		}
	}()

	// Create proposer and submit 3 values
	proposer := NewProposer(100, "", network.GetNodeNetwork(100), 1, 2, 3)
	proposer.Submit("alpha")
	proposer.Submit("beta")
	proposer.Submit("gamma")
	proposer.Close()

	go proposer.RunMulti()

	// Create learner to collect all 3 slots
	learner := NewLearner(200, network.GetNodeNetwork(200), 1, 2, 3)
	resultCh := make(chan map[int]string, 1)
	go func() {
		resultCh <- learner.LearnMulti(3)
	}()

	select {
	case decided := <-resultCh:
		expected := map[int]string{0: "alpha", 1: "beta", 2: "gamma"}
		for slot, want := range expected {
			got, ok := decided[slot]
			if !ok {
				t.Errorf("slot %d: not decided", slot)
			} else if got != want {
				t.Errorf("slot %d: got %q, want %q", slot, got, want)
			}
		}
	case <-time.After(15 * time.Second):
		learner.Stop()
		t.Fatal("TestMultiDecree timed out after 15 seconds")
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

	// Create proposers with leader election.
	// Proposer 101 (higher ID) wins election and proposes inputString2.
	proposer1 := NewProposer(100, inputString1, network.GetNodeNetwork(100), 1, 2, 3)
	proposer2 := NewProposer(101, inputString2, network.GetNodeNetwork(101), 1, 2, 3)
	proposer1.SetPeers(101)
	proposer2.SetPeers(100)

	go proposer1.Run()
	go proposer2.Run()

	// Create learner with timeout
	learner := NewLearner(200, network.GetNodeNetwork(200), 1, 2, 3)

	resultCh := make(chan string, 1)
	go func() {
		resultCh <- learner.Learn()
	}()

	select {
	case learnedValue := <-resultCh:
		slog.Info(fmt.Sprintf("Learner %d picked up value %s", learner.id, learnedValue))
		// With leader election, proposer 101 (higher ID) wins and proposes inputString2.
		if learnedValue != inputString2 {
			t.Errorf("Learner learned %q, want %q (from leader proposer 101)",
				learnedValue, inputString2)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("TestWithMultipleProposers timed out after 15 seconds")
	}
}
