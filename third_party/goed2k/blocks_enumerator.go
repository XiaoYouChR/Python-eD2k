package goed2k

type BlocksEnumerator struct {
	pieceCount        int
	blocksInLastPiece int
}

func NewBlocksEnumerator(pieceCount, blocksInLastPiece int) BlocksEnumerator {
	return BlocksEnumerator{
		pieceCount:        pieceCount,
		blocksInLastPiece: blocksInLastPiece,
	}
}

func (b BlocksEnumerator) BlocksInPiece(pieceIndex int) int {
	if pieceIndex == b.pieceCount-1 {
		return b.blocksInLastPiece
	}
	return BlocksPerPiece
}
