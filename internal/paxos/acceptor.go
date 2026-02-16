package paxos

import (
	"fmt"
	"log/slog"
	"os"
)


// Acceptor
type Acceptor struct {
	id int
	learners []int

	acceptedMessage messageData
	promisedMessage messageData
	node            PaxosNode
}

func NewAcceptor(id int, node PaxosNode, learners ...int) *Acceptor {
	return &Acceptor{
		id: id,
		node: node,
		learners: learners,
		promisedMessage: messageData{},
	}
}

// Receive a proposal message and return if accepted or not
// Phase 2b: accept unless we have already promised a higher number
func (a *Acceptor) receiveProposeMessage(msg messageData) bool {
	if a.promisedMessage.getMessageNumber() > msg.getMessageNumber() {
		slog.Debug("Not taking proposed message",
			"Acceptor ID", a.id,
			"Proposal ID", msg.getMessageNumber(),
			"Promised ID", a.promisedMessage.getMessageNumber(),
		)
		return false
	}
	a.acceptedMessage = msg
	slog.Info("Accepted given proposed message",
		"Acceptor ID", a.id,
		"Proposal ID", msg.getMessageNumber(),
	)
	return true
}

// Receive message of category Prepared and return an Ack Message
func (a *Acceptor) receivePreparedMessage(msg messageData) *messageData {
	if a.promisedMessage.getMessageNumber() >= msg.getMessageNumber() {
		slog.Error("Already accepted a larger proposal value message",
			"Acceptor ID", a.id,
			"Accepted Proposal ID", a.promisedMessage.getMessageNumber(),
			"Request Proposal ID", msg.getMessageNumber(),
		)
		return nil
	}
	// Include previously accepted value (if any) so proposer can adopt it (P2c)
	ackValue := msg.value
	ackNumber := msg.messageNumber
	if a.acceptedMessage.getMessageNumber() > 0 {
		ackValue = a.acceptedMessage.value
		ackNumber = a.acceptedMessage.messageNumber
	}
	ack := messageData{
		messageSender: a.id,
		messageRecipient: msg.messageSender,
		messageNumber: ackNumber,
		value: ackValue,
		messageCategory: AckMessage,  // Promise
	}
	ack.printMessage("Inside receivePreparedMessage")
	a.promisedMessage = msg

	return &ack
}

func (a *Acceptor) Accept() {
	for{
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
						messageSender: a.id,
						messageRecipient: learnerID,
						messageCategory: AcceptMessage,
						messageNumber: message.messageNumber,
						value: message.value,
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
