package tailer

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSimplePublish(t *testing.T) {
	pub := NewPublisher()
	expected := []byte(`{"foo":{"ready":false,"error":""}}`)

	pub.Add("foo", Status{})
	got, status := pub.GetAll()

	require.Equal(t, 202, status)
	require.Equal(t, expected, got)
}

func TestMissingPublish(t *testing.T) {
	pub := NewPublisher()

	pub.Add("foo", Status{})
	_, status := pub.GetStatuses([]string{"bar"})

	require.Equal(t, 404, status)
}

func TestReadyPublish(t *testing.T) {
	pub := NewPublisher()
	now := time.Now().UTC()

	out := fmt.Sprintf(`{"foo":{"ready":true,"at":%q,"error":""}}`, now.Format(time.RFC3339Nano))
	expected := []byte(out)

	pub.Add("foo", Status{Ready: true, At: &now})
	got, status := pub.GetAll()

	require.Equal(t, 200, status)
	require.Equal(t, expected, got)
}

func TestPublishWithFilteredStatusCheck(t *testing.T) {
	pub := NewPublisher()
	now := time.Now().UTC()

	out := fmt.Sprintf(`{"foo":{"ready":true,"at":%q,"error":""}}`, now.Format(time.RFC3339Nano))
	expected := []byte(out)

	pub.Add("foo", Status{Ready: true, At: &now})
	pub.Add("bar", Status{})

	// one ready, one not, should respond w/status 202
	_, allStatus := pub.GetAll()
	require.Equal(t, 202, allStatus)

	// caller only cares about "foo" service - it's ready, status should be 200
	got, status := pub.GetStatuses([]string{"foo"})
	require.Equal(t, 200, status)
	require.Equal(t, expected, got)
}

func TestPublishWithFilteredStatusCheckAndError(t *testing.T) {
	pub := NewPublisher()
	now := time.Now().UTC()

	out := fmt.Sprintf(`{"foo":{"ready":true,"at":%q,"error":""}}`, now.Format(time.RFC3339Nano))
	expected := []byte(out)

	pub.Add("foo", Status{Ready: true, At: &now})
	pub.Add("bar", Status{})
	pub.Add("baz", Status{Ready: false, At: &now, Error: "ouch"})

	// one ready, one not, one fatal error - should respond w/status 503
	_, allStatus := pub.GetAll()
	require.Equal(t, 503, allStatus)

	// caller only cares about "foo" and "bar" services - one ready, one not, respond w/202
	_, status := pub.GetStatuses([]string{"foo", "bar"})
	require.Equal(t, 202, status)

	// caller only cares about "foo" service - it's ready, status should be 200
	got, status := pub.GetStatuses([]string{"foo"})
	require.Equal(t, 200, status)
	require.Equal(t, expected, got)
}
