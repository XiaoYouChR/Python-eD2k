package goed2k

import (
	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/protocol"
)

const EndGameDPLimit = 4

type PieceState byte

const (
	PieceNone PieceState = iota
	PieceDownloading
	PieceHave
)

type PiecePicker struct {
	BlocksEnumerator
	pieceStatus       []PieceState
	downloadingPieces []DownloadingPiece
}

func NewPiecePicker(pieceCount, blocksInLastPiece int) PiecePicker {
	return PiecePicker{
		BlocksEnumerator:  NewBlocksEnumerator(pieceCount, blocksInLastPiece),
		pieceStatus:       make([]PieceState, pieceCount),
		downloadingPieces: make([]DownloadingPiece, 0),
	}
}

func (p *PiecePicker) GetDownloadingPiece(index int) *DownloadingPiece {
	for i := range p.downloadingPieces {
		if p.downloadingPieces[i].PieceIndex == index {
			return &p.downloadingPieces[i]
		}
	}
	return nil
}

func (p *PiecePicker) MarkAsFinished(b data.PieceBlock) bool {
	dp := p.GetDownloadingPiece(b.PieceIndex)
	if dp != nil {
		dp.FinishBlock(b.PieceBlock)
		return true
	}
	return false
}

func (p *PiecePicker) MarkAsDownloading(b data.PieceBlock, peer *Peer) bool {
	dp := p.GetDownloadingPiece(b.PieceIndex)
	if dp != nil {
		dp.RequestBlock(b.PieceBlock, peer, PeerSpeedSlow)
		return true
	}
	return false
}

func (p *PiecePicker) MarkAsWriting(b data.PieceBlock) bool {
	dp := p.GetDownloadingPiece(b.PieceIndex)
	if dp != nil {
		return dp.WriteBlock(b.PieceBlock)
	}
	return false
}

func (p *PiecePicker) AbortDownload(b data.PieceBlock, peer *Peer) {
	dp := p.GetDownloadingPiece(b.PieceIndex)
	if dp != nil {
		dp.AbortDownloading(b.PieceBlock, peer)
	}
}

func (p *PiecePicker) IsBlockDownloaded(b data.PieceBlock) bool {
	if p.IsPieceFinished(b.PieceIndex) {
		return true
	}
	dp := p.GetDownloadingPiece(b.PieceIndex)
	if dp != nil {
		return dp.IsDownloaded(b.PieceBlock)
	}
	return false
}

func (p *PiecePicker) DownloadPiece(pieceIndex int) {
	if p.pieceStatus[pieceIndex] == PieceHave {
		return
	}
	if p.pieceStatus[pieceIndex] == PieceNone {
		p.downloadingPieces = append(p.downloadingPieces, NewDownloadingPiece(pieceIndex, p.BlocksInPiece(pieceIndex)))
		p.pieceStatus[pieceIndex] = PieceDownloading
	}
}

func (p *PiecePicker) ChooseNextPiece() bool {
	roundRobin := 0
	for i := 0; i < len(p.pieceStatus); i++ {
		if roundRobin == len(p.pieceStatus) {
			roundRobin = 0
		}
		current := roundRobin
		if p.pieceStatus[current] == PieceNone {
			p.downloadingPieces = append(p.downloadingPieces, NewDownloadingPiece(current, p.BlocksInPiece(current)))
			p.pieceStatus[current] = PieceDownloading
			return true
		}
		roundRobin++
	}
	return false
}

func (p *PiecePicker) addDownloadingBlocks(rq *[]data.PieceBlock, orderLength int, peer *Peer, speed PeerSpeed, endGame bool, available *protocol.BitField) int {
	res := 0
	for i := range p.downloadingPieces {
		if !pieceAllowed(available, p.downloadingPieces[i].PieceIndex) {
			continue
		}
		res += p.downloadingPieces[i].PickBlocks(rq, orderLength-res, peer, speed, endGame)
		if res == orderLength {
			break
		}
	}
	return res
}

func (p *PiecePicker) PickPieces(rq *[]data.PieceBlock, orderLength int, peer *Peer, speed PeerSpeed) {
	p.PickPiecesWithAvailability(rq, orderLength, peer, speed, nil)
}

func (p *PiecePicker) PickPiecesWithAvailability(rq *[]data.PieceBlock, orderLength int, peer *Peer, speed PeerSpeed, available *protocol.BitField) {
	numRequested := p.addDownloadingBlocks(rq, orderLength, peer, speed, false, available)
	if speed != PeerSpeedSlow && numRequested < orderLength && p.IsEndGame() {
		numRequested += p.addDownloadingBlocks(rq, orderLength-numRequested, peer, speed, true, available)
	}
	if numRequested < orderLength && p.ChooseNextPieceWithAvailability(available) {
		p.PickPiecesWithAvailability(rq, orderLength-numRequested, peer, speed, available)
	}
}

func (p *PiecePicker) ChooseNextPieceWithAvailability(available *protocol.BitField) bool {
	roundRobin := 0
	for i := 0; i < len(p.pieceStatus); i++ {
		if roundRobin == len(p.pieceStatus) {
			roundRobin = 0
		}
		current := roundRobin
		if p.pieceStatus[current] == PieceNone && pieceAllowed(available, current) {
			p.downloadingPieces = append(p.downloadingPieces, NewDownloadingPiece(current, p.BlocksInPiece(current)))
			p.pieceStatus[current] = PieceDownloading
			return true
		}
		roundRobin++
	}
	return false
}

func (p *PiecePicker) RestorePiece(pieceIndex int) {
	dp := p.GetDownloadingPiece(pieceIndex)
	if dp != nil {
		dst := p.downloadingPieces[:0]
		for _, item := range p.downloadingPieces {
			if item.PieceIndex != pieceIndex {
				dst = append(dst, item)
			}
		}
		p.downloadingPieces = dst
	}
	p.pieceStatus[pieceIndex] = PieceNone
}

func (p PiecePicker) NumHave() int {
	res := 0
	for _, b := range p.pieceStatus {
		if b == PieceHave {
			res++
		}
	}
	return res
}

func (p PiecePicker) TotalPieces() int          { return len(p.pieceStatus) }
func (p PiecePicker) NumPieces() int            { return len(p.pieceStatus) }
func (p PiecePicker) NumDownloadingPieces() int { return len(p.downloadingPieces) }
func (p PiecePicker) GetPieceCount() int        { return len(p.pieceStatus) }

func (p PiecePicker) IsEndGame() bool {
	return p.TotalPieces()-p.NumHave()-p.NumDownloadingPieces() == 0 || p.NumDownloadingPieces() > EndGameDPLimit
}

func (p *PiecePicker) WeHave(pieceIndex int) {
	dst := p.downloadingPieces[:0]
	for _, item := range p.downloadingPieces {
		if item.PieceIndex != pieceIndex {
			dst = append(dst, item)
		}
	}
	p.downloadingPieces = dst
	p.pieceStatus[pieceIndex] = PieceHave
}

func (p *PiecePicker) RestoreHave(pieceIndex int) {
	p.pieceStatus[pieceIndex] = PieceHave
}

func (p PiecePicker) HavePiece(pieceIndex int) bool {
	return p.pieceStatus[pieceIndex] == PieceHave
}

func (p PiecePicker) IsPieceFinished(pieceIndex int) bool {
	if p.pieceStatus[pieceIndex] == PieceNone {
		return false
	}
	if p.pieceStatus[pieceIndex] == PieceHave {
		return true
	}
	dp := p.GetDownloadingPiece(pieceIndex)
	if dp == nil {
		return false
	}
	return dp.BlocksCount() == (dp.FinishedBlocksCount() + dp.WritingBlocksCount())
}

func (p *PiecePicker) WeHaveBlock(b data.PieceBlock) {
	dp := p.GetDownloadingPiece(b.PieceIndex)
	if dp == nil {
		p.downloadingPieces = append(p.downloadingPieces, NewDownloadingPiece(b.PieceIndex, p.BlocksInPiece(b.PieceIndex)))
		dp = &p.downloadingPieces[len(p.downloadingPieces)-1]
		p.pieceStatus[b.PieceIndex] = PieceDownloading
	}
	dp.FinishBlock(b.PieceBlock)
}

func (p PiecePicker) GetDownloadingQueue() []DownloadingPiece {
	out := make([]DownloadingPiece, len(p.downloadingPieces))
	copy(out, p.downloadingPieces)
	return out
}

func pieceAllowed(available *protocol.BitField, pieceIndex int) bool {
	if available == nil || available.Len() == 0 {
		return true
	}
	return available.GetBit(pieceIndex)
}
