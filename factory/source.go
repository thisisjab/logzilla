package factory

import (
	"fmt"
	"log/slog"

	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/source"
)

// BuildSource creates a LogSource based on the provided configuration.
func BuildSource(cfg source.Config, logger *slog.Logger) (engine.LogSource, error) {
	switch cfg.Type {
	case "file":
		var fileConfig source.FileLogSourceConfig // Use the concrete config type from the source/file package
		err := cfg.Config.Decode(&fileConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot parse file source config: %w", err)
		}
		return source.NewFileLogSource(fileConfig, logger)
	case "shell":
		var shellConfig source.ShellLogSourceConfig // Use the concrete config type from the source/shell package
		err := cfg.Config.Decode(&shellConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot parse shell source config: %w", err)
		}
		return source.NewShellLogSource(shellConfig, logger)
	default:
		return nil, fmt.Errorf("unknown log source type: %s", cfg.Type)
	}
}

// BuildSources creates a slice of LogSource instances based on the provided configuration.
func BuildSources(cfgs []source.Config, logger *slog.Logger) ([]engine.LogSource, error) {
	sources := make([]engine.LogSource, 0, len(cfgs))
	for i, cfg := range cfgs {
		source, err := BuildSource(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("error when building source at index %d: %w", i, err)
		}
		sources = append(sources, source)
	}
	return sources, nil
}
