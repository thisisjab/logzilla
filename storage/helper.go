package storage

import "github.com/thisisjab/logzilla/entity"

func parseLogLevel(level string) entity.LogLevel { //nolint:unused
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
		return entity.LogLevelUnknown
	}
}
