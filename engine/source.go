package engine

import (
	"context"

	"github.com/thisisjab/logzilla/entity"
)

// LogSource is an interface that defines the contract for log sources.
type LogSource interface {
	Provide(ctx context.Context, logChan chan<- entity.LogRecord) error
	ProcessorNames() []string
	SourceName() string
}
