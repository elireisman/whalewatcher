package config

import (
	"fmt"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

// The config file model - a mapping of container names to monitoring configuration
type Config struct {
	Containers map[string]Container `yaml:"containers"`
}

// The configuration for a single app whalewatcher should monitor
type Container struct {
	// regex pattern to match in log indicating service readiness
	Pattern string `yaml:"pattern"`

	// backwards compatible attribute for specifying more than one pattern
	Patterns []string `yaml:"patterns"`

	// optional: time to tail the target container's log
	// (without errors) before marking the container ready
	MaxWaitMillis int `yaml:"max_wait_millis"`

	// optional: only log lines more recently produced than this will be
	// fetched during tailing. accepts a time.Duration string
	Since string `yaml:"since"`
}

// load config YAML from a file mounted into whalewatcher's container
func FromFile(pathToFile string) (*Config, error) {
	conf := &Config{Containers: map[string]Container{}}

	raw, err := ioutil.ReadFile(pathToFile)
	if err != nil {
		return conf, fmt.Errorf("failed to read expected config file at path %q", pathToFile)
	}

	err = yaml.Unmarshal([]byte(raw), &conf)
	return conf, err
}

// local config YAML from an env var injected into whalewatcher's container env
func FromVar(varName string) (*Config, error) {
	conf := &Config{Containers: map[string]Container{}}

	raw := os.Getenv(varName)
	if len(raw) == 0 {
		return conf, fmt.Errorf("expected config env var %q was empty or unset", varName)
	}

	err := yaml.Unmarshal([]byte(raw), &conf)
	return conf, err
}
