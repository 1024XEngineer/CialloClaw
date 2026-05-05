# CialloClaw 协议设计文档（v5）

## 1. 文档范围

本文档定义 CialloClaw 的正式协议边界，覆盖：

- JSON-RPC 2.0 通信规则
- 方法集合与生命周期
- Notification / Subscription 事件
- 正式状态枚举
- 正式错误码
- 请求 / 响应结构
- stable 接口详细定义
- planned 接口预留约束

本文档与 `/packages/protocol/rpc`、`/packages/protocol/types`、`/packages/protocol/schemas` 保持一致。若冲突，以仓库真源为准，随后回写本文档。

---

## 2. 协议边界、接入层与传输

### 2.1 总体边界

- 前端只通过 JSON-RPC 2.0 与后端通信。
- 后端是唯一对外协议出口。
- worker / sidecar / plugin 不直接暴露给前端。
- 前端不得 import Go 服务内部实现。

### 2.2 传输层

当前 P0 正式传输层：

- Windows：Named Pipe
- 调试态：本地 IPC / localhost HTTP（仅兼容，不是正式主承诺）
- 流式事件：Notification / Subscription

### 2.3 如何理解协议分层

1. 调用层：request / response  
   用于前端显式请求后端能力。
2. 推送层：notification  
   用于任务状态、正式交付、安全待确认等异步变化通知。
3. 订阅层：subscription  
   用于长生命周期任务和仪表盘持续刷新。
4. 结构层：types / schemas  
   用于冻结字段、对象和验证规则。
5. 约束层：状态、错误码、分页、返回规则  
   用于保证不同模块对同一对象的理解一致。

### 2.4 接口接入层职责

协议接入层在运行时承担三类职责：

- **JSON-RPC 2.0 Server**：作为前后端唯一稳定边界，负责解析请求、返回响应、生成标准错误结构。
- **Session / Task 接口承接**：把前端的输入、确认、查询、控制请求统一收口到 `task` 主对象体系，而不是让前端直接面向 `run / step / event`。
- **订阅 / 通知 / 事件流**：向前端推送 `task.updated`、`delivery.ready`、`approval.pending` 以及插件运行态事件，保证仪表盘、任务详情和安全卫士能同步刷新。

接口接入层的设计边界是：

- 前端不得跳过接口接入层去调用 worker / sidecar / plugin；
- worker / sidecar / plugin 的结果必须先进入 `tool_call / event / delivery_result` 链，再通过接口层向前端暴露；
- 接口层只做协议承接、结构校验和对象分发，不在这一层承载具体业务决策。


---

## 3. 命名、对象与方法组说明

### 3.1 方法命名

- 统一使用 `dot.case`
- 统一以 `agent.` 为业务前缀
- 例如：`agent.task.start`

### 3.2 事件命名

- Notification 统一使用 `dot.case`
- 例如：`task.updated`、`delivery.ready`

### 3.3 关键对象说明

- `task`：对外主对象，是前端任务列表、详情页、正式交付和安全摘要的统一锚点；当前允许后端写回隐藏协作会话的 `session_id`，用于延续同一轮桌面任务。
- `run`：执行对象，是后端编排和工具链的运行实例。
- `bubble_message`：轻量承接对象，用于状态反馈和短结果返回。
- `delivery_result`：正式交付对象，统一描述结果以气泡、文档、结果页、打开文件或任务详情交付。
- `artifact`：正式产物对象，例如 Markdown 文档、导出文件、截图、结构化结果。
- `approval_request`：待授权对象，高风险动作必须先落到这里。
- `audit_record`：审计对象，用于记录真实动作和结果。
- `memory_summary`：长期记忆对象，用于镜子、偏好、阶段总结。
- `plugin`：对外插件主对象，用于插件列表页和详情页承接插件基础信息、来源、权限与能力概览。
- `plugin_runtime_state`：插件运行态对象，用于展示运行状态、健康度、传输方式、最近可见时间和能力摘要。
- `plugin_metric_snapshot`：插件指标快照对象，用于展示启动次数、成功失败次数和最近运行时间。
- `plugin_runtime_event`：插件运行事件对象，用于展示最近状态变化和健康事件。
- `plugin_tool_contract`：插件工具合同对象，用于描述单个工具的输入合同、输出合同、风险提示与交付映射。
- `plugin_data_contract`：插件数据合同对象，用于冻结 `schema_ref / schema_json / fields` 三层展示结构。

### 3.4 方法族说明

- `agent.input.*`：近场承接入口，负责长按语音、悬停输入等。
- `agent.task.*`：任务生命周期方法，负责创建、详情、控制与产物查询。
- `agent.delivery.*`：正式交付结果的统一解析与打开。
- `agent.recommendation.*`：主动推荐与反馈。
- `agent.task_inspector.*`：巡检配置与执行。
- `agent.notepad.*`：事项与任务之间的桥接。
- `agent.dashboard.*`：仪表盘首页与模块取数。
- `agent.mirror.*`：镜子和长期记忆视图。
- `agent.security.*`：安全卫士、授权、审计、恢复。
- `agent.settings.*`：设置中心。
- `agent.screen.*`：屏幕感知与场景推荐分析。
- `agent.plugin.*`：插件扩展能力方法组，负责插件列表、详情与运行态查询。
- `agent.model.* / agent.skill.*`：扩展能力方法组，当前多数为 planned。

补充说明：

- 当前仓库实现里，部分场景感知信号也会通过 `agent.input.*`、`agent.task.*` 与 `agent.recommendation.*` 的 `context / scene` 字段并入主链。
- 多模型与 Skill 安装当前仍主要停留在路线图阶段，正式方法与协议资产后续仍需继续冻结。

---

## 4. 通用结构与阅读说明

### 4.1 请求结构

```json
{
  "jsonrpc": "2.0",
  "id": "req_xxx",
  "method": "agent.xxx.xxx",
  "params": {
    "request_meta": {
      "trace_id": "trace_xxx",
      "client_time": "2026-04-09T10:00:00+08:00"
	      }
  }
}
```

`request_meta` 是所有请求的统一链路头，至少用于：

- 端到端排查问题；
- 把前端请求与后端 trace / audit / eval 关联起来；
- 在失败时把 `trace_id` 原路回传给前端。

### 4.2 成功响应结构

```json
{
  "jsonrpc": "2.0",
  "id": "req_xxx",
  "result": {
    "data": {},
    "meta": {
      "server_time": "2026-04-09T10:00:01+08:00"
    },
    "warnings": []
  }
}
```

返回体中：

- `data` 承载业务对象；
- `meta` 承载服务端辅助信息；
- `warnings` 承载弱提醒，不等同于错误。

### 4.3 错误响应结构

```json
{
  "jsonrpc": "2.0",
  "id": "req_xxx",
  "error": {
    "code": 1003002,
    "message": "TOOL_EXECUTION_FAILED",
    "data": {
      "trace_id": "trace_xxx",
      "detail": "tool execution failed"
    }
  }
}
```

错误体中：

- `code` 是正式错误码；
- `message` 是稳定错误名；
- `data.trace_id` 用于追踪；
- `data.detail` 只作为排查辅助，不可作为前端业务判断依据。

### 4.4 Notification 结构

```json
{
  "jsonrpc": "2.0",
  "method": "task.updated",
  "params": {
    "task_id": "task_001",
    "status": "processing"
  }
}
```

Notification 只负责“状态变化推送”，不承载复杂业务命令。

### 4.5 通用分页结构

```json
{
  "page": {
    "limit": 20,
    "offset": 0,
    "total": 135,
    "has_more": true
  }
}
```

### 4.6 返回规则

- 任务类接口：统一返回 `task`，按需附带 `delivery_result`、`bubble_message`；`agent.input.submit` 对无任务锚点的纯社交 / 闲聊输入可返回 `task = null`
- 列表类接口：统一返回 `items` + `page`
- 安全类接口：统一返回 `approval_request / authorization_record / audit_record / recovery_point`，按需附带 `impact_scope`
- 设置类接口：统一返回 `effective_settings` 或 `setting_item`、`apply_mode`、`need_restart`

---

## 5. 正式状态枚举与直观解释

### 5.1 任务状态 `task_status`

- `processing`：任务正在执行。
- `waiting_auth`：命中高风险动作，等待授权。
- `waiting_input`：等待用户补充必要输入。
- `confirming_intent`：系统已识别出候选意图，等待用户确认或纠偏。
- `paused`：任务被用户或系统主动暂停。
- `blocked`：任务因依赖、环境或外部条件未满足而阻塞。
- `failed`：任务执行失败。
- `completed`：任务完成。
- `cancelled`：任务被主动取消。
- `ended_unfinished`：任务结束但没有完成，常见于中断退出或放弃执行。

### 5.2 任务列表分组 `task_list_group`

- `unfinished`：未结束任务。
- `finished`：已结束任务。

### 5.3 巡检事项桶 `todo_bucket`

- `upcoming`：近期要做。
- `later`：后续安排。
- `recurring_rule`：重复事项规则。
- `closed`：已结束。

### 5.4 风险等级 `risk_level`

- `green`：可静默执行。
- `yellow`：执行前询问。
- `red`：强制人工确认。

### 5.5 安全状态 `security_status`

- `normal`：正常。
- `pending_confirmation`：存在待确认操作。
- `intercepted`：已拦截。
- `execution_error`：执行异常。
- `recoverable`：可恢复。
- `recovered`：已恢复。

### 5.6 交付类型 `delivery_type`

- `bubble`：气泡轻量交付。
- `workspace_document`：写入工作区文档。
- `result_page`：结果页交付。
- `open_file`：直接打开文件。
- `reveal_in_folder`：打开文件夹并高亮文件。
- `task_detail`：跳转任务详情。

### 5.7 语音状态 `voice_session_state`

- `listening`：正在听。
- `locked`：锁定通话。
- `processing`：语音结束，正在理解或处理。
- `cancelled`：本次语音已取消。
- `finished`：本次语音已完成。

### 5.8 入口来源 `request_source`

- `floating_ball`
- `dashboard`
- `tray_panel`

### 5.9 触发动作 `request_trigger`

- `voice_commit`
- `hover_text_input`
- `text_selected_click`
- `file_drop`
- `error_detected`
- `recommendation_click`

### 5.10 输入类型 `input_type`

- `text`
- `text_selection`
- `file`
- `error`

### 5.11 输入模式 `input_mode`

- `voice`
- `text`

### 5.12 任务来源类型 `task_source_type`

- `voice`
- `hover_input`
- `selected_text`
- `dragged_file`
- `todo`
- `error_signal`
- `screen_capture`

### 5.13 气泡类型 `bubble_message_type`

- `status`
- `intent_confirm`
- `result`

### 5.14 授权决策 / 状态

- `approval_decision`：`allow_once / deny_once`
- `approval_status`：`pending / approved / denied`

### 5.15 设置相关

- `settings_scope`：`all / general / floating_ball / memory / task_automation / models`
- `apply_mode`：`immediate / restart_required / next_task_effective`
- `theme_mode`：`follow_system / light / dark`
- `position_mode`：`fixed / draggable`

### 5.16 过程状态

- `task_step_status`：`pending / running / completed / failed / skipped / cancelled`
- `step_status`：`pending / running / completed / failed / skipped / cancelled`
- `todo_status`：`normal / due_today / overdue / completed / cancelled`
- `recommendation_scene`：`hover / selected_text / idle / error`
- `recommendation_feedback`：`positive / negative / ignore`
- `task_control_action`：`pause / resume / cancel / restart`
- `notepad_action`：`complete / cancel / move_upcoming / toggle_recurring / cancel_recurring / restore / delete`
- `time_unit`：`minute / hour / day / week`
- `run_status`：`processing / completed`

### 5.17 状态使用约束

- 对外产品态统一以 `task_status` 为主。
- 内核态 `run_status` 仅保留最小兼容状态，不得替代 `task_status` 对外暴露。
- 悬浮球主状态机、承接状态机、气泡生命周期都属于前端局部状态，不直接进入正式状态枚举。
- 文档中未登记的状态值不得进入实现。

### 5.18 插件扩展相关

- `plugin_kind`：`worker / sidecar`
- `plugin_source_type`：`builtin / local_dir / github / marketplace`
- `plugin_health_status`：`unknown / healthy / degraded / failed / stopped / unavailable`
- `plugin_tool_source_type`：`builtin / worker / sidecar`
- `plugin_tool_contract.risk_hint`：对齐 `risk_level` 语义，当前允许返回 `green / yellow / red`

## 6. 错误码设计

### 6.1 分段

- `0`：成功
- `1001xxx`：Task / Session / Run / Step
- `1002xxx`：协议与参数
- `1003xxx`：工具调用
- `1004xxx`：权限与风险
- `1005xxx`：存储与数据库
- `1006xxx`：worker / sidecar / plugin
- `1007xxx`：系统与平台

当前仓库错误码真源 `packages/protocol/errors/codes.ts` 已正式登记到 `1007xxx`。此外，为后续功能扩展预留：

- `1008xxx`：模型与前馈配置
- `1009xxx`：评估与人工升级

### 6.2 如何理解错误段

- `1001xxx`：任务不存在、状态非法、task/run 映射问题。
- `1002xxx`：请求结构不合法、schema 校验失败、方法不存在。
- `1003xxx`：工具找不到、工具失败、超时、输出不合法。
- `1004xxx`：必须授权、授权被拒绝、工作区越界、能力被禁止。
- `1005xxx`：数据库、Artifact、恢复点、Stronghold、RAG 等落盘能力异常。
- `1006xxx`：worker / sidecar / plugin 进程不可用或输出非法。
- `1007xxx`：平台和执行环境问题。
- `1008xxx`：模型与前馈配置异常；其中 `1008001`、`1008002`、`1008003` 已登记到错误码真源，其余编号继续预留。
- `1009xxx`：结果审查、Doom Loop、Eval、Human-in-the-loop 升级异常，当前为预留段。

### 6.3 推荐错误码表

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

#### 权限与风险

- `1004001` `APPROVAL_REQUIRED`
- `1004002` `APPROVAL_REJECTED`
- `1004003` `WORKSPACE_BOUNDARY_DENIED`
- `1004004` `COMMAND_NOT_ALLOWED`
- `1004005` `CAPABILITY_DENIED`

#### 存储与数据库

- `1005001` `SQLITE_WRITE_FAILED`
- `1005002` `ARTIFACT_NOT_FOUND`
- `1005003` `CHECKPOINT_CREATE_FAILED`
- `1005004` `STRONGHOLD_ACCESS_FAILED`
- `1005005` `RAG_INDEX_UNAVAILABLE`
- `1005006` `RECOVERY_POINT_NOT_FOUND`

#### Worker / Sidecar / Plugin

- `1006001` `WORKER_NOT_AVAILABLE`
- `1006002` `PLAYWRIGHT_SIDECAR_FAILED`
- `1006003` `OCR_WORKER_FAILED`
- `1006004` `MEDIA_WORKER_FAILED`

#### 预留错误码（尚未登记到错误码真源）

以下错误码常量保留给后续功能使用。在它们正式写入 `packages/protocol/errors/codes.ts` 前，只能作为规划预留，不得被文档误解为当前仓库已经实现：

##### Worker / Plugin 扩展预留

- `1006005` `PLUGIN_NOT_AVAILABLE`
- `1006006` `PLUGIN_PERMISSION_DENIED`
- `1006007` `PLUGIN_OUTPUT_INVALID`

#### 系统 / 平台

- `1007001` `PLATFORM_NOT_SUPPORTED`
- `1007002` `TAURI_PLUGIN_FAILED`
- `1007003` `DOCKER_BACKEND_UNAVAILABLE`
- `1007004` `SANDBOX_PROFILE_INVALID`
- `1007005` `PATH_POLICY_VIOLATION`
- `1007006` `INSPECTION_FILESYSTEM_UNAVAILABLE`
- `1007007` `INSPECTION_SOURCE_NOT_FOUND`
- `1007008` `INSPECTION_SOURCE_UNREADABLE`

##### 模型与前馈配置

- `1008001` `MODEL_PROVIDER_NOT_FOUND`
- `1008002` `MODEL_NOT_ALLOWED`
- `1008003` `MODEL_RUNTIME_UNAVAILABLE`

##### 模型与前馈配置预留

- `1008004` `SKILL_NOT_FOUND`
- `1008005` `BLUEPRINT_NOT_FOUND`
- `1008006` `PROMPT_TEMPLATE_NOT_FOUND`
- `1008007` `LSP_DIAGNOSTIC_UNAVAILABLE`

##### 评估与升级预留

- `1009001` `REVIEW_FAILED`
- `1009002` `DOOM_LOOP_DETECTED`
- `1009003` `EVAL_SNAPSHOT_WRITE_FAILED`
- `1009004` `HUMAN_REVIEW_REQUIRED`

### 6.4 错误处理规则

- 前端只认错误码和错误类型，不猜字符串。
- Go 返回错误时必须带 `id` 或 `trace_id`。
- worker / sidecar / plugin 错误必须包装成统一错误码。
- 插件安装 / 启停失败必须落到 `1006xxx`。
- 多模型切换失败在对应能力正式落地后应落到 `1008xxx`。
- 审查失败 / 熔断 / 人工升级在对应能力正式落地后应落到 `1009xxx`。

## 7. 方法集合与原子功能映射

### 7.1 stable

#### A. 入口承接 / 场景助手

- `agent.input.submit`
- `agent.task.start`
- `agent.task.confirm`
- `agent.recommendation.get`
- `agent.recommendation.feedback.submit`
- `agent.screen.analyze`

#### B. 任务状态 / 结果交付 / 巡检

- `agent.task.list`
- `agent.task.detail.get`
- `agent.task.control`
- `agent.task.events.list`
- `agent.task.tool_calls.list`
- `agent.task.steer`
- `agent.task.artifact.list`
- `agent.task.artifact.open`
- `agent.delivery.open`
- `agent.task_inspector.config.get`
- `agent.task_inspector.config.update`
- `agent.task_inspector.run`
- `agent.notepad.list`
- `agent.notepad.update`
- `agent.notepad.convert_to_task`

#### C. 仪表盘 / 镜子 / 安全卫士

- `agent.dashboard.overview.get`
- `agent.dashboard.input.start`
- `agent.dashboard.module.get`
- `agent.mirror.overview.get`
- `agent.security.summary.get`
- `agent.security.restore_points.list`
- `agent.security.restore.apply`
- `agent.security.pending.list`
- `agent.security.respond`
- `agent.security.audit.list`

#### D. 设置中心

- `agent.settings.get`
- `agent.settings.update`

#### E. 插件扩展

- `agent.plugin.runtime.list`
- `agent.plugin.list`
- `agent.plugin.detail.get`

### 7.2 planned

- `agent.mirror.memory.manage`
- `agent.plugin.enable`
- `agent.plugin.disable`
- `agent.model.list`
- `agent.model.activate`
- `agent.skill.install`
- `agent.skill.list`

补充说明：

- 上述多模型与 Skill 方法当前仍以 planned 为准；实际冻结顺序仍以后续 `/packages/protocol` 真源为准。

### 7.3 原子功能与方法映射说明

以下原子功能不应误判为“需要新增正式方法”：

- **悬浮球单击 / 双击 / 长按 / 上滑 / 下滑 / 悬停** 属于前端交互动作，本地先进入前端状态机，再映射到 `agent.input.submit`、`agent.task.start` 或本地 UI 行为。
- **文本选中承接、文件拖拽承接、错误信息承接** 统一收敛到 `agent.task.start`。
- **意图确认与纠偏** 统一使用 `agent.task.confirm`，用于采纳系统猜测或覆盖为用户修正后的意图。
- 气泡置顶 / 删除 / 恢复：优先作为前端局部能力，必要时再引出设置或历史管理接口
- **主动推荐与反馈** 统一使用 `agent.recommendation.get` 和 `agent.recommendation.feedback.submit`。
- **屏幕截图、剪贴板、鼠标停留等场景感知信号** 统一使用 `agent.screen.analyze`，用于判断是否刷新推荐，不直接替代 `agent.task.start` 创建正式任务。
- **任务成果列表、产物打开与最终交付打开** 统一使用 `agent.task.artifact.*` 与 `agent.delivery.open`。
- 长结果自动分流：由交付内核决定，不新增方法
- 一键中断：复用 `agent.task.control`
- **运行中补充 follow-up 指令** 统一使用 `agent.task.steer`，用于向当前任务追加 steering 信息。
- **运行时事件查看与调试观察** 统一使用 `agent.task.events.list`，用于补充查看正式事件流，不替代 `task` 主对象查询。
- **任务工具调用查看与排障观察** 统一使用 `agent.task.tool_calls.list`，用于补充查看正式 `tool_call` 记录，不替代 `task` 主对象查询。
- **插件扩展列表、插件详情与运行态展示** 统一使用 `agent.plugin.list`、`agent.plugin.detail.get`、`agent.plugin.runtime.list`；插件启停、安装以及多模型、技能安装当前阶段仍优先通过 `agent.settings.get / update` 与仪表盘模块承接，待对象、权限与来源字段完全冻结后再升级为独立正式接口。
- **任务巡检、事项转任务** 统一使用 `agent.task_inspector.*` 与 `agent.notepad.*`。
- **仪表盘首页、镜子、安全卫士** 统一使用 `agent.dashboard.*`、`agent.mirror.*`、`agent.security.*`。

# 8. stable 开发接口详细定义

以下内容在不改变前述架构边界、模型真源、错误码体系和跨平台原则的前提下，对 stable 范围接口进行详细展开。

## 8.1 入口承接 / 语音 / 场景助手

### 8.1.1 `agent.input.submit`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户长按悬浮球说完一句话并松开时
  - 用户悬停输入框输入一句轻量文本并提交时
  - 用户通过仪表盘固定悬浮入口提交一段自由文本时
- **系统处理**：
  - 统一承接语音转写文本和轻量输入文本
  - 结合当前页面、选中文本、附带文件做上下文识别
  - 对正式工作请求创建或续接 `task`，并直接进入处理或等待必要补充输入
  - 对无任务锚点的纯社交 / 闲聊输入，可仅返回脱离 `task` 的轻量气泡
- **入参**：会话信息、触发来源、输入内容、上下文、语音元信息、执行偏好
- **出参**：任务对象或 `null`、气泡消息、按需附带正式交付结果

### agent.input.submit 入参说明

| 字段                         | 中文说明                       |
| ---------------------------- | ------------------------------ |
| `request_meta.trace_id`      | 请求链路追踪 ID                |
| `request_meta.client_time`   | 前端发起时间                   |
| `session_id`                 | 当前会话 ID                    |
| `source`                     | 来源位置，如悬浮球、仪表盘     |
| `trigger`                    | 触发方式，如语音提交、轻量输入 |
| `input.type`                 | 输入对象类型，固定为 `text`    |
| `input.text`                 | 用户输入文本                   |
| `input.input_mode`           | 输入模式，语音或文字           |
| `context.page`               | 当前页面上下文                 |
| `context.page.title`         | 当前页面标题                   |
| `context.page.url`           | 当前页面 URL                   |
| `context.page.browser_kind`  | 当前浏览器分类，取值为 `chrome / edge / other_browser / non_browser` |
| `context.page.process_path`  | 当前宿主进程路径               |
| `context.page.process_id`    | 当前宿主进程 ID                |
| `context.page.app_name`      | 当前宿主应用名                 |
| `context.page.window_title`  | 当前窗口标题                   |
| `context.page.visible_text`  | 当前页面可见文本摘录           |
| `context.selection.text`     | 当前选中文本                   |
| `context.files`              | 当前附带文件列表               |
| `context.screen.summary`     | 当前屏幕摘要，可用于视觉型任务上下文 |
| `context.screen.visible_text` | 当前屏幕可见文本摘录          |
| `context.behavior.last_action` | 最近行为信号，例如 `copy`    |
| `context.behavior.dwell_millis` | 当前场景停留时长             |
| `voice_meta`                 | 语音会话元信息                 |
| `options.confirm_required`   | 是否强制先进入意图确认         |
| `options.preferred_delivery` | 偏好的结果交付方式             |

补充约束：

- 为保持既有协议字面量兼容性，非 Chrome / Edge 的浏览器宿主继续使用 `other_browser`；当前它仅表示“浏览器宿主存在，但不在 Chromium takeover v1 的正式支持范围内”。
- 正式 `context.page.url` / `input.page_context.url` 进入任务或 RPC 载荷前应统一去除凭据、query 与 hash，避免把易变或敏感 URL 片段冻结进正式上下文。
- `floating_ball` 来源的普通文本/语音提交（`hover_text_input`、`voice_commit`）应尽力补齐当前前台窗口的 `context.page` 附着提示；`dashboard / tray_panel` 的普通文本输入则仍按显式视觉上下文请求决定是否补齐前台页面信息。
- 当输入文本和 `context.page / context.screen / context.behavior` 同时表明用户想“查看当前页面/屏幕”时，后端可直接推断为受控视觉型任务，并继续走既有 `task -> approval_request -> event -> artifact / delivery_result` 链路。
- 这类视觉型任务的 `task.source_type` 应返回 `screen_capture`，表示正式任务围绕当前屏幕采样展开，而不是普通 `hover_input` 文本处理。
- `agent.task.start` 不接受显式 `intent` 入参；若客户端误传该字段，协议层应忽略，并继续由后端结合 `input / context` 统一推断，不需要新增平行入口。
- 当客户端省略 `session_id` 时，后端应负责选择或创建隐藏协作 session，并把最终使用的 `session_id` 写回返回的 `task` 对象，而不是要求前端自行猜测生命周期。
- 当普通文本被判定为无任务锚点的纯社交 / 闲聊输入时，后端可返回 `data.task = null`、`data.delivery_result = null` 与 `task_id` 为空的 `bubble_message`；前端只能把它当作轻量承接反馈，不得写入正式任务链、任务详情或正式交付出口。
- 若现有 task 已处于 `waiting_auth`、`blocked` 或 `paused`，后端不得通过隐式 follow-up 直接改写原 task 的后续执行语义；此时应新开 task 或等待显式恢复/授权链路处理。
- 若同一 `session` 内只有一个 `waiting_input / confirming_intent` 任务，普通文本补充可续接到该任务；`options.confirm_required = true` 只表示本次补充后仍需确认，不应把普通文本补充机械拆成新 task。
- 文件、选区、错误等结构化补充证据若要续接旧 task，仍必须存在共享页面 / 窗口 / App 锚点、共享选区 / 报错 / 附件血缘，或其他能证明属于旧任务的 lineage。

### agent.input.submit 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_input_001",
  "method": "agent.input.submit",
  "params": {
    "request_meta": {
      "trace_id": "trace_001",
      "client_time": "2026-04-07T10:20:00+08:00"
    },
    "session_id": "sess_001",
    "source": "dashboard",
    "trigger": "hover_text_input",
    "input": {
      "type": "text",
      "text": "帮我整理一下这页内容，输出成三点摘要",
      "input_mode": "text"
    },
    "context": {
      "page": {
        "title": "Q3 复盘草稿",
        "app_name": "browser",
        "url": "local://current-page",
        "window_title": "Chrome - Q3 复盘草稿",
        "visible_text": "发布说明缺少回滚策略和验收结论。"
      },
      "screen": {
        "summary": "浏览器中打开了一页发布清单，页面中有缺失项提示。",
        "visible_text": "Warning: release notes are incomplete."
      },
      "behavior": {
        "last_action": "copy",
        "dwell_millis": 18000
      },
      "selection": {
        "text": "这里是一段当前选中的补充上下文"
      },
      "files": []
    },
    "options": {
      "preferred_delivery": "bubble"
    }
  }
}
```

### agent.input.submit 出参说明

| 字段                     | 中文说明                         |
| ------------------------ | -------------------------------- |
| `data.task`              | 正式任务对象；纯社交 / 闲聊输入可返回 `null` |
| `data.task.task_id`      | 新建或续接任务 ID，仅当 `data.task` 非空时存在 |
| `data.task.session_id`   | 后端最终采用的隐藏协作会话 ID，仅当 `data.task` 非空时存在 |
| `data.task.title`        | 任务标题，仅当 `data.task` 非空时存在 |
| `data.task.source_type`  | 任务来源类型，仅当 `data.task` 非空时存在 |
| `data.task.status`       | 当前任务状态，仅当 `data.task` 非空时存在 |
| `data.task.current_step` | 当前步骤，仅当 `data.task` 非空时存在 |
| `data.bubble_message`    | 气泡承接内容                     |
| `data.delivery_result`   | 若后端已直接完成，可返回正式交付 |
| `meta.server_time`       | 服务端响应时间                   |

### agent.input.submit 出参示例

正式任务请求示例：

```json
{
  "jsonrpc": "2.0",
  "id": "req_input_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_001",
        "session_id": "sess_001",
        "title": "整理当前页面内容",
        "source_type": "hover_input",
        "status": "processing",
        "current_step": "analyze_input"
      },
      "bubble_message": {
        "bubble_id": "bubble_001",
        "task_id": "task_001",
        "type": "status",
        "text": "已接收你的输入，正在整理当前页面内容。"
      },
      "delivery_result": null
    },
    "meta": {
      "server_time": "2026-04-07T10:20:01+08:00"
    },
    "warnings": []
  }
}
```

纯社交 / 闲聊输入示例：

```json
{
  "jsonrpc": "2.0",
  "id": "req_input_chat_001",
  "result": {
    "data": {
      "task": null,
      "bubble_message": {
        "bubble_id": "bubble_chat_001",
        "task_id": "",
        "type": "result",
        "text": "你好，我在。"
      },
      "delivery_result": null
    },
    "meta": {
      "server_time": "2026-04-07T10:20:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.2 `agent.task.start`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户选中文本后点击悬浮球
  - 用户拖拽文件到悬浮球
- **系统处理**：
  - 识别输入对象与对象上下文
  - 创建正式 `task` 并决定处理路径
  - 根据配置直接处理或进入等待补充输入状态
- **入参**：会话信息、触发方式、任务输入对象、补充上下文、交付偏好
- **出参**：任务对象、气泡消息、按需附带正式交付结果

### agent.task.start 入参说明

| 字段                       | 中文说明 |
| -------------------------- | -------- |
| `request_meta.trace_id`    | 请求链路追踪 ID |
| `request_meta.client_time` | 前端发起时间 |
| `session_id`               | 当前会话 ID |
| `source`                   | 来源位置，取值来自 `request_source` |
| `trigger`                  | 触发动作，取值来自 `request_trigger` |
| `input.type`               | 输入对象类型，取值来自 `input_type` |
| `input.text`               | 当 `input.type = text_selection` 时表示选中文本内容；当 `input.type = file` 时可表示用户对附件的明确说明 |
| `input.files`              | 当 `input.type = file` 时传入，表示拖入文件列表 |
| `input.page_context`       | 与输入对象关联的页面上下文，按需传入 |
| `input.page_context.title` | 当前页面标题，可用于页面级任务标题与上下文冻结 |
| `input.page_context.url`   | 当前页面 URL |
| `input.page_context.browser_kind` | 当前浏览器分类，取值为 `chrome / edge / other_browser / non_browser` |
| `input.page_context.process_path` | 当前宿主进程路径 |
| `input.page_context.process_id` | 当前宿主进程 ID |
| `input.page_context.app_name` | 当前宿主应用名 |
| `input.page_context.window_title` | 当前窗口标题 |
| `input.page_context.visible_text` | 当前页面可见文本摘录 |
| `context.selection.text`   | 当前选区补充文本，按需传入 |
| `context.files`            | 补充文件上下文，按需传入 |
| `context.screen.summary`   | 当前屏幕摘要，可用于视觉型任务上下文 |
| `context.screen.visible_text` | 当前屏幕可见文本摘录 |
| `context.behavior.last_action` | 最近行为信号，例如 `copy` |
| `context.behavior.dwell_millis` | 当前场景停留时长 |
| `delivery.preferred`       | 优先交付方式 |
| `delivery.fallback`        | 兜底交付方式 |
| `options.confirm_required` | 是否强制先进入意图确认；不用于绕过风险授权 |

补充约束：

- 当输入文本和 `page_context / screen / behavior` 同时表明用户想“查看当前页面/屏幕”时，后端可直接推断为受控视觉型任务，并继续走既有 `task -> approval_request -> event -> artifact / delivery_result` 链路。
- 这类视觉型任务的 `task.source_type` 应返回 `screen_capture`，表示正式任务围绕当前屏幕采样展开，而不是普通 `hover_input` 文本处理。
- `agent.task.start` 不接受显式 `intent` 入参；视觉型任务仍由后端根据 `input / context / delivery` 统一推断，不要求前端发明平行入口。
- `options.confirm_required = true` 仅表示调用方要求先进入意图确认；`false` 或省略只表示不强制确认，不得跳过风险授权、审计、恢复点或必要澄清。
- 当 `input.type = file` 且 `input.text` 已包含用户对附件的明确说明时，后端应直接进入 Agent Loop / 治理 / 执行链路，不应重复返回通用意图确认气泡。
- 当 `input.type = file` 但缺少明确说明时，后端仍可进入意图确认或补充输入状态，避免对裸附件直接执行不明确任务。
- 当客户端省略 `session_id` 时，后端应负责选择或创建隐藏协作 session，并把最终使用的 `session_id` 写回返回的 `task` 对象；若判断为同一任务的补充输入，则应续到原 task 而不是机械新开 task。
- `task.session_id` 是正式协议字段，schema、类型层和 `task.updated` 通知都必须返回该字段；若当前任务没有关联隐藏协作 session，应返回 `null`，而不是省略字段。
- 若现有 task 已处于 `waiting_auth`、`blocked` 或 `paused`，后端不得通过隐式 follow-up 直接改写原 task 的后续执行语义；此时应新开 task 或等待显式恢复/授权链路处理。
- 当后端在正式主链路中已经解析出结构化意图或视觉任务信号时，不得仅凭“当前只有一个 waiting task”或“本次输入是结构化对象”就把新输入并回旧 task；只有存在共享页面 / 窗口 / App 锚点、共享选区 / 报错 / 附件血缘，或其他能证明属于旧任务的 lineage 时，才允许视为旧 task 的 continuation；多候选场景下只有存在唯一任务特定匹配时才允许续接。
- 悬浮球默认入口锚点（如 `desktop / local://shell-ball`）只表示输入从悬浮球进入系统，不得作为共享页面 / 窗口 / App 锚点来证明 continuation。
- continuation 分类发给模型的信号必须至少带上当前输入解析后的 `intent_name / delivery_type` 和候选 task 的 `intent_name / delivery_type`；仅靠 `explicit_intent_present=true` 之类布尔位不足以支撑正式路由判断。

### agent.task.start 入参示例

以下示例展示“文本选中后点击悬浮球”的 `text_selection` 场景。

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_start_001",
  "method": "agent.task.start",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_001",
      "client_time": "2026-04-07T10:31:00+08:00"
    },
    "session_id": "sess_001",
    "source": "floating_ball",
    "trigger": "text_selected_click",
    "input": {
      "type": "text_selection",
      "text": "这里放用户选中的文本内容",
      "page_context": {
        "app_name": "Chrome",
        "url": "https://example.com/release",
        "image_url": "xxx.png"
      }
    },
    "context": {
      "selection": {
        "text": "这里是补充上下文"
      },
      "files": []
    },
    "delivery": {
      "preferred": "bubble",
      "fallback": "workspace_document"
    }
  }
}
```

### agent.task.start 出参说明

| 字段                     | 中文说明                         |
| ------------------------ | -------------------------------- |
| `data.task.task_id`      | 新建任务 ID                      |
| `data.task.session_id`   | 后端最终采用的隐藏协作会话 ID     |
| `data.task.title`        | 任务标题                         |
| `data.task.source_type`  | 任务来源类型                     |
| `data.task.status`       | 当前任务状态                     |
| `data.task.current_step` | 当前步骤                         |
| `data.bubble_message`    | 气泡承接内容                     |
| `data.delivery_result`   | 若后端已直接完成，可返回正式交付 |
| `meta.server_time`       | 服务端响应时间                   |

### agent.task.start 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_start_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_101",
        "session_id": "sess_001",
        "title": "解释选中文本",
        "source_type": "selected_text",
        "status": "processing",
        "current_step": "analyze_object"
      },
      "bubble_message": {
        "bubble_id": "bubble_101",
        "task_id": "task_101",
        "type": "status",
        "text": "已接收这段选中文本，正在分析处理路径。"
      },
      "delivery_result": null
    },
    "meta": {
      "server_time": "2026-04-07T10:31:01+08:00"
    },
    "warnings": []
  }
}
```

---

补充说明：

- `task.source_type` 当前稳定取值至少包括 `voice`、`hover_input`、`selected_text`、`dragged_file`、`todo`、`error_signal`、`screen_capture`。
- 其中 `screen_capture` 表示该任务虽然可能由自然语言触发，但其正式执行对象已经切换为“当前屏幕/页面采样证据”，因此后续会进入授权、artifact 证据和交付链。

---

### 8.1.3 `agent.task.confirm`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 系统猜测出意图后，用户点击“确认”
  - 用户认为系统猜错时，提交修正后的意图
- **系统处理**：
  - 采纳确认结果
  - 更新任务意图
  - 推进到正式执行阶段
- **入参**：任务 ID、是否确认、修正后的意图
- **出参**：更新后的任务对象、状态气泡

补充约束：

- `confirmed = true` 时，表示用户确认系统当前猜测的意图正确，此时 `corrected_content` 可省略；若传入也应被忽略，不得覆盖当前意图。
- `confirmed = false` 时，若调用方传入完整的 `corrected_content`，后端以该对象覆盖任务当前意图后再推进执行。
- `confirmed = false` 且未传入 `corrected_content` 时，后端不得直接取消任务；应保留任务在 `corrected_content`，并返回要求用户重新说明目标或补充修正意图的状态气泡。
- 本接口只处理“意图确认 / 纠偏”这一承接阶段，不替代 `agent.task.control` 的暂停、继续、取消、重启控制语义。

### agent.task.confirm 入参说明

| 字段                         | 中文说明             |
| ---------------------------- | -------------------- |
| `request_meta.trace_id`      | 请求链路追踪 ID      |
| `request_meta.client_time`   | 前端发起时间         |
| `task_id`                    | 目标任务 ID          |
| `confirmed`                  | 是否确认系统猜测正确 |
| `corrected_content`      | 修正后的用户想法     |

### agent.task.confirm 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_confirm_001",
  "method": "agent.task.confirm",
  "params": {
    "request_meta": {
      "trace_id": "trace_confirm_001",
      "client_time": "2026-04-07T10:32:00+08:00"
    },
    "task_id": "task_101",
    "confirmed": false,
    "corrected_content": "修正后的用户想法"
    }
  }
}
```

### agent.task.confirm 出参说明

| 字段                  | 中文说明         |
| --------------------- | ---------------- |
| `data.task.task_id`   | 任务 ID          |
| `data.task.status`    | 更新后的任务状态 |
| `data.task.corrected_content`    | 生效后的任务意图 |
| `data.task.current_step` | 当前步骤      |
| `data.bubble_message` | 状态提示气泡     |

### agent.task.confirm 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_confirm_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_101",
        "status": "processing",
        "corrected_content": "修正后的用户想法",
        "current_step": "generate_output"
      },
      "bubble_message": {
        "bubble_id": "bubble_102",
        "task_id": "task_101",
        "type": "status",
        "text": "已按新的要求开始处理"
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:32:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.4 `agent.task.artifact.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户打开任务详情页成果区时
  - 仪表盘结果区需要列出指定任务真实产物时
- **系统处理**：
  - 按 `task_id` 查询真实 artifact store
  - 返回稳定分页结构，供前端渲染列表和翻页
- **入参**：任务 ID、分页参数
- **出参**：产物列表、分页信息

### agent.task.artifact.list 入参说明

| 字段      | 中文说明    |
| --------- | ----------- |
| `task_id` | 目标任务 ID |
| `limit`   | 每页条数    |
| `offset`  | 偏移量      |

### agent.task.artifact.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_artifact_list_001",
  "method": "agent.task.artifact.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_artifact_list_001",
      "client_time": "2026-04-07T10:43:00+08:00"
    },
    "task_id": "task_201",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.task.artifact.list 出参说明

| 字段         | 中文说明 |
| ------------ | -------- |
| `data.items` | 产物列表 |
| `data.page`  | 分页信息 |

### agent.task.artifact.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_artifact_list_001",
  "result": {
    "data": {
      "items": [
        {
          "artifact_id": "art_001",
          "task_id": "task_201",
          "artifact_type": "generated_doc",
          "title": "Q3复盘.md",
          "path": "D:/CialloClawWorkspace/Q3复盘.md",
          "mime_type": "text/markdown"
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:43:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.5 `agent.task.artifact.open`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户在任务详情或结果区点击某个 artifact 时
- **系统处理**：
  - 根据 `task_id + artifact_id` 定位真实 artifact
  - 返回与之对齐的 `delivery_result`、`open_action`、`resolved_payload`
- **入参**：任务 ID、产物 ID
- **出参**：产物对象、交付结果、打开动作、解析后的载荷

### agent.task.artifact.open 入参说明

| 字段          | 中文说明 |
| ------------- | -------- |
| `task_id`     | 任务 ID  |
| `artifact_id` | 产物 ID  |

### agent.task.artifact.open 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_artifact_open_001",
  "method": "agent.task.artifact.open",
  "params": {
    "request_meta": {
      "trace_id": "trace_artifact_open_001",
      "client_time": "2026-04-07T10:44:00+08:00"
    },
    "task_id": "task_201",
    "artifact_id": "art_001"
  }
}
```

### agent.task.artifact.open 出参说明

| 字段                  | 中文说明         |
| --------------------- | ---------------- |
| `data.artifact`       | 目标产物对象     |
| `data.delivery_result`| 交付结果         |
| `data.open_action`    | 最终打开动作     |
| `data.resolved_payload` | 解析后的打开载荷 |

### agent.task.artifact.open 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_artifact_open_001",
  "result": {
    "data": {
      "artifact": {
        "artifact_id": "art_001",
        "task_id": "task_201",
        "artifact_type": "generated_doc",
        "title": "Q3复盘.md",
        "path": "D:/CialloClawWorkspace/Q3复盘.md",
        "mime_type": "text/markdown"
      },
      "delivery_result": {
        "type": "open_file",
        "title": "打开产物",
        "payload": {
          "path": "D:/CialloClawWorkspace/Q3复盘.md"
        },
        "preview_text": "已打开文件"
      },
      "open_action": "open_file",
      "resolved_payload": {
        "path": "D:/CialloClawWorkspace/Q3复盘.md"
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:44:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.6 `agent.delivery.open`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 前端需要统一打开最终交付对象时
  - 入口可能来自任务主交付、结果页、任务详情或某个 artifact
- **系统处理**：
  - 若携带 `artifact_id`，优先基于真实 artifact 解析打开动作
  - 若未携带 `artifact_id`，则基于任务当前 `delivery_result` 解析打开动作
  - 返回统一的 `delivery_result`、`open_action`、`resolved_payload`
- **入参**：任务 ID，可选产物 ID
- **出参**：交付结果、打开动作、解析后的载荷，按需附带产物对象

### agent.delivery.open 入参说明

| 字段          | 中文说明                   |
| ------------- | -------------------------- |
| `task_id`     | 任务 ID                    |
| `artifact_id` | 从具体产物入口打开时可传入 |

### agent.delivery.open 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_delivery_open_001",
  "method": "agent.delivery.open",
  "params": {
    "request_meta": {
      "trace_id": "trace_delivery_open_001",
      "client_time": "2026-04-07T10:45:00+08:00"
    },
    "task_id": "task_201"
  }
}
```

### agent.delivery.open 出参说明

| 字段                    | 中文说明         |
| ----------------------- | ---------------- |
| `data.delivery_result`  | 主交付对象       |
| `data.open_action`      | 最终打开动作     |
| `data.resolved_payload` | 解析后的打开载荷 |
| `data.artifact`         | 命中的产物对象   |

### agent.delivery.open 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_delivery_open_001",
  "result": {
    "data": {
      "delivery_result": {
        "type": "workspace_document",
        "title": "处理结果",
        "payload": {
          "path": "D:/CialloClawWorkspace/Q3复盘.md",
          "task_id": "task_201"
        },
        "preview_text": "已为你写入文档并打开"
      },
      "open_action": "open_file",
      "resolved_payload": {
        "path": "D:/CialloClawWorkspace/Q3复盘.md"
      },
      "artifact": null
    },
    "meta": {
      "server_time": "2026-04-07T10:45:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.7 `agent.recommendation.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户悬停悬浮球
  - 当前场景满足主动推荐触发条件
- **系统处理**：
  - 结合当前页面、选区、场景信号生成推荐
  - 返回推荐项列表与是否命中冷却
- **入参**：来源、场景、上下文
- **出参**：推荐项列表、是否命中冷却

### agent.recommendation.get 入参说明

| 字段                     | 中文说明                           |
| ------------------------ | ---------------------------------- |
| `source`                 | 来源位置                           |
| `scene`                  | 当前场景，取值来自 `recommendation_scene` |
| `context.page_title`     | 页面标题                           |
| `context.app_name`       | 宿主应用                           |
| `context.selection_text` | 当前选中文本                       |

### agent.recommendation.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_recommendation_001",
  "method": "agent.recommendation.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_recommendation_001",
      "client_time": "2026-04-07T11:10:00+08:00"
    },
    "source": "floating_ball",
    "scene": "hover",
    "context": {
      "page_title": "当前页面标题",
      "app_name": "browser",
      "selection_text": "这里是一段当前选中的文本"
    }
  }
}
```

### agent.recommendation.get 出参说明

| 字段                             | 中文说明         |
| -------------------------------- | ---------------- |
| `data.cooldown_hit`              | 是否命中推荐冷却 |
| `data.items`                     | 推荐项列表       |
| `data.items[].recommendation_id` | 推荐项 ID        |
| `data.items[].text`              | 推荐文案         |

### agent.recommendation.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_recommendation_001",
  "result": {
    "data": {
      "cooldown_hit": false,
      "items": [
        {
          "recommendation_id": "rec_001",
          "text": "要不要我帮你总结这段内容？"
        },
        {
          "recommendation_id": "rec_002",
          "text": "也可以直接翻译这段内容"
        }
      ]
    },
    "meta": {
      "server_time": "2026-04-07T11:10:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.8 `agent.recommendation.feedback.submit`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户对推荐点击喜欢、不喜欢、忽略
- **系统处理**：记录推荐反馈，用于短期纠偏和长期自适应
- **入参**：推荐项 ID、反馈类型
- **出参**：是否生效

### agent.recommendation.feedback.submit 入参说明

| 字段                | 中文说明                     |
| ------------------- | ---------------------------- |
| `recommendation_id` | 推荐项 ID                    |
| `feedback`          | 反馈结果，取值来自 `recommendation_feedback` |

### agent.recommendation.feedback.submit 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_recommendation_feedback_001",
  "method": "agent.recommendation.feedback.submit",
  "params": {
    "request_meta": {
      "trace_id": "trace_recommendation_feedback_001",
      "client_time": "2026-04-07T11:11:00+08:00"
    },
    "recommendation_id": "rec_001",
    "feedback": "positive"
  }
}
```

### agent.recommendation.feedback.submit 出参说明

| 字段           | 中文说明           |
| -------------- | ------------------ |
| `data.applied` | 是否已成功写入反馈 |

### agent.recommendation.feedback.submit 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_recommendation_feedback_001",
  "result": {
    "data": {
      "applied": true
    },
    "meta": {
      "server_time": "2026-04-07T11:11:01+08:00"
    },
    "warnings": []
  }
}
```


---

### 8.1.9 `agent.screen.analyze`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户在某个页面持续停留，需要系统判断是否应主动给出帮助时
  - 前端已采集到截图、剪贴板、鼠标停留等场景信号，需要统一做一次场景分析时
- **系统处理**：
  - 结合页面信息、屏幕截图、剪贴板内容、鼠标位置和最近行为做轻量场景判断
  - 返回当前是否需要刷新推荐，以及推荐文案和触发原因
- **入参**：页面信息、屏幕截图、剪贴板内容、鼠标信息、行为信号
- **出参**：推荐结果

### agent.screen.analyze 入参说明

| 字段                        | 中文说明 |
| --------------------------- | -------- |
| `request_meta.trace_id`     | 请求链路追踪 ID |
| `request_meta.client_time`  | 前端发起时间 |
| `page.app_name`             | 当前宿主应用名称 |
| `page.url`                  | 当前页面 URL |
| `screen.image_url`          | 当前截图文件路径或可解析地址 |
| `clipboard.context`         | 当前剪贴板上下文内容 |
| `clipboard.mime_type`       | 剪贴板内容 MIME 类型 |
| `mouse.isactive`            | 鼠标当前是否处于活跃状态；字段名保持与协议示例一致 |
| `mouse.position.x`          | 鼠标横坐标 |
| `mouse.position.y`          | 鼠标纵坐标 |
| `behavior.dwell_millis`     | 当前场景停留时长，单位毫秒 |
| `behavior.last_action`      | 最近一次关键动作，例如 `copy` |

### agent.screen.analyze 入参示例

```json
{
   "jsonrpc": "2.0",
   "id": "req_input_001",
   "method": "agent.screen.analyze",
   "params": {
      "request_meta": {
        "trace_id": "trace_001",
        "client_time": "2026-04-07T10:20:00+08:00"
      },
      "page": {
        "app_name": "Chrome",
        "url": "https://example.com/release"
      },
      "screen": {
        "image_url": "C://.tem/example/screenshot.png"
      },
      "clipboard": {
        "context": "translate this paragraph",
        "mime_type": "text/plain"
      },
      "mouse": {
        "isactive": true,
        "position": {
          "x": 100,
          "y": 200
        }
      },
      "behavior": {
        "dwell_millis": 18000,
        "last_action": "copy"
      }
   }
}
```

### agent.screen.analyze 出参说明

| 字段                                 | 中文说明 |
| ------------------------------------ | -------- |
| `data.recommendation.should_refresh` | 当前是否应刷新推荐内容 |
| `data.recommendation.content`        | 给前端展示的推荐文案；当 `should_refresh = true` 时应可直接展示 |
| `data.recommendation.reason`         | 触发本次推荐的原因标识 |
| `meta.server_time`                   | 服务端响应时间 |

### agent.screen.analyze 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_xxx",
  "result": {
    "data": {
      "recommendation": {
        "should_refresh": true,
        "content": "需要帮忙翻译这段话吗？",
        "reason": "copy_behavior"
      }
    },
    "meta": {
      "server_time": "2026-04-09T10:00:01+08:00"
    },
    "warnings": []
  }
}
```

补充说明：

- 当前实现里，部分屏幕/页面/剪贴板信号也会进入 `agent.input.submit`、`agent.task.start` 和 `agent.recommendation.get` 的上下文字段；这属于实现侧复用，不影响本文保留 `agent.screen.analyze` 的原始设计口径。


## 8.2 任务状态 / 任务巡检

### 8.2.1 `agent.task.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户打开仪表盘任务状态页时
- **系统处理**：按未完成 / 已结束分组拉取任务列表
- **入参**：分组、分页、排序
- **出参**：任务列表、分页信息

### agent.task.list 入参说明

| 字段         | 中文说明                         |
| ------------ | -------------------------------- |
| `group`      | 列表分组，取值来自 `task_list_group` |
| `limit`      | 每页条数                         |
| `offset`     | 偏移量                           |
| `sort_by`    | 排序字段                         |
| `sort_order` | 排序方向                         |

### agent.task.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_list_001",
  "method": "agent.task.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_list_001",
      "client_time": "2026-04-07T10:40:00+08:00"
    },
    "group": "unfinished",
    "limit": 20,
    "offset": 0,
    "sort_by": "updated_at",
    "sort_order": "desc"
  }
}
```

### agent.task.list 出参说明

| 字段                        | 中文说明 |
| --------------------------- | -------- |
| `data.items`                | 任务列表 |
| `data.items[].task_id`      | 任务 ID  |
| `data.items[].session_id`   | 任务归属的隐藏协作会话 ID |
| `data.items[].title`        | 任务标题 |
| `data.items[].status`       | 任务状态 |
| `data.items[].current_step` | 当前步骤 |
| `data.items[].risk_level`   | 风险等级 |
| `data.page`                 | 分页信息 |

### agent.task.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_list_001",
  "result": {
    "data": {
      "items": [
        {
          "task_id": "task_201",
          "session_id": "sess_001",
          "title": "整理 Q3 复盘要点",
          "source_type": "hover_input",
          "status": "processing",
          "current_step": "generate_summary",
          "risk_level": "green",
          "started_at": "2026-04-07T10:00:00+08:00",
          "updated_at": "2026-04-07T10:40:00+08:00"
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:40:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.2 `agent.task.detail.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户进入任务详情页时
- **系统处理**：返回任务头部、时间线、成果、正式引用、记忆引用、安全摘要与单个正式安全锚点
- **入参**：任务 ID
- **出参**：任务详情对象

补充约束：

- `approval_request` 是任务详情里的单个安全锚点，只在当前任务处于 `waiting_auth` 且仍持有活跃正式授权对象时返回；否则返回 `null`。
- `authorization_record` 返回当前任务当前执行尝试最近一条正式授权记录；若当前尝试还没有进入授权决策阶段则返回 `null`，不能把重启前旧尝试的 allow / deny 结果继续作为当前授权状态返回。
- `audit_record` 返回当前任务当前执行尝试最近一条正式审计记录；若当前尝试还没有正式审计记录则返回 `null`，不能把重启前旧尝试的审计结果继续作为当前安全摘要返回。
- 该字段只服务当前 task 的详情承接，不替代 `agent.security.pending.list` 对全局待确认项的聚合查询。
- `security_summary.pending_authorizations` 在任务详情中收敛为 `0 | 1`，仅反映当前 task 是否存在这一个活跃安全锚点。
- `security_summary.latest_restore_point` 的正式类型为 `RecoveryPoint | null`。
- 对屏幕感知类任务，任务详情应通过正式 `delivery_result`、`artifact`、事件和治理对象回看模型结论、截图证据、OCR 摘要和授权过程，而不是直接渲染平台采样结果或裸 worker 输出。
- 当 `task_run.snapshot_json` 与一等运行态存储同时存在时，`delivery_result` 与 `citations` 必须以前者的正式一等存储记录为准，兼容快照只能作为缺省回退，不能覆盖更新后的正式交付或引用链。
- 若任务存在正式视觉或上下文引用，`citations` 应返回稳定 `citation` 对象列表，并在需要时补齐 `artifact_id / artifact_type / evidence_role / excerpt_text / screen_session_id` 等结构化字段，用于区分截图证据、OCR 摘要和引用片段，而不是把引用信息混进 artifact 扩展字段或裸 tool output。
- `citations` 当前只承诺返回“当前 attempt 的正式引用链”；其一等存储写入仍是 task 级替换，不保证旧 attempt 的 citation 历史长期保留。
- 当 `tasks / task_steps` 已进入结构化读取路径时，`citations` 仍必须可从一等存储重建；不能把 `task_run` 兼容快照当作任务详情正式引用链的唯一来源。

### agent.task.detail.get 入参说明

| 字段      | 中文说明    |
| --------- | ----------- |
| `task_id` | 目标任务 ID |

### agent.task.detail.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_detail_001",
  "method": "agent.task.detail.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_detail_001",
      "client_time": "2026-04-07T10:41:00+08:00"
    },
    "task_id": "task_201"
  }
}
```

### agent.task.detail.get 出参说明

| 字段                     | 中文说明       |
| ------------------------ | -------------- |
| `data.task`              | 任务基础信息   |
| `data.task.session_id`   | 任务归属的隐藏协作会话 ID |
| `data.timeline`          | 步骤时间线     |
| `data.delivery_result`   | 最新正式交付结果；若当前任务尚未形成正式交付则返回 `null` |
| `data.artifacts`         | 产出物列表     |
| `data.citations`         | 正式引用列表；可携带截图证据、OCR 摘要和引用片段的结构化元信息 |
| `data.mirror_references` | 命中的镜子记忆 |
| `data.approval_request`  | 当前任务的正式安全锚点 |
| `data.authorization_record` | 当前任务最近一条正式授权记录 |
| `data.audit_record`      | 当前任务最近一条正式审计记录 |
| `data.security_summary`  | 安全摘要       |
| `data.runtime_summary`   | 运行态摘要，包含最新 runtime event、停止原因、最近失败错误码 / 分类 / 摘要与 observation signals |

其中 `data.timeline` 条目对应对外 `task_step` / `task_steps` 视图对象，不直接暴露内核 `step` / `steps`。

### agent.task.detail.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_detail_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_201",
        "session_id": "sess_001",
        "title": "整理 Q3 复盘要点",
        "status": "processing",
        "source_type": "hover_input",
        "started_at": "2026-04-07T10:00:00+08:00",
        "updated_at": "2026-04-07T10:40:00+08:00",
        "current_step": "generate_summary"
      },
      "timeline": [
        {
          "step_id": "step_1",
          "task_id": "task_201",
          "name": "recognize_input_object",
          "status": "completed",
          "order_index": 1,
          "input_summary": "识别到拖入文件",
          "output_summary": "确认是文档总结任务"
        },
        {
          "step_id": "step_2",
          "task_id": "task_201",
          "name": "generate_summary",
          "status": "running",
          "order_index": 2,
          "input_summary": "读取文档内容",
          "output_summary": "正在生成摘要"
        }
      ],
      "delivery_result": {
        "type": "task_detail",
        "title": "屏幕分析结果",
        "payload": {
          "path": "D:/CialloClawWorkspace/screen-artifacts/frame_001.png",
          "url": null,
          "task_id": "task_201"
        },
        "preview_text": "当前屏幕主要错误为缺少 release asset。"
      },
      "artifacts": [
        {
          "artifact_id": "art_001",
          "task_id": "task_201",
          "artifact_type": "generated_doc",
          "title": "Q3复盘.md",
          "path": "D:/CialloClawWorkspace/Q3复盘.md",
          "mime_type": "text/markdown"
        }
      ],
      "citations": [
        {
          "citation_id": "cit_001",
          "task_id": "task_201",
          "run_id": "run_201",
          "source_type": "file",
          "source_ref": "art_001",
          "label": "error_evidence | screen_capture | release asset missing",
          "artifact_id": "art_001",
          "artifact_type": "screen_capture",
          "evidence_role": "error_evidence",
          "excerpt_text": "release asset missing",
          "screen_session_id": "screen_sess_001"
        }
      ],
      "mirror_references": [
        {
          "memory_id": "pref_001",
          "reason": "当前任务命中了用户的输出偏好",
          "summary": "偏好简洁三点式摘要"
        }
      ],
      "approval_request": null,
      "authorization_record": {
        "authorization_record_id": "auth_001",
        "task_id": "task_201",
        "approval_id": "appr_001",
        "decision": "allow_once",
        "remember_rule": false,
        "operator": "user",
        "created_at": "2026-04-07T10:39:40+08:00"
      },
      "audit_record": {
        "audit_id": "audit_001",
        "task_id": "task_201",
        "type": "execution",
        "action": "execute_task",
        "summary": "已根据正式授权完成摘要生成。",
        "target": "workspace/Q3复盘.md",
        "result": "success",
        "created_at": "2026-04-07T10:40:10+08:00"
      },
      "security_summary": {
        "security_status": "normal",
        "risk_level": "green",
        "pending_authorizations": 0,
        "latest_restore_point": {
          "recovery_point_id": "rp_001",
          "task_id": "task_201",
          "summary": "生成摘要前的工作区快照",
          "created_at": "2026-04-07T10:39:58+08:00",
          "objects": ["workspace/Q3复盘.md"]
        }
      },
      "runtime_summary": {
        "events_count": 4,
        "latest_event_type": "loop.round.completed",
        "active_steering_count": 0,
        "latest_failure_code": null,
        "latest_failure_category": null,
        "latest_failure_summary": null,
        "loop_stop_reason": null,
        "observation_signals": ["page_title", "visible_text"]
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:41:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.3 `agent.task.control`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户点击暂停、继续、取消、重启等操作时
- **系统处理**：执行任务状态控制并返回最新状态
- **重启语义**：`restart` 仅适用于已结束任务；后端保留 `task_id`，分配新的 `run_id` 与执行尝试编号，并进入正式执行链路。新尝试必须先重新经过同会话串行队列与风险治理 / 授权判断，不能只把任务状态改回 `processing`，也不能绕过治理后直接执行。
- **追加边界**：已结束任务不能通过 `agent.task.steer` 补充要求；重启后的追加只在新尝试仍处于可 steering 的 `agent_loop` 执行段，或任务处于 `waiting_auth / blocked` 状态时成立。
- **入参**：任务 ID、动作、动作参数
- **出参**：更新后的任务、状态气泡

### agent.task.control 入参说明

| 字段        | 中文说明     |
| ----------- | ------------ |
| `task_id`   | 目标任务 ID  |
| `action`    | 控制动作，取值来自 `task_control_action` |
| `arguments` | 动作附加参数 |

### agent.task.control 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_control_001",
  "method": "agent.task.control",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_control_001",
      "client_time": "2026-04-07T10:42:00+08:00"
    },
    "task_id": "task_201",
    "action": "pause",
    "arguments": {}
  }
}
```

### agent.task.control 出参说明

| 字段                  | 中文说明     |
| --------------------- | ------------ |
| `data.task.task_id`   | 任务 ID      |
| `data.task.status`    | 最新任务状态 |
| `data.bubble_message` | 控制结果提示 |

### agent.task.control 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_control_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_201",
        "status": "paused"
      },
      "bubble_message": {
        "bubble_id": "bubble_201",
        "task_id": "task_201",
        "type": "status",
        "text": "任务已暂停"
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:42:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.4 `agent.task.events.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：任务详情页、任务巡检页或调试入口需要查看正式运行时事件时
- **系统处理**：按 `task_id` 返回归一化后的 `events` 记录，覆盖 `loop.*`、`task.steered` 等兼容运行时事件
- **入参**：任务 ID、分页参数
- **出参**：事件列表、分页信息

### agent.task.events.list 入参说明

| 字段      | 中文说明    |
| --------- | ----------- |
| `task_id` | 目标任务 ID |
| `limit`   | 每页条数    |
| `offset`  | 偏移量      |

### agent.task.events.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_events_001",
  "method": "agent.task.events.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_events_001",
      "client_time": "2026-04-18T10:43:00+08:00"
    },
    "task_id": "task_201",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.task.events.list 出参说明

| 字段                     | 中文说明 |
| ------------------------ | -------- |
| `data.items`             | 事件列表 |
| `data.items[].event_id`  | 事件 ID  |
| `data.items[].run_id`    | 关联 run |
| `data.items[].task_id`   | 关联 task |
| `data.items[].step_id`   | 关联 step |
| `data.items[].type`      | 事件类型 |
| `data.items[].level`     | 事件级别 |
| `data.items[].payload_json` | 事件负载 JSON |
| `data.items[].created_at` | 创建时间 |
| `data.page`              | 分页信息 |

### agent.task.events.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_events_001",
  "result": {
    "data": {
      "items": [
        {
          "event_id": "evt_loop_run_201_001",
          "run_id": "run_201",
          "task_id": "task_201",
          "step_id": "run_201_step_loop_01",
          "type": "loop.round.completed",
          "level": "info",
          "payload_json": "{\"loop_round\":1,\"stop_reason\":\"completed\"}",
          "created_at": "2026-04-18T10:43:01+08:00"
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-18T10:43:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.5 `agent.task.tool_calls.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：任务详情页、巡检页或排障入口需要查看正式 `tool_call` 记录时
- **系统处理**：按 `task_id` 返回结构化 `tool_call` 列表，补充查看工具输入、输出摘要、状态与耗时
- **入参**：任务 ID、可选 run ID、分页参数
- **出参**：工具调用列表、分页信息

### agent.task.tool_calls.list 入参说明

| 字段      | 中文说明         |
| --------- | ---------------- |
| `task_id` | 目标任务 ID      |
| `run_id`  | 可选的 run 过滤  |
| `limit`   | 每页条数         |
| `offset`  | 偏移量           |

### agent.task.tool_calls.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_tool_calls_001",
  "method": "agent.task.tool_calls.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_tool_calls_001",
      "client_time": "2026-04-18T10:45:00+08:00"
    },
    "task_id": "task_201",
    "run_id": "run_201",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.task.tool_calls.list 出参说明

| 字段                         | 中文说明 |
| ---------------------------- | -------- |
| `data.items`                 | 工具调用列表 |
| `data.items[].tool_call_id`  | 工具调用 ID |
| `data.items[].run_id`        | 关联 run |
| `data.items[].task_id`       | 关联 task |
| `data.items[].step_id`       | 关联 step |
| `data.items[].created_at`    | 创建时间 |
| `data.items[].tool_name`     | 工具名 |
| `data.items[].status`        | 调用状态 |
| `data.items[].input`         | 输入摘要 |
| `data.items[].output`        | 输出摘要 |
| `data.items[].error_code`    | 错误码 |
| `data.items[].duration_ms`   | 耗时 |
| `data.page`                  | 分页信息 |

### agent.task.tool_calls.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_tool_calls_001",
  "result": {
    "data": {
      "items": [
        {
          "tool_call_id": "tool_call_001",
          "run_id": "run_201",
          "task_id": "task_201",
          "step_id": null,
          "created_at": "2026-04-18T10:45:00+08:00",
          "tool_name": "read_file",
          "status": "succeeded",
          "input": {
            "path": "notes/source.txt"
          },
          "output": {
            "path": "notes/source.txt",
            "summary_output": {
              "path": "notes/source.txt",
              "excerpt": "hello from file"
            }
          },
          "error_code": null,
          "duration_ms": 12
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-18T10:45:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.6 `agent.task.steer`

- **状态边界**：`agent.task.steer` 只用于可延迟消费 steering 的任务状态。`processing` 必须是可轮询的 `agent_loop` 执行路径；`waiting_auth` 与 `blocked` 可先记录到恢复执行；`waiting_input / confirming_intent / paused / terminal` 必须拒绝，让客户端改走 `agent.input.submit`、确认或控制链路。
- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户在任务运行中补充新的 follow-up 指令时
- **系统处理**：把新的 steering 文本写入当前 task 的运行态，并允许 Agent Loop 在后续轮次主动消费；若当前 `processing` task 不是可轮询的 `agent_loop` 执行路径，则不得返回“已记录”假确认，应让普通输入创建 / 排队新 task，或等后续恢复执行路径消费已经排队的 steering。
- **入参**：任务 ID、追加消息
- **出参**：更新后的任务对象、状态气泡

### agent.task.steer 入参说明

| 字段      | 中文说明       |
| --------- | -------------- |
| `task_id` | 目标任务 ID    |
| `message` | 追加 steering 文本 |

### agent.task.steer 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_steer_001",
  "method": "agent.task.steer",
  "params": {
    "request_meta": {
      "trace_id": "trace_task_steer_001",
      "client_time": "2026-04-18T10:44:00+08:00"
    },
    "task_id": "task_201",
    "message": "Also include a short summary section."
  }
}
```

### agent.task.steer 出参说明

| 字段                  | 中文说明     |
| --------------------- | ------------ |
| `data.task`           | 更新后的任务 |
| `data.bubble_message` | steering 记录结果提示 |

### agent.task.steer 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_steer_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_201",
        "status": "waiting_auth",
        "loop_stop_reason": null
      },
      "bubble_message": {
        "bubble_id": "bubble_201",
        "task_id": "task_201",
        "type": "status",
        "text": "已记录新的补充要求，后续执行会纳入该指令。"
      }
    },
    "meta": {
      "server_time": "2026-04-18T10:44:01+08:00"
    },
    "warnings": []
  }
}
```

### 8.2.7 Agent Loop 运行时通知

当前阶段，以下通知方法已进入正式调试/流式通道，可用于前端或调试观察运行时进展：

- `task.updated`
- `delivery.ready`
- `approval.pending`
- `task.steered`
- `loop.started`
- `loop.round.started`
- `loop.retrying`
- `loop.compacted`
- `loop.round.completed`
- `loop.completed`
- `loop.failed`
- `task.session_queued`
- `task.session_resumed`
- `mirror.overview.updated`

其中 `loop.*` 事件服务于 Agent Loop / ReAct 运行时观察，不替代正式 `task` 对象本身；当前 query 读侧仍以 `task` 为主对象、以 `agent.task.events.list` 为事件补充视图。

---

### 8.2.8 `agent.task_inspector.config.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户进入巡检配置页时
- **系统处理**：返回当前巡检配置；当前实现从 `settings.task_automation` 读取正式真源，`agent.task_inspector.config.*` 作为巡检配置兼容入口存在，不再维护独立于 settings snapshot 的第二份正式配置
- **入参**：无业务入参
- **出参**：巡检配置快照

### agent.task_inspector.config.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_inspector_config_get_001",
  "method": "agent.task_inspector.config.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_inspector_config_get_001",
      "client_time": "2026-04-07T10:49:00+08:00"
    }
  }
}
```

### agent.task_inspector.config.get 出参说明

| 字段                          | 中文说明               |
| ----------------------------- | ---------------------- |
| `data.task_sources`           | 巡检来源目录           |
| `data.inspection_interval`    | 巡检频率               |
| `data.inspect_on_file_change` | 文件变化时是否立即巡检 |
| `data.inspect_on_startup`     | 启动时是否巡检         |
| `data.remind_before_deadline` | 截止前提醒             |
| `data.remind_when_stale`      | 长时间未处理提醒       |

### agent.task_inspector.config.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_inspector_config_get_001",
  "result": {
    "data": {
      "task_sources": ["D:/workspace/todos"],
      "inspection_interval": {
        "unit": "minute",
        "value": 30
      },
      "inspect_on_file_change": true,
      "inspect_on_startup": true,
      "remind_before_deadline": true,
      "remind_when_stale": true
    },
    "meta": {
      "server_time": "2026-04-07T10:49:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.9 `agent.task_inspector.config.update`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户修改巡检配置并保存时
- **系统处理**：写入巡检配置，返回生效结果；当前实现把该更新收口到 `settings.task_automation`，避免巡检配置与正式 settings snapshot 分裂
- **入参**：巡检来源、巡检频率、触发开关
- **出参**：已生效配置

### agent.task_inspector.config.update 入参说明

| 字段                     | 中文说明           |
| ------------------------ | ------------------ |
| `task_sources`           | 巡检来源目录列表   |
| `inspection_interval`    | 巡检频率           |
| `inspect_on_file_change` | 文件变化时立即巡检 |
| `inspect_on_startup`     | 启动时巡检         |
| `remind_before_deadline` | 截止前提醒         |
| `remind_when_stale`      | 长时间未处理提醒   |

### agent.task_inspector.config.update 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_inspector_config_update_001",
  "method": "agent.task_inspector.config.update",
  "params": {
    "request_meta": {
      "trace_id": "trace_inspector_config_update_001",
      "client_time": "2026-04-07T10:49:30+08:00"
    },
    "task_sources": ["D:/workspace/todos"],
    "inspection_interval": {
      "unit": "minute",
      "value": 15
    },
    "inspect_on_file_change": true,
    "inspect_on_startup": true,
    "remind_before_deadline": true,
    "remind_when_stale": false
  }
}
```

### agent.task_inspector.config.update 出参说明

| 字段                    | 中文说明         |
| ----------------------- | ---------------- |
| `data.updated`          | 是否更新成功     |
| `data.effective_config` | 生效后的巡检配置 |

### agent.task_inspector.config.update 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_inspector_config_update_001",
  "result": {
    "data": {
      "updated": true,
      "effective_config": {
        "task_sources": ["D:/workspace/todos"],
        "inspection_interval": {
          "unit": "minute",
          "value": 15
        },
        "inspect_on_file_change": true,
        "inspect_on_startup": true,
        "remind_before_deadline": true,
        "remind_when_stale": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:49:31+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.10 `agent.task_inspector.run`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户手动点击“立即巡检”时
- **系统处理**：执行一次任务巡检并返回摘要；当 `target_sources` 未提供时，服务端回退到 `settings.task_automation.task_sources`。若来源目录不存在、越界或不可访问，接口返回正式错误而不是成功的 `0/0/0` 摘要
- **入参**：触发原因、目标来源
- **出参**：巡检摘要、建议
- **常见错误**：`1004003 WORKSPACE_BOUNDARY_DENIED`、`1007006 INSPECTION_FILESYSTEM_UNAVAILABLE`、`1007007 INSPECTION_SOURCE_NOT_FOUND`、`1007008 INSPECTION_SOURCE_UNREADABLE`

### agent.task_inspector.run 入参说明

| 字段             | 中文说明         |
| ---------------- | ---------------- |
| `reason`         | 触发原因         |
| `target_sources` | 本次巡检目标目录 |

### agent.task_inspector.run 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_inspector_run_001",
  "method": "agent.task_inspector.run",
  "params": {
    "request_meta": {
      "trace_id": "trace_inspector_run_001",
      "client_time": "2026-04-07T10:50:00+08:00"
    },
    "reason": "manual",
    "target_sources": ["D:/workspace/todos"]
  }
}
```

### agent.task_inspector.run 出参说明

| 字段                 | 中文说明     |
| -------------------- | ------------ |
| `data.inspection_id` | 本次巡检 ID  |
| `data.summary`       | 巡检摘要     |
| `data.suggestions`   | 后续建议列表 |

### agent.task_inspector.run 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_inspector_run_001",
  "result": {
    "data": {
      "inspection_id": "insp_001",
      "summary": {
        "parsed_files": 3,
        "identified_items": 12,
        "due_today": 2,
        "overdue": 1,
        "stale": 3
      },
      "suggestions": [
        "优先处理今天到期的复盘邮件",
        "下周评审材料建议先生成草稿"
      ]
    },
    "meta": {
      "server_time": "2026-04-07T10:50:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.11 `agent.notepad.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户查看近期要做、后续安排、重复事项、已结束时
- **系统处理**：返回指定分组的事项列表，并在同一 read model 里补足 notes 详情页需要的详情字段
- **入参**：分组、分页
- **出参**：事项列表、分页信息

### agent.notepad.list 入参说明

| 字段     | 中文说明 |
| -------- | -------- |
| `group`  | 事项分组，取值来自 `todo_bucket` |
| `limit`  | 每页条数 |
| `offset` | 偏移量   |

### agent.notepad.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_notepad_list_001",
  "method": "agent.notepad.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_notepad_list_001",
      "client_time": "2026-04-07T10:55:00+08:00"
    },
    "group": "upcoming",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.notepad.list 出参说明

| 字段                                  | 中文说明                 |
| ------------------------------------- | ------------------------ |
| `data.items`                          | 事项列表                 |
| `data.items[].item_id`                | 事项 ID                  |
| `data.items[].title`                  | 事项标题                 |
| `data.items[].bucket`                 | 所属分组                 |
| `data.items[].status`                 | 当前状态                 |
| `data.items[].type`                   | 事项类型                 |
| `data.items[].due_at`                 | 到期时间                 |
| `data.items[].agent_suggestion`       | Agent 建议               |
| `data.items[].recurring_enabled`      | 重复规则当前是否开启     |
| `data.items[].note_text`              | 背景与说明               |
| `data.items[].prerequisite`           | 前置条件                 |
| `data.items[].repeat_rule`            | 重复规则文本             |
| `data.items[].next_occurrence_at`     | 下次发生时间             |
| `data.items[].recent_instance_status` | 最近一次执行状态摘要     |
| `data.items[].effective_scope`        | 生效范围                 |
| `data.items[].ended_at`               | 结束时间                 |
| `data.items[].linked_task_id`         | 已转正式任务后的 task ID |
| `data.items[].related_resources`      | 相关资料列表             |
| `data.page`                           | 分页信息                 |

### agent.notepad.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_notepad_list_001",
  "result": {
    "data": {
      "items": [
        {
          "item_id": "todo_001",
          "title": "整理 Q3 复盘要点",
          "bucket": "upcoming",
          "status": "due_today",
          "type": "one_time",
          "due_at": "2026-04-07T18:00:00+08:00",
          "agent_suggestion": "先生成一个 3 点摘要",
          "recurring_enabled": null,
          "note_text": "先把本周关键结论和风险项整理成一页摘要，再决定是否扩写为正式文档。",
          "prerequisite": "确认会议纪要和图表截图都已齐全。",
          "repeat_rule": null,
          "next_occurrence_at": null,
          "recent_instance_status": null,
          "effective_scope": null,
          "ended_at": null,
          "linked_task_id": null,
          "related_resources": [
            {
              "resource_id": "todo_001_minutes",
              "label": "会议纪要目录",
              "path": "workspace/meetings",
              "resource_type": "folder",
              "open_action": "reveal_in_folder",
              "open_payload": {
                "path": "workspace/meetings",
                "task_id": null,
                "url": null
              }
            }
          ]
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:55:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.12 `agent.notepad.update`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户在 notes 详情页对事项执行状态变更动作时
- **系统处理**：按动作更新事项状态或分组，并返回更新后的事项结果
- **入参**：事项 ID、动作类型
- **出参**：更新后的事项、建议刷新的分组、按需附带删除结果

当前稳定支持的动作：

- `complete`
- `cancel`
- `move_upcoming`
- `toggle_recurring`
- `cancel_recurring`
- `restore`
- `delete`

### agent.notepad.update 入参说明

| 字段      | 中文说明               |
| --------- | ---------------------- |
| `item_id` | 事项 ID                |
| `action`  | 变更动作，取值来自 `notepad_action` |

### agent.notepad.update 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_notepad_update_001",
  "method": "agent.notepad.update",
  "params": {
    "request_meta": {
      "trace_id": "trace_notepad_update_001",
      "client_time": "2026-04-07T10:55:30+08:00"
    },
    "item_id": "todo_001",
    "action": "complete"
  }
}
```

### agent.notepad.update 出参说明

| 字段                     | 中文说明                    |
| ------------------------ | --------------------------- |
| `data.notepad_item`      | 更新后的事项；若删除则为 `null` |
| `data.refresh_groups`    | 建议前端重新拉取的分组列表  |
| `data.deleted_item_id`   | 若执行删除动作，返回被删除的事项 ID |

### agent.notepad.update 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_notepad_update_001",
  "result": {
    "data": {
      "notepad_item": {
        "item_id": "todo_001",
        "bucket": "closed",
        "status": "completed",
        "ended_at": "2026-04-07T10:55:31+08:00"
      },
      "refresh_groups": ["upcoming", "closed"],
      "deleted_item_id": null
    },
    "meta": {
      "server_time": "2026-04-07T10:55:31+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.2.13 `agent.notepad.convert_to_task`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户点击“交给 Agent 处理”时
- **系统处理**：将事项转成任务，并保留来源关系
- **入参**：事项 ID、确认标记
- **出参**：新任务对象、更新后的来源事项、建议刷新的事项分组

### agent.notepad.convert_to_task 入参说明

| 字段        | 中文说明         |
| ----------- | ---------------- |
| `item_id`   | 事项 ID          |
| `confirmed` | 是否确认转为任务 |

### agent.notepad.convert_to_task 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_notepad_convert_001",
  "method": "agent.notepad.convert_to_task",
  "params": {
    "request_meta": {
      "trace_id": "trace_notepad_convert_001",
      "client_time": "2026-04-07T10:56:00+08:00"
    },
    "item_id": "todo_001",
    "confirmed": true
  }
}
```

### agent.notepad.convert_to_task 出参说明

| 字段                         | 中文说明                        |
| ---------------------------- | ------------------------------- |
| `data.task.task_id`          | 新任务 ID                       |
| `data.task.title`            | 任务标题                        |
| `data.task.source_type`      | 来源类型，通常为 `todo`         |
| `data.task.status`           | 初始任务状态                    |
| `data.notepad_item.item_id`  | 来源事项 ID                     |
| `data.notepad_item.bucket`   | 来源事项仍所在的 bucket         |
| `data.notepad_item.linked_task_id` | 来源事项关联的新 task ID |
| `data.refresh_groups`        | 建议前端重新拉取的事项分组列表  |

### agent.notepad.convert_to_task 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_notepad_convert_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_401",
        "title": "整理 Q3 复盘要点",
        "source_type": "todo",
        "status": "confirming_intent"
      },
      "notepad_item": {
        "item_id": "todo_001",
        "bucket": "upcoming",
        "linked_task_id": "task_401"
      },
      "refresh_groups": ["upcoming"]
    },
    "meta": {
      "server_time": "2026-04-07T10:56:01+08:00"
    },
    "warnings": []
  }
}
```

---

## 8.3 仪表盘 / 镜子 / 安全卫士

### 8.3.1 `agent.dashboard.overview.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户双击打开仪表盘首页时
- **系统处理**：返回首页焦点摘要、信任摘要等总览数据
- **入参**：是否专注模式、需要包含的区块
- **出参**：首页总览对象

### agent.dashboard.overview.get 入参说明

| 字段         | 中文说明           |
| ------------ | ------------------ |
| `focus_mode` | 是否以专注模式取数 |
| `include`    | 需要返回的首页区块 |

### agent.dashboard.overview.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_overview_001",
  "method": "agent.dashboard.overview.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_dashboard_overview_001",
      "client_time": "2026-04-07T11:00:00+08:00"
    },
    "focus_mode": false,
    "include": [
      "focus_summary",
      "trust_summary",
      "quick_actions",
      "global_state",
      "high_value_signal"
    ]
  }
}
```

### agent.dashboard.overview.get 出参说明

| 字段                              | 中文说明     |
| --------------------------------- | ------------ |
| `data.overview.focus_summary`     | 当前焦点摘要 |
| `data.overview.trust_summary`     | 信任摘要     |
| `data.overview.quick_actions`     | 快速操作     |
| `data.overview.global_state`      | 全局状态     |
| `data.overview.high_value_signal` | 重点事件提示 |

### agent.dashboard.overview.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_overview_001",
  "result": {
    "data": {
      "overview": {
        "focus_summary": {
          "task_id": "task_201",
          "title": "整理 Q3 复盘要点",
          "status": "processing",
          "current_step": "正在生成摘要",
          "next_action": "等待用户查看结果",
          "updated_at": "2026-04-07T10:40:00+08:00"
        },
        "trust_summary": {
          "risk_level": "yellow",
          "pending_authorizations": 1,
          "has_restore_point": true,
          "workspace_path": "D:/CialloClawWorkspace"
        }
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:00:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.3.2 `agent.dashboard.input.start`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户通过中心球语音输入并完成转写时
  - 用户通过中心球轻量文本输入框提交跳转指令时
- **系统处理**：
  - 接收用户输入文本
  - 识别目标页面并解析跳转路由
  - 返回是否匹配成功及目标页面信息
- **入参**：会话 ID、输入模式、输入文本
- **出参**：是否匹配成功、目标页面、目标路由、识别置信度

### agent.dashboard.input.start 入参说明

| 字段         | 中文说明                         |
| ------------ | -------------------------------- |
| `session_id` | 当前会话标识                     |
| `input_mode` | 输入模式，`voice / text`         |
| `input_text` | 用户最终文本，语音输入需先完成转写 |

### agent.dashboard.input.start 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_input_start_001",
  "method": "agent.dashboard.input.start",
  "params": {
    "request_meta": {
      "trace_id": "trace_dashboard_input_start_001",
      "client_time": "2026-04-07T11:00:30+08:00"
    },
    "session_id": "sess_001",
    "input_mode": "voice",
    "input_text": "打开安全卫士页面"
  }
}
```

### agent.dashboard.input.start 出参说明

| 字段               | 中文说明             |
| ------------------ | -------------------- |
| `data.matched`     | 是否成功匹配目标页面 |
| `data.target_page` | 目标页面编码         |
| `data.target_url`  | 目标路由             |
| `data.confidence`  | 识别置信度           |

### agent.dashboard.input.start 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_input_start_001",
  "result": {
    "data": {
      "matched": true,
      "target_page": "security_guard",
      "target_url": "app://dashboard/security",
      "confidence": 0.96
    },
    "meta": {
      "server_time": "2026-04-07T11:00:31+08:00"
    },
    "warnings": []
  }
}
```

补充说明：

- 当前桌面实现中，仪表盘首页的数据获取主要仍由 `agent.dashboard.overview.get`、`agent.dashboard.module.get` 与 `agent.recommendation.get` 组合承接；本文保留 `agent.dashboard.input.start` 原始设计说明，供后续协议冻结时继续对齐。

---

### 8.3.3 `agent.dashboard.module.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户切换仪表盘一级模块时
- **系统处理**：根据模块和标签页返回对应数据
- **入参**：模块名称、标签页
- **出参**：模块数据

### agent.dashboard.module.get 入参说明

| 字段     | 中文说明 |
| -------- | -------- |
| `module` | 模块名称 |
| `tab`    | 子标签页 |

### agent.dashboard.module.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_module_001",
  "method": "agent.dashboard.module.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_dashboard_module_001",
      "client_time": "2026-04-07T11:01:00+08:00"
    },
    "module": "mirror",
    "tab": "daily_summary"
  }
}
```

### agent.dashboard.module.get 出参说明

| 字段              | 中文说明     |
| ----------------- | ------------ |
| `data.module`     | 当前模块     |
| `data.tab`        | 当前标签页   |
| `data.summary`    | 统计摘要     |
| `data.highlights` | 亮点信息列表 |

当 `module = "tasks"` 时，`data.summary` 在通用统计摘要之外还会补充以下任务模块字段：

| 字段                                  | 中文说明 |
| ------------------------------------- | -------- |
| `data.summary.processing_tasks`       | 当前未完成任务中处于 `processing` 的数量 |
| `data.summary.waiting_auth_tasks`     | 当前未完成任务中处于 `waiting_auth` 的数量 |
| `data.summary.blocked_tasks`          | 当前未完成任务中处于 `blocked`、`failed`、`ended_unfinished` 或 `paused` 的数量 |
| `data.summary.focus_task_id`          | 当前焦点任务的 `task_id`，用于把任务模块运行态与 `agent.dashboard.overview.get` 的 `focus_summary.task_id` 对齐 |
| `data.summary.focus_runtime_summary`  | 当前焦点任务的运行态摘要，仅当 `focus_task_id` 与前端持有的焦点任务一致时才应被展示 |
| `data.summary.focus_runtime_summary.events_count` | 焦点任务累计运行事件数 |
| `data.summary.focus_runtime_summary.latest_event_type` | 焦点任务最近一个关键运行事件类型 |
| `data.summary.focus_runtime_summary.active_steering_count` | 焦点任务当前待消费的 steering 条数 |
| `data.summary.focus_runtime_summary.loop_stop_reason` | 焦点任务最近一次 loop 停止原因 |

### agent.dashboard.module.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_module_001",
  "result": {
    "data": {
      "module": "tasks",
      "tab": "focus",
      "summary": {
        "completed_tasks": 3,
        "generated_outputs": 5,
        "authorizations_used": 1,
        "exceptions": 0,
        "processing_tasks": 2,
        "waiting_auth_tasks": 1,
        "blocked_tasks": 1,
        "focus_task_id": "task_focus_001",
        "focus_runtime_summary": {
          "events_count": 6,
          "latest_event_type": "loop.retrying",
          "active_steering_count": 2,
          "loop_stop_reason": "waiting_for_user_confirmation"
        }
      },
      "highlights": [
        "焦点任务仍在执行中，当前步骤为 gather_context。",
        "最近停止原因：waiting_for_user_confirmation。",
        "当前仍有 2 条追加要求待消费。"
      ]
    },
    "meta": {
      "server_time": "2026-04-07T11:01:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.3.4 `agent.mirror.overview.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户进入镜子页时
- **系统处理**：返回历史概要、日报、画像、记忆引用概览
- **入参**：需要包含的镜子区块
- **出参**：镜子概览数据

补充约束：

- 当前 stable 范围内，镜子域冻结 `agent.mirror.overview.get` 与 `agent.mirror.conversation.list`
- `agent.mirror.memory.manage` 仍属于 planned，前端不得绕过协议真源提前调用

### agent.mirror.overview.get 入参说明

| 字段      | 中文说明           |
| --------- | ------------------ |
| `include` | 需要返回的镜子区块 |

### agent.mirror.overview.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_mirror_overview_001",
  "method": "agent.mirror.overview.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_mirror_overview_001",
      "client_time": "2026-04-07T11:01:30+08:00"
    },
    "include": [
      "history_summary",
      "daily_summary",
      "profile",
      "memory_references"
    ]
  }
}
```

### agent.mirror.overview.get 出参说明

| 字段                     | 中文说明           |
| ------------------------ | ------------------ |
| `data.history_summary`   | 历史概要           |
| `data.daily_summary`     | 日报摘要           |
| `data.profile`           | 用户画像           |
| `data.memory_references` | 本次命中的记忆引用 |

### agent.mirror.overview.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_mirror_overview_001",
  "result": {
    "data": {
      "history_summary": [
        "最近两周反复处理周报与复盘类任务",
        "更偏好简洁、可复用的输出格式"
      ],
      "daily_summary": {
        "date": "2026-04-07",
        "completed_tasks": 3,
        "generated_outputs": 5
      },
      "profile": {
        "work_style": "偏好结构化输出",
        "preferred_output": "3点摘要",
        "active_hours": "10-12h"
      },
      "memory_references": [
        {
          "memory_id": "pref_001",
          "reason": "当前任务命中了用户的输出偏好"
        }
      ]
    },
    "meta": {
      "server_time": "2026-04-07T11:01:31+08:00"
    },
    "warnings": []
  }
}
```

### 8.3.4.1 `agent.mirror.conversation.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户在镜子页查看近期会话承接历史时
- **系统处理**：
  - 按时间倒序返回正式持久化的镜子会话记录
  - 支持按 `task_id`、`source` 与 `status` 做过滤
- **入参**：可选过滤条件、分页参数
- **出参**：镜子会话记录列表、分页信息

补充约束：

- 该接口只返回后端正式持久化的记录，不读取前端本地缓存
- 返回顺序固定为 `updated_at DESC, created_at DESC, record_id DESC`
- `record_id` 是后端生成的历史行 ID；调用方不得从 `trace_id` 推导它，也不得假设同一 `trace_id` 只会对应一条历史记录。

### agent.mirror.conversation.list 入参说明

| 字段 | 中文说明 |
| --- | --- |
| `task_id` | 可选任务 ID；为空时不过滤 |
| `source` | 可选来源；用于区分悬浮球、仪表盘、托盘面板 |
| `status` | 可选状态；支持 `submitted / responded / failed` |
| `limit` | 每页条数 |
| `offset` | 分页偏移 |

### agent.mirror.conversation.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_mirror_conversation_list_001",
  "method": "agent.mirror.conversation.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_mirror_conversation_list_001",
      "client_time": "2026-04-07T11:04:30+08:00"
    },
    "task_id": "task_301",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.mirror.conversation.list 出参说明

| 字段 | 中文说明 |
| --- | --- |
| `data.items[].record_id` | 记录 ID |
| `data.items[].trace_id` | 请求 trace ID |
| `data.items[].source` | 请求来源 |
| `data.items[].trigger` | 触发方式 |
| `data.items[].input_mode` | 输入模式 |
| `data.items[].session_id` | 关联会话 ID；无则为 `null` |
| `data.items[].task_id` | 关联任务 ID；无则为 `null` |
| `data.items[].user_text` | 用户输入文本 |
| `data.items[].agent_text` | 返回给用户的气泡文本；无则为 `null` |
| `data.items[].agent_bubble_type` | 气泡类型；无则为 `null` |
| `data.items[].status` | 记录状态 |
| `data.items[].error_message` | 失败信息；无则为 `null` |
| `data.page` | 分页信息 |

### agent.mirror.conversation.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_mirror_conversation_list_001",
  "result": {
    "data": {
      "items": [
        {
          "record_id": "mirror_conversation_1712459070000000000",
          "trace_id": "trace_001",
          "created_at": "2026-04-07T11:04:30+08:00",
          "updated_at": "2026-04-07T11:04:36+08:00",
          "source": "dashboard",
          "trigger": "voice_commit",
          "input_mode": "voice",
          "session_id": "session_001",
          "task_id": "task_301",
          "user_text": "帮我总结这个报错",
          "agent_text": "我已整理出报错原因和下一步建议。",
          "agent_bubble_type": "result",
          "status": "responded",
          "error_message": null
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:04:37+08:00"
    },
    "warnings": []
  }
}
```


### 8.4.4 `agent.settings.runtime_paths.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板需要展示本地运行根目录、数据库、密钥库与工作区路径时
- **系统处理**：返回当前正在生效的运行路径快照，不依赖前端拼接或推断
- **入参**：无业务入参
- **出参**：运行根目录、数据库路径、密钥库路径、工作区路径

补充约束：

- `workspace_path` 必须返回当前运行实例真实使用的工作区路径，而不是仅返回尚未重启生效的设置草稿。
- `runtime_root / database_path / secret_store_path` 用于“查看与定位”，不等价于任何清理或重置动作授权。

### agent.settings.runtime_paths.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_settings_runtime_paths_001",
  "result": {
    "data": {
      "runtime_root": "D:/Users/demo/AppData/Local/CialloClaw",
      "database_path": "D:/Users/demo/AppData/Local/CialloClaw/data/cialloclaw.db",
      "secret_store_path": "D:/Users/demo/AppData/Local/CialloClaw/data/cialloclaw.stronghold.db",
      "workspace_path": "D:/Users/demo/AppData/Local/CialloClaw/workspace"
    },
    "meta": {
      "server_time": "2026-04-21T10:08:00Z"
    },
    "warnings": []
  }
}
```

### 8.4.5 `agent.log.execution.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板或排障视图需要查看全局执行日志时
- **系统处理**：
  - 聚合 `events / tool_calls / audit_records` 正式读模型
  - 按时间倒序返回统一日志结构
- **入参**：可选 `task_id`、可选 `source`、分页参数
- **出参**：执行日志列表、分页信息

补充约束：

- `source` 当前支持 `event / tool_call / audit`。
- 返回顺序固定为 `created_at DESC, log_id DESC`。
- 本接口是查询面，不承接任何恢复、重放或再次执行动作。

### agent.log.execution.list 出参关键字段

| 字段 | 中文说明 |
| --- | --- |
| `data.items[].log_id` | 统一日志 ID，例如 `event:...`、`tool_call:...`、`audit:...` |
| `data.items[].task_id` | 关联任务 ID；无则为 `null` |
| `data.items[].run_id` | 关联运行 ID；无则为 `null` |
| `data.items[].source` | 日志来源 |
| `data.items[].kind` | 事件类型、工具名或审计动作 |
| `data.items[].level` | 归一化级别 |
| `data.items[].summary` | 摘要 |
| `data.items[].detail` | 原始细节或兼容摘要 |
| `data.items[].status` | 来源对象状态；无则为 `null` |
| `data.items[].error_code` | 归一化错误码；无则为 `null` |

### 8.4.6 `agent.log.error.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板需要只查看失败、错误或异常信号时
- **系统处理**：
  - 复用 `agent.log.execution.list` 的统一读模型
  - 仅保留失败工具调用、错误事件与失败审计记录
- **入参**：与 `agent.log.execution.list` 相同
- **出参**：错误日志列表、分页信息

补充约束：

- `agent.log.error.list` 是 `agent.log.execution.list` 的错误子集视图，不会返回正常成功日志。
- 若调用方同时传入 `task_id` 与 `source`，后端先按来源过滤，再做错误子集裁剪。

### 8.4.7 `agent.settings.restore_defaults`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板需要触发“恢复默认设置”时
- **系统处理**：
  - 创建一个正式 `task` 作为本次全局维护动作锚点
  - 生成 `approval_request` 与影响范围说明，等待用户通过 `agent.security.respond` 继续执行
- **入参**：无业务入参
- **出参**：维护任务、授权请求、状态气泡、影响范围

补充约束：

- 本接口本身不直接执行设置恢复，只负责进入授权链路。
- 授权通过后，后端会恢复普通设置默认值、同步巡检默认配置，并清除当前模型 API Key。

### 8.4.8 `agent.memory.delete_all`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板需要触发“删除全部记忆”时
- **系统处理**：
  - 创建正式维护任务
  - 生成 `approval_request`，等待 `agent.security.respond` 决策
- **入参**：无业务入参
- **出参**：维护任务、授权请求、状态气泡、影响范围

补充约束：

- 授权通过后，后端会清空正式记忆摘要与召回命中记录。
- 本接口不会删除任务历史、工作区文件或控制面板普通设置。

### 8.4.9 `agent.task.history.delete`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板需要触发“删除任务历史”时
- **系统处理**：
  - 创建正式维护任务
  - 生成 `approval_request`，等待 `agent.security.respond` 决策
- **入参**：无业务入参
- **出参**：维护任务、授权请求、状态气泡、影响范围

补充约束：

- 授权通过后，后端会清空历史任务、执行日志、审计/恢复记录与镜子会话历史，但会保留当前维护任务作为本次动作的正式留痕锚点。
- 本接口不会删除工作区里已经落盘的文件。

---

### 8.3.5 `agent.security.summary.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户进入安全卫士总览页时
- **系统处理**：返回风险状态、恢复点、费用摘要
- **入参**：无业务入参
- **出参**：安全总览

### agent.security.summary.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_summary_001",
  "method": "agent.security.summary.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_summary_001",
      "client_time": "2026-04-07T11:02:00+08:00"
    }
  }
}
```

### agent.security.summary.get 出参说明

| 字段                                  | 中文说明         |
| ------------------------------------- | ---------------- |
| `data.summary.security_status`        | 安全状态         |
| `data.summary.pending_authorizations` | 待确认数量       |
| `data.summary.latest_restore_point`   | 最近恢复点       |
| `data.summary.token_cost_summary`     | Token 与费用摘要 |

### agent.security.summary.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_summary_001",
  "result": {
    "data": {
      "summary": {
        "security_status": "pending_confirmation",
        "pending_authorizations": 1,
        "latest_restore_point": {
          "recovery_point_id": "rp_001",
          "created_at": "2026-04-07T10:15:00+08:00"
        },
        "token_cost_summary": {
          "current_task_tokens": 2847,
          "current_task_cost": 0.12,
          "today_tokens": 9321,
          "today_cost": 0.46,
          "single_task_limit": 10.0,
          "daily_limit": 50.0,
          "budget_auto_downgrade": true
        }
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:02:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.3.6 `agent.security.pending.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户查看待确认操作列表时
- **系统处理**：返回待确认安全事件
- **入参**：分页参数
- **出参**：审批请求列表、分页信息

### agent.security.pending.list 入参说明

| 字段     | 中文说明 |
| -------- | -------- |
| `limit`  | 每页条数 |
| `offset` | 偏移量   |

### agent.security.pending.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_pending_001",
  "method": "agent.security.pending.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_pending_001",
      "client_time": "2026-04-07T11:03:00+08:00"
    },
    "limit": 20,
    "offset": 0
  }
}
```

### agent.security.pending.list 出参说明

| 字段                          | 中文说明       |
| ----------------------------- | -------------- |
| `data.items`                  | 待确认事件列表 |
| `data.items[].approval_id`    | 审批请求 ID    |
| `data.items[].task_id`        | 关联任务 ID    |
| `data.items[].operation_name` | 操作名称       |
| `data.items[].risk_level`     | 风险等级       |
| `data.items[].target_object`  | 目标对象       |
| `data.page`                   | 分页信息       |

### agent.security.pending.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_pending_001",
  "result": {
    "data": {
      "items": [
        {
          "approval_id": "appr_001",
          "task_id": "task_301",
          "operation_name": "write_file",
          "risk_level": "red",
          "target_object": "C:/Users/demo/Desktop/report.docx",
          "reason": "out_of_workspace",
          "status": "pending"
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:03:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.3.7 `agent.security.respond`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户点击“允许本次”或“拒绝本次”时
- **系统处理**：记录授权结果，更新任务状态
- **入参**：任务 ID、审批 ID、决策结果、是否记住规则
- **出参**：授权记录、任务状态、状态气泡

补充约束：

- 普通审批流返回 `authorization_record`、`task`、`bubble_message`，并可按需附带 `impact_scope`
- 若当前审批对应的是 `agent.security.restore.apply` 的第二阶段执行，则返回形状切换为 `applied`、`task`、`recovery_point`、`audit_record`、`bubble_message`
- `agent.security.respond` 不再额外暴露 `delivery_result`；正式交付结果仍以任务运行态、`delivery.ready` 通知和交付相关接口为准

### agent.security.respond 入参说明

| 字段            | 中文说明     |
| --------------- | ------------ |
| `task_id`       | 目标任务 ID  |
| `approval_id`   | 审批请求 ID  |
| `decision`      | 决策结果     |
| `remember_rule` | 是否记住规则 |

### agent.security.respond 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_respond_001",
  "method": "agent.security.respond",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_respond_001",
      "client_time": "2026-04-07T11:04:00+08:00"
    },
    "task_id": "task_301",
    "approval_id": "appr_001",
    "decision": "allow_once",
    "remember_rule": false
  }
}
```

### agent.security.respond 出参说明

| 字段                        | 中文说明         |
| --------------------------- | ---------------- |
| `data.authorization_record` | 授权记录         |
| `data.task`                 | 更新后的任务状态 |
| `data.bubble_message`       | 状态提示气泡     |
| `data.impact_scope`         | 可选的影响范围摘要 |

### agent.security.respond 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_respond_001",
  "result": {
    "data": {
      "authorization_record": {
        "authorization_record_id": "auth_001",
        "task_id": "task_301",
        "approval_id": "appr_001",
        "decision": "allow_once",
        "remember_rule": false,
        "operator": "user",
        "created_at": "2026-04-07T11:04:01+08:00"
      },
      "task": {
        "task_id": "task_301",
        "status": "processing"
      },
      "bubble_message": {
        "bubble_id": "bubble_301",
        "task_id": "task_301",
        "type": "status",
        "text": "已允许本次操作，任务继续执行"
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:04:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.3.8 `agent.security.restore_points.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户在安全卫士或任务详情中查看恢复点列表时
- **系统处理**：按任务或全局范围返回恢复点列表
- **入参**：可选任务 ID、分页参数
- **出参**：恢复点列表、分页信息

补充约束：

- `data.items[].mode = workspace_snapshot` 表示该恢复点可直接走 `agent.security.restore.apply`。
- `data.items[].mode = manual_backup` 表示该恢复点只提供人工恢复资产，不支持直接走自动回滚。
- 为兼容旧测试桩或历史缓存，只有在某个恢复点未显式返回 `mode` 时，调用方才应按 `workspace_snapshot` 解释。
- 若后端遇到脏数据、未知值或未来版本尚未识别的恢复模式，必须按不可自动恢复处理，不能把它们隐式升级成 `workspace_snapshot`。

### agent.security.restore_points.list 入参说明

| 字段      | 中文说明        |
| --------- | --------------- |
| `task_id` | 可选的任务 ID   |
| `limit`   | 每页条数        |
| `offset`  | 分页偏移        |

### agent.security.restore_points.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_restore_points_001",
  "method": "agent.security.restore_points.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_restore_points_001",
      "client_time": "2026-04-07T11:05:00+08:00"
    },
    "task_id": "task_301",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.security.restore_points.list 出参说明

| 字段                             | 中文说明     |
| -------------------------------- | ------------ |
| `data.items`                     | 恢复点列表   |
| `data.items[].recovery_point_id` | 恢复点 ID    |
| `data.items[].task_id`           | 关联任务 ID  |
| `data.items[].summary`           | 恢复点说明   |
| `data.items[].mode`              | 恢复模式     |
| `data.items[].objects`           | 关联对象清单 |
| `data.page`                      | 分页信息     |

### agent.security.restore_points.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_restore_points_001",
  "result": {
    "data": {
      "items": [
        {
          "recovery_point_id": "rp_001",
          "task_id": "task_301",
          "summary": "write_file_before_change",
          "created_at": "2026-04-07T11:04:30+08:00",
          "mode": "workspace_snapshot",
          "objects": ["workspace/notes/output.md"]
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:05:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.3.9 `agent.security.restore.apply`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户选定某个恢复点并发起回滚时
- **系统处理**：先进入高风险授权链路；授权通过后执行恢复点对应的工作区回滚，并回写任务、安全状态与审计记录
- **入参**：可选任务 ID、恢复点 ID
- **出参**：首次调用返回待授权状态；授权通过后由 `agent.security.respond` 返回最终恢复结果

### agent.security.restore.apply 入参说明

| 字段                | 中文说明       |
| ------------------- | -------------- |
| `task_id`           | 可选的任务 ID  |
| `recovery_point_id` | 目标恢复点 ID  |

### agent.security.restore.apply 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_restore_apply_001",
  "method": "agent.security.restore.apply",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_restore_apply_001",
      "client_time": "2026-04-07T11:06:00+08:00"
    },
    "task_id": "task_301",
    "recovery_point_id": "rp_001"
  }
}
```

### agent.security.restore.apply 出参说明

| 字段                  | 中文说明         |
| --------------------- | ---------------- |
| `data.applied`        | 当前阶段是否已完成恢复；首次调用固定为 `false` |
| `data.task`           | 更新后的任务对象；首次调用进入 `waiting_auth` |
| `data.recovery_point` | 本次使用的恢复点 |
| `data.audit_record`   | 恢复审计记录；首次调用通常为 `null` |
| `data.bubble_message` | 状态提示气泡     |

### agent.security.restore.apply 两阶段说明

1. 第一次调用 `agent.security.restore.apply` 只创建高风险授权请求，并返回 `waiting_auth`
2. 用户确认后，再通过 `agent.security.respond` 执行真正的恢复动作
3. 最终的恢复成功/失败、审计记录和状态气泡在 `agent.security.respond` 响应中返回

补充约束：

- 只有 `mode = workspace_snapshot` 的恢复点支持 `agent.security.restore.apply`。
- `mode = manual_backup` 的恢复点用于维护类高风险动作的人工恢复锚点；调用方不能把它误当作可自动回滚的工作区快照。

### agent.security.restore.apply 错误说明

| 错误码 | 错误名 | 中文说明 |
| ------ | ------ | -------- |
| `1005001` | `SQLITE_WRITE_FAILED` | 恢复点读取或持久化存储查询失败 |
| `1005006` | `RECOVERY_POINT_NOT_FOUND` | 指定恢复点不存在，或与目标任务不匹配 |
| `1005007` | `RECOVERY_POINT_MANUAL_ONLY` | 目标恢复点只提供人工恢复资产，不支持自动回滚 |

### agent.security.restore.apply 首次出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_restore_apply_001",
  "result": {
    "data": {
      "applied": false,
      "task": {
        "task_id": "task_301",
        "status": "waiting_auth"
      },
      "recovery_point": {
        "recovery_point_id": "rp_001",
        "task_id": "task_301",
        "summary": "write_file_before_change",
        "created_at": "2026-04-07T11:04:30+08:00",
        "mode": "workspace_snapshot",
        "objects": ["workspace/notes/output.md"]
      },
      "audit_record": null,
      "bubble_message": {
        "bubble_id": "bubble_301",
        "task_id": "task_301",
        "type": "status",
        "text": "恢复点回滚属于高风险操作，请先确认授权。"
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:06:01+08:00"
    },
    "warnings": []
  }
}
```

### agent.security.respond 恢复完成出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_respond_restore_001",
  "result": {
    "data": {
      "applied": true,
      "task": {
        "task_id": "task_301",
        "status": "completed"
      },
      "recovery_point": {
        "recovery_point_id": "rp_001",
        "task_id": "task_301",
        "summary": "write_file_before_change",
        "created_at": "2026-04-07T11:04:30+08:00",
        "mode": "workspace_snapshot",
        "objects": ["workspace/notes/output.md"]
      },
      "audit_record": {
        "audit_id": "audit_001",
        "task_id": "task_301",
        "type": "recovery",
        "action": "restore_apply",
        "summary": "已根据恢复点 rp_001 恢复 1 个对象。",
        "target": "workspace/notes/output.md",
        "result": "success",
        "created_at": "2026-04-07T11:06:01+08:00"
      },
      "bubble_message": {
        "bubble_id": "bubble_301",
        "task_id": "task_301",
        "type": "status",
        "text": "已根据恢复点 rp_001 恢复 1 个对象。"
      }
    }
  }
}
```

---

### 8.3.10 `agent.security.audit.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户在安全卫士中查看审计明细时
  - 任务详情需要展示审计区时
- **系统处理**：
  - 按指定任务范围或全局范围拉取审计记录
  - 返回稳定分页结构供前端展示
- **入参**：可选任务 ID、分页参数
- **出参**：审计记录列表、分页信息

补充约束：

- 必须传入 `task_id`
- 接口当前只返回指定任务的审计记录
- 当同一 `task_id` 已经发生 `restart` 且当前任务持有新的 `run_id` 时，默认只返回当前执行尝试的审计记录；旧尝试的审计历史可以继续保留在存储层，但不能和当前 attempt 的任务详情 / 审计明细混在一起展示

### agent.security.audit.list 入参说明

| 字段      | 中文说明                 |
| --------- | ------------------------ |
| `task_id` | 可选任务 ID；为空时查询全部任务 |
| `limit`   | 每页条数                 |
| `offset`  | 分页偏移                 |

### agent.security.audit.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_audit_list_001",
  "method": "agent.security.audit.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_audit_list_001",
      "client_time": "2026-04-07T11:07:00+08:00"
    },
    "task_id": "task_301",
    "limit": 20,
    "offset": 0
  }
}
```

### agent.security.audit.list 出参说明

| 字段         | 中文说明     |
| ------------ | ------------ |
| `data.items` | 审计记录列表 |
| `data.page`  | 分页信息     |

### agent.security.audit.list 全局查询示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_audit_list_all_001",
  "method": "agent.security.audit.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_security_audit_list_all_001",
      "client_time": "2026-04-07T11:07:00+08:00"
    },
    "limit": 20,
    "offset": 0
  }
}
```

### agent.security.audit.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_security_audit_list_001",
  "result": {
    "data": {
      "items": [
        {
          "audit_id": "audit_001",
          "task_id": "task_301",
          "type": "recovery",
          "action": "restore_apply",
          "summary": "已根据恢复点 rp_001 恢复 1 个对象。",
          "target": "workspace/notes/output.md",
          "result": "success",
          "created_at": "2026-04-07T11:06:01+08:00"
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:07:01+08:00"
    },
    "warnings": []
  }
}
```


## 8.4 设置中心

### 8.4.1 `agent.settings.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户打开设置面板时
- **系统处理**：返回当前设置快照；若 Stronghold 读取模型凭证配置状态失败，返回统一错误 `STRONGHOLD_ACCESS_FAILED`
- **入参**：查询范围
- **出参**：设置快照

补充约束：

- `agent.settings.get` 返回的是设置读取快照，其中模型配置使用 `models.credentials.*` 组织敏感配置状态与连接信息。
- `provider_api_key_configured` 只表示当前提供方凭证是否已配置成功，不回传任何明文密钥。

### agent.settings.get 入参说明

| 字段                        | 中文说明 |
| --------------------------- | -------- |
| `request_meta.trace_id`     | 请求链路追踪 ID |
| `request_meta.client_time`  | 前端发起时间 |
| `scope`                     | 获取范围；与当前设置分组一致，支持 `all / general / floating_ball / memory / task_automation / models` |

### agent.settings.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_settings_get_001",
  "method": "agent.settings.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_settings_get_001",
      "client_time": "2026-04-07T11:05:00+08:00"
    },
    "scope": "all"
  }
}
```

### agent.settings.get 出参说明

| 字段                                                 | 中文说明 |
| ---------------------------------------------------- | -------- |
| `data.settings.general.language`                     | 界面语言 |
| `data.settings.general.auto_launch`                  | 是否开机自启动 |
| `data.settings.general.theme_mode`                   | 主题模式，取值来自 `theme_mode` |
| `data.settings.general.voice_notification_enabled`   | 是否开启语音通知 |
| `data.settings.general.voice_type`                   | 语音播报音色 |
| `data.settings.general.download.workspace_path`      | 默认工作区路径 |
| `data.settings.general.download.ask_before_save_each_file` | 每次保存文件前是否再次确认 |
| `data.settings.floating_ball.auto_snap`              | 悬浮球是否自动吸边 |
| `data.settings.floating_ball.idle_translucent`       | 空闲时是否半透明 |
| `data.settings.floating_ball.position_mode`          | 悬浮球位置模式，取值来自 `position_mode` |
| `data.settings.floating_ball.size`                   | 悬浮球尺寸档位 |
| `data.settings.memory.enabled`                       | 是否启用记忆 |
| `data.settings.memory.lifecycle`                     | 记忆保留周期 |
| `data.settings.memory.work_summary_interval.unit`    | 工作总结刷新周期单位，取值来自 `time_unit` |
| `data.settings.memory.work_summary_interval.value`   | 工作总结刷新周期数值 |
| `data.settings.memory.profile_refresh_interval.unit` | 画像刷新周期单位，取值来自 `time_unit` |
| `data.settings.memory.profile_refresh_interval.value` | 画像刷新周期数值 |
| `data.settings.task_automation.inspect_on_startup`   | 启动时是否自动巡检 |
| `data.settings.task_automation.inspect_on_file_change` | 文件变化时是否自动巡检 |
| `data.settings.task_automation.inspection_interval.unit` | 巡检周期单位，取值来自 `time_unit` |
| `data.settings.task_automation.inspection_interval.value` | 巡检周期数值 |
| `data.settings.task_automation.task_sources`         | 巡检任务来源目录列表 |
| `data.settings.task_automation.remind_before_deadline` | 是否在截止前提醒 |
| `data.settings.task_automation.remind_when_stale`    | 是否对长期未处理事项提醒 |
| `data.settings.models.provider`                      | 当前模型提供方 |
| `data.settings.models.credentials.budget_auto_downgrade` | 预算不足时是否自动降级 |
| `data.settings.models.credentials.provider_api_key_configured` | 当前提供方 API Key 是否已配置；只返回布尔状态，不返回明文 |
| `data.settings.models.credentials.base_url`          | 模型服务基地址 |
| `data.settings.models.credentials.model`             | 当前生效模型名 |
| `meta.server_time`                                   | 服务端响应时间 |

### agent.settings.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_settings_get_001",
  "result": {
    "data": {
      "settings": {
        "general": {
          "language": "zh-CN",
          "auto_launch": true,
          "theme_mode": "follow_system",
          "voice_notification_enabled": true,
          "voice_type": "default_female",
          "download": {
            "workspace_path": "D:/CialloClawWorkspace",
            "ask_before_save_each_file": true
          }
        },
        "floating_ball": {
          "auto_snap": true,
          "idle_translucent": true,
          "position_mode": "draggable",
          "size": "medium"
        },
        "memory": {
          "enabled": true,
          "lifecycle": "30d",
          "work_summary_interval": {
            "unit": "day",
            "value": 7
          },
          "profile_refresh_interval": {
            "unit": "week",
            "value": 2
          }
        },
        "task_automation": {
          "inspect_on_startup": true,
          "inspect_on_file_change": true,
          "inspection_interval": {
            "unit": "minute",
            "value": 15
          },
          "task_sources": [
            "D:/workspace/todos"
          ],
          "remind_before_deadline": true,
          "remind_when_stale": false
        },
          "models": {
    		"provider": "openai",
    		"credentials": {
      		"budget_auto_downgrade": true,
      		"provider_api_key_configured": true,
      		"base_url": "https://api.openai.com/v1",
      		"model": "gpt-3.5-turbo"
    	  }
  		}
      }
    },
    "meta": {
      "server_time": "2026-04-07T11:05:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.4.2 `agent.settings.update`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户修改设置并点击保存时
- **系统处理**：写入设置并返回当前生效快照与生效方式；`models.api_key` 只用于当前请求写入 Stronghold，不会进入正式设置快照或回传明文。模型路由与凭证更新会在不打断当前任务的前提下重建 future-task 运行时模型配置。
- **入参**：要更新的设置项
- **出参**：已更新字段、生效设置、生效方式、是否需重启

补充约束：

- `agent.settings.update` 的 `models` 采用写入导向结构，使用扁平字段提交提供方、凭证和模型选择。
- 本接口响应里的 `effective_settings.models.*` 保持与更新请求相同的扁平路径，便于前端直接对照本次保存结果。
- `models.api_key` 仅在本次请求内使用；响应体里只通过 `provider_api_key_configured` 回传布尔状态。
- `models.provider`、`models.base_url`、`models.model` 以及模型凭证写入/删除返回 `apply_mode = next_task_effective`；当前正在执行的任务继续使用原有运行时模型快照，后续新任务使用更新后的运行时模型配置。
- 打包版默认 `general.download.workspace_path` 会解析为用户本机的 `AppLocalData/CialloClaw/workspace`，历史 `workspace` 相对占位值会在 settings snapshot 读取时迁移到该绝对目录。
- 打包版默认 `task_automation.task_sources` 会解析为 `${workspace_path}/todos`；settings snapshot 读取时仅会把历史默认占位值（`workspace/todos` 或旧的 `D:/workspace/todos`）迁移到该绝对目录，用户自定义的 `workspace/...` 多根来源会保持原样。
- 桌面宿主 `desktop_get_runtime_defaults` 会同时暴露当前运行时 `data_path`，用于控制面板展示本地存储位置并打开正式 `data` 目录。
- `general.download.workspace_path` 当前不会热重建 bootstrap 时已经绑定的 workspace runtime（例如文件系统、执行后端与 execution workspace）；更新该字段会写入正式 settings snapshot，并返回 `apply_mode = restart_required` 与 `need_restart = true`，用于显式提示“重启后端后生效”。
- 桌面宿主侧 `desktop_open_local_path`、`desktop_reveal_local_path` 只允许使用当前 bootstrap 生效的 `workspace root` 或宿主明确白名单的 runtime 子目录（当前仅接受 `temp/...` 前缀，并解析到 runtime temp 目录）；source-note 路径解析允许使用当前 `workspace root` 或宿主 `runtime root`。这些路径解析都不再回退到编译时 repo root，也不会因为待重启的 `workspace_path` 草稿而漂移本地打开范围。
- 仪表盘 `trust_summary.workspace_path` 与 `out_of_workspace` 判断展示的是当前运行时真实生效的 workspace 根目录，而不是待重启后才会生效的 settings 草稿值。

### agent.settings.update 入参说明

| 字段                                      | 中文说明 |
| ----------------------------------------- | -------- |
| `request_meta.trace_id`                   | 请求链路追踪 ID |
| `request_meta.client_time`                | 前端发起时间 |
| `memory.enabled`                          | 是否启用记忆 |
| `memory.lifecycle`                        | 记忆保留周期 |
| `task_automation.inspection_interval.unit` | 巡检周期单位，取值来自 `time_unit` |
| `task_automation.inspection_interval.value` | 巡检周期数值 |
| `task_automation.inspect_on_file_change`  | 文件变化时是否自动巡检 |
| `models.provider`                         | 要切换到的模型提供方 |
| `models.budget_auto_downgrade`            | 预算不足时是否自动降级 |
| `models.api_key`                          | 当前请求临时写入 Stronghold 的 API Key；保存后不回显 |
| `models.base_url`                         | 模型服务基地址 |
| `models.model`                            | 目标模型名 |

### agent.settings.update 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_settings_update_001",
  "method": "agent.settings.update",
  "params": {
    "request_meta": {
      "trace_id": "trace_settings_update_001",
      "client_time": "2026-04-07T11:06:00+08:00"
    },
    "memory": {
      "enabled": true,
      "lifecycle": "30d"
    },
    "task_automation": {
      "inspection_interval": {
        "unit": "minute",
        "value": 15
      },
      "inspect_on_file_change": true
    },
    "models": {
      "provider": "openai",
      "budget_auto_downgrade": true,
      "api_key": "sk-example",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-3.5-turbo"
    }
  }
}
```

### agent.settings.update 出参说明

| 字段                                                   | 中文说明 |
| ------------------------------------------------------ | -------- |
| `data.updated_keys`                                    | 已更新字段列表，字段路径与请求对象路径一致 |
| `data.effective_settings.memory.enabled`               | 生效后的记忆开关 |
| `data.effective_settings.memory.lifecycle`             | 生效后的记忆保留周期 |
| `data.effective_settings.task_automation.inspection_interval.unit` | 生效后的巡检周期单位 |
| `data.effective_settings.task_automation.inspection_interval.value` | 生效后的巡检周期数值 |
| `data.effective_settings.task_automation.inspect_on_file_change` | 生效后的文件变化自动巡检开关 |
| `data.effective_settings.models.provider`              | 生效后的模型提供方 |
| `data.effective_settings.models.budget_auto_downgrade` | 生效后的自动降级开关 |
| `data.effective_settings.models.provider_api_key_configured` | API Key 是否已配置成功；只返回布尔状态 |
| `data.effective_settings.models.base_url`              | 生效后的模型服务基地址 |
| `data.effective_settings.models.model`                 | 生效后的模型名 |
| `data.apply_mode`                                      | 配置生效方式，取值来自 `apply_mode` |

---

### 8.4.3 `agent.settings.model.validate`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：控制面板点击“测试连接”时，以及模型设置保存前的只读预校验阶段
- **系统处理**：使用当前草稿中的 `models` 路由字段与已保存密钥（或本次请求临时提供的 `models.api_key`）构建一次只读探测，检查文本生成与工具调用是否可用
- **入参**：可选模型草稿字段；未提供的字段沿用当前运行时有效配置
- **出参**：结构化校验结果、失败原因、规范化后的 provider 与文本/工具调用就绪状态

补充约束：

- 本接口是只读探测，不会修改正式设置快照、Stronghold 密钥或当前任务状态。
- `models.api_key` 若在本次请求中提供，仅用于本次校验，不会回显明文。
- 返回 `ok = true` 时表示当前模型配置已通过文本生成与工具调用校验；返回 `ok = false` 时，控制面板应阻止本次保存并直接展示校验失败原因。

### agent.settings.model.validate 出参关键字段

| 字段 | 中文说明 |
| --- | --- |
| `data.ok` | 当前模型配置是否通过校验 |
| `data.status` | 结构化校验状态，例如缺少 API Key、鉴权失败、接口不存在、工具调用不可用 |
| `data.message` | 面向用户的校验说明文案 |
| `data.provider` | 前端提交或当前设置中展示的 provider 文本 |
| `data.canonical_provider` | 后端规范化后实际走的 provider 路由 |
| `data.base_url` | 本次校验使用的模型基地址 |
| `data.model` | 本次校验使用的模型名 |
| `data.text_generation_ready` | 文本生成探测是否成功 |
| `data.tool_calling_ready` | 工具调用探测是否成功 |

### agent.settings.update 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_settings_update_001",
  "result": {
    "data": {
      "updated_keys": [
        "memory.enabled",
        "memory.lifecycle",
        "task_automation.inspection_interval",
        "task_automation.inspect_on_file_change"
      ],
      "effective_settings": {
        "memory": {
          "enabled": true,
          "lifecycle": "30d"
        },
        "task_automation": {
          "inspection_interval": {
            "unit": "minute",
            "value": 15
          },
          "inspect_on_file_change": true
        },
        "models": {
          "provider": "openai",
          "budget_auto_downgrade": true,
          "provider_api_key_configured": true,
          "base_url": "https://api.openai.com/v1",
          "model": "gpt-3.5-turbo"
        }
      },
      "apply_mode": "next_task_effective",
      "need_restart": false
    },
    "meta": {
      "server_time": "2026-04-07T11:06:01+08:00"
    },
    "warnings": []
  }
}
```



---

## 8.5 插件扩展

以下内容在不改变前述插件边界、Go service 统一编排与事件流约束的前提下，对插件扩展模块的 stable 查询接口进行详细展开。当前阶段只冻结列表、详情与运行态查询，不允许前端绕过协议直接触发插件执行。

### 8.5.1 `agent.plugin.runtime.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户打开插件扩展首页的运行态概览区时
  - 用户进入插件详情页，需要刷新健康状态、指标和最近事件时
  - 仪表盘或调试视图需要统一查看插件运行态时
- **系统处理**：
  - 查询当前已声明插件运行态
  - 返回运行态列表、指标快照和最近事件
  - 该接口只用于展示运行态，不承接插件执行
- **入参**：请求链路头
- **出参**：运行态列表、指标快照、最近事件

补充约束：

- `agent.plugin.runtime.list` 是插件扩展模块的运行态读侧接口，前端不得把它误解为“可直接调插件”的入口。
- `data.items`、`data.metrics`、`data.events` 分别代表当前运行态快照、指标快照和最近事件列表；三者允许为空数组，但返回结构必须稳定。
- 插件运行态事件仍通过统一 Notification 通道补充刷新；读侧查询以本接口为准。

### agent.plugin.runtime.list 入参说明

| 字段                        | 中文说明 |
| --------------------------- | -------- |
| `request_meta.trace_id`     | 请求链路追踪 ID |
| `request_meta.client_time`  | 前端发起时间 |

### agent.plugin.runtime.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_plugin_runtime_001",
  "method": "agent.plugin.runtime.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_plugin_runtime_001",
      "client_time": "2026-04-20T10:08:00Z"
    }
  }
}
```

### agent.plugin.runtime.list 出参说明

| 字段                            | 中文说明 |
| ------------------------------- | -------- |
| `data.items`                    | 插件运行态列表，对应 `plugin_runtime_state` 视图对象 |
| `data.metrics`                  | 插件指标快照列表，对应 `plugin_metric_snapshot` 视图对象 |
| `data.events`                   | 最近插件运行事件列表，对应 `plugin_runtime_event` 视图对象 |
| `meta.server_time`              | 服务端响应时间 |

### agent.plugin.runtime.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_plugin_runtime_001",
  "result": {
    "data": {
      "items": [
        {
          "name": "ocr_worker",
          "kind": "worker",
          "status": "running",
          "transport": "named_pipe",
          "health": "healthy",
          "last_seen_at": "2026-04-20T10:08:01Z",
          "last_error": null,
          "capabilities": ["extract_text", "ocr_image", "ocr_pdf"]
        }
      ],
      "metrics": [
        {
          "name": "ocr_worker",
          "kind": "worker",
          "start_count": 3,
          "success_count": 12,
          "failure_count": 1,
          "last_started_at": "2026-04-20T09:58:10Z",
          "last_failed_at": "2026-04-20T09:20:00Z",
          "last_seen_at": "2026-04-20T10:08:01Z"
        }
      ],
      "events": [
        {
          "name": "ocr_worker",
          "kind": "worker",
          "event_type": "plugin.runtime.healthy",
          "payload": {
            "health": "healthy"
          },
          "created_at": "2026-04-20T10:08:01Z"
        }
      ]
    },
    "meta": {
      "server_time": "2026-04-20T10:08:01Z"
    },
    "warnings": []
  }
}
```

---

### 8.5.2 `agent.plugin.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户打开插件扩展首页或插件列表页时
  - 前端需要按关键字、插件类型或健康状态筛选插件卡片时
- **系统处理**：
  - 聚合插件基础信息、来源、权限、能力概览与当前运行态摘要
  - 返回插件视图模型列表，而不是裸 `worker / sidecar` 运行实例列表
- **入参**：分页参数、查询条件、插件类型过滤、健康状态过滤
- **出参**：插件列表、分页信息

补充约束：

- `agent.plugin.list` 的目标对象是 `plugin`，不是运行中实例行；前端插件首页应以本接口返回的 `plugin_id` 作为主列表锚点。
- 当前阶段插件来源可优先返回 `builtin`，`enabled` 也可先按真实后端能力返回稳定布尔值；前端不得因为当前多为内置插件而绕过正式字段。
- 本接口只承接插件列表与筛选查询，不替代 `agent.plugin.runtime.list` 的运行态明细，也不承接插件启停。

### agent.plugin.list 入参说明

| 字段                        | 中文说明 |
| --------------------------- | -------- |
| `request_meta.trace_id`     | 请求链路追踪 ID |
| `request_meta.client_time`  | 前端发起时间 |
| `page.limit`                | 每页条数 |
| `page.offset`               | 偏移量 |
| `query`                     | 按插件名、展示名或摘要做模糊查询 |
| `kinds`                     | 插件类型过滤，取值来自 `plugin_kind` |
| `health`                    | 健康状态过滤，取值来自 `plugin_health_status` |

### agent.plugin.list 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_plugin_list_001",
  "method": "agent.plugin.list",
  "params": {
    "request_meta": {
      "trace_id": "trace_plugin_list_001",
      "client_time": "2026-04-20T10:00:00Z"
    },
    "page": {
      "limit": 20,
      "offset": 0
    },
    "query": "ocr",
    "kinds": ["worker"],
    "health": ["healthy", "unknown"]
  }
}
```

### agent.plugin.list 出参说明

| 字段                                  | 中文说明 |
| ------------------------------------- | -------- |
| `data.items`                          | 插件列表，对应 `plugin` 视图对象列表 |
| `data.items[].plugin_id`              | 插件稳定主键 |
| `data.items[].display_name`           | 前端展示名称 |
| `data.items[].summary`                | 插件摘要 |
| `data.items[].version`                | 当前插件版本 |
| `data.items[].source`                 | 插件来源，取值来自 `plugin_source_type` |
| `data.items[].entry`                  | 插件入口描述 |
| `data.items[].enabled`                | 是否启用 |
| `data.items[].permissions`            | 权限声明列表 |
| `data.items[].capabilities`           | 插件能力概览列表 |
| `data.items[].runtimes`               | 关联运行态快照列表 |
| `data.page`                           | 分页信息 |
| `meta.server_time`                    | 服务端响应时间 |

### agent.plugin.list 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_plugin_list_001",
  "result": {
    "data": {
      "items": [
        {
          "plugin_id": "ocr",
          "name": "ocr",
          "display_name": "OCR Worker",
          "summary": "Extract text from files, images and PDFs.",
          "version": "builtin-1",
          "source": "builtin",
          "entry": "worker://ocr_worker",
          "enabled": true,
          "permissions": ["workspace:read"],
          "capabilities": [
            {
              "tool_name": "extract_text",
              "display_name": "文本提取",
              "description": "通过 OCR worker 从文本文件、PDF 或图片中提取正文内容",
              "source": "worker",
              "risk_hint": "green"
            },
            {
              "tool_name": "ocr_image",
              "display_name": "图片 OCR",
              "description": "通过 OCR worker 对图片执行文字识别",
              "source": "worker",
              "risk_hint": "green"
            },
            {
              "tool_name": "ocr_pdf",
              "display_name": "PDF OCR",
              "description": "通过 OCR worker 对 PDF 执行文本提取或 OCR 识别",
              "source": "worker",
              "risk_hint": "green"
            }
          ],
          "runtimes": [
            {
              "name": "ocr_worker",
              "kind": "worker",
              "status": "running",
              "transport": "named_pipe",
              "health": "healthy",
              "last_seen_at": "2026-04-20T10:00:02Z",
              "last_error": null,
              "capabilities": ["extract_text", "ocr_image", "ocr_pdf"]
            }
          ]
        }
      ],
      "page": {
        "limit": 20,
        "offset": 0,
        "total": 1,
        "has_more": false
      }
    },
    "meta": {
      "server_time": "2026-04-20T10:00:02Z"
    },
    "warnings": []
  }
}
```

---

### 8.5.3 `agent.plugin.detail.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户进入插件详情页时
  - 前端需要展示某个插件下全部工具的入参合同、出参合同与交付映射时
  - 详情页需要按需带出运行态、指标和最近事件时
- **系统处理**：
  - 查询插件基础信息
  - 按需聚合运行态、指标和最近事件
  - 一次性返回插件下全部工具的 `input_contract / output_contract`
- **入参**：插件 ID、是否包含运行态、是否包含指标、是否包含最近事件
- **出参**：插件详情、运行态、指标、最近事件、工具合同列表

补充约束：

- `agent.plugin.detail.get` 是插件详情页的核心接口；当前阶段不单独拆 `agent.plugin.input.get`、`agent.plugin.output.get`。
- `input_contract` 与 `output_contract` 由后端聚合整理后返回；前端不得解析后端源码或假设存在独立 schema 文件仓库。
- `schema_ref` 作为后续正式 schema 文件化的兼容锚点保留；当前允许 `schema_json = null`。
- 当后端仅冻结了 `ToolMetadata` 而尚未补齐字段级 schema 展开时，`input_contract.fields` 与 `output_contract.fields` 允许返回空数组，前端应优先消费 `schema_ref`、展示名、来源与风险信息。
- `include_runtime`、`include_metrics`、`include_events` 省略时按 `true` 处理；若显式传 `false`，对应返回字段应保持空数组而不是缺字段。
- 本接口只返回展示和合同信息，不允许前端借由详情页直接触发插件执行。

### agent.plugin.detail.get 入参说明

| 字段                        | 中文说明 |
| --------------------------- | -------- |
| `request_meta.trace_id`     | 请求链路追踪 ID |
| `request_meta.client_time`  | 前端发起时间 |
| `plugin_id`                 | 目标插件稳定主键 |
| `include_runtime`           | 是否返回运行态列表；缺省按 `true` 处理 |
| `include_metrics`           | 是否返回指标快照；缺省按 `true` 处理 |
| `include_events`            | 是否返回最近事件；缺省按 `true` 处理 |

### agent.plugin.detail.get 入参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_plugin_detail_001",
  "method": "agent.plugin.detail.get",
  "params": {
    "request_meta": {
      "trace_id": "trace_plugin_detail_001",
      "client_time": "2026-04-20T10:05:00Z"
    },
    "plugin_id": "ocr",
    "include_runtime": true,
    "include_metrics": true,
    "include_events": true
  }
}
```

### agent.plugin.detail.get 出参说明

| 字段                                                     | 中文说明 |
| -------------------------------------------------------- | -------- |
| `data.plugin`                                            | 插件基础信息，对应 `plugin` 视图对象 |
| `data.runtimes`                                          | 关联运行态列表 |
| `data.metrics`                                           | 关联指标快照列表 |
| `data.recent_events`                                     | 最近插件事件列表 |
| `data.tools`                                             | 插件工具合同列表，对应 `plugin_tool_contract` 视图对象 |
| `data.tools[].tool_name`                                 | 工具内部名 |
| `data.tools[].display_name`                              | 工具展示名 |
| `data.tools[].description`                               | 工具说明 |
| `data.tools[].source`                                    | 工具来源，取值来自 `plugin_tool_source_type` |
| `data.tools[].risk_hint`                                 | 风险提示，语义对齐 `risk_level` |
| `data.tools[].timeout_sec`                               | 工具超时秒数 |
| `data.tools[].supports_dry_run`                          | 是否支持 dry run |
| `data.tools[].input_contract.schema_ref`                 | 输入合同 schema 引用 |
| `data.tools[].input_contract.schema_json`                | 输入合同 schema JSON；当前可为 `null` |
| `data.tools[].input_contract.fields`                     | 输入合同字段列表；若仅注册了 `schema_ref` 而未展开字段，可返回空数组 |
| `data.tools[].output_contract.schema_ref`                | 输出合同 schema 引用 |
| `data.tools[].output_contract.schema_json`               | 输出合同 schema JSON；当前可为 `null` |
| `data.tools[].output_contract.fields`                    | 输出合同字段列表；若仅注册了 `schema_ref` 而未展开字段，可返回空数组 |
| `data.tools[].delivery_mapping`                          | 该工具结果如何映射到 `tool_call / artifact / delivery_result` |
| `meta.server_time`                                       | 服务端响应时间 |

### agent.plugin.detail.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_plugin_detail_001",
  "result": {
    "data": {
      "plugin": {
        "plugin_id": "ocr",
        "name": "ocr",
        "display_name": "OCR Worker",
        "summary": "Extract text from files, images and PDFs.",
        "version": "builtin-1",
        "source": "builtin",
        "entry": "worker://ocr_worker",
        "enabled": true,
        "permissions": ["workspace:read"]
      },
      "runtimes": [
        {
          "name": "ocr_worker",
          "kind": "worker",
          "status": "running",
          "transport": "named_pipe",
          "health": "healthy",
          "last_seen_at": "2026-04-20T10:05:01Z",
          "last_error": null,
          "capabilities": ["extract_text", "ocr_image", "ocr_pdf"]
        }
      ],
      "metrics": [
        {
          "name": "ocr_worker",
          "kind": "worker",
          "start_count": 3,
          "success_count": 12,
          "failure_count": 1,
          "last_started_at": "2026-04-20T09:58:10Z",
          "last_failed_at": "2026-04-20T09:20:00Z",
          "last_seen_at": "2026-04-20T10:05:01Z"
        }
      ],
      "recent_events": [
        {
          "name": "ocr_worker",
          "kind": "worker",
          "event_type": "plugin.runtime.healthy",
          "payload": {
            "health": "healthy"
          },
          "created_at": "2026-04-20T10:05:01Z"
        }
      ],
      "tools": [
        {
          "tool_name": "ocr_image",
          "display_name": "图片 OCR",
          "description": "通过 OCR worker 对图片执行文字识别",
          "source": "worker",
          "risk_hint": "green",
          "timeout_sec": 30,
          "supports_dry_run": false,
          "input_contract": {
            "schema_ref": "tools/ocr_image/input",
            "schema_json": null,
            "fields": [
              {
                "name": "path",
                "type": "string",
                "required": true,
                "description": "待识别图片路径",
                "example": "D:/workspace/invoice.png"
              },
              {
                "name": "language",
                "type": "string",
                "required": false,
                "description": "OCR 语言提示",
                "example": "zh-CN"
              }
            ]
          },
          "output_contract": {
            "schema_ref": "tools/ocr_image/output",
            "schema_json": null,
            "fields": [
              {
                "name": "path",
                "type": "string",
                "required": true,
                "description": "原始输入路径"
              },
              {
                "name": "text",
                "type": "string",
                "required": true,
                "description": "识别后的正文文本"
              },
              {
                "name": "language",
                "type": "string",
                "required": false,
                "description": "识别语言"
              },
              {
                "name": "page_count",
                "type": "integer",
                "required": true,
                "description": "页数"
              },
              {
                "name": "source",
                "type": "string",
                "required": true,
                "description": "来源运行时，默认 ocr_worker"
              }
            ]
          },
          "delivery_mapping": {
            "emits_tool_call": true,
            "artifact_types": [],
            "delivery_types": ["task_detail"],
            "citation_source_types": []
          }
        }
      ]
    },
    "meta": {
      "server_time": "2026-04-20T10:05:01Z"
    },
    "warnings": []
  }
}
```

## 9. Notification / Subscription 说明

### 9.1 事件语义

- `task.updated`：任务主状态或关键摘要变化；通知参数至少包含 `task_id`、`session_id`、`status`
- `delivery.ready`：正式交付已可被前端承接
- `approval.pending`：出现待授权动作
- `task.steered`：运行中补充要求已经写入任务链
- `task.session_queued`：同一 `session` 下的新任务进入串行等待
- `task.session_resumed`：队列中的任务重新恢复执行
- `mirror.overview.updated`：镜子概览摘要刷新，前端可按 `revision` 触发回拉
- `loop.*`：Agent Loop / ReAct 运行时通知集合，用于调试与任务详情观察
- `plugin.updated`：插件状态变化（包括首次注册后可见的状态快照）
- `plugin.metric.updated`：插件指标变化
- `plugin.task.updated`：插件关联任务变化

以下命名不属于正式前端订阅事件：
- `plugin.registered`：插件注册属于后端内部事件，前端首次可见状态并入 `plugin.updated`
- `overview.ready`：仪表盘初始化结果通过 `agent.dashboard.overview.get` 的正常响应返回

### 9.2 前端使用约束

- 订阅只用于状态同步，不绕过正式请求。
- Notification 到达后，前端应按正式主键刷新对应对象：`task.*` 以 `task_id` 为主键，`plugin.*` 以 `plugin_id` 或运行态主键 `name + kind` 为锚点，而不是临时拼装新对象。
- 若通知缺少关键主键，视为非法事件。

---

## 10. 协议演进规则

1. 新增方法前先判断是否可复用现有方法族。
2. 前端局部动作不得直接升级为正式方法。
3. 新增字段必须同时更新：schema、types、示例、数据设计、模块设计。
4. 若方法仅用于 planned，不得先在前端硬编码调用。
5. 协议的“说明”和“示例”必须随着字段变化一起更新，不能只改字段清单。


## 11. 协议禁止事项

- 不允许扩 REST 作为主协议
- 不允许前端直接消费临时 JSON
- 不允许用字符串猜业务成功失败
- 不允许未登记方法直接进入实现
- 不允许原子功能直接生成临时私有接口
