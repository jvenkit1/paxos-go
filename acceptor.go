package paxos

import (
	"fmt"
	"github.com/sirupsen/logrus"
)


// Acceptor
type acceptor struct {
	id int
	learners []int

	acceptedMessage messageData
	promisedMessage messageData
	node paxosNode
}

func newAcceptor(id int, node paxosNode, learners ...int) *acceptor {
	return &acceptor{
		id: id,
		node: node,
		learners: learners,
		promisedMessage: messageData{},
	}
}

// Receive a proposal message and return if accepted or not
func (a *acceptor) receiveProposeMessage(msg messageData) bool {
	if a.acceptedMessage.getMessageNumber() < msg.getMessageNumber() || a.acceptedMessage.getMessageNumber() > msg.getMessageNumber() {
		logrus.WithFields(logrus.Fields{
			"Acceptor ID": a.id,
			"Proposal ID": msg.getMessageNumber(),
		}).Debug("Not taking proposed message")
		return false
	}
	logrus.WithFields(logrus.Fields{
		"Acceptor ID": a.id,
		"Proposal ID": msg.getMessageNumber(),
	}).Info("Accepted given proposed message")
	return true
}

// Receive message of category Prepared and return an Ack Message
func (a *acceptor) receivePreparedMessage(msg messageData) *messageData {
	if a.promisedMessage.getMessageNumber() >= msg.getMessageNumber() {
		logrus.WithFields(logrus.Fields{
			"Acceptor ID": a.id,
			"Accepted Proposal ID": a.promisedMessage.getMessageNumber(),
			"Request Proposal ID": msg.getMessageNumber(),
		}).Error("Already accepted a larger proposal value message")
		return nil
	}
	ack := messageData{
		messageSender: a.id,
		messageRecipient: msg.messageSender,
		messageNumber: msg.messageNumber,
		value: msg.value,
		messageCategory: AckMessage,  // Promise
	}
	ack.printMessage("Inside receivePreparedMessage")
	a.acceptedMessage = ack

	return &ack
}

func (a *acceptor) run() {
	for{
		logrus.Infof("Acceptor %d waiting for message", a.id)
		message := a.node.receive()
		if message == nil {
			// null message obtained
			logrus.Infof("Null obtained")
			return
		} else {
			logrus.Info("Not Null")
		}
		message.printMessage(fmt.Sprintf("Acceptor %d received message", a.id))
		switch message.messageCategory {
		case PrepareMessage:
			ack := a.receivePreparedMessage(*message)
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
			logrus.Fatalf("Sending unsupported message in acceptor %d", a.id)
		}
	}
}
