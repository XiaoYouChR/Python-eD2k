package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type CallbackRequest struct {
	ClientID int32
}

func (c *CallbackRequest) Get(src *bytes.Reader) error {
	value, err := protocol.ReadInt32(src)
	if err != nil {
		return err
	}
	c.ClientID = value
	return nil
}

func (c CallbackRequest) Put(dst *bytes.Buffer) error {
	return protocol.WriteInt32(dst, c.ClientID)
}

func (c CallbackRequest) BytesCount() int {
	return 4
}
