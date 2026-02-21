package paxos

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func TestProposalNumberMonotonicity(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "test", node, 1, 2, 3)

	prev := 0
	for i := 0; i < 100; i++ {
		p.seq++
		num := p.getProposerNumber()
		if num <= prev {
			t.Errorf("Proposal number not monotonically increasing: seq=%d produced %d, previous was %d", p.seq, num, prev)
		}
		prev = num
	}
}

func TestProposalNumberUniqueness(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100, 101, 200)

	node1 := env.GetNodeNetwork(100)
	node2 := env.GetNodeNetwork(101)
	node3 := env.GetNodeNetwork(200)

	p1 := NewProposer(100, "val1", node1, 1, 2, 3)
	p2 := NewProposer(101, "val2", node2, 1, 2, 3)
	p3 := NewProposer(200, "val3", node3, 1, 2, 3)

	seen := make(map[int]bool)
	proposers := []*Proposer{p1, p2, p3}
	for _, p := range proposers {
		for i := 0; i < 50; i++ {
			p.seq++
			num := p.getProposerNumber()
			if seen[num] {
				t.Errorf("Duplicate proposal number %d from proposer %d at seq %d", num, p.id, p.seq)
			}
			seen[num] = true
		}
	}
}

func TestProposalNumberLargeID(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 9999)
	node := env.GetNodeNetwork(9999)
	p := NewProposer(9999, "test", node, 1, 2, 3)

	prev := 0
	for i := 0; i < 10; i++ {
		p.seq++
		num := p.getProposerNumber()
		if num <= prev {
			t.Errorf("Proposal number not monotonic for large id=9999: seq=%d produced %d, previous was %d", p.seq, num, prev)
		}
		prev = num
	}
}

func TestMajority(t *testing.T) {
	tests := []struct {
		numAcceptors    int
		expectedMajority int
	}{
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{5, 3},
		{6, 4},
		{7, 4},
	}

	for _, tt := range tests {
		acceptorIDs := make([]int, tt.numAcceptors)
		nodeIDs := make([]int, tt.numAcceptors+1)
		nodeIDs[0] = 100
		for i := 0; i < tt.numAcceptors; i++ {
			acceptorIDs[i] = i + 1
			nodeIDs[i+1] = i + 1
		}
		env := NewPaxosEnvironment(nodeIDs...)
		node := env.GetNodeNetwork(100)
		p := NewProposer(100, "test", node, acceptorIDs...)

		if p.majority() != tt.expectedMajority {
			t.Errorf("majority() with %d acceptors: got %d, want %d", tt.numAcceptors, p.majority(), tt.expectedMajority)
		}
	}
}

func TestReachedMajorityExact(t *testing.T) {
	// With 3 acceptors, majority is 2. Verify that exactly 2 promises suffices.
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "test", node, 1, 2, 3)

	p.seq = 1
	proposalNum := p.getProposerNumber()

	// Simulate 2 promises (exactly majority)
	p.acceptors[1] = messageData{messageNumber: proposalNum}
	p.acceptors[2] = messageData{messageNumber: proposalNum}

	if !p.reachedMajority() {
		t.Errorf("reachedMajority() should be true with exactly %d promises out of %d acceptors", 2, 3)
	}
}

func TestPrepareIncrementsSeqOnce(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "test", node, 1, 2, 3)

	p.prepare()
	seqAfterFirst := p.seq
	if seqAfterFirst != 1 {
		t.Errorf("seq after first prepare: got %d, want 1", seqAfterFirst)
	}

	p.prepare()
	seqAfterSecond := p.seq
	if seqAfterSecond != 2 {
		t.Errorf("seq after second prepare: got %d, want 2", seqAfterSecond)
	}
}

func TestPrepareSendsToAllAcceptors(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "test", node, 1, 2, 3)

	msgs := p.prepare()
	if len(msgs) != 3 {
		t.Errorf("prepare() should send to all 3 acceptors, got %d messages", len(msgs))
	}
	for _, msg := range msgs {
		if msg.messageCategory != PrepareMessage {
			t.Errorf("prepare() should produce PrepareMessage, got %d", msg.messageCategory)
		}
		if msg.messageNumber != p.proposalNumber {
			t.Errorf("message number %d doesn't match proposal number %d", msg.messageNumber, p.proposalNumber)
		}
	}
}

func TestPrepareResetsPromiseState(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "test", node, 1, 2, 3)

	// Simulate some promises from a previous round
	p.acceptors[1] = messageData{messageNumber: 5000}
	p.acceptors[2] = messageData{messageNumber: 5000}

	// prepare() should reset all acceptor state
	p.prepare()
	for id, msg := range p.acceptors {
		if msg.getMessageNumber() != 0 {
			t.Errorf("acceptor %d should be reset after prepare(), got messageNumber %d", id, msg.getMessageNumber())
		}
	}
}

func TestValueAdoptionP2c(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "my-value", node, 1, 2, 3)

	p.seq = 1
	p.getProposerNumber()

	// Acceptor 1 reports a previously accepted value with a high proposal number
	p.receivePromise(messageData{
		messageSender:   1,
		messageNumber:   5001, // previously accepted proposal number
		messageCategory: AckMessage,
		value:           "adopted-value",
	})

	// Acceptor 2 reports no previously accepted value (number matches current proposal)
	p.receivePromise(messageData{
		messageSender:   2,
		messageNumber:   10100, // current proposal number
		messageCategory: AckMessage,
		value:           "my-value",
	})

	// P2c: proposer should adopt "adopted-value" because 5001 < 10100 but
	// the highest value across all promises determines adoption
	// Actually: 10100 > 5001, so "my-value" has the higher number
	// Let's fix: make the accepted value have a higher number
	p2 := NewProposer(100, "original", node, 1, 2, 3)
	p2.seq = 2
	p2.getProposerNumber()

	// Acceptor 1 had previously accepted proposal 15001
	p2.receivePromise(messageData{
		messageSender:   1,
		messageNumber:   15001,
		messageCategory: AckMessage,
		value:           "higher-value",
	})

	// Acceptor 2 had previously accepted proposal 10002
	p2.receivePromise(messageData{
		messageSender:   2,
		messageNumber:   10002,
		messageCategory: AckMessage,
		value:           "lower-value",
	})

	if p2.proposalValue != "higher-value" {
		t.Errorf("P2c: proposer should adopt value from highest-numbered promise, got %q, want %q", p2.proposalValue, "higher-value")
	}
}

func TestProposeOnlyToPromisedAcceptors(t *testing.T) {
	env := NewPaxosEnvironment(1, 2, 3, 100)
	node := env.GetNodeNetwork(100)
	p := NewProposer(100, "test", node, 1, 2, 3)

	p.seq = 1
	p.getProposerNumber()

	// Only acceptors 1 and 2 promised
	p.acceptors[1] = messageData{messageNumber: p.proposalNumber, value: "test"}
	p.acceptors[2] = messageData{messageNumber: p.proposalNumber, value: "test"}
	// Acceptor 3 did not promise (zero value)

	msgs := p.propose()
	if len(msgs) != 2 {
		t.Errorf("propose() should send only to 2 promised acceptors, got %d", len(msgs))
	}
	for _, msg := range msgs {
		if msg.messageCategory != ProposeMessage {
			t.Errorf("propose() should produce ProposeMessage, got %d", msg.messageCategory)
		}
	}
}

func TestLeaderElectionHighestIDWins(t *testing.T) {
	env := NewPaxosEnvironment(100, 101)

	p1 := NewProposer(100, "val1", env.GetNodeNetwork(100))
	p2 := NewProposer(101, "val2", env.GetNodeNetwork(101))
	p1.SetPeers(101)
	p2.SetPeers(100)

	// Run elections concurrently.
	done := make(chan int, 2)
	go func() {
		p1.electLeader()
		done <- p1.id
	}()
	go func() {
		p2.electLeader()
		done <- p2.id
	}()

	// Wait for both to finish (election timeout ~500ms each).
	<-done
	<-done

	if p1.isLeader {
		t.Error("Proposer 100 (lower ID) should not be leader")
	}
	if !p2.isLeader {
		t.Error("Proposer 101 (higher ID) should be leader")
	}
}

func TestFollowerDefersToLeader(t *testing.T) {
	network := NewPaxosEnvironment(1, 2, 3, 100, 101, 200)

	// Create acceptors.
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

	// Proposer 100 (follower) proposes "from-follower".
	// Proposer 101 (leader) proposes "from-leader".
	proposer1 := NewProposer(100, "from-follower", network.GetNodeNetwork(100), 1, 2, 3)
	proposer2 := NewProposer(101, "from-leader", network.GetNodeNetwork(101), 1, 2, 3)
	proposer1.SetPeers(101)
	proposer2.SetPeers(100)

	go proposer1.Run()
	go proposer2.Run()

	// Only the leader (101) runs Paxos, so the learner must learn "from-leader".
	learner := NewLearner(200, network.GetNodeNetwork(200), 1, 2, 3)
	resultCh := make(chan string, 1)
	go func() {
		resultCh <- learner.Learn()
	}()

	select {
	case learnedValue := <-resultCh:
		slog.Info(fmt.Sprintf("Learner %d picked up value %s", learner.id, learnedValue))
		if learnedValue != "from-leader" {
			t.Errorf("Expected leader's value %q, got %q", "from-leader", learnedValue)
		}
	case <-time.After(10 * time.Second):
		learner.Stop()
		t.Fatal("TestFollowerDefersToLeader timed out")
	}
}

func TestLeaderFailover(t *testing.T) {
	// Proposer 100 has peer 101, but 101 never starts.
	// After the election window, 100 should become leader and achieve consensus.
	network := NewPaxosEnvironment(1, 2, 3, 100, 101, 200)

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

	proposer := NewProposer(100, "failover-value", network.GetNodeNetwork(100), 1, 2, 3)
	proposer.SetPeers(101)

	go proposer.Run()

	learner := NewLearner(200, network.GetNodeNetwork(200), 1, 2, 3)
	resultCh := make(chan string, 1)
	go func() {
		resultCh <- learner.Learn()
	}()

	select {
	case learnedValue := <-resultCh:
		slog.Info(fmt.Sprintf("Learner %d picked up value %s", learner.id, learnedValue))
		if learnedValue != "failover-value" {
			t.Errorf("Expected %q, got %q", "failover-value", learnedValue)
		}
	case <-time.After(10 * time.Second):
		learner.Stop()
		t.Fatal("TestLeaderFailover timed out")
	}
}
