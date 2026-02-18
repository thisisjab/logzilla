package api

import "errors"

type CORSConfig struct {
	TrustedOrigins []string `yaml:"trusted_origins"`
}

type Config struct {
	Addr     string     `yaml:"addr"`
	CertFile string     `yaml:"cert_file"`
	KeyFile  string     `yaml:"key_file"`
	CORS     CORSConfig `yaml:"cors"`
}

func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("api server address is required")
	}

	return nil
}
