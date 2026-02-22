package paxos

import (
	"log/slog"
)

type Learner struct {
	id               int
	numAcceptors     int
	acceptedMessages map[int]map[int]messageData // slot -> acceptor ID -> messageData
	node             PaxosNode
	done             chan struct{}
}

func NewLearner(id int, node PaxosNode, acceptorIDList ...int) *Learner {
	return &Learner{
		id:               id,
		node:             node,
		numAcceptors:     len(acceptorIDList),
		acceptedMessages: make(map[int]map[int]messageData),
		done:             make(chan struct{}),
	}
}

func (l *Learner) Stop() {
	close(l.done)
}

func (l *Learner) majority() int {
	return l.numAcceptors/2 + 1
}

func (l *Learner) validateAcceptMessage(acceptedMessage messageData) {
	slot := acceptedMessage.slot
	if l.acceptedMessages[slot] == nil {
		l.acceptedMessages[slot] = make(map[int]messageData)
	}
	current := l.acceptedMessages[slot][acceptedMessage.messageSender]
	if current.getMessageNumber() < acceptedMessage.getMessageNumber() {
		l.acceptedMessages[slot][acceptedMessage.messageSender] = acceptedMessage
	}
}

func (l *Learner) chosen(slot int) (messageData, bool) {
	slotMessages, exists := l.acceptedMessages[slot]
	if !exists {
		return messageData{}, false
	}

	acceptedMessageCount := make(map[int]int)
	acceptedMessageMap := make(map[int]messageData)

	for _, message := range slotMessages {
		proposalNumber := message.getMessageNumber()
		if proposalNumber == 0 {
			continue // skip uninitialized entries
		}
		acceptedMessageCount[proposalNumber] += 1
		acceptedMessageMap[proposalNumber] = message
	}

	for number, message := range acceptedMessageMap {
		if acceptedMessageCount[number] >= l.majority() {
			return message, true
		}
	}

	return messageData{}, false
}

func (l *Learner) Learn() string {
	for {
		select {
		case <-l.done:
			return ""
		default:
		}
		msg := l.node.receive()
		if msg == nil {
			continue
		}
		l.validateAcceptMessage(*msg)
		learnedMessage, learned := l.chosen(msg.slot)
		if !learned {
			slog.Info("Learner hasn't learned anything yet.")
			continue
		}
		return learnedMessage.value
	}
}
