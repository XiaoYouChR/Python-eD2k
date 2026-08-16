package goed2k

import (
	"bytes"
	"net"
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
	serverproto "github.com/monkeyWie/goed2k/protocol/server"
)

func TestServerSearchRequestEncodesQueryAndFilters(t *testing.T) {
	packet := serverproto.SearchRequest{
		Query:      "shake it off",
		MinSize:    1024,
		FileType:   "Audio",
		Extension:  "mp3",
		MinSources: 5,
	}
	var buf bytes.Buffer
	if err := packet.Put(&buf); err != nil {
		t.Fatalf("put search request: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty encoded search request")
	}
}

func TestSessionOnServerSearchResultAggregatesResults(t *testing.T) {
	session := NewSession(NewSettings())
	addr := &net.TCPAddr{IP: net.IPv4(45, 82, 80, 155), Port: 5687}
	server := NewServerConnection("a", addr, session)
	server.handshakeCompleted = true
	session.serverConnection = server
	session.serverConnections["a"] = server

	handle, err := session.StartSearch(SearchParams{Query: "shake it off", Scope: SearchScopeServer})
	if err != nil {
		t.Fatalf("start search: %v", err)
	}

	entry := serverproto.SharedFileEntry{
		Hash: protocol.MustHashFromString("CFB72F36AE2B939C787EA9F64A9506B1"),
		Tags: protocol.TagList{
			protocol.NewStringTag(protocol.FTFilename, "Taylor Swift - Shake It Off.mp3"),
			protocol.NewUInt32Tag(protocol.FTFileSize, 8787318),
			protocol.NewUInt32Tag(protocol.FTSources, 12),
		},
	}

	session.OnServerSearchResult(server, &serverproto.SearchResult{Results: []serverproto.SharedFileEntry{entry}})
	snapshot := handle.Snapshot()
	if len(snapshot.Results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(snapshot.Results))
	}
	if snapshot.Results[0].FileName != "Taylor Swift - Shake It Off.mp3" {
		t.Fatalf("unexpected filename %q", snapshot.Results[0].FileName)
	}
	if snapshot.Results[0].Sources != 12 {
		t.Fatalf("unexpected sources %d", snapshot.Results[0].Sources)
	}
}
