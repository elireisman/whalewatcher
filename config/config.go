package config

import (
	"fmt"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

// The config file model
type Config struct {
	Apps map[string]App `yaml:"apps"`
}

// The configuration for a single app whalewatcher should monitor
type App struct {
	// path to the log file to monitor for readiness
	Path string `yaml:"path"`
	// regex pattern to match in log indicating service readiness
	Pattern string `yaml:"pattern"`
}

// load config YAML from a file mounted into whalewatcher's container
func FromFile(pathToFile string) (*Config, error) {
	conf := &Config{Apps: map[string]App{}}

	raw, err := ioutil.ReadFile(pathToFile)
	if err != nil {
		return conf, fmt.Errorf("failed to read expected config file at path %q", pathToFile)
	}

	err = yaml.Unmarshal([]byte(raw), &conf)
	return conf, err
}

// local config YAML from an env var injected into whalewatcher's container env
func FromVar(varName string) (*Config, error) {
	conf := &Config{Apps: map[string]App{}}

	raw := os.Getenv(varName)
	if len(raw) == 0 {
		return conf, fmt.Errorf("expected config env var %q was empty or unset", varName)
	}

	err := yaml.Unmarshal([]byte(raw), &conf)
	return conf, err
}
