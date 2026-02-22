package paxos

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrStopped is returned when an operation is attempted on a stopped Node.
var ErrStopped = errors.New("node stopped")

// routedNode implements nodeNetwork for a single role within a Node.
// It routes messages either locally (between co-located roles) or
// externally (via the Transport).
type routedNode struct {
	nodeID  int
	router  *messageRouter
	inbound chan messageData
}

func (rn *routedNode) send(m messageData) {
	m.timestamp = time.Now().String()
	if m.messageRecipient == rn.nodeID {
		rn.router.deliverLocal(m)
		return
	}
	rn.router.transport.Send(toPublicMessage(m))
}

func (rn *routedNode) receive() *messageData {
	return rn.receiveWithTimeout(time.Second)
}

func (rn *routedNode) receiveWithTimeout(timeout time.Duration) *messageData {
	if timeout <= 0 {
		return nil
	}
	select {
	case msg := <-rn.inbound:
		return &msg
	case <-time.After(timeout):
		return nil
	}
}

// messageRouter is the central routing hub shared by all 3 routedNodes within a Node.
type messageRouter struct {
	nodeID     int
	transport  Transport
	proposerCh chan messageData
	acceptorCh chan messageData
	learnerCh  chan messageData
	ctx        context.Context
	cancel     context.CancelFunc
}

func (mr *messageRouter) queueFor(mt messageType) chan messageData {
	switch mt {
	case PrepareMessage, ProposeMessage:
		return mr.acceptorCh
	case AckMessage, HeartbeatMessage:
		return mr.proposerCh
	case AcceptMessage:
		return mr.learnerCh
	default:
		return nil
	}
}

func (mr *messageRouter) deliverLocal(m messageData) {
	ch := mr.queueFor(m.messageCategory)
	if ch == nil {
		return
	}
	select {
	case ch <- m:
	case <-mr.ctx.Done():
	}
}

func (mr *messageRouter) run() {
	for {
		msg, err := mr.transport.Receive(mr.ctx)
		if err != nil {
			return
		}
		mr.deliverLocal(toInternalMessage(msg))
	}
}

// Node is the high-level facade that wires a Proposer, Acceptor, and Learner
// behind a single Transport so users don't assemble the pieces manually.
type Node struct {
	id        int
	proposer  *Proposer
	acceptor  *Acceptor
	learner   *Learner
	router    *messageRouter
	committed chan Entry
	done      chan struct{}
	stopOnce  sync.Once
}

// NewNode creates a Node that participates in Paxos consensus.
// id is this node's unique identifier. peerIDs are the other nodes in the cluster.
// transport is the networking layer for inter-node communication.
func NewNode(id int, peerIDs []int, transport Transport) *Node {
	allIDs := make([]int, 0, 1+len(peerIDs))
	allIDs = append(allIDs, id)
	allIDs = append(allIDs, peerIDs...)

	ctx, cancel := context.WithCancel(context.Background())

	router := &messageRouter{
		nodeID:     id,
		transport:  transport,
		proposerCh: make(chan messageData, 1024),
		acceptorCh: make(chan messageData, 1024),
		learnerCh:  make(chan messageData, 1024),
		ctx:        ctx,
		cancel:     cancel,
	}

	proposerNode := &routedNode{nodeID: id, router: router, inbound: router.proposerCh}
	acceptorNode := &routedNode{nodeID: id, router: router, inbound: router.acceptorCh}
	learnerNode := &routedNode{nodeID: id, router: router, inbound: router.learnerCh}

	proposer := NewProposer(id, "", proposerNode, allIDs...)
	proposer.SetPeers(peerIDs...)

	acceptor := NewAcceptor(id, acceptorNode, allIDs...)
	learner := NewLearner(id, learnerNode, allIDs...)

	return &Node{
		id:        id,
		proposer:  proposer,
		acceptor:  acceptor,
		learner:   learner,
		router:    router,
		committed: make(chan Entry, 64),
		done:      make(chan struct{}),
	}
}

// Start launches the background goroutines that drive the Paxos protocol.
func (n *Node) Start(ctx context.Context) {
	go n.router.run()
	go n.acceptor.Accept()
	go n.runProposer()
	go n.runLearner()
}

func (n *Node) runProposer() {
	n.proposer.electLeader()
	if !n.proposer.isLeader {
		return
	}
	slot := 0
	for {
		select {
		case value, ok := <-n.proposer.values:
			if !ok {
				return
			}
			n.proposer.runSlot(slot, value)
			slot++
		case <-n.done:
			return
		}
	}
}

func (n *Node) runLearner() {
	decided := make(map[int]bool)
	for {
		select {
		case msg := <-n.router.learnerCh:
			n.learner.validateAcceptMessage(msg)
			chosen, ok := n.learner.chosen(msg.slot)
			if ok && !decided[msg.slot] {
				decided[msg.slot] = true
				entry := Entry{
					Slot:  msg.slot,
					Value: []byte(chosen.value),
				}
				select {
				case n.committed <- entry:
				case <-n.done:
					return
				}
			}
		case <-n.done:
			return
		}
	}
}

// Propose submits a value for consensus.
func (n *Node) Propose(ctx context.Context, value []byte) error {
	select {
	case n.proposer.values <- string(value):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-n.done:
		return ErrStopped
	}
}

// Committed returns a channel that emits decided entries.
func (n *Node) Committed() <-chan Entry {
	return n.committed
}

// Stop gracefully shuts down the Node.
func (n *Node) Stop() {
	n.stopOnce.Do(func() {
		close(n.done)
		n.acceptor.Stop()
		n.router.cancel()
	})
}
