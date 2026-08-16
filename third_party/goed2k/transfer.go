package goed2k

import (
	"os"
	"path/filepath"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/protocol"
)

const InvalidETA int64 = -1

type Transfer struct {
	hash               protocol.Hash
	createTime         int64
	size               int64
	numPieces          int
	filePath           string
	stat               Statistics
	closedStat         Statistics
	picker             PiecePicker
	policy             Policy
	pm                 *PieceManager
	hashSet            []protocol.Hash
	session            *Session
	pause              bool
	abort              bool
	handler            disk.FileHandler
	needSaveResumeData bool
	state              TransferState
	peersInfo          []PeerInfo
	connections        []*PeerConnection
	nextSourcesRequest int64
	nextDHTRequest     int64
	speedMon           SpeedMonitor
	pendingResumeIO    int
	uploadPriority     UploadPriority
	pendingPieceHashes map[int]bool
}

func NewTransfer(s *Session, atp AddTransferParams) (*Transfer, error) {
	t := &Transfer{
		hash:               atp.Hash,
		createTime:         atp.CreateTime,
		size:               atp.Size,
		numPieces:          int(DivCeil(atp.Size, PieceSize)),
		filePath:           atp.FilePath,
		stat:               NewStatistics(),
		closedStat:         NewStatistics(),
		hashSet:            make([]protocol.Hash, 0),
		session:            s,
		pause:              atp.Paused,
		handler:            atp.Handler,
		state:              LoadingResumeData,
		connections:        make([]*PeerConnection, 0),
		speedMon:           NewSpeedMonitor(30),
		uploadPriority:     UploadPriorityNormal,
		pendingPieceHashes: make(map[int]bool),
	}
	blocksInLastPiece := int(DivCeil(atp.Size%PieceSize, BlockSize))
	if blocksInLastPiece == 0 {
		blocksInLastPiece = 1
	}
	t.picker = NewPiecePicker(t.numPieces, blocksInLastPiece)
	t.policy = NewPolicy(t)

	if t.handler == nil && atp.FilePath != "" {
		t.handler = disk.NewDesktopFileHandler(atp.FilePath)
	}
	if t.handler != nil {
		t.pm = NewPieceManager(t.handler, t.numPieces, blocksInLastPiece)
	}

	if atp.ResumeData != nil {
		t.restoreResumeData(atp.ResumeData)
	} else {
		t.state = Downloading
		t.needSaveResumeData = true
	}

	return t, nil
}

func (t *Transfer) GetHash() protocol.Hash {
	return t.hash
}

func (t *Transfer) GetCreateTime() int64 {
	return t.createTime
}

func (t *Transfer) Size() int64 {
	return t.size
}

func (t *Transfer) GetFilePath() string {
	if t.filePath != "" {
		return t.filePath
	}
	if t.handler != nil {
		return t.handler.Path()
	}
	return ""
}

func (t *Transfer) GetFile() *os.File {
	if t.pm == nil {
		return nil
	}
	return t.pm.GetFile()
}

func (t *Transfer) FileName() string {
	path := t.GetFilePath()
	if path == "" {
		return t.hash.String()
	}
	return filepath.Base(path)
}

func (t *Transfer) UploadPriority() UploadPriority {
	return t.uploadPriority
}

func (t *Transfer) SetUploadPriority(priority UploadPriority) {
	t.uploadPriority = priority
}

func (t *Transfer) Pause() {
	t.pause = true
}

func (t *Transfer) Resume() {
	t.pause = false
}

func (t *Transfer) IsPaused() bool {
	return t.pause
}

func (t *Transfer) IsAborted() bool {
	return t.abort
}

func (t *Transfer) WantMorePeers() bool {
	return !t.IsPaused() && !t.IsFinished() && t.policy.NumConnectCandidates() > 0
}

func (t *Transfer) AddStats(s Statistics) {
	t.closedStat.Add(s)
	t.refreshStats()
}

func (t *Transfer) AddPeer(endpoint protocol.Endpoint, sourceFlag int) error {
	if t.session != nil {
		t.session.mu.Lock()
		defer t.session.mu.Unlock()
	}
	peer := NewPeerWithSource(endpoint, true, sourceFlag)
	_, err := t.policy.AddPeer(peer)
	return err
}

func (t *Transfer) RemovePeerConnection(c *PeerConnection) {
	if t.session != nil {
		t.session.mu.Lock()
		defer t.session.mu.Unlock()
	}
	t.policy.ConnectionClosed(c, CurrentTime())
	c.SetPeer(nil)
	dst := t.connections[:0]
	for _, existing := range t.connections {
		if existing != c {
			dst = append(dst, existing)
		}
	}
	t.connections = dst
}

func (t *Transfer) ConnectToPeer(peerInfo *Peer) (*PeerConnection, error) {
	if peerInfo == nil {
		return nil, NewError(IllegalArgument)
	}
	if t.session != nil {
		t.session.mu.Lock()
	}
	peerInfo.LastConnected = CurrentTime()
	peerInfo.NextConnection = 0
	c := NewPeerConnection(t.session, peerInfo.Endpoint, t, peerInfo)
	t.session.connections = append(t.session.connections, c)
	t.connections = append(t.connections, c)
	t.policy.SetConnection(peerInfo, c)
	if t.session != nil {
		t.session.mu.Unlock()
	}
	if err := c.Connect(); err != nil {
		return nil, err
	}
	peerInfo.Connection = c
	return c, nil
}

func (t *Transfer) AttachPeer(c *PeerConnection) error {
	if c == nil {
		return NewError(IllegalArgument)
	}
	if t.IsPaused() {
		return NewError(TransferPaused)
	}
	if t.IsAborted() {
		return NewError(TransferAborted)
	}
	if t.IsFinished() {
		return NewError(TransferFinished)
	}
	if t.session != nil {
		t.session.mu.Lock()
		defer t.session.mu.Unlock()
	}
	if err := t.policy.NewConnection(c); err != nil {
		return err
	}
	t.connections = append(t.connections, c)
	t.session.connections = append(t.session.connections, c)
	c.SetTransfer(t)
	return nil
}

func (t *Transfer) AttachIncomingPeer(c *PeerConnection) error {
	if c == nil {
		return NewError(IllegalArgument)
	}
	if t.IsAborted() {
		return NewError(TransferAborted)
	}
	if t.session != nil {
		t.session.mu.Lock()
		defer t.session.mu.Unlock()
	}
	for _, existing := range t.connections {
		if existing == c {
			c.SetTransfer(t)
			return nil
		}
	}
	if err := t.policy.NewConnection(c); err != nil {
		return err
	}
	t.connections = append(t.connections, c)
	c.SetTransfer(t)
	return nil
}

func (t *Transfer) TryConnectPeer(sessionTime int64) (bool, error) {
	if !t.WantMorePeers() {
		return false, nil
	}
	return t.policy.ConnectOnePeer(sessionTime)
}

func (t *Transfer) IsFinished() bool {
	return t.numPieces == 0 || t.picker.NumHave() == t.picker.NumPieces()
}

func (t *Transfer) ResumeData() *protocol.TransferResumeData {
	trd := &protocol.TransferResumeData{}
	trd.Hashes = append(trd.Hashes, t.hashSet...)
	trd.Pieces = protocol.NewBitField(t.picker.NumPieces())
	for i := 0; i < t.numPieces; i++ {
		if t.picker.HavePiece(i) {
			trd.Pieces.SetBit(i)
		}
	}
	for _, dp := range t.picker.GetDownloadingQueue() {
		for j := 0; j < dp.BlocksCount(); j++ {
			if dp.IsFinished(j) {
				trd.DownloadedBlocks = append(trd.DownloadedBlocks, data.NewPieceBlock(dp.PieceIndex, j))
			}
		}
	}
	for _, peer := range t.policy.peers {
		if peer.Endpoint.Defined() {
			trd.Peers = append(trd.Peers, peer.Endpoint)
		}
	}
	t.needSaveResumeData = false
	return trd
}

func (t *Transfer) GetPeersInfo() []PeerInfo {
	t.peersInfo = t.peersInfo[:0]
	for _, c := range t.connections {
		if c == nil {
			continue
		}
		t.peersInfo = append(t.peersInfo, c.GetInfo())
	}
	out := make([]PeerInfo, len(t.peersInfo))
	copy(out, t.peersInfo)
	return out
}

func (t *Transfer) ActiveConnections() int {
	res := 0
	for _, c := range t.connections {
		if c != nil && !c.IsDisconnecting() {
			res++
		}
	}
	return res
}

func (t *Transfer) NeedMoreSources() bool {
	if t.IsPaused() || t.IsAborted() || t.IsFinished() {
		return false
	}
	return t.ActiveConnections() < t.session.settings.SessionConnectionsLimit
}

func (t *Transfer) refreshStats() {
	t.stat = NewStatistics()
	t.stat.Add(t.closedStat)
	for _, c := range t.connections {
		if c == nil || c.IsDisconnecting() {
			continue
		}
		t.stat.Merge(c.Statistics())
	}
}

func (t *Transfer) GetStatus() TransferStatus {
	totalDone := int64(0)
	for pieceIndex := 0; pieceIndex < t.picker.NumPieces(); pieceIndex++ {
		if t.picker.HavePiece(pieceIndex) {
			totalDone += t.pieceSize(pieceIndex)
		}
	}
	for _, dp := range t.picker.GetDownloadingQueue() {
		if t.picker.HavePiece(dp.PieceIndex) {
			continue
		}
		for blockIndex := 0; blockIndex < dp.BlocksCount(); blockIndex++ {
			if !dp.IsDownloaded(blockIndex) {
				continue
			}
			totalDone += int64(data.NewPieceBlock(dp.PieceIndex, blockIndex).Size(t.size))
		}
	}
	if totalDone > t.size {
		totalDone = t.size
	}
	totalReceived := t.receivedBytes(totalDone)
	state := t.state
	if t.pause {
		state = PausedState
	} else if state != Finished && totalDone >= t.size && t.size > 0 {
		state = Verifying
	}

	status := TransferStatus{
		Paused:            t.pause,
		DownloadRate:      int(t.stat.DownloadRate()),
		Upload:            t.stat.TotalUpload(),
		UploadRate:        int(t.stat.UploadRate()),
		NumPeers:          t.policy.Size(),
		DownloadingPieces: t.picker.NumDownloadingPieces(),
		TotalDone:         totalDone,
		TotalReceived:     totalReceived,
		TotalWanted:       t.size,
		ETA:               InvalidETA,
		Pieces:            protocol.NewBitField(t.picker.NumPieces()),
		NumPieces:         t.picker.NumHave(),
		State:             state,
	}
	for i := 0; i < t.picker.NumPieces(); i++ {
		if t.picker.HavePiece(i) {
			status.Pieces.SetBit(i)
		}
	}
	averageSpeed := t.speedMon.AverageSpeed()
	if averageSpeed != InvalidSpeed {
		if averageSpeed == 0 {
			status.ETA = InvalidETA
		} else {
			status.ETA = (status.TotalWanted - status.TotalDone) / averageSpeed
		}
	}
	return status
}

func (t *Transfer) pieceSize(pieceIndex int) int64 {
	if pieceIndex < 0 || pieceIndex >= t.numPieces {
		return 0
	}
	begin := int64(pieceIndex) * PieceSize
	if begin >= t.size {
		return 0
	}
	end := begin + PieceSize
	if end > t.size {
		end = t.size
	}
	return end - begin
}

func (t *Transfer) receivedBytes(totalDone int64) int64 {
	totalReceived := totalDone
	inflight := t.inflightBlockReceived()
	for _, received := range inflight {
		totalReceived += received
	}
	if totalReceived > t.size {
		totalReceived = t.size
	}
	return totalReceived
}

func (t *Transfer) PieceSnapshots() []PieceSnapshot {
	inflight := t.inflightBlockReceived()
	pieces := make([]PieceSnapshot, 0, t.picker.NumPieces())
	for pieceIndex := 0; pieceIndex < t.picker.NumPieces(); pieceIndex++ {
		totalBytes := t.pieceSize(pieceIndex)
		blocksTotal := t.picker.BlocksInPiece(pieceIndex)
		snapshot := PieceSnapshot{
			Index:       pieceIndex,
			State:       PieceSnapshotMissing,
			TotalBytes:  totalBytes,
			BlocksTotal: blocksTotal,
		}
		if t.picker.HavePiece(pieceIndex) {
			snapshot.State = PieceSnapshotFinished
			snapshot.DoneBytes = totalBytes
			snapshot.ReceivedBytes = totalBytes
			snapshot.BlocksDone = blocksTotal
			pieces = append(pieces, snapshot)
			continue
		}
		if dp := t.picker.GetDownloadingPiece(pieceIndex); dp != nil {
			snapshot.State = PieceSnapshotDownloading
			for blockIndex := 0; blockIndex < dp.BlocksCount(); blockIndex++ {
				block := data.NewPieceBlock(pieceIndex, blockIndex)
				blockSize := int64(block.Size(t.size))
				switch {
				case dp.Blocks[blockIndex].IsFinished():
					snapshot.BlocksDone++
					snapshot.DoneBytes += blockSize
					snapshot.ReceivedBytes += blockSize
				case dp.Blocks[blockIndex].IsWriting():
					snapshot.BlocksWriting++
					snapshot.DoneBytes += blockSize
					snapshot.ReceivedBytes += blockSize
				case dp.Blocks[blockIndex].IsRequested():
					snapshot.BlocksPending++
					snapshot.ReceivedBytes += inflight[block.BlocksOffset()]
				}
			}
		}
		if snapshot.ReceivedBytes > snapshot.TotalBytes {
			snapshot.ReceivedBytes = snapshot.TotalBytes
		}
		if snapshot.DoneBytes > snapshot.TotalBytes {
			snapshot.DoneBytes = snapshot.TotalBytes
		}
		pieces = append(pieces, snapshot)
	}
	return pieces
}

func (t *Transfer) inflightBlockReceived() map[int64]int64 {
	inflight := make(map[int64]int64)
	for _, c := range t.connections {
		if c == nil || c.IsDisconnecting() {
			continue
		}
		for _, pb := range c.downloadQueue {
			if pb.Received <= 0 || t.picker.IsBlockDownloaded(pb.Block) {
				continue
			}
			size := int64(pb.Block.Size(t.size))
			received := pb.Received
			if received > size {
				received = size
			}
			key := pb.Block.BlocksOffset()
			if inflight[key] < received {
				inflight[key] = received
			}
		}
	}
	return inflight
}

func (t *Transfer) Abort(deleteFile bool) error {
	t.abort = true
	for _, c := range t.connections {
		if c != nil {
			c.Close(TransferAborted)
		}
	}
	t.connections = nil
	if t.session != nil {
		t.session.SubmitDiskTask(NewAsyncRelease(t, deleteFile))
	}
	return nil
}

func (t *Transfer) PauseWithDisconnect() {
	t.pause = true
	for _, c := range t.connections {
		if c != nil {
			c.Close(TransferPaused)
		}
	}
	t.connections = nil
	t.needSaveResumeData = true
}

func (t *Transfer) ResumeWithState() {
	t.pause = false
	for i := range t.policy.peers {
		if t.policy.peers[i].Connection == nil {
			t.policy.peers[i].NextConnection = 0
			t.policy.peers[i].LastConnected = 0
		}
	}
	t.nextSourcesRequest = 0
	t.nextDHTRequest = 0
	t.needSaveResumeData = true
}

func (t *Transfer) ForceSourceDiscoveryNow() {
	t.nextSourcesRequest = 0
	t.nextDHTRequest = 0
}

func (t *Transfer) nextServerSourcesInterval(activeConnections, connectCandidates int, sent bool) int64 {
	if sent {
		nextInterval := Minutes(1)
		if activeConnections == 0 && connectCandidates == 0 {
			nextInterval = Seconds(5)
		} else if activeConnections <= 1 || connectCandidates <= 1 {
			nextInterval = Seconds(10)
		}
		return nextInterval
	}

	// No handshake-complete server is available or the request could not be queued.
	if activeConnections == 0 && connectCandidates == 0 {
		return Seconds(5)
	}
	return Seconds(10)
}

func (t *Transfer) nextDHTSourcesInterval(activeConnections, connectCandidates int, sent bool) int64 {
	if sent {
		nextInterval := Minutes(2)
		if activeConnections == 0 && connectCandidates == 0 {
			nextInterval = Seconds(30)
		} else if activeConnections <= 1 || connectCandidates <= 1 {
			nextInterval = Minutes(1)
		}
		return nextInterval
	}

	// DHT is unavailable or could not start; retry, but never every tick.
	if activeConnections == 0 && connectCandidates == 0 {
		return Seconds(30)
	}
	return Minutes(1)
}

func (t *Transfer) QueuePieceHash(pieceIndex int) bool {
	if pieceIndex < 0 || pieceIndex >= t.picker.NumPieces() {
		return false
	}
	if t.picker.HavePiece(pieceIndex) || t.pendingPieceHashes[pieceIndex] {
		return false
	}
	if !t.picker.IsPieceFinished(pieceIndex) {
		return false
	}
	if t.session == nil || t.pm == nil {
		return false
	}
	t.pendingPieceHashes[pieceIndex] = true
	t.session.SubmitDiskTask(NewAsyncHash(t, pieceIndex))
	return true
}

func (t *Transfer) WeHave(pieceIndex int) {
	t.picker.WeHave(pieceIndex)
}

func (t *Transfer) GetPieceManager() *PieceManager {
	return t.pm
}

func (t *Transfer) AvailablePieces() protocol.BitField {
	bits := protocol.NewBitField(t.picker.NumPieces())
	for i := 0; i < t.picker.NumPieces(); i++ {
		if t.picker.HavePiece(i) {
			bits.SetBit(i)
		}
	}
	return bits
}

func (t *Transfer) UploadHashSet() []protocol.Hash {
	if len(t.hashSet) > 0 {
		out := make([]protocol.Hash, len(t.hashSet))
		copy(out, t.hashSet)
		return out
	}
	if t.size <= PieceSize {
		return []protocol.Hash{t.hash}
	}
	return nil
}

func (t *Transfer) CanUpload() bool {
	return t != nil && !t.abort && t.pm != nil && t.picker.NumHave() > 0
}

func (t *Transfer) CanUploadRange(begin, end int64) bool {
	if !t.CanUpload() || end <= begin || begin < 0 || end > t.size {
		return false
	}
	reqs, err := data.MakePeerRequests(begin, end, t.size)
	if err != nil {
		return false
	}
	for _, req := range reqs {
		if !t.picker.HavePiece(req.Piece) {
			return false
		}
	}
	return true
}

func (t *Transfer) ReadRange(begin, end int64) ([]byte, error) {
	if t.pm == nil {
		return nil, NewError(NoTransfer)
	}
	return t.pm.ReadRange(begin, end)
}

func (t *Transfer) SecondTick(accumulator *Statistics, tickIntervalMS int64) {
	if t.NeedMoreSources() {
		now := CurrentTime()
		activeConnections := t.ActiveConnections()
		connectCandidates := t.policy.NumConnectCandidates()
		if activeConnections <= 1 && connectCandidates <= 1 && t.nextSourcesRequest > now+Seconds(5) {
			t.nextSourcesRequest = now
		}
		if activeConnections <= 1 && connectCandidates <= 1 && t.nextDHTRequest > now+Seconds(10) {
			t.nextDHTRequest = now
		}
		if t.nextSourcesRequest <= now {
			sent := t.session.SendSourcesRequest(t.hash, t.size)
			t.nextSourcesRequest = now + t.nextServerSourcesInterval(activeConnections, connectCandidates, sent)
		}
		if t.nextDHTRequest < now {
			sent := t.session.SendDHTSourcesRequest(t.hash, t.size, t)
			t.nextDHTRequest = now + t.nextDHTSourcesInterval(activeConnections, connectCandidates, sent)
		}
	}

	for _, c := range t.connections {
		if c == nil {
			continue
		}
		c.SecondTick(tickIntervalMS)
	}
	t.refreshStats()
	if accumulator != nil {
		accumulator.Add(t.stat)
	}
	t.speedMon.AddSample(t.stat.DownloadRate())
}

func (t *Transfer) OnBlockWriteCompleted(block data.PieceBlock, _ [][]byte, ec BaseErrorCode) {
	if ec.Code() == NoError.Code() {
		t.picker.MarkAsFinished(block)
		t.QueuePieceHash(block.PieceIndex)
		t.needSaveResumeData = true
		return
	}
	t.picker.AbortDownload(block, nil)
	t.PauseWithDisconnect()
}

func (t *Transfer) OnPieceHashCompleted(pieceIndex int, hash protocol.Hash) {
	delete(t.pendingPieceHashes, pieceIndex)
	if pieceIndex < len(t.hashSet) && !t.hashSet[pieceIndex].Equal(hash) {
		debugPeerf("transfer %s piece %d hash mismatch got=%s want=%s", t.hash.String(), pieceIndex, hash.String(), t.hashSet[pieceIndex].String())
		t.picker.RestorePiece(pieceIndex)
	} else {
		debugPeerf("transfer %s piece %d hash ok=%s", t.hash.String(), pieceIndex, hash.String())
		t.WeHave(pieceIndex)
	}
	t.needSaveResumeData = true
	if t.IsFinished() {
		t.finished()
	}
}

func (t *Transfer) OnBlockRestoreCompleted(block data.PieceBlock, ec BaseErrorCode) {
	if t.pendingResumeIO > 0 {
		t.pendingResumeIO--
	}
	if ec.Code() != NoError.Code() {
		t.PauseWithDisconnect()
		return
	}
	t.picker.WeHaveBlock(block)
	t.needSaveResumeData = true
	if t.pendingResumeIO == 0 {
		if t.IsFinished() {
			t.state = Finished
		} else {
			t.state = Downloading
		}
	}
}

func (t *Transfer) OnReleaseFile(_ BaseErrorCode, _ [][]byte, _ bool) {}

func (t *Transfer) finished() {
	for _, c := range t.connections {
		if c != nil {
			c.Close(TransferFinished)
		}
	}
	t.connections = nil
	t.state = Finished
	if t.session != nil {
		t.session.SubmitDiskTask(NewAsyncRelease(t, false))
		t.session.PublishTransferToServer(t)
	}
	t.needSaveResumeData = true
}

func (t *Transfer) AsyncRestoreBlock(block data.PieceBlock) {
	if t.session != nil {
		t.session.SubmitDiskTask(NewAsyncRestore(t, block, t.size))
	}
}

func (t *Transfer) SetHashSet(hash protocol.Hash, hashes []protocol.Hash) {
	if len(t.hashSet) != 0 {
		return
	}
	if !t.hash.Equal(hash) {
		return
	}
	t.hashSet = append(t.hashSet, hashes...)
	t.needSaveResumeData = true
}

func (t *Transfer) NeedResumeDataSave() bool {
	return t.needSaveResumeData
}

func (t *Transfer) restoreResumeData(resumeData *protocol.TransferResumeData) {
	t.state = LoadingResumeData
	t.hashSet = append(t.hashSet, resumeData.Hashes...)
	for i := 0; i < resumeData.Pieces.Len(); i++ {
		if resumeData.Pieces.GetBit(i) {
			t.picker.RestoreHave(i)
		}
	}
	for _, endpoint := range resumeData.Peers {
		if endpoint.Defined() {
			_ = t.AddPeer(endpoint, int(PeerResume))
		}
	}
	if len(resumeData.DownloadedBlocks) == 0 {
		if t.IsFinished() {
			t.state = Finished
		} else {
			t.state = Downloading
		}
		return
	}
	for _, block := range resumeData.DownloadedBlocks {
		t.pendingResumeIO++
		if t.pm != nil && t.session != nil {
			t.AsyncRestoreBlock(block)
			continue
		}
		t.picker.WeHaveBlock(block)
		t.pendingResumeIO--
	}
	if t.pendingResumeIO == 0 {
		t.state = Downloading
		if t.IsFinished() {
			t.state = Finished
		}
	}
}
