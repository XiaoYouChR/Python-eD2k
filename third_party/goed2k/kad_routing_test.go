package goed2k

import (
	"net"
	"testing"
	"time"

	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

func mustUDPAddr(t *testing.T, value string) *net.UDPAddr {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", value)
	if err != nil {
		t.Fatalf("resolve udp addr %s: %v", value, err)
	}
	return addr
}

func TestKadRoutingTablePromotesReplacementOnFailure(t *testing.T) {
	self := kadproto.NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796"))
	table := newKadRoutingTable(self, 1)

	live := &KadRoutingNode{
		ID:       kadproto.NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
		Addr:     mustUDPAddr(t, "1.2.3.4:4672"),
		Pinged:   true,
		LastSeen: time.Now(),
	}
	replacement := &KadRoutingNode{
		ID:       kadproto.NewID(protocol.MustHashFromString("31D6CFE0D14CE931B73C59D7E0C04BC0")),
		Addr:     mustUDPAddr(t, "5.6.7.8:4672"),
		Pinged:   true,
		LastSeen: time.Now(),
	}

	table.NodeSeen(live)
	table.HeardAbout(replacement)
	table.NodeFailed(live)

	nodes := table.FindClosest(self, 10, true)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 live node after promotion, got %d", len(nodes))
	}
	if !nodes[0].ID.Hash.Equal(replacement.ID.Hash) {
		t.Fatalf("expected replacement node to be promoted, got %s", nodes[0].ID.Hash.String())
	}
}

func TestKadRPCManagerExpiresTransactions(t *testing.T) {
	manager := newKadRPCManager()
	tx := &kadRPCTransaction{
		endpointKey: "1.2.3.4:4672",
		opcode:      kadproto.BootstrapResOp,
		sentTime:    time.Now().Add(-13 * time.Second),
	}
	manager.transactions = append(manager.transactions, tx)

	_, expired := manager.Tick(time.Now())
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired transaction, got %d", len(expired))
	}
	if len(manager.transactions) != 0 {
		t.Fatalf("expected transaction list to be empty, got %d", len(manager.transactions))
	}
}
