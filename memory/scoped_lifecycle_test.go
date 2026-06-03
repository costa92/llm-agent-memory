package memory

import (
	"context"
	"fmt"
	"testing"
)

func TestScopedLifecycle_ConsolidateScoped_OnlyPromotesMatchingScope(t *testing.T) {
	sm := newScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := Scope{User: "alice", Project: "p1"}
	scopeB := Scope{User: "bob", Project: "p1"}

	ctxA := WithScope(context.Background(), scopeA)
	ctxB := WithScope(context.Background(), scopeB)

	// Alice writes a working item with importance high enough to promote.
	if _, err := sm.Add(ctxA, KindWorking, MemoryItem{
		Content: "alice note", Importance: 0.9,
	}); err != nil {
		t.Fatalf("alice Add: %v", err)
	}
	// Bob writes one too.
	if _, err := sm.Add(ctxB, KindWorking, MemoryItem{
		Content: "bob note", Importance: 0.9,
	}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	// Alice runs ConsolidateScoped. Only her item should be promoted.
	n, err := slm.ConsolidateScoped(ctxA, ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}
	if n != 1 {
		t.Fatalf("promoted = %d, want 1", n)
	}

	// Inspect the episodic tier via the inner *Manager: exactly one item,
	// and it carries Alice's scope.
	pages, err := sm.Inner().ListAll(context.Background(), ListFilter{}, 100, nil)
	if err != nil {
		t.Fatalf("inner ListAll: %v", err)
	}
	epi := pages[KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("episodic count = %d, want 1", len(epi))
	}
	if epi[0].Content != "alice note" {
		t.Errorf("promoted content = %q, want %q", epi[0].Content, "alice note")
	}
}

func TestScopedLifecycle_ConsolidateScoped_DoesNotPromoteOtherScope(t *testing.T) {
	sm := newScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := Scope{User: "alice"}
	scopeB := Scope{User: "bob"}

	ctxA := WithScope(context.Background(), scopeA)
	ctxB := WithScope(context.Background(), scopeB)

	if _, err := sm.Add(ctxA, KindWorking, MemoryItem{Content: "a", Importance: 0.9}); err != nil {
		t.Fatalf("alice Add: %v", err)
	}
	if _, err := sm.Add(ctxB, KindWorking, MemoryItem{Content: "b", Importance: 0.9}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	// Bob runs ConsolidateScoped. Alice's item must NOT be promoted.
	n, err := slm.ConsolidateScoped(ctxB, ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}
	if n != 1 {
		t.Fatalf("promoted = %d, want 1 (only bob)", n)
	}

	pages, _ := sm.Inner().ListAll(context.Background(), ListFilter{}, 100, nil)
	epi := pages[KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("episodic count = %d, want 1", len(epi))
	}
	if epi[0].Content != "b" {
		t.Errorf("episodic content = %q, want %q (alice leak!)", epi[0].Content, "b")
	}
}

func TestScopedLifecycle_ForgetScoped_OnlyDeletesMatchingScope(t *testing.T) {
	sm := newScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := Scope{User: "alice"}
	scopeB := Scope{User: "bob"}

	ctxA := WithScope(context.Background(), scopeA)
	ctxB := WithScope(context.Background(), scopeB)

	// Both users add a low-importance episodic item.
	if _, err := sm.Add(ctxA, KindEpisodic, MemoryItem{Content: "a", Importance: 0.1}); err != nil {
		t.Fatalf("alice Add: %v", err)
	}
	if _, err := sm.Add(ctxB, KindEpisodic, MemoryItem{Content: "b", Importance: 0.1}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	// Alice forgets by importance threshold 0.5. Bob's item must survive.
	n, err := slm.ForgetScoped(ctxA, KindEpisodic, ForgetOptions{
		Strategy:  ForgetByImportance,
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("ForgetScoped: %v", err)
	}
	if n != 1 {
		t.Fatalf("forgotten = %d, want 1", n)
	}

	pages, _ := sm.Inner().ListAll(context.Background(), ListFilter{}, 100, nil)
	epi := pages[KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("survivors = %d, want 1 (bob)", len(epi))
	}
	if epi[0].Content != "b" {
		t.Errorf("surviving content = %q, want %q", epi[0].Content, "b")
	}
}

func TestScopedLifecycle_StatsScoped_CountsOnlyMatchingScope(t *testing.T) {
	sm := newScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := Scope{User: "alice"}
	scopeB := Scope{User: "bob"}

	ctxA := WithScope(context.Background(), scopeA)
	ctxB := WithScope(context.Background(), scopeB)

	if _, err := sm.Add(ctxA, KindWorking, MemoryItem{Content: "a1", Importance: 0.5}); err != nil {
		t.Fatalf("alice Add 1: %v", err)
	}
	if _, err := sm.Add(ctxA, KindWorking, MemoryItem{Content: "a2", Importance: 0.5}); err != nil {
		t.Fatalf("alice Add 2: %v", err)
	}
	if _, err := sm.Add(ctxB, KindWorking, MemoryItem{Content: "b1", Importance: 0.5}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	statsA, err := slm.StatsScoped(ctxA)
	if err != nil {
		t.Fatalf("StatsScoped(alice): %v", err)
	}
	if got := statsA[KindWorking].Count; got != 2 {
		t.Errorf("alice working Count = %d, want 2", got)
	}

	statsB, err := slm.StatsScoped(ctxB)
	if err != nil {
		t.Fatalf("StatsScoped(bob): %v", err)
	}
	if got := statsB[KindWorking].Count; got != 1 {
		t.Errorf("bob working Count = %d, want 1", got)
	}
}

func TestScopedLifecycle_ConsolidateScoped_PagesThroughLargeScope(t *testing.T) {
	// Verify pagination loop: the M1 impl calls ListAll with no cursor,
	// silently capping at one underlying page (coremem v0.7.0 defaults
	// to 50). With 180 working items above threshold the M1 impl drops
	// the tail; the cursor-aware Task 2 fix must promote all 180.
	// Working capacity in newCoreWorking is 16 — too small for 180.
	// Build a manager directly with a wide-capacity working memory.
	w, e, s := newWorkingWithCapacity(t, 256), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w},
		Episodic: TierOptions{Memory: e, Lister: e, Exporter: e, Importer: e},
		Semantic: TierOptions{Memory: s, Lister: s, Exporter: s, Importer: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wideSM, err := NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}

	slm, err := NewScopedLifecycleManager(wideSM)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	ctx := WithScope(context.Background(), Scope{User: "page-user"})
	const total = 180
	for i := 0; i < total; i++ {
		if _, err := wideSM.Add(ctx, KindWorking, MemoryItem{
			Content:    fmt.Sprintf("item-%03d", i),
			Importance: 0.9,
		}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}

	n, err := slm.ConsolidateScoped(ctx, ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}
	if n != total {
		t.Fatalf("ConsolidateScoped promoted = %d, want %d (pagination dropped %d items)",
			n, total, total-n)
	}
}

func TestScopedLifecycle_ForgetScoped_PagesThroughLargeScope(t *testing.T) {
	w, e, s := newWorkingWithCapacity(t, 16), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w},
		Episodic: TierOptions{Memory: e, Lister: e, Exporter: e, Importer: e},
		Semantic: TierOptions{Memory: s, Lister: s, Exporter: s, Importer: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wideSM, err := NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}
	slm, err := NewScopedLifecycleManager(wideSM)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	ctx := WithScope(context.Background(), Scope{User: "page-forget"})
	const total = 170
	for i := 0; i < total; i++ {
		if _, err := wideSM.Add(ctx, KindEpisodic, MemoryItem{
			Content:    fmt.Sprintf("forgettable-%03d", i),
			Importance: 0.1,
		}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}

	n, err := slm.ForgetScoped(ctx, KindEpisodic, ForgetOptions{
		Strategy:  ForgetByImportance,
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("ForgetScoped: %v", err)
	}
	if n != total {
		t.Fatalf("ForgetScoped removed = %d, want %d (pagination dropped %d)", n, total, total-n)
	}
}

func TestScopedLifecycle_StatsScoped_CountsAllPages(t *testing.T) {
	w, e, s := newWorkingWithCapacity(t, 256), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w},
		Episodic: TierOptions{Memory: e, Lister: e, Exporter: e, Importer: e},
		Semantic: TierOptions{Memory: s, Lister: s, Exporter: s, Importer: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wideSM, err := NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}
	slm, err := NewScopedLifecycleManager(wideSM)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	ctx := WithScope(context.Background(), Scope{User: "page-stats"})
	const total = 160
	for i := 0; i < total; i++ {
		if _, err := wideSM.Add(ctx, KindWorking, MemoryItem{
			Content:    fmt.Sprintf("countable-%03d", i),
			Importance: 0.5,
		}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}

	stats, err := slm.StatsScoped(ctx)
	if err != nil {
		t.Fatalf("StatsScoped: %v", err)
	}
	if got := stats[KindWorking].Count; got != total {
		t.Errorf("StatsScoped Count = %d, want %d", got, total)
	}
}

func TestScopedLifecycle_StatsScoped_IncludesActiveButEmptyKinds(t *testing.T) {
	// Regression: prior to the listAllScoped guard fix, an active kind
	// with zero items in scope was silently dropped from the result map.
	// M1 returned a populated Capacity for such kinds; assert that
	// contract is restored.
	w, e, s := newWorkingWithCapacity(t, 256), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w},
		Episodic: TierOptions{Memory: e, Lister: e, Exporter: e, Importer: e},
		Semantic: TierOptions{Memory: s, Lister: s, Exporter: s, Importer: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wideSM, err := NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}
	slm, err := NewScopedLifecycleManager(wideSM)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	ctx := WithScope(context.Background(), Scope{User: "empty-kinds"})
	// Add one item only to Working — leave Episodic and Semantic empty.
	if _, err := wideSM.Add(ctx, KindWorking, MemoryItem{
		Content: "solo", Importance: 0.5,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	stats, err := slm.StatsScoped(ctx)
	if err != nil {
		t.Fatalf("StatsScoped: %v", err)
	}

	// All 3 active kinds must appear in the result map.
	for _, k := range []Kind{KindWorking, KindEpisodic, KindSemantic} {
		if _, ok := stats[k]; !ok {
			t.Errorf("stats[%v] missing — active-but-empty kinds must be reported", k)
		}
	}
	// KindWorking: Count=1, Capacity > 0 (from inner StatsAll)
	if stats[KindWorking].Count != 1 {
		t.Errorf("KindWorking Count = %d, want 1", stats[KindWorking].Count)
	}
	// Empty kinds: Count=0, Capacity > 0 (the regression assertion)
	if stats[KindEpisodic].Count != 0 {
		t.Errorf("KindEpisodic Count = %d, want 0", stats[KindEpisodic].Count)
	}
}

func TestScopedLifecycle_ConsolidateScoped_EmitsConsolidatedTotalEvent(t *testing.T) {
	rec := &recordingObserver{}
	slm, err := NewScopedLifecycleManager(newScopedManager(t), WithObserver(rec))
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}
	ctx := WithScope(context.Background(), Scope{User: "obs-user"})
	if _, err := slm.sm.Add(ctx, KindWorking, MemoryItem{Content: "a", Importance: 0.9}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slm.ConsolidateScoped(ctx, ConsolidateOptions{Threshold: 0.7}); err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}

	got := rec.snapshot()
	var found *Event
	for i := range got {
		if got[i].Name == EventConsolidatedTotal {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no %q event emitted (events: %v)", EventConsolidatedTotal, got)
	}
	if n, _ := found.Attrs["n"].(int); n != 1 {
		t.Errorf("event Attrs[\"n\"] = %v, want 1", found.Attrs["n"])
	}
}

func TestScopedLifecycle_ForgetScoped_EmitsForgottenTotalEvent(t *testing.T) {
	rec := &recordingObserver{}
	slm, err := NewScopedLifecycleManager(newScopedManager(t), WithObserver(rec))
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}
	ctx := WithScope(context.Background(), Scope{User: "forget-obs"})
	if _, err := slm.sm.Add(ctx, KindEpisodic, MemoryItem{Content: "a", Importance: 0.1}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slm.ForgetScoped(ctx, KindEpisodic, ForgetOptions{
		Strategy:  ForgetByImportance,
		Threshold: 0.5,
	}); err != nil {
		t.Fatalf("ForgetScoped: %v", err)
	}

	got := rec.snapshot()
	var found *Event
	for i := range got {
		if got[i].Name == EventForgottenTotal {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no %q event emitted", EventForgottenTotal)
	}
	if n, _ := found.Attrs["n"].(int); n != 1 {
		t.Errorf("Attrs[\"n\"] = %v, want 1", found.Attrs["n"])
	}
	if k, _ := found.Attrs["kind"].(Kind); k != KindEpisodic {
		t.Errorf("Attrs[\"kind\"] = %v, want %v", found.Attrs["kind"], KindEpisodic)
	}
}
