package storage

import (
	"strings"

	"github.com/thisisjab/logzilla/entity"
)

func parseLogLevel(level any) entity.LogLevel { //nolint:unused
	if l, ok := level.(string); ok {
		level = strings.ToLower(l)
	}

	switch level {
	case "debug", 1:
		return entity.LogLevelDebug
	case "info", 2:
		return entity.LogLevelInfo
	case "warn", 3:
		return entity.LogLevelWarn
	case "error", 4:
		return entity.LogLevelError
	case "fatal", 5:
		return entity.LogLevelFatal
	default:
		return entity.LogLevelUnknown
	}
}
