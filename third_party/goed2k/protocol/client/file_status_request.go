package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type FileStatusRequest struct {
	Hash protocol.Hash
}

func (f *FileStatusRequest) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	f.Hash = hash
	return nil
}

func (f FileStatusRequest) Put(dst *bytes.Buffer) error {
	return protocol.WriteHash(dst, f.Hash)
}

func (f FileStatusRequest) BytesCount() int {
	return 16
}
