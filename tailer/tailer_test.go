package tailer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/elireisman/whalewatcher/config"

	"github.com/hpcloud/tail"
	"github.com/stretchr/testify/require"
)

func TestLineMatch(t *testing.T) {
	targetConf := config.Container{Pattern: `[Tt]est x?foo \d+$`}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)

	line := &tail.Line{Text: "this is a Test foo 123"}
	tailer.ProcessLine(line, 1)

	require.True(t, pub.state["foo"].Ready)
}

func TestLineMatchMultiPattern(t *testing.T) {
	targetConf := config.Container{
		Patterns: []string{
			`[Tt]est x?foo \d+$`,
			`^[A-Gx-z]+ \d+.\d+`,
		},
	}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)

	line := &tail.Line{Text: "this is a Test foo 123"}
	tailer.ProcessLine(line, 1)

	require.True(t, pub.state["foo"].Ready)
}

func TestLineNoMatch(t *testing.T) {
	targetConf := config.Container{Pattern: `[Tt]est \d+`}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)

	line := &tail.Line{Text: "no similarity to speak of"}
	tailer.ProcessLine(line, 1)
	require.False(t, pub.state["foo"].Ready)

	line = &tail.Line{Text: "test 2"}
	tailer.ProcessLine(line, 2)
	require.True(t, pub.state["foo"].Ready)
}

func TestLineMatchWithMaxWaitOverride(t *testing.T) {
	targetConf := config.Container{Pattern: `[Tt]est x?foo \d+$`, MaxWaitMillis: 2000}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)
	require.Equal(t, 2*time.Second, tailer.AwaitReady)

	line := &tail.Line{Text: "this is a Test foo 123"}
	tailer.ProcessLine(line, 1)

	require.True(t, pub.state["foo"].Ready)
}

func TestLineMatchWithMaxWaitDefault(t *testing.T) {
	targetConf := config.Container{Pattern: `[Tt]est x?foo \d+$`}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)
	require.Equal(t, tailer.AwaitStartup, tailer.AwaitReady)

	line := &tail.Line{Text: "this is a Test foo 123"}
	tailer.ProcessLine(line, 1)

	require.True(t, pub.state["foo"].Ready)
}

func TestLineMatchWithSinceFilter(t *testing.T) {
	targetConf := config.Container{
		Pattern: `[Tt]est x?foo \d+$`,
		Since:   "12h",
	}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)
	require.Equal(t, 12*time.Hour, tailer.Since)

	line := &tail.Line{Text: "this is a Test foo 123"}
	tailer.ProcessLine(line, 1)

	require.True(t, pub.state["foo"].Ready)
}

func TestLineMatchWithSinceDefault(t *testing.T) {
	targetConf := config.Container{
		Pattern: `[Tt]est x?foo \d+$`,
	}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), tailer.Since)

	line := &tail.Line{Text: "this is a Test foo 123"}
	tailer.ProcessLine(line, 1)

	require.True(t, pub.state["foo"].Ready)
}

func TestLineMatchWithInvalidSince(t *testing.T) {
	targetConf := config.Container{
		Pattern: `[Tt]est x?foo \d+$`,
		Since:   "XX__#23",
	}
	pub := NewPublisher()
	_, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.Error(t, err)
}

func TestLineError(t *testing.T) {
	targetConf := config.Container{Pattern: `[Tt]est \d+`}
	pub := NewPublisher()
	tailer, err := New(context.TODO(), nil, pub, "foo", targetConf, time.Second)
	require.NoError(t, err)

	line := &tail.Line{Text: "foo bar baz", Err: fmt.Errorf("oh the humanity")}
	tailer.ProcessLine(line, 1)

	require.False(t, pub.state["foo"].Ready)
	require.NotEmpty(t, pub.state["foo"].Error)
}
