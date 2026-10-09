package entity

import "strings"

// LogLevel represents the severity level of a log entry.
type LogLevel uint8

const (
	LevelUnknown LogLevel = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

const (
	levelUnknownStr = "UNKNOWN"
	levelDebugStr   = "DEBUG"
	levelInfoStr    = "INFO"
	levelWarnStr    = "WARN"
	levelErrorStr   = "ERROR"
	levelFatalStr   = "FATAL"
)

// String returns the canonical uppercase string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return levelDebugStr
	case LevelInfo:
		return levelInfoStr
	case LevelWarn:
		return levelWarnStr
	case LevelError:
		return levelErrorStr
	case LevelFatal:
		return levelFatalStr
	default:
		return levelUnknownStr
	}
}

// IsValid reports whether the log level is a defined severity level (Debug through Fatal).
func (l LogLevel) IsValid() bool {
	return l >= LevelDebug && l <= LevelFatal
}

// ParseLogLevel parses a string into a LogLevel, accepting common aliases and case-insensitive strings.
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "dbg", "d":
		return LevelDebug
	case "info", "inf", "i", "notice", "information":
		return LevelInfo
	case "warn", "warning", "w":
		return LevelWarn
	case "error", "err", "e":
		return LevelError
	case "fatal", "crit", "critical", "panic", "f":
		return LevelFatal
	default:
		return LevelUnknown
	}
}
