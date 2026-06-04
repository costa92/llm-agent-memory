[English](./README.md) | [简体中文](./README.zh-CN.md)

# llm-agent-memory

Sibling Go module under the `llm-agent-ecosystem` umbrella. Builds the
native memory engines on top of the leaf contract
`github.com/costa92/llm-agent-contract/memory` with three additive
capabilities — no dependency on the framework.

Status: 2.0.0 (M0–M4 of the master memory roadmap; `/v2` module path after the contract decoupling).

## Import

```go
import "github.com/costa92/llm-agent-memory/v2/memory"
```

## What this module adds

- `ScopedLifecycleManager` — scope-honoring Consolidate/Forget/Stats.
- `Consolidator` — dedupe-aware Working→Episodic promotion.
- `UnifiedSearcher` — `SearchUnified(ctx, query, topK)` cross-tier merge.
- `Observer` interface + 7 canonical event-name constants (`EventAddTotal`, `EventSearchTotal`, `EventSearchHits`, `EventConsolidatedTotal`, `EventForgottenTotal`, `EventSnapshotItems`, `EventSnapshotVectorsBytes`) + `WithObserver` Option for all 4 constructors (Phase B-1 observability hooks).
- `ParallelSearcher.SearchAllParallel(ctx, query, topK)` — stdlib goroutine fan-out matching `Manager.SearchAll` shape; `UnifiedSearcher.SearchUnified` now routes through it by default (Phase B-3).
- `Consolidator.ExportAll(ctx, dir)` thin wrap emitting per-kind snapshot events.
- `memory.Manager` — capability-interface-typed coordinator (D-1). Accepts decorator-wrapped `memory.Memory` interface values without a cast.
- `memory.RecallEngine.Recall(ctx, query, opts)` — unified recall facade (D-2). The v1 public recall surface.

## Boundary

This module depends only on the leaf contract
`github.com/costa92/llm-agent-contract/memory` (data types + interfaces)
and provides the native engine implementations. It has no dependency on
the `llm-agent` framework module.

See `docs/superpowers/plans/2026-05-25-llm-agent-memory-roadmap.md`
in the umbrella for the full subproject roadmap.

## Migration from v0.x

See `docs/memory-v1-migration.zh-CN.md` in the umbrella repo for
the full migration recipe. New code should construct
`*memory.Manager` directly and wire capabilities explicitly.

Durable backend implementations and gateway/service composition are provided by
separate modules, not by this SDK package.
