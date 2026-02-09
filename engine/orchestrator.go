package engine

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/thisisjab/logzilla/entity"
)

const WorkersCount = 10

// LogSource is an interface that defines the contract for log sources.
type LogSource interface {
	Provide(ctx context.Context, logChan chan<- entity.LogRecord) error
	ProcessorNames() []string
	SourceName() string
}

// LogProcessor is an interface that defines the contract for log processors.
type LogProcessor interface {
	Process(logRecord entity.LogRecord) (entity.LogRecord, error)
}

type Config struct {
	Sources    map[string]LogSource
	Processors map[string]LogProcessor
}

type Engine struct {
	cfg    Config
	logger *slog.Logger
}

func New(cfg Config, logger *slog.Logger) *Engine {
	return &Engine{cfg: cfg, logger: logger}
}

func (e *Engine) ValidateConfig() error {
	if len(e.cfg.Sources) == 0 {
		return errors.New("no log sources configured")
	}

	return nil
}

func (e *Engine) Run(ctx context.Context) error {
	if err := e.ValidateConfig(); err != nil {
		return err
	}

	// 1. Start consuming logs from all sources.
	// This returns a channel that acts as the load balancer queue.
	rawLogs := e.consumeLogs(ctx)

	// 2. Create a channel for processed results.
	// Buffer size 100 is fine, but you might tune this based on throughput.
	results := make(chan entity.LogRecord, 100)

	// 3. Use a WaitGroup to wait for all workers to finish processing.
	var workersWg sync.WaitGroup

	// 4. Start the 10 workers.
	for i := range WorkersCount {
		workersWg.Go(func() {
			e.logger.Debug("Started processor worker.", "id", i+1)
			e.processorWorker(ctx, rawLogs, results)
		})
	}

	// 5. Wait for all workers to finish in a separate goroutine.
	// Once done, close the results channel to signal the main loop that work is complete.
	go func() {
		workersWg.Wait()
		e.logger.Debug("Closing results channel.")
		close(results)
	}()

	// 6. Main blocking loop: Read from results until the channel is closed.
	for {
		select {
		case <-ctx.Done():
			// Context cancelled (e.g., user hit Ctrl+C)
			return ctx.Err()
		case res, ok := <-results:
			if !ok {
				// Channel closed and drained, meaning all workers are done.
				return nil
			}
			// Handle the processed log
			// TODO: implement storage
			e.logger.Info("New processed log.", "message", res.Message)
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
				e.logger.Error("Failed to start log source.", "name", name, "error", err)
			}
		}(n, s)
	}

	go func() {
		sourceWg.Wait()
		close(rawLogs)
	}()

	return rawLogs
}

func (e *Engine) processLog(rawLog entity.LogRecord) entity.LogRecord {
	src, ok := e.cfg.Sources[rawLog.Source]
	if !ok {
		e.logger.Error("Source not found", "source", rawLog.Source)
		return rawLog
	}

	for _, pName := range src.ProcessorNames() {
		p := e.cfg.Processors[pName]
		if p == nil {
			e.logger.Warn("Processor not found", "processor", pName)
			continue
		}

		processedLog, err := p.Process(rawLog)
		if err != nil {
			e.logger.Error("Failed to process log", "error", err)
			continue
		}

		rawLog = processedLog
	}

	return rawLog
}

func (e *Engine) processorWorker(ctx context.Context, jobs <-chan entity.LogRecord, results chan<- entity.LogRecord) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-jobs:
			if !ok {
				// The jobs channel is closed and empty. No more work.
				return
			}
			// Process and send to results
			processed := e.processLog(j)

			select {
			case results <- processed:
			case <-ctx.Done():
				// If we can't send because context is cancelled, exit.
				return
			}
		}
	}
}
