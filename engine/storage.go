package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/thisisjab/logzilla/entity"
)

// Storage represents a storage interface for the engine.
// Storage needs handle buffering by itself.
type Storage interface {
	StoreProcessedLogs(ctx context.Context, logs ...entity.LogRecord) error
}

// storageManager manages storage operations like inserting, buffering, and flushing logs.
// Note that you should never disable buffering and scheduled flushing together.
type storageManager struct {
	storage         Storage
	logger          *slog.Logger
	processedBuffer []entity.LogRecord
	processedMutex  sync.Mutex
	wg              sync.WaitGroup

	// bufferMaxSize defines the maximum items that buffer holds before flushing.
	// If value is reached, buffer will be flushed immediately.
	// Setting this to zero will disable buffering.
	bufferMaxSize uint

	// flushInterval defines the interval at which buffer will be flushed.
	// Setting flushInterval to 0 will disable scheduled flushing.
	flushInterval time.Duration
}

func newStorageManager(logger *slog.Logger, storage Storage, bufferMaxSize uint, flushInterval time.Duration) *storageManager {
	return &storageManager{
		logger:          logger,
		storage:         storage,
		bufferMaxSize:   bufferMaxSize,
		processedBuffer: make([]entity.LogRecord, 0, bufferMaxSize),
		flushInterval:   flushInterval,
	}
}

func (sm *storageManager) run(ctx context.Context) {
	var ticker *time.Ticker

	if sm.flushInterval > 0 {
		ticker = time.NewTicker(sm.flushInterval)
		defer ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			sm.flushBuffers(ctx)
			sm.wg.Wait()
			return
		// Please don't panic by this syntax. This was new to me as well.
		// If ticker is nil, reading from it's channel will panic.
		// So we do this trick that returns a channel that blocks forever if ticker is disabled.
		case <-func() <-chan time.Time {
			if ticker != nil {
				return ticker.C
			}
			return make(chan time.Time) // blocks forever if ticker is disabled
		}():
			sm.flushBuffers(ctx)
		}
	}
}

func (sm *storageManager) flushBuffers(ctx context.Context) {
	var processedToFlush []entity.LogRecord

	// Swap processed buffer
	sm.processedMutex.Lock()
	if len(sm.processedBuffer) > 0 {
		processedToFlush = sm.processedBuffer
		sm.processedBuffer = make([]entity.LogRecord, 0, sm.bufferMaxSize)
	}
	sm.processedMutex.Unlock()

	if len(processedToFlush) > 0 {
		sm.flushProcessedLogs(ctx, processedToFlush)
	}
}

func (sm *storageManager) flushProcessedLogs(ctx context.Context, toFlush []entity.LogRecord) {
	sm.wg.Go(func() {
		if err := sm.storage.StoreProcessedLogs(ctx, toFlush...); err != nil {
			sm.logger.Error("failed to flush processed logs", "error", err)
			return
		}

		sm.logger.Debug("flushed processed logs successfuly", "count", len(toFlush))
	})
}

func (sm *storageManager) addProcessedLogs(ctx context.Context, logs ...entity.LogRecord) {
	if len(logs) == 0 {
		return
	}

	var toFlush []entity.LogRecord

	sm.processedMutex.Lock()
	sm.processedBuffer = append(sm.processedBuffer, logs...)

	// Check if buffer reached flush size
	if sm.bufferMaxSize > 0 && uint(len(sm.processedBuffer)) >= sm.bufferMaxSize {
		toFlush = sm.processedBuffer
		sm.processedBuffer = make([]entity.LogRecord, 0, sm.bufferMaxSize)
	}
	sm.processedMutex.Unlock()

	// Flush asynchronously if needed
	if toFlush != nil {
		sm.flushProcessedLogs(ctx, toFlush)
	}
}
