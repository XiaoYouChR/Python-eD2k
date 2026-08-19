package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type QueueRanking struct {
	Rank     uint16
	Padding1 uint32
	Padding2 uint32
	Padding3 uint16
}

func (q *QueueRanking) Get(src *bytes.Reader) error {
	value, err := protocol.ReadUInt16(src)
	if err != nil {
		return err
	}
	q.Rank = value
	if src.Len() >= 4 {
		if q.Padding1, err = protocol.ReadUInt32(src); err != nil {
			return err
		}
	}
	if src.Len() >= 4 {
		if q.Padding2, err = protocol.ReadUInt32(src); err != nil {
			return err
		}
	}
	if src.Len() >= 2 {
		if q.Padding3, err = protocol.ReadUInt16(src); err != nil {
			return err
		}
	}
	return nil
}

func (q QueueRanking) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteUInt16(dst, q.Rank); err != nil {
		return err
	}
	if err := protocol.WriteUInt32(dst, q.Padding1); err != nil {
		return err
	}
	if err := protocol.WriteUInt32(dst, q.Padding2); err != nil {
		return err
	}
	return protocol.WriteUInt16(dst, q.Padding3)
}

func (q QueueRanking) BytesCount() int {
	return 12
}
