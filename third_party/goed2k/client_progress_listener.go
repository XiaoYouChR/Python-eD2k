package goed2k

import (
	"time"

	"github.com/monkeyWie/goed2k/protocol"
)

// TransferProgressSnapshot is a lightweight per-transfer progress snapshot.
type TransferProgressSnapshot struct {
	Hash              protocol.Hash
	FileName          string
	FilePath          string
	State             TransferState
	Paused            bool
	Removed           bool
	TotalDone         int64
	TotalReceived     int64
	TotalWanted       int64
	DownloadingPieces int
	ActivePeers       int
	NumPeers          int
}

// TransferProgressEvent contains only transfers whose progress/state changed.
type TransferProgressEvent struct {
	At        time.Time
	Transfers []TransferProgressSnapshot
}

// SubscribeTransferProgress subscribes to per-transfer progress changes.
//
// Events are emitted only when a transfer's received bytes, done bytes, state,
// pause flag, or removal status changes.
func (c *Client) SubscribeTransferProgress() (<-chan TransferProgressEvent, func()) {
	return c.SubscribeTransferProgressBuffered(8)
}

// SubscribeTransferProgressBuffered is the buffered variant of SubscribeTransferProgress.
func (c *Client) SubscribeTransferProgressBuffered(buffer int) (<-chan TransferProgressEvent, func()) {
	if buffer <= 0 {
		buffer = 1
	}
	ch := make(chan TransferProgressEvent, buffer)

	c.progressMu.Lock()
	if c.progressListeners == nil {
		c.progressListeners = make(map[int]chan TransferProgressEvent)
	}
	c.nextProgressID++
	id := c.nextProgressID
	c.progressListeners[id] = ch
	c.progressMu.Unlock()

	c.emitCurrentProgressTo(ch)

	cancel := func() {
		c.progressMu.Lock()
		existing, ok := c.progressListeners[id]
		if ok {
			delete(c.progressListeners, id)
			close(existing)
		}
		c.progressMu.Unlock()
	}
	return ch, cancel
}

func (c *Client) buildTransferProgressSnapshots() []TransferProgressSnapshot {
	transfers := c.TransferSnapshots()
	snapshots := make([]TransferProgressSnapshot, 0, len(transfers))
	for _, transfer := range transfers {
		snapshots = append(snapshots, TransferProgressSnapshot{
			Hash:              transfer.Hash,
			FileName:          transfer.FileName,
			FilePath:          transfer.FilePath,
			State:             transfer.Status.State,
			Paused:            transfer.Status.Paused,
			TotalDone:         transfer.Status.TotalDone,
			TotalReceived:     transfer.Status.TotalReceived,
			TotalWanted:       transfer.Status.TotalWanted,
			DownloadingPieces: transfer.Status.DownloadingPieces,
			ActivePeers:       transfer.ActivePeers,
			NumPeers:          transfer.Status.NumPeers,
		})
	}
	return snapshots
}

func (c *Client) emitTransferProgressUpdate(force bool) {
	current := c.buildTransferProgressSnapshots()
	currentMap := make(map[protocol.Hash]TransferProgressSnapshot, len(current))
	for _, snapshot := range current {
		currentMap[snapshot.Hash] = snapshot
	}

	c.progressMu.Lock()
	if c.lastProgress == nil {
		c.lastProgress = make(map[protocol.Hash]TransferProgressSnapshot)
	}
	changes := make([]TransferProgressSnapshot, 0, len(current))
	for _, snapshot := range current {
		prev, ok := c.lastProgress[snapshot.Hash]
		if force || !ok || transferProgressChanged(prev, snapshot) {
			changes = append(changes, snapshot)
		}
	}
	for hash, prev := range c.lastProgress {
		if _, ok := currentMap[hash]; ok {
			continue
		}
		prev.Removed = true
		changes = append(changes, prev)
	}
	c.lastProgress = currentMap
	if len(changes) == 0 {
		c.progressMu.Unlock()
		return
	}
	event := TransferProgressEvent{
		At:        time.Now(),
		Transfers: changes,
	}
	for _, ch := range c.progressListeners {
		select {
		case ch <- event:
		default:
		}
	}
	c.progressMu.Unlock()
}

func transferProgressChanged(prev, next TransferProgressSnapshot) bool {
	return prev.State != next.State ||
		prev.Paused != next.Paused ||
		prev.TotalDone != next.TotalDone ||
		prev.TotalReceived != next.TotalReceived
}

func (c *Client) emitCurrentProgressTo(ch chan TransferProgressEvent) {
	event := TransferProgressEvent{
		At:        time.Now(),
		Transfers: c.buildTransferProgressSnapshots(),
	}
	select {
	case ch <- event:
	default:
	}
}
