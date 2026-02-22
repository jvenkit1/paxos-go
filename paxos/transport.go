package paxos

import "context"

// MessageType identifies the kind of Paxos protocol message.
type MessageType int

const (
	PrepareMsg   MessageType = iota + 1
	ProposeMsg
	AcceptMsg
	AckMsg
	HeartbeatMsg
)

// Message is the public, transport-level representation of a Paxos message.
type Message struct {
	From   int
	To     int
	Type   MessageType
	Number int
	Value  []byte
	Slot   int
}

// Entry represents a decided value for a given slot.
type Entry struct {
	Slot  int
	Value []byte
}

// Transport is the pluggable networking interface for Paxos nodes.
type Transport interface {
	Send(msg Message) error
	Receive(ctx context.Context) (Message, error)
}

func toPublicMessage(m messageData) Message {
	return Message{
		From:   m.messageSender,
		To:     m.messageRecipient,
		Type:   MessageType(m.messageCategory),
		Number: m.messageNumber,
		Value:  []byte(m.value),
		Slot:   m.slot,
	}
}

func toInternalMessage(m Message) messageData {
	return messageData{
		messageSender:    m.From,
		messageRecipient: m.To,
		messageCategory:  messageType(m.Type),
		messageNumber:    m.Number,
		value:            string(m.Value),
		slot:             m.Slot,
	}
}
