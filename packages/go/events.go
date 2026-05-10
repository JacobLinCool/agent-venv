package agentvenv

import (
	"sync"
	"time"
)

// EventKind is one of the well-known kinds in spec/events.schema.json.
// Implementations may emit additional vendor-prefixed kinds (x.go.something)
// but only these are part of the cross-language contract.
type EventKind string

const (
	EventEnvCreated          EventKind = "env.created"
	EventEnvAttached         EventKind = "env.attached"
	EventProfileMaterialized EventKind = "profile.materialized"
	EventCredentialsCopied   EventKind = "credentials.copied"
	EventCredentialsRefresh  EventKind = "credentials.refreshed"
	EventEnvDestroyed        EventKind = "env.destroyed"
	EventRegistryRead        EventKind = "registry.read"
	EventRegistryWritten     EventKind = "registry.written"
	EventError               EventKind = "error"
)

// Event is one entry in an environment's event log.
type Event struct {
	TimestampMs int64          `json:"ts_ms"`
	Kind        EventKind      `json:"kind"`
	Detail      map[string]any `json:"-"`
}

// EventSink receives events as they are emitted. Implementations must not
// block; the library does not buffer for slow sinks.
type EventSink interface{ Emit(Event) }

// EventSinkFunc adapts a plain function into an EventSink.
type EventSinkFunc func(Event)

func (f EventSinkFunc) Emit(e Event) { f(e) }

type eventLog struct {
	mu     sync.Mutex
	origin time.Time
	items  []Event
	sink   EventSink
}

func newEventLog(sink EventSink) *eventLog {
	return &eventLog{origin: time.Now(), sink: sink}
}

func (l *eventLog) emit(kind EventKind, detail map[string]any) Event {
	e := Event{
		TimestampMs: time.Since(l.origin).Milliseconds(),
		Kind:        kind,
		Detail:      detail,
	}
	l.mu.Lock()
	l.items = append(l.items, e)
	l.mu.Unlock()
	if l.sink != nil {
		defer func() { _ = recover() }()
		l.sink.Emit(e)
	}
	return e
}

func (l *eventLog) all() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Event, len(l.items))
	copy(out, l.items)
	return out
}
