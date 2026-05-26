package memory

import (
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

// TestManager_TierOptions_FieldsAreCapabilityInterfaces is a
// compile-time assertion. If TierOptions ever loses an interface field
// or starts carrying a concrete type, this test will fail to compile —
// the exact signal we want.
//
// The D-1 exit criterion is: ManagerOptions fields typed as interfaces
// (Memory, Lister, Exporter, Importer, optional LifecycleMemory). The
// sibling-owned Options.<Tier> is the carrier; this test pins the
// shape.
func TestManager_TierOptions_FieldsAreCapabilityInterfaces(t *testing.T) {
	// One TierOptions per kind. Every field is an interface — the
	// composite literal succeeds only if the types match.
	var (
		_ coremem.Memory   = (TierOptions{}).Memory
		_ coremem.Lister   = (TierOptions{}).Lister
		_ coremem.Exporter = (TierOptions{}).Exporter
		_ coremem.Importer = (TierOptions{}).Importer
		_ LifecycleMemory  = (TierOptions{}).Lifecycle
	)

	// Options carries three TierOptions plus a SnapshotStore plus an
	// optional *coremem.Manager escape hatch (see "Open Decisions
	// Resolved").
	opts := Options{
		Working:       TierOptions{},
		Episodic:      TierOptions{},
		Semantic:      TierOptions{},
		SnapshotStore: nil,
		CoreManager:   nil,
	}
	if opts.Working.Memory != nil || opts.Episodic.Memory != nil || opts.Semantic.Memory != nil {
		t.Errorf("Options zero-value has non-nil capability fields: %+v", opts)
	}
}
