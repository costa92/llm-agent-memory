package memory

import (
	"context"
	"fmt"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

func TestConsolidator_FirstPromote_WritesDedupeMetadata(t *testing.T) {
	mgr := newCoreManager(t)
	c, err := NewConsolidator(mgr)
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}

	ctx := context.Background()
	id, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{
		Content: "important note", Importance: 0.9,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	n, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 1 {
		t.Fatalf("promoted = %d, want 1", n)
	}

	// Source item must now carry the dedupe metadata keys.
	src, err := mgr.Get(ctx, coremem.KindWorking, id)
	if err != nil {
		t.Fatalf("Get source: %v", err)
	}
	if _, ok := src.Metadata[MetaKeyConsolidatedAt]; !ok {
		t.Errorf("Metadata[%q] missing", MetaKeyConsolidatedAt)
	}
	if got, _ := src.Metadata[MetaKeyPromotionCount].(int); got != 1 {
		t.Errorf("Metadata[%q] = %v, want int 1", MetaKeyPromotionCount, src.Metadata[MetaKeyPromotionCount])
	}

	// The episodic clone must carry the back-reference.
	pages, _ := mgr.ListAll(ctx, coremem.ListFilter{}, 100, nil)
	epi := pages[coremem.KindEpisodic].Items
	if len(epi) != 1 {
		t.Fatalf("episodic count = %d, want 1", len(epi))
	}
	if got, _ := epi[0].Metadata[MetaKeyPromotedFrom].(string); got != id {
		t.Errorf("episodic Metadata[%q] = %q, want %q", MetaKeyPromotedFrom, got, id)
	}
}

func TestConsolidator_SecondCall_DoesNotRePromote(t *testing.T) {
	mgr := newCoreManager(t)
	c, err := NewConsolidator(mgr)
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}

	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{
		Content: "promote me once", Importance: 0.9,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	n1, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate #1: %v", err)
	}
	if n1 != 1 {
		t.Fatalf("Consolidate #1 promoted = %d, want 1", n1)
	}

	n2, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate #2: %v", err)
	}
	if n2 != 0 {
		t.Errorf("Consolidate #2 promoted = %d, want 0 (promote-once policy)", n2)
	}

	// Episodic must still hold exactly one copy.
	pages, _ := mgr.ListAll(ctx, coremem.ListFilter{}, 100, nil)
	if got := len(pages[coremem.KindEpisodic].Items); got != 1 {
		t.Errorf("episodic count = %d, want 1 (no duplicate)", got)
	}
}

func TestConsolidator_DedupeMetadata_RoundTripsThroughExportImport(t *testing.T) {
	// Build mgr A, promote once, export Working snapshot, import into a
	// fresh mgr B, then assert: (a) the source still carries the dedupe
	// metadata, and (b) re-running Consolidate on mgr B is a no-op.
	mgrA := newCoreManager(t)
	c, err := NewConsolidator(mgrA)
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}

	ctx := context.Background()
	if _, err := mgrA.Add(ctx, coremem.KindWorking, coremem.MemoryItem{
		Content: "ride-along", Importance: 0.9,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7}); err != nil {
		t.Fatalf("Consolidate: %v", err)
	}

	snaps, err := mgrA.ExportAll(ctx, "")
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	workingSnap, ok := snaps[coremem.KindWorking]
	if !ok {
		t.Fatalf("working snapshot missing")
	}

	// Force a JSON round-trip so map[string]any types reflect what an
	// over-the-wire reload would produce.
	roundTripped := jsonRoundTripSnap(t, workingSnap)

	mgrB := newCoreManager(t)
	rpt, err := mgrB.ImportAll(ctx, map[coremem.Kind]coremem.Snapshot{
		coremem.KindWorking: roundTripped,
	}, "", coremem.ImportReplace)
	if err != nil {
		t.Fatalf("ImportAll: %v", err)
	}
	if rpt[coremem.KindWorking].Loaded != 1 {
		t.Fatalf("Loaded = %d, want 1", rpt[coremem.KindWorking].Loaded)
	}

	// Re-run Consolidate on mgr B — must be a no-op because the
	// imported source item still carries MetaKeyPromotionCount == 1.
	cB, err := NewConsolidator(mgrB)
	if err != nil {
		t.Fatalf("NewConsolidator B: %v", err)
	}
	n, err := cB.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate B: %v", err)
	}
	if n != 0 {
		t.Errorf("Consolidate after import promoted = %d, want 0 (metadata must survive round-trip)", n)
	}
}

func TestConsolidator_Consolidate_PagesThroughLargeWorkingSet(t *testing.T) {
	mgr, err := coremem.NewManager(coremem.ManagerOptions{
		Working:  newCoreWorkingWithCapacity(t, 256),
		Episodic: newCoreEpisodic(t),
		Semantic: newCoreSemantic(t),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	c, err := NewConsolidator(mgr)
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}

	ctx := context.Background()
	const total = 175
	for i := 0; i < total; i++ {
		if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{
			Content:    fmt.Sprintf("paged-%03d", i),
			Importance: 0.9,
		}); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}

	n, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7})
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != total {
		t.Fatalf("Consolidate promoted = %d, want %d (pagination dropped %d)", n, total, total-n)
	}
}

func TestConsolidator_Consolidate_EmitsConsolidatedTotalEvent(t *testing.T) {
	rec := &recordingObserver{}
	c, err := NewConsolidator(newCoreManager(t), WithObserver(rec))
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}
	ctx := context.Background()
	mgr := c.mgr
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "x", Importance: 0.9}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "y", Importance: 0.9}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7}); err != nil {
		t.Fatalf("Consolidate: %v", err)
	}

	got := rec.snapshot()
	var found *Event
	for i := range got {
		if got[i].Name == EventConsolidatedTotal {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no %q event emitted (events: %v)", EventConsolidatedTotal, got)
	}
	if n, _ := found.Attrs["n"].(int); n != 2 {
		t.Errorf("event Attrs[\"n\"] = %v, want 2", found.Attrs["n"])
	}
}

func TestConsolidator_Consolidate_EmitsAddTotalPerPromotion(t *testing.T) {
	rec := &recordingObserver{}
	mgr := newCoreManager(t)
	c, err := NewConsolidator(mgr, WithObserver(rec))
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}
	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "p1", Importance: 0.9}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "p2", Importance: 0.9}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := c.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7}); err != nil {
		t.Fatalf("Consolidate: %v", err)
	}

	addCount := 0
	for _, e := range rec.snapshot() {
		if e.Name == EventAddTotal {
			addCount++
			if k, _ := e.Attrs["kind"].(coremem.Kind); k != coremem.KindEpisodic {
				t.Errorf("EventAddTotal kind = %v, want %v", e.Attrs["kind"], coremem.KindEpisodic)
			}
		}
	}
	if addCount != 2 {
		t.Errorf("EventAddTotal count = %d, want 2 (one per promoted item)", addCount)
	}
}

func TestConsolidator_ExportAll_EmitsSnapshotItemsAndVectorBytes(t *testing.T) {
	rec := &recordingObserver{}
	mgr := newCoreManager(t)
	c, err := NewConsolidator(mgr, WithObserver(rec))
	if err != nil {
		t.Fatalf("NewConsolidator: %v", err)
	}
	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindEpisodic, coremem.MemoryItem{Content: "snap-me", Importance: 0.5}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	snaps, err := c.ExportAll(ctx, "")
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(snaps[coremem.KindEpisodic].Items) != 1 {
		t.Fatalf("expected 1 episodic snapshot item, got %d", len(snaps[coremem.KindEpisodic].Items))
	}

	var items, bytes *Event
	for _, e := range rec.snapshot() {
		switch e.Name {
		case EventSnapshotItems:
			if k, _ := e.Attrs["kind"].(coremem.Kind); k == coremem.KindEpisodic {
				items = &e
			}
		case EventSnapshotVectorsBytes:
			if k, _ := e.Attrs["kind"].(coremem.Kind); k == coremem.KindEpisodic {
				bytes = &e
			}
		}
	}
	if items == nil {
		t.Errorf("no %q event for KindEpisodic", EventSnapshotItems)
	} else if n, _ := items.Attrs["n"].(int); n != 1 {
		t.Errorf("snapshot items n = %v, want 1", items.Attrs["n"])
	}
	if bytes == nil {
		t.Errorf("no %q event for KindEpisodic", EventSnapshotVectorsBytes)
	} else if b, _ := bytes.Attrs["bytes"].(int); b <= 0 {
		t.Errorf("vector bytes = %v, want > 0", bytes.Attrs["bytes"])
	}
}
