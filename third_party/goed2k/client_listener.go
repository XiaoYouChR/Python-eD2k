package goed2k

import (
	"time"

	"github.com/monkeyWie/goed2k/protocol"
)

// ClientStatusEvent is a point-in-time snapshot emitted by a Client listener.
type ClientStatusEvent struct {
	At     time.Time
	Status ClientStatus
	DHT    DHTStatus
}

// TransferSnapshots returns the transfer snapshots carried by this event.
func (e ClientStatusEvent) TransferSnapshots() []TransferSnapshot {
	return e.Status.Transfers
}

// TransferState returns the current state for a transfer hash carried by this event.
func (e ClientStatusEvent) TransferState(hash protocol.Hash) (TransferState, bool) {
	for _, transfer := range e.Status.Transfers {
		if transfer.Hash.Compare(hash) == 0 {
			return transfer.Status.State, true
		}
	}
	return "", false
}

// TransferStates returns all transfer states in this event keyed by transfer hash.
func (e ClientStatusEvent) TransferStates() map[protocol.Hash]TransferState {
	out := make(map[protocol.Hash]TransferState, len(e.Status.Transfers))
	for _, transfer := range e.Status.Transfers {
		out[transfer.Hash] = transfer.Status.State
	}
	return out
}

// SubscribeStatus registers a non-blocking status listener using the default buffer size.
func (c *Client) SubscribeStatus() (<-chan ClientStatusEvent, func()) {
	return c.SubscribeStatusBuffered(8)
}

// SubscribeStatusBuffered registers a non-blocking status listener.
//
// The returned channel receives snapshots as the client state changes. If the
// receiver falls behind and the channel buffer is full, newer snapshots may be
// dropped instead of blocking the client loop.
//
// The returned cancel function unregisters the listener and closes the channel.
func (c *Client) SubscribeStatusBuffered(buffer int) (<-chan ClientStatusEvent, func()) {
	if buffer <= 0 {
		buffer = 1
	}
	ch := make(chan ClientStatusEvent, buffer)

	c.listenersMu.Lock()
	if c.listeners == nil {
		c.listeners = make(map[int]chan ClientStatusEvent)
	}
	c.nextListenerID++
	id := c.nextListenerID
	c.listeners[id] = ch
	c.listenersMu.Unlock()

	c.emitCurrentStatusTo(ch)

	cancel := func() {
		c.listenersMu.Lock()
		existing, ok := c.listeners[id]
		if ok {
			delete(c.listeners, id)
			close(existing)
		}
		c.listenersMu.Unlock()
	}
	return ch, cancel
}

func (c *Client) emitStatusUpdate() {
	event := ClientStatusEvent{
		At:     time.Now(),
		Status: c.Status(),
		DHT:    c.DHTStatus(),
	}

	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()
	for _, ch := range c.listeners {
		select {
		case ch <- event:
		default:
		}
	}
}

func (c *Client) emitCurrentStatusTo(ch chan ClientStatusEvent) {
	event := ClientStatusEvent{
		At:     time.Now(),
		Status: c.Status(),
		DHT:    c.DHTStatus(),
	}
	select {
	case ch <- event:
	default:
	}
}
