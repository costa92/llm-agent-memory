package memory

import (
	"context"
	"errors"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

// TestRecallEngine_PublicShape_Compiles pins the v1.0.0 surface of the
// D-2 RecallEngine facade. Any field rename, removal, or type change
// breaks compilation here.
func TestRecallEngine_PublicShape_Compiles(t *testing.T) {
	var _ TierMask = TierWorking | TierEpisodic | TierSemantic | AllTiers
	if AllTiers != (TierWorking | TierEpisodic | TierSemantic) {
		t.Errorf("AllTiers != Working|Episodic|Semantic — bitmask drift")
	}

	// RecallOptions documented fields.
	_ = RecallOptions{
		TopK:              10,
		Tiers:             AllTiers,
		Budgets:           map[coremem.Kind]int{coremem.KindWorking: 2},
		IncludeProvenance: true,
	}

	// UnifiedRecall documented fields.
	_ = UnifiedRecall{
		Results:      []coremem.SearchResult{},
		PerTier:      map[coremem.Kind]TierStats{coremem.KindWorking: {Considered: 0, Returned: 0}},
		TotalDropped: 0,
	}

	// Constructor signature.
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: newCoreWorking(t)}})
	eng, err := NewRecallEngine(mgr)
	if err != nil {
		t.Fatalf("NewRecallEngine: %v", err)
	}
	if eng == nil {
		t.Fatal("NewRecallEngine returned nil")
	}

	// nil manager surfaces the typed sentinel.
	if _, err := NewRecallEngine(nil); !errors.Is(err, ErrRecallEngineManagerRequired) {
		t.Errorf("NewRecallEngine(nil) err = %v, want ErrRecallEngineManagerRequired", err)
	}

	// Recall callable from a smoke test.
	if _, err := eng.Recall(context.Background(), "anything", RecallOptions{}); err != nil {
		t.Errorf("Recall on empty: %v", err)
	}
}
