package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/entity"
)

// LogProcessor is an interface that defines the contract for log processors.
type LogProcessor interface {
	Process(logRecord entity.LogRecord) (entity.LogRecord, error)
}

const WorkersCount = 10

type processorManager struct {
	interval     time.Duration
	sources      map[string]LogSource
	processors   map[string]LogProcessor
	logger       *slog.Logger
	workersCount int
	wg           sync.WaitGroup
}

func newProcessorManager(logger *slog.Logger, sources map[string]LogSource, processors map[string]LogProcessor, workersCount int, interval time.Duration) *processorManager {
	return &processorManager{
		interval:     interval,
		sources:      sources,
		processors:   processors,
		logger:       logger,
		workersCount: workersCount,
	}
}

func (pm *processorManager) run(ctx context.Context, rawLogsChan <-chan entity.LogRecord, results chan<- entity.LogRecord) {
	spawnWorker := func(workerId int) {
		for {
			select {
			case <-ctx.Done():
				return
			case j, ok := <-rawLogsChan:
				if !ok {
					// The jobs channel is closed and empty. No more work.
					return
				}
				// Process and send to results
				processed := pm.processLog(j)
				processed.ID = uuid.New()

				pm.logger.Debug("processed log", "worker_id", workerId, "log_id", processed.ID)

				select {
				case results <- processed:
				case <-ctx.Done():
					// If we can't send because context is cancelled, exit.
					return
				}
			}
		}
	}

	for i := 0; i < pm.workersCount; i++ {
		pm.wg.Go(func() {
			spawnWorker(i)
		})
	}

	pm.wg.Wait()
}

func (pm *processorManager) processLog(rawLog entity.LogRecord) entity.LogRecord {
	src, ok := pm.sources[rawLog.Source]
	if !ok {
		pm.logger.Error("Source not found", "source", rawLog.Source)
		return rawLog
	}

	for _, pName := range src.ProcessorNames() {
		p := pm.processors[pName]
		if p == nil {
			pm.logger.Warn("Processor not found", "processor", pName)
			continue
		}

		processedLog, err := p.Process(rawLog)
		if err != nil {
			pm.logger.Error("Failed to process log", "error", err)
			continue
		}

		rawLog = processedLog
	}

	return rawLog
}
