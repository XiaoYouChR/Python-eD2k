package goed2k

import (
	"net"
	"time"

	"github.com/monkeyWie/goed2k/protocol"
)

type kadRPCTransaction struct {
	endpointKey string
	opcode      byte
	target      *protocol.Hash
	sentTime    time.Time
	multi       bool
	shortTimed  bool
	observer    *kadObserver
}

type kadRPCManager struct {
	shortTimeout time.Duration
	timeout      time.Duration
	transactions []*kadRPCTransaction
}

func newKadRPCManager() *kadRPCManager {
	return &kadRPCManager{
		shortTimeout: 2 * time.Second,
		timeout:      12 * time.Second,
		transactions: make([]*kadRPCTransaction, 0),
	}
}

func (m *kadRPCManager) Invoke(tx *kadRPCTransaction) {
	if m == nil || tx == nil {
		return
	}
	tx.sentTime = time.Now()
	m.transactions = append(m.transactions, tx)
}

func (m *kadRPCManager) Incoming(addr *net.UDPAddr, opcode byte, target *protocol.Hash) *kadRPCTransaction {
	if m == nil || addr == nil {
		return nil
	}
	key := addr.String()
	for i, tx := range m.transactions {
		if tx == nil || tx.endpointKey != key || tx.opcode != opcode {
			continue
		}
		if tx.target != nil && target != nil && !tx.target.Equal(*target) {
			continue
		}
		if tx.target != nil && target == nil {
			continue
		}
		if !tx.multi {
			m.transactions = append(m.transactions[:i], m.transactions[i+1:]...)
		}
		return tx
	}
	return nil
}

func (m *kadRPCManager) Tick(now time.Time) (shortTimed []*kadRPCTransaction, expired []*kadRPCTransaction) {
	if m == nil {
		return nil, nil
	}
	dst := m.transactions[:0]
	for _, tx := range m.transactions {
		if tx == nil {
			continue
		}
		if !tx.shortTimed && now.Sub(tx.sentTime) >= m.shortTimeout {
			tx.shortTimed = true
			shortTimed = append(shortTimed, tx)
		}
		if now.Sub(tx.sentTime) >= m.timeout {
			expired = append(expired, tx)
			continue
		}
		dst = append(dst, tx)
	}
	m.transactions = dst
	return shortTimed, expired
}
