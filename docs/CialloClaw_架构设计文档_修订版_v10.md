# CialloClaw 架构设计文档（修订版 v10）

说明：本文件为仓库内归档副本，依据用户提供的 v10 架构文档整理并同步到 `docs/`。若外部原文继续演进，应以最新原文为准，并同步本副本。

---

## 1. 文档目的

本文档用于将 CialloClaw 的产品定义、交互规则、模块职责、治理要求、技术选型与协作方式，收敛为可实施的系统架构方案。

CialloClaw 的核心目标不是做聊天框中心的桌面 AI，而是做一个：

- 常驻桌面
- 低打扰承接
- 围绕任务现场协作
- 可确认执行
- 可恢复回滚

产品主形态包括：

- 悬浮球
- 轻量承接层
- 仪表盘工作台
- 后台能力系统
- 操作面板

---

## 2. 架构设计原则

### 2.1 总体原则

1. 桌面现场优先。
2. 轻量承接优先。
3. 先提示、再确认、后执行。
4. 事件驱动、可恢复。
5. 记忆与运行态分离。
6. Windows 优先、跨平台预留。
7. 抽象先于平台细节。
8. 前后端严格解耦。
9. 跨语言通信优先使用标准协议。
10. AI 受约束接入。
11. 主链路优先。
12. 模型接入标准化。

### 2.2 不做原则

- 不做终端为主入口的 Agent 壳
- 不做流程编排中心的重平台
- 不做默认静默执行高风险动作的工具
- 不把聊天窗口作为默认主入口
- 不把桌面 UI 与业务后端耦合成不可替换的一体模块

---

## 3. 总体架构设计

### 3.1 架构总览

CialloClaw 采用：

**前端架构 + JSON-RPC 协议边界 + 后端 Harness 架构**

- 前端只负责桌面交互承接与状态呈现
- 后端 Harness 只负责任务运行、能力编排、治理与数据闭环
- 两者之间唯一稳定边界为 JSON-RPC 2.0

### 3.2 前端架构

前端采用五层结构：

1. 运行环境 / 宿主层
2. 表现层
3. 应用编排层
4. 状态与服务层
5. 平台集成与协议适配层

重点不是传统页面树，而是围绕：

- 悬浮球近场交互
- 气泡生命周期
- 轻量输入
- 结果分流
- 仪表盘低频查看

### 3.3 后端 Harness 架构

后端采用六层结构：

1. 接口接入层
2. Harness 内核层
3. 能力接入层
4. 治理安全层
5. 数据存储层
6. 平台与执行适配层

---

## 4. 前后端通信边界

前后端通信边界固定为 JSON-RPC 2.0。

当前 Windows 主链路通信要求：

- 主前后端通信传输层优先采用本地 IPC
- Windows 当前实现优先使用 Named Pipe
- `localhost HTTP` / `WebSocket` 仅作为调试态或兼容态，不作为 P0 主前后端通信主路线
- 前端 UI 资源继续走 Tauri 默认应用协议

约束：

- 后端只暴露 JSON-RPC 方法、通知与订阅
- 后端不感知 Tauri / React / 路由 / 组件树细节
- 所有可持续扩展的前后端接口统一定义为 JSON-RPC method / params / result / notification

---

## 5. 分层说明

### 5.1 前端

- 宿主层：Tauri 2 Windows 宿主、官方插件、多窗口生命周期
- 表现层：悬浮球、气泡、轻量输入区、仪表盘、结果承接界面、控制面板
- 应用层：入口编排、意图确认、推荐调度、任务执行协调、结果分发
- 状态与服务层：Zustand、本地状态、查询缓存、服务封装
- 平台集成层：Typed JSON-RPC Client、Named Pipe 连接适配、托盘、快捷键、拖拽、文件、本地存储

### 5.2 后端

- 接口接入层：JSON-RPC Server、session/task 生命周期、订阅与通知管理
- Harness 内核层：任务编排、上下文采集、意图识别、任务状态机、记忆管理、结果交付、插件系统
- 能力接入层：模型调用、工具执行、Playwright、OCR、媒体处理、RAG
- 治理与安全层：风险评估、授权、审计、恢复点、预算、边界校验
- 数据存储层：SQLite、本地检索索引、workspace、artifact、Stronghold
- 平台与执行适配层：Windows 实现、文件系统抽象、系统能力抽象、执行后端适配

---

## 6. 平台与跨平台原则

当前只开发 Windows。

必须同时保留未来跨平台扩展所需抽象：

- 文件系统
- 路径
- 通知
- 快捷键
- 剪贴板
- 屏幕授权
- 执行后端适配
- 进程管理

严格约束：

- 不允许在业务代码里写死 `C:\`、`D:\`、`/Users/...`、`/home/...`
- 所有路径必须通过 `FileSystemAdapter` 统一归一化
- 所有工作区访问必须以 workspace root 为边界
- 平台逻辑不能绕开抽象层

---

## 7. 平台适配层设计

后端 Harness 内部必须引入 Platform Adapter Layer，至少抽象：

- `FileSystemAdapter`
- `PathPolicy`
- `OSCapabilityAdapter`
- `ExecutionBackendAdapter`
- `StorageAdapter`

额外要求：

- `OSCapabilityAdapter` 需要覆盖 Named Pipe 生命周期与权限管理
- 业务代码必须依赖接口，不能依赖平台特有 API 或路径细节

---

## 8. 功能架构

### 8.1 入口与轻量承接

悬浮球入口：

- 单击：轻量承接当前对象
- 双击：打开仪表盘
- 长按：语音主入口
- 悬停：轻量输入 + 主动推荐
- 文件拖拽：进入意图确认
- 文本选中：进入可操作提示态

轻量承接层职责：

- 识别任务对象
- 意图分析
- 用户确认 / 修正
- 输出短结果或状态
- 决定是否分流到文档、结果页、任务详情

### 8.2 任务状态架构

任务状态模块包括：

- 任务头部
- 步骤时间线
- 关键上下文
- 成果区
- 信任摘要
- 操作区

### 8.3 便签协作 / 巡检

底层能力包括：

- 指定 `.md` 任务文件夹监听
- Markdown 任务项识别
- 日期、优先级、状态、标签提取
- 巡检频率与变更触发
- 到期提醒、陈旧提醒
- 下一步动作建议

### 8.4 镜子记忆

镜子模块分三层：

1. 短期记忆
2. 长期记忆
3. 镜子总结

长期记忆支持本地 RAG 检索，但写入与检索必须与运行态解耦。

### 8.5 安全卫士

安全卫士负责：

- 工作区边界控制
- 风险分级
- 授权确认
- 影响范围展示
- 一键中断
- 恢复与回滚
- Token 与费用治理
- 审计日志
- Docker 沙盒执行策略

风险模型：绿 / 黄 / 红。

### 8.6 操作面板

操作面板是系统配置中心，不承接任务，不替代仪表盘。

---

## 9. 核心实现逻辑

### 9.1 主链路

P0 主链路统一为 task-centric：

用户输入或触发 → 悬浮球承接 → 返回意图或直接执行 → 气泡确认 → 前端通过 JSON-RPC 创建或更新 task → Go service 执行基础工具链 → 如遇风险则挂起等待授权 → 生成短结果或正式交付对象 → 仪表盘展示 task / artifact / audit / recovery_point → 完成一次记忆检索或记忆摘要回填。

### 9.2 高风险闭环

高风险动作流程：

风险评估 → checkpoint → 影响范围展示 → 授权确认 → Docker 执行 → 审计写入 → 失败时恢复 / 回滚。

### 9.3 记忆闭环

任务完成 → 阶段摘要 → 记忆候选 → 写入 MemorySummary → FTS5 + sqlite-vec → 后续任务检索 → 去重 / 排序 / 摘要回填。

---

## 10. 模块详细划分

### 10.1 前端

- `app/`：三个入口启动与装配
- `features/`：具体业务功能模块
- `stores/`：Zustand 交互状态
- `queries/`：TanStack Query 查询定义
- `services/`：前端服务封装
- `rpc/`：Typed JSON-RPC Client / 订阅桥接
- `platform/`：窗口、托盘、快捷键、拖拽、文件、本地存储、Named Pipe 连接适配
- `models/`：ViewModel 映射

### 10.2 后端接口接入层

- JSON-RPC Server
- task / session 创建与更新
- 实时状态推送
- 交付对象与 artifact 发布
- Windows Named Pipe 监听与接入

### 10.3 后端内核层

- orchestrator
- context
- intent
- runengine
- memory
- delivery
- plugin

### 10.4 后端能力层

- OpenAI Responses SDK
- tool registry / adapter
- Playwright sidecar
- OCR / media worker
- RAG / 记忆检索

### 10.5 治理与安全层

- risk
- audit
- checkpoint
- budget
- workspace 边界策略

---

## 11. 数据架构

数据分五层：

1. SQLite + WAL：结构化运行态
2. 本地记忆检索与 RAG 索引层
3. Workspace 文件系统
4. Artifact 外置大对象区
5. Stronghold 机密区

关键约束：

- 对外产品界面统一围绕 `task_id`
- 后端保留 `run` / `step` / `event` / `tool_call` 执行对象
- `task` 与 `run` 必须存在稳定映射关系
- `BubbleMessage` 与 `DeliveryResult` 必须分层

---

## 12. 技术选型

### 12.1 前端

- Tauri 2
- React 18
- TypeScript
- Vite
- Tailwind CSS
- Radix UI
- shadcn/ui
- Floating UI
- lucide-react
- Motion

### 12.2 前端状态与数据访问

- Zustand
- TanStack Query
- zod
- Typed JSON-RPC Client

### 12.3 后端与存储

- Go 1.26 local service / harness service
- JSON-RPC 2.0
- Windows Named Pipe 作为主前后端 IPC 方案
- SQLite + WAL
- SQLite FTS5 + sqlite-vec
- Tauri Stronghold

### 12.4 外部能力与执行

- Node.js sidecar + Playwright
- getDisplayMedia + MediaRecorder + FFmpeg + yt-dlp + Tesseract
- Manifest + 独立进程 Worker + stdio / 本地 IPC / JSON-RPC
- Docker 外部执行后端

### 12.5 LLM

- OpenAI 官方 Responses API SDK

---

## 13. 结论

CialloClaw 的正确架构方向是：

**Tauri 2 桌面宿主 + React 18 前端 + JSON-RPC 2.0 协议边界 + Go 1.26 Harness Service + OpenAI 官方 Responses API SDK + SQLite + 本地 RAG + Stronghold + 外部 worker / sidecar / plugin + 容器优先执行后端 + Windows 优先、跨平台抽象预留。**

更具体地说，CialloClaw 是：

**一个以前后端分离为基本架构、以 Tauri 2 为 Windows 桌面宿主、以 React 18 + TypeScript + Vite 为前端、以 Go 1.26 Harness Service 为主业务后端、以 JSON-RPC 2.0 与本地 IPC 为前后端通信边界、以 task-centric API 契约和 run-centric 执行内核双层模型、以 SQLite + 本地 RAG 为数据基础的轻量桌面协作 Agent。**
