package paxos

import (
	"testing"
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
