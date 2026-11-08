package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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

func closeEmitter(t *testing.T, e *Emitter) {
	t.Helper()
	require.NoError(t, e.Close())
}

func TestEmitterNotify(t *testing.T) {
	e := NewEmitter()
	one := &recorder{}
	two := &recorder{}
	e.Subscribe(one)
	e.Subscribe(two)

	ev := Event{TS: 100, Action: ActionShorten, UserID: "u1", URL: "https://example.com"}
	e.Notify(ev)
	closeEmitter(t, e)

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
	closeEmitter(t, e)

	assert.Empty(t, one.events)
}

func TestEmitterNotifyWithoutObservers(t *testing.T) {
	e := NewEmitter()
	e.Notify(Event{Action: ActionFollow})
	closeEmitter(t, e)
}

func TestEmitterCloseDrainsQueue(t *testing.T) {
	e := NewEmitter()
	rec := &recorder{}
	e.Subscribe(rec)

	const events = 100
	for i := 0; i < events; i++ {
		e.Notify(Event{TS: int64(i), Action: ActionShorten})
	}
	closeEmitter(t, e)

	require.Len(t, rec.events, events)
}

func TestFileSinkWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	sink := NewFileSink(path)
	ev := Event{TS: time.Now().Unix(), Action: ActionFollow, UserID: "user-1", URL: "https://long.example/path"}

	require.NoError(t, sink.Write(ev))
	require.NoError(t, sink.Write(ev))
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	require.Len(t, lines, 2)

	var got Event
	require.NoError(t, json.Unmarshal(lines[0], &got))
	assert.Equal(t, ev, got)
}

func TestFileSinkCloseIsIdempotent(t *testing.T) {
	sink := NewFileSink(filepath.Join(t.TempDir(), "audit.log"))

	require.NoError(t, sink.Close())
	require.NoError(t, sink.Close())
}

func TestFileSinkWriteAfterClose(t *testing.T) {
	sink := NewFileSink(filepath.Join(t.TempDir(), "audit.log"))
	require.NoError(t, sink.Close())

	err := sink.Write(Event{TS: 1, Action: ActionShorten})
	require.ErrorIs(t, err, os.ErrClosed)
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
	defer sink.Close()
	ev := Event{TS: 42, Action: ActionShorten, UserID: "user-1", URL: "https://long.example/path"}

	require.NoError(t, sink.Write(ev))

	assert.EqualValues(t, ev.TS, received["ts"])
	assert.Equal(t, string(ev.Action), received["action"])
	assert.Equal(t, ev.UserID, received["user_id"])
	assert.Equal(t, ev.URL, received["url"])
}

type failingSink struct {
	err error
}

func (f failingSink) Write(Event) error {
	return f.err
}

func TestEmitterNotifySinkErrorDoesNotPanic(t *testing.T) {
	e := NewEmitter()
	e.Subscribe(failingSink{err: errors.New("sink down")})

	e.Notify(Event{TS: 1, Action: ActionShorten, UserID: "u1", URL: "https://example.com"})
	closeEmitter(t, e)
}

func TestEmitterUnsubscribeUnknownSink(t *testing.T) {
	e := NewEmitter()
	keep := &recorder{}
	e.Subscribe(keep)
	e.Unsubscribe(&recorder{})

	e.Notify(Event{Action: ActionFollow})
	closeEmitter(t, e)

	require.Len(t, keep.events, 1)
}

type blockingSink struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingSink) Write(Event) error {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return nil
}

func TestEmitterNotifyDoesNotBlockOnSlowSink(t *testing.T) {
	e := NewEmitter()
	sink := &blockingSink{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	e.Subscribe(sink)

	e.Notify(Event{TS: 1, Action: ActionShorten})
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("sink did not start")
	}

	notified := make(chan struct{})
	go func() {
		e.Notify(Event{TS: 2, Action: ActionFollow})
		close(notified)
	}()
	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("Notify blocked on a slow sink")
	}

	close(sink.release)
	closeEmitter(t, e)
}

type closableSink struct {
	recorder
	closed bool
}

func (s *closableSink) Close() error {
	s.closed = true
	return nil
}

func TestEmitterCloseClosesSinks(t *testing.T) {
	e := NewEmitter()
	sink := &closableSink{}
	e.Subscribe(sink)

	e.Notify(Event{Action: ActionShorten})
	closeEmitter(t, e)

	assert.True(t, sink.closed)
	require.Len(t, sink.events, 1)
}

func TestEmitterConcurrentSubscribeNotify(t *testing.T) {
	e := NewEmitter()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sink := &recorder{}
			e.Subscribe(sink)
			e.Notify(Event{Action: ActionShorten})
			e.Unsubscribe(sink)
		}()
	}
	wg.Wait()
	closeEmitter(t, e)
}

func TestFileSinkWriteOpenError(t *testing.T) {
	sink := NewFileSink(filepath.Join(t.TempDir(), "missing-dir", "audit.log"))

	err := sink.Write(Event{TS: 1, Action: ActionShorten})
	require.Error(t, err)
}

func TestHTTPSinkWriteNetworkError(t *testing.T) {
	sink := NewHTTPSink("http://127.0.0.1:1/audit")
	defer sink.Close()

	err := sink.Write(Event{TS: 1, Action: ActionShorten})
	require.Error(t, err)
}
