# Changelog

`github.com/costa92/llm-agent-memory` 的所有重要变更都会记录在本文件中。

<!-- Keep a Changelog format: https://keepachangelog.com/en/1.1.0/ -->
<!-- Semver: https://semver.org/ -->

## [2.0.0] - 2026-06-03

> 破坏性变更：模块路径现为 `github.com/costa92/llm-agent-memory/v2`。
> 解决了倒置依赖——基础记忆 module 不再向上依赖框架
> `github.com/costa92/llm-agent`。

### Changed

- **模块路径 → `/v2`**（Go 主版本提升）。请将 import 更新为
  `github.com/costa92/llm-agent-memory/v2/memory`。
- **移除了对 `github.com/costa92/llm-agent` 的依赖。** 共享的
  记忆契约（接口 + 数据类型 `MemoryItem`/`SearchResult`/
  `Snapshot`/`Kind`/`Scope`/`Source`/`Category`/选项、哨兵错误，
  以及 8 个元数据辅助函数）现在来自叶子契约
  `github.com/costa92/llm-agent-contract/memory`（锚定 `v0.1.0`）。
  本地以 `coremem` 为后端的类型别名、`AdaptCore*` 适配器家族，
  以及 `*FromCore`/`*ToCore` 转换器均已移除；`MemoryItem` 现在是
  契约类型的直接别名（不再有重复的 struct）。

### Added

- **`(*Manager).Lookup(kind) (Memory, error)`**——导出的按 kind 记忆
  查找，使 `*Manager` 满足契约 `Manager` 接口，从而让 MemoryTool 适配器
  可以面向接口而非具体引擎。

### Internal

- `Sanitizer`/`SanitizerFunc`/`WithSanitizer` 现在在本 module 内原生定义
  （它们属于策略行为，不属于叶子契约）。

## [1.0.0] - 2026-05-26

> 首个主版本发布。完整迁移指南见伞形仓库中的
> `docs/memory-v1-migration.zh-CN.md`。

### Added

- **`memory.Manager`（D-1）**——由兄弟仓拥有、以能力接口为类型的
  协调器。通过 `NewManager(Options{...})` 构造。每一层的
  `TierOptions` 携带五个能力字段——`Memory`、`Lister`、
  `Exporter`、`Importer`、`Lifecycle`——均以接口为类型。这
  解除了 v0.7 的限制：被 `coremem.WithSanitizer` 包装的
  记忆此前无法安装进 `coremem.ManagerOptions`（后者要求
  一个具体的 `*coremem.WorkingMemory`）。
- **`LifecycleMemory` 接口（D-1）**——`Consolidate(ctx, opts)` +
  `Forget(ctx, kind, opts)`。一项新能力，核心的
  `coremem.Manager` 通过包私有访问来执行；外部
  后端（Postgres、pgvector）现在可以原生实现生命周期。
- **`memory.RecallEngine.Recall(ctx, query, opts) (UnifiedRecall, error)`（D-2）**——
  v1 统一唤回门面。层感知变为内部细节。支持
  按层预算、层选择位掩码，以及按层溯源。
- **`memory.RecallOptions` / `memory.UnifiedRecall` / `memory.TierStats` /
  `memory.TierMask`**——围绕 `Recall` 的公共面。

### Deprecated

- `memory.UnifiedSearcher`——优先使用 `memory.RecallEngine.Recall`。
  在 v1.x 线中仍可用；在 v2.0.0 移除。
- `memory.ParallelSearcher`——优先使用 `memory.RecallEngine.Recall`。
  在 v1.x 线中仍可用；在 v2.0.0 移除。

### Dependencies

- **没有新的第三方依赖。** `modernc.org/sqlite v1.50.1` 及其
  传递闭包保持不变。纯 Go 路径得以保留。

### Compatibility

- 核心 `github.com/costa92/llm-agent v0.7.0` 未被改动。兄弟仓
  v1.0.0 锚定核心 v0.7.0；兄弟仓的 v1.x 线可以针对任意
  保留了公共记忆面的核心 v0.7.x 发布。
- 所有 M1/M2/M3 公共 API（`ScopedLifecycleManager`、`Consolidator`、
  `WritePolicy`、`PolicyEnforcingMemory`、`PolicyAdapter`、`SQLiteStore`、
  `Observer`、每一个事件名常量）保持不变。这些之中尚无任何一个
  进入 v0.3.0 的弃用窗口——它们在 v1.x 中
  仍是规范用法。


## [0.3.0] - 2026-05-26

### Added

- `WritePolicy` 接口，带 `Decide(ctx, ProposedWrite) WritePolicyDecision`，
  覆盖 `docs/memory-roadmap.zh-CN.md` §4.3（C-1）中记录的全部四种决策：
  user-saved、agent-inferred、reject、redact。包含
  `Verdict` 枚举（`accept` / `redact` / `reject`）、`WriteSource` 枚举
  （`user_saved` / `agent_inferred` / `system`），以及 `PolicyFunc`
  函数到接口的适配器。
- `PolicyEnforcingMemory` 包装器，消费一个 `WritePolicy` 并
  将每个 verdict 翻译为一次 `*coremem.Manager.Add` 调用（或一次
  带有别名 `ErrRejectedByPolicy` 的拒绝）。当策略返回与
  输入不同的 `Kind` 时，会重新路由写入的 kind。
- `PolicyAdapter` 让一个 `WritePolicy` 满足既有的
  `coremem.Sanitizer` 接口，供接线到 `WithSanitizer` 的调用方使用。
  如果策略试图重新路由 kind，则返回
  `ErrPolicyKindRerouteUnsupported`（Sanitizer 的返回三元组没有 kind 槽位）。
- `SQLiteStore`（C-2）：`coremem.SnapshotStore` 的首个非文件系统实现。
  实现了 `Save` / `Load` / `LoadKind` /
  `Delete` / `List`。带幂等的代码内迁移器（schema v1，两张
  表，一个索引），通过 `ErrSchemaVersionAhead` 拒绝未来版本。
  可通过 `coremem.Manager.ExportAll` / `ImportAll` 往返。
- `EventWritePolicyDecided` observer 事件，由
  `PolicyEnforcingMemory.Add` 为全部三种 verdict 发出。Attrs schema：
  `verdict`、`input_kind`、`decided_kind`、`source`、`reason`。

### Dependencies

- 首个第三方依赖：`modernc.org/sqlite`（纯 Go，无 CGO）。
  理由：在不强迫下游兄弟仓引入 CGO 或破坏交叉编译的前提下，
  实现一个非文件系统的 SnapshotStore。
  核心 `llm-agent` 仍保持仅标准库——此依赖被限制在
  兄弟仓内。

## [0.2.0] - 2026-05-26

### Fixed

- `ConsolidateScoped`、`ForgetScoped`、`StatsScoped`，以及
  `Consolidator.Consolidate` 现在会翻页游标，而不再
  悄无声息地丢弃首个底层页之后的条目（解决了
  终审 I-1 发现的问题）。

### Added

- `Observer` 接口，带 `Event{Name, Attrs}` 载荷 schema
  （在 v0.2.0 锁定）和七个规范的事件名常量
  （`memory_add_total`、`memory_search_total`、`memory_search_hits`、
  `memory_consolidated_total`、`memory_forgotten_total`、
  `memory_snapshot_items`、`memory_snapshot_vectors_bytes`），对应
  主路线图的 Phase B-1。
- 为 `NewScopedLifecycleManager`、
  `NewConsolidator`、`NewUnifiedSearcher` 以及新的
  `NewParallelSearcher` 提供 `WithObserver(Observer) Option`。零配置（不传该 option）即文档规定的
  空操作。
- `ParallelSearcher.SearchAllParallel(ctx, query, topK)`——仅标准库的
  按 kind 扇出，其按 kind 的 map 形态与
  `coremem.Manager.SearchAll` 相同。`UnifiedSearcher.SearchUnified` 现在
  默认将其扇出委托给 `ParallelSearcher`。
- `Consolidator.ExportAll(ctx, dir)` 薄包装，发出按 kind 的
  `memory_snapshot_items` 和 `memory_snapshot_vectors_bytes`。

### Notes

- Phase B-2（工作记忆驱逐的 embed 复用）**推迟到一个核心 PR**：
  它位于 `coremem.WorkingMemory.evictIfOverCapacity`（包
  私有）内部，无法从本兄弟仓包装。一个回归测试
  锚定了驱逐语义，使最终的上游变更不会
  悄无声息地破坏本消费方。

## [0.1.0] - 2026-05-25

### Added

- 初始子项目脚手架（主路线图 M0）。
- `memory.Version` 常量。
- `memory.ScopedLifecycleManager`，带 `ConsolidateScoped`、
  `ForgetScoped`、`StatsScoped`（Phase A 条目 A-1）。
- `memory.Consolidator`，带提升一次的去重元数据
  （`_consolidated_at`、`_promoted_from`、`_promotion_count`）
  （Phase A 条目 A-2）。
- `memory.UnifiedSearcher.SearchUnified(ctx, query, topK)`
  （Phase A 条目 A-3）。

### Known Limitations

- `ConsolidateScoped` / `ForgetScoped` / `StatsScoped` / `Consolidator.Consolidate`
  每次调用最多处理一页（`Consolidate*` 为 50，`Forget*`/`Stats*` 为 100）
  条目。匹配条目数超过页上限的作用域或层，将只对首页
  生效。一个游标感知的分页
  循环已列入 M2 路线图。
