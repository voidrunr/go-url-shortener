// Package audit provides best-effort asynchronous delivery of audit events.
//
// Events are described by Event and delivered to every subscribed Sink. An
// Emitter owns one buffered queue and one worker per sink, so a slow sink does
// not delay requests or other sinks. Delivery is at-most-once: events are not
// retried, and an event is dropped when the queue of a sink is full.
//
// Call Close on the emitter during shutdown to drain queued events and release
// resources held by sinks.
package audit
