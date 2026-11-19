package audit

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// Action identifies the audited operation.
type Action string

const (
	// ActionShorten is emitted when a URL is shortened.
	ActionShorten Action = "shorten"
	// ActionFollow is emitted when a short URL is resolved.
	ActionFollow Action = "follow"
)

// Event is a single audit record.
type Event struct {
	// Action is the audited operation.
	Action Action `json:"action"`
	// UserID is the authenticated user ID, or empty for anonymous requests.
	UserID string `json:"user_id"`
	// URL is the original URL involved in the operation.
	URL string `json:"url"`
	// TS is the event time as a Unix timestamp in seconds.
	TS int64 `json:"ts"`
}

// Sink receives audit events. Write implementations must be safe for
// concurrent use because an Emitter may deliver to the same sink from more
// than one emitter instance.
type Sink interface {
	Write(Event) error
}

const defaultQueueSize = 256

// Emitter fans out audit events to subscribed sinks in the background.
//
// Subscriptions are stored as an immutable snapshot, so Notify does not take a
// writer lock and does not allocate. Notify never blocks on sink I/O; when a
// sink queue is full the event for that sink is dropped and a warning is
// logged.
//
// An Emitter must be closed when it is no longer needed.
type Emitter struct {
	workers atomic.Pointer[[]*sinkWorker]
	mu      sync.RWMutex
	closed  bool
}

// NewEmitter returns an Emitter with no subscribers.
func NewEmitter() *Emitter {
	e := &Emitter{}
	empty := []*sinkWorker{}
	e.workers.Store(&empty)
	return e
}

// Subscribe adds o to the set of sinks that receive events. Subscribing the
// same sink more than once is allowed and results in duplicate delivery.
// Subscribe is a no-op after Close.
func (e *Emitter) Subscribe(o Sink) {
	if o == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}

	current := *e.workers.Load()
	next := make([]*sinkWorker, len(current)+1)
	copy(next, current)
	next[len(current)] = newSinkWorker(o)
	e.workers.Store(&next)
}

// Unsubscribe removes the first subscription of o and stops its worker. Events
// already queued for that subscription are delivered before the worker exits.
func (e *Emitter) Unsubscribe(o Sink) {
	e.mu.Lock()
	current := *e.workers.Load()

	index := -1
	for i, w := range current {
		if w.sink == o {
			index = i
			break
		}
	}
	if index == -1 {
		e.mu.Unlock()
		return
	}

	next := make([]*sinkWorker, 0, len(current)-1)
	next = append(next, current[:index]...)
	next = append(next, current[index+1:]...)
	e.workers.Store(&next)
	removed := current[index]
	e.mu.Unlock()

	_ = removed.close()
}

// Notify enqueues ev for delivery to all subscribers. It never waits for sink
// I/O. An event is dropped for a sink whose queue is full.
func (e *Emitter) Notify(ev Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed {
		return
	}

	for _, w := range *e.workers.Load() {
		w.enqueue(ev)
	}
}

// Close stops accepting new events, drains every sink queue, and closes sinks
// that implement io.Closer. It is safe to call Close more than once.
func (e *Emitter) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	workers := *e.workers.Load()
	empty := []*sinkWorker{}
	e.workers.Store(&empty)
	e.mu.Unlock()

	var errs []error
	for _, w := range workers {
		if err := w.close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type sinkWorker struct {
	sink      Sink
	queue     chan Event
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func newSinkWorker(sink Sink) *sinkWorker {
	w := &sinkWorker{
		sink:  sink,
		queue: make(chan Event, defaultQueueSize),
		done:  make(chan struct{}),
	}
	w.wg.Add(1)
	go w.run()
	return w
}

func (w *sinkWorker) enqueue(ev Event) {
	select {
	case w.queue <- ev:
	default:
		log.Warn().
			Int64("ts", ev.TS).
			Str("action", string(ev.Action)).
			Msg("audit queue full, dropping event")
	}
}

func (w *sinkWorker) run() {
	defer w.wg.Done()

	for {
		select {
		case ev := <-w.queue:
			w.deliver(ev)
		case <-w.done:
			w.drain()
			return
		}
	}
}

func (w *sinkWorker) drain() {
	for {
		select {
		case ev := <-w.queue:
			w.deliver(ev)
		default:
			return
		}
	}
}

func (w *sinkWorker) deliver(ev Event) {
	if err := w.sink.Write(ev); err != nil {
		log.Error().Err(err).
			Int64("ts", ev.TS).
			Str("action", string(ev.Action)).
			Msg("failed to deliver audit event")
	}
}

func (w *sinkWorker) close() error {
	w.closeOnce.Do(func() {
		close(w.done)
	})
	w.wg.Wait()

	if closer, ok := w.sink.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
