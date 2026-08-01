package config

import (
	"fmt"
	"os"
)

// Config is a client/server configuration
type Config struct {
	Debug bool   `json:"verbose"`
	Addr  string `json:"addr"`
}

// ReadEnv reads config from env variables.
func (c *Config) ReadEnv() {
	if c.Addr == "" {
		c.Addr = os.Getenv("YACT_ADDR")
	}
}

// Validate config and checks the required fields.
func (c *Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("address is not set (flag '-addr' or YACT_ADDR environment variable)")
	}

	return nil
}
