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
