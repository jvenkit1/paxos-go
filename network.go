package paxos

import (
	"time"
)

/*
This file defines the network framework for the processes.
*/

type environment struct {
	cfg *Config
	receiveQueues map[int]chan messageData
}

type paxosNode struct {
	id int
	env *environment
}

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
	m.timestamp = time.Now().String()
	env.receiveQueues[m.messageRecipient] <- m
}

// Client represented by given id receives the message
func (env *environment) receiveMessage(id int) *messageData{
	select {
	case msg := <-env.receiveQueues[id]:
		return &msg
	case <-time.After(time.Second):
		return nil
	}
}

func (node *paxosNode) send(m messageData){
	//m.printMessage("Printing inside send()")
	node.env.sendMessage(m)
}

func (node *paxosNode) receive() *messageData {
	return node.env.receiveMessage(node.id)
}