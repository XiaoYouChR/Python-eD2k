package goed2k

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
)

type memoryClientStateStore struct {
	state *ClientState
}

func (m *memoryClientStateStore) Load() (*ClientState, error) {
	return cloneClientState(m.state), nil
}

func (m *memoryClientStateStore) Save(state *ClientState) error {
	m.state = cloneClientState(state)
	return nil
}

func TestClientAddLinkSupportsPerTransferOutputDir(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	dirA := filepath.Join(t.TempDir(), "a")
	dirB := filepath.Join(t.TempDir(), "b")

	handleA, pathA, err := client.AddLink("ed2k://|file|one.bin|1024|31D6CFE0D16AE931B73C59D7E0C089C0|/", dirA)
	if err != nil {
		t.Fatalf("add link A: %v", err)
	}
	handleB, pathB, err := client.AddLink("ed2k://|file|two.bin|2048|31D6CFE0D14CE931B73C59D7E0C04BC0|/", dirB)
	if err != nil {
		t.Fatalf("add link B: %v", err)
	}

	if !handleA.IsValid() || !handleB.IsValid() {
		t.Fatal("expected both transfer handles to be valid")
	}
	if pathA != filepath.Join(dirA, "one.bin") {
		t.Fatalf("unexpected target path A: %s", pathA)
	}
	if pathB != filepath.Join(dirB, "two.bin") {
		t.Fatalf("unexpected target path B: %s", pathB)
	}
	if got := len(client.Session().GetTransfers()); got != 2 {
		t.Fatalf("expected 2 transfers, got %d", got)
	}
}

func TestClientPauseResumeAndRemoveTransfer(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	handle, _, err := client.AddLink("ed2k://|file|song.mp3|2048|31D6CFE0D10EE931B73C59D7E0C06FC0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}
	hash := handle.GetHash()

	if err := client.PauseTransfer(hash); err != nil {
		t.Fatalf("pause transfer: %v", err)
	}
	if !client.FindTransfer(hash).IsPaused() {
		t.Fatal("expected paused transfer")
	}

	if err := client.ResumeTransfer(hash); err != nil {
		t.Fatalf("resume transfer: %v", err)
	}
	if client.FindTransfer(hash).IsPaused() {
		t.Fatal("expected resumed transfer")
	}

	if err := client.RemoveTransfer(hash, false); err != nil {
		t.Fatalf("remove transfer: %v", err)
	}
	if got := len(client.Transfers()); got != 0 {
		t.Fatalf("expected no transfers after remove, got %d", got)
	}
}

func TestClientPauseTransferDisconnectsActivePeers(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	handle, _, err := client.AddLink("ed2k://|file|pause.bin|2048|31D6CFE0D10EE931B73C59D7E0C06FC0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}

	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := handle.transfer.AddPeer(endpoint, int(PeerServer)); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	peerInfo := handle.transfer.policy.FindPeer(endpoint)
	if peerInfo == nil {
		t.Fatal("expected peer in policy")
	}
	peer := NewPeerConnection(client.Session(), endpoint, handle.transfer, peerInfo)
	peer.SetPeer(peerInfo)
	handle.transfer.connections = append(handle.transfer.connections, peer)
	handle.transfer.policy.SetConnection(peerInfo, peer)

	if err := client.PauseTransfer(handle.GetHash()); err != nil {
		t.Fatalf("pause transfer: %v", err)
	}
	if !handle.IsPaused() {
		t.Fatal("expected transfer to be paused")
	}
	if got := handle.GetStatus().State; got != PausedState {
		t.Fatalf("expected paused state, got %s", got)
	}
	if !peer.IsDisconnecting() {
		t.Fatal("expected active peer connection to be disconnected on pause")
	}
	if got := handle.ActiveConnections(); got != 0 {
		t.Fatalf("expected no active connections after pause, got %d", got)
	}

	peer.OnDisconnect(TransferPaused)
	if got := handle.transfer.policy.NumConnectCandidates(); got != 1 {
		t.Fatalf("expected paused peer to remain reconnect candidate, got %d", got)
	}

	if err := client.ResumeTransfer(handle.GetHash()); err != nil {
		t.Fatalf("resume transfer: %v", err)
	}
	if handle.IsPaused() {
		t.Fatal("expected resumed transfer")
	}
	if got := handle.transfer.policy.NumConnectCandidates(); got != 1 {
		t.Fatalf("expected reconnect candidate after resume, got %d", got)
	}
	if handle.transfer.nextSourcesRequest != 0 {
		t.Fatalf("expected nextSourcesRequest reset on resume, got %d", handle.transfer.nextSourcesRequest)
	}
	if handle.transfer.nextDHTRequest != 0 {
		t.Fatalf("expected nextDHTRequest reset on resume, got %d", handle.transfer.nextDHTRequest)
	}
}

func TestClientSaveAndLoadStateRestoresProgress(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0

	statePath := filepath.Join(t.TempDir(), "state.json")
	outputDir := filepath.Join(t.TempDir(), "downloads")

	client := NewClient(settings)
	client.SetStatePath(statePath)
	client.serverAddr = "176.123.5.89:4725"

	handle, targetPath, err := client.AddLink("ed2k://|file|resume.bin|1945600|31D6CFE0D16AE931B73C59D7E0C089C0|/", outputDir)
	if err != nil {
		t.Fatalf("add link: %v", err)
	}
	block := data.NewPieceBlock(0, 0)
	if _, err := handle.transfer.pm.WriteBlock(block, make([]byte, BlockSize)); err != nil {
		t.Fatalf("seed block data: %v", err)
	}
	handle.transfer.picker.WeHaveBlock(block)
	peerEndpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := handle.transfer.AddPeer(peerEndpoint, int(PeerResume)); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	handle.Pause()

	if err := client.SaveState(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	restored := NewClient(settings)
	if err := restored.LoadState(statePath); err != nil {
		t.Fatalf("load state: %v", err)
	}
	restoredHandle := restored.FindTransfer(handle.GetHash())
	if !restoredHandle.IsValid() {
		t.Fatal("expected restored transfer handle to be valid")
	}
	if restoredHandle.GetFilePath() != targetPath {
		t.Fatalf("unexpected restored path: %s", restoredHandle.GetFilePath())
	}
	if !restoredHandle.IsPaused() {
		t.Fatal("expected restored transfer to stay paused")
	}
	if restored.serverAddr != client.serverAddr {
		t.Fatalf("expected restored server address %q, got %q", client.serverAddr, restored.serverAddr)
	}

	for i := 0; i < 50; i++ {
		UpdateCachedTime()
		restored.session.SecondTick(CurrentTime(), 100)
		if restoredHandle.GetStatus().TotalDone >= BlockSize {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	status := restoredHandle.GetStatus()
	if status.TotalDone < BlockSize {
		t.Fatalf("expected restored progress >= %d, got %d", BlockSize, status.TotalDone)
	}
	if status.NumPeers != 1 {
		t.Fatalf("expected restored peer count 1, got %d", status.NumPeers)
	}
}

func TestClientSupportsCustomStateStore(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0

	store := &memoryClientStateStore{}
	client := NewClient(settings)
	client.SetStateStore(store)
	client.serverAddr = "176.123.5.89:4725"

	handle, path, err := client.AddLink("ed2k://|file|custom.bin|2048|31D6CFE0D14CE931B73C59D7E0C04BC0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}
	handle.Pause()
	if err := client.SaveState(""); err != nil {
		t.Fatalf("save state with custom store: %v", err)
	}
	if store.state == nil || len(store.state.Transfers) != 1 {
		t.Fatal("expected custom store to capture one transfer")
	}

	restored := NewClient(settings)
	restored.SetStateStore(store)
	if err := restored.LoadState(""); err != nil {
		t.Fatalf("load state with custom store: %v", err)
	}

	restoredHandle := restored.FindTransfer(handle.GetHash())
	if !restoredHandle.IsValid() {
		t.Fatal("expected restored handle from custom store")
	}
	if restoredHandle.GetFilePath() != path {
		t.Fatalf("unexpected restored path: %s", restoredHandle.GetFilePath())
	}
	if !restoredHandle.IsPaused() {
		t.Fatal("expected restored paused transfer")
	}
	if restored.ServerAddress() != client.ServerAddress() {
		t.Fatalf("expected restored server address %q, got %q", client.ServerAddress(), restored.ServerAddress())
	}
}

func TestClientAddLinkRequestsSourcesImmediatelyWhenServerReady(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	serverAddr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	serverConn := NewServerConnection("test", serverAddr, client.session)
	serverConn.handshakeCompleted = true
	client.session.serverConnection = serverConn
	client.session.serverConnections["test"] = serverConn

	handle, _, err := client.AddLink("ed2k://|file|immediate.bin|2048|31D6CFE0D16AE931B73C59D7E0C089C0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}
	if !handle.IsValid() {
		t.Fatal("expected valid transfer handle")
	}
	if got := len(serverConn.PendingPackets()); got == 0 {
		t.Fatal("expected immediate GetFileSources packet after AddLink")
	}
}

func TestClientSubscribeStatusReceivesSnapshots(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	events, cancel := client.SubscribeStatus()
	defer cancel()

	select {
	case event := <-events:
		if len(event.Status.Transfers) != 0 {
			t.Fatalf("expected initial empty transfer list, got %d", len(event.Status.Transfers))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial status event")
	}

	if _, _, err := client.AddLink("ed2k://|file|listen.bin|1024|31D6CFE0D16AE931B73C59D7E0C089C0|/", t.TempDir()); err != nil {
		t.Fatalf("add link: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if len(event.Status.Transfers) == 1 {
				state, ok := event.TransferState(event.Status.Transfers[0].Hash)
				if !ok {
					t.Fatal("expected transfer state lookup to succeed")
				}
				if state == "" {
					t.Fatal("expected non-empty transfer state")
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for transfer status event")
		}
	}
}

func TestClientSubscribeTransferProgressReceivesStateChanges(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	events, cancel := client.SubscribeTransferProgress()
	defer cancel()

	select {
	case event := <-events:
		if len(event.Transfers) != 0 {
			t.Fatalf("expected initial empty progress event, got %d transfers", len(event.Transfers))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial progress event")
	}

	handle, _, err := client.AddLink("ed2k://|file|progress.bin|2048|31D6CFE0D16AE931B73C59D7E0C089C0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}

	var created bool
	deadline := time.After(2 * time.Second)
	for !created {
		select {
		case event := <-events:
			for _, transfer := range event.Transfers {
				if transfer.Hash.Compare(handle.GetHash()) == 0 && !transfer.Removed {
					created = true
					break
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for transfer progress creation event")
		}
	}

	if err := client.PauseTransfer(handle.GetHash()); err != nil {
		t.Fatalf("pause transfer: %v", err)
	}

	deadline = time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			for _, transfer := range event.Transfers {
				if transfer.Hash.Compare(handle.GetHash()) == 0 && transfer.State == PausedState {
					return
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for paused progress event")
		}
	}
}

func TestClientStatusIncludesServersPeersAndTransfers(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0

	client := NewClient(settings)
	handle, filePath, err := client.AddLink("ed2k://|file|status.bin|1945600|31D6CFE0D14CE931B73C59D7E0C04BC0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}

	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := handle.transfer.AddPeer(endpoint, int(PeerServer)); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	peer := NewPeerWithSource(endpoint, true, int(PeerServer))
	connection := NewPeerConnection(client.Session(), endpoint, handle.transfer, &peer)
	connection.remotePeerInfo.ModName = "aMule"
	connection.Connection.stat.ReceiveBytes(16, 128)
	connection.Connection.stat.SendBytes(8, 64)
	connection.Connection.stat.SecondTick(1000)
	handle.transfer.connections = append(handle.transfer.connections, connection)
	handle.transfer.WeHave(0)
	handle.transfer.refreshStats()

	serverAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:4661")
	if err != nil {
		t.Fatalf("resolve server addr: %v", err)
	}
	serverConn := NewServerConnection("127.0.0.1:4661", serverAddr, client.Session())
	serverConn.handshakeCompleted = true
	serverConn.clientID = 12345
	serverConn.tcpFlags = 7
	serverConn.auxPort = 4672
	serverConn.Connection.stat.ReceiveBytes(32, 0)
	serverConn.Connection.stat.SendBytes(16, 0)
	serverConn.Connection.stat.SecondTick(1000)

	client.session.mu.Lock()
	client.session.configuredServers[serverConn.GetIdentifier()] = serverAddr
	client.session.serverConnections[serverConn.GetIdentifier()] = serverConn
	client.session.serverConnection = serverConn
	client.session.mu.Unlock()

	status := client.Status()
	if len(status.Servers) != 1 {
		t.Fatalf("expected 1 server snapshot, got %d", len(status.Servers))
	}
	if len(status.Transfers) != 1 {
		t.Fatalf("expected 1 transfer snapshot, got %d", len(status.Transfers))
	}
	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer snapshot, got %d", len(status.Peers))
	}

	server := status.Servers[0]
	if server.Identifier != "127.0.0.1:4661" {
		t.Fatalf("unexpected server identifier: %s", server.Identifier)
	}
	if !server.HandshakeCompleted || !server.Primary || !server.Configured {
		t.Fatalf("unexpected server flags: %+v", server)
	}
	if server.ClientID != 12345 || server.DownloadRate <= 0 || server.UploadRate <= 0 {
		t.Fatalf("unexpected server counters: %+v", server)
	}

	transfer := status.Transfers[0]
	if transfer.FilePath != filePath {
		t.Fatalf("unexpected transfer path: %s", transfer.FilePath)
	}
	if transfer.ActivePeers != 1 || transfer.Status.TotalDone != handle.GetSize() {
		t.Fatalf("unexpected transfer snapshot: %+v", transfer)
	}
	if transfer.Status.Upload != 72 || transfer.Status.DownloadRate <= 0 {
		t.Fatalf("unexpected transfer traffic counters: %+v", transfer.Status)
	}
	if len(transfer.Pieces) != 1 {
		t.Fatalf("expected 1 piece snapshot, got %d", len(transfer.Pieces))
	}
	if transfer.Status.DownloadingPieces != 0 {
		t.Fatalf("expected no downloading pieces, got %d", transfer.Status.DownloadingPieces)
	}
	if transfer.Pieces[0].State != PieceSnapshotFinished || transfer.Pieces[0].DoneBytes != handle.GetSize() {
		t.Fatalf("unexpected piece snapshot: %+v", transfer.Pieces[0])
	}

	peerSnapshot := status.Peers[0]
	if peerSnapshot.TransferHash.Compare(handle.GetHash()) != 0 {
		t.Fatalf("unexpected peer transfer hash: %s", peerSnapshot.TransferHash.String())
	}
	if peerSnapshot.Peer.DownloadSpeed <= 0 || peerSnapshot.Peer.UploadSpeed <= 0 {
		t.Fatalf("unexpected peer counters: %+v", peerSnapshot.Peer)
	}
	if peerSnapshot.Peer.ModName != "aMule" {
		t.Fatalf("unexpected peer mod name: %s", peerSnapshot.Peer.ModName)
	}

	if status.TotalDone != handle.GetSize() || status.Upload != 72 || status.DownloadRate <= 0 {
		t.Fatalf("unexpected client totals: %+v", status)
	}
}

func TestClientStatusIncludesDownloadingPieceSnapshots(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0

	client := NewClient(settings)
	handle, _, err := client.AddLink("ed2k://|file|piece.bin|19456000|31D6CFE0D14CE931B73C59D7E0C04BC0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}

	endpoint, err := protocol.EndpointFromString("1.2.3.4", 4662)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	peer := NewPeerWithSource(endpoint, true, int(PeerServer))
	conn := NewPeerConnection(client.Session(), endpoint, handle.transfer, &peer)
	conn.downloadQueue = append(conn.downloadQueue, PendingBlock{
		Block:      data.NewPieceBlock(0, 0),
		DataSize:   BlockSize,
		CreateTime: CurrentTime(),
		Received:   4096,
	})
	handle.transfer.connections = append(handle.transfer.connections, conn)
	handle.transfer.picker.DownloadPiece(0)
	handle.transfer.picker.MarkAsDownloading(data.NewPieceBlock(0, 0), conn.GetPeer())

	status := client.Status()
	if len(status.Transfers) != 1 {
		t.Fatalf("expected 1 transfer snapshot, got %d", len(status.Transfers))
	}
	transfer := status.Transfers[0]
	if transfer.Status.DownloadingPieces != 1 {
		t.Fatalf("expected 1 downloading piece, got %d", transfer.Status.DownloadingPieces)
	}
	if len(transfer.Pieces) != 2 {
		t.Fatalf("expected 2 piece snapshots, got %d", len(transfer.Pieces))
	}

	piece := transfer.Pieces[0]
	if piece.State != PieceSnapshotDownloading {
		t.Fatalf("expected downloading piece state, got %s", piece.State)
	}
	if piece.BlocksPending != 1 || piece.ReceivedBytes != 4096 || piece.DoneBytes != 0 {
		t.Fatalf("unexpected downloading piece snapshot: %+v", piece)
	}
	if transfer.Pieces[1].State != PieceSnapshotMissing {
		t.Fatalf("expected second piece to be missing, got %s", transfer.Pieces[1].State)
	}
}

func TestTransferStatusShowsVerifyingWhenAllBytesReceivedButNotFinished(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	session := NewSession(settings)

	handle, err := session.AddTransferParams(NewAddTransferParamsFromHandler(
		protocol.EMule,
		CurrentTimeMillis(),
		int64(BlockSize),
		nil,
		false,
	))
	if err != nil {
		t.Fatalf("add transfer: %v", err)
	}

	block := data.NewPieceBlock(0, 0)
	handle.transfer.picker.WeHaveBlock(block)
	handle.transfer.state = Downloading

	status := handle.GetStatus()
	if status.TotalDone != int64(BlockSize) {
		t.Fatalf("expected total done %d, got %d", BlockSize, status.TotalDone)
	}
	if status.State != Verifying {
		t.Fatalf("expected verifying state, got %s", status.State)
	}
}

func TestServerSnapshotIDClass(t *testing.T) {
	low := ServerSnapshot{ClientID: 12345}
	if got := low.IDClass(); got != "LOW_ID" {
		t.Fatalf("expected LOW_ID, got %s", got)
	}
	high := ServerSnapshot{ClientID: 0x7f000001}
	if got := high.IDClass(); got != "HIGH_ID" {
		t.Fatalf("expected HIGH_ID, got %s", got)
	}
	unknown := ServerSnapshot{}
	if got := unknown.IDClass(); got != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN, got %s", got)
	}
}

func TestOnBlockWriteCompletedQueuesPieceHash(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	session := NewSession(settings)

	tmpDir := t.TempDir()
	handle, err := session.AddTransferWithHandler(protocol.EMule, int64(BlockSize), disk.NewDesktopFileHandler(filepath.Join(tmpDir, "hash.bin")))
	if err != nil {
		t.Fatalf("add transfer: %v", err)
	}

	block := data.NewPieceBlock(0, 0)
	handle.transfer.picker.WeHaveBlock(block)
	handle.transfer.OnBlockWriteCompleted(block, nil, NoError)

	if !handle.transfer.pendingPieceHashes[0] {
		t.Fatal("expected pending piece hash after block write completion")
	}
	if session.diskTaskCount() == 0 {
		t.Fatal("expected async hash task to be queued")
	}
}

func TestClientSaveAndLoadStateRestoresUploadSettings(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0

	store := &memoryClientStateStore{}
	client := NewClient(settings)
	client.SetStateStore(store)

	handle, _, err := client.AddLink("ed2k://|file|upload.bin|2048|31D6CFE0D14CE931B73C59D7E0C04BC0|/", t.TempDir())
	if err != nil {
		t.Fatalf("add link: %v", err)
	}
	if err := client.SetTransferUploadPriority(handle.GetHash(), UploadPriorityPowerShare); err != nil {
		t.Fatalf("set upload priority: %v", err)
	}
	client.SetFriendSlot(protocol.EMule, true)
	if err := client.SaveState(""); err != nil {
		t.Fatalf("save state: %v", err)
	}

	restored := NewClient(settings)
	restored.SetStateStore(store)
	if err := restored.LoadState(""); err != nil {
		t.Fatalf("load state: %v", err)
	}
	restoredHandle := restored.FindTransfer(handle.GetHash())
	if !restoredHandle.IsValid() {
		t.Fatal("expected restored transfer")
	}
	if restoredHandle.transfer.UploadPriority() != UploadPriorityPowerShare {
		t.Fatalf("expected restored upload priority %d, got %d", UploadPriorityPowerShare, restoredHandle.transfer.UploadPriority())
	}
	if !restored.session.IsFriendSlot(protocol.EMule) {
		t.Fatal("expected restored friend slot flag")
	}
}

func TestClientWaitReturnsWhenStopped(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	if _, _, err := client.AddLink("ed2k://|file|wait.bin|2048|31D6CFE0D14CE931B73C59D7E0C04BC0|/", t.TempDir()); err != nil {
		t.Fatalf("add link: %v", err)
	}
	if err := client.Start(); err != nil {
		t.Fatalf("start client: %v", err)
	}
	defer client.Close()

	done := make(chan error, 1)
	go func() {
		done <- client.Wait()
	}()

	time.Sleep(50 * time.Millisecond)
	if err := client.Stop(); err != nil {
		t.Fatalf("stop client: %v", err)
	}

	select {
	case err := <-done:
		if err != ErrClientStopped {
			t.Fatalf("expected ErrClientStopped, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return after stop")
	}
}

func TestClientStartAutoAssignsTCPAndUDPPorts(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0
	settings.EnableDHT = true

	client := NewClient(settings)
	if err := client.Start(); err != nil {
		t.Fatalf("start client: %v", err)
	}
	defer client.Close()

	if got := client.Session().GetListenPort(); got <= 0 {
		t.Fatalf("expected auto-assigned tcp port, got %d", got)
	}
	if got := client.Session().GetUDPPort(); got <= 0 {
		t.Fatalf("expected auto-assigned udp port, got %d", got)
	}
	if got := client.DHTStatus().ListenPort; got <= 0 {
		t.Fatalf("expected dht status to expose auto-assigned udp port, got %d", got)
	}
}

func TestClientEnableDHTLoadsNodesDat(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0
	settings.EnableDHT = true

	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write nodes count: %v", err)
	}
	if _, err := payload.Write(make([]byte, 16)); err != nil {
		t.Fatalf("write id: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0x0100007f)); err != nil {
		t.Fatalf("write ip: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint16(4672)); err != nil {
		t.Fatalf("write udp port: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint16(4661)); err != nil {
		t.Fatalf("write tcp port: %v", err)
	}
	if err := payload.WriteByte(8); err != nil {
		t.Fatalf("write version: %v", err)
	}

	nodesDatPath := filepath.Join(t.TempDir(), "nodes.dat")
	if err := os.WriteFile(nodesDatPath, payload.Bytes(), 0o644); err != nil {
		t.Fatalf("write nodes.dat: %v", err)
	}

	client := NewClient(settings)
	baseNodes := len(client.EnableDHT().nodes)
	if err := client.LoadDHTNodesDat(nodesDatPath); err != nil {
		t.Fatalf("load nodes.dat: %v", err)
	}

	tracker := client.GetDHTTracker()
	if tracker == nil {
		t.Fatal("expected DHT tracker to be configured")
	}
	if got := len(tracker.nodes); got < baseNodes+1 {
		t.Fatalf("expected at least %d DHT nodes, got %d", baseNodes+1, got)
	}
}

func TestClientLoadDHTNodesDatSupportsHTTPFileAndMultipleSources(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0
	settings.EnableDHT = true

	makeNodesDat := func(hash string, ip uint32, verified bool) []byte {
		var payload bytes.Buffer
		if err := binary.Write(&payload, binary.LittleEndian, uint32(0)); err != nil {
			t.Fatalf("write zero prefix: %v", err)
		}
		if err := binary.Write(&payload, binary.LittleEndian, uint32(2)); err != nil {
			t.Fatalf("write version: %v", err)
		}
		if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
			t.Fatalf("write contact count: %v", err)
		}
		id := protocol.MustHashFromString(hash)
		for idx := 0; idx < 16; idx++ {
			if err := payload.WriteByte(id.At((idx/4)*4 + 3 - (idx % 4))); err != nil {
				t.Fatalf("write id: %v", err)
			}
		}
		if err := binary.Write(&payload, binary.LittleEndian, ip); err != nil {
			t.Fatalf("write ip: %v", err)
		}
		if err := binary.Write(&payload, binary.LittleEndian, uint16(4672)); err != nil {
			t.Fatalf("write udp port: %v", err)
		}
		if err := binary.Write(&payload, binary.LittleEndian, uint16(4661)); err != nil {
			t.Fatalf("write tcp port: %v", err)
		}
		if err := payload.WriteByte(8); err != nil {
			t.Fatalf("write version byte: %v", err)
		}
		if _, err := payload.Write(make([]byte, 8)); err != nil {
			t.Fatalf("write udp key: %v", err)
		}
		if verified {
			if err := payload.WriteByte(1); err != nil {
				t.Fatalf("write verified: %v", err)
			}
		} else {
			if err := payload.WriteByte(0); err != nil {
				t.Fatalf("write verified: %v", err)
			}
		}
		return payload.Bytes()
	}

	pathA := filepath.Join(t.TempDir(), "a.nodes.dat")
	if err := os.WriteFile(pathA, makeNodesDat("23A8CEFF57A7A32D562D649ED7893796", 0x0100007f, true), 0o644); err != nil {
		t.Fatalf("write nodes a: %v", err)
	}
	pathB := filepath.Join(t.TempDir(), "b.nodes.dat")
	if err := os.WriteFile(pathB, makeNodesDat("31D6CFE0D16AE931B73C59D7E0C089C0", 0x0200007f, false), 0o644); err != nil {
		t.Fatalf("write nodes b: %v", err)
	}
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(makeNodesDat("31D6CFE0D14CE931B73C59D7E0C04BC0", 0x0300007f, true))
	}))
	defer httpSrv.Close()

	client := NewClient(settings)
	baseNodes := len(client.EnableDHT().nodes)
	if err := client.LoadDHTNodesDat(pathA, "file://"+pathB, httpSrv.URL); err != nil {
		t.Fatalf("load multiple nodes.dat sources: %v", err)
	}

	tracker := client.GetDHTTracker()
	if tracker == nil {
		t.Fatal("expected tracker to be initialized")
	}
	if got := len(tracker.nodes); got < baseNodes+3 {
		t.Fatalf("expected at least %d known nodes, got %d", baseNodes+3, got)
	}
	live, _ := tracker.table.Size()
	if live == 0 {
		t.Fatal("expected ordinary nodes.dat contacts to populate routing table")
	}
}

func TestClientLoadDHTNodesDatIgnoresFailedSourceWhenAnotherSucceeds(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0
	settings.EnableDHT = true

	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write zero prefix: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write contact count: %v", err)
	}
	id := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	for idx := 0; idx < 16; idx++ {
		if err := payload.WriteByte(id.At((idx/4)*4 + 3 - (idx % 4))); err != nil {
			t.Fatalf("write id: %v", err)
		}
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0x0100007f)); err != nil {
		t.Fatalf("write ip: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint16(4672)); err != nil {
		t.Fatalf("write udp port: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint16(4661)); err != nil {
		t.Fatalf("write tcp port: %v", err)
	}
	if err := payload.WriteByte(8); err != nil {
		t.Fatalf("write version byte: %v", err)
	}
	if _, err := payload.Write(make([]byte, 8)); err != nil {
		t.Fatalf("write udp key: %v", err)
	}
	if err := payload.WriteByte(1); err != nil {
		t.Fatalf("write verified: %v", err)
	}

	goodPath := filepath.Join(t.TempDir(), "good.nodes.dat")
	if err := os.WriteFile(goodPath, payload.Bytes(), 0o644); err != nil {
		t.Fatalf("write nodes.dat: %v", err)
	}

	client := NewClient(settings)
	baseNodes := len(client.EnableDHT().nodes)
	if err := client.LoadDHTNodesDat(filepath.Join(t.TempDir(), "missing.nodes.dat"), goodPath); err != nil {
		t.Fatalf("expected partial success, got error: %v", err)
	}

	tracker := client.GetDHTTracker()
	if tracker == nil {
		t.Fatal("expected tracker to be initialized")
	}
	if got := len(tracker.nodes); got < baseNodes+1 {
		t.Fatalf("expected at least %d known nodes, got %d", baseNodes+1, got)
	}
}

func TestClientLoadBootstrapNodesDatUsesRouterNodes(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0
	settings.EnableDHT = true

	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write zero prefix: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(3)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write bootstrap edition: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write contact count: %v", err)
	}
	id := protocol.MustHashFromString("31D6CFE0D10EE931B73C59D7E0C06FC0")
	for idx := 0; idx < 16; idx++ {
		if err := payload.WriteByte(id.At((idx/4)*4 + 3 - (idx % 4))); err != nil {
			t.Fatalf("write id: %v", err)
		}
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint32(0x0100007f)); err != nil {
		t.Fatalf("write ip: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint16(4672)); err != nil {
		t.Fatalf("write udp port: %v", err)
	}
	if err := binary.Write(&payload, binary.LittleEndian, uint16(4661)); err != nil {
		t.Fatalf("write tcp port: %v", err)
	}
	if err := payload.WriteByte(8); err != nil {
		t.Fatalf("write version byte: %v", err)
	}

	path := filepath.Join(t.TempDir(), "bootstrap.nodes.dat")
	if err := os.WriteFile(path, payload.Bytes(), 0o644); err != nil {
		t.Fatalf("write bootstrap nodes.dat: %v", err)
	}

	client := NewClient(settings)
	baseRouterNodes := len(client.EnableDHT().table.RouterNodes())
	if err := client.LoadDHTNodesDat(path); err != nil {
		t.Fatalf("load bootstrap nodes.dat: %v", err)
	}
	tracker := client.GetDHTTracker()
	if tracker == nil {
		t.Fatal("expected tracker to be initialized")
	}
	if got := len(tracker.table.RouterNodes()); got < baseRouterNodes+1 {
		t.Fatalf("expected at least %d router/bootstrap nodes, got %d", baseRouterNodes+1, got)
	}
}

func TestClientLoadServerMetParsesFixture(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	path := filepath.Join("..", "jed2k", "core", "src", "main", "resources", "server.met")
	entries, err := client.LoadServerMet(path)
	if err != nil {
		t.Fatalf("load server.met: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected server entries from fixture")
	}
	found := false
	for _, entry := range entries {
		if entry.Address() == "91.200.42.47:3883" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected known server address in fixture, got %#v", entries[0].Address())
	}
}

func TestClientLoadServerMetParsesServerListED2KLink(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	fixturePath := filepath.Join("..", "jed2k", "core", "src", "main", "resources", "server.met")
	payload, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	linkValue := fmt.Sprintf("ed2k://|serverlist|%s|/", srv.URL)
	entries, err := client.LoadServerMet(linkValue)
	if err != nil {
		t.Fatalf("load serverlist link: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries from serverlist link")
	}
}

func TestClientConnectServerLinkTracksConfiguredServer(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	client := NewClient(settings)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil && conn != nil {
			_ = conn.Close()
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	linkValue := fmt.Sprintf("ed2k://|server|127.0.0.1|%d|/", addr.Port)
	if err := client.ConnectServerLink(linkValue); err != nil {
		t.Fatalf("connect server link: %v", err)
	}
	if client.ServerAddress() != fmt.Sprintf("127.0.0.1:%d", addr.Port) {
		t.Fatalf("unexpected saved server address: %q", client.ServerAddress())
	}
}

func TestClientAddDHTBootstrapNodesParsesCommaSeparated(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0

	client := NewClient(settings)
	baseNodes := len(client.EnableDHT().nodes)
	if err := client.AddDHTBootstrapNodes("1.2.3.4:4665, 5.6.7.8:5678"); err != nil {
		t.Fatalf("add DHT bootstrap nodes: %v", err)
	}

	tracker := client.GetDHTTracker()
	if tracker == nil {
		t.Fatal("expected DHT tracker to be configured")
	}
	if got := len(tracker.nodes); got < baseNodes+2 {
		t.Fatalf("expected at least %d DHT nodes, got %d", baseNodes+2, got)
	}
}

func TestClientDHTStatusReflectsTrackerState(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0

	client := NewClient(settings)
	tracker := client.EnableDHT()
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4665")
	if err != nil {
		t.Fatalf("resolve dht addr: %v", err)
	}
	tracker.addOrUpdateNode(kadproto.NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")), addr, 4661, 8, true)
	tracker.table.NodeSeen(tracker.nodes[addr.String()])
	status := client.DHTStatus()
	if !status.Bootstrapped {
		t.Fatal("expected bootstrapped DHT status")
	}
	if status.LiveNodes != 1 {
		t.Fatalf("expected 1 live node, got %d", status.LiveNodes)
	}
	if status.KnownNodes != 1 {
		t.Fatalf("expected 1 known node, got %d", status.KnownNodes)
	}
}

func TestClientSaveAndLoadStateRestoresDHTState(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0

	statePath := filepath.Join(t.TempDir(), "state.json")
	client := NewClient(settings)
	client.SetStatePath(statePath)
	tracker := client.EnableDHT()
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve dht addr: %v", err)
	}
	router, err := net.ResolveUDPAddr("udp", "5.6.7.8:4672")
	if err != nil {
		t.Fatalf("resolve router addr: %v", err)
	}
	tracker.table.AddRouterNode(router)
	tracker.SetStoragePoint(router)
	tracker.addOrUpdateNode(kadproto.NewID(protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")), addr, 4661, 8, true)
	tracker.table.NodeSeen(tracker.nodes[addr.String()])

	if err := client.SaveState(""); err != nil {
		t.Fatalf("save state: %v", err)
	}

	restored := NewClient(settings)
	if err := restored.LoadState(statePath); err != nil {
		t.Fatalf("load state: %v", err)
	}
	status := restored.DHTStatus()
	if status.LiveNodes != 1 {
		t.Fatalf("expected restored live nodes 1, got %d", status.LiveNodes)
	}
	if status.RouterNodes != 1 {
		t.Fatalf("expected restored router nodes 1, got %d", status.RouterNodes)
	}
	if status.StoragePoint != router.String() {
		t.Fatalf("expected restored storage point %s, got %s", router.String(), status.StoragePoint)
	}
}

func TestClientSetDHTStoragePoint(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0

	client := NewClient(settings)
	if err := client.SetDHTStoragePoint("1.2.3.4:4672"); err != nil {
		t.Fatalf("set storage point: %v", err)
	}
	if got := client.DHTStatus().StoragePoint; got != "1.2.3.4:4672" {
		t.Fatalf("expected storage point to be set, got %s", got)
	}
}

func TestClientSearchDHTKeywordsUsesTracker(t *testing.T) {
	settings := NewSettings()
	settings.ListenPort = 0
	settings.UDPPort = 0

	client := NewClient(settings)
	tracker := client.EnableDHT()
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:4672")
	if err != nil {
		t.Fatalf("resolve dht addr: %v", err)
	}
	keyword := protocol.MustHashFromString("23A8CEFF57A7A32D562D649ED7893796")
	tracker.addOrUpdateNode(kadproto.NewID(keyword), addr, 4661, 8, true)
	tracker.table.NodeSeen(tracker.nodes[addr.String()])

	called := false
	if !client.SearchDHTKeywords(keyword, func(entries []kadproto.SearchEntry) {
		called = true
	}) {
		t.Fatal("expected SearchDHTKeywords to start traversal")
	}
	if called {
		t.Fatal("search callback should not be invoked in pure unit test setup")
	}
}

func cloneClientState(src *ClientState) *ClientState {
	if src == nil {
		return nil
	}
	dst := &ClientState{
		Version:       src.Version,
		ServerAddress: src.ServerAddress,
		Transfers:     make([]ClientTransferState, 0, len(src.Transfers)),
		Credits:       append([]ClientCreditState(nil), src.Credits...),
		FriendSlots:   append([]protocol.Hash(nil), src.FriendSlots...),
	}
	if src.DHT != nil {
		dst.DHT = cloneDHTState(src.DHT)
	}
	for _, transfer := range src.Transfers {
		dst.Transfers = append(dst.Transfers, ClientTransferState{
			Hash:       transfer.Hash,
			Size:       transfer.Size,
			CreateTime: transfer.CreateTime,
			TargetPath: transfer.TargetPath,
			Paused:     transfer.Paused,
			UploadPrio: transfer.UploadPrio,
			ResumeData: cloneResumeData(transfer.ResumeData),
		})
	}
	return dst
}

func cloneDHTState(src *ClientDHTState) *ClientDHTState {
	if src == nil {
		return nil
	}
	dst := &ClientDHTState{
		SelfID:              src.SelfID,
		Firewalled:          src.Firewalled,
		LastBootstrap:       src.LastBootstrap,
		LastRefresh:         src.LastRefresh,
		LastFirewalledCheck: src.LastFirewalledCheck,
		StoragePoint:        src.StoragePoint,
		Nodes:               append([]ClientDHTNodeState(nil), src.Nodes...),
		RouterNodes:         append([]string(nil), src.RouterNodes...),
	}
	return dst
}
