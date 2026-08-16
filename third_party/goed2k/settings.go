package goed2k

import (
	"fmt"
	"log/slog"

	"github.com/monkeyWie/goed2k/protocol"
)

type Settings struct {
	Logger                  *slog.Logger
	UserAgent               protocol.Hash
	ModName                 string
	ClientName              string
	ListenPort              int
	UDPPort                 int
	EnableDHT               bool
	EnableUPnP              bool
	Version                 int
	ModMajor                int
	ModMinor                int
	ModBuild                int
	MaxFailCount            int
	MaxPeerListSize         int
	MinPeerReconnectTime    int
	PeerConnectionTimeout   int
	SessionConnectionsLimit int
	UploadSlots             int
	MaxUploadRateKB         int
	SlotAllocationKB        int
	UploadQueueSize         int
	BufferPoolSize          int
	MaxConnectionsPerSecond int
	CompressionVersion      int
	ServerSearchTimeout     int
	DHTSearchTimeout        int
	ReconnectToServer       bool
	ServerPingTimeout       int64
}

func NewSettings() Settings {
	userAgent := protocol.EMule
	if randomUserAgent, err := protocol.RandomHash(true); err == nil {
		userAgent = randomUserAgent
	}
	return Settings{
		UserAgent:               userAgent,
		ModName:                 "jed2k",
		ClientName:              "jed2k",
		ListenPort:              4661,
		UDPPort:                 4662,
		EnableDHT:               false,
		EnableUPnP:              false,
		Version:                 0x3c,
		ModMajor:                0,
		ModMinor:                0,
		ModBuild:                0,
		MaxFailCount:            20,
		MaxPeerListSize:         100,
		MinPeerReconnectTime:    10,
		PeerConnectionTimeout:   5,
		SessionConnectionsLimit: 20,
		UploadSlots:             3,
		MaxUploadRateKB:         0,
		SlotAllocationKB:        3,
		UploadQueueSize:         500,
		BufferPoolSize:          250,
		MaxConnectionsPerSecond: 10,
		CompressionVersion:      0,
		ServerSearchTimeout:     15,
		DHTSearchTimeout:        8,
		ReconnectToServer:       false,
		ServerPingTimeout:       0,
	}
}

func (s Settings) String() string {
	return fmt.Sprintf("Settings{userAgent=%s, modName='%s', clientName='%s', listenPort=%d, udpPort=%d, enableDHT=%t, enableUPnP=%t, version=%d, modMajor=%d, modMinor=%d, modBuild=%d, maxFailCount=%d, maxPeerListSize=%d, minPeerReconnectTime=%d, peerConnectionTimeout=%d, sessionConnectionsLimit=%d, uploadSlots=%d, maxUploadRateKB=%d, slotAllocationKB=%d, uploadQueueSize=%d, bufferPoolSize=%d, maxConnectionsPerSecond=%d, compressionVersion=%d, serverSearchTimeout=%d, dhtSearchTimeout=%d, serverPingTimeout=%d, reconnectToServer=%t}",
		s.UserAgent.String(), s.ModName, s.ClientName, s.ListenPort, s.UDPPort, s.EnableDHT, s.EnableUPnP, s.Version, s.ModMajor, s.ModMinor, s.ModBuild, s.MaxFailCount, s.MaxPeerListSize, s.MinPeerReconnectTime, s.PeerConnectionTimeout, s.SessionConnectionsLimit, s.UploadSlots, s.MaxUploadRateKB, s.SlotAllocationKB, s.UploadQueueSize, s.BufferPoolSize, s.MaxConnectionsPerSecond, s.CompressionVersion, s.ServerSearchTimeout, s.DHTSearchTimeout, s.ServerPingTimeout, s.ReconnectToServer)
}
