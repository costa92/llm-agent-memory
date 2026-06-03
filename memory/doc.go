// Package memory provides the native working / episodic / semantic
// memory engines and a capability-interface-typed Manager, built on the
// data types + interfaces defined by the leaf contract
// github.com/costa92/llm-agent-contract/memory. The public type surface
// (MemoryItem, Kind, Snapshot, Manager, ...) is aliased from that
// contract so the contract is the single source of truth.
//
// On top of the engines it adds three lifecycle capabilities:
//
//   - ScopedLifecycleManager — adds ConsolidateScoped, ForgetScoped,
//     and StatsScoped methods that honor non-zero ctx scope.
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
// The contract import is aliased as `contractmem` in the alias file to
// avoid a name collision with this package's own name.
package memory
