package paxos

import "log/slog"

type messageType int

const (
	PrepareMessage messageType = iota + 1
	ProposeMessage  // propose a value - proposer - acceptor
	AcceptMessage  // accept a given value - acceptor - learner
	AckMessage  // promise response - acceptor - proposer
)

var messages [4]string

type messageData struct {
	messageSender int  // sender of the message
	messageRecipient int  // recipient of the message
	messageNumber int  // current Sequence number of the message
	messageCategory messageType
	value string  // value contained in the string
	timestamp string
}

func init() {
	messages[0] = "PrepareMessage"
	messages[1] = "ProposeMessage"
	messages[2] = "AcceptMessage"
	messages[3] = "AckMessage"
}

func (m *messageData) getProposalValue() string {
	return m.value
}

func (m *messageData) getMessageNumber() int {
	return m.messageNumber
}

func (m *messageData) printMessage(str string) {
	slog.Info(str,
		"Source", m.messageSender,
		"Destination", m.messageRecipient,
		"Value", m.value,
		"Sequence Number", m.messageNumber,
		"Category", messages[m.messageCategory-1],
	)
}
