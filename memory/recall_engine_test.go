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

func TestRecallEngine_Recall_TierMask_Working_OmitsOtherTiers(t *testing.T) {
	ctx := context.Background()
	w, e, s := newCoreWorking(t), newCoreEpisodic(t), newCoreSemantic(t)
	if _, err := w.Add(ctx, coremem.MemoryItem{Content: "w-only"}); err != nil {
		t.Fatalf("w Add: %v", err)
	}
	if _, err := e.Add(ctx, coremem.MemoryItem{Content: "e-only"}); err != nil {
		t.Fatalf("e Add: %v", err)
	}
	if _, err := s.Add(ctx, coremem.MemoryItem{Content: "s-only"}); err != nil {
		t.Fatalf("s Add: %v", err)
	}
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
		Semantic: TierOptions{Memory: s},
	})
	eng, _ := NewRecallEngine(mgr)
	got, err := eng.Recall(ctx, "only", RecallOptions{TopK: 10, Tiers: TierWorking, IncludeProvenance: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if _, ok := got.PerTier[coremem.KindEpisodic]; ok {
		t.Errorf("PerTier should omit KindEpisodic when Tiers=TierWorking: %+v", got.PerTier)
	}
	if _, ok := got.PerTier[coremem.KindSemantic]; ok {
		t.Errorf("PerTier should omit KindSemantic when Tiers=TierWorking: %+v", got.PerTier)
	}
	for _, r := range got.Results {
		if r.Item.Content == "e-only" || r.Item.Content == "s-only" {
			t.Errorf("Recall returned out-of-tier item %q under TierWorking mask", r.Item.Content)
		}
	}
}

func TestRecallEngine_Recall_PerTierBudget_CapsCandidates(t *testing.T) {
	ctx := context.Background()
	w := newCoreWorking(t)
	for i := 0; i < 5; i++ {
		if _, err := w.Add(ctx, coremem.MemoryItem{Content: "bursty"}); err != nil {
			t.Fatalf("w Add: %v", err)
		}
	}
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	eng, _ := NewRecallEngine(mgr)
	got, err := eng.Recall(ctx, "bursty", RecallOptions{
		TopK:              10,
		Tiers:             TierWorking,
		Budgets:           map[coremem.Kind]int{coremem.KindWorking: 2},
		IncludeProvenance: true,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if got.PerTier[coremem.KindWorking].Considered > 2 {
		t.Errorf("PerTier.Considered = %d, want <= 2 (budget cap)", got.PerTier[coremem.KindWorking].Considered)
	}
}

// TestRecallEngine_OverWithSanitizerWrappedManager_NoCast is the
// joint D-1 + D-2 exit-criterion proof: a WithSanitizer-wrapped Memory
// installs into a sibling Manager (D-1), and that Manager is then
// recall-able via RecallEngine.Recall (D-2). The two breaks compose.
func TestRecallEngine_OverWithSanitizerWrappedManager_NoCast(t *testing.T) {
	ctx := context.Background()
	w := newCoreWorking(t)
	tagger := coremem.SanitizerFunc(func(_ context.Context, _ coremem.Kind, it coremem.MemoryItem) (coremem.MemoryItem, bool, error) {
		it.Tags = append(it.Tags, "via-sanitizer")
		return it, true, nil
	})
	wrapped := coremem.WithSanitizer(w, tagger)
	mgr, err := NewManager(Options{Working: TierOptions{Memory: wrapped}})
	if err != nil {
		t.Fatalf("NewManager(wrapped): %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "alpha"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	eng, err := NewRecallEngine(mgr)
	if err != nil {
		t.Fatalf("NewRecallEngine: %v", err)
	}
	got, err := eng.Recall(ctx, "alpha", RecallOptions{TopK: 5})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(got.Results) == 0 {
		t.Fatal("Recall returned 0 results, want at least 1")
	}
	hasTag := false
	for _, tag := range got.Results[0].Item.Tags {
		if tag == "via-sanitizer" {
			hasTag = true
		}
	}
	if !hasTag {
		t.Errorf("Result[0].Tags = %v, want via-sanitizer tag — sanitizer chain did not run inside RecallEngine path", got.Results[0].Item.Tags)
	}
}
