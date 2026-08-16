package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type NoFileStatus struct {
	Hash protocol.Hash
}

func (n *NoFileStatus) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	n.Hash = hash
	return nil
}

func (n NoFileStatus) Put(dst *bytes.Buffer) error {
	return protocol.WriteHash(dst, n.Hash)
}

func (n NoFileStatus) BytesCount() int {
	return 16
}
