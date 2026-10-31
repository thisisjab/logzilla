package storage

import (
	"cmp"
	"sync"
	"sync/atomic"

	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/wal"
)

// DefaultMaxMemTableBytes is the default threshold (64MB) after which a MemTable is marked full.
const DefaultMaxMemTableBytes int64 = 64 * 1024 * 1024

// MemTable represents an in-memory sorted buffer for log records backed by a skipList.
type MemTable struct {
	mu        sync.RWMutex
	list      *skipList[entity.LogRecord]
	sizeBytes atomic.Int64
	maxBytes  int64
}

// NewMemTable constructs a new MemTable with the given size limit in bytes.
// If maxBytes <= 0, DefaultMaxMemTableBytes is used.
func NewMemTable(maxBytes int64) *MemTable {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxMemTableBytes
	}

	compareLogRecord := func(a, b entity.LogRecord) int {
		if c := cmp.Compare(a.Timestamp, b.Timestamp); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	}

	return &MemTable{
		list:     newSkipList(compareLogRecord),
		maxBytes: maxBytes,
	}
}

// Put inserts a LogRecord into the MemTable and returns whether the MemTable has reached its capacity.
func (m *MemTable) Put(record entity.LogRecord) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	entryBytes := wal.RecordHeaderSize + int64(len(record.CollectorName)+len(record.Raw)+len(record.Message))
	m.list.Add(record)
	newSize := m.sizeBytes.Add(entryBytes)

	return newSize >= m.maxBytes
}

// SizeBytes returns the approximate accumulated payload size in bytes.
func (m *MemTable) SizeBytes() int64 {
	return m.sizeBytes.Load()
}

// Len returns the number of log records in the MemTable.
func (m *MemTable) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.list.Len()
}

// Iterator returns an iterator over the underlying skip list in ascending order.
// The caller should not mutate the MemTable while iterating.
func (m *MemTable) Iterator() *skipListIterator[entity.LogRecord] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.list.Iterator()
}
