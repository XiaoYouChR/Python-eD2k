package protocol

import (
	"bytes"
	"encoding/binary"
)

func ReadHash(src *bytes.Reader) (Hash, error) {
	raw, err := readBytes(src, 16)
	if err != nil {
		return Invalid, err
	}
	return HashFromBytes(raw)
}

func WriteHash(dst *bytes.Buffer, hash Hash) error {
	_, err := dst.Write(hash.Bytes())
	return err
}

func ReadEndpoint(src *bytes.Reader) (Endpoint, error) {
	var ip int32
	if err := binary.Read(src, binary.LittleEndian, &ip); err != nil {
		return Endpoint{}, err
	}
	var port uint16
	if err := binary.Read(src, binary.LittleEndian, &port); err != nil {
		return Endpoint{}, err
	}
	return NewEndpoint(ip, int(port)), nil
}

func WriteEndpoint(dst *bytes.Buffer, endpoint Endpoint) error {
	if err := binary.Write(dst, binary.LittleEndian, endpoint.IP()); err != nil {
		return err
	}
	return binary.Write(dst, binary.LittleEndian, uint16(endpoint.Port()))
}

func ReadInt32(src *bytes.Reader) (int32, error) {
	return readInt32(src)
}

func WriteInt32(dst *bytes.Buffer, value int32) error {
	return writeInt32(dst, value)
}

func ReadUInt16(src *bytes.Reader) (uint16, error) {
	return readUInt16(src)
}

func WriteUInt16(dst *bytes.Buffer, value uint16) error {
	return writeUInt16(dst, value)
}

func ReadUInt32(src *bytes.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(src, binary.LittleEndian, &value)
	return value, err
}

func WriteUInt32(dst *bytes.Buffer, value uint32) error {
	return writeUInt32(dst, value)
}

func ReadUInt64(src *bytes.Reader) (uint64, error) {
	return readUInt64(src)
}

func WriteUInt64(dst *bytes.Buffer, value uint64) error {
	return writeUInt64(dst, value)
}

func ReadBytes(src *bytes.Reader, size int) ([]byte, error) {
	return readBytes(src, size)
}
