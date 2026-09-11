package wal

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWAL(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("returns error with empty dir", func(t *testing.T) {
		w, err := New("", 1024, 100*time.Millisecond, logger)
		assert.Nil(t, w)
		assert.Error(t, err)
		assert.EqualError(t, err, "dir is nil")
	})

	t.Run("returns error with zero maxBytes", func(t *testing.T) {
		w, err := New(t.TempDir(), 0, 100*time.Millisecond, logger)
		assert.Nil(t, w)
		assert.Error(t, err)
		assert.EqualError(t, err, "maxBytes cannot be 0")
	})

	t.Run("returns error with zero syncInterval", func(t *testing.T) {
		w, err := New(t.TempDir(), 1024, 0, logger)
		assert.Nil(t, w)
		assert.Error(t, err)
		assert.EqualError(t, err, "syncInterval must be greater than 0")
	})

	t.Run("returns error with negative syncInterval", func(t *testing.T) {
		w, err := New(t.TempDir(), 1024, -1*time.Second, logger)
		assert.Nil(t, w)
		assert.Error(t, err)
		assert.EqualError(t, err, "syncInterval must be greater than 0")
	})

	t.Run("returns error with nil logger", func(t *testing.T) {
		w, err := New(t.TempDir(), 1024, 100*time.Millisecond, nil)
		assert.Nil(t, w)
		assert.Error(t, err)
		assert.EqualError(t, err, "logger is nil")
	})

	t.Run("returns error when creating directory fails", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "wal_blocking_file_*")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		w, err := New(tmpFile.Name(), 1024, 100*time.Millisecond, logger)
		assert.Nil(t, w)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot create data dir")
	})

	t.Run("creates directory if it does not exist", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "wal_data")
		assert.NoDirExists(t, dir)

		w, err := New(dir, 1024, 100*time.Millisecond, logger)
		assert.NoError(t, err)
		assert.NotNil(t, w)
		defer w.Close()

		assert.DirExists(t, dir)
	})

	t.Run("success with valid config", func(t *testing.T) {
		dir := t.TempDir()
		maxBytes := uint(1024)
		syncInterval := 100 * time.Millisecond

		w, err := New(dir, maxBytes, syncInterval, logger)
		assert.NoError(t, err)
		assert.NotNil(t, w)
		defer w.Close()

		assert.DirExists(t, dir)
		assert.NotNil(t, w.ticker)
		assert.NotNil(t, w.file)
		assert.FileExists(t, w.file.Name())
		assert.Equal(t, dir, w.dir)
		assert.Equal(t, maxBytes, w.maxBytes)
		assert.Equal(t, logger, w.logger)

		entries, err := os.ReadDir(dir)
		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.False(t, entries[0].IsDir())
		assert.True(t, strings.HasSuffix(entries[0].Name(), ".wal"))
	})
}

func TestRotate(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("creates file with 0644 permissions when file is nil", func(t *testing.T) {
		dir := t.TempDir()
		w := &WAL{
			dir:    dir,
			logger: logger,
		}

		err := w.rotate()
		assert.NoError(t, err)
		assert.NotNil(t, w.file)
		defer w.file.Close()

		info, err := os.Stat(w.file.Name())
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
		assert.True(t, strings.HasSuffix(w.file.Name(), ".wal"))
		assert.Equal(t, dir, filepath.Dir(w.file.Name()))
	})

	t.Run("syncs and closes existing file before creating new one", func(t *testing.T) {
		dir := t.TempDir()
		w, err := New(dir, 1024, 100*time.Millisecond, logger)
		assert.NoError(t, err)
		defer w.Close()

		// Write data to the active file
		err = w.Append(Record("test log line"))
		assert.NoError(t, err)

		oldFile := w.file
		oldFileName := oldFile.Name()

		time.Sleep(2 * time.Millisecond) // Ensure distinct millisecond timestamp

		w.mu.Lock()
		err = w.rotate()
		w.mu.Unlock()
		assert.NoError(t, err)

		// 1. Old file is closed
		assert.ErrorIs(t, oldFile.Sync(), os.ErrClosed)

		// 2. Old file data is intact on disk
		content, err := os.ReadFile(oldFileName)
		assert.NoError(t, err)
		assert.NotEmpty(t, content)

		// 3. New file is created and distinct
		assert.NotNil(t, w.file)
		assert.NotEqual(t, oldFileName, w.file.Name())
		assert.FileExists(t, w.file.Name())
	})

	t.Run("returns error when sync fails on existing file", func(t *testing.T) {
		dir := t.TempDir()
		w := &WAL{
			dir:    dir,
			logger: logger,
		}

		// Open a valid file first
		err := w.rotate()
		assert.NoError(t, err)

		// Close the underlying file handle prematurely to force Sync() to fail
		_ = w.file.Close()

		err = w.rotate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot sync WAL file")
	})
}

func TestAppend(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("successfully appends single and multiple records to WAL file", func(t *testing.T) {
		dir := t.TempDir()
		w, err := New(dir, 1024*1024, 100*time.Millisecond, logger)
		assert.NoError(t, err)

		fileName := w.file.Name()

		// 1. Append first record
		rec1 := Record("collector_a: message 1")
		err = w.Append(rec1)
		assert.NoError(t, err)

		// 2. Append second record
		rec2 := Record("collector_b: message 2")
		err = w.Append(rec2)
		assert.NoError(t, err)

		w.Close()

		// Read back and compare binary content
		data, err := os.ReadFile(fileName)
		assert.NoError(t, err)

		assert.Contains(t, string(data), "collector_a: message 1")
		assert.Contains(t, string(data), "collector_b: message 2")

		// Total size must be sum of both encoded records:
		expectedSize := len(rec1) + len(rec2)
		assert.Len(t, data, expectedSize)
	})

	t.Run("returns error when write fails", func(t *testing.T) {
		dir := t.TempDir()
		w, err := New(dir, 1024, 100*time.Millisecond, logger)
		assert.NoError(t, err)
		defer w.Close()

		// Close file to simulate write failure
		_ = w.file.Close()

		err = w.Append(Record("some data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot store log")
	})

	t.Run("concurrent appends from multiple goroutines", func(t *testing.T) {
		dir := t.TempDir()
		w, err := New(dir, 1024*1024, 10*time.Millisecond, logger)
		assert.NoError(t, err)
		defer w.Close()

		const numGoroutines = 10
		const logsPerGoroutine = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := range numGoroutines {
			go func(workerID int) {
				defer wg.Done()
				for j := range logsPerGoroutine {
					logMsg := Record(fmt.Sprintf("worker %d message %d", workerID, j))
					appendErr := w.Append(logMsg)
					assert.NoError(t, appendErr)
				}
			}(i)
		}

		wg.Wait()

		// Verify total written bytes matches expected sum
		entries, err := os.ReadDir(dir)
		assert.NoError(t, err)
		assert.NotEmpty(t, entries)

		totalBytes := 0
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".wal") {
				info, err := entry.Info()
				assert.NoError(t, err)
				totalBytes += int(info.Size())
			}
		}

		expectedTotalBytes := 0
		for i := range numGoroutines {
			for j := range logsPerGoroutine {
				logMsg := fmt.Sprintf("worker %d message %d", i, j)
				expectedTotalBytes += len(logMsg)
			}
		}

		assert.Equal(t, expectedTotalBytes, totalBytes)
	})
}

func TestSync(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("does not panic when file is nil", func(t *testing.T) {
		w := &WAL{
			file:   nil,
			logger: logger,
		}

		assert.NotPanics(t, func() {
			w.sync()
		})
	})

	t.Run("does not rotate if file size is below maxBytes", func(t *testing.T) {
		dir := t.TempDir()
		// maxBytes = 100
		w, err := New(dir, 100, 10*time.Second, logger)
		assert.NoError(t, err)
		defer w.Close()

		initialFileName := w.file.Name()

		// Append record of size 30 bytes (< 100)
		rec := Record(strings.Repeat("a", 30))
		err = w.Append(rec)
		assert.NoError(t, err)

		w.sync()

		assert.Equal(t, initialFileName, w.file.Name())
	})

	t.Run("rotates file when size reaches or exceeds maxBytes", func(t *testing.T) {
		dir := t.TempDir()
		// maxBytes = 50
		w, err := New(dir, 50, 10*time.Second, logger)
		assert.NoError(t, err)
		defer w.Close()

		initialFile := w.file
		initialFileName := initialFile.Name()

		// 1. First record: 30 bytes (< 50)
		rec := Record(strings.Repeat("a", 30))
		err = w.Append(rec)
		assert.NoError(t, err)

		w.sync()
		assert.Equal(t, initialFileName, w.file.Name(), "should not rotate when size < maxBytes")

		// 2. Second record: total size now 60 bytes (>= 50)
		err = w.Append(rec)
		assert.NoError(t, err)

		time.Sleep(2 * time.Millisecond) // Ensure unique timestamp for new file

		// 3. sync() triggers rotation
		w.sync()

		// Verify rotation happened:
		// - Active file changed
		assert.NotEqual(t, initialFileName, w.file.Name())
		assert.FileExists(t, w.file.Name())

		// - Old file was synced and closed
		assert.ErrorIs(t, initialFile.Sync(), os.ErrClosed)

		// - Old file on disk contains all 60 bytes
		oldFileInfo, err := os.Stat(initialFileName)
		assert.NoError(t, err)
		assert.Equal(t, int64(60), oldFileInfo.Size())
	})

	t.Run("background ticker automatically rotates file when maxBytes exceeded", func(t *testing.T) {
		dir := t.TempDir()
		// Small syncInterval (20ms) and small maxBytes (50 bytes)
		w, err := New(dir, 50, 20*time.Millisecond, logger)
		assert.NoError(t, err)
		defer w.Close()

		initialFileName := w.file.Name()

		// Write enough to exceed 50 bytes (30 + 30 = 60 bytes)
		rec := Record(strings.Repeat("a", 30))
		err = w.Append(rec)
		assert.NoError(t, err)
		err = w.Append(rec)
		assert.NoError(t, err)

		// Wait for background ticker goroutine to trigger sync and rotate
		assert.Eventually(t, func() bool {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.file.Name() != initialFileName
		}, 3*time.Second, 20*time.Millisecond)

		// Verify that both old and new files exist
		entries, err := os.ReadDir(dir)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})
}

func TestClose(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("stops ticker, closes stopCh, and performs final sync on file", func(t *testing.T) {
		dir := t.TempDir()
		// Large sync interval so background ticker doesn't trigger during test
		w, err := New(dir, 1024*1024, 1*time.Hour, logger)
		assert.NoError(t, err)

		fileName := w.file.Name()

		// Write a log entry
		rec := Record("closing data payload")
		err = w.Append(rec)
		assert.NoError(t, err)

		// Call Close()
		w.Close()

		// 1. Verify stopCh is closed
		select {
		case _, ok := <-w.stopCh:
			assert.False(t, ok, "stopCh should be closed")
		default:
			t.Fatal("stopCh is not closed or blocked")
		}

		// 2. Verify final sync wrote the log data to disk
		data, err := os.ReadFile(fileName)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "closing data payload")

		assert.Len(t, data, len(rec))
	})
}
