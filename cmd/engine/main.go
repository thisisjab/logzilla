package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/source"
	"github.com/thisisjab/logzilla/storage"
	"gopkg.in/yaml.v3"
)

func main() {
	// 1. Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	cfgPath := flag.String("config", "./config.yaml", "path to config file")
	flag.Parse()

	fileContent, err := os.ReadFile(*cfgPath)
	if err != nil {
		panic(fmt.Errorf("cannot read config file content: %w", err))
	}

	var cfg Config
	if err := yaml.Unmarshal(fileContent, &cfg); err != nil {
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	engineCfg, logger, err := parseConfig(cfg)
	if err != nil {
		if logger != nil {
			logger.Error("cannot parse config file", "error", err)
		}
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	// Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the engine in a separate goroutine so we can wait for signals
	go func() {
		sig := <-sigChan
		logger.Info("received signal. shutting down.", "signal", sig)
		cancel()
	}()

	// Create engine
	engine, err := engine.New(*engineCfg, logger)
	if err != nil {
		logger.Error("engine error.", "error", err)
		os.Exit(1)
	}

	// Run the engine.
	if err := engine.Run(ctx); err != nil {
		logger.Error("engine error.", "error", err)
	}

	logger.Info("engine stopped.")
}

type Config struct {
	Logger                     LoggerConfig      `yaml:"logger"`
	Storage                    StorageConfig     `yaml:"storage"`
	Processors                 []ProcessorConfig `yaml:"processors"`
	Sources                    []SourceConfig    `yaml:"sources"`
	RawLogsBufferMaxSize       uint              `yaml:"raw_logs_buffer_max_size"`
	StorageFlushInterval       time.Duration     `yaml:"storage_flush_interval"`
	ProcessedLogsBufferMaxSize uint              `yaml:"processed_logs_buffer_max_size"`
	ProcessorWorkersCount      uint              `yaml:"processor_workers_count"`
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
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Config any    `yaml:"config"`
}

type SourceConfig struct {
	Name       string   `yaml:"name"`
	Type       string   `yaml:"type"`
	Processors []string `yaml:"processors"`
	Config     any      `yaml:"config"`
}

func parseConfig(cfg Config) (*engine.Config, *slog.Logger, error) {
	logger, err := parseLoggerConfig(cfg.Logger)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create logger: %w", err)
	}

	st, err := parseStorageConfig(cfg.Storage)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create storage: %w", err)
	}

	processors := make([]engine.LogProcessor, len(cfg.Processors))
	for _, pc := range cfg.Processors {
		p, err := parseProcessorConfig(logger, pc)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create processor: %w", err)
		}
		processors = append(processors, p)
	}

	sources := make([]engine.LogSource, len(cfg.Processors))
	for _, sc := range cfg.Sources {
		s, err := parseSourceConfig(logger, sc)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create source: %w", err)
		}
		sources = append(sources, s)
	}

	return &engine.Config{
		RawLogsBufferMaxSize:       cfg.RawLogsBufferMaxSize,
		StorageFlushInterval:       cfg.StorageFlushInterval,
		ProcessedLogsBufferMaxSize: cfg.ProcessedLogsBufferMaxSize,
		ProcessorWorkersCount:      cfg.ProcessorWorkersCount,
		Storage:                    st,
		Processors:                 processors,
		Sources:                    sources,
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

func parseStorageConfig(cfg StorageConfig) (engine.Storage, error) {
	switch cfg.Type {
	case "clickhouse":
		clickHouseConfig, ok := cfg.Config.(storage.ClickHouseStorageConfig)
		if !ok {
			return nil, fmt.Errorf("cannot parse clickhouse storage config")
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
		fileConfig, ok := cfg.Config.(source.FileLogSourceConfig)
		if !ok {
			return nil, fmt.Errorf("cannot parse file source config")
		}

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
		jsonConfig, ok := cfg.Config.(processor.JsonLogProcessorConfig)
		if !ok {
			return nil, fmt.Errorf("cannot parse json processor config")
		}

		p, err := processor.NewJsonLogProcessor(jsonConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create json processor: %w", err)
		}

		return p, nil
	default:
		return nil, fmt.Errorf("invalid log processor type: %s", cfg.Type)
	}
}
