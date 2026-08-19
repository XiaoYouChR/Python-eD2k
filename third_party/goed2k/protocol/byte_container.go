package protocol

import (
	"bytes"
	"errors"
)

type ByteContainer16 struct {
	Value []byte
}

func ByteContainer16FromString(value string) ByteContainer16 {
	return ByteContainer16{Value: []byte(value)}
}

func (b *ByteContainer16) Get(src *bytes.Reader) error {
	size, err := ReadUInt16(src)
	if err != nil {
		return err
	}
	if size > 4096 {
		return errors.New("buffer too large")
	}
	value, err := readBytes(src, int(size))
	if err != nil {
		return err
	}
	b.Value = value
	return nil
}

func (b ByteContainer16) Put(dst *bytes.Buffer) error {
	if err := WriteUInt16(dst, uint16(len(b.Value))); err != nil {
		return err
	}
	_, err := dst.Write(b.Value)
	return err
}

func (b ByteContainer16) BytesCount() int {
	return 2 + len(b.Value)
}

func (b ByteContainer16) AsString() string {
	return string(b.Value)
}
