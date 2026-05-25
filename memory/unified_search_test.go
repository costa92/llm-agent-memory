package memory

import (
	"context"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

func TestUnifiedSearcher_FansOutToAllTiers(t *testing.T) {
	mgr := newCoreManager(t)
	u, err := NewUnifiedSearcher(mgr)
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}

	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "go modules", Importance: 0.5}); err != nil {
		t.Fatalf("working Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "go modules history", Importance: 0.5}); err != nil {
		t.Fatalf("episodic Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindSemantic, coremem.MemoryItem{Content: "go modules guide", Tags: []string{"go"}, Importance: 0.5}); err != nil {
		t.Fatalf("semantic Add: %v", err)
	}

	results, err := u.SearchUnified(ctx, "go modules", 10)
	if err != nil {
		t.Fatalf("SearchUnified: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("got 0 results, want ≥ 1 from each tier merged")
	}

	// Collect the set of contents we saw — must include at least one
	// item from each of the three tiers (proven by content marker).
	seen := make(map[string]bool, len(results))
	for _, r := range results {
		seen[r.Item.Content] = true
	}
	for _, want := range []string{"go modules", "go modules history", "go modules guide"} {
		if !seen[want] {
			t.Errorf("SearchUnified missing %q (results: %v)", want, contentsOf(results))
		}
	}
}

// contentsOf is a small test helper to print the content slice on
// failure. Kept inside _test.go so it doesn't leak into the public API.
func contentsOf(rs []coremem.SearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Item.Content
	}
	return out
}

func TestUnifiedSearcher_DedupesByIDAndContent(t *testing.T) {
	mgr := newCoreManager(t)
	u, err := NewUnifiedSearcher(mgr)
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}

	ctx := context.Background()
	// Same Content + same ID across Working and Episodic — should
	// collapse to a single result.
	const sharedID = "fixed-id-001"
	const sharedContent = "duplicated note"
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{ID: sharedID, Content: sharedContent, Importance: 0.5}); err != nil {
		t.Fatalf("working Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{ID: sharedID, Content: sharedContent, Importance: 0.5}); err != nil {
		t.Fatalf("episodic Add: %v", err)
	}

	results, err := u.SearchUnified(ctx, "duplicated note", 10)
	if err != nil {
		t.Fatalf("SearchUnified: %v", err)
	}

	dupCount := 0
	for _, r := range results {
		if r.Item.ID == sharedID && r.Item.Content == sharedContent {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Errorf("dup count = %d, want 1 (got results: %v)", dupCount, contentsOf(results))
	}
}
