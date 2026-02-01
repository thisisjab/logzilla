package source

import (
	"context"

	"github.com/thisisjab/logzilla/entity"
)

type LogSource interface {
	SourceName() string
	Provide(ctx context.Context, logChan chan<- entity.RawLogRecord) error
}
