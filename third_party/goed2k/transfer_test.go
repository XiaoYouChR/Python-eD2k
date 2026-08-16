package goed2k

import (
	"bytes"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/protocol"
	serverproto "github.com/monkeyWie/goed2k/protocol/server"
)

type testDiskCallable struct {
	transfer *Transfer
	call     func()
}

func (t testDiskCallable) Transfer() *Transfer {
	return t.transfer
}

func (t testDiskCallable) Call() AsyncOperationResult {
	if t.call != nil {
		t.call()
	}
	return testDiskResult{onCompleted: nil}
}

type testDiskResult struct {
	onCompleted func()
}

func (t testDiskResult) OnCompleted() {
	if t.onCompleted != nil {
		t.onCompleted()
	}
}

func (t testDiskResult) Code() BaseErrorCode {
	return NoError
}

type testDiskCallableWithCompletion struct {
	transfer    *Transfer
	onCompleted func()
}

func (t testDiskCallableWithCompletion) Transfer() *Transfer {
	return t.transfer
}

func (t testDiskCallableWithCompletion) Call() AsyncOperationResult {
	return testDiskResult{onCompleted: t.onCompleted}
}

func newTestTransfer(t *testing.T) (*Session, *Transfer) {
	t.Helper()
	UpdateCachedTime()
	session := NewSession(NewSettings())
	transfer, err := NewTransfer(session, AddTransferParams{
		Hash:       protocol.EMule,
		CreateTime: CurrentTimeMillis(),
		Size:       PieceSize * 2,
	})
	if err != nil {
		t.Fatalf("new transfer: %v", err)
	}
	session.transfers[transfer.hash] = transfer
	return session, transfer
}

func TestTransferStatsDropRateAfterPeerDisconnect(t *testing.T) {
	session, transfer := newTestTransfer(t)
	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := transfer.AddPeer(endpoint, int(PeerServer)); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	peer := transfer.policy.FindPeer(endpoint)
	if peer == nil {
		t.Fatal("peer not found in policy")
	}

	conn := NewPeerConnection(session, endpoint, transfer, peer)
	transfer.connections = append(transfer.connections, conn)
	session.connections = append(session.connections, conn)
	transfer.policy.SetConnection(peer, conn)

	conn.stat.ReceiveBytes(100, 900)
	transfer.SecondTick(nil, 1000)
	before := transfer.GetStatus()
	if before.DownloadRate <= 0 {
		t.Fatalf("expected positive download rate before disconnect, got %d", before.DownloadRate)
	}

	conn.Close(ConnectionTimeout)
	conn.OnDisconnect(ConnectionTimeout)
	transfer.SecondTick(nil, 1000)
	after := transfer.GetStatus()

	if after.DownloadRate != 0 {
		t.Fatalf("expected rate to drop to zero after disconnect, got %d", after.DownloadRate)
	}
	if transfer.ActiveConnections() != 0 {
		t.Fatalf("expected no active connections after disconnect, got %d", transfer.ActiveConnections())
	}
	if transfer.policy.NumConnectCandidates() != 1 {
		t.Fatalf("expected peer to be reconnectable after disconnect, got %d candidates", transfer.policy.NumConnectCandidates())
	}
}

func TestTransferRequestsSourcesSoonWhenNoActivePeersRemain(t *testing.T) {
	session, transfer := newTestTransfer(t)
	serverAddr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	session.serverConnection = NewServerConnection("test", serverAddr, session)
	session.serverConnection.handshakeCompleted = true

	now := CurrentTime()
	transfer.nextSourcesRequest = now + Minutes(1)
	transfer.nextDHTRequest = now + Minutes(10)
	transfer.SecondTick(nil, 1000)

	if got := len(session.serverConnection.PendingPackets()); got == 0 {
		t.Fatal("expected queued GetFileSources packet when no active peers remain")
	}
	if transfer.nextSourcesRequest-now > Seconds(5) {
		t.Fatalf("expected next source retry within 5s, got %dms", transfer.nextSourcesRequest-now)
	}
	if transfer.nextDHTRequest-now > Seconds(10) {
		t.Fatalf("expected next DHT retry within 10s, got %dms", transfer.nextDHTRequest-now)
	}
}

func TestSessionPumpIOHandlesServerDisconnectCleanup(t *testing.T) {
	UpdateCachedTime()
	settings := NewSettings()
	settings.ReconnectToServer = true
	session := NewSession(settings)
	serverAddr := &net.TCPAddr{IP: net.IPv4(176, 123, 5, 89), Port: 4725}
	session.serverConnection = NewServerConnection("test", serverAddr, session)
	session.serverConnection.Close(EndOfStream)

	session.PumpIO()

	if session.serverConnection != nil {
		t.Fatal("expected server connection to be cleared after disconnect handling")
	}
}

func TestSessionSendSourcesRequestBroadcastsToAllServers(t *testing.T) {
	session, transfer := newTestTransfer(t)
	addrA := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	addrB := &net.TCPAddr{IP: net.IPv4(176, 123, 5, 89), Port: 4725}

	serverA := NewServerConnection("a", addrA, session)
	serverA.handshakeCompleted = true
	serverB := NewServerConnection("b", addrB, session)
	serverB.handshakeCompleted = true

	session.serverConnection = serverA
	session.serverConnections["a"] = serverA
	session.serverConnections["b"] = serverB

	if !session.SendSourcesRequest(transfer.hash, transfer.size) {
		t.Fatal("expected sources request broadcast to succeed")
	}
	if got := len(serverA.PendingPackets()); got == 0 {
		t.Fatal("expected queued GetFileSources packet on server A")
	}
	if got := len(serverB.PendingPackets()); got == 0 {
		t.Fatal("expected queued GetFileSources packet on server B")
	}
}

func TestSessionServerDisconnectKeepsOtherServersActive(t *testing.T) {
	UpdateCachedTime()
	session := NewSession(NewSettings())
	addrA := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	addrB := &net.TCPAddr{IP: net.IPv4(176, 123, 5, 89), Port: 4725}

	serverA := NewServerConnection("a", addrA, session)
	serverA.handshakeCompleted = true
	serverA.clientID = 1
	serverB := NewServerConnection("b", addrB, session)
	serverB.handshakeCompleted = true
	serverB.clientID = 2

	session.serverConnection = serverA
	session.serverConnections["a"] = serverA
	session.serverConnections["b"] = serverB
	session.clientID = serverA.clientID

	serverA.Close(EndOfStream)
	session.PumpIO()

	if session.serverConnection == nil {
		t.Fatal("expected another active server to become primary")
	}
	if session.serverConnection.GetIdentifier() != "b" {
		t.Fatalf("expected server b to become primary, got %s", session.serverConnection.GetIdentifier())
	}
	if session.clientID != serverB.clientID {
		t.Fatalf("expected clientID to switch to server b identity, got %d", session.clientID)
	}
}

func TestSessionOnServerIDChangeRequestsSourcesForExistingTransfers(t *testing.T) {
	session, transfer := newTestTransfer(t)
	addr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	server := NewServerConnection("a", addr, session)
	server.handshakeCompleted = true
	session.serverConnections["a"] = server

	session.OnServerIDChange(server, 1234, 0, 0)

	if got := len(server.PendingPackets()); got == 0 {
		t.Fatal("expected source request to be queued after server handshake")
	}
	now := CurrentTime()
	if transfer.nextSourcesRequest <= now {
		t.Fatalf("expected transfer nextSourcesRequest to be scheduled after handshake, got %d", transfer.nextSourcesRequest)
	}
	if transfer.nextDHTRequest <= now {
		t.Fatalf("expected transfer nextDHTRequest to be backed off after handshake, got %d", transfer.nextDHTRequest)
	}
}

func TestSessionOnServerIDChangePublishesFinishedTransfers(t *testing.T) {
	session, transfer := newTestTransfer(t)
	transfer.state = Finished
	addr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	server := NewServerConnection("a", addr, session)
	server.handshakeCompleted = true
	session.serverConnections["a"] = server

	session.OnServerIDChange(server, 1234, 0, 0)

	packets := server.PendingPackets()
	if len(packets) == 0 {
		t.Fatal("expected packets after finished transfer publish")
	}
	combiner := serverproto.NewPacketCombiner()
	found := false
	for _, raw := range packets {
		_, packet, err := combiner.UnpackFrame(raw)
		if err != nil {
			t.Fatalf("unpack frame: %v", err)
		}
		offer, ok := packet.(*serverproto.OfferFiles)
		if !ok {
			continue
		}
		found = true
		if len(offer.Entries) != 1 {
			t.Fatalf("expected one offered file, got %d", len(offer.Entries))
		}
		if !offer.Entries[0].Hash.Equal(transfer.GetHash()) {
			t.Fatalf("unexpected offered hash: %s", offer.Entries[0].Hash.String())
		}
		if got, ok := offer.Entries[0].StringTag(protocol.FTFilename); !ok || got != transfer.FileName() {
			t.Fatalf("unexpected offered filename: %q %t", got, ok)
		}
	}
	if !found {
		t.Fatal("expected OfferFiles packet after handshake")
	}
}

func TestTransferFinishedPublishesToConnectedServer(t *testing.T) {
	session, transfer := newTestTransfer(t)
	addr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	server := NewServerConnection("a", addr, session)
	server.handshakeCompleted = true
	session.serverConnection = server
	session.serverConnections["a"] = server
	session.clientID = 1234

	transfer.finished()

	packets := server.PendingPackets()
	if len(packets) == 0 {
		t.Fatal("expected offered file packet after transfer finished")
	}
	combiner := serverproto.NewPacketCombiner()
	found := false
	for _, raw := range packets {
		_, packet, err := combiner.UnpackFrame(raw)
		if err != nil {
			t.Fatalf("unpack frame: %v", err)
		}
		if _, ok := packet.(*serverproto.OfferFiles); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected OfferFiles packet after transfer finished")
	}
}

func TestRequestSourcesNowBacksOffWhenDHTUnavailable(t *testing.T) {
	session, transfer := newTestTransfer(t)
	addr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	server := NewServerConnection("a", addr, session)
	server.handshakeCompleted = true
	session.serverConnection = server
	session.serverConnections["a"] = server

	now := CurrentTime()
	if !session.RequestSourcesNow(transfer) {
		t.Fatal("expected server source request to succeed")
	}

	if transfer.nextSourcesRequest <= now {
		t.Fatalf("expected next server source request to be scheduled, got %d", transfer.nextSourcesRequest)
	}
	if transfer.nextDHTRequest-now < Seconds(30) {
		t.Fatalf("expected dht retry to back off after failure, got %dms", transfer.nextDHTRequest-now)
	}
}

func TestPiecePickerRespectsPeerAvailability(t *testing.T) {
	session, transfer := newTestTransfer(t)
	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := transfer.AddPeer(endpoint, int(PeerServer)); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	peer := transfer.policy.FindPeer(endpoint)
	if peer == nil {
		t.Fatal("expected peer in policy")
	}

	conn := NewPeerConnection(session, endpoint, transfer, peer)
	conn.remotePieces = protocol.NewBitField(transfer.picker.NumPieces())
	conn.remotePieces.SetBit(1)

	blocks := make([]data.PieceBlock, 0)
	transfer.picker.PickPiecesWithAvailability(&blocks, RequestQueueSize, conn.GetPeer(), conn.Speed(), &conn.remotePieces)

	if len(blocks) == 0 {
		t.Fatal("expected at least one requested block")
	}
	for _, block := range blocks {
		if block.PieceIndex != 1 {
			t.Fatalf("expected only piece 1 to be selected, got piece %d", block.PieceIndex)
		}
	}
}

func TestPeerDisconnectReturnsRequestedBlocksToPicker(t *testing.T) {
	session, transfer := newTestTransfer(t)

	endpointA, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint A: %v", err)
	}
	endpointB, err := protocol.EndpointFromString("2.3.4.5", 4662)
	if err != nil {
		t.Fatalf("endpoint B: %v", err)
	}
	if err := transfer.AddPeer(endpointA, int(PeerServer)); err != nil {
		t.Fatalf("add peer A: %v", err)
	}
	if err := transfer.AddPeer(endpointB, int(PeerServer)); err != nil {
		t.Fatalf("add peer B: %v", err)
	}

	peerA := transfer.policy.FindPeer(endpointA)
	peerB := transfer.policy.FindPeer(endpointB)
	if peerA == nil || peerB == nil {
		t.Fatal("expected peers in policy")
	}

	connA := NewPeerConnection(session, endpointA, transfer, peerA)
	connA.remotePieces = protocol.NewBitField(transfer.picker.NumPieces())
	connA.remotePieces.SetBit(0)
	transfer.connections = append(transfer.connections, connA)
	session.connections = append(session.connections, connA)
	transfer.policy.SetConnection(peerA, connA)

	connA.RequestBlocks()
	if len(connA.downloadQueue) != RequestQueueSize {
		t.Fatalf("expected %d requested blocks, got %d", RequestQueueSize, len(connA.downloadQueue))
	}

	connA.Close(ConnectionTimeout)
	connA.OnDisconnect(ConnectionTimeout)

	connB := NewPeerConnection(session, endpointB, transfer, peerB)
	connB.remotePieces = protocol.NewBitField(transfer.picker.NumPieces())
	connB.remotePieces.SetBit(0)
	transfer.connections = append(transfer.connections, connB)
	session.connections = append(session.connections, connB)
	transfer.policy.SetConnection(peerB, connB)

	connB.RequestBlocks()
	if len(connB.downloadQueue) != RequestQueueSize {
		t.Fatalf("expected disconnected peer requests to return to picker, got %d blocks", len(connB.downloadQueue))
	}
}

func TestPeerConnectionClosesOnStalledPendingRequest(t *testing.T) {
	session, transfer := newTestTransfer(t)

	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := transfer.AddPeer(endpoint, int(PeerServer)); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	peer := transfer.policy.FindPeer(endpoint)
	if peer == nil {
		t.Fatal("expected peer in policy")
	}

	conn := NewPeerConnection(session, endpoint, transfer, peer)
	conn.downloadQueue = append(conn.downloadQueue, PendingBlock{
		Block:      data.NewPieceBlock(0, 0),
		DataSize:   BlockSize,
		CreateTime: CurrentTime() - Seconds(20),
	})
	conn.lastReceive = CurrentTime() - Seconds(6)

	conn.SecondTick(1000)

	if !conn.IsDisconnecting() {
		t.Fatal("expected stalled pending request to close connection")
	}
}

func TestTransferStatusDoesNotExceedFileSizeForPartialLastBlock(t *testing.T) {
	session := NewSession(NewSettings())
	size := int64(45)*BlockSize + 32118
	transfer, err := NewTransfer(session, AddTransferParams{
		Hash:       protocol.EMule,
		CreateTime: CurrentTimeMillis(),
		Size:       size,
	})
	if err != nil {
		t.Fatalf("new transfer: %v", err)
	}

	for blockIndex := 0; blockIndex < transfer.picker.BlocksInPiece(0); blockIndex++ {
		transfer.picker.WeHaveBlock(data.NewPieceBlock(0, blockIndex))
	}

	status := transfer.GetStatus()
	if status.TotalDone != size {
		t.Fatalf("expected total done %d, got %d", size, status.TotalDone)
	}
	if status.TotalWanted != size {
		t.Fatalf("expected total wanted %d, got %d", size, status.TotalWanted)
	}
}

func TestTransferStatusReportsPartialReceivedBytes(t *testing.T) {
	session, transfer := newTestTransfer(t)

	endpointA, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint A: %v", err)
	}
	endpointB, err := protocol.EndpointFromString("2.3.4.5", 4662)
	if err != nil {
		t.Fatalf("endpoint B: %v", err)
	}
	peerA := NewPeerWithSource(endpointA, true, int(PeerServer))
	peerB := NewPeerWithSource(endpointB, true, int(PeerServer))

	connA := NewPeerConnection(session, endpointA, transfer, &peerA)
	connB := NewPeerConnection(session, endpointB, transfer, &peerB)
	connA.downloadQueue = append(connA.downloadQueue, PendingBlock{
		Block:      data.NewPieceBlock(0, 0),
		DataSize:   BlockSize,
		CreateTime: CurrentTime(),
		Received:   1024,
	})
	connB.downloadQueue = append(connB.downloadQueue, PendingBlock{
		Block:      data.NewPieceBlock(0, 0),
		DataSize:   BlockSize,
		CreateTime: CurrentTime(),
		Received:   4096,
	})
	transfer.connections = append(transfer.connections, connA, connB)

	status := transfer.GetStatus()
	if status.TotalDone != 0 {
		t.Fatalf("expected committed bytes 0, got %d", status.TotalDone)
	}
	if status.TotalReceived != 4096 {
		t.Fatalf("expected realtime received bytes 4096, got %d", status.TotalReceived)
	}
	if status.TotalReceived <= status.TotalDone {
		t.Fatalf("expected realtime received bytes to exceed committed bytes, got %d <= %d", status.TotalReceived, status.TotalDone)
	}
}

func TestSessionProcessDiskTasksAllowsCompletionToQueueFollowup(t *testing.T) {
	session, transfer := newTestTransfer(t)
	var completed atomic.Int32
	session.SubmitDiskTask(testDiskCallableWithCompletion{
		transfer: transfer,
		onCompleted: func() {
			completed.Add(1)
			session.SubmitDiskTask(testDiskCallable{
				transfer: transfer,
				call: func() {
					completed.Add(1)
				},
			})
		},
	})

	for i := 0; i < 50; i++ {
		session.processDiskTasks()
		if completed.Load() == 2 && session.diskTaskCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected follow-up disk task to complete, got completed=%d tasks=%d", completed.Load(), session.diskTaskCount())
}

func TestPeerContinueBufferedIncomingStopsOnIncompleteFrame(t *testing.T) {
	session, transfer := newTestTransfer(t)
	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	peer := NewPeerConnection(session, endpoint, transfer, nil)

	var header protocol.PacketHeader
	header.ResetWithKey(protocol.PK(protocol.EdonkeyProt, 0x46), 32)
	buf := bytes.NewBuffer(make([]byte, 0, protocol.PacketHeaderSize))
	if err := header.Put(buf); err != nil {
		t.Fatalf("put header: %v", err)
	}
	peer.AppendIncoming(buf.Bytes())

	done := make(chan struct{})
	go func() {
		peer.continueBufferedIncoming()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("continueBufferedIncoming did not return on incomplete frame")
	}

	if peer.IncomingBytes() == 0 {
		t.Fatal("expected incomplete frame to remain buffered for a future read")
	}
	if peer.IsDisconnecting() {
		t.Fatal("expected incomplete frame to wait for more bytes, not disconnect")
	}
}

func TestUploadQueuePrefersPeerWithBetterCredits(t *testing.T) {
	session := NewSession(NewSettings())
	queue := session.UploadQueue()

	hashA := protocol.EMule
	hashB := protocol.LibED2K
	session.Credits().ApplySnapshot([]ClientCreditState{
		{PeerHash: hashA, Uploaded: 100000, Downloaded: 4000000},
		{PeerHash: hashB, Uploaded: 1000000, Downloaded: 1000000},
	})

	endpointA, err := protocol.EndpointFromString("1.1.1.1", 4662)
	if err != nil {
		t.Fatalf("endpoint A: %v", err)
	}
	endpointB, err := protocol.EndpointFromString("2.2.2.2", 4662)
	if err != nil {
		t.Fatalf("endpoint B: %v", err)
	}

	peerA := &PeerConnection{
		Connection:   NewConnection(session),
		endpoint:     endpointA,
		remoteHash:   hashA,
		uploadState:  UploadStateOnQueue,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	peerB := &PeerConnection{
		Connection:   NewConnection(session),
		endpoint:     endpointB,
		remoteHash:   hashB,
		uploadState:  UploadStateOnQueue,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	now := CurrentTime()
	peerA.SetUploadWaitStart(now - Seconds(10))
	peerB.SetUploadWaitStart(now - Seconds(10))

	queue.waiting = []*PeerConnection{peerB, peerA}
	queue.sortWaiting()

	if len(queue.waiting) != 2 {
		t.Fatalf("expected 2 queued peers, got %d", len(queue.waiting))
	}
	if queue.waiting[0] != peerA {
		t.Fatal("expected peer with better credits to sort first")
	}
	if peerA.UploadQueueRank() != 1 || peerB.UploadQueueRank() != 2 {
		t.Fatalf("unexpected queue ranks: A=%d B=%d", peerA.UploadQueueRank(), peerB.UploadQueueRank())
	}
}

func TestUploadQueuePrefersHighIDClientForNextSlot(t *testing.T) {
	session := NewSession(NewSettings())
	queue := session.UploadQueue()
	session.Credits().ApplySnapshot([]ClientCreditState{
		{PeerHash: protocol.EMule, Uploaded: 100000, Downloaded: 4000000},
		{PeerHash: protocol.LibED2K, Uploaded: 1000000, Downloaded: 1000000},
	})

	highEndpoint, err := protocol.EndpointFromString("3.3.3.3", 4662)
	if err != nil {
		t.Fatalf("high endpoint: %v", err)
	}
	lowEndpoint, err := protocol.EndpointFromString("4.4.4.4", 4662)
	if err != nil {
		t.Fatalf("low endpoint: %v", err)
	}

	highPeerInfo := NewPeerWithSource(highEndpoint, true, int(PeerIncoming))
	lowPeerInfo := NewPeerWithSource(lowEndpoint, false, int(PeerIncoming))
	highPeer := &PeerConnection{
		Connection:   NewConnection(session),
		endpoint:     highEndpoint,
		peerInfo:     &highPeerInfo,
		remoteHash:   protocol.LibED2K,
		uploadState:  UploadStateOnQueue,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	lowPeer := &PeerConnection{
		Connection:   NewConnection(session),
		endpoint:     lowEndpoint,
		peerInfo:     &lowPeerInfo,
		remoteHash:   protocol.EMule,
		uploadState:  UploadStateOnQueue,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	now := CurrentTime()
	highPeer.SetUploadWaitStart(now - Seconds(5))
	lowPeer.SetUploadWaitStart(now - Seconds(20))

	queue.waiting = []*PeerConnection{lowPeer, highPeer}
	queue.sortWaiting()
	queue.addUpNextClient(nil)

	if len(queue.uploading) != 1 || queue.uploading[0] != highPeer {
		t.Fatal("expected high-id peer to get next upload slot")
	}
	if !lowPeer.UploadAddNextConnect() {
		t.Fatal("expected low-id peer to be marked add-next-connect")
	}
}

func TestUploadQueueLowIDAddNextConnectGetsExtraSlot(t *testing.T) {
	session := NewSession(NewSettings())
	session.settings.UploadSlots = 1
	queue := session.UploadQueue()

	highEndpoint, err := protocol.EndpointFromString("5.5.5.5", 4662)
	if err != nil {
		t.Fatalf("high endpoint: %v", err)
	}
	lowEndpoint, err := protocol.EndpointFromString("6.6.6.6", 4662)
	if err != nil {
		t.Fatalf("low endpoint: %v", err)
	}

	highPeerInfo := NewPeerWithSource(highEndpoint, true, int(PeerIncoming))
	lowPeerInfo := NewPeerWithSource(lowEndpoint, false, int(PeerIncoming))
	highPeer := &PeerConnection{
		Connection:   NewConnection(session),
		endpoint:     highEndpoint,
		peerInfo:     &highPeerInfo,
		uploadState:  UploadStateUploading,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	lowPeer := &PeerConnection{
		Connection:   NewConnection(session),
		endpoint:     lowEndpoint,
		peerInfo:     &lowPeerInfo,
		uploadState:  UploadStateOnQueue,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	lowPeer.SetUploadWaitStart(CurrentTime() - Seconds(10))
	lowPeer.SetUploadAddNextConnect(true)

	queue.uploading = []*PeerConnection{highPeer}
	queue.waiting = []*PeerConnection{lowPeer}
	queue.lastSlotHighID = true

	queue.AddClientToQueue(lowPeer)

	if len(queue.uploading) != 2 {
		t.Fatalf("expected low-id peer to get extra slot, uploading=%d", len(queue.uploading))
	}
	if queue.uploading[1] != lowPeer {
		t.Fatal("expected low-id peer promoted to uploading")
	}
	if lowPeer.UploadAddNextConnect() {
		t.Fatal("expected add-next-connect flag cleared after promotion")
	}
}

func TestUploadQueueFriendSlotNeverKicked(t *testing.T) {
	session := NewSession(NewSettings())
	queue := session.UploadQueue()
	peer := &PeerConnection{
		Connection:   NewConnection(session),
		uploadState:  UploadStateUploading,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
		friendSlot:   true,
	}
	queue.allowKicking = true
	queue.uploading = []*PeerConnection{peer}
	peer.SetUploadStartTime(CurrentTime() - maxUploadTime - 1)

	if queue.CheckForTimeOver(peer) {
		t.Fatal("expected friend slot upload to never be kicked")
	}
}

func TestUploadQueuePowerShareProtectedWhenVipSlotsBelowHalf(t *testing.T) {
	session, transfer := newTestTransfer(t)
	queue := session.UploadQueue()
	transfer.SetUploadPriority(UploadPriorityPowerShare)
	peer := &PeerConnection{
		Connection:   NewConnection(session),
		transfer:     transfer,
		uploadState:  UploadStateUploading,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	queue.allowKicking = true
	queue.uploading = []*PeerConnection{peer}
	peer.SetUploadStartTime(CurrentTime() - maxUploadTime - 1)

	if queue.CheckForTimeOver(peer) {
		t.Fatal("expected powershare upload to be protected while vip slots are below half")
	}
}

func TestUploadQueueSuspendAndResumeUpload(t *testing.T) {
	session, transfer := newTestTransfer(t)
	queue := session.UploadQueue()
	peer := &PeerConnection{
		Connection:   NewConnection(session),
		transfer:     transfer,
		uploadState:  UploadStateUploading,
		uploadBlocks: make([]RequestedUploadBlock, 0),
		uploadDone:   make([]RequestedUploadBlock, 0),
	}
	queue.uploading = []*PeerConnection{peer}

	removed := queue.SuspendUpload(transfer.GetHash(), false)
	if removed != 1 {
		t.Fatalf("expected 1 suspended upload, got %d", removed)
	}
	if !queue.isSuspended(transfer.GetHash()) {
		t.Fatal("expected transfer to be marked suspended")
	}
	if len(queue.waiting) != 1 || queue.waiting[0] != peer {
		t.Fatal("expected suspended peer moved back to waiting queue")
	}

	queue.ResumeUpload(transfer.GetHash())
	if queue.isSuspended(transfer.GetHash()) {
		t.Fatal("expected resume to clear suspended mark")
	}
}

func TestUploadQueueMaxSlotsUsesUploadRate(t *testing.T) {
	settings := NewSettings()
	settings.MaxUploadRateKB = 0
	settings.SlotAllocationKB = 3
	session := NewSession(settings)
	session.accumulator.SendBytes(12*1024, 0)
	queue := session.UploadQueue()

	if got := queue.maxSlots(); got < minUploadClientsAllowed {
		t.Fatalf("expected dynamic slot count >= %d, got %d", minUploadClientsAllowed, got)
	}
}
