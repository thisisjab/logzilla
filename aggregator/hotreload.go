package aggregator

import (
	"context"

	"github.com/spf13/viper"
	"github.com/thisisjab/logzilla/collector"
)

// collectorState holds all the current collectors of an aggregator.
type collectorState struct {
	isActive bool
	// ref holds a pointer to the actual collector.
	ref collector.Collector
	// cancel function is saved so that later we are able to stop only a single collector in case.
	cancel context.CancelFunc
}

// viperConfig is the struct that is expected from config file.
type viperConfig struct {
	Collectors map[string]struct {
		Type     string         `mapstructure:"type"`
		IsActive *bool          `mapstructure:"isActive"`
		Args     map[string]any `mapstructure:"args"`
	} `mapstructure:"collectors"`
}

// loadConfig reads yaml file and updates config.
func (agg *Aggregator) loadConfig(ctx context.Context) error {
	var cfg viperConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	agg.updateConfig(ctx, cfg)

	return nil
}

// updateConfig reads updated yaml config file and starts/stops/modify collectors.
func (agg *Aggregator) updateConfig(ctx context.Context, newConfig viperConfig) {
	// NOTE:
	// Any config file change currently rebuilds and restarts all collectors.
	// A future version should diff the old and new configs and only restart
	// collectors whose configuration actually changed.

	agg.mu.Lock()
	defer agg.mu.Unlock()

	// Remove collectors that no longer exist.
	for name, existing := range agg.collectors {
		if _, exists := newConfig.Collectors[name]; !exists {
			if existing.isActive {
				agg.logger.Info("stopping collector", "name", name)
				existing.cancel()
			}

			agg.logger.Info("collector removed", "name", name)
			delete(agg.collectors, name)
		}
	}

	// Rebuild and restart all configured collectors.
	for name, cfg := range newConfig.Collectors {
		// Let's consider nil IsActive as true since no one wants to pass is_active = true
		active := cfg.IsActive == nil || *cfg.IsActive

		parsed, err := collector.Build(cfg.Type, cfg.Args)
		if err != nil {
			agg.logger.Error("cannot build collector", "name", name, "error", err)
			continue
		}

		// Stop previous instance if it existed.
		if existing, exists := agg.collectors[name]; exists && existing.isActive {
			agg.logger.Info("stopping collector", "name", name)
			existing.cancel()
		}

		state := &collectorState{
			ref:      parsed,
			isActive: active,
		}

		if active {
			collectorCtx, cancel := context.WithCancel(ctx)
			state.cancel = cancel

			agg.collectorWg.Go(func() {
				agg.logger.Info("starting collector", "name", name)

				err := parsed.Collect(collectorCtx, agg.logsChan)
				if err != nil {
					agg.logger.Error("error from collector", "name", name, "error", err)
				}
			})
		}

		agg.collectors[name] = state
	}
}
