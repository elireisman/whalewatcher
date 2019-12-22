package tailer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/elireisman/whalewatcher/config"
	"github.com/hpcloud/tail"
)

// status reported for each app
type Event struct {
	Ready bool
	At    time.Time
	Error string
}

// publishes status of each app, reporting when the log tailer
// has matched it's pattern and the app is warmed up and ready
// to serve, or an error message if the tailer fails.
type Publisher struct {
	lock   *sync.RWMutex
	logger *log.Logger
	state  map[string]Event
}

// Update status for a particular registered app
func (p *Publisher) Add(key string, evt Event) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.state[key] = evt
}

// Obtain status response for a selection of registered apps
func (p *Publisher) GetStatuses(apps []string) ([]byte, int) {
	p.lock.RLock()

	out := map[string]Event{}
	for _, name := range apps {
		evt, ok := p.state[name]
		// if there is no app by that name registered, 400
		if !ok {
			p.lock.RUnlock()
			msg := fmt.Sprintf("requested app (%s) is not registered", name)
			p.logger.Printf("ERROR %s", msg)
			return []byte(msg), http.StatusNotFound
		}

		out[name] = evt
	}
	p.lock.RUnlock()

	// if the event payload won't marshal, respond 500
	buf, err := json.Marshal(out)
	if err != nil {
		p.logger.Printf("ERROR failed to marshal status map: %s", err)
		return []byte("failed to serialize status map"), http.StatusInternalServerError
	}

	return buf, determineStatus(out)
}

// JSON status for all registered apps
func (p *Publisher) GetAll() ([]byte, int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	// if the event payload won't marshal, respond 500
	buf, err := json.Marshal(p.state)
	if err != nil {
		p.logger.Printf("ERROR failed to marshal status map: %s", err)
		return []byte("failed to marshal status map"), http.StatusInternalServerError
	}

	return buf, determineStatus(p.state)
}

func NewPublisher() *Publisher {
	return &Publisher{
		lock:   &sync.RWMutex{},
		logger: log.New(os.Stdout, "[publisher] ", log.LstdFlags),
		state:  map[string]Event{},
	}
}

type Tailer struct {
	Ctx       context.Context
	Name      string
	App       config.App
	Publisher *Publisher
	Pattern   *regexp.Regexp
	Driver    *tail.Tail
}

func Tail(ctx context.Context, pub *Publisher, name string, app config.App) (*Tailer, error) {
	tailCfg := tail.Config{
		Logger:    log.New(os.Stdout, fmt.Sprintf("[%s] ", name), log.LstdFlags),
		MustExist: true,
		Follow:    true,
	}

	t, err := tail.TailFile(app.Path, tailCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file for tailing at path %q, got: %s", app.Path, err)
	}

	check, err := regexp.Compile(app.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile log scanning pattern /%s/ got: %s", app.Pattern, err)
	}

	// stub an entry so callers to the HTTP service know the app is registered
	pub.Add(name, Event{})

	return &Tailer{
		Ctx:       ctx,
		Name:      name,
		App:       app,
		Publisher: pub,
		Pattern:   check,
		Driver:    t,
	}, nil
}

// to be run in a goroutine by the caller
func (t *Tailer) Start() {
	defer t.Driver.Cleanup()

	for {
		select {
		case err := <-t.Ctx.Done():
			t.Driver.Logger.Printf("INFO tailer for app %q shutting down: %s", t.Name, err)
			return

		case line, ok := <-t.Driver.Lines:
			if !ok {
				t.Driver.Logger.Printf("INFO tailer for app %q shutting down", t.Name)
				return
			}

			if line.Err != nil {
				evt := Event{
					At:    time.Now().UTC(),
					Error: line.Err.Error(),
				}
				t.Driver.Logger.Printf("ERROR fatal error while tailing log: %s", line.Err)
				t.Publisher.Add(t.Name, evt)
				return
			}

			if t.Pattern.MatchString(line.Text) {
				evt := Event{
					Ready: true,
					At:    time.Now().UTC(),
				}
				t.Driver.Logger.Printf("INFO log line matched for app %q", t.Name)
				t.Publisher.Add(t.Name, evt)
				return
			}
		}
	}
}

// HTTP Status code in a response is determined in aggregate
// based on the apps requested:
// - if any requested app has experienced a tailing error: 503
// - if any requested app is not ready yet: 202
// - if all requested apps are error free and ready: 200
func determineStatus(out map[string]Event) int {
	status := http.StatusOK
	for _, evt := range out {
		if len(evt.Error) > 0 {
			status = http.StatusServiceUnavailable
			break
		}
		if !evt.Ready {
			status = http.StatusAccepted
			break
		}
	}

	return status
}
