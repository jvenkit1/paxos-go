package paxos

import (
	"github.com/sirupsen/logrus"
)

type learner struct {
	id int
	acceptedMessages map[int]messageData
	node paxosNode
}

func NewLearner(id int, node paxosNode, acceptorIDList ...int) *learner {
	newLearner := learner{
		id: id,
		node: node,
	}
	newLearner.acceptedMessages = make(map[int]messageData)
	for _, acceptorID := range acceptorIDList {
		newLearner.acceptedMessages[acceptorID] = messageData{}
	}
	return &newLearner
}

func (l *learner) majority() int{
	return len(l.acceptedMessages)/2 + 1
}

func (l *learner) validateAcceptMessage(acceptedMessage messageData) {
	currentAcceptedMessage := l.acceptedMessages[acceptedMessage.messageSender]  // checking if the latest accepted message from the sender is the current message
	if currentAcceptedMessage.getMessageNumber() < acceptedMessage.getMessageNumber() {
		l.acceptedMessages[acceptedMessage.messageSender] = acceptedMessage
	}
}

func (l *learner) chosen() (messageData, bool) {
	acceptedMessageCount := make(map[int]int)
	acceptedMessageMap := make(map[int]messageData)

	for _, message := range l.acceptedMessages {
		proposalNumber := message.getMessageNumber()
		acceptedMessageCount[proposalNumber]+=1
		acceptedMessageMap[proposalNumber]=message
	}

	for number, message := range acceptedMessageMap {
		if acceptedMessageCount[number] > l.majority() {
			return message, true
		}
	}

	return messageData{}, false
}

func (l *learner) run() string{
	for{
		msg := l.node.receive()
		if msg==nil {
			continue
		}
		l.validateAcceptMessage(*msg)
		learnedMessage, learned := l.chosen()
		if !learned {
			logrus.Info("Learner hasn't learned anything yet.")
			continue
		}
		return learnedMessage.value
	}
}