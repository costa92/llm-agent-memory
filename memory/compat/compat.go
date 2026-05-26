// Package compat provides a one-release-window bridge for callers
// that constructed *coremem.Manager via coremem.NewManager(coremem.
// ManagerOptions{...}). At v1.0.0 of github.com/costa92/llm-agent-
// memory, new code is expected to construct *memory.Manager directly
// with the capability-interface memory.Options. This package eases
// the upgrade path:
//
//  - Drop-in: replace `coreMgr, _ := coremem.NewManager(coremem.
//    ManagerOptions{...})` with `mgr := compat.NewManagerFromCore(coreMgr)`
//    and your downstream code that calls Add / Get / Search / etc.
//    keeps working unchanged.
//
//  - Field-by-field: replace `coremem.NewManager(opts)` with
//    `compat.NewManagerFromLegacyOptions(opts)` directly.
//
// Removal window: this sub-package stays in the v1.x line. It is
// REMOVED at v2.0.0 of github.com/costa92/llm-agent-memory. See
// docs/memory-v1-migration.zh-CN.md for the canonical upgrade
// recipe.
package compat

import (
	"fmt"

	"github.com/costa92/llm-agent-memory/memory"
	coremem "github.com/costa92/llm-agent/memory"
)

// LegacyOptions is an alias of coremem.ManagerOptions. The alias lets
// callers write `var opts compat.LegacyOptions = ...` and then pass
// it to either coremem.NewManager or compat.NewManagerFromLegacyOptions
// without a conversion.
//
// Deprecated: prefer memory.Options. LegacyOptions is removed at v2.0.0.
type LegacyOptions = coremem.ManagerOptions

// NewManagerFromCore wraps an existing *coremem.Manager in the v1
// sibling Manager surface. The returned *memory.Manager exposes every
// method the sibling Manager has; lifecycle calls (Consolidate /
// Forget) fall back to the wrapped *coremem.Manager.
//
// The wrapped *coremem.Manager is NOT cloned. Mutations to it (e.g.
// direct .Add calls) are visible through the returned wrapper, and
// vice versa.
//
// Returns nil if coreMgr is nil — callers should check before use.
//
// Deprecated: prefer constructing memory.NewManager(memory.Options{...})
// directly. NewManagerFromCore is removed at v2.0.0.
func NewManagerFromCore(coreMgr *coremem.Manager) *memory.Manager {
	if coreMgr == nil {
		return nil
	}
	mgr, err := memory.NewManager(memory.Options{
		CoreManager: coreMgr,
	})
	if err != nil {
		panic(fmt.Sprintf("compat: unexpected NewManager error: %v", err))
	}
	return mgr
}

// NewManagerFromLegacyOptions adapts the v0.x coremem.ManagerOptions
// shape to the v1 memory.Options shape — every concrete-typed field
// becomes the corresponding TierOptions.Memory (interface-typed)
// entry, with the same SnapshotStore passed through. Returns
// memory.ErrNoTiers (the v1 sentinel) if every tier was nil — same
// semantic as coremem.NewManager returning coremem.ErrNoMemories.
//
// Deprecated: prefer constructing memory.NewManager(memory.Options{...})
// directly. NewManagerFromLegacyOptions is removed at v2.0.0.
func NewManagerFromLegacyOptions(opts LegacyOptions) (*memory.Manager, error) {
	v1opts := memory.Options{SnapshotStore: opts.SnapshotStore}
	if opts.Working != nil {
		v1opts.Working = memory.TierOptions{
			Memory:   opts.Working,
			Lister:   opts.Working,
			Exporter: opts.Working,
			Importer: opts.Working,
		}
	}
	if opts.Episodic != nil {
		v1opts.Episodic = memory.TierOptions{
			Memory:   opts.Episodic,
			Lister:   opts.Episodic,
			Exporter: opts.Episodic,
			Importer: opts.Episodic,
		}
	}
	if opts.Semantic != nil {
		v1opts.Semantic = memory.TierOptions{
			Memory:   opts.Semantic,
			Lister:   opts.Semantic,
			Exporter: opts.Semantic,
			Importer: opts.Semantic,
		}
	}
	return memory.NewManager(v1opts)
}
