package wal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// [UNIX Millisecond (8 Bytes)] [ULID (16 bytes)] [Collector Name Length (4 bytes)] [Collector Name] [Log Data Length (4 bytes)] [Log Data]

// Record defines the binary wire format of each log entry inside the WAL:
type Record []byte

// WAL implements a write-ahead log for persisting ingested collector logs to disk.
// It manages sequential file rotation based on size thresholds and periodic fsync intervals.
type WAL struct {
	logger   *slog.Logger
	maxBytes uint
	ticker   *time.Ticker
	stopCh   chan struct{}
	dir      string
	file     *os.File
	mu       sync.Mutex
}

// New creates and initializes a new WAL instance in the specified directory.
// It creates the data directory if it does not exist, opens the initial WAL file,
// and starts a background goroutine for periodic fsync operations.
func New(dir string, maxBytes uint, syncInterval time.Duration, logger *slog.Logger) (*WAL, error) {
	if dir == "" {
		return nil, errors.New("dir is nil")
	}

	if maxBytes == 0 {
		return nil, errors.New("maxBytes cannot be 0")
	}

	if syncInterval <= 0 {
		return nil, errors.New("syncInterval must be greater than 0")
	}

	if logger == nil {
		return nil, errors.New("logger is nil")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create data dir: %w", err)
	}

	w := &WAL{
		logger:   logger,
		maxBytes: maxBytes,
		dir:      dir,
		stopCh:   make(chan struct{}),
	}

	if err := w.rotate(); err != nil {
		return nil, fmt.Errorf("cannot create WAL: %w", err)
	}

	w.ticker = time.NewTicker(syncInterval)

	go func() {
		for {
			select {
			case <-w.ticker.C:
				w.sync()
			case <-w.stopCh:
				return
			}
		}
	}()

	return w, nil
}

// Close stops the background sync ticker and performs a final sync of the active WAL file.
func (w *WAL) Close() {
	w.ticker.Stop()
	close(w.stopCh)

	w.sync()
}

// sync flushes in-memory file buffers to disk (fsync).
// If the active file size exceeds maxBytes, it triggers file rotation.
func (w *WAL) sync() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return
	}

	if err := w.file.Sync(); err != nil {
		w.logger.Error("cannot sync WAL file", "file", w.file.Name(), "error", err)
	}

	if s, err := w.file.Stat(); err != nil {
		w.logger.Error("cannot get stat of WAL file", "file", w.file.Name(), "error", err)
	} else if s.Size() >= int64(w.maxBytes) {
		if err := w.rotate(); err != nil {
			w.logger.Error("cannot rotate WAL file", "error", err)
		}
	}
}

// rotate closes the current active WAL file (if open) and opens a new segment file.
// Caller must hold w.mu before invoking rotate.
func (w *WAL) rotate() error {
	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("cannot sync WAL file: %w", err)
		}

		if err := w.file.Close(); err != nil {
			return fmt.Errorf("cannot close WAL file: %w", err)
		}
		w.file = nil
	}

	fileName := filepath.Join(w.dir, fmt.Sprintf("%d.wal", time.Now().UnixMilli()))

	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot create WAL file: %w", err)
	}

	w.file = f

	return nil
}

// Append writes data to the active WAL file under lock.
func (w *WAL) Append(rec Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Write(rec); err != nil {
		return fmt.Errorf("cannot store log: %w", err)
	}

	return nil
}
