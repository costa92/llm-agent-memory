package memory

import (
	"context"
	"testing"
)

type recordingVectorBackend struct {
	called int
}

func (r *recordingVectorBackend) Candidates(_ context.Context, _ string, items map[string]MemoryItem, _ map[string][]float32, _ Embedder) ([]VectorCandidate, error) {
	r.called++
	out := make([]VectorCandidate, 0, len(items))
	for _, item := range items {
		score := 0.05
		if item.Content == "chosen episodic memory" || item.Content == "chosen semantic memory" {
			score = 0.95
		}
		out = append(out, VectorCandidate{Item: item, Similarity: score})
	}
	return out, nil
}

func TestM8CVectorBackend_EpisodicSearchUsesConfiguredBackend(t *testing.T) {
	ctx := context.Background()
	backend := &recordingVectorBackend{}

	mem, err := NewEpisodic(newEmbedder(), EpisodicOptions{VectorBackend: backend})
	if err != nil {
		t.Fatalf("NewEpisodic: %v", err)
	}
	if _, err := mem.Add(ctx, MemoryItem{Content: "other episodic memory"}); err != nil {
		t.Fatalf("Add other: %v", err)
	}
	if _, err := mem.Add(ctx, MemoryItem{Content: "chosen episodic memory"}); err != nil {
		t.Fatalf("Add chosen: %v", err)
	}

	hits, err := mem.Search(ctx, "ignored-by-backend", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if backend.called != 1 {
		t.Fatalf("backend called %d times, want 1", backend.called)
	}
	if len(hits) == 0 || hits[0].Item.Content != "chosen episodic memory" {
		t.Fatalf("top hit = %#v, want chosen episodic memory", hits)
	}
}

func TestM8CVectorBackend_SemanticSearchUsesConfiguredBackend(t *testing.T) {
	ctx := context.Background()
	backend := &recordingVectorBackend{}

	mem, err := NewSemantic(newEmbedder(), SemanticOptions{VectorBackend: backend})
	if err != nil {
		t.Fatalf("NewSemantic: %v", err)
	}
	if _, err := mem.Add(ctx, MemoryItem{Content: "other semantic memory", Tags: []string{"x"}}); err != nil {
		t.Fatalf("Add other: %v", err)
	}
	if _, err := mem.Add(ctx, MemoryItem{Content: "chosen semantic memory", Tags: []string{"x"}}); err != nil {
		t.Fatalf("Add chosen: %v", err)
	}

	hits, err := mem.Search(ctx, "tag:x ignored-by-backend", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if backend.called != 1 {
		t.Fatalf("backend called %d times, want 1", backend.called)
	}
	if len(hits) == 0 || hits[0].Item.Content != "chosen semantic memory" {
		t.Fatalf("top hit = %#v, want chosen semantic memory", hits)
	}
}
