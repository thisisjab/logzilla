package engine

import (
	"context"

	"github.com/thisisjab/logzilla/entity"
)

// LogSource is an interface that defines the contract for log sources (providers).
type LogSource interface {
	Name() string
	Provide(ctx context.Context, logChan chan<- entity.LogRecord) error
	ProcessorNames() []string
}
