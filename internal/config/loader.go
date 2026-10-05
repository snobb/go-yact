package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
	"go.yaml.in/yaml/v2"
)

const (
	ConfigFileName = "config.yml"
	EnvVarPrefix   = "YACT"
)

var (
	envconfigProcessFunc = envconfig.Process
	osReadFileFunc       = os.ReadFile
	yamlUnmarshalFunc    = yaml.Unmarshal
)

// Configer represents a valid configuration object.
type Configer interface {
	Validate() error
	SetDefaults()
}

// Load loads config file into the provided configuration struct.
func Load[T Configer](configPath string, cfg T) error {
	if _, err := os.Stat(configPath); err == nil {
		if err := loadFile(configPath, cfg); err != nil {
			return fmt.Errorf("unable to load %q config file > %w", configPath, err)
		}
	}

	// no config files are
	if err := envconfigProcessFunc(EnvVarPrefix, cfg); err != nil {
		return fmt.Errorf("unable to parse env variables > %w", err)
	}

	return nil
}

// Load and unmarshal yaml config file.
func loadFile[T Configer](fileName string, cfg T) error {
	data, err := osReadFileFunc(fileName)
	if err != nil {
		return err
	}

	return yamlUnmarshalFunc(data, cfg)
}

func ConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config/yact", ConfigFileName)
}
