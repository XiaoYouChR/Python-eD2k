package goed2k

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestConnectionConnectDoesNotBlockSessionLoop(t *testing.T) {
	dialStarted := make(chan struct{})
	connection := NewConnection(NewSession(NewSettings()))
	connection.dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		close(dialStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	returned := make(chan error, 1)
	go func() {
		returned <- connection.Connect(&net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 4662})
	}()

	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Connect blocked while the dial was pending")
	}
	select {
	case <-dialStarted:
	case <-time.After(time.Second):
		t.Fatal("dial did not start")
	}
	if !connection.IsConnecting() {
		t.Fatal("expected connection to remain in the connecting state")
	}
	connection.Close(NoError)
}

func TestConnectionPollConnectInstallsSocket(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	connection := NewConnection(NewSession(NewSettings()))
	connection.dialContext = func(context.Context, string, string) (net.Conn, error) {
		return client, nil
	}

	if err := connection.Connect(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4662}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for connection.socket == nil && time.Now().Before(deadline) {
		if err := connection.PollConnect(); err != nil {
			t.Fatalf("poll connect: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	if connection.socket == nil {
		t.Fatal("expected PollConnect to install the connected socket")
	}
	connection.Close(NoError)
}

func TestConnectionDoReadDrainsMoreThanOneLegacyChunk(t *testing.T) {
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()

	connection := NewConnection(NewSession(NewSettings()))
	connection.socket = reader
	payload := make([]byte, 128*1024)
	writeDone := make(chan error, 1)
	go func() {
		_, err := writer.Write(payload)
		writeDone <- err
	}()

	if err := connection.DoRead(); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := connection.IncomingBytes(); got <= 16*1024 {
		t.Fatalf("expected one tick to drain more than 16 KiB, got %d", got)
	}
	_ = reader.Close()
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("writer remained blocked after the test socket closed")
	}
}
