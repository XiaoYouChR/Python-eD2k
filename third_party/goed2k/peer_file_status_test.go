package goed2k

import (
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
	clientproto "github.com/monkeyWie/goed2k/protocol/client"
)

func TestZeroLengthFileStatusMeansCompleteSource(t *testing.T) {
	session, transfer := newTestTransfer(t)
	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	conn := NewPeerConnection(session, endpoint, transfer, nil)
	conn.HandleFileStatusAnswer(&clientproto.FileStatusAnswer{
		Hash:     transfer.GetHash(),
		BitField: protocol.NewBitField(0),
	})

	want := transfer.picker.NumPieces()
	if conn.remotePieces.Len() != want {
		t.Fatalf("complete source part map has length %d, want %d", conn.remotePieces.Len(), want)
	}
	if conn.remotePieces.Count() != want {
		t.Fatalf("complete source reports %d available pieces, want %d", conn.remotePieces.Count(), want)
	}
}

func TestFileStatusReusesCachedHashSet(t *testing.T) {
	session, transfer := newTestTransfer(t)
	transfer.SetHashSet(transfer.GetHash(), []protocol.Hash{protocol.LibED2K, protocol.Terminal})
	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatal(err)
	}
	conn := NewPeerConnection(session, endpoint, transfer, nil)
	conn.HandleFileStatusAnswer(&clientproto.FileStatusAnswer{
		Hash:     transfer.GetHash(),
		BitField: protocol.NewBitField(0),
	})

	packets := conn.PendingPackets()
	if len(packets) != 1 || len(packets[0]) < protocol.PacketHeaderSize {
		t.Fatalf("expected one start-upload packet, got %d", len(packets))
	}
	if opcode := packets[0][protocol.PacketHeaderSize-1]; opcode != 0x54 {
		t.Fatalf("expected start-upload opcode, got 0x%02X", opcode)
	}
}
