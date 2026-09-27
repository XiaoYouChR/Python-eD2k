package daemon_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/XiaoYouChR/Python-eD2k/internal/daemon"
)

func addLinkTo(t *testing.T, outputDir string) (errorCode string) {
	t.Helper()
	link := "ed2k://|file|a.bin|4|0123456789ABCDEF0123456789ABCDEF|/"
	start, _ := json.Marshal(map[string]any{
		"dataDir":  t.TempDir(),
		"settings": map[string]any{"listenPort": 0, "enableDht": false},
	})
	add, _ := json.Marshal(map[string]any{"link": link, "outputDir": outputDir})
	input := bytes.NewBufferString(fmt.Sprintf(
		"{\"version\":1,\"id\":1,\"method\":\"start\",\"params\":%s}\n"+
			"{\"version\":1,\"id\":2,\"method\":\"addLink\",\"params\":%s}\n"+
			"{\"version\":1,\"id\":3,\"method\":\"close\",\"params\":{}}\n", start, add))
	var output bytes.Buffer
	if err := daemon.New(input, &output).Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decoder := json.NewDecoder(&output)
	for decoder.More() {
		var response struct {
			ID    int `json:"id"`
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.ID == 2 {
			if response.Error == nil {
				return ""
			}
			return response.Error.Code
		}
	}
	t.Fatal("no addLink response")
	return ""
}

func TestAddLinkAdoptsEmptyPlaceholder(t *testing.T) {
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "a.bin"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := addLinkTo(t, outputDir); code != "" {
		t.Fatalf("addLink error = %s, want adopted", code)
	}
}

func TestAddLinkRefusesExistingFile(t *testing.T) {
	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "a.bin"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := addLinkTo(t, outputDir); code != "OUTPUT_EXISTS" {
		t.Fatalf("addLink error = %q, want OUTPUT_EXISTS", code)
	}
}
