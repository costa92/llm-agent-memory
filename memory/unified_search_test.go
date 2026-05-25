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

func TestUnifiedSearcher_SortsByScoreDescending(t *testing.T) {
	mgr := newCoreManager(t)
	u, err := NewUnifiedSearcher(mgr)
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}

	ctx := context.Background()
	// Two clearly-distinguishable contents so any tier produces a
	// score difference. We don't assert exact scores — just that the
	// returned slice is monotonically non-increasing in Score.
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "go modules", Importance: 0.5}); err != nil {
		t.Fatalf("Add 1: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "unrelated cooking recipe", Importance: 0.5}); err != nil {
		t.Fatalf("Add 2: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindSemantic, coremem.MemoryItem{Content: "go modules guide", Tags: []string{"go"}, Importance: 0.5}); err != nil {
		t.Fatalf("Add 3: %v", err)
	}

	results, err := u.SearchUnified(ctx, "go modules", 10)
	if err != nil {
		t.Fatalf("SearchUnified: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Errorf("results not sorted desc at i=%d: %v < %v", i, results[i-1].Score, results[i].Score)
		}
	}
}

func TestUnifiedSearcher_HonorsTopK(t *testing.T) {
	mgr := newCoreManager(t)
	u, err := NewUnifiedSearcher(mgr)
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}

	ctx := context.Background()
	// Seed 5 distinct items across tiers, all matching the query.
	contents := []struct {
		kind    coremem.Kind
		content string
	}{
		{coremem.KindWorking, "go alpha"},
		{coremem.KindWorking, "go bravo"},
		{coremem.KindEpisodic, "go charlie"},
		{coremem.KindEpisodic, "go delta"},
		{coremem.KindSemantic, "go echo"},
	}
	for _, c := range contents {
		if _, err := mgr.Add(ctx, c.kind, coremem.MemoryItem{Content: c.content, Importance: 0.5}); err != nil {
			t.Fatalf("Add %s/%s: %v", c.kind, c.content, err)
		}
	}

	results, err := u.SearchUnified(ctx, "go", 3)
	if err != nil {
		t.Fatalf("SearchUnified: %v", err)
	}
	if got := len(results); got > 3 {
		t.Errorf("len(results) = %d, want ≤ 3", got)
	}
}

func TestUnifiedSearcher_DoesNotAlterCoreSearchAll(t *testing.T) {
	mgr := newCoreManager(t)
	u, err := NewUnifiedSearcher(mgr)
	if err != nil {
		t.Fatalf("NewUnifiedSearcher: %v", err)
	}

	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "alpha", Importance: 0.5}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// First run unified — must not mutate state that SearchAll sees.
	if _, err := u.SearchUnified(ctx, "alpha", 5); err != nil {
		t.Fatalf("SearchUnified: %v", err)
	}

	out, err := mgr.SearchAll(ctx, "alpha", 5)
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if len(out[coremem.KindEpisodic]) == 0 {
		t.Errorf("SearchAll lost the episodic result post-SearchUnified")
	}
}
