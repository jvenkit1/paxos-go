package paxos

import (
	"time"
)

/*
This file defines the network framework for the processes.
*/

type Environment struct {
	cfg *Config
	receiveQueues map[int]chan messageData
}

type PaxosNode struct {
	id int
	env *Environment
}

func NewPaxosEnvironment(nodes ...int) *Environment {
	env := Environment{
		receiveQueues: make(map[int]chan messageData, 0),
	}
	for _, node := range nodes {
		env.receiveQueues[node] = make(chan messageData, 1024)
	}
	return &env
}

func (env *Environment) GetNodeNetwork(id int) PaxosNode {
	return PaxosNode{
		id: id,
		env: env,
	}
}

// sends a message to the target node. Basically gets added to the correct channel
func (env *Environment) sendMessage(m messageData) {
	m.timestamp = time.Now().String()
	env.receiveQueues[m.messageRecipient] <- m
}

// Client represented by given id receives the message
func (env *Environment) receiveMessage(id int) *messageData{
	select {
	case msg := <-env.receiveQueues[id]:
		return &msg
	case <-time.After(time.Second):
		return nil
	}
}

func (node *PaxosNode) send(m messageData){
	//m.printMessage("Printing inside send()")
	node.env.sendMessage(m)
}

func (node *PaxosNode) receive() *messageData {
	return node.env.receiveMessage(node.id)
}