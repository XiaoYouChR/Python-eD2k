package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type FileRequest struct {
	Hash protocol.Hash
}

func (f *FileRequest) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	f.Hash = hash
	return nil
}

func (f FileRequest) Put(dst *bytes.Buffer) error {
	return protocol.WriteHash(dst, f.Hash)
}

func (f FileRequest) BytesCount() int {
	return 16
}
