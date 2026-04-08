# 架构说明

这里用于存放基于架构设计文档沉淀出来的实现侧说明。

- 前端：Tauri 2 + React 18 + TypeScript + Vite
- 后端：通过 JSON-RPC 2.0 对外暴露的 Go harness service
- 传输：Windows 主链路优先使用 Named Pipe，本地 HTTP 仅保留调试兼容态
- Worker：独立进程 sidecar
- 存储：SQLite + WAL、workspace 文件、artifact 存储与本地 RAG

当前工程骨架已经按这些边界分层，后续功能开发应继续留在各自职责范围内推进。
