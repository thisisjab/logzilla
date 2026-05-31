package processor

import "go.yaml.in/yaml/v3"

type Config struct {
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}
