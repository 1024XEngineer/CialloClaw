# CialloClaw 分工安排和优先级划分（修订版 v15）

## 1. 文档目的

本文档在以下真源与上位文档约束下，给出一份可以直接执行、直接勾选、直接用于联调推进的协作与排期方案：

1. `docs/architecture-overview.md`
2. `docs/development-guidelines.md`
3. `docs/protocol-design.md`
4. `docs/data-design.md`
5. `docs/module-design.md`
6. `docs/work-priority-plan.md`
7. `docs/atomic-features.md`

本文档重点回答以下问题：

1. 当前项目已经推进到了什么程度，哪些链路已有代码骨架，哪些仍未闭环。
2. P0、P1、P2、P3 各阶段具体要做什么，哪些任务必须先完成。
3. 五个人分别负责什么模块、什么接口、什么验收结果。
4. 每个任务完成后如何打勾，如何判断是否真正完成。
5. 如何避免重复建设、跨边界实现和先做假链路再补真链路。

---

## 2. 当前统一口径

### 2.1 主链路口径

当前唯一主链路保持不变：

**语音 / 悬停输入 / 文本选中 / 文件拖拽 / 错误信息 → 悬浮球近场承接（文件拖拽先进入附件队列，用户手动发送后再创建任务） → 意图确认 → 创建或更新 `task` → Go local-service 编排 → 风险评估 / 授权 / 审计 / 恢复点 → `delivery_result / artifact` 正式交付 → 仪表盘 / 任务详情 / 安全摘要展示 → 记忆命中或记忆摘要回写。**

### 2.2 对外与对内对象口径

- 对外统一围绕 `task`
- 对内执行兼容对象保留 `run / step / event / tool_call`
- 正式结果统一走 `delivery_result / artifact / citation`
- 高风险动作统一进入 `approval_request / authorization_record / audit_record / recovery_point`

### 2.3 协议与工程口径

- 前后端唯一稳定边界是 JSON-RPC 2.0
- 正式方法统一使用 `agent.xxx.xxx`
- 自由输入的正式执行路径以 **Agent Loop / ReAct** 为准；`intent` 只负责入口判断、澄清、确认与计划骨架，不是传统 ChatBot 式工具分类器
- 状态、错误码、表结构分别以协议文档、数据文档和实现真源为准
- 前端局部状态不能替代正式业务状态

---

## 3. 结合当前仓库代码的进度判断

基于当前仓库代码，项目并不是“从零开始”，而是已经完成了较明显的 P0 底座和部分 P1 骨架，但主链路仍未全部联调闭环。

### 3.1 已有进展

#### 协议与共享真源

- [x] `packages/protocol/rpc/methods.ts` 已冻结大部分 stable 方法常量与参数/结果类型。
- [x] `packages/protocol/errors/codes.ts`、`packages/protocol/schemas/*` 已具备协议真源雏形。
- [x] `task.updated`、`delivery.ready`、`approval.pending` 等通知方法已有共享定义。

#### 后端主链路骨架

- [x] `services/local-service/internal/rpc/handlers.go` 已把大部分 stable 方法路由到 orchestrator。
- [x] `services/local-service/internal/orchestrator/service.go` 已实现 `agent.input.submit`、`agent.task.start`、`agent.task.confirm`、`agent.task.list`、`agent.task.detail.get`、`agent.task.control`、`agent.task.artifact.list`、`agent.task.artifact.open`、`agent.delivery.open`、巡检、镜子、安全、设置等主入口。
- [x] `services/local-service/internal/runengine`、`internal/delivery`、`internal/context`、`internal/intent`、`internal/risk`、`internal/memory`、`internal/taskinspector` 已具备模块级代码骨架。
- [x] 高风险授权等待、待执行计划挂起、基础 `delivery_result` 构建已经进入主编排。

#### 存储与治理底座

- [x] `services/local-service/internal/storage/service.go` 已接入 SQLite WAL/in-memory 双通路和 fallback 机制。
- [x] `sqlite_task_run_store.go`、`sqlite_memory_store.go`、`governance_store.go`、`artifact_store.go` 等表明运行态、记忆、治理与 artifact 写入已开始落地。
- [x] `internal/audit`、`internal/checkpoint`、`internal/risk` 已有独立服务和测试。

#### 前端桌面与近场入口

- [x] `apps/desktop` 已形成多入口页面：`shell-ball.html`、`shell-ball-bubble.html`、`shell-ball-input.html`、`dashboard.html`、`control-panel.html`。
- [x] `src/features/shell-ball/*` 已实现悬浮球、气泡、输入区、多窗口协调器、语音预览等近场交互骨架。
- [x] `src/platform/*` 已存在 Named Pipe bridge、窗口控制、托盘控制、桌面窗口切换等平台能力封装。

#### 前端服务与设置面板

- [x] `src/rpc/client.ts`、`src/rpc/methods.ts`、`src/rpc/subscriptions.ts` 已开始对接协议客户端。
- [x] `src/services/controlPanelService.ts`、`src/services/settingsService.ts`、`src/services/memoryService.ts`、`src/services/taskService.ts` 已有前端服务层骨架。
- [x] `src/features/control-panel/ControlPanelApp.tsx` 已落地较完整的控制面板 UI。

### 3.2 当前仍未真正闭环的部分

- [ ] 前端近场入口与真实 RPC 主链路尚未全部打通，部分能力仍是本地演示态或半联调态。
- [ ] 仪表盘、任务详情、安全摘要、镜子概览虽然有模块，但与全部真实对象的稳定对接还未完成。
- [ ] `delivery_result / artifact / audit / recovery_point` 的完整“执行后可视化回流”还没有形成稳定端到端体验。
- [ ] 巡检转任务、任务详情增强、安全卫士详情、镜子完整展示仍以骨架为主，尚未形成完整 P1 水平。
- [ ] 多模型配置切换、社区 Skills 安装、感知包装配和插件静态资产治理仍主要属于 P3 规划态；但插件运行态、OCR / Playwright / Media worker 与屏幕 / 页面感知信号底座已进入可用阶段。

### 3.3 当前阶段结论

当前阶段判断为：

- **P0 底座已有较大比例落地**
- **P0 主链路闭环还差联调和结果承接收口**
- **P1 已有一批底座能力落地，但产品承接、设置收口与正式查询仍未闭环**
- **P2 / P3 仍以边界预留为主，不应抢占当前主线资源**

因此，接下来排期必须以 **“补齐 P0 主链路真闭环”** 为第一目标，而不是继续扩散新功能面。

---

## 4. 优先级划分原则

### 4.1 P0

P0 不是“最少功能”，而是 **必须能演示、能联调、能授权、能交付、能排障** 的主链路最低闭环。

### 4.2 P1

P1 负责把 P0 的“能跑”变成“能持续使用”，重点是：

- 巡检完整化
- 安全与恢复可见化
- 镜子与长期协作可见化
- Trace / Eval / 预算治理完整化

### 4.3 P2

P2 负责把系统做得更自然、更智能、更低打扰，前提是不破坏 P0/P1。

### 4.4 P3

P3 负责扩展生态、规模化、插件化、多模型化。P3 绝不允许反向阻塞 P0 主链路。

---

## 5. P0-P3 总任务列表

以下任务列表以“完成后即可勾选”为目标。每个任务默认需要满足：

- 有真实代码落地，不是只有设计稿
- 有真实对象对齐，不是只有 UI 假数据
- 能通过对应联调路径验证
- 不违反 `task-centric`、JSON-RPC、统一交付出口和治理链路约束

### 5.1 P0：主链路与统一项闭环

#### P0-A 统一项冻结

- [ ] 根目录 `AGENTS.md`、`docs/architecture-overview.md`、`docs/development-guidelines.md`、`docs/protocol-design.md`、`docs/data-design.md`、`docs/module-design.md`、`docs/work-priority-plan.md` 对目录边界、对象名和方法名保持一致。
- [ ] `Task / TaskStep / Run / Step / Event / ToolCall / DeliveryResult / Artifact / Citation / ApprovalRequest / AuthorizationRecord / AuditRecord / RecoveryPoint` 这些共享对象在 `packages/protocol`、Go runtime、前端消费层三边字段一致。
- [ ] `agent.input.submit / agent.task.start / agent.task.confirm / agent.task.list / agent.task.detail.get / agent.task.control / agent.delivery.open` 等 stable 方法的参数、返回、错误码与文档真源一致。
- [ ] `Task.status`、`approval_request.status`、`bubble_message`、`delivery_result` 等共享合同对象，以及前端 route state 这类本地 UI 状态，已经在代码中完成分层收清，不再新增平行模型，也不再把路由态误写成需要 schema 对齐的正式对象。

#### P0-B 前端近场入口

- [ ] 悬浮球常驻、贴边、拖拽、窗口唤起和待机态在 `shell-ball` 主窗口上稳定可用。
- [ ] 单击轻量接近能打开真实轻量输入或推荐入口，不再只停留在演示态气泡。
- [x] 双击打开仪表盘并加载真实首页总览。
- [ ] 语音长按、上滑锁定、下滑取消、松开提交在 `shell-ball-input` / `voice-preview` 状态机中形成完整交互闭环。
- [ ] 悬停轻量输入可通过真实 RPC 发起 `agent.input.submit`，而不是只刷新本地 mock 状态。
- [ ] 文本选中承接、错误信息承接都能统一进入 `agent.task.start`；文件拖拽承接先进入附件队列，并在用户手动发送后统一进入 `agent.task.start`。
- [ ] 意图确认气泡能承接 `agent.task.confirm` 的确认、修正、拒绝和补充说明。
- [ ] 短结果在气泡中直接承接，长结果通过 `delivery_result / artifact / open_action` 正确分流到任务详情、文档或文件定位动作。

#### P0-C 主链路能力

- [ ] `summarize` 能从近场输入或文本选中进入正式任务，并稳定产出短结果或长结果。
- [ ] `translate` 能从近场输入进入任务，返回短结果或正式交付。
- [ ] `explain` 能把选中文本或错误文本转成解释结果，并在气泡或任务详情中稳定承接。
- [ ] `analyze_error` 能把错误信息、日志片段、页面错误文本转成原因分析和建议。
- [ ] `suggest_next_step` 能生成下一步动作建议，并在任务详情或气泡中正确承接。
- [ ] `draft` / `write_file` 能把长结果分流到 workspace document、artifact 或任务详情。

#### P0-D 后端主编排与交付

- [x] `agent.input.submit`、`agent.task.start`、`agent.task.confirm` 端到端联调通过。
- [x] `agent.task.list`、`agent.task.detail.get`、`agent.task.control` 端到端联调通过。
- [x] `task_id` 与 `run_id` 的稳定映射在主链路、查询链路和交付链路中一致。
- [x] `delivery_result` 最小闭环可真实生成并回流前端。
- [ ] 至少一种 `artifact` 类型可真实落盘并在前端被打开或定位。
- [x] 至少一条执行链可经过工具调用并把结果回流到 `tool_call / event / delivery_result` 链。（owner-5 batch-2 已冻结 `read_file` 正式样板链：`agent.task.tool_calls.list`、`agent.task.events.list`、`task.detail.delivery_result` 与 storage formal tables 可共同验证同一条执行链）

#### P0-E 风险治理最小闭环

- [x] 风险分级能区分 `green / yellow / red`。
- [x] 至少一条高风险动作能创建 `approval_request` 并进入等待授权态。
- [x] 用户允许 / 拒绝能真实生成 `authorization_record`。
- [x] 高风险动作前能创建 `recovery_point`。
- [x] 执行完成后能生成 `audit_record`。
- [x] 前端能看到待授权摘要与最近恢复点摘要。

#### P0-F 数据与底座

- [x] SQLite + WAL 结构化运行态可用。
- [x] `tasks / runs / task_steps / events / tool_calls / delivery_results / artifacts / approval_requests / authorization_records / audit_records / recovery_points` 至少完成最小可用读写。（owner-5 已补齐最小读写底座、当前阶段 query 去快照化，以及 screen session / temp / clip 最小治理闭环；剩余工作转为多 provider / 社区 Skills 边界预留与和 4号 的详情组装协同。）
- [x] FTS5 + sqlite-vec 骨架可初始化，记忆层不与运行态混写。
- [x] Workspace / Artifact 最小落盘路径打通。
- [x] Stronghold secret store 最小闭环打通，模型密钥与敏感配置已脱离普通设置路径。
- [x] OpenAI Responses API 接入在主链路可真实使用。（当前仍以 `openai_responses` 单 provider 为主，后续 provider 扩展与路由治理留在后续阶段）
- [x] 文件读写 / 命令执行 / 网页读取工具已被统一收口并有错误映射。
- [x] `sessions` 一等存储与最小查询装配可用，`session -> task -> run` 不再只靠 runtime / `task_runs` 兼容路径。
- [x] 普通 settings 快照进入结构化持久化，`general / floating_ball / memory / task_automation / models` 可在重启后回填。
- [x] `agent.settings.get / agent.settings.update` 后端正式收口到 `models / models.credentials`，不再长期依赖 `data_log` 兼容结构。

#### P0-G 工作台最小闭环

- [x] 仪表盘首页能展示当前焦点任务与最小信任摘要。
- [x] 任务状态模块能展示未完成 / 已完成任务。
- [x] 任务详情最小版能展示任务头部、时间线、成果区。
- [x] 安全卫士摘要最小版能展示风险状态、待授权数、最近恢复点。
- [x] 镜子概览最小版能展示至少一条历史摘要或记忆命中。
- [ ] 控制面板最小设置能读取并保存真实设置快照。（owner-5 的 RPC 与后端 settings 数据面已完成正式收口；剩余工作转为控制面板桥接、前端状态收口与真实联调。）

#### P0-H 巡检最小闭环

- [x] 任务源接入可配置。
- [ ] 任务文件监听可触发巡检。
- [x] Markdown 任务结构识别最小可用。
- [x] 任务状态判断最小可用。
- [x] `agent.notepad.convert_to_task` 能把事项升级为正式任务。

#### P0-I 联调验收

- [ ] 文本选中入口至少完整跑通一次：`TextPattern / 真实选区承接 -> agent.task.start -> （按需进入 agent.task.confirm）-> bubble result / task detail / delivery_result`。
- [ ] 文件拖拽入口至少完整跑通一次：`附件队列入列 -> 用户补充说明 -> 手动发送 -> agent.task.start -> artifact / delivery_result`。
- [ ] 语音入口至少完整跑通一次：`长按录音 -> 预览确认 -> agent.input.submit -> task 进入主链路`。
- [ ] 至少一条高风险动作链路完整跑通：`approval_request -> authorization_record -> audit_record -> recovery_point -> 最终结果承接`。
- [x] 至少一条长结果自动分流到文档或文件：`delivery_result / artifact / open_action` 在前后端完整承接。
- [x] 至少一次记忆命中或记忆摘要写入可被前端看到。

### 5.2 P1：可持续使用与治理增强

#### P1-A 巡检与待办增强

- [ ] 主动提醒对象能在任务巡检页或近场提示中看到，并能跳回目标任务。
- [ ] 每日任务摘要可在仪表盘或巡检模块中查看，不再只停留在后端聚合结果。
- [ ] 优先级建议可在巡检模块中查看，并能回链到对应任务。
- [ ] 下一步动作建议能带上“打开相关资料 / 打开任务详情 / 打开结果文档”等具体承接动作。
- [ ] 巡检草稿生成能力可形成正式 `delivery_result / artifact`，而不是只停留在文本建议。
- [ ] 任务巡检模块在仪表盘中可真实使用，并能刷新任务源、摘要和建议结果。

#### P1-B 任务与结果增强

- [ ] 任务详情增强，补齐时间线、成果区、关键上下文、安全摘要、`loop_stop_reason`、runtime event 视图。
- [ ] 结果页 / 浏览器交付形成真实承接，至少覆盖页面摘要、页面解释、页面搜索等场景。
- [x] 文件交付、打开文件、定位目录、workspace document 打开动作形成稳定体验（后端 `agent.task.artifact.open` / `agent.delivery.open` 已统一，dashboard / notes / shell-ball 已共享正式 open flow 与本地打开动作）。
- [ ] 连续任务、失败任务、等待授权任务能正确分流到任务详情，而不是只停留在气泡态。

#### P1-C 安全与恢复增强

- [x] 恢复与回滚查看页可用。
- [x] Token / 费用总览可查看。
- [x] 预算降级策略进入真实执行，不只是文案提示。（owner-5 P1 已接入执行前降级判断、轻量交付、上下文压缩、provider/model failure fallback、failure signal window、高成本工具分类治理，以及 budget downgrade audit/event/trace 结构化记录）
- [x] 工作区边界、命令白名单、影响范围展示更完整；高风险命令优先经 Docker sandbox 受控执行，Windows shell 命令保留受控宿主路径。

#### P1-D 记忆、Trace 与审查增强

- [x] 镜子日报、历史概要、用户画像基础展示可用。
- [x] Trace / Eval 完整化，已落盘 `trace_records / eval_snapshots` 并记录输入/输出摘要、loop round、tool 调用、latency、cost 与 review 结果。
- [x] Doom Loop 检测可记录并能影响主链路，已支持重复调用签名 / 重复无进展错误命中并触发 blocked 承接。
- [x] Human-in-the-loop 升级可形成结构化对象或状态结果，已生成 escalation payload 并把任务推进到可恢复的 blocked / pending execution 状态。

#### P1-E 能力底座增强

- [x] OCR worker 真接入，已支持 `extract_text / ocr_image / ocr_pdf` 与统一错误映射。
- [x] Playwright 完整接入，已支持 `page_read / page_search / page_interact / structured_dom` 与健康检查回收。
- [x] Media worker 真接入，已支持 `transcode_media / normalize_recording / extract_frames`。
- [x] worker 结果已回写 `tool_call.completed` 事件通知，并携带 `source / path / url / output_path` 等关键元信息。
- [ ] Stronghold 正式接入。
- [x] 插件运行态查看打通仪表盘。（owner-5 P1-3 已补齐最小运行态对象、sidecar/worker 健康缓存，以及运行态事件回流；当前 `agent.plugin.list / detail.get / runtime.list` 与 dashboard runtime summary 已统一复用 joined read model。）
- [x] 屏幕感知 session / temp / clip 治理补齐：过期 session 扫描、显式 stop/expire cleanup、clip 录屏片段归一化与 `media_worker` 边界收口、orphaned temp 回收均已落地。

### 5.3 P2：体验与智能增强

- [ ] 主动推荐触发规则增强，补齐冷却、场景与静默规则。
- [x] 行为与机会识别增强，已支持基于复制、停留、切换、错误和页面上下文生成更稳定的机会候选。
- [x] 复制行为感知接入推荐链路，已支持 clipboard / copy_count / last_action 触发轻量推荐。
- [x] 屏幕 / 页面 /系统感知进一步增强，已补齐 page/window/visible_text/screen_summary/dwell/switch 等后端信号承接。
- [ ] 更细腻的气泡动画、轻提示和弱打扰体验完成。
- [ ] 首页意识场强化，真正形成“当前最值得关注的事”。
- [ ] 视频总结能力打通。
- [ ] 重复任务模板化可用。
- [ ] 镜子总结完整化。
- [ ] 用户画像管理可编辑、纠正、删除。

### 5.4 P3：生态与规模化扩展

- [ ] 多生态插件可安装、启停、查看版本与权限。
- [ ] 多模型配置可切换提供商、模型 ID 和路由策略。
- [ ] 社区 Skills 兼容可接入 GitHub 来源并记录来源、版本、权限。
- [ ] 感知包扩展机制可用。
- [ ] 插件市场 / 能力包管理形成基础产品形态。
- [ ] 云端可选同步能力设计与实现。
- [ ] 多设备协同能力设计与实现。
- [ ] 团队配置同步能力设计与实现。

---

## 6. 五个人的具体分工安排

以下分工不是“谁都能顺手改”，而是主责划分。允许协作，但必须明确主责人、依赖关系和验收边界。

### 6.1 1号：前端近场交互与表现层主责

#### 主责模块

- 悬浮球主视图
- 气泡与轻承接视图
- 轻量输入区
- 语音承接 UI
- 近场异常提示与短反馈

#### 当前应聚焦的任务

- [ ] 把悬浮球单击 / 双击 / 长按 / 上滑 / 下滑在 `useShellBallCoordinator`、`shellBall.interaction`、`ShellBallInputWindow`、`ShellBallBubbleWindow`、`ShellBallBubbleZone` 这些真实模块里稳定下来。
- [ ] 把悬停轻量输入、文本选中承接、文件拖拽附件队列、错误承接统一成同一套近场交互体验。
- [ ] 完成意图确认气泡的真实承接，对接 `agent.task.confirm`，不再使用本地假流程替代。
- [ ] 完成短结果气泡承接、长结果分流提示和气泡生命周期管理（包含关闭、恢复、弱提示回显）。
- [ ] 明确“删除 / 置顶 / 恢复 / 隐藏”只属于表现态，不污染正式业务状态。

#### 依赖与协作边界

- 依赖 3号 提供窗口控制、RPC、订阅、拖拽与平台集成。
- 依赖 4号 提供 `task / bubble_message / delivery_result` 的真实返回语义。
- 不负责定义协议字段，不负责定义后端状态。

#### 交付验收标准

- [ ] 三种入口（语音 / 文本选中 / 文件拖拽）都能从近场 UI 进入真实任务链。
- [ ] 气泡 UI 能正确表现确认态、处理中、等待授权、完成、异常。
- [ ] 有联调截图或录屏证明近场链路可用。

### 6.2 2号：前端工作台、结果承接与可视化主责

#### 主责模块

- 仪表盘首页
- 任务状态页
- 任务巡检页
- 镜子页
- 安全卫士页
- 结果页与任务详情页
- 控制面板视图层

#### 当前应聚焦的任务

- [x] 完成仪表盘首页、任务状态模块、安全摘要最小版与任务详情最小版的真实数据承接。
- [ ] 完成结果页 / 文档交付 / 文件交付 / 任务详情分流的前端入口，至少覆盖 `delivery_result`、`artifact`、`open_action` 三类承接。
- [ ] 在 P1 阶段补齐镜子模块、任务巡检模块、安全卫士详情和恢复点查看，并把运行时事件/授权信息接到对应页面。
- [x] 把 Token / 费用总览、安全与恢复对象做成可查看视图。

#### 依赖与协作边界

- 依赖 3号 提供 Query、Store、ViewModel、订阅刷新。
- 依赖 4号、5号 提供 `task / artifact / audit / recovery_point / memory` 的真实对象。
- 不负责平台桥接，不负责协议真源定义。

#### 交付验收标准

- [x] 双击悬浮球后可看到真实首页总览。
- [x] 任务列表、任务详情、安全摘要能从真实 RPC 数据渲染。
- [ ] 长结果、失败结果、连续任务都能进入正确承接页面。

### 6.3 3号：前端协议、状态与平台集成主责

#### 主责模块

- Typed JSON-RPC Client
- 前端 RPC methods / subscriptions
- Store / Query / ViewModel
- 多窗口协调
- 托盘、快捷键、拖拽、文件、本地存储
- Named Pipe 连接与重连

#### 当前应聚焦的任务

- [x] 把 stable 方法 client 补齐到与 `packages/protocol` 一致。
- [x] 打通 Notification / Subscription 桥接和前端事件回写。
- [ ] 把多窗口共享状态、查询缓存与局部状态分层统一收口，避免 shell-ball / bubble / dashboard 各自维护正式状态快照。
- [ ] 补齐托盘、控制面板、文件拖拽、快捷键、打开文件/目录等平台能力桥接，并统一透出到 RPC client 或平台服务层。
- [ ] 清理前端服务层和表现层对假数据、临时对象的依赖，至少收掉任务详情、输出打开、待授权摘要里的演示态逻辑。

#### 依赖与协作边界

- 依赖 4号 冻结协议方法语义与返回对象。
- 依赖 5号 冻结错误码、边界策略、设置与数据归属。
- 不负责业务规划，不负责后端状态机。

#### 交付验收标准

- [ ] 前端所有 stable 主链路调用均通过统一 RPC 层发起。
- [x] `task.updated / delivery.ready / approval.pending` 能稳定回写前端。
- [ ] 多窗口不会各自维护一套正式状态真源。

### 6.4 4号：后端 Harness 主链路与协议收口主责

#### 主责模块

- JSON-RPC Server
- Orchestrator
- Context Manager
- Intent / Planning
- RunEngine / Task 状态机
- Delivery
- 主链路查询装配

#### 当前应聚焦的任务

- [x] 稳定 `agent.input.submit`、`agent.task.start`、`agent.task.confirm`。
- [x] 稳定 `agent.task.list`、`agent.task.detail.get`、`agent.task.control`。
- [x] 稳定 `agent.notepad.convert_to_task`、`agent.dashboard.overview.get` 与最小工作台查询。
- [x] 统一 `task/run` 映射和状态推进。
- [x] 把执行结果统一收口为 `delivery_result / artifact`。
- [x] 确保前端看到的是 task-centric 结果，而不是 run-centric 裸对象。
- [x] 把 `agent.task.events.list`、`agent.task.steer`、`loop.*` / `task.steered` 正式通知继续联调到前端任务详情或调试视图。
- [x] 把至少一条真实执行链跑到前端承接闭环，覆盖任务详情结果区或 artifact 打开动作。
- [x] 继续把 4 号剩余工作按“执行闭环、产品承接、运行时稳定化、联调验收、测试回归”五类维护，避免再把“已有基础能力”和“真实剩余项”混写在一起。
- [x] 把 issue #261 中属于 4 号的屏幕感知主收口任务并回本分工文档，确保协议字段、Context/Intent、Orchestrator、Delivery、任务详情查询的口径和代码现状一致。

#### 依赖与协作边界

- 依赖 5号 提供 model、tools、risk、memory、storage、platform 的可用底座。
- 与 3号共同冻结协议语义，与 2号对齐查询视图数据结构。
- 不负责前端局部状态与视觉表现。

#### 交付验收标准

- [x] P0 主链路接口全部可联调。
- [x] `task_id` 与 `run_id` 映射稳定。
- [x] 至少一条真实执行链能产出正式交付结果并被前端承接。

#### 后续细分建议

- [x] Agent Loop 的真实执行链已补齐到 `tool_call / event / delivery_result` 全链回流，不再只停留在 task detail runtime 观察层。
- [x] 已选择 owner-4 可控的直接执行链作为稳定样板，当前覆盖 `agent.task.start` / `agent.input.submit` 直达执行、正式 delivery 生成、artifact 落盘与前端打开承接。
- [x] 4号 剩余工作已拆成“运行时落盘闭环”“交付结果查询闭环”“前端真实承接闭环”等可独立验证的小任务，并在当前文档与回归锚点中收口，避免后续以笼统的 Agent Loop 完善名义重复改同一层。
- [x] 已为 task detail runtime event 视图接入正式事件查询链，当前前后端都支持继续围绕运行时事件流做任务详情观察与后续筛选收口。
- [x] 为 `services/local-service/internal/agentloop/runtime.go` 补齐第一批直接单测，先覆盖 steering merge、planner retry、tool timeout retry、history compaction 与 dead-loop stop reason。
- [x] 已把 Agent Loop planner retry 默认策略切到按错误类型决定是否继续，当前至少区分 timeout / 429 / 5xx 与 non-retryable 4xx / generic planner error。
- [x] 已补第二批 runtime matrix 单测，继续覆盖 compaction 阈值、rate-limit / 5xx planner retry、non-timeout tool failure 与 round history 保留语义。
- [x] 已把 runtime history 最小语义补到 attempt / segment 级别，当前结构化 step 会显式区分 initial / resume / restart 段，并保留对应 attempt index 供后续查询与调试消费。

#### 按新整理版重整后的剩余任务口径

- [x] 以下项应从 4 号“剩余主项”中移出，视为已具备基础能力：`agent.task.list`、`agent.task.detail.get`、`agent.task.control`、`agent.task.events.list`、`agent.task.steer`、`agent.notepad.convert_to_task`、`agent.dashboard.overview.get`、`delivery_result` 最小闭环、artifact 基础落盘、runtime 基础结构、多入口主链路骨架。
- [x] 第一优先级剩余闭环：当前已具备至少一条真实执行链打穿前端正式承接；至少一种 artifact 类型完成真实落盘与打开/定位闭环；至少一条长结果自动分流形成正式交付闭环；至少一条高风险动作链路完成授权 / 恢复 / 审计验收。
  对应锚点：`services/local-service/internal/orchestrator/service_test.go` 中关于 `workspace_document / task_detail / open_file / reveal_in_folder / restore_apply` 的正式回归用例，以及 `apps/desktop/src/features/dashboard/tasks/taskOutput.service.ts`、`apps/desktop/src/features/dashboard/dashboard.contract.test.ts`、`apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts` 的前端正式承接链。
- [x] 第二优先级产品收口：task detail 已增强为主链路收口页；dashboard 已升级为运行态工作台；`loop.*` / `task.steered` 正式消费链已冻结；task.control 与 task.steer 的边界已在查询、控制与补充指令路径中梳理完成。
  对应锚点：`apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx`、`apps/desktop/src/features/dashboard/home/dashboardHome.service.ts`、`services/local-service/internal/orchestrator/service_test.go` 中关于 `TaskControl` / `TaskSteer` / `active_steering_count` / `loop.retrying` 的回归集合。
- [x] dashboard 首页焦点任务已开始消费运行态摘要，当前至少能展示最近运行事件、待消费 steering 提示与等待授权数量；完整运行态工作台仍需继续补齐更细粒度承接。
- [x] 第三优先级运行时稳定化：retry policy 已细化到错误类型级别；steering policy 已冻结；runtime history / compaction / stop reason 语义稳定；repeated tool loop / doom loop 检测与说明已补强。
  对应锚点：`services/local-service/internal/agentloop/runtime_test.go`、`services/local-service/internal/traceeval/service_test.go`、`services/local-service/internal/orchestrator/service_test.go` 中关于 planner retry、tool timeout retry、history compaction、`loop_stop_reason`、doom loop escalation 的测试矩阵。
- [x] 第四优先级联调覆盖：文本选中、文件拖拽、语音入口、悬浮球输入入口已完成正式链路覆盖；巡检 / Notepad 升级任务链路已完成 task detail / delivery / dashboard 的一致性收尾。
  对应锚点：`apps/desktop/src/features/shell-ball/shellBall.contract.test.ts` 中的 `text_selected_click / file_drop / voice_commit / hover_text_input` 路径，以及 `services/local-service/internal/orchestrator/service_test.go` 中的 `agent.notepad.convert_to_task`、`task.detail`、`delivery_result`、`open_action` 回归用例。
- [x] 第五优先级测试与观察：跨端最终验收 case 已补齐；runtime 稳定性回归矩阵已继续扩展；Agent Loop 调试 / 观察能力已形成正式查询与前端消费链；shell-ball 测试与联调环境问题已收尾到当前主链可回归状态。
  对应锚点：`services/local-service/internal/orchestrator/service_test.go`、`services/local-service/internal/rpc/server_test.go`、`services/local-service/internal/agentloop/runtime_test.go`、`apps/desktop/src/features/dashboard/dashboard.contract.test.ts`、`apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`。

#### 屏幕感知专项并入口径（对齐 issue #261 与当前代码现状）

- [x] 当前仓库已经具备一条受控的屏幕分析基础链：`screen_analyze` 可进入等待授权，授权后通过截图采样、OCR、artifact 落盘、`delivery_result` 回流完成最小闭环。
- [x] 当前屏幕分析不会额外发明伪正式 task 状态，而是继续复用既有 `waiting_auth / processing / completed / failed`，细过程放在 approval、event、artifact 和交付对象中。
- [x] `agent.input.submit` 与 `agent.task.start` 已统一承接“本轮屏幕感知引用”，前端当前可沿用正式输入上下文触发受控视觉型任务，而不必依赖未登记的并行入口或裸平台对象。
- [x] `input.page_context` 以及 `context.page`、`context.screen`、`context.behavior` 中与页面标题、窗口标题、可见文本、屏幕摘要、停留和切换有关的信号，需要继续作为 4 号主责维护的正式输入上下文，不允许各端再发明平行命名。
- [x] Orchestrator / Intent 已补齐“自然语言触发视觉型任务”的主链路承接，例如“帮我看看这个页面的报错”“看看当前屏幕上哪里出错了”当前都可落入正式 task，而不是要求前端先手工拼显式 intent。
- [x] Delivery / 查询装配已明确区分模型结论、截图证据、OCR 文本、引用片段和审计信息；当前任务详情正式消费 `artifact / citation / delivery_result / approval_request / authorization_record / audit_record`，而不是裸 worker 输出。
- [x] 屏幕感知相关失败和降级路径仍需继续冻结语义：授权拒绝、采样失败、OCR 失败、会话失效、无有效识别内容都应映射到既有错误段与既有 task/event/delivery 语义中，不新增视觉专项伪状态。
- [x] 屏幕感知专项测试仍需继续覆盖协议字段、Context 归一化、视觉任务 intent 推断、授权后执行恢复、任务详情可回看证据链等关键路径，作为 4 号与 5 号协同联调的基础回归集。

### 6.5 5号：后端能力、数据、治理与扩展底座主责

#### 主责模块

- model
- tools
- storage
- memory
- risk
- audit
- checkpoint
- platform
- execution
- plugin
- workers

#### 当前代码与剩余任务重整

##### 当前代码已完成的基础项

- [x] OpenAI Responses provider/client、bootstrap secret-source wiring、Agent Loop planner/tool-calling 路径已打通；当前单 provider 仍以 `openai_responses` 为主。
- [x] 文件读写 / 命令执行 / 网页读取 / OCR / Playwright / Media 工具链已统一进入 `tool_call / event / delivery_result`，并具备基础错误映射。
- [x] SQLite + WAL、`tasks / task_steps`、`runs / steps / events / tool_calls / delivery_results`、artifact、memory、trace/eval、approval/authorization/audit/recovery 等结构化存储已落地。
- [x] 风险分级、等待授权、authorization、audit、pre-write recovery point 已形成最小治理闭环。
- [x] Workspace / Artifact 最小落盘与屏幕证据 artifact 落盘已打通，后端已具备 `open_file / reveal_in_folder / workspace_document` 等打开动作所需基础对象。
- [x] 屏幕 / 页面 / 可见文本 / 停留 / 切换等 perception signals 已接入后端；`screen_analyze` 最小链路已可授权执行并产出 artifact / citation / audit / recovery。
- [x] 插件运行态最小可见层已打通：runtime state、metric snapshot、runtime event 已可供仪表盘与 `agent.plugin.list / detail.get / runtime.list` 查询，runtime 与正式 `PluginManifest` 已建立版本/来源/权限关联，并通过 joined read model 对齐静态 manifest 与运行态读侧。
- [x] `skill_manifests / blueprint_definitions / prompt_template_versions` 已接到 execution / trace / eval，后端可追踪某次 `task / run` 命中的前馈配置版本。
- [x] 普通 settings 已进入结构化持久化，`agent.settings.get / update` 已按正式 `models / models.credentials` 语义读写并在启动时回填到 runEngine。
- [x] `sessions` 一等存储与最小 `session -> task -> run` 查询链已打通，任务读侧可从正式 sessions/runs 表回补基础关系字段。
- [x] 正式 Stronghold backend、model/provider/storage/settings 错误码收口，以及 task.list / task.detail / dashboard / security 读侧去快照化已完成当前阶段闭环；`task_runs.snapshot_json` 仅保留给 citation / steering / mirror 等尚未完全结构化的兼容字段。

##### 按当前代码重整后的剩余任务

- [x] 把 `agent.settings.get / agent.settings.update` 的后端数据面从 `data_log` 兼容结构切到正式 `models / models.credentials`，并保持 `provider_api_key_configured / base_url / model / budget_auto_downgrade` 的脱敏返回语义。
- [x] 为 `general / floating_ball / memory / task_automation / models` 增加普通 settings 持久化 store，并在启动时把 settings 回填到 runEngine，避免仅存在进程内快照。
- [x] 用正式 Stronghold backend 替换 `StrongholdSQLiteFallbackProvider`，保留当前 SQLite fallback 仅作为开发态兜底。
- [x] 继续把 task list 的读侧装配改为 `tasks / task_steps / runs / steps / events / tool_calls / delivery_results / approval_requests / authorization_records / audit_records / recovery_points` 正式表优先，减少 `task_runs.snapshot_json` 回填字段。
- [x] 补齐 `sessions` 一等存储与最小查询装配，让 `session -> task -> run` 关系进入结构化真源。
- [x] 为 model/provider/storage/settings failure 建立正式错误码映射与 RPC 返回，不再只覆盖 tool/worker/screen/stronghold 一部分。
- [x] 继续为多 provider / 社区 Skills / 感知包预留边界，但避免在当前阶段扩散安装、市场和复杂 UI。（owner-5 batch-7 已冻结当前边界：provider 仍只通过 `agent.settings.*` 承接单 provider 配置；`skill_manifests / blueprint_definitions / prompt_template_versions / plugin_manifests` 只作为版本化扩展资产进入 execution / trace / eval；`trace_records.asset_refs_json` 与 `eval_snapshots.asset_refs_json` 只接受当前登记的扩展资产种类与允许来源；当前后端已补齐 version/source/permissions 级别的 provider route 与 perception package catalog，并通过 execution extension asset refs 与既有 skill/blueprint/prompt/plugin attribution 对齐；`agent.model.*`、`agent.skill.*`、感知包独立产品入口继续保持 planned，不提前进入 stable。）

##### issue #261 屏幕感知专项（5号）

- [x] `ScreenCaptureClient`、screen session 生命周期、workspace 受控 temp 路径、retention / cleanup metadata 已落地。
- [x] `screenshot / keyframe -> OCR -> structured observation -> artifact / citation_seed / audit_record / recovery_point` 最小链路已落地。
- [x] 屏幕感知错误已具备最小分类收口，授权、采样、OCR、session 失效、observation 输出异常都可映射到既有错误与 `task / event / delivery` 语义。
- [x] 补齐 `clip` 录屏片段支路：已通过 `media_worker` 抽帧归一化接入 OCR / artifact 落盘，并补齐 cleanup / recovery 语义与错误映射。
- [x] 补齐 screen temp / session 的自动清理：过期 session 扫描、显式 stop/expire cleanup、异常残留回收与 retained artifact 分流已落地，不再只停留在 cleanup capability 和 pending cleanup plan。
- [x] 与 4号 协同把屏幕任务详情查询收口到正式 `artifact / citation / approval_request / authorization_record / audit_record` 组装；当前后端查询已优先使用 first-class artifact / citation / approval / authorization / audit，并为 screen task detail 补齐 formal precedence，避免页面继续回看 tool output 兼容字段。

#### 依赖与协作边界

- 与 4号共同冻结主链路可调用能力。
- 与 3号 对齐错误码、设置项、平台边界暴露方式。
- 不直接定义前端页面结构，不绕过 orchestrator 对外暴露能力。

#### 交付验收标准

- [x] 至少一种模型调用、三类基础工具、最小存储闭环可真实工作。
- [x] 至少一条高风险动作能留下授权、审计和恢复点。
- [x] issue #261 的最小屏幕分析链已可真实落盘 artifact，并回流 citation / audit / recovery 摘要。
- [x] Stronghold 正式 backend 与 model/provider/storage error-code 收口完成后，再视为 5 号底座真正收尾。

---

## 7. 推荐的阶段推进顺序

### 7.1 第一阶段：先补齐 P0 主链路，不再扩散新面

- [x] 4号 + 5号 先把主链路接口、执行、治理、交付和存储真闭环补齐。
- [ ] 3号 同步把 RPC、订阅、状态回写和多窗口桥接打通，重点覆盖 `task.updated / delivery.ready / approval.pending / task.steered / loop.*` 与打开文件/目录动作桥接。
- [ ] 1号 + 2号 在真实对象基础上接近场入口与工作台承接，不再依赖本地假数据，重点覆盖文本选中、文件拖拽、语音入口、任务详情结果区、artifact 打开动作。

### 7.2 第二阶段：补 P1 可持续使用能力

- [ ] 巡检、任务详情、安全详情、镜子基础展示、恢复与回滚查看、Token/费用总览依次推进，重点形成真实页面入口与结果承接，而不是只停留在后端聚合数据。
- [ ] OCR / Playwright / Stronghold / Trace / Eval / HITL 按“先接入、再展示、再治理”的顺序推进，重点形成 task detail、安全详情、调试视图中的可见结果。

### 7.3 第三阶段：再做 P2 体验和智能增强

- [ ] 主动推荐、上下文感知、复制行为感知、视频总结、镜子增强、用户画像管理进入排期，重点补齐推荐冷却策略、屏幕/页面感知进入主链路、视频总结正式交付、用户画像编辑面板。

### 7.4 第四阶段：最后再做 P3 生态扩展

- [ ] 多插件、多模型、社区 Skills、感知包、云同步和团队协作在 P0/P1 稳定后再进入主排期。

---

## 8. 依赖关系与禁止顺序

### 8.1 正确顺序

- [ ] 先冻结协议、对象、状态、错误码、表结构和边界。
- [ ] 再打通主链路编排、执行、治理和交付。
- [ ] 再由前端接上真实状态和真实结果承接。
- [ ] 再做 P1 持续使用增强。
- [ ] 最后再做 P2 / P3 增强与生态。

### 8.2 不允许的顺序

- [ ] 不允许前端先写完整页面再倒逼协议和对象。
- [ ] 不允许 worker / plugin 直接输出临时 JSON 给前端长期消费。
- [ ] 不允许 1号、2号、3号 各自维护一套状态模型。
- [ ] 不允许 4号、5号 分别维护两套 schema 或两套错误码语义。
- [ ] 不允许以“先演示”为理由绕过授权、审计、恢复点和正式交付出口。

---

## 9. 里程碑与勾选标准

### 9.1 M0：统一项冻结

- [ ] 目录、命名、核心对象、协议方法、错误码、数据主模型、主链路、跨平台抽象全部对齐，且 `packages/protocol`、Go runtime、前端类型层无未登记临时字段。

### 9.2 M1：P0 主链路跑通

- [ ] 三种主入口至少各完整跑通一次：文本选中、文件拖拽、语音输入。
- [ ] 至少一种高风险动作跑通授权链，并能在安全摘要或任务详情看到授权 / 审计 / 恢复点结果。
- [ ] 至少一种正式交付真实可见，且支持 `open_action` 或 artifact 打开动作。
- [ ] 仪表盘能看到 `task / artifact / audit / recovery_point` 的最小结果，任务详情能看到 `delivery_result / security_summary / timeline` 的基础承接。
- [x] 至少有一次记忆命中或记忆摘要写入。

### 9.3 M2：P1 可持续使用

- [ ] 任务巡检、镜子、安全详情、恢复查看、Trace/Eval、Token/费用和预算降级进入可用态，并有至少一个真实页面入口或联调录像证明可用。

### 9.4 M3：P2 / P3 增强

- [ ] 推荐增强、感知增强、视频总结、多模型、多插件、社区 Skills 等进入稳定迭代，并明确哪些属于后端底座、哪些属于前端承接。

---

## 10. 每周协作执行建议

### 10.1 每周开始前

- [ ] 每个人确认本周任务是否属于自己主责边界。
- [ ] 每个人确认任务是否属于 P0 / P1 / P2 / P3 中的哪一层。
- [ ] 每个人确认是否涉及协议、状态、错误码、表结构变更。

### 10.2 每周开发中

- [ ] 有对象变更先更新真源，再更新实现。
- [ ] 有联调链路优先拉通真实链路，不做 UI 假链路替代。
- [ ] 有高风险动作必须补授权、审计、恢复点路径。

### 10.3 每周结束前

- [ ] 每个已完成任务都在本文档中勾选。
- [ ] 每个已完成任务都有对应代码位置和联调证明。
- [ ] 若新增协议 / 数据 / 状态 / 边界，必须同步回写 `docs/` 真源。

---

## 11. 当前阶段一句话结论

当前项目最重要的事不是继续铺新功能，而是 **基于已经存在的协议、后端编排、存储与桌面多窗口骨架，把 P0 主链路真正联调闭环，再有节奏地推进 P1，可持续使用之后再做 P2/P3 扩展。**
