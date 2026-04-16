# AGENTS.md

本文件用于约束所有在本仓库内工作的 Agent / AI Coding Assistant / 自动化脚本代理。

目标不是让 Agent 自由发挥，而是让 Agent **先理解项目、再遵守边界、再按主链路工作**。

`AGENTS.md` 只保留跨任务共享的最小执行规则；具体设计、真源和产品细节按任务去 `docs/` 中对应文档查询。

------

## 1. 你正在参与什么项目

本项目是 **CialloClaw**，一个以桌面现场协作为核心的本地 Agent 系统。

它不是一个"默认打开聊天窗口"的普通聊天产品，而是一个围绕以下场景构建的桌面助手：

- 悬浮球近场入口
- 文本选中、文件拖拽、错误信息承接
- 意图确认后再执行
- 统一走 `task` 主对象推进
- 正式结果通过 `delivery_result / artifact / citation` 交付
- 高风险动作必须进入授权、审计、恢复链路
- 支持任务巡检、镜子记忆、安全卫士、插件与多模型扩展

在项目语义上，必须始终牢记：

- **对外主对象是 `task`**
- **后端执行兼容对象是 `run / step / event / tool_call`**
- **正式调用边界是 JSON-RPC 2.0，方法名统一为 `agent.domain.action`**
- **正式交付出口是 `delivery_result / artifact / citation`**
- **风险动作必须经过授权 / 审计 / 恢复点保护**

------

## 2. 开始工作前的最小阅读规则

`AGENTS.md` 只承载所有任务都必须遵守的最小执行规则，不再要求 Agent 每次开始工作前机械性重读整套 `/docs`。

### 默认必做

- 先读当前 `AGENTS.md`
- 先判断任务属于哪个功能域、会碰哪些正式对象、是否涉及高风险动作
- 先结合 `docs/work-priority-plan.md` 判断当前任务属于哪个优先级分组，以及是否已命中文档中的现有任务项
- 再按任务范围读取对应真源或设计文档；只读与当前任务直接相关的部分

### 仅在以下情况补读通用文档

- 第一次进入仓库，尚未建立主链路和边界认知
- 任务跨多个功能域，或对象归属、优先级、职责边界不清
- 任务会调整架构、主链路、统一规范，或你发现文档之间存在冲突

此时优先补读：

- `docs/architecture-overview.md`
- `docs/development-guidelines.md`
- `docs/work-priority-plan.md`

### 文档索引 / 路由表

| 任务类型 | 应读取的文档 |
| --- | --- |
| 新增 / 修改 JSON-RPC 方法、schema、事件、错误码、正式协议对象、正式状态枚举 | `docs/development-guidelines.md`、`docs/protocol-design.md`，必要时再核对 `packages/protocol` 真源 |
| 新增 / 修改表结构、字段、索引、数据分层 | `docs/development-guidelines.md`、`docs/data-design.md` |
| 调整模块职责、模块边界、状态机、时序、联调链路 | `docs/module-design.md` |
| 常规后端 / service 实现变更（如 orchestrator、delivery、risk、memory、run engine），但不修改协议或数据真源 | `docs/module-design.md`；若触及主链路、统一边界或跨模块约束，再补读 `docs/architecture-overview.md`、`docs/development-guidelines.md` |
| 调整系统分层、主链路、跨端边界、平台抽象 | `docs/architecture-overview.md` |
| 调整前端交互、悬浮球、仪表盘、控制面板、设置体验 | `docs/product-interaction-design.md`、`docs/dashboard-design.md`、`docs/control-panel-settings.md` |
| 判断任务属于 P0 / P1 / P2 / P3、负责人分工、任务是否已完成或需要补排期 | `docs/work-priority-plan.md` |
| 判断需求是否已有原子功能登记、该归入哪个功能域 | `docs/atomic-features.md` |

如果仓库中的设计文档进行了英文重命名或后续版本替换，以 **`docs/` 目录中的最新英文命名文件** 为准。

### 阅读要求

- 读取目标是解决当前任务需要确认的真源问题，而不是为了形式把所有文档都"打开一遍"
- 若当前任务答案仍不确定，再继续向上补读相关上位文档，不得直接凭猜测实现
- 若当前任务会新增或修改正式协议、正式状态、错误码、核心表结构或统一约束，不得只读单一领域文档，必须同时补读对应上位规范文档

### 禁止行为

- 在未定位相关真源前，擅自新增正式对象名
- 在未定位相关真源前，擅自新增 JSON-RPC 方法、状态枚举 / 错误码 / 表字段
- 直接开始大段编码
- 以"先做 demo / 先跑通再说"为理由绕开主链路
- 以"之前读过一次"为理由忽略当前任务真正相关的文档

------

## 3. 文档优先级与冲突处理

若多个信息源冲突，按以下优先级处理：

1. 仓库中最新的协议真源与 schema 真源
2. `docs/architecture-overview.md`
3. `docs/development-guidelines.md`
4. `docs/protocol-design.md`
5. `docs/data-design.md`
6. `docs/module-design.md`
7. 与当前任务直接相关的产品设计文档：`docs/product-interaction-design.md`、`docs/dashboard-design.md`、`docs/control-panel-settings.md`
8. `docs/work-priority-plan.md`
9. `docs/atomic-features.md`
10. 模块内 README / 注释 / 局部实现
11. 你自己的推测

当发现冲突时：

- 优先停下，不要继续扩散实现
- 明确写出冲突点
- 选择高优先级真源对齐
- 必要时在提交说明中注明"按上位文档对齐"

------

## 4. 你必须遵守的核心工程铁律

### 4.1 主链路优先，禁止平行造链

所有实现必须优先服务以下主链路：

```
输入承接 -> 意图确认 -> task 创建/更新 -> Go service 编排 -> 风险评估/授权 -> 正式交付 -> 仪表盘展示 -> 记忆命中或摘要回写
```

禁止：

- 先做 UI 假链路，再说后面接后端
- 先让 worker 直接产出前端结果，再补协议
- 先做页面结构，再倒逼对象模型
- 先发明一套"临时对象/临时状态"用于演示

### 4.2 对外 task-centric，对内 run-centric

必须坚持：

- 前端、仪表盘、结果页、任务详情都围绕 `task`
- 后端编排保留 `run / step / event / tool_call`
- `task_id` 与 `run_id` 必须稳定映射
- 不允许让 `run` 替代 `task` 成为对外主对象

### 4.3 正式边界只有 JSON-RPC

前端与后端唯一稳定边界是 JSON-RPC 2.0。Windows 正式传输链路优先使用 Named Pipe，HTTP / SSE 仅保留调试兼容态。

禁止：

- 前端直接调用数据库、worker、LSP、模型 SDK、Playwright、OCR
- 后端绕过协议层与前端搞"隐式字段约定"
- 一个能力同时保留两套方法名

### 4.4 正式结果统一出口

所有正式结果必须通过：

- `delivery_result`
- `artifact`
- `citation`

禁止：

- 工具调用结果直接冒充正式交付
- worker 直接给前端回临时 JSON 当正式结果
- 在 UI 里自己拼"伪交付对象"长期使用

### 4.5 风险治理不是可选项

涉及下列动作时，必须默认进入风险与治理链路：

- 改文件
- 发消息
- 执行命令
- 安装依赖
- 工作区外写入
- 访问敏感路径
- 任何可能影响系统或数据的外部动作

必须考虑：

- 风险分级
- 授权确认
- 审计记录
- 恢复点
- 可中断 / 可恢复

### 4.6 记忆、运行态、Trace 严格分层

禁止混写：

- 不要把长期记忆直接塞进 `tasks`
- 不要把 Trace/Eval 当正式业务真源
- 不要用 `todo_items` 代替 `tasks`
- 不要把前端局部状态误当正式状态

------

## 5. 命名、对象、状态的硬约束

### 5.1 命名规则

- 类型名：`PascalCase`
- 数据库 / 协议字段：`snake_case`
- JSON-RPC 方法：`agent.domain.action`
- 事件名：`dot.case`
- 工具名：`snake_case`

### 5.2 保留核心对象

以下词是系统保留词，不得擅自换名或造近义词：

- `session`
- `task`
- `task_step`
- `run`
- `step`
- `event`
- `tool_call`
- `delivery_result`
- `artifact`
- `citation`
- `approval_request`
- `authorization_record`
- `audit_record`
- `recovery_point`
- `memory_summary`
- `memory_candidate`
- `retrieval_hit`
- `trace_record`
- `eval_snapshot`
- `skill_manifest`
- `blueprint_definition`
- `prompt_template_version`

### 5.3 状态约束

- 对外状态以协议文档登记的正式枚举为准
- `task.status` 不能直接等于 `run.status`
- `todo_item.status` 不能直接映射为 `task.status`
- 悬浮球状态、气泡生命周期、输入框草稿态都只是前端局部状态

### 5.4 错误码约束

- 统一使用正式错误码体系
- 不允许仅返回字符串错误
- 前端按错误码处理，不猜文案
- Worker / plugin / sidecar 错误必须包装进统一错误结构

------

## 6. 目录与模块边界

### 6.1 顶层目录认知

工作时默认按以下目录分层理解：

- `apps/desktop`：桌面前端
- `services/local-service`：Go 本地 Harness Service
- `workers/`：独立 worker / sidecar
- `packages/protocol`：协议、类型、schema 真源
- `packages/ui` / `packages/config`：共享 UI 与工程配置；协议真源仍统一收口到 `packages/protocol`
- `docs/`：架构、协议、数据、模块、规范文档

### 6.2 前端边界

前端只负责：

- 交互承接
- 视图呈现
- 局部状态机
- ViewModel / Query / Store
- 平台能力桥接
- RPC Client

前端不负责：

- 编排真源
- 模型调用决策
- 风险审查最终裁决
- 数据真源定义

### 6.3 后端边界

后端负责：

- JSON-RPC Server
- Orchestrator
- Context Manager
- Intent / Planning
- RunEngine / Task 状态机
- Delivery
- Memory
- Risk / Audit / Recovery
- Storage
- Trace / Eval
- Tool / Plugin / Sandbox / Adapter

后端不得：

- 感知前端页面组件树
- 依赖 Tauri 细节
- 输出未登记对象给前端长期消费

------

## 7. 接任务前先做的检查清单

当你准备实现一个需求时，先回答下面 10 个问题；答不清不要开始写代码。

1. 这个需求属于哪个功能域？
2. 这是 P0、P1、P2 还是 P3？会不会阻塞 P0 主链路？
3. 它承接在哪个前端模块？
4. 它编排在哪个后端模块？
5. 它影响哪些正式对象？
6. 是否需要新增协议方法？如果需要，是否已登记到协议真源？
7. 是否需要新增表 / 字段 / 索引？如果需要，是否已回写数据文档？
8. 是否涉及风险动作、授权、审计、恢复点？
9. 最终结果通过什么 `delivery_result` 类型交付？
10. 是否会打破 `task-centric` 主链路？

若任一问题答案是"不确定"，先读文档、补设计、再编码。

------

## 8. 不同类型任务的工作方式

### 8.1 做前端时

你必须：

- 先确认真实对象字段来自协议真源
- 先区分局部状态与正式状态
- 先通过服务层 / RPC Client 调用后端
- 先设计交付承接方式，再补表现

你禁止：

- 自造正式状态
- 自造长期依赖的假数据结构
- 直接调用后端内部实现或系统能力
- 让 UI 文案替代业务状态

### 8.2 做后端时

你必须：

- 保持 `task` 与 `run` 稳定映射
- 让工具结果先进入 `tool_call / event / delivery_result` 链
- 让风险动作可授权、可审计、可恢复
- 让错误进入正式错误码体系

你禁止：

- 绕过编排器直连前端
- 绕过交付内核直接回结果
- 把记忆逻辑侵入运行态状态机
- 把平台细节写死在业务层

### 8.3 做协议 / schema 时

你必须：

- 保持方法名统一
- 保持对象字段统一
- 保持错误码统一
- 保持 stable / planned 边界清晰

你禁止：

- 先实现再补协议
- 同一能力保留两套名字
- 返回未登记字段
- 把通知当隐藏命令通道

### 8.4 做数据层时

你必须：

- 明确用途、主键、外键、索引、读写者、生命周期
- 保持运行态 / 记忆 / 前馈配置 / Trace-Eval 分层
- 保持 `task` 与 `run` 双层模型不混写

你禁止：

- 只改 SQL 不改文档
- 只加字段不写语义
- 用 trace 表代替业务真源
- 把 `todo_items` 与 `tasks` 混成一层

------

## 9. 对 Agent 的具体执行要求

### 9.1 修改任何代码前

必须先做：

- 按第 2 章路由阅读相关文档，不要机械性重读整套 `docs/`
- 至少用 `docs/work-priority-plan.md` 判断当前任务属于哪个优先级分组，以及是否命中现有任务项；若任务会影响阶段优先级、负责人分工、验收勾选或当前排期，再进一步检查文档是否一致
- 阅读相关目录现有实现
- 找出现有真源位置
- 给出最小改动方案
- 明确会改哪些文件
- 明确不会改哪些边界

### 9.2 生成代码时

遵循：

- 先最小可用，再扩展
- 优先补齐主链路，不优先做外围增强
- 尽量沿用已有目录、命名、对象、接口
- 一次提交只解决一个明确边界的问题

生成代码时必须避免：

- 大范围"顺手重构"
- 把多个问题混在一个提交里
- 创建名字相近的重复对象
- 引入文档未定义的长期结构

### 9.3 改动后自检

提交前至少自检：

- 是否遵守命名规范
- 是否遵守模块边界
- 是否影响 `task` 主链路
- 是否新增了协议 / 表结构 / 错误码
- 是否需要同步文档
- 是否覆盖失败路径、授权路径、恢复路径

------

## 10. 文档回写规则

以下改动完成后，必须同步回写 `docs/`：

- 新增或修改正式协议方法
- 新增或修改正式状态枚举
- 新增或修改错误码
- 新增或修改核心表 / 字段 / 索引
- 新增或调整主链路时序
- 新增或调整模块职责边界
- 新增或调整 P0 / P1 优先级判断

此外，针对 `docs/work-priority-plan.md`，必须额外遵守以下规则：

- 开始实现、联调、评审、提交前，至少核对当前任务所属优先级分组，以及是否命中文档中的现有任务项。
- 若当前工作会影响任务清单、负责人分工、阶段优先级或里程碑验收，则在开始实现、联调、评审、提交前进一步检查当前本地代码进度与文档是否一致。
- 若代码已真实完成文档中某个任务，必须在同一次工作中把该任务勾选为已完成，不允许代码已完成但文档长期不更新。
- 若当前优先级分组下的任务已全部完成，或某负责人的当前批次任务已全部完成，必须基于最新代码、上位文档和现有边界，继续补出下一批可执行任务，并同步更新负责人分工与优先级划分。
- 更新下一批任务时，必须延续 `task-centric` 主链路、P0 优先规则和既有模块边界，不得擅自跳级、跳域或发明未登记任务体系。

不要只改代码不改文档，也不要只改文档不改真源。

------

## 11. P0 优先级守则

当前阶段默认优先级：

- **第一优先：P0 主链路闭环**
- **第二优先：P0 底座冻结项**
- **第三优先：不破坏 P0 的 P1 补强**
- **最后才是 P2 / P3 体验增强与生态扩展**

凡是下列事情，若会阻塞 P0，一律降级处理：

- 复杂动画
- 过早插件化
- 过早多模型策略细化
- 过早做高级感知包
- 过早做漂亮但未接真实对象的页面

------

## 12. 你应该如何汇报你的工作

当你完成一次实现或准备提交时，输出建议遵循以下结构：

1. 目标是什么
2. 依据了哪些真源文档
3. 改了哪些文件
4. 没改哪些边界
5. 风险点是什么
6. 后续还需要谁继续接

如果只是草拟方案，不要伪装成"已经实现完成"。

------

## 13. 明确禁止的行为清单

你不得：

- 跳过 `docs/` 直接写代码
- 编造项目不存在的目录结构
- 编造协议方法、状态名、错误码、表名
- 把聊天式长对话当成产品主入口
- 把前端局部状态当成后端正式状态
- 让 worker 直接成为正式交付出口
- 绕开授权、审计、恢复点执行高风险动作
- 把平台实现细节直接写进业务逻辑
- 在未确认真源前大改命名
- 以"只是 demo"为理由破坏统一项

------

## 14. 建议的工作模式

推荐你每次都按以下顺序工作：

1. 先读 `AGENTS.md`
2. 再按第 2 章路由读 `docs/` 对应文档，不机械性重读整套 `docs/`
3. 再读相关目录当前实现
4. 再写"理解摘要 + 改动计划"
5. 再实施最小改动
6. 再做自检
7. 最后回写文档与说明

------

## 15. 文档维护规则

当开发者提出新需求或新功能时，Agent 必须遵循以下流程：

### 15.1 检查现有文档

- 先根据任务类型定位 `/docs` 下的对应文档，不默认通读整个 `/docs`
- 若是新需求或新功能，先检查 `docs/atomic-features.md`，确认是否已登记以及应归入哪个功能域
- 改协议、正式状态、错误码时对照 `docs/development-guidelines.md` 与 `docs/protocol-design.md`；改表结构时对照 `docs/development-guidelines.md` 与 `docs/data-design.md`；改模块边界时对照 `docs/module-design.md`
- 改前端交互时对照 `docs/product-interaction-design.md`、`docs/dashboard-design.md`、`docs/control-panel-settings.md`
- 任务跨域、涉及主链路或统一边界时，再补读 `docs/architecture-overview.md`、`docs/development-guidelines.md`、`docs/work-priority-plan.md`

### 15.2 判断冲突与补充

- 如果新需求与现有文档不存在冲突，必须在同一次工作中自动在 `/docs` 下相关英文命名文档里添加或补充内容
- 如果新需求与现有文档存在冲突，必须明确指出冲突点，并在修改前获得确认
- 补充内容时，必须遵循现有文档的结构、风格和命名规范

### 15.3 文档更新原则

- 所有文档使用英文命名，内容可包含中英文混合
- 更新文档时，保持与原文档的一致性和连贯性
- 如果文档发生重命名、替换或真源迁移，必须删除旧文档，只保留一份正式英文命名版本
- 新增章节时，延续现有章节编号规则
- 每次文档更新后，在 commit message 中说明更新了哪些文档及其原因

------

## 16. 代码注释规范

### 16.1 注释语言要求

- **项目代码必须使用英文注释**，禁止使用中文注释
- 如果在当前改动范围内发现中文注释，必须立即删除，并在同一次改动里修改成英文注释
- 每次新增代码时，必须编写详细的英文注释，确保开发者未来回看旧代码时仍能清楚理解其含义、边界和原因

### 16.2 注释内容要求

注释应清晰说明：

- **功能说明**：这段代码的作用是什么
- **参数说明**：参数的含义、类型、取值范围
- **返回值说明**：返回值的含义、类型、可能的情况
- **关键逻辑说明**：复杂逻辑的实现思路、为什么这样实现
- **边界条件说明**：特殊情况的处理方式、错误处理逻辑
- **依赖关系说明**：依赖哪些模块、被哪些模块依赖

### 16.3 注释风格规范

```go
// FunctionName does something specific.
// It takes parameter1 as input and returns result.
//
// Parameters:
//   - parameter1: Description of parameter1, type, valid range
//   - parameter2: Description of parameter2, type, valid range
//
// Returns:
//   - resultType: Description of return value
//   - error: Description of possible errors, nil if successful
//
// Example:
//   result, err := FunctionName(value)
//   if err != nil {
//       // handle error
//   }
func FunctionName(parameter1 type1, parameter2 type2) (resultType, error) {
    // Implementation comment explaining the key logic
    // Step 1: Initialize variables
    // Step 2: Process data
    // Step 3: Return result
}
```

### 16.4 自动修正机制

当 Agent 在代码审查或维护中发现中文注释时：

1. 如果该注释位于当前已修改或本次要修改的文件中，先记录其位置
2. 删除中文注释
3. 根据代码逻辑编写详细的英文注释
4. 在 commit message 中说明修正了哪些文件的中文注释
5. 如果中文注释位于当前任务范围之外，不要为了顺手清理而打散原子提交；应记录为独立后续任务或专门的注释治理提交

### 16.5 注释的长期价值

详细的英文注释能够：

- 让开发者在回看旧代码时快速理解代码意图
- 降低代码维护成本和认知负担
- 便于团队成员协作和知识传递
- 支持代码审查和质量保证

------

## 17. Git 与 Commit 规范

### 17.1 基本原则

- Git 操作必须遵循仓库规范与团队协作边界，避免为了省事把多个逻辑问题混入一次提交
- 一个 commit 只解决一个明确、独立、可回退的逻辑边界问题
- 如果一个功能由多个可独立验证的细节部分组成，必须拆成多个 commits；协议补齐、后端编排、前端承接、文档回写这类可独立验收的部分，不允许为了提 PR 压成单个提交
- 与某个实现细节强相关的文档回写、注释修正和测试补充，应与该细节一并提交或作为紧邻的原子提交
- 提交前必须检查 `git diff`、`git status` 与 `git diff --staged`，确认提交范围与意图一致

### 17.2 Commit Message 规范

- 优先使用约定式提交格式：`<type>(<scope>): <subject>`
- 常用 `type` 包括：`feat`、`fix`、`docs`、`refactor`、`test`、`chore`、`ci`
- `subject` 必须直接说明本次提交解决了什么，不允许使用空泛描述

### 17.3 历史与 PR 要求

- 准备 PR 时应保留可读的分块提交历史，让评审者能按功能细节逐步审查
- 不要把"一个功能开发完了"当成只保留一个 commit 的理由；具体功能中的每一块可独立验证细节，都应保留为对应提交
- 不要无理由把整个功能开发过程压缩为单个 commit；除非开发者明确要求，否则不要擅自 squash
- 未经明确要求，不得改写已经共享给他人的提交历史

------

## 18. 一句话总原则

> 在 CialloClaw 中，Agent 的首要职责不是"尽快写代码"，而是**先对齐真源、守住边界、服务主链路，再做最小正确实现**。
