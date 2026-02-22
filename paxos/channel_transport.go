package paxos

import (
	"context"
	"fmt"
)

// channelTransport is an in-process Transport backed by Go channels.
type channelTransport struct {
	id      int
	inbound chan Message
	peers   map[int]chan Message
}

// NewChannelTransportGroup creates a group of connected in-process transports.
func NewChannelTransportGroup(ids ...int) map[int]Transport {
	channels := make(map[int]chan Message, len(ids))
	for _, id := range ids {
		channels[id] = make(chan Message, 1024)
	}

	transports := make(map[int]Transport, len(ids))
	for _, id := range ids {
		transports[id] = &channelTransport{
			id:      id,
			inbound: channels[id],
			peers:   channels,
		}
	}
	return transports
}

func (t *channelTransport) Send(msg Message) error {
	ch, ok := t.peers[msg.To]
	if !ok {
		return fmt.Errorf("channelTransport: unknown recipient %d", msg.To)
	}
	ch <- msg
	return nil
}

func (t *channelTransport) Receive(ctx context.Context) (Message, error) {
	select {
	case msg := <-t.inbound:
		return msg, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}
