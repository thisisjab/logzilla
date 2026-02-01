package processor

import "github.com/thisisjab/logzilla/entity"

// LogProcessor is an interface that defines the contract for log processors.
type LogProcessor interface {
	Process(logRecord entity.RawLogRecord) (entity.ProcessedLogRecord, error)
}
