package ingestor

import (
	"context"
	"time"

	"github.com/thisisjab/logzilla/collector"
)

// collectorState holds all the current collectors of an ingestor.
type collectorState struct {
	isActive bool
	// ref holds a pointer to the actual collector.
	ref collector.Collector
	// cancel function is saved so that later we are able to stop only a single collector in case.
	cancel context.CancelFunc
}

// walConfig defines write-ahead log settings in configuration.
type walConfig struct {
	Dir          string        `mapstructure:"dir"`
	MaxBytes     uint          `mapstructure:"max_bytes"`
	SyncInterval time.Duration `mapstructure:"sync_interval"`
}

// viperConfig is the struct that is expected from config file.
type viperConfig struct {
	GRPCPort   int       `mapstructure:"grpcPort"`
	WAL        walConfig `mapstructure:"wal"`
	Collectors map[string]struct {
		Type     string         `mapstructure:"type"`
		IsActive *bool          `mapstructure:"isActive"`
		Args     map[string]any `mapstructure:"args"`
	} `mapstructure:"collectors"`
}

// loadConfig reads yaml file and updates config.
func (ing *Ingestor) loadConfig(ctx context.Context) error {
	var cfg viperConfig
	if err := ing.v.Unmarshal(&cfg); err != nil {
		return err
	}

	ing.updateConfig(ctx, cfg)

	return nil
}

// updateConfig reads updated yaml config file and starts/stops/modify collectors.
func (ing *Ingestor) updateConfig(ctx context.Context, newConfig viperConfig) {
	// NOTE:
	// Any config file change currently rebuilds and restarts all collectors.
	// A future version should diff the old and new configs and only restart
	// collectors whose configuration actually changed.
	// WAL settings (dir, max_bytes, sync_interval) are skipped from hot reload.

	ing.collectorsMu.Lock()
	defer ing.collectorsMu.Unlock()

	// Remove collectors that no longer exist.
	for name, existing := range ing.collectors {
		if _, exists := newConfig.Collectors[name]; !exists {
			if existing.isActive {
				ing.logger.Info("stopping collector", "name", name)
				existing.cancel()
			}

			ing.logger.Info("collector removed", "name", name)
			delete(ing.collectors, name)
		}
	}

	// Rebuild and restart all configured collectors.
	for cName, cfg := range newConfig.Collectors {
		// Let's consider nil IsActive as true since no one wants to pass is_active = true
		active := cfg.IsActive == nil || *cfg.IsActive

		parsed, err := collector.Build(cName, cfg.Type, cfg.Args)
		if err != nil {
			ing.logger.Error("cannot build collector", "name", cName, "error", err)
			continue
		}

		// Stop previous instance if it existed.
		if existing, exists := ing.collectors[cName]; exists && existing.isActive {
			ing.logger.Info("stopping collector", "name", cName)
			existing.cancel()
		}

		state := &collectorState{
			ref:      parsed,
			isActive: active,
		}

		if active {
			collectorCtx, cancel := context.WithCancel(ctx)
			state.cancel = cancel

			ing.collectorWg.Go(func() {
				ing.logger.Info("starting collector", "name", cName)

				err := parsed.Collect(collectorCtx, ing.cb)
				if err != nil {
					ing.logger.Error("error from collector", "name", cName, "error", err)
				}
			})
		}

		ing.collectors[cName] = state
	}
}
