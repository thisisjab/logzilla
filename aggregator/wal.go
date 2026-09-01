package aggregator

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// record defines the binary wire format of each log entry inside the WAL:
// [ULID (16 bytes)] [Collector Name Length (4 bytes)] [Collector Name] [Log Data Length (4 bytes)] [Log Data]
type record []byte

// wal implements a write-ahead log for persisting ingested collector logs to disk.
// It manages sequential file rotation based on size thresholds and periodic fsync intervals.
type wal struct {
	logger   *slog.Logger
	maxBytes uint
	ticker   *time.Ticker
	stopCh   chan struct{}
	dir      string
	file     *os.File
	mu       sync.Mutex
}

// newWAL creates and initializes a new WAL instance in the specified directory.
// It creates the data directory if it does not exist, opens the initial WAL file,
// and starts a background goroutine for periodic fsync operations.
func newWAL(dir string, maxBytes uint, syncInterval time.Duration, logger *slog.Logger) (*wal, error) {
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

	w := &wal{
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

// close stops the background sync ticker and performs a final sync of the active WAL file.
func (w *wal) close() {
	w.ticker.Stop()
	close(w.stopCh)

	w.sync()
}

// sync flushes in-memory file buffers to disk (fsync).
// If the active file size exceeds maxBytes, it triggers file rotation.
func (w *wal) sync() {
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
func (w *wal) rotate() error {
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

// append serializes the given collector name and log data into a WAL record
// and writes it to the active WAL file under lock.
func (w *wal) append(collector, data string) error {
	rec := encodeRecord(collector, data)

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Write(rec); err != nil {
		return fmt.Errorf("cannot store log: %w", err)
	}

	return nil
}

// encodeRecord marshals a collector identifier and log payload into the binary record layout:
// [16B ULID] [4B BigEndian ColLen] [ColName] [4B BigEndian DataLen] [LogData]
func encodeRecord(collector, data string) record {
	logID := ulid.Make()

	colLen := uint32(len(collector))
	dataLen := uint32(len(data))

	totalLen := 16 + 4 + int(colLen) + 4 + int(dataLen)
	rec := make(record, totalLen)

	// Write ULID (16 bytes)
	copy(rec[0:16], logID[:])

	// Write Collector Name Length (4 bytes)
	binary.BigEndian.PutUint32(rec[16:20], colLen)

	// Write Collector Name
	copy(rec[20:20+colLen], collector)

	// Write Log Data Length (4 bytes)
	binary.BigEndian.PutUint32(rec[20+colLen:24+colLen], dataLen)

	// Write Log Data
	copy(rec[24+colLen:24+colLen+dataLen], data)

	return rec
}
