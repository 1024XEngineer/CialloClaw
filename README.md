# CialloClaw

[![code coverage](https://codecov.io/gh/1024XEngineer/CialloClaw/graph/badge.svg?branch=main)](https://codecov.io/gh/1024XEngineer/CialloClaw/tree/main)

CialloClaw 是一个运行在本地电脑上的桌面 AI 助手。
它会接住你正在处理的内容，帮你整理任务、给出结果，并把过程放到桌面工作台里继续跟进。

## 先看效果

- 项目网站：https://1024xengineer.github.io/CialloClaw/
- 最新版本下载：https://github.com/1024XEngineer/CialloClaw/releases/latest

如果你只是想先看看这个项目适不适合自己，建议先打开项目网站。

## 它现在能做什么

- 接住一句话输入、语音输入、选中文本和拖入文件
- 帮你解释内容、整理重点、分析问题、生成草稿
- 在执行关键操作前先向你确认
- 把任务、结果和记录放到桌面工作台里统一查看

## 技术栈

- 桌面宿主：Tauri 2
- 桌面前端：React 18、TypeScript、Vite
- UI 与状态：Tailwind CSS、styled-components、Zustand、TanStack Query、zod
- 本地服务：Go 本地 `local-service`
- 协议边界：JSON-RPC 2.0
- 本地存储：SQLite + WAL
- Worker：Node.js sidecar，承载 Playwright、OCR、媒体处理能力

## 适合谁

- 想在本地电脑上使用桌面 AI 助手的人
- 想把“提问、执行、跟进结果”放在同一个桌面流程里的人
- 想参与 CialloClaw 开发或测试的协作者

## 快速开始

如果你想从源码启动当前先行版，按下面顺序执行：

```bash
pnpm install
pnpm dev:service
pnpm --dir apps/desktop exec tauri dev
```

启动后请保持 `pnpm dev:service` 所在终端继续运行，再在另一个终端启动桌面端。

## 使用方式

你可以先从这几个最简单的动作开始：

- 对悬浮球输入一句话，让它直接帮你处理
- 长按悬浮球说出需求
- 选中一段文字后点击悬浮球，让它解释或翻译
- 把文件拖到悬浮球附近，再补一句你想让它做什么
- 双击悬浮球，打开工作台查看任务和结果

## 仓库结构

```text
apps/
  desktop/                Tauri 桌面应用，包含悬浮球、气泡、工作台和控制面板
  website/                项目官网与对外说明页面
services/
  local-service/          Go 本地服务，负责任务编排、执行、治理和交付
workers/
  playwright-worker/      浏览器自动化 worker
  ocr-worker/             OCR worker
  media-worker/           媒体处理 worker
packages/
  protocol/               共享协议、JSON-RPC 方法、schema 与类型
  ui/                     共享 UI 组件与样式基础
  config/                 共享工程配置
docs/
  *.md                    架构、协议、数据、模块和排期真源文档
scripts/
  dev/                    本地联调脚本说明
  ci/                     CI 与检查脚本
```

## 项目组成

- `apps/desktop`：用户真正直接交互的桌面端入口
- `services/local-service`：任务主链路中枢，负责 `task`、`run`、风险治理和正式交付
- `workers/*`：补充浏览器、OCR、媒体等侧边执行能力
- `packages/protocol`：前后端共享的稳定协议边界
- `packages/ui`、`packages/config`：共享 UI 能力和工程配置
- `docs/`：当前实现与协作规则的真源文档

## 开发者入口

如果你要继续参与开发或排查问题，再看这些内容：

- 共享协作规则：`AGENTS.md`
- 当前优先级与分工：`docs/work-priority-plan.md`
- 架构总览：`docs/architecture-overview.md`
- 开发规范：`docs/development-guidelines.md`
- 项目设计与协议文档：`docs/`

## 说明

- 当前仓库仍在持续迭代，桌面端、本地服务和网站会一起更新
- 如果代码和说明出现冲突，以仓库里的最新真源文档和实现为准
