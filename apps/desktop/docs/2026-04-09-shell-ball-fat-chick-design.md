# Shell-Ball 小胖啾形态系统设计

## 1. 目标

将 `apps/desktop` 的 `shell-ball` 从当前的科技光球占位形态，升级为基于北长尾山雀“小胖啾”视觉语言的近场承接入口，同时严格遵守仓库既有约束：

- 第一阶段只实现 `shell-ball` 前端形态系统
- 只覆盖文档冻结的 7 个展示态
- 以 mock / demo 切换作为验收手段
- 不直接接入真实 Tauri 窗口行为、真实语音、真实 RPC 主链路

目标不是制作完整桌宠，而是在现有 `shell-ball` 边界内，交付一个更贴近产品定位的吉祥物化悬浮球主体，并为第二阶段真实事件驱动接入预留稳定边界。

## 2. 背景与约束

### 2.1 仓库级约束

- 前端宿主为 `Tauri 2 + React 18 + TypeScript + Vite`
- 前后端稳定边界仍然是 `JSON-RPC 2.0`
- 对外产品主对象仍然是 `task`
- 不新增协议字段、错误码、JSON-RPC 方法、正式任务状态
- 平台相关行为不能直接写进业务视觉层

### 2.2 Shell-Ball 模块约束

依据 `apps/desktop/docs/shell-ball-development-guidelines.md`：

- 当前阶段目标是先完成 `shell-ball` 的前端形态系统
- 当前首个功能切片固定为 `shell-ball` 形态系统
- 首批必须覆盖的展示态固定为：
  - `idle`
  - `hover_input`
  - `confirming_intent`
  - `processing`
  - `waiting_auth`
  - `voice_listening`
  - `voice_locked`
- 第一阶段验收方式固定为“状态驱动 + 前端可切换演示”
- `ShellBallApp` 只保留 feature 根容器职责
- 状态源优先集中在 `apps/desktop/src/stores/shellBallStore.ts`

### 2.3 本次设计的边界选择

用户已确认采用以下边界：

- 将“小胖啾”作为 `shell-ball` 的正式视觉主体
- 采用“鸟球 + 可展开承接面板”结构
- 第一阶段先做前端演示态，不直接接入真实桌宠拖拽/贴边/穿透能力
- 采用“状态映射式”方案，而不是直接照搬完整桌宠状态机

## 3. 设计总览

### 3.1 核心思路

保留 `shell-ball` 现有的产品语义与阶段边界，将小胖啾作为唯一主体形象，并把其动作、表情、节奏和局部提示映射到文档冻结的 7 个展示态上。

换句话说：

- `shell-ball` 的正式展示态不变
- 小胖啾只是新的视觉和动效承载体
- 真实任务语义仍围绕 `task` 展开
- 第一阶段只做“状态驱动的形态演示系统”

### 3.2 视觉方向

视觉方向采用“圆润、低重心、奶白蓬松、克制科技感”的桌面吉祥物风格：

- 主体为奶白绒感球形身体
- 头顶浅灰绒毛、灰黑翅膀与长尾
- 通过眼神、歪头、扑腾节奏、声波环和局部标记表达状态
- 避免强发光能量球、霓虹电光、抽象赛博球心等风格

主体始终是同一只鸟，不通过频繁变换造型表达状态，而通过同一主体的姿态、速度和承接面板变化表达差异。

## 4. 状态语义映射

### 4.1 状态分层

本次实现明确分成两层状态：

1. `ShellBallVisualState`
   - 只负责 7 个前端展示态
   - 驱动小胖啾姿态、局部动画、面板开合与辅助视觉

2. `ShellBallDemoViewModel`
   - 只负责第一阶段承接面板里的标题、副文案、提示文本、badge、风险提示、语音提示等演示内容
   - 由 `ShellBallVisualState` 映射到 feature 内部 demo fixture
   - 不直接控制鸟球本体动画

这样可以避免将展示态、协议态和真实任务执行态混为一体。

`ShellBallDemoViewModel` 在第一阶段冻结为以下展示契约：

```ts
type ShellBallPanelMode = "hidden" | "peek" | "compact" | "full";

type ShellBallDemoViewModel = {
  badgeTone:
    | "status"
    | "intent_confirm"
    | "processing"
    | "waiting_auth";
  badgeLabel: string;
  title: string;
  subtitle: string;
  helperText: string;
  panelMode: ShellBallPanelMode;
  showRiskBlock: boolean;
  riskTitle?: string;
  riskText?: string;
  showVoiceHint: boolean;
  voiceHintText?: string;
};
```

要求：

- 第一阶段承接面板文案只能来自这个 view model
- 不允许在组件内部临时发明平行字段
- 若后续需要扩展展示内容，必须先补这个契约，再补实现

### 4.2 七态映射规则

#### `idle`

- 小胖啾缓慢呼吸，轻微上下浮动
- 保持自然眨眼和轻微尾羽摆动
- 面板默认收起，仅保留最弱存在感

#### `hover_input`

- 小胖啾轻微歪头并放大一点点
- 承接面板进入轻展开态
- 用于表达“我准备承接输入”

#### `confirming_intent`

- 小胖啾身体微微前倾，眼神更专注
- 面板完整展开，展示当前任务标题、意图和下一步说明
- 这是最明确的“正在确认你要我做什么”状态

#### `processing`

- 小胖啾进入更急促的轻扑腾节奏
- 面板维持展开，但内容收敛到处理中反馈
- 强调正在推进而不是等待输入

#### `waiting_auth`

- 小胖啾姿态收紧，翅膀与身体动作趋于克制
- 面板展开并突出风险提示、授权说明和状态 badge
- 不通过夸张抖动表达危险，而是通过谨慎与停顿表达“等待确认”

#### `voice_listening`

- 小胖啾按收音节奏起伏
- 周围出现柔和声波环或脉冲纹理
- 面板仅轻量展开，显示正在收听提示

#### `voice_locked`

- 小胖啾姿态更稳定、更集中
- 声波环更规律，表达持续锁定的录入态
- 面板强调“持续收音 / 可结束”的语义

## 5. 交互与布局

### 5.1 结构选择

采用“鸟球主视图 + 紧凑可展开承接面板”的结构，而不是保留当前完全并排的左球右卡片展示结构。

目标效果：

- 默认用户先看到小胖啾主体
- 状态进入承接阶段时，再展开承接信息
- 让 `shell-ball` 更接近桌面悬浮球，而不是一个静态说明页

### 5.2 面板展开规则

第一阶段冻结如下状态矩阵：

| 状态 | `panelMode` | 显示区块 | demo 切换器 |
| --- | --- | --- | --- |
| `idle` | `hidden` | 不显示承接面板正文，仅保留主体 | 显示 |
| `hover_input` | `peek` | `badge`、`title`、`helperText` | 显示 |
| `confirming_intent` | `full` | `badge`、`title`、`subtitle`、`helperText` | 显示 |
| `processing` | `compact` | `badge`、`title`、`subtitle`、`helperText` | 显示 |
| `waiting_auth` | `full` | `badge`、`title`、`subtitle`、`helperText`、`risk` | 显示 |
| `voice_listening` | `peek` | `badge`、`title`、`subtitle`、`helperText`、`voice hint` | 显示 |
| `voice_locked` | `compact` | `badge`、`title`、`subtitle`、`helperText`、`voice hint` | 显示 |

补充规则：

- `hidden`：不渲染正文面板，只保留鸟球主体与辅助控制
- `peek`：展示最小提示层，不出现密集信息区块
- `compact`：展示单列信息，不出现冗长说明块
- `full`：展示完整承接信息，可包含风险区块

### 5.3 演示切换器

保留第一阶段必须存在的 demo 切换器：

- 只负责切换前端展示态
- 第一阶段只读取 feature 内部 demo fixture，不读取真实任务数据
- 第一阶段固定由 `ShellBallApp` 常驻渲染，不从 panel view model 派生
- 位置和视觉上不抢主体
- 作为验收工具而不是正式产品主控件

### 5.4 Demo 数据契约

第一阶段承接面板内容统一来自 `ShellBallVisualState -> ShellBallDemoViewModel` 的静态映射，不引入真实 `taskStore` 读取。

这意味着：

- `idle` 有自己的默认说明文案
- `confirming_intent` 有自己的意图确认演示文案
- `processing` 有自己的处理中演示文案
- `waiting_auth` 有自己的授权等待演示文案
- 语音态有自己的语音提示演示文案

第一阶段不存在“视觉状态来自 demo，面板文案来自真实 task”这种混合优先级。

第二阶段如果接入真实任务数据，再新增适配层处理优先级：

- 优先使用真实映射后的 view model
- 无真实数据时回退到 demo fixture

### 5.5 冻结的第一阶段 demo fixture

第一阶段 7 态的演示内容冻结如下：

| 状态 | `badgeLabel` | `title` | `subtitle` | `helperText` | `showRiskBlock` | `showVoiceHint` |
| --- | --- | --- | --- | --- | --- | --- |
| `idle` | `待机` | `小胖啾正在桌面待命` | `轻量承接入口已就绪` | `悬停后可进入输入承接态` | `false` | `false` |
| `hover_input` | `悬停输入` | `把想法轻轻交给小胖啾` | `近场承接面板已展开` | `可继续进入意图确认或语音输入` | `false` | `false` |
| `confirming_intent` | `确认意图` | `准备整理当前请求并生成执行重点` | `请确认这是不是你现在想做的事` | `确认后将进入处理流程` | `false` | `false` |
| `processing` | `处理中` | `正在整理内容并提炼重点` | `小胖啾正在推进当前任务` | `处理完成后会返回短结果或正式交付` | `false` | `false` |
| `waiting_auth` | `等待授权` | `此操作需要进一步确认` | `检测到潜在影响范围，正在等待授权` | `确认后才会继续执行后续动作` | `true` | `false` |
| `voice_listening` | `语音收听` | `我在认真听你说` | `当前处于轻量收音状态` | `继续说话，或切换到持续收音` | `false` | `true` |
| `voice_locked` | `持续收音` | `持续收音已锁定` | `语音输入会保持开启直到结束` | `说完后可主动结束本次语音输入` | `false` | `true` |

补充规则：

- 第一阶段按上表作为默认 fixture，不允许实现时随意改写语义
- `StatusBadge` tone 必须由稳定字段驱动，不能从中文文案反推
- `badgeTone` 直接复用现有 `StatusBadge` 已支持的 tone 取值，不额外发明新的视觉语义层
- 若后续要修改 demo fixture，应先更新 spec，再更新实现

第一阶段推荐完整 fixture 结构如下：

```ts
const shellBallDemoFixtures: Record<ShellBallVisualState, ShellBallDemoViewModel> = {
  idle: {
    badgeTone: "status",
    badgeLabel: "待机",
    title: "小胖啾正在桌面待命",
    subtitle: "轻量承接入口已就绪",
    helperText: "悬停后可进入输入承接态",
    panelMode: "hidden",
    showRiskBlock: false,
    showVoiceHint: false,
  },
  hover_input: {
    badgeTone: "status",
    badgeLabel: "悬停输入",
    title: "把想法轻轻交给小胖啾",
    subtitle: "近场承接面板已展开",
    helperText: "可继续进入意图确认或语音输入",
    panelMode: "peek",
    showRiskBlock: false,
    showVoiceHint: false,
  },
  confirming_intent: {
    badgeTone: "intent_confirm",
    badgeLabel: "确认意图",
    title: "准备整理当前请求并生成执行重点",
    subtitle: "请确认这是不是你现在想做的事",
    helperText: "确认后将进入处理流程",
    panelMode: "full",
    showRiskBlock: false,
    showVoiceHint: false,
  },
  processing: {
    badgeTone: "processing",
    badgeLabel: "处理中",
    title: "正在整理内容并提炼重点",
    subtitle: "小胖啾正在推进当前任务",
    helperText: "处理完成后会返回短结果或正式交付",
    panelMode: "compact",
    showRiskBlock: false,
    showVoiceHint: false,
  },
  waiting_auth: {
    badgeTone: "waiting_auth",
    badgeLabel: "等待授权",
    title: "此操作需要进一步确认",
    subtitle: "检测到潜在影响范围，正在等待授权",
    helperText: "确认后才会继续执行后续动作",
    panelMode: "full",
    showRiskBlock: true,
    riskTitle: "潜在影响范围",
    riskText: "本次操作可能修改当前工作区内容，需要你明确允许后继续。",
    showVoiceHint: false,
  },
  voice_listening: {
    badgeTone: "status",
    badgeLabel: "语音收听",
    title: "我在认真听你说",
    subtitle: "当前处于轻量收音状态",
    helperText: "继续说话，或切换到持续收音",
    panelMode: "peek",
    showRiskBlock: false,
    showVoiceHint: true,
    voiceHintText: "正在收听，请自然说出你的请求。",
  },
  voice_locked: {
    badgeTone: "processing",
    badgeLabel: "持续收音",
    title: "持续收音已锁定",
    subtitle: "语音输入会保持开启直到结束",
    helperText: "说完后可主动结束本次语音输入",
    panelMode: "compact",
    showRiskBlock: false,
    showVoiceHint: true,
    voiceHintText: "持续收音中，结束前不会自动退出。",
  },
};
```

## 6. 组件拆分与文件边界

### 6.1 容器层

`ShellBallApp` 继续只承担容器职责：

- 读取 `shellBallStore`
- 组织小胖啾本体、承接面板、demo 切换器的布局

不再承载具体视觉造型和大量状态分支。

### 6.2 建议文件组织

建议在 `apps/desktop/src/features/shell-ball` 下拆出以下边界：

- `ShellBallApp.tsx`
  - feature 根容器
- `shellBall.types.ts`
  - 冻结展示态类型与演示视图模型类型
  - 只允许定义 presentation-specific 类型，例如 `ShellBallVisualState`、`ShellBallDemoViewModel`
  - 不允许重定义 `Task` 等协议实体；若后续需要真实任务类型，必须直接从 `@cialloclaw/protocol` 引入
- `shellBall.demo.ts`
  - `ShellBallVisualState -> ShellBallDemoViewModel` 的静态 fixture 映射
- `shellBall.motion.ts`
  - 只导出纯状态配置、动画参数和布局映射
  - 不承载 DOM 结构或 JSX
- `components/ShellBallMascot.tsx`
  - 小胖啾主体、局部结构和 motion 渲染层
  - 只消费 `shellBall.motion.ts` 提供的纯配置
- `components/ShellBallPanel.tsx`
  - 承接面板
- `components/ShellBallDemoSwitcher.tsx`
  - 前端演示切换器

如果实现过程中发现某个文件过重，可以进一步拆出 `components/parts/*`，但第一轮不预设过细层级。

### 6.3 Store 边界

`apps/desktop/src/stores/shellBallStore.ts` 应收敛为最小状态源：

- `visualState`
- `setVisualState`

是否展开面板优先由状态推导，而不是再维护一份平行布尔值；只有在后续出现确实无法通过状态推导解决的交互问题时，才考虑新增独立 UI 控制字段。

## 7. 视觉与动效原则

### 7.1 主体原则

- 小胖啾始终是同一主体
- 状态变化优先通过呼吸节奏、眼神、歪头角度、翅膀摆动、尾羽摆动、外圈辅助环表达
- 不在第一阶段引入桌宠化拖拽压缩、贴边藏匿、飞行负载联动等真实宿主行为

### 7.2 局部表现

- 眼睛：眨眼、专注、收紧
- 翅膀：待机轻摆、处理中轻扑腾、等待授权趋于克制
- 声波环：只出现在语音相关态
- 风险标记：只在 `waiting_auth` 中作为局部视觉重点

### 7.3 样式原则

- 优先使用现有 Tailwind 与项目全局 token
- 小胖啾相关强业务样式可收口在 `features/shell-ball` 私有范围内
- 不向 `packages/ui` 扩展新的通用基础组件

## 8. 与现有 task 骨架的关系

第一阶段不将 `shell-ball` 直接绑定到真实 `agent.task.start`、`taskStore` 或真实订阅流，但会在文案语义上保持 task-centric：

- `confirming_intent`、`processing`、`waiting_auth` 的演示文案与 badge 对齐 `task` 概念
- 所有演示内容都来自 feature 内静态 fixture，而不是来自真实任务读数
- 这样既能保持主链路语义，也不会突破当前“demo-only”阶段边界

这样既不突破当前阶段边界，也能保证后续接入真实主链路时不需要推翻 UI 架构。

## 9. 验证方案

### 9.1 必须满足的验收结果

- 可以稳定切换并演示全部 7 个展示态
- 每个状态至少在以下 4 个维度产生清晰差异：
  - 鸟球姿态
  - 局部动画
  - 状态标签
  - 主说明 / 辅助文案
- `ShellBallApp` 保持容器职责清晰
- `shellBallStore` 保持最小状态源
- 优先复用现有 UI 能力，不新增无必要通用组件

### 9.2 技术验证

- 通过 `pnpm --dir apps/desktop typecheck`
- 通过 `pnpm --dir apps/desktop build`
- 手工演示 7 态切换，确认布局、动画与文案一致性

当前 `apps/desktop` 未引入前端单测基础设施，因此第一阶段验证以类型检查、构建和手工演示为主；如果后续补充测试基础设施，可再将状态映射和元信息提取为可测试纯函数。

## 10. 非目标

以下内容不属于本次切片：

- 真实 Tauri 窗口拖拽、贴边、穿透、置顶
- 真实 CPU 负载事件驱动
- 真实后台任务状态事件联动
- 真实语音识别输入
- 真实文件拖拽和文本选中承接
- dashboard / control-panel 联动
- 协议、错误码、JSON-RPC 方法扩展
- 通用 UI 基础组件建设

## 11. 第二阶段接入建议

第二阶段如果要接入真实桌宠行为或真实任务流，建议新增“视觉状态映射层”，而不是直接把协议判断和平台能力塞进小胖啾主体组件中。

建议路径：

1. 新增 `task.status` / `voice_session_state` -> `ShellBallVisualState` 的映射层
2. 在容器层新增真实 view model 适配器，负责把协议实体转换成承接面板展示模型
3. 新增宿主或平台 hook，用于处理窗口 hover、拖拽、贴边、点击穿透等行为
4. 让 `ShellBallMascot` 继续只做“接收视觉状态并播放姿态”
5. 将真实事件驱动接在容器层或平台适配层，而不是接在视觉组件内部

这样可以保证第一阶段的形态系统能够平滑升级为第二阶段的真实桌宠交互，而不是被整套推翻重做。

## 12. 结论

本设计选择在不突破仓库既有架构与 `shell-ball` 第一阶段边界的前提下，把小胖啾落成 `shell-ball` 的正式主体形态系统：

- 状态语义不改
- 产品边界不改
- 主体视觉显著升级
- 容器、视觉、面板、状态源边界更清晰
- 为下一阶段真实宿主行为与真实任务流接入保留稳定扩展点

这是当前仓库约束下，最稳妥也最符合产品气质的落地方式。
