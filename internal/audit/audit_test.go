package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	events []Event
}

func (r *recorder) Write(ev Event) error {
	r.events = append(r.events, ev)
	return nil
}

func TestEmitterNotify(t *testing.T) {
	e := NewEmitter()
	one := &recorder{}
	two := &recorder{}
	e.Subscribe(one)
	e.Subscribe(two)

	ev := Event{TS: 100, Action: ActionShorten, UserID: "u1", URL: "https://example.com"}
	e.Notify(ev)

	require.Len(t, one.events, 1)
	require.Len(t, two.events, 1)
	assert.Equal(t, ev, one.events[0])
	assert.Equal(t, ev, two.events[0])
}

func TestEmitterUnsubscribe(t *testing.T) {
	e := NewEmitter()
	one := &recorder{}
	e.Subscribe(one)
	e.Unsubscribe(one)

	e.Notify(Event{Action: ActionShorten})

	assert.Empty(t, one.events)
}

func TestEmitterNotifyWithoutObservers(t *testing.T) {
	e := NewEmitter()
	e.Notify(Event{Action: ActionFollow})
}

func TestFileSinkWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	sink := NewFileSink(path)
	ev := Event{TS: time.Now().Unix(), Action: ActionFollow, UserID: "user-1", URL: "https://long.example/path"}

	require.NoError(t, sink.Write(ev))
	require.NoError(t, sink.Write(ev))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	require.Len(t, lines, 2)

	var got Event
	require.NoError(t, json.Unmarshal(lines[0], &got))
	assert.Equal(t, ev, got)
}

func TestHTTPSinkWrite(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewHTTPSink(srv.URL)
	ev := Event{TS: 42, Action: ActionShorten, UserID: "user-1", URL: "https://long.example/path"}

	require.NoError(t, sink.Write(ev))

	assert.EqualValues(t, ev.TS, received["ts"])
	assert.Equal(t, string(ev.Action), received["action"])
	assert.Equal(t, ev.UserID, received["user_id"])
	assert.Equal(t, ev.URL, received["url"])
}
