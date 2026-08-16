package protocol

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const hashSize = 16

type Hash struct {
	value [hashSize]byte
}

var (
	Terminal = MustHashFromString("31D6CFE0D16AE931B73C59D7E0C089C0")
	LibED2K  = MustHashFromString("31D6CFE0D14CE931B73C59D7E0C04BC0")
	EMule    = MustHashFromString("31D6CFE0D10EE931B73C59D7E0C06FC0")
	Invalid  Hash
)

func MustHashFromString(value string) Hash {
	h, err := HashFromString(value)
	if err != nil {
		panic(err)
	}
	return h
}

func HashFromString(value string) (Hash, error) {
	if len(value) != hashSize*2 {
		return Invalid, errors.New("illegal argument")
	}
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != hashSize {
		return Invalid, errors.New("illegal argument")
	}
	return HashFromBytes(raw)
}

func HashFromBytes(value []byte) (Hash, error) {
	if len(value) != hashSize {
		return Invalid, errors.New("illegal argument")
	}
	var res Hash
	copy(res.value[:], value)
	return res, nil
}

func RandomHash(eMule bool) (Hash, error) {
	var source [hashSize]byte
	if _, err := rand.Read(source[:]); err != nil {
		return Invalid, err
	}
	if eMule {
		source[5] = 14
		source[14] = 111
	}
	return HashFromBytes(source[:])
}

func (h *Hash) Assign(other Hash) Hash {
	copy(h.value[:], other.value[:])
	return *h
}

func (h Hash) At(index int) byte {
	return h.value[index]
}

func (h *Hash) Set(index int, value byte) {
	h.value[index] = value
}

func (h Hash) Bytes() []byte {
	raw := make([]byte, hashSize)
	copy(raw, h.value[:])
	return raw
}

func (h Hash) String() string {
	return strings.ToUpper(hex.EncodeToString(h.value[:]))
}

func (h Hash) Equal(other Hash) bool {
	return bytes.Equal(h.value[:], other.value[:])
}

func (h Hash) Compare(other Hash) int {
	return bytes.Compare(h.value[:], other.value[:])
}

func (h Hash) IsZero() bool {
	return h.Equal(Invalid)
}

func (h Hash) MarshalJSON() ([]byte, error) {
	if h.Equal(Invalid) {
		return json.Marshal("")
	}
	return json.Marshal(h.String())
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		*h = Invalid
		return nil
	}
	hash, err := HashFromString(value)
	if err != nil {
		return err
	}
	*h = hash
	return nil
}

func HashFromData(value []byte) (Hash, error) {
	sum := md4Sum(value)
	return HashFromBytes(sum[:])
}

func HashFromHashSet(hashes []Hash) Hash {
	if len(hashes) == 0 {
		return Invalid
	}
	if len(hashes) == 1 {
		return hashes[0]
	}
	buffer := make([]byte, 0, len(hashes)*hashSize)
	for _, hash := range hashes {
		buffer = append(buffer, hash.value[:]...)
	}
	sum := md4Sum(buffer)
	hash, err := HashFromBytes(sum[:])
	if err != nil {
		return Invalid
	}
	return hash
}
