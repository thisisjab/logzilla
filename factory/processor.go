package factory

import (
	"fmt"

	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/processor"
)

// BuildProcessor creates a LogProcessor based on the provided configuration.
func BuildProcessor(cfg processor.Config) (engine.LogProcessor, error) {
	switch cfg.Type {
	case "json":
		var jsonConfig processor.JsonLogProcessorConfig
		err := cfg.Config.Decode(&jsonConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot parse json processor config: %w", err)
		}

		return processor.NewJsonLogProcessor(jsonConfig)
	case "lua":
		var luaConfig processor.LuaLogProcessorConfig // Use the concrete config type from the processor/lua package
		err := cfg.Config.Decode(&luaConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot parse lua processor config: %w", err)
		}
		return processor.NewLuaLogProcessor(luaConfig)
	default:
		return nil, fmt.Errorf("unknown log processor type: %s", cfg.Type)
	}
}

// BuildProcessors creates a slice of LogProcessor instances based on the provided configuration.
func BuildProcessors(cfgs []processor.Config) ([]engine.LogProcessor, error) {
	processors := make([]engine.LogProcessor, 0, len(cfgs))
	for i, cfg := range cfgs {
		processor, err := BuildProcessor(cfg)
		if err != nil {
			return nil, fmt.Errorf("error when building processor at index %d: %w", i, err)
		}
		processors = append(processors, processor)
	}
	return processors, nil
}
