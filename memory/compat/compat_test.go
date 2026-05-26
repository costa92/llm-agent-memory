package compat

import (
	"context"
	"testing"
	"time"

	coremem "github.com/costa92/llm-agent/memory"
	"github.com/costa92/llm-agent/llm"
	"github.com/costa92/llm-agent-memory/memory"
)

// TestCompat_LegacyOptions_IsAliasOfCoreManagerOptions pins the
// type-alias relationship. Any drift between LegacyOptions and
// coremem.ManagerOptions breaks compilation here.
func TestCompat_LegacyOptions_IsAliasOfCoreManagerOptions(t *testing.T) {
	// Direct field set via composite literal in BOTH directions: pure
	// type-aliases let either side initialize the other.
	var asCore coremem.ManagerOptions = LegacyOptions{}
	var asLegacy LegacyOptions = coremem.ManagerOptions{}
	_ = asCore
	_ = asLegacy
}

// TestCompat_NewManagerFromCore_BridgesEverySurfaceMethod pins the
// bridge contract: a *coremem.Manager → *memory.Manager round-trip
// satisfies every method the sibling Manager exposes.
func TestCompat_NewManagerFromCore_BridgesEverySurfaceMethod(t *testing.T) {
	emb := llm.NewScriptedLLM(llm.WithEmbedDimensions(64))
	w, _ := coremem.NewWorking(emb, coremem.WorkingOptions{Capacity: 16, Decay: 24 * time.Hour})
	e, _ := coremem.NewEpisodic(emb, coremem.EpisodicOptions{})
	s, _ := coremem.NewSemantic(emb, coremem.SemanticOptions{})
	coreMgr, err := coremem.NewManager(coremem.ManagerOptions{Working: w, Episodic: e, Semantic: s})
	if err != nil {
		t.Fatalf("coremem.NewManager: %v", err)
	}

	mgr := NewManagerFromCore(coreMgr)
	if mgr == nil {
		t.Fatal("NewManagerFromCore returned nil")
	}
	ctx := context.Background()
	if _, err := mgr.Add(ctx, coremem.KindWorking, coremem.MemoryItem{Content: "bridged"}); err != nil {
		t.Errorf("Add via bridged mgr: %v", err)
	}
	if _, err := mgr.Search(ctx, coremem.KindWorking, "bridged", 5); err != nil {
		t.Errorf("Search via bridged mgr: %v", err)
	}
	// Consolidate works because we wired the CoreManager fallback inside
	// NewManagerFromCore.
	if _, err := mgr.Consolidate(ctx, coremem.ConsolidateOptions{Threshold: 0.7}); err != nil {
		t.Errorf("Consolidate via bridged mgr: %v", err)
	}
	// Ensure the bridged mgr is a *memory.Manager (the v1 surface).
	var _ *memory.Manager = mgr
}

// TestCompat_NewManagerFromLegacyOptions_AcceptsCoreShape pins the
// legacy-options bridge for the most common caller shape.
func TestCompat_NewManagerFromLegacyOptions_AcceptsCoreShape(t *testing.T) {
	emb := llm.NewScriptedLLM(llm.WithEmbedDimensions(64))
	w, _ := coremem.NewWorking(emb, coremem.WorkingOptions{})
	mgr, err := NewManagerFromLegacyOptions(LegacyOptions{Working: w})
	if err != nil {
		t.Fatalf("NewManagerFromLegacyOptions: %v", err)
	}
	if _, err := mgr.Add(context.Background(), coremem.KindWorking, coremem.MemoryItem{Content: "legacy"}); err != nil {
		t.Errorf("Add via legacy-options mgr: %v", err)
	}
}
