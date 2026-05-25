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
	sm *coremem.ScopedManager
}

// ErrScopedManagerRequired is returned by NewScopedLifecycleManager
// when the inner *coremem.ScopedManager is nil.
var ErrScopedManagerRequired = errors.New("memory: scoped manager required")

// NewScopedLifecycleManager wraps an existing *coremem.ScopedManager.
// Returns ErrScopedManagerRequired if inner is nil.
func NewScopedLifecycleManager(inner *coremem.ScopedManager) (*ScopedLifecycleManager, error) {
	if inner == nil {
		return nil, ErrScopedManagerRequired
	}
	return &ScopedLifecycleManager{sm: inner}, nil
}

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
	// Enumerate working items in this scope via the ctx-aware
	// ScopedManager.ListAll, which applies scope filtering automatically.
	pages, err := s.sm.ListAll(ctx, coremem.ListFilter{}, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("memory: list working: %w", err)
	}
	working := pages[coremem.KindWorking].Items
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
		clone.ID = "" // let episodic re-generate
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
