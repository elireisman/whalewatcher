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

func (p *Publisher) Add(key string, evt Event) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.state[key] = evt
}

func (p *Publisher) Get(key string) ([]byte, int) {
	evt, ok := p.state[key]
	if !ok {
		return []byte{}, http.StatusNotFound
	}

	buf, err := json.Marshal(evt)
	if err != nil {
		p.logger.Printf("ERROR failed to marshal event(%+v): %s", evt, err)
		return []byte{}, http.StatusInternalServerError
	}

	return buf, http.StatusOK
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
	App       config.App
	Publisher *Publisher
	Pattern   *regexp.Regexp
	Driver    *tail.Tail
}

func Tail(ctx context.Context, pub *Publisher, app config.App) (*Tailer, error) {
	tailCfg := tail.Config{
		Logger:    log.New(os.Stdout, fmt.Sprintf("[%s] ", app.Name), log.LstdFlags),
		MustExist: true,
		Follow:    true,
	}

	t, err := tail.TailFile(app.PathToLog, tailCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file for tailing at path %q, got: %s", app.PathToLog, err)
	}

	check, err := regexp.Compile(app.MessagePattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile log scanning pattern /%s/ got: %s", app.MessagePattern, err)
	}

	// stub an entry so callers to the HTTP service know the app is registered
	pub.Add(app.Name, Event{})

	return &Tailer{
		Ctx:       ctx,
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
			t.Driver.Logger.Printf("INFO tailer for app %q shutting down: %s", t.App.Name, err)
			return

		case line, ok := <-t.Driver.Lines:
			if !ok {
				t.Driver.Logger.Printf("INFO tailer for app %q shutting down", t.App.Name)
				return
			}

			if line.Err != nil {
				evt := Event{
					At:    time.Now().UTC(),
					Error: line.Err.Error(),
				}
				t.Driver.Logger.Printf("ERROR fatal error while tailing log: %s", line.Err)
				t.Publisher.Add(t.App.Name, evt)
				return
			}

			if t.Pattern.MatchString(line.Text) {
				evt := Event{
					Ready: true,
					At:    time.Now().UTC(),
				}
				t.Driver.Logger.Printf("INFO log line matched for app %q", t.App.Name)
				t.Publisher.Add(t.App.Name, evt)
				return
			}
		}
	}
}
