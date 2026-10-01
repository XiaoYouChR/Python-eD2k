package disk

import (
	"errors"
	"os"
	"sync"
)

type FileHandler interface {
	File() *os.File
	Path() string
	Close() error
	DeleteFile() error
}

type DesktopFileHandler struct {
	path string
	mu   sync.Mutex
	file *os.File
}

func NewDesktopFileHandler(path string) *DesktopFileHandler {
	return &DesktopFileHandler{path: path}
}

func (h *DesktopFileHandler) File() *os.File {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file == nil {
		h.file, _ = os.OpenFile(h.path, os.O_RDWR|os.O_CREATE, 0o644)
	}
	return h.file
}

func (h *DesktopFileHandler) Path() string {
	return h.path
}

func (h *DesktopFileHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file == nil {
		return nil
	}
	err := h.file.Close()
	h.file = nil
	return err
}

func (h *DesktopFileHandler) DeleteFile() error {
	_ = h.Close()
	if err := os.Remove(h.path); err != nil {
		return errors.New("unable to delete file")
	}
	return nil
}
