package memory

import (
	"context"
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
