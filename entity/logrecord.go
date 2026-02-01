package entity

import "time"

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
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

// RawLogRecord represents a log record that is not processed and received from a log source.
type RawLogRecord struct {
	Source    string    `json:"source"`
	Data      []byte    `json:"data"`
	Level     LogLevel  `json:"level"`
	Timestamp time.Time `json:"timestamp"`
}

// ProcessedLogRecord represents a log record that has been processed by a processor.
type ProcessedLogRecord struct {
	Source    string         `json:"source"`
	Message   string         `json:"message"`
	Level     LogLevel       `json:"level"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata"`
}
