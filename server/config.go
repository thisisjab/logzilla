package server

import "errors"

type CORSConfig struct {
	TrustedOrigins []string `yaml:"trusted-origins"`
}

type Config struct {
	Addr     string     `yaml:"addr"`
	CertFile string     `yaml:"cert-file"`
	KeyFile  string     `yaml:"key-file"`
	CORS     CORSConfig `yaml:"cors"`
}

func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("api server address is required")
	}

	return nil
}
