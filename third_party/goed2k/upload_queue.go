package goed2k

import (
	"slices"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	maxUploadTime           = 60 * 60 * 1000
	maxUploadData           = 10 * 1024 * 1024
	minUploadClientsAllowed = 2
	maxUploadClientsAllowed = 250
)

type UploadQueue struct {
	session         *Session
	waiting         []*PeerConnection
	uploading       []*PeerConnection
	lastStartUpload int64
	lastSort        int64
	allowKicking    bool
	lastSlotHighID  bool
	suspended       map[string]bool
}

func NewUploadQueue(session *Session) *UploadQueue {
	return &UploadQueue{
		session:        session,
		waiting:        make([]*PeerConnection, 0),
		uploading:      make([]*PeerConnection, 0),
		lastSlotHighID: true,
		suspended:      make(map[string]bool),
	}
}

func (q *UploadQueue) AddClientToQueue(client *PeerConnection) {
	if client == nil || client.IsDisconnecting() {
		return
	}
	client.lastUploadRequest = CurrentTime()
	if q.IsOnUploadQueue(client) {
		maxSlots := q.maxSlots()
		if client.UploadAddNextConnect() && q.lastSlotHighID {
			maxSlots++
		}
		if client.UploadAddNextConnect() && len(q.uploading) < maxSlots {
			client.SetUploadAddNextConnect(false)
			q.RemoveFromWaitingQueue(client)
			q.addUpNextClient(client)
			q.lastSlotHighID = false
			return
		}
		client.SendQueueRanking(client.UploadQueueRank())
		return
	}
	if q.IsUploading(client) {
		client.SendAcceptUpload()
		return
	}
	if client.transfer == nil || q.isSuspended(client.transfer.GetHash()) {
		return
	}
	if q.session != nil && q.session.settings.UploadQueueSize > 0 && len(q.waiting) >= q.session.settings.UploadQueueSize {
		return
	}
	now := CurrentTime()
	client.ClearUploadWaitStart()
	if len(q.waiting) == 0 && now-q.lastStartUpload >= Seconds(1) && len(q.uploading) < q.maxSlots() {
		q.addUpNextClient(client)
		q.lastStartUpload = now
		return
	}
	client.SetUploadWaitStart(now)
	client.SetUploadState(UploadStateOnQueue)
	q.waiting = append(q.waiting, client)
	q.sortWaiting()
	client.SendQueueRanking(client.UploadQueueRank())
}

func (q *UploadQueue) Process() {
	if q == nil {
		return
	}
	now := CurrentTime()
	if len(q.waiting) == 0 || now-q.lastStartUpload < Seconds(1) {
		q.allowKicking = false
	} else if len(q.uploading) < q.maxSlots() {
		q.allowKicking = false
		q.lastStartUpload = now
		q.addUpNextClient(nil)
	} else {
		q.allowKicking = true
	}

	for _, client := range append([]*PeerConnection(nil), q.uploading...) {
		if client == nil {
			continue
		}
		if client.IsDisconnecting() || client.socket == nil {
			q.RemoveFromUploadQueue(client)
			continue
		}
		client.SendBlockData()
	}
	if now-q.lastSort > Minutes(2) {
		q.sortWaiting()
	}
}

func (q *UploadQueue) RemoveFromUploadQueue(client *PeerConnection) bool {
	if client == nil {
		return false
	}
	removed := false
	if idx := slices.Index(q.uploading, client); idx >= 0 {
		q.uploading = append(q.uploading[:idx], q.uploading[idx+1:]...)
		removed = true
	}
	if q.RemoveFromWaitingQueue(client) {
		removed = true
	}
	if removed {
		client.SetUploadState(UploadStateNone)
		client.ClearUploadBlockRequests()
	}
	return removed
}

func (q *UploadQueue) RemoveFromWaitingQueue(client *PeerConnection) bool {
	if client == nil {
		return false
	}
	idx := slices.Index(q.waiting, client)
	if idx < 0 {
		return false
	}
	q.waiting = append(q.waiting[:idx], q.waiting[idx+1:]...)
	client.SetUploadState(UploadStateNone)
	client.SetUploadQueueRank(0)
	client.ClearUploadWaitStart()
	q.recomputeRanks()
	return true
}

func (q *UploadQueue) IsOnUploadQueue(client *PeerConnection) bool {
	return slices.Index(q.waiting, client) >= 0
}

func (q *UploadQueue) IsUploading(client *PeerConnection) bool {
	return slices.Index(q.uploading, client) >= 0
}

func (q *UploadQueue) sortWaiting() {
	slices.SortStableFunc(q.waiting, func(a, b *PeerConnection) int {
		if a == nil || b == nil {
			return 0
		}
		scoreA := a.UploadScore()
		scoreB := b.UploadScore()
		if scoreA != scoreB {
			if scoreA > scoreB {
				return -1
			}
			return 1
		}
		if a.UploadWaitStart() == b.UploadWaitStart() {
			return a.Endpoint().Compare(b.Endpoint())
		}
		if a.UploadWaitStart() < b.UploadWaitStart() {
			return -1
		}
		return 1
	})
	q.lastSort = CurrentTime()
	q.recomputeRanks()
}

func (q *UploadQueue) recomputeRanks() {
	for i, client := range q.waiting {
		if client == nil {
			continue
		}
		client.SetUploadQueueRank(uint16(i + 1))
	}
}

func (q *UploadQueue) addUpNextClient(direct *PeerConnection) {
	var client *PeerConnection
	if direct != nil {
		client = direct
	} else {
		if len(q.waiting) == 0 {
			return
		}
		best := -1
		for i, candidate := range q.waiting {
			if candidate == nil {
				continue
			}
			if candidate.IsUploadLowID() && !candidate.IsUploadConnected() {
				candidate.SetUploadAddNextConnect(true)
				continue
			}
			best = i
			break
		}
		if best < 0 {
			return
		}
		client = q.waiting[best]
		q.waiting = append(q.waiting[:best], q.waiting[best+1:]...)
		q.recomputeRanks()
	}
	if client == nil || q.IsUploading(client) {
		return
	}
	client.SetUploadState(UploadStateUploading)
	client.SetUploadQueueRank(0)
	client.SetUploadAddNextConnect(false)
	client.SetUploadStartTime(CurrentTime())
	client.ResetUploadSession()
	q.uploading = append(q.uploading, client)
	q.lastSlotHighID = !client.IsUploadLowID()
	if client.socket != nil {
		client.SendAcceptUpload()
	}
}

func (q *UploadQueue) CheckForTimeOver(client *PeerConnection) bool {
	if !q.allowKicking || client == nil {
		return false
	}
	if client.FriendSlot() {
		return false
	}
	if client.transfer != nil && client.transfer.UploadPriority() == UploadPriorityPowerShare {
		vips := 0
		for _, current := range q.uploading {
			if current == nil {
				continue
			}
			if current.FriendSlot() || (current.transfer != nil && current.transfer.UploadPriority() == UploadPriorityPowerShare) {
				vips++
			}
		}
		if vips <= q.maxSlots()/2 {
			return false
		}
	}
	if client.UploadStartDelay() > maxUploadTime || client.UploadSession() > maxUploadData {
		q.allowKicking = false
		return true
	}
	return false
}

func (q *UploadQueue) maxSlots() int {
	if q == nil || q.session == nil {
		return minUploadClientsAllowed
	}
	if q.session.settings.MaxUploadRateKB <= 0 {
		slotAllocation := q.session.settings.SlotAllocationKB
		if slotAllocation <= 0 {
			slotAllocation = 3
		}
		nMaxSlots := int(q.session.accumulator.UploadRate()/1024)/slotAllocation + 2
		if nMaxSlots < minUploadClientsAllowed {
			nMaxSlots = minUploadClientsAllowed
		}
		if nMaxSlots > maxUploadClientsAllowed {
			nMaxSlots = maxUploadClientsAllowed
		}
		return nMaxSlots
	}
	slotAllocation := q.session.settings.SlotAllocationKB
	if slotAllocation <= 0 {
		slotAllocation = 3
	}
	nMaxSlots := int(float64(q.session.settings.MaxUploadRateKB)/float64(slotAllocation) + 0.5)
	if nMaxSlots < minUploadClientsAllowed {
		nMaxSlots = minUploadClientsAllowed
	}
	if nMaxSlots > maxUploadClientsAllowed {
		nMaxSlots = maxUploadClientsAllowed
	}
	return nMaxSlots
}

func (q *UploadQueue) isSuspended(hash protocol.Hash) bool {
	if q == nil || hash.Equal(protocol.Invalid) {
		return false
	}
	return q.suspended[hash.String()]
}

func (q *UploadQueue) ResumeUpload(hash protocol.Hash) {
	if q == nil || hash.Equal(protocol.Invalid) {
		return
	}
	delete(q.suspended, hash.String())
}

func (q *UploadQueue) SuspendUpload(hash protocol.Hash, terminate bool) uint16 {
	if q == nil || hash.Equal(protocol.Invalid) {
		return 0
	}
	if !terminate {
		q.suspended[hash.String()] = true
	}
	removed := uint16(0)
	for _, client := range append([]*PeerConnection(nil), q.uploading...) {
		if client == nil || client.transfer == nil || !client.transfer.GetHash().Equal(hash) {
			continue
		}
		q.RemoveFromUploadQueue(client)
		if !terminate {
			client.SetUploadState(UploadStateOnQueue)
			client.SetUploadWaitStart(CurrentTime())
			q.waiting = append(q.waiting, client)
			q.sortWaiting()
			client.SendQueueRanking(client.UploadQueueRank())
		}
		removed++
	}
	return removed
}
