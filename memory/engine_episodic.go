package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type EpisodicMemory struct {
	store *scoredStore
	opts  EpisodicOptions
}

type EpisodicOptions struct {
	RecencyHalfLifeDays float64
	SavedBoost          float64
	VectorBackend       VectorBackend
}

func NewEpisodic(e Embedder, opts EpisodicOptions) (*EpisodicMemory, error) {
	if e == nil {
		return nil, ErrEmbedderRequired
	}
	if opts.RecencyHalfLifeDays <= 0 {
		opts.RecencyHalfLifeDays = 30
	}
	if opts.VectorBackend == nil {
		opts.VectorBackend = inMemoryVectorBackend{}
	}
	return &EpisodicMemory{store: newScoredStore("epi", e), opts: opts}, nil
}

func (m *EpisodicMemory) Type() Kind { return KindEpisodic }

func (m *EpisodicMemory) Add(ctx context.Context, item MemoryItem) (string, error) {
	return m.store.add(ctx, item)
}

func (m *EpisodicMemory) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}
	if topK <= 0 {
		topK = 5
	}
	items, vecs := m.store.snapshot()
	candidates, err := m.opts.VectorBackend.Candidates(ctx, query, items, vecs, m.store.embedder)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(items))
	halfLife := time.Duration(m.opts.RecencyHalfLifeDays * 24 * float64(time.Hour))
	for _, candidate := range candidates {
		it := candidate.Item
		if IsDisabled(it) {
			continue
		}
		recency := timeDecay(it.CreatedAt, halfLife)
		score := (candidate.Similarity*0.8 + recency*0.2) * importanceMultiplier(it.Importance) *
			savedBoostMultiplier(it, m.opts.SavedBoost)
		out = append(out, SearchResult{Item: it, Score: score})
	}
	sortDesc(out)
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

func (m *EpisodicMemory) Get(_ context.Context, id string) (MemoryItem, error) {
	return m.store.get(id)
}

func (m *EpisodicMemory) Update(ctx context.Context, id string, fn func(*MemoryItem)) error {
	return m.store.update(ctx, id, fn)
}

func (m *EpisodicMemory) Remove(_ context.Context, id string) error {
	return m.store.remove(id)
}

func (m *EpisodicMemory) Stats() Stats {
	return m.store.stats(0)
}

func (m *EpisodicMemory) List(_ context.Context, filter ListFilter, pageSize int, cursor string) (ListPage, error) {
	return listFromStore(m.store, filter, pageSize, cursor)
}

func (m *EpisodicMemory) Export(_ context.Context) (Snapshot, error) {
	return exportFromStore(m.store, KindEpisodic), nil
}

func (m *EpisodicMemory) Import(_ context.Context, snap Snapshot, mode ImportMode) (ImportReport, error) {
	if snap.Kind != KindEpisodic {
		return ImportReport{}, fmt.Errorf("%w: got %s, want episodic", ErrSnapshotKindMismatch, snap.Kind)
	}
	return importIntoStore(m.store, snap, mode)
}

func RestoreEpisodic(e Embedder, snap Snapshot, opts EpisodicOptions) (*EpisodicMemory, error) {
	if e == nil {
		return nil, ErrEmbedderRequired
	}
	m, err := NewEpisodic(e, opts)
	if err != nil {
		return nil, err
	}
	if _, err := m.Import(context.Background(), snap, ImportReplace); err != nil {
		return nil, err
	}
	return m, nil
}
