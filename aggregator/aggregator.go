package aggregator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Aggregator collects logs from collectors, stores in WALs,
// and eventually sends WALs to the cluster node to be stored.
type Aggregator struct {
	v      *viper.Viper
	logger *slog.Logger
	// collectorsPath defines the yaml file that collectors and cluster nodes are read from.
	// In case of any change to the file, it will trigger a hot reload.
	collectorsPath string
	// mu is a mutex used for updating collectors and cluster nodes in case of hot reload.
	mu sync.RWMutex
	// collectorsWg is used to wait till all collectors exit in case of termination
	collectorWg sync.WaitGroup
	// collectors holds list of collectors with their state.
	collectors map[string]*collectorState
	// wal handles persisting logs to disk.
	wal *wal
	// cb is the callback function for getting ingested logs from collectors.
	cb func(collectorName, data string) error
}

type Config struct {
	CollectorsPath string
	Logger         *slog.Logger
	Viper          *viper.Viper
}

func New(cfg Config) (*Aggregator, error) {
	if cfg.Logger == nil {
		return nil, errors.New("logger is nil")
	}

	if cfg.CollectorsPath == "" {
		return nil, errors.New("collectors path is nil")
	}

	if !strings.HasSuffix(cfg.CollectorsPath, ".yml") && !strings.HasSuffix(cfg.CollectorsPath, ".yaml") {
		return nil, errors.New("collectors path must end in .yaml or .yml")
	}

	v := cfg.Viper
	if v == nil {
		v = viper.New()
	}

	v.SetConfigFile(cfg.CollectorsPath)
	v.SetDefault("wal.dir", "./data/wal")
	v.SetDefault("wal.max_bytes", uint(10*1024*1024))
	v.SetDefault("wal.sync_interval", 5*time.Second)

	// Attempt reading static config (if config file exists already)
	_ = v.ReadInConfig()

	walDir := v.GetString("wal.dir")
	walMaxBytes := v.GetUint("wal.max_bytes")
	walSyncInterval := v.GetDuration("wal.sync_interval")

	w, err := newWAL(walDir, walMaxBytes, walSyncInterval, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot create WAL: %w", err)
	}

	agg := &Aggregator{
		v:              v,
		logger:         cfg.Logger,
		collectorsPath: cfg.CollectorsPath,

		collectors: make(map[string]*collectorState),
		wal:        w,
	}

	cb := func(collectorName, data string) error {
		err := agg.wal.append(collectorName, data)

		if err != nil {
			return fmt.Errorf("cannot append to WAL: %w", err)
		}

		return nil
	}

	agg.cb = cb

	return agg, nil
}

func (agg *Aggregator) Ingest(ctx context.Context) error {
	agg.v.SetConfigFile(agg.collectorsPath)

	if err := agg.v.ReadInConfig(); err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	// Load initial config
	if err := agg.loadConfig(ctx); err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}

	// Watch for any change
	agg.v.OnConfigChange(func(in fsnotify.Event) {
		if err := agg.v.ReadInConfig(); err != nil {
			agg.logger.Error("cannot read config", "error", err)
			return
		}

		err := agg.loadConfig(ctx)
		if err != nil {
			agg.logger.Error("cannot load config", "error", err)
			return
		}
	})
	agg.v.WatchConfig()

	<-ctx.Done()

	// Stop all collectors
	agg.mu.Lock()
	for name, c := range agg.collectors {
		if c.cancel != nil {
			agg.logger.Info("stopping collector", "name", name)
			c.cancel()
		}
	}
	agg.mu.Unlock()

	agg.collectorWg.Wait()

	if agg.wal != nil {
		agg.wal.close()
	}

	return nil
}
