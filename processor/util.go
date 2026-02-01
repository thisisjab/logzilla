package processor

import (
	"github.com/thisisjab/logzilla/entity"
)

func parseLevel(level string) entity.LogLevel {
	switch level {
	case "DEBUG":
		return entity.LogLevelDebug
	case "INFO":
		return entity.LogLevelInfo
	case "WARN":
		return entity.LogLevelWarn
	case "ERROR":
		return entity.LogLevelError
	case "FATAL":
		return entity.LogLevelFatal
	default:
		return entity.LogLevelInfo
	}
}
