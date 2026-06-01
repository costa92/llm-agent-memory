package memory

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/costa92/llm-agent/llm"
	coremem "github.com/costa92/llm-agent/memory"
)

// newCoreEmbedder returns a deterministic ScriptedLLM embedder with
// 64-dim vectors — matches the pattern in
// github.com/costa92/llm-agent/memory/memory_test.go newWorking.
func newCoreEmbedder() coremem.Embedder {
	return llm.NewScriptedLLM(llm.WithEmbedDimensions(64))
}

// newEmbedder returns the deterministic embedder used by local-engine tests.
func newEmbedder() Embedder {
	return newCoreEmbedder()
}

// newWorking builds a local *WorkingMemory with capacity 16 and a 24h
// decay window. This is the preferred constructor for sibling-owned tests.
func newWorking(t *testing.T) *WorkingMemory {
	t.Helper()
	w, err := NewWorking(newEmbedder(), WorkingOptions{
		Capacity: 16,
		Decay:    24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("memory.NewWorking: %v", err)
	}
	return w
}

// newEpisodic builds a local *EpisodicMemory with default options.
func newEpisodic(t *testing.T) *EpisodicMemory {
	t.Helper()
	m, err := NewEpisodic(newEmbedder(), EpisodicOptions{})
	if err != nil {
		t.Fatalf("memory.NewEpisodic: %v", err)
	}
	return m
}

// newSemantic builds a local *SemanticMemory with default options.
func newSemantic(t *testing.T) *SemanticMemory {
	t.Helper()
	m, err := NewSemantic(newEmbedder(), SemanticOptions{})
	if err != nil {
		t.Fatalf("memory.NewSemantic: %v", err)
	}
	return m
}

// newWorkingWithCapacity builds a local *WorkingMemory with a custom
// capacity while keeping the standard 24h decay.
func newWorkingWithCapacity(t *testing.T, capacity int) *WorkingMemory {
	t.Helper()
	w, err := NewWorking(newEmbedder(), WorkingOptions{
		Capacity: capacity,
		Decay:    24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("memory.NewWorking(cap=%d): %v", capacity, err)
	}
	return w
}

// coreWorkingForAdapter builds a *coremem.WorkingMemory with capacity 16
// and a 24h decay window. It exists for the few tests that must exercise
// the core adapter bridge (AdaptCoreMemory / coremem.WithSanitizer); those
// tests still need a genuine core memory to wrap. Capacity is generous so
// eviction is not triggered by the small test corpora.
func coreWorkingForAdapter(t *testing.T) *coremem.WorkingMemory {
	t.Helper()
	w, err := coremem.NewWorking(newCoreEmbedder(), coremem.WorkingOptions{
		Capacity: 16,
		Decay:    24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("coremem.NewWorking: %v", err)
	}
	return w
}

// newCoreEpisodic builds a *coremem.EpisodicMemory with default options.
func newCoreEpisodic(t *testing.T) *coremem.EpisodicMemory {
	t.Helper()
	m, err := coremem.NewEpisodic(newCoreEmbedder(), coremem.EpisodicOptions{})
	if err != nil {
		t.Fatalf("coremem.NewEpisodic: %v", err)
	}
	return m
}

// newCoreSemantic builds a *coremem.SemanticMemory with default options.
func newCoreSemantic(t *testing.T) *coremem.SemanticMemory {
	t.Helper()
	m, err := coremem.NewSemantic(newCoreEmbedder(), coremem.SemanticOptions{})
	if err != nil {
		t.Fatalf("coremem.NewSemantic: %v", err)
	}
	return m
}

func newScopedManager(t *testing.T) *ScopedManager {
	t.Helper()
	w, e, s := newWorking(t), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w},
		Episodic: TierOptions{Memory: e, Lister: e, Exporter: e, Importer: e},
		Semantic: TierOptions{Memory: s, Lister: s, Exporter: s, Importer: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	sm, err := NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}
	return sm
}

// newCoreWorkingWithCapacity builds a *coremem.WorkingMemory with the
// requested capacity (24h decay). Use this in pagination tests where the
// default capacity of 16 from newCoreWorking is too small.
func newCoreWorkingWithCapacity(t *testing.T, capacity int) *coremem.WorkingMemory {
	t.Helper()
	w, err := coremem.NewWorking(newCoreEmbedder(), coremem.WorkingOptions{
		Capacity: capacity,
		Decay:    24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("coremem.NewWorking(cap=%d): %v", capacity, err)
	}
	return w
}

// jsonRoundTripSnap encodes then decodes a Snapshot through
// encoding/json. This forces Metadata maps to use the concrete types
// that the wire format actually produces (int → float64, etc.) so
// downstream readers like promotionCountOf are tested under the same
// conditions an Import-from-disk path would see.
func jsonRoundTripSnap(t *testing.T, snap Snapshot) Snapshot {
	t.Helper()
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal snapshot: %v", err)
	}
	var out Snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal snapshot: %v", err)
	}
	return out
}
