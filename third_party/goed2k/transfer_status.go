package goed2k

import (
	"fmt"

	"github.com/monkeyWie/goed2k/protocol"
)

type TransferState string

const (
	LoadingResumeData TransferState = "LOADING_RESUME_DATA"
	Downloading       TransferState = "DOWNLOADING"
	PausedState       TransferState = "PAUSED"
	Verifying         TransferState = "VERIFYING"
	Finished          TransferState = "FINISHED"
)

type TransferStatus struct {
	Paused            bool
	DownloadRate      int
	Upload            int64
	UploadRate        int
	NumPeers          int
	DownloadingPieces int
	TotalDone         int64
	TotalReceived     int64
	TotalWanted       int64
	ETA               int64
	Pieces            protocol.BitField
	NumPieces         int
	State             TransferState
}

func (s TransferStatus) String() string {
	return fmt.Sprintf("TransferStatus{paused=%t, downloadRate=%d, upload=%d, uploadRate=%d, numPeers=%d, downloadingPieces=%d, totalDone=%d, totalReceived=%d, totalWanted=%d, eta=%d, pieces=%v, numPieces=%d}",
		s.Paused, s.DownloadRate, s.Upload, s.UploadRate, s.NumPeers, s.DownloadingPieces, s.TotalDone, s.TotalReceived, s.TotalWanted, s.ETA, s.Pieces.Bits(), s.NumPieces)
}
