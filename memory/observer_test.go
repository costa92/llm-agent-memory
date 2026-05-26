package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	coremem "github.com/costa92/llm-agent/memory"
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
	// These string values are part of the public v0.2.0 contract.
	// Changing any of them breaks downstream adapters (Prometheus, OTel,
	// log emitters) silently. New events may be added; existing names
	// must never be renamed or removed.
	want := map[string]string{
		"EventAddTotal":             "memory_add_total",
		"EventSearchTotal":          "memory_search_total",
		"EventSearchHits":           "memory_search_hits",
		"EventConsolidatedTotal":    "memory_consolidated_total",
		"EventForgottenTotal":       "memory_forgotten_total",
		"EventSnapshotItems":        "memory_snapshot_items",
		"EventSnapshotVectorsBytes": "memory_snapshot_vectors_bytes",
	}
	got := map[string]string{
		"EventAddTotal":             EventAddTotal,
		"EventSearchTotal":          EventSearchTotal,
		"EventSearchHits":           EventSearchHits,
		"EventConsolidatedTotal":    EventConsolidatedTotal,
		"EventForgottenTotal":       EventForgottenTotal,
		"EventSnapshotItems":        EventSnapshotItems,
		"EventSnapshotVectorsBytes": EventSnapshotVectorsBytes,
	}
	for name, wantVal := range want {
		if got[name] != wantVal {
			t.Errorf("%s = %q, want %q — public contract violation", name, got[name], wantVal)
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

// typedNilObserver demonstrates the documented "interface wrapping nil
// concrete pointer" footgun. This test pins the contract — `emit` does
// NOT use reflection to detect this case. If a future change tries to
// add reflection-based nil detection (which would slow the hot path),
// this test will catch the behavioral change.
//
// OnEvent dereferences the receiver (touches a field) so the call
// panics with a real-world nil-deref signature — matching what any
// non-trivial Observer (e.g. recordingObserver with its mutex) would
// do. An empty method body would NOT panic on a nil pointer receiver
// in Go, which would silently hide the footgun.
type typedNilObserver struct {
	calls int
}

func (t *typedNilObserver) OnEvent(Event) { t.calls++ }

func TestObserver_TypedNilInterface_PanicsAsDocumented(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on typed-nil interface, got nil — " +
				"if reflection was added to emit, document the new contract")
		}
	}()
	var r *typedNilObserver // nil concrete pointer
	var o Observer = r      // interface wrapping nil — does NOT equal nil interface
	emit(o, EventAddTotal, nil)
	t.Errorf("emit did not panic — see typedNilObserver docstring")
}

func TestObserver_ScopedLifecycleManager_AcceptsWithObserver(t *testing.T) {
	rec := &recordingObserver{}
	slm, err := NewScopedLifecycleManager(newCoreScopedManager(t), WithObserver(rec))
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}
	if slm.observer() != rec {
		t.Errorf("WithObserver did not install the observer reference")
	}
}

func TestObserver_Consolidator_AcceptsWithObserver(t *testing.T) {
	rec := &recordingObserver{}
	c, err := NewConsolidator(newCoreManager(t), WithObserver(rec))
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}
	if c.observer() != rec {
		t.Errorf("WithObserver did not install the observer reference")
	}
}

func TestObserver_UnifiedSearcher_AcceptsWithObserver(t *testing.T) {
	rec := &recordingObserver{}
	u, err := NewUnifiedSearcher(newCoreManager(t), WithObserver(rec))
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}
	if u.observer() != rec {
		t.Errorf("WithObserver did not install the observer reference")
	}
}

func TestObserver_B2_WorkingEvictionStillPicksLowestScoredItem(t *testing.T) {
	// B-2 — embed-reuse — lives inside coremem.WorkingMemory's private
	// evictIfOverCapacity. It cannot be wrapped from this sibling. This
	// test does NOT assert embed call count (that would require an
	// embedder spy that does not exist in the sibling); it pins the
	// observable property B-2 promises to preserve: when capacity is
	// exceeded, the LOWEST-scored item is evicted. When the upstream
	// optimization lands, this test must continue to pass.
	w, err := coremem.NewWorking(newCoreEmbedder(), coremem.WorkingOptions{
		Capacity: 2,
		Decay:    24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWorking: %v", err)
	}
	mgr, err := coremem.NewManager(coremem.ManagerOptions{
		Working:  w,
		Episodic: newCoreEpisodic(t),
		Semantic: newCoreSemantic(t),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "low", Importance: 0.1}); err != nil {
		t.Fatalf("Add low: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "mid", Importance: 0.5}); err != nil {
		t.Fatalf("Add mid: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "high", Importance: 0.9}); err != nil {
		t.Fatalf("Add high (triggers eviction): %v", err)
	}

	pages, err := mgr.ListAll(ctx, coremem.ListFilter{}, 100, nil)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	contents := map[string]bool{}
	for _, it := range pages[coremem.KindWorking].Items {
		contents[it.Content] = true
	}
	if contents["low"] {
		t.Errorf("expected the lowest-scored item to be evicted; survivors: %v", contents)
	}
	if !contents["high"] {
		t.Errorf("expected the highest-scored item to survive; survivors: %v", contents)
	}
}
