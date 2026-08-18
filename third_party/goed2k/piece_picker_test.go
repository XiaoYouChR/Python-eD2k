package goed2k

import (
	"testing"

	"github.com/monkeyWie/goed2k/data"
)

func TestPiecePickerStartsUntouchedPieceBeforeDuplicateRequests(t *testing.T) {
	picker := NewPiecePicker(10, 2)
	slowPeer := &Peer{}
	fastPeer := &Peer{}
	for pieceIndex := 0; pieceIndex < 5; pieceIndex++ {
		picker.DownloadPiece(pieceIndex)
		piece := picker.GetDownloadingPiece(pieceIndex)
		for blockIndex := 0; blockIndex < piece.BlocksCount(); blockIndex++ {
			piece.RequestBlock(blockIndex, slowPeer, PeerSpeedSlow)
		}
	}

	var picked []data.PieceBlock
	picker.PickPieces(&picked, 1, fastPeer, PeerSpeedFast)

	if len(picked) != 1 {
		t.Fatalf("expected one block, got %d", len(picked))
	}
	if picked[0].PieceIndex != 5 {
		t.Fatalf("picked duplicate from piece %d while piece 5 was untouched", picked[0].PieceIndex)
	}
}

func TestPiecePickerDuplicatesOnlyWhenNoUntouchedPieceIsAvailable(t *testing.T) {
	picker := NewPiecePicker(1, 2)
	slowPeer := &Peer{}
	fastPeer := &Peer{}
	picker.DownloadPiece(0)
	piece := picker.GetDownloadingPiece(0)
	for blockIndex := 0; blockIndex < piece.BlocksCount(); blockIndex++ {
		piece.RequestBlock(blockIndex, slowPeer, PeerSpeedSlow)
	}

	var picked []data.PieceBlock
	picker.PickPieces(&picked, 1, fastPeer, PeerSpeedFast)

	if len(picked) != 1 || picked[0].PieceIndex != 0 {
		t.Fatalf("expected one end-game duplicate, got %+v", picked)
	}
}
