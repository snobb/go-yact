package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/snobb/go-yact/internal/certgen"
)

const (
	// CertDirSuffix is the default subdirectory under $HOME for TLS certs.
	CertDirSuffix = ".config/yact/certs"
)

// TLSConfig holds the TLS configuration.
type TLSConfig struct {
	CertDir string `envconfig:"cert_dir" yaml:"cert_dir"`
}

// DefaultCertDir returns the default certificate directory path.
func DefaultCertDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, CertDirSuffix)
}

// SetDefaults sets default values for TLSConfig.
func (t *TLSConfig) SetDefaults() {
	if t.CertDir == "" {
		t.CertDir = DefaultCertDir()
	}
}

// Validate checks that CertDir is set.
func (t *TLSConfig) Validate() error {
	if t.CertDir == "" {
		return fmt.Errorf("certificate directory is not set (flag '-cert-dir' or YACT_TLS_CONFIG_CERT_DIR environment variable)")
	}
	return nil
}

// ServerPaths returns CA, server cert, and server key paths.
func (t *TLSConfig) ServerPaths() (ca, cert, key string) {
	return filepath.Join(t.CertDir, certgen.CACertFile),
		filepath.Join(t.CertDir, certgen.ServerCertFile),
		filepath.Join(t.CertDir, certgen.ServerKeyFile)
}

// ClientPaths returns CA, client cert, and client key paths.
func (t *TLSConfig) ClientPaths() (ca, cert, key string) {
	return filepath.Join(t.CertDir, certgen.CACertFile),
		filepath.Join(t.CertDir, certgen.ClientCertFile),
		filepath.Join(t.CertDir, certgen.ClientKeyFile)
}
