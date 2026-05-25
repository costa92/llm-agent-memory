# Changelog

All notable changes to `github.com/costa92/llm-agent-memory` will be
documented in this file.

<!-- Keep a Changelog format: https://keepachangelog.com/en/1.1.0/ -->
<!-- Semver: https://semver.org/ -->

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
