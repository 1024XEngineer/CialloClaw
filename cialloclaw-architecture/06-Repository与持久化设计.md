# 06-Repository 与持久化设计

> 目标：为 CialloClaw 提供一套**本地优先、接口先行、实现可替换**的持久化方案，使系统可以在不改业务层代码的前提下切换底层存储，并支持会话恢复、任务追踪、审计、记忆沉淀与功能扩展。

---

## 1. 文档定位

本文件回答 5 个问题：

1. 哪些对象必须持久化，哪些只保存在内存中。
2. 每类对象应该落到什么存储介质。
3. Repository 接口应该如何定义。
4. 事务、索引、恢复、清理如何设计。
5. 后续扩展功能如何在不破坏现有架构的前提下新增存储能力。

本文件与以下文档配套阅读：

- `01-核心状态模型与状态机.md`
- `02-事件总线与内部接口协议.md`
- `04-事件字典.md`
- `05-模块职责与依赖边界.md`

---

## 2. 设计原则

### 2.1 本地优先

CialloClaw 是本地桌面 Agent，不依赖云端服务器，因此默认持久化方案应优先选择：

- 本地文件系统
- 本地 SQLite
- 可选本地向量索引

### 2.2 接口先行

上层模块只依赖 repository 接口，不直接依赖 SQLite、JSON 文件、向量库或具体 ORM。

### 2.3 状态与日志分离

- **状态对象**表示“当前是什么”
- **日志对象**表示“如何变成现在”

两者必须分开存储与管理。

### 2.4 结构化数据与大对象分离

- 结构化状态：Session、Task、Loop、Approval、Log 元数据 → SQLite
- 大对象/附件：截图、原始页面、生成文件、导出结果、快照 → 文件系统

### 2.5 可恢复优先于极致性能

本项目早期的核心目标不是最高吞吐，而是：

- 崩溃后可恢复
- 状态不丢
- 可调试
- 可追踪

### 2.6 实现可替换

推荐默认实现，但不把存储实现绑定进业务设计。

例如：

- MVP：SQLite + Filesystem
- 后期：Badger / BoltDB / DuckDB / 自研索引

都不应影响上层模块接口。

---

## 3. 存储分层建议

建议采用三层存储模型：

```text
Storage Layer
├── Structured Store     # SQLite，存状态与索引
├── Blob Store           # 本地文件系统，存大对象
└── Vector / Search Store # 可选，存 embedding 或检索索引
```

### 3.1 Structured Store（结构化存储）

适合保存：

- session
- task
- task_step
- loop
- loop_iteration
- approval
- working_memory 的结构化索引
- episodic_memory 元数据
- semantic_memory
- procedural_memory
- log_entry 元数据
- event_record（如果开启事件留档）

**默认推荐：SQLite**

原因：

- 嵌入式，易于部署
- 支持事务
- 支持索引、查询、排序
- 对本地桌面应用足够稳定

### 3.2 Blob Store（文件存储）

适合保存：

- 屏幕截图
- OCR 原文
- 网页原始提取结果
- 大段 LLM 输入输出归档
- 导出的 markdown / txt / html / code
- 会话快照文件
- 工具执行产生的大文件

**默认推荐：本地文件系统目录树**

### 3.3 Vector / Search Store（可选）

适合保存：

- embedding
- 向量索引
- RAG 检索索引

MVP 阶段可以先不实现独立向量数据库，而是：

- 本地文件 + 索引文件
- 或简单 SQLite + 外挂索引方案

---

## 4. 哪些对象必须持久化

---

### 4.1 必须持久化

#### Session

持久化原因：

- 崩溃恢复
- 历史追踪
- 多会话切换

#### Task / TaskStep

持久化原因：

- 任务执行与恢复
- UI 展示历史
- 调试与审计
- 生成 procedural memory

#### Loop / LoopIteration

持久化原因：

- 循环任务恢复
- 收敛分析
- 避免重复状态

#### Approval

持久化原因：

- 用户确认链路不可丢失
- 可超时恢复
- 审计要求

#### LogEntry

持久化原因：

- 排障
- 回放
- 审计
- 记忆提炼来源

#### Episodic / Semantic / Procedural Memory

持久化原因：

- 长期记忆
- 个性化与复用
- 跨会话检索

#### Profile

持久化原因：

- 用户偏好
- 隐私设置
- 使用习惯

---

### 4.2 可选持久化

#### WorkingMemory

默认策略：

- 内存中维护当前态
- 周期性持久化摘要或快照

原因：

- Working memory 更新频繁
- 不适合全量、每次写盘
- 但需要支持异常恢复，因此应定期快照

#### Event Record

不是所有事件都必须持久化。

建议策略：

- 审计事件：必须持久化
- 状态迁移关键事件：建议持久化
- 瞬时 UI 事件：通常不持久化

---

## 5. 推荐目录结构

```text
internal/state/
├── session/
│   ├── model.go
│   ├── repository.go
│   ├── sqlite_repository.go
│   └── snapshot.go
├── task/
│   ├── model.go
│   ├── step.go
│   ├── repository.go
│   └── sqlite_repository.go
├── loop/
│   ├── model.go
│   ├── iteration.go
│   ├── repository.go
│   └── sqlite_repository.go
├── approval/
│   ├── model.go
│   ├── repository.go
│   └── sqlite_repository.go
├── memory/
│   ├── working.go
│   ├── episodic.go
│   ├── semantic.go
│   ├── procedural.go
│   ├── repository.go
│   └── sqlite_repository.go
├── profile/
│   ├── model.go
│   ├── repository.go
│   └── sqlite_repository.go
├── log/
│   ├── model.go
│   ├── repository.go
│   └── sqlite_repository.go
└── blob/
    ├── repository.go
    ├── filesystem_repository.go
    └── path.go
```

说明：

- `repository.go` 定义接口
- `sqlite_repository.go` / `filesystem_repository.go` 放默认实现
- 上层永远依赖接口而不是具体实现

---

## 6. 推荐存储映射

---

### 6.1 Session

建议表：`sessions`

关键字段：

- id
- user_id
- device_id
- status
- started_at
- last_active_at
- ended_at
- current_task_id
- trace_root_id
- metadata_json

索引建议：

- `idx_sessions_status`
- `idx_sessions_started_at`
- `idx_sessions_last_active_at`

---

### 6.2 Task

建议表：`tasks`

关键字段：

- id
- session_id
- parent_task_id
- kind
- title
- goal
- status
- priority
- planner
- assignee_agent
- plan_id
- loop_id
- retry_count
- max_retry
- trace_id
- input_json
- output_json
- error_text
- created_at
- started_at
- updated_at
- completed_at

索引建议：

- `idx_tasks_session_id`
- `idx_tasks_status`
- `idx_tasks_parent_task_id`
- `idx_tasks_trace_id`
- `idx_tasks_created_at`

---

### 6.3 TaskStep

建议表：`task_steps`

关键字段：

- id
- task_id
- step_index
- name
- description
- status
- tool_name
- agent_name
- input_json
- output_json
- error_text
- started_at
- ended_at

索引建议：

- `idx_task_steps_task_id`
- `idx_task_steps_status`
- `(task_id, step_index)` 唯一索引

---

### 6.4 Loop

建议表：`loops`

关键字段：

- id
- session_id
- task_id
- status
- strategy
- current_iteration
- max_iterations
- stop_condition_json
- convergence_state_json
- backoff_state_json
- last_result_summary
- last_score
- breakpoint
- trace_id
- created_at
- updated_at
- ended_at

索引建议：

- `idx_loops_task_id`
- `idx_loops_status`
- `idx_loops_trace_id`

---

### 6.5 LoopIteration

建议表：`loop_iterations`

关键字段：

- id
- loop_id
- iteration_index
- input_summary
- action_summary
- output_summary
- score
- decision
- started_at
- ended_at

索引建议：

- `idx_loop_iterations_loop_id`
- `(loop_id, iteration_index)` 唯一索引

---

### 6.6 Approval

建议表：`approvals`

关键字段：

- id
- session_id
- task_id
- task_step_id
- loop_id
- status
- risk
- action_type
- action_summary
- proposed_args_json
- resource_refs_json
- requested_by
- requested_at
- decided_by
- decided_at
- timeout_at
- reason
- resume_token
- trace_id

索引建议：

- `idx_approvals_session_id`
- `idx_approvals_task_id`
- `idx_approvals_status`
- `idx_approvals_timeout_at`

---

### 6.7 LogEntry

建议表：`log_entries`

关键字段：

- id
- timestamp
- level
- session_id
- task_id
- task_step_id
- loop_id
- approval_id
- event_id
- trace_id
- span_id
- parent_span_id
- category
- message
- payload_json

索引建议：

- `idx_logs_timestamp`
- `idx_logs_session_id`
- `idx_logs_task_id`
- `idx_logs_trace_id`
- `idx_logs_level`
- `idx_logs_category`

---

### 6.8 Memory

#### episodic_memories

- id
- session_id
- task_id
- loop_id
- summary
- event_refs_json
- artifact_refs_json
- importance
- tags_json
- created_at

#### semantic_memories

- id
- subject
- predicate
- object
- confidence
- source_episode_ids_json
- updated_at

#### procedural_memories

- id
- name
- trigger_pattern
- steps_json
- success_rate
- source_task_ids_json
- updated_at

---

### 6.9 Blob

建议目录：

```text
.data/
├── blobs/
│   ├── screenshots/
│   ├── ocr/
│   ├── extracts/
│   ├── exports/
│   ├── snapshots/
│   └── artifacts/
```

Blob 元数据不直接靠文件名推断，应在结构化表中保存引用：

- blob_id
- blob_type
- file_path
- content_type
- size
- sha256
- created_at
- owner_task_id / owner_session_id

---

## 7. Repository 接口设计原则

### 7.1 一个聚合根一个 repository

建议按聚合根定义：

- SessionRepository
- TaskRepository
- LoopRepository
- ApprovalRepository
- MemoryRepository
- LogRepository
- BlobRepository

不要做一个大而全的 `StateRepository`。

### 7.2 接口要表达业务意图，而不只是 CRUD

不推荐只写：

- Save
- Update
- Delete
- Find

更推荐表达业务意图，例如：

- CreateSession
- MarkSessionClosed
- AttachTaskToSession
- MarkTaskRunning
- MarkTaskSucceeded
- CreateApproval
- ResolveApproval
- AppendLog
- AppendLoopIteration
- StoreEpisode
- FindPendingApprovals

### 7.3 Repository 不做业务编排

Repository 只负责：

- 读写存储
- 简单查询
- 原子更新

不负责：

- task 规划
- loop 决策
- approval 策略判断
- 事件发布

---

## 8. 推荐接口草案

### 8.1 SessionRepository

```go
package session

type Repository interface {
    Create(ctx context.Context, s *Session) error
    GetByID(ctx context.Context, id string) (*Session, error)
    Update(ctx context.Context, s *Session) error
    MarkStatus(ctx context.Context, id string, status SessionStatus) error
    ListActive(ctx context.Context) ([]*Session, error)
    DeleteSnapshot(ctx context.Context, sessionID string) error
}
```

### 8.2 TaskRepository

```go
package task

type Repository interface {
    Create(ctx context.Context, t *Task) error
    GetByID(ctx context.Context, id string) (*Task, error)
    ListBySession(ctx context.Context, sessionID string) ([]*Task, error)
    ListActiveBySession(ctx context.Context, sessionID string) ([]*Task, error)
    Update(ctx context.Context, t *Task) error
    UpdateStatus(ctx context.Context, id string, status TaskStatus, errText *string) error

    CreateStep(ctx context.Context, step *TaskStep) error
    UpdateStep(ctx context.Context, step *TaskStep) error
    ListSteps(ctx context.Context, taskID string) ([]*TaskStep, error)
}
```

### 8.3 LoopRepository

```go
package loop

type Repository interface {
    Create(ctx context.Context, l *Loop) error
    GetByID(ctx context.Context, id string) (*Loop, error)
    GetByTaskID(ctx context.Context, taskID string) (*Loop, error)
    Update(ctx context.Context, l *Loop) error
    AppendIteration(ctx context.Context, it *LoopIteration) error
    ListIterations(ctx context.Context, loopID string) ([]*LoopIteration, error)
}
```

### 8.4 ApprovalRepository

```go
package approval

type Repository interface {
    Create(ctx context.Context, a *Approval) error
    GetByID(ctx context.Context, id string) (*Approval, error)
    ListPendingBySession(ctx context.Context, sessionID string) ([]*Approval, error)
    ListPendingExpired(ctx context.Context, before time.Time) ([]*Approval, error)
    Resolve(ctx context.Context, id string, status ApprovalStatus, decidedBy string, reason *string) error
}
```

### 8.5 MemoryRepository

```go
package memory

type Repository interface {
    SaveWorkingSnapshot(ctx context.Context, wm *WorkingMemory) error
    GetWorkingSnapshot(ctx context.Context, sessionID string) (*WorkingMemory, error)

    StoreEpisode(ctx context.Context, m *EpisodicMemory) error
    SearchEpisodes(ctx context.Context, query EpisodeQuery) ([]*EpisodicMemory, error)

    UpsertSemantic(ctx context.Context, m *SemanticMemory) error
    SearchSemantic(ctx context.Context, query SemanticQuery) ([]*SemanticMemory, error)

    UpsertProcedural(ctx context.Context, m *ProceduralMemory) error
    SearchProcedural(ctx context.Context, query ProceduralQuery) ([]*ProceduralMemory, error)
}
```

### 8.6 LogRepository

```go
package log

type Repository interface {
    Append(ctx context.Context, entry *LogEntry) error
    Query(ctx context.Context, q Query) ([]*LogEntry, error)
    QueryByTrace(ctx context.Context, traceID string) ([]*LogEntry, error)
}
```

### 8.7 BlobRepository

```go
package blob

type Repository interface {
    Put(ctx context.Context, meta *Meta, reader io.Reader) error
    Get(ctx context.Context, blobID string) (*Meta, io.ReadCloser, error)
    Delete(ctx context.Context, blobID string) error
}
```

---

## 9. 事务设计

### 9.1 哪些更新需要事务

以下场景建议在一个事务中完成：

#### 场景 A：创建 Task + TaskStep

- 创建 task
- 创建 step 列表
- 更新 session.active_task_ids（如有）

#### 场景 B：Approval 放行后恢复执行

- 更新 approval.status
- 更新相关 task / step 状态
- 写审计日志

#### 场景 C：Loop 迭代提交

- 更新 loop.current_iteration
- 写 loop_iteration
- 更新 task 状态或结果摘要

### 9.2 事务边界原则

- **状态更新要原子化**
- **事件发布不放在数据库事务内部阻塞执行**

推荐模式：

1. 先提交状态事务
2. 再发布后续事件
3. 如果事件发布失败，写错误日志并由恢复逻辑补偿

### 9.3 不做分布式事务

本项目是本地单进程桌面 Agent，不需要引入复杂分布式事务。

---

## 10. 并发与幂等

### 10.1 状态变更必须可幂等

例如：

- 重复收到同一个 approval response
- 重复写入 loop iteration
- 重复执行同一个 task step

建议引入：

- 幂等键 `idempotency_key`
- 唯一索引 `(task_id, step_index)` / `(loop_id, iteration_index)`
- 基于状态机的合法迁移检查

### 10.2 Repository 层不负责复杂去重决策

去重决策由：

- event middleware
- application service
- loop controller

负责。

repository 只做：

- 唯一约束
- 乐观失败

---

## 11. 恢复策略

### 11.1 应恢复哪些对象

程序启动后，应优先恢复：

- status = active / paused 的 session
- status = queued / planned / running / waiting / blocked 的 task
- status = running / paused 的 loop
- status = pending 且未过期的 approval
- 最近 working memory snapshot

### 11.2 恢复原则

#### Session

- 如果进程异常退出，active session 可恢复为 paused 或 recovering

#### Task

- `running` 状态的 task 启动后不要直接继续执行
- 应先标记为 `waiting` 或 `recovering`，再由调度器决定是否恢复

#### Approval

- `pending` 的 approval 继续有效
- 超时则转为 `expired`

#### Loop

- 恢复 last iteration
- 重新校验 stop condition
- 必要时要求用户确认是否继续

### 11.3 快照策略

建议对以下内容做快照：

- Session 概览
- Working memory
- 当前活跃 task 列表
- UI 视图必要状态

快照可落入：

- SQLite 表
- 或 `.data/blobs/snapshots/` 文件

---

## 12. 清理与保留策略

### 12.1 Log

建议：

- 最近 7~30 天保留热数据
- 更旧日志压缩导出或归档

### 12.2 WorkingMemory

建议：

- 会话结束后只保留摘要快照
- 详细 working memory 可清理

### 12.3 Blob

建议：

- 记录引用计数或 owner 关系
- 无引用 blob 可清理
- 大文件要定期扫描

### 12.4 EpisodicMemory

建议：

- 保留较久
- 通过 summarize 机制压缩旧 episode

---

## 13. 查询模式建议

Repository 不只是为了写入，也要服务这些典型查询：

### 13.1 UI 查询

- 当前 session 的 active task
- 当前 pending approvals
- 最近 loop 进度
- 最近日志

### 13.2 恢复查询

- 所有未结束 session
- 所有可恢复 task
- 所有待处理 approval

### 13.3 审计查询

- 某个 trace 的完整 log
- 某个 approval 的决策链
- 某个 task 的执行过程

### 13.4 记忆提炼查询

- 某个 session 的高重要度 episode
- 某类 task 的成功 workflow
- 某类失败的共性错误

---

## 14. 推荐默认实现

### 14.1 MVP 版本

建议默认落地：

- Structured Store：SQLite
- Blob Store：本地文件系统
- Memory Search：先做简单索引 + 结构化查询

### 14.2 第二阶段

- 补齐 embedding / vector index
- 支持高级检索
- 支持 log replay

### 14.3 不建议一开始做的事

- 引入重型 ORM
- 引入复杂分布式存储
- 为“未来可能支持云端”过度抽象

---

## 15. 实施顺序

### 第 1 批

先实现：

- SessionRepository
- TaskRepository
- ApprovalRepository
- LogRepository
- BlobRepository

### 第 2 批

再实现：

- LoopRepository
- Working memory snapshot
- 恢复逻辑

### 第 3 批

最后实现：

- Episodic / Semantic / Procedural memory
- 向量检索
- 自动清理归档

---

## 16. 开发约束

1. 上层模块禁止直接操作 SQLite。
2. 上层模块禁止直接拼接 SQL。
3. Repository 接口定义放在各领域模块下，具体实现放在默认实现文件中。
4. 任何新增状态对象，都必须先定义 model + repository interface。
5. 任何新增大文件产物，都必须通过 BlobRepository 统一落盘。
6. 新增功能若产生可恢复状态，必须定义恢复语义。

---

## 17. 一句话总结

CialloClaw 的持久化设计应采用：

**Session 统筹运行时，Task 承载业务，Loop 管理迭代，Approval 控制风险，Memory 沉淀知识，Log 保留事实；结构化状态落 SQLite，大对象走文件系统，上层永远面向 repository 接口编程。**
