package aggregator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"github.com/thisisjab/logzilla/processor"
)

type processorConfig struct {
	Name       string   `mapstructure:"name"`
	Script     string   `mapstructure:"script"`
	ScriptPath string   `mapstructure:"script_path"`
	Collectors []string `mapstructure:"collectors"`
}

// buildRouterFromViper parses the "processors" section from viper configuration
// and builds a configured processor.Router.
func buildRouterFromViper(v *viper.Viper) (*processor.Router, error) {
	router := processor.NewRouter()

	var configs []processorConfig
	// StringToSliceHookFunc automatically converts scalar strings ("*.json") to []string{"*.json"}
	decoderOpt := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToSliceHookFunc(","),
	))

	if err := v.UnmarshalKey("processors", &configs, decoderOpt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal processors config: %w", err)
	}

	for _, cfg := range configs {
		trimmedName := strings.TrimSpace(cfg.Name)
		if trimmedName == "" {
			return nil, errors.New("processor name cannot be empty")
		}

		script := strings.TrimSpace(cfg.Script)
		scriptPath := strings.TrimSpace(cfg.ScriptPath)
		if script == "" && scriptPath == "" {
			return nil, fmt.Errorf("processor %q requires either 'script' or 'script_path'", trimmedName)
		}

		var (
			proc *processor.LuaProcessor
			err  error
		)

		if script != "" {
			proc, err = processor.NewLuaProcessor(trimmedName, script)
		} else {
			proc, err = processor.NewLuaProcessorFromFile(trimmedName, scriptPath)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to initialize processor %q: %w", trimmedName, err)
		}

		if len(cfg.Collectors) == 0 {
			return nil, fmt.Errorf("processor %q must define at least one collector pattern", trimmedName)
		}

		for _, pat := range cfg.Collectors {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			if err := router.Add(pat, proc); err != nil {
				return nil, fmt.Errorf("failed to add collector pattern %q for processor %q: %w", pat, trimmedName, err)
			}
		}
	}

	return router, nil
}
