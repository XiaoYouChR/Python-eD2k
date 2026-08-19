package logx

import (
	"io"
	"log/slog"
	"sync"
)

var (
	mu     sync.RWMutex
	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
)

func SetLogger(l *slog.Logger) {
	mu.Lock()
	defer mu.Unlock()
	if l == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
		return
	}
	logger = l
}

func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}
