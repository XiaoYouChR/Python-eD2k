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
