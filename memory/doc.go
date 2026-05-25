// Package memory extends github.com/costa92/llm-agent/memory with
// three additive capabilities introduced in milestones M0 + M1 of the
// llm-agent-memory roadmap:
//
//   - ScopedLifecycleManager — adds ConsolidateScoped, ForgetScoped,
//     and StatsScoped methods that honor non-zero ctx scope.
//     Closes the v0.7 limitation on
//     github.com/costa92/llm-agent/memory.ScopedManager.
//
//   - Consolidator — Working→Episodic promotion with dedupe metadata
//     so the same working item is not promoted twice. Writes the
//     reserved metadata keys MetaKeyConsolidatedAt, MetaKeyPromotedFrom,
//     and MetaKeyPromotionCount on source items.
//
//   - UnifiedSearcher — SearchUnified(ctx, query, topK) fans out to
//     working/episodic/semantic, merges, dedupes by (ID, Content),
//     sorts by score descending, and returns a single []SearchResult.
//
// All three components wrap (never modify) core memory types. Every
// Go file in this package aliases the core import as `coremem` to
// avoid a name collision with this package's own name.
package memory
