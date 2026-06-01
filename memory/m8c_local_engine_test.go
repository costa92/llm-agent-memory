package memory

import (
	"context"
	"testing"
)

func TestM8CLocalEngine_WorkingManagerRecallRoundTrip(t *testing.T) {
	ctx := context.Background()

	w, err := NewWorking(newCoreEmbedder(), WorkingOptions{
		Capacity: 16,
	})
	if err != nil {
		t.Fatalf("NewWorking: %v", err)
	}

	mgr, err := NewManager(Options{
		Working: TierOptions{
			Memory:   w,
			Lister:   w,
			Exporter: w,
			Importer: w,
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	id, err := mgr.Add(ctx, KindWorking, MemoryItem{
		Content:    "local engine benchmark memory",
		Importance: 0.8,
	})
	if err != nil {
		t.Fatalf("Manager.Add: %v", err)
	}
	if id == "" {
		t.Fatal("Manager.Add returned empty id")
	}

	got, err := mgr.Get(ctx, KindWorking, id)
	if err != nil {
		t.Fatalf("Manager.Get: %v", err)
	}
	if got.Content != "local engine benchmark memory" {
		t.Fatalf("Content = %q, want local engine benchmark memory", got.Content)
	}

	engine, err := NewRecallEngine(mgr)
	if err != nil {
		t.Fatalf("NewRecallEngine: %v", err)
	}
	recall, err := engine.Recall(ctx, "benchmark", RecallOptions{TopK: 3})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(recall.Results) == 0 {
		t.Fatal("Recall returned 0 results, want at least 1")
	}
	if recall.Results[0].Item.Content != "local engine benchmark memory" {
		t.Fatalf("Recall content = %q, want local engine benchmark memory", recall.Results[0].Item.Content)
	}
}
