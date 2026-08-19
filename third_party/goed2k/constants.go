package goed2k

const (
	PieceSize        int64 = 9728000
	BlockSize        int64 = 190 * 1024
	BlockSizeInt           = int(BlockSize)
	BlocksPerPiece         = int(PieceSize / BlockSize)
	HighestLowIDED2K int64 = 16777216
	RequestQueueSize       = 3
	PartsInRequest         = 3
)
