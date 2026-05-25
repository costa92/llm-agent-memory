// Package memorykit is the short brand name for the standalone
// memory extension SDK whose module path is
// github.com/costa92/llm-agent-memory.
//
// The root package is a documentation anchor only: it exports no
// symbols. Callers import the subpackage:
//
//     import "github.com/costa92/llm-agent-memory/memory"
//
// The subpackage adds three additive capabilities on top of
// github.com/costa92/llm-agent/memory without modifying core:
//
//   - ScopedLifecycleManager — scope-honoring ConsolidateScoped /
//     ForgetScoped / StatsScoped (closes the v0.7 gap noted on
//     llm-agent/memory/scoped_manager.go:12-17).
//   - Consolidator — Working→Episodic promotion with dedupe metadata
//     so the same working item is not promoted twice.
//   - UnifiedSearcher — SearchUnified(ctx, query, topK) returning a
//     single merged + deduped + sorted []SearchResult.
//
// The memorykit name diverges from the llm-agent-memory module path
// on purpose, to give the SDK a concise import-free identity.
package memorykit
