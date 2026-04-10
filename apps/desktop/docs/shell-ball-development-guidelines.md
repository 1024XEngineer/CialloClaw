# Shell-Ball 前端开发约束

## 1. 文档目的

本文件用于固化 `apps/desktop` 前端当前阶段对 `shell-ball` 功能开发的共识，作为仓库级规范在前端范围内的补充执行说明。

本文件不替代仓库根规范；发生冲突时，优先级仍然遵循：

1. `docs/CialloClaw_架构设计文档_修订版_v10.md`
2. `docs/CialloClaw开发统一规范_修订版_v10.md`
3. `docs/Agent.md`
4. `packages/protocol`
5. 本文件

## 2. 适用范围

- 目录范围：`apps/desktop`
- 当前聚焦模块：`apps/desktop/src/features/shell-ball`
- 当前阶段目标：先完成 `shell-ball` 的前端形态系统，再逐步接入真实链路

## 3. 开发总原则

### 3.1 严格按规范开发

- 前端实现必须严格遵循仓库现有架构文档、开发统一规范和 `Agent.md`
- 命名、状态、协议语义优先复用仓库已冻结定义，不得自行发明平行概念
- 对外任务主对象统一使用 `task`，不得在前端自造另一套领域主对象

### 3.2 一个功能一个功能推进

- 当前只开发 `shell-ball` 的一个明确功能切片
- 在当前切片未完成前，不提前扩展到 dashboard、control-panel、artifact、delivery、审计、记忆等其他模块
- 任何后续扩展都应在当前切片完成并验收后，再开启下一切片

### 3.3 一次只执行一个小任务

- 实施时必须采用小步推进方式，一次只做一个小任务
- 当前任务未完成前，不并行推进多个前端任务
- 小任务应满足：边界清晰、文件变更有限、可独立验收、可独立回退

### 3.4 UI 库优先，必要时再补业务私有组件

- 优先复用现有 UI 能力，不优先手写新的通用基础组件
- 当前优先复用：`@cialloclaw/ui`、`lucide-react`、`motion`、`@floating-ui/react`
- `@cialloclaw/ui` 当前已有组件包括：`PanelSurface`、`StatusBadge`
- 若现成 UI 能力已足够，不得重复造轮子
- 只有在现有 UI 库无法满足且确属 `shell-ball` 专属业务形态时，才允许在 `apps/desktop/src/features/shell-ball` 内新增私有组件
- 不得借本次开发顺手扩展一套新的通用 `Button`、`Input`、`Popover`、`Tabs` 等基础组件
- 如后续确认某组件被多个前端页面稳定复用，再单独评估是否进入 `packages/ui`

### 3.5 当前视觉方向约束

- `shell-ball` 当前外形不是抽象科技光球，而是圆润、低重心、吉祥物化的小鸟球体
- 视觉表达优先依赖轮廓、材质、体积、表情和姿态，不依赖强光晕或大面积发光特效
- 当前阶段明确禁止将 `shell-ball` 做成外圈强发光、中心强炫光、霓虹能量球风格
- 后续状态变化应以同一主体形象为基础，通过姿态、外圈反馈、承接面板和局部细节变化表达，不频繁切换主体造型

## 4. 当前首个功能切片

### 4.1 功能目标

当前首个功能切片定义为：

`shell-ball` 形态系统

目标是先将悬浮球近场承接的核心 UI 形态落出来，并支持前端可切换演示。

### 4.2 本阶段验收方式

- 采用“状态驱动 + 前端可切换演示”方式验收
- 第一版先使用 mock 状态驱动，不直接强绑定真实 RPC、Named Pipe、流式订阅、语音识别或文件拖拽实现
- 验收重点是：形态是否完整、边界是否清晰、是否符合文档语义

## 5. Shell-Ball 首批形态范围

当前首批必须覆盖以下形态：

- `idle`
- `hover_input`
- `confirming_intent`
- `processing`
- `waiting_auth`
- `voice_listening`
- `voice_locked`

其中：

- `confirming_intent`、`processing`、`waiting_auth` 应优先复用仓库既有状态语义
- `voice_listening`、`voice_locked` 作为语音相关展示态，与仓库中的 `voice_session_state` 方向保持一致
- `idle`、`hover_input` 作为前端壳层展示态，不与正式任务模型混淆

## 6. 第一阶段实现边界

### 6.1 范围内

- `shell-ball` 的状态驱动形态系统
- 前端演示切换能力
- 悬浮球球体、承接面板、状态标签、辅助文案、语音视觉反馈
- 与文档语义一致的状态切换展示

### 6.2 范围外

- 真实 `agent.task.start` / `agent.task.confirm` 接入
- `subscribeTask` 实时订阅接入
- 真实语音识别输入
- 真实文件拖拽承接
- 真实文本选中承接
- dashboard 联动
- artifact / delivery / audit / memory 展示

## 7. 组件与代码组织原则

- `ShellBallApp` 只保留 feature 根容器职责，不继续膨胀成大而全条件分支组件
- 形态相关实现优先收口在 `apps/desktop/src/features/shell-ball` 内部
- 状态源优先集中在 `apps/desktop/src/stores/shellBallStore.ts`
- 静态文案、状态元信息、展示映射应集中管理，避免散落在多个组件里
- 除非确有必要，第一阶段不让 `shell-ball` 直接依赖真实 `task` 运行流

## 8. 建议的小任务顺序

按“一次只执行一个小任务”原则，建议顺序如下：

1. 冻结 `shell-ball` 形态类型
2. 收敛 `shellBallStore` 为最小状态源
3. 精简 `ShellBallApp` 为容器层
4. 实现 `idle` 形态
5. 实现 `hover_input` 形态
6. 实现 `confirming_intent` 形态
7. 实现 `processing` 形态
8. 实现 `waiting_auth` 形态
9. 实现 `voice_listening` 形态
10. 实现 `voice_locked` 形态
11. 增加前端演示切换器
12. 执行 `typecheck`、`build` 与手工验收

## 9. 验收要求

第一阶段完成时，至少满足：

- 可以在前端页面中稳定切换并演示全部 7 个形态
- 每个形态至少在以下方面存在明确差异：球体视觉、状态标识、主说明、辅助提示
- 实现保持小步提交和清晰边界
- 优先复用现有 UI 能力，未无意义造轮子
- 通过 `pnpm --dir apps/desktop typecheck`
- 通过 `pnpm --dir apps/desktop build`

## 10. 后续扩展原则

- 第二阶段再考虑将前端展示态逐步接入真实协议状态
- 接入真实链路时，优先新增适配层，不推翻第一阶段已建立的形态系统边界
- 若后续需要新增新的 `shell-ball` 交互切片，仍需先明确切片目标、边界和验收标准，再进入开发
