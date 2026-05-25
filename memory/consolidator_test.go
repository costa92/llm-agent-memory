package memory

import (
	"context"
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
