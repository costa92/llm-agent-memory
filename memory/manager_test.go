package memory

import (
	"context"
	"errors"
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

func TestNewManager_AllTiersNil_ReturnsErrNoTiers(t *testing.T) {
	_, err := NewManager(Options{})
	if !errors.Is(err, ErrNoTiers) {
		t.Fatalf("NewManager(empty) err = %v, want errors.Is ErrNoTiers", err)
	}
}

func TestNewManager_AtLeastOneTier_Succeeds(t *testing.T) {
	w := newCoreWorking(t)
	mgr, err := NewManager(Options{Working: TierOptions{Memory: w}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewManager returned nil mgr")
	}
}

func TestManager_HasKind_ReportsActiveTiers(t *testing.T) {
	w := newCoreWorking(t)
	mgr, err := NewManager(Options{Working: TierOptions{Memory: w}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if !mgr.HasKind(coremem.KindWorking) {
		t.Error("HasKind(Working) = false, want true")
	}
	if mgr.HasKind(coremem.KindEpisodic) {
		t.Error("HasKind(Episodic) = true, want false")
	}
}

func TestManager_Add_DispatchesToWiredTier(t *testing.T) {
	w := newCoreWorking(t)
	mgr, err := NewManager(Options{Working: TierOptions{Memory: w}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	id, err := mgr.Add(context.Background(), coremem.KindWorking, coremem.MemoryItem{Content: "hello"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id == "" {
		t.Fatal("Add returned empty id")
	}
	got, err := w.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get on underlying tier: %v", err)
	}
	if got.Content != "hello" {
		t.Errorf("got.Content = %q, want %q", got.Content, "hello")
	}
}

func TestManager_Add_DisabledKind_ReturnsErrTierDisabled(t *testing.T) {
	w := newCoreWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	_, err := mgr.Add(context.Background(), coremem.KindEpisodic, coremem.MemoryItem{Content: "x"})
	if !errors.Is(err, ErrTierDisabled) {
		t.Errorf("Add to disabled kind err = %v, want errors.Is ErrTierDisabled", err)
	}
	if !errors.Is(err, coremem.ErrKindDisabled) {
		t.Errorf("Add to disabled kind err = %v, want errors.Is coremem.ErrKindDisabled (compat)", err)
	}
}
