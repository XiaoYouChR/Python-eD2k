package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/protocol"
)

const partsInRequest = 3

type RequestParts32 struct {
	Hash        protocol.Hash
	CurrentFree int
	BeginOffset [partsInRequest]uint32
	EndOffset   [partsInRequest]uint32
}

func (r *RequestParts32) AppendRange(rg data.Range) {
	r.BeginOffset[r.CurrentFree] = uint32(rg.Left)
	r.EndOffset[r.CurrentFree] = uint32(rg.Right)
	r.CurrentFree++
}

func (r *RequestParts32) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	r.Hash = hash
	for i := 0; i < partsInRequest; i++ {
		v, err := protocol.ReadUInt32(src)
		if err != nil {
			return err
		}
		r.BeginOffset[i] = v
	}
	for i := 0; i < partsInRequest; i++ {
		v, err := protocol.ReadUInt32(src)
		if err != nil {
			return err
		}
		r.EndOffset[i] = v
	}
	r.CurrentFree = partsInRequest
	return nil
}

func (r RequestParts32) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, r.Hash); err != nil {
		return err
	}
	for i := 0; i < partsInRequest; i++ {
		if err := protocol.WriteUInt32(dst, r.BeginOffset[i]); err != nil {
			return err
		}
	}
	for i := 0; i < partsInRequest; i++ {
		if err := protocol.WriteUInt32(dst, r.EndOffset[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r RequestParts32) BytesCount() int {
	return 16 + partsInRequest*2*4
}

type RequestParts64 struct {
	Hash        protocol.Hash
	CurrentFree int
	BeginOffset [partsInRequest]uint64
	EndOffset   [partsInRequest]uint64
}

func (r *RequestParts64) AppendRange(rg data.Range) {
	r.BeginOffset[r.CurrentFree] = uint64(rg.Left)
	r.EndOffset[r.CurrentFree] = uint64(rg.Right)
	r.CurrentFree++
}

func (r *RequestParts64) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	r.Hash = hash
	for i := 0; i < partsInRequest; i++ {
		v, err := protocol.ReadUInt64(src)
		if err != nil {
			return err
		}
		r.BeginOffset[i] = v
	}
	for i := 0; i < partsInRequest; i++ {
		v, err := protocol.ReadUInt64(src)
		if err != nil {
			return err
		}
		r.EndOffset[i] = v
	}
	r.CurrentFree = partsInRequest
	return nil
}

func (r RequestParts64) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, r.Hash); err != nil {
		return err
	}
	for i := 0; i < partsInRequest; i++ {
		if err := protocol.WriteUInt64(dst, r.BeginOffset[i]); err != nil {
			return err
		}
	}
	for i := 0; i < partsInRequest; i++ {
		if err := protocol.WriteUInt64(dst, r.EndOffset[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r RequestParts64) BytesCount() int {
	return 16 + partsInRequest*2*8
}
