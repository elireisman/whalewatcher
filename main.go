package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/elireisman/whalewatcher/config"
	"github.com/elireisman/whalewatcher/tailer"
)

var (
	ConfigPath string
	ConfigVar  string
	Port       int
)

func init() {
	flag.StringVar(&ConfigPath, "config-path", "/etc/whalewatcher/config.yaml", "path to YAML config file")
	flag.StringVar(&ConfigVar, "config-var", "", "env var storing the YAML config; overrides config-path if present")
	flag.IntVar(&Port, "port", 8080, "port to serve the readiness check endpoint on")
}

func main() {
	flag.Parse()

	conf, err := populateConfig()
	if err != nil {
		panic(err)
	}

	publisher := tailer.NewPublisher()

	mux := buildMux(publisher)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", Port),
		Handler: mux,
	}

	ctx, shutdownTailers := context.WithCancel(context.Background())
	shutdownComplete := make(chan bool)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		shutdownTailers()
		srv.Shutdown(context.Background())
		close(shutdownComplete)
	}()

	for name, app := range conf.Apps {
		appTailer, err := tailer.Tail(ctx, publisher, name, app)
		if err != nil {
			panic(err)
		}

		go appTailer.Start()
	}

	if err := srv.ListenAndServe(); err != nil {
		// TODO: logger, note this in [whalewatcher] scope!
	}

	<-shutdownComplete
}

// compose handler tree for /api and /html
func buildMux(pub *tailer.Publisher) http.Handler {
	mux := http.NewServeMux()

	// displays all app statuses and redirects to /api endpoint
	// if an app name appears on the path
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !checkMethod(w, r) {
			return
		}

		appName := strings.TrimPrefix(r.URL.Path, "/")
		if len(appName) > 0 {
			newPath := fmt.Sprintf("%s://%s/api/%s", r.URL.Scheme, r.URL.Host, appName)
			http.Redirect(w, r, newPath, 301)
			return
		}

		out, status := pub.GetAll()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(out)
	})

	// programmatic access, returns plain JSON
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if !checkMethod(w, r) {
			return
		}

		appName := strings.TrimPrefix(r.URL.Path, "/api/")
		if len(appName) == 0 {
			http.Error(w, "no app name provided in URL path", http.StatusBadRequest)
			return
		}

		out, status := pub.Get(appName)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(out)
	})

	return mux
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

func checkMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}

	w.Header().Add("Allow", "GET")
	w.WriteHeader(http.StatusMethodNotAllowed)
	io.WriteString(w, "invalid request method")

	return false
}
