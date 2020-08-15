package paxos

import (
	"github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestWithOneProposer(t *testing.T){
	network := NewPaxosEnvironment(1, 2, 3, 100, 200)
	inputString := "Hello World"

	// Create acceptors
	var acceptorList []Acceptor
	acceptorID := 1
	for acceptorID < 4 {
		node := network.getNodeNetwork(acceptorID)
		acceptorList = append(acceptorList, *NewAcceptor(acceptorID, node, 200))
		acceptorID+=1
	}

	// Create Proposer
	proposer := NewProposer(100, inputString, network.getNodeNetwork(100), 1, 2, 3)
	go proposer.run()

	for index :=range acceptorList {
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

func TestWithMultipleProposers(t *testing.T){
	network := NewPaxosEnvironment(1, 2, 3, 100, 101, 200)
	inputString1 := "Hello World"
	inputString2 := "Paxos"

	// Create acceptors
	var acceptorList []Acceptor
	acceptorID := 1
	for acceptorID < 4 {
		node := network.getNodeNetwork(acceptorID)
		acceptorList = append(acceptorList, *NewAcceptor(acceptorID, node, 200))
		acceptorID+=1
	}

	// Create Proposer 1
	proposer1 := NewProposer(100, inputString1, network.getNodeNetwork(100), 1, 2, 3)
	go proposer1.run()

	for index :=range acceptorList {
		go acceptorList[index].run()
	}

	// Create Proposer2
	proposer2 := NewProposer(101, inputString2, network.getNodeNetwork(101), 1, 2, 3)
	time.AfterFunc(time.Second, func() {
		proposer2.run()
	})

	for index :=range acceptorList {
		go acceptorList[index].run()
	}

	// Create learner.
	learner := NewLearner(200, network.getNodeNetwork(200), 1, 2, 3)
	learnedValue := learner.run()

	logrus.Infof("Learner %d picked up value %s", learner.id, learnedValue)

	if learnedValue != inputString1 {
		t.Errorf("Learner learned %v instead of %v", learnedValue, inputString1)
	}
}