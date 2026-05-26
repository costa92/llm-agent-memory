package memory

import (
	"context"
	"math"
	"sort"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

func TestParallelSearcher_SearchAllParallel_MatchesCoreSearchAll(t *testing.T) {
	mgr := newCoreManager(t)
	ps, err := NewParallelSearcher(mgr)
	if err != nil {
		t.Fatalf("NewParallelSearcher: %v", err)
	}

	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "alpha", Importance: 0.5}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "alpha", Importance: 0.5}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindSemantic, coremem.MemoryItem{Content: "alpha-guide", Tags: []string{"a"}, Importance: 0.5}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	want, err := mgr.SearchAll(ctx, "alpha", 5)
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	got, err := ps.SearchAllParallel(ctx, "alpha", 5)
	if err != nil {
		t.Fatalf("SearchAllParallel: %v", err)
	}

	if !sameKindKeys(want, got) {
		t.Fatalf("kind keys differ: want %v, got %v", kindsOf(want), kindsOf(got))
	}
	for kind := range want {
		w := normalizeResults(want[kind])
		g := normalizeResults(got[kind])
		if len(w) != len(g) {
			t.Errorf("kind %v: len(want)=%d, len(got)=%d", kind, len(w), len(g))
			continue
		}
		for i := range w {
			// Score is time-dependent in WorkingMemory (score() calls
			// time.Now() per invocation), so DeepEqual is too strict —
			// serial SearchAll and parallel SearchAllParallel are called
			// microseconds apart and the time-decay factor differs in
			// the ~10th decimal place. Allow 1e-6 slop (well above the
			// ~5e-10 drift observed; tight enough to catch real bugs).
			if w[i].Item.ID != g[i].Item.ID {
				t.Errorf("kind %v idx %d: ID want=%q got=%q", kind, i, w[i].Item.ID, g[i].Item.ID)
			}
			if w[i].Item.Content != g[i].Item.Content {
				t.Errorf("kind %v idx %d: Content want=%q got=%q", kind, i, w[i].Item.Content, g[i].Item.Content)
			}
			if math.Abs(w[i].Score-g[i].Score) > 1e-6 {
				t.Errorf("kind %v idx %d: Score want=%v got=%v (delta=%v exceeds 1e-6)",
					kind, i, w[i].Score, g[i].Score, math.Abs(w[i].Score-g[i].Score))
			}
		}
	}
}

// sameKindKeys returns true if a and b have the same set of map keys.
func sameKindKeys(a, b map[coremem.Kind][]coremem.SearchResult) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func kindsOf(m map[coremem.Kind][]coremem.SearchResult) []coremem.Kind {
	out := make([]coremem.Kind, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}

// normalizeResults sorts a per-kind result slice by (Score desc, ID asc)
// so reflect.DeepEqual is independent of any deliberate parallel
// reordering.
func normalizeResults(rs []coremem.SearchResult) []coremem.SearchResult {
	out := make([]coremem.SearchResult, len(rs))
	copy(out, rs)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Item.ID < out[j].Item.ID
	})
	return out
}
