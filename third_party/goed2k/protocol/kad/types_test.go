package kad

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func TestIDRoundTrip(t *testing.T) {
	original := NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796"))
	var buffer bytes.Buffer
	if err := original.Put(&buffer); err != nil {
		t.Fatalf("put id: %v", err)
	}
	var restored ID
	if err := restored.Get(bytes.NewReader(buffer.Bytes())); err != nil {
		t.Fatalf("get id: %v", err)
	}
	if !restored.Hash.Equal(original.Hash) {
		t.Fatalf("expected %s, got %s", original.Hash.String(), restored.Hash.String())
	}
}

func TestSearchEntryExtractsSourceEndpoint(t *testing.T) {
	entry := SearchEntry{
		Tags: []Tag{
			{ID: TagSourceType, UInt64: 1},
			{ID: TagSourceIP, UInt64: uint64(uint32(0x0100007f))},
			{ID: TagSourcePort, UInt64: 4662},
		},
	}
	endpoint, ok := entry.SourceEndpoint()
	if !ok {
		t.Fatal("expected source endpoint to be extracted")
	}
	if endpoint.Port() != 4662 {
		t.Fatalf("unexpected port %d", endpoint.Port())
	}
	if endpoint.String() != "127.0.0.1:4662" {
		t.Fatalf("unexpected endpoint %s", endpoint.String())
	}
}

func TestBootstrapResRoundTrip(t *testing.T) {
	packet, err := BootstrapRes{
		ID:      NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		TCPPort: 4661,
		Version: KademliaVersion,
		Contacts: []Entry{
			{
				ID: NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
				Endpoint: Endpoint{
					IP:      0x0100007f,
					UDPPort: 4672,
					TCPPort: 4661,
				},
				Version: 8,
			},
		},
	}.Pack()
	if err != nil {
		t.Fatalf("pack bootstrap res: %v", err)
	}
	opcode, payload, err := DecodePacket(packet)
	if err != nil {
		t.Fatalf("decode packet: %v", err)
	}
	if opcode != BootstrapResOp {
		t.Fatalf("expected opcode %x, got %x", BootstrapResOp, opcode)
	}
	var restored BootstrapRes
	if err := restored.Unpack(payload); err != nil {
		t.Fatalf("unpack bootstrap res: %v", err)
	}
	if !restored.ID.Hash.Equal(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")) {
		t.Fatalf("unexpected id %s", restored.ID.Hash.String())
	}
	if restored.TCPPort != 4661 || restored.Version != KademliaVersion {
		t.Fatalf("unexpected header values tcp=%d version=%d", restored.TCPPort, restored.Version)
	}
	if len(restored.Contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(restored.Contacts))
	}
}

func TestReqResRoundTrip(t *testing.T) {
	reqPacket, err := Req{
		SearchType: FindNode,
		Target:     NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Receiver:   NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
	}.Pack()
	if err != nil {
		t.Fatalf("pack req: %v", err)
	}
	opcode, payload, err := DecodePacket(reqPacket)
	if err != nil {
		t.Fatalf("decode req: %v", err)
	}
	if opcode != ReqOp {
		t.Fatalf("expected opcode %x, got %x", ReqOp, opcode)
	}
	var req Req
	if err := req.Unpack(payload); err != nil {
		t.Fatalf("unpack req: %v", err)
	}
	if req.SearchType != FindNode {
		t.Fatalf("unexpected search type %d", req.SearchType)
	}

	resPacket, err := Res{
		Target: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Results: []Entry{
			{
				ID: NewID(protocol.MustHashFromString("31D6CFE0D14CE931B73C59D7E0C04BC0")),
				Endpoint: Endpoint{
					IP:      0x0100007f,
					UDPPort: 4672,
					TCPPort: 4661,
				},
				Version: 8,
			},
		},
	}.Pack()
	if err != nil {
		t.Fatalf("pack res: %v", err)
	}
	opcode, payload, err = DecodePacket(resPacket)
	if err != nil {
		t.Fatalf("decode res: %v", err)
	}
	if opcode != ResOp {
		t.Fatalf("expected opcode %x, got %x", ResOp, opcode)
	}
	var res Res
	if err := res.Unpack(payload); err != nil {
		t.Fatalf("unpack res: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Results))
	}
}

func TestPublishSourcesReqAndPublishResRoundTrip(t *testing.T) {
	reqPacket, err := PublishSourcesReq{
		FileID: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Source: SearchEntry{
			ID: NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
			Tags: []Tag{
				{Type: TagTypeUint8, ID: TagSourceType, UInt64: 1},
				{Type: TagTypeUint32, ID: TagSourceIP, UInt64: 0x0100007f},
				{Type: TagTypeUint16, ID: TagSourcePort, UInt64: 4662},
			},
		},
	}.Pack()
	if err != nil {
		t.Fatalf("pack publish source req: %v", err)
	}
	opcode, payload, err := DecodePacket(reqPacket)
	if err != nil {
		t.Fatalf("decode publish source req: %v", err)
	}
	if opcode != PublishSourceReqOp {
		t.Fatalf("expected opcode %x, got %x", PublishSourceReqOp, opcode)
	}
	var req PublishSourcesReq
	if err := req.Unpack(payload); err != nil {
		t.Fatalf("unpack publish source req: %v", err)
	}
	if _, ok := req.Source.SourceEndpoint(); !ok {
		t.Fatal("expected source endpoint to survive publish req round trip")
	}

	resPacket, err := PublishRes{
		FileID: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Count:  1,
	}.Pack()
	if err != nil {
		t.Fatalf("pack publish res: %v", err)
	}
	opcode, payload, err = DecodePacket(resPacket)
	if err != nil {
		t.Fatalf("decode publish res: %v", err)
	}
	if opcode != PublishResOp {
		t.Fatalf("expected opcode %x, got %x", PublishResOp, opcode)
	}
	var res PublishRes
	if err := res.Unpack(payload); err != nil {
		t.Fatalf("unpack publish res: %v", err)
	}
	if res.Count != 1 {
		t.Fatalf("expected publish res count 1, got %d", res.Count)
	}
}

func TestParseBootstrapNodesDat(t *testing.T) {
	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write zero prefix: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write bootstrap edition: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write contact count: %v", err)
	}
	entry := Entry{
		ID: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Endpoint: Endpoint{
			IP:      0x0100007f,
			UDPPort: 4672,
			TCPPort: 4661,
		},
		Version: 8,
	}
	if err := entry.Put(&payload); err != nil {
		t.Fatalf("write bootstrap contact: %v", err)
	}

	nodes, err := ParseNodesDat(payload.Bytes())
	if err != nil {
		t.Fatalf("parse bootstrap nodes.dat: %v", err)
	}
	if nodes.Version != 3 {
		t.Fatalf("expected version 3, got %d", nodes.Version)
	}
	if nodes.BootstrapEdition != 1 {
		t.Fatalf("expected bootstrap edition 1, got %d", nodes.BootstrapEdition)
	}
	if len(nodes.Contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(nodes.Contacts))
	}
	if nodes.Contacts[0].Verified {
		t.Fatal("bootstrap contact should not be marked verified")
	}
}

func TestSearchKeysReqAndPublishKeysReqRoundTrip(t *testing.T) {
	searchPacket, err := SearchKeysReq{
		Target:   NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		StartPos: 0,
	}.Pack()
	if err != nil {
		t.Fatalf("pack search keys req: %v", err)
	}
	opcode, payload, err := DecodePacket(searchPacket)
	if err != nil {
		t.Fatalf("decode search keys req: %v", err)
	}
	if opcode != SearchKeysReqOp {
		t.Fatalf("expected opcode %x, got %x", SearchKeysReqOp, opcode)
	}
	var searchReq SearchKeysReq
	if err := searchReq.Unpack(payload); err != nil {
		t.Fatalf("unpack search keys req: %v", err)
	}

	publishPacket, err := PublishKeysReq{
		KeywordID: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Sources: []SearchEntry{
			{
				ID: NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
				Tags: []Tag{
					{Type: TagTypeString, ID: 0x01, String: "demo.epub"},
				},
			},
		},
	}.Pack()
	if err != nil {
		t.Fatalf("pack publish keys req: %v", err)
	}
	opcode, payload, err = DecodePacket(publishPacket)
	if err != nil {
		t.Fatalf("decode publish keys req: %v", err)
	}
	if opcode != PublishKeysReqOp {
		t.Fatalf("expected opcode %x, got %x", PublishKeysReqOp, opcode)
	}
	var publishReq PublishKeysReq
	if err := publishReq.Unpack(payload); err != nil {
		t.Fatalf("unpack publish keys req: %v", err)
	}
	if len(publishReq.Sources) != 1 {
		t.Fatalf("expected 1 publish key source, got %d", len(publishReq.Sources))
	}
}

func TestParseNodesDatRejectsImpossibleContactCount(t *testing.T) {
	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write zero prefix: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0xFFFFFFFF)); err != nil {
		t.Fatalf("write contact count: %v", err)
	}

	if _, err := ParseNodesDat(payload.Bytes()); err == nil {
		t.Fatal("expected impossible contact count to fail")
	}
}

func TestPublishNotesReqAndPublishNotesResRoundTrip(t *testing.T) {
	reqPacket, err := PublishNotesReq{
		FileID: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Notes: []SearchEntry{
			{
				ID: NewID(protocol.MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")),
				Tags: []Tag{
					{Type: TagTypeString, ID: 0x01, String: "note"},
				},
			},
		},
	}.Pack()
	if err != nil {
		t.Fatalf("pack publish notes req: %v", err)
	}
	opcode, payload, err := DecodePacket(reqPacket)
	if err != nil {
		t.Fatalf("decode publish notes req: %v", err)
	}
	if opcode != PublishNotesReqOp {
		t.Fatalf("expected opcode %x, got %x", PublishNotesReqOp, opcode)
	}
	var req PublishNotesReq
	if err := req.Unpack(payload); err != nil {
		t.Fatalf("unpack publish notes req: %v", err)
	}
	if len(req.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(req.Notes))
	}

	resPacket, err := PublishNotesRes{
		PublishRes: PublishRes{
			FileID: NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
			Count:  1,
		},
	}.Pack()
	if err != nil {
		t.Fatalf("pack publish notes res: %v", err)
	}
	opcode, payload, err = DecodePacket(resPacket)
	if err != nil {
		t.Fatalf("decode publish notes res: %v", err)
	}
	if opcode != PublishNotesResOp {
		t.Fatalf("expected opcode %x, got %x", PublishNotesResOp, opcode)
	}
	var res PublishNotesRes
	if err := res.Unpack(payload); err != nil {
		t.Fatalf("unpack publish notes res: %v", err)
	}
	if res.Count != 1 {
		t.Fatalf("expected note publish res count 1, got %d", res.Count)
	}
}

func TestFirewalledPacketsRoundTrip(t *testing.T) {
	reqPacket, err := FirewalledReq{
		TCPPort: 4661,
		ID:      NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")),
		Options: 0,
	}.Pack()
	if err != nil {
		t.Fatalf("pack firewalled req: %v", err)
	}
	opcode, payload, err := DecodePacket(reqPacket)
	if err != nil {
		t.Fatalf("decode firewalled req: %v", err)
	}
	if opcode != FirewalledReqOp {
		t.Fatalf("expected opcode %x, got %x", FirewalledReqOp, opcode)
	}
	var req FirewalledReq
	if err := req.Unpack(payload); err != nil {
		t.Fatalf("unpack firewalled req: %v", err)
	}
	if req.TCPPort != 4661 {
		t.Fatalf("unexpected firewalled req port %d", req.TCPPort)
	}

	resPacket, err := FirewalledRes{IP: 0x0100007f}.Pack()
	if err != nil {
		t.Fatalf("pack firewalled res: %v", err)
	}
	opcode, payload, err = DecodePacket(resPacket)
	if err != nil {
		t.Fatalf("decode firewalled res: %v", err)
	}
	if opcode != FirewalledResOp {
		t.Fatalf("expected opcode %x, got %x", FirewalledResOp, opcode)
	}
	var res FirewalledRes
	if err := res.Unpack(payload); err != nil {
		t.Fatalf("unpack firewalled res: %v", err)
	}
	if res.IP != 0x0100007f {
		t.Fatalf("unexpected firewalled res ip %x", res.IP)
	}
}
