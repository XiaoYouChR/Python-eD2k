package goed2k

type PieceSnapshotState string

const (
	PieceSnapshotMissing     PieceSnapshotState = "MISSING"
	PieceSnapshotDownloading PieceSnapshotState = "DOWNLOADING"
	PieceSnapshotFinished    PieceSnapshotState = "FINISHED"
)

type PieceSnapshot struct {
	Index         int
	State         PieceSnapshotState
	TotalBytes    int64
	DoneBytes     int64
	ReceivedBytes int64
	BlocksTotal   int
	BlocksDone    int
	BlocksWriting int
	BlocksPending int
}
