package config

import (
	"fmt"
)

// TLSConfig holds the TLS configuration
type TLSConfig struct {
	CAPath   string `envconfig:"ca_path" yaml:"ca_path"`
	CertPath string `envconfig:"cert_path" yaml:"cert_path"`
	KeyPath  string `envconfig:"key_path" yaml:"key_path"`
}

// Validate config and checks the required fields.
func (t *TLSConfig) Validate() error {
	if t.CAPath == "" {
		return fmt.Errorf("ca certificate is not set (flag '-ca' or YACT_CA_PATH environment variable)")
	}

	if t.CertPath == "" {
		return fmt.Errorf("client certificate is not set (flag '-cert' or YACT_CERT_PATH environment variable)")
	}

	if t.KeyPath == "" {
		return fmt.Errorf("key is not set (flag '-key' or YACT_KEY_PATH environment variable)")
	}

	return nil
}
