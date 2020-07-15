package paxos

type messageType int

const (
	Prepare messageType = iota + 1
	ProposeMessage  // propose a value - proposer - acceptor
	AcceptMessage  // accept a given value - acceptor - learner
	AckMessage  // promise response - acceptor - proposer
)

type messageData struct {
	messageSender int32  // sender of the message
	messageRecipient int32  // recipient of the message
	messageNumber int32  // current Sequence number of the message
	messageCategory messageType
	value string  // value contained in the string

}

func (m *messageData) getProposalValue() string {
	return m.value
}

func (m *messageData) getMessageNumber() int32 {
	return m.messageNumber
}