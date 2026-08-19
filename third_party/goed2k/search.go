package goed2k

import (
	"sort"
	"strings"
	"sync"

	"github.com/monkeyWie/goed2k/protocol"
	kadproto "github.com/monkeyWie/goed2k/protocol/kad"
	serverproto "github.com/monkeyWie/goed2k/protocol/server"
)

type SearchScope uint8

const (
	SearchScopeServer SearchScope = 1 << iota
	SearchScopeDHT
	SearchScopeAll = SearchScopeServer | SearchScopeDHT
)

type SearchState string

const (
	SearchStateIdle     SearchState = "IDLE"
	SearchStateRunning  SearchState = "RUNNING"
	SearchStateFinished SearchState = "FINISHED"
	SearchStateStopped  SearchState = "STOPPED"
	SearchStateFailed   SearchState = "FAILED"
)

type SearchParams struct {
	Query              string
	Scope              SearchScope
	MinSize            int64
	MaxSize            int64
	MinSources         int
	MinCompleteSources int
	FileType           string
	Extension          string
}

type SearchResultSource uint8

const (
	SearchResultServer SearchResultSource = 1 << iota
	SearchResultKAD
)

type SearchResult struct {
	Hash            protocol.Hash
	FileName        string
	FileSize        int64
	Sources         int
	CompleteSources int
	MediaBitrate    int
	MediaLength     int
	MediaCodec      string
	Extension       string
	FileType        string
	Source          SearchResultSource
}

func (r SearchResult) ED2KLink() string {
	if r.FileName == "" || r.FileSize <= 0 || r.Hash == protocol.Invalid {
		return ""
	}
	return FormatLink(r.FileName, r.FileSize, r.Hash)
}

type SearchSnapshot struct {
	ID         uint32
	Params     SearchParams
	State      SearchState
	Results    []SearchResult
	UpdatedAt  int64
	StartedAt  int64
	ServerBusy bool
	DHTBusy    bool
	KadKeyword string
	Error      string
}

type SearchHandle struct {
	session *Session
	id      uint32
}

func (h SearchHandle) ID() uint32 {
	return h.id
}

func (h SearchHandle) IsValid() bool {
	return h.session != nil && h.id != 0
}

func (h SearchHandle) Snapshot() SearchSnapshot {
	if h.session == nil {
		return SearchSnapshot{}
	}
	return h.session.SearchSnapshot()
}

func (h SearchHandle) Stop() error {
	if h.session == nil {
		return nil
	}
	return h.session.StopSearch(h.id)
}

type searchTask struct {
	mu         sync.Mutex
	id         uint32
	params     SearchParams
	state      SearchState
	results    map[string]SearchResult
	startedAt  int64
	updatedAt  int64
	deadlineAt int64
	serverBusy bool
	dhtBusy    bool
	kadKeyword string
	errText    string
}

func newSearchTask(id uint32, params SearchParams, startedAt int64) *searchTask {
	return &searchTask{
		id:        id,
		params:    params,
		state:     SearchStateRunning,
		results:   make(map[string]SearchResult),
		startedAt: startedAt,
		updatedAt: startedAt,
	}
}

func (s *searchTask) snapshot() SearchSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := make([]SearchResult, 0, len(s.results))
	for _, result := range s.results {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Sources != results[j].Sources {
			return results[i].Sources > results[j].Sources
		}
		if results[i].CompleteSources != results[j].CompleteSources {
			return results[i].CompleteSources > results[j].CompleteSources
		}
		if results[i].FileSize != results[j].FileSize {
			return results[i].FileSize > results[j].FileSize
		}
		return results[i].FileName < results[j].FileName
	})
	return SearchSnapshot{
		ID:         s.id,
		Params:     s.params,
		State:      s.state,
		Results:    results,
		UpdatedAt:  s.updatedAt,
		StartedAt:  s.startedAt,
		ServerBusy: s.serverBusy,
		DHTBusy:    s.dhtBusy,
		KadKeyword: s.kadKeyword,
		Error:      s.errText,
	}
}

func (s *searchTask) setDeadline(deadline int64) {
	s.mu.Lock()
	if deadline > s.deadlineAt {
		s.deadlineAt = deadline
	}
	s.updatedAt = CurrentTime()
	s.mu.Unlock()
}

func (s *searchTask) finishServer() {
	s.mu.Lock()
	s.serverBusy = false
	s.finishLocked()
	s.mu.Unlock()
}

func (s *searchTask) finishDHT() {
	s.mu.Lock()
	s.dhtBusy = false
	s.finishLocked()
	s.mu.Unlock()
}

func (s *searchTask) stop() {
	s.mu.Lock()
	s.serverBusy = false
	s.dhtBusy = false
	s.state = SearchStateStopped
	s.updatedAt = CurrentTime()
	s.mu.Unlock()
}

func (s *searchTask) fail(err error) {
	s.mu.Lock()
	s.serverBusy = false
	s.dhtBusy = false
	s.state = SearchStateFailed
	if err != nil {
		s.errText = err.Error()
	}
	s.updatedAt = CurrentTime()
	s.mu.Unlock()
}

func (s *searchTask) onTick(now int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SearchStateRunning {
		return
	}
	if s.serverBusy && s.deadlineAt > 0 && now >= s.deadlineAt {
		s.serverBusy = false
	}
	s.finishLocked()
}

func (s *searchTask) finishLocked() {
	if s.state != SearchStateRunning {
		return
	}
	if !s.serverBusy && !s.dhtBusy {
		s.state = SearchStateFinished
	}
	s.updatedAt = CurrentTime()
}

func (s *searchTask) mergeResult(result SearchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := result.Hash.String()
	if existing, ok := s.results[key]; ok {
		if existing.FileName == "" {
			existing.FileName = result.FileName
		}
		if existing.FileSize == 0 {
			existing.FileSize = result.FileSize
		}
		if result.Sources > existing.Sources {
			existing.Sources = result.Sources
		}
		if result.CompleteSources > existing.CompleteSources {
			existing.CompleteSources = result.CompleteSources
		}
		if existing.MediaBitrate == 0 {
			existing.MediaBitrate = result.MediaBitrate
		}
		if existing.MediaLength == 0 {
			existing.MediaLength = result.MediaLength
		}
		if existing.MediaCodec == "" {
			existing.MediaCodec = result.MediaCodec
		}
		if existing.Extension == "" {
			existing.Extension = result.Extension
		}
		if existing.FileType == "" {
			existing.FileType = result.FileType
		}
		existing.Source |= result.Source
		s.results[key] = existing
	} else {
		s.results[key] = result
	}
	s.updatedAt = CurrentTime()
}

func makeSearchResultFromServer(entry serverproto.SharedFileEntry) SearchResult {
	result := SearchResult{
		Hash:   entry.Hash,
		Source: SearchResultServer,
	}
	if name, ok := entry.StringTag(protocol.FTFilename); ok {
		result.FileName = name
	}
	if size, ok := entry.UIntTag(protocol.FTFileSize); ok {
		result.FileSize = int64(size)
	}
	if hi, ok := entry.UIntTag(protocol.FTFileSizeHi); ok {
		result.FileSize += int64(hi << 32)
	}
	if sources, ok := entry.UIntTag(protocol.FTSources); ok {
		result.Sources = int(sources)
	}
	if complete, ok := entry.UIntTag(protocol.FTCompleteSources); ok {
		result.CompleteSources = int(complete)
	}
	if bitrate, ok := entry.UIntTag(protocol.FTMediaBitrate); ok {
		result.MediaBitrate = int(bitrate)
	}
	if length, ok := entry.UIntTag(protocol.FTMediaLength); ok {
		result.MediaLength = int(length)
	}
	if codec, ok := entry.StringTag(protocol.FTMediaCodec); ok {
		result.MediaCodec = codec
	}
	if ext, ok := entry.StringTag(protocol.FTFileFormat); ok {
		result.Extension = ext
	}
	if fileType, ok := entry.StringTag(protocol.FTFileType); ok {
		result.FileType = fileType
	}
	return result
}

func makeSearchResultFromKAD(entry kadproto.SearchEntry) SearchResult {
	result := SearchResult{
		Hash:   entry.ID.Hash,
		Source: SearchResultKAD,
	}
	if name, ok := entry.StringTag(protocol.FTFilename); ok {
		result.FileName = name
	}
	if size, ok := entry.UIntTag(protocol.FTFileSize); ok {
		result.FileSize = int64(size)
	}
	if hi, ok := entry.UIntTag(protocol.FTFileSizeHi); ok {
		result.FileSize += int64(hi << 32)
	}
	if sources, ok := entry.UIntTag(protocol.FTSources); ok {
		result.Sources = int(sources)
	}
	if complete, ok := entry.UIntTag(protocol.FTCompleteSources); ok {
		result.CompleteSources = int(complete)
	}
	if bitrate, ok := entry.UIntTag(protocol.FTMediaBitrate); ok {
		result.MediaBitrate = int(bitrate)
	}
	if length, ok := entry.UIntTag(protocol.FTMediaLength); ok {
		result.MediaLength = int(length)
	}
	if codec, ok := entry.StringTag(protocol.FTMediaCodec); ok {
		result.MediaCodec = codec
	}
	if ext, ok := entry.StringTag(protocol.FTFileFormat); ok {
		result.Extension = ext
	}
	if fileType, ok := entry.StringTag(protocol.FTFileType); ok {
		result.FileType = fileType
	}
	return result
}

func normalizeSearchParams(params SearchParams) SearchParams {
	params.Query = strings.TrimSpace(params.Query)
	if params.Scope == 0 {
		params.Scope = SearchScopeAll
	}
	return params
}

func pickKadKeyword(query string) string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return ""
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return strings.ContainsRune(" ()[]{}<>,._-!?:;\\/\"\t\r\n", r)
	})
	best := ""
	for _, field := range fields {
		if len([]byte(field)) < 3 {
			continue
		}
		if len(field) > len(best) {
			best = field
		}
	}
	return best
}
