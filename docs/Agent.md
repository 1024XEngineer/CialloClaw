# Agent.md

本文件定义 CialloClaw 仓库内 Coding Agent 的统一执行规则。Agent 在修改代码、文档、协议、配置、目录结构前，必须先遵守本文件。若与局部实现、临时约定、个人习惯冲突，以本文件为准。规则来源于《CialloClaw 开发统一规范（修订版 v10）》的可执行整理版。

---

## 1. 身份与目标

你是 CialloClaw 仓库中的 Coding Agent。

你的目标不是“尽快写代码”，而是：

- 在统一架构边界内交付可合并代码；
- 维持统一协议、统一命名、统一模型、统一目录；
- 避免前端、后端、worker、共享层之间职责污染；
- 保证主链路始终可运行、可演示、可扩展；
- 保证 AI 生成结果可审查、可维护、可回滚。

---

## 2. 最高优先级规则

发生冲突时，按以下顺序决策：

1. 最新架构设计文档中的架构边界与技术路线
2. 本文件
3. `/packages/protocol`
4. 模块局部 README、注释、实现细节
5. 开发者或 Agent 的临时判断

禁止为了“先跑通”绕过高优先级规则。

---

## 3. 开发铁律

1. 不允许直接拿 PRD 开写，必须先对齐统一项。
2. 进入大面积编码前，必须先冻结以下 7 项：
   - 统一目录结构
   - 统一命名规范
   - 统一主数据模型
   - 统一 JSON-RPC 协议
   - 统一错误码
   - 统一 Demo 主链路
   - 统一跨平台抽象接口
3. 任何新增字段、事件、接口、错误码，必须先更新协议或规范，再写实现。
4. 任何平台相关代码不得直接写入业务层，必须经过抽象层。
5. 任何模型接入不得散落在业务逻辑中，必须通过统一模型接入层进入。
6. 任何记忆检索不得直接侵入运行态状态机，必须经 Memory 内核统一接入。
7. AI 生成代码必须人工整理后才能合入。
8. 前端、后端、worker 不得分别维护同一领域对象的多套定义。

---

## 4. 仓库边界与依赖方向

### 4.1 顶层目录职责

- `/apps/desktop`：Tauri + React 桌面前端
- `/services/local-service`：Go 本地服务 / JSON-RPC server / 编排内核
- `/workers/*`：外部能力进程，例如 playwright、ocr、media
- `/packages/protocol`：全仓唯一协议真源之一
- `/packages/ui`：共享 UI 组件
- `/packages/config`：共享配置
- `/docs`：架构、协议、Demo、里程碑文档
- `/scripts`：开发、构建、CI 脚本

### 4.2 正确依赖方向

- `/packages/*` 是底层共享定义层，可被所有模块依赖。
- `/services/local-service` 与 `/workers/*` 可依赖 `/packages/*`。
- `/apps/desktop` 可依赖 `/packages/*`，并通过 JSON-RPC 调用 `/services/local-service`。
- `/services/local-service` 可以编排 `/workers/*`。
- `/apps/desktop` 不能直接依赖 Go 服务内部实现。
- `/apps/desktop` 不能直接调用 worker。
- `/workers/*` 不能依赖前端页面逻辑，也不能依赖 Go 主链路内部状态机细节。

### 4.3 明确禁止

- 不允许新增与 `/apps`、`/services`、`/workers` 平级的临时业务目录。
- 不允许前后端各自复制一份协议类型。
- 不允许 worker 内嵌业务主链路状态模型。
- 不允许使用 `misc`、`common`、`temp`、`new` 等模糊正式目录承载业务代码。

---

## 5. 唯一真源

以下内容只能在指定目录定义：

- 主数据模型：`/packages/protocol/types` 与 `/packages/protocol/schemas`
- JSON-RPC 方法：`/packages/protocol/rpc`
- 错误码：`/packages/protocol/errors`
- 协议示例：`/packages/protocol/examples`

禁止：

- 前端私自定义 task/run/step/event/tool_call 主模型
- Go 服务绕过共享协议返回临时结构
- worker 返回结构绕过统一事件与交付体系

---

## 6. 核心对象命名与语义

以下词为保留核心词，不允许重命名、替换或发明同义词：

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

额外要求：

- `task` 是前端与正式交付视角的主对象。
- `run` 是后端执行内核对象。
- `task_id` 与 `run_id` 必须可追踪映射，不能各自漂移。
- 同一概念全仓只能有一个名字。
- 禁止无语义差异的同义词混用，例如：
  - `item` / `artifact`
  - `action` / `tool_call`
  - `memory_hit` / `retrieval_hit`
  - `rpc_method` / `api_name`

---

## 7. 命名规则

### 7.1 前端

- React 组件：`PascalCase`
- Hook：`useXxx`
- Store 文件：`xxxStore.ts`
- feature 目录：`kebab-case`
- TypeScript 类型：`PascalCase`
- 变量 / 函数：`camelCase`
- 常量：`SCREAMING_SNAKE_CASE`

### 7.2 Go

- 导出类型：`PascalCase`
- 包名：短小、全小写
- JSON 字段：`snake_case`
- 数据表 / 字段：`snake_case`
- 事件类型：`dot.case`
- JSON-RPC 方法组：`dot.case`
- tool 名称：`snake_case`

### 7.3 Worker

- worker 名称：`snake_case`
- tool 名称：`snake_case`
- artifact 类型：`snake_case`
- provider 名称：`snake_case`

---

## 8. 统一状态约束

对外产品态以 `task_status` 为主；`run_status` 仅保留后端最小兼容状态，不得替代 `task_status` 对外暴露。

文档中未登记的状态值不得进入实现。允许状态如下：

### 8.1 task_status

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

### 8.2 task_list_group

- `unfinished`
- `finished`

### 8.3 todo_bucket

- `upcoming`
- `later`
- `recurring_rule`
- `closed`

### 8.4 risk_level

- `green`
- `yellow`
- `red`

### 8.5 security_status

- `normal`
- `pending_confirmation`
- `intercepted`
- `execution_error`
- `recoverable`
- `recovered`

### 8.6 delivery_type

- `bubble`
- `workspace_document`
- `result_page`
- `open_file`
- `reveal_in_folder`
- `task_detail`

### 8.7 其他已冻结枚举

以下枚举仅允许使用既有值：

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

## 9. 主数据模型要求

核心实体固定如下：

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

建模要求：

- `Task` 是前端暴露主对象。
- `Run`、`Step`、`Event`、`ToolCall` 是后端执行兼容层对象。
- `Task` 与 `Run` 必须双向可映射。
- 对外任务界面、仪表盘、气泡、正式交付、审计摘要统一围绕 `task_id` 组织。
- `TodoItem` 与 `Task` 必须分层：前者表示未来安排，后者表示已进入执行。
- `RecurringRule` 只描述重复规则，不等同任务实例。
- 记忆检索结果只能通过引用关联，不得混入运行态原始状态表。

### 9.1 模型变更规则

新增字段、状态、实体前，必须先完成以下动作：

1. 更新 `/packages/protocol`
2. 明确用途
3. 明确关系
4. 给出最小字段
5. 标明是否进入 SQLite
6. 标明是否进入 RAG

禁止：

- 未冻结字段进入数据库和协议
- 前端为了展示方便私加源模型字段
- `Task` 与 `Run` 映射只存在于实现细节而不进入协议层

---

## 10. JSON-RPC 统一规则

### 10.1 协议标准

前后端统一通信协议为 JSON-RPC 2.0。

承载范围：

- request / response
- notification
- subscription

传输层 P0 仅允许本地受控通信，例如：

- Named Pipe（Windows）
- 本地 IPC
- Unix Domain Socket（macOS / Linux）
- WebSocket（仅限本地受控连接）

`localhost HTTP` 仅可作为调试态或兼容态，不是 P0 主承诺方式。

### 10.2 方法命名规则

- 所有 Agent 层 JSON-RPC 方法名必须使用 `dot.case`
- 所有 Notification 名称必须使用 `dot.case`
- 前端不得手写方法字符串，必须通过统一 RPC client 封装调用

### 10.3 stable 方法集合

当前正式联调范围仅包括：

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

planned 但当前不承诺联调的方法：

- `agent.security.audit.list`
- `agent.security.restore_points.list`
- `agent.security.restore.apply`
- `agent.mirror.memory.manage`
- `agent.task.artifact.list`
- `agent.task.artifact.open`
- `agent.delivery.open`

### 10.4 返回结构规则

- 任务类接口统一返回：`task`、`delivery_result`，必要时附带 `bubble_message`
- 列表类接口统一返回：`items`、`page`
- 安全类接口统一返回：`approval_request` / `authorization_record` / `audit_record` / `recovery_point`，必要时附带 `impact_scope`
- 设置类接口统一返回：`effective_settings` 或 `setting_item`、`apply_mode`、`need_restart`

### 10.5 严格禁止

- 不允许继续扩 REST 风格主协议
- 不允许前端直接消费临时 JSON
- 不允许通过字符串判断业务成功失败
- 不允许未登记方法直接进入实现

---

## 11. 错误码规则

### 11.1 分段

- `0`：成功
- `1001xxx`：Task / Session / Run / Step 错误
- `1002xxx`：协议与参数错误
- `1003xxx`：工具调用错误
- `1004xxx`：权限与风险错误
- `1005xxx`：存储与数据库错误
- `1006xxx`：worker / sidecar 错误
- `1007xxx`：系统与平台错误

### 11.2 推荐错误码

#### Task / Session / Run

- `1001001` `TASK_NOT_FOUND`
- `1001002` `SESSION_NOT_FOUND`
- `1001003` `STEP_NOT_FOUND`
- `1001004` `TASK_STATUS_INVALID`
- `1001005` `TASK_ALREADY_FINISHED`
- `1001006` `RUN_NOT_FOUND`

#### 协议与参数

- `1002001` `INVALID_PARAMS`
- `1002002` `INVALID_EVENT_TYPE`
- `1002003` `UNSUPPORTED_RESULT_TYPE`
- `1002004` `SCHEMA_VALIDATION_FAILED`
- `1002005` `JSON_RPC_METHOD_NOT_FOUND`

#### 工具调用

- `1003001` `TOOL_NOT_FOUND`
- `1003002` `TOOL_EXECUTION_FAILED`
- `1003003` `TOOL_TIMEOUT`
- `1003004` `TOOL_OUTPUT_INVALID`

#### 风险与权限

- `1004001` `APPROVAL_REQUIRED`
- `1004002` `APPROVAL_REJECTED`
- `1004003` `WORKSPACE_BOUNDARY_DENIED`
- `1004004` `COMMAND_NOT_ALLOWED`
- `1004005` `CAPABILITY_DENIED`

#### 存储

- `1005001` `SQLITE_WRITE_FAILED`
- `1005002` `ARTIFACT_NOT_FOUND`
- `1005003` `CHECKPOINT_CREATE_FAILED`
- `1005004` `STRONGHOLD_ACCESS_FAILED`
- `1005005` `RAG_INDEX_UNAVAILABLE`

#### Worker / Sidecar

- `1006001` `WORKER_NOT_AVAILABLE`
- `1006002` `PLAYWRIGHT_SIDECAR_FAILED`
- `1006003` `OCR_WORKER_FAILED`
- `1006004` `MEDIA_WORKER_FAILED`

#### 系统 / 平台

- `1007001` `PLATFORM_NOT_SUPPORTED`
- `1007002` `TAURI_PLUGIN_FAILED`
- `1007003` `DOCKER_BACKEND_UNAVAILABLE`
- `1007004` `SANDBOX_PROFILE_INVALID`
- `1007005` `PATH_POLICY_VIOLATION`

### 11.3 错误处理约束

- 前端只认错误码和错误类型，不猜字符串。
- 历史 `40001`、`400xx` 旧格式全部废弃。
- Go 返回错误时必须带 JSON-RPC `id` 或 `trace_id`。
- worker 错误必须包装成统一错误码。
- 新错误码必须先登记到 `/packages/protocol/errors`。

---

## 12. Demo 主链路

唯一 P0 主链路必须保持 task-centric：

用户输入或触发 → 悬浮球承接 → 返回意图或直接执行 → 气泡确认 → 前端通过 JSON-RPC 创建或更新 task → Go service 执行工具链 → 命中风险时等待授权 → 生成短结果或正式交付对象 → dashboard 展示 task / artifact / audit / recovery_point → 至少挂载一次记忆检索或记忆摘要。

缺少以下任一节点，都不算主链路完整：

1. 入口触发
2. 意图返回或直接执行判定
3. 气泡确认
4. `agent.task.start` 或等价 task 创建 / 更新
5. 工具链执行
6. 风险挂起与授权承接（如命中风险）
7. `delivery_result` 或 artifact 生成
8. dashboard 回显 task / artifact / audit / recovery_point
9. 至少一次记忆检索或记忆摘要挂载

---

## 13. 跨平台抽象规则

### 13.1 总原则

所有平台能力必须先定义抽象接口，再写平台实现。

禁止在业务逻辑中直接出现：

- `C:\`
- `D:\`
- `\\`
- `/Users/...`
- `/home/...`
- 平台专属路径拼接

### 13.2 必须存在的抽象

#### FileSystemAdapter

至少包含：

- `Join(...)`
- `Clean(path)`
- `Abs(path)`
- `Rel(base, target)`
- `Normalize(path)`
- `EnsureWithinWorkspace(path)`
- `ReadFile(path)`
- `WriteFile(path, content)`
- `Move(src, dst)`
- `MkdirAll(path)`

#### PathPolicy

负责：

- 屏蔽盘符差异
- 屏蔽路径分隔符差异
- 统一路径合法性校验
- 统一 workspace 边界校验

#### OSCapabilityAdapter

负责：

- 托盘
- 通知
- 快捷键
- 剪贴板
- 屏幕授权
- 外部命令启动
- sidecar 生命周期管理

#### ExecutionBackendAdapter

负责：

- Docker
- SandboxProfile
- ResourceLimit
- Remote Backend

#### StorageAdapter

负责：

- SQLite
- RAG 索引
- Artifact 外置存储
- Stronghold 机密存储

### 13.3 合并阻断条件

以下情况直接阻断合并：

- 业务代码写死平台路径
- 路径拼接直接使用字符串加法
- 平台逻辑直接写入 orchestrator / delivery / memory
- 未经过 `EnsureWithinWorkspace` 就写文件
- sidecar / worker 启动绕过平台抽象层

---

## 14. LLM / AI 服务接入规则

### 14.1 当前唯一实现标准

统一模型接入方式为 OpenAI 官方 Responses API SDK。

### 14.2 接入位置

模型 SDK 接入、模型配置和调用封装统一放在：

- `/services/local-service/internal/model`

业务模块只能通过该目录访问模型，不得在以下目录直接调用 SDK：

- `orchestrator`
- `intent`
- `memory`
- `delivery`
- 其他业务目录

### 14.3 强制要求

- 模型切换通过配置完成，主要项包括 `model_id`、`endpoint`、`api_key`、预算策略
- 所有模型入参与出参必须经过 `/packages/protocol` 建模
- tool calling 结果回填必须走统一 `ToolCall` / `Event` 体系
- 每次模型调用必须记录：
  - `model`
  - `token_usage`
  - `latency`
  - `request_id`
  - `run_id`
  - `task_id`
- 前端不得直接持有模型调用能力

### 14.4 禁止

- 禁止在业务层直接写 Responses SDK 调用
- 禁止不同模块各自封装一套模型 client
- 禁止模型输出未经 schema 校验直接透传前端
- 禁止把模型密钥写入 SQLite 或明文配置

---

## 15. SQLite + RAG + Workspace 存储规则

### 15.1 存储职责划分

#### SQLite + WAL

负责：

- 结构化运行态
- 审计
- 授权记录
- checkpoint
- 任务状态
- 事件索引
- token 计量

#### 本地记忆检索层

负责：

- SQLite FTS5 检索
- sqlite-vec 向量召回
- 候选过滤
- 排序与摘要回填

#### Workspace

负责：

- 生成文档
- 草稿
- 报告
- 导出文件
- 模板

#### Artifact

负责：

- 截图
- 录屏
- 音视频中间产物
- 大对象外置文件

#### Stronghold

负责：

- API Key
- 模型令牌
- 敏感配置
- 密钥材料

### 15.2 存储约束

- 结构化运行态不得写入 RAG
- 语义记忆不得直接塞进主状态表
- 大对象不得直接存入 SQLite
- 敏感配置不得落入 SQLite 或 workspace
- 所有写入必须带 task 关联信息、run 关联信息或来源信息

### 15.3 记忆检索约束

- 本地记忆检索只能通过 Memory 层调用
- 检索实现固定为 `SQLite FTS5 + sqlite-vec`
- 返回结构必须标准化为 `MemoryCandidate` / `RetrievalHit`
- 召回、过滤、排序必须与主状态机解耦

---

## 16. AI 编码约束

### 16.1 AI 可以做的事

- 生成模块骨架
- 生成重复样板代码
- 补测试样例
- 补协议样例
- 补文档初稿
- 补注释初稿

### 16.2 AI 禁止做的事

- 擅自新增目录
- 擅自新增状态名
- 擅自发明字段
- 擅自发明 JSON-RPC 方法
- 擅自新增错误码
- 擅自写平台专属路径逻辑
- 输出超长文件直接入主干
- 前后端各自定义一套 task / run 映射结构

### 16.3 AI 产出流程

1. 先引用共享 schema
2. 再引用统一 Prompt 模板
3. 只生成当前模块最小闭环代码
4. 人工整理 import、命名、注释、测试
5. 本地跑主链路相关验证
6. 通过后再提交

---

## 17. Coding Agent 执行清单

开始任何修改前，先检查：

- 是否触碰协议真源？
- 是否新增字段、状态、错误码、RPC 方法？
- 是否影响 task / run / step / event / tool_call 主模型？
- 是否影响主链路？
- 是否涉及平台抽象层？
- 是否涉及模型 SDK、Memory、Stronghold、RAG、SQLite？
- 是否把平台逻辑写进业务层？
- 是否引入了未登记的新命名？
- 是否会造成前后端模型漂移？

如果任一答案为“是”，必须先更新对应真源或抽象层，再写代码。

---

## 18. PR 自检要求

每次提交前，至少补充并自检以下事项：

- 本次修改影响哪一个统一项
- 是否新增字段 / 事件 / 方法 / 错误码
- 是否影响 Demo 主链路
- 是否涉及平台抽象层
- 是否涉及本地记忆检索 / 模型 SDK / Stronghold
- 是否由 AI 生成，人工整理点有哪些
- 最小验证说明是否完整

---

## 19. 一句话总纲

共享定义在最底层，后端 Harness 在中间编排，worker 提供外部能力，前端只能通过 JSON-RPC 协议调用后端；任何实现都不得突破这一边界。
