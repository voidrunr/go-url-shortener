package service

import (
	"time"

	"github.com/rs/zerolog/log"
)

type deleteRequest struct {
	userID string
	codes  []string
}

// deleter asynchronously marks URLs as deleted. Requests arriving from
// multiple handlers are accumulated in a buffer and flushed as batched
// updates (fan-in pattern).
type deleter struct {
	repo          Repository
	queue         chan deleteRequest
	buffer        []deleteRequest
	flushInterval time.Duration
	done          chan struct{}
}

func newDeleter(repo Repository, flushInterval time.Duration, queueSize int) *deleter {
	return &deleter{
		repo:          repo,
		queue:         make(chan deleteRequest, queueSize),
		flushInterval: flushInterval,
		done:          make(chan struct{}),
	}
}

func (d *deleter) start() {
	go d.worker()
}

func (d *deleter) enqueue(req deleteRequest) {
	d.queue <- req
}

func (d *deleter) worker() {
	ticker := time.NewTicker(d.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case req := <-d.queue:
			d.buffer = append(d.buffer, req)
		case <-ticker.C:
			d.flush()
		case <-d.done:
			return
		}
	}
}

func (d *deleter) flush() {
	if len(d.buffer) == 0 {
		return
	}

	groups := make(map[string][]string)
	for _, req := range d.buffer {
		groups[req.userID] = append(groups[req.userID], req.codes...)
	}
	d.buffer = d.buffer[:0]

	for userID, codes := range groups {
		if err := d.repo.DeleteBatch(userID, codes); err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("failed to delete urls")
		}
	}
}
