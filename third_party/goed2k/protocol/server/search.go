package server

import (
	"bytes"
	"strings"

	"github.com/monkeyWie/goed2k/protocol"
)

const (
	searchTypeBool   byte = 0x00
	searchTypeString byte = 0x01
	searchTypeStrTag byte = 0x02
	searchTypeUint32 byte = 0x03
	searchTypeUint64 byte = 0x08
	searchOpEqual    byte = 0x00
	searchOpGreater  byte = 0x01
	searchOpLess     byte = 0x02
)

type SearchRequest struct {
	Query              string
	MinSize            int64
	MaxSize            int64
	MinSources         int
	MinCompleteSources int
	FileType           string
	Extension          string
}

func (s *SearchRequest) Get(src *bytes.Reader) error {
	return nil
}

func (s *SearchRequest) Put(dst *bytes.Buffer) error {
	terms := make([]searchTerm, 0, 8)
	if s.FileType != "" {
		terms = append(terms, searchStringTag(protocol.FTFileType, s.FileType))
	}
	if s.Extension != "" {
		terms = append(terms, searchStringTag(protocol.FTFileFormat, s.Extension))
	}
	if s.MinSize > 0 {
		terms = append(terms, searchNumericTag(protocol.FTFileSize, searchOpGreater, uint64(s.MinSize)))
	}
	if s.MaxSize > 0 {
		terms = append(terms, searchNumericTag(protocol.FTFileSize, searchOpLess, uint64(s.MaxSize)))
	}
	if s.MinSources > 0 {
		terms = append(terms, searchNumericTag(protocol.FTSources, searchOpGreater, uint64(s.MinSources)))
	}
	if s.MinCompleteSources > 0 {
		terms = append(terms, searchNumericTag(protocol.FTCompleteSources, searchOpGreater, uint64(s.MinCompleteSources)))
	}
	for _, token := range tokenizeSearchQuery(s.Query) {
		terms = append(terms, searchString(token))
	}
	for i, term := range terms {
		if i > 0 {
			if err := dst.WriteByte(searchTypeBool); err != nil {
				return err
			}
			if err := dst.WriteByte(0x00); err != nil { // AND
				return err
			}
		}
		if err := term.put(dst); err != nil {
			return err
		}
	}
	return nil
}

func (s *SearchRequest) BytesCount() int {
	size := 0
	terms := 0
	if s.FileType != "" {
		size += searchStringTag(protocol.FTFileType, s.FileType).bytesCount()
		terms++
	}
	if s.Extension != "" {
		size += searchStringTag(protocol.FTFileFormat, s.Extension).bytesCount()
		terms++
	}
	if s.MinSize > 0 {
		size += searchNumericTag(protocol.FTFileSize, searchOpGreater, uint64(s.MinSize)).bytesCount()
		terms++
	}
	if s.MaxSize > 0 {
		size += searchNumericTag(protocol.FTFileSize, searchOpLess, uint64(s.MaxSize)).bytesCount()
		terms++
	}
	if s.MinSources > 0 {
		size += searchNumericTag(protocol.FTSources, searchOpGreater, uint64(s.MinSources)).bytesCount()
		terms++
	}
	if s.MinCompleteSources > 0 {
		size += searchNumericTag(protocol.FTCompleteSources, searchOpGreater, uint64(s.MinCompleteSources)).bytesCount()
		terms++
	}
	for _, token := range tokenizeSearchQuery(s.Query) {
		size += searchString(token).bytesCount()
		terms++
	}
	if terms > 1 {
		size += (terms - 1) * 2
	}
	return size
}

type SearchMore struct{}

func (*SearchMore) Get(src *bytes.Reader) error { return nil }
func (*SearchMore) Put(dst *bytes.Buffer) error { return nil }
func (*SearchMore) BytesCount() int             { return 0 }

type SharedFileEntry struct {
	Hash     protocol.Hash
	ClientID int32
	Port     uint16
	Tags     protocol.TagList
}

func (s *SharedFileEntry) Get(src *bytes.Reader) error {
	hash, err := protocol.ReadHash(src)
	if err != nil {
		return err
	}
	clientID, err := protocol.ReadInt32(src)
	if err != nil {
		return err
	}
	port, err := protocol.ReadUInt16(src)
	if err != nil {
		return err
	}
	var tags protocol.TagList
	if err := tags.Get(src); err != nil {
		return err
	}
	s.Hash = hash
	s.ClientID = clientID
	s.Port = port
	s.Tags = tags
	return nil
}

func (s *SharedFileEntry) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteHash(dst, s.Hash); err != nil {
		return err
	}
	if err := protocol.WriteInt32(dst, s.ClientID); err != nil {
		return err
	}
	if err := protocol.WriteUInt16(dst, s.Port); err != nil {
		return err
	}
	return s.Tags.Put(dst)
}

func (s *SharedFileEntry) BytesCount() int {
	return 16 + 4 + 2 + s.Tags.BytesCount()
}

func (s SharedFileEntry) StringTag(id byte) (string, bool) {
	for _, tag := range s.Tags {
		if tag.ID == id {
			return tag.String, true
		}
	}
	return "", false
}

func (s SharedFileEntry) UIntTag(id byte) (uint64, bool) {
	for _, tag := range s.Tags {
		if tag.ID == id {
			if tag.Type == protocol.TagTypeString || tag.Type >= protocol.TagTypeStr1 {
				return 0, false
			}
			return tag.UInt64, true
		}
	}
	return 0, false
}

type SearchResult struct {
	Results     []SharedFileEntry
	MoreResults bool
}

func (s *SearchResult) Get(src *bytes.Reader) error {
	count, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	s.Results = make([]SharedFileEntry, int(count))
	for i := 0; i < int(count); i++ {
		if err := s.Results[i].Get(src); err != nil {
			return err
		}
	}
	if src.Len() > 0 {
		flag, err := src.ReadByte()
		if err != nil {
			return err
		}
		s.MoreResults = flag != 0
	}
	return nil
}

func (s *SearchResult) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteUInt32(dst, uint32(len(s.Results))); err != nil {
		return err
	}
	for i := range s.Results {
		if err := s.Results[i].Put(dst); err != nil {
			return err
		}
	}
	if s.MoreResults {
		return dst.WriteByte(1)
	}
	return dst.WriteByte(0)
}

func (s *SearchResult) BytesCount() int {
	size := 4 + 1
	for i := range s.Results {
		size += s.Results[i].BytesCount()
	}
	return size
}

type searchTerm interface {
	put(dst *bytes.Buffer) error
	bytesCount() int
}

type searchStringTerm struct {
	value string
	tagID byte
	tagged bool
}

func searchString(value string) searchStringTerm {
	return searchStringTerm{value: value}
}

func searchStringTag(id byte, value string) searchStringTerm {
	return searchStringTerm{value: value, tagID: id, tagged: true}
}

func (s searchStringTerm) put(dst *bytes.Buffer) error {
	if s.tagged {
		if err := dst.WriteByte(searchTypeStrTag); err != nil {
			return err
		}
	} else {
		if err := dst.WriteByte(searchTypeString); err != nil {
			return err
		}
	}
	if err := protocol.WriteUInt16(dst, uint16(len(s.value))); err != nil {
		return err
	}
	if _, err := dst.WriteString(s.value); err != nil {
		return err
	}
	if s.tagged {
		if err := protocol.WriteUInt16(dst, 1); err != nil {
			return err
		}
		return dst.WriteByte(s.tagID)
	}
	return nil
}

func (s searchStringTerm) bytesCount() int {
	size := 1 + 2 + len(s.value)
	if s.tagged {
		size += 2 + 1
	}
	return size
}

type searchNumericTerm struct {
	tagID    byte
	operator byte
	value    uint64
}

func searchNumericTag(id, operator byte, value uint64) searchNumericTerm {
	return searchNumericTerm{tagID: id, operator: operator, value: value}
}

func (s searchNumericTerm) put(dst *bytes.Buffer) error {
	if s.value <= 0xffffffff {
		if err := dst.WriteByte(searchTypeUint32); err != nil {
			return err
		}
		if err := protocol.WriteUInt32(dst, uint32(s.value)); err != nil {
			return err
		}
	} else {
		if err := dst.WriteByte(searchTypeUint64); err != nil {
			return err
		}
		if err := protocol.WriteUInt64(dst, s.value); err != nil {
			return err
		}
	}
	if err := dst.WriteByte(s.operator); err != nil {
		return err
	}
	if err := protocol.WriteUInt16(dst, 1); err != nil {
		return err
	}
	return dst.WriteByte(s.tagID)
}

func (s searchNumericTerm) bytesCount() int {
	size := 1 + 1 + 2 + 1
	if s.value <= 0xffffffff {
		return size + 4
	}
	return size + 8
}

func tokenizeSearchQuery(query string) []string {
	fields := strings.FieldsFunc(strings.TrimSpace(query), func(r rune) bool {
		return strings.ContainsRune(" ()[]{}<>,._-!?:;\\/\"\t\r\n", r)
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		out = append(out, field)
	}
	return out
}
