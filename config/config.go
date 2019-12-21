package config

import (
	"fmt"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

// The configuration for a single app whalewatcher should monitor
type App struct {
	Name           string
	PathToLog      string
	MessagePattern string
}

// The config file
type Config struct {
	Apps []App
}

func FromFile(pathToFile string) (*Config, error) {
	conf := &Config{}

	raw, err := ioutil.ReadFile(pathToFile)
	if err != nil {
		return conf, fmt.Errorf("failed to read expected config file at path %q", pathToFile)
	}

	err = yaml.Unmarshal([]byte(raw), &conf)
	return conf, err
}

func FromVar(varName string) (*Config, error) {
	conf := &Config{}

	raw := os.Getenv(varName)
	if len(raw) == 0 {
		return conf, fmt.Errorf("expected config env var %q was empty or unset", varName)
	}

	err := yaml.Unmarshal([]byte(raw), &conf)
	return conf, err
}
