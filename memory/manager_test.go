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

func TestManager_GetUpdateRemove_RoundTrip(t *testing.T) {
	ctx := context.Background()
	w := newCoreWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})

	id, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "rt"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := mgr.Get(ctx, coremem.KindWorking, id)
	if err != nil || got.Content != "rt" {
		t.Fatalf("Get: got=%+v err=%v", got, err)
	}
	if err := mgr.Update(ctx, coremem.KindWorking, id, func(it *coremem.MemoryItem) { it.Content = "rt2" }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := mgr.Get(ctx, coremem.KindWorking, id)
	if got2.Content != "rt2" {
		t.Errorf("after Update, Content = %q, want %q", got2.Content, "rt2")
	}
	if err := mgr.Remove(ctx, coremem.KindWorking, id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := mgr.Get(ctx, coremem.KindWorking, id); !errors.Is(err, coremem.ErrNotFound) {
		t.Errorf("Get after Remove err = %v, want errors.Is ErrNotFound", err)
	}
}

func TestManager_Search_DispatchesToCorrectTier(t *testing.T) {
	ctx := context.Background()
	w, e, s := newCoreWorking(t), newCoreEpisodic(t), newCoreSemantic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
		Semantic: TierOptions{Memory: s},
	})
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "episodic-fact"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	res, err := mgr.Search(ctx, coremem.KindEpisodic, "episodic-fact", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("Search returned 0 results, want at least 1")
	}
	if res[0].Item.Content != "episodic-fact" {
		t.Errorf("res[0].Content = %q, want %q", res[0].Item.Content, "episodic-fact")
	}
}

func TestManager_Stats_OnlyActiveTiers(t *testing.T) {
	w := newCoreWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	stats := mgr.StatsAll()
	if _, ok := stats[coremem.KindWorking]; !ok {
		t.Errorf("stats missing KindWorking entry: %+v", stats)
	}
	if _, ok := stats[coremem.KindEpisodic]; ok {
		t.Errorf("stats has KindEpisodic but tier was not wired: %+v", stats)
	}
}

func TestManager_SearchAll_FansAcrossActiveTiers(t *testing.T) {
	ctx := context.Background()
	w, e := newCoreWorking(t), newCoreEpisodic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
	})
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "wfact"}); err != nil {
		t.Fatalf("Add working: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "efact"}); err != nil {
		t.Fatalf("Add episodic: %v", err)
	}
	got, err := mgr.SearchAll(ctx, "fact", 5)
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if _, ok := got[coremem.KindWorking]; !ok {
		t.Errorf("SearchAll missing KindWorking entry: %+v", got)
	}
	if _, ok := got[coremem.KindEpisodic]; !ok {
		t.Errorf("SearchAll missing KindEpisodic entry: %+v", got)
	}
	if _, ok := got[coremem.KindSemantic]; ok {
		t.Errorf("SearchAll includes KindSemantic but tier was not wired: %+v", got)
	}
}

func TestManager_ListAll_PrefersTierLister_FallsBackToMemoryAssertion(t *testing.T) {
	ctx := context.Background()
	w := newCoreWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "list-me"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	pages, err := mgr.ListAll(ctx, coremem.ListFilter{}, 10, nil)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	p := pages[coremem.KindWorking]
	if len(p.Items) != 1 || p.Items[0].Content != "list-me" {
		t.Errorf("ListAll Working page = %+v, want one item with Content=list-me", p)
	}
}

func TestManager_Consolidate_NoLifecycle_NoCoreManager_ReturnsCapabilityMissing(t *testing.T) {
	w := newCoreWorking(t)
	e := newCoreEpisodic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
	})
	_, err := mgr.Consolidate(context.Background(), coremem.ConsolidateOptions{})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Errorf("Consolidate err = %v, want errors.Is ErrCapabilityMissing", err)
	}
}

func TestManager_Consolidate_WithCoreManagerFallback_Succeeds(t *testing.T) {
	ctx := context.Background()
	w := newCoreWorking(t)
	e := newCoreEpisodic(t)
	coreMgr, _ := coremem.NewManager(coremem.ManagerOptions{Working: w, Episodic: e})
	mgr, _ := NewManager(Options{
		Working:     TierOptions{Memory: w},
		Episodic:    TierOptions{Memory: e},
		CoreManager: coreMgr,
	})
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "promote me", Importance: 0.9}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	n, err := mgr.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n < 1 {
		t.Errorf("Consolidate promoted = %d, want >= 1", n)
	}
}
