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

## 项目组成

- `apps/desktop`：桌面端，包含悬浮球、气泡、工作台和控制面板
- `services/local-service`：本地服务，负责任务编排、执行和结果回流
- `apps/website`：项目网站与说明文档
- `packages/protocol`：共享协议、方法和数据结构
- `docs/`：开发设计文档和协作真源

## 开发者入口

如果你要继续参与开发或排查问题，再看这些内容：

- 共享协作规则：`AGENTS.md`
- 当前优先级与分工：`docs/work-priority-plan.md`
- 用户说明文档：`docs/user-guide.md`
- 项目设计与协议文档：`docs/`

## 说明

- 当前仓库仍在持续迭代，桌面端、本地服务和网站会一起更新
- 如果代码和说明出现冲突，以仓库里的最新真源文档和实现为准
