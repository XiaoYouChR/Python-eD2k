package kad

import (
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func TestPacketCombinerRoundTripSearchSourcesReq(t *testing.T) {
	combiner := PacketCombiner{}
	raw, err := combiner.Pack(SearchSourcesReq{
		Target:   NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		StartPos: 7,
		Size:     12345,
	})
	if err != nil {
		t.Fatalf("pack search sources req: %v", err)
	}
	opcode, msg, err := combiner.Unpack(raw)
	if err != nil {
		t.Fatalf("unpack search sources req: %v", err)
	}
	if opcode != SearchSrcReqOp {
		t.Fatalf("expected opcode %x, got %x", SearchSrcReqOp, opcode)
	}
	req, ok := msg.(*SearchSourcesReq)
	if !ok {
		t.Fatalf("expected SearchSourcesReq, got %T", msg)
	}
	if req.StartPos != 7 || req.Size != 12345 {
		t.Fatalf("unexpected unpacked request %+v", req)
	}
}

func TestPacketCombinerRoundTripFirewalledReq(t *testing.T) {
	combiner := PacketCombiner{}
	raw, err := combiner.Pack(FirewalledReq{
		TCPPort: 4661,
		ID:      NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Options: 3,
	})
	if err != nil {
		t.Fatalf("pack firewalled req: %v", err)
	}
	opcode, msg, err := combiner.Unpack(raw)
	if err != nil {
		t.Fatalf("unpack firewalled req: %v", err)
	}
	if opcode != FirewalledReqOp {
		t.Fatalf("expected opcode %x, got %x", FirewalledReqOp, opcode)
	}
	req, ok := msg.(*FirewalledReq)
	if !ok {
		t.Fatalf("expected FirewalledReq, got %T", msg)
	}
	if req.TCPPort != 4661 || req.Options != 3 {
		t.Fatalf("unexpected firewalled req %+v", req)
	}
}
