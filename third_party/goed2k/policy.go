package goed2k

import (
	"math/rand"
	"slices"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	MaxPeerListSize     = 100
	MinReconnectTimeout = 10
)

type Policy struct {
	roundRobin int
	peers      []Peer
	transfer   *Transfer
	rnd        *rand.Rand
}

func NewPolicy(t *Transfer) Policy {
	return Policy{
		peers:    make([]Peer, 0),
		transfer: t,
		rnd:      rand.New(rand.NewSource(CurrentTimeMillis())),
	}
}

func (p Policy) IsConnectCandidate(pe Peer) bool {
	return !(pe.Connection != nil || !pe.Connectable || pe.FailCount > 10)
}

func (p Policy) IsEraseCandidate(pe Peer) bool {
	if pe.Connection != nil || p.IsConnectCandidate(pe) {
		return false
	}
	return pe.FailCount > 0
}

func (p Policy) Get(endpoint protocol.Endpoint) *Peer {
	for i := range p.peers {
		if p.peers[i].Endpoint.Equal(endpoint) {
			return &p.peers[i]
		}
	}
	return nil
}

func (p *Policy) AddPeer(peer Peer) (bool, error) {
	if MaxPeerListSize != 0 && len(p.peers) >= MaxPeerListSize {
		p.ErasePeers()
		if len(p.peers) >= MaxPeerListSize {
			return false, NewError(PeerLimitExceeded)
		}
	}
	insertPos, found := slices.BinarySearchFunc(p.peers, peer, func(a, b Peer) int { return a.Compare(b) })
	if found {
		p.peers[insertPos].SourceFlag |= peer.SourceFlag
		return false, nil
	}
	p.peers = slices.Insert(p.peers, insertPos, peer)
	return true, nil
}

func (p *Policy) ErasePeers() {
	if MaxPeerListSize == 0 || len(p.peers) == 0 {
		return
	}
	eraseCandidate := -1
	roundRobin := p.rnd.Intn(len(p.peers))
	lowWatermark := MaxPeerListSize * 95 / 100
	if lowWatermark == MaxPeerListSize {
		lowWatermark--
	}
	for iterations := minInt(len(p.peers), 300); iterations > 0; iterations-- {
		if len(p.peers) < lowWatermark {
			break
		}
		if roundRobin == len(p.peers) {
			roundRobin = 0
		}
		pe := p.peers[roundRobin]
		current := roundRobin
		if p.IsEraseCandidate(pe) && (eraseCandidate == -1 || !p.ComparePeerErase(p.peers[eraseCandidate], pe)) {
			if p.shouldEraseImmediately(pe) {
				if eraseCandidate > current {
					eraseCandidate--
				}
				p.peers = slices.Delete(p.peers, current, current+1)
			} else {
				eraseCandidate = current
			}
		}
		roundRobin++
	}
	if eraseCandidate > -1 {
		p.peers = slices.Delete(p.peers, eraseCandidate, eraseCandidate+1)
	}
}

func (p Policy) ComparePeers(lhs, rhs Peer) bool {
	if lhs.FailCount != rhs.FailCount {
		return lhs.FailCount < rhs.FailCount
	}
	lhsLocal := IsLocalAddress(lhs.Endpoint.IP())
	rhsLocal := IsLocalAddress(rhs.Endpoint.IP())
	if lhsLocal != rhsLocal {
		return lhsLocal
	}
	if lhs.LastConnected != rhs.LastConnected {
		return lhs.LastConnected < rhs.LastConnected
	}
	if lhs.NextConnection != rhs.NextConnection {
		return lhs.NextConnection < rhs.NextConnection
	}
	lhsRank := p.GetSourceRank(lhs.SourceFlag)
	rhsRank := p.GetSourceRank(rhs.SourceFlag)
	if lhsRank != rhsRank {
		return lhsRank > rhsRank
	}
	return false
}

func (p Policy) ComparePeerErase(lhs, rhs Peer) bool {
	if lhs.FailCount != rhs.FailCount {
		return lhs.FailCount > rhs.FailCount
	}
	lhsResumeDataSource := (lhs.SourceFlag & int(PeerResume)) == int(PeerResume)
	rhsResumeDataSource := (rhs.SourceFlag & int(PeerResume)) == int(PeerResume)
	if lhsResumeDataSource != rhsResumeDataSource {
		return lhsResumeDataSource
	}
	if lhs.Connectable != rhs.Connectable {
		return !lhs.Connectable
	}
	return false
}

func (p *Policy) FindConnectCandidate(sessionTime int64) *Peer {
	candidate := -1
	eraseCandidate := -1
	if p.roundRobin >= len(p.peers) {
		p.roundRobin = 0
	}
	for iteration := 0; iteration < minInt(len(p.peers), 300); iteration++ {
		if p.roundRobin >= len(p.peers) {
			p.roundRobin = 0
		}
		pe := p.peers[p.roundRobin]
		current := p.roundRobin
		if len(p.peers) > MaxPeerListSize {
			if p.IsEraseCandidate(pe) && (eraseCandidate == -1 || !p.ComparePeerErase(p.peers[eraseCandidate], pe)) {
				if p.shouldEraseImmediately(pe) {
					if eraseCandidate > current {
						eraseCandidate--
					}
					if candidate > current {
						candidate--
					}
					p.peers = slices.Delete(p.peers, current, current+1)
					continue
				}
				eraseCandidate = current
			}
		}
		p.roundRobin++
		if !p.IsConnectCandidate(pe) {
			continue
		}
		if candidate != -1 && p.ComparePeers(p.peers[candidate], pe) {
			continue
		}
		if pe.NextConnection != 0 && pe.NextConnection < sessionTime {
			continue
		}
		if pe.LastConnected != 0 && (sessionTime < pe.LastConnected+Seconds(int64(pe.FailCount+1))*MinReconnectTimeout) {
			continue
		}
		candidate = current
	}
	if eraseCandidate != -1 {
		if candidate > eraseCandidate {
			candidate--
		}
		p.peers = slices.Delete(p.peers, eraseCandidate, eraseCandidate+1)
	}
	if candidate == -1 {
		return nil
	}
	return &p.peers[candidate]
}

func (p *Policy) ConnectOnePeer(sessionTime int64) (bool, error) {
	peerInfo := p.FindConnectCandidate(sessionTime)
	if peerInfo != nil {
		_, err := p.transfer.ConnectToPeer(peerInfo)
		if err != nil {
			return false, err
		}
		return peerInfo.Connection != nil, nil
	}
	return false, nil
}

func (p *Policy) ConnectionClosed(c *PeerConnection, sessionTime int64) {
	peer := c.peerInfo
	if peer == nil {
		return
	}
	peer.Connection = nil
	if c.DisconnectCode().Code() == TransferPaused.Code() {
		peer.NextConnection = 0
		peer.LastConnected = 0
		return
	}
	peer.LastConnected = sessionTime
	if c.failed {
		peer.FailCount++
	}
	if !peer.Connectable {
		for i := range p.peers {
			if p.peers[i].Equal(*peer) {
				p.peers = slices.Delete(p.peers, i, i+1)
				break
			}
		}
	}
}

func (p *Policy) SetConnection(peer *Peer, c *PeerConnection) {
	peer.Connection = c
}

func (p Policy) shouldEraseImmediately(peer Peer) bool {
	return (peer.SourceFlag & int(PeerResume)) == int(PeerResume)
}

func (p Policy) Size() int { return len(p.peers) }

func (p Policy) NumConnectCandidates() int {
	res := 0
	if p.transfer == nil || p.transfer.IsFinished() {
		return res
	}
	for _, peer := range p.peers {
		if p.IsConnectCandidate(peer) && !p.IsEraseCandidate(peer) {
			res++
		}
	}
	return res
}

func (p *Policy) NewConnection(c *PeerConnection) error {
	peer := p.Get(c.Endpoint())
	if peer != nil {
		if peer.Connection != nil {
			return NewError(DuplicatePeerConnection)
		}
	} else {
		value := NewPeerWithSource(c.Endpoint(), false, 0)
		added, err := p.AddPeer(value)
		if err != nil {
			return err
		}
		if !added {
			return NewError(DuplicatePeer)
		}
		peer = p.Get(c.Endpoint())
	}
	peer.Connection = c
	c.SetPeer(peer)
	return nil
}

func (p Policy) GetSourceRank(sourceBitmask int) int {
	ret := 0
	if IsBit(int32(sourceBitmask), int32(PeerServer)) {
		ret |= 1 << 5
	}
	if IsBit(int32(sourceBitmask), int32(PeerDHT)) {
		ret |= 1 << 4
	}
	if IsBit(int32(sourceBitmask), int32(PeerIncoming)) {
		ret |= 1 << 3
	}
	if IsBit(int32(sourceBitmask), int32(PeerResume)) {
		ret |= 1 << 2
	}
	return ret
}

func (p Policy) FindPeer(ep protocol.Endpoint) *Peer {
	pos, found := slices.BinarySearchFunc(p.peers, NewPeer(ep), func(a, b Peer) int { return a.Compare(b) })
	if found {
		return &p.peers[pos]
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
