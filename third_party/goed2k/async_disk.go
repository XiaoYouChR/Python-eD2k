package goed2k

import (
	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/protocol"
)

type AsyncOperationResult interface {
	OnCompleted()
	Code() BaseErrorCode
}

type TransferCallable interface {
	Transfer() *Transfer
	Call() AsyncOperationResult
}

type transferCallable struct {
	transfer *Transfer
}

func (t transferCallable) Transfer() *Transfer {
	return t.transfer
}

type AsyncWrite struct {
	transferCallable
	Block  data.PieceBlock
	Buffer []byte
}

func NewAsyncWrite(block data.PieceBlock, buffer []byte, transfer *Transfer) *AsyncWrite {
	return &AsyncWrite{transferCallable: transferCallable{transfer: transfer}, Block: block, Buffer: buffer}
}

func (a *AsyncWrite) Call() AsyncOperationResult {
	buffers, err := a.transfer.GetPieceManager().WriteBlock(a.Block, a.Buffer)
	if err != nil {
		return &AsyncWriteResult{Block: a.Block, Buffers: [][]byte{a.Buffer}, Transfer: a.transfer, EC: IOException}
	}
	return &AsyncWriteResult{Block: a.Block, Buffers: buffers, Transfer: a.transfer, EC: NoError}
}

type AsyncWriteResult struct {
	Block    data.PieceBlock
	Buffers  [][]byte
	Transfer *Transfer
	EC       BaseErrorCode
}

func (a *AsyncWriteResult) OnCompleted() {
	a.Transfer.OnBlockWriteCompleted(a.Block, a.Buffers, a.EC)
}

func (a *AsyncWriteResult) Code() BaseErrorCode {
	return a.EC
}

type AsyncHash struct {
	transferCallable
	PieceIndex int
}

func NewAsyncHash(transfer *Transfer, pieceIndex int) *AsyncHash {
	return &AsyncHash{transferCallable: transferCallable{transfer: transfer}, PieceIndex: pieceIndex}
}

func (a *AsyncHash) Call() AsyncOperationResult {
	hash := a.transfer.GetPieceManager().HashPiece(a.PieceIndex)
	return &AsyncHashResult{Hash: hash, Transfer: a.transfer, PieceIndex: a.PieceIndex}
}

type AsyncHashResult struct {
	Hash       protocol.Hash
	Transfer   *Transfer
	PieceIndex int
}

func (a *AsyncHashResult) OnCompleted() {
	a.Transfer.OnPieceHashCompleted(a.PieceIndex, a.Hash)
}

func (a *AsyncHashResult) Code() BaseErrorCode {
	return NoError
}

type AsyncRestore struct {
	transferCallable
	Block    data.PieceBlock
	FileSize int64
}

func NewAsyncRestore(transfer *Transfer, block data.PieceBlock, fileSize int64) *AsyncRestore {
	return &AsyncRestore{transferCallable: transferCallable{transfer: transfer}, Block: block, FileSize: fileSize}
}

func (a *AsyncRestore) Call() AsyncOperationResult {
	buffers, restored, err := a.transfer.GetPieceManager().RestoreBlock(a.Block, a.FileSize)
	if err != nil {
		return &AsyncRestoreResult{Block: a.Block, Buffers: nil, Transfer: a.transfer, EC: IOException}
	}
	return &AsyncRestoreResult{Block: a.Block, Buffers: append(buffers, restored), Transfer: a.transfer, EC: NoError}
}

type AsyncRestoreResult struct {
	Block    data.PieceBlock
	Buffers  [][]byte
	Transfer *Transfer
	EC       BaseErrorCode
}

func (a *AsyncRestoreResult) OnCompleted() {
	a.Transfer.OnBlockRestoreCompleted(a.Block, a.EC)
}

func (a *AsyncRestoreResult) Code() BaseErrorCode {
	return a.EC
}

type AsyncRelease struct {
	transferCallable
	DeleteFile bool
}

func NewAsyncRelease(transfer *Transfer, deleteFile bool) *AsyncRelease {
	return &AsyncRelease{transferCallable: transferCallable{transfer: transfer}, DeleteFile: deleteFile}
}

func (a *AsyncRelease) Call() AsyncOperationResult {
	buffers, _ := a.transfer.GetPieceManager().ReleaseFile(a.DeleteFile)
	return &AsyncReleaseResult{Transfer: a.transfer, Buffers: buffers, DeleteFile: a.DeleteFile, EC: NoError}
}

type AsyncReleaseResult struct {
	Transfer   *Transfer
	Buffers    [][]byte
	DeleteFile bool
	EC         BaseErrorCode
}

func (a *AsyncReleaseResult) OnCompleted() {
	a.Transfer.OnReleaseFile(a.EC, a.Buffers, a.DeleteFile)
}

func (a *AsyncReleaseResult) Code() BaseErrorCode {
	return a.EC
}
