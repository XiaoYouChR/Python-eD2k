package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

const eMuleProtocol byte = 0x01

type ExtendedHandshake struct {
	Version         byte
	ProtocolVersion byte
	Properties      protocol.TagList
}

func (e *ExtendedHandshake) Get(src *bytes.Reader) error {
	version, err := src.ReadByte()
	if err != nil {
		return err
	}
	protoVersion, err := src.ReadByte()
	if err != nil {
		return err
	}
	e.Version = version
	e.ProtocolVersion = protoVersion
	if src.Len() > 0 {
		if err := e.Properties.Get(src); err != nil {
			return err
		}
	}
	return nil
}

func (e ExtendedHandshake) Put(dst *bytes.Buffer) error {
	if err := dst.WriteByte(e.Version); err != nil {
		return err
	}
	if err := dst.WriteByte(e.ProtocolVersion); err != nil {
		return err
	}
	return e.Properties.Put(dst)
}

func (e ExtendedHandshake) BytesCount() int {
	return 2 + e.Properties.BytesCount()
}
