package goed2k

import (
	"math"
	"sort"

	"github.com/monkeyWie/goed2k/protocol"
)

type ClientCreditState struct {
	PeerHash   protocol.Hash
	Uploaded   uint64
	Downloaded uint64
}

type PeerCredit struct {
	PeerHash   protocol.Hash
	Uploaded   uint64
	Downloaded uint64
}

type PeerCreditManager struct {
	credits map[string]*PeerCredit
}

func NewPeerCreditManager() *PeerCreditManager {
	return &PeerCreditManager{credits: make(map[string]*PeerCredit)}
}

func (m *PeerCreditManager) key(hash protocol.Hash) string {
	if hash.Equal(protocol.Invalid) {
		return ""
	}
	return hash.String()
}

func (m *PeerCreditManager) credit(hash protocol.Hash) *PeerCredit {
	if m == nil {
		return nil
	}
	key := m.key(hash)
	if key == "" {
		return nil
	}
	if credit, ok := m.credits[key]; ok {
		return credit
	}
	credit := &PeerCredit{PeerHash: hash}
	m.credits[key] = credit
	return credit
}

func (m *PeerCreditManager) AddUploaded(hash protocol.Hash, bytes int64) {
	if bytes <= 0 {
		return
	}
	if credit := m.credit(hash); credit != nil {
		credit.Uploaded += uint64(bytes)
	}
}

func (m *PeerCreditManager) AddDownloaded(hash protocol.Hash, bytes int64) {
	if bytes <= 0 {
		return
	}
	if credit := m.credit(hash); credit != nil {
		credit.Downloaded += uint64(bytes)
	}
}

func (m *PeerCreditManager) ScoreRatio(hash protocol.Hash) float64 {
	credit := m.credit(hash)
	if credit == nil || credit.Downloaded < 1000000 {
		return 1.0
	}
	result := 10.0
	if credit.Uploaded > 0 {
		result = (float64(credit.Downloaded) * 2.0) / float64(credit.Uploaded)
	}
	limit := math.Sqrt(float64(credit.Downloaded)/1048576.0 + 2.0)
	if result > limit {
		result = limit
	}
	if result < 1.0 {
		return 1.0
	}
	if result > 10.0 {
		return 10.0
	}
	return result
}

func (m *PeerCreditManager) Snapshot() []ClientCreditState {
	if m == nil {
		return nil
	}
	states := make([]ClientCreditState, 0, len(m.credits))
	for _, credit := range m.credits {
		if credit == nil {
			continue
		}
		states = append(states, ClientCreditState{
			PeerHash:   credit.PeerHash,
			Uploaded:   credit.Uploaded,
			Downloaded: credit.Downloaded,
		})
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].PeerHash.String() < states[j].PeerHash.String()
	})
	return states
}

func (m *PeerCreditManager) ApplySnapshot(states []ClientCreditState) {
	if m == nil {
		return
	}
	m.credits = make(map[string]*PeerCredit, len(states))
	for _, state := range states {
		if state.PeerHash.Equal(protocol.Invalid) {
			continue
		}
		m.credits[state.PeerHash.String()] = &PeerCredit{
			PeerHash:   state.PeerHash,
			Uploaded:   state.Uploaded,
			Downloaded: state.Downloaded,
		}
	}
}
