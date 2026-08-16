package data

import "errors"

const (
	prPieceSize int64 = 9728000
	prBlockSize int64 = 190 * 1024
)

var (
	errInvalidPRParameter  = errors.New("invalid peer request parameter")
	errPeerRequestOverflow = errors.New("peer request overflow")
)

type PeerRequest struct {
	Piece  int
	Start  int64
	Length int64
}

func MakePeerRequest(begin, end int64) (PeerRequest, error) {
	if end <= begin || begin < 0 {
		return PeerRequest{}, errInvalidPRParameter
	}
	pr := PeerRequest{
		Piece:  int(begin / prPieceSize),
		Start:  begin % prPieceSize,
		Length: end - begin,
	}
	if pr.Length > prPieceSize {
		return PeerRequest{}, errPeerRequestOverflow
	}
	return pr, nil
}

func MakePeerRequests(begin, end, fsize int64) ([]PeerRequest, error) {
	if begin > fsize {
		begin = fsize
	}
	if end > fsize {
		end = fsize
	}
	reqs := make([]PeerRequest, 0)
	for i := begin; i < end; {
		next := i + prPieceSize - i%prPieceSize
		if next > end {
			next = end
		}
		pr, err := MakePeerRequest(i, next)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, pr)
		i += pr.Length
	}
	return reqs, nil
}

func MakePeerRequestFromBlock(block PieceBlock, fsize int64) (PeerRequest, error) {
	r := block.Range(fsize)
	return MakePeerRequest(r.Left, r.Right)
}

func (p PeerRequest) InBlockOffset() int64 {
	return p.Start % prBlockSize
}

func (p PeerRequest) Range() Range {
	begin := int64(p.Piece)*prPieceSize + p.Start
	end := begin + p.Length
	return MakeRange(begin, end)
}
