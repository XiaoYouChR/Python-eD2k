package goed2k

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monkeyWie/goed2k/disk"
	"github.com/monkeyWie/goed2k/protocol"
)

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestLocalPeerUploadServesRequestedData(t *testing.T) {
	payload := bytes.Repeat([]byte("goed2k-upload-"), 16000)
	hash, err := protocol.HashFromData(payload)
	if err != nil {
		t.Fatalf("hash payload: %v", err)
	}

	seedPath := filepath.Join(t.TempDir(), "seed.bin")
	if err := os.WriteFile(seedPath, payload, 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	downloadPath := filepath.Join(t.TempDir(), "download.bin")

	seedSettings := NewSettings()
	seedSettings.ListenPort = reserveTCPPort(t)
	seedSession := NewSession(seedSettings)
	if err := seedSession.Listen(); err != nil {
		t.Fatalf("seed listen: %v", err)
	}
	defer seedSession.CloseListener()
	defer seedSession.DisconnectFrom()

	seedHandle, err := seedSession.AddTransferWithHandler(hash, int64(len(payload)), disk.NewDesktopFileHandler(seedPath))
	if err != nil {
		t.Fatalf("seed add transfer: %v", err)
	}
	seedHandle.transfer.WeHave(0)

	leechSession := NewSession(NewSettings())
	defer leechSession.CloseListener()
	defer leechSession.DisconnectFrom()

	leechHandle, err := leechSession.AddTransferWithHandler(hash, int64(len(payload)), disk.NewDesktopFileHandler(downloadPath))
	if err != nil {
		t.Fatalf("leech add transfer: %v", err)
	}
	endpoint, err := protocol.EndpointFromString("127.0.0.1", seedSettings.ListenPort)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if err := leechHandle.transfer.AddPeer(endpoint, int(PeerResume)); err != nil {
		t.Fatalf("add peer: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		UpdateCachedTime()
		seedSession.SecondTick(CurrentTime(), 100)
		leechSession.SecondTick(CurrentTime(), 100)
		if leechHandle.IsFinished() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	status := leechHandle.GetStatus()
	if !leechHandle.IsFinished() {
		t.Fatalf("expected downloaded payload, got done=%d peers=%d rate=%d", status.TotalDone, status.NumPeers, status.DownloadRate)
	}

	data, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatalf("read download file: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatal("uploaded payload mismatch")
	}
}
