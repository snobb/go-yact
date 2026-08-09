package proxy

import (
	"fmt"
	"time"

	"github.com/snobb/go-yact/internal/config"
)

// Default values for grpc server.
const (
	DefaultKeepAliveInterval = 1 * time.Minute
	DefaultKeepAliveTimeout  = 20 * time.Second
)

// Config is a proxy server configuration.
type Config struct {
	TLS       config.TLSConfig `envconfig:"tls_config" yaml:"tls_config"`
	ProxyAddr string           `envconfig:"proxy_addr" yaml:"proxy_addr"`

	KeepAliveInterval time.Duration `envconfig:"keep_alive_interval" yaml:"keep_alive_interval"`
	KeepAliveTimeout  time.Duration `envconfig:"keep_alive_timeout" yaml:"keep_alive_timeout"`
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

// SetDefaults sets the default values to the struct.
func (c *Config) SetDefaults() {
	if c.ProxyAddr == "" {
		c.ProxyAddr = ":8008"
	}

	if c.KeepAliveInterval == 0 {
		c.KeepAliveInterval = DefaultKeepAliveInterval
	}

	if c.KeepAliveTimeout == 0 {
		c.KeepAliveTimeout = DefaultKeepAliveTimeout
	}
}
