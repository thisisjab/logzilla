package storage

import (
	"slices"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/wal"
)

func TestNewMemTable(t *testing.T) {
	t.Run("uses DefaultMaxMemTableBytes when non-positive capacity provided", func(t *testing.T) {
		m := NewMemTable(0)
		require.NotNil(t, m)
		assert.Equal(t, DefaultMaxMemTableBytes, m.maxBytes)
		assert.Equal(t, int64(0), m.SizeBytes())
		assert.Equal(t, 0, m.Len())
	})

	t.Run("uses custom capacity", func(t *testing.T) {
		m := NewMemTable(1024)
		require.NotNil(t, m)
		assert.Equal(t, int64(1024), m.maxBytes)
	})
}

func TestMemTable_PutAndOrdering(t *testing.T) {
	t.Run("orders records by timestamp ascending and ID tiebreaker", func(t *testing.T) {
		m := NewMemTable(1024 * 1024)
		now := time.Now().UnixNano()

		id1 := ulid.Make()
		id2 := ulid.Make()
		if id1.Compare(id2) > 0 {
			id1, id2 = id2, id1
		}

		rec1 := entity.LogRecord{
			ID:            id1,
			Timestamp:     now,
			CollectorName: "collector",
			Raw:           "raw-1",
			Message:       "msg-1",
		}
		rec2 := entity.LogRecord{
			ID:            id2,
			Timestamp:     now,
			CollectorName: "collector",
			Raw:           "raw-2",
			Message:       "msg-2",
		}
		rec3 := entity.LogRecord{
			ID:            ulid.Make(),
			Timestamp:     now - 1000,
			CollectorName: "collector",
			Raw:           "raw-3",
			Message:       "msg-3",
		}

		m.Put(rec2)
		m.Put(rec1)
		m.Put(rec3)

		assert.Equal(t, 3, m.Len())

		it := m.Iterator()
		var records []entity.LogRecord
		for it.Next() {
			records = append(records, it.Value())
		}

		require.Len(t, records, 3)
		assert.Equal(t, rec3.ID, records[0].ID)
		assert.Equal(t, rec1.ID, records[1].ID)
		assert.Equal(t, rec2.ID, records[2].ID)
	})
}

func TestMemTable_CapacityAccounting(t *testing.T) {
	t.Run("reports full when reaching threshold", func(t *testing.T) {
		entrySize := wal.RecordHeaderSize + int64(len("syslog")+len("raw")+len("hello"))
		m := NewMemTable(entrySize * 2)

		rec1 := entity.LogRecord{
			ID:            ulid.Make(),
			Timestamp:     1,
			CollectorName: "syslog",
			Raw:           "raw",
			Message:       "hello",
		}
		rec2 := entity.LogRecord{
			ID:            ulid.Make(),
			Timestamp:     2,
			CollectorName: "syslog",
			Raw:           "raw",
			Message:       "hello",
		}

		isFull := m.Put(rec1)
		assert.False(t, isFull)
		assert.Equal(t, entrySize, m.SizeBytes())

		isFull = m.Put(rec2)
		assert.True(t, isFull)
		assert.Equal(t, entrySize*2, m.SizeBytes())
	})
}

func TestMemTable_Iterator(t *testing.T) {
	t.Run("iterates all records in order", func(t *testing.T) {
		m := NewMemTable(1024 * 1024)
		for i := 10; i >= 1; i-- {
			m.Put(entity.LogRecord{
				ID:        ulid.Make(),
				Timestamp: int64(i * 100),
				Message:   "item",
			})
		}

		it := m.Iterator()
		var timestamps []int64
		for it.Next() {
			timestamps = append(timestamps, it.Value().Timestamp)
		}

		assert.Len(t, timestamps, 10)
		assert.True(t, slices.IsSorted(timestamps))
	})
}
