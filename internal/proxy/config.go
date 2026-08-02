package proxy

import (
	"fmt"
	"os"
	"time"

	"github.com/snobb/go-yact/internal/config"
)

const (
	MaxConnectionIdle        = 15 * time.Minute
	MaxConnectionAge         = 30 * time.Minute
	DefaultKeepAliveInterval = 1 * time.Minute
	DefaultKeepAliveTimeout  = 20 * time.Second
)

// Config is a proxy server configuration.
type Config struct {
	*config.CommonConfig

	ProxyAddr         string        `json:"proxy_addr"`
	KeepAliveInterval time.Duration `json:"keep_alive_interval"`
	KeepAliveTimeout  time.Duration `json:"keep_alive_timeout"`
}

// ReadEnv reads config from env variables.
func (c *Config) ReadEnv() {
	c.CommonConfig.ReadEnv()

	if c.ProxyAddr == "" {
		c.ProxyAddr = os.Getenv("YACT_PROXY_ADDR")
	}
}

// Validate config and checks the required fields.
func (c *Config) Validate() error {
	if err := c.CommonConfig.Validate(); err != nil {
		return err
	}

	if c.ProxyAddr == "" {
		return fmt.Errorf("proxy address is not set (flag '-addr' or YACT_PROXY_ADDR environment variable)")
	}

	return nil
}
