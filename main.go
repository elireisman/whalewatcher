package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/elireisman/whalewatcher/config"
	"github.com/elireisman/whalewatcher/tailer"
)

var (
	BaseDir    string
	ConfigPath string
	ConfigVar  string
)

func init() {
	flag.StringVar(&BaseDir, "dir", "/var/log/watched", "base directory in which to search for app log files/dirs of interest")
	flag.StringVar(&ConfigPath, "config-path", "/etc/whalewatcher/config.yaml", "path to YAML configuration file for log monitoring")
	flag.StringVar(&ConfigVar, "config-var", "", "the name of the env var into which the YAML config will be injected")
}

func main() {
	flag.Parse()

	// TODO: if BaseDir doesn't exist (isn't mounted to whalewatcher's container already) then BOOM!

	conf, err := populateConfig()
	if err != nil {
		panic(err)
	}

	publisher := tailer.NewPublisher()

	ctx, cancelable := context.WithCancel(context.Background())

	for _, app := range conf.Apps {
		appTailer, err := tailer.Tail(ctx, publisher, app)
		if err != nil {
			panic(err)
		}

		go appTailer.Start()
	}

	// TODO: set up context with signal handlers to cancel and shutdown on SIGTERM, SIGINT
	defer cancelable()

	// TODO: set up HTTP server and handler that takes endpoints based on registered apps

	// TODO: Dockerfile and Docker Compose setup with example services to monitor

	// TODO: Makefile (and for docker container etc.)
}

func populateConfig() (*config.Config, error) {
	if len(ConfigVar) > 0 {
		return config.FromVar(ConfigVar)
	}

	if len(ConfigPath) > 0 {
		return config.FromFile(ConfigPath)
	}

	return nil, fmt.Errorf("failed to locate YAML config at path %q or in env var %q", ConfigPath, ConfigVar)
}
