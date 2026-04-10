# CialloClaw

CialloClaw 是一个面向 Windows 优先落地的桌面协作 Agent 工程骨架，当前仓库结构基于最新架构设计文档与开发统一规范 v10 搭建。

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
  local-service/          Go 本地 Harness Service 骨架
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

- `apps/desktop`：符合规范的多入口前端骨架，已划分 `app`、`features`、`stores`、`services`、`rpc`、`platform` 等目录
- `services/local-service`：按 rpc、orchestrator、runengine、memory、risk、storage、platform、model 等层拆分的 Go 服务骨架，并预留 task / run 映射位置
- `packages/protocol`：统一的 `Task` / `Run` 主模型、状态枚举、dot.case JSON-RPC 方法、错误码、schema 与协议样例
- `packages/ui` 与 `packages/config`：共享 UI 与共享工程规则基础目录
- `workers/*`：Playwright、OCR、media 三类 sidecar worker 骨架
- `docs/*` 与 `scripts/*`：与统一规范一致的文档层和脚本层占位结构

## 文档入口

- `docs/CialloClaw开发统一规范_修订版_v10.md`
- `docs/CialloClaw_架构设计文档_修订版_v10.md`
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

- 当前阶段重点是先冻结目录边界、协议真源和模块职责，再继续往 P0 主链路推进。
- 业务实现仍然是最小骨架，下一步应继续接通真实的 JSON-RPC handler、task / run 状态映射、SQLite 落盘和 worker 编排。
- 最高优先级约束仍以最新架构设计文档、开发统一规范 v10 与 `AGENTS.md` 为准。
