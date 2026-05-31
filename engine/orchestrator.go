package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/thisisjab/logzilla/entity"
)

// Config defines all required settings and components for Engine to work properly.
type Config struct {
	// Sources define log sources receiving (or polling) logs.
	// Sources cannot have duplicate names.
	Sources []LogSource
	// Processors define all processing rules for logs. Processors can be reused across sources.
	// Processors cannot have duplicate names.
	Processors []LogProcessor
	// Storage defines where logs are stored.
	// Each storage supports different features. However, I put my best effort to make all storages offer the same features.
	Storage EngineStorage
	// StorrageFlushInterval defines how often storage manager should flush logs to storage.
	// If value is zero, storage manager will not flush logs automatically.
	StorageFlushInterval time.Duration `yaml:"storage-flush-interval"`
	// InBufferSize defines how many logs can be buffered before being consumed by processors.
	InBufferSize uint `yaml:"in-buffer-size"`
	// OutBufferSize defines how many processed logs can be buffered before being written to storage.
	// If StorageFlushInterval is zero, processed logs will be written to storage as soon as buffer is full.
	OutBufferSize uint `yaml:"out-buffer-size"`
	// ProcessorWorkersCount defines how many workers should be used for processing logs.
	ProcessorWorkersCount uint `yaml:"processor-workers-count"`
}

// Engine orchestrates different components such as log sources (readers) and processors.
// It starts by validating configuration and then starts consuming logs from all sources.
//
// Incoming logs will reside in **InBuffer** until they are processed by processors.
// Each log undergoes processing by the hierarchy that is defined in the configuration.
// For example, if there is a source X that needs to be processed by processor P1, and P2,
// then logs will be processed by P1, then P2, and finally stored in **OutBuffer**.
//
// Logs in **OutBuffer** will be written to storage periodically if StorageFlushInterval is set.
// Otherwise, logs will be written to storage as soon as **OutBuffer** is full.
type Engine struct {
	cfg            Config
	logger         *slog.Logger
	storageManager *engineStorageManager
}

// New creates a new Engine.
func New(cfg Config, logger *slog.Logger) (*Engine, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &Engine{
		cfg:            cfg,
		logger:         logger,
		storageManager: newStorageManager(logger, cfg.Storage, cfg.OutBufferSize, cfg.StorageFlushInterval)}, nil
}

// validate validates configuration.
func (c Config) validate() error {
	// Basic validations
	if len(c.Sources) == 0 {
		return errors.New("no log sources are configured")
	}

	if c.Storage == nil {
		return errors.New("no log storage is configured")
	}

	if c.InBufferSize == 0 {
		return errors.New("in buffer size cannot be zero")
	}

	if c.OutBufferSize == 0 {
		return errors.New("out buffer size cannot be zero")
	}

	if c.ProcessorWorkersCount == 0 {
		return errors.New("processor workers cannot be zero")
	}

	processors := make(map[string]bool)
	// Validate if processors have unique names
	for i, p := range c.Processors {
		if _, ok := processors[p.Name()]; ok {
			return fmt.Errorf("processor %d has duplicate name: %s", i, p.Name())
		}
		processors[p.Name()] = true
	}

	// Validate if sources have unique names and their processors exist
	sources := make(map[string]bool)
	for i, s := range c.Sources {
		if _, ok := sources[s.Name()]; ok {
			return fmt.Errorf("source %d has duplicate name: %s", i, s.Name())
		}
		sources[s.Name()] = true

		for _, p := range s.ProcessorNames() {
			if _, ok := processors[p]; !ok {
				return fmt.Errorf("source %s (%d) has processor %s that does not exist", s.Name(), i, p)
			}
		}
	}

	return nil
}

// Run starts the engine. It's a blocking call that runs the engine until the context is cancelled OR the engine is stopped.
func (eng *Engine) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	// Start consuming logs from all sources.
	// unprocessedLogsChan channel will contain all logs that are recently received from any source.
	unprocessedLogsChan := eng.consumeLogs(ctx)

	processedLogsChan := make(chan entity.LogRecord, eng.cfg.OutBufferSize)

	pf := newProcessorFanout(eng.logger, eng.cfg.Sources, eng.cfg.Processors, eng.cfg.ProcessorWorkersCount)

	// Storage manager handles buffering, and periodic saves.
	wg.Go(func() { eng.storageManager.run(ctx) })

	// Process fanout handles fan-out pattern.
	wg.Go(func() { pf.run(ctx, unprocessedLogsChan, processedLogsChan) })

	for {
		select {
		case <-ctx.Done():
			// All goroutines (storage manager & processor fanout) will eventually return when context is cancelled.
			wg.Wait()

			return nil
		case p, ok := <-processedLogsChan:
			if !ok {
				return nil
			}

			eng.storageManager.addProcessedLogs(ctx, p)
		}
	}
}

// consumeLogs gathers logs from defined sources, then sends to processorFanout to be processed.
// Later, processed logs will be sent to storage manager.
func (eng *Engine) consumeLogs(ctx context.Context) <-chan entity.LogRecord {
	unprocessedLogs := make(chan entity.LogRecord, eng.cfg.InBufferSize)

	var sourceWg sync.WaitGroup

	// Spawn sources
	for _, s := range eng.cfg.Sources {
		sourceWg.Add(1)
		go func(name string, src LogSource) {
			defer sourceWg.Done()

			// Provice is a blocking call, therefore when it returns, it's either by an error or context cancellation.
			err := src.Provide(ctx, unprocessedLogs)

			if err != nil {
				eng.logger.Error("failed to start log source", "name", name, "error", err)
			}
		}(s.Name(), s)
	}

	go func() {
		// Since all sources are blocking, we need to wait for them to finish.
		// This means if context is cancelled, all sources will be cancelled automatically as well
		sourceWg.Wait()
		close(unprocessedLogs)
	}()

	return unprocessedLogs
}
