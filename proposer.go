package paxos

import (
	"github.com/sirupsen/logrus"
)

type Proposer struct {
	id             int
	seq            int
	proposalNumber int
	proposalValue  string
	acceptors      map[int]messageData
	node           PaxosNode
}

func NewProposer(id int, value string, node PaxosNode, acceptors ...int) *Proposer{
	newProposer := Proposer{
		id: id,
		seq: 0,
		proposalValue: value,
		node: node,
	}
	newProposer.acceptors = make(map[int]messageData, len(acceptors))
	for _, acceptor := range acceptors {
		newProposer.acceptors[acceptor] = messageData{}
	}
	return &newProposer
}

func (p *Proposer) majority() int {
	return len(p.acceptors)/2 + 1
}

const maxNodes = 10000

func (p *Proposer) getProposerNumber() int {
	p.proposalNumber = p.seq*maxNodes + p.id
	return p.proposalNumber
}

func (p *Proposer) getPromiseCount() int {
	promiseCount := 0
	for _, message := range p.acceptors {
		logrus.WithFields(logrus.Fields{
			"Proposer ID": p.id,
			"Acceptor Count": len(p.acceptors),
			"Current Proposal Number": p.getProposerNumber(),
			"Message Sequence Number": message.getMessageNumber(),
		}).Info("Proposer information")
		if message.getMessageNumber() == p.getProposerNumber() {
			promiseCount+=1
		}
	}
	return promiseCount
}

// consistency quorum
func (p *Proposer) reachedMajority() bool {
	return p.getPromiseCount() >= p.majority()
}

// send prepare message to the majority of acceptors
func (p *Proposer) prepare() []messageData {
	p.seq+=1
	sentCount := 0
	var messageList []messageData
	for acceptorID, _ := range p.acceptors {
		message := messageData{
			messageSender: p.id,
			messageRecipient: acceptorID,
			messageCategory: PrepareMessage,
			messageNumber: p.getProposerNumber(),
			value: p.proposalValue,
		}
		messageList = append(messageList, message)
		sentCount+=1
		if sentCount >= p.majority() {
			break
		}
	}
	return messageList
}

// send propose message to the majority of acceptors
func (p *Proposer) propose() []messageData {
	sentCount := 0
	var messageList []messageData
	for acceptorID, acceptorMessage := range p.acceptors {
		if acceptorMessage.getMessageNumber() == p.getProposerNumber() {
			message := messageData{
				messageSender: p.id,
				messageRecipient: acceptorID,
				messageCategory: ProposeMessage,
				messageNumber: p.getProposerNumber(),
				value: p.proposalValue,
			}
			messageList = append(messageList, message)
			sentCount += 1
		}
		if sentCount >= p.majority() {
			break
		}
	}
	return messageList
}

func (p *Proposer) receivePromise(promiseMessage messageData) {
	promise := p.acceptors[promiseMessage.messageSender]
	if promise.getMessageNumber() < promiseMessage.getMessageNumber() {
		p.acceptors[promiseMessage.messageSender] = promiseMessage
		if promiseMessage.getMessageNumber() > p.getProposerNumber() {
			p.proposalNumber = promiseMessage.getMessageNumber()
			p.proposalValue = promiseMessage.getProposalValue()
		}
	}
}

func (p *Proposer) Run() {
	for !p.reachedMajority() {
		messageList := p.prepare()
		for _, message := range messageList {
			p.node.send(message)
		}

		msg := p.node.receive()
		if msg==nil {
			continue
		}
		msg.printMessage("Proposer received message")
		if msg.messageCategory == AckMessage {
			logrus.Infof("Ack message received from %d", msg.messageSender)
			p.receivePromise(*msg)
		}else {
			logrus.Fatal("Unsupported Message format")
		}
	}
	// Majority has been reached
	// Proposer now sends message to the acceptor
	proposerMessageList := p.propose()
	for _, message := range proposerMessageList {
		p.node.send(message)
	}
}