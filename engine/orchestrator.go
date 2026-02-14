package engine

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/thisisjab/logzilla/entity"
)

type Config struct {
	Sources              map[string]LogSource
	Processors           map[string]LogProcessor
	Storage              Storage
	StorageFlushInterval time.Duration
	BufferMaxSize        uint
}

type Engine struct {
	cfg            Config
	logger         *slog.Logger
	storageManager *storageManager
}

func New(cfg Config, logger *slog.Logger) (*Engine, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &Engine{cfg: cfg, logger: logger, storageManager: newStorageManager(logger, cfg.Storage, cfg.BufferMaxSize, cfg.StorageFlushInterval)}, nil
}

func (c Config) validate() error {
	if len(c.Sources) == 0 {
		return errors.New("no log sources are configured")
	}

	// TODO: validate used processors do exists (defined in configuration)

	if c.Storage == nil {
		return errors.New("no log storage is configured")
	}

	if c.BufferMaxSize == 0 && c.StorageFlushInterval == 0 {
		return errors.New("buffer max size and storage flush interval cannot both be zero")
	}

	return nil
}

func (e *Engine) Run(ctx context.Context) error {
	// Start consuming logs from all sources.
	rawLogs := e.consumeLogs(ctx)

	var wg sync.WaitGroup
	processedLogs := make(chan entity.LogRecord, 1000)

	pm := newProcessorManager(e.logger, e.cfg.Sources, e.cfg.Processors, WorkersCount, 10*time.Second)

	wg.Go(func() { e.storageManager.run(ctx) })
	wg.Go(func() { pm.run(ctx, rawLogs, processedLogs) })

	for {
		select {
		case <-ctx.Done():
			// Context cancelled (e.g., user hit Ctrl+C)
			wg.Wait()
			return ctx.Err()
		case p, ok := <-processedLogs:
			if !ok {
				return nil
			}
			e.storageManager.addProcessedLogs(ctx, p)
		}
	}
}

func (e *Engine) consumeLogs(ctx context.Context) <-chan entity.LogRecord {
	// Increase buffer size to handle bursts (e.g., 5000 logs/sec)
	const chanSize = 5000
	rawLogs := make(chan entity.LogRecord, chanSize)
	e.logger.Info("Created incoming logs channel.", "size", chanSize)

	var sourceWg sync.WaitGroup

	for n, s := range e.cfg.Sources {
		sourceWg.Add(1)
		go func(name string, src LogSource) {
			defer sourceWg.Done()
			err := src.Provide(ctx, rawLogs)

			if err != nil {
				e.logger.Error("failed to start log source.", "name", name, "error", err)
			}
		}(n, s)
	}

	go func() {
		sourceWg.Wait()
		close(rawLogs)
	}()

	return rawLogs
}
