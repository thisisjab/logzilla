package processor

import (
	"strings"

	"github.com/thisisjab/logzilla/entity"
)

func parseLevel(level string) entity.LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return entity.LogLevelDebug
	case "info":
		return entity.LogLevelInfo
	case "warn":
		return entity.LogLevelWarn
	case "error":
		return entity.LogLevelError
	case "fatal":
		return entity.LogLevelFatal
	default:
		return entity.LogLevelUnknown
	}
}
