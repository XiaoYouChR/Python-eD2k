package goed2k

import (
	"strings"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	PeerIncoming byte = 0x1
	PeerServer   byte = 0x2
	PeerDHT      byte = 0x4
	PeerResume   byte = 0x8
)

type PeerInfo struct {
	DownloadSpeed        int
	PayloadDownloadSpeed int
	UploadSpeed          int
	PayloadUploadSpeed   int
	RemotePieces         protocol.BitField
	FailCount            int
	Endpoint             protocol.Endpoint
	ModName              string
	Version              int
	ModVersion           int
	StrModVersion        string
	SourceFlag           int
}

func PeerSourceLabels(sourceFlag int) []string {
	labels := make([]string, 0, 4)
	if (sourceFlag & int(PeerServer)) != 0 {
		labels = append(labels, "server")
	}
	if (sourceFlag & int(PeerDHT)) != 0 {
		labels = append(labels, "kad")
	}
	if (sourceFlag & int(PeerResume)) != 0 {
		labels = append(labels, "resume")
	}
	if (sourceFlag & int(PeerIncoming)) != 0 {
		labels = append(labels, "incoming")
	}
	if len(labels) == 0 {
		labels = append(labels, "unknown")
	}
	return labels
}

func (p PeerInfo) SourceLabels() []string {
	return PeerSourceLabels(p.SourceFlag)
}

func (p PeerInfo) SourceString() string {
	return strings.Join(p.SourceLabels(), "|")
}

func (p PeerInfo) HasSource(source byte) bool {
	return (p.SourceFlag & int(source)) != 0
}
