package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	coremem "github.com/costa92/llm-agent/memory"
)

// ScopedLifecycleManager wraps a *coremem.ScopedManager and adds three
// lifecycle methods that honor the ctx scope (closing the v0.7 gap on
// coremem.ScopedManager: Consolidate / Forget / StatsAll all ignore
// scope upstream — see llm-agent/memory/scoped_manager.go:128-144).
//
// Scope enforcement strategy: enumerate items via the exported
// coremem.Lister interface (which all three bundled memory types
// implement), filter by ctx scope using coremem's matching rules, then
// act on only the matching IDs.
type ScopedLifecycleManager struct {
	sm  *coremem.ScopedManager
	cfg *config
}

// forgetPair is the (id, importance) tuple used by the capacity-based
// Forget branch. Kept package-private — no caller need.
type forgetPair struct {
	id  string
	imp float64
}

// ErrScopedManagerRequired is returned by NewScopedLifecycleManager
// when the inner *coremem.ScopedManager is nil.
var ErrScopedManagerRequired = errors.New("memory: scoped manager required")

// NewScopedLifecycleManager wraps an existing *coremem.ScopedManager.
// Returns ErrScopedManagerRequired if inner is nil.
func NewScopedLifecycleManager(inner *coremem.ScopedManager, opts ...Option) (*ScopedLifecycleManager, error) {
	if inner == nil {
		return nil, ErrScopedManagerRequired
	}
	return &ScopedLifecycleManager{sm: inner, cfg: newConfig(opts)}, nil
}

// observer exposes the configured observer for in-package callers and
// tests. Package-private — callers should not depend on the accessor.
func (s *ScopedLifecycleManager) observer() Observer { return s.cfg.observer }

// ConsolidateScoped promotes Working→Episodic only for items whose
// stored scope matches the ctx scope. A zero-value ctx scope behaves
// like coremem.Manager.Consolidate (wildcard — every item considered).
//
// Threshold defaults to 0.7 if unset, mirroring coremem.Consolidate.
// MinAge is honored verbatim.
func (s *ScopedLifecycleManager) ConsolidateScoped(ctx context.Context, opts coremem.ConsolidateOptions) (int, error) {
	if opts.Threshold <= 0 {
		opts.Threshold = 0.7
	}
	mgr := s.sm.Inner()
	allItems, err := s.listAllScoped(ctx, 200)
	if err != nil {
		return 0, fmt.Errorf("memory: list working: %w", err)
	}
	working := allItems[coremem.KindWorking]
	count := 0
	for _, it := range working {
		if it.Importance < opts.Threshold {
			continue
		}
		if opts.MinAge > 0 {
			if it.CreatedAt.IsZero() {
				continue
			}
			if !it.CreatedAt.Add(opts.MinAge).Before(timeNow()) {
				continue
			}
		}
		clone := it
		clone.ID = ""
		if _, err := mgr.Add(ctx, coremem.KindEpisodic, clone); err != nil {
			return count, fmt.Errorf("memory: consolidate-scoped add: %w", err)
		}
		count++
	}
	return count, nil
}

// timeNow is overridable in tests if a future task needs deterministic
// clocks; today it is a plain alias to time.Now.
var timeNow = func() time.Time { return time.Now() }

// ForgetScoped applies the given Forget strategy ONLY to items whose
// stored scope matches the ctx scope. A zero-value ctx scope behaves
// like coremem.Manager.Forget (every item considered).
//
// Pinned items are always skipped, mirroring coremem.Manager.Forget.
// Strategies supported: ForgetByImportance, ForgetByAge, ForgetByCapacity.
func (s *ScopedLifecycleManager) ForgetScoped(ctx context.Context, kind coremem.Kind, opts coremem.ForgetOptions) (int, error) {
	mgr := s.sm.Inner()
	allItems, err := s.listAllScoped(ctx, 200)
	if err != nil {
		return 0, fmt.Errorf("memory: list %s: %w", kind, err)
	}
	candidates := allItems[kind]
	switch opts.Strategy {
	case coremem.ForgetByImportance:
		count := 0
		for _, it := range candidates {
			if coremem.IsPinned(it) {
				continue
			}
			if it.Importance < opts.Threshold {
				if err := mgr.Remove(ctx, kind, it.ID); err == nil {
					count++
				}
			}
		}
		return count, nil
	case coremem.ForgetByAge:
		if opts.MaxAge <= 0 {
			return 0, fmt.Errorf("memory: forget by age requires MaxAge > 0")
		}
		now := timeNow()
		count := 0
		for _, it := range candidates {
			if coremem.IsPinned(it) {
				continue
			}
			if now.Sub(it.CreatedAt) > opts.MaxAge {
				if err := mgr.Remove(ctx, kind, it.ID); err == nil {
					count++
				}
			}
		}
		return count, nil
	case coremem.ForgetByCapacity:
		if opts.Keep <= 0 {
			return 0, nil
		}
		// Sort by importance ascending; evict the lowest first. Pinned
		// items are excluded entirely (they don't count toward Keep nor
		// get removed).
		all := make([]forgetPair, 0, len(candidates))
		for _, it := range candidates {
			if coremem.IsPinned(it) {
				continue
			}
			all = append(all, forgetPair{it.ID, it.Importance})
		}
		if len(all) <= opts.Keep {
			return 0, nil
		}
		sortPairsByImpAsc(all)
		toEvict := len(all) - opts.Keep
		count := 0
		for i := 0; i < toEvict; i++ {
			if err := mgr.Remove(ctx, kind, all[i].id); err == nil {
				count++
			}
		}
		return count, nil
	default:
		return 0, fmt.Errorf("memory: unknown forget strategy %q", opts.Strategy)
	}
}

// StatsScoped returns per-kind Stats covering only items whose stored
// scope matches the ctx scope. A zero-value ctx scope behaves like
// coremem.Manager.StatsAll (every item counted).
//
// Returned Stats.Capacity mirrors the underlying memory's capacity
// (NOT a scope-local cap), because capacity is a per-memory-type
// attribute, not a per-scope one.
func (s *ScopedLifecycleManager) StatsScoped(ctx context.Context) (map[coremem.Kind]coremem.Stats, error) {
	allItems, err := s.listAllScoped(ctx, 200)
	if err != nil {
		return nil, fmt.Errorf("memory: stats list: %w", err)
	}
	innerStats := s.sm.Inner().StatsAll()
	out := make(map[coremem.Kind]coremem.Stats, len(allItems))
	now := timeNow()
	for kind, items := range allItems {
		var (
			count   = len(items)
			impSum  float64
			oldest  time.Time
			hasItem bool
		)
		for _, it := range items {
			impSum += it.Importance
			if !hasItem || it.CreatedAt.Before(oldest) {
				oldest = it.CreatedAt
				hasItem = true
			}
		}
		var avg float64
		if count > 0 {
			avg = impSum / float64(count)
		}
		var oldestAge time.Duration
		if hasItem {
			oldestAge = now.Sub(oldest)
		}
		out[kind] = coremem.Stats{
			Count:         count,
			Capacity:      innerStats[kind].Capacity,
			OldestAge:     oldestAge,
			AvgImportance: avg,
		}
	}
	return out, nil
}

// sortPairsByImpAsc is a small sort helper kept package-local so the
// ForgetByCapacity branch above does not pull in coremem internals.
// Insertion sort — stable, simple, and the only place in this
// package that needs ordering. N is small (page size 100).
func sortPairsByImpAsc(pairs []forgetPair) {
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j-1].imp > pairs[j].imp; j-- {
			pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
		}
	}
}

// listAllScoped enumerates every item across every active kind in the
// ctx scope, paging through ScopedManager.ListAll until each kind's
// NextCursor is the empty string. Returns the accumulated per-kind
// items. pageSize is the per-hop request size; the loop is unbounded.
//
// This closes the silent-truncation bug in the M1 helpers, which
// called ListAll once with no cursor and capped per-call processing at
// a single page.
func (s *ScopedLifecycleManager) listAllScoped(ctx context.Context, pageSize int) (map[coremem.Kind][]coremem.MemoryItem, error) {
	if pageSize <= 0 {
		pageSize = 200
	}
	out := make(map[coremem.Kind][]coremem.MemoryItem)
	cursors := map[coremem.Kind]string{}
	for {
		pages, err := s.sm.ListAll(ctx, coremem.ListFilter{}, pageSize, cursors)
		if err != nil {
			return nil, fmt.Errorf("paged list: %w", err)
		}
		anyMore := false
		nextCursors := map[coremem.Kind]string{}
		for kind, page := range pages {
			out[kind] = append(out[kind], page.Items...)
			if page.NextCursor != "" {
				nextCursors[kind] = page.NextCursor
				anyMore = true
			}
		}
		if !anyMore {
			return out, nil
		}
		cursors = nextCursors
	}
}
