package goed2k

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"

	"github.com/monkeyWie/goed2k/data"
	"github.com/monkeyWie/goed2k/disk"
)

func TestUploadReadDoesNotMoveConcurrentBlockWrite(t *testing.T) {
	handler := disk.NewDesktopFileHandler(filepath.Join(t.TempDir(), "shared.bin"))
	defer handler.Close()
	written := bytes.Repeat([]byte{0xAA}, BlockSizeInt)
	served := bytes.Repeat([]byte{0x55}, BlockSizeInt)
	if _, err := handler.File().WriteAt(append(bytes.Repeat([]byte{0}, BlockSizeInt), served...), 0); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 300 {
			pm := NewPieceManager(handler, 1, BlocksPerPiece)
			if _, err := pm.WriteBlock(data.NewPieceBlock(0, 0), written); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		pm := NewPieceManager(handler, 1, BlocksPerPiece)
		for range 300 {
			got, err := pm.ReadRange(BlockSize, 2*BlockSize)
			if err != nil {
				t.Error(err)
				return
			}
			if !bytes.Equal(got, served) {
				t.Error("upload read returned bytes from a concurrent write")
				return
			}
		}
	}()
	wg.Wait()

	pm := NewPieceManager(handler, 1, BlocksPerPiece)
	got, err := pm.ReadRange(0, 2*BlockSize)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:BlockSizeInt], written) || !bytes.Equal(got[BlockSizeInt:], served) {
		t.Fatal("block write landed at the upload read offset")
	}
}
