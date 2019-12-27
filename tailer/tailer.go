package tailer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/elireisman/whalewatcher/config"

	docker_types "github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/hpcloud/tail"
)

// performs the log monitoring and status publishing for one service container
type Tailer struct {
	Ctx  context.Context
	Name string
	ID   string

	Service   config.Service
	Publisher *Publisher
	Pattern   *regexp.Regexp

	Client *docker.Client
	Driver *tail.Tail
	Reader io.ReadCloser
	Writer *os.File

	Done chan bool
}

func Tail(ctx context.Context, client *docker.Client, pub *Publisher, containerName string, target config.Service) (*Tailer, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] [%s] ", containerName, target.ID), log.LstdFlags)

	options := docker_types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true}
	rdr, err := client.ContainerLogs(ctx, target.ID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain reader for container log stream: %s", err)
	}

	check, err := regexp.Compile(target.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile log scanning pattern /%s/ got: %s", target.Pattern, err)
	}

	pipeFile := pipeName(containerName, target.ID)
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	wtr, err := os.OpenFile(pipeFile, flags, os.ModeNamedPipe)
	if err != nil {
		return nil, fmt.Errorf("failed to open named pipe %s: %s", pipeFile, err)
	}

	tailCfg := tail.Config{
		Logger:    logger,
		MustExist: true,
		Follow:    true,
		Pipe:      true,
	}
	t, err := tail.TailFile(pipeFile, tailCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to tail container logs from pipe %s: %s", pipeFile, err)
	}

	// now this tailer can be safely registered and visible to callers
	pub.Add(containerName, Status{})

	return &Tailer{
		Ctx:       ctx,
		Name:      containerName,
		ID:        target.ID,
		Publisher: pub,
		Pattern:   check,
		Driver:    t,
		Reader:    rdr,
		Writer:    wtr,
		Done:      make(chan bool),
	}, nil
}

// to be run in a goroutine by the caller
func (t *Tailer) Start() {
	defer func() {
		t.Driver.Cleanup()

		close(t.Done)
		t.Reader.Close()
		t.Writer.Close()
	}()

	pipeFile := pipeName(t.Name, t.ID)

	// copy log stream from Docker client into named pipe for tailer to consume
	go func() {
		for {
			select {
			case <-t.Done:
				t.Driver.Logger.Printf("INFO named pipe %s closing", pipeFile)
				return

			default:
				if _, err := io.Copy(t.Writer, t.Reader); err != nil {
					t.Driver.Logger.Printf("ERROR copying container logs to named pipe %s: %s", pipeFile, err)
				}
			}
		}
	}()

	// consume log lines until the context is canceled (global shutdown triggered)
	// an unrecoverable tailing error occurs, or a matching log line is found
	for {
		select {
		case err := <-t.Ctx.Done():
			t.Driver.Logger.Printf("INFO tailer shutting down gracefully: %s", err)
			return

		case line, ok := <-t.Driver.Lines:
			if !ok {
				t.Driver.Logger.Printf("INFO tailer for service shutting down (feed closed)")
				return
			}

			if line.Err != nil {
				t.Driver.Logger.Printf("ERROR while tailing log for service: %s", line.Err)
				t.Publisher.Add(t.Name, Status{
					At:    time.Now().UTC(),
					Error: line.Err.Error(),
				})
				return
			}

			if t.Pattern.MatchString(line.Text) {
				var lineInfo string
				lineNum, err := t.Driver.Tell()
				if err == nil {
					lineInfo = fmt.Sprintf("near line %d", lineNum)
				}

				t.Driver.Logger.Printf("INFO target pattern matched log %s", lineInfo)

				t.Publisher.Add(t.Name, Status{
					Ready: true,
					At:    time.Now().UTC(),
				})
				return
			}
		}
	}
}

// HTTP Status code in a response is determined
// in aggregate based on the apps requested:
//
// - if any tailed service (in a user request) is not registered: 404
// - if any service has experienced a tailing error: 503
// - if the status update list fails to serialize: 500
// - if any tailed service is not ready yet: 202
// - if all tailed services are error free and ready: 200
func determineStatus(out map[string]Status) int {
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

func pipeName(containerName, containerID string) string {
	return fmt.Sprintf("%s_%s_whalewatcher", containerName, containerID)
}
