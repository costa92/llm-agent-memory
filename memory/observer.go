package memory

// Observer is the optional sink for memory metric events. A nil
// Observer is the zero-config no-op — callers who do not opt in see
// exactly the same behavior they got in M1. Implementations MUST NOT
// block, MUST NOT panic, and MUST NOT return an error: any failure to
// record is the implementation's problem. The hot path is unconditional
// so OnEvent is called on the goroutine that emitted the event.
//
// The Observer interface is intentionally minimal — it gives consumers
// a single typed funnel. Adapters (Prometheus, OTel, log emitters) live
// outside this package.
//
// A nil Observer (untyped nil) is the documented no-op path: emit
// returns immediately. Passing an interface value that wraps a nil
// concrete pointer (e.g., `var r *MyObserver; var o Observer = r`)
// is undefined behavior and may panic — emit cannot detect this case
// without reflection, which would impose hot-path cost not justified
// by the misuse pattern.
type Observer interface {
	OnEvent(e Event)
}

// Event is the typed payload delivered to Observer.OnEvent. Name is one
// of the canonical event-name constants declared below (EventAddTotal,
// EventSearchTotal, ...). Attrs is an optional bag of structured
// attributes whose schema is frozen per event-name at v0.2.0; future
// additions are backwards-compatible (new keys may appear, existing
// keys are never renamed or removed).
//
// Attribute schemas per event name (v0.2.0):
//
//	EventAddTotal:              {"kind": coremem.Kind}
//	EventSearchTotal:           {"query_len": int}
//	EventSearchHits:            {"n": int}            // hit count
//	EventConsolidatedTotal:     {"n": int}            // promoted count
//	EventForgottenTotal:        {"kind": coremem.Kind, "n": int}
//	EventSnapshotItems:         {"kind": coremem.Kind, "n": int}
//	EventSnapshotVectorsBytes:  {"kind": coremem.Kind, "bytes": int}
//	EventWritePolicyDecided:    {"verdict": string, "input_kind": coremem.Kind,
//	                             "decided_kind": coremem.Kind, "source": string,
//	                             "reason": string}
type Event struct {
	Name  string
	Attrs map[string]any
}

// Canonical event names. These mirror the seven minimum-observability
// metrics from docs/memory-roadmap.zh-CN.md §4.2 B-1. Consumers should
// switch on these constants (NOT on raw string literals).
const (
	EventAddTotal             = "memory_add_total"
	EventSearchTotal          = "memory_search_total"
	EventSearchHits           = "memory_search_hits"
	EventConsolidatedTotal    = "memory_consolidated_total"
	EventForgottenTotal       = "memory_forgotten_total"
	EventSnapshotItems        = "memory_snapshot_items"
	EventSnapshotVectorsBytes = "memory_snapshot_vectors_bytes"
	// EventWritePolicyDecided is emitted by PolicyEnforcingMemory.Add
	// after every policy decision (accept, redact, reject). Attrs schema:
	//   "verdict":      string  (one of VerdictAccept/Redact/Reject)
	//   "input_kind":   coremem.Kind
	//   "decided_kind": coremem.Kind  (mirrors input_kind on reject)
	//   "source":       string  (the WriteSource of the ProposedWrite)
	//   "reason":       string  (the WritePolicyDecision.Reason)
	EventWritePolicyDecided = "memory_write_policy_decided"
)

// emit is the no-op-guarded internal emitter used by every Observer
// call site in this package. A nil Observer is the documented
// zero-config path — emit returns immediately. Otherwise the event is
// constructed (zero-allocation for nil Attrs) and forwarded.
//
// If Observer.OnEvent panics, the panic propagates to the caller —
// emit does NOT recover. Observers MUST NOT panic (see Observer
// godoc); recover/log/drop wrappers should be implemented at the
// adapter layer.
func emit(o Observer, name string, attrs map[string]any) {
	if o == nil {
		return
	}
	o.OnEvent(Event{Name: name, Attrs: attrs})
}

// Option is the functional-option type used by the constructors
// in this package (NewScopedLifecycleManager, NewConsolidator,
// NewUnifiedSearcher; NewParallelSearcher is added in Phase B-3,
// Task 11). All options are backwards-compatible additions; an
// empty option list is the documented zero-config behavior.
type Option func(*config)

// config is the internal shared config struct accumulated by the
// variadic option list. Today it carries only an Observer; future
// options (e.g. WithSerialSearch) extend this struct.
type config struct {
	observer Observer
}

// WithObserver installs the given Observer on the constructed wrapper.
// A nil Observer is treated as the zero-config no-op and elides the
// emit call entirely. If WithObserver is supplied more than once, the
// last call wins (standard functional-options semantics).
func WithObserver(o Observer) Option {
	return func(c *config) { c.observer = o }
}

// newConfig is the shared option-folding helper used by every
// constructor in this package. It always returns a non-nil *config —
// emit-site code may dereference cfg.observer unconditionally without
// nil-checking cfg itself.
func newConfig(opts []Option) *config {
	c := &config{}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// B2Status documents the Phase B-2 optimization status. The optimization
// (reuse the Working.Add embedding inside evictIfOverCapacity) lives
// inside coremem.WorkingMemory's private call site
// (llm-agent/memory/working.go:56-63 and 179-185) and cannot be applied
// from this sibling without modifying core. This package therefore
// tracks B-2 as deferred to an upstream PR.
//
// The observable property B-2 promises to preserve — capacity-full
// eviction picks the lowest-scored item — is pinned by the regression
// test TestObserver_B2_WorkingEvictionStillPicksLowestScoredItem so
// the eventual upstream PR cannot cause a silent regression.
//
// Status: pending upstream issue at github.com/costa92/llm-agent
// referencing working.go:56-63. File the issue at M2 kick-off.
const B2Status = "deferred-to-core"
