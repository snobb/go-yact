package client

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/snobb/go-yact/internal/config"
)

// Default values for grpc client.
const (
	DefaultKeepAliveInterval = 1 * time.Minute
	DefaultKeepAliveTimeout  = 5 * time.Minute
)

type BindAddress struct {
	BindAddress string `envconfig:"bind_address" yaml:"bind_address"`
	LocalPort   int    `envconfig:"local_port" yaml:"local_port"`
}

// Config is a client configuration.
type Config struct {
	TLS       config.TLSConfig `envconfig:"tls_config" yaml:"tls_config"`
	ProxyAddr string           `envconfig:"proxy_addr" yaml:"proxy_addr"`
	ToAddrs   []BindAddress    `envconfig:"to_addrs" yaml:"to_addrs"`

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
	c.TLS.SetDefaults()

	c.TLS.CertDir = filepath.Join(c.TLS.CertDir, "client1")

	if c.ProxyAddr == "" {
		c.ProxyAddr = "0.0.0.0:8001"
	}

	if len(c.ToAddrs) == 0 {
		c.ToAddrs = []BindAddress{{BindAddress: ":443", LocalPort: 443}}
	}

	if c.KeepAliveInterval == 0 {
		c.KeepAliveInterval = DefaultKeepAliveInterval
	}

	if c.KeepAliveTimeout == 0 {
		c.KeepAliveTimeout = DefaultKeepAliveTimeout
	}
}

func (c Config) BindAddresses() []string {
	addrs := make([]string, 0, len(c.ToAddrs))
	for _, addr := range c.ToAddrs {
		addrs = append(addrs, addr.BindAddress)
	}
	return addrs
}
