package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"

	coremem "github.com/costa92/llm-agent/memory"
)

// UnifiedSearcher wraps a *coremem.Manager and exposes SearchUnified,
// a single cross-tier recall surface that merges, dedupes, sorts, and
// caps results across Working / Episodic / Semantic. It complements
// (does not replace) coremem.Manager.SearchAll, which keeps the per-
// kind buckets useful for debugging.
//
// Score semantics: each tier returns scores in its own scale. v0.1
// performs a HEURISTIC merge — scores are kept as-is and sorted
// descending. A future task may introduce per-tier normalization
// (this is captured in the roadmap as an M1 open question).
type UnifiedSearcher struct {
	mgr *coremem.Manager
	cfg *config
}

// ErrUnifiedManagerRequired is returned by NewUnifiedSearcher when the
// inner *coremem.Manager is nil.
var ErrUnifiedManagerRequired = errors.New("memory: unified searcher requires manager")

// NewUnifiedSearcher wraps an existing *coremem.Manager. Returns
// ErrUnifiedManagerRequired if inner is nil.
func NewUnifiedSearcher(inner *coremem.Manager, opts ...Option) (*UnifiedSearcher, error) {
	if inner == nil {
		return nil, ErrUnifiedManagerRequired
	}
	return &UnifiedSearcher{mgr: inner, cfg: newConfig(opts)}, nil
}

// observer exposes the configured observer for in-package callers and
// tests. Package-private — callers should not depend on the accessor.
func (u *UnifiedSearcher) observer() Observer { return u.cfg.observer }

// SearchUnified fans out the query to every active memory kind via
// coremem.Manager.SearchAll, merges the per-kind result lists into a
// single slice, dedupes by (Item.ID, Item.Content) keeping the
// highest-scoring entry, sorts by Score descending, and truncates to
// topK (when topK > 0; topK ≤ 0 returns the full merged set).
//
// The per-kind topK passed to SearchAll is the same topK argument the
// caller provides, so each tier returns its top-topK candidates before
// merge. This means SearchUnified inspects at most 3 × topK candidates.
func (u *UnifiedSearcher) SearchUnified(ctx context.Context, query string, topK int) ([]coremem.SearchResult, error) {
	emit(u.observer(), EventSearchTotal, map[string]any{"query_len": len(query)})
	perKind, err := u.mgr.SearchAll(ctx, query, topK)
	if err != nil {
		return nil, fmt.Errorf("memory: unified search fan-out: %w", err)
	}
	// Merge.
	merged := make([]coremem.SearchResult, 0)
	for _, kind := range []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic, coremem.KindSemantic} {
		merged = append(merged, perKind[kind]...)
	}
	// Dedupe by (ID, Content). Keep the highest-scoring entry per key.
	type key struct {
		id      string
		content string
	}
	best := make(map[key]coremem.SearchResult, len(merged))
	for _, r := range merged {
		k := key{id: r.Item.ID, content: r.Item.Content}
		prev, ok := best[k]
		if !ok || r.Score > prev.Score {
			best[k] = r
		}
	}
	out := make([]coremem.SearchResult, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	// Sort by Score desc; break ties on ID asc for determinism.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Item.ID < out[j].Item.ID
	})
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	emit(u.observer(), EventSearchHits, map[string]any{"n": len(out)})
	return out, nil
}
