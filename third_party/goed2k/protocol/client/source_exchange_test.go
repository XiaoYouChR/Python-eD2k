package client

import (
	"bytes"
	"testing"

	"github.com/monkeyWie/goed2k/protocol"
)

func TestSourceExchangeV3RoundTrip(t *testing.T) {
	want := SourceExchangeAnswer{
		Hash: protocol.EMule,
		Sources: []SourceExchangeSource{{
			ClientID:   0x01020304,
			ClientPort: 4662,
			ServerIP:   0x05060708,
			ServerPort: 4661,
			UserHash:   protocol.LibED2K,
		}},
	}
	var raw bytes.Buffer
	if err := want.Put(&raw); err != nil {
		t.Fatalf("put: %v", err)
	}
	var got SourceExchangeAnswer
	if err := got.Get(bytes.NewReader(raw.Bytes())); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.Hash.Equal(want.Hash) || len(got.Sources) != 1 || got.Sources[0] != want.Sources[0] {
		t.Fatalf("unexpected round trip: %+v", got)
	}
}

func TestSourceExchangeV3RejectsWrongEntrySize(t *testing.T) {
	var raw bytes.Buffer
	if err := protocol.WriteHash(&raw, protocol.EMule); err != nil {
		t.Fatal(err)
	}
	if err := protocol.WriteUInt16(&raw, 1); err != nil {
		t.Fatal(err)
	}
	raw.Write(make([]byte, sourceExchangeV3EntrySize-1))
	var answer SourceExchangeAnswer
	if err := answer.Get(bytes.NewReader(raw.Bytes())); err == nil {
		t.Fatal("expected malformed source entry to be rejected")
	}
}
