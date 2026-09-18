package ingestor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer lis.Close()
	return lis.Addr().(*net.TCPAddr).Port
}

func TestNew(t *testing.T) {
	t.Run("returns error with nil logger", func(t *testing.T) {
		ing, err := New(Config{
			Logger: nil,
		})

		assert.Nil(t, ing)
		assert.Error(t, err)
		assert.Errorf(t, err, "logger is nil")
	})

	t.Run("returns error with empty collectors path", func(t *testing.T) {
		ing, err := New(Config{
			Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		})

		assert.Nil(t, ing)
		assert.Error(t, err)
		assert.Errorf(t, err, "collectors path is nil")
	})

	t.Run("returns error with invalid extension", func(t *testing.T) {
		ing, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: "config.txt",
		})

		assert.Nil(t, ing)
		assert.Error(t, err)
		assert.Errorf(t, err, "collectors path must end in .yaml or .yml")
	})

	t.Run("success with valid config", func(t *testing.T) {
		v := viper.New()
		v.Set("wal.dir", t.TempDir())

		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		path := "config.yaml"
		ing, err := New(Config{
			Logger:         logger,
			CollectorsPath: path,
			viper:          v,
		})

		assert.NoError(t, err)
		assert.NotNil(t, ing)
		assert.Equal(t, logger, ing.logger)
		assert.Equal(t, path, ing.collectorsPath)
		assert.NotNil(t, ing.collectors)
	})
}

func TestIngestor_PollWAL(t *testing.T) {
	walDir := t.TempDir()
	v := viper.New()
	v.Set("wal.dir", walDir)

	tmpConfigFile, err := os.CreateTemp("", "ing_config_*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpConfigFile.Name())
	_, err = fmt.Fprintf(tmpConfigFile, "wal:\n  dir: %s\ncollectors: {}\n", walDir)
	assert.NoError(t, err)
	tmpConfigFile.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ing, err := New(Config{
		Logger:         logger,
		CollectorsPath: tmpConfigFile.Name(),
		viper:          v,
	})
	assert.NoError(t, err)

	// Write segments
	f1Data := []byte("seg 1")
	f2Data := []byte("seg 2")
	assert.NoError(t, os.WriteFile(filepath.Join(walDir, "1000.wal"), f1Data, 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(walDir, "2000.wal"), f2Data, 0644))

	// Initial poll with lastWalID = 0
	ch := ing.PollWAL(context.Background(), 0)
	var segs []WALSegment
	for s := range ch {
		assert.NoError(t, s.Err)
		segs = append(segs, s)
	}
	assert.Len(t, segs, 2)
	assert.Equal(t, int64(1000), segs[0].ID)
	assert.Equal(t, f1Data, segs[0].Data)
	assert.Equal(t, int64(2000), segs[1].ID)
	assert.Equal(t, f2Data, segs[1].Data)

	// Subsequent poll with lastWalID = 1000
	ch2 := ing.PollWAL(context.Background(), 1000)
	var segs2 []WALSegment
	for s := range ch2 {
		assert.NoError(t, s.Err)
		segs2 = append(segs2, s)
	}
	assert.Len(t, segs2, 1)
	assert.Equal(t, int64(2000), segs2[0].ID)

	// 1000.wal should have been deleted
	assert.NoFileExists(t, filepath.Join(walDir, "1000.wal"))
	assert.FileExists(t, filepath.Join(walDir, "2000.wal"))
}

func TestIngestor_Ingest(t *testing.T) {
	t.Run("invalid config initial setup returns with error", func(t *testing.T) {
		v := viper.New()
		v.Set("wal.dir", t.TempDir())
		ing, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: "nonexistent.yaml",
			viper:          v,
		})
		assert.NoError(t, err)

		ctx := t.Context()

		err = ing.Ingest(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read config")
	})

	t.Run("with invalid yaml structure initial config load fails", func(t *testing.T) {
		v := viper.New()
		v.Set("wal.dir", t.TempDir())
		tmpFile, err := os.CreateTemp("", "invalid_struct_*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		// valid yaml syntax, but structure is wrong for unmarshal (collectors is a list instead of a map)
		_, err = tmpFile.WriteString("collectors:\n  - type: file")
		assert.NoError(t, err)
		tmpFile.Close()

		ing, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: tmpFile.Name(),
			viper:          v,
		})
		assert.NoError(t, err)

		ctx := t.Context()

		err = ing.Ingest(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load initial config")
	})

	t.Run("by updating config file config reloads", func(t *testing.T) {
		walDir := t.TempDir()

		// Create dummy files for file collector validation
		log1, err := os.CreateTemp("", "log1_*.log")
		assert.NoError(t, err)
		defer os.Remove(log1.Name())
		log1.Close()

		log2, err := os.CreateTemp("", "log2_*.log")
		assert.NoError(t, err)
		defer os.Remove(log2.Name())
		log2.Close()

		// Initial config with c1 active
		configContent := fmt.Sprintf(`
wal:
  dir: %s
collectors:
  c1:
    type: file
    isActive: true
    args:
      path: %s
`, walDir, log1.Name())

		tmpConfigFile, err := os.CreateTemp("", "config_*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpConfigFile.Name())

		_, err = tmpConfigFile.WriteString(configContent)
		assert.NoError(t, err)
		tmpConfigFile.Close()

		ing, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: tmpConfigFile.Name(),
		})
		assert.NoError(t, err)

		ctx := t.Context()

		// Run Ingest in a goroutine
		go func() {
			_ = ing.Ingest(ctx)
		}()

		// Wait for c1 to be created and active
		assert.Eventually(t, func() bool {
			ing.collectorsMu.RLock()
			defer ing.collectorsMu.RUnlock()
			c, exists := ing.collectors["c1"]
			return exists && c.isActive
		}, 5*time.Second, 100*time.Millisecond)

		// Now update config:
		// 1. Remove c1
		// 2. Add c2 (active)
		// 3. Add c3 (inactive/disabled)
		updatedConfigContent := fmt.Sprintf(`
wal:
  dir: %s
collectors:
  c2:
    type: file
    isActive: true
    args:
      path: %s
  c3:
    type: file
    isActive: false
    args:
      path: %s
`, walDir, log2.Name(), log1.Name())

		// Truncate and rewrite config file
		f, err := os.OpenFile(tmpConfigFile.Name(), os.O_WRONLY|os.O_TRUNC, 0644)
		assert.NoError(t, err)
		_, err = f.WriteString(updatedConfigContent)
		assert.NoError(t, err)
		err = f.Sync()
		assert.NoError(t, err)
		f.Close()

		// Wait for hotreload to apply changes:
		// c1 should be removed
		// c2 should be active
		// c3 should be inactive
		assert.Eventually(t, func() bool {
			ing.collectorsMu.RLock()
			defer ing.collectorsMu.RUnlock()
			_, c1Exists := ing.collectors["c1"]
			c2, c2Exists := ing.collectors["c2"]
			c3, c3Exists := ing.collectors["c3"]
			return !c1Exists && c2Exists && c2.isActive && c3Exists && !c3.isActive
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("on context.done all collectors are stopped", func(t *testing.T) {
		walDir := t.TempDir()

		logFile, err := os.CreateTemp("", "log_*.log")
		assert.NoError(t, err)
		defer os.Remove(logFile.Name())
		logFile.Close()

		configContent := fmt.Sprintf(`
wal:
  dir: %s
collectors:
  c1:
    type: file
    isActive: true
    args:
      path: %s
`, walDir, logFile.Name())

		tmpConfigFile, err := os.CreateTemp("", "config_*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpConfigFile.Name())
		_, err = tmpConfigFile.WriteString(configContent)
		assert.NoError(t, err)
		tmpConfigFile.Close()

		ing, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: tmpConfigFile.Name(),
		})
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Start Ingest
		ingestDone := make(chan error, 1)
		go func() {
			ingestDone <- ing.Ingest(ctx)
		}()

		// Wait for collector to start
		assert.Eventually(t, func() bool {
			ing.collectorsMu.RLock()
			defer ing.collectorsMu.RUnlock()
			c, exists := ing.collectors["c1"]
			return exists && c.isActive
		}, 5*time.Second, 100*time.Millisecond)

		// Cancel context
		cancel()

		// Ingest should return nil
		select {
		case err := <-ingestDone:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Ingest did not exit in time")
		}
	})
}
