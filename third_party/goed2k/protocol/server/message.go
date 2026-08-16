package server

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

type Message struct {
	Value protocol.ByteContainer16
}

func (m *Message) Get(src *bytes.Reader) error {
	return m.Value.Get(src)
}

func (m Message) Put(dst *bytes.Buffer) error {
	return m.Value.Put(dst)
}

func (m Message) BytesCount() int {
	return m.Value.BytesCount()
}

func (m Message) AsString() string {
	return m.Value.AsString()
}
