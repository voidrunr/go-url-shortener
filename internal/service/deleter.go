package service

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type deleteRequest struct {
	userID string
	codes  []string
}

const defaultBacklogLimit = 128

// deleter asynchronously marks URLs as deleted. Requests arriving from
// multiple handlers are accumulated in a buffer and flushed as batched
// updates (fan-in pattern).
type deleter struct {
	repo          Repository
	queue         chan deleteRequest
	sem           chan struct{}
	buffer        []deleteRequest
	flushInterval time.Duration
	bufferLimit   int
	done          chan struct{}
}

func newDeleter(repo Repository, flushInterval time.Duration, queueSize int, bufferLimit int) *deleter {
	return &deleter{
		repo:          repo,
		queue:         make(chan deleteRequest, queueSize),
		sem:           make(chan struct{}, defaultBacklogLimit),
		flushInterval: flushInterval,
		bufferLimit:   bufferLimit,
		done:          make(chan struct{}),
	}
}

func (d *deleter) start() {
	go d.worker()
}

// enqueue hands the request to the worker without ever blocking the caller.
// Once the queue is full, the send is delegated to a goroutine whose count is
// bounded by the semaphore; when both are saturated the request is dropped.
func (d *deleter) enqueue(req deleteRequest) {
	select {
	case d.queue <- req:
		return
	default:
	}

	select {
	case d.sem <- struct{}{}:
		go func() {
			defer func() { <-d.sem }()
			select {
			case d.queue <- req:
			case <-d.done:
			}
		}()
	default:
		log.Warn().Str("user_id", req.userID).Int("codes", len(req.codes)).Msg("delete queue full, dropping request")
	}
}

func (d *deleter) worker() {
	ticker := time.NewTicker(d.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case req := <-d.queue:
			d.buffer = append(d.buffer, req)
			if len(d.buffer) >= d.bufferLimit {
				d.flush()
			}
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
		if err := d.repo.DeleteBatch(context.Background(), userID, codes); err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("failed to delete urls")
		}
	}
}
