# llm-agent-memory

Sibling Go module under the `llm-agent-ecosystem` umbrella. Extends
`github.com/costa92/llm-agent/memory` with three additive
capabilities — no modification to core.

Status: 1.0.0 (M0–M4 of the master memory roadmap).

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
- `memory.Manager` — capability-interface-typed coordinator (D-1). Accepts decorator-wrapped `coremem.Memory` interface values without a cast.
- `memory.RecallEngine.Recall(ctx, query, opts)` — unified recall facade (D-2). The v1 public recall surface.
- `memory/compat` sub-package — `NewManagerFromCore` / `NewManagerFromLegacyOptions` for one-release-window backwards compatibility.

## Boundary

This module **wraps** core. It does not fork or modify any file under
`github.com/costa92/llm-agent/memory`. The core SDK remains
stdlib-only and authoritative.

See `docs/superpowers/plans/2026-05-25-llm-agent-memory-roadmap.md`
in the umbrella for the full subproject roadmap.

## Migration from v0.x

See `docs/memory-v1-migration.zh-CN.md` in the umbrella repo for
the full migration recipe. TL;DR — new code should construct
`*memory.Manager` directly; existing `*coremem.Manager` callers
can wrap via `compat.NewManagerFromCore` to opt into the v1
surface without rewriting their wiring.
