package entity

import (
	"time"

	"github.com/google/uuid"
)

type LogLevel uint8

const (
	LogLevelUnknown LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

func (l LogLevel) String() string {
	return [...]string{"unknown", "debug", "info", "warn", "error", "fatal"}[l]
}

// LogRecord represents a log record received from a log source.
type LogRecord struct {
	ID        uuid.UUID      `json:"id"`
	Source    string         `json:"source"`
	RawData   []byte         `json:"-"`
	Level     LogLevel       `json:"level"`
	Timestamp time.Time      `json:"timestamp"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
}
