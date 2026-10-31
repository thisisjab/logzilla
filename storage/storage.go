package storage

import (
	"errors"
	"fmt"
	"sync"

	"github.com/thisisjab/logzilla/entity"
)

var (
	// ErrStorageClosed is returned when an operation is performed on a closed Storage instance.
	ErrStorageClosed = errors.New("storage is closed")
)

// Storage manages active and immutable MemTables, backpressure, and persistence.
type Storage struct {
	mu               sync.Mutex
	flushCond        *sync.Cond
	active           *MemTable
	immutable        *MemTable
	maxMemTableBytes int64
	closed           bool
}

// NewStorage constructs a new Storage manager with the specified per-MemTable byte limit.
func NewStorage(maxMemTableBytes int64) *Storage {
	if maxMemTableBytes <= 0 {
		maxMemTableBytes = DefaultMaxMemTableBytes
	}

	s := &Storage{
		active:           NewMemTable(maxMemTableBytes),
		maxMemTableBytes: maxMemTableBytes,
	}
	s.flushCond = sync.NewCond(&s.mu)
	return s
}

// Put appends a LogRecord to the active MemTable.
// If active MemTable capacity is reached, it triggers a swap to immutable.
// If an immutable MemTable is still flushing to disk, Put stalls until the flush completes.
func (s *Storage) Put(record entity.LogRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	isFull := s.active.Put(record)
	if isFull {
		if err := s.swapLocked(); err != nil {
			return fmt.Errorf("swap memtable: %w", err)
		}
	}

	return nil
}

// swapLocked moves the full active MemTable to immutable and creates a fresh active MemTable.
// If immutable is not nil (i.e. flush in progress), it waits on flushCond.
// Must be called with s.mu held.
func (s *Storage) swapLocked() error {
	for s.immutable != nil && !s.closed {
		s.flushCond.Wait()
	}

	if s.closed {
		return ErrStorageClosed
	}

	s.immutable = s.active
	s.active = NewMemTable(s.maxMemTableBytes)

	// TODO: flush immutable to SSTable on disk.
	// Once SSTable persistence is implemented, trigger background flush here.
	// For now, simulate flush completion so writes do not stall indefinitely.
	s.immutable = nil
	s.flushCond.Signal()

	return nil
}

// ActiveMemTable returns the currently active MemTable.
func (s *Storage) ActiveMemTable() *MemTable {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// ImmutableMemTable returns the immutable MemTable currently flushing, or nil.
func (s *Storage) ImmutableMemTable() *MemTable {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.immutable
}

// Close stops the storage manager and wakes up any stalled goroutines.
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.flushCond.Broadcast()
	return nil
}
