package goed2k

import (
	"io"
	"os"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/protocol"
)

type PieceManager struct {
	BlocksEnumerator
	handler   disk.FileHandler
	blockMgrs []*BlockManager
}

func NewPieceManager(handler disk.FileHandler, pieceCount, blocksInLastPiece int) *PieceManager {
	return &PieceManager{
		BlocksEnumerator: NewBlocksEnumerator(pieceCount, blocksInLastPiece),
		handler:          handler,
		blockMgrs:        make([]*BlockManager, 0),
	}
}

func (p *PieceManager) getBlockManager(piece int) *BlockManager {
	for _, mgr := range p.blockMgrs {
		if mgr.PieceIndex() == piece {
			return mgr
		}
	}
	mgr := NewBlockManager(piece, p.BlocksInPiece(piece))
	p.blockMgrs = append(p.blockMgrs, mgr)
	return mgr
}

func (p *PieceManager) WriteBlock(block data.PieceBlock, buffer []byte) ([][]byte, error) {
	file := p.handler.File()
	if file == nil {
		return nil, NewError(IOException)
	}
	debugPeerf("piece manager write block piece=%d block=%d len=%d", block.PieceIndex, block.PieceBlock, len(buffer))
	bytesOffset := block.BlocksOffset() * BlockSize
	if _, err := file.Seek(bytesOffset, io.SeekStart); err != nil {
		_ = p.handler.Close()
		return nil, NewError(IOException)
	}
	if _, err := file.Write(buffer); err != nil {
		_ = p.handler.Close()
		return nil, NewError(IOException)
	}
	return p.getBlockManager(block.PieceIndex).RegisterBlock(block.PieceBlock, buffer), nil
}

func (p *PieceManager) RestoreBlock(block data.PieceBlock, fileSize int64) ([][]byte, []byte, error) {
	file := p.handler.File()
	if file == nil {
		return nil, nil, NewError(IOException)
	}
	bytesOffset := block.BlocksOffset() * BlockSize
	if _, err := file.Seek(bytesOffset, io.SeekStart); err != nil {
		return nil, nil, NewError(IOException)
	}
	buffer := make([]byte, block.Size(fileSize))
	if _, err := io.ReadFull(file, buffer); err != nil {
		return nil, nil, NewError(IOException)
	}
	res := p.getBlockManager(block.PieceIndex).RegisterBlock(block.PieceBlock, buffer)
	return res, buffer, nil
}

func (p *PieceManager) ReadRange(begin, end int64) ([]byte, error) {
	if end <= begin {
		return nil, NewError(IllegalArgument)
	}
	file := p.handler.File()
	if file == nil {
		return nil, NewError(IOException)
	}
	if _, err := file.Seek(begin, io.SeekStart); err != nil {
		return nil, NewError(IOException)
	}
	buffer := make([]byte, end-begin)
	if _, err := io.ReadFull(file, buffer); err != nil {
		return nil, NewError(IOException)
	}
	return buffer, nil
}

func (p *PieceManager) HashPiece(pieceIndex int) protocol.Hash {
	mgr := p.getBlockManager(pieceIndex)
	debugPeerf("piece manager hash piece=%d buffers=%d hashed=%d", pieceIndex, mgr.ByteBuffersCount(), mgr.HashedSize())
	hash := mgr.PieceHash()
	for i, item := range p.blockMgrs {
		if item == mgr {
			p.blockMgrs = append(p.blockMgrs[:i], p.blockMgrs[i+1:]...)
			break
		}
	}
	return hash
}

func (p *PieceManager) ReleaseFile(deleteFile bool) ([][]byte, error) {
	_ = p.handler.Close()
	if deleteFile {
		_ = p.handler.DeleteFile()
	}
	return p.Abort(), nil
}

func (p *PieceManager) Abort() [][]byte {
	res := make([][]byte, 0)
	for _, mgr := range p.blockMgrs {
		res = append(res, mgr.Buffers()...)
	}
	p.blockMgrs = nil
	return res
}

func (p *PieceManager) DeleteFile() error {
	return p.handler.DeleteFile()
}

func (p *PieceManager) GetFile() *os.File {
	return p.handler.File()
}
