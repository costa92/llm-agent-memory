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

// newCoreWorking builds a *coremem.WorkingMemory with capacity 16
// and a 24h decay window. Capacity is generous so eviction is not
// triggered by the small test corpora.
func newCoreWorking(t *testing.T) *coremem.WorkingMemory {
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

// newCoreManager wires all three memory kinds into a *coremem.Manager.
func newCoreManager(t *testing.T) *coremem.Manager {
	t.Helper()
	mgr, err := coremem.NewManager(coremem.ManagerOptions{
		Working:  newCoreWorking(t),
		Episodic: newCoreEpisodic(t),
		Semantic: newCoreSemantic(t),
	})
	if err != nil {
		t.Fatalf("coremem.NewManager: %v", err)
	}
	return mgr
}

// newCoreScopedManager wraps the manager produced by newCoreManager.
func newCoreScopedManager(t *testing.T) *coremem.ScopedManager {
	t.Helper()
	sm, err := coremem.NewScopedManager(newCoreManager(t))
	if err != nil {
		t.Fatalf("coremem.NewScopedManager: %v", err)
	}
	return sm
}

// jsonRoundTripSnap encodes then decodes a Snapshot through
// encoding/json. This forces Metadata maps to use the concrete types
// that the wire format actually produces (int → float64, etc.) so
// downstream readers like promotionCountOf are tested under the same
// conditions an Import-from-disk path would see.
func jsonRoundTripSnap(t *testing.T, snap coremem.Snapshot) coremem.Snapshot {
	t.Helper()
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal snapshot: %v", err)
	}
	var out coremem.Snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal snapshot: %v", err)
	}
	return out
}
