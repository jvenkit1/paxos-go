package paxos

type messageType int

const (
	PrepareMessage messageType = iota + 1
	ProposeMessage  // propose a value - proposer - acceptor
	AcceptMessage  // accept a given value - acceptor - learner
	AckMessage  // promise response - acceptor - proposer
)

type messageData struct {
	messageSender int  // sender of the message
	messageRecipient int  // recipient of the message
	messageNumber int  // current Sequence number of the message
	messageCategory messageType
	value string  // value contained in the string

}

func (m *messageData) getProposalValue() string {
	return m.value
}

func (m *messageData) getMessageNumber() int {
	return m.messageNumber
}