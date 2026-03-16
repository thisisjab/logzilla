package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/thisisjab/logzilla/api"
	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/querier"
	"github.com/thisisjab/logzilla/source"
	"github.com/thisisjab/logzilla/storage"
	"go.yaml.in/yaml/v3"
)

// ParsedConfig contains parsed and validated configuration for engine, api server, logger, etc.
type ParsedConfig struct {
	EngineConfig    engine.Config
	APIServerConfig api.Config
	Storage         parsedStorage
}

// ConfigSchema defines the format of `config.yaml`.
type ConfigSchema struct {
	Engine     EngineConfig      `yaml:"engine"`
	Server     api.Config        `yaml:"api-server"`
	Logger     LoggerConfig      `yaml:"logger"`
	Storage    StorageConfig     `yaml:"storage"`
	Processors []ProcessorConfig `yaml:"processors"`
	Sources    []SourceConfig    `yaml:"sources"`
}

// Engine config defines all the settings used
type EngineConfig struct {
	RawLogsBufferSize       uint          `yaml:"raw-logs-buffer-size"`
	StorageFlushInterval    time.Duration `yaml:"storage-flush-interval"`
	ProcessedLogsBufferSize uint          `yaml:"processed-logs-buffer-size"`
	ProcessorWorkersCount   uint          `yaml:"processor-workers-count"`
}

type LoggerConfig struct {
	Level  string `yaml:"level"`
	Type   string `yaml:"type"`
	Output string `yaml:"output"`
}

type StorageConfig struct {
	Type   string `yaml:"type"`
	Config any    `yaml:"config"`
}

type ProcessorConfig struct {
	Type   string `yaml:"type"`
	Config any    `yaml:"config"`
}

type SourceConfig struct {
	Type       string   `yaml:"type"`
	Processors []string `yaml:"processors"`
	Config     any      `yaml:"config"`
}

func (cfg ConfigSchema) Parse() (*ParsedConfig, *slog.Logger, error) {
	logger, err := parseLoggerConfig(cfg.Logger)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create logger: %w", err)
	}

	st, err := parseStorageConfig(cfg.Storage)
	if err != nil {
		return nil, logger, fmt.Errorf("cannot create storage: %w", err)
	}

	processors := make([]engine.LogProcessor, len(cfg.Processors))
	for i, pc := range cfg.Processors {
		p, err := parseProcessorConfig(logger, pc)
		if err != nil {
			return nil, logger, fmt.Errorf("cannot create processor: %w", err)
		}
		processors[i] = p
	}

	sources := make([]engine.LogSource, len(cfg.Sources))
	for i, sc := range cfg.Sources {
		s, err := parseSourceConfig(logger, sc)
		if err != nil {
			return nil, logger, fmt.Errorf("cannot create log source: %w", err)
		}
		sources[i] = s
	}

	return &ParsedConfig{
		APIServerConfig: cfg.Server,
		Storage:         st,
		EngineConfig: engine.Config{
			RawLogsBufferMaxSize:       cfg.Engine.RawLogsBufferSize,
			StorageFlushInterval:       cfg.Engine.StorageFlushInterval,
			ProcessedLogsBufferMaxSize: cfg.Engine.ProcessedLogsBufferSize,
			ProcessorWorkersCount:      cfg.Engine.ProcessorWorkersCount,
			Storage:                    st,
			Processors:                 processors,
			Sources:                    sources,
		},
	}, logger, nil
}

func parseLoggerConfig(cfg LoggerConfig) (*slog.Logger, error) {
	var logger *slog.Logger
	var handler slog.Handler

	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	w := os.Stdout
	switch cfg.Type {
	case "json":
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	case "text":
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	case "colored-text":
		handler = tint.NewHandler(w, &tint.Options{Level: level, AddSource: true})
	default:
		return nil, fmt.Errorf("invalid log type: %s", cfg.Type)
	}

	logger = slog.New(handler)

	return logger, nil
}

type parsedStorage interface {
	storage.Storage
	querier.QuerierStorage
	engine.EngineStorage
}

func parseStorageConfig(cfg StorageConfig) (parsedStorage, error) {
	switch cfg.Type {
	case "clickhouse":
		var clickHouseConfig storage.ClickHouseStorageConfig

		if err := remarshal(cfg.Config, &clickHouseConfig); err != nil {
			return nil, fmt.Errorf("cannot parse clickhouse storage config: %w", err)
		}

		s, err := storage.NewClickHouseStorage(clickHouseConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create clickhouse storage: %w", err)
		}

		return s, nil

	default:
		return nil, fmt.Errorf("invalid storage type: %s", cfg.Type)
	}
}

func parseSourceConfig(logger *slog.Logger, cfg SourceConfig) (engine.LogSource, error) {
	switch cfg.Type {
	case "file":
		var fileConfig source.FileLogSourceConfig
		err := remarshal(cfg.Config, &fileConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create file source: %w", err)
		}

		fileConfig.ProcessorNames = cfg.Processors

		s, err := source.NewFileLogSource(logger, fileConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create file source: %w", err)
		}

		return s, nil
	default:
		return nil, fmt.Errorf("invalid log source type: %s", cfg.Type)
	}
}

func parseProcessorConfig(logger *slog.Logger, cfg ProcessorConfig) (engine.LogProcessor, error) {
	switch cfg.Type {
	case "json":
		var jsonConfig processor.JsonLogProcessorConfig
		err := remarshal(cfg.Config, &jsonConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create json processor: %w", err)
		}

		p, err := processor.NewJsonLogProcessor(jsonConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create json processor: %w", err)
		}

		return p, nil
	case "lua":
		var luaConfig processor.LuaLogProcessorConfig
		err := remarshal(cfg.Config, &luaConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create lua processor: %w", err)
		}

		p, err := processor.NewLuaLogProcessor(luaConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create lua processor: %w", err)
		}

		return p, nil
	default:
		return nil, fmt.Errorf("invalid log processor type: %s", cfg.Type)
	}
}

// remarshal takes an input value, marshals it to YAML, and then unmarshals it into a new value of the same type.
// This is useful for converting generic interfaces (like map[string]any) into concrete struct types.
// The output parameter must be a pointer to the target type.
func remarshal(input any, output any) error {
	// Marshal the input to YAML
	yamlBytes, err := yaml.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	// Unmarshal the YAML into the output
	if err := yaml.Unmarshal(yamlBytes, output); err != nil {
		return fmt.Errorf("failed to unmarshal from YAML: %w", err)
	}

	return nil
}
