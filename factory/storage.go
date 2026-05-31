package factory

import (
	"context"
	"fmt"

	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/querier"
	"github.com/thisisjab/logzilla/storage"
)

type storageType interface {
	engine.EngineStorage
	querier.QuerierStorage
	Open(ctx context.Context) error
	Close(ctx context.Context) error
}

// BuildStorage creates a storage instance based on the provided configuration.
func BuildStorage(cfg storage.Config) (storageType, error) {
	switch cfg.Type {
	case "clickhouse":
		var clickhouseConfig storage.ClickHouseStorageConfig
		if err := cfg.Config.Decode(&clickhouseConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal clickhouse config: %w", err)
		}

		return storage.NewClickHouseStorage(clickhouseConfig)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
