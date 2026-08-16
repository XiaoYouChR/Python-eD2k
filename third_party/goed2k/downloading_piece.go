package goed2k

import "github.com/monkeyWie/goed2k/data"

type BlockState byte

const (
	StateNone BlockState = iota
	StateRequested
	StateWriting
	StateFinished
)

type DownloadingBlock struct {
	state            BlockState
	downloadersCount int16
	lastDownloader   *Peer
	speed            PeerSpeed
}

func (b DownloadingBlock) IsRequested() bool { return b.state == StateRequested }
func (b DownloadingBlock) IsFinished() bool  { return b.state == StateFinished }
func (b DownloadingBlock) IsWriting() bool   { return b.state == StateWriting }
func (b DownloadingBlock) IsFree() bool      { return b.state == StateNone }

func (b *DownloadingBlock) Request(p *Peer, speed PeerSpeed) {
	b.downloadersCount++
	b.lastDownloader = p
	b.state = StateRequested
	b.speed = speed
}

func (b *DownloadingBlock) Write() {
	b.downloadersCount = 0
	b.lastDownloader = nil
	b.state = StateWriting
}

func (b *DownloadingBlock) Finish() {
	b.state = StateFinished
}

func (b *DownloadingBlock) Abort(p *Peer) {
	if b.downloadersCount > 0 {
		b.downloadersCount--
	}
	if b.lastDownloader == p {
		b.lastDownloader = nil
	}
	if b.downloadersCount == 0 {
		b.state = StateNone
	}
}

type DownloadingPiece struct {
	PieceIndex  int
	blocksCount int
	Blocks      []DownloadingBlock
}

func NewDownloadingPiece(pieceIndex, blocksCount int) DownloadingPiece {
	blocks := make([]DownloadingBlock, blocksCount)
	return DownloadingPiece{PieceIndex: pieceIndex, blocksCount: blocksCount, Blocks: blocks}
}

func (d DownloadingPiece) calculateStateBlocks(state BlockState) int {
	res := 0
	for _, b := range d.Blocks {
		if b.state == state {
			res++
		}
	}
	return res
}

func (d DownloadingPiece) FinishedBlocksCount() int    { return d.calculateStateBlocks(StateFinished) }
func (d DownloadingPiece) DownloadingBlocksCount() int { return d.calculateStateBlocks(StateRequested) }
func (d DownloadingPiece) WritingBlocksCount() int     { return d.calculateStateBlocks(StateWriting) }
func (d DownloadingPiece) DownloadedCount() int {
	return d.WritingBlocksCount() + d.FinishedBlocksCount()
}
func (d DownloadingPiece) TotalBlocks() int { return len(d.Blocks) }
func (d DownloadingPiece) BlocksCount() int { return d.blocksCount }

func (d *DownloadingPiece) FinishBlock(blockIndex int) {
	d.Blocks[blockIndex].Finish()
}

func (d *DownloadingPiece) RequestBlock(blockIndex int, p *Peer, speed PeerSpeed) {
	d.Blocks[blockIndex].Request(p, speed)
}

func (d *DownloadingPiece) WriteBlock(blockIndex int) bool {
	if d.Blocks[blockIndex].IsFree() || d.Blocks[blockIndex].IsRequested() {
		d.Blocks[blockIndex].Write()
		return true
	}
	return false
}

func (d DownloadingPiece) IsDownloaded(blockIndex int) bool {
	return d.Blocks[blockIndex].IsFinished() || d.Blocks[blockIndex].IsWriting()
}

func (d DownloadingPiece) IsFinished(blockIndex int) bool {
	return d.Blocks[blockIndex].IsFinished()
}

func (d *DownloadingPiece) AbortDownloading(blockIndex int, p *Peer) {
	d.Blocks[blockIndex].Abort(p)
}

func (d *DownloadingPiece) PickBlocks(rq *[]data.PieceBlock, orderLength int, peer *Peer, speed PeerSpeed, endGame bool) int {
	res := 0
	if !endGame && d.DownloadingBlocksCount() == d.TotalBlocks() {
		return res
	}
	for i := 0; i < d.TotalBlocks() && res < orderLength; i++ {
		if d.Blocks[i].IsFree() {
			*rq = append(*rq, data.NewPieceBlock(d.PieceIndex, i))
			d.Blocks[i].Request(peer, speed)
			res++
			continue
		}
		if endGame && d.Blocks[i].IsRequested() {
			if d.Blocks[i].downloadersCount < 2 && d.Blocks[i].speed < speed && d.Blocks[i].lastDownloader != peer {
				d.Blocks[i].Request(peer, speed)
				*rq = append(*rq, data.NewPieceBlock(d.PieceIndex, i))
				res++
			}
		}
	}
	return res
}
