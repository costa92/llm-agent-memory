package memory

import (
	"context"
	"errors"
	"fmt"

	coremem "github.com/costa92/llm-agent/memory"
)

// Reserved metadata keys written by Consolidator.Consolidate on
// promotion. They start with a single underscore to match the
// convention used by coremem's private keys (_scope, _source, etc.)
// and to clearly mark them as internal-extension.
const (
	// MetaKeyConsolidatedAt is written on the source working item as a
	// time.Time (formatted RFC 3339 when JSON-round-tripped via
	// encoding/json — go's default behavior).
	MetaKeyConsolidatedAt = "_consolidated_at"

	// MetaKeyPromotedFrom is written on the episodic clone as a string
	// pointing at the source working item's ID.
	MetaKeyPromotedFrom = "_promoted_from"

	// MetaKeyPromotionCount is written on the source working item as an
	// int. Incremented on each successful promotion (currently capped at
	// 1 by the promote-once policy enforced by Consolidator.Consolidate).
	MetaKeyPromotionCount = "_promotion_count"
)

// Consolidator wraps a *coremem.Manager and exposes a dedupe-aware
// Consolidate that mirrors coremem.Manager.Consolidate (copy
// Working→Episodic by importance + min-age) but additionally:
//
//  1. Writes MetaKeyConsolidatedAt and MetaKeyPromotionCount on the
//     source working item via coremem.Manager.Update.
//  2. Writes MetaKeyPromotedFrom on the episodic clone.
//  3. Skips items whose MetaKeyPromotionCount is already ≥ 1
//     (the v0.1 promote-once policy).
//
// Source items are NOT removed (mirrors coremem semantics). Pinned and
// disabled status on the source are preserved verbatim by Update.
type Consolidator struct {
	mgr *coremem.Manager
	cfg *config
}

// ErrManagerRequired is returned by NewConsolidator when the inner
// *coremem.Manager is nil. (Same sentinel name as coremem's, but
// distinct identity — callers should errors.Is on the local one.)
var ErrManagerRequired = errors.New("memory: manager required")

// NewConsolidator wraps an existing *coremem.Manager. Returns
// ErrManagerRequired if inner is nil.
func NewConsolidator(inner *coremem.Manager, opts ...Option) (*Consolidator, error) {
	if inner == nil {
		return nil, ErrManagerRequired
	}
	return &Consolidator{mgr: inner, cfg: newConfig(opts)}, nil
}

// observer exposes the configured observer for in-package callers and
// tests. Package-private — callers should not depend on the accessor.
func (c *Consolidator) observer() Observer { return c.cfg.observer }

// Consolidate enumerates Working via the Lister capability, applies
// Threshold + MinAge, skips items already promoted (MetaKeyPromotionCount
// ≥ 1), copies survivors into Episodic with MetaKeyPromotedFrom set,
// and writes MetaKeyConsolidatedAt + MetaKeyPromotionCount on each
// source. Returns the number of items promoted in this call.
//
// Threshold defaults to 0.7 if unset (matches coremem).
func (c *Consolidator) Consolidate(ctx context.Context, opts coremem.ConsolidateOptions) (int, error) {
	if opts.Threshold <= 0 {
		opts.Threshold = 0.7
	}
	allItems, err := c.listAllPaged(ctx, 200)
	if err != nil {
		return 0, fmt.Errorf("memory: consolidate list: %w", err)
	}
	working := allItems[coremem.KindWorking]
	now := timeNow()
	count := 0
	for _, it := range working {
		if it.Importance < opts.Threshold {
			continue
		}
		if opts.MinAge > 0 && now.Sub(it.CreatedAt) < opts.MinAge {
			continue
		}
		if promotionCountOf(it) >= 1 {
			continue
		}
		clone := it
		clone.ID = "" // let episodic re-generate
		if clone.Metadata == nil {
			clone.Metadata = map[string]any{}
		} else {
			// Deep-copy the metadata map so we don't mutate the source
			// item's map indirectly.
			cp := make(map[string]any, len(clone.Metadata)+1)
			for k, v := range clone.Metadata {
				cp[k] = v
			}
			clone.Metadata = cp
		}
		clone.Metadata[MetaKeyPromotedFrom] = it.ID
		if _, err := c.mgr.Add(ctx, coremem.KindEpisodic, clone); err != nil {
			return count, fmt.Errorf("memory: consolidate add: %w", err)
		}
		srcID := it.ID
		err := c.mgr.Update(ctx, coremem.KindWorking, srcID, func(m *coremem.MemoryItem) {
			if m.Metadata == nil {
				m.Metadata = map[string]any{}
			}
			m.Metadata[MetaKeyConsolidatedAt] = now
			m.Metadata[MetaKeyPromotionCount] = promotionCountOf(*m) + 1
		})
		if err != nil {
			return count, fmt.Errorf("memory: consolidate stamp source: %w", err)
		}
		count++
	}
	emit(c.observer(), EventConsolidatedTotal, map[string]any{"n": count})
	return count, nil
}

// promotionCountOf reads MetaKeyPromotionCount from an item, tolerating
// both int (what we write) and float64 (what JSON round-trips produce).
// Returns 0 if absent or wrong type.
func promotionCountOf(it coremem.MemoryItem) int {
	if it.Metadata == nil {
		return 0
	}
	raw, ok := it.Metadata[MetaKeyPromotionCount]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// listAllPaged enumerates every item across every active kind via
// Manager.ListAll, paging through cursors until each kind reports an
// empty NextCursor. Mirrors ScopedLifecycleManager.listAllScoped — the
// two cannot share a helper today because Consolidator wraps a
// *coremem.Manager, not a *coremem.ScopedManager.
//
// Active-but-empty kinds materialize as map keys with empty slices
// (parity with the underlying ListAll contract — see scoped_lifecycle.go
// listAllScoped fix in commit 68e17d8).
func (c *Consolidator) listAllPaged(ctx context.Context, pageSize int) (map[coremem.Kind][]coremem.MemoryItem, error) {
	if pageSize <= 0 {
		pageSize = 200
	}
	out := make(map[coremem.Kind][]coremem.MemoryItem)
	cursors := map[coremem.Kind]string{}
	for {
		pages, err := c.mgr.ListAll(ctx, coremem.ListFilter{}, pageSize, cursors)
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
