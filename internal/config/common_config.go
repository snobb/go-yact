package config

import (
	"fmt"
	"os"
)

// CommonConfig is a common client/server configuration
type CommonConfig struct {
	CAPath   string
	CertPath string
	KeyPath  string

	Debug bool `json:"verbose"`
}

// ReadEnv reads config from env variables.
func (c *CommonConfig) ReadEnv() {
	if c.CAPath == "" {
		c.CAPath = os.Getenv("YACT_CA_PATH")
	}

	if c.CertPath == "" {
		c.CertPath = os.Getenv("YACT_CERT_PATH")
	}

	if c.KeyPath == "" {
		c.KeyPath = os.Getenv("YACT_KEY_PATH")
	}
}

// Validate config and checks the required fields.
func (c *CommonConfig) Validate() error {
	if c.CAPath == "" {
		return fmt.Errorf("ca certificate is not set (flag '-ca' or YACT_CA_PATH environment variable)")
	}

	if c.CertPath == "" {
		return fmt.Errorf("client certificate is not set (flag '-cert' or YACT_CERT_PATH environment variable)")
	}

	if c.KeyPath == "" {
		return fmt.Errorf("key is not set (flag '-key' or YACT_KEY_PATH environment variable)")
	}

	return nil
}
