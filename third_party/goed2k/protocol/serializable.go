package protocol

import (
	"bytes"
	"encoding/binary"
)

type Serializable interface {
	Get(src *bytes.Reader) error
	Put(dst *bytes.Buffer) error
	BytesCount() int
}

func readByte(src *bytes.Reader) (byte, error) {
	return src.ReadByte()
}

func writeByte(dst *bytes.Buffer, value byte) error {
	return dst.WriteByte(value)
}

func readInt32(src *bytes.Reader) (int32, error) {
	var value int32
	err := binary.Read(src, binary.LittleEndian, &value)
	return value, err
}

func writeInt32(dst *bytes.Buffer, value int32) error {
	return binary.Write(dst, binary.LittleEndian, value)
}

func writeUInt32(dst *bytes.Buffer, value uint32) error {
	return binary.Write(dst, binary.LittleEndian, value)
}

func readUInt16(src *bytes.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(src, binary.LittleEndian, &value)
	return value, err
}

func writeUInt16(dst *bytes.Buffer, value uint16) error {
	return binary.Write(dst, binary.LittleEndian, value)
}

func readUInt64(src *bytes.Reader) (uint64, error) {
	var value uint64
	err := binary.Read(src, binary.LittleEndian, &value)
	return value, err
}

func writeUInt64(dst *bytes.Buffer, value uint64) error {
	return binary.Write(dst, binary.LittleEndian, value)
}

func readBytes(src *bytes.Reader, size int) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, size)
	_, err := src.Read(buf)
	return buf, err
}
