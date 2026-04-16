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

- `task`：对外主对象，是前端任务列表、详情页、正式交付和安全摘要的统一锚点。
- `run`：执行对象，是后端编排和工具链的运行实例。
- `bubble_message`：轻量承接对象，用于意图确认、状态反馈和短结果返回。
- `delivery_result`：正式交付对象，统一描述结果以气泡、文档、结果页、打开文件或任务详情交付。
- `artifact`：正式产物对象，例如 Markdown 文档、导出文件、截图、结构化结果。
- `approval_request`：待授权对象，高风险动作必须先落到这里。
- `audit_record`：审计对象，用于记录真实动作和结果。
- `memory_summary`：长期记忆对象，用于镜子、偏好、阶段总结。

### 3.4 方法族说明

- `agent.input.*`：近场承接入口，负责长按语音、悬停输入等。
- `agent.task.*`：任务生命周期方法，负责创建、确认、详情、控制。
- `agent.recommendation.*`：主动推荐与反馈。
- `agent.task_inspector.*`：巡检配置与执行。
- `agent.notepad.*`：事项与任务之间的桥接。
- `agent.dashboard.*`：仪表盘首页与模块取数。
- `agent.mirror.*`：镜子和长期记忆视图。
- `agent.security.*`：安全卫士、授权、审计、恢复。
- `agent.settings.*`：设置中心。
- `agent.plugin.* / agent.model.* / agent.skill.*`：扩展能力方法组，当前多数为 planned。

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

- 任务类接口：统一返回 `task`，按需附带 `delivery_result`、`bubble_message`
- 列表类接口：统一返回 `items` + `page`
- 安全类接口：统一返回 `approval_request / authorization_record / audit_record / recovery_point`，按需附带 `impact_scope`
- 设置类接口：统一返回 `effective_settings` 或 `setting_item`、`apply_mode`、`need_restart`

---

## 5. 正式状态枚举与直观解释

### 5.1 任务状态 `task_status`

- `confirming_intent`：等待用户确认系统识别出的意图。
- `processing`：任务正在执行。
- `waiting_auth`：命中高风险动作，等待授权。
- `waiting_input`：等待用户补充必要输入。
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

### 5.13 气泡类型 `bubble_message_type`

- `status`
- `intent_confirm`
- `result`

### 5.14 授权决策 / 状态

- `approval_decision`：`allow_once / deny_once`
- `approval_status`：`pending / approved / denied`

### 5.15 设置相关

- `settings_scope`：`all / general / floating_ball / memory / task_automation / data_log`
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
- `time_unit`：`minute / hour / day / week`
- `run_status`：`processing / completed`

### 5.17 状态使用约束

- 对外产品态统一以 `task_status` 为主。
- 内核态 `run_status` 仅保留最小兼容状态，不得替代 `task_status` 对外暴露。
- 悬浮球主状态机、承接状态机、气泡生命周期都属于前端局部状态，不直接进入正式状态枚举。
- 文档中未登记的状态值不得进入实现。

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
- `1008xxx`：模型、Skill、Blueprint、Prompt 模板、LSP 前馈能力异常，当前为预留段。
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

##### 模型与前馈配置预留

- `1008001` `MODEL_PROVIDER_NOT_FOUND`
- `1008002` `MODEL_NOT_ALLOWED`
- `1008003` `SKILL_NOT_FOUND`
- `1008004` `BLUEPRINT_NOT_FOUND`
- `1008005` `PROMPT_TEMPLATE_NOT_FOUND`
- `1008006` `LSP_DIAGNOSTIC_UNAVAILABLE`

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

#### B. 任务状态 / 巡检

- `agent.task.list`
- `agent.task.detail.get`
- `agent.task.control`
- `agent.task_inspector.config.get`
- `agent.task_inspector.config.update`
- `agent.task_inspector.run`
- `agent.notepad.list`
- `agent.notepad.convert_to_task`

#### C. 仪表盘 / 镜子 / 安全卫士

- `agent.dashboard.overview.get`
- `agent.dashboard.module.get`
- `agent.mirror.overview.get`
- `agent.security.summary.get`
- `agent.security.restore_points.list`
- `agent.security.restore.apply`
- `agent.security.pending.list`
- `agent.security.respond`

#### D. 设置中心

- `agent.settings.get`
- `agent.settings.update`

### 7.2 planned

- `agent.mirror.memory.manage`
- `agent.plugin.list`
- `agent.plugin.enable`
- `agent.plugin.disable`
- `agent.model.list`
- `agent.model.activate`
- `agent.skill.install`
- `agent.skill.list`

### 7.3 原子功能与方法映射说明

以下原子功能不应误判为“需要新增正式方法”：

- **悬浮球单击 / 双击 / 长按 / 上滑 / 下滑 / 悬停** 属于前端交互动作，本地先进入前端状态机，再映射到 `agent.input.submit`、`agent.task.start` 或本地 UI 行为。
- **文本选中承接、文件拖拽承接、错误信息承接** 统一收敛到 `agent.task.start`。
- 气泡置顶 / 删除 / 恢复：优先作为前端局部能力，必要时再引出设置或历史管理接口
- **主动推荐与反馈** 统一使用 `agent.recommendation.get` 和 `agent.recommendation.feedback.submit`。`
- 长结果自动分流：由交付内核决定，不新增方法
- 一键中断：复用 `agent.task.control`
- **插件、多模型、技能安装** 当前阶段先通过 `agent.settings.get / update` 与仪表盘模块承接，待对象、权限与来源字段完全冻结后再升级为独立正式接口。
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
- **系统处理**：
  - 统一承接语音转写文本和轻量输入文本
  - 结合当前页面、选中文本、附带文件做上下文识别
  - 创建 `task`，并进入意图确认或直接执行
- **入参**：会话信息、触发来源、输入内容、上下文、语音元信息、执行偏好
- **出参**：任务对象、气泡消息

### agent.input.submit 入参说明

| 字段                         | 中文说明                       |
| ---------------------------- | ------------------------------ |
| `request_meta.trace_id`      | 请求链路追踪 ID                |
| `request_meta.client_time`   | 前端发起时间                   |
| `session_id`                 | 当前会话 ID                    |
| `source`                     | 来源位置，如悬浮球             |
| `trigger`                    | 触发方式，如语音提交、轻量输入 |
| `input.type`                 | 输入对象类型                   |
| `input.text`                 | 用户输入文本                   |
| `input.input_mode`           | 输入模式，语音或文字           |
| `context.page`               | 当前页面上下文                 |
| `context.selection.text`     | 当前选中文本                   |
| `context.files`              | 当前附带文件列表               |
| `voice_meta`                 | 语音会话元信息                 |
| `options.confirm_required`   | 是否先走意图确认               |
| `options.preferred_delivery` | 偏好的结果交付方式             |

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
    "source": "floating_ball",
    "trigger": "voice_commit",
    "input": {
      "type": "text",
      "text": "帮我总结一下这段内容",
      "input_mode": "voice"
    },
    "context": {
      "page": {
        "title": "当前页面标题",
        "app_name": "browser",
        "url": "local://current-page"
      },
      "selection": {
        "text": "原始选中文本"
      },
      "files": []
    },
    "voice_meta": {
      "voice_session_id": "vs_001",
      "is_locked_session": true,
      "asr_confidence": 0.93,
      "segment_id": "seg_003"
    },
    "options": {
      "confirm_required": true,
      "preferred_delivery": "bubble"
    }
  }
}
```

### agent.input.submit 出参说明

| 字段                     | 中文说明       |
| ------------------------ | -------------- |
| `data.task.task_id`      | 新建任务 ID    |
| `data.task.title`        | 任务标题       |
| `data.task.source_type`  | 任务来源类型   |
| `data.task.status`       | 当前任务状态   |
| `data.task.intent`       | 当前推测意图   |
| `data.task.current_step` | 当前步骤       |
| `data.bubble_message`    | 气泡承接内容   |
| `meta.server_time`       | 服务端响应时间 |

### agent.input.submit 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_input_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_001",
        "title": "总结当前内容",
        "source_type": "voice",
        "status": "confirming_intent",
        "intent": {
          "name": "summarize",
          "arguments": {
            "style": "key_points"
          }
        },
        "current_step": "intent_confirmation"
      },
      "bubble_message": {
        "bubble_id": "bubble_001",
        "task_id": "task_001",
        "type": "intent_confirm",
        "text": "你是想总结这段内容吗？"
      }
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
  - 系统识别到错误信息并进入承接流程
- **系统处理**：
  - 识别任务对象
  - 分析意图
  - 根据配置直接处理或进入意图确认
- **入参**：会话信息、触发方式、任务输入对象、意图、交付偏好

### 8.1.3 `agent.task.artifact.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：任务详情页、仪表盘结果区需要列出指定 `task_id` 的真实 artifact 时。
- **系统处理**：按 `task_id` 查询真实 artifact store，并返回稳定分页结构。
- **入参**：`task_id`、`limit`、`offset`
- **出参**：`items` + `page`

### 8.1.4 `agent.task.artifact.open`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户在任务详情或结果区点击某个 artifact，需要得到稳定打开动作时。
- **系统处理**：根据 `task_id + artifact_id` 定位真实 artifact，并返回与之对齐的 `delivery_result`、`open_action`、`resolved_payload`。
- **入参**：`task_id`、`artifact_id`
- **出参**：`artifact`、`delivery_result`、`open_action`、`resolved_payload`

### 8.1.5 `agent.delivery.open`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：前端需要统一打开最终交付对象时，无论入口来自任务主交付还是某个 artifact。
- **系统处理**：
  - 若携带 `artifact_id`，则优先基于真实 artifact 解析打开动作；
  - 若未携带 `artifact_id`，则基于任务当前 `delivery_result` 解析打开动作；
  - 返回统一的 `delivery_result`、`open_action`、`resolved_payload`，供前端直接执行打开。
- **入参**：`task_id`，可选 `artifact_id`
- **出参**：`delivery_result`、`open_action`、`resolved_payload`，按需附带 `artifact`
- **出参**：任务对象、气泡消息、交付结果（如已完成）

### agent.task.start 入参说明

| 字段                 | 中文说明                           |
| -------------------- | ---------------------------------- |
| `session_id`         | 当前会话 ID                        |
| `source`             | 来源位置                           |
| `trigger`            | 触发动作，如文本选中点击、文件拖拽 |
| `input.type`         | 输入对象类型                       |
| `input.text`         | 文本内容                           |
| `input.files`        | 文件列表                           |
| `input.page_context` | 页面上下文                         |
| `intent.name`        | 明确指定的意图                     |
| `intent.arguments`   | 意图参数                           |
| `delivery.preferred` | 优先交付方式                       |
| `delivery.fallback`  | 兜底交付方式                       |

### agent.task.start 入参示例

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
        "title": "当前页面标题",
        "url": "local://current-page",
        "app_name": "browser"
      }
    },
    "intent": {
      "name": "explain",
      "arguments": {}
    },
    "delivery": {
      "preferred": "bubble",
      "fallback": "workspace_document"
    }
  }
}
```

### agent.task.start 出参说明

| 字段                   | 中文说明     |
| ---------------------- | ------------ |
| `data.task`            | 任务主对象   |
| `data.task.status`     | 当前任务状态 |
| `data.bubble_message`  | 当前气泡内容 |
| `data.delivery_result` | 正式交付结果 |
| `warnings`             | 弱提示信息   |

### agent.task.start 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_task_start_001",
  "result": {
    "data": {
      "task": {
        "task_id": "task_101",
        "title": "解释选中文本",
        "source_type": "selected_text",
        "status": "completed",
        "intent": {
          "name": "explain",
          "arguments": {}
        },
        "current_step": "return_result"
      },
      "bubble_message": {
        "bubble_id": "bubble_101",
        "task_id": "task_101",
        "type": "result",
        "text": "这段内容的意思是：……"
      },
      "delivery_result": {
        "type": "bubble",
        "title": "解释结果",
        "payload": {},
        "preview_text": "结果已通过气泡返回"
      }
    },
    "meta": {
      "server_time": "2026-04-07T10:31:01+08:00"
    },
    "warnings": []
  }
}
```

---

### 8.1.3 `agent.task.confirm`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 系统猜测出意图后，用户点击“确认”
  - 用户认为猜错时，提交修正后的意图
- **系统处理**：
  - 采纳确认结果
  - 更新任务意图
  - 推进到正式执行阶段
- **入参**：任务 ID、是否确认、修正后的意图
- **出参**：更新后的任务对象、状态气泡

### agent.task.confirm 入参说明

| 字段                         | 中文说明             |
| ---------------------------- | -------------------- |
| `task_id`                    | 目标任务 ID          |
| `confirmed`                  | 是否确认系统猜测正确 |
| `corrected_intent.name`      | 修正后的意图名称     |
| `corrected_intent.arguments` | 修正后的意图参数     |

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
    "corrected_intent": {
      "name": "rewrite",
      "arguments": {
        "tone": "professional",
        "length": "short"
      }
    }
  }
}
```

### agent.task.confirm 出参说明

| 字段                  | 中文说明         |
| --------------------- | ---------------- |
| `data.task.task_id`   | 任务 ID          |
| `data.task.status`    | 更新后的任务状态 |
| `data.task.intent`    | 生效后的意图     |
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
        "intent": {
          "name": "rewrite",
          "arguments": {
            "tone": "professional",
            "length": "short"
          }
        },
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

### 8.1.4 `agent.recommendation.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：
  - 用户悬停悬浮球
  - 当前场景满足主动推荐触发条件
- **系统处理**：
  - 结合当前页面、选区、场景信号生成推荐
  - 返回推荐项与对应意图
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
| `data.items[].intent`            | 推荐对应的意图   |

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
          "text": "要不要我帮你总结这段内容？",
          "intent": {
            "name": "summarize",
            "arguments": {
              "style": "key_points"
            }
          }
        },
        {
          "recommendation_id": "rec_002",
          "text": "也可以直接翻译这段内容",
          "intent": {
            "name": "translate",
            "arguments": {
              "target_language": "zh-CN"
            }
          }
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

### 8.1.5 `agent.recommendation.feedback.submit`

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
- **系统处理**：返回任务头部、时间线、成果、记忆引用、安全摘要与单个正式安全锚点
- **入参**：任务 ID
- **出参**：任务详情对象

补充约束：

- `approval_request` 是任务详情里的单个安全锚点，只在当前任务处于 `waiting_auth` 且仍持有活跃正式授权对象时返回；否则返回 `null`。
- 该字段只服务当前 task 的详情承接，不替代 `agent.security.pending.list` 对全局待确认项的聚合查询。
- `security_summary.pending_authorizations` 在任务详情中收敛为 `0 | 1`，仅反映当前 task 是否存在这一个活跃安全锚点。
- `security_summary.latest_restore_point` 的正式类型为 `RecoveryPoint | null`。

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
| `data.timeline`          | 步骤时间线     |
| `data.artifacts`         | 产出物列表     |
| `data.mirror_references` | 命中的镜子记忆 |
| `data.approval_request`  | 当前任务的正式安全锚点 |
| `data.security_summary`  | 安全摘要       |

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
      "mirror_references": [
        {
          "memory_id": "pref_001",
          "reason": "当前任务命中了用户的输出偏好",
          "summary": "偏好简洁三点式摘要"
        }
      ],
      "approval_request": null,
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

### 8.2.4 `agent.task_inspector.config.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户进入巡检配置页时
- **系统处理**：返回当前巡检配置
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

### 8.2.5 `agent.task_inspector.config.update`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户修改巡检配置并保存时
- **系统处理**：写入巡检配置，返回生效结果
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

### 8.2.6 `agent.task_inspector.run`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户手动点击“立即巡检”时
- **系统处理**：执行一次任务巡检并返回摘要
- **入参**：触发原因、目标来源
- **出参**：巡检摘要、建议

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

### 8.2.7 `agent.notepad.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户查看近期要做、后续安排、重复事项、已结束时
- **系统处理**：返回指定分组的事项列表
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

| 字段                            | 中文说明   |
| ------------------------------- | ---------- |
| `data.items`                    | 事项列表   |
| `data.items[].item_id`          | 事项 ID    |
| `data.items[].title`            | 事项标题   |
| `data.items[].bucket`           | 所属分组   |
| `data.items[].status`           | 当前状态   |
| `data.items[].agent_suggestion` | Agent 建议 |
| `data.page`                     | 分页信息   |

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
          "agent_suggestion": "先生成一个 3 点摘要"
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

### 8.2.8 `agent.notepad.convert_to_task`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户点击“交给 Agent 处理”时
- **系统处理**：将事项转成任务，并保留来源关系
- **入参**：事项 ID、确认标记
- **出参**：新任务对象

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

| 字段                    | 中文说明              |
| ----------------------- | --------------------- |
| `data.task.task_id`     | 新任务 ID             |
| `data.task.title`       | 任务标题              |
| `data.task.source_type` | 来源类型，通常为 `todo` |
| `data.task.status`      | 初始任务状态          |

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
      }
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

### 8.3.2 `agent.dashboard.module.get`

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

### agent.dashboard.module.get 出参示例

```json
{
  "jsonrpc": "2.0",
  "id": "req_dashboard_module_001",
  "result": {
    "data": {
      "module": "mirror",
      "tab": "daily_summary",
      "summary": {
        "completed_tasks": 3,
        "generated_outputs": 5,
        "authorizations_used": 1,
        "exceptions": 0
      },
      "highlights": [
        "完成了 3 项内容整理任务",
        "生成了 1 份方案稿和 2 份摘要",
        "命中 2 条历史偏好记忆"
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

### 8.3.3 `agent.mirror.overview.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户进入镜子页时
- **系统处理**：返回历史概要、日报、画像、记忆引用概览
- **入参**：需要包含的镜子区块
- **出参**：镜子概览数据

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

---

### 8.3.4 `agent.security.summary.get`

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

### 8.3.5 `agent.security.pending.list`

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

### 8.3.6 `agent.security.respond`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户点击“允许本次”或“拒绝本次”时
- **系统处理**：记录授权结果，更新任务状态
- **入参**：任务 ID、审批 ID、决策结果、是否记住规则
- **出参**：授权记录、任务状态、状态气泡

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

### 8.3.7 `agent.security.restore_points.list`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户在安全卫士或任务详情中查看恢复点列表时
- **系统处理**：按任务或全局范围返回恢复点列表
- **入参**：可选任务 ID、分页参数
- **出参**：恢复点列表、分页信息

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

### 8.3.8 `agent.security.restore.apply`

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

### agent.security.restore.apply 错误说明

| 错误码 | 错误名 | 中文说明 |
| ------ | ------ | -------- |
| `1005001` | `SQLITE_WRITE_FAILED` | 恢复点读取或持久化存储查询失败 |
| `1005002` | `ARTIFACT_NOT_FOUND` | 指定恢复点不存在，或与目标任务不匹配 |

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

## 8.4 设置中心

### 8.4.1 `agent.settings.get`

- **请求方式**：JSON-RPC 2.0
- **接口调用时机**：用户打开设置面板时
- **系统处理**：返回当前设置快照；若 Stronghold 读取敏感配置状态失败，返回统一错误 `STRONGHOLD_ACCESS_FAILED`
- **入参**：查询范围
- **出参**：设置快照

### agent.settings.get 入参说明

| 字段    | 中文说明                   |
| ------- | -------------------------- |
| `scope` | 获取范围，如全部或单个分组 |

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

| 字段                            | 中文说明         |
| ------------------------------- | ---------------- |
| `data.settings.general`         | 通用设置         |
| `data.settings.floating_ball`   | 悬浮球设置       |
| `data.settings.memory`          | 记忆设置         |
| `data.settings.task_automation` | 任务与自动化设置 |
| `data.settings.data_log`        | 数据与日志设置（包含脱敏机密状态） |

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
        "data_log": {
          "provider": "openai",
          "budget_auto_downgrade": true,
          "provider_api_key_configured": true
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
- **系统处理**：写入设置并返回生效结果；`data_log.api_key` 只用于当前请求写入 Stronghold，不会进入正式设置快照
- **入参**：要更新的设置项
- **出参**：已更新字段、生效设置、生效方式、是否需重启

### agent.settings.update 入参说明

| 字段              | 中文说明           |
| ----------------- | ------------------ |
| `memory`          | 记忆设置变更       |
| `task_automation` | 任务自动化设置变更 |
| `general`         | 通用设置变更       |
| `floating_ball`   | 悬浮球设置变更     |
| `data_log`        | 数据与日志设置变更；允许携带临时写入 Stronghold 的 `api_key` |

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
    "data_log": {
      "provider": "openai",
      "budget_auto_downgrade": true,
      "api_key": "sk-example"
    }
  }
}
```

### agent.settings.update 出参说明

| 字段                      | 中文说明       |
| ------------------------- | -------------- |
| `data.updated_keys`       | 已更新字段列表 |
| `data.effective_settings` | 生效后的设置   |
| `data.apply_mode`         | 生效方式       |
| `data.need_restart`       | 是否需要重启   |

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
        "data_log": {
          "provider": "openai",
          "budget_auto_downgrade": true,
          "provider_api_key_configured": true
        }
      },
      "apply_mode": "immediate",
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

## 9. Notification / Subscription 说明

### 9.1 事件语义

- `task.updated`：任务主状态或关键摘要变化
- `delivery.ready`：正式交付已可被前端承接
- `approval.pending`：出现待授权动作
- `plugin.updated`：插件状态变化（包括首次注册后可见的状态快照）
- `plugin.metric.updated`：插件指标变化
- `plugin.task.updated`：插件关联任务变化

以下命名不属于正式前端订阅事件：
- `plugin.registered`：插件注册属于后端内部事件，前端首次可见状态并入 `plugin.updated`
- `overview.ready`：仪表盘初始化结果通过 `agent.dashboard.overview.get` 的正常响应返回

### 9.2 前端使用约束

- 订阅只用于状态同步，不绕过正式请求。
- Notification 到达后，前端应以 `task_id` 为主键刷新状态，而不是临时拼装新对象。
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
