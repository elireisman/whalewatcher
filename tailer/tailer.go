package tailer

import (
	"context"
	"fmt"
	"io"
	"log"
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
	// await target container startup and obtain container ID for this run
	if !t.obtainIDForRunningContainer() {
		return
	}

	// build pipeline to stream target container's logs into local tailer
	pipeFile := pipeName(t.Name, t.ID)
	if !t.buildLogPipeline(pipeFile) {
		return
	}

	defer func() {
		// tear down the tailer (named pipe -> log tailer)
		t.Driver.Cleanup()
		// trigger the Docker (log stream -> named pipe) to tear down
		close(t.Done)
	}()

	// copy log stream from Docker client into named pipe for tailer to consume
	go func() {
		defer func() {
			t.Writer.Close()
			t.Reader.Close()
		}()

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
	lineCount := 0
	for {
		select {
		case err := <-t.Ctx.Done():
			t.Logger.Printf("INFO tailer shutting down gracefully: %s", err)
			return

		case line, ok := <-t.Driver.Lines:
			lineCount++
			if !ok {
				t.Logger.Printf("INFO tailer for service shutting down (feed closed)")
				return
			}

			t.ProcessLine(line, lineCount)
		}
	}
}

// handle processing each log line, publish result if error or match occurs
func (t *Tailer) ProcessLine(line *tail.Line, lineCount int) {
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
		t.Logger.Printf("INFO target pattern matched at line %d: %s", lineCount, line.Text)

		now := time.Now().UTC()
		t.Publisher.Add(t.Name, Status{
			Ready: true,
			At:    &now,
		})
		return
	}
}

func (t *Tailer) buildLogPipeline(pipeFile string) bool {
	var err error

	// wire up the log stream from the container to our monitor
	opts := docker_types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true}
	t.Reader, err = t.Client.ContainerLogs(t.Ctx, t.ID, opts)
	if err != nil {
		t.publishError(err, "failed to obtain reader for container log stream")
		return false
	}

	// use a named pipe to stream the log output into a form the tailer can accept
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	t.Writer, err = os.OpenFile(pipeFile, flags, os.ModeNamedPipe)
	if err != nil {
		t.publishError(err, "failed to open named pipe %s", pipeFile)
		return false
	}

	// wire up the tailer
	tailCfg := tail.Config{
		Logger:    t.Logger,
		MustExist: true,
		Follow:    true,
		Pipe:      true,
	}
	t.Driver, err = tail.TailFile(pipeFile, tailCfg)
	if err != nil {
		t.publishError(err, "failed to tail container logs from pipe %s", pipeFile)
		return false
	}

	return true
}

// obtain the container ID for the target service, once it's up
func (t *Tailer) obtainIDForRunningContainer() bool {
	t.Logger.Printf("INFO awaiting container startup for interval: %s", t.Await)

	timeoutCtx, cancelable := context.WithTimeout(t.Ctx, t.Await)
	defer cancelable()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

FindIDLoop:
	for {
		select {
		case <-timeoutCtx.Done():
			t.publishError(timeoutCtx.Err(), "failed to obtain container ID for %s in %s", t.Name, t.Await)
			return false

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
			t.Logger.Println("INFO awaiting container startup")
		}
	}

	return true
}

func (t *Tailer) publishError(err error, format string, args ...interface{}) {
	msg := fmt.Sprintf(format+": "+err.Error(), args...)
	t.Logger.Println("ERROR " + msg)
	now := time.Now().UTC()
	t.Publisher.Add(t.Name, Status{At: &now, Error: msg})
}

func pipeName(containerName, containerID string) string {
	return fmt.Sprintf("%s_%s_ww", containerName, containerID)
}
