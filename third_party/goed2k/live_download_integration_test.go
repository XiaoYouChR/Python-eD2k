package goed2k

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	liveTestServer = "45.82.80.155:5687"
	liveTestLink   = "ed2k://|file|Microsoft%20Office%20Professional%20Plus%20Vl%202019%20-%201810%20-%20Ita%20Funziona%20Anche%20Su%20Windows%207,%208.1%20&%20Server%202012%20(16%20Novembre%202018)%20By%20Grisu.rar|3748437185|E90D507F17B677F168F584A2E5C93E0D|/"
	liveSmallLink  = "ed2k://|file|Taylor,%20Elizabeth%20-%20Prohibido%20morir%20aqui%20[66672]%20(r1.0).epub|434885|23A8CEFF57A7A32D562D649ED7893796|/"
	liveSmallMD5   = "edc6bfb876f88feba57d388c66729ef3"
)

var liveFallbackPeers = []string{
	"95.246.116.103:4662",
	"185.219.47.156:11509",
	"151.48.55.193:15000",
	"87.13.47.25:39521",
}

var liveSmallFallbackPeers = []string{
	"87.125.240.91:61115",
	"92.59.255.85:14340",
	"62.99.0.188:4662",
}

func TestLiveDownloadReceivesPayload(t *testing.T) {
	link, err := ParseEMuleLink(liveTestLink)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, link.StringValue)
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open target file: %v", err)
	}
	defer file.Close()

	settings := NewSettings()
	settings.PeerConnectionTimeout = 30
	settings.ReconnectToServer = true
	session := NewSession(settings)
	handle, err := session.AddTransfer(link.Hash, link.NumberValue, file)
	if err != nil {
		t.Fatalf("add transfer: %v", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", liveTestServer)
	if err != nil {
		t.Fatalf("resolve server: %v", err)
	}
	if err := session.ConnectTo(liveTestServer, addr); err != nil {
		t.Fatalf("connect server: %v", err)
	}

	deadline := time.Now().Add(90 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	startedAt := time.Now()
	lastTick := time.Now()
	lastReconnect := time.Now()
	lastSourceRequest := time.Now()
	fallbackInjected := false

	var lastStatus TransferStatus
	for time.Now().Before(deadline) {
		<-tick.C
		now := time.Now()
		UpdateCachedTime()
		session.SecondTick(CurrentTime(), now.Sub(lastTick).Milliseconds())
		lastTick = now
		lastStatus = handle.GetStatus()
		if lastStatus.NumPeers == 0 &&
			session.serverConnection != nil &&
			session.serverConnection.IsHandshakeCompleted() &&
			now.Sub(lastSourceRequest) >= 3*time.Second {
			session.SendSourcesRequest(link.Hash, link.NumberValue)
			lastSourceRequest = now
		}
		if lastStatus.NumPeers == 0 && !fallbackInjected && now.Sub(startedAt) >= 10*time.Second {
			t.Log("inject fallback peers for live download validation")
			transfer := handle.transfer
			if transfer != nil {
				for _, addrText := range liveFallbackPeers {
					addr, err := net.ResolveTCPAddr("tcp", addrText)
					if err != nil {
						t.Fatalf("resolve fallback peer %s: %v", addrText, err)
					}
					_ = transfer.AddPeer(protocol.EndpointFromInet(addr), int(PeerServer))
				}
			}
			fallbackInjected = true
		}
		if lastStatus.NumPeers == 0 && now.Sub(lastReconnect) >= 8*time.Second {
			t.Log("reconnect server to retry source discovery")
			session.DisconnectFrom()
			if err := session.ConnectTo(liveTestServer, addr); err != nil {
				t.Fatalf("reconnect server: %v", err)
			}
			lastReconnect = now
			lastSourceRequest = now
		}
		if lastStatus.TotalDone > 0 {
			t.Logf("received payload: done=%d peers=%d rate=%d", lastStatus.TotalDone, lastStatus.NumPeers, lastStatus.DownloadRate)
			return
		}
	}

	t.Fatalf("expected non-zero downloaded payload, got done=%d peers=%d rate=%d recv=%d",
		lastStatus.TotalDone,
		lastStatus.NumPeers,
		lastStatus.DownloadRate,
		lastStatus.TotalReceived)
}

func TestLiveDownloadCompletesAndMatchesMD5(t *testing.T) {
	link, err := ParseEMuleLink(liveSmallLink)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, link.StringValue)
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open target file: %v", err)
	}
	defer file.Close()

	settings := NewSettings()
	settings.PeerConnectionTimeout = 30
	settings.ReconnectToServer = true
	session := NewSession(settings)
	handle, err := session.AddTransfer(link.Hash, link.NumberValue, file)
	if err != nil {
		t.Fatalf("add transfer: %v", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", liveTestServer)
	if err != nil {
		t.Fatalf("resolve server: %v", err)
	}
	if err := session.ConnectTo(liveTestServer, addr); err != nil {
		t.Fatalf("connect server: %v", err)
	}

	deadline := time.Now().Add(10 * time.Minute)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	startedAt := time.Now()
	lastTick := time.Now()
	lastReconnect := time.Now()
	lastSourceRequest := time.Now()
	fallbackInjected := false
	var lastStatus TransferStatus

	for time.Now().Before(deadline) {
		<-tick.C
		now := time.Now()
		UpdateCachedTime()
		session.SecondTick(CurrentTime(), now.Sub(lastTick).Milliseconds())
		lastTick = now
		lastStatus = handle.GetStatus()

		if transfer := handle.transfer; transfer != nil && transfer.NeedMoreSources() &&
			session.serverConnection != nil &&
			session.serverConnection.IsHandshakeCompleted() &&
			now.Sub(lastSourceRequest) >= 3*time.Second {
			session.SendSourcesRequest(link.Hash, link.NumberValue)
			lastSourceRequest = now
		}

		if lastStatus.NumPeers == 0 && !fallbackInjected && now.Sub(startedAt) >= 10*time.Second {
			t.Log("inject fallback peers for complete download validation")
			if transfer := handle.transfer; transfer != nil {
				for _, addrText := range liveSmallFallbackPeers {
					peerAddr, err := net.ResolveTCPAddr("tcp", addrText)
					if err != nil {
						t.Fatalf("resolve fallback peer %s: %v", addrText, err)
					}
					_ = transfer.AddPeer(protocol.EndpointFromInet(peerAddr), int(PeerServer))
				}
			}
			fallbackInjected = true
		}

		if lastStatus.NumPeers == 0 && now.Sub(lastReconnect) >= 8*time.Second {
			t.Log("reconnect server to retry source discovery")
			session.DisconnectFrom()
			if err := session.ConnectTo(liveTestServer, addr); err != nil {
				t.Fatalf("reconnect server: %v", err)
			}
			lastReconnect = now
			lastSourceRequest = now
		}

		if handle.IsFinished() {
			if err := file.Sync(); err != nil {
				t.Fatalf("sync file: %v", err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatalf("seek file: %v", err)
			}
			sum := md5.New()
			if _, err := io.Copy(sum, file); err != nil {
				t.Fatalf("hash file: %v", err)
			}
			actualMD5 := hex.EncodeToString(sum.Sum(nil))
			if actualMD5 != liveSmallMD5 {
				t.Fatalf("md5 mismatch: got %s want %s", actualMD5, liveSmallMD5)
			}
			t.Logf("download completed: size=%d md5=%s", lastStatus.TotalDone, actualMD5)
			return
		}
	}

	t.Fatalf("expected completed download, got done=%d peers=%d active=%d rate=%d",
		lastStatus.TotalDone,
		lastStatus.NumPeers,
		handle.ActiveConnections(),
		lastStatus.DownloadRate)
}
