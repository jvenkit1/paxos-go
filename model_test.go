package paxos

import (
	"github.com/sirupsen/logrus"
	"testing"
)

func TestWithOneProposer(t *testing.T){
	network := newPaxosEnvironment(100, 1, 2, 3, 200)
	inputString := "Hello World"

	// Create acceptors
	var acceptorList []acceptor
	acceptorID := 1
	for acceptorID < 4 {
		node := network.getNodeNetwork(acceptorID)
		acceptorList = append(acceptorList, *newAcceptor(acceptorID, node, 200))
		acceptorID+=1
	}

	// Create Proposer
	proposer := NewProposer(100, inputString, network.getNodeNetwork(100), 1, 2, 3)
	go proposer.run()

	for index, _ :=range acceptorList {
		go acceptorList[index].run()
	}

	// Create learner.
	learner := NewLearner(200, network.getNodeNetwork(200), 1, 2, 3)
	learnedValue := learner.run()

	logrus.Infof("Learner %d picked up value %s", learner.id, learnedValue)

	if learnedValue != inputString {
		t.Errorf("Learner learned wrong proposal")
	}
}