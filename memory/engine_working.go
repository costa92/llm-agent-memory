package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type WorkingMemory struct {
	store *scoredStore
	opts  WorkingOptions
}

type WorkingOptions struct {
	Capacity   int
	Decay      time.Duration
	SavedBoost float64
}

func NewWorking(e Embedder, opts WorkingOptions) (*WorkingMemory, error) {
	if e == nil {
		return nil, ErrEmbedderRequired
	}
	if opts.Capacity <= 0 {
		opts.Capacity = 50
	}
	if opts.Decay <= 0 {
		opts.Decay = 24 * time.Hour
	}
	return &WorkingMemory{store: newScoredStore("wrk", e), opts: opts}, nil
}

func (w *WorkingMemory) Type() Kind { return KindWorking }

func (w *WorkingMemory) Add(ctx context.Context, item MemoryItem) (string, error) {
	id, err := w.store.add(ctx, item)
	if err != nil {
		return "", err
	}
	w.evictIfOverCapacity(ctx, item.Content)
	return id, nil
}

func (w *WorkingMemory) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}
	if topK <= 0 {
		topK = 5
	}
	qv, err := queryEmbedding(ctx, w.store.embedder, query)
	if err != nil {
		return nil, err
	}
	items, vecs := w.store.snapshot()
	out := make([]SearchResult, 0, len(items))
	for id, it := range items {
		if IsDisabled(it) {
			continue
		}
		score := w.score(query, qv, it, vecs[id])
		out = append(out, SearchResult{Item: it, Score: score})
	}
	sortDesc(out)
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

func (w *WorkingMemory) Get(_ context.Context, id string) (MemoryItem, error) {
	return w.store.get(id)
}

func (w *WorkingMemory) Update(ctx context.Context, id string, fn func(*MemoryItem)) error {
	return w.store.update(ctx, id, fn)
}

func (w *WorkingMemory) Remove(_ context.Context, id string) error {
	return w.store.remove(id)
}

func (w *WorkingMemory) Stats() Stats {
	return w.store.stats(w.opts.Capacity)
}

func (w *WorkingMemory) List(_ context.Context, filter ListFilter, pageSize int, cursor string) (ListPage, error) {
	return listFromStore(w.store, filter, pageSize, cursor)
}

func (w *WorkingMemory) Export(_ context.Context) (Snapshot, error) {
	return exportFromStore(w.store, KindWorking), nil
}

func (w *WorkingMemory) Import(_ context.Context, snap Snapshot, mode ImportMode) (ImportReport, error) {
	if snap.Kind != KindWorking {
		return ImportReport{}, fmt.Errorf("%w: got %s, want working", ErrSnapshotKindMismatch, snap.Kind)
	}
	return importIntoStore(w.store, snap, mode)
}

func RestoreWorking(e Embedder, snap Snapshot, opts WorkingOptions) (*WorkingMemory, error) {
	if e == nil {
		return nil, ErrEmbedderRequired
	}
	w, err := NewWorking(e, opts)
	if err != nil {
		return nil, err
	}
	if _, err := w.Import(context.Background(), snap, ImportReplace); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *WorkingMemory) score(query string, qv []float32, it MemoryItem, iv []float32) float64 {
	vec := vectorScore(qv, iv)
	kw := keywordScore(query, it.Content)
	decay := timeDecay(it.CreatedAt, w.opts.Decay)
	return (vec*0.7 + kw*0.3) * decay * importanceMultiplier(it.Importance) *
		savedBoostMultiplier(it, w.opts.SavedBoost)
}

func (w *WorkingMemory) evictIfOverCapacity(ctx context.Context, probe string) {
	if w.opts.Capacity <= 0 {
		return
	}
	w.store.mu.Lock()
	if len(w.store.items) <= w.opts.Capacity {
		w.store.mu.Unlock()
		return
	}
	w.store.mu.Unlock()

	qv, err := queryEmbedding(ctx, w.store.embedder, probe)
	if err != nil {
		w.evictOldest()
		return
	}
	items, vecs := w.store.snapshot()
	type pair struct {
		id    string
		score float64
	}
	scores := make([]pair, 0, len(items))
	for id, it := range items {
		s := w.score(probe, qv, it, vecs[id])
		scores = append(scores, pair{id, s})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score < scores[j].score })
	_ = w.store.remove(scores[0].id)
}

func (w *WorkingMemory) evictOldest() {
	items, _ := w.store.snapshot()
	var (
		oldestID string
		oldest   = time.Now()
	)
	for id, it := range items {
		if it.CreatedAt.Before(oldest) {
			oldest = it.CreatedAt
			oldestID = id
		}
	}
	if oldestID != "" {
		_ = w.store.remove(oldestID)
	}
}

func sortDesc(out []SearchResult) {
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
}
