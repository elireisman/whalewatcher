package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/elireisman/whalewatcher/config"
	"github.com/elireisman/whalewatcher/tailer"

	docker "github.com/docker/docker/client"
)

var (
	ConfigPath string
	ConfigVar  string
	WaitMillis int
	Port       int
)

func init() {
	flag.StringVar(&ConfigPath, "config-path", "/etc/whalewatcher/config.yaml", "path to YAML config file")
	flag.StringVar(&ConfigVar, "config-var", "", "env var storing the YAML config; overrides config-path if present")
	flag.IntVar(&WaitMillis, "wait-millis", 60000, "time to await each container startup; also default time to await ready status")
	flag.IntVar(&Port, "port", 4444, "status API will be served on this port")
}

func main() {
	flag.Parse()

	conf, err := populateConfig()
	if err != nil {
		panic(err)
	}

	logger := log.New(os.Stdout, "[server] ", log.LstdFlags)
	publisher := tailer.NewPublisher()

	srv := &http.Server{
		Addr:     fmt.Sprintf(":%d", Port),
		Handler:  handler(publisher),
		ErrorLog: logger,
	}

	ctx, shutdownTailers := context.WithCancel(context.Background())
	shutdownComplete := make(chan bool)

	client, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer client.Close()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		logger.Printf("INFO graceful shutdown initiated")
		shutdownTailers()
		srv.Shutdown(context.Background())
		close(shutdownComplete)
	}()

	// start a log monitor for each registered service we found an ID for
	awaitStartup := time.Duration(WaitMillis) * time.Millisecond
	for name, svc := range conf.Containers {
		svcTailer, err := tailer.New(ctx, client, publisher, name, svc, awaitStartup)
		if err != nil {
			panic(err)
		}

		go svcTailer.Start()
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Printf("INFO Server shutting down (%s)", err)
	}

	<-shutdownComplete
	logger.Printf("INFO shutdown complete")
}

// build http.Handler that processes status events
func handler(pub *tailer.Publisher) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !checkMethod(w, r) {
			return
		}

		query := r.URL.Query()
		rawStatuses := query.Get("status")
		statuses := strings.Split(rawStatuses, ",")

		var out []byte
		var status int

		if len(rawStatuses) == 0 || (len(statuses) == 1 && statuses[0] == "*") {
			out, status = pub.GetAll()
		} else {
			out, status = pub.GetStatuses(statuses)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(out)
	})

	return mux
}

// hydrate the YAML configuration from a file or env var
func populateConfig() (*config.Config, error) {
	if len(ConfigVar) > 0 {
		return config.FromVar(ConfigVar)
	}

	if len(ConfigPath) > 0 {
		return config.FromFile(ConfigPath)
	}

	return nil, fmt.Errorf("failed to locate YAML config at path %q or in env var %q", ConfigPath, ConfigVar)
}

// ensure we only respond to GET methods
func checkMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}

	w.Header().Add("Allow", "GET")
	w.WriteHeader(http.StatusMethodNotAllowed)
	io.WriteString(w, "invalid request method")

	return false
}
