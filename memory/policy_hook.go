package memory

import (
	"context"
	"errors"
)

// This file is the sibling-owned home of the Sanitizer policy hook.
// It was lifted verbatim from the framework's memory policy_hook.go
// during the contract-collapse (Phase 1a): the contract
// leaf intentionally does NOT export Sanitizer / SanitizerFunc /
// WithSanitizer because they are policy behavior, not part of the data
// + interface contract. They live here, typed against the local
// (contract-backed) Memory / Kind / MemoryItem aliases.

// Sanitizer inspects an item before it enters storage. The return
// triple describes the verdict:
//
//   - (newItem, true,  nil)    keep the item (possibly redacted).
//   - (_,       false, nil)    silently reject; Memory.Add returns
//     ErrRejectedByPolicy.
//   - (_,       _,     != nil) propagate the error to the Add caller.
//
// Sanitizers are chained left-to-right: each receives the item the
// previous stage emitted. The first stage that returns keep=false
// short-circuits — subsequent stages are not invoked.
//
// Sanitizers run ONLY on Add. Read paths (Get, Search, Update, Remove,
// Stats) bypass the chain to keep the audit trail and lookup semantics
// independent of policy mutations.
type Sanitizer interface {
	Sanitize(ctx context.Context, kind Kind, item MemoryItem) (MemoryItem, bool, error)
}

// SanitizerFunc adapts a plain function to the Sanitizer interface.
type SanitizerFunc func(ctx context.Context, kind Kind, item MemoryItem) (MemoryItem, bool, error)

// Sanitize calls f.
func (f SanitizerFunc) Sanitize(ctx context.Context, kind Kind, item MemoryItem) (MemoryItem, bool, error) {
	return f(ctx, kind, item)
}

// ErrRejectedByPolicy is returned by Memory.Add when a Sanitizer in
// the chain returns keep=false. Callers should treat this as a
// non-retryable application-level reject, distinct from transport or
// embedding failures.
var ErrRejectedByPolicy = errors.New("memory: item rejected by policy")

// sanitizingMemory wraps a Memory and runs the chain on Add. All other
// methods pass through unchanged. The type is intentionally unexported
// — callers always obtain it through the WithSanitizer factory and
// hold a Memory interface value.
type sanitizingMemory struct {
	inner Memory
	chain []Sanitizer
}

// WithSanitizer wraps inner with the given sanitizer chain. When chain
// is empty, returns inner verbatim (no allocation, no observable
// behavior change).
func WithSanitizer(inner Memory, chain ...Sanitizer) Memory {
	if len(chain) == 0 {
		return inner
	}
	return &sanitizingMemory{inner: inner, chain: chain}
}

// Type passes through to inner.
func (sm *sanitizingMemory) Type() Kind { return sm.inner.Type() }

// Add runs the chain. The first stage to return keep=false short-circuits.
func (sm *sanitizingMemory) Add(ctx context.Context, item MemoryItem) (string, error) {
	cur := item
	for _, s := range sm.chain {
		out, keep, err := s.Sanitize(ctx, sm.Type(), cur)
		if err != nil {
			return "", err
		}
		if !keep {
			return "", ErrRejectedByPolicy
		}
		cur = out
	}
	return sm.inner.Add(ctx, cur)
}

// Search passes through to inner.
func (sm *sanitizingMemory) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	return sm.inner.Search(ctx, query, topK)
}

// Get passes through to inner.
func (sm *sanitizingMemory) Get(ctx context.Context, id string) (MemoryItem, error) {
	return sm.inner.Get(ctx, id)
}

// Update passes through to inner. Note that mutations made inside fn
// are NOT run through the chain — sanitizer is an Add-time hook only.
func (sm *sanitizingMemory) Update(ctx context.Context, id string, fn func(*MemoryItem)) error {
	return sm.inner.Update(ctx, id, fn)
}

// Remove passes through to inner.
func (sm *sanitizingMemory) Remove(ctx context.Context, id string) error {
	return sm.inner.Remove(ctx, id)
}

// Stats passes through to inner.
func (sm *sanitizingMemory) Stats() Stats {
	return sm.inner.Stats()
}
