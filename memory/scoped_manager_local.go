package memory

import (
	"context"
)

// ScopedManager wraps a sibling Manager and enforces ctx scope on reads and writes.
type ScopedManager struct {
	inner *Manager
}

// NewScopedManager wraps an existing sibling Manager.
func NewScopedManager(inner *Manager) (*ScopedManager, error) {
	if inner == nil {
		return nil, ErrManagerRequired
	}
	return &ScopedManager{inner: inner}, nil
}

// Inner returns the underlying sibling manager.
func (sm *ScopedManager) Inner() *Manager { return sm.inner }

func (sm *ScopedManager) Add(ctx context.Context, kind Kind, item MemoryItem) (string, error) {
	stampScope(&item, ScopeFrom(ctx))
	return sm.inner.Add(ctx, kind, item)
}

func (sm *ScopedManager) Get(ctx context.Context, kind Kind, id string) (MemoryItem, error) {
	it, err := sm.inner.Get(ctx, kind, id)
	if err != nil {
		return MemoryItem{}, err
	}
	if !ScopeFrom(ctx).Matches(readScope(it)) {
		return MemoryItem{}, ErrNotFound
	}
	return it, nil
}

func (sm *ScopedManager) Update(ctx context.Context, kind Kind, id string, fn func(*MemoryItem)) error {
	it, err := sm.inner.Get(ctx, kind, id)
	if err != nil {
		return err
	}
	if !ScopeFrom(ctx).Matches(readScope(it)) {
		return ErrNotFound
	}
	return sm.inner.Update(ctx, kind, id, fn)
}

func (sm *ScopedManager) Remove(ctx context.Context, kind Kind, id string) error {
	it, err := sm.inner.Get(ctx, kind, id)
	if err != nil {
		return err
	}
	if !ScopeFrom(ctx).Matches(readScope(it)) {
		return ErrNotFound
	}
	return sm.inner.Remove(ctx, kind, id)
}

func (sm *ScopedManager) Search(ctx context.Context, kind Kind, query string, topK int) ([]SearchResult, error) {
	raw, err := sm.inner.Search(ctx, kind, query, topK)
	if err != nil {
		return nil, err
	}
	return filterByScope(raw, ScopeFrom(ctx)), nil
}

func (sm *ScopedManager) SearchAll(ctx context.Context, query string, topK int) (map[Kind][]SearchResult, error) {
	raw, err := sm.inner.SearchAll(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	s := ScopeFrom(ctx)
	out := make(map[Kind][]SearchResult, len(raw))
	for kind, results := range raw {
		out[kind] = filterByScope(results, s)
	}
	return out, nil
}

func (sm *ScopedManager) ListAll(ctx context.Context, filter ListFilter, pageSize int, cursors map[Kind]string) (map[Kind]ListPage, error) {
	if s := ScopeFrom(ctx); !s.IsZero() {
		filter.Scope = s
	}
	return sm.inner.ListAll(ctx, filter, pageSize, cursors)
}

func (sm *ScopedManager) Consolidate(ctx context.Context, opts ConsolidateOptions) (int, error) {
	return sm.inner.Consolidate(ctx, opts)
}

func (sm *ScopedManager) Forget(ctx context.Context, kind Kind, opts ForgetOptions) (int, error) {
	return sm.inner.Forget(ctx, kind, opts)
}

func (sm *ScopedManager) StatsAll() map[Kind]Stats {
	return sm.inner.StatsAll()
}

func filterByScope(results []SearchResult, s Scope) []SearchResult {
	if s.IsZero() {
		return results
	}
	out := make([]SearchResult, 0, len(results))
	for _, r := range results {
		if s.Matches(readScope(r.Item)) {
			out = append(out, r)
		}
	}
	return out
}
