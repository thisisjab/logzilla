package ingestor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"github.com/thisisjab/logzilla/wal"
)

// WALSegment represents a WAL file's metadata and data, or an error encountered during polling.
type WALSegment struct {
	ID   int64
	Data []byte
	Err  error
}

// Ingestor collects logs from collectors, stores in WALs,
// and eventually sends WALs to the cluster node to be stored.
type Ingestor struct {
	v      *viper.Viper
	logger *slog.Logger
	// collectorsPath defines the yaml file that collectors and cluster nodes are read from.
	// In case of any change to the file, it will trigger a hot reload.
	collectorsPath string
	// collectorsMu guards collectors map during hot reloads and termination.
	collectorsMu sync.RWMutex
	// pollMu serializes WAL polling operations and disk cleanup.
	pollMu sync.Mutex
	// collectorsWg is used to wait till all collectors exit in case of termination
	collectorWg sync.WaitGroup
	// collectors holds list of collectors with their state.
	collectors map[string]*collectorState
	// wal handles persisting logs to disk.
	wal *wal.WAL
	// cb is the callback function for getting ingested logs from collectors.
	cb func(collectorName, data string) error
}

type Config struct {
	CollectorsPath string
	Logger         *slog.Logger
	viper          *viper.Viper
}

func New(cfg Config) (*Ingestor, error) {
	if cfg.Logger == nil {
		return nil, errors.New("logger is nil")
	}

	if cfg.CollectorsPath == "" {
		return nil, errors.New("collectors path is nil")
	}

	if !strings.HasSuffix(cfg.CollectorsPath, ".yml") && !strings.HasSuffix(cfg.CollectorsPath, ".yaml") {
		return nil, errors.New("collectors path must end in .yaml or .yml")
	}

	v := cfg.viper
	if v == nil { // NOTE: when testing, viper is passed by the test func
		v = viper.New()
	}

	v.SetConfigFile(cfg.CollectorsPath)
	// TODO: add these configs to hot reload config struct as well
	v.SetDefault("wal.dir", "./data/wal")
	v.SetDefault("wal.max_bytes", uint(10*1024*1024))
	v.SetDefault("wal.sync_interval", 100*time.Millisecond)
	v.SetDefault("grpcPort", 9393)

	// Attempt reading static config (if config file exists already)
	_ = v.ReadInConfig()

	walDir := v.GetString("wal.dir")
	walMaxBytes := v.GetUint("wal.max_bytes")
	walSyncInterval := v.GetDuration("wal.sync_interval")

	w, err := wal.New(walDir, walMaxBytes, walSyncInterval, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot create WAL: %w", err)
	}

	ing := &Ingestor{
		v:              v,
		logger:         cfg.Logger,
		collectorsPath: cfg.CollectorsPath,

		collectors: make(map[string]*collectorState),
		wal:        w,
	}

	cb := func(collectorName, data string) error {
		err := ing.wal.Append(encodeRecord(collectorName, data))

		if err != nil {
			return fmt.Errorf("cannot append to WAL: %w", err)
		}

		return nil
	}

	ing.cb = cb

	return ing, nil
}

// PollWAL polls unread closed WAL files and streams them through a channel.
// Implements the WALPoller interface needed by the gRPC service.
func (ing *Ingestor) PollWAL(ctx context.Context, lastWalID int64) <-chan WALSegment {
	ch := make(chan WALSegment)

	go func() {
		defer close(ch)

		ing.pollMu.Lock()
		defer ing.pollMu.Unlock()

		if ing.wal == nil {
			select {
			case ch <- WALSegment{Err: errors.New("wal is not initialized")}:
			case <-ctx.Done():
			}
			return
		}

		dir := ing.wal.Dir()
		entries, err := os.ReadDir(dir)
		if err != nil {
			select {
			case ch <- WALSegment{Err: fmt.Errorf("cannot read WAL directory: %w", err)}:
			case <-ctx.Done():
			}
			return
		}

		activeFile := ing.wal.ActiveFileName()
		type walCandidate struct {
			id   int64
			name string
		}
		var candidates []walCandidate

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".wal") || name == activeFile {
				continue
			}

			idStr := strings.TrimSuffix(name, ".wal")
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				ing.logger.Warn("skipping WAL file with invalid timestamp filename", "file", name, "error", err)
				continue
			}

			if lastWalID > 0 && id <= lastWalID {
				filePath := filepath.Join(dir, name)
				if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
					ing.logger.Error("failed to remove acknowledged WAL file", "file", filePath, "error", err)
				}
				continue
			}

			if id > lastWalID {
				candidates = append(candidates, walCandidate{id: id, name: name})
			}
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].id < candidates[j].id
		})

		for _, cand := range candidates {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Re-check active file name in case rotation happened
			if cand.name == ing.wal.ActiveFileName() {
				continue
			}

			filePath := filepath.Join(dir, cand.name)
			data, err := os.ReadFile(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				select {
				case ch <- WALSegment{Err: fmt.Errorf("failed to read WAL file %s: %w", cand.name, err)}:
				case <-ctx.Done():
				}
				return
			}

			select {
			case ch <- WALSegment{ID: cand.id, Data: data}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

func (ing *Ingestor) Ingest(ctx context.Context) error {
	defer func() {
		ing.collectorsMu.Lock()
		for name, c := range ing.collectors {
			if c.cancel != nil {
				ing.logger.Info("stopping collector", "name", name)
				c.cancel()
			}
		}
		ing.collectorsMu.Unlock()
		ing.collectorWg.Wait()
		if ing.wal != nil {
			ing.wal.Close()
		}
	}()

	ing.v.SetConfigFile(ing.collectorsPath)

	if err := ing.v.ReadInConfig(); err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	// Load initial config
	if err := ing.loadConfig(ctx); err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}

	// Watch for any change
	ing.v.OnConfigChange(func(in fsnotify.Event) {
		if err := ing.v.ReadInConfig(); err != nil {
			ing.logger.Error("cannot read config", "error", err)
			return
		}

		err := ing.loadConfig(ctx)
		if err != nil {
			ing.logger.Error("cannot load config", "error", err)
			return
		}
	})
	ing.v.WatchConfig()

	<-ctx.Done()

	return nil
}
