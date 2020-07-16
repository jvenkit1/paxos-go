package paxos

import (
	"github.com/sirupsen/logrus"
)


// Acceptor
type acceptor struct {
	ballotNum int
	id int
	learners []int

	acceptedMessage messageData
	promisedMessage messageData
	node paxosNode
}

func newAcceptor(ballotNum int, id int, node paxosNode, learners ...int) *acceptor {
	return &acceptor{
		ballotNum: ballotNum,
		id: id,
		node: node,
		learners: learners,
		promisedMessage: messageData{},
	}
}

// Receive a proposal message and return if accepted or not
func (a *acceptor) receiveProposeMessage(msg messageData) bool {
	if a.acceptedMessage.getProposalValue() != msg.getProposalValue() {
		logrus.WithFields(logrus.Fields{
			"Acceptor ID": a.id,
			"Proposal ID": msg.getProposalValue(),
		}).Debug("Not taking proposed message")
		return false
	}
	logrus.WithFields(logrus.Fields{
		"Acceptor ID": a.id,
		"Proposal ID": msg.getProposalValue(),
	}).Info("Accepted given proposed message")
	return true
}

// Receive message of category Prepared and return an Ack Message
func (a *acceptor) receivePreparedMessage(msg messageData) *messageData {
	if a.promisedMessage.getProposalValue() >= msg.getProposalValue() {
		logrus.WithFields(logrus.Fields{
			"Acceptor ID": a.id,
			"Accepted Proposal ID": a.promisedMessage.getProposalValue(),
			"Request Proposal ID": msg.getProposalValue(),
		}).Debug("Already accepted a larger proposal value message")
		return nil
	}
	var ack messageData
	ack.messageSender = a.id
	ack.messageRecipient = msg.messageSender
	ack.messageNumber = msg.messageNumber
	ack.value = msg.value
	ack.messageCategory = AckMessage  // Promise
	a.acceptedMessage = ack

	return &ack
}

func (a *acceptor) run() {
	for{
		logrus.Info("Acceptor %d waiting for message", a.id)
		message := a.node.receive()
		if message == nil {
			// null message obtained
			continue
		}
		switch message.messageCategory {
		case PrepareMessage:
			ack := a.receivePreparedMessage(*message)
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
					a.node.send(sendMessage)
				}
			}
		default:
			logrus.Fatal("Sending unsupported message in acceptor %d", a.id)
		}
	}
}
