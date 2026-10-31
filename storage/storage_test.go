package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/wal"
)

func TestStorage_Put(t *testing.T) {
	t.Run("inserts records into active memtable", func(t *testing.T) {
		s := NewStorage(1024 * 1024)
		defer s.Close()

		rec := entity.LogRecord{
			ID:            ulid.Make(),
			Timestamp:     time.Now().UnixMilli(),
			CollectorName: "syslog",
			Raw:           "raw log line",
			Message:       "message",
		}

		err := s.Put(rec)
		require.NoError(t, err)

		active := s.ActiveMemTable()
		require.NotNil(t, active)
		assert.Equal(t, 1, active.Len())
	})

	t.Run("swaps active memtable when threshold reached", func(t *testing.T) {
		recordSize := wal.RecordHeaderSize + int64(len("syslog")+len("raw")+len("msg"))
		s := NewStorage(recordSize) // threshold reached on 1st record
		defer s.Close()

		rec1 := entity.LogRecord{
			ID:            ulid.Make(),
			Timestamp:     100,
			CollectorName: "syslog",
			Raw:           "raw",
			Message:       "msg",
		}
		rec2 := entity.LogRecord{
			ID:            ulid.Make(),
			Timestamp:     200,
			CollectorName: "syslog",
			Raw:           "raw",
			Message:       "msg",
		}

		err := s.Put(rec1)
		require.NoError(t, err)

		// After putting rec1, capacity reached and swapped: active is now a new MemTable.
		assert.Equal(t, 0, s.ActiveMemTable().Len())

		err = s.Put(rec2)
		require.NoError(t, err)
		assert.Equal(t, 0, s.ActiveMemTable().Len()) // swapped again
	})
}

func TestStorage_WriteStallAndBackpressure(t *testing.T) {
	t.Run("stalls when immutable is occupied until released", func(t *testing.T) {
		recordSize := wal.RecordHeaderSize + int64(len("col")+len("r")+len("m"))
		s := NewStorage(recordSize)
		defer s.Close()

		// Manually set immutable to simulate an in-progress SSTable flush
		s.mu.Lock()
		s.immutable = NewMemTable(recordSize)
		s.mu.Unlock()

		putStarted := make(chan struct{})
		putDone := make(chan error, 1)

		go func() {
			close(putStarted)
			err := s.Put(entity.LogRecord{
				ID:            ulid.Make(),
				Timestamp:     1,
				CollectorName: "col",
				Raw:           "r",
				Message:       "m",
			})
			putDone <- err
		}()

		<-putStarted

		// Ensure Put is stalled because active is full and immutable != nil
		select {
		case <-putDone:
			t.Fatal("expected Put to stall while immutable is non-nil")
		case <-time.After(50 * time.Millisecond):
		}

		// Simulate flush completing by clearing immutable and notifying condition
		s.mu.Lock()
		s.immutable = nil
		s.flushCond.Signal()
		s.mu.Unlock()

		select {
		case err := <-putDone:
			require.NoError(t, err)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Put did not resume after flush condition signaled")
		}
	})
}

func TestStorage_Close(t *testing.T) {
	t.Run("unblocks stalled writers with ErrStorageClosed", func(t *testing.T) {
		recordSize := wal.RecordHeaderSize + int64(10)
		s := NewStorage(recordSize)

		s.mu.Lock()
		s.immutable = NewMemTable(recordSize)
		s.mu.Unlock()

		done := make(chan error, 1)
		go func() {
			err := s.Put(entity.LogRecord{
				ID:        ulid.Make(),
				Timestamp: 1,
				Raw:       "0123456789",
			})
			done <- err
		}()

		time.Sleep(20 * time.Millisecond)
		require.NoError(t, s.Close())

		select {
		case err := <-done:
			assert.ErrorIs(t, err, ErrStorageClosed)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("close did not unblock stalled writer")
		}

		// Subsequent Put fails immediately
		err := s.Put(entity.LogRecord{})
		assert.ErrorIs(t, err, ErrStorageClosed)
	})
}

func TestStorage_ConcurrentWrites(t *testing.T) {
	t.Run("safely handles concurrent Put calls", func(t *testing.T) {
		s := NewStorage(1024 * 1024)
		defer s.Close()

		var wg sync.WaitGroup
		const workers = 8
		const countPerWorker = 100

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < countPerWorker; i++ {
					err := s.Put(entity.LogRecord{
						ID:            ulid.Make(),
						Timestamp:     time.Now().UnixNano(),
						CollectorName: "collector",
						Raw:           "raw log line",
						Message:       "message text",
					})
					assert.NoError(t, err)
				}
			}()
		}

		wg.Wait()
		assert.GreaterOrEqual(t, s.ActiveMemTable().Len(), 0)
	})
}
