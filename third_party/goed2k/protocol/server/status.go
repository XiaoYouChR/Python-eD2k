package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type Status struct {
	UsersCount int32
	FilesCount int32
}

func (s *Status) Get(src *bytes.Reader) error {
	users, err := protocol.ReadInt32(src)
	if err != nil {
		return err
	}
	files, err := protocol.ReadInt32(src)
	if err != nil {
		return err
	}
	s.UsersCount = users
	s.FilesCount = files
	return nil
}

func (s Status) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteInt32(dst, s.UsersCount); err != nil {
		return err
	}
	return protocol.WriteInt32(dst, s.FilesCount)
}

func (s Status) BytesCount() int { return 8 }
