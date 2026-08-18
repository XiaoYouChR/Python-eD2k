package client

import (
	"bytes"

	"github.com/monkeyWie/goed2k/protocol"
)

// QueueRank is the legacy eD2k queue-rank response. Modern eMule clients use
// QueueRanking, but eDonkey-compatible peers may still send this 32-bit form.
type QueueRank struct {
	Rank uint32
}

func (q *QueueRank) Get(src *bytes.Reader) error {
	value, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	q.Rank = value
	return nil
}

func (q QueueRank) Put(dst *bytes.Buffer) error {
	return protocol.WriteUInt32(dst, q.Rank)
}

func (q QueueRank) BytesCount() int { return 4 }
