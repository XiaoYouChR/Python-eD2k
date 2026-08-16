package protocol

import (
	"encoding/json"

	"github.com/monkeyWie/goed2k/data"
)

type TransferResumeData struct {
	Hashes           []Hash
	Pieces           BitField
	DownloadedBlocks []data.PieceBlock
	Peers            []Endpoint
}

type transferResumeDataJSON struct {
	Hashes           []Hash            `json:"hashes,omitempty"`
	Pieces           []bool            `json:"pieces,omitempty"`
	DownloadedBlocks []data.PieceBlock `json:"downloaded_blocks,omitempty"`
	Peers            []Endpoint        `json:"peers,omitempty"`
}

func (r TransferResumeData) MarshalJSON() ([]byte, error) {
	return json.Marshal(transferResumeDataJSON{
		Hashes:           append([]Hash(nil), r.Hashes...),
		Pieces:           r.Pieces.Bits(),
		DownloadedBlocks: append([]data.PieceBlock(nil), r.DownloadedBlocks...),
		Peers:            append([]Endpoint(nil), r.Peers...),
	})
}

func (r *TransferResumeData) UnmarshalJSON(data []byte) error {
	var wire transferResumeDataJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	bits := NewBitField(len(wire.Pieces))
	for i, piece := range wire.Pieces {
		if piece {
			bits.SetBit(i)
		}
	}
	r.Hashes = append(r.Hashes[:0], wire.Hashes...)
	r.Pieces = bits
	r.DownloadedBlocks = append(r.DownloadedBlocks[:0], wire.DownloadedBlocks...)
	r.Peers = append(r.Peers[:0], wire.Peers...)
	return nil
}
