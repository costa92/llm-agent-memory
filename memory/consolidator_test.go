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
