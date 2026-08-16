package protocol

import "encoding/binary"

func md4Sum(data []byte) [16]byte {
	msg := make([]byte, len(data), len(data)+72)
	copy(msg, data)
	msg = append(msg, 0x80)
	for len(msg)%64 != 56 {
		msg = append(msg, 0)
	}
	bitLen := uint64(len(data)) * 8
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], bitLen)
	msg = append(msg, lenBuf[:]...)

	a := uint32(0x67452301)
	b := uint32(0xefcdab89)
	c := uint32(0x98badcfe)
	d := uint32(0x10325476)

	for i := 0; i < len(msg); i += 64 {
		var x [16]uint32
		for j := 0; j < 16; j++ {
			x[j] = binary.LittleEndian.Uint32(msg[i+4*j : i+4*(j+1)])
		}
		aa, bb, cc, dd := a, b, c, d

		a = ff(a, b, c, d, x[0], 3)
		d = ff(d, a, b, c, x[1], 7)
		c = ff(c, d, a, b, x[2], 11)
		b = ff(b, c, d, a, x[3], 19)
		a = ff(a, b, c, d, x[4], 3)
		d = ff(d, a, b, c, x[5], 7)
		c = ff(c, d, a, b, x[6], 11)
		b = ff(b, c, d, a, x[7], 19)
		a = ff(a, b, c, d, x[8], 3)
		d = ff(d, a, b, c, x[9], 7)
		c = ff(c, d, a, b, x[10], 11)
		b = ff(b, c, d, a, x[11], 19)
		a = ff(a, b, c, d, x[12], 3)
		d = ff(d, a, b, c, x[13], 7)
		c = ff(c, d, a, b, x[14], 11)
		b = ff(b, c, d, a, x[15], 19)

		a = gg(a, b, c, d, x[0], 3)
		d = gg(d, a, b, c, x[4], 5)
		c = gg(c, d, a, b, x[8], 9)
		b = gg(b, c, d, a, x[12], 13)
		a = gg(a, b, c, d, x[1], 3)
		d = gg(d, a, b, c, x[5], 5)
		c = gg(c, d, a, b, x[9], 9)
		b = gg(b, c, d, a, x[13], 13)
		a = gg(a, b, c, d, x[2], 3)
		d = gg(d, a, b, c, x[6], 5)
		c = gg(c, d, a, b, x[10], 9)
		b = gg(b, c, d, a, x[14], 13)
		a = gg(a, b, c, d, x[3], 3)
		d = gg(d, a, b, c, x[7], 5)
		c = gg(c, d, a, b, x[11], 9)
		b = gg(b, c, d, a, x[15], 13)

		a = hh(a, b, c, d, x[0], 3)
		d = hh(d, a, b, c, x[8], 9)
		c = hh(c, d, a, b, x[4], 11)
		b = hh(b, c, d, a, x[12], 15)
		a = hh(a, b, c, d, x[2], 3)
		d = hh(d, a, b, c, x[10], 9)
		c = hh(c, d, a, b, x[6], 11)
		b = hh(b, c, d, a, x[14], 15)
		a = hh(a, b, c, d, x[1], 3)
		d = hh(d, a, b, c, x[9], 9)
		c = hh(c, d, a, b, x[5], 11)
		b = hh(b, c, d, a, x[13], 15)
		a = hh(a, b, c, d, x[3], 3)
		d = hh(d, a, b, c, x[11], 9)
		c = hh(c, d, a, b, x[7], 11)
		b = hh(b, c, d, a, x[15], 15)

		a += aa
		b += bb
		c += cc
		d += dd
	}

	var out [16]byte
	binary.LittleEndian.PutUint32(out[0:4], a)
	binary.LittleEndian.PutUint32(out[4:8], b)
	binary.LittleEndian.PutUint32(out[8:12], c)
	binary.LittleEndian.PutUint32(out[12:16], d)
	return out
}

func ff(a, b, c, d, x uint32, s uint) uint32 {
	return rol(a+((b&c)|(^b&d))+x, s)
}

func gg(a, b, c, d, x uint32, s uint) uint32 {
	return rol(a+((b&c)|(b&d)|(c&d))+x+0x5a827999, s)
}

func hh(a, b, c, d, x uint32, s uint) uint32 {
	return rol(a+(b^c^d)+x+0x6ed9eba1, s)
}

func rol(x uint32, s uint) uint32 {
	return (x << s) | (x >> (32 - s))
}
