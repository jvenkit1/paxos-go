package paxos

import (
	"time"
)

/*
This file defines the network framework for the processes.
*/

type Environnment struct {
	cfg *Config
	receiveQueues map[int]chan messageData
}

type PaxosNode struct {
	id int
	env *Environnment
}

func NewPaxosEnvironment(nodes ...int) *Environnment {
	env := Environnment{
		receiveQueues: make(map[int]chan messageData, 0),
	}
	for _, node := range nodes {
		env.receiveQueues[node] = make(chan messageData, 1024)
	}
	return &env
}

func (env *Environnment) GetNodeNetwork(id int) PaxosNode {
	return PaxosNode{
		id: id,
		env: env,
	}
}

// sends a message to the target node. Basically gets added to the correct channel
func (env *Environnment) sendMessage(m messageData) {
	m.timestamp = time.Now().String()
	env.receiveQueues[m.messageRecipient] <- m
}

// Client represented by given id receives the message
func (env *Environnment) receiveMessage(id int) *messageData{
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