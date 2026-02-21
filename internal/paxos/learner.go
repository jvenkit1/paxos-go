package paxos

import (
	"log/slog"
)

type Learner struct {
	id               int
	acceptedMessages map[int]messageData
	node             PaxosNode
	done             chan struct{}
}

func NewLearner(id int, node PaxosNode, acceptorIDList ...int) *Learner {
	newLearner := Learner{
		id:   id,
		node: node,
		done: make(chan struct{}),
	}
	newLearner.acceptedMessages = make(map[int]messageData)
	for _, acceptorID := range acceptorIDList {
		newLearner.acceptedMessages[acceptorID] = messageData{}
	}
	return &newLearner
}

func (l *Learner) Stop() {
	close(l.done)
}

func (l *Learner) majority() int{
	return len(l.acceptedMessages)/2 + 1
}

func (l *Learner) validateAcceptMessage(acceptedMessage messageData) {
	currentAcceptedMessage := l.acceptedMessages[acceptedMessage.messageSender]  // checking if the latest accepted message from the sender is the current message
	if currentAcceptedMessage.getMessageNumber() < acceptedMessage.getMessageNumber() {
		l.acceptedMessages[acceptedMessage.messageSender] = acceptedMessage
	}
}

func (l *Learner) chosen() (messageData, bool) {
	acceptedMessageCount := make(map[int]int)
	acceptedMessageMap := make(map[int]messageData)

	for _, message := range l.acceptedMessages {
		proposalNumber := message.getMessageNumber()
		if proposalNumber == 0 {
			continue // skip uninitialized entries
		}
		acceptedMessageCount[proposalNumber]+=1
		acceptedMessageMap[proposalNumber]=message
	}

	for number, message := range acceptedMessageMap {
		if acceptedMessageCount[number] >= l.majority() {
			return message, true
		}
	}

	return messageData{}, false
}

func (l *Learner) Learn() string{
	for{
		select {
		case <-l.done:
			return ""
		default:
		}
		msg := l.node.receive()
		if msg==nil {
			continue
		}
		l.validateAcceptMessage(*msg)
		learnedMessage, learned := l.chosen()
		if !learned {
			slog.Info("Learner hasn't learned anything yet.")
			continue
		}
		return learnedMessage.value
	}
}
