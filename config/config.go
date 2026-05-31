package config

import (
	"fmt"
	"os"

	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/server"
	"github.com/thisisjab/logzilla/source"
	"github.com/thisisjab/logzilla/storage"
	"go.yaml.in/yaml/v3"
)

// ConfigSchema defines the format of `config.yaml`.
type ConfigSchema struct {
	Engine     engine.Config      `yaml:"engine"`
	Server     server.Config      `yaml:"server"`
	Logger     LoggerConfig       `yaml:"logger"`
	Storage    storage.Config     `yaml:"storage"`
	Processors []processor.Config `yaml:"processors"`
	Sources    []source.Config    `yaml:"sources"`
}

type LoggerConfig struct {
	Level   string `yaml:"level"`
	Handler string `yaml:"handler"`
}

func Parse(path string) (*ConfigSchema, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file content: %w", err)
	}

	var cfg ConfigSchema
	err = yaml.Unmarshal(fileContent, &cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("cannot validate config: %w", err)
	}

	return &cfg, nil
}

func (cfg ConfigSchema) Validate() error {
	err := validateLoggerConfig(cfg.Logger)
	if err != nil {
		return fmt.Errorf("cannot create logger: %w", err)
	}

	err = validateStorageConfig(cfg.Storage)
	if err != nil {
		return fmt.Errorf("cannot create storage: %w", err)
	}

	processors := make([]processor.Config, len(cfg.Processors))
	for i, pc := range cfg.Processors {
		err := parseProcessorConfig(pc)
		if err != nil {
			return fmt.Errorf("cannot create processor: %w", err)
		}
		processors[i] = pc
	}

	sources := make([]source.Config, len(cfg.Sources))
	for i, sc := range cfg.Sources {
		err := parseSourceConfig(sc)
		if err != nil {
			return fmt.Errorf("cannot create log source: %w", err)
		}
		sources[i] = sc
	}

	return nil
}

func validateLoggerConfig(cfg LoggerConfig) error {
	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	switch cfg.Handler {
	case "json", "text", "colored-text":
	default:
		return fmt.Errorf("invalid log type: %s", cfg.Handler)
	}

	return nil
}

func validateStorageConfig(cfg storage.Config) error {
	switch cfg.Type {
	case "clickhouse":
		var clickHouseConfig storage.ClickHouseStorageConfig

		if err := reMarshal(cfg.Config, &clickHouseConfig); err != nil {
			return fmt.Errorf("cannot parse clickhouse storage config: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("invalid storage type: %s", cfg.Type)
	}
}

func parseSourceConfig(cfg source.Config) error {
	switch cfg.Type {
	case "file":
		var fileConfig source.FileLogSourceConfig
		err := reMarshal(cfg.Config, &fileConfig)
		if err != nil {
			return fmt.Errorf("cannot create file source: %w", err)
		}

		return nil

	case "shell":
		var shellConfig source.ShellLogSourceConfig
		err := reMarshal(cfg.Config, &shellConfig)
		if err != nil {
			return fmt.Errorf("cannot create shell source: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("invalid log source type: %s", cfg.Type)
	}
}

func parseProcessorConfig(cfg processor.Config) error {
	switch cfg.Type {
	case "json":
		var jsonConfig processor.JsonLogProcessorConfig
		err := reMarshal(cfg.Config, &jsonConfig)
		if err != nil {
			return fmt.Errorf("cannot create json processor: %w", err)
		}

		return nil
	case "lua":
		var luaConfig processor.LuaLogProcessorConfig
		err := reMarshal(cfg.Config, &luaConfig)
		if err != nil {
			return fmt.Errorf("cannot create lua processor: %w", err)
		}

		return nil
	default:
		return fmt.Errorf("invalid log processor type: %s", cfg.Type)
	}
}

// reMarshal takes an input value, marshals it to YAML, and then unmarshals it into a new value of the same type.
// This is useful for converting generic interfaces (like map[string]any) into concrete struct types.
// The output parameter must be a pointer to the target type.
func reMarshal(input any, output any) error {
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
