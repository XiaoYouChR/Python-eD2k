package client

import (
	"bytes"
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func TestPacketCombinerSupportsLegacyQueueRank(t *testing.T) {
	combiner := NewPacketCombiner()
	raw, err := combiner.Pack("client.QueueRank", &QueueRank{Rank: 70000})
	if err != nil {
		t.Fatal(err)
	}
	header := protocol.PacketHeader{}
	reader := bytes.NewReader(raw)
	if err := header.Get(reader); err != nil {
		t.Fatal(err)
	}
	packet, err := combiner.Unpack(header, raw[protocol.PacketHeaderSize:])
	if err != nil {
		t.Fatal(err)
	}
	rank, ok := packet.(*QueueRank)
	if !ok || rank.Rank != 70000 {
		t.Fatalf("unexpected queue rank packet: %#v", packet)
	}
}
