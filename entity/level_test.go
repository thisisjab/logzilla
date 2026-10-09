package entity_test

import (
	"testing"

	"github.com/thisisjab/logzilla/entity"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    entity.LogLevel
		expected string
	}{
		{name: "unknown", level: entity.LevelUnknown, expected: "UNKNOWN"},
		{name: "debug", level: entity.LevelDebug, expected: "DEBUG"},
		{name: "info", level: entity.LevelInfo, expected: "INFO"},
		{name: "warn", level: entity.LevelWarn, expected: "WARN"},
		{name: "error", level: entity.LevelError, expected: "ERROR"},
		{name: "fatal", level: entity.LevelFatal, expected: "FATAL"},
		{name: "out of range", level: entity.LogLevel(99), expected: "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLogLevel_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		level    entity.LogLevel
		expected bool
	}{
		{name: "unknown", level: entity.LevelUnknown, expected: false},
		{name: "debug", level: entity.LevelDebug, expected: true},
		{name: "info", level: entity.LevelInfo, expected: true},
		{name: "warn", level: entity.LevelWarn, expected: true},
		{name: "error", level: entity.LevelError, expected: true},
		{name: "fatal", level: entity.LevelFatal, expected: true},
		{name: "out of range", level: entity.LogLevel(42), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.IsValid(); got != tt.expected {
				t.Errorf("LogLevel.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected entity.LogLevel
	}{
		{name: "empty", input: "", expected: entity.LevelUnknown},
		{name: "debug uppercase", input: "DEBUG", expected: entity.LevelDebug},
		{name: "debug lowercase", input: "debug", expected: entity.LevelDebug},
		{name: "debug alias dbg", input: "dbg", expected: entity.LevelDebug},
		{name: "debug alias d", input: "d", expected: entity.LevelDebug},
		{name: "info uppercase", input: "INFO", expected: entity.LevelInfo},
		{name: "info lowercase", input: "info", expected: entity.LevelInfo},
		{name: "info alias inf", input: "inf", expected: entity.LevelInfo},
		{name: "info alias notice", input: "notice", expected: entity.LevelInfo},
		{name: "info alias information", input: "information", expected: entity.LevelInfo},
		{name: "warn uppercase", input: "WARN", expected: entity.LevelWarn},
		{name: "warn lowercase", input: "warn", expected: entity.LevelWarn},
		{name: "warn alias warning", input: "warning", expected: entity.LevelWarn},
		{name: "warn alias w", input: "w", expected: entity.LevelWarn},
		{name: "error uppercase", input: "ERROR", expected: entity.LevelError},
		{name: "error lowercase", input: "error", expected: entity.LevelError},
		{name: "error alias err", input: "err", expected: entity.LevelError},
		{name: "error alias e", input: "e", expected: entity.LevelError},
		{name: "fatal uppercase", input: "FATAL", expected: entity.LevelFatal},
		{name: "fatal lowercase", input: "fatal", expected: entity.LevelFatal},
		{name: "fatal alias crit", input: "crit", expected: entity.LevelFatal},
		{name: "fatal alias critical", input: "critical", expected: entity.LevelFatal},
		{name: "fatal alias panic", input: "panic", expected: entity.LevelFatal},
		{name: "whitespace padded", input: "  error \n", expected: entity.LevelError},
		{name: "unrecognized string", input: "custom_level", expected: entity.LevelUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entity.ParseLogLevel(tt.input); got != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
