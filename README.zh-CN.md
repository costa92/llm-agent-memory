[English](./README.md) | [简体中文](./README.zh-CN.md)

# llm-agent-memory

`llm-agent-ecosystem` 伞形仓库下的兄弟 Go module。在叶子契约
`github.com/costa92/llm-agent-contract/memory` 之上构建原生记忆引擎，
提供三项仅增量的能力——不依赖框架。

状态：2.0.0（记忆主路线图的 M0–M4；契约解耦后采用 `/v2` 模块路径）。

## Import

```go
import "github.com/costa92/llm-agent-memory/v2/memory"
```

## 本模块新增的内容

- `ScopedLifecycleManager`——尊重作用域的 Consolidate/Forget/Stats。
- `Consolidator`——感知去重的 工作记忆→情景记忆 提升。
- `UnifiedSearcher`——`SearchUnified(ctx, query, topK)` 跨层合并。
- `Observer` 接口 + 7 个规范的事件名常量（`EventAddTotal`、`EventSearchTotal`、`EventSearchHits`、`EventConsolidatedTotal`、`EventForgottenTotal`、`EventSnapshotItems`、`EventSnapshotVectorsBytes`）+ 为全部 4 个构造函数提供的 `WithObserver` Option（Phase B-1 可观测性钩子）。
- `ParallelSearcher.SearchAllParallel(ctx, query, topK)`——仅标准库的 goroutine 扇出，形态与 `Manager.SearchAll` 一致；`UnifiedSearcher.SearchUnified` 现在默认通过它路由（Phase B-3）。
- `Consolidator.ExportAll(ctx, dir)` 薄包装，按 kind 发出快照事件。
- `memory.Manager`——以能力接口为类型的协调器（D-1）。无需类型转换即可接受被装饰器包装的 `memory.Memory` 接口值。
- `memory.RecallEngine.Recall(ctx, query, opts)`——统一唤回门面（D-2）。即 v1 公共唤回面。

## 边界

本模块仅依赖叶子契约
`github.com/costa92/llm-agent-contract/memory`（数据类型 + 接口），
并提供原生引擎实现。它不依赖
`llm-agent` 框架 module。

完整子项目路线图见伞形仓库中的
`docs/superpowers/plans/2026-05-25-llm-agent-memory-roadmap.md`。

## 从 v0.x 迁移

完整迁移方案见伞形仓库中的 `docs/memory-v1-migration.zh-CN.md`。
新代码应直接构造 `*memory.Manager` 并显式接线各项能力。

持久后端实现以及网关/服务组合由独立的 module 提供，而非由本 SDK 包提供。
