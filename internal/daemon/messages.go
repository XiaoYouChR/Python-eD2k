package daemon

import "encoding/json"

const rpcVersion = 1

const (
	codeInternal         = "INTERNAL"
	codeInvalidLink      = "INVALID_LINK"
	codeInvalidRequest   = "INVALID_REQUEST"
	codeNotRunning       = "NOT_RUNNING"
	codeTransferExists   = "TRANSFER_EXISTS"
	codeTransferNotFound = "TRANSFER_NOT_FOUND"
)

type request struct {
	Version int             `json:"version"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	Version int      `json:"version"`
	ID      int      `json:"id"`
	Result  any      `json:"result,omitempty"`
	Error   *failure `json:"error,omitempty"`
}

type notification struct {
	Version int    `json:"version"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type commandError struct {
	code  string
	cause error
}

func (e *commandError) Error() string {
	return e.cause.Error()
}

type startParams struct {
	DataDir  string   `json:"dataDir"`
	Settings settings `json:"settings"`
}

type addLinkParams struct {
	Link      string `json:"link"`
	OutputDir string `json:"outputDir"`
}

type transferParams struct {
	Hash string `json:"hash"`
}

type removeParams struct {
	Hash       string `json:"hash"`
	DeleteFile bool   `json:"deleteFile"`
}

type settings struct {
	Servers           []string `json:"servers"`
	ServerMet         string   `json:"serverMet"`
	DHTNodes          []string `json:"dhtNodes"`
	NodesDat          string   `json:"nodesDat"`
	ListenPort        int      `json:"listenPort"`
	UDPPort           int      `json:"udpPort"`
	EnableDHT         bool     `json:"enableDht"`
	EnableUPnP        bool     `json:"enableUpnp"`
	ReconnectToServer bool     `json:"reconnectToServer"`
}

type snapshot struct {
	Transfers []transfer `json:"transfers"`
}

type transfer struct {
	Hash         string `json:"hash"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	State        string `json:"state"`
	Done         int64  `json:"done"`
	Received     int64  `json:"received"`
	DownloadRate int    `json:"downloadRate"`
	UploadRate   int    `json:"uploadRate"`
	Peers        int    `json:"peers"`
}
