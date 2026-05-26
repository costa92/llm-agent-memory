// Package memory — manager.go is the Phase D-1 implementation of the
// sibling-owned, capability-interface-typed Manager that replaces
// coremem.Manager as the recommended construction surface in v1.0.0+.
//
// Why this exists (vs reusing coremem.Manager directly):
//
//  1. coremem.ManagerOptions fields are typed *coremem.WorkingMemory /
//     *coremem.EpisodicMemory / *coremem.SemanticMemory (see
//     coremem/manager.go:22-35). That makes it impossible to install
//     a decorator like coremem.WithSanitizer (which returns the
//     coremem.Memory interface, NOT a concrete pointer — see
//     coremem/policy_hook.go:37-45 and the LIMITATION block at
//     coremem/doc.go:122-128).
//
//  2. Future external backends (Postgres, pgvector, Redis) cannot
//     impersonate the concrete coremem types. With interface-typed
//     TierOptions, any object satisfying coremem.Memory + coremem.Lister
//     + coremem.Exporter + coremem.Importer can be installed.
//
// What it does NOT do: this Manager is NOT a coremem.Memory itself.
// It is a coordinator with a Kind-discriminated dispatch surface
// mirroring coremem.Manager's public API.
//
// Compatibility: the v0.7 coremem.Manager is unaffected. The compat/
// sub-package provides a one-line bridge for callers wired to
// coremem.NewManager(coremem.ManagerOptions{...}).
package memory

import (
	"context"
	"errors"
	"fmt"

	coremem "github.com/costa92/llm-agent/memory"
)

// LifecycleMemory is the new capability interface introduced in v1.0.0.
// It models the two operations that coremem.Manager performs by reaching
// through the Memory interface into private *scoredStore state (see
// coremem/manager.go:191 and :239). External backends that want to
// expose lifecycle semantics implement this interface directly; the
// bundled coremem types do not satisfy it (they need a small adapter —
// see coreManagerLifecycle in this file).
//
// Consolidate's semantics mirror coremem.Manager.Consolidate: promote
// items from this kind into the next-higher kind (typically
// Working → Episodic) based on opts.Threshold and opts.MinAge. Returns
// the count promoted.
//
// Forget's semantics mirror coremem.Manager.Forget on the receiving
// kind: apply the chosen strategy (importance / age / capacity) and
// return the count deleted.
type LifecycleMemory interface {
	Consolidate(ctx context.Context, opts coremem.ConsolidateOptions) (int, error)
	Forget(ctx context.Context, kind coremem.Kind, opts coremem.ForgetOptions) (int, error)
}

// TierOptions wires the per-kind capability set. Memory is required;
// every other field is optional — if nil, the corresponding Manager
// method either skips this tier (for read-side fan-outs like ListAll /
// ExportAll) or returns ErrCapabilityMissing (for direct calls like
// Consolidate). The bundled coremem types satisfy Memory + Lister +
// Exporter + Importer — for those, a single object can fill four
// fields:
//
//   w, _ := coremem.NewWorking(emb, coremem.WorkingOptions{})
//   opts.Working = memory.TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w}
//
// Lifecycle requires the explicit LifecycleMemory interface (or a
// *coremem.Manager-backed adapter — see Options.CoreManager). The
// bundled types do NOT satisfy LifecycleMemory directly because the
// operation crosses tier boundaries.
type TierOptions struct {
	Memory    coremem.Memory   // required
	Lister    coremem.Lister   // optional
	Exporter  coremem.Exporter // optional
	Importer  coremem.Importer // optional
	Lifecycle LifecycleMemory  // optional
}

// Options is the v1.0.0 analogue of coremem.ManagerOptions. Pass to
// NewManager. At least one tier's Memory field must be non-nil.
type Options struct {
	Working  TierOptions
	Episodic TierOptions
	Semantic TierOptions

	// SnapshotStore mirrors coremem.ManagerOptions.SnapshotStore. Used
	// by ExportAll/ImportAll when persistKey != "". Nil keeps
	// persistence in-memory.
	SnapshotStore coremem.SnapshotStore

	// CoreManager is an OPTIONAL escape hatch. When non-nil, lifecycle
	// methods (Consolidate, Forget) on tiers whose Lifecycle field is
	// nil fall back to delegating into this *coremem.Manager via the
	// coreManagerLifecycle adapter. Keeps the compat-shim path
	// ergonomic (one line to bridge a legacy *coremem.Manager into the
	// new sibling Manager surface).
	//
	// CoreManager is consulted ONLY for Lifecycle fallback today. It
	// does NOT supplant a tier whose Memory field is nil.
	CoreManager *coremem.Manager
}

// Manager is the sibling-owned, capability-interface-typed coordinator.
// Construct via NewManager. Goroutine-safe: every method is a thin
// dispatch on capability fields whose implementations are themselves
// goroutine-safe in the bundled coremem types.
type Manager struct {
	opts Options
}

// --- sentinel errors ------------------------------------------------------

// ErrNoTiers is returned by NewManager when every tier's Memory field
// is nil. Analogue of coremem.ErrNoMemories.
var ErrNoTiers = errors.New("memory: manager requires at least one tier with a Memory")

// ErrTierDisabled is returned when a method targets a kind whose
// TierOptions.Memory is nil. errors.Is-compatible with
// coremem.ErrKindDisabled for callers already comparing against the
// core sentinel.
var ErrTierDisabled = fmt.Errorf("memory: tier disabled: %w", coremem.ErrKindDisabled)

// ErrCapabilityMissing is returned when a tier is present but the
// requested capability (Lister, Lifecycle, etc.) was not wired into
// its TierOptions and no fallback (e.g. Options.CoreManager) is
// available. The error message names the kind and the missing
// capability.
var ErrCapabilityMissing = errors.New("memory: capability missing on tier")

// ErrUnknownKind is returned by dispatch helpers when an unrecognized
// Kind value is passed.
var ErrUnknownKind = errors.New("memory: unknown kind")

// NewManager validates opts and returns a *Manager. Returns ErrNoTiers
// if every tier's Memory is nil.
func NewManager(opts Options) (*Manager, error) {
	if opts.Working.Memory == nil && opts.Episodic.Memory == nil && opts.Semantic.Memory == nil {
		return nil, ErrNoTiers
	}
	return &Manager{opts: opts}, nil
}

// HasKind reports whether a tier is wired for the given kind. A tier is
// "wired" iff its TierOptions.Memory is non-nil. Useful for callers
// that want to branch before calling Add / Search.
func (m *Manager) HasKind(kind coremem.Kind) bool {
	t, err := m.tierFor(kind)
	if err != nil {
		return false
	}
	return t.Memory != nil
}

// tierFor returns the TierOptions for the given kind. Returns
// ErrUnknownKind if kind is not one of KindWorking / KindEpisodic /
// KindSemantic; returns the TierOptions (with possibly-nil Memory)
// otherwise. Callers must check tier.Memory before dispatching.
func (m *Manager) tierFor(kind coremem.Kind) (TierOptions, error) {
	switch kind {
	case coremem.KindWorking:
		return m.opts.Working, nil
	case coremem.KindEpisodic:
		return m.opts.Episodic, nil
	case coremem.KindSemantic:
		return m.opts.Semantic, nil
	default:
		return TierOptions{}, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
}

// requireMemory returns the tier's Memory or ErrTierDisabled.
func (m *Manager) requireMemory(kind coremem.Kind) (coremem.Memory, error) {
	t, err := m.tierFor(kind)
	if err != nil {
		return nil, err
	}
	if t.Memory == nil {
		return nil, fmt.Errorf("memory: manager %s: %w", kind, ErrTierDisabled)
	}
	return t.Memory, nil
}

// Add dispatches to the wired tier's Memory.Add. Returns
// ErrTierDisabled if the kind has no Memory wired.
func (m *Manager) Add(ctx context.Context, kind coremem.Kind, item coremem.MemoryItem) (string, error) {
	mem, err := m.requireMemory(kind)
	if err != nil {
		return "", err
	}
	return mem.Add(ctx, item)
}

// Get fetches an item from the named tier.
func (m *Manager) Get(ctx context.Context, kind coremem.Kind, id string) (coremem.MemoryItem, error) {
	mem, err := m.requireMemory(kind)
	if err != nil {
		return coremem.MemoryItem{}, err
	}
	return mem.Get(ctx, id)
}

// Update mutates an item in the named tier.
func (m *Manager) Update(ctx context.Context, kind coremem.Kind, id string, fn func(*coremem.MemoryItem)) error {
	mem, err := m.requireMemory(kind)
	if err != nil {
		return err
	}
	return mem.Update(ctx, id, fn)
}

// Remove deletes an item from the named tier.
func (m *Manager) Remove(ctx context.Context, kind coremem.Kind, id string) error {
	mem, err := m.requireMemory(kind)
	if err != nil {
		return err
	}
	return mem.Remove(ctx, id)
}

// Search runs Memory.Search on one named tier.
func (m *Manager) Search(ctx context.Context, kind coremem.Kind, query string, topK int) ([]coremem.SearchResult, error) {
	mem, err := m.requireMemory(kind)
	if err != nil {
		return nil, err
	}
	return mem.Search(ctx, query, topK)
}

// StatsAll returns Stats for every active tier. Tiers without a wired
// Memory are omitted from the result map. Parity with
// coremem.Manager.StatsAll.
func (m *Manager) StatsAll() map[coremem.Kind]coremem.Stats {
	out := make(map[coremem.Kind]coremem.Stats, 3)
	for _, kind := range []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic, coremem.KindSemantic} {
		t, _ := m.tierFor(kind)
		if t.Memory == nil {
			continue
		}
		out[kind] = t.Memory.Stats()
	}
	return out
}
