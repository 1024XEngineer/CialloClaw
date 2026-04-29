# CialloClaw 模块详细设计文档（v6）

## 1. 文档定位

本文档承接《设计补充说明》中原本属于“模块实现层”的内容，专门描述模块职责、接口、状态机、时序图、异常处理与联调边界。  
它不是架构总览，也不是协议真源，更不是数据表清单；它回答的是：

- 前端与后端各模块分别负责什么；
- 关键链路在模块层如何协同；
- 哪些状态属于前端局部状态，哪些属于正式业务状态；
- 哪些时序必须在联调时真实跑通；
- 模块输入、输出、依赖、失败路径和降级路径分别是什么；
- 模块之间如何通过正式对象、正式协议和正式状态衔接，而不是靠临时字段或局部约定拼接。

### 1.1 文档边界

本文档只处理“模块实现层”问题，具体包括：

- 模块定位、职责、输入、输出、依赖、异常处理；
- 关键交互链路、编排链路、治理链路和恢复链路的时序；
- 前端局部状态机与后端正式状态之间的映射边界；
- 关键模块之间的联调顺序与验收点。

以下内容不作为本文档真源：

- JSON-RPC 方法、请求响应 schema、状态枚举、错误码真源：以协议设计文档为准；
- 表结构、字段、索引、DDL、对象落表关系：以数据设计文档为准；
- 系统总体分层、平台取舍、NFR 总体定义：以架构设计文档和架构总览文档为准；
- 团队优先级和排期责任：以分工安排和优先级划分文档为准。

### 1.2 阅读顺序

建议按以下顺序阅读本文档：

1. 先看第 2 章，建立前后端模块分层认识；
2. 再看第 3 章和第 4 章，理解前后端各模块的职责和边界；
3. 再看第 5 章，理解前馈决策与反馈闭环如何跨模块工作；
4. 再看第 6 章和第 7 章，理解关键链路时序、状态机与职责落地；
5. 最后结合协议设计文档与数据设计文档，确认方法、对象和表结构真源。

---

## 2. 模块分层总览

### 2.1 前端模块总览

- 前端工程与桌面宿主
- 表现层模块
- 应用编排层模块
- 状态管理与查询层模块
- 前端服务层模块
- 平台集成与协议适配层模块

### 2.2 后端模块总览

- 本地接入层模块
- 任务编排与运行层模块
- 能力与存储层模块（能力接入与检索子层）
- 治理与交付层模块
- 能力与存储层模块（平台、执行与持久化适配子层）

### 2.3 模块间协作总原则

1. **前端只承接交互与视图，不拥有正式执行真源**  
   前端可以拥有局部状态机，但不能自行发明正式业务状态，也不能绕过协议层改变后端状态。

2. **后端 Harness 是唯一编排中枢**  
   Context Manager、Intent、Skill / Blueprint、Prompt、RunEngine、Memory、Delivery、Hooks、Review、Trace 等都必须回到 Harness 主链路，而不是由前端、worker 或插件各自分散协调。

3. **能力与存储层中的能力接入子层只提供能力，不持有主业务状态**
   模型、工具、Playwright、OCR、LSP、RAG 只提供标准输入输出，不能自持 task/run 状态机。

4. **治理与交付层不等于外围附属，而是主链路一部分**
   风险评估、授权、审查、正式交付、Trace、熔断、恢复、预算控制必须能影响主链路，不只是做日志记录。

5. **平台与执行适配子层必须抽象，不得反向污染业务层**
   文件、路径、通知、快捷键、执行后端等能力必须经抽象层暴露，业务层不能依赖具体平台实现名和平台路径。

---

## 2.4 系统总体方案与协议边界补充

本节仅作为进入具体模块设计前的总览桥接，用于说明前端、后端、协议边界和平台能力之间的协作前提；详细模块图、分层职责、具体流程图与功能链路，以下文章节为准。

### 2.4.1 总体方案

CialloClaw 采用 **“前端桌面承接层 + JSON-RPC 协议边界 + 后端任务编排与运行层”** 的总体方案：

- 前端负责桌面交互承接、状态呈现与结果展示；
- 后端负责任务运行、能力编排、治理与数据闭环；
- 两者之间唯一稳定边界为 **JSON-RPC 2.0**；
- worker / sidecar / plugin 不直接面对前端，统一经过 Go service 编排；
- 正式接口、正式对象、正式错误码与正式状态都必须以统一协议资产为真源。

### 2.4.2 协议边界约束

- 前端不得直接 import Go 服务内部实现；
- worker 不得被前端直接调用，必须经过 Go service 编排；
- 所有正式联调接口都必须登记到 `/packages/protocol/rpc`；
- 所有类型定义必须登记到 `/packages/protocol/types` 与 `/packages/protocol/schemas`；
- 所有错误必须回落到统一 `100xxxx` 错误码体系。

### 2.4.3 阅读提示

- 第 3 章负责前端模块边界、前端承接链与前端架构图；
- 第 4 章负责后端分层、后端总览图与各后端模块设计；
- 第 5 章负责系统级核心流程图与流程语义；
- 第 6 章负责按功能模块组织的具体实现、接口、状态与异常路径。

### 2.4.4 架构分层与协作阅读

本节用于统一“模块设计文档应该怎么读”的层级视角。当前系统仍按 5 层理解：

| 逻辑层 | 当前文档主要承接位置 | 关键职责 | 模块边界提醒 |
| --- | --- | --- | --- |
| 桌面入口层 | 第 3 章、6.6~6.11 | 近场承接、轻量反馈、任务工作台、控制入口 | 不直接推进主状态机，不直连模型、数据库、worker |
| 本地接入层 | 第 3.6 节、4.1 节 | JSON-RPC 收口、对象锚定、查询装配、通知回流 | 不替代编排器做业务决策 |
| 任务编排与运行层 | 第 4.2 节、5.1~5.3 节 | 任务创建/续接、上下文归一、意图建议、运行控制、会话串行 | 是唯一正式任务中枢 |
| 治理与交付层 | 第 4.4 节、5.3~5.4 节 | 风险判断、授权承接、正式交付、审计、恢复、预算治理 | 不新建平行业务任务 |
| 能力与存储层 | 第 4.3 节、4.5 节、5.5 节 | 模型/工具/插件接入、执行隔离、结构化存储、检索与机密存储 | 只提供能力与真源，不直接面向前端输出产品语义 |

阅读要点：

- 模块图中的实线主要表示主执行链，虚线或回流箭头主要表示授权、结果、状态和恢复的正式反馈链。
- 推荐、巡检、感知和镜像引用等辅助链路可以服务主链，但都不能绕过 `task` 主对象和正式协议边界直接改写业务状态。

从模块协作角度，当前主链路应始终按下面的顺序理解：

1. 桌面入口层只提交事实和动作，不预判“这是新任务还是旧任务续接”。
2. 本地接入层负责把请求绑定到 `task_id / session_id / trace_id` 等稳定锚点，并统一回流结果。
3. 任务编排与运行层负责决定创建、续接、排队、等待授权、恢复续跑和最终状态收敛。
4. 治理与交付层负责把高风险动作、正式结果、恢复结果、审计记录和记忆写入组织成正式对象链。
5. 能力与存储层只提供受控能力出口和结构化真源入口，不直接越层向前端提供产品语义。

补充约束：

- 接入层不能把 `runengine` 的内部缓存、worker 原始输出或 provider 响应直接透传给前端长期消费。
- 治理层返回的是审批对象、交付对象、恢复结果和安全摘要；编排层返回的是状态推进与下一步待处理状态，两者不能越权。
- 长期协作链与主任务链必须分层：推荐、巡检、记忆、镜像引用和 Trace / Eval 都围绕主链工作，但不能直接篡改 `task.status`。

### 2.4.5 关键架构对象与边界

模块文档虽然不直接替代架构真源，但需要对齐一组稳定对象边界，避免后续章节把“运行兼容对象”和“前端正式对象”混写：

| 对象层级 | 当前稳定对象 | 在模块文档中的阅读方式 |
| --- | --- | --- |
| 对外正式对象 | `Task`、`TaskStep`、`BubbleMessage`、`DeliveryResult`、`Artifact`、`ApprovalRequest`、`AuthorizationRecord`、`AuditRecord`、`RecoveryPoint`、`TodoItem`、`RecurringRule`、`MirrorReference` | 作为前端工作台、控制面板、安全摘要与正式交付的消费对象 |
| 后端执行兼容对象 | `Run`、`Step`、`Event`、`ToolCall`、`Citation` | 作为执行、排障、事件观察和工具回放的兼容链，不直接替代 `Task` 对外暴露 |
| 治理与长期协作对象 | `MemorySummary`、`MemoryCandidate`、`RetrievalHit`、`TraceRecord`、`EvalSnapshot` | 作为记忆、镜像、追踪评估与回放链的结构化对象 |
| 运行时协调结构 | `TaskContextSnapshot`、`Suggestion`、`runengine.TaskRecord`、持久化写入计划、恢复候选 | 仅用于编排与运行控制，不构成新的正式协议对象 |

补充边界：

- `task` 是前端与交付层唯一正式主对象，`run` 仍然只是执行兼容对象。
- `task` 与其主 `run` 及派生的 `step / event / tool_call` 必须保持稳定映射，任务详情看到的是这组对象的受控投影，而不是底层结构直接透传。
- 推荐、巡检、感知信号和镜像引用默认都不是任务主状态；它们只有在用户接受或升级条件满足时，才会重新进入正式任务入口。

### 2.4.6 从架构总览到模块设计的快速定位

如果读者先看了 [architecture-overview.md](./architecture-overview.md)，可以按下面的索引直接跳到本文件的对应细化位置：

| 架构总览关注点 | 在本文件中优先阅读的位置 | 说明 |
| --- | --- | --- |
| 第 4 章“总体架构” | `2.4.4`、`3.0`、`4.0~4.5` | 先看总览桥接，再看前端总览图；后端总览图已经拆进 `4.1~4.5` 对应章节 |
| 第 5 章“关键架构对象与边界” | `2.4.5`、`3.7`、`4.2`、`5.11` | 先看对象边界，再看功能域和交付分流 |
| 第 6 章“逻辑分层与协作边界” | `2.4.4`、`3.1~3.7`、`4.1~4.5` | 前端层和后端层的职责、输入输出、边界在这里逐层展开 |
| 第 7 章“核心链路设计” | `5.0~5.11`、`6.1~6.11` | 第 5 章看系统级主链，第 6 章看功能级时序 |

阅读建议：

- 想先理解“系统怎么分层”，从 `2.4.4~2.4.6` 进入。
- 想直接看“后端某一层怎么细化”，从 `4.1~4.5` 进入。
- 想追某条主链从入口到交付怎么跑，先看第 5 章，再跳第 6 章对应时序图。


## 3. 前端模块设计

### 3.0 前端系统总览

下面采用架构总览文档 v15 的前端系统总览图，作为本章具体模块展开前的统一视图。它强调的是“用户—桌面宿主—表现与交互层—应用编排层—状态与服务层—平台与协议层”的承接关系；后续 3.1 ~ 3.6 再分别展开每一层的职责、接口、状态边界和异常处理。

#### 3.0.1 前端系统总览图

```mermaid
flowchart TB
    U[用户]

    subgraph ENV[运行环境]
        direction TB
        TAURI[桌面宿主<br/>（Tauri 2 for Windows）]
    end

    subgraph P1[表现与交互层]
        direction TB
        FB[悬浮球]
        BUBBLE[气泡]
        INPUT[轻量输入区]
        DASH[仪表盘]
        PANEL[控制面板]
    end

    subgraph P2[应用编排层]
        direction TB
        ENTRY[交互入口编排]
        CONFIRM[意图确认流程]
        RECOMMEND[推荐调度]
        COORD[任务执行协调]
        DISPATCH[结果分发]
    end

    subgraph P3[状态与服务层]
        direction TB
        STATE[状态管理]
        QUERY[查询缓存]
        SERVICE[前端服务封装]
    end

    subgraph P4[平台与协议层]
        direction TB
        RPC[结构化协议客户端<br/>（Typed JSON-RPC Client）]
        SUB[订阅与通知适配]
        PLATFORM[窗口/托盘/快捷键/拖拽/文件/本地存储]
    end

    U --> FB
    U --> PLATFORM
    TAURI --> PLATFORM
    FB --> ENTRY
    BUBBLE --> CONFIRM
    INPUT --> CONFIRM
    ENTRY --> STATE
    CONFIRM --> STATE
    COORD --> STATE
    DISPATCH --> STATE
    STATE --> QUERY
    QUERY --> SERVICE
    SERVICE --> RPC
    RPC --> SUB
    PLATFORM --> RPC
```

#### 3.0.2 图示说明

- **运行环境**：Tauri 2 Windows 宿主负责多窗口、托盘、快捷键、通知和本地平台能力承载。
- **表现与交互层**：悬浮球、气泡、轻量输入区、仪表盘和控制面板负责用户直接可见的交互呈现。
- **应用编排层**：负责把单击、双击、长按、悬停、文本选中、文件拖拽等动作收敛为统一承接动作，并串起意图确认、推荐调度、执行协调与结果分发。
- **状态与服务层**：负责前端局部状态、查询缓存和服务封装，不承担后端正式状态真源。
- **平台与协议层**：负责 Typed JSON-RPC Client、事件订阅适配和窗口 / 托盘 / 快捷键 / 拖拽 / 文件 / 本地存储等平台桥接。


## 3.1 前端工程与桌面宿主

### 模块定位
该模块负责承载 Tauri 2 Windows 宿主、多窗口入口和应用生命周期，是所有前端交互窗口的运行基础，不直接承担业务推理与任务编排。

### 层内子模块图

```mermaid
flowchart TB
    subgraph F1[前端工程与桌面宿主]
        direction TB
        BOOT[启动装配器]
        WINDOW[窗口注册表]
        LAYOUT[布局与恢复管理器]
        TRAY[托盘 / 快捷键桥]
        LIFE[生命周期协调器]
    end

    BOOT --> WINDOW
    BOOT --> LAYOUT
    BOOT --> TRAY
    BOOT --> LIFE
    LIFE -.->|恢复 / 挂起 / 退出| WINDOW
    LAYOUT -.->|恢复布局| WINDOW
```

### 职责
- 承载 Tauri 2 Windows 宿主；
- 管理多窗口：悬浮球、仪表盘、控制面板；
- 统一前端入口分包与生命周期控制；
- 为托盘、通知、快捷键、更新提供宿主桥接。

### 核心能力
- 应用启动、唤起、最小化、恢复、退出；
- 多入口分包：`shell-ball`、`dashboard`、`control-panel`；
- 托盘、通知、快捷键、更新等宿主能力接入；
- 崩溃后窗口状态恢复与上次布局恢复。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 输出与反馈 |
| --- | --- | --- | --- |
| 启动装配器 | 应用冷启动、热恢复、自动启动 | 启动配置、窗口清单、恢复策略 | 初始化窗口、托盘和平台桥 |
| 窗口注册表 | 打开/关闭/显隐/聚焦请求 | 窗口句柄、窗口角色映射 | 悬浮球、仪表盘、控制面板窗口实例 |
| 布局与恢复管理器 | 最近布局、本地缓存、崩溃恢复 | 布局快照、默认布局决策 | 启动后的窗口位置和显示状态 |
| 托盘 / 快捷键桥 | 托盘菜单点击、快捷键命中 | 宿主动作事件 | 打开窗口、切换显示、唤起控制面板 |
| 生命周期协调器 | `resume / suspend / quit / update` | 生命周期事件、能力可用性标记 | 给状态层和平台层的宿主状态信号 |

### 输入
- Tauri 生命周期事件；
- 平台插件事件；
- 本地持久化配置；
- 自动启动和恢复策略配置。

### 输出
- 前端多窗口运行环境；
- 提供给平台集成层的窗口句柄和生命周期信号；
- 提供给状态管理层的启动上下文与宿主能力可用性。

### 依赖
- Tauri 2；
- 平台集成与协议适配层；
- 本地存储；
- 设置快照。

### 关键接口
- `app.bootstrap()`：应用冷启动入口；
- `windowManager.open(windowName)`：打开指定窗口；
- `windowManager.restoreLastLayout()`：恢复最近布局；
- `appLifecycle.onResume / onSuspend / onQuit`：生命周期桥接。

### 状态与边界
- 宿主层只负责窗口和插件生命周期，不管理业务态；
- 不得在宿主层直接构建 `task`、`delivery_result` 等对象；
- 多窗口共享业务状态必须回到状态管理层，不得在窗口进程内各自缓存真源。

### 异常处理
- 宿主启动失败：进入只显示错误页或托盘提示的降级态；
- 窗口恢复失败：回退到默认布局并记录日志；
- 平台插件不可用：降级为纯窗口模式，不阻断主链路；
- 更新器异常：提示后续手动更新，不影响主流程。

### 联调重点
- 冷启动是否能恢复最近状态；
- 多窗口是否互不污染状态；
- 托盘和主窗口是否存在重复实例；
- 最小化、隐藏、恢复、置顶等行为是否一致。

---

## 3.2 表现层模块

### 模块定位
该模块负责把状态和结果直接呈现给用户，不承担协议拼装、对象建模和业务执行决策。

### 层内子模块图

```mermaid
flowchart TB
    subgraph F2[表现层]
        direction TB
        BALL[悬浮球视图]
        BUBBLE[气泡视图]
        INPUT[轻量输入视图]
        DASH[仪表盘视图]
        PANEL[控制面板视图]
        RESULT[结果承接视图]
    end

    BALL --> BUBBLE
    BALL --> INPUT
    INPUT --> RESULT
    DASH --> RESULT
    PANEL --> DASH
```

### 职责
- 负责悬浮球、气泡、轻量输入区、仪表盘界面、结果承接界面、控制面板界面的直接显示；
- 保持近场交互低打扰、短反馈和结果可承接；
- 根据前端局部状态机渲染不同提示态。

### 核心能力
- 悬浮球控制器：拖拽、贴边、大小与透明度；
- 气泡控制器：意图判断展示、短结果展示、生命周期管理、置顶与恢复；
- 轻量输入区：一句话补充、确认/修正、附件补充、快捷动作入口；
- 仪表盘界面：任务状态、便签协作、镜子模块、安全卫士、插件面板；
- 结果承接界面：结果页、文档打开提示、文件结果提示、任务详情入口；
- 控制面板界面：设置项配置、行为开关、记忆策略、自动化规则、成本与数据治理、密钥与模型配置。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 输出与反馈 |
| --- | --- | --- | --- |
| 悬浮球视图 | 拖拽、单击、双击、长按、悬停 | 悬浮球局部状态、交互事件 | 触发近场入口编排 |
| 气泡视图 | 后端短反馈、确认请求、授权提示 | `bubbleVM`、生命周期状态 | 轻量反馈、确认入口、恢复入口 |
| 轻量输入视图 | 用户补充说明、附件、快捷动作 | 输入草稿、附件草稿、提交动作 | 进入应用编排层 |
| 仪表盘视图 | 任务列表、详情、安全摘要、插件状态 | 页面视图模型、筛选条件 | 持续任务工作台 |
| 控制面板视图 | 设置快照、保存结果、重启提示 | 表单草稿、校验错误、apply mode | 系统级配置入口 |
| 结果承接视图 | `delivery_result / artifact` 投影 | 结果入口视图模型 | 文档、文件、结果页、任务详情入口 |

### 输入
- 悬浮球状态、气泡状态、轻量输入状态；
- 任务详情、交付结果、安全摘要、镜子概览；
- 平台事件：拖拽、托盘、文件打开反馈；
- 前端服务层返回的视图模型。

### 输出
- 用户可见的近场反馈；
- 用户下一步动作入口；
- 视觉状态提示，不直接写业务主状态；
- 表现层事件：点击、双击、长按、悬停、删除、恢复、置顶、打开详情等。

### 关键接口
- `renderShellBall(viewState)`：渲染悬浮球；
- `renderBubble(bubbleVM)`：渲染气泡；
- `renderLightInput(inputVM)`：渲染轻量输入区；
- `renderDashboard(dashboardVM)`：渲染仪表盘；
- `renderControlPanel(settingsVM)`：渲染控制面板。

### 状态与边界
- 不直接调用后端；
- 不定义正式协议字段；
- 不修改 `task_status` 等正式业务状态；
- 只能消费应用编排层和状态层提供的数据；
- “隐藏 / 置顶 / 删除 / 恢复”属于表现态，不等同于业务删除和业务完成。

### 异常处理
- 数据缺失时使用占位视图；
- 交付失败时展示待确认或异常提示，而不是自造业务对象；
- 多窗口切换时保持状态只读，不自行重置任务链路；
- 渲染失败时保底显示纯文本提示，不中断后续操作。

### 联调重点
- 气泡和结果页是否严格区分“轻量承接”和“正式交付”；
- 删除/恢复/置顶等表现动作是否不会污染正式任务状态；
- 长结果分流是否只改变交付出口，不改变主任务对象；
- 安全待确认时是否能在表现层正确给出确认入口。

---

## 3.3 应用编排层模块

### 模块定位
该模块负责把用户动作编排成标准任务请求，并把后端返回结果分流到气泡、结果页、任务详情或仪表盘。

### 层内子模块图

```mermaid
flowchart TB
    subgraph F3[应用编排层]
        direction TB
        ENTRY[入口动作路由器]
        OBJECT[对象识别与承接器]
        CONFIRM[意图确认协调器]
        EXEC[任务发起协调器]
        RESULT[结果分流协调器]
        RECOMMEND[推荐调度器]
    end

    ENTRY --> OBJECT --> CONFIRM --> EXEC --> RESULT
    ENTRY --> RECOMMEND --> CONFIRM
    RESULT -.->|打开任务 / 打开文件 / 打开文档| ENTRY
```

### 职责
- 把前端输入动作编排为可提交给后端的任务请求；
- 统一承接单击、双击、长按、悬停、文本选中、文件拖拽等入口；
- 管理意图确认、推荐调度、任务发起与结果分发。

### 核心能力
- 交互入口编排：统一处理单击、双击、长按、悬停、文本选中、文件拖拽；
- 意图确认流程：对象识别后的候选意图组织、输出方式建议、修正与确认；
- 推荐调度：推荐触发条件、冷却时间、用户活跃度与当前上下文判断；
- 任务发起与执行协调：轻量任务、持续任务、授权等待、暂停与恢复；
- 结果分发：短结果、长文档、网页结果、单文件、多文件、连续任务等多出口交付。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 输出与反馈 |
| --- | --- | --- | --- |
| 入口动作路由器 | 单击、双击、长按、悬停、拖拽、选中 | 标准前端动作事件 | 路由到对象承接、推荐或窗口动作 |
| 对象识别与承接器 | 文本、文件、语音、推荐对象 | `taskObject`、对象有效性、输入摘要 | 进入意图确认或直接发起任务 |
| 意图确认协调器 | 已识别对象、后端建议、用户修正 | 确认 payload、候选输出方式 | `agent.task.confirm` 或回退待机 |
| 任务发起协调器 | start / submit / control 意图 | 标准 RPC 请求、局部 loading 状态 | 发起任务、控制任务、处理授权等待 |
| 结果分流协调器 | `delivery_result / artifact / task` 投影 | 交付出口决策、打开动作 | 气泡、结果页、文档、任务详情入口 |
| 推荐调度器 | 上下文感知信号、冷却策略 | 推荐请求、推荐展示节流 | 轻提示或升级成正式任务 |

### 输入
- 表现层的用户动作；
- 状态管理层中的当前任务对象和上下文摘要；
- 前端服务层返回的任务、推荐和设置数据。

### 输出
- 标准化的 RPC 请求；
- 对表现层的渲染指令；
- 对状态层的局部状态更新；
- 对平台层的窗口或系统动作请求。

### 关键接口
- `handleClickShellBall()`
- `handleDoubleClickShellBall()`
- `handleLongPressVoiceStart() / handleVoiceCommit() / handleVoiceCancel()`
- `handleHoverRecommendation()`
- `handleTaskObjectStart(inputObject)`
- `handleIntentConfirm(confirmPayload)`
- `dispatchResult(deliveryResult, artifacts)`

### 状态与边界
- 不直接操作 Tauri 原生 API；
- 不直接解析数据库对象；
- 不跳过前端服务层直连协议层；
- 所有正式请求均走 Typed JSON-RPC Client；
- 一次交互动作只能产出一个标准任务请求或一个前端局部状态变化，不能同时写两套状态源。

### 异常处理
- 入口判断不收敛：回退到确认气泡或澄清问题，允许用户修正；
- 推荐失败：静默降级，不阻断悬停或点击动作；
- 结果分流失败：保底回退到气泡交付或任务详情入口；
- 当前任务对象失效：撤销编排，回到可唤起或待机态；
- 双击和单击冲突：以编排层统一防抖/判定策略为准。

### 联调重点
- 单击/双击/长按手势冲突；
- 轻量交付与正式交付的分流条件；
- 推荐冷却和悬停场景不要误触发；
- 长按中断补充是否能正确进入二次编排。

---

## 3.4 状态管理与查询层模块

### 模块定位
该模块负责承接前端局部状态机、查询缓存和后端订阅回写，是前端“本地反应层”。

### 层内子模块图

```mermaid
flowchart TB
    subgraph F4[状态管理与查询层]
        direction TB
        LOCAL[局部状态仓]
        QUERY[查询缓存]
        EVENT[事件归并器]
        SELECTOR[视图选择器]
    end

    LOCAL --> SELECTOR
    QUERY --> SELECTOR
    EVENT --> LOCAL
    EVENT --> QUERY
```

### 职责
- 承接前端本地状态、查询缓存和订阅结果；
- 区分前端局部状态与正式异步数据；
- 防止表现层和应用编排层重复维护状态。

### 局部状态
- 悬浮球状态：待机、可唤起、承接中、意图确认中、处理中、等待确认、完成、异常；
- 气泡状态：数量限制、所属任务、透明化、隐藏、消散、恢复、置顶；
- 轻量输入状态：输入内容、附件、提交态、禁用态；
- 当前任务对象状态：文本、文件、语音上下文、悬停上下文等对象摘要与有效性；
- 意图确认状态：系统猜测意图、用户修正意图、候选输出方式、确认进度；
- 语音状态：收音、锁定通话、取消、打断、响应中；
- 仪表盘状态与控制面板状态：当前模块、焦点区、筛选项、未保存修改。

### 正式异步数据
- 任务列表；
- 任务详情；
- 记忆命中；
- 安全待确认项；
- 插件运行态。

### 输入
- 前端服务层请求结果；
- Notification / Subscription 推送；
- 表现层和应用编排层局部状态变更。

### 输出
- 给表现层的渲染数据；
- 给应用编排层的上下文与冷却信息；
- 给平台层的窗口和通知刷新依据；
- 给查询层的失效与重新拉取信号。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 输出与反馈 |
| --- | --- | --- | --- |
| 局部状态仓 | 悬浮球、气泡、输入、语音、面板动作 | 前端局部状态机快照 | 直接驱动表现层 |
| 查询缓存 | RPC 查询结果、列表分页、详情数据 | 缓存记录、失效标记 | 任务列表、详情和设置视图 |
| 事件归并器 | `task.updated / delivery.ready / approval.pending / loop.*` | 事件去重、按 `task_id` 合并后的更新 | 回写局部状态和查询缓存 |
| 视图选择器 | 局部状态 + 正式异步数据 | 表现层可消费的 VM | 表现层、应用编排层共享视图数据 |

### 关键接口
- `shellBallStore`
- `bubbleStore`
- `inputStore`
- `taskObjectStore`
- `intentConfirmStore`
- `voiceStore`
- `dashboardStore`
- `queryCache.syncTask(task)`
- `eventReducer.apply(notification)`

### 约束
- 局部状态不可直接映射为正式协议状态；
- 对后端推送的 `task.updated` 等事件必须以 `task_id` 为锚点回写；
- 不允许在 store 中自造与协议冲突的字段；
- 局部状态机必须能在页面刷新或窗口切换后恢复到可解释状态；
- 查询缓存不应承担唯一真源角色。

### 异常处理
- 推送重复到达：按 `task_id + updated_at` 或事件顺序去重；
- 本地缓存失效：回退到重新拉取；
- 异步数据与局部状态冲突：以正式业务对象为准，局部状态做弱回滚；
- 多窗口竞争刷新：采用单源 store 或事件总线顺序写入。

### 联调重点
- 前端状态图和后端正式状态是否严格分层；
- 订阅风暴时是否能稳态更新；
- 多窗口共享状态是否一致；
- 前端任务状态图是否能映射到真实任务对象的变化，而不是自成体系。

---

## 3.5 前端服务层模块

### 模块定位
该模块负责把应用编排层的业务意图翻译成具体服务调用，是“前端业务能力封装层”。

### 层内子模块图

```mermaid
flowchart TB
    subgraph F5[前端服务层]
        direction TB
        TASK[任务服务]
        CTX[上下文服务]
        RECO[推荐服务]
        FILE[文件服务]
        MEMORY[记忆服务]
        SAFE[安全服务]
        SETTINGS[设置服务]
    end

    TASK --> SAFE
    TASK --> FILE
    TASK --> MEMORY
    CTX --> TASK
    RECO --> TASK
    SETTINGS --> TASK
```

### 核心服务
- 上下文服务：获取当前任务现场上下文、悬停/选中/当前界面相关输入；
- 任务服务：发起任务、查询任务状态、获取任务步骤、历史任务与任务详情；
- 推荐服务：推荐内容、推荐问题、候选动作请求；
- 语音服务：长按语音、锁定通话、语音结果提交与回传；
- 文件服务：文件解析、附件处理、结果文件查询、工作区文件打开；
- 记忆服务：镜子摘要、用户画像、近期记忆命中读取；
- 安全服务：待确认操作、风险等级、审计记录、恢复点与授权提交；
- 设置服务：设置读取、保存、校验与默认值回填。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 输出与反馈 |
| --- | --- | --- | --- |
| 上下文服务 | 近场对象、页面、语音、拖拽输入 | 结构化上下文请求体 | 给任务服务和推荐服务使用 |
| 任务服务 | start / submit / confirm / control / query | 标准 RPC 参数与响应映射 | 正式任务对象和任务详情 |
| 推荐服务 | 感知信号、冷却规则、用户反馈 | 推荐请求、反馈请求 | 推荐列表和推荐反馈结果 |
| 文件服务 | 拖拽文件、artifact 打开、结果文件定位 | 文件元信息、打开动作请求 | 文件摘要、文档入口、打开结果 |
| 记忆服务 | 镜子概览、近期命中、用户画像 | 记忆查询请求 | 记忆摘要、检索命中 |
| 安全服务 | 待授权项、授权响应、恢复点查询 | 安全请求体、安全视图模型 | 授权结果、安全摘要 |
| 设置服务 | 设置读取、保存、草稿校验 | 设置 patch、校验结果、apply mode | 设置快照、保存结果、重启提示 |

### 输入
- 应用编排层的结构化需求；
- 协议客户端、平台集成层；本地存储仅用于前端草稿、窗口布局和面板状态缓存。

### 输出
- 标准业务对象；
- 错误码和弱提醒；
- 对状态层的可消费结果；
- 可供表现层消费的视图模型。

### 关键接口
- `contextService.captureCurrentContext()`
- `taskService.startTask() / confirmTask() / getTaskDetail() / controlTask()`
- `recommendationService.getRecommendations()`
- `voiceService.submitVoiceInput()`
- `fileService.parseDroppedFile() / openArtifact()`
- `memoryService.getMirrorOverview()`
- `securityService.getPendingApprovals() / respondApproval()`
- `settingsService.getSnapshot() / updateSettings()`

### 边界
- 不直接访问数据库；
- 不直接读取 Go 内部结构体；
- 正式设置快照只能通过 `agent.settings.get / agent.settings.update` 读取和更新；
- 只能通过协议和平台适配层访问系统能力；
- 返回值必须对齐正式对象模型，不得额外夹带隐式字段给表现层做依赖。

### 异常处理
- RPC 超时：返回可重试错误，不直接吞掉；
- 文件读取失败：保留对象上下文但降级为无文件摘要；
- 设置保存失败：不覆盖前端草稿态；
- 安全授权失败：必须把待确认对象留在前端可见区；
- 语音提交失败：保留收音摘要并允许用户重试或改文本输入。

### 联调重点
- 不同服务返回对象是否完全对齐协议文档；
- 服务层是否屏蔽了平台实现细节；
- 推荐、记忆、安全等服务是否能够失败降级但不打断主链路；
- 文件、语音、上下文和任务服务是否能拼成一条标准任务输入链。

---

## 3.6 平台集成与协议适配层模块

### 模块定位
该模块负责把前端世界与桌面平台能力、协议传输层连接起来，是“前端最底层的系统桥接层”。

### 层内子模块图

```mermaid
flowchart TB
    subgraph F6[平台集成与协议适配层]
        direction TB
        RPC[结构化协议客户端<br/>（Typed JSON-RPC Client）]
        PIPE[命名管道与调试传输<br/>（Named Pipe）]
        SUB[通知与订阅适配]
        WIN[窗口桥]
        TRAY[托盘 / 快捷键桥]
        FILE[文件 / 系统动作桥]
        CACHE[本地存储桥]
    end

    RPC --> PIPE
    RPC --> SUB
    WIN --> RPC
    TRAY --> WIN
    FILE --> RPC
    CACHE --> WIN
```

### 核心能力
- Typed JSON-RPC Client：统一 method、params、result、错误模型和订阅注册；
- Windows Named Pipe 连接适配：负责主前后端本地 IPC 建链、重连、权限与错误处理；
- 调试态兼容传输：当前仍保留本地 HTTP / SSE 调试链路，但对象语义与通知语义必须与 Named Pipe 正式链路保持一致；
- 订阅与通知适配：`task.updated`、`delivery.ready`、`approval.pending`、`plugin.updated` 等事件桥接；
- 窗口集成：悬浮球窗口、仪表盘窗口、控制面板窗口的打开、关闭、显隐、聚焦、置顶；
- 托盘集成：托盘图标、托盘菜单、托盘级快捷入口；
- 快捷键集成：全局快捷键注册、释放与冲突处理；
- 拖拽集成：桌面文件拖入、原生 DragEvent 桥接、应用内拖拽协同；
- 文件系统集成：打开文件、打开文件夹、高亮结果文件、读取本地文件元信息；
- 本地存储集成：前端草稿缓存、偏好镜像、面板状态记忆；正式设置真源仍通过 `agent.settings.*` 维护；
- 外部能力集成：浏览器打开、剪贴板桥接和其他 Tauri 插件统一接入。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 输出与反馈 |
| --- | --- | --- | --- |
| Typed JSON-RPC Client | 服务层标准请求 | method/params/result/error 统一模型 | 标准协议响应 |
| Named Pipe / 调试传输 | 本地 IPC 建链、断链、重连 | 连接状态、传输错误、重试状态 | 稳定的 RPC 传输通道 |
| 通知与订阅适配 | `task.updated / delivery.ready / approval.pending / plugin.updated` | 订阅注册、通知事件对象 | 回写状态层和查询缓存 |
| 窗口桥 | 打开/关闭/显隐/聚焦请求 | 窗口动作请求、窗口句柄 | 多窗口协同结果 |
| 托盘 / 快捷键桥 | 托盘菜单、全局快捷键 | 宿主输入事件 | 唤起窗口和触发入口动作 |
| 文件 / 系统动作桥 | 打开文件、定位文件夹、浏览器打开 | 平台动作请求 | 文件、目录、浏览器等宿主动作结果 |
| 本地存储桥 | 草稿、布局、面板状态缓存 | 本地缓存键值 | 前端草稿和恢复信息 |

### 输入
- 上层请求；
- Tauri 插件回调；
- Named Pipe / IPC 消息。

### 输出
- 标准协议调用结果；
- 事件流；
- 文件、窗口、通知等系统动作结果。

### 关键接口
- `rpcClient.call(method, params)`
- `rpcClient.subscribe(eventName, handler)`
- `pipeTransport.connect() / reconnect()`
- `windowBridge.open(name, options)`
- `trayBridge.registerMenu()`
- `shortcutBridge.register(shortcut)`
- `fileBridge.open(path) / reveal(path)`

### 边界
- 不持有业务真源；
- 不解释 task/run 语义；
- 只负责传输、系统能力和平台动作适配。

### 异常处理
- IPC 断开：自动重连并向状态层发出降级信号；
- 插件异常：记录并切换为功能关闭态；
- 打开文件/浏览器失败：回退为仅展示路径或交付结果文本；
- 权限不足：返回正式错误并触发安全提示。

### 联调重点
- Named Pipe 建链、断链、重连的体验；
- 多窗口与托盘协同；
- 通知事件和查询请求之间是否顺序一致；
- 本地打开文件、显示文件夹、高亮产物是否符合桌面体验。

---

## 3.7 产品功能域与模块承接映射

本节从产品承接视角补充模块实现层设计。它不替代后续模块分层，而是回答系统到底面向哪些用户动作与产品域提供能力，以及这些能力在前后端模块中分别由谁承接。

### 3.7.1 入口与轻量承接域

系统默认以悬浮球为近场入口，以气泡和轻量输入区作为任务承接层，而不是以聊天页作为主入口。该功能域负责把语音、悬停输入、文本选中、文件拖拽和推荐点击统一转为任务请求，并在当前现场完成对象识别、意图确认、短结果返回和下一步分流。

核心入口包括：

- 左键单击：轻量接近或承接当前对象
- 左键双击：打开仪表盘
- 左键长按：语音主入口，上滑锁定、下滑取消
- 鼠标悬停：显示轻量输入与主动推荐
- 文件拖拽：先进入附件队列；带明确说明的新任务可直接进入执行链路，裸文件或补充证据按确认 / 等待输入骨架承接
- 文本选中：进入可操作提示态，再进入意图确认

对应模块分工：
- 前端表现层负责入口可见形态；
- 应用编排层负责统一动作归一化；
- 状态管理层负责轻承接局部状态；
- 后端本地接入层与任务编排与运行层负责把对象升级为正式 task 请求。

### 3.7.2 任务状态与持续追踪域

该功能域负责承接“已经被 Agent 接手并正在推进”的工作，使用户能够在仪表盘中查看任务头部、步骤时间线、关键上下文、成果区、信任摘要与操作区。任务状态域面向的是用户可见进度，而不是内核态 `run / step` 的实现细节。

其核心结构包括：

- 任务头部：名称、来源、状态、开始时间、更新时间
- 步骤时间线：已完成、进行中、未开始
- 关键上下文：资料、记忆摘要、规则约束
- 成果区：草稿、文件、网页、模板、清单
- 信任摘要：风险状态、待授权数、恢复点、边界触发
- 操作区：暂停、继续、取消、修改、重启、查看安全详情

对应模块分工：
- 前端仪表盘与结果承接界面负责视图组织；
- 状态管理层负责 `task` 与局部视图态映射；
- 后端运行控制器、治理与交付层负责真实状态推进。

### 3.7.3 便签巡检与事项转任务域

该功能域面向未来安排和长期待办，不直接等同于执行中任务。它负责监听任务文件夹、解析 Markdown 任务项、识别日期与规则、做巡检提醒，并在需要时把事项升级为正式 `task`。根据统一模型约束，`TodoItem` 与 `Task` 必须分层：前者表示未来安排 / 巡检事项，后者表示已进入执行。

分类结构包括：

- 近期要做
- 后续安排
- 重复事项
- 已结束事项

底层能力包括：

- 指定 `.md` 任务文件夹监听
- Markdown 任务项识别
- 日期、优先级、状态、标签提取
- 巡检频率、变更即巡检、启动时巡检
- 到期提醒、长时间未处理提醒
- 下一步动作建议、打开资料、生成草稿

对应模块分工：
- 前端工作台负责呈现事项桶与转任务入口；
- 后端巡检服务、规则引擎和 `agent.notepad.convert_to_task` 负责正式升级。

实现约束：

- `notes` 详情补强优先复用现有 `TodoItem / RecurringRule` 数据来源，不新增独立底座对象名；
- 详情补强字段中，`note_text`、`prerequisite`、`related_resources`、`linked_task_id` 已进入稳定 `TodoItem` 协议投影；`planned_at`、`ended_at` 等存储补强字段继续留在后端运行态与持久化层承接。
- 重复事项补强字段中，`repeat_rule_text`、`next_occurrence_at`、`recent_instance_status`、`effective_scope` 用于规则引擎与巡检底座；前端稳定消费的协议字段仍以 `TodoItem.repeat_rule / next_occurrence_at / recent_instance_status / effective_scope / recurring_enabled` 为准。
- complete / cancel / restore / toggle-recurring / delete 等事项动作已经通过 `agent.notepad.update` 进入稳定 RPC 面；运行态与存储层继续负责真实生命周期收敛。
- “打开相关资料”当前通过 `related_resources[].open_action / open_payload` 与共享 delivery open 语义承接，不额外冻结专用 `agent.notepad.open_resource` 接口。

### 3.7.4 镜子记忆与长期协作域

镜子不是聊天记录页，而是长期协作的认知层，用于沉淀短期记忆、长期记忆和镜子总结。该功能域与运行态状态机严格分层，长期记忆支持本地 RAG 检索，但写入与检索都必须通过 Memory 内核统一接入。

三层结构为：

1. 短期记忆：支撑连续任务理解
2. 长期记忆：偏好、习惯、阶段性信息沉淀
3. 镜子总结：日报、阶段总结、用户画像显性展示

设计约束：

- 默认本地存储，可一键开关
- 长期记忆与运行态恢复状态分离
- 用户可见、可管理、可删除
- 周期总结和画像更新受控制面板配置控制

### 3.7.5 安全卫士与恢复治理域

该功能域是用户可感知的治理外显层，对应后端风险评估、授权确认、审计、恢复点与回滚能力。其职责不是重新实现治理逻辑，而是把绿/黄/红三级风险、待确认动作、影响范围、恢复点和中断操作清晰暴露给用户。

主要能力包括：

- 工作区边界控制
- 风险分级
- 授权确认
- 影响范围展示
- 一键中断
- 恢复与回滚
- Token 与费用治理
- 审计日志
- Docker 沙盒执行策略接入

### 3.7.6 控制面板与系统配置域

控制面板是系统配置中心，不承接任务，不替代仪表盘。它承担通用设置、悬浮球、记忆、任务巡检、模型与安全等系统级配置职责，是桌面宿主与本地 Harness 行为约束的显式入口。

主入口为托盘右键，信息架构分为：

- 通用设置
- 悬浮球
- 记忆
- 任务与巡检
- 模型与安全

补充约束：

- `task_automation` 当前以 `agent.settings.get / update` 中的正式 settings snapshot 为真源；`agent.task_inspector.config.get / update` 仅作为巡检配置兼容入口复用同一份 `task_automation` 数据，不再维护独立 runtime config。
- “关于”、日志查看、数据清理等入口仍属后续扩展项，当前桌面控制面板尚未冻结为独立导航分组。

### 3.7.7 扩展能力中心与多模型配置域

该功能域用于承接产品的成长性能力，不面向普通用户暴露复杂底层实现，而是在基础闭环稳定后，为进阶用户提供可扩展的插件、技能和模型能力。它对应原子功能中的“多生态插件、多模型配置、感知包扩展、兼容社区 Skills 生态”，同时受统一规范约束：插件必须通过 Go service 编排，模型接入必须通过统一 SDK 接入层与配置入口进入，不允许直接散落在业务逻辑中。

主要能力包括：

- 多生态插件：通过 Manifest + 独立 Worker + 本地 IPC / JSON-RPC 扩展感知、处理和交付能力。
- 感知包扩展：允许按办公、开发、娱乐等场景加载不同感知包，但必须走统一权限、边界和审计链路。
- 社区 Skills 兼容：支持安装经过验证的 Skills 资产，来源可包括 GitHub 等外部仓库，但安装和启用需受版本、权限和工作区策略控制。
- 多模型配置：支持提供商切换、模型 ID 切换、本地模型接入和不同能力使用不同模型策略。
- 工具路由与边界策略：允许针对不同插件、技能和模型配置不同的工具权限、执行边界和成本策略。

设计约束：

- 插件与技能不直接面向前端开放调用，必须通过 `/services/local-service` 统一编排。
- 模型切换不改变 `task / run / delivery_result` 等核心协议对象。
- 插件、技能、模型配置都必须有版本、来源与权限描述，以便进入审计和 Trace；文档中的 built-in skill / blueprint / prompt、model provider route 与 plugin manifest 应统一归到 extension asset attribution 边界。

### 3.7.8 上下文感知与主动协助域

该功能域用于把“当前桌面发生了什么”和“此刻是否应该帮忙”转为可计算的输入信号，对应原子功能中的复制行为感知、屏幕/页面感知、行为与机会识别、主动推荐触发规则等能力。它并不直接替代任务入口，而是为入口承接、推荐系统和 Context Manager 提供更稳定的任务对象来源。

主要能力包括：

- 复制行为感知：识别复制、粘贴、选中等行为是否形成协作机会。
- 屏幕与页面感知：读取当前页面标题、窗口、选区、剪贴板、可视区域和停留状态。
- 行为与机会识别：基于停留、切换、重复失败、错误出现等信号，判断是否触发轻提示。
- 主动推荐触发规则：定义什么场景允许推荐、什么场景必须静默、是否存在冷却时间与置信度阈值。
- 特定对象扩展：为错误对象、文件对象、文本对象和后续的视频对象预留统一接入形态。

设计约束：

- 强意图优先于弱信号，用户显式触发必须覆盖主动推荐。
- 复制、停留、切换等弱信号默认保守处理，不得变成高频打扰源。
- 感知信号只能作为输入候选，正式执行仍需经过意图确认、风险治理和交付内核。

实现约束：

- 后端可维护 richer perception signal snapshot（如 `clipboard_text`、`window_title`、`visible_text`、`screen_summary`、`dwell_millis`、`copy_count`、`window_switch_count`、`page_switch_count`），并可把内建 perception package 的 version/source/permissions 通过 execution attribution 回流到 Trace/Eval，但不得绕过现有稳定 RPC 直接发明新的正式协议对象；
- recommendation 与 dashboard 可消费这些 richer signals 做机会识别和高价值信号增强，但正式推荐触发边界仍以稳定协议真源为准；
- 屏幕 / 页面 / 复制 / 停留 / 切换信号属于上下文候选输入，不得直接替代 `task` 创建、授权或正式交付链路；
- 感知能力增强应优先服务主动推荐、Context Manager 和 memory query，而不是先扩散到新的前端页面状态模型。

## 4. 后端模块设计

### 4.0 后端分层阅读总览

后端章节以 [architecture-overview.md](./architecture-overview.md) 的层级命名为主来展开：

- `4.1` 对应 **本地接入层**；
- `4.2` 对应 **任务编排与运行层**；
- `4.4` 对应 **治理与交付层**；
- `4.3` 与 `4.5` 共同对应 **能力与存储层**。

其中，架构总览里的“能力与存储层”在本模块文档里拆成两个实现视角：

- `4.3` 负责“模型、工具、worker、检索”等**能力接入与检索子层**；
- `4.5` 负责“平台抽象、执行后端、对象仓储、路径与机密存储”等**平台、执行与持久化适配子层**。

#### 4.0.1 后端分层与反馈总览图

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '16px'}}}%%
flowchart TB
    subgraph L1[4.1 本地接入层]
        direction TB
        RPC[协议服务<br/>（JSON-RPC Server）]
        ROUTE[任务与会话路由<br/>（Task / Session）]
        QUERY[查询装配与通知回流]
    end

    subgraph L2[4.2 任务编排与运行层]
        direction TB
        ORCH[任务编排器]
        CTX[上下文归一与准备器]
        PLAN[入口判断与规划器]
        RUN[运行控制器<br/>（TaskRecord）]
        LANE[会话续接与串行器]
    end

    subgraph L3[4.4 治理与交付层]
        direction TB
        RISK[风险与授权]
        REVIEW[审查 / 追踪 / 评估<br/>（Trace / Eval）]
        DELIVERY[正式交付 / 记忆沉淀]
        RECOVER[审计 / 恢复]
    end

    subgraph L4[4.3 + 4.5 能力与存储层]
        direction TB
        CAP[模型 / 工具 / 插件 / 工作者]
        STORE[对象仓储 / 索引 / 产物<br/>（Artifact）]
        PLATFORM[路径策略 / 宿主能力 / 执行后端]
    end

    RPC --> ROUTE --> ORCH
    QUERY --> ROUTE
    ORCH --> CTX --> PLAN --> RUN
    ORCH --> LANE --> RUN
    RUN --> CAP --> PLATFORM
    RUN --> RISK
    RUN --> DELIVERY
    CAP --> STORE
    DELIVERY --> STORE
    REVIEW --> STORE
    RECOVER --> STORE

    RISK -.->|待授权通知 / 授权记录| QUERY
    DELIVERY -.->|交付就绪 / 产物 / 引用| QUERY
    REVIEW -.->|循环事件 / 追踪摘要 / 评估摘要| QUERY
    RECOVER -.->|恢复点 / 恢复结果| QUERY
    QUERY -.->|任务更新 / 安全摘要 / 刷新投影| RPC
```

#### 4.0.2 阅读说明

- 想看“请求怎样被稳定锚定到 `task / session / trace`”，先读 `4.1`。
- 想看“到底是谁编排、AI 在哪里参与、谁推进状态”，先读 `4.2`。
- 想看“模型、工具、worker 和检索怎样被统一接入”，读 `4.3`。
- 想看“高风险动作、正式交付、审查、恢复怎样回流主链”，读 `4.4`。
- 想看“数据、路径、机密存储和执行后端怎样兜底”，读 `4.5`。


## 4.1 本地接入层模块

### 模块定位
本地接入层负责把桌面前端发起的 JSON-RPC 请求收口成稳定的后端入口，并把运行期通知、正式交付和查询投影以统一口径回流给前端。它是前后端之间唯一稳定边界的实现层，不负责业务规划本身。

### 层内子模块图

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '16px'}}}%%
flowchart TB
    subgraph A1[本地接入层]
        direction TB
        JRPCS[协议服务<br/>（JSON-RPC Server）]
        TASKAPI[任务与会话路由器<br/>（Task / Session）]
        QUERY[查询装配器]
        STREAM[通知回流器]
    end

    subgraph A2[任务编排与运行层]
        direction TB
        ORCH[任务编排器]
        RUN[运行控制器]
    end

    subgraph A3[治理与交付层]
        direction TB
        GOV[授权 / 交付 / 恢复结果]
    end

    JRPCS --> TASKAPI --> ORCH
    TASKAPI --> QUERY
    ORCH --> RUN
    RUN -.->|任务更新 / 循环事件 / 人工转向| STREAM
    GOV -.->|待授权 / 交付就绪 / 恢复结果| STREAM
    QUERY --> JRPCS
    STREAM --> JRPCS
```

### 组成
- JSON-RPC 2.0 Server
- Task / Session 路由器
- 查询装配器
- 通知回流器

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 对外输出与反馈 |
| --- | --- | --- | --- |
| JSON-RPC 2.0 Server | 前端方法调用、订阅建立、健康检查请求 | 规范化请求体、错误包装、`trace_id` | 标准成功响应、标准错误响应 |
| Task / Session 路由器 | `agent.input.submit`、`agent.task.start`、`agent.task.confirm`、`agent.task.control` 等稳定方法 | `task_id / session_id / trace_id` 锚点、标准参数对象 | 下发到编排层的标准请求 |
| 查询装配器 | 任务列表、详情、控制面板、安全摘要等查询请求 | 任务快照、治理摘要、存储查询结果的装配视图 | 统一的查询响应，不透传运行缓存 |
| 通知回流器 | `TaskRecord` 通知队列、治理层回流对象 | 有序通知批次、订阅投影、重放顺序 | `task.updated / delivery.ready / approval.pending / loop.*` 等正式通知 |

### 职责
- 解析请求并校验 schema；
- 把请求绑定到稳定的 `task / session / trace` 锚点；
- 把查询请求装配成前端可消费的正式对象；
- 回放运行期通知和治理回流；
- 向前端返回正式对象和标准错误结构。

### 上下游关系
- 上游是桌面入口层发起的 JSON-RPC 请求，以及前端对正式对象的查询和控制动作。
- 下游一方面是任务编排与运行层，另一方面是治理与交付层回流的审批、交付和恢复对象。
- 该层向上游返回的是标准响应、聚合查询和稳定通知，而不是底层运行缓存、worker 原始输出或 provider 响应。

### 输入
- 前端 JSON-RPC 请求；
- `runengine.TaskRecord` 通知队列；
- 治理与交付层输出的正式对象与回流事件。

### 输出
- 标准 JSON-RPC 成功响应和错误响应；
- 订阅流与运行通知；
- 任务列表、任务详情、仪表盘、安全摘要等查询视图。

### 关键接口
- `handleRequest(jsonrpcRequest)`
- `publishNotification(method, params)`
- `registerSubscription(topic, subscriber)`
- `marshalResult(data, meta, warnings)`
- `marshalError(code, message, traceId)`

### 边界
- 不负责任务规划；
- 不负责状态机推进；
- 不负责模型、工具或数据库直连执行；
- 只做协议收口、对象装配、错误包装和通知回流。

### 异常处理
- 非法方法：返回统一协议错误码；
- 参数不合法：走 schema 校验错误；
- 内核异常：包装成正式错误结构，不透传临时栈信息；
- 订阅方断开：回收订阅资源，不阻断主运行对象。

### 联调重点
- `task_id / session_id / trace_id` 是否贯穿请求、查询、通知和交付；
- Notification 是否只承担状态变化，而不变成隐藏命令通道；
- 长任务查询结果与通知回放是否口径一致；
- 错误码和 `trace_id` 是否总能回传。

---

## 4.2 任务编排与运行层模块

### 模块定位
任务编排与运行层负责把“前端提交的事实”收敛成“可执行、可暂停、可恢复、可交付”的正式任务主链。当前编排主体不是 AI 自由发挥，而是 `orchestrator.Service` 与 `runengine.Engine` 共同维护的一条固定主流程：

1. 先捕获并归一上下文；
2. 再做入口判断与计划骨架生成；
3. 再创建或续接正式 `task`；
4. 再经过会话串行、治理判断与受控执行；
5. 最后把执行结果、通知、交付和恢复结论重新回流到正式对象链。

AI 参与的环节只发生在“轻量建议”和“受控执行”中，不直接拥有状态机、授权流、队列或正式交付语义。

### 层内子模块图

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '16px'}}}%%
flowchart TB
    REQ[标准请求<br/>（任务 / 会话 / 追踪锚点）] --> ORCH[任务编排器]

    subgraph B1[前馈准备]
        direction TB
        CTX[上下文归一与准备器<br/>（TaskContextSnapshot）]
        PLAN[入口判断与规划器<br/>（Suggestion / ContinuationDecision）]
        ASSET[资产路由与提示词组装器<br/>（Skill / Blueprint / Prompt）]
    end

    subgraph B2[运行控制]
        direction TB
        RUN[运行控制器<br/>（runengine.TaskRecord）]
        LANE[会话续接与串行器]
        HANDOFF[记忆计划与交付交接协调器]
    end

    subgraph B3[执行桥接]
        direction TB
        EXECREQ[执行请求<br/>（execution.Request）]
        EXEC[受控执行循环<br/>（execution / agentloop）]
        EXECRES[执行结果与循环事件<br/>（execution.Result / loop.*）]
    end

    ORCH --> CTX --> PLAN --> ASSET --> RUN
    ORCH --> LANE --> RUN
    RUN --> HANDOFF
    RUN --> EXECREQ --> EXEC --> EXECRES --> RUN
    EXECRES --> HANDOFF

    GOV[治理与交付层结论] -.->|授权 / 审查 / 恢复| RUN
    GOV -.->|拦截 / 重规划 / 阻断| ORCH
    EXECRES -.->|追踪 / 人工介入触发| ORCH
    RUN -.->|任务更新 / 交付就绪 / 人工转向| RESP[本地接入层回流]
```

### 编排主体与边界

- **主编排者是后端代码，不是模型本身**：`orchestrator.Service` 决定调用顺序、层间交接和异常分流，`runengine.Engine` 决定正式状态推进和通知回放。
- **AI 只参与受控决策点**：当前代码里 AI 主要参与 `task_continuation` 分类、`intent` 的轻量建议，以及 `execution.Service` 内的 Prompt 生成和执行循环，不直接写 `task.status`。
- **入口规划不是完整工作流引擎**：当前 `intent.Service` 只生成入口级 `Suggestion` 和计划骨架；真正的复杂推理、工具调用和 ReAct/Agent Loop 在执行阶段发生。
- **正式状态推进点只有 `runengine.Engine`**：创建任务、确认意图、进入执行、等待授权、等待补充输入、阻断、失败、完成、队列恢复都必须回到 `TaskRecord` 收口。

### 组成
- 任务编排器
- 上下文归一与准备器
- 入口判断与规划器
- 资产路由与 Prompt 组装器
- 运行控制器
- 会话续接与串行器
- 记忆计划与交付交接协调器
- 执行分段与子任务协调器
- 插件运行态协调器
- 反馈回流与边界收敛器

### 关键中间产物

- `TaskContextSnapshot`：上下文归一后的稳定快照，是入口判断和执行请求的共同输入基线。
- `taskContinuationDecision`：续接现有任务还是新开任务的决策结果。
- `Suggestion`：入口判断与规划器输出的轻量建议，包含 `Intent`、`RequiresConfirm`、`TaskTitle`、`DirectDeliveryType` 等字段。
- `runengine.CreateTaskInput`：把 `Suggestion`、快照和交付偏好折叠成正式建任务输入。
- `runengine.TaskRecord`：桥接 `task` 与 `run` 的核心运行态结构，保存时间线、审批对象、通知队列、记忆计划、交付结果等。
- `execution.Request / execution.Result`：进入受控执行循环前后的标准桥接件。
- `NotificationRecord`：等待本地接入层回放的有序通知批次。

### 输入
- 本地接入层下发的标准任务请求；
- `TaskContextSnapshot` 候选上下文；
- 能力与存储层返回的模型、工具、worker、检索结果；
- 治理与交付层返回的审批、审查、熔断、恢复和人工复核结论。

### 输出
- 正式 `task` 投影；
- 执行兼容对象 `run / step / event / tool_call`；
- `delivery_result / artifact / citation` 的交接请求；
- `approval_request / authorization_record / recovery_point` 的交接请求；
- 给本地接入层回放的有序通知。

### 关键接口
- `orchestrator.SubmitInput()`
- `orchestrator.StartTask()`
- `orchestrator.ConfirmTask()`
- `orchestrator.TaskControl()`
- `runengine.CreateTask()`
- `runengine.ConfirmTask()`
- `runengine.BeginExecution()`
- `runengine.EmitRuntimeNotification()`

### 边界
- 本层可以调用能力与存储层和治理与交付层，但不能反向依赖平台具体实现；
- 本层只维护统一任务语义，不引入页面视角对象模型；
- 所有 worker、插件、子执行分支输出都必须包装成标准对象链回流。

### 异常处理
- 前馈不收敛：进入澄清、确认或等待补充输入；
- 会话冲突：进入 `session_queue` 排队，不并行污染同一 `session`；
- 执行失败：由运行控制器收敛到失败态，再交给治理与交付层补审计与恢复结果；
- 人工复核命中：回到重新规划或恢复执行路径，而不是静默吞掉失败。

### 联调重点
- `task` 与 `run` 是否稳定映射；
- 前馈决策是否在关键调用前执行，而不是只在任务开始时执行一次；
- 同一 `session` 的排队、续接、暂停、恢复、取消是否都回到了统一状态机；
- 审批、重规划、需要补充输入、Doom Loop 与恢复结果是否都能重新进入编排循环；
- worker / plugin / sidecar 输出是否全部回到了统一对象链。

### 实现边界
- `context.Service` 的稳定输出是 `TaskContextSnapshot`，职责是把页面、选区、文件、错误和行为信号归一化，而不是直接推进 `task.status`。
- `intent.Service` 当前仍是轻量建议层，其产物是 `Suggestion`；是否创建任务、是否等待确认、是否直接执行，仍由编排层与运行态收敛。
- `runengine.TaskRecord` 是当前后端桥接 `task` 与 `run` 的核心运行态结构，用于承接主对象投影、治理摘要、通知队列和会话排队。
- `execution.Request / execution.Result` 是执行桥接件，不是正式业务对象；正式状态仍需回到 `runengine` 收口。
- `task.updated / delivery.ready / approval.pending / loop.* / task.steered / task.session_resumed` 是本层最重要的反馈线出口。

### 4.2.1 任务编排器

#### 模块定位
任务编排器由 `orchestrator.Service` 承担，是这一层的总调度器。它负责把不同入口方法统一收敛到一条固定主流程，而不是把每个入口各写一套独立流程。

#### 核心职责
- 收口 `SubmitInput / StartTask / ConfirmTask / TaskControl`；
- 判断创建新任务还是续接现有任务；
- 把 `Suggestion` 折叠成正式 `CreateTaskInput`；
- 协调会话排队、治理判断、执行启动与结果回流；
- 把控制动作翻译成状态机可接受的正式迁移。

#### 内部处理细节
1. 先调用 `context.Capture()` 生成 `TaskContextSnapshot`。
2. 再调用 `maybeContinueExistingTask()` 判定是继续未完成任务还是创建新任务。
3. 对新任务调用 `intent.Suggest()` 生成 `Suggestion`，并据此决定 `status / current_step / delivery_type`。
4. 用 `runengine.CreateTask()` 或 `runengine.ConfirmTask()` 建立正式 `task -> run` 映射。
5. 调用 `attachMemoryReadPlans()` 把本轮记忆检索计划先挂到任务运行态。
6. 若同一 `session` 已有活动任务，调用 `queueTaskIfSessionBusy()` 进入排队。
7. 若存在高风险动作或策略拦截，再进入 `handleTaskGovernanceDecision()`。
8. 只有在前述步骤都通过后，才调用 `executeTask()` 真正开始执行。

#### 关键中间产物
- `taskContinuationDecision`
- `runengine.CreateTaskInput`
- 统一 RPC 响应包：`task / bubble_message / delivery_result`

#### 对上下游的影响
- 上游看到的是统一的 `task` 主对象，而不是编排器内部条件分支。
- 下游的上下文准备器、规划器、运行控制器、治理层都以编排器输出的标准对象为输入。

#### 异常处理
- 续接候选任务失效：回退成新任务创建；
- 规划与执行冲突：回退到确认或人工复核；
- 排队恢复失败：保留当前任务并记录错误，不隐式创建新任务。

### 4.2.2 上下文归一与准备器

#### 模块定位
上下文归一与准备器由 `context.Service` 承担，负责把前端和平台层传来的杂乱现场信号压平成统一快照。它输出的是“运行前快照”，不是已经可直接喂给模型的最终 Prompt。

#### 核心职责
- 把 `input.*` 和 `context.*` 合并成一份稳定结构；
- 把文本、选区、错误、文件、页面、窗口、屏幕、行为、剪贴板等来源统一命名；
- 为后续记忆查询、入口判断和执行请求提供同一份基线快照。

#### 上下文范围
- 请求入口上下文：`source / trigger / input_type / input_mode / text`
- 近场对象上下文：`selection_text / error_text / files`
- 页面与窗口上下文：`page_title / page_url / app_name / window_title / visible_text`
- 屏幕与行为上下文：`screen_summary / hover_target / last_action / dwell_millis / copy_count / switch_count`
- 系统补充上下文：`clipboard_text`

#### 关键中间产物
- `TaskContextSnapshot`
- 基于快照生成的 `memory query`、`task title subject`、`screen subject`

#### 对规划器和运行控制器的影响
- 规划器根据快照决定 `Intent`、`TaskTitle`、`RequiresConfirm` 和交付类型。
- 运行控制器把同一快照写入 `TaskRecord.Snapshot`，用于续接、排队恢复、人工复核后的重规划和 Trace 摘要。

#### 异常处理
- 输入源缺失：降级为最小快照；
- 字段冲突：优先采用已规范的 `input.* / context.*` 合并规则；
- 快照不完整：允许继续走确认或等待补充输入，不直接报错。

### 4.2.3 入口判断与规划器

#### 模块定位
入口判断与规划器由 `intent.Service` 和任务续接分类器共同承担，负责把“收到什么输入”转换成“当前任务主链应该怎样进入”。它产出的是入口级计划骨架，不是完整的执行工作流。

#### 核心职责
- 判定当前输入是“等待补充输入”“确认意图”“直接执行”还是“续接现有任务”；
- 生成任务标题、任务来源类型、默认交付方式和确认文案；
- 为 `screen_analyze` 之类特殊入口补齐无需确认的快捷路径；
- 只在必要时触发模型参与的续接分类，不让模型接管状态机。

#### 具体判断方式
- **确定性规则**：空输入直接进入 `waiting_input`；显式 `intent` 优先。
- **轻量建议**：`intent.Suggest()` 输出 `Suggestion`，包含 `IntentConfirmed / RequiresConfirm / TaskTitle / DirectDeliveryType` 等字段。
- **确认策略**：`options.confirm_required` 只负责阻止当前输入直接执行；附件等对象型入口若已经带有明确用户说明，应直接进入 Agent Loop / 治理 / 执行链路，裸对象或低置信度输入才停在确认或澄清。
- **续接分类**：`maybeContinueExistingTask()` 先走确定性与启发式规则，不足时才调用模型做 coarse-grained continuation classification；不论 `confirm_required` 是否为 `true`，文件、选区、错误等结构化补充证据若要续接到 `waiting_input / confirming_intent` 这类待用户补充的任务，都必须带有任务特定证据（如 lineage、共享文件、同一页面 / 窗口 / 对象锚点），不能只因为输入是结构化对象就续接；多候选场景下只有存在唯一任务特定匹配时才允许续接；不能续接正在执行的任务，也不能自动执行；悬浮球默认入口锚点（如 `desktop / local://shell-ball`）只表示补充证据从哪里进入系统，不应作为正向续接证据，也不应覆盖或冲突于原 task 的真实页面 / 应用锚点。
- **执行阶段分工**：入口规划只决定“以什么意图、什么交付类型、是否要确认进入主链”；真正的 ReAct/Agent Loop 计划在执行阶段发生。

#### 关键中间产物
- `Suggestion`
- `taskContinuationDecision`
- `pending confirmation bubble text`

#### 运行控制器如何承接
- 编排器把 `Suggestion` 转成 `CreateTaskInput`；
- 运行控制器据此种下 `Status`、`CurrentStep`、`PreferredDelivery / FallbackDelivery` 和首条 `Timeline`；
- 若 `RequiresConfirm = true`，先停在 `confirming_intent`，等待后续 `agent.task.confirm`；
- 若 `RequiresConfirm = false`，直接进入治理判断与执行。

#### 异常处理
- 低置信度：进入确认或澄清；
- 特定能力不可用：如屏幕能力不可用时，降级为 `agent_loop` 继续处理；
- 续接分类失败：回退到启发式判断，再不行则新开任务。

### 4.2.4 资产路由与 Prompt 组装器

#### 模块定位
这是一个逻辑子模块，不完全等同于当前仓库中的单独 package。它负责在正式执行前把 Skill、Blueprint、Prompt 模板、AGENTS 规则和架构约束装配到执行桥接件中。

#### 核心职责
- 决定本轮执行需要引用哪些技能资产和模板资产；
- 组装任务 continuation classifier Prompt 与正式执行 Prompt；
- 把资产命中结果登记进 Trace / Eval，而不是只留在临时字符串里。

#### 实现落点
- `task_continuation.go` 负责 continuation classifier Prompt；
- `execution/service.go` 中的 `buildPrompt()` 负责正式执行 Prompt；
- `storage` 中的 `skill_manifest / blueprint_definition / prompt_template_version` 负责资产真源；
- `traceeval` 负责记录资产引用和版本命中。

#### 关键中间产物
- Prompt 文本
- 资产引用：`skill_manifest / blueprint_definition / prompt_template_version`
- `execution.Request` 中的执行约束与交付偏好

#### 异常处理
- 模板缺失：回退到默认 Prompt，并在 Trace 中记录；
- 资产读取失败：不阻断主链，但必须降级并带上可观测记录；
- 约束过长：由上下文准备器优先裁剪，而不是让 Prompt 无限膨胀。

### 4.2.5 运行控制器

#### 模块定位
运行控制器由 `runengine.Engine` 承担，是正式状态推进点。它不决定用户意图，但决定任务何时进入执行、何时等待授权、何时排队、何时失败、何时重新打开等待输入，以及何时完成交付。

#### 核心职责
- 维护 `TaskRecord` 这一条正式运行态主记录；
- 建立 `task_id -> run_id -> timeline` 稳定映射；
- 种下和更新 `ApprovalRequest / PendingExecution / Authorization / ArtifactPlans / Notifications`；
- 记录 `LatestEvent / LatestToolCall / LoopStopReason`；
- 统一产出 `task.updated / delivery.ready / tool_call.completed / loop.*` 等通知。

#### 关键中间产物
- `runengine.TaskRecord`
- `NotificationRecord`
- `PendingExecutionPlan`
- `TaskStepRecord` 时间线

#### 如何承接规划器产物
- `CreateTaskInput` 决定初始 `Status / CurrentStep / Timeline / Snapshot / Intent`；
- `ConfirmTask()` 把确认后的意图和标题写回同一任务；
- `BeginExecution()` 把任务真正推进到运行态；
- `CompleteTask()`、`ReopenWaitingInput()`、`ReopenIntentConfirmation()`、`FailTaskExecution()` 等接口负责后续收敛。

#### 反馈线
- 把执行结果回写成 `delivery_result / artifact / citation` 投影；
- 把运行事件回写成 `LatestEvent / LatestToolCall`；
- 把通知缓存到 `TaskRecord.Notifications`，等待本地接入层按序 drain。

#### 异常处理
- 未知状态迁移：直接阻断并记录错误；
- 非法恢复或越级控制：要求治理层或人工复核先介入；
- 通知 drain 失败：保留在运行态缓存，避免丢失正式事件。

### 4.2.6 会话续接与串行器

#### 模块定位
该子模块负责把“桌面连续补充”收敛到同一 `session` 语义下，决定是续接、排队还是新开任务。它解决的是协作连续性问题，不是模型推理问题。

#### 核心职责
- 显式 `session_id` 与隐式最近活动 `session` 的复用；
- 未完成任务候选收集与 15 分钟续接窗口判断；
- 同一 `session` 的单通道串行执行；
- `session_queue` 排队、自动恢复、追加 steering message；
- 暂停 / 恢复 / 取消等控制动作的 lane 级收敛。

#### 关键中间产物
- `taskContinuationContext`
- `taskContinuationDecision`
- `task.session_resumed / task.steered` 通知

#### 运行控制器如何承接
- 若 lane 忙，则运行控制器把任务转入 `blocked + session_queue`；
- 当前序任务完成后，通过 `NextQueuedTaskForSession()` 和 `ResumeQueuedTask()` 恢复；
- 追加消息通过 `AppendSteeringMessage()` 并入同一 `TaskRecord`。

#### 异常处理
- 候选任务状态不允许续接：直接新开任务；
- 隐式会话过期：不复用旧 lane；
- 队列恢复失败：保留阻断态并生成正式错误，而不是静默丢失任务。

### 4.2.7 记忆计划与交付交接协调器

#### 模块定位
该子模块负责在“真正执行前”和“执行完成后”分别挂接记忆与交付的计划对象。它负责的是**交接与计划**，不是最终的记忆持久化或交付发布拥有者。

#### 核心职责
- 在任务开始或确认后，通过 `attachMemoryReadPlans()` 预登记本轮记忆召回计划；
- 在执行完成后，把 `delivery_result / artifact / citation` 的后续写入和查询补全交给治理与交付层、能力与存储层；
- 保证即使进程重启，也能说明“这个任务原本打算读什么记忆、写什么交付”。

#### 关键中间产物
- `MemoryReadPlans`
- `ArtifactPlans`
- `StorageWritePlan`
- `citation_seed`

#### 对其它层的影响
- 规划器和执行器都以这些计划为前置约束；
- 治理与交付层据此构造正式交付与记忆沉淀；
- 存储层据此做 artifact、citation、memory 的真正落盘。

#### 异常处理
- 计划登记失败：不直接终止任务，但必须进入 Trace；
- 后续交付写入失败：保留结果摘要和失败说明，避免任务看似“成功但无交付”。

### 4.2.8 执行分段与子任务协调器

#### 模块定位
该子模块负责把一次任务执行拆成“首次执行、恢复执行、重启执行”等分段，并隔离长执行分支带来的噪音。当前仓库中它主要体现为 `execution.Request` 的 `AttemptIndex / SegmentKind / SteeringMessages`，而不是独立的子任务树服务。

#### 核心职责
- 为一次任务执行标记 `initial / resume / restart` 分段；
- 隔离长任务的 steering message 和重试上下文；
- 把执行尝试和人类复核后的继续执行放回同一主任务，而不是分叉出新的正式主对象；
- 为后续真正的一等子任务能力预留边界。

#### 关键中间产物
- `AttemptIndex`
- `SegmentKind`
- `SteeringMessages`

#### 异常处理
- 分段信息失真：回退为安全的 `initial` 段；
- 重试路径有副作用：禁止自动升级到人类复核后继续执行的快捷路径。

### 4.2.9 插件运行态协调器

#### 模块定位
该子模块负责把插件运行态、健康状态、权限边界和事件回传并入正式任务链与观测链。它关注的是“插件怎样纳入主链”，而不是前端怎样画插件 UI。

#### 核心职责
- 插件注册信息、能力声明、权限边界、版本与健康状态统一纳入注册表；
- 插件输出的运行指标、事件、结果摘要统一写入事件流；
- 插件命中资产版本进入 Trace / Eval；
- 仪表盘、任务详情和安全摘要消费的永远是正式事件与正式查询视图。

#### 实现约束
- 当前代码已具备 `agent.plugin.runtime.list / agent.plugin.list / agent.plugin.detail.get` 的查询装配；
- 插件运行态与任务详情 runtime 观察已经进入正式可见层；
- 后续仍应以正式任务详情、仪表盘和安全摘要承接为主，不在本模块文档里提前冻结超出当前协议稿范围的附加执行接口。

#### 异常处理
- 插件启动失败：写入正式插件错误事件；
- 插件权限不足：直接拒绝并进入安全或审计链路。

### 4.2.10 实现边界与反馈回流

#### 实现边界
- 资产路由与 Prompt 组装当前仍是逻辑切面，尚未独立成单独 service；
- 入口规划只解决“怎样入链”，不直接替代执行期 Planner；
- 记忆与交付在本层只负责挂计划与交接，不在本层完成最终拥有。

#### 反馈回流主线
- **治理到运行控制器**：授权通过、策略拒绝、恢复结果、人工复核结果都会重新改写 `TaskRecord`。
- **执行到运行控制器**：`execution.Result`、`tool_call.completed`、`loop.*`、`need_user_input` 都会触发状态收敛。
- **运行控制器到本地接入层**：通过 `TaskRecord.Notifications` 有序回放 `task.updated / delivery.ready / task.steered / task.session_resumed`。
- **人工复核到规划器**：`replan` 会调用 `ReopenIntentConfirmation()` 回到确认阶段，`approve` 才会恢复执行。

---

## 4.3 能力与存储层模块（能力接入与检索子层）

### 模块定位
本章承接架构总览中的“能力与存储层”里偏能力接入的一半：统一接入模型、工具、worker、sidecar、代码语义和检索能力，并把它们转换成可被任务编排与运行层消费的标准结果。它负责“提供能力”，不负责“拥有任务状态”。

### 能力接入与检索关系图

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '16px'}}}%%
flowchart TB
    ORCH[任务编排器]
    RUN[运行控制器]
    GOV[治理与交付层]
    CTX[上下文归一与准备器]

    subgraph B3[能力与存储层<br/>能力接入与检索子层]
        direction TB
        MODEL[模型接入]
        TOOL[工具执行适配器]
        CODE[代码语义能力<br/>（LSP）]
        WEB[网页自动化与页面能力<br/>（Playwright）]
        MEDIA[识别与媒体工作器<br/>（OCR / 媒体 / 视频 Worker）]
        RETRIEVE[记忆检索<br/>（RAG）]
        SCREEN[授权式屏幕 / 视频能力]
    end

    ORCH --> MODEL
    RUN --> TOOL
    RUN --> CODE
    RUN --> WEB
    RUN --> MEDIA
    ORCH --> RETRIEVE
    CTX --> SCREEN

    WEB -.->|工具调用记录 / 引用种子| RUN
    MEDIA -.->|产物候选 / 证据摘要| RUN
    RETRIEVE -.->|检索命中 / 召回摘要| ORCH
    TOOL -.->|治理评估 / 工具错误| GOV
```

### 组成
- 模型接入
- 工具执行适配器
- LSP / 代码语义能力
- Node Playwright Sidecar
- OCR / 媒体 / 视频 Worker
- 授权式屏幕 / 视频能力
- RAG / 记忆检索层

### 子模块说明
- **模型接入**：统一承接 OpenAI Responses API 和后续模型路由，不允许业务层直连 provider SDK。
- **工具执行适配器**：统一承接文件、网页、命令和其它工具调用，并把结果回写到正式对象链。
- **LSP / 代码语义能力**：为前馈和审查提供代码级语义支撑，而不是作为独立业务入口。
- **Playwright / OCR / Media Worker**：承接浏览器自动化、文字识别、媒体处理等外部能力，并通过统一 health-check 与错误码收口。
- **授权式屏幕 / 视频能力**：承接需要用户授权的采样与录制输入，但仍要回到主编排和治理链。
- **RAG / 记忆检索层**：提供本地召回与摘要回填能力，不直接修改任务主状态机。

### 层内处理细节

| 子模块 | 进入条件 | 关键中间产物 | 返回与反馈 |
| --- | --- | --- | --- |
| 模型接入 | 编排层已经给出 `execution.Request` | 模型输入摘要、模型调用元数据、原始生成结果 | 标准化模型输出、token/cost/latency 元数据 |
| 工具执行适配器 | 运行控制器决定进入某个工具意图 | `ToolCallRecord`、治理评估、错误分类 | `tool_call.completed`、统一错误码、交付候选 |
| LSP / 代码语义能力 | 需要前馈补充或审查代码语义 | 诊断结果、引用关系、符号信息 | 结构化语义结果，不直接改写任务状态 |
| Playwright / OCR / 媒体 Worker | 工具路由落到浏览器、OCR、媒体处理 | worker 请求、健康检查结果、`citation_seed`、artifact 元数据 | `tool_call`、artifact、证据摘要、worker 错误 |
| 授权式屏幕 / 视频能力 | 入口或执行意图需要屏幕/视频采样 | capture candidate、授权状态、录制片段元数据 | 待授权能力结果或可执行素材路径 |
| RAG / 记忆检索层 | 编排层挂接了记忆读取计划 | `retrieval_hit`、召回摘要、排序结果 | 给编排层的可注入记忆片段与检索摘要 |

### 输入
- 任务编排与运行层发出的能力请求；
- 平台适配层提供的执行环境；
- 配置和权限边界。

### 输出
- 标准化工具结果；
- 模型输出；
- LSP 诊断；
- OCR / 网页 / 视频处理结果。

### 上下游关系
- 上游来自任务编排与运行层和治理与交付层的能力调用请求、检索请求与执行请求。
- 下游是模型 provider、工具实现、sidecar worker、执行后端、检索索引和本地文件系统。
- 该层向上返回的是标准化结果和统一错误语义，而不是底层实现细节。

### 异常处理
- worker 不可用：统一包装错误码；
- sidecar 挂起：交给治理层做降级或升级；
- 模型超时：可触发重试、降级或预算保护。

### 联调重点
- 每种能力结果是否能回到标准对象链；
- 失败时是否一定有正式错误码和 Trace；
- 与平台层、治理层、编排层是否存在职责交叉。

### 4.3.1 模型接入

#### 模块定位
统一承接大模型接入，禁止业务层自行直连模型 SDK。

#### 职责
- 使用 OpenAI 官方 Responses API SDK；
- 对接标准 API，不自行实现 API 标准，也不自行维护一套独立客户端协议；
- 模型切换以配置为主：模型 ID、API 端点、密钥、预算策略；
- 支持 tool calling、流式结果与多轮关联；
- 模型调用审计与预算治理纳入统一链路。

#### 处理主线
1. 根据任务意图、预算策略和设置快照解析当前模型路由。
2. 读取密钥与 provider 配置，形成一次受控 `model invocation`。
3. 调用 Responses API，接收文本、工具调用和 usage 元数据。
4. 把 provider 响应归一成执行层可消费的标准输出块。
5. 把 `latency / token / cost / route` 元数据回写给 Trace / Eval 和成本治理。

#### 关键中间产物
- model route descriptor
- model invocation metadata
- normalized response blocks

### 4.3.2 工具执行适配器

#### 模块定位
统一路由文件、网页、命令等工具调用。

#### 职责
- 文件读写；
- 网页浏览与搜索；
- 命令执行；
- Workspace 内构建、测试、补丁生成；
- 外部执行后端路由。

补充约束：`exec_command` 默认优先路由到 Docker sandbox；仅对 `cmd` / `powershell` / `pwsh` 这类 Windows shell 入口保留受控宿主执行路径，避免在 Windows 主目标上把本地命令误送入 Linux 容器。

#### 处理主线
1. 根据 `Intent` 和 arguments 解析目标工具及目标对象。
2. 在真正执行前先生成治理评估，判断是否需要授权、是否越界、是否需要恢复点。
3. 形成标准工具请求并路由到文件、网页、命令、worker 或执行后端。
4. 把返回值归一成 `ToolCallRecord / tool output / artifact candidate / citation_seed`。
5. 把错误统一映射为正式错误码，而不是透传底层异常文本。

#### 关键中间产物
- governance assessment
- tool request payload
- `ToolCallRecord`

### 4.3.3 LSP / 代码语义能力

#### 模块定位
提供 IDE 视角的代码语义能力，服务前馈和审查。

#### 职责
- Go-to-definition；
- Hover / 类型与文档信息；
- Diagnostics；
- Rename / Refactor；
- 跨文件引用追踪；
- 作为 Context Manager 和 Agent Review 的代码语义输入源；
- 优先以独立 Worker 或 sidecar 形式接入，不把多语言语义分析硬塞进主业务内核。

#### 处理主线
1. 接收来自上下文准备器或审查引擎的符号定位请求。
2. 调用对应语言服务，获取 definition、hover、diagnostics 或引用信息。
3. 把语言服务结果压平为统一语义摘要，而不是把 IDE 私有结构直接透出。
4. 把结果交给上下文准备器补全 Prompt，或交给审查引擎做一致性检查。

#### 关键中间产物
- symbol lookup result
- diagnostics summary
- code reference snapshot

### 4.3.4 Node Playwright Sidecar

#### 模块定位
统一承接网页自动化能力。

#### 职责
- 浏览器自动化；
- 表单填写与页面操作；
- 网页抓取；
- 结构化 DOM/页面结果回传。

#### 实现约束
- Playwright sidecar 至少支持 `page_read`、`page_search`、`page_interact`、`structured_dom` 四类正式能力；
- sidecar 启动前必须通过健康检查，避免把未就绪 worker 暴露给主执行链；
- 传输层失败要清空 ready 状态并触发回收，普通请求失败则保留 ready 状态；
- 页面交互与结构化 DOM 结果必须通过 `tool_call -> event -> delivery_result` 链回写，而不是前端直连 sidecar；
- `tool_call.completed` 事件需要回写 worker/source/output 元信息，便于任务详情、通知订阅和后续审计复用。

#### 处理主线
1. 先确认 sidecar 健康状态与浏览器能力可用。
2. 接收标准页面能力请求，路由到 `page_read / page_search / page_interact / structured_dom`。
3. 把页面结果、结构化 DOM、截图或 URL 元数据组装成标准工具输出。
4. 为需要证据链的场景生成 `citation_seed` 与 artifact 候选。
5. 通过 `tool_call.completed` 和正式交付链回流，而不是独立暴露页面结果。

### 4.3.5 OCR / 媒体 / 视频 Worker

#### 模块定位
统一承接 OCR、媒体处理和视频摘要相关能力。

#### 职责
- Tesseract OCR；
- FFmpeg 转码与抽帧；
- yt-dlp 下载与元数据提取；
- MediaRecorder 结果后处理。

#### 实现约束
- OCR worker 至少支持 `extract_text`、`ocr_image`、`ocr_pdf`；
- Media worker 至少支持 `transcode_media`、`normalize_recording`、`extract_frames`；
- worker 启动必须通过健康检查，失败时返回统一 worker 错误码并触发 ready 状态清理；
- worker 输出需要带回标准化摘要和 artifact 元信息，供后续 `task_detail`、审计与交付链复用；
- worker 结果应回写 `tool_call.completed` 事件通知，至少包含 `source`、目标路径或 URL、以及关键产出元信息。

#### 处理主线
1. 根据任务意图选择 OCR、转码、抽帧或媒体后处理路径。
2. 调用对应 worker，并在调用前确认 worker ready 状态。
3. 把文本提取结果、帧列表、媒体产物和元数据归一成标准工具输出。
4. 为屏幕分析、录屏分析等场景补充 artifact 与 citation 候选。
5. 把 worker 状态和错误映射到正式通知和正式错误码。

### 4.3.6 授权式屏幕 / 视频能力

#### 模块定位
承接需要用户授权的屏幕和视频输入。

#### 职责
- `getDisplayMedia` 发起用户授权捕获；
- `MediaRecorder` 负责录制；
- 本地 worker 做切片、转码、OCR 与摘要。

#### 处理主线
1. 先生成待授权能力请求，并进入治理层授权链。
2. 授权通过后，再调用 `getDisplayMedia` 或录制能力采样。
3. 对原始截图、关键帧或录屏片段形成候选素材。
4. 把素材交给 OCR / 媒体 worker，产出可分析文本和证据产物。
5. 最终仍通过正式任务链交付，不形成前端私有结果通道。

### 4.3.7 RAG / 记忆检索层

#### 模块定位
提供本地检索能力，但不直接参与任务主状态机。

#### 职责
- 记忆向量化；
- 记忆候选召回；
- 记忆去重；
- 记忆排序；
- 记忆回填摘要；
- 结构化状态与语义检索解耦；
- 优先采用本地存储、本地索引、本地检索闭环。

#### 处理主线
1. 接收编排层登记的 memory read plan 或后续检索请求。
2. 并行执行关键词召回、向量召回和精确键值检索。
3. 合并候选结果并做去重、排序、摘要压缩。
4. 返回 `retrieval_hit` 与可注入摘要，而不是直接改写任务运行态。
5. 把检索命中情况回写给 Trace / Eval 作为前馈证据。

---

## 4.4 治理与交付层模块

### 模块定位
治理与交付层负责把“可以执行”收敛成“允许执行、可被观察、可被恢复、可被正式交付”的主链结果。它既是守门层，也是正式反馈和交付回流层；风险、授权、审查、Trace / Eval、恢复、正式交付、记忆沉淀都在这里并到同一条任务主链上。

### 治理与交付关系图

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '16px'}}}%%
flowchart TB
    RUN[运行控制器]
    ACCESS[本地接入层]

    subgraph B4[治理与交付层]
        direction TB
        SAFE[风险评估 / 授权承接]
        REVIEW[结果审查 / 钩子<br/>（Hooks）]
        TRACE[追踪与评估<br/>（Trace / Eval）]
        DELIVERY[正式交付协调]
        MEMORY[记忆 / 镜像沉淀]
        LOOP[循环检测 / 人工介入<br/>（Loop / HITL）]
        AUDIT[审计 / 恢复]
        POLICY[成本 / 边界策略]
    end

    RUN --> SAFE
    RUN --> REVIEW
    RUN --> TRACE
    RUN --> DELIVERY
    RUN --> MEMORY
    RUN --> LOOP
    SAFE --> AUDIT
    REVIEW --> DELIVERY
    DELIVERY --> MEMORY
    SAFE --> POLICY
    LOOP --> AUDIT

    SAFE -.->|待授权通知 / 授权记录| ACCESS
    DELIVERY -.->|交付就绪 / 产物 / 引用| ACCESS
    MEMORY -.->|记忆摘要 / 检索摘要| ACCESS
    AUDIT -.->|审计记录 / 恢复点 / 恢复结果| ACCESS
    TRACE -.->|追踪记录 / 评估结果 / 循环诊断| ACCESS
```

### 组成
- 风险评估与授权承接
- 正式结果交付协调
- 记忆与镜像沉淀
- 结果审查与 Hooks
- Trace / Eval
- Doom Loop / Human-in-the-loop
- 审计与恢复
- 成本与边界策略

### 子模块说明
- **风险评估 / 授权承接**：判断动作风险、形成待授权对象，并把用户决策重新并入主链。
- **正式结果交付协调**：把执行结果装配成 `delivery_result / artifact / citation`，并决定如何进入通知和查询视图。
- **记忆与镜像沉淀**：决定哪些运行结果应沉淀为长期记忆或镜像引用，哪些只能保留为运行态痕迹。
- **结果审查 / Hooks / Trace / Eval**：负责对输出做质量检查、记录命中、沉淀可观测性与评估快照。
- **Doom Loop / Human-in-the-loop**：在执行异常、重复无进展或高不确定性场景下，负责熔断、重规划和人工升级。
- **审计与恢复**：负责动作留痕、恢复点创建、恢复结果回流。
- **成本与边界策略**：负责预算降级、白名单、工作区边界和执行约束，保证高风险动作不会越界。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 返回与反馈 |
| --- | --- | --- | --- |
| 风险评估与授权承接 | 运行层提交的拟执行动作、目标范围、能力请求 | `risk_level`、`impact_scope`、`approval_request`、`pending_execution` | `approval.pending`、授权结果、阻断或继续执行结论 |
| 正式结果交付协调 | 执行结果、artifact/citation 候选、交付偏好 | `delivery_result`、artifact 计划、交付说明 | `delivery.ready`、任务详情交付、文件/文档/结果页入口 |
| 记忆与镜像沉淀 | 已完成阶段结果、检索计划、摘要候选 | `memory_candidate`、`memory_summary`、镜像引用关系 | 后续检索命中、任务详情中的记忆摘要 |
| 结果审查与 Hooks | 执行结果、工具调用记录、交付对象 | 审查结论、hook 命中、结构化失败原因 | 继续交付、重试、HITL 或阻断 |
| Trace / Eval | 模型调用、tool_call、输出摘要、资产命中 | `trace_record`、`eval_snapshot`、review result | 调试视图、loop 诊断、评估快照 |
| Doom Loop / HITL | 重复错误、无进展调用、审查失败 | Doom Loop 命中、人工复核 payload | `blocked`、重规划、人工继续执行 |
| 审计与恢复 | 高风险执行前后、恢复请求 | `audit_record`、`recovery_point`、恢复结果 | 安全摘要、恢复入口、恢复结果回流 |
| 成本与边界策略 | 预算设置、workspace/path policy、worker 权限 | budget downgrade decision、policy deny reason | 执行降级、策略拦截、正式错误码 |

### 输入
- 任务编排与运行层的过程对象；
- worker 和工具输出；
- 风险配置、预算、平台边界。

### 输出
- 审查结果；
- 授权请求；
- 正式交付对象；
- 记忆与镜像对象；
- 审计记录；
- 恢复点；
- 熔断或升级指令；
- Trace / Eval 记录。

### 上下游关系
- 上游来自任务编排与运行层提交的运行结果、拟执行动作、停止原因、工具输出和恢复请求。
- 下游一方面落到存储与执行边界，另一方面通过本地接入层把授权请求、结果摘要和恢复结果回流给前端。
- 该层既参与执行前拦截，也参与执行后收敛，不能只在出错后补日志。

### 异常处理
- 审查失败：回退到用户确认、降级或 HITL；
- 交付构建失败：回退到保底交付出口，并保留失败说明；
- 审计失败：记录错误但不能静默丢失高风险动作；
- 熔断触发：必须生成结构化失败结果。

### 联调重点
- 风险、交付、记忆、恢复、Trace 是否都能回到主链路；
- 是否存在“动作做了但没进入审计”的空洞；
- 是否存在“错误发生了但前端看不到正式对象”的问题。

### 实现边界
- `risk.Service` 只负责输出可测试的风险判断结果；等待授权、继续执行、终止收敛和恢复回流仍由编排层与运行态接管。
- `delivery.Service` 与记忆沉淀相关写入在逻辑分层上归入治理与交付层；任务编排与运行层只负责挂计划和触发交接，不拥有最终发布语义。
- `checkpoint` 模块定位为恢复点能力的最小收口层，不独占完整回滚编排；恢复结果必须重新并入正式任务主链。

### 4.4.1 风险评估引擎

#### 输入维度
- 动作类型；
- 目标范围；
- 是否跨工作区；
- 是否可逆；
- 是否涉及凭据/金钱/身份；
- 是否需要联网/下载/安装；
- 是否需要容器执行。

#### 输出
- `risk_level`
- `approval_request`
- `impact_scope`
- 是否需要恢复点。

#### 处理主线
1. 读取动作类型、目标对象、工作区范围和副作用类型。
2. 判断是否允许直接执行、是否必须授权、是否必须先建恢复点。
3. 生成 `impact_scope` 与待授权说明。
4. 把判断结果回交运行层，由运行层决定等待授权、继续执行或阻断。

### 4.4.2 结果审查引擎

#### 职责
- Linter / CI 反馈收集；
- Test Harness 结果汇总；
- 结构化 Schema 校验；
- 架构意图一致性检查；
- 文档与代码语义准确性审查；
- 输出格式、引用与交付前检查。

#### 输出
- 审查通过 / 不通过；
- 结构化失败原因；
- 需要人工升级或重试的建议。

#### 处理主线
1. 收集执行结果、工具调用记录和交付对象。
2. 运行 lint、schema、格式、引用和语义一致性检查。
3. 输出通过、不通过或需要人工复核的结构化结论。
4. 把结论回交治理与交付层决定继续交付、重规划或 HITL。

### 4.4.3 Hooks 引擎

#### 职责
- `pre_plan`；
- `pre_tool_use`；
- `post_tool_use`；
- `pre_delivery`；
- `post_delivery`；
- `stop`；
- 安全检查、日志埋点、格式规范化与总结逻辑挂载。

#### 处理主线
1. 在规划、工具调用、交付和停止边界上挂接规则。
2. 输出拦截、放行、补写元数据或格式修正结果。
3. 不直接推进任务状态，而是把 hook 结果交回运行层和治理层收敛。

### 4.4.4 Trace / Eval 引擎

#### 职责
- 模型输入/输出、延迟、成本记录；
- Tool 调用序列与结果记录；
- Planning loop 轮次记录；
- Skill / Blueprint / Prompt 命中记录；
- 正确率、完成率、token 效率与违规次数评估；
- 回放与对比实验基础数据沉淀。

实现约束：

- Trace / Eval 先通过 `trace_records / eval_snapshots` 落盘，不绕过现有稳定 RPC 直接发明新的前端正式对象；
- Trace 必须优先记录模型输入/输出摘要、loop round、tool 调用、latency、cost 和 review 结果，而不是只保留最近一次字符串日志；
- Trace / Eval 必须记录当前 `task / run` 实际命中的 `Skill / Blueprint / Prompt / Plugin` 资产版本，便于回放和问题定位；
- Eval Snapshot 先作为后端质量/回放真源，前端展示与协议冻结应通过稳定协议真源统一收口。

#### 处理主线
1. 收集本轮执行的模型输入摘要、输出摘要、tool calls、资产命中和 token/cost。
2. 生成 `trace_record` 与 `eval_snapshot`。
3. 检测 Doom Loop 和人工复核触发条件。
4. 将结果写入存储，并把 review/eval 结论回交治理层。

### 4.4.5 Doom Loop 检测与熔断引擎

#### 职责
- 相同错误连续出现检测；
- 相同文件短时反复修改检测；
- Tool 序列高度重复检测；
- 输出无实质变化检测；
- 熔断、换路、重规划与人工升级触发。

实现约束：

- Doom Loop 检测先在后端执行链路中产出结构化命中结果，并能改变主链路状态；
- 当前阶段允许通过 `trace_records.review_result`、Eval 状态和运行态 blocked task 承接结果，但不直接新增前端协议对象；
- 规则应优先覆盖重复调用签名和重复无进展错误等高确定性命中。

#### 处理主线
1. 检查重复错误、重复调用签名、重复修改和无进展输出。
2. 命中后判断应重规划还是升级人工复核。
3. 输出结构化命中原因，并通过运行层把任务推进到重规划或 blocked 状态。

### 4.4.6 Entropy Cleanup 引擎

#### 职责
- 临时文件与测试产物清理；
- 过期 memory 条目清理；
- 冗余上下文摘要清理；
- 孤立 TODO / FIXME 标记收敛；
- 废弃产物与无用分支状态回收。

#### 处理主线
1. 在任务完成、失败或阶段结束后检查临时产物与冗余上下文。
2. 只清理不再影响正式交付和恢复链的临时对象。
3. 把清理结果回写为审计或 Trace 记录，保证可追踪。

### 4.4.7 Human-in-the-loop 升级引擎

#### 职责
- 高风险操作前确认；
- 低置信度结果升级人工介入；
- Doom Loop 命中后人工检查；
- 需求模糊与架构冲突场景人工决策；
- 结构化失败报告与回滚确认。

实现约束：

- Human-in-the-loop 升级先形成结构化 escalation payload，并把任务推进到可持续恢复的 blocked / pending 承接状态；
- escalation payload 可先挂在运行态 pending execution 侧，正式协议暴露方式仍以稳定协议真源为准；
- 人工升级必须与 Trace / Eval / Doom Loop 命中保持稳定引用关系，避免只留下孤立文案。

#### 处理主线
1. 接收审查失败、Doom Loop 命中或高风险确认请求。
2. 生成结构化 escalation payload，包括原因、建议动作、复核说明和可选 corrected intent。
3. 通过运行层把任务推进到 `blocked` 或重新确认状态。
4. 在人工批准或要求重规划后重新并入任务主链。

### 4.4.8 审计与追踪引擎

#### 职责
- 文件操作记录；
- 网页操作记录；
- 命令操作记录；
- 系统动作记录；
- 错误日志；
- Token 日志；
- 费用日志。

#### 处理主线
1. 在高风险动作执行前后收集标准审计字段。
2. 把工具调用、授权结果、恢复结果和预算事件转成统一审计记录。
3. 为安全摘要、恢复入口和问题排查提供正式读模型基础。

### 4.4.9 恢复与回滚引擎

#### 职责
- 任务工作区级回滚；
- checkpoint 恢复；
- diff/sync plan 展示；
- 容器执行失败后的恢复策略。

#### 处理主线
1. 在需要恢复点的动作前准备 checkpoint。
2. 在失败、中断或显式恢复请求时执行恢复策略。
3. 生成恢复结果、安全摘要和后续续跑信号。
4. 把恢复结果重新并入运行层，而不是在治理层内部结束。

### 4.4.10 成本治理引擎

#### 职责
- 输入/输出 Token 统计；
- 模型配置与预算策略；
- 降级执行；
- 熔断与预算提醒。

#### 实现约束
- `budget_auto_downgrade` 已进入 Harness 主链路：编排层在执行前依据 token/cost、provider 可用性和 failure signal window 评估预算策略。
- 执行层在模型或 provider 失败后会转入 lightweight delivery fallback，并对高成本工具类别执行阻断。
- 命中结果统一回流到 audit / event / trace 链路，而不是只停留在设置项展示。

#### 处理主线
1. 在执行前计算当前预算策略、provider 可用性和高成本能力风险。
2. 必要时生成 downgrade decision，并修改执行请求。
3. 在命中、失败和恢复时把预算事件写入 audit / event / trace。

### 4.4.11 边界与策略引擎

#### 职责
- workspace 前缀校验；
- 命令白名单；
- 网络代理与外连策略；
- sidecar / worker / plugin 权限边界；
- 插件权限显式授权。

#### 处理主线
1. 在执行前校验路径、命令、网络和插件权限边界。
2. 生成 deny / allow / approval required 三类策略结果。
3. 将结果回交风险评估与运行层，而不是直接跳过主链执行。

---

## 4.5 能力与存储层模块（平台、执行与持久化适配子层）

### 模块定位
本章承接架构总览中的“能力与存储层”里偏平台与真源的一半：负责对象仓储、索引、Workspace / Artifact、路径策略、机密存储，以及平台与执行后端抽象。它解决的是“能力和数据怎样被稳定托底”，而不是“任务该怎么编排”。

### 平台、执行与持久化支撑关系图

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '16px'}}}%%
flowchart TB
    RUN[运行控制器]
    GOV[治理与交付层]
    CAP[能力接入与检索子层]

    subgraph B5[能力与存储层<br/>平台、执行与持久化适配子层]
        direction TB
        STORE[对象仓储与持久化<br/>（SQLite + WAL）]
        INDEX[本地检索索引]
        WORKSPACE[工作区与产物存储<br/>（Workspace / Artifact）]
        SECRET[路径策略与机密存储<br/>（Stronghold）]
        FSABS[文件系统抽象]
        OSABS[系统能力抽象]
        EXECABS[执行后端适配]
        DOCKER[Docker 沙箱<br/>（Docker Sandbox）]
        WIN[Windows 实现]
    end

    RUN --> STORE
    GOV --> STORE
    GOV --> INDEX
    GOV --> WORKSPACE
    CAP --> EXECABS
    CAP --> INDEX
    RUN --> SECRET
    FSABS --> WIN
    OSABS --> WIN
    EXECABS --> DOCKER

    STORE -.->|任务 / 运行 / 事件 / 交付查询| RUN
    INDEX -.->|检索命中 / 索引查找| CAP
    WORKSPACE -.->|产物路径 / 文档路径| GOV
    SECRET -.->|路径策略 / 机密读取| CAP
```

### 组成
- 对象仓储与运行态持久化
- 本地索引与检索
- Workspace / Artifact 存储
- 路径策略与机密存储
- 文件系统抽象层
- 系统能力抽象层
- 执行后端适配层

### 子模块说明
- **对象仓储与运行态持久化**：承接 `task / run / event / tool_call / delivery_result / approval_request` 等正式对象和兼容对象的持久化。
- **本地索引与检索**：承接 FTS5、向量检索和后续结构化召回索引。
- **Workspace / Artifact 存储**：承接工作区文档、交付产物和证据文件。
- **路径策略与机密存储**：负责 workspace 边界、敏感路径校验和 Stronghold 等机密存储。
- **文件系统抽象层**：统一约束工作区、产物目录和路径策略下的文件读写。
- **系统能力抽象层**：统一承接窗口、通知、系统动作、环境元数据等平台能力。
- **执行后端适配层**：统一承接 Docker sandbox、受控宿主执行和未来跨平台执行后端。

### 层内处理细节

| 子模块 | 处理入口 | 关键中间产物 | 返回与反馈 |
| --- | --- | --- | --- |
| 对象仓储与运行态持久化 | 运行控制器和治理层提交的正式对象写入计划 | `TaskRunRecord`、`DeliveryResultRecord`、`CitationRecord` 等持久化记录 | 查询结果、运行态恢复、任务详情真源 |
| 本地索引与检索 | 记忆沉淀、文本索引写入、召回请求 | FTS5 条目、向量索引、结构化键值命中 | `retrieval_hit`、检索摘要、排序结果 |
| Workspace / Artifact 存储 | 正式交付、屏幕证据、文档写入 | artifact 元数据、路径映射、交付 payload | 文档路径、artifact 列表、文件定位入口 |
| 路径策略与机密存储 | 高风险执行前的路径检查和密钥读取 | workspace allowlist、secret handle、敏感路径决策 | allow / deny、配置读取、统一错误码 |
| 文件系统抽象层 | 文件读写、路径归一化、artifact 落盘 | 规范化路径、文件句柄、workspace 边界校验 | 安全文件操作结果 |
| 系统能力抽象层 | 通知、快捷键、剪贴板、屏幕授权、sidecar 生命周期 | 平台调用请求、环境元数据 | 宿主能力结果或平台级降级信号 |
| 执行后端适配层 | 工具与执行请求 | sandbox profile、resource limit、backend routing | 受控执行结果、执行环境错误 |

### 输入
- 上层标准接口请求；
- 平台资源和环境信息。

### 输出
- 统一路径、文件读写、窗口与通知能力；
- 容器或执行后端路由结果。

### 上下游关系
- 上游来自能力与存储层的能力接入与检索子层，以及治理与交付层的执行请求、路径策略请求和平台能力请求。
- 下游是 Windows 当前实现、Docker sandbox 和未来跨平台保留接口。
- 该层对上只暴露抽象能力，不把平台细节、硬编码路径或宿主实现差异泄漏给上层。

### 异常处理
- 平台能力不可用：提供明确降级结果；
- 路径越界：直接阻断并记录；
- 执行后端异常：回退或创建恢复点。

### 联调重点
- 业务层是否真的不感知平台路径和平台实现名；
- 工作区边界是否在所有写入入口都生效；
- Docker / SandboxProfile / 资源限制是否可观测、可恢复。

### 4.5.1 文件系统抽象层

#### 职责
- 路径归一化；
- Workspace 边界校验；
- 跨平台路径读写；
- Artifact 文件落盘；
- 不暴露平台专属路径实现。

#### 处理主线
1. 统一把上层目标路径归一为受控路径对象。
2. 执行 workspace 边界校验和 artifact 目录映射。
3. 提供读写、落盘和文件存在性检查结果给上层。

### 4.5.2 系统能力抽象层

#### 职责
- 通知；
- 快捷键；
- 剪贴板；
- 屏幕授权；
- 外部命令启动；
- sidecar 生命周期管理。

#### 处理主线
1. 接收上层平台动作请求。
2. 选择 Windows 当前实现或未来跨平台实现。
3. 返回统一能力结果，而不是暴露宿主私有 API 细节。

### 4.5.3 执行后端适配层

#### 职责
- Docker；
- SandboxProfile；
- ResourceLimit；
- Remote Backend；
- Windows 当前实现优先，其他平台保留接口。

#### 处理主线
1. 根据工具类型、风险等级和策略选择执行后端。
2. 附加 sandbox profile、resource limit 和工作目录约束。
3. 返回统一执行结果和失败原因，供运行层和治理层继续收敛。

---

## 5. 核心流程与模块展开

### 5.0 核心链路阅读方式

本章承接架构设计稿里的“核心链路设计”视角，但不直接重复架构文档里的整章叙述，而是把系统级链路拆进当前模块文档已有的流程图中：

- **标准任务创建与执行链路**：主要看 5.1、5.2，以及 6.1 的功能时序；重点关注输入如何进入 `task`、上下文如何归一、意图如何建议、执行如何进入主状态机。
- **授权、恢复点与恢复链路**：主要看 5.3 与 6.2；重点关注风险判断、授权确认、恢复点、恢复结果和审计如何并入正式对象链。
- **会话续接、串行与人工控制链路**：当前分散在 4.2、6.1、6.9 与 6.10；重点关注同一 `session` 下的排队、续接、追加要求、暂停/恢复/取消。
- **推荐与巡检升级链路**：主要看 5.6、5.10，以及 3.7.3 / 3.7.8；重点关注事项、推荐和感知信号如何在不污染主状态机的前提下升级为正式任务。
- **结果回流与任务详情刷新链路**：主要看 5.7、5.11 与 6.11；重点关注 `task.updated / delivery.ready / approval.pending / loop.*` 如何驱动任务详情、安全摘要和轻反馈统一刷新。

阅读约束：

- 本章的流程图优先解释“主链怎样流转”，第 6 章再解释“具体功能怎样实现”。
- 同一条语义只保留一份主说明：系统级约束放在本章，功能级交互细节放在第 6 章，不在两章重复展开同一段说明。

### 5.1 主动输入闭环图

```mermaid
flowchart TB
    A[用户触发
语音/悬停输入/选中文本/拖拽文件]
    B[前端事件归一与路由]
    C[提交输入与发起任务<br/>（agent.input.submit / agent.task.start）]
    D[后端上下文采集]
    E[上下文管理器组装上下文<br/>（Context Manager）]
    F[技能与蓝图路由<br/>（Skill / Blueprint）]
    G[提示词组装 / AGENTS 规则 / 架构约束注入<br/>（Prompt）]
    H[任务对象识别]
    I[意图分析与规划]
    J{是否需要确认}
    K[气泡确认/修正]
    L[正式创建或更新任务]
    M[风险评估]
    N{风险等级}
    O[直接执行]
    P[挂起等待授权]
    Q[受控能力执行<br/>（工具 / 子代理 / 工作者 / 边车）]
    R[结果校验<br/>（Linter / CI / Test Harness）]
    S[结果审查与钩子<br/>（Agent Review / Hooks）]
    T{结果类型判断}
    U[气泡返回短结果]
    V[生成工作区文档并打开<br/>（workspace）]
    W[生成结果页 / 结构化产物<br/>（artifact）]
    X[任务状态回写]
    Y[追踪与评估<br/>（Trace / Eval）]
    Z[记忆沉淀 / 熵清理 / 恢复点更新]

    A --> B --> C --> D --> E --> F --> G --> H --> I --> J
    J -- 是 --> K --> L
    J -- 否 --> L
    L --> M --> N
    N -- 绿 --> O
    N -- 黄/红 --> P --> O
    O --> Q --> R --> S --> T
    T -- 短结果 --> U --> X
    T -- 长文本 --> V --> X
    T -- 结构化 --> W --> X
    X --> Y --> Z
```

### 5.2 前馈决策展开图

```mermaid
flowchart TB
    A[任务进入后端主编排]
    B[任务编排器绑定任务 / 会话 / 追踪锚点]
    C[上下文归一与准备器生成任务上下文快照<br/>（TaskContextSnapshot）]
    D[挂接记忆读取计划 / AGENTS 规则 / 资产引用]
    E[入口判断与规划器生成入口建议<br/>（Suggestion）]
    F{是否需要确认或补充输入}
    G[返回气泡确认 / 等待补充输入<br/>（waiting_input）]
    H[折叠为建任务输入<br/>（CreateTaskInput）]
    I[运行控制器创建运行记录<br/>（TaskRecord）]
    J[资产路由与 Prompt 组装]
    K[生成执行请求<br/>（execution.Request）]
    L[进入受控执行循环 / 规划循环<br/>（Planning Loop）]
    M[执行结果与循环事件<br/>（execution.Result / loop.*）]

    A --> B --> C --> D --> E --> F
    F -- 是 --> G
    F -- 否 --> H --> I --> J --> K --> L --> M
    M -. 结果与停止原因回流 .-> I
    M -. trace / eval / review .-> D
```

设计要点：

- `TaskContextSnapshot` 是入口阶段的统一上下文快照，不等同于最终 Prompt。
- `Suggestion` 只决定“怎样入链”，例如 `Intent`、`RequiresConfirm`、`TaskTitle` 和交付偏好，不直接替代执行期 Planner。
- 真正的 ReAct / Agent Loop 发生在 `execution.Request` 已经成形并进入受控执行循环之后。

### 5.3 风险执行与回滚图

```mermaid
flowchart TB
    A[任务提交高风险动作]
    B[风险引擎评估]
    C[创建恢复点<br/>（checkpoint）]
    D[展示影响范围 / 风险等级 / 恢复点]
    E{用户是否确认}
    F[拒绝执行并保留记录]
    G[进入 Docker 沙箱]
    H{执行是否成功}
    I[更新任务成果 / 状态]
    J[写入审计日志]
    K[发起恢复 / 回滚]
    L[展示恢复结果与影响面]

    A --> B --> C --> D --> E
    E -- 否 --> F
    E -- 是 --> G --> H
    H -- 成功 --> I --> J
    H -- 失败/中断 --> K --> L --> J
```

### 5.4 反馈闭环与熔断图

```mermaid
flowchart TB
    A[执行结果 / 工具调用 / 循环事件<br/>（execution.Result / tool_call / loop.*）]
    B[运行控制器记录最新事件 / 最新工具调用<br/>（LatestEvent / LatestToolCall）]
    C[治理与交付层执行审查 / 钩子 / 追踪]
    D{是否通过}
    E[正式交付协调]
    F[完成任务 / 更新展示<br/>（runengine.CompleteTask / SetPresentation）]
    G[写入任务更新 / 交付就绪通知<br/>（task.updated / delivery.ready）]
    H[本地接入层回流任务详情与轻反馈]
    I[循环异常 / 人工介入评估<br/>（Doom Loop / HITL）]
    J{重规划还是人工复核}
    K[重开意图确认 / 重开等待输入]
    L[阻断并进入人工复核 / 结构化失败]

    A --> B --> C --> D
    D -- 是 --> E --> F --> G --> H
    D -- 否 --> I --> J
    J -- 重规划 --> K --> G --> H
    J -- 人工复核 --> L --> G --> H
    C -. 审查摘要 / 追踪摘要 / 评估摘要 .-> H
```

设计要点：

- 反馈闭环不是“执行完再补日志”，而是执行结果先回到运行控制器，再进入治理与交付层决定继续、重规划、人工复核或正式交付。
- `task.updated` 和 `delivery.ready` 是前端看到反馈链的正式出口；它们必须与任务详情查询口径一致。

### 5.5 记忆写入与检索图

```mermaid
flowchart TB
    A[任务完成 / 阶段完成]
    B[生成阶段摘要]
    C[记忆候选抽取]
    D[规则过滤 / 审查筛选]
    E{是否满足写入条件}
    F[丢弃或仅保留运行态引用]
    G[写入记忆摘要<br/>（MemorySummary）]
    H[写入全文索引<br/>（FTS5）]
    I[生成向量并写入语义索引<br/>（sqlite-vec）]
    J[建立运行与记忆引用关系<br/>（Run / Memory）]
    K[记录追踪与评估摘要<br/>（Trace / Eval）]
    L[后续任务触发检索]
    M[上下文管理器发起召回请求<br/>（Context Manager）]
    N[FTS5 关键词召回]
    O[sqlite-vec 语义召回]
    P[结构化 KV / 精确匹配]
    Q[去重 / 排序 / 摘要回填]
    R[返回记忆命中结果]

    A --> B --> C --> D --> E
    E -- 否 --> F
    E -- 是 --> G --> H --> I --> J --> K
    L --> M
    M --> N
    M --> O
    M --> P
    N --> Q
    O --> Q
    P --> Q --> R
```

---

### 5.6 任务巡检转任务图

```mermaid
flowchart TB
    A[任务文件夹 / 心跳任务 / 定时任务<br/>（Heartbeat / Cron）]
    B[任务文件监听器]
    C[Markdown 文档解析]
    D[任务结构抽取
标题 / 日期 / 状态 / 标签 / 重复规则]
    E[巡检规则引擎]
    F{任务分类}
    G[近期要做]
    H[后续安排]
    I[重复事项]
    J[已结束]
    K{是否需要 Agent 接手}
    L[提醒 / 建议 / 打开资料]
    M[升级为正式任务<br/>（agent.notepad.convert_to_task）]
    N[写入任务状态模块]
    O[建立来源关联]
    P[任务执行 / 成果沉淀 / 安全治理]

    A --> B --> C --> D --> E --> F
    F --> G --> K
    F --> H
    F --> I
    F --> J
    K -- 否 --> L
    K -- 是 --> M --> N --> O --> P
```

### 5.7 仪表盘订阅与任务视图刷新图

```mermaid
flowchart TB
    A[后端任务状态变化]
    B[生成任务更新 / 交付就绪 / 待授权通知]
    C[协议通知与订阅<br/>（JSON-RPC Notification / Subscription）]
    D[前端协议适配层接收事件]
    E[前端事件总线分发]
    F[任务列表刷新]
    G[任务详情刷新]
    H[安全摘要刷新]
    I[插件面板刷新]
    J[气泡短反馈刷新]

    A --> B --> C --> D --> E
    E --> F
    E --> G
    E --> H
    E --> I
    E --> J
```

### 5.8 结果审查、Trace 与熔断闭环说明

`5.4 反馈闭环与熔断图` 已经覆盖了结果审查、Hooks、Trace / Eval、Doom Loop、Human-in-the-loop 与 Entropy Cleanup 的主链闭环。这里不再重复放同一张图，只补充当前模块文档需要强调的实现侧要点：

- `Trace / Eval` 不只服务离线排障，还会回流任务详情、安全摘要和调试视图。
- `loop.*` 生命周期事件、工具调用记录、审查结论和失败原因，必须与 `task / run / step` 建立稳定引用关系。
- 当结果未通过审查或命中 Doom Loop 时，系统要么重规划，要么升级到 Human-in-the-loop，而不是静默吞掉失败。


### 5.9 长按语音与滑动控制图

```mermaid
flowchart TB
    A[用户长按悬浮球]
    B[进入收音态]
    C{手势方向}
    D[上滑锁定通话]
    E[下滑取消本次收音]
    F[松开提交本轮语音]
    G[保持持续通话态]
    H[不进入处理链路]
    I[转写语音并进入输入提交流程<br/>（agent.input.submit）]
    J[根据上下文进入意图确认或任务执行]

    A --> B --> C
    C -- 上滑 --> D --> G
    C -- 下滑 --> E --> H
    C -- 松开 --> F --> I --> J
```

### 5.10 主动推荐触发与轻提示图

```mermaid
flowchart TB
    A[页面停留/复制/选中/错误/切换]
    B[上下文感知模块]
    C[行为与机会识别]
    D{是否满足触发规则}
    E[静默不打扰]
    F[调用推荐获取接口<br/>（agent.recommendation.get）]
    G[生成推荐文案与建议动作]
    H[气泡轻提示 / 悬停输入承接]
    I[用户忽略 / 反馈 / 点击进入任务]
    J[提交推荐反馈<br/>（agent.recommendation.feedback.submit）]

    A --> B --> C --> D
    D -- 否 --> E
    D -- 是 --> F --> G --> H --> I --> J
```

### 5.11 结果分流与失败中断交付图

```mermaid
flowchart TB
    A[任务执行结束或阶段完成]
    B[治理与交付层判断交付形态]
    C[构建正式交付结果 / 产物 / 引用<br/>（DeliveryResult / Artifact / Citation）]
    D[运行控制器写回展示结果与完成态<br/>（presentation）]
    E[通知队列写入任务更新 / 交付就绪]
    F[本地接入层回流]
    G{交付形态}
    H[气泡短交付]
    I[工作区文档交付]
    J[结果页 / 浏览器交付]
    K[文件交付 / 定位到文件夹]
    L[任务详情交付]
    M[失败 / 中断说明]
    N[安全摘要 / 恢复点 / 重试入口]

    A --> B --> C --> D --> E --> F --> G
    G -- 短结果 --> H
    G -- 长文本 --> I
    G -- 结构化结果 --> J
    G -- 文件产物 --> K
    G -- 连续任务 --> L
    A -. 执行失败 / 被阻断 .-> M --> N --> F
```

设计要点：

- 结果先形成正式交付对象，再决定前端展示位置；前端不应自己拼接“临时结果”。
- 失败与中断也必须走正式反馈链，至少要给出卡点说明、是否可恢复、以及恢复点或任务详情入口。

### 5.12 本章与后续章节的边界说明

- 第 5 章保留系统级流程图，重点回答“主链路怎样流转、前馈怎样收敛、风险怎样治理、反馈怎样闭环、记忆怎样沉淀与召回”。
- 第 6 章保留功能模块级时序和状态图，重点回答“具体某个功能如何实现、模块之间如何交互、异常时怎么降级或回滚”。
- 因此，第 5 章中的图和说明偏系统视角；第 6 章中的图和说明偏功能视角。两章之间允许存在引用关系，但不再重复解释同一层级的语义。

### 5.13 模块联动约束

- 前馈决策结果必须显式传入编排层和 Trace 记录；
- 反馈闭环的失败结果必须能回流到状态机或人工升级流程；
- 子任务与插件的输出同样要走审查、审计和交付对象链；
- 任何前馈命中和反馈命中都不允许只存在日志中而无结构化记录；
- 前端局部状态、后端正式状态、数据库对象和交付对象之间必须有单向映射，不允许相互混写；
- 模块联调时必须优先保证主链路，再补充扩展能力链路。


## 6. 关键时序图与实现说明

### 6.1 近场对象承接时序：文本选中 / 文件拖拽 -> 确认或直接执行

**链路目标**：把文本选中、文件拖拽等“近场对象承接”转换成正式任务。  
**主要模块参与**：表现层、应用编排层、协议适配层、本地接入层、上下文归一与准备器、入口判断与规划器、运行控制器、风险评估与授权承接、正式结果交付协调。
**关键结果**：形成 `task`，必要时生成 `bubble_message` 进行意图确认，最终得到 `delivery_result`。  
**异常分支**：高风险动作进入授权链路；对象失效则回退到待机或确认失败态。  
**实现说明**：此链路是所有近场承接动作的统一模板，文本选中、拖拽文件、错误信息承接和主动机会承接都应落在同一确认 / 执行骨架上；带明确说明的文件任务可以直接进入治理与执行，裸文件仍进入确认或补充输入，带任务特定证据的结构化补充证据可续接到正在等待用户输入的原 task。

```mermaid
sequenceDiagram
    participant User as 用户
    participant UI as 桌面前端（Tauri）
    participant RPC as 协议层（JSON-RPC）
    participant API as 后端服务（Go）
    participant SAFE as 风险引擎
    participant TOOL as 受控能力层（工具 / 工作者 / 边车）
    participant DEL as 结果交付
    participant DASH as 仪表盘

    User->>UI: 选中文本 / 拖拽文件 / 触发悬浮球
    UI->>RPC: 发起任务（agent.task.start）
    RPC->>API: 协议请求
    API->>API: 采集上下文并识别意图
    API-->>RPC: 返回任务 / 气泡消息 / 正式交付
    RPC-->>UI: 协议响应
    UI-->>User: 气泡展示意图判断，允许修正
    User->>UI: 确认或修正意图
    UI->>RPC: 确认任务（agent.task.confirm）
    RPC->>API: 协议请求
    API->>SAFE: 风险评估
    SAFE-->>API: 返回绿/黄/红等级
    alt 黄/红
        API-->>RPC: 待授权通知（approval.pending）
        RPC-->>UI: 通知授权请求
        UI-->>User: 展示授权确认
        User->>UI: 允许本次 / 拒绝
        UI->>RPC: 提交授权响应（agent.security.respond）
        RPC->>API: 协议请求
    end
    API->>TOOL: 执行工具调用
    TOOL-->>API: 返回结果
    API->>DEL: 结果交付
    DEL-->>RPC: 任务更新 / 交付就绪
    RPC-->>UI: 推送状态与结果
    DEL-->>DASH: 更新任务状态、成果、日志、恢复点
```

### 6.2 高风险治理时序：授权 -> 执行 -> 回滚

**链路目标**：保证高风险动作不会绕过授权、审计和恢复点。  
**主要模块参与**：应用编排层、本地接入层、风险评估引擎、恢复与回滚引擎、执行后端适配层、审计与追踪引擎。
**关键结果**：在执行前形成 `approval_request` 和 `recovery_point`，在执行后形成 `authorization_record` 和 `audit_record`。  
**异常分支**：执行失败或用户中断时，必须显式回滚或保留可恢复信息。  
**实现说明**：此链路是治理链的最小闭环，凡是跨工作区、命令执行、联网下载、删除/覆盖等动作，都必须从这里经过。

设计约束：对于 `exec_command` 这类高风险执行，默认应优先进入 Docker Sandbox，并且支持上下文中断后的容器清理；仅在 Windows shell 命令入口上允许走受控宿主路径，其他失败情形不能静默回退到宿主直接执行。

```mermaid
sequenceDiagram
    participant User as 用户
    participant RPC as 协议层（JSON-RPC）
    participant API as 后端服务（Go）
    participant SAFE as 风险引擎
    participant SNAP as 恢复点服务
    participant EXEC as 外部执行后端
    participant AUDIT as 审计日志
    participant UI as 桌面前端（Tauri）

    UI->>RPC: 确认任务（agent.task.confirm）
    RPC->>API: 协议请求
    API->>SAFE: 提交高风险动作计划
    SAFE->>SNAP: 创建恢复点
    SNAP-->>SAFE: 返回恢复点标识
    SAFE-->>API: 风险结果
    API-->>RPC: 待授权通知（approval.pending）
    RPC-->>UI: 展示风险等级、影响范围、恢复点
    UI-->>User: 等待人工确认
    User->>UI: 允许本次
    UI->>RPC: 提交授权响应（agent.security.respond）
    RPC->>API: 协议请求
    API->>EXEC: 在 Docker 沙箱中执行
    EXEC-->>API: 返回执行结果
    API->>AUDIT: 写入命令/文件/网页/系统动作日志
    alt 执行成功
        API-->>RPC: 任务更新 / 交付就绪
        RPC-->>UI: 更新任务状态与成果
    else 执行失败或用户中断
        API->>SNAP: 发起恢复/回滚
        SNAP-->>API: 恢复完成
        API-->>RPC: 任务更新
        RPC-->>UI: 展示恢复结果与影响面
    end
```

### 6.3 记忆沉淀时序：写入 -> 检索

**链路目标**：把阶段结果沉淀为长期记忆，并在镜子视图或后续调用中进行召回。  
**主要模块参与**：任务运行时、记忆与镜像沉淀、SQLite FTS5、sqlite-vec、镜子相关接口。
**关键结果**：形成 `memory_summary / memory_candidate / retrieval_hit` 三类对象。  
**异常分支**：记忆写入失败不影响任务完成，但必须有 Trace 记录；召回失败时镜子视图可降级为空结果。  
**实现说明**：该链路强调记忆层与运行态主表分层，命中结果必须通过标准对象返回，不能直接改写 `task` 主表。

```mermaid
sequenceDiagram
    participant TASK as 任务运行时
    participant MEM as 记忆内核
    participant FTS as 全文索引（SQLite FTS5）
    participant VEC as 语义索引（sqlite-vec）
    participant RPC as 协议层（JSON-RPC）
    participant UI as 前端仪表盘

    TASK->>MEM: 提交阶段结果 / 摘要 / 上下文引用
    MEM->>MEM: 判断是否写入长期记忆
    alt 满足写入条件
        MEM->>FTS: 写入文本索引
        MEM->>VEC: 写入向量与元数据
        MEM->>MEM: 建立任务与记忆引用
    else 不满足写入条件
        MEM->>MEM: 仅保留运行态引用
    end
    UI->>RPC: 获取镜子概览（agent.mirror.overview.get）
    RPC->>MEM: 发起检索
    MEM->>FTS: 关键词召回
    MEM->>VEC: 向量召回
    FTS-->>MEM: 文本候选
    VEC-->>MEM: 向量候选
    MEM-->>RPC: 去重/排序后的命中结果
    RPC-->>UI: 返回镜子概览与记忆命中摘要
```

### 6.4 插件运行态时序：执行 -> 事件流 -> 仪表盘

**链路目标**：把插件运行态、指标和产物纳入统一事件流和仪表盘。  
**主要模块参与**：插件运行态协调器、事件流、本地接入层、仪表盘模块。
**关键结果**：插件运行不直接暴露给前端，而是通过 `plugin.updated` 等事件与 `agent.plugin.runtime.list / agent.plugin.list / agent.plugin.detail.get` 查询结果统一展示。  
**实现说明**：插件视图不能成为独立协议体系，仍必须通过标准事件和标准对象链回流；详情页和列表页消费的是后端聚合后的正式读模型，而不是裸 worker / sidecar 缓存。

```mermaid
sequenceDiagram
    participant PM as 插件管理器
    participant PLUG as 插件工作器
    participant EVT as 事件流（Event Stream）
    participant RPC as 协议层（JSON-RPC）
    participant UI as 仪表盘

    PM->>PLUG: 启动插件进程
    PLUG-->>PM: 注册能力 / 版本 / 权限信息
    PM->>EVT: 写入首个插件状态快照（plugin.updated）
    loop 运行期间
        PLUG-->>PM: 指标 / 心跳 / 结果摘要 / 错误
        PM->>EVT: 写入插件状态 / 指标 / 任务事件
        EVT-->>RPC: 事件订阅推送
        RPC-->>UI: 插件状态与指标更新
    end
    UI->>RPC: 查询模块 / 插件列表 / 插件详情
    RPC->>PM: 获取运行态、插件目录与最近产物
    PM-->>RPC: 返回聚合数据
    RPC-->>UI: 展示插件面板
```

### 6.5 启动恢复时序：初始化 -> 首页可用

**链路目标**：在前端启动后尽快恢复上次状态，拉起任务概览、记忆索引和插件运行态。  
**关键结果**：`agent.dashboard.overview.get` 以正常响应返回首页可用所需的最小数据集合。  
**异常分支**：记忆或插件预热失败不能阻断首页可打开。  
**实现说明**：启动链路优先保证“首页可用”和“未完成任务可见”，而不是等所有后台能力全预热完成。

```mermaid
sequenceDiagram
    participant UI as 桌面前端（Tauri）
    participant RPC as 协议层（JSON-RPC）
    participant API as 后端服务（Go）
    participant DB as 数据仓储（SQLite）
    participant MEM as 记忆内核
    participant PM as 插件管理器

    UI->>RPC: 获取仪表盘概览（agent.dashboard.overview.get）
    RPC->>API: 初始化阶段首页概览请求
    API->>DB: 读取未完成任务 / 配置 / 审计索引
    API->>MEM: 预热常用记忆索引
    API->>PM: 加载插件注册表并恢复插件状态
    DB-->>API: 返回任务状态与配置
    MEM-->>API: 记忆索引就绪
    PM-->>API: 插件状态就绪
    API-->>RPC: 返回仪表盘概览结果
    RPC-->>UI: 返回首页摘要 / 未完成任务 / 插件状态 / 安全提醒
```

### 6.6 语音承接时序：长按 -> 锁定/取消 -> 提交

**链路目标**：将长按、上滑、下滑和松开等语音手势转换为标准任务输入。  
**关键结果**：形成语音会话状态、提交到 `agent.input.submit`，并回填气泡和任务对象。  
**重点约束**：上滑锁定和下滑取消属于前端局部状态，不直接映射正式业务状态。  
**实现说明**：长按语音是主入口之一，必须支持“锁定通话”和“打断补充”，但这些都应通过前端状态机协调，而不是发明新业务状态。

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 表现层
    participant A as 应用编排层
    participant S as 状态管理层
    participant V as 前端服务层
    participant I as 平台集成层

    U->>P: 左键长按悬浮球
    P->>S: 更新悬浮球状态=承接中
    P->>S: 更新语音状态=收音中
    P->>A: 通知发起语音协作
    A->>V: 调用语音服务，启动语音输入
    A->>V: 调用上下文服务，请求当前任务上下文

    alt 上滑锁定
        U->>P: 上滑
        P->>S: 更新语音状态=锁定通话
        P-->>U: 展现持续通话状态
    else 下滑取消
        U->>P: 下滑
        P->>S: 更新语音状态=已取消
        A-->>V: 取消语音请求
        P-->>U: 回退到待机状态
    else 松开结束本轮输入
        U->>P: 松开
        P->>S: 更新语音状态=输入结束
        A->>V: 提交语音内容与上下文
        V->>V: 语音理解与任务分析
        V-->>A: 返回理解结果与任务建议
        A->>S: 更新悬浮球状态=处理中
        A->>S: 更新当前任务对象状态
        A->>V: 调用任务服务，发起处理
        V-->>A: 返回结果
        A->>S: 更新悬浮球状态=完成
        A->>S: 更新气泡状态
        A->>P: 渲染气泡内容
        P-->>U: 展示状态、结果与下一步建议
    end

    opt 回应过程中被打断
        U->>P: 再次长按补充需求
        P->>S: 更新语音状态=再次收音
        A->>V: 追加语音内容并重新协调任务
    end
```

### 6.7 悬停轻承接时序：推荐 -> 输入 -> 处理

**链路目标**：在不强打扰用户的前提下，提供主动推荐和一句话补充入口。  
**关键结果**：可在不新建独立 RPC 的情况下，复用推荐和任务提交能力。  
**重点约束**：推荐失败应静默降级，不可影响用户继续单击或长按主链路。  
**实现说明**：推荐冷却、命中条件和轻量承接应全部由编排层与状态层协调，表现层只负责提示态。

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 表现层
    participant A as 应用编排层
    participant S as 状态管理层
    participant V as 前端服务层

    U->>P: 鼠标悬停悬浮球
    P->>A: 通知进入悬停检测
    A->>S: 读取悬浮球状态、冷却信息、当前任务对象状态

    alt 满足触发条件
        A->>V: 调用上下文服务，获取当前界面上下文
        A->>V: 调用推荐服务，生成推荐内容
        V-->>A: 返回推荐问题与建议动作
        A->>S: 更新悬浮球状态=可唤起
        A->>S: 更新气泡状态
        A->>S: 更新轻量输入状态=可编辑
        A->>P: 显示气泡内容
        A->>P: 显示轻量输入区
        P-->>U: 展示推荐内容与补充输入入口

        opt 用户补充一句话
            U->>P: 在轻量输入区输入需求
            P->>S: 更新轻量输入状态
            P->>A: 提交补充需求
            A->>V: 调用任务服务发起处理
            V-->>A: 返回处理结果
            A->>S: 更新气泡状态
            A->>P: 更新气泡内容
            P-->>U: 展示结果
        end
    else 不满足触发条件
        A->>S: 保持当前状态
        P-->>U: 不触发推荐
    end
```

### 6.8 机会对象承接时序：文本 / 文件 / 推荐 -> 确认或执行

**链路目标**：把文本、文件、推荐机会这些对象统一进入确认 / 执行骨架。
**关键结果**：需要确认时用 `bubble_message` 承接，确认后转入正式 `task` 执行；不需要确认且说明明确时直接进入治理与执行。
**重点约束**：文件解析、机会识别和对象识别不得维护各自独立的确认状态；裸文件和低置信度对象必须停在确认或补充输入。
**实现说明**：无论对象来源是选中文本、拖拽文件还是主动推荐，后续都应收敛到同一套确认与执行骨架；带任务特定证据的结构化补充证据可以续接待用户补充的原 task，但不能绕过确认门禁去自动执行。

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 表现层
    participant A as 应用编排层
    participant S as 状态管理层
    participant V as 前端服务层
    participant I as 平台集成层

    alt 文本选中
        U->>P: 选中一段文本
        P->>A: 通知识别到文本对象
        A->>S: 更新当前任务对象状态=文本
    else 文本拖拽
        U->>P: 将文本拖向悬浮球
        P->>A: 通知识别到拖拽文本对象
        A->>S: 更新当前任务对象状态=拖拽文本
    else 文件拖拽
        U->>I: 将文件拖入悬浮球区域
        I-->>P: 传入文件对象
        P->>A: 通知识别到文件对象
        A->>V: 调用文件服务，解析文件
        V-->>A: 返回文件摘要与类型信息
        A->>S: 更新当前任务对象状态=文件
    else 识别到协作机会
        A->>V: 调用上下文服务获取当前上下文
        A->>V: 调用推荐服务分析协作机会
        V-->>A: 返回可协作机会
        A->>S: 更新当前任务对象状态=协作机会
    end

    A->>S: 更新悬浮球状态=可操作提示态
    A->>P: 刷新悬浮球样式
    P-->>U: 提示可继续发起协作

    U->>P: 左键单击悬浮球
    P->>A: 触发统一承接流程
    A->>V: 调用上下文服务补充任务上下文
    A->>V: 分析用户可能意图
    V-->>A: 返回意图猜测与建议输出方式
    A->>S: 更新意图确认状态
    A->>S: 更新气泡状态
    A->>S: 更新轻量输入状态=可修正
    A->>P: 显示气泡内容
    A->>P: 显示轻量输入区
    P-->>U: 展示意图确认内容

    alt 用户确认当前意图
        U->>P: 点击确认
        P->>A: 提交确认
        A->>V: 调用任务服务发起处理
        V-->>A: 返回结果
        A->>S: 更新气泡状态
        A->>P: 更新气泡内容并触发结果分发
        P-->>U: 展示结果
    else 用户修正意图
        U->>P: 在轻量输入区修改意图
        P->>A: 提交修正后的意图
        A->>V: 按新意图发起处理
        V-->>A: 返回结果
        A->>S: 更新气泡状态
        A->>P: 更新气泡内容并触发结果分发
        P-->>U: 展示结果
    end
```

### 6.9 仪表盘打开时序：双击悬浮球 -> 首页加载

**链路目标**：通过双击快速进入低频工作台视图。  
**关键结果**：打开仪表盘窗口并装载任务、安全、镜子和设置摘要。  
**实现说明**：双击属于窗口和视图级动作，不直接创建新任务，但会触发多服务并发取数。

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 表现层
    participant A as 应用编排层
    participant S as 状态管理层
    participant V as 前端服务层
    participant I as 平台集成层

    U->>P: 双击悬浮球
    P->>A: 通知打开仪表盘
    A->>S: 更新仪表盘状态=打开中
    A->>I: 请求打开仪表盘窗口
    I-->>P: 仪表盘窗口已打开

    A->>V: 调用任务服务，获取任务摘要
    A->>V: 调用记忆服务，获取镜子摘要
    A->>V: 调用安全服务，获取待确认项与恢复点
    A->>V: 调用设置服务，获取控制项摘要

    V-->>A: 返回仪表盘首页数据
    A->>S: 更新仪表盘状态=已打开
    A->>P: 渲染仪表盘界面
    P-->>U: 展示首页焦点区与各模块入口
```

### 6.10 控制面板打开时序：托盘 -> 设置读取 / 保存

**链路目标**：提供与近场窗口独立的系统级设置和控制入口。  
**关键结果**：通过 `agent.settings.get / agent.settings.update` 读取和更新正式设置快照，并按 `apply_mode / need_restart` 驱动前端表现。  
**实现说明**：控制面板是低频设置域，应以正式协议为主边界；前端本地存储只承接未保存草稿、窗口布局和面板状态，不作为正式设置真源。

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 表现层
    participant A as 应用编排层
    participant S as 状态管理层
    participant V as 前端服务层
    participant RPC as 协议客户端（JSON-RPC）
    participant API as 后端服务（Go）
    participant I as 平台集成层

    U->>I: 右键点击托盘
    I-->>U: 展示托盘菜单
    U->>I: 点击打开控制面板
    I->>A: 通知打开控制面板
    A->>S: 更新控制面板状态=打开中
    A->>V: 调用设置服务，读取当前设置
    V->>RPC: 获取设置（agent.settings.get）
    RPC->>API: 协议请求
    API-->>RPC: 返回设置快照与应用方式
    RPC-->>V: 返回正式设置快照
    V-->>A: 返回设置项与当前值
    A->>P: 渲染控制面板界面
    A->>S: 更新控制面板状态=已打开
    P-->>U: 展示控制面板

    opt 用户修改设置并保存
        U->>P: 修改设置项并点击保存
        P->>A: 提交设置变更
        A->>V: 调用设置服务保存设置
        V->>RPC: 更新设置（agent.settings.update）
        RPC->>API: 协议请求
        API-->>RPC: 返回生效设置 / 应用方式 / 重启标记
        RPC-->>V: 返回保存结果
        V-->>A: 返回保存结果
        A->>S: 更新控制面板状态=已保存
        A->>P: 按 apply_mode 更新提示
        P-->>U: 提示保存成功或需要重启
    end
```

### 6.11 结果分流时序：完成 -> 交付出口

**链路目标**：把一个完成任务的结果分发到最合适的交付出口。  
**关键结果**：统一形成 `delivery_result`，并可能伴随 `artifact`。  
**重点约束**：长结果自动分流属于交付内核策略，不新增独立协议方法。  
**实现说明**：分发顺序应遵循“先告知，再正式交付，再提供入口”，保证用户能理解发生了什么。

```mermaid
sequenceDiagram
    participant U as 用户
    participant P as 表现层
    participant A as 应用编排层
    participant S as 状态管理层
    participant V as 前端服务层
    participant I as 平台集成层

    Note over U,I: 前提：任务已完成，系统拿到结果

    V-->>A: 返回任务结果
    A->>A: 判断结果类型与交付方式
    A->>S: 更新结果分发状态
    A->>S: 更新气泡状态
    A->>P: 先展示结果摘要与状态说明
    P-->>U: 气泡呈现已完成或已生成结果

    alt 短结果或轻量结果
        A->>P: 直接渲染到气泡
        P-->>U: 展示简短结果与下一步建议
    else 长文本或可编辑内容
        A->>V: 调用文件服务，生成工作区文档
        V->>I: 写入本地文件系统
        I-->>V: 返回文档路径
        V-->>A: 返回生成结果
        A->>P: 更新气泡提示=已写入文档并打开
        A->>I: 打开生成的文档
        I-->>U: 打开工作区文档
    else 网页结果或结构化结果
        A->>P: 更新气泡提示=正在打开结果页
        A->>I: 调用外部能力，打开浏览器或结果页
        I-->>U: 展示浏览器或结果页
    else 单个文件产物
        A->>P: 更新气泡提示=已生成文件，正在打开
        A->>I: 打开生成文件
        I-->>U: 展示目标文件
    else 多文件产物或导出结果
        A->>P: 更新气泡提示=已导出，正在定位文件夹
        A->>I: 打开文件夹并高亮结果
        I-->>U: 展示文件夹及目标文件
    else 连续任务或可追踪任务
        A->>P: 更新气泡提示=可在任务详情中查看
        A->>S: 更新仪表盘状态
        A->>I: 打开仪表盘或任务详情窗口
        I-->>U: 展示任务详情或历史任务页
    else 异常或待确认结果
        A->>P: 更新气泡提示=需要确认或执行异常
        A->>S: 更新悬浮球状态=等待确认或异常
        P-->>U: 展示确认入口或异常说明
    end
```

### 6.12 前端状态图：悬浮球

**说明**：这是前端局部状态机，不等同于正式 `task_status`。它描述的是近场交互承接过程。  
**实现补充**：悬浮球状态图与应用编排层强相关，不能在表现层单独推进。

```mermaid
stateDiagram-v2
    [*] --> 待机

    待机 --> 可唤起: 用户靠近或悬停达到阈值
    待机 --> 承接中: 长按语音 / 拖拽对象进入 / 文本选中提示
    待机 --> 意图确认中: 用户点击悬浮球进入确认流程
    待机 --> 处理中: 已接收任务并开始处理

    可唤起 --> 待机: 用户离开或未满足触发条件
    可唤起 --> 承接中: 用户继续输入 / 拖拽 / 长按
    可唤起 --> 意图确认中: 用户左键单击悬浮球

    承接中 --> 意图确认中: 已识别任务对象且需要确认意图
    承接中 --> 处理中: 任务可直接执行
    承接中 --> 待机: 用户取消或对象失效

    意图确认中 --> 处理中: 用户确认或修正意图后执行
    意图确认中 --> 等待确认: 系统给出待确认事项
    意图确认中 --> 待机: 用户取消或关闭

    处理中 --> 完成: 任务成功完成
    处理中 --> 等待确认: 处理中出现待确认动作
    处理中 --> 异常: 执行失败 / 理解异常 / 环境异常

    等待确认 --> 处理中: 用户确认继续
    等待确认 --> 待机: 用户忽略或取消
    等待确认 --> 异常: 确认失败或条件不满足

    完成 --> 待机: 结果已查看且状态回落
    异常 --> 待机: 用户关闭或恢复默认状态
```

### 6.13 前端状态图：气泡生命周期

**说明**：气泡生命周期属于表现态，不应该写入后端正式主状态表。  
**实现补充**：气泡删除、恢复、置顶等动作只影响表现层和局部状态，不应生成新的业务对象。

```mermaid
stateDiagram-v2
    [*] --> 显现

    显现 --> 隐藏: 鼠标离开悬浮球区域10s
    显现 --> 置顶显现: 用户置顶
    显现 --> 已销毁: 用户删除
    显现 --> 已销毁: 气泡数量超过阈值，旧气泡被销毁

    隐藏 --> 显现: 重新唤起/再次显示
    隐藏 --> 置顶显现: 用户置顶
    隐藏 --> 已销毁: 隐藏超过5分钟
    隐藏 --> 已销毁: 用户删除

    置顶显现 --> 显现: 用户取消置顶
    置顶显现 --> 已销毁: 用户删除

    已销毁 --> [*]
```

### 6.14 前端状态图：语音承接

**说明**：该状态机用于描述语音承接过程，和 `voice_session_state` 有映射关系，但不等价于后端 `task_status`。  
**实现补充**：语音状态机必须允许“锁定通话”和“打断补充”，但这些都不能直接推动正式任务状态。

```mermaid
stateDiagram-v2
    [*] --> 待机

    待机 --> 准备收音: 用户长按悬浮球
    准备收音 --> 收音中: 收音启动成功
    准备收音 --> 待机: 启动失败或用户放弃

    收音中 --> 锁定通话: 用户上滑锁定
    收音中 --> 已取消: 用户下滑取消
    收音中 --> 输入结束: 用户松开结束本轮输入

    锁定通话 --> 输入结束: 用户主动结束通话
    锁定通话 --> 已取消: 用户取消本轮语音

    输入结束 --> 理解处理中: 提交语音内容并进入理解
    理解处理中 --> 响应中: 系统开始返回结果
    理解处理中 --> 异常: 理解失败或处理失败

    响应中 --> 待机: 当前轮结束
    响应中 --> 收音中: 用户再次打断并补充需求

    已取消 --> 待机
    异常 --> 待机
```

### 6.15 前端状态图：意图确认

**说明**：这是前端承接流程和后端规划流程之间的桥接状态机，结束点通常会进入 `agent.task.confirm` 或直接执行。  
**实现补充**：所有对象型入口都应复用同一确认 / 执行骨架，不允许各入口维护不同的确认状态定义；带明确说明的文件任务可直接进入主执行链路，裸文件或低置信度对象进入确认 / 补充输入，带任务特定证据的结构化补充证据可挂回 `waiting_input / confirming_intent` 任务但仍不得自动执行。

```mermaid
stateDiagram-v2
    [*] --> 无任务对象

    无任务对象 --> 已识别任务对象: 文本选中 / 文本拖拽 / 文件拖拽 / 识别到协作机会
    已识别任务对象 --> 意图分析中: 用户点击悬浮球或系统进入确认流程

    意图分析中 --> 等待用户确认: 返回意图猜测与建议输出方式
    意图分析中 --> 已取消: 对象失效或用户关闭

    等待用户确认 --> 已确认: 用户接受当前意图
    等待用户确认 --> 已修正意图: 用户修改意图或输出方式
    等待用户确认 --> 已取消: 用户取消或忽略

    已修正意图 --> 已确认: 用户提交修正结果
    已修正意图 --> 已取消: 用户放弃

    已确认 --> 执行中: 发起任务执行
    执行中 --> [*]

    已取消 --> 无任务对象
```

### 6.16 前端状态图：任务视图

**说明**：该图是前端任务视图的状态表达，和正式 `task_status` 相对应，但会包含一些前端视角的分组和转移。  
**实现补充**：视图状态不能替代协议状态；所有正式状态判断以协议对象为准。

```mermaid
stateDiagram-v2
    [*] --> 待发起

    待发起 --> 正在进行: 任务正式开始
    正在进行 --> 接近完成: 已完成大部分步骤
    接近完成 --> 已完成: 结果生成完成

    正在进行 --> 等待授权: 出现待授权操作
    正在进行 --> 等待补充信息: 缺少必要输入
    正在进行 --> 暂停: 用户主动暂停
    正在进行 --> 阻塞: 上游条件不满足
    正在进行 --> 失败: 执行失败
    正在进行 --> 执行异常: 运行过程异常中断

    等待授权 --> 正在进行: 用户授权通过
    等待授权 --> 已取消: 用户拒绝授权

    等待补充信息 --> 正在进行: 用户补充信息
    等待补充信息 --> 已结束未完成: 长时间未补充或流程结束

    暂停 --> 正在进行: 用户继续任务
    暂停 --> 已取消: 用户取消任务

    阻塞 --> 正在进行: 阻塞条件解除
    阻塞 --> 已结束未完成: 未恢复即结束

    失败 --> 正在进行: 用户重试或恢复
    失败 --> 已结束未完成: 放弃处理

    执行异常 --> 正在进行: 异常恢复成功
    执行异常 --> 已结束未完成: 未恢复即结束

    已完成 --> [*]
    已取消 --> [*]
    已结束未完成 --> [*]
```

---

## 7. 模块职责明细

本章改为“快速定位索引”，不再机械重复第 3、4 章已经展开过的职责说明。读者如果已经通过架构总览定位到某个层级，可直接用下面两张表跳到对应详细章节。

### 7.1 前端模块快速索引

| 模块 | 详细章节 | 核心职责摘要 |
| --- | --- | --- |
| 前端工程与桌面宿主 | `3.1` | 承载 Tauri 宿主、多窗口和应用生命周期 |
| 表现层模块 | `3.2` | 呈现悬浮球、气泡、轻量输入区、仪表盘与控制面板 |
| 应用编排层模块 | `3.3` | 把单击、双击、长按、悬停、拖拽等动作收口成统一任务请求 |
| 状态管理与查询层模块 | `3.4` | 收口前端局部状态、查询缓存和订阅回写 |
| 前端服务层模块 | `3.5` | 把业务意图翻译成 RPC 和平台能力调用 |
| 平台集成与协议适配层模块 | `3.6` | 承接 Named Pipe、通知桥、窗口/托盘/快捷键/文件等平台能力 |
| 产品功能域与模块承接映射 | `3.7` | 用产品域视角串起前端模块和后端承接边界 |

### 7.2 后端模块快速索引

| 模块 | 详细章节 | 核心职责摘要 |
| --- | --- | --- |
| 本地接入层 | `4.1` | JSON-RPC 收口、对象锚定、查询装配、通知回流 |
| 任务编排与运行层 | `4.2` | 任务编排、上下文归一、入口规划、运行控制、会话串行与插件协调 |
| 能力与存储层（能力接入与检索子层） | `4.3` | 模型、工具、worker、sidecar、RAG 与屏幕/视频能力接入 |
| 治理与交付层 | `4.4` | 风险评估、正式交付、记忆沉淀、Trace / Eval、恢复、预算和边界治理 |
| 能力与存储层（平台、执行与持久化适配子层） | `4.5` | 对象仓储、索引、文件系统、系统能力、执行后端和跨平台抽象 |
| 系统级主链流程 | `5.0~5.11` | 用流程图解释主链、反馈链、推荐与结果分流 |
| 功能级时序与状态图 | `6.1~6.16` | 用时序图和状态图解释具体功能如何实现 |

### 7.3 联调验收清单

以下内容在模块文档层面必须能真实联调：

1. 文本选中、文件拖拽、语音输入三种入口至少各跑通一次；
2. 至少一条高风险动作链路能形成授权、审计和恢复点；
3. 至少一条记忆写入和一次镜子召回链路真实可见；
4. 至少一条长结果自动分流到文档或文件；
5. 至少一条插件状态能进仪表盘；
6. 悬浮球、气泡、语音、意图确认、前端任务状态图能和真实链路对应；
7. 任一 worker / sidecar 失败都能被前端看到正式错误或待确认结果，而不是静默丢失；
8. 任一高风险动作都不能绕过恢复点和审计链；
9. 任一前馈命中和反馈命中都能在 Trace 中找到结构化记录；
10. 前端局部状态和后端正式状态不能互相替代或互相污染。
