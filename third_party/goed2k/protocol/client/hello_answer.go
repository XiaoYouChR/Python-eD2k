package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type HelloAnswer struct {
	Hash        protocol.Hash
	Point       protocol.Endpoint
	Properties  protocol.TagList
	ServerPoint protocol.Endpoint
}

func (h *HelloAnswer) Get(src *bytes.Reader) error {
	raw, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	h.Hash = raw
	point, err := protocol.ReadEndpoint(src)
	if err != nil {
		return err
	}
	h.Point = point
	var properties protocol.TagList
	if err := properties.Get(src); err != nil {
		return err
	}
	h.Properties = properties
	serverPoint, err := protocol.ReadEndpoint(src)
	if err != nil {
		return err
	}
	h.ServerPoint = serverPoint
	return nil
}

func (h HelloAnswer) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, h.Hash); err != nil {
		return err
	}
	if err := protocol.WriteEndpoint(dst, h.Point); err != nil {
		return err
	}
	if err := h.Properties.Put(dst); err != nil {
		return err
	}
	return protocol.WriteEndpoint(dst, h.ServerPoint)
}

func (h HelloAnswer) BytesCount() int {
	return 16 + 6 + h.Properties.BytesCount() + 6
}
