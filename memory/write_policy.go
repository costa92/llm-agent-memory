// Package memory — write_policy.go declares the Phase C-1 write-policy
// surface: a single interface that consumes a ProposedWrite and emits
// a WritePolicyDecision covering accept / redact / reject. The
// interface intentionally lifts the four decisions documented in
// docs/memory-roadmap.zh-CN.md §4.3 (item C-1) — user-saved,
// agent-inferred, reject, redact — into a single funnel so upper
// layers don't reinvent them per-feature.
//
// WritePolicy is NOT a wrap of coremem.Sanitizer. Sanitizer can only
// accept-or-reject-or-mutate (see policy_hook.go:23). WritePolicy
// additionally carries the final Kind (so a user-typed "remember
// this" can reroute to Episodic), the final Importance + Tags, and
// a structured Reason field used by the EventWritePolicyDecided
// observer event.
//
// For callers wired to the existing coremem.Sanitizer chain,
// PolicyAdapter lets a WritePolicy satisfy the narrower interface —
// see PolicyAdapter godoc for the rerouting limitation.
package memory

import (
	"context"
	"errors"

	coremem "github.com/costa92/llm-agent/memory"
)

// WriteSource tags the origin of a ProposedWrite. Policies use it to
// pick different rules for user-typed memories vs agent-inferred
// memories. Add new variants by appending; never renumber.
type WriteSource string

const (
	// SourceUserSaved means a human explicitly asked to remember this.
	// Policies typically promote these directly to Episodic.
	SourceUserSaved WriteSource = "user_saved"
	// SourceAgentInferred means the agent inferred a fact from
	// conversation. Policies typically gate these by importance.
	SourceAgentInferred WriteSource = "agent_inferred"
	// SourceSystem means a background process (consolidator,
	// importer, migration) is writing. Policies usually pass through.
	SourceSystem WriteSource = "system"
)

// Verdict is the policy's decision on a ProposedWrite.
type Verdict string

const (
	// VerdictAccept means the wrapper writes Decision.Item to
	// Decision.Kind verbatim.
	VerdictAccept Verdict = "accept"
	// VerdictRedact means the wrapper writes Decision.Item to
	// Decision.Kind — identical write path as VerdictAccept, but
	// observability distinguishes "this was a redaction" from "this was
	// a clean accept" so consumers can count them separately.
	VerdictRedact Verdict = "redact"
	// VerdictReject means the wrapper does NOT write; Add returns
	// ErrRejectedByPolicy. Decision.Reason flows into the observer
	// event and the wrapped error's context if the caller logs it.
	VerdictReject Verdict = "reject"
)

// ProposedWrite is the input to WritePolicy.Decide. Kind is the
// caller's intended kind; the policy may override it via the returned
// Decision.Kind. Item is the caller's intended payload; the policy
// may mutate, redact, or replace it. Hint is a free-form per-call
// bag of caller context (e.g. the chat-channel ID); core never
// inspects it.
type ProposedWrite struct {
	Kind   coremem.Kind
	Item   coremem.MemoryItem
	Source WriteSource
	Hint   map[string]any
}

// WritePolicyDecision is the output of WritePolicy.Decide. Item and
// Kind are ignored when Verdict == VerdictReject. Reason is opaque
// to core (flows into observer events and reject errors).
type WritePolicyDecision struct {
	Verdict Verdict
	Kind    coremem.Kind
	Item    coremem.MemoryItem
	Reason  string
}

// WritePolicy is the single funnel for write decisions. Implementations
// MUST be goroutine-safe (Decide is called from arbitrary caller
// goroutines). Implementations MUST NOT block on external I/O on the
// hot path — the wrapper does not budget for that today.
type WritePolicy interface {
	Decide(ctx context.Context, in ProposedWrite) WritePolicyDecision
}

// PolicyFunc adapts a plain function to the WritePolicy interface.
type PolicyFunc func(ctx context.Context, in ProposedWrite) WritePolicyDecision

// Decide calls f.
func (f PolicyFunc) Decide(ctx context.Context, in ProposedWrite) WritePolicyDecision {
	return f(ctx, in)
}

// ErrRejectedByPolicy is returned by PolicyEnforcingMemory.Add when
// the configured WritePolicy returns VerdictReject. It aliases the
// identically-named core sentinel so consumers can errors.Is(err,
// memory.ErrRejectedByPolicy) without dual-importing the core
// package. The alias is intentional: there is exactly one rejection
// condition across the stack.
var ErrRejectedByPolicy = coremem.ErrRejectedByPolicy

// ErrPolicyKindRerouteUnsupported is returned by PolicyAdapter.Sanitize
// when the wrapped policy returns a Decision.Kind that differs from
// the input kind. Sanitizer cannot reroute (its return triple does
// not carry a kind), so PolicyAdapter rejects rather than silently
// dropping the reroute. Consumers wanting reroute semantics must
// use PolicyEnforcingMemory directly.
var ErrPolicyKindRerouteUnsupported = errors.New("memory: policy adapter cannot reroute kind via Sanitizer interface")

// PolicyEnforcingMemory wraps a *coremem.Manager and routes every Add
// through the configured WritePolicy. The wrapper does not implement
// the coremem.Memory interface — its Add takes a ProposedWrite (with
// Source + Hint context) rather than a bare MemoryItem, because the
// policy contract is richer than coremem.Memory.Add.
//
// Read paths (Get, Search, Update, Remove, Stats, ListAll) are not
// exposed by this wrapper: policy enforcement is an Add-time concern,
// and callers needing reads operate on the underlying *coremem.Manager
// directly. This mirrors the M2 Consolidator pattern (writes only).
type PolicyEnforcingMemory struct {
	mgr    *coremem.Manager
	policy WritePolicy
	cfg    *config
}

// ErrPolicyEnforcingManagerRequired is returned when the inner
// *coremem.Manager is nil.
var ErrPolicyEnforcingManagerRequired = errors.New("memory: policy-enforcing memory requires manager")

// ErrPolicyRequired is returned when the WritePolicy is nil.
var ErrPolicyRequired = errors.New("memory: policy-enforcing memory requires a non-nil WritePolicy")

// NewPolicyEnforcingMemory wraps an existing *coremem.Manager with the
// given policy. opts is the shared functional-option list from
// observer.go (e.g., WithObserver).
func NewPolicyEnforcingMemory(inner *coremem.Manager, policy WritePolicy, opts ...Option) (*PolicyEnforcingMemory, error) {
	if inner == nil {
		return nil, ErrPolicyEnforcingManagerRequired
	}
	if policy == nil {
		return nil, ErrPolicyRequired
	}
	return &PolicyEnforcingMemory{
		mgr:    inner,
		policy: policy,
		cfg:    newConfig(opts),
	}, nil
}

// observer exposes the configured observer for in-package call sites
// and tests. Package-private — callers should not depend on the
// accessor.
func (p *PolicyEnforcingMemory) observer() Observer { return p.cfg.observer }

// Add dispatches in through the WritePolicy. On VerdictAccept and
// VerdictRedact, the decided Item is written to the decided Kind.
// On VerdictReject, ErrRejectedByPolicy is returned. The EventWrite-
// PolicyDecided observer event is emitted in all three cases.
func (p *PolicyEnforcingMemory) Add(ctx context.Context, in ProposedWrite) (string, error) {
	decision := p.policy.Decide(ctx, in)
	switch decision.Verdict {
	case VerdictAccept, VerdictRedact:
		id, err := p.mgr.Add(ctx, decision.Kind, decision.Item)
		if err != nil {
			return "", err
		}
		emit(p.observer(), EventWritePolicyDecided, map[string]any{
			"verdict":      string(decision.Verdict),
			"input_kind":   in.Kind,
			"decided_kind": decision.Kind,
			"source":       string(in.Source),
			"reason":       decision.Reason,
		})
		return id, nil
	case VerdictReject:
		emit(p.observer(), EventWritePolicyDecided, map[string]any{
			"verdict":      string(decision.Verdict),
			"input_kind":   in.Kind,
			"decided_kind": in.Kind, // no reroute happened; mirror input
			"source":       string(in.Source),
			"reason":       decision.Reason,
		})
		return "", ErrRejectedByPolicy
	default:
		return "", errors.New("memory: write policy returned unknown verdict")
	}
}
