# CialloClaw MVP (Go Core + WPF Shell)

可运行的桌面端 AI 助手 MVP，核心形态为像素角色悬浮球，支持任务环、聊天弹出层、控制面板、主动提醒和全景仪表盘。

## 已实现能力

- 悬浮球（透明置顶、拖拽、贴边）
- 悬停任务环（径向快捷动作，默认读取 Mock 任务）
- 左键聊天弹出层（多 Agent 选择 + 会话）
- 右键系统菜单（暂停/控制面板/仪表盘/退出）
- 多 Agent 隔离（权限边界、记忆边界、路由边界）
- 任务便签巡检（Mock 任务解析结果）
- 主动提醒分级（L1/L2 轻提醒 + 频率门控）
- 插件 / Skills 兼容层（`/api/skills` + 控制面板展示）
- 全景操作仪表盘（时间线/工具调用/证据/推理摘要 + Browser/Terminal/File/Decision 流）
- Go Core SSE 事件流驱动 UI（不展示原生思维链）

## 技术架构

- `go-core`：Go HTTP 服务 + SSE 事件总线（Mock）
- `wpf-shell/CialloClaw.Shell`：WPF 前端壳，MVVM + SSE 客户端
- 流式协议：SSE
- 数据模型：`session / run / step(event) / tool_call / citation / artifact(summary)`

## 快速运行

### 1) 启动 Go Core

```powershell
cd D:\Code\GO\CialloClaw\go-core
go run .
```

默认监听：`http://127.0.0.1:18080`

### 2) 启动 WPF Shell

```powershell
cd D:\Code\GO\CialloClaw\wpf-shell\CialloClaw.Shell
dotnet run
```

### 3) 一键双开（两个终端）

```powershell
cd D:\Code\GO\CialloClaw
powershell -ExecutionPolicy Bypass -File .\scripts\start-all.ps1
```

## 开箱即用 EXE

### 1) 打包单入口程序

```powershell
cd D:\Code\GO\CialloClaw
powershell -ExecutionPolicy Bypass -File .\scripts\publish-exe.ps1
```

### 2) 直接双击运行

打包后运行：

`D:\Code\GO\CialloClaw\dist\CialloClaw\CialloClaw.exe`

说明：

- `CialloClaw.exe` 会自动探活本机 Core 服务
- 若 Core 未运行，会自动启动同目录下 `core\go-core.exe`
- 用户只需启动一个 exe 即可使用全部 MVP 功能

## 目录

- `go-core/main.go`：Mock 后端 + SSE + Chat 运行模拟
- `wpf-shell/CialloClaw.Shell/Services/ShellHost.cs`：窗口协调器、Agent 切换、SSE 分发
- `wpf-shell/CialloClaw.Shell/Views/*`：悬浮球/聊天/控制面板/仪表盘/提醒窗口
- `wpf-shell/CialloClaw.Shell/ViewModels/*`：各模块状态与命令
- `wpf-shell/CialloClaw.Shell/Assets/pixel/*`：像素角色素材
- `docs/mvp-blueprint.md`：MVP 设计蓝图与交互规范

## 说明

- 当前全部为 Mock 数据，不接真实模型与真实工具执行。
- 事件展示遵循：时间线、工具调用、证据/引用、推理摘要。
- 未展示原生思维链。
