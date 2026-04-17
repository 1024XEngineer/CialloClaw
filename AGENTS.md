# AGENTS.md

本文件用于约束所有在本仓库内工作的 Agent / AI Coding Assistant / 自动化脚本代理。

目标不是让 Agent 自由发挥，而是让 Agent **先理解项目、再遵守边界、再按主链路工作**。

根目录 `AGENTS.md` 只保留前后端都必须共享的最核心规则。
前端专项规则请继续读取 `apps/desktop/AGENTS.md`。
后端专项规则请继续读取 `services/local-service/AGENTS.md`。

------

## 1. 项目共同语义

本项目是 **CialloClaw**，一个以桌面现场协作为核心的本地 Agent 系统。

在项目语义上，必须始终牢记：

- **对外主对象是 `task`**
- **后端执行兼容对象是 `run / step / event / tool_call`**
- **正式调用边界是 JSON-RPC 2.0，方法名统一为 `agent.domain.action`**
- **正式交付出口是 `delivery_result / artifact / citation`**
- **风险动作必须经过授权 / 审计 / 恢复点保护**

------

## 2. 最小阅读规则

开始任何改动前，必须先做：

- 先读当前 `AGENTS.md`
- 再根据改动范围读取对应子目录中的 `AGENTS.md`
- 先结合 `docs/work-priority-plan.md` 判断当前任务属于哪个优先级分组，以及是否命中文档中的现有任务项
- 再按任务范围读取对应真源或设计文档；只读与当前任务直接相关的部分

如果任务跨多个功能域、涉及主链路或统一边界不清、或者文档之间有冲突，再优先补读：

- `docs/architecture-overview.md`
- `docs/development-guidelines.md`
- `docs/work-priority-plan.md`

禁止：

- 在未定位相关真源前擅自新增正式对象名
- 在未定位相关真源前擅自新增 JSON-RPC 方法、状态枚举 / 错误码 / 表字段
- 以“先做 demo / 先跑通再说”为理由绕开主链路

------

## 3. 冲突处理优先级

若多个信息源冲突，按以下优先级处理：

1. 仓库中最新的协议真源与 schema 真源
2. `docs/architecture-overview.md`
3. `docs/development-guidelines.md`
4. 任务直接相关的领域文档与真源
5. 模块内 README / 注释 / 局部实现
6. 你自己的推测

发现冲突时：

- 优先停下，不要继续扩散实现
- 明确写出冲突点
- 选择高优先级真源对齐

------

## 4. 共享工程铁律

### 4.1 主链路优先

所有实现必须优先服务以下主链路：

```
输入承接 -> 意图确认 -> task 创建/更新 -> Go service 编排 -> 风险评估/授权 -> 正式交付 -> 仪表盘展示 -> 记忆命中或摘要回写
```

禁止：

- 先做 UI 假链路，再说后面接后端
- 先发明一套临时对象 / 临时状态用于演示
- 先让 worker 直接产出前端结果，再补协议

### 4.2 task-centric 与 run-centric

必须坚持：

- 前端、仪表盘、结果页、任务详情都围绕 `task`
- 后端编排保留 `run / step / event / tool_call`
- `task_id` 与 `run_id` 必须稳定映射

### 4.3 正式边界与正式出口

必须坚持：

- 前后端唯一稳定边界是 JSON-RPC 2.0
- 所有正式结果统一通过 `delivery_result / artifact / citation`
- 下列动作默认视为高风险：文件写入、命令执行、依赖安装、工作区外写入、敏感路径访问，以及任何会改变系统 / 数据状态的动作
- 高风险动作默认进入风险、授权、审计、恢复点链路

禁止：

- 前端直接调用数据库、worker、模型 SDK 或后端内部实现
- 后端绕过协议层与前端搞隐式字段约定
- 工具调用结果直接冒充正式交付

### 4.4 记忆、运行态、Trace 严格分层

禁止混写：

- 不要把长期记忆直接塞进 `tasks`
- 不要把 Trace/Eval 当正式业务真源
- 不要用 `todo_items` 代替 `tasks`
- 不要把前端局部状态误当正式状态

### 4.5 注释优先级默认高于赶工

必须坚持：

- 每次新增代码时，都必须同时补齐英文注释
- 每次修改复杂逻辑时，都必须检查现有注释是否仍然准确；若已失真，必须在同一次改动中修正
- 评审和自检时，必须把注释完整性与准确性视为默认检查项

禁止：

- 在当前改动范围内保留中文注释或失效注释
- 以“后面再补注释”为理由留尾债

------

## 5. 共享命名与对象约束

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

------

## 6. 目录路由

- 修改 `packages/protocol`、共享 schema、JSON-RPC 方法、错误码、事件、正式字段前，必须优先补读 `docs/protocol-design.md`，必要时补读 `docs/data-design.md`
- 修改 `packages/ui`、`packages/config`、共享设计令牌、共享前端配置前，必须先补读 `docs/development-guidelines.md` 与 `docs/work-priority-plan.md`
- 修改 `workers`、工具能力边界、执行结果映射、交付物落盘约定前，必须先补读 `docs/module-design.md`、`docs/development-guidelines.md`，必要时核对 `docs/protocol-design.md` 与 `docs/data-design.md`
- 修改根目录 schema / 数据 / 协议相关文档或共享真源前，必须先定位对应真源文档，不得只依据局部实现或单侧目录规则判断
- 修改 `apps/desktop` 及其子目录前，必须补读 `apps/desktop/AGENTS.md`
- 修改 `services/local-service` 及其子目录前，必须补读 `services/local-service/AGENTS.md`
- 任务跨越前后端时，必须同时遵守根目录与对应子目录 `AGENTS.md`

------

## 7. Git 与 Commit 规范

必须坚持：

- 一个 commit 只解决一个明确、独立、可回退的逻辑边界问题
- 一个功能中的可独立验证细节必须拆成多个 commits，不要为了提 PR 压成单个提交
- 与某个实现细节强相关的文档回写、注释修正和测试补充，应与该细节一并提交或作为紧邻的原子提交
- 提交前必须检查 `git diff`、`git status` 与 `git diff --staged`

Commit message 优先使用：`<type>(<scope>): <subject>`

------

## 8. 一句话总原则

> 在 CialloClaw 中，Agent 的首要职责不是“尽快写代码”，而是**先对齐真源、守住边界、服务主链路，再做最小正确实现**。
