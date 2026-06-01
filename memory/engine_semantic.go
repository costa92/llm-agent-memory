package memory

import (
	"context"
	"fmt"
	"strings"
)

type SemanticMemory struct {
	store *scoredStore
	opts  SemanticOptions
}

type SemanticOptions struct {
	SavedBoost    float64
	VectorBackend VectorBackend
}

func NewSemantic(e Embedder, opts SemanticOptions) (*SemanticMemory, error) {
	if e == nil {
		return nil, ErrEmbedderRequired
	}
	if opts.VectorBackend == nil {
		opts.VectorBackend = inMemoryVectorBackend{}
	}
	return &SemanticMemory{store: newScoredStore("sem", e), opts: opts}, nil
}

func (m *SemanticMemory) Type() Kind { return KindSemantic }

func (m *SemanticMemory) Add(ctx context.Context, item MemoryItem) (string, error) {
	return m.store.add(ctx, item)
}

func (m *SemanticMemory) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}
	if topK <= 0 {
		topK = 5
	}
	queryText, queryTags := parseTagPrefix(query)
	items, vecs := m.store.snapshot()
	candidates, err := m.opts.VectorBackend.Candidates(ctx, queryText, items, vecs, m.store.embedder)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(candidates))
	for _, candidate := range candidates {
		it := candidate.Item
		if IsDisabled(it) {
			continue
		}
		if len(queryTags) > 0 && !anyTagMatches(queryTags, it.Tags) {
			continue
		}
		tag := tagOverlap(queryTags, it.Tags)
		score := (candidate.Similarity*0.7 + tag*0.3) * importanceMultiplier(it.Importance) *
			savedBoostMultiplier(it, m.opts.SavedBoost)
		out = append(out, SearchResult{Item: it, Score: score})
	}
	sortDesc(out)
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

func (m *SemanticMemory) Get(_ context.Context, id string) (MemoryItem, error) {
	return m.store.get(id)
}

func (m *SemanticMemory) Update(ctx context.Context, id string, fn func(*MemoryItem)) error {
	return m.store.update(ctx, id, fn)
}

func (m *SemanticMemory) Remove(_ context.Context, id string) error {
	return m.store.remove(id)
}

func (m *SemanticMemory) Stats() Stats {
	return m.store.stats(0)
}

func (m *SemanticMemory) List(_ context.Context, filter ListFilter, pageSize int, cursor string) (ListPage, error) {
	return listFromStore(m.store, filter, pageSize, cursor)
}

func (m *SemanticMemory) Export(_ context.Context) (Snapshot, error) {
	return exportFromStore(m.store, KindSemantic), nil
}

func (m *SemanticMemory) Import(_ context.Context, snap Snapshot, mode ImportMode) (ImportReport, error) {
	if snap.Kind != KindSemantic {
		return ImportReport{}, fmt.Errorf("%w: got %s, want semantic", ErrSnapshotKindMismatch, snap.Kind)
	}
	return importIntoStore(m.store, snap, mode)
}

func RestoreSemantic(e Embedder, snap Snapshot, opts SemanticOptions) (*SemanticMemory, error) {
	if e == nil {
		return nil, ErrEmbedderRequired
	}
	m, err := NewSemantic(e, opts)
	if err != nil {
		return nil, err
	}
	if _, err := m.Import(context.Background(), snap, ImportReplace); err != nil {
		return nil, err
	}
	return m, nil
}

func parseTagPrefix(query string) (string, []string) {
	const prefix = "tag:"
	q := strings.TrimSpace(query)
	if !strings.HasPrefix(q, prefix) {
		return query, nil
	}
	rest := q[len(prefix):]
	sp := strings.IndexByte(rest, ' ')
	if sp < 0 {
		return "", splitCSV(rest)
	}
	tagsCSV := rest[:sp]
	body := strings.TrimSpace(rest[sp+1:])
	return body, splitCSV(tagsCSV)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func anyTagMatches(want, have []string) bool {
	wantLower := make(map[string]bool, len(want))
	for _, t := range want {
		wantLower[strings.ToLower(t)] = true
	}
	for _, t := range have {
		if wantLower[strings.ToLower(t)] {
			return true
		}
	}
	return false
}
