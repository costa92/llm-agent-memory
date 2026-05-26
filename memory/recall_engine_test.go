package memory

import (
	"context"
	"errors"
	"math"
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

// TestRecallEngine_Recall_ParityWithUnifiedSearcher locks the v0.x
// SearchUnified surface as the floor for v1 RecallEngine.Recall. Same
// inputs, same dedupe rules, same sort order — same result slice. If
// a future RecallEngine refactor changes the merged ordering or the
// dedupe key, this test surfaces the regression immediately.
func TestRecallEngine_Recall_ParityWithUnifiedSearcher(t *testing.T) {
	ctx := context.Background()
	w, e, s := newCoreWorking(t), newCoreEpisodic(t), newCoreSemantic(t)
	coreMgr, _ := coremem.NewManager(coremem.ManagerOptions{Working: w, Episodic: e, Semantic: s})

	// Seed each tier with one distinct + one shared item.
	for _, kind := range []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic, coremem.KindSemantic} {
		if _, err := coreMgr.Add(ctx, kind, coremem.MemoryItem{Content: "shared-across-tiers"}); err != nil {
			t.Fatalf("Add shared %s: %v", kind, err)
		}
	}
	if _, err := coreMgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "w-only"}); err != nil {
		t.Fatalf("Add w-only: %v", err)
	}

	// V0 path: UnifiedSearcher.
	uni, err := NewUnifiedSearcher(coreMgr)
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}
	uniRes, err := uni.SearchUnified(ctx, "shared", 10)
	if err != nil {
		t.Fatalf("SearchUnified: %v", err)
	}

	// V1 path: RecallEngine via sibling Manager wrapping coreMgr.
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
		Semantic: TierOptions{Memory: s},
	})
	eng, err := NewRecallEngine(mgr)
	if err != nil {
		t.Fatalf("NewRecallEngine: %v", err)
	}
	v1, err := eng.Recall(ctx, "shared", RecallOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	// Length parity.
	if len(v1.Results) != len(uniRes) {
		t.Fatalf("len(Results) = %d, want %d (parity with UnifiedSearcher)", len(v1.Results), len(uniRes))
	}
	// Item-by-item parity (Content + Score). We do NOT compare ID
	// because the merge in V1 picks the higher-scoring tier's ID, while
	// V0's map iteration is unordered for ties — both are correct under
	// the dedupe rule, and Content is the stable identity here.
	for i := range v1.Results {
		if v1.Results[i].Item.Content != uniRes[i].Item.Content {
			t.Errorf("Results[%d].Content = %q, want %q", i, v1.Results[i].Item.Content, uniRes[i].Item.Content)
		}
		// Score parity tolerates ~1e-6 FP slop: WorkingMemory.score()
		// calls time.Now() per Search invocation, so V0 and V1 paths see
		// slightly different recency factors. Same lesson as M2 Task 11.
		if math.Abs(v1.Results[i].Score-uniRes[i].Score) > 1e-6 {
			t.Errorf("Results[%d].Score = %v, want %v", i, v1.Results[i].Score, uniRes[i].Score)
		}
	}
}
