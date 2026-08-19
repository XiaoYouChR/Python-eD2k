package kad

import (
	"bytes"
	"compress/zlib"
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func TestPacketCombinerUnpacksCompressedPacket(t *testing.T) {
	original, err := SearchRes{
		Source: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Target: NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
	}.Pack()
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(original[2:]); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	packet := append([]byte{PackedProtocolHeader, SearchResOp}, compressed.Bytes()...)

	opcode, message, err := (PacketCombiner{}).Unpack(packet)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != SearchResOp {
		t.Fatalf("opcode = %#x, want %#x", opcode, SearchResOp)
	}
	if _, ok := message.(*SearchRes); !ok {
		t.Fatalf("message type = %T, want *SearchRes", message)
	}
}

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
