package aggregator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Aggregator collects logs from collectors, stores in WALs,
// and eventually sends WALs to the cluster node to be stored.
type Aggregator struct {
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
	// logsChan holds any log that has been collectted from any collector.
	logsChan chan string
}

type Config struct {
	CollectorsPath string
	Logger     *slog.Logger
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

	agg := &Aggregator{
		logger:     cfg.Logger,
		collectorsPath: cfg.CollectorsPath,

		collectors: make(map[string]*collectorState),

		// TODO: find optimal value for channel size
		logsChan: make(chan string, 100),
	}

	return agg, nil
}

func (agg *Aggregator) Ingest(ctx context.Context) error {
	viper.SetConfigFile(agg.collectorsPath)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("cannot read config: %w", err)
	}

	// Load initial config
	if err := agg.loadConfig(ctx); err != nil {
		return fmt.Errorf("failed to load inital config: %w", err)
	}

	// Watch for any change
	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		if err := viper.ReadInConfig(); err != nil {
			agg.logger.Error("cannot read config", "error", err)
			return
		}

		err := agg.loadConfig(ctx)
		if err != nil {
			agg.logger.Error("cannot load config", "error", err)
			return
		}
	})

	for {
		select {
		case l := <-agg.logsChan:
			// TODO: change with actual logic
			fmt.Printf("new log: %s\n", l)
		case <-ctx.Done():
			// Stop all collectors
			for name, c := range agg.collectors {
				agg.logger.Info("stopping collector", "name", name)
				c.cancel()
			}

			agg.collectorWg.Wait()

			return nil
		}
	}
}
