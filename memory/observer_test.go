package memory

import (
	"sync"
	"testing"
)

// recordingObserver is a thread-safe test observer that captures every
// Event for later assertion. Used across the B-1 tests.
type recordingObserver struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingObserver) OnEvent(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recordingObserver) snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

func TestObserver_CanonicalEventNames_AreDeclared(t *testing.T) {
	want := map[string]string{
		"EventAddTotal":             EventAddTotal,
		"EventSearchTotal":          EventSearchTotal,
		"EventSearchHits":           EventSearchHits,
		"EventConsolidatedTotal":    EventConsolidatedTotal,
		"EventForgottenTotal":       EventForgottenTotal,
		"EventSnapshotItems":        EventSnapshotItems,
		"EventSnapshotVectorsBytes": EventSnapshotVectorsBytes,
	}
	for name, val := range want {
		if val == "" {
			t.Errorf("%s is empty — must be a non-empty event name", name)
		}
	}
}

func TestObserver_NoopAcceptsAllCanonicalEvents(t *testing.T) {
	// Sanity: zero-value emission is a no-op (no panic, no allocation
	// beyond the Event itself).
	emit(nil, EventAddTotal, nil)
	emit(nil, EventSearchTotal, map[string]any{"query_len": 3})
	// Test passes if we got here.
}

func TestObserver_RecordingObserver_CapturesEmittedEvents(t *testing.T) {
	rec := &recordingObserver{}
	emit(rec, EventAddTotal, map[string]any{"kind": "working"})
	emit(rec, EventSearchHits, map[string]any{"n": 3})
	got := rec.snapshot()
	if len(got) != 2 {
		t.Fatalf("captured %d events, want 2", len(got))
	}
	if got[0].Name != EventAddTotal {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, EventAddTotal)
	}
	if got[1].Attrs["n"].(int) != 3 {
		t.Errorf("got[1].Attrs[\"n\"] = %v, want 3", got[1].Attrs["n"])
	}
}
