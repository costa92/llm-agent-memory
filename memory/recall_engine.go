// Package memory — recall_engine.go is the Phase D-2 implementation of
// the unified recall facade. RecallEngine.Recall is the v1.0.0 public
// recall surface; tier-awareness (working / episodic / semantic
// fan-out) becomes an internal implementation detail.
//
// Composition: RecallEngine wraps a *Manager (the D-1 surface), not
// a *coremem.Manager — the sibling Manager IS the v1 interface
// dispatcher, and reusing it keeps the tier-routing logic single-
// sourced. Callers wired to a legacy *coremem.Manager bridge through
// memory/compat.NewManagerFromCore.
//
// Algorithm: see (*RecallEngine).Recall godoc.
package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	coremem "github.com/costa92/llm-agent/memory"
)

// TierMask selects which tiers participate in a Recall. The zero value
// (0) is treated as AllTiers — every active tier on the wrapped
// Manager participates.
type TierMask uint8

// TierMask constants for the three bundled kinds.
const (
	TierWorking  TierMask = 1 << 0
	TierEpisodic TierMask = 1 << 1
	TierSemantic TierMask = 1 << 2
	AllTiers     TierMask = TierWorking | TierEpisodic | TierSemantic
)

// RecallOptions configures one Recall call. Frozen at v1.0.0 — new
// fields may be appended (non-breaking); existing fields are never
// renamed or removed in any v1.x release.
type RecallOptions struct {
	// TopK is the global cap on the merged result slice. <=0 → 10.
	TopK int

	// Tiers is a bitmask of participating tiers. The zero value is
	// treated as AllTiers.
	Tiers TierMask

	// Budgets is an OPTIONAL per-tier upper bound on candidates pulled
	// from each tier before merge. Nil = each participating tier
	// returns TopK candidates (matches the legacy UnifiedSearcher
	// semantics). A tier present with a value <=0 is treated as
	// "use TopK".
	Budgets map[coremem.Kind]int

	// IncludeProvenance, when true, populates UnifiedRecall.PerTier
	// with per-tier Considered/Returned counts. Default false to keep
	// the fast path allocation-minimal.
	IncludeProvenance bool
}

// TierStats is one row of UnifiedRecall.PerTier.
type TierStats struct {
	Considered int // candidates returned by this tier before merge
	Returned   int // count of Results that originated from this tier
}

// UnifiedRecall is the single recall result type. Frozen at v1.0.0.
type UnifiedRecall struct {
	Results      []coremem.SearchResult
	PerTier      map[coremem.Kind]TierStats
	TotalDropped int
}

// RecallEngine wraps a *Manager and exposes Recall(ctx, query, opts).
type RecallEngine struct {
	mgr *Manager
	cfg *config
}

// ErrRecallEngineManagerRequired is returned by NewRecallEngine when
// mgr is nil.
var ErrRecallEngineManagerRequired = errors.New("memory: recall engine requires manager")

// NewRecallEngine constructs a RecallEngine. Options use the shared
// Option type (WithObserver, etc.).
func NewRecallEngine(mgr *Manager, opts ...Option) (*RecallEngine, error) {
	if mgr == nil {
		return nil, ErrRecallEngineManagerRequired
	}
	return &RecallEngine{mgr: mgr, cfg: newConfig(opts)}, nil
}

// observer exposes the configured observer for in-package call sites.
func (r *RecallEngine) observer() Observer { return r.cfg.observer }

// recallKindResult is the per-tier work item exchanged through the
// buffered channel during fan-out.
type recallKindResult struct {
	kind    coremem.Kind
	results []coremem.SearchResult
	err     error
}

// Recall fans the query out to every participating tier, merges +
// dedupes + sorts + truncates per opts, and returns a UnifiedRecall.
//
// Algorithm:
//  1. Normalize opts (Tiers=0 → AllTiers; TopK<=0 → 10).
//  2. Pick participating tiers: AND opts.Tiers with the set of active
//     tiers on r.mgr.
//  3. For each participating tier, compute per-tier budget:
//     opts.Budgets[kind] if positive, else opts.TopK.
//  4. Fan out one goroutine per tier; each calls r.mgr.Search(ctx,
//     kind, query, perTierBudget).
//  5. Merge per-kind results into a single slice, dedupe by
//     (Item.ID, Item.Content) keeping the highest-scoring entry,
//     remember the tier of origin per surviving entry.
//  6. Sort by Score desc; tie-break on Item.ID asc for determinism.
//  7. Compute TotalDropped = totalCandidates - len(dedupedSlice).
//  8. Truncate to opts.TopK.
//  9. Populate PerTier.Returned (if requested) by walking the
//     truncated slice and counting per-kind.
//
// Emits EventSearchTotal once and EventSearchHits once. Errors from
// any tier short-circuit and surface wrapped with the offending kind.
func (r *RecallEngine) Recall(ctx context.Context, query string, opts RecallOptions) (UnifiedRecall, error) {
	if opts.Tiers == 0 {
		opts.Tiers = AllTiers
	}
	if opts.TopK <= 0 {
		opts.TopK = 10
	}
	emit(r.observer(), EventSearchTotal, map[string]any{"query_len": len(query)})

	participating := r.participating(opts.Tiers)
	if len(participating) == 0 {
		return UnifiedRecall{Results: []coremem.SearchResult{}, PerTier: maybePerTier(opts)}, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan recallKindResult, len(participating))
	var wg sync.WaitGroup
	for _, kind := range participating {
		budget := opts.TopK
		if b, ok := opts.Budgets[kind]; ok && b > 0 {
			budget = b
		}
		wg.Add(1)
		go func(k coremem.Kind, lim int) {
			defer wg.Done()
			res, err := r.mgr.Search(ctx, k, query, lim)
			ch <- recallKindResult{kind: k, results: res, err: err}
		}(kind, budget)
	}
	wg.Wait()
	close(ch)

	perKind := make(map[coremem.Kind][]coremem.SearchResult, len(participating))
	for got := range ch {
		if errors.Is(got.err, coremem.ErrKindDisabled) || errors.Is(got.err, ErrTierDisabled) {
			continue
		}
		if got.err != nil {
			return UnifiedRecall{}, fmt.Errorf("memory: recall %s: %w", got.kind, got.err)
		}
		perKind[got.kind] = got.results
	}

	// Merge + dedupe + tier-of-origin tracking.
	type dedupeKey struct {
		id      string
		content string
	}
	type dedupeVal struct {
		result coremem.SearchResult
		kind   coremem.Kind
	}
	best := make(map[dedupeKey]dedupeVal)
	considered := make(map[coremem.Kind]int, len(perKind))
	totalCandidates := 0
	// Iterate in the canonical tier order so first-write deterministic.
	for _, kind := range []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic, coremem.KindSemantic} {
		results := perKind[kind]
		considered[kind] = len(results)
		totalCandidates += len(results)
		for _, sr := range results {
			k := dedupeKey{id: sr.Item.ID, content: sr.Item.Content}
			prev, ok := best[k]
			if !ok || sr.Score > prev.result.Score {
				best[k] = dedupeVal{result: sr, kind: kind}
			}
		}
	}

	// Materialize merged slice; sort by score desc, ID asc tie-break.
	merged := make([]coremem.SearchResult, 0, len(best))
	origin := make(map[dedupeKey]coremem.Kind, len(best))
	for k, v := range best {
		merged = append(merged, v.result)
		origin[k] = v.kind
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Score != merged[j].Score {
			return merged[i].Score > merged[j].Score
		}
		return merged[i].Item.ID < merged[j].Item.ID
	})

	totalDropped := totalCandidates - len(merged)
	if opts.TopK > 0 && len(merged) > opts.TopK {
		totalDropped += len(merged) - opts.TopK
		merged = merged[:opts.TopK]
	}

	out := UnifiedRecall{Results: merged, TotalDropped: totalDropped}
	if opts.IncludeProvenance {
		perTier := make(map[coremem.Kind]TierStats, len(participating))
		for _, kind := range participating {
			perTier[kind] = TierStats{Considered: considered[kind], Returned: 0}
		}
		for _, sr := range merged {
			k := dedupeKey{id: sr.Item.ID, content: sr.Item.Content}
			if kind, ok := origin[k]; ok {
				st := perTier[kind]
				st.Returned++
				perTier[kind] = st
			}
		}
		out.PerTier = perTier
	}
	emit(r.observer(), EventSearchHits, map[string]any{"n": len(out.Results)})
	return out, nil
}

// participating returns the canonical-order list of kinds that are
// both selected by the mask and active on the wrapped Manager.
func (r *RecallEngine) participating(mask TierMask) []coremem.Kind {
	pick := func(k coremem.Kind, bit TierMask) (coremem.Kind, bool) {
		if mask&bit == 0 {
			return "", false
		}
		return k, r.mgr.HasKind(k)
	}
	out := make([]coremem.Kind, 0, 3)
	if k, ok := pick(coremem.KindWorking, TierWorking); ok {
		out = append(out, k)
	}
	if k, ok := pick(coremem.KindEpisodic, TierEpisodic); ok {
		out = append(out, k)
	}
	if k, ok := pick(coremem.KindSemantic, TierSemantic); ok {
		out = append(out, k)
	}
	return out
}

// maybePerTier returns a non-nil empty PerTier map when provenance is
// requested even on the zero-tier short-circuit, so callers can rely
// on a non-nil map shape.
func maybePerTier(opts RecallOptions) map[coremem.Kind]TierStats {
	if !opts.IncludeProvenance {
		return nil
	}
	return map[coremem.Kind]TierStats{}
}
