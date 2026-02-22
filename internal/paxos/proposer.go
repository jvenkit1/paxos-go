package paxos

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

const electionTimeout = 500 * time.Millisecond

type Proposer struct {
	id             int
	seq            int
	proposalNumber int
	proposalValue  string
	acceptors      map[int]messageData
	node           PaxosNode
	peers          []int
	isLeader       bool
	slot           int
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
			messageSender:    p.id,
			messageRecipient: acceptorID,
			messageCategory:  PrepareMessage,
			messageNumber:    p.proposalNumber,
			value:            p.proposalValue,
			slot:             p.slot,
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
				messageSender:    p.id,
				messageRecipient: acceptorID,
				messageCategory:  ProposeMessage,
				messageNumber:    p.proposalNumber,
				value:            p.proposalValue,
				slot:             p.slot,
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

// SetPeers configures the other proposer IDs that participate in leader election.
// When peers are set, electLeader runs before the Paxos protocol.
// When no peers are set, election is skipped (backward compatible).
func (p *Proposer) SetPeers(peers ...int) {
	p.peers = peers
}

// electLeader implements a simple highest-alive-ID-wins election.
// Each proposer broadcasts a heartbeat to its peers and listens for
// the duration of electionTimeout. If a heartbeat from a higher-ID
// peer arrives, this proposer yields leadership.
func (p *Proposer) electLeader() {
	if len(p.peers) == 0 {
		p.isLeader = true
		return
	}

	// Broadcast heartbeat to all peers.
	for _, peerID := range p.peers {
		p.node.send(messageData{
			messageSender:    p.id,
			messageRecipient: peerID,
			messageCategory:  HeartbeatMessage,
		})
	}

	// Listen for heartbeats until the election window closes.
	p.isLeader = true
	deadline := time.Now().Add(electionTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		msg := p.node.receiveWithTimeout(remaining)
		if msg == nil {
			break
		}
		if msg.messageCategory == HeartbeatMessage && msg.messageSender > p.id {
			p.isLeader = false
		}
	}

	if p.isLeader {
		slog.Info("Elected as leader", "Proposer ID", p.id)
	} else {
		slog.Info("Deferring to higher-ID leader", "Proposer ID", p.id)
	}
}

func (p *Proposer) Run() {
	p.electLeader()
	if !p.isLeader {
		return
	}

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
		// Random backoff to reduce livelock probability with competing proposers
		backoff := time.Duration(rand.Intn(150)+50) * time.Millisecond
		time.Sleep(backoff)
	}

	// Phase 2a: send propose messages to acceptors that promised
	proposerMessageList := p.propose()
	for _, message := range proposerMessageList {
		p.node.send(message)
	}
}
