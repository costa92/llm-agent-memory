package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"

	coremem "github.com/costa92/llm-agent/memory"
)

// Deprecated: prefer RecallEngine.Recall (v1.0.0). ParallelSearcher
// remains in the v1.x line for backwards compatibility; it will be
// removed at v2.0.0. See docs/memory-v1-migration.zh-CN.md.
//
// ParallelSearcher wraps a *coremem.Manager and exposes
// SearchAllParallel — a drop-in replacement for
// coremem.Manager.SearchAll that fans out one goroutine per kind by
// dispatching to (*coremem.Manager).Search(ctx, kind, ...). The
// returned per-kind map is identical in shape and content to core's
// serial implementation; the only observable difference is wall-time.
//
// Concurrency primitive: stdlib sync.WaitGroup + buffered channel +
// context.WithCancel. The module is stdlib-only (master-roadmap §3
// dependency policy), so we explicitly avoid golang.org/x/sync/errgroup.
//
// Error-path note: on a non-disabled per-kind error, SearchAllParallel
// returns (nil, err); coremem.Manager.SearchAll returns (partialOut, err).
// Today no in-repo caller consumes the partial map on error.
type ParallelSearcher struct {
	mgr *coremem.Manager
	cfg *config
}

// ErrParallelManagerRequired is returned by NewParallelSearcher when
// the inner *coremem.Manager is nil.
var ErrParallelManagerRequired = errors.New("memory: parallel searcher requires manager")

// NewParallelSearcher wraps an existing *coremem.Manager. Options use
// the shared Option type (WithObserver, etc.). Returns
// ErrParallelManagerRequired if inner is nil.
func NewParallelSearcher(inner *coremem.Manager, opts ...Option) (*ParallelSearcher, error) {
	if inner == nil {
		return nil, ErrParallelManagerRequired
	}
	return &ParallelSearcher{mgr: inner, cfg: newConfig(opts)}, nil
}

// observer exposes the configured observer for in-package callers.
func (p *ParallelSearcher) observer() Observer { return p.cfg.observer }

// parallelKindResult is the per-kind work item exchanged through the
// buffered channel. err is forwarded raw; the receiver loop checks for
// coremem.ErrKindDisabled to silently skip inactive kinds (parity with
// coremem.Manager.SearchAll, manager.go:100-104).
type parallelKindResult struct {
	kind    coremem.Kind
	results []coremem.SearchResult
	err     error
}

// SearchAllParallel fans out the query to every active kind. Returns
// the same map shape as coremem.Manager.SearchAll: disabled kinds are
// omitted from the result map; active kinds are always present (even
// with an empty []SearchResult slice). topK is forwarded per-kind
// verbatim. On any non-disabled error, returns that error wrapped with
// the offending kind.
//
// ctx-derived cancel: deferred-cancel ensures any goroutine still in
// flight when an error short-circuits the result-collection loop sees
// ctx.Done(); this is a hardening step for M3+ when callers wire real
// cancellation semantics — today wg.Wait blocks until all 3 finish.
//
// If multiple kinds error, the reported kind is non-deterministic
// (channel receive order). The error message format includes the
// offending kind for caller introspection.
func (p *ParallelSearcher) SearchAllParallel(ctx context.Context, query string, topK int) (map[coremem.Kind][]coremem.SearchResult, error) {
	kinds := []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic, coremem.KindSemantic}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan parallelKindResult, len(kinds))
	var wg sync.WaitGroup
	for _, kind := range kinds {
		wg.Add(1)
		go func(k coremem.Kind) {
			defer wg.Done()
			res, err := p.mgr.Search(ctx, k, query, topK)
			ch <- parallelKindResult{kind: k, results: res, err: err}
		}(kind)
	}
	wg.Wait()
	close(ch)

	out := make(map[coremem.Kind][]coremem.SearchResult, len(kinds))
	for r := range ch {
		if errors.Is(r.err, coremem.ErrKindDisabled) {
			continue
		}
		if r.err != nil {
			return nil, fmt.Errorf("memory: parallel search %s: %w", r.kind, r.err)
		}
		out[r.kind] = r.results
	}
	return out, nil
}
