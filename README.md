# llm-agent-memory

Sibling Go module under the `llm-agent-ecosystem` umbrella. Extends
`github.com/costa92/llm-agent/memory` with three additive
capabilities — no modification to core.

Status: 0.1.0 (M0 + M1 of the master memory roadmap).

## Import

```go
import "github.com/costa92/llm-agent-memory/memory"
```

## What this module adds

- `ScopedLifecycleManager` — scope-honoring Consolidate/Forget/Stats.
- `Consolidator` — dedupe-aware Working→Episodic promotion.
- `UnifiedSearcher` — `SearchUnified(ctx, query, topK)` cross-tier merge.

## Boundary

This module **wraps** core. It does not fork or modify any file under
`github.com/costa92/llm-agent/memory`. The core SDK remains
stdlib-only and authoritative.

See `docs/superpowers/plans/2026-05-25-llm-agent-memory-roadmap.md`
in the umbrella for the full subproject roadmap.
