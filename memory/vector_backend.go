package memory

import "context"

// VectorCandidate is a backend-produced similarity candidate for a memory item.
type VectorCandidate struct {
	Item       MemoryItem
	Similarity float64
}

// VectorBackend abstracts vector candidate generation so read-mostly memories
// can swap in non-scoredStore search implementations without changing their
// ranking logic.
type VectorBackend interface {
	Candidates(ctx context.Context, query string, items map[string]MemoryItem, vectors map[string][]float32, embedder Embedder) ([]VectorCandidate, error)
}

type inMemoryVectorBackend struct{}

func (inMemoryVectorBackend) Candidates(ctx context.Context, query string, items map[string]MemoryItem, vectors map[string][]float32, embedder Embedder) ([]VectorCandidate, error) {
	qv, err := queryEmbedding(ctx, embedder, query)
	if err != nil {
		return nil, err
	}

	out := make([]VectorCandidate, 0, len(items))
	for id, item := range items {
		out = append(out, VectorCandidate{
			Item:       item,
			Similarity: vectorScore(qv, vectors[id]),
		})
	}
	return out, nil
}
