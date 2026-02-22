package paxos

import (
	"testing"
	"time"
)

func newTestAcceptor(id int) (*Acceptor, *Environment) {
	env := NewPaxosEnvironment(id, 100, 200)
	node := env.GetNodeNetwork(id)
	return NewAcceptor(id, node, 200), env
}

func TestPromiseTracking(t *testing.T) {
	a, _ := newTestAcceptor(1)

	prepare := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}

	ack := a.receivePreparedMessage(prepare)
	if ack == nil {
		t.Fatal("receivePreparedMessage returned nil for valid prepare")
	}

	// promisedMessage should be updated
	if a.promisedMessages[0].getMessageNumber() != 10100 {
		t.Errorf("promisedMessage not updated: got %d, want 10100", a.promisedMessages[0].getMessageNumber())
	}

	// acceptedMessage should NOT be updated by a prepare
	if a.acceptedMessages[0].getMessageNumber() != 0 {
		t.Errorf("acceptedMessage should be untouched after prepare: got %d, want 0", a.acceptedMessages[0].getMessageNumber())
	}
}

func TestRejectLowerPrepare(t *testing.T) {
	a, _ := newTestAcceptor(1)

	// First prepare with higher number
	high := messageData{
		messageSender:   100,
		messageNumber:   20100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}
	a.receivePreparedMessage(high)

	// Second prepare with lower number should return nil
	low := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}
	ack := a.receivePreparedMessage(low)
	if ack != nil {
		t.Error("receivePreparedMessage should return nil for prepare with number lower than promised")
	}
}

func TestAcceptProposal(t *testing.T) {
	a, _ := newTestAcceptor(1)

	// Promise for proposal 10100
	prepare := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}
	a.receivePreparedMessage(prepare)

	// Propose with same number — should be accepted
	propose := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: ProposeMessage,
		value:           "hello",
	}
	accepted := a.receiveProposeMessage(propose)
	if !accepted {
		t.Error("receiveProposeMessage should accept proposal with number equal to promised")
	}

	// acceptedMessage should now be updated
	if a.acceptedMessages[0].getMessageNumber() != 10100 {
		t.Errorf("acceptedMessage not updated after accept: got %d, want 10100", a.acceptedMessages[0].getMessageNumber())
	}
	if a.acceptedMessages[0].value != "hello" {
		t.Errorf("acceptedMessage value wrong: got %q, want %q", a.acceptedMessages[0].value, "hello")
	}
}

func TestRejectProposalWithLowerNumber(t *testing.T) {
	a, _ := newTestAcceptor(1)

	// Promise for higher proposal
	prepare := messageData{
		messageSender:   100,
		messageNumber:   20100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}
	a.receivePreparedMessage(prepare)

	// Propose with lower number — should be rejected
	propose := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: ProposeMessage,
		value:           "hello",
	}
	accepted := a.receiveProposeMessage(propose)
	if accepted {
		t.Error("receiveProposeMessage should reject proposal with number lower than promised")
	}
}

func TestAcceptProposalWithHigherNumber(t *testing.T) {
	a, _ := newTestAcceptor(1)

	// Promise for proposal 10100
	prepare := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}
	a.receivePreparedMessage(prepare)

	// Propose with higher number — should be accepted (no higher promise)
	propose := messageData{
		messageSender:   100,
		messageNumber:   20100,
		messageCategory: ProposeMessage,
		value:           "world",
	}
	accepted := a.receiveProposeMessage(propose)
	if !accepted {
		t.Error("receiveProposeMessage should accept proposal with number higher than promised")
	}
}

func TestPreviouslyAcceptedValueInPromise(t *testing.T) {
	a, _ := newTestAcceptor(1)

	// First round: prepare and accept a value
	prepare1 := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: PrepareMessage,
		value:           "first",
	}
	a.receivePreparedMessage(prepare1)

	propose1 := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: ProposeMessage,
		value:           "first",
	}
	a.receiveProposeMessage(propose1)

	// Second round: new prepare with higher number
	// Ack should include the previously accepted value
	prepare2 := messageData{
		messageSender:   101,
		messageNumber:   20101,
		messageCategory: PrepareMessage,
		value:           "second",
	}
	ack := a.receivePreparedMessage(prepare2)
	if ack == nil {
		t.Fatal("receivePreparedMessage returned nil for valid higher prepare")
	}
	if ack.value != "first" {
		t.Errorf("ack should include previously accepted value: got %q, want %q", ack.value, "first")
	}
	if ack.messageNumber != 10100 {
		t.Errorf("ack should include previously accepted proposal number: got %d, want 10100", ack.messageNumber)
	}
}

func TestNoAcceptedValueInFirstPromise(t *testing.T) {
	a, _ := newTestAcceptor(1)

	// First prepare ever — no previously accepted value
	prepare := messageData{
		messageSender:   100,
		messageNumber:   10100,
		messageCategory: PrepareMessage,
		value:           "hello",
	}
	ack := a.receivePreparedMessage(prepare)
	if ack == nil {
		t.Fatal("receivePreparedMessage returned nil for first prepare")
	}
	// With no prior accepted value, ack should carry the prepare's value/number
	if ack.value != "hello" {
		t.Errorf("first ack should carry prepare's value: got %q, want %q", ack.value, "hello")
	}
	if ack.messageNumber != 10100 {
		t.Errorf("first ack should carry prepare's number: got %d, want 10100", ack.messageNumber)
	}
}

func TestAcceptorGracefulShutdown(t *testing.T) {
	a, _ := newTestAcceptor(1)

	exited := make(chan struct{})
	go func() {
		a.Accept()
		close(exited)
	}()

	time.Sleep(50 * time.Millisecond)
	a.Stop()

	select {
	case <-exited:
		// Success: Accept() returned
	case <-time.After(3 * time.Second):
		t.Fatal("Acceptor did not shut down within 3 seconds after Stop()")
	}
}
