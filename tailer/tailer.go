package tailer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/elireisman/whalewatcher/config"

	docker_types "github.com/docker/docker/api/types"
	docker_filters "github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"github.com/hpcloud/tail"
)

// performs the log monitoring and status publishing for one service container
type Tailer struct {
	Ctx     context.Context
	Name    string
	ID      string
	Pattern *regexp.Regexp
	Await   time.Duration

	Publisher *Publisher
	Client    *docker.Client
	Driver    *tail.Tail
	Reader    io.ReadCloser
	Writer    *os.File

	Logger *log.Logger
	Done   chan bool
}

func New(ctx context.Context, client *docker.Client, pub *Publisher, containerName string, target config.Service, await time.Duration) (*Tailer, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("[monitoring: %s] ", containerName), log.LstdFlags)

	check, err := regexp.Compile(target.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile log scanning pattern /%s/ got: %s", target.Pattern, err)
	}
	logger.Printf("INFO regex pattern compiled: %s", target.Pattern)

	// register the specified app
	pub.Add(containerName, Status{})
	logger.Println("INFO container registered for monitoring")

	// the remaining fields will be populated when Start() is called
	return &Tailer{
		Ctx:       ctx,
		Name:      containerName,
		ID:        "UNKNOWN",
		Pattern:   check,
		Await:     await,
		Publisher: pub,
		Client:    client,
		Logger:    logger,
		Done:      make(chan bool),
	}, nil
}

// caller should execute this in a goroutine
func (t *Tailer) Start() {
	var err error

	// await target container startup and obtain container ID for this run
	if err = t.awaitContainerUp(); err != nil {
		t.publishError(err, "target container not up")
		return
	}

	// wire up the log stream from the container to our monitor
	opts := docker_types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true}
	t.Reader, err = t.Client.ContainerLogs(t.Ctx, t.ID, opts)
	if err != nil {
		t.publishError(err, "failed to obtain reader for container log stream")
		return
	}

	pipeFile := pipeName(t.Name, t.ID)
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	t.Writer, err = os.OpenFile(pipeFile, flags, os.ModeNamedPipe)
	if err != nil {
		t.publishError(err, "failed to open named pipe %s", pipeFile)
		return
	}

	tailCfg := tail.Config{
		Logger:    t.Logger,
		MustExist: true,
		Follow:    true,
		Pipe:      true,
	}
	t.Driver, err = tail.TailFile(pipeFile, tailCfg)
	if err != nil {
		t.publishError(err, "failed to tail container logs from pipe %s", pipeFile)
		return
	}

	defer func() {
		t.Driver.Cleanup()

		close(t.Done)
		t.Reader.Close()
		t.Writer.Close()
	}()

	// copy log stream from Docker client into named pipe for tailer to consume
	go func() {
		for {
			select {
			case <-t.Done:
				t.Logger.Printf("INFO named pipe %s closing", pipeFile)
				return

			default:
				if _, err := io.Copy(t.Writer, t.Reader); err != nil {
					t.Logger.Printf("ERROR copying container logs to named pipe %s: %s", pipeFile, err)
				}
			}
		}
	}()

	// consume log lines until the context is canceled (global shutdown triggered)
	// an unrecoverable tailing error occurs, or a matching log line is found
	for {
		select {
		case err := <-t.Ctx.Done():
			t.Logger.Printf("INFO tailer shutting down gracefully: %s", err)
			return

		case line, ok := <-t.Driver.Lines:
			if !ok {
				t.Logger.Printf("INFO tailer for service shutting down (feed closed)")
				return
			}

			if line.Err != nil {
				t.Logger.Printf("ERROR while tailing log for service: %s", line.Err)
				now := time.Now().UTC()
				t.Publisher.Add(t.Name, Status{
					At:    &now,
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

				t.Logger.Printf("INFO target pattern matched log %s", lineInfo)

				now := time.Now().UTC()
				t.Publisher.Add(t.Name, Status{
					Ready: true,
					At:    &now,
				})
				return
			}
		}
	}
}

func (t *Tailer) awaitContainerUp() error {
	t.Logger.Println("INFO awaiting container startup")

	timeoutCtx, cancelable := context.WithTimeout(t.Ctx, t.Await)
	defer cancelable()

	// obtain the container ID for the target service
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

FindIDLoop:
	for {
		select {
		case <-t.Ctx.Done():
			return fmt.Errorf("failed to obtain container ID for %s in %s: %s", t.Name, t.Await, t.Ctx.Err())

		case <-ticker.C:
			opts := docker_types.ContainerListOptions{Filters: docker_filters.NewArgs()}
			opts.Filters.Add("status", "running")

			containers, err := t.Client.ContainerList(timeoutCtx, opts)
			if err != nil {
				t.Logger.Printf("WARN failed to obtain container listing: %s", err)
				continue
			}

			for _, container := range containers {
				if t.Name == strings.TrimPrefix(container.Names[0], "/") {
					t.ID = container.ID
					t.Logger.Printf("INFO container %s is up", t.ID)
					break FindIDLoop
				}
			}
			t.Logger.Println("INFO awaiting container %s startup", t.Name)
		}
	}

	// await running status for the container, until the remaining wait time expires
	status, err := t.Client.ContainerWait(timeoutCtx, t.ID)
	if err != nil {
		return fmt.Errorf("problem while awaiting running status for container %s: %s", t.ID, err)
	}
	if status != 200 {
		return fmt.Errorf("container %s could not be verified as running in %s (status %d)", t.ID, t.Await, status)
	}

	return nil
}

func (t *Tailer) publishError(err error, format string, args ...interface{}) {
	msg := fmt.Sprintf(format+": "+err.Error(), args...)
	t.Logger.Println("ERROR " + msg)
	now := time.Now().UTC()
	t.Publisher.Add(t.Name, Status{At: &now, Error: msg})
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
