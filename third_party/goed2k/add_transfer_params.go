package goed2k

import (
	"os"

	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/protocol"
)

type AddTransferParams struct {
	Hash       protocol.Hash
	CreateTime int64
	Size       int64
	FilePath   string
	Paused     bool
	ResumeData *protocol.TransferResumeData
	Handler    disk.FileHandler
}

func NewAddTransferParamsFromFile(h protocol.Hash, createTime int64, size int64, file *os.File, paused bool) AddTransferParams {
	atp := AddTransferParams{
		Hash:       h,
		CreateTime: createTime,
		Size:       size,
		Paused:     paused,
	}
	if file != nil {
		atp.FilePath = file.Name()
		atp.Handler = disk.NewDesktopFileHandler(file.Name())
	}
	return atp
}

func NewAddTransferParamsFromHandler(h protocol.Hash, createTime int64, size int64, handler disk.FileHandler, paused bool) AddTransferParams {
	atp := AddTransferParams{
		Hash:       h,
		CreateTime: createTime,
		Size:       size,
		Paused:     paused,
		Handler:    handler,
	}
	if handler != nil {
		atp.FilePath = handler.Path()
	}
	return atp
}

func (a *AddTransferParams) SetExternalFileHandler(handler disk.FileHandler) {
	a.Handler = handler
	if handler != nil {
		a.FilePath = handler.Path()
	}
}
