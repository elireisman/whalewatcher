package tailer

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// status reported for each app
type Status struct {
	Ready bool       `json:"ready"`
	At    *time.Time `json:"at,omitempty"`
	Error string     `json:"error"`
}

// obtain a publisher
func NewPublisher() *Publisher {
	return &Publisher{
		lock:   &sync.RWMutex{},
		logger: log.New(os.Stdout, "[publisher] ", log.LstdFlags),
		state:  map[string]Status{},
	}
}

// publishes status of each app, reporting when the log tailer
// has matched it's pattern and the app is warmed up and ready
// to serve, or an error message if the tailer fails.
type Publisher struct {
	lock   *sync.RWMutex
	logger *log.Logger
	state  map[string]Status
}

// Update status for a particular registered app
func (p *Publisher) Add(key string, evt Status) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.state[key] = evt
}

// Obtain serialized status update for a selection of registered services
func (p *Publisher) GetStatuses(services []string) ([]byte, int) {
	out, err := p.populate(services)
	if err != nil {
		// error means some services in the list weren't found; respond w/404
		return []byte(err.Error()), http.StatusNotFound
	}

	// if the event payload won't marshal, respond w/500
	buf, err := json.Marshal(out)
	if err != nil {
		p.logger.Printf("ERROR failed to marshal status map: %s", err)
		return []byte("failed to serialize status map"), http.StatusInternalServerError
	}

	// respond w/200 if all services are ready, otherwise 202
	return buf, determineStatus(out)
}

// Obtain serialized status update for all registered apps
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

// fetch status updates only for the registered services supplied by the caller
func (p *Publisher) populate(services []string) (map[string]Status, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	out := map[string]Status{}
	for _, name := range services {
		evt, ok := p.state[name]
		if !ok {
			// if there is no app by that name registered, error
			msg := fmt.Sprintf("requested service (%s) is not registered", name)
			p.logger.Printf("ERROR %s", msg)
			return nil, errors.New(msg)
		}

		out[name] = evt
	}

	return out, nil
}
