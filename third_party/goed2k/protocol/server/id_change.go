package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type IdChange struct {
	ClientID int32
	TCPFlags int32
	AuxPort  int32
}

func (i *IdChange) Get(src *bytes.Reader) error {
	clientID, err := protocol.ReadInt32(src)
	if err != nil {
		return err
	}
	i.ClientID = clientID
	if src.Len() >= 4 {
		tcpFlags, err := protocol.ReadInt32(src)
		if err != nil {
			return err
		}
		i.TCPFlags = tcpFlags
	}
	if src.Len() >= 4 {
		auxPort, err := protocol.ReadInt32(src)
		if err != nil {
			return err
		}
		i.AuxPort = auxPort
	}
	return nil
}

func (i IdChange) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteInt32(dst, i.ClientID); err != nil {
		return err
	}
	if err := protocol.WriteInt32(dst, i.TCPFlags); err != nil {
		return err
	}
	return protocol.WriteInt32(dst, i.AuxPort)
}

func (i IdChange) BytesCount() int { return 12 }
