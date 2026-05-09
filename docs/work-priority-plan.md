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
- 无任务锚点的纯社交 / 闲聊输入不属于正式执行路径，可只返回脱离 `task` 的轻量气泡；一旦输入需要执行、确认、授权、交付或持续追踪，必须回到 `task` 主链路。
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

- [x] 根目录 `AGENTS.md`、`docs/architecture-overview.md`、`docs/development-guidelines.md`、`docs/protocol-design.md`、`docs/data-design.md`、`docs/module-design.md`、`docs/work-priority-plan.md` 对目录边界、对象名和方法名保持一致。
- [ ] `Task / TaskStep / Run / Step / Event / ToolCall / DeliveryResult / Artifact / Citation / ApprovalRequest / AuthorizationRecord / AuditRecord / RecoveryPoint` 这些共享对象在 `packages/protocol`、Go runtime、前端消费层三边字段一致。
- [ ] `agent.input.submit / agent.task.start / agent.task.confirm / agent.task.list / agent.task.detail.get / agent.task.control / agent.delivery.open` 等 stable 方法的参数、返回、错误码与文档真源一致。
- [ ] `Task.status`、`approval_request.status`、`bubble_message`、`delivery_result` 等共享合同对象，以及前端 route state 这类本地 UI 状态，已经在代码中完成分层收清，不再新增平行模型，也不再把路由态误写成需要 schema 对齐的正式对象。

#### P0-B 前端近场入口

- [ ] 悬浮球常驻、贴边、拖拽、窗口唤起和待机态在 `shell-ball` 主窗口上稳定可用。
- [ ] 单击轻量接近能打开真实轻量输入或推荐入口，不再只停留在演示态气泡。
- [x] 双击打开仪表盘并加载真实首页总览。
- [x] 语音长按、上滑锁定、下滑取消、松开提交在 `shell-ball-input` / `voice-preview` 状态机中形成完整交互闭环。
- [x] 悬停轻量输入可通过真实 RPC 发起 `agent.input.submit`，而不是只刷新本地 mock 状态。
- [x] 文本选中承接已统一进入 `agent.task.start`；文件拖拽承接先进入附件队列，并在用户手动发送后统一进入 `agent.task.start`。
- [ ] 错误信息承接的服务层封装已存在，但近场入口还没有接入统一 UI 动作。
- [ ] 意图确认气泡能承接 `agent.task.confirm` 的确认、修正、拒绝和补充说明。
- [x] 短结果在气泡中直接承接，长结果通过 `delivery_result / artifact / open_action` 正确分流到任务详情、文档或文件定位动作。

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
- [x] 至少一种 `artifact` 类型可真实落盘并在前端被打开或定位。（当前 `workspace_document / open_file / reveal_in_folder / task_detail` 已通过后端交付链与前端 `taskOutput.service.ts` 收口）
- [x] 至少一条执行链可经过工具调用并把结果回流到 `tool_call / event / delivery_result` 链。

#### P0-E 风险治理最小闭环

- [x] 风险分级能区分 `green / yellow / red`。
- [x] 至少一条高风险动作能创建 `approval_request` 并进入等待授权态。
- [x] 用户允许 / 拒绝能真实生成 `authorization_record`。
- [x] 高风险动作前能创建 `recovery_point`。
- [x] 执行完成后能生成 `audit_record`。
- [x] 前端能看到待授权摘要与最近恢复点摘要。

#### P0-F 数据与底座

- [x] SQLite + WAL 结构化运行态可用。
- [x] `tasks / runs / task_steps / events / tool_calls / delivery_results / artifacts / approval_requests / authorization_records / audit_records / recovery_points` 至少完成最小可用读写。
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
- [x] 控制面板最小设置能读取并保存真实设置快照。（控制面板已通过真实 JSON-RPC 读取、保存并回显 `general / floating_ball / memory / models` 正式设置快照；真实保存链路已有桌面 contract test 覆盖。）

#### P0-H 巡检最小闭环

- [x] 任务源接入可配置。
- [ ] 任务文件监听可触发巡检。
- [x] Markdown 任务结构识别最小可用。
- [x] 任务状态判断最小可用。
- [x] `agent.notepad.convert_to_task` 能把事项升级为正式任务。

#### P0-I 联调验收

- [ ] 文本选中入口至少完整跑通一次：`TextPattern / 真实选区承接 -> agent.task.start -> （按需进入 agent.task.confirm）-> bubble result / task detail / delivery_result`。
- [ ] 文件拖拽入口至少完整跑通一次：`附件队列入列 -> 用户补充说明 -> 手动发送 -> agent.task.start -> artifact / delivery_result`。
- [ ] 语音入口至少完整跑通一次：`长按录音 -> 录音完成主动提交 -> agent.input.submit -> task 进入主链路`。
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
- [x] 结果页 / 浏览器交付形成真实承接，至少覆盖页面摘要、页面解释、页面搜索等场景。（后端已统一 `result_page.payload.url` 与 `agent.delivery.open.resolved_payload.url`；`page_read / page_search / browser_snapshot` 默认收口到正式结果页，dashboard / shell-ball 也已共享正式打开链路与结果页承接。）
- [x] 文件交付、打开文件、定位目录、workspace document 打开动作形成稳定体验（后端 `agent.task.artifact.open` / `agent.delivery.open` 已统一，dashboard / notes / shell-ball 已共享正式 open flow 与本地打开动作）。
- [ ] 连续任务、失败任务、等待授权任务能正确分流到任务详情，而不是只停留在气泡态。

#### P1-C 安全与恢复增强

- [x] 恢复与回滚查看页可用。
- [x] Token / 费用总览可查看。
- [x] 预算降级策略进入真实执行，不只是文案提示。
- [x] 工作区边界、命令白名单、影响范围展示更完整；高风险命令优先经 Docker sandbox 受控执行，Windows shell 命令保留受控宿主路径。

#### P1-D 记忆、Trace 与审查增强

- [x] 镜子日报、历史概要、用户画像基础展示可用。
- [x] Trace / Eval 完整化，已落盘 `trace_records / eval_snapshots` 并记录输入/输出摘要、loop round、tool 调用、latency、cost 与 review 结果。
- [x] Doom Loop 检测可记录并能影响主链路，已支持重复调用签名 / 重复无进展错误命中并触发 blocked 承接。
- [x] Human-in-the-loop 升级可形成结构化对象或状态结果，已生成 escalation payload 并把任务推进到可恢复的 blocked / pending execution 状态。

#### P1-E 能力底座增强

- [x] OCR worker 真接入，已支持 `extract_text / ocr_image / ocr_pdf` 与统一错误映射。
- [x] Playwright 完整接入，已支持 `page_read / page_search / page_interact / structured_dom` 与健康检查回收。
- [x] Chromium 真实浏览器附着底座已接入，已支持 `browser_attach_current / browser_snapshot / browser_tabs_list` 的最小可用链路；其中 `browser_attach_current / browser_snapshot` 属于当前默认开放的低风险浏览器观察能力，`browser_tabs_list / browser_navigate / browser_tab_focus / browser_interact` 仍进入后端治理与审批边界，agent loop 暂不开放这些需要审批的浏览器能力。
- [x] Playwright 完整接入，已支持 `page_read / page_search / page_interact / structured_dom` 与健康检查回收；3b 已补齐执行层 attach hint 注入、attach-only snapshot 续跑保留和既有 `browser_*` intent 的主链路回归修复，但进程级 session narrowing 仍待后续扩展 attach contract 与 worker 目标选择逻辑。
- [x] Media worker 真接入，已支持 `transcode_media / normalize_recording / extract_frames`。
- [x] worker 结果已回写 `tool_call.completed` 事件通知，并携带 `source / path / url / output_path` 等关键元信息。
- [x] Stronghold 正式接入。
- [x] 插件运行态查看打通仪表盘。
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

#### 当前代码与剩余任务重整

##### 当前代码已完成的基础项

- [x] 悬浮球语音承接链已经具备完整的手势闭环：长按进入收音、上滑锁定、下滑取消、松开提交，以及录音时的 helper 提示与输入辅窗联动。
- [x] 近场正式发起链已经覆盖悬停输入、语音提交、文本选中、文件拖拽四类入口，并且会继续复用后端返回的同一条会话，不再在前端自造会话链路。
- [x] 近场快捷能力已经形成最小闭环：剪贴板、截屏、当前窗口上下文、结果自动打开、任务详情跳转、置顶气泡辅窗都已经能够串到同一套近场承接体验里。

##### 按当前代码重整后的剩余任务

- [ ] 补齐“气泡操作”这个原子功能：普通消息气泡需要直接支持置顶、删除、恢复，并且置顶后的独立辅窗和主气泡要保持同一套生命周期。
- [ ] 补齐“意图确认气泡”这个原子功能：当系统进入意图确认态时，用户需要能在气泡里直接确认、取消、修正意图，而不是只看到一段说明文本。
- [ ] 补齐“错误信息承接”这个原子功能：当前窗口或当前页面出现报错时，用户需要能直接把错误作为任务对象送入错误分析链，而不是先手动改写成普通输入。
- [ ] 补齐“近场主链可回归”这个原子功能：悬停输入、语音提交、文本选中、文件拖拽、意图确认这几条近场链路要能单独回归，不再被仪表盘协议漂移一起拖死。

#### 依赖与协作边界

- 依赖 3号 提供窗口控制、RPC、订阅、拖拽与平台集成。
- 依赖 4号 提供 `task / bubble_message / delivery_result` 的真实返回语义。
- 不负责定义协议字段，不负责定义后端状态。

#### 交付验收标准

- [x] 悬停输入、语音提交、文本选中、文件拖拽四条近场入口都已具备正式链路，不再只靠 demo 数据驱动。
- [ ] 用户在气泡 UI 上能直接完成确认、取消、置顶、删除这些近场操作，不需要再绕到别的页面或隐藏事件链。
- [ ] `corepack pnpm --dir apps/desktop test:shell-ball` 恢复通过，且不再被 dashboard/task-detail 的协议漂移阻塞编译。

### 6.2 2号：前端工作台、结果承接与可视化主责

#### 主责模块

- 仪表盘首页
- 任务状态页
- 任务巡检页
- 镜子页
- 安全卫士页
- 结果页与任务详情页
- 控制面板视图层

#### 当前代码与剩余任务重整

##### 当前代码已完成的基础项

- [x] 任务工作台已经具备最小可用的任务主视图：任务列表、任务详情、运行时事件、任务控制、补充指令这些操作都已经能在同一套任务页里完成。
- [x] 正式结果承接已经形成统一出口：产物列表、正式交付、文件打开、任务详情跳转、安全页深链都已经能走同一套结果承接路径。
- [x] 仪表盘首页、镜子页、事项页、安全页都已经从“页面骨架”进入“真实数据入口”阶段，用户可以直接在这些页面看到正式对象而不是空壳。

##### 按当前代码重整后的剩余任务

- [ ] 补齐“任务详情正式结构迁移”这个原子功能：任务页和安全页需要统一切到最新的 task detail 正式结构，不能继续按旧 detail payload 取字段。
- [ ] 补齐“安全页失败/证据展示”这个原子功能：失败摘要、引用证据、聚焦任务详情要基于当前正式对象重新装配，而不是继续沿用旧字段拼接。
- [ ] 补齐“仪表盘合同回归”这个原子功能：dashboard 的合同测试要恢复成可执行回归线，不能停留在测试文件自己都编不过的状态。
- [ ] 补齐“页面数据模式冻结”这个原子功能：要明确哪些页面继续保留开发态 mock，哪些页面只允许 RPC 不可用时的只读 fallback，不能每个页面自己定义一套口径。
- [ ] 补齐“示例数据与正式装配拆层”这个原子功能：mock 示例数据只负责演示，正式字段校验、导航状态和页面装配不能再反向依赖 mock 页面行为。

#### 依赖与协作边界

- 依赖 3号 提供 Query、Store、ViewModel、订阅刷新。
- 依赖 4号、5号 提供 `task / artifact / audit / recovery_point / memory` 的真实对象。
- 不负责平台桥接，不负责协议真源定义。

#### 交付验收标准

- [x] 双击悬浮球后可看到真实首页总览。
- [x] 任务列表、任务详情、安全摘要，以及正式结果打开链都已经有真实页面承接。
- [ ] 任务页、安全页、事项页、镜子页全部能在最新 protocol 下稳定编译，并恢复仪表盘合同测试回归线。

### 6.3 3号：前端协议、状态与平台集成主责

#### 主责模块

- Typed JSON-RPC Client
- 前端 RPC methods / subscriptions
- Store / Query / ViewModel
- 多窗口协调
- 托盘、快捷键、拖拽、文件、本地存储
- Named Pipe 连接与重连

#### 当前代码与剩余任务重整

##### 当前代码已完成的基础项

- [x] 前端正式 RPC 包装层已经成型：页面层已经可以通过统一方法入口访问 task、security、settings、dashboard、mirror、notepad、plugin 等 stable 能力，而不是自己手写 RPC 方法名。
- [x] 前端正式通知桥已经成型：任务更新、结果就绪、待授权、镜子更新、任务运行时通知都已经能通过统一订阅层回流到页面。
- [x] 桌面平台桥已经成型：Named Pipe 请求/订阅、多窗口创建与聚焦、托盘打开控制面板、shell-ball helper window/pinned bubble 管理都已经有统一平台层入口。
- [x] 后端拥有的会话复用机制已经接入前端：前端会继续沿用后端返回的会话，而不是自己生成一套独立 session。

##### 按当前代码重整后的剩余任务

- [ ] 补齐“前端正式真源收口”这个原子功能：要把 seeded demo task、页面私有 formal state 这些并行真源收掉，让任务真源只剩 query/store + protocol adapter 两层。
- [ ] 补齐“设置协议适配统一”这个原子功能：桌面本地设置、测试桩、合同校验都要统一按最新 settings snapshot 语义工作，不能再有人按旧 `data_log / 扁平 models` 读写。
- [ ] 补齐“页面刷新规则共享”这个原子功能：首页、任务页、事项页、镜子页、安全页里重复的 RPC fallback、source badge、刷新计划、invalidate 规则要收成共享策略。
- [ ] 补齐“通知回流共享桥”这个原子功能：任务更新、结果就绪、运行时通知命中后，页面刷新动作要走共享 bridge，不再让任务页、安全页、shell-ball 各自手写一份回流逻辑。

#### 依赖与协作边界

- 依赖 4号 冻结协议方法语义与返回对象。
- 依赖 5号 冻结错误码、边界策略、设置与数据归属。
- 不负责业务规划，不负责后端状态机。

#### 交付验收标准

- [x] stable 方法包装、核心通知桥、多窗口 / 托盘 / Named Pipe 基础桥都已经具备正式入口。
- [x] `task.updated / delivery.ready / approval.pending` 能稳定回写前端。
- [ ] 前端只剩 query/store + protocol adapter 两层正式真源，不再保留 seeded demo task、旧协议测试桩和页面私有 formal state 三套并行口径。
- [ ] `corepack pnpm --dir apps/desktop test:shell-ball` 与 `corepack pnpm --dir apps/desktop test:dashboard` 在最新 protocol 导出下恢复通过。

### 6.4 4号：后端 Harness 主链路与协议收口主责

#### 主责模块

- JSON-RPC Server
- Orchestrator
- Context Manager
- Intent / Planning
- RunEngine / Task 状态机
- Delivery
- 主链路查询装配

#### 当前代码与剩余任务重整

##### 当前代码已完成的基础项

- [x] “任务创建与确认主链”已经闭环：用户输入能进入正式 task，任务可以被确认、补充或继续推进，不再停留在裸 run 或演示态对象。
- [x] “任务查询与控制主链”已经闭环：任务列表、任务详情、任务控制、Notepad 升级任务、仪表盘总览都已经统一回到 task-centric 读侧，而不是前后端各维护一套对象。
- [x] “正式交付出口”已经闭环：执行结果已经统一进入 `delivery_result / artifact`，并且至少有一条真实执行链可以被前端直接承接。
- [x] “运行时观察与补充指令”已经闭环：任务事件、补充指令、`task.steered` 与 `loop.*` 通知都已经进入正式查询和前端消费链。
- [x] “屏幕感知主链”已经具备正式入口：屏幕/页面上下文能够进入 task 创建、等待授权、证据落盘和任务详情回看，不再依赖并行入口或裸平台对象。
- [x] “运行时稳定化”已经形成第一轮底座：planner retry、tool timeout retry、history compaction、doom loop 检测、stop reason 语义都已经进入正式运行时语义。

#### 依赖与协作边界

- 依赖 5号 提供 model、tools、risk、memory、storage、platform 的可用底座。
- 与 3号共同冻结协议语义，与 2号对齐查询视图数据结构。
- 不负责前端局部状态与视觉表现。

#### 交付验收标准

- [x] P0 主链路接口全部可联调。
- [x] `task_id` 与 `run_id` 映射稳定。
- [x] 至少一条真实执行链能产出正式交付结果并被前端承接。

##### 按当前代码重整后的剩余任务

- [ ] 补齐“运行态工作台细粒度承接”这个原子功能：当前任务详情和仪表盘已经能看到运行态摘要，但还需要把更多运行中信号稳定承接到工作台，而不是只展示最小摘要。
- [ ] 补齐“高风险动作跨端验收”这个原子功能：至少一条高风险动作需要持续保持“授权请求 -> 恢复点 -> 审计 -> 结果承接”全链可回归，而不是只在单次联调时跑通过。
- [ ] 补齐“屏幕感知失败语义冻结”这个原子功能：授权拒绝、采样失败、OCR 失败、会话失效、无有效识别内容都要继续沿用既有 task/event/delivery 语义，不得扩散出新的伪状态。
- [ ] 补齐“后端主链回归基线”这个原子功能：任务创建、任务详情、结果承接、运行时稳定化、视觉任务、补充指令这些主链能力要维持成一套可长期执行的回归集合。当前已补齐 issue 367 相关的 Agent Loop 能力暴露、补充指令、文件/网页只读能力与 budget downgrade 边界回归，但仍缺把任务创建、任务详情、正式结果承接和视觉任务统一编入同一套长期回归集合。

##### 屏幕感知专项

- [x] “视觉任务进入正式主链”这个原子功能已经成立：自然语言触发的屏幕分析可以进入正式 task，而不是依赖并行入口或前端私有对象。
- [x] “屏幕证据正式交付”这个原子功能已经成立：截图证据、OCR 文本、引用片段、授权与审计信息都能够通过正式对象回到任务详情。
- [x] “视觉任务状态复用”这个原子功能已经成立：屏幕感知不会再额外发明一套 task 状态，而是复用既有 `waiting_auth / processing / completed / failed` 主链语义。

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

- [x] “模型执行底座”已经成立：OpenAI Responses provider、planner/tool-calling、secret-source wiring 已经能支撑正式执行主链。
- [x] “工具回流正式化”已经成立：文件读写、命令执行、网页读取、OCR、Playwright、Media 这些执行结果都能统一进入 `tool_call / event / delivery_result`。
- [x] “结构化运行态存储”已经成立：任务、运行、事件、交付、产物、记忆、治理、Trace/Eval 都已经有 first-class 存储层，不再依赖单一快照字段。
- [x] “风险治理底座”已经成立：风险分级、等待授权、授权记录、审计记录、恢复点都已经具备最小闭环。
- [x] “Workspace / Artifact 落盘底座”已经成立：正式交付、文件落盘、屏幕证据落盘，以及 `open_file / reveal_in_folder / workspace_document` 这类打开动作所需对象都已经具备。
- [x] “屏幕/页面感知底座”已经成立：页面、屏幕、可见文本、停留、切换等信号都能进入后端，并支撑 `screen_analyze` 正式链路。
- [x] “插件与扩展资产可见层”已经成立：插件运行态、版本化 skill/blueprint/prompt 资产都已经能进入 execution / trace / eval 与仪表盘查询。
- [x] “正式设置与密钥存储”已经成立：settings 已进入结构化持久化，Stronghold 状态与错误码也已成为正式设置语义的一部分。
- [x] “session -> task -> run 关系真源”已经成立：会话、任务、运行之间的关系已经进入结构化查询链，而不是只靠 runtime 兼容快照。

##### 按当前代码重整后的剩余任务

- [ ] 继续守住“设置正式真源”这个原子功能：后端 `settings.get / settings.update` 必须继续以 `models / models.credentials` 为唯一正式语义，不能被前端兼容别名重新拉回旧结构。
- [ ] 继续守住“扩展能力边界冻结”这个原子功能：多 provider、社区 Skills、感知包继续保持 planned 边界，不在当前阶段提前扩散成稳定产品入口。
- [ ] 继续守住“读侧去快照化”这个原子功能：task list / task detail / dashboard / security 等读侧要持续优先使用正式表和正式对象，不回退成 snapshot_json 拼装。
- [ ] 继续守住“后端错误码正式化”这个原子功能：model/provider/storage/settings/stronghold 相关失败都要继续走正式错误码，不回退成局部字符串报错。

##### issue #261 屏幕感知专项（5号）

- [x] “屏幕采样到正式证据”这个原子功能已经成立：截图/关键帧可以进入 OCR、结构化观察、artifact、citation seed、审计和恢复点链路。
- [x] “录屏片段支路”这个原子功能已经成立：clip 片段可以通过 media worker 抽帧进入同一套 OCR 与 artifact 落盘链路。
- [x] “screen session 清理治理”这个原子功能已经成立：过期 session 扫描、显式 stop/expire cleanup、异常残留回收、retained artifact 分流都已落地。
- [x] “屏幕任务详情正式装配”这个原子功能已经成立：screen task detail 已经优先使用 artifact / citation / approval / authorization / audit，而不是回看 tool output 兼容字段。

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
