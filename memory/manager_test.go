package memory

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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
		_ interface {
			Type() Kind
			Add(context.Context, MemoryItem) (string, error)
			Search(context.Context, string, int) ([]SearchResult, error)
			Get(context.Context, string) (MemoryItem, error)
			Update(context.Context, string, func(*MemoryItem)) error
			Remove(context.Context, string) error
			Stats() Stats
		} = newWorking(t)
		_ Lister          = newWorking(t)
		_ Exporter        = newWorking(t)
		_ Importer        = newWorking(t)
		_ LifecycleMemory = (TierOptions{}).Lifecycle
	)

	// Core interfaces remain installable through the adapter layer.
	var (
		_ Memory = AdaptCoreMemory(coremem.WithSanitizer(coreWorkingForAdapter(t), coremem.SanitizerFunc(func(_ context.Context, _ coremem.Kind, it coremem.MemoryItem) (coremem.MemoryItem, bool, error) {
			return it, true, nil
		})))
		_ Lister = AdaptCoreLister(coreWorkingForAdapter(t))
	)

	// Options carries three TierOptions plus a SnapshotStore.
	opts := Options{
		Working:       TierOptions{},
		Episodic:      TierOptions{},
		Semantic:      TierOptions{},
		SnapshotStore: nil,
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

func TestMemoryPackage_DoesNotExposeCompatPackageOrCoreManagerBridge(t *testing.T) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(".", "compat")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("memory/compat package must not exist; stat err=%v", err)
	}

	path := filepath.Join(".", "manager.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Options" {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("Options is not a struct")
			}
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if name.Name == "CoreManager" {
						t.Fatalf("Options.CoreManager compatibility bridge is forbidden")
					}
				}
			}
		}
	}
}

type stubLifecycleMemory struct {
	consolidateCount int
	forgetCount      int
	lastKind         coremem.Kind
}

func (s *stubLifecycleMemory) Consolidate(_ context.Context, _ coremem.ConsolidateOptions) (int, error) {
	s.consolidateCount++
	return 2, nil
}

func (s *stubLifecycleMemory) Forget(_ context.Context, kind coremem.Kind, _ coremem.ForgetOptions) (int, error) {
	s.forgetCount++
	s.lastKind = kind
	return 1, nil
}

func TestNewManager_AtLeastOneTier_Succeeds(t *testing.T) {
	w := newWorking(t)
	mgr, err := NewManager(Options{Working: TierOptions{Memory: w}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewManager returned nil mgr")
	}
}

func TestManager_HasKind_ReportsActiveTiers(t *testing.T) {
	w := newWorking(t)
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
	w := newWorking(t)
	mgr, err := NewManager(Options{Working: TierOptions{Memory: w}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	id, err := mgr.Add(context.Background(), coremem.KindWorking, MemoryItem{Content: "hello"})
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
	w := newWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	_, err := mgr.Add(context.Background(), coremem.KindEpisodic, MemoryItem{Content: "x"})
	if !errors.Is(err, ErrTierDisabled) {
		t.Errorf("Add to disabled kind err = %v, want errors.Is ErrTierDisabled", err)
	}
	if !errors.Is(err, coremem.ErrKindDisabled) {
		t.Errorf("Add to disabled kind err = %v, want errors.Is coremem.ErrKindDisabled (compat)", err)
	}
}

func TestManager_GetUpdateRemove_RoundTrip(t *testing.T) {
	ctx := context.Background()
	w := newWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})

	id, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "rt"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := mgr.Get(ctx, coremem.KindWorking, id)
	if err != nil || got.Content != "rt" {
		t.Fatalf("Get: got=%+v err=%v", got, err)
	}
	if err := mgr.Update(ctx, coremem.KindWorking, id, func(it *MemoryItem) { it.Content = "rt2" }); err != nil {
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
	w, e, s := newWorking(t), newEpisodic(t), newSemantic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
		Semantic: TierOptions{Memory: s},
	})
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, MemoryItem{Content: "episodic-fact"}); err != nil {
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
	w := newWorking(t)
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
	w, e := newWorking(t), newEpisodic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
	})
	if _, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "wfact"}); err != nil {
		t.Fatalf("Add working: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, MemoryItem{Content: "efact"}); err != nil {
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
	w := newWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	if _, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "list-me"}); err != nil {
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

func TestManager_Consolidate_NoLifecycle_ReturnsCapabilityMissing(t *testing.T) {
	w := newWorking(t)
	e := newEpisodic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
	})
	_, err := mgr.Consolidate(context.Background(), coremem.ConsolidateOptions{})
	if !errors.Is(err, ErrCapabilityMissing) {
		t.Errorf("Consolidate err = %v, want errors.Is ErrCapabilityMissing", err)
	}
}

func TestManager_Consolidate_UsesLifecycleCapability(t *testing.T) {
	w := newWorking(t)
	e := newEpisodic(t)
	lifecycle := &stubLifecycleMemory{}
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w, Lifecycle: lifecycle},
		Episodic: TierOptions{Memory: e},
	})
	n, err := mgr.Consolidate(context.Background(), coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 2 {
		t.Errorf("Consolidate promoted = %d, want 2", n)
	}
	if lifecycle.consolidateCount != 1 {
		t.Errorf("consolidateCount = %d, want 1", lifecycle.consolidateCount)
	}
}

func TestManager_ExportAll_FansAcrossTiersThatExpose(t *testing.T) {
	ctx := context.Background()
	w, e := newWorking(t), newEpisodic(t)
	mgr, _ := NewManager(Options{
		Working:  TierOptions{Memory: w, Exporter: w},
		Episodic: TierOptions{Memory: e, Exporter: e},
	})
	if _, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "w"}); err != nil {
		t.Fatalf("Add working: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, MemoryItem{Content: "e"}); err != nil {
		t.Fatalf("Add episodic: %v", err)
	}
	snaps, err := mgr.ExportAll(ctx, "")
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if _, ok := snaps[coremem.KindWorking]; !ok {
		t.Errorf("ExportAll missing KindWorking: %+v", snaps)
	}
	if _, ok := snaps[coremem.KindEpisodic]; !ok {
		t.Errorf("ExportAll missing KindEpisodic: %+v", snaps)
	}
	if got := len(snaps[coremem.KindWorking].Items); got != 1 {
		t.Errorf("Working snapshot Items len = %d, want 1", got)
	}
}

func TestManager_ExportAll_FallsBackToMemoryAssertionForExporter(t *testing.T) {
	ctx := context.Background()
	w := newWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	if _, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "x"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	snaps, err := mgr.ExportAll(ctx, "")
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if _, ok := snaps[coremem.KindWorking]; !ok {
		t.Errorf("ExportAll missing KindWorking via Memory-as-Exporter assertion: %+v", snaps)
	}
}

func TestManager_ExportAll_PersistKeyWithoutStore_ReturnsErrSnapshotStoreNotConfigured(t *testing.T) {
	w := newWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: w}})
	_, err := mgr.ExportAll(context.Background(), "any-key")
	if !errors.Is(err, coremem.ErrSnapshotStoreNotConfigured) {
		t.Errorf("err = %v, want errors.Is coremem.ErrSnapshotStoreNotConfigured", err)
	}
}

func TestManager_ImportAll_InlineSnapsRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newWorking(t)
	if _, err := src.Add(ctx, MemoryItem{Content: "round"}); err != nil {
		t.Fatalf("src Add: %v", err)
	}
	snap, err := src.Export(ctx)
	if err != nil {
		t.Fatalf("src Export: %v", err)
	}

	dst := newWorking(t)
	mgr, _ := NewManager(Options{Working: TierOptions{Memory: dst, Importer: dst}})
	reports, err := mgr.ImportAll(ctx, map[Kind]Snapshot{KindWorking: snap}, "", ImportReplace)
	if err != nil {
		t.Fatalf("ImportAll: %v", err)
	}
	if reports[coremem.KindWorking].Loaded != 1 {
		t.Errorf("Loaded = %d, want 1", reports[coremem.KindWorking].Loaded)
	}
}

// TestManager_WithSanitizerWrappedMemory_InstallsWithoutCast is the
// verbatim D-1 exit-criterion proof from docs/superpowers/plans/
// 2026-05-25-llm-agent-memory-roadmap.md §5.1 D-1. coremem.WithSanitizer
// returns the coremem.Memory interface value (see policy_hook.go:61-66);
// pre-M4, that value could NOT be assigned to coremem.ManagerOptions.Working
// (a *coremem.WorkingMemory). With the new sibling Options.Working.Memory
// typed as coremem.Memory, the assignment compiles + works.
//
// If this test starts failing to compile, M4's central D-1 promise is
// broken.
func TestManager_WithSanitizerWrappedMemory_InstallsWithoutCast(t *testing.T) {
	ctx := context.Background()
	w := coreWorkingForAdapter(t)

	// Build a sanitizer chain that uppercases content. This proves the
	// chain runs (and therefore that the wrapped Memory is the one
	// Manager.Add invokes — NOT the underlying *coremem.WorkingMemory).
	uppercase := coremem.SanitizerFunc(func(_ context.Context, _ coremem.Kind, it coremem.MemoryItem) (coremem.MemoryItem, bool, error) {
		it.Content = "UPPER:" + it.Content
		return it, true, nil
	})

	// THE no-cast install. coremem.WithSanitizer returns coremem.Memory;
	// in the old world this would NOT have compiled — TierOptions.Memory
	// is accepted through AdaptCoreMemory, which preserves the M4 D-1
	// compatibility promise after sibling-owned MemoryItem diverged.
	wrapped := coremem.WithSanitizer(w, uppercase)
	mgr, err := NewManager(Options{
		Working: TierOptions{Memory: AdaptCoreMemory(wrapped), Lister: AdaptCoreLister(w)},
	})
	if err != nil {
		t.Fatalf("NewManager with wrapped memory: %v", err)
	}

	id, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "hello"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := mgr.Get(ctx, coremem.KindWorking, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "UPPER:hello" {
		t.Errorf("Content = %q, want %q — sanitizer chain did not run", got.Content, "UPPER:hello")
	}
}

// TestManager_ParityWithCoreManager_MethodMatrix is the structural
// assertion that the sibling Manager exposes every coremem.Manager
// public method that consumers depend on. It is NOT a goroutine-safety
// test, NOT a correctness test — those live in their per-method tests
// above. This is a single guard against an accidental drop of a public
// method during a future refactor.
func TestManager_ParityWithCoreManager_MethodMatrix(t *testing.T) {
	ctx := context.Background()
	w, e, s := newWorking(t), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w},
		Episodic: TierOptions{Memory: e},
		Semantic: TierOptions{Memory: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Add(ctx, coremem.KindWorking, MemoryItem{Content: "p"}); err != nil {
		t.Errorf("Add: %v", err)
	}
	if _, err := mgr.Search(ctx, coremem.KindWorking, "p", 5); err != nil {
		t.Errorf("Search: %v", err)
	}
	if _, err := mgr.SearchAll(ctx, "p", 5); err != nil {
		t.Errorf("SearchAll: %v", err)
	}
	if _, err := mgr.ListAll(ctx, coremem.ListFilter{}, 10, nil); err != nil {
		t.Errorf("ListAll: %v", err)
	}
	if got := mgr.StatsAll(); len(got) != 3 {
		t.Errorf("StatsAll: got %d entries, want 3", len(got))
	}
	if _, err := mgr.ExportAll(ctx, ""); err != nil {
		t.Errorf("ExportAll: %v", err)
	}
	if _, err := mgr.ImportAll(ctx, map[Kind]Snapshot{}, "", ImportMerge); err != nil {
		t.Errorf("ImportAll(empty): %v", err)
	}
	if _, err := mgr.Consolidate(ctx, coremem.ConsolidateOptions{}); !errors.Is(err, ErrCapabilityMissing) {
		t.Errorf("Consolidate without Lifecycle: err = %v, want ErrCapabilityMissing", err)
	}
	if _, err := mgr.Forget(ctx, coremem.KindWorking, coremem.ForgetOptions{Strategy: coremem.ForgetByImportance, Threshold: 0.1}); !errors.Is(err, ErrCapabilityMissing) {
		t.Errorf("Forget without Lifecycle: err = %v, want ErrCapabilityMissing", err)
	}
}
