package client

import (
	"fmt"
	"os"

	"github.com/snobb/go-yact/internal/config"
)

// Config is a client configuration.
type Config struct {
	*config.CommonConfig

	ProxyAddr string `json:"proxy_addr"`
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
