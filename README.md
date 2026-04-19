# CialloClaw

[![code coverage](https://codecov.io/gh/1024XEngineer/CialloClaw/graph/badge.svg?branch=main)](https://codecov.io/gh/1024XEngineer/CialloClaw/tree/main)

CialloClaw 是一个面向 Windows 优先落地的桌面协作 Agent 工程仓库，当前实现、目录边界与协作约束以根目录 `AGENTS.md` 和 `/docs` 下的最新英文文档主集为准。当前主集对应架构总览 v15、开发统一规范 v19、协议设计 v5、数据设计 v6、模块设计 v6、分工优先级 v14 与原子功能表。

## 技术基线

- 桌面宿主：Tauri 2
- 前端：React 18 + TypeScript + Vite + Tailwind CSS
- 前端状态：Zustand + TanStack Query + zod
- 本地 Harness 服务：Go
- 前后端边界：JSON-RPC 2.0 + 本地受控 IPC
- 存储：SQLite + WAL，本地 RAG 使用 FTS5 + sqlite-vec
- Worker：Node.js sidecar，承载 Playwright、OCR、媒体处理能力

## 仓库结构

```text
apps/
  desktop/                桌面端主工程，包含 shell-ball、dashboard、control-panel 三个入口
services/
  local-service/          Go 本地 Harness Service，包含 RPC、编排、执行、治理、存储与工具链模块
workers/
  playwright-worker/      浏览器自动化 worker 骨架
  ocr-worker/             OCR worker 骨架
  media-worker/           媒体处理 worker 骨架
packages/
  protocol/               共享主模型、RPC 方法、错误码、schema、example
  ui/                     共享 UI 基础组件
  config/                 共享 tsconfig、lint、Prompt 约束与工程配置
docs/
  architecture/           架构说明与边界说明
  protocol/               协议文档入口
  demo/                   P0 主链路文档
  milestones/             里程碑说明
scripts/
  dev/                    开发阶段脚本说明
  build/                  构建脚本说明
  ci/                     CI 脚本说明
```

## 当前已包含内容

- `apps/desktop`：多入口桌面前端，已包含 `shell-ball`、`dashboard`、`control-panel` 及配套 `features`、`services`、`rpc`、`platform` 目录
- `services/local-service`：已包含 `rpc`、`orchestrator`、`runengine`、`context`、`intent`、`execution`、`delivery`、`audit`、`risk`、`memory`、`storage`、`platform`、`model`、`tools`、`taskinspector` 等本地 Harness 模块
- `packages/protocol`：统一维护协议模型、JSON-RPC 方法、错误码、schema 与示例
- `packages/ui` 与 `packages/config`：共享 UI 基础能力与工程配置；协议真源仍统一收口到 `packages/protocol`
- `workers/*`：Playwright、OCR、media 三类 sidecar worker
- `docs/*` 与 `scripts/*`：当前文档真源与开发 / 构建 / CI 脚本目录

## 当前 Agent Loop 状态

- 后端已具备独立 `Agent Loop / ReAct` runtime，不再只是一次性 intent 分类后生成总结
- 已支持结构化 `runs / steps / events / delivery_results` 归一化持久化
- 已支持 `agent.task.events.list` 查询任务运行时事件视图
- 已支持 `agent.task.steer` 追加 follow-up 指令，并在后续 loop round 中消费 active-run steering
- 已支持正式 `loop.*` 与 `task.steered` 通知方法，便于调试流与任务详情观察运行时进展
- 已支持 loop stop reason 暴露、planner/tool retry 与最小 active-run steering

当前仍在持续补强：更细的 retry classification、更多 runtime 直接单测、以及更完整的前端承接。

## 文档入口

### 核心技术文档

- `docs/architecture-overview.md`
- `docs/development-guidelines.md`
- `docs/protocol-design.md`
- `docs/data-design.md`
- `docs/module-design.md`
- `docs/work-priority-plan.md`
- `docs/atomic-features.md`

### 产品设计文档

- `docs/dashboard-design.md` - 仪表盘展示设计文档
- `docs/control-panel-settings.md` - 控制面板设置设计文档
- `docs/product-interaction-design.md` - 产品交互设计汇总文档

产品设计文档统一存放在 `docs/` 下并使用英文文件名；若发生重命名、替换或真源迁移，必须删除旧版本，只保留一份正式文档真源。

### Agent 工作规范

- `AGENTS.md`

## 统一协议口径

- 对外主对象统一为 `task`，后端执行兼容层保留 `run / step / event / tool_call`
- JSON-RPC 方法统一使用 `agent.domain.action`，例如 `agent.task.start`、`agent.task.confirm`、`agent.task.list`
- Notification 统一使用 `dot.case`，例如 `task.updated`、`task.steered`、`loop.started`
- 正式结果统一通过 `delivery_result / artifact / citation` 交付
- Windows 正式传输链路优先使用 Named Pipe，本地 HTTP / SSE 仅保留调试兼容态

## 协作约束

- 新需求或新功能应先检查现有 `docs/` 文档；若不冲突，必须在同一次工作中自动补充或更新相关文档。
- 项目代码注释统一使用英文；若在当前改动范围内发现中文注释，必须在同一次改动中删除并改写为英文注释，新代码也必须补充足够详细的英文注释。
- Git 提交应保持细粒度，遵循约定式提交格式；一个可独立理解和回退的变更应对应一个 commit，不要把整条功能链路压成单个提交。

## 快速开始

```bash
pnpm install
go test ./...
go run ./services/local-service/cmd/server
pnpm --dir apps/desktop dev
```

## 说明

- 当前阶段仍以 `task-centric` 主链路、正式协议边界和治理闭环为最高优先级。
- 仓库已包含本地 Harness、协议真源、桌面前端与 sidecar worker 的主干目录和实现模块；新增改动应继续按根目录 `AGENTS.md` 与 `/docs` 主集对齐。
- 若代码、注释与文档发生冲突，以仓库真源和最新英文文档主集为准，并在实现后同步回写文档。
