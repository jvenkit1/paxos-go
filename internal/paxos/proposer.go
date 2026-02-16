package paxos

import (
	"fmt"
	"log/slog"
	"os"
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
		slog.Info("Proposer information",
			"Proposer ID", p.id,
			"Acceptor Count", len(p.acceptors),
			"Current Proposal Number", p.proposalNumber,
			"Message Sequence Number", message.getMessageNumber(),
		)
		if message.getMessageNumber() == p.proposalNumber {
			promiseCount+=1
		}
	}
	return promiseCount
}

// consistency quorum
func (p *Proposer) reachedMajority() bool {
	return p.getPromiseCount() >= p.majority()
}

// send prepare message to all acceptors
func (p *Proposer) prepare() []messageData {
	p.seq+=1
	p.getProposerNumber()
	// Reset acceptor promise state for this new round
	for acceptorID := range p.acceptors {
		p.acceptors[acceptorID] = messageData{}
	}
	var messageList []messageData
	for acceptorID := range p.acceptors {
		message := messageData{
			messageSender: p.id,
			messageRecipient: acceptorID,
			messageCategory: PrepareMessage,
			messageNumber: p.proposalNumber,
			value: p.proposalValue,
		}
		messageList = append(messageList, message)
	}
	return messageList
}

// send propose message to acceptors that promised
func (p *Proposer) propose() []messageData {
	var messageList []messageData
	for acceptorID, acceptorMessage := range p.acceptors {
		if acceptorMessage.getMessageNumber() > 0 {
			message := messageData{
				messageSender: p.id,
				messageRecipient: acceptorID,
				messageCategory: ProposeMessage,
				messageNumber: p.proposalNumber,
				value: p.proposalValue,
			}
			messageList = append(messageList, message)
		}
	}
	return messageList
}

// receivePromise records a promise from an acceptor and adopts
// the highest-numbered previously accepted value (P2c invariant).
func (p *Proposer) receivePromise(promiseMessage messageData) {
	_, known := p.acceptors[promiseMessage.messageSender]
	if !known {
		return
	}
	p.acceptors[promiseMessage.messageSender] = promiseMessage

	// P2c: if the acceptor reports a previously accepted value,
	// adopt it if its proposal number is higher than any we've seen.
	if promiseMessage.getMessageNumber() > 0 && promiseMessage.value != "" {
		// Track highest accepted proposal number across all promises
		highestNum := 0
		highestVal := p.proposalValue
		for _, msg := range p.acceptors {
			if msg.getMessageNumber() > highestNum && msg.value != "" {
				highestNum = msg.getMessageNumber()
				highestVal = msg.value
			}
		}
		p.proposalValue = highestVal
	}
}

func (p *Proposer) Run() {
	for {
		// Phase 1a: send prepare messages
		messageList := p.prepare()
		for _, message := range messageList {
			p.node.send(message)
		}

		// Phase 1b: collect promises until majority or timeout
		for !p.reachedMajority() {
			msg := p.node.receive()
			if msg == nil {
				// Timeout — no more messages, re-prepare with higher number
				break
			}
			msg.printMessage("Proposer received message")
			if msg.messageCategory == AckMessage {
				slog.Info(fmt.Sprintf("Ack message received from %d", msg.messageSender))
				p.receivePromise(*msg)
			} else {
				slog.Error("Unsupported Message format")
				os.Exit(1)
			}
		}

		if p.reachedMajority() {
			break
		}
		// Did not reach majority — loop will re-prepare with higher seq
		slog.Info("Proposer did not reach majority, retrying",
			"Proposer ID", p.id,
			"Proposal Number", p.proposalNumber,
		)
	}

	// Phase 2a: send propose messages to acceptors that promised
	proposerMessageList := p.propose()
	for _, message := range proposerMessageList {
		p.node.send(message)
	}
}
