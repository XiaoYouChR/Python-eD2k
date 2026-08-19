package server

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/monkeyWie/goed2k/protocol"
)

// OfferFiles is the client-to-server shared file publish packet.
type OfferFiles struct {
	Entries []SharedFileEntry
}

func (o *OfferFiles) Get(src *bytes.Reader) error {
	count, err := protocol.ReadUInt32(src)
	if err != nil {
		return err
	}
	o.Entries = make([]SharedFileEntry, int(count))
	for idx := range o.Entries {
		if err := o.Entries[idx].Get(src); err != nil {
			return err
		}
	}
	return nil
}

func (o OfferFiles) Put(dst *bytes.Buffer) error {
	if err := protocol.WriteUInt32(dst, uint32(len(o.Entries))); err != nil {
		return err
	}
	for idx := range o.Entries {
		if err := o.Entries[idx].Put(dst); err != nil {
			return err
		}
	}
	return nil
}

func (o OfferFiles) BytesCount() int {
	size := 4
	for idx := range o.Entries {
		size += o.Entries[idx].BytesCount()
	}
	return size
}

func NewOfferFiles(clientID int32, listenPort int, files []OfferFile) OfferFiles {
	entries := make([]SharedFileEntry, 0, len(files))
	for _, file := range files {
		if file.Hash.IsZero() || file.Size <= 0 {
			continue
		}
		entry := SharedFileEntry{
			Hash:     file.Hash,
			ClientID: clientID,
			Port:     uint16(listenPort),
			Tags: protocol.TagList{
				protocol.NewStringTag(protocol.FTFilename, file.Name),
				protocol.NewUInt32Tag(protocol.FTFileSize, uint32(file.Size)),
			},
		}
		if fileType := fileTypeFromName(file.Name); fileType != "" {
			entry.Tags = append(entry.Tags, protocol.NewStringTag(protocol.FTFileType, fileType))
		}
		if ext := fileExt(file.Name); ext != "" {
			entry.Tags = append(entry.Tags, protocol.NewStringTag(protocol.FTFileFormat, ext))
		}
		if file.Size > 0xffffffff {
			entry.Tags = append(entry.Tags,
				protocol.NewUInt32Tag(protocol.FTFileSizeHi, uint32(uint64(file.Size)>>32)),
			)
		}
		entries = append(entries, entry)
	}
	return OfferFiles{Entries: entries}
}

type OfferFile struct {
	Hash protocol.Hash
	Name string
	Size int64
}

func fileExt(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

func fileTypeFromName(name string) string {
	switch fileExt(name) {
	case "mp3", "flac", "aac", "ogg", "wav", "m4a":
		return "Audio"
	case "mp4", "mkv", "avi", "wmv", "mov", "mpeg":
		return "Video"
	case "jpg", "jpeg", "png", "gif", "bmp", "webp":
		return "Image"
	case "zip", "rar", "7z", "tar", "gz":
		return "Archive"
	case "iso", "bin", "nrg":
		return "Iso"
	case "pdf", "doc", "docx", "txt", "epub":
		return "Document"
	case "exe", "msi", "apk", "deb", "rpm":
		return "Program"
	default:
		return ""
	}
}
