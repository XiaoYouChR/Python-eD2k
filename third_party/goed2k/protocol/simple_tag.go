package protocol

import (
	"bytes"
	"fmt"
)

const (
	TagTypeString = 0x02
	TagTypeUint32 = 0x03
	TagTypeUint16 = 0x08
	TagTypeUint8  = 0x09
	TagTypeUint64 = 0x0B
	TagTypeStr1   = 0x11

	tagMinBytes = 2
)

type SimpleTag struct {
	Type   byte
	ID     byte
	Name   string
	String string
	UInt32 uint32
	UInt64 uint64
}

func (t *SimpleTag) Get(src *bytes.Reader) error {
	tagType, err := src.ReadByte()
	if err != nil {
		return err
	}
	t.Type = tagType & 0x7f
	t.ID = 0
	t.Name = ""
	t.String = ""
	t.UInt32 = 0
	t.UInt64 = 0
	if tagType&0x80 != 0 {
		tagID, err := src.ReadByte()
		if err != nil {
			return err
		}
		t.ID = tagID
	} else {
		nameSize, err := ReadUInt16(src)
		if err != nil {
			return err
		}
		raw, err := ReadBytes(src, int(nameSize))
		if err != nil {
			return err
		}
		if nameSize == 1 {
			t.ID = raw[0]
		} else {
			t.Name = string(raw)
		}
	}
	switch t.Type {
	case TagTypeUint8:
		value, err := src.ReadByte()
		if err != nil {
			return err
		}
		t.UInt32 = uint32(value)
		t.UInt64 = uint64(value)
	case TagTypeUint16:
		value, err := ReadUInt16(src)
		if err != nil {
			return err
		}
		t.UInt32 = uint32(value)
		t.UInt64 = uint64(value)
	case TagTypeUint32:
		value, err := ReadUInt32(src)
		if err != nil {
			return err
		}
		t.UInt32 = value
		t.UInt64 = uint64(value)
	case TagTypeUint64:
		value, err := ReadUInt64(src)
		if err != nil {
			return err
		}
		t.UInt32 = uint32(value)
		t.UInt64 = value
	case TagTypeString:
		size, err := ReadUInt16(src)
		if err != nil {
			return err
		}
		value, err := ReadBytes(src, int(size))
		if err != nil {
			return err
		}
		t.String = string(value)
	default:
		if t.Type >= TagTypeStr1 && t.Type <= TagTypeStr1+15 {
			size := int(t.Type-TagTypeStr1) + 1
			value, err := ReadBytes(src, size)
			if err != nil {
				return err
			}
			t.String = string(value)
		}
	}
	return nil
}

func (t SimpleTag) Put(dst *bytes.Buffer) error {
	if t.Name != "" {
		if err := dst.WriteByte(t.Type); err != nil {
			return err
		}
		if err := WriteUInt16(dst, uint16(len(t.Name))); err != nil {
			return err
		}
		if _, err := dst.WriteString(t.Name); err != nil {
			return err
		}
	} else {
		if err := dst.WriteByte(t.Type | 0x80); err != nil {
			return err
		}
		if err := dst.WriteByte(t.ID); err != nil {
			return err
		}
	}
	switch t.Type {
	case TagTypeUint8:
		return dst.WriteByte(byte(t.UInt64))
	case TagTypeUint16:
		return WriteUInt16(dst, uint16(t.UInt64))
	case TagTypeUint32:
		if t.UInt32 != 0 {
			return WriteUInt32(dst, t.UInt32)
		}
		return WriteUInt32(dst, uint32(t.UInt64))
	case TagTypeUint64:
		return WriteUInt64(dst, t.UInt64)
	case TagTypeString:
		if err := WriteUInt16(dst, uint16(len(t.String))); err != nil {
			return err
		}
		_, err := dst.WriteString(t.String)
		return err
	default:
		if t.Type >= TagTypeStr1 && t.Type <= TagTypeStr1+15 {
			_, err := dst.WriteString(t.String)
			return err
		}
	}
	return nil
}

func (t SimpleTag) BytesCount() int {
	size := 2
	if t.Name != "" {
		size = 1 + 2 + len(t.Name)
	}
	switch t.Type {
	case TagTypeUint8:
		return size + 1
	case TagTypeUint16:
		return size + 2
	case TagTypeUint32:
		return size + 4
	case TagTypeUint64:
		return size + 8
	case TagTypeString:
		return size + 2 + len(t.String)
	default:
		return size + len(t.String)
	}
}

func NewUInt32Tag(id byte, value uint32) SimpleTag {
	return SimpleTag{Type: TagTypeUint32, ID: id, UInt32: value, UInt64: uint64(value)}
}

func NewStringTag(id byte, value string) SimpleTag {
	if len(value) >= 1 && len(value) <= 16 {
		return SimpleTag{Type: TagTypeStr1 + byte(len(value)-1), ID: id, String: value}
	}
	return SimpleTag{Type: TagTypeString, ID: id, String: value}
}

type TagList []SimpleTag

func (t *TagList) Get(src *bytes.Reader) error {
	count, err := ReadUInt32(src)
	if err != nil {
		return err
	}
	maxCount := uint32(src.Len() / tagMinBytes)
	if count > maxCount {
		return fmt.Errorf("tag list declares %d tags, but payload can contain at most %d", count, maxCount)
	}
	list := make([]SimpleTag, int(count))
	for i := 0; i < int(count); i++ {
		if err := list[i].Get(src); err != nil {
			return err
		}
	}
	*t = list
	return nil
}

func (t TagList) Put(dst *bytes.Buffer) error {
	if err := WriteUInt32(dst, uint32(len(t))); err != nil {
		return err
	}
	for _, tag := range t {
		if err := tag.Put(dst); err != nil {
			return err
		}
	}
	return nil
}

func (t TagList) BytesCount() int {
	size := 4
	for _, tag := range t {
		size += tag.BytesCount()
	}
	return size
}
