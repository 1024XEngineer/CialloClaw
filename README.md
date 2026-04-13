# CialloClaw

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
- `packages/ui` 与 `packages/config`：共享 UI 基础能力与工程配置真源
- `workers/*`：Playwright、OCR、media 三类 sidecar worker
- `docs/*` 与 `scripts/*`：当前文档真源与开发 / 构建 / CI 脚本目录

## 文档入口

- `docs/architecture-overview.md`
- `docs/development-guidelines.md`
- `docs/protocol-design.md`
- `docs/data-design.md`
- `docs/module-design.md`
- `docs/work-priority-plan.md`
- `docs/atomic-features.md`
- `AGENTS.md`

## 当前协议方向

- 对外主对象统一为 `task`
- 后端执行兼容层保留 `run`
- 当前稳定方法组使用 `dot.case`，例如 `agent.task.start`、`agent.task.confirm`、`agent.task.list`
- Notification 统一使用 `dot.case`，例如 `task.updated`
- Windows 主前后端传输链路优先使用 Named Pipe，本地 HTTP 仅保留调试兼容态

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
