package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type StartUpload struct {
	Hash protocol.Hash
}

func (s *StartUpload) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	s.Hash = hash
	return nil
}

func (s StartUpload) Put(dst *bytes.Buffer) error {
	return protocol.WriteHash(dst, s.Hash)
}

func (s StartUpload) BytesCount() int {
	return 16
}
