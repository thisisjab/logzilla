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

// LogRecord represents a log record that is not processed and received from a log source.
type LogRecord struct {
	Source    string         `json:"source"`
	RawData   []byte         `json:"raw_data"`
	Level     LogLevel       `json:"level"`
	Timestamp time.Time      `json:"timestamp"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
}
