package protocol

import "bytes"

type BitField struct {
	bytes []byte
	size  int
}

func NewBitField(size int) BitField {
	bf := BitField{}
	bf.Resize(size)
	return bf
}

func (b *BitField) Resize(size int) {
	newBytes := make([]byte, bitsToBytes(size))
	copy(newBytes, b.bytes)
	b.bytes = newBytes
	b.size = size
	b.clearTrailingBits()
}

func (b *BitField) ResizeWith(size int, val bool) {
	oldSize := b.size
	oldSizeBytes := bitsToBytes(oldSize)
	b.Resize(size)
	if oldSize >= b.size {
		return
	}
	newSizeBytes := bitsToBytes(b.size)
	if val {
		if oldSizeBytes != 0 && oldSize&7 != 0 {
			b.bytes[oldSizeBytes-1] |= byte(0xff >> (oldSize & 7))
		}
		if oldSizeBytes < newSizeBytes {
			for i := oldSizeBytes; i < newSizeBytes; i++ {
				b.bytes[i] = 0xff
			}
		}
		b.clearTrailingBits()
	}
}

func (b *BitField) SetBit(index int) {
	b.bytes[index/8] |= byte(0x80 >> (index & 7))
}

func (b BitField) GetBit(index int) bool {
	if index < 0 || index >= b.size {
		return false
	}
	return (b.bytes[index/8] & byte(0x80>>(index&7))) != 0
}

func (b *BitField) ClearBit(index int) {
	b.bytes[index/8] &= ^byte(0x80 >> (index & 7))
}

func (b BitField) Bits() []bool {
	out := make([]bool, b.size)
	for i := 0; i < b.size; i++ {
		out[i] = b.GetBit(i)
	}
	return out
}

func (b BitField) Len() int {
	return b.size
}

func (b *BitField) SetAll() {
	for i := range b.bytes {
		b.bytes[i] = 0xff
	}
	b.clearTrailingBits()
}

func (b *BitField) ClearAll() {
	for i := range b.bytes {
		b.bytes[i] = 0
	}
}

func (b BitField) Count() int {
	ret := 0
	for i := 0; i < b.size; i++ {
		if b.GetBit(i) {
			ret++
		}
	}
	return ret
}

func (b *BitField) Get(src *bytes.Reader) error {
	size, err := readUInt16(src)
	if err != nil {
		return err
	}
	temp, err := readBytes(src, bitsToBytes(int(size)))
	if err != nil {
		return err
	}
	b.Assign(temp, int(size))
	return nil
}

func (b BitField) Put(dst *bytes.Buffer) error {
	if err := writeUInt16(dst, uint16(b.size)); err != nil {
		return err
	}
	_, err := dst.Write(b.bytes)
	return err
}

func (b BitField) BytesCount() int {
	return 2 + len(b.bytes)
}

func (b *BitField) Assign(raw []byte, bits int) {
	b.Resize(bits)
	copy(b.bytes, raw)
	b.clearTrailingBits()
}

func bitsToBytes(count int) int {
	return (count + 7) / 8
}

func (b *BitField) clearTrailingBits() {
	if (b.size&7) != 0 && len(b.bytes) > 0 {
		b.bytes[bitsToBytes(b.size)-1] &= byte(0xff << (8 - (b.size & 7)))
	}
}
