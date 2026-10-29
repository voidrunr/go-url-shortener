package audit

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

type Event struct {
	TS     int64  `json:"ts"`
	Action Action `json:"action"`
	UserID string `json:"user_id"`
	URL    string `json:"url"`
}

type Sink interface {
	Write(Event) error
}

type Emitter struct {
	mu        sync.RWMutex
	observers []Sink
}

func NewEmitter() *Emitter {
	return &Emitter{}
}

func (e *Emitter) Subscribe(o Sink) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.observers = append(e.observers, o)
}

func (e *Emitter) Unsubscribe(o Sink) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, obs := range e.observers {
		if obs == o {
			e.observers = append(e.observers[:i], e.observers[i+1:]...)
			return
		}
	}
}

func (e *Emitter) Notify(ev Event) {
	e.mu.RLock()
	observers := make([]Sink, len(e.observers))
	copy(observers, e.observers)
	e.mu.RUnlock()

	for _, obs := range observers {
		if err := obs.Write(ev); err != nil {
			log.Error().Err(err).
				Int64("ts", ev.TS).
				Str("action", string(ev.Action)).
				Msg("failed to deliver audit event")
		}
	}
}
