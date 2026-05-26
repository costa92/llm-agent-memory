# Changelog

All notable changes to `github.com/costa92/llm-agent-memory` will be
documented in this file.

<!-- Keep a Changelog format: https://keepachangelog.com/en/1.1.0/ -->
<!-- Semver: https://semver.org/ -->

## [0.3.0] - 2026-05-26

### Added

- `WritePolicy` interface with `Decide(ctx, ProposedWrite) WritePolicyDecision`
  covering all four documented decisions from `docs/memory-roadmap.zh-CN.md`
  §4.3 (C-1): user-saved, agent-inferred, reject, redact. Includes
  `Verdict` enum (`accept` / `redact` / `reject`), `WriteSource` enum
  (`user_saved` / `agent_inferred` / `system`), and the `PolicyFunc`
  function-to-interface adapter.
- `PolicyEnforcingMemory` wrapper that consumes a `WritePolicy` and
  translates each verdict into a `*coremem.Manager.Add` call (or a
  rejection with the aliased `ErrRejectedByPolicy`). Reroutes the
  write kind when the policy returns a different `Kind` than the
  input.
- `PolicyAdapter` lets a `WritePolicy` satisfy the existing
  `coremem.Sanitizer` interface for callers wired to `WithSanitizer`.
  Returns `ErrPolicyKindRerouteUnsupported` if the policy attempts
  to reroute kinds (Sanitizer's return triple has no kind slot).
- `SQLiteStore` (C-2): first non-filesystem implementation of
  `coremem.SnapshotStore`. Implements `Save` / `Load` / `LoadKind` /
  `Delete` / `List`. Idempotent in-code migrator (schema v1, two
  tables, one index) with future-version refusal via
  `ErrSchemaVersionAhead`. Round-trips through `coremem.Manager.
  ExportAll` / `ImportAll`.
- `EventWritePolicyDecided` observer event, emitted by
  `PolicyEnforcingMemory.Add` for all three verdicts. Attrs schema:
  `verdict`, `input_kind`, `decided_kind`, `source`, `reason`.

### Dependencies

- First third-party dependency: `modernc.org/sqlite` (pure-Go, no CGO).
  Justification: enables a non-filesystem SnapshotStore without
  forcing CGO on downstream siblings or breaking cross-compile.
  Core `llm-agent` remains stdlib-only — this dep is contained to
  the sibling.

## [0.2.0] - 2026-05-26

### Fixed

- `ConsolidateScoped`, `ForgetScoped`, `StatsScoped`, and
  `Consolidator.Consolidate` now page through cursors instead of
  silently dropping items past the first underlying page (closes the
  final-review I-1 finding).

### Added

- `Observer` interface with `Event{Name, Attrs}` payload schema
  (locked at v0.2.0) and seven canonical event-name constants
  (`memory_add_total`, `memory_search_total`, `memory_search_hits`,
  `memory_consolidated_total`, `memory_forgotten_total`,
  `memory_snapshot_items`, `memory_snapshot_vectors_bytes`) per
  Phase B-1 of the master roadmap.
- `WithObserver(Observer) Option` for `NewScopedLifecycleManager`,
  `NewConsolidator`, `NewUnifiedSearcher`, and the new
  `NewParallelSearcher`. Zero-config (no option) is the documented
  no-op.
- `ParallelSearcher.SearchAllParallel(ctx, query, topK)` — stdlib-only
  per-kind fan-out with the same per-kind map shape as
  `coremem.Manager.SearchAll`. `UnifiedSearcher.SearchUnified` now
  delegates its fan-out through `ParallelSearcher` by default.
- `Consolidator.ExportAll(ctx, dir)` thin wrap that emits per-kind
  `memory_snapshot_items` and `memory_snapshot_vectors_bytes`.

### Notes

- Phase B-2 (Working eviction embed-reuse) is **deferred to a core PR**:
  it lives inside `coremem.WorkingMemory.evictIfOverCapacity` (package-
  private) and cannot be wrapped from this sibling. A regression test
  pins the eviction semantics so the eventual upstream change cannot
  silently break this consumer.

## [0.1.0] - 2026-05-25

### Added

- Initial subproject scaffolding (M0 of master roadmap).
- `memory.Version` constant.
- `memory.ScopedLifecycleManager` with `ConsolidateScoped`,
  `ForgetScoped`, `StatsScoped` (Phase A item A-1).
- `memory.Consolidator` with promote-once dedupe metadata
  (`_consolidated_at`, `_promoted_from`, `_promotion_count`)
  (Phase A item A-2).
- `memory.UnifiedSearcher.SearchUnified(ctx, query, topK)`
  (Phase A item A-3).

### Known Limitations

- `ConsolidateScoped` / `ForgetScoped` / `StatsScoped` / `Consolidator.Consolidate`
  process at most one page (50 for `Consolidate*`, 100 for `Forget*`/`Stats*`)
  of items per call. Scopes or tiers with more matching items than the page
  cap will see only the first page acted upon. A cursor-aware pagination
  loop is on the M2 roadmap.
