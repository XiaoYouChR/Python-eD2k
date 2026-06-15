package daemon_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/XiaoYouChR/Python-eD2k/internal/daemon"
)

func TestRunReportsStableErrorCode(t *testing.T) {
	input := bytes.NewBufferString(
		"{\"version\":1,\"id\":1,\"method\":\"snapshot\",\"params\":{}}\n" +
			"{\"version\":1,\"id\":2,\"method\":\"close\",\"params\":{}}\n",
	)
	var output bytes.Buffer

	if err := daemon.New(input, &output).Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var response struct {
		ID    int `json:"id"`
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(&output).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 1 || response.Error == nil || response.Error.Code != "NOT_RUNNING" {
		t.Fatalf("response = %+v", response)
	}
}
