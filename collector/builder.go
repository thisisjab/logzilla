package collector

import (
	"fmt"
)

// Build gets a type and map of settings and returns a collector
func Build(t string, args map[string]any) (Collector, error) {
	switch t {

	case "file" :
		path, exists := args["path"]

		if !exists {
			return nil, fmt.Errorf("cannot setup collector of type %s: path not set", t)
		}

		c, err := NewFileCollector(fmt.Sprintf("%s", path))
		if err != nil {
			return nil, fmt.Errorf("cannot setup collector of type %s: %w", t, err)
		}

		return c, nil

	default:
		return nil, fmt.Errorf("collector of type %s is unknown", t)
	}
}
