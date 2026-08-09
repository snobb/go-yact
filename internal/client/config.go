package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/snobb/go-yact/internal/config"
)

// Default values for grpc client.
const (
	DefaultKeepAliveInterval = 20 * time.Second
	DefaultKeepAliveTimeout  = 5 * time.Second
)

// Config is a client configuration.
type Config struct {
	TLS       config.TLSConfig `envconfig:"tls_config" yaml:"tls_config"`
	ProxyAddr string           `envconfig:"proxy_addr" yaml:"proxy_addr"`
	ToAddr    string           `envconfig:"to_addr" yaml:"to_addr"`
	LocalPort int              `envconfig:"local_portr" yaml:"local_port"`

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

	if c.ToAddr == "" {
		c.ToAddr = ":443"
	}

	if c.LocalPort == 0 {
		addrTokens := strings.Split(c.ToAddr, ":")
		c.LocalPort, _ = strconv.Atoi(addrTokens[1])
	}

	if c.KeepAliveInterval == 0 {
		c.KeepAliveInterval = DefaultKeepAliveInterval
	}

	if c.KeepAliveTimeout == 0 {
		c.KeepAliveTimeout = DefaultKeepAliveTimeout
	}
}
