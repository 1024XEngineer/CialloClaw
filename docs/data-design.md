# CialloClaw 数据设计文档（v6）

## 1. 文档范围

本文档定义 CialloClaw 的数据分层、核心表、字段、索引、约束、DDL 与演进规则。  
数据设计必须同时满足：

- task-centric 对外对象组织
- run-centric 内核执行兼容
- 记忆 / Trace / 前馈配置 与运行态严格分层
- SQLite + WAL 本地优先
- 可支持 FTS5 / sqlite-vec / Workspace / Stronghold

---

## 2. 如何阅读本数据设计

阅读本章时，请先把对象按“作用”分成四层理解：

1. **产品主对象层**：`tasks / task_steps / delivery_results / artifacts / todo_items / recurring_rules`
2. **执行兼容层**：`runs / steps / events / tool_calls`
3. **治理与记忆层**：`approval_requests / authorization_records / audit_records / recovery_points / memory_*`
4. **前馈与反馈层**：`skill_manifests / blueprint_definitions / prompt_template_versions / plugin_manifests / trace_records / eval_snapshots`

这样就能避免把所有表都看成“普通业务表”。

---

## 3. 存储分层

### 3.1 结构化运行态数据库（SQLite + WAL）

用于存储：

- `sessions`
- `tasks`
- `task_steps`
- `todo_items`
- `recurring_rules`
- `runs`
- `steps`
- `events`
- `tool_calls`
- `delivery_results`
- `artifacts`
- `approval_requests`
- `authorization_records`
- `audit_records`
- `recovery_points`

说明：这一层是主链路真源，重点关注任务生命周期、正式交付、安全治理和最小可恢复性。

### 3.2 本地记忆检索层

用于存储：

- `memory_summaries`
- `memory_candidates`
- `retrieval_hits`
- FTS5 文本索引
- sqlite-vec 向量索引

说明：这一层服务镜子、偏好和长期记忆召回，不与运行态主表混写。

### 3.3 前馈配置与版本层

用于存储：

- `skill_manifests`
- `blueprint_definitions`
- `prompt_template_versions`

说明：这一层保存执行前的配置资产、版本和来源，方便审计和回放。

### 3.4 Trace / Eval 结果层

用于存储：

- `trace_records`
- `eval_snapshots`

说明：这一层服务运行追踪、质量评估和回放，不应该为前端页面直接承担主业务真源角色。

### 3.5 文件与机密层

- Workspace：工作区文件
- Artifact：正式产物
- Stronghold：密钥与敏感配置

补充约束：

- Stronghold 必须独立于普通设置快照与业务状态存储路径；
- 当前正式 Stronghold backend 使用专用机密存储路径；Windows 上由 DPAPI-backed secret store 承担正式实现，若正式 backend 打开失败，SQLite fallback 只能作为开发态兼容兜底，不得继续冒充正式真源；
- 模型 API Key、长期令牌和其他敏感配置不能继续作为普通 `settings` 字段或环境变量的正式真源；
- `settings.get / settings.update` 只能暴露脱敏状态，例如“是否已配置”，不能回传真实 secret。

---

## 4. 建模原则

1. `task` 是正式交付与前端展示主对象。
2. `run` 是后端执行对象，必须与 `task` 稳定映射。
3. `TodoItem` 与 `Task` 必须分层。
4. `RecurringRule` 描述周期规则，不直接等于任务实例。
5. 记忆检索不得污染运行态主表。
6. Trace / Eval 不得直接写进 `tasks` 主表。
7. 插件、多模型、Skills、Blueprint、Prompt 模板必须有独立表和版本。
8. 所有表名、字段名使用 `snake_case`。

---

## 5. 核心实体与关系

### 5.1 对外 API 契约对象与聚合视图

- `Task`
- `TaskStep`
- `BubbleMessage`
- `DeliveryResult`
- `Artifact`
- `TodoItem`
- `RecurringRule`
- `ApprovalRequest`
- `AuthorizationRecord`
- `AuditRecord`
- `ImpactScope`
- `RecoveryPoint`
- `TokenCostSummary`
- `MirrorReference`
- `SettingsSnapshot`
- `SettingItem`
- `AsyncJob`（预留）

补充说明：

- `BubbleMessage`、`ImpactScope`、`TokenCostSummary`、`SettingsSnapshot`、`SettingItem` 属于协议层聚合视图，不对应独立持久化主表。
- `AsyncJob` 当前仅保留在协议类型层，尚未接入 stable RPC 方法或结构化存储真源，不能误写成已落地的一等数据表。

### 5.2 后端执行与兼容对象

- `Session`
- `Run`
- `Step`
- `Event`
- `ToolCall`
- `Citation`
- `MemorySummary`
- `MemoryCandidate`
- `RetrievalHit`
- `SkillManifest`
- `BlueprintDefinition`
- `PromptTemplateVersion`
- `TraceRecord`
- `EvalSnapshot`
- `PluginManifest`
- `PluginRuntimeState`
- `PluginMetricSnapshot`

### 5.3 数据真源约束

- 对外产品界面、仪表盘、气泡、正式交付与安全摘要统一围绕 `task_id` 组织。
- 后端 Harness 内核保留 `run`、`step`、`event`、`tool_call` 作为执行链路对象。
- `task` 与 `run` 必须存在稳定映射关系，不允许各自漂移。
- 轻量承接对象与正式交付对象必须分层：`BubbleMessage` 负责承接提示，`DeliveryResult` 负责正式交付。
- `todo_items` 与 `tasks` 必须分层：前者表示未来安排或巡检事项，后者表示已进入执行。
- `recurring_rules` 只描述重复规则和提醒逻辑，不直接等于运行中的任务实例。

### 5.4 关键关系

- Session 拥有多个 Task，并允许在执行层映射到多个 Run。
- 一个 Task 必须映射到一个主 Run，允许存在内核级子步骤、SubAgent 子任务和工具调用。
- Task 拥有多个 TaskStep。
- Run 拥有多个 Step、Event、ToolCall。
- Task 可生成多个 Artifact，并关联 ApprovalRequest、AuthorizationRecord、AuditRecord、RecoveryPoint。
- DeliveryResult 是任务正式交付对象；BubbleMessage 是轻量承接对象。
- TodoItem 与 Task 明确分层：TodoItem 表示未来安排或巡检事项，Task 表示已进入执行。
- RecurringRule 描述重复规则，不直接等同于任务实例。
- TodoItem 可关联一个或多个 RecurringRule，也可在人工确认后转换为 Task。
- `notes` 详情补强继续挂在既有 `todo_items / recurring_rules` 语义上推进，不新增独立 `note_details` 或平行读模型真源表。
- notes 详情当前已经通过 `TodoItem` 协议投影稳定暴露 `note_text / prerequisite / related_resources / linked_task_id` 等字段；底层仍继续在既有 `todo_items / recurring_rules` 上承接 `planned_at / repeat_rule_text / next_occurrence_at / recent_instance_status / effective_scope` 等补强语义。
- notes 动作底座（complete / cancel / restore / toggle-recurring / delete）应直接更新既有 `todo_items / recurring_rules` 生命周期，不得绕过它们直接改写 `tasks`。
- MemorySummary、MemoryCandidate、RetrievalHit 通过引用关联 Task 与 Run，不混存原始运行态。
- SkillManifest、BlueprintDefinition、PromptTemplateVersion、PluginManifest 与具体 Run 之间必须可追踪，便于 Trace / Eval、回放和问题定位。
- TraceRecord、EvalSnapshot、Hook 记录、审查结果和熔断事件应与 Task / Run / Step 形成稳定引用关系。

### 5.5 表总览与职责矩阵

| 表名 | 角色 | 主要读者 | 主要写者 | 说明 |
| --- | --- | --- | --- | --- |
| `sessions` | 会话容器 | 前端首页、后端编排 | Orchestrator | 组织一组任务与运行对象 |
| `tasks` | 产品主对象 | 前端任务页、交付页 | Orchestrator / Delivery | 所有正式任务展示主表 |
| `task_steps` | 前端阶段步骤 | 任务详情页 | Orchestrator | 面向用户可见时间线 |
| `todo_items` | 巡检事项 | 巡检页、仪表盘 | Inspector | 尚未转为正式任务的事项 |
| `recurring_rules` | 周期规则 | 巡检引擎 | Inspector | 重复事项与提醒规则 |
| `runs` | 执行主对象 | 后端编排、排障 | RunEngine | 与 `task` 一一映射 |
| `steps` | 内部执行步骤 | 后端排障、回放 | RunEngine | 支撑 ReAct 细粒度步骤 |
| `events` | 事件流 | 订阅、回放、排障 | RunEngine / Hooks | 关键状态变化 |
| `tool_calls` | 工具调用 | 排障、审计 | Tool Router | worker / sidecar 输出统一入口 |
| `delivery_results` | 正式交付 | 前端承接层 | Delivery | 统一结果出口 |
| `artifacts` | 正式产物 | 前端结果页 | Delivery / Workers | 文档、文件、结构化结果 |
| `approval_requests` | 待授权 | 安全页 | RiskEngine | 高风险动作预执行对象 |
| `authorization_records` | 授权决策 | 安全页、审计 | RiskEngine | 用户允许 / 拒绝结果 |
| `audit_records` | 审计留痕 | 安全页、排障 | RiskEngine / Tool Router | 记录真实动作 |
| `recovery_points` | 恢复点 | 安全页、恢复流程 | Checkpoint | 回滚锚点 |
| `memory_summaries` | 长期记忆 | 镜子、Memory | Memory Engine | 正式长期记忆 |
| `memory_candidates` | 候选记忆 | Memory 审查流程 | Memory Engine | 候选层 |
| `retrieval_hits` | 召回命中 | 排障、Eval | Memory Engine | 一次任务中的命中记录 |
| `skill_manifests` | Skill 配置 | 前馈层 | Skill Loader | Skill 元数据和版本 |
| `blueprint_definitions` | 蓝图定义 | 前馈层 | Blueprint Loader | 计划模板 |
| `prompt_template_versions` | Prompt 模板 | 前馈层 | Prompt Composer | Prompt 版本 |
| `plugin_manifests` | 插件静态资产 | 扩展层、仪表盘 | Plugin Manager | 插件版本、来源、权限、runtime 关联 |
| `settings_snapshots` | 普通设置快照 | 设置中心 | Settings Manager | `general / floating_ball / memory / task_automation / models` 正式非敏感设置 |
| `trace_records` | 运行轨迹 | 排障、Eval | Trace Engine | LLM、规则、延迟、成本 |
| `eval_snapshots` | 评估快照 | Eval、回放 | Eval Engine | 质量和回归结果 |

### 5.6 Notes detail enrichment guidance

- 当前 notes 详情补强属于 owner-5 数据底座准备工作，而不是新增协议对象。
- 运行态可以先维护 richer note metadata，RPC 暴露前必须由 4 号在 `packages/protocol` 冻结字段与方法。
- 如果后续需要持久化 notes 详情补强字段，应扩展既有 `todo_items / recurring_rules` 存储结构，而不是新建平行真源。

---

## 6. 功能模块与数据表映射

### 6.1 入口承接与意图确认
- 主要表：`sessions / tasks / task_steps`
- 关键字段：`tasks.request_source / request_trigger / intent_name / intent_arguments_json`
- 作用：承接近场输入并形成正式 `task` 主锚点。

### 6.2 任务巡检转任务
- 主要表：`todo_items / recurring_rules / tasks / task_steps`
- 关键字段：`todo_items.bucket / status / due_at / source_path`，`recurring_rules.rule_type / cron_expr / interval_* / reminder_strategy / repeat_rule_text / next_occurrence_at`
- 作用：把巡检来源事项与正式任务分层管理。

### 6.3 任务执行与结果交付
- 主要表：`tasks / runs / steps / events / tool_calls / delivery_results / artifacts`
- 关键字段：`tasks.primary_run_id`，`runs.task_id`，`delivery_results.type`，`artifacts.artifact_type`
- 作用：维持 task-centric 与 run-centric 双层对象的稳定映射。

### 6.4 记忆写入与本地检索
- 主要表：`memory_summaries / memory_candidates / retrieval_hits / trace_records`
- 关键字段：`memory_summaries.category`，`memory_candidates.review_status`，`retrieval_hits.score / source`
- 作用：实现记忆写入、召回、命中记录与排障追踪。

### 6.5 风险授权与恢复回滚
- 主要表：`approval_requests / authorization_records / audit_records / recovery_points`
- 关键字段：`approval_requests.risk_level / status / impact_scope_json`，`authorization_records.decision`
- 作用：构成高风险动作授权、审计和恢复闭环。

### 6.6 仪表盘订阅与任务视图刷新
- 主要表：`tasks / task_steps / events / delivery_results / artifacts / approval_requests / trace_records`
- 关键字段：`events.type / payload_json`
- 作用：为任务列表、详情、安全摘要和结果区提供统一真源和事件驱动刷新。
- 当前 owner-5 P0-2 迁移策略采用 `task_runs` 兼容快照与 `tasks / task_steps` 一等记录双写；任务列表、详情、安全摘要和部分 overview 已优先读取 `tasks / task_steps / runs / events / tool_calls / delivery_results / approval_requests / authorization_records / audit_records / recovery_points`，`task_runs` 只继续回填 citation / steering / mirror 等尚未完全结构化的兼容字段。

### 6.7 结果审查、Trace 与熔断
- 主要表：`trace_records / eval_snapshots / events / tool_calls / audit_records`
- 关键字段：`trace_records.loop_round / rule_hits_json / review_result`，`eval_snapshots.metrics_json`
- 作用：支撑排障、熔断、回归和评估链路。

### 6.8 主动推荐与上下文感知
- 主要表：`tasks / events / trace_records`
- 关键字段：`tasks.request_source / request_trigger`
- 作用：当前阶段不新增独立主表，通过事件和 Trace 先承接推荐和感知结果。

### 6.9 扩展能力中心与多模型配置
- 主要表：`skill_manifests / blueprint_definitions / prompt_template_versions / plugin_manifests / trace_records / eval_snapshots`
- 作用：保证前馈配置与扩展能力可版本化、可追踪，并能回溯某次 `task / run` 实际命中的配置与插件资产版本。

### 6.10 设置中心与普通设置持久化
- 主要表：`settings_snapshots`
- 关键字段：`snapshot_key / snapshot_json / updated_at`
- 作用：持久化普通非敏感设置快照；模型密钥仍只能进入 Stronghold，不得回落到普通 settings 真源。

## 7A. 关键字段说明与表级语义

本节用于补足架构总览 / 旧架构文档中“关键字段说明”的数据视角版本。  
其目的不是重复 DDL，而是解释**哪些字段决定主链路语义、为什么这些字段不能乱用**。

### 7A.1 tasks

- `task_id`：前端、仪表盘、正式交付和安全摘要统一围绕它组织，是产品主锚点。
- `session_id`：用于把一组任务组织在同一个协作上下文内，不等于 run 容器。
- `status`：对外任务状态真源，必须来自统一 `task_status`，不能直接复用 run 内核状态。
- `request_source / request_trigger`：记录入口来源与触发方式，用于回溯入口承接、推荐触发和桌面交互动作。
- `intent_name / intent_arguments_json`：记录已确认意图及其结构化参数，不得写入仅供临时推测的草稿意图。
- `primary_run_id`：task 与 run 的主映射锚点，保证任务页、执行排障和结果交付之间关系稳定。
- `risk_level`：表达当前任务的总体风险等级，用于安全摘要和治理入口，不等于单个动作的瞬时风险判断。

### 7A.2 todo_items / recurring_rules

- `todo_items.item_id`：巡检事项主键，表示尚未进入正式执行态的未来安排。
- `todo_items.bucket`：事项桶位，区分 `upcoming / later / recurring_rule / closed` 这类用户可见分组。
- `todo_items.status`：事项状态，不得直接映射成 `task.status`。
- `todo_items.source_bucket / previous_bucket / previous_due_at / previous_status`：记录事项最近一次关闭前的可恢复位置、计划时间和状态，用于重启后仍可正确 restore。
- `todo_items.note_text / note_text_origin / prerequisite / planned_at / ended_at / related_resources_json / linked_task_id`：承接 notes 详情补强字段，并投影到当前稳定 `TodoItem` 协议对象；其中 `note_text_origin` 用于区分用户正文和运行时补出的展示文案，避免重启后把 display-only 文本误当成正式 task 输入。
- `todo_items.linked_task_id`：事项被升级为正式任务后才允许写入，用于建立来源追踪。
- `todo_items.source_path / source_line`：用于把巡检结果精确回链到 Markdown 来源。
- `recurring_rules.rule_type / cron_expr / interval_*`：用于描述规则引擎侧的重复规则计算输入，不直接等价于前端展示文本。
- `recurring_rules.repeat_rule_text / next_occurrence_at / recent_instance_status / effective_scope / enabled`：用于支撑前端便签详情与 `TodoItem.repeat_rule / next_occurrence_at / recent_instance_status / effective_scope / recurring_enabled` 投影。
- `recurring_rules.reminder_strategy`：定义到期前提醒、错过提醒、静默巡检等策略，是巡检规则的一部分，而不是任务执行逻辑。

### 7A.3 runs / steps

- `run_id`：后端执行主键，与 `task_id` 形成稳定主映射。
- `runs.task_id`：唯一约束保证一条正式 task 对应一个主 run。
- `runs.source_type`：执行来源，用于和前端请求来源、巡检来源和推荐来源回溯。
- `steps.order_index`：定义 ReAct / Planning Loop 的顺序，不用于前端直接展示。
- `steps.input_summary / output_summary`：服务编排排障和执行回放，前端阶段时间线优先看 `task_steps`。

### 7A.4 events

- `type`：必须是 `dot.case`，用于统一通知、订阅、审计和排障事件命名。
- `payload_json`：事件的结构化载荷，必须能映射到明确 schema，不能成为任意塞字段的容器。
- `step_id`：允许为空，用于挂接任务级事件和步骤级事件。
- `level`：用于区分 info / warning / error 等事件重要性，支持前端摘要和告警策略。

### 7A.5 tool_calls

- `tool_name`：必须使用 `snake_case`，统一工具层命名。
- `input_json / output_json`：保留原始输入输出，用于回放、排障与审查。
- `error_code`：统一错误码，便于前端和审查链路识别失败类型。
- `duration_ms`：记录耗时，用于性能分析、超时治理和工具质量评估。

### 7A.6 delivery_results / artifacts

- `delivery_results.type`：交付类型，必须来自统一 `delivery_type`，决定结果承载位置。
- `delivery_results.payload_json`：描述结果如何被打开、展示或跳转，是正式交付结构，不是工具原始输出。
- `delivery_results.preview_text`：供气泡、仪表盘或任务摘要使用的短预览。
- `delivery_results.run_id`：标记本次正式交付属于哪个执行尝试；同一 `task_id` 发生 `restart` 后，任务详情必须优先读取当前 `run_id` 的交付结果，不能把上一轮执行的结果继续当作当前结果展示。
- `artifacts.artifact_type`：产物类型，统一使用 `snake_case`。
- `artifacts.path / mime_type`：分别描述文件位置与媒体类型。
- `artifacts.delivery_type / delivery_payload_json`：用于把 artifact 与正式 `delivery_result` 打开语义稳定关联，支持任务详情、artifact 列表与统一打开动作复用同一套结果结构。
- `artifacts.run_id`：标记本次产物属于哪个执行尝试；restart 后任务详情中的 evidence / artifact 承接必须跟随当前 `run_id`，避免旧尝试产物污染当前尝试的正式视图。

### 7A.7 approval_requests / authorization_records / audit_records / recovery_points

- `approval_requests.risk_level`：表达单次待授权动作的风险等级。
- `approval_requests.impact_scope_json`：用于展示影响范围和边界命中结果，是前端“为什么要确认”的核心依据。
- `authorization_records.decision`：记录 allow_once / deny_once 等正式决策，不得只在任务状态上体现。
- `authorization_records.run_id`：标记授权决策属于哪个执行尝试；同一 `task_id` 发生 `restart` 后，任务详情必须优先读取当前 `run_id` 的授权记录，不能把上一轮尝试的 allow / deny 结果继续展示为当前授权状态。
- `audit_records.action / target / result`：记录真实发生的关键动作与执行结果。
- `audit_records.run_id`：标记审计记录来自哪个执行尝试；同一任务重启后，当前任务详情和失败摘要必须只读取当前 `run_id` 的审计记录，避免延续上一轮失败结论。
- `recovery_points.objects_json`：用于定义恢复 / 回滚涉及的对象集合，是恢复锚点而不是普通日志。

### 7A.8 memory_summaries / memory_candidates / retrieval_hits

- `memory_summaries.category`：区分 preference / profile / stage_summary 等类别。
- `memory_summaries.lifecycle`：控制记忆生命周期，避免所有记忆默认长期保留。
- `memory_candidates.review_status`：控制候选记忆是否能够进入正式长期层。
- `retrieval_hits.score`：召回分数，仅用于排序和过滤，不直接进入运行态主表。
- `retrieval_hits.source`：区分 FTS、向量召回、结构化 KV 等来源，便于分析命中质量。

### 7A.9 skill_manifests / blueprint_definitions / prompt_template_versions / plugin_manifests

- `version`：前馈配置版本号，是回放、问题定位和评估的关键字段。
- `manifest_json / definition_json / template_body`：运行时装配真源，必须保持可审计和可回溯。
- `source / summary`：前馈配置资产的来源与摘要，用于后续审计、加载器诊断和边界预留。
- `variables_json`：Prompt 模板变量声明，预留后续参数化装配边界，但当前阶段不提前扩展生态 UI。
- `plugin_manifests.entry / capabilities_json / permissions_json / runtime_names_json`：插件静态资产真源，必须能稳定指向当前 runtime 读模型，而不是只保留运行时缓存。

### 7A.10 trace_records / eval_snapshots

- `loop_round`：当前 ReAct 循环轮次，是 Doom Loop 检测的重要依据。
- `llm_input_summary / llm_output_summary`：保留关键摘要，帮助排障而不强依赖原始长上下文。
- `asset_refs_json`：记录当前 `task / run` 实际命中的 Skill / Blueprint / Prompt / Plugin 资产版本，是回放和问题定位的关键字段。
- `rule_hits_json`：命中规则、Hook、审查项摘要。
- `review_result`：表达通过、失败或需人工升级的结果。
- `eval_snapshots.metrics_json`：保存评估维度与分数，是回归和质量分析的真源。

### 7A.10A settings_snapshots

- `snapshot_key`：当前阶段固定为单行普通设置快照锚点，不与 secret store 混写。
- `snapshot_json`：只保存 `general / floating_ball / memory / task_automation / models` 的非敏感设置；不得写入明文 API Key。
- `updated_at`：用于设置中心判断最新落盘时间与重启后的回填顺序。

---

## 7. 核心表与 DDL

> 说明：以下 DDL 基于 SQLite 语法，字段中文说明使用 SQL 注释表达。生产实现时可继续细化默认值、触发器与外键策略。

### 7.1 sessions

```sql
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,                 -- 会话ID
    title TEXT NOT NULL,                         -- 会话标题
    status TEXT NOT NULL,                        -- 会话状态
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL                     -- 更新时间
);
CREATE INDEX idx_sessions_updated_at ON sessions(updated_at DESC);
```

### 7.2 tasks

```sql
CREATE TABLE tasks (
    task_id TEXT PRIMARY KEY,                    -- 任务ID
    session_id TEXT NOT NULL,                    -- 所属会话ID
    title TEXT NOT NULL,                         -- 任务标题
    source_type TEXT NOT NULL,                   -- 任务来源类型
    status TEXT NOT NULL,                        -- 任务状态
    intent_name TEXT,                            -- 意图名称
    intent_arguments_json TEXT,                  -- 意图参数(JSON)
    current_step TEXT,                           -- 当前步骤
    risk_level TEXT NOT NULL,                    -- 风险等级
    request_source TEXT,                         -- 请求来源
    request_trigger TEXT,                        -- 触发动作
    started_at TEXT NOT NULL,                    -- 开始时间
    updated_at TEXT NOT NULL,                    -- 更新时间
    finished_at TEXT,                            -- 完成时间
    primary_run_id TEXT,                         -- 主run映射
    FOREIGN KEY(session_id) REFERENCES sessions(session_id)
);
CREATE INDEX idx_tasks_session_id ON tasks(session_id);
CREATE INDEX idx_tasks_status_updated_at ON tasks(status, updated_at DESC);
CREATE INDEX idx_tasks_primary_run_id ON tasks(primary_run_id);
```

### 7.3 task_steps

```sql
CREATE TABLE task_steps (
    step_id TEXT PRIMARY KEY,                    -- 任务步骤ID
    task_id TEXT NOT NULL,                       -- 所属任务ID
    name TEXT NOT NULL,                          -- 步骤名
    status TEXT NOT NULL,                        -- 步骤状态
    order_index INTEGER NOT NULL,                -- 顺序
    input_summary TEXT,                          -- 输入摘要
    output_summary TEXT,                         -- 输出摘要
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL,                    -- 更新时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_task_steps_task_order ON task_steps(task_id, order_index);
```

### 7.4 todo_items

```sql
CREATE TABLE todo_items (
    item_id TEXT PRIMARY KEY,                    -- 事项ID
    title TEXT NOT NULL,                         -- 事项标题
    bucket TEXT NOT NULL,                        -- 分桶(upcoming/later/recurring_rule/closed)
    status TEXT NOT NULL,                        -- 当前状态
    source_path TEXT,                            -- 来源文件路径
    source_line INTEGER,                         -- 来源行号
    source_bucket TEXT,                          -- 最近一次关闭前或原始开放桶位
    due_at TEXT,                                 -- 截止时间
    tags_json TEXT,                              -- 标签(JSON)
    agent_suggestion TEXT,                       -- Agent建议
    note_text TEXT,                              -- 备注正文/详情摘要
    note_text_origin TEXT,                       -- note_text 来源(user_provided/derived_default)
    prerequisite TEXT,                           -- 前置条件
    planned_at TEXT,                             -- 原计划时间
    previous_bucket TEXT,                        -- 最近一次关闭前桶位
    previous_due_at TEXT,                        -- 最近一次关闭前计划时间
    previous_status TEXT,                        -- 最近一次关闭前状态
    ended_at TEXT,                               -- 结束时间
    related_resources_json TEXT,                 -- 相关资料(JSON)
    linked_task_id TEXT,                         -- 转任务后的task_id
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL                     -- 更新时间
);
CREATE INDEX idx_todo_items_bucket_due_at ON todo_items(bucket, due_at);
CREATE INDEX idx_todo_items_linked_task_id ON todo_items(linked_task_id);
```

### 7.5 recurring_rules

```sql
CREATE TABLE recurring_rules (
    rule_id TEXT PRIMARY KEY,                    -- 规则ID
    item_id TEXT NOT NULL,                       -- 所属事项ID
    rule_type TEXT NOT NULL,                     -- 规则类型
    cron_expr TEXT,                              -- cron表达式
    interval_value INTEGER,                      -- 间隔值
    interval_unit TEXT,                          -- 间隔单位(day/week/month)
    reminder_strategy TEXT NOT NULL,             -- 提醒策略
    enabled INTEGER NOT NULL DEFAULT 1,          -- 是否启用
    repeat_rule_text TEXT,                       -- 规则展示文本
    next_occurrence_at TEXT,                     -- 下次发生时间
    recent_instance_status TEXT,                 -- 最近一次状态
    effective_scope TEXT,                        -- 生效范围
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL,                    -- 更新时间,
    FOREIGN KEY(item_id) REFERENCES todo_items(item_id)
);
CREATE INDEX idx_recurring_rules_item_id ON recurring_rules(item_id);
```

### 7.6 runs

```sql
CREATE TABLE runs (
    run_id TEXT PRIMARY KEY,                     -- 执行实例ID
    task_id TEXT NOT NULL,                       -- 对应任务ID
    session_id TEXT NOT NULL,                    -- 所属会话ID
    source_type TEXT NOT NULL,                   -- 运行来源
    status TEXT NOT NULL,                        -- run状态
    started_at TEXT NOT NULL,                    -- 开始时间
    finished_at TEXT,                            -- 完成时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id),
    FOREIGN KEY(session_id) REFERENCES sessions(session_id)
);
CREATE UNIQUE INDEX idx_runs_task_id ON runs(task_id);
CREATE INDEX idx_runs_session_id ON runs(session_id);
```

### 7.7 steps

```sql
CREATE TABLE steps (
    step_id TEXT PRIMARY KEY,                    -- 执行步骤ID
    run_id TEXT NOT NULL,                        -- 所属run
    task_id TEXT NOT NULL,                       -- 所属task
    loop_round INTEGER NOT NULL DEFAULT 0,       -- ReAct/Agent Loop 轮次
    name TEXT NOT NULL,                          -- 步骤名
    status TEXT NOT NULL,                        -- 步骤状态
    order_index INTEGER NOT NULL,                -- 顺序
    input_summary TEXT,                          -- 输入摘要
    output_summary TEXT,                         -- 输出摘要
    stop_reason TEXT,                            -- 当前步骤停止原因
    started_at TEXT NOT NULL,                    -- 开始时间
    completed_at TEXT,                           -- 完成时间
    planner_input TEXT,                          -- planner 输入摘要/原文
    planner_output TEXT,                         -- planner 输出摘要/原文
    observation TEXT,                            -- 最近观测
    tool_name TEXT,                              -- 本步主要工具
    tool_call_id TEXT,                           -- 关联 tool_call
    FOREIGN KEY(run_id) REFERENCES runs(run_id),
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_steps_run_order ON steps(run_id, order_index);
```

### 7.8 events

```sql
CREATE TABLE events (
    event_id TEXT PRIMARY KEY,                   -- 事件ID
    run_id TEXT NOT NULL,                        -- 所属run
    task_id TEXT NOT NULL,                       -- 所属task
    step_id TEXT,                                -- 所属step
    type TEXT NOT NULL,                          -- 事件类型(dot.case)
    level TEXT NOT NULL,                         -- 事件级别
    payload_json TEXT NOT NULL,                  -- 事件载荷(JSON)
    created_at TEXT NOT NULL,                    -- 事件时间
    FOREIGN KEY(run_id) REFERENCES runs(run_id),
    FOREIGN KEY(task_id) REFERENCES tasks(task_id),
    FOREIGN KEY(step_id) REFERENCES steps(step_id)
);
CREATE INDEX idx_events_task_time ON events(task_id, created_at DESC);
CREATE INDEX idx_events_type_time ON events(type, created_at DESC);
```

### 7.9 tool_calls

```sql
CREATE TABLE tool_calls (
    tool_call_id TEXT PRIMARY KEY,               -- 工具调用ID
    run_id TEXT NOT NULL,                        -- 所属run
    task_id TEXT NOT NULL,                       -- 所属task
    step_id TEXT,                                -- 所属step
    tool_name TEXT NOT NULL,                     -- 工具名(snake_case)
    status TEXT NOT NULL,                        -- 调用状态
    input_json TEXT NOT NULL,                    -- 输入(JSON)
    output_json TEXT,                            -- 输出(JSON)
    error_code INTEGER,                          -- 错误码
    duration_ms INTEGER NOT NULL DEFAULT 0,      -- 耗时
    created_at TEXT NOT NULL,                    -- 调用时间
    FOREIGN KEY(run_id) REFERENCES runs(run_id),
    FOREIGN KEY(task_id) REFERENCES tasks(task_id),
    FOREIGN KEY(step_id) REFERENCES steps(step_id)
);
CREATE INDEX idx_tool_calls_task_time ON tool_calls(task_id, created_at DESC);
CREATE INDEX idx_tool_calls_name_status ON tool_calls(tool_name, status);
```

### 7.10 delivery_results

```sql
CREATE TABLE delivery_results (
    delivery_result_id TEXT PRIMARY KEY,         -- 交付ID
    task_id TEXT NOT NULL,                       -- 所属task
    run_id TEXT,                                 -- 所属run
    type TEXT NOT NULL,                          -- 交付类型
    title TEXT NOT NULL,                         -- 标题
    payload_json TEXT NOT NULL,                  -- 交付载荷(JSON)
    preview_text TEXT,                           -- 预览文本
    created_at TEXT NOT NULL,                    -- 创建时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_delivery_results_task_time ON delivery_results(task_id, created_at DESC);
CREATE INDEX idx_delivery_results_task_run_time ON delivery_results(task_id, run_id, created_at DESC);
```

### 7.11 artifacts

```sql
CREATE TABLE artifacts (
    artifact_id TEXT PRIMARY KEY,                -- 产物ID
    task_id TEXT NOT NULL,                       -- 所属task
    run_id TEXT,                                 -- 所属run
    artifact_type TEXT NOT NULL,                 -- 产物类型
    title TEXT NOT NULL,                         -- 标题
    path TEXT NOT NULL,                          -- 文件路径
    mime_type TEXT NOT NULL,                     -- MIME类型
    delivery_type TEXT NOT NULL,                 -- 对齐 delivery_result.type
    delivery_payload_json TEXT NOT NULL,         -- 对齐 delivery_result.payload(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_artifacts_task_time ON artifacts(task_id, created_at DESC);
CREATE INDEX idx_artifacts_task_run_time ON artifacts(task_id, run_id, created_at DESC, artifact_id DESC);
CREATE INDEX idx_artifacts_type ON artifacts(artifact_type);
```

### 7.12 approval_requests

```sql
CREATE TABLE approval_requests (
    approval_id TEXT PRIMARY KEY,                -- 授权请求ID
    task_id TEXT NOT NULL,                       -- 所属task
    operation_name TEXT NOT NULL,                -- 操作名
    risk_level TEXT NOT NULL,                    -- 风险等级
    target_object TEXT NOT NULL,                 -- 目标对象
    reason TEXT NOT NULL,                        -- 触发原因
    status TEXT NOT NULL,                        -- 请求状态
    impact_scope_json TEXT,                      -- 影响范围(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL,                    -- 更新时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_approval_requests_task_status ON approval_requests(task_id, status);
```

`approval_requests.status` uses `pending / approved / denied / resolved`; `resolved`
closes a stale pending request that was replaced by a newer authorization cycle
without implying user approval or denial.

### 7.13 authorization_records

```sql
CREATE TABLE authorization_records (
    authorization_record_id TEXT PRIMARY KEY,    -- 授权记录ID
    task_id TEXT NOT NULL,                       -- 所属task
    run_id TEXT,                                 -- 所属run
    approval_id TEXT NOT NULL,                   -- 授权请求ID
    decision TEXT NOT NULL,                      -- allow_once / deny_once
    operator TEXT NOT NULL,                      -- 操作者
    remember_rule INTEGER NOT NULL DEFAULT 0,    -- 是否记忆规则
    created_at TEXT NOT NULL,                    -- 创建时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id),
    FOREIGN KEY(approval_id) REFERENCES approval_requests(approval_id)
);
CREATE INDEX idx_authorization_records_task_time ON authorization_records(task_id, created_at DESC);
CREATE INDEX idx_authorization_records_task_run_time ON authorization_records(task_id, run_id, created_at DESC);
```

### 7.14 audit_records

```sql
CREATE TABLE audit_records (
    audit_id TEXT PRIMARY KEY,                   -- 审计ID
    task_id TEXT NOT NULL,                       -- 所属task
    run_id TEXT,                                 -- 所属run
    type TEXT NOT NULL,                          -- 类型(file/web/command/system)
    action TEXT NOT NULL,                        -- 动作
    summary TEXT NOT NULL,                       -- 摘要
    target TEXT,                                 -- 目标对象
    result TEXT NOT NULL,                        -- 执行结果
    trace_id TEXT,                               -- 关联trace
    created_at TEXT NOT NULL,                    -- 创建时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_audit_records_task_time ON audit_records(task_id, created_at DESC);
CREATE INDEX idx_audit_records_task_run_time ON audit_records(task_id, run_id, created_at DESC);
```

### 7.15 recovery_points

```sql
CREATE TABLE recovery_points (
    recovery_point_id TEXT PRIMARY KEY,          -- 恢复点ID
    task_id TEXT NOT NULL,                       -- 所属task
    summary TEXT NOT NULL,                       -- 恢复点说明
    objects_json TEXT NOT NULL,                  -- 关联对象(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间
    FOREIGN KEY(task_id) REFERENCES tasks(task_id)
);
CREATE INDEX idx_recovery_points_task_time ON recovery_points(task_id, created_at DESC);
```

### 7.16 memory_summaries

```sql
CREATE TABLE memory_summaries (
    memory_id TEXT PRIMARY KEY,                  -- 记忆ID
    task_id TEXT,                                -- 来源task
    run_id TEXT,                                 -- 来源run
    category TEXT NOT NULL,                      -- 类别(preference/profile/stage_summary)
    summary TEXT NOT NULL,                       -- 记忆摘要
    source TEXT NOT NULL,                        -- 来源
    lifecycle TEXT NOT NULL,                     -- 生命周期
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL                     -- 更新时间
);
CREATE INDEX idx_memory_summaries_category_time ON memory_summaries(category, updated_at DESC);
```

### 7.17 memory_candidates

```sql
CREATE TABLE memory_candidates (
    memory_candidate_id TEXT PRIMARY KEY,        -- 候选ID
    task_id TEXT,                                -- 来源task
    run_id TEXT,                                 -- 来源run
    reason TEXT NOT NULL,                        -- 抽取原因
    summary TEXT NOT NULL,                       -- 候选摘要
    review_status TEXT NOT NULL,                 -- 审查状态
    created_at TEXT NOT NULL                     -- 创建时间
);
CREATE INDEX idx_memory_candidates_review ON memory_candidates(review_status, created_at DESC);
```

### 7.18 retrieval_hits

```sql
CREATE TABLE retrieval_hits (
    retrieval_hit_id TEXT PRIMARY KEY,           -- 命中ID
    task_id TEXT NOT NULL,                       -- 所属task
    run_id TEXT NOT NULL,                        -- 所属run
    memory_id TEXT NOT NULL,                     -- 命中的记忆ID
    score REAL NOT NULL,                         -- 命中分数
    source TEXT NOT NULL,                        -- 命中来源
    summary TEXT NOT NULL,                       -- 命中摘要
    created_at TEXT NOT NULL,                    -- 创建时间,
    FOREIGN KEY(memory_id) REFERENCES memory_summaries(memory_id)
);
CREATE INDEX idx_retrieval_hits_task_score ON retrieval_hits(task_id, score DESC);
```

### 7.19 skill_manifests

```sql
CREATE TABLE skill_manifests (
    skill_manifest_id TEXT PRIMARY KEY,          -- Skill Manifest ID
    name TEXT NOT NULL,                          -- Skill 名称
    version TEXT NOT NULL,                       -- 版本
    source TEXT NOT NULL,                        -- 来源
    summary TEXT NOT NULL,                       -- 摘要
    manifest_json TEXT NOT NULL,                 -- 配置(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL                     -- 更新时间
);
```

### 7.20 blueprint_definitions

```sql
CREATE TABLE blueprint_definitions (
    blueprint_definition_id TEXT PRIMARY KEY,    -- 蓝图定义ID
    name TEXT NOT NULL,                          -- 蓝图名称
    version TEXT NOT NULL,                       -- 版本
    source TEXT NOT NULL,                        -- 来源
    summary TEXT NOT NULL,                       -- 摘要
    definition_json TEXT NOT NULL,               -- 蓝图内容(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL                     -- 更新时间
);
```

### 7.21 prompt_template_versions

```sql
CREATE TABLE prompt_template_versions (
    prompt_template_version_id TEXT PRIMARY KEY, -- 模板版本ID
    template_name TEXT NOT NULL,                 -- 模板名称
    version TEXT NOT NULL,                       -- 版本
    source TEXT NOT NULL,                        -- 来源
    summary TEXT NOT NULL,                       -- 摘要
    template_body TEXT NOT NULL,                 -- 模板内容
    variables_json TEXT NOT NULL,                -- 模板变量(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间
    updated_at TEXT NOT NULL                     -- 更新时间
);
```

### 7.21A plugin_manifests

```sql
CREATE TABLE plugin_manifests (
    plugin_id TEXT PRIMARY KEY,                   -- 插件ID
    name TEXT NOT NULL,                           -- 插件名称
    version TEXT NOT NULL,                        -- 版本
    entry TEXT NOT NULL,                          -- 插件入口
    source TEXT NOT NULL,                         -- 来源
    summary TEXT NOT NULL,                        -- 摘要
    capabilities_json TEXT NOT NULL,              -- 能力声明(JSON)
    permissions_json TEXT NOT NULL,               -- 权限边界(JSON)
    runtime_names_json TEXT NOT NULL,             -- 关联runtime名称(JSON)
    created_at TEXT NOT NULL,                     -- 创建时间
    updated_at TEXT NOT NULL                      -- 更新时间
);
```

### 7.21B settings_snapshots

```sql
CREATE TABLE settings_snapshots (
    snapshot_key TEXT PRIMARY KEY,               -- 当前快照锚点
    snapshot_json TEXT NOT NULL,                 -- 普通设置快照(JSON)
    updated_at TEXT NOT NULL                     -- 更新时间
);
```

### 7.22 trace_records

```sql
CREATE TABLE trace_records (
    trace_id TEXT PRIMARY KEY,                   -- Trace ID
    task_id TEXT NOT NULL,                       -- 所属task
    run_id TEXT,                                 -- 所属run
    loop_round INTEGER NOT NULL DEFAULT 0,       -- 循环轮次
    llm_input_summary TEXT,                      -- 输入摘要
    llm_output_summary TEXT,                     -- 输出摘要
    latency_ms INTEGER NOT NULL DEFAULT 0,       -- 延迟
    cost REAL NOT NULL DEFAULT 0,                -- 成本
    asset_refs_json TEXT,                        -- 命中扩展资产(JSON)
    rule_hits_json TEXT,                         -- 规则命中(JSON)
    review_result TEXT,                          -- 审查结果
    created_at TEXT NOT NULL                     -- 创建时间
);
CREATE INDEX idx_trace_records_task_time ON trace_records(task_id, created_at DESC);
```

### 7.23 eval_snapshots

```sql
CREATE TABLE eval_snapshots (
    eval_snapshot_id TEXT PRIMARY KEY,           -- 评估快照ID
    trace_id TEXT NOT NULL,                      -- 关联trace
    task_id TEXT NOT NULL,                       -- 所属task
    status TEXT NOT NULL,                        -- 评估状态
    asset_refs_json TEXT,                        -- 命中扩展资产(JSON)
    metrics_json TEXT NOT NULL,                  -- 指标(JSON)
    created_at TEXT NOT NULL,                    -- 创建时间,
    FOREIGN KEY(trace_id) REFERENCES trace_records(trace_id)
);
CREATE INDEX idx_eval_snapshots_task_time ON eval_snapshots(task_id, created_at DESC);
```

---

## 8. FTS5 与向量索引

### 8.1 文本索引建议

```sql
CREATE VIRTUAL TABLE memory_summaries_fts USING fts5(
    memory_id UNINDEXED,
    summary,
    content=''
);
```

### 8.2 向量索引建议

向量索引由 sqlite-vec 维护，推荐字段：

- `memory_id`
- `embedding`
- `category`
- `updated_at`

### 8.3 为什么双索引并存

- FTS5 适合关键词和精确短语。
- sqlite-vec 适合语义相近召回。
- `retrieval_hits` 负责把两者命中结果重新组织为任务可追踪记录。

---

## 9. 约束规则

### 9.1 主键规则

- 所有主键统一使用 TEXT，避免前后端生成策略不一致。
- ID 前缀建议和对象名对应，例如 `task_`、`run_`、`tool_`、`audit_`。

### 9.2 外键与删除规则

- 运行态主链表优先使用逻辑删除或状态终止，不轻易物理删除。
- 记忆和评估数据可按策略归档，但不能随意删除任务主记录。
- 清理动作必须经过 Entropy Cleanup 或后台治理流程，不允许业务代码私自清表。

### 9.3 索引设计原则

- 所有列表主查询字段都要有索引。
- 高频联表锚点 `task_id / run_id / session_id` 必须有索引。
- 不为低频试验字段提前加过多索引。
- 索引变更必须同步更新数据设计文档。

### 9.4 字段说明要求

除了 SQL 注释外，每张关键表都必须至少写明：

1. 表是干什么的；
2. 谁写它；
3. 谁读它；
4. 它和哪条链路相关；
5. 如果字段不足以让人理解，就必须补正文说明，而不能只放裸 DDL。

### 9.5 时间字段

- 统一使用 ISO 8601 字符串。
- 所有正式表至少有 `created_at`。
- 更新型表必须有 `updated_at`。

### 9.6 JSON 字段

以下字段统一保存 JSON 字符串：

- `intent_arguments_json`
- `payload_json`
- `input_json`
- `output_json`
- `impact_scope_json`
- `objects_json`
- `manifest_json`
- `blueprint_json`
- `rule_hits_json`
- `metrics_json`
- `tags_json`

### 9.7 业务约束

- `tasks.primary_run_id` 必须能追踪到 `runs.run_id`。
- `runs.task_id` 唯一，保证一 `task` 一主 `run`。
- `events.type` 必须是 `dot.case`。
- `tool_calls.tool_name` 必须是 `snake_case`。
- `delivery_results.type` 必须来自 `delivery_type`。
- 记忆命中记录不得直接更新 `tasks.status`。
- worker / sidecar / plugin 输出若要落盘，必须先经过 `tool_calls / events / delivery_results / artifacts` 对象链，不能自创主表。
- `todo_items.linked_task_id` 只能在事项已转换为正式任务后写入。
- `recurring_rules` 不得直接驱动 `runs`，必须先经过事项或任务编排层。

---

## 10. 建表顺序建议

1. `sessions`
2. `tasks`
3. `task_steps`
4. `todo_items`
5. `recurring_rules`
6. `runs`
7. `steps`
8. `events`
9. `tool_calls`
10. `delivery_results`
11. `artifacts`
12. `approval_requests`
13. `authorization_records`
14. `audit_records`
15. `recovery_points`
16. `memory_candidates`
17. `memory_summaries`
18. `retrieval_hits`
19. `skill_manifests`
20. `blueprint_definitions`
21. `prompt_template_versions`
22. `plugin_manifests`
23. `settings_snapshots`
24. `trace_records`
25. `eval_snapshots`

---

## 11. 演进规则

1. 新表必须说明：用途、主键、外键、索引、读写方、生命周期。
2. 新字段必须说明：是否进入协议、是否进入查询条件、是否需要索引。
3. DDL 变更必须同步回写数据设计文档。
4. 若只是新增一个展示字段，但没有对象语义说明，不允许直接入库。
5. 表设计若难以直观理解，优先补说明，不优先继续拆文档。

## 12. 禁止事项

- 不允许在业务代码里直接拼 DDL。
- 不允许前端自创数据库字段。
- 不允许 worker 自行定义一套持久化主表。
- 不允许把记忆、Trace、前馈配置混入 `tasks` 主表。
- 不允许未登记的索引或约束直接合入主干。
- 不允许为了“先跑起来”跳过表用途说明、索引说明和数据归属说明。
