package paxos

import (
	"fmt"
	"log/slog"
	"os"
)

// Acceptor
type Acceptor struct {
	id       int
	learners []int

	acceptedMessages map[int]messageData // key: slot
	promisedMessages map[int]messageData // key: slot
	node             nodeNetwork
	done             chan struct{}
}

func NewAcceptor(id int, node nodeNetwork, learners ...int) *Acceptor {
	return &Acceptor{
		id:               id,
		node:             node,
		learners:         learners,
		promisedMessages: make(map[int]messageData),
		acceptedMessages: make(map[int]messageData),
		done:             make(chan struct{}),
	}
}

func (a *Acceptor) Stop() {
	close(a.done)
}

// Receive a proposal message and return if accepted or not
// Phase 2b: accept unless we have already promised a higher number
func (a *Acceptor) receiveProposeMessage(msg messageData) bool {
	slot := msg.slot
	promised := a.promisedMessages[slot]
	if promised.getMessageNumber() > msg.getMessageNumber() {
		slog.Debug("Not taking proposed message",
			"Acceptor ID", a.id,
			"Slot", slot,
			"Proposal ID", msg.getMessageNumber(),
			"Promised ID", promised.getMessageNumber(),
		)
		return false
	}
	a.acceptedMessages[slot] = msg
	slog.Info("Accepted given proposed message",
		"Acceptor ID", a.id,
		"Slot", slot,
		"Proposal ID", msg.getMessageNumber(),
	)
	return true
}

// Receive message of category Prepared and return an Ack Message
func (a *Acceptor) receivePreparedMessage(msg messageData) *messageData {
	slot := msg.slot
	promised := a.promisedMessages[slot]
	if promised.getMessageNumber() >= msg.getMessageNumber() {
		slog.Error("Already accepted a larger proposal value message",
			"Acceptor ID", a.id,
			"Slot", slot,
			"Accepted Proposal ID", promised.getMessageNumber(),
			"Request Proposal ID", msg.getMessageNumber(),
		)
		return nil
	}
	// Include previously accepted value (if any) so proposer can adopt it (P2c)
	accepted := a.acceptedMessages[slot]
	ackValue := msg.value
	ackNumber := msg.messageNumber
	if accepted.getMessageNumber() > 0 {
		ackValue = accepted.value
		ackNumber = accepted.messageNumber
	}
	ack := messageData{
		messageSender:    a.id,
		messageRecipient: msg.messageSender,
		messageNumber:    ackNumber,
		value:            ackValue,
		messageCategory:  AckMessage, // Promise
		slot:             slot,
	}
	ack.printMessage("Inside receivePreparedMessage")
	a.promisedMessages[slot] = msg

	return &ack
}

func (a *Acceptor) Accept() {
	for {
		select {
		case <-a.done:
			return
		default:
		}
		slog.Info(fmt.Sprintf("Acceptor %d waiting for message", a.id))
		message := a.node.receive()
		if message == nil {
			// null message obtained
			continue
		}
		message.printMessage(fmt.Sprintf("Acceptor %d received message", a.id))
		switch message.messageCategory {
		case PrepareMessage:
			ack := a.receivePreparedMessage(*message)
			if ack == nil {
				continue
			}
			ack.printMessage("Sending ACK message")
			a.node.send(*ack)
			continue
		case ProposeMessage:
			acceptedMessage := a.receiveProposeMessage(*message)
			if acceptedMessage == true {
				// send to all learners
				for _, learnerID := range a.learners {
					sendMessage := messageData{
						messageSender:    a.id,
						messageRecipient: learnerID,
						messageCategory:  AcceptMessage,
						messageNumber:    message.messageNumber,
						value:            message.value,
						slot:             message.slot,
					}
					sendMessage.printMessage(fmt.Sprintf("Sending message to learner %d", learnerID))
					a.node.send(sendMessage)
				}
			}
		default:
			slog.Error(fmt.Sprintf("Sending unsupported message in acceptor %d", a.id))
			os.Exit(1)
		}
	}
}
