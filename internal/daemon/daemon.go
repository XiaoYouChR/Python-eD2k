package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	goed2k "github.com/monkeyWie/goed2k"
	"github.com/monkeyWie/goed2k/protocol"
)

type Daemon struct {
	input        io.Reader
	encoder      *json.Encoder
	writeMu      sync.Mutex
	client       *goed2k.Client
	statusCancel func()
	statusDone   chan struct{}
}

func New(input io.Reader, output io.Writer) *Daemon {
	return &Daemon{input: input, encoder: json.NewEncoder(output)}
}

func (d *Daemon) Run() (runErr error) {
	defer func() {
		if err := d.close(); runErr == nil {
			runErr = err
		}
	}()
	scanner := bufio.NewScanner(d.input)
	for scanner.Scan() {
		var incoming request
		if err := json.Unmarshal(scanner.Bytes(), &incoming); err != nil {
			return fmt.Errorf("decode request: %w", err)
		}
		result, shouldClose, err := d.runRequest(incoming)
		response := response{Version: rpcVersion, ID: incoming.ID, Result: result}
		if err != nil {
			response.Result = nil
			response.Error = toFailure(err)
		}
		if err := d.write(response); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
		if incoming.Method == "start" && err == nil {
			d.subscribeStatus()
		}
		if shouldClose {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	return nil
}

func (d *Daemon) runRequest(request request) (any, bool, error) {
	if request.Version != rpcVersion {
		return nil, false, fail(codeInvalidRequest, fmt.Errorf("unsupported RPC version %d", request.Version))
	}
	switch request.Method {
	case "start":
		result, err := d.start(request.Params)
		return result, false, err
	case "snapshot":
		result, err := d.snapshot()
		return result, false, err
	case "addLink":
		result, err := d.addLink(request.Params)
		return result, false, err
	case "pause":
		result, err := d.pause(request.Params)
		return result, false, err
	case "resume":
		result, err := d.resume(request.Params)
		return result, false, err
	case "remove":
		return nil, false, d.remove(request.Params)
	case "close":
		return nil, true, d.close()
	default:
		return nil, false, fail(codeInvalidRequest, fmt.Errorf("unknown method %q", request.Method))
	}
}

func (d *Daemon) start(raw json.RawMessage) (snapshot, error) {
	if d.client != nil {
		return snapshot{}, fail(codeInvalidRequest, errors.New("client is already running"))
	}
	var params startParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return snapshot{}, fmt.Errorf("decode start params: %w", err)
	}
	if params.DataDir == "" {
		return snapshot{}, fail(codeInvalidRequest, errors.New("dataDir is required"))
	}
	if err := os.MkdirAll(params.DataDir, 0o755); err != nil {
		return snapshot{}, fmt.Errorf("create dataDir: %w", err)
	}

	config := goed2k.NewSettings()
	config.ListenPort = params.Settings.ListenPort
	config.UDPPort = params.Settings.UDPPort
	config.EnableDHT = params.Settings.EnableDHT
	config.EnableUPnP = params.Settings.EnableUPnP
	config.ReconnectToServer = params.Settings.ReconnectToServer

	client := goed2k.NewClient(config)
	statePath := filepath.Join(params.DataDir, "state.json")
	client.SetStatePath(statePath)
	if err := client.LoadState(""); err != nil {
		return snapshot{}, fmt.Errorf("load state: %w", err)
	}
	if err := client.Start(); err != nil {
		return snapshot{}, fmt.Errorf("start client: %w", err)
	}
	d.client = client
	if err := d.bootstrap(params.Settings); err != nil {
		_ = d.close()
		return snapshot{}, err
	}
	return d.snapshot()
}

func (d *Daemon) bootstrap(config settings) error {
	if config.ServerMetSource != "" {
		if err := d.client.ConnectServerMet(config.ServerMetSource); err != nil {
			return fmt.Errorf("connect server.met: %w", err)
		}
	}
	if len(config.Servers) > 0 {
		if err := d.client.ConnectServers(config.Servers...); err != nil {
			return fmt.Errorf("connect servers: %w", err)
		}
	}
	if config.NodesDatSource != "" {
		if err := d.client.LoadDHTNodesDat(config.NodesDatSource); err != nil {
			return fmt.Errorf("load nodes.dat: %w", err)
		}
	}
	if len(config.DHTNodes) > 0 {
		if err := d.client.AddDHTBootstrapNodes(config.DHTNodes...); err != nil {
			return fmt.Errorf("add DHT nodes: %w", err)
		}
	}
	return nil
}

func (d *Daemon) addLink(raw json.RawMessage) (transfer, error) {
	if d.client == nil {
		return transfer{}, fail(codeNotRunning, errors.New("client is not running"))
	}
	var params addLinkParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return transfer{}, fmt.Errorf("decode addLink params: %w", err)
	}
	link, err := goed2k.ParseEMuleLink(params.Link)
	if err != nil {
		return transfer{}, fail(codeInvalidLink, err)
	}
	if link.Type != goed2k.LinkFile {
		return transfer{}, fail(codeInvalidLink, errors.New("unsupported link type"))
	}
	outputDir, err := filepath.Abs(params.OutputDir)
	if err != nil {
		return transfer{}, fmt.Errorf("resolve outputDir: %w", err)
	}
	targetPath := filepath.Join(outputDir, link.StringValue)
	relativePath, err := filepath.Rel(outputDir, targetPath)
	if err != nil {
		return transfer{}, fmt.Errorf("resolve target path: %w", err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return transfer{}, fail(codeInvalidLink, errors.New("file name escapes outputDir"))
	}
	if d.client.FindTransfer(link.Hash).IsValid() {
		return transfer{}, fail(codeTransferExists, errors.New("transfer already exists"))
	}
	for _, item := range d.client.TransferSnapshots() {
		existingPath, err := filepath.Abs(item.FilePath)
		if err != nil {
			return transfer{}, fmt.Errorf("resolve existing transfer path: %w", err)
		}
		conflicts := existingPath == targetPath
		if runtime.GOOS == "windows" {
			conflicts = strings.EqualFold(existingPath, targetPath)
		}
		if conflicts {
			return transfer{}, fail(codeTransferExists, errors.New("output path already in use"))
		}
	}
	if _, _, err := d.client.AddLink(params.Link, outputDir); err != nil {
		return transfer{}, err
	}
	return d.transfer(link.Hash)
}

func (d *Daemon) pause(raw json.RawMessage) (transfer, error) {
	if d.client == nil {
		return transfer{}, fail(codeNotRunning, errors.New("client is not running"))
	}
	var params transferParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return transfer{}, fmt.Errorf("decode pause params: %w", err)
	}
	hash, err := protocol.HashFromString(params.Hash)
	if err != nil {
		return transfer{}, fail(codeInvalidRequest, err)
	}
	if !d.client.FindTransfer(hash).IsValid() {
		return transfer{}, fail(codeTransferNotFound, errors.New("transfer not found"))
	}
	if err := d.client.PauseTransfer(hash); err != nil {
		return transfer{}, err
	}
	return d.transfer(hash)
}

func (d *Daemon) resume(raw json.RawMessage) (transfer, error) {
	if d.client == nil {
		return transfer{}, fail(codeNotRunning, errors.New("client is not running"))
	}
	var params transferParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return transfer{}, fmt.Errorf("decode resume params: %w", err)
	}
	hash, err := protocol.HashFromString(params.Hash)
	if err != nil {
		return transfer{}, fail(codeInvalidRequest, err)
	}
	if !d.client.FindTransfer(hash).IsValid() {
		return transfer{}, fail(codeTransferNotFound, errors.New("transfer not found"))
	}
	if err := d.client.ResumeTransfer(hash); err != nil {
		return transfer{}, err
	}
	return d.transfer(hash)
}

func (d *Daemon) remove(raw json.RawMessage) error {
	if d.client == nil {
		return fail(codeNotRunning, errors.New("client is not running"))
	}
	var params removeParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return fmt.Errorf("decode remove params: %w", err)
	}
	hash, err := protocol.HashFromString(params.Hash)
	if err != nil {
		return fail(codeInvalidRequest, err)
	}
	if !d.client.FindTransfer(hash).IsValid() {
		return fail(codeTransferNotFound, errors.New("transfer not found"))
	}
	return d.client.RemoveTransfer(hash, params.DeleteFile)
}

func (d *Daemon) snapshot() (snapshot, error) {
	if d.client == nil {
		return snapshot{}, fail(codeNotRunning, errors.New("client is not running"))
	}
	return toSnapshot(d.client.TransferSnapshots()), nil
}

func (d *Daemon) transfer(hash protocol.Hash) (transfer, error) {
	for _, item := range d.client.TransferSnapshots() {
		if item.Hash.Equal(hash) {
			return toTransfer(item), nil
		}
	}
	return transfer{}, errors.New("transfer not found")
}

func (d *Daemon) close() error {
	if d.client == nil {
		return nil
	}
	d.stopStatus()
	err := d.client.Stop()
	d.client = nil
	return err
}

func toTransfer(item goed2k.TransferSnapshot) transfer {
	return transfer{
		Hash:         item.Hash.String(),
		Name:         item.FileName,
		Path:         item.FilePath,
		Size:         item.Size,
		State:        string(item.Status.State),
		Done:         item.Status.TotalDone,
		Received:     item.Status.TotalReceived,
		DownloadRate: item.Status.DownloadRate,
		UploadRate:   item.Status.UploadRate,
		Peers:        item.Status.NumPeers,
	}
}

func toSnapshot(items []goed2k.TransferSnapshot) snapshot {
	current := snapshot{Transfers: make([]transfer, 0, len(items))}
	for _, item := range items {
		current.Transfers = append(current.Transfers, toTransfer(item))
	}
	return current
}

func (d *Daemon) subscribeStatus() {
	events, cancel := d.client.SubscribeStatus()
	d.statusCancel = cancel
	d.statusDone = make(chan struct{})
	go func() {
		defer close(d.statusDone)
		for event := range events {
			if err := d.write(notification{
				Version: rpcVersion,
				Method:  "snapshot",
				Params:  toSnapshot(event.TransferSnapshots()),
			}); err != nil {
				return
			}
		}
	}()
}

func (d *Daemon) stopStatus() {
	if d.statusCancel == nil {
		return
	}
	d.statusCancel()
	<-d.statusDone
	d.statusCancel = nil
	d.statusDone = nil
}

func (d *Daemon) write(value any) error {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	return d.encoder.Encode(value)
}

func fail(code string, cause error) error {
	return &commandError{code: code, cause: cause}
}

func toFailure(err error) *failure {
	var command *commandError
	if errors.As(err, &command) {
		return &failure{Code: command.code, Message: command.Error()}
	}
	return &failure{Code: codeInternal, Message: err.Error()}
}
