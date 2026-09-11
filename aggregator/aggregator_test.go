package aggregator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Run("returns error with nil logger", func(t *testing.T) {
		agg, err := New(Config{
			Logger: nil,
		})

		assert.Nil(t, agg)
		assert.Error(t, err)
		assert.Errorf(t, err, "logger is nil")
	})

	t.Run("returns error with empty collectors path", func(t *testing.T) {
		agg, err := New(Config{
			Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		})

		assert.Nil(t, agg)
		assert.Error(t, err)
		assert.Errorf(t, err, "collectors path is nil")
	})

	t.Run("returns error with invalid extension", func(t *testing.T) {
		agg, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: "config.txt",
		})

		assert.Nil(t, agg)
		assert.Error(t, err)
		assert.Errorf(t, err, "collectors path must end in .yaml or .yml")
	})

	t.Run("success with valid config", func(t *testing.T) {
		v := viper.New()
		v.Set("wal.dir", t.TempDir())

		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		path := "config.yaml"
		agg, err := New(Config{
			Logger:         logger,
			CollectorsPath: path,
			Viper:          v,
		})

		assert.NoError(t, err)
		assert.NotNil(t, agg)
		assert.Equal(t, logger, agg.logger)
		assert.Equal(t, path, agg.collectorsPath)
		assert.NotNil(t, agg.collectors)
	})
}

func TestAggregator_Ingest(t *testing.T) {
	t.Run("invalid config initial setup returns with error", func(t *testing.T) {
		v := viper.New()
		v.Set("wal.dir", t.TempDir())
		agg, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: "nonexistent.yaml",
			Viper:          v,
		})
		assert.NoError(t, err)

		ctx := t.Context()

		err = agg.Ingest(ctx)
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

		agg, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: tmpFile.Name(),
			Viper:          v,
		})
		assert.NoError(t, err)

		ctx := t.Context()

		err = agg.Ingest(ctx)
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

		agg, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: tmpConfigFile.Name(),
		})
		assert.NoError(t, err)

		ctx := t.Context()

		// Run Ingest in a goroutine
		go func() {
			_ = agg.Ingest(ctx)
		}()

		// Wait for c1 to be created and active
		assert.Eventually(t, func() bool {
			agg.mu.RLock()
			defer agg.mu.RUnlock()
			c, exists := agg.collectors["c1"]
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
			agg.mu.RLock()
			defer agg.mu.RUnlock()
			_, c1Exists := agg.collectors["c1"]
			c2, c2Exists := agg.collectors["c2"]
			c3, c3Exists := agg.collectors["c3"]
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

		agg, err := New(Config{
			Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
			CollectorsPath: tmpConfigFile.Name(),
		})
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Start Ingest
		ingestDone := make(chan error, 1)
		go func() {
			ingestDone <- agg.Ingest(ctx)
		}()

		// Wait for collector to start
		assert.Eventually(t, func() bool {
			agg.mu.RLock()
			defer agg.mu.RUnlock()
			c, exists := agg.collectors["c1"]
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
