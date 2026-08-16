package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type CallbackRequestIncoming struct {
	Point protocol.Endpoint
}

func (c *CallbackRequestIncoming) Get(src *bytes.Reader) error {
	ep, err := protocol.ReadEndpoint(src)
	if err != nil {
		return err
	}
	c.Point = ep
	return nil
}

func (c CallbackRequestIncoming) Put(dst *bytes.Buffer) error {
	return protocol.WriteEndpoint(dst, c.Point)
}

func (c CallbackRequestIncoming) BytesCount() int { return 6 }
