package paxos

import (
	"github.com/sirupsen/logrus"
)

/*
This file defines the network framework for the processes.
*/

type environment struct {
	cfg *Config
	receiveQueues map[int]chan messageData
}

type network interface {
	sendMessage()
	recvMessage()
	addProcess()
	removeProcess()
}

type paxosNode struct {
	id int
	env *environment
}


// Paxos network must be composed of

func newPaxosEnvironment(nodes ...int) *environment {
	env := environment{
		receiveQueues: make(map[int]chan messageData, 0),
	}
	for _, node := range nodes {
		env.receiveQueues[node] = make(chan messageData, 1024)
	}
	return &env
}

func (env *environment) getNodeNetwork(id int) paxosNode {
	return paxosNode{
		id: id,
		env: env,
	}
}

// sends a message to the target node. Basically gets added to the correct channel
func (env *environment) sendMessage(m messageData) {
	logrus.WithFields(logrus.Fields{
		"Source": m.messageSender,
		"Destination": m.messageRecipient,
		"Value": m.value,
		"Type": m.messageCategory,
	}).Info("Sending message")
	env.receiveQueues[m.messageRecipient] <- m
}

// Client represented by given id receives the message
func (env *environment) receiveMessage(id int) *messageData{
	select {
	case msg := <-env.receiveQueues[id]:
		logrus.WithFields(logrus.Fields{
			"Source": msg.messageSender,
			"Destination": msg.messageRecipient,
			"Value": msg.value,
			"Type": msg.messageCategory,
		}).Info("Received message")
		return &msg
	//case <-time.After(time.Second):
	//	return nil
	}
}

func (node *paxosNode) send(m messageData){
	node.env.sendMessage(m)
}

func (node *paxosNode) receive() *messageData {
	return node.env.receiveMessage(node.id)
}