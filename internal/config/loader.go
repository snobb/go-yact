package config

import (
	"fmt"
	"os"
	"path"

	"github.com/kelseyhightower/envconfig"
	"go.yaml.in/yaml/v2"
)

const (
	configFileName = "yactrc.yaml"
	EnvVarPrefix   = "YACT_"
)

var (
	envconfigProcessFunc = envconfig.Process
	osReadFileFunc       = os.ReadFile
	yamlUnmarshalFunc    = yaml.Unmarshal

	searchDirs []string
)

func init() {
	searchDirs = getSearchDirs()
}

// Configer represents a valid configuration object.
type Configer interface {
	Validate() error
}

// Load loads config file into the provided configuration struct.
func Load[T Configer](cfg *T) error {
	for _, dir := range searchDirs {
		file := path.Join(dir, configFileName)
		if _, err := os.Stat(file); err == nil {
			if err := loadFile(file, cfg); err != nil {
				return fmt.Errorf("unable to load %q config file > %w", file, err)
			}
		}
	}

	// no config files are
	if err := envconfigProcessFunc(EnvVarPrefix, cfg); err != nil {
		return fmt.Errorf("unable to parse env variables > %w", err)
	}

	return nil
}

// Load and unmarshal yaml config file.
func loadFile[T Configer](fileName string, cfg *T) error {
	data, err := osReadFileFunc(fileName)
	if err != nil {
		return err
	}

	return yamlUnmarshalFunc(data, cfg)
}

// so far only unix-like systems are supported.
func getHomeDir() string {
	return os.Getenv("HOME")
}

func getSearchDirs() []string {
	dirs := []string{}

	path, err := os.Getwd()
	if err == nil {
		dirs = append(dirs, path)
	}

	dirs = append(dirs, getHomeDir(), "/etc")

	return dirs
}
