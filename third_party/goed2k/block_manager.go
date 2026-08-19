package goed2k

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type BlockManager struct {
	piece           int
	lastHashedBlock int
	buffers         [][]byte
	hashed          bytes.Buffer
	pieceHash       *protocol.Hash
}

func NewBlockManager(piece, buffersCount int) *BlockManager {
	return &BlockManager{
		piece:           piece,
		lastHashedBlock: -1,
		buffers:         make([][]byte, buffersCount),
	}
}

func (b *BlockManager) RegisterBlock(blockIndex int, buffer []byte) [][]byte {
	freeBuffers := make([][]byte, 0)
	b.buffers[blockIndex] = bytes.Clone(buffer)
	debugPeerf("block manager piece=%d register block=%d len=%d last=%d", b.piece, blockIndex, len(buffer), b.lastHashedBlock)
	if b.lastHashedBlock+1 == blockIndex {
		for i := blockIndex; i < len(b.buffers); i++ {
			if b.buffers[i] == nil {
				break
			}
			b.lastHashedBlock++
			b.hashed.Write(b.buffers[i])
			debugPeerf("block manager piece=%d hashed block=%d total=%d", b.piece, i, b.hashed.Len())
			freeBuffers = append(freeBuffers, b.buffers[i])
			b.buffers[i] = nil
		}
	}
	return freeBuffers
}

func (b *BlockManager) PieceHash() protocol.Hash {
	if b.pieceHash == nil {
		h, _ := protocol.HashFromData(b.hashed.Bytes())
		b.pieceHash = &h
	}
	return *b.pieceHash
}

func (b *BlockManager) PieceIndex() int {
	return b.piece
}

func (b *BlockManager) ByteBuffersCount() int {
	res := 0
	for _, buf := range b.buffers {
		if buf != nil {
			res++
		}
	}
	return res
}

func (b *BlockManager) HashedSize() int {
	return b.hashed.Len()
}

func (b *BlockManager) Buffers() [][]byte {
	res := make([][]byte, 0)
	for _, buf := range b.buffers {
		if buf != nil {
			res = append(res, buf)
		}
	}
	return res
}
