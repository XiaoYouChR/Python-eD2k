package client

import "bytes"

type Hello struct {
	HashLength byte
	HelloAnswer
}

func (h *Hello) Get(src *bytes.Reader) error {
	value, err := src.ReadByte()
	if err != nil {
		return err
	}
	h.HashLength = value
	return h.HelloAnswer.Get(src)
}

func (h Hello) Put(dst *bytes.Buffer) error {
	if err := dst.WriteByte(h.HashLength); err != nil {
		return err
	}
	return h.HelloAnswer.Put(dst)
}

func (h Hello) BytesCount() int {
	return 1 + h.HelloAnswer.BytesCount()
}
