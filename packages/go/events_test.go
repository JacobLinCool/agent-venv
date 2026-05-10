package agentvenv

import (
	"testing"
	"time"
)

func TestEventLogEmitsMonotonicTimestamps(t *testing.T) {
	log := newEventLog(nil)
	a := log.emit(EventEnvCreated, map[string]any{"path": "/tmp/x"})
	time.Sleep(2 * time.Millisecond)
	b := log.emit(EventProfileMaterialized, map[string]any{"file_count": 1})
	if a.TimestampMs > b.TimestampMs {
		t.Fatalf("timestamps not monotonic: %d > %d", a.TimestampMs, b.TimestampMs)
	}
	all := log.all()
	if len(all) != 2 {
		t.Fatalf("expected 2 events, got %d", len(all))
	}
	if all[0].Kind != EventEnvCreated || all[1].Kind != EventProfileMaterialized {
		t.Fatal("event order wrong")
	}
}

func TestEventSinkReceivesCallback(t *testing.T) {
	var received []Event
	sink := EventSinkFunc(func(e Event) { received = append(received, e) })
	log := newEventLog(sink)
	log.emit(EventEnvCreated, nil)
	log.emit(EventEnvDestroyed, nil)
	if len(received) != 2 {
		t.Fatalf("sink saw %d events", len(received))
	}
}

func TestEventDetailMerge(t *testing.T) {
	log := newEventLog(nil)
	e := log.emit(EventEnvCreated, map[string]any{"name": "foo"})
	name, _ := e.Detail["name"].(string)
	if name != "foo" {
		t.Fatalf("detail.name: %v", e.Detail["name"])
	}
}
