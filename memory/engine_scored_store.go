package memory

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	contractmem "github.com/costa92/llm-agent-contract/memory"
)

var (
	ErrNotFound         = contractmem.ErrNotFound
	ErrEmptyQuery       = contractmem.ErrEmptyQuery
	ErrEmbedderRequired = contractmem.ErrEmbedderRequired
)

type scoredStore struct {
	mu       sync.RWMutex
	items    map[string]MemoryItem
	vectors  map[string][]float32
	embedder Embedder
	prefix   string
	seq      int
}

func newScoredStore(prefix string, e Embedder) *scoredStore {
	return &scoredStore{
		items:    make(map[string]MemoryItem),
		vectors:  make(map[string][]float32),
		embedder: e,
		prefix:   prefix,
	}
}

func (s *scoredStore) add(ctx context.Context, item MemoryItem) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == "" {
		s.seq++
		item.ID = fmt.Sprintf("%s_%d_%d", s.prefix, time.Now().UnixNano(), s.seq)
	}
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.AccessedAt = now
	if item.Importance < 0 {
		item.Importance = 0
	}
	if item.Importance > 1 {
		item.Importance = 1
	}
	vec, err := queryEmbedding(ctx, s.embedder, item.Content)
	if err != nil {
		return "", fmt.Errorf("memory: embed: %w", err)
	}
	s.items[item.ID] = item
	s.vectors[item.ID] = vec
	return item.ID, nil
}

func (s *scoredStore) get(id string) (MemoryItem, error) {
	s.mu.RLock()
	item, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return MemoryItem{}, ErrNotFound
	}

	s.mu.Lock()
	if current, ok := s.items[id]; ok {
		current.AccessedAt = time.Now().UTC()
		s.items[id] = current
		item = current
	}
	s.mu.Unlock()
	return item, nil
}

func (s *scoredStore) update(ctx context.Context, id string, fn func(*MemoryItem)) error {
	s.mu.Lock()
	item, ok := s.items[id]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	prevContent := item.Content
	fn(&item)
	item.AccessedAt = time.Now().UTC()
	if item.Importance < 0 {
		item.Importance = 0
	}
	if item.Importance > 1 {
		item.Importance = 1
	}
	contentChanged := item.Content != prevContent
	s.items[id] = item
	s.mu.Unlock()

	if contentChanged {
		vec, err := queryEmbedding(ctx, s.embedder, item.Content)
		if err != nil {
			return fmt.Errorf("memory: re-embed: %w", err)
		}
		s.mu.Lock()
		s.vectors[id] = vec
		s.mu.Unlock()
	}
	return nil
}

func (s *scoredStore) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	delete(s.vectors, id)
	return nil
}

func (s *scoredStore) snapshot() (map[string]MemoryItem, map[string][]float32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	itemsCopy := make(map[string]MemoryItem, len(s.items))
	vecsCopy := make(map[string][]float32, len(s.vectors))
	for k, v := range s.items {
		itemsCopy[k] = v
	}
	for k, v := range s.vectors {
		cp := make([]float32, len(v))
		copy(cp, v)
		vecsCopy[k] = cp
	}
	return itemsCopy, vecsCopy
}

func (s *scoredStore) stats(capacity int) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := Stats{Count: len(s.items), Capacity: capacity}
	if len(s.items) == 0 {
		return out
	}
	var importanceSum float64
	oldest := time.Now()
	for _, it := range s.items {
		importanceSum += it.Importance
		if it.CreatedAt.Before(oldest) {
			oldest = it.CreatedAt
		}
	}
	out.AvgImportance = importanceSum / float64(len(s.items))
	out.OldestAge = time.Since(oldest)
	return out
}

func keywordScore(query, content string) float64 {
	q := strings.ToLower(query)
	c := strings.ToLower(content)
	tokens := splitTokens(q)
	if len(tokens) == 0 {
		return 0
	}
	hits := 0
	seen := map[string]bool{}
	for _, t := range tokens {
		if seen[t] {
			continue
		}
		seen[t] = true
		if strings.Contains(c, t) {
			hits++
		}
	}
	return float64(hits) / float64(len(seen))
}

func tagOverlap(queryTags, itemTags []string) float64 {
	if len(queryTags) == 0 {
		return 0
	}
	set := make(map[string]bool, len(itemTags))
	for _, t := range itemTags {
		set[strings.ToLower(t)] = true
	}
	hits := 0
	for _, t := range queryTags {
		if set[strings.ToLower(t)] {
			hits++
		}
	}
	return float64(hits) / float64(len(queryTags))
}

func importanceMultiplier(imp float64) float64 {
	return 0.8 + imp*0.4
}

func savedBoostMultiplier(it MemoryItem, boost float64) float64 {
	if boost <= 0 {
		boost = 1.0
	}
	if IsPinned(it) || GetSource(it) == SourceUserSaved {
		return boost
	}
	return 1.0
}

func timeDecay(createdAt time.Time, halfLife time.Duration) float64 {
	age := time.Since(createdAt)
	if age <= 0 || halfLife <= 0 {
		return 1
	}
	return math.Exp(-float64(age) / float64(halfLife))
}

func splitTokens(s string) []string {
	out := make([]string, 0, len(s)/4)
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func queryEmbedding(ctx context.Context, e Embedder, query string) ([]float32, error) {
	vectors, _, err := e.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}
	return vectors[0], nil
}

func vectorScore(qv, iv []float32) float64 {
	if len(qv) == 0 || len(iv) == 0 {
		return 0
	}
	return vectorCosineSimilarity(qv, iv)
}

func vectorCosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		normA += af * af
		normB += bf * bf
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
