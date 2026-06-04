package engine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/entity"
)

// LogProcessor defines the contract for log processors.
type LogProcessor interface {
	Name() string
	Process(logRecord entity.LogRecord) (entity.LogRecord, error)
}

// processorFanout orchestrates multiple workers that should process incoming (raw) logs.
type processorFanout struct {
	sources      map[string]LogSource
	processors   map[string]LogProcessor
	logger       *slog.Logger
	workersCount uint
	wg           sync.WaitGroup
}

// newProcessorFanout creates a new processorFanout.
func newProcessorFanout(logger *slog.Logger, sources []LogSource, processors []LogProcessor, workersCount uint) *processorFanout {
	// s and p are maps of source and processor names to their corresponding objects.
	s := make(map[string]LogSource)
	p := make(map[string]LogProcessor)

	for _, source := range sources {
		s[source.Name()] = source
	}

	for _, processor := range processors {
		p[processor.Name()] = processor
	}

	return &processorFanout{
		sources:      s,
		processors:   p,
		logger:       logger,
		workersCount: workersCount,
	}
}

// run reads unprocessed logs and processes the log, then pushes the processed log back to results channel.
func (pf *processorFanout) run(ctx context.Context, unprocessedLogs <-chan entity.LogRecord, results chan<- entity.LogRecord) {
	spawnWorker := func(workerId int) {
		for j := range unprocessedLogs {
			processed := pf.processLog(j)
			processed.ID = uuid.New()
			select {
			case results <- processed:
				pf.logger.Debug("processed log", "worker_id", workerId, "id", processed.ID)
			case <-ctx.Done():
				return
			}
		}
	}

	for i := 0; i < int(pf.workersCount); i++ {
		pf.wg.Go(func() {
			spawnWorker(i)
		})
	}

	pf.wg.Wait()
	close(results)
}

// processLog is the actual function that processes an unprocessed log based on it's source and corresponding processors.
func (pf *processorFanout) processLog(unprocessedLog entity.LogRecord) entity.LogRecord {
	src, ok := pf.sources[unprocessedLog.Source]
	if !ok {
		pf.logger.Error("source not found", "source", unprocessedLog.Source)
		return unprocessedLog
	}

	// NOTE: it's kind of unnecessary to check if processors exists for a source as it's done in engine as well, but I'm keeping it for now.
	for _, pName := range src.ProcessorNames() {
		p := pf.processors[pName]
		if p == nil {
			pf.logger.Warn("processor not found", "processor", pName)
			continue
		}

		processedLog, err := p.Process(unprocessedLog)
		if err != nil {
			pf.logger.Error("failed to process log", "processor", pName, "error", err)
			continue
		}

		unprocessedLog = processedLog
	}

	return unprocessedLog
}
