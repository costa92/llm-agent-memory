package memory

import (
	"context"
	"fmt"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

func TestScopedLifecycle_ConsolidateScoped_OnlyPromotesMatchingScope(t *testing.T) {
	sm := newCoreScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := coremem.Scope{User: "alice", Project: "p1"}
	scopeB := coremem.Scope{User: "bob", Project: "p1"}

	ctxA := coremem.WithScope(context.Background(), scopeA)
	ctxB := coremem.WithScope(context.Background(), scopeB)

	// Alice writes a working item with importance high enough to promote.
	if _, err := sm.Add(ctxA, coremem.KindWorking, coremem.MemoryItem{
		Content: "alice note", Importance: 0.9,
	}); err != nil {
		t.Fatalf("alice Add: %v", err)
	}
	// Bob writes one too.
	if _, err := sm.Add(ctxB, coremem.KindWorking, coremem.MemoryItem{
		Content: "bob note", Importance: 0.9,
	}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	// Alice runs ConsolidateScoped. Only her item should be promoted.
	n, err := slm.ConsolidateScoped(ctxA, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}
	if n != 1 {
		t.Fatalf("promoted = %d, want 1", n)
	}

	// Inspect the episodic tier via the inner *Manager: exactly one item,
	// and it carries Alice's scope.
	pages, err := sm.Inner().ListAll(context.Background(), coremem.ListFilter{}, 100, nil)
	if err != nil {
		t.Fatalf("inner ListAll: %v", err)
	}
	epi := pages[coremem.KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("episodic count = %d, want 1", len(epi))
	}
	if epi[0].Content != "alice note" {
		t.Errorf("promoted content = %q, want %q", epi[0].Content, "alice note")
	}
}

func TestScopedLifecycle_ConsolidateScoped_DoesNotPromoteOtherScope(t *testing.T) {
	sm := newCoreScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := coremem.Scope{User: "alice"}
	scopeB := coremem.Scope{User: "bob"}

	ctxA := coremem.WithScope(context.Background(), scopeA)
	ctxB := coremem.WithScope(context.Background(), scopeB)

	if _, err := sm.Add(ctxA, coremem.KindWorking, coremem.MemoryItem{Content: "a", Importance: 0.9}); err != nil {
		t.Fatalf("alice Add: %v", err)
	}
	if _, err := sm.Add(ctxB, coremem.KindWorking, coremem.MemoryItem{Content: "b", Importance: 0.9}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	// Bob runs ConsolidateScoped. Alice's item must NOT be promoted.
	n, err := slm.ConsolidateScoped(ctxB, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}
	if n != 1 {
		t.Fatalf("promoted = %d, want 1 (only bob)", n)
	}

	pages, _ := sm.Inner().ListAll(context.Background(), coremem.ListFilter{}, 100, nil)
	epi := pages[coremem.KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("episodic count = %d, want 1", len(epi))
	}
	if epi[0].Content != "b" {
		t.Errorf("episodic content = %q, want %q (alice leak!)", epi[0].Content, "b")
	}
}

func TestScopedLifecycle_ForgetScoped_OnlyDeletesMatchingScope(t *testing.T) {
	sm := newCoreScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := coremem.Scope{User: "alice"}
	scopeB := coremem.Scope{User: "bob"}

	ctxA := coremem.WithScope(context.Background(), scopeA)
	ctxB := coremem.WithScope(context.Background(), scopeB)

	// Both users add a low-importance episodic item.
	if _, err := sm.Add(ctxA, coremem.KindEpisodic, coremem.MemoryItem{Content: "a", Importance: 0.1}); err != nil {
		t.Fatalf("alice Add: %v", err)
	}
	if _, err := sm.Add(ctxB, coremem.KindEpisodic, coremem.MemoryItem{Content: "b", Importance: 0.1}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	// Alice forgets by importance threshold 0.5. Bob's item must survive.
	n, err := slm.ForgetScoped(ctxA, coremem.KindEpisodic, coremem.ForgetOptions{
		Strategy:  coremem.ForgetByImportance,
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("ForgetScoped: %v", err)
	}
	if n != 1 {
		t.Fatalf("forgotten = %d, want 1", n)
	}

	pages, _ := sm.Inner().ListAll(context.Background(), coremem.ListFilter{}, 100, nil)
	epi := pages[coremem.KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("survivors = %d, want 1 (bob)", len(epi))
	}
	if epi[0].Content != "b" {
		t.Errorf("surviving content = %q, want %q", epi[0].Content, "b")
	}
}

func TestScopedLifecycle_StatsScoped_CountsOnlyMatchingScope(t *testing.T) {
	sm := newCoreScopedManager(t)
	slm, err := NewScopedLifecycleManager(sm)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	scopeA := coremem.Scope{User: "alice"}
	scopeB := coremem.Scope{User: "bob"}

	ctxA := coremem.WithScope(context.Background(), scopeA)
	ctxB := coremem.WithScope(context.Background(), scopeB)

	if _, err := sm.Add(ctxA, coremem.KindWorking, coremem.MemoryItem{Content: "a1", Importance: 0.5}); err != nil {
		t.Fatalf("alice Add 1: %v", err)
	}
	if _, err := sm.Add(ctxA, coremem.KindWorking, coremem.MemoryItem{Content: "a2", Importance: 0.5}); err != nil {
		t.Fatalf("alice Add 2: %v", err)
	}
	if _, err := sm.Add(ctxB, coremem.KindWorking, coremem.MemoryItem{Content: "b1", Importance: 0.5}); err != nil {
		t.Fatalf("bob Add: %v", err)
	}

	statsA, err := slm.StatsScoped(ctxA)
	if err != nil {
		t.Fatalf("StatsScoped(alice): %v", err)
	}
	if got := statsA[coremem.KindWorking].Count; got != 2 {
		t.Errorf("alice working Count = %d, want 2", got)
	}

	statsB, err := slm.StatsScoped(ctxB)
	if err != nil {
		t.Fatalf("StatsScoped(bob): %v", err)
	}
	if got := statsB[coremem.KindWorking].Count; got != 1 {
		t.Errorf("bob working Count = %d, want 1", got)
	}
}

func TestScopedLifecycle_ConsolidateScoped_PagesThroughLargeScope(t *testing.T) {
	// Verify pagination loop: the M1 impl called ListAll with no cursor,
	// silently capping at one page. With 180 working items above
	// threshold, the M1 impl would promote at most 100 and silently drop
	// the remaining 80.
	sm := newCoreScopedManager(t)
	// Working capacity in newCoreWorking is 16 — too small for 180.
	// Build a manager directly with a wide-capacity working memory.
	mgr, err := coremem.NewManager(coremem.ManagerOptions{
		Working:  newCoreWorkingWithCapacity(t, 256),
		Episodic: newCoreEpisodic(t),
		Semantic: newCoreSemantic(t),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wideSM, err := coremem.NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}
	_ = sm // pin the helper so unused-var doesn't bite if tests later add it

	slm, err := NewScopedLifecycleManager(wideSM)
	if err != nil {
		t.Fatalf("NewScopedLifecycleManager: %v", err)
	}

	ctx := coremem.WithScope(context.Background(), coremem.Scope{User: "page-user"})
	const total = 180
	for i := 0; i < total; i++ {
		if _, err := wideSM.Add(ctx, coremem.KindWorking, coremem.MemoryItem{
			Content:    fmt.Sprintf("item-%03d", i),
			Importance: 0.9,
		}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}

	n, err := slm.ConsolidateScoped(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("ConsolidateScoped: %v", err)
	}
	if n != total {
		t.Fatalf("ConsolidateScoped promoted = %d, want %d (pagination dropped %d items)",
			n, total, total-n)
	}
}
