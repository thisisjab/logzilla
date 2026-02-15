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
	Sources                    map[string]LogSource
	Processors                 map[string]LogProcessor
	Storage                    Storage
	StorageFlushInterval       time.Duration
	RawLogsBufferMaxSize       uint
	ProcessedLogsBufferMaxSize uint
	ProcessorWorkersCount      uint
}

// Engine orchestrates different components such as log sources (readers) and processors.
type Engine struct {
	cfg            Config
	logger         *slog.Logger
	storageManager *storageManager
}

func New(cfg Config, logger *slog.Logger) (*Engine, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &Engine{
		cfg:            cfg,
		logger:         logger,
		storageManager: newStorageManager(logger, cfg.Storage, cfg.RawLogsBufferMaxSize, cfg.StorageFlushInterval)}, nil
}

func (c Config) validate() error {
	if len(c.Sources) == 0 {
		return errors.New("no log sources are configured")
	}

	// TODO: validate used processors do exists (defined in configuration)

	if c.Storage == nil {
		return errors.New("no log storage is configured")
	}

	if c.RawLogsBufferMaxSize == 0 && c.StorageFlushInterval == 0 {
		return errors.New("buffer max size and storage flush interval cannot both be zero")
	}

	if c.ProcessedLogsBufferMaxSize == 0 {
		return errors.New("processed logs buffer max size cannot be zero")
	}

	if c.ProcessorWorkersCount == 0 {
		return errors.New("processor workers cannot be zero")
	}

	return nil
}

func (e *Engine) Run(ctx context.Context) error {
	// Start consuming logs from all sources.
	// rawLogs will contain all raw logs from all sources.
	rawLogs := e.consumeLogs(ctx)

	var wg sync.WaitGroup
	processedLogs := make(chan entity.LogRecord, e.cfg.ProcessedLogsBufferMaxSize)

	pm := newProcessorManager(e.logger, e.cfg.Sources, e.cfg.Processors, e.cfg.ProcessorWorkersCount, 10*time.Second)

	// Storage manager handles buffering, and periodic saves.
	wg.Go(func() { e.storageManager.run(ctx) })
	// Process manager handles fan-out pattern.
	wg.Go(func() { pm.run(ctx, rawLogs, processedLogs) })

	for {
		select {
		case <-ctx.Done():
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
	rawLogs := make(chan entity.LogRecord, e.cfg.RawLogsBufferMaxSize)
	e.logger.Info("created incoming logs channel.", "size", e.cfg.RawLogsBufferMaxSize)

	var sourceWg sync.WaitGroup

	// Spawn sources
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
