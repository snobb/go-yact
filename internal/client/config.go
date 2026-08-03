package client

import (
	"fmt"

	"github.com/snobb/go-yact/internal/config"
)

// Config is a client configuration.
type Config struct {
	TLS       config.TLSConfig `envconfig:"tls_config" yaml:"tls_config"`
	ProxyAddr string           `envconfig:"proxy_addr" yaml:"proxy_addr"`
}

// Validate config and checks the required fields.
func (c Config) Validate() error {
	if err := c.TLS.Validate(); err != nil {
		return err
	}

	if c.ProxyAddr == "" {
		return fmt.Errorf("proxy address is not set (flag '-addr' or YACT_PROXY_ADDR environment variable)")
	}

	return nil
}
