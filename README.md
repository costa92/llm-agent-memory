# llm-agent-memory

Sibling Go module under the `llm-agent-ecosystem` umbrella. Extends
`github.com/costa92/llm-agent/memory` with three additive
capabilities — no modification to core.

Status: 0.2.0 (M0 + M1 + M2 of the master memory roadmap).

## Import

```go
import "github.com/costa92/llm-agent-memory/memory"
```

## What this module adds

- `ScopedLifecycleManager` — scope-honoring Consolidate/Forget/Stats.
- `Consolidator` — dedupe-aware Working→Episodic promotion.
- `UnifiedSearcher` — `SearchUnified(ctx, query, topK)` cross-tier merge.
- `Observer` interface + 7 canonical event-name constants (`EventAddTotal`, `EventSearchTotal`, `EventSearchHits`, `EventConsolidatedTotal`, `EventForgottenTotal`, `EventSnapshotItems`, `EventSnapshotVectorsBytes`) + `WithObserver` Option for all 4 constructors (Phase B-1 observability hooks).
- `ParallelSearcher.SearchAllParallel(ctx, query, topK)` — stdlib goroutine fan-out matching `coremem.Manager.SearchAll` shape; `UnifiedSearcher.SearchUnified` now routes through it by default (Phase B-3).
- `Consolidator.ExportAll(ctx, dir)` thin wrap emitting per-kind snapshot events.

## Boundary

This module **wraps** core. It does not fork or modify any file under
`github.com/costa92/llm-agent/memory`. The core SDK remains
stdlib-only and authoritative.

See `docs/superpowers/plans/2026-05-25-llm-agent-memory-roadmap.md`
in the umbrella for the full subproject roadmap.
