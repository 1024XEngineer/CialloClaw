# CialloClaw 开发统一规范（修订版 v10）

说明：本文件为仓库内归档副本，依据用户提供的 v10 规范整理并同步到 `docs/`。后续若原文更新，应以最新架构设计文档与规范原文为准，再同步本副本。

---

## 1. 文档目标

本规范用于确保 CialloClaw 在多人协作 + AI 编码模式下，能够保持：

- 统一架构边界
- 统一协议、统一数据模型、统一命名
- 桌面端、后端服务、worker、共享包职责稳定
- AI 生成代码可控、可审查、可合并
- 主链路可运行、可演示、可扩展

规范优先级：

1. 最新架构设计文档中的架构边界与技术路线
2. 本开发统一规范
3. `/packages/protocol`
4. 模块局部 README / 注释 / 实现细节
5. 临时判断

---

## 2. 开发铁律

1. 不能直接拿 PRD 开写。
2. 进入大面积编码前，必须先冻结 7 个统一项。
3. 新增字段、事件、接口、错误码，必须先补文档与协议真源。
4. 平台相关代码不得直接写入业务层，必须经过抽象层。
5. 模型接入不得散落在业务逻辑中，必须通过统一 SDK 接入层进入。
6. 记忆检索不得直接侵入运行态状态机，必须经 Memory 内核统一接入。
7. AI 生成代码必须人工整理后才能合入。
8. 前端、后端、worker 不得分别维护同一领域对象的多套定义。

---

## 3. 开发前必须冻结的 7 个统一项

1. 统一目录结构
2. 统一命名规范
3. 统一主数据模型
4. 统一 JSON-RPC 协议
5. 统一错误码
6. 统一 Demo 主链路
7. 统一跨平台抽象接口

---

## 4. 命名规范

### 4.1 总原则

- 一个概念全仓库只能有一个名字。
- UI 名称、协议名称、数据库字段、事件类型必须能互相映射。
- 对外产品层使用 `task` 作为主对象；后端内核兼容层使用 `run` 作为执行对象；二者必须一一映射。
- 禁止无语义差异同义词混用。

### 4.2 前端命名

- 组件：`PascalCase`
- Hook：`useXxx`
- Store：`xxxStore.ts`
- feature 目录：`kebab-case`
- 变量 / 函数：`camelCase`
- 常量：`SCREAMING_SNAKE_CASE`

### 4.3 Go 命名

- 导出类型：`PascalCase`
- 包名：小写短名
- JSON 字段：`snake_case`
- 事件类型：`dot.case`
- JSON-RPC 方法组：`dot.case`
- tool 名称：`snake_case`

### 4.4 Worker 命名

- worker 名称：`snake_case`
- tool 名称：`snake_case`
- artifact 类型：`snake_case`
- provider 名称：`snake_case`

### 4.5 保留核心词

- `task`
- `task_step`
- `session`
- `run`
- `step`
- `event`
- `tool_call`
- `citation`
- `artifact`
- `delivery_result`
- `approval_request`
- `authorization_record`
- `audit_record`
- `recovery_point`
- `memory_summary`
- `memory_candidate`
- `retrieval_hit`

---

## 5. 状态统一

### 5.1 task_status

- `confirming_intent`
- `processing`
- `waiting_auth`
- `waiting_input`
- `paused`
- `blocked`
- `failed`
- `completed`
- `cancelled`
- `ended_unfinished`

### 5.2 其他冻结状态

- `task_list_group`: `unfinished`, `finished`
- `todo_bucket`: `upcoming`, `later`, `recurring_rule`, `closed`
- `risk_level`: `green`, `yellow`, `red`
- `security_status`: `normal`, `pending_confirmation`, `intercepted`, `execution_error`, `recoverable`, `recovered`
- `delivery_type`: `bubble`, `workspace_document`, `result_page`, `open_file`, `reveal_in_folder`, `task_detail`
- `voice_session_state`: `listening`, `locked`, `processing`, `cancelled`, `finished`
- `request_source`: `floating_ball`, `dashboard`, `tray_panel`
- `request_trigger`: `voice_commit`, `hover_text_input`, `text_selected_click`, `file_drop`, `error_detected`, `recommendation_click`
- `input_type`: `text`, `text_selection`, `file`, `error`
- `input_mode`: `voice`, `text`
- `task_source_type`: `voice`, `hover_input`, `selected_text`, `dragged_file`, `todo`, `error_signal`
- `bubble_message_type`: `status`, `intent_confirm`, `result`
- `approval_decision`: `allow_once`, `deny_once`
- `approval_status`: `pending`, `approved`, `denied`
- `settings_scope`: `all`, `general`, `floating_ball`, `memory`, `task_automation`, `data_log`
- `apply_mode`: `immediate`, `restart_required`, `next_task_effective`
- `theme_mode`: `follow_system`, `light`, `dark`
- `position_mode`: `fixed`, `draggable`
- `task_step_status`: `pending`, `running`, `completed`, `failed`, `skipped`, `cancelled`
- `step_status`: `pending`, `running`, `completed`, `failed`, `skipped`, `cancelled`
- `todo_status`: `normal`, `due_today`, `overdue`, `completed`, `cancelled`
- `recommendation_scene`: `hover`, `selected_text`, `idle`, `error`
- `recommendation_feedback`: `positive`, `negative`, `ignore`
- `task_control_action`: `pause`, `resume`, `cancel`, `restart`
- `time_unit`: `minute`, `hour`, `day`, `week`
- `run_status`: `processing`, `completed`

---

## 6. 主数据模型真源

主数据模型唯一真源位于：

- `/packages/protocol/types`
- `/packages/protocol/schemas`

统一核心实体包括：

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
- `AsyncJob`
- `Session`
- `Run`
- `Step`
- `Event`
- `ToolCall`
- `Citation`
- `MemorySummary`
- `MemoryCandidate`
- `RetrievalHit`

要求：

- `Task` 是前端与正式交付视角主对象。
- `Run` / `Step` / `Event` / `ToolCall` 是后端执行兼容层对象。
- `Task` 与 `Run` 必须双向可映射。
- 对外产品界面统一围绕 `task_id` 组织。
- `TodoItem` 与 `Task` 必须分层。
- 记忆检索结果不得混入运行态原始状态表。

---

## 7. JSON-RPC 协议

### 7.1 协议原则

- 前后端统一使用 JSON-RPC 2.0。
- 传输层统一定义为本地受控 IPC / 流式通信。
- P0 优先支持：Named Pipe（Windows）、本地 IPC、Unix Domain Socket、受控 WebSocket。
- `localhost HTTP` 仅作为调试态或兼容态保留，不是主承诺传输方式。

### 7.2 stable 方法集合

- `agent.input.submit`
- `agent.task.start`
- `agent.task.confirm`
- `agent.recommendation.get`
- `agent.recommendation.feedback.submit`
- `agent.task.list`
- `agent.task.detail.get`
- `agent.task.control`
- `agent.task_inspector.config.get`
- `agent.task_inspector.config.update`
- `agent.task_inspector.run`
- `agent.notepad.list`
- `agent.notepad.convert_to_task`
- `agent.dashboard.overview.get`
- `agent.dashboard.module.get`
- `agent.mirror.overview.get`
- `agent.security.summary.get`
- `agent.security.pending.list`
- `agent.security.respond`
- `agent.settings.get`
- `agent.settings.update`

### 7.3 返回规则

- 任务类接口返回：`task`、`delivery_result`，必要时附带 `bubble_message`
- 列表类接口返回：`items`、`page`
- 安全类接口返回：`approval_request` / `authorization_record` / `audit_record` / `recovery_point`，必要时附带 `impact_scope`
- 设置类接口返回：`effective_settings` 或 `setting_item`、`apply_mode`、`need_restart`

---

## 8. 错误码

### 8.1 分段

- `0`：成功
- `1001xxx`：Task / Session / Run / Step
- `1002xxx`：协议与参数
- `1003xxx`：工具调用
- `1004xxx`：权限与风险
- `1005xxx`：存储与数据库
- `1006xxx`：worker / sidecar
- `1007xxx`：系统与平台

### 8.2 推荐错误码

- `1001001` `TASK_NOT_FOUND`
- `1001002` `SESSION_NOT_FOUND`
- `1001003` `STEP_NOT_FOUND`
- `1001004` `TASK_STATUS_INVALID`
- `1001005` `TASK_ALREADY_FINISHED`
- `1001006` `RUN_NOT_FOUND`
- `1002001` `INVALID_PARAMS`
- `1002002` `INVALID_EVENT_TYPE`
- `1002003` `UNSUPPORTED_RESULT_TYPE`
- `1002004` `SCHEMA_VALIDATION_FAILED`
- `1002005` `JSON_RPC_METHOD_NOT_FOUND`
- `1003001` `TOOL_NOT_FOUND`
- `1003002` `TOOL_EXECUTION_FAILED`
- `1003003` `TOOL_TIMEOUT`
- `1003004` `TOOL_OUTPUT_INVALID`
- `1004001` `APPROVAL_REQUIRED`
- `1004002` `APPROVAL_REJECTED`
- `1004003` `WORKSPACE_BOUNDARY_DENIED`
- `1004004` `COMMAND_NOT_ALLOWED`
- `1004005` `CAPABILITY_DENIED`
- `1005001` `SQLITE_WRITE_FAILED`
- `1005002` `ARTIFACT_NOT_FOUND`
- `1005003` `CHECKPOINT_CREATE_FAILED`
- `1005004` `STRONGHOLD_ACCESS_FAILED`
- `1005005` `RAG_INDEX_UNAVAILABLE`
- `1006001` `WORKER_NOT_AVAILABLE`
- `1006002` `PLAYWRIGHT_SIDECAR_FAILED`
- `1006003` `OCR_WORKER_FAILED`
- `1006004` `MEDIA_WORKER_FAILED`
- `1007001` `PLATFORM_NOT_SUPPORTED`
- `1007002` `TAURI_PLUGIN_FAILED`
- `1007003` `DOCKER_BACKEND_UNAVAILABLE`
- `1007004` `SANDBOX_PROFILE_INVALID`
- `1007005` `PATH_POLICY_VIOLATION`

历史 `400xx` 风格错误码全部废弃。

---

## 9. Demo 主链路

P0 主链路统一为 task-centric：

用户输入或触发 → 悬浮球承接 → 返回意图或直接执行 → 气泡确认 → 前端创建或更新 task → Go service 执行工具链 → 命中风险时等待授权 → 生成 `delivery_result` 或 artifact → dashboard 展示 task / artifact / audit / recovery_point → 至少挂载一次记忆检索或记忆摘要。

---

## 10. 统一目录结构

```text
/apps
  /desktop
/services
  /local-service
/workers
  /playwright-worker
  /ocr-worker
  /media-worker
/packages
  /protocol
  /ui
  /config
/docs
  /architecture
  /protocol
  /demo
  /milestones
/scripts
  /dev
  /build
  /ci
```

要求：

- 协议只能放 `/packages/protocol`
- 错误码只能放 `/packages/protocol/errors`
- JSON-RPC 方法定义只能放 `/packages/protocol/rpc`
- 平台适配代码只能放 `/services/local-service/internal/platform` 或 Tauri 宿主层
- 模型 SDK 接入只能放 `/services/local-service/internal/model`

---

## 11. 跨平台抽象接口

必须抽象以下接口：

- `FileSystemAdapter`
- `PathPolicy`
- `OSCapabilityAdapter`
- `ExecutionBackendAdapter`
- `StorageAdapter`

阻断条件：

- 业务代码写死平台路径
- 路径拼接直接用字符串加法
- 平台逻辑写进 orchestrator / delivery / memory
- 未经过 `EnsureWithinWorkspace` 就写文件
- sidecar / worker 启动绕过平台抽象层

---

## 12. 模型与存储约束

### 12.1 模型接入

- 当前唯一标准：OpenAI 官方 Responses API SDK
- 接入位置：`/services/local-service/internal/model`
- 模型调用必须记录 `model`、`token_usage`、`latency`、`request_id`、`run_id`、`task_id`

### 12.2 存储分工

- SQLite + WAL：结构化运行态、审计、授权、checkpoint、事件索引、token 计量
- 本地记忆检索：SQLite FTS5 + sqlite-vec
- Workspace：生成文档、草稿、报告、导出文件、模板
- Artifact：截图、录屏、音视频中间产物、大对象外置文件
- Stronghold：密钥、令牌、敏感配置

---

## 13. AI 编码规则

AI 可以：

- 生成骨架
- 生成重复样板
- 补测试样例
- 补协议样例
- 补文档初稿

AI 禁止：

- 擅自新增目录
- 擅自新增状态名
- 擅自发明字段
- 擅自发明 JSON-RPC 方法
- 擅自新增错误码
- 擅自写平台专属路径逻辑
- 前后端各自定义一套 task / run 映射结构

AI 产出流程：

1. 先引用共享 schema
2. 再引用统一 Prompt 模板
3. 只生成当前模块最小闭环代码
4. 人工整理 import / 命名 / 注释 / 测试
5. 本地跑主链路相关验证
6. 通过后再提交

---

## 14. 自检清单

开始任何修改前，必须先检查：

- 是否触碰协议真源？
- 是否新增字段、状态、错误码、RPC 方法？
- 是否影响 task / run / step / event / tool_call 主模型？
- 是否影响主链路？
- 是否涉及平台抽象层？
- 是否涉及模型 SDK、Memory、Stronghold、RAG、SQLite？
- 是否把平台逻辑写进业务层？
- 是否引入未登记命名？
- 是否会造成前后端模型漂移？
