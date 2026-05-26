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
