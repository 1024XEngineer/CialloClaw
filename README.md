# CialloClaw 仪表盘

> 一个创新的 AI 驱动仪表盘界面，通过「意识场」隐喻可视化 AI 代理状态与系统交互。

## 概述

CialloClaw 是一个视觉震撼的仪表盘原型，将 AI 代理的活动呈现为围绕中心「意识核心」运行的行星。这个沉浸式界面为监控 AI 任务进度、管理异步协作和接收系统通知提供了直观的交互方式。

### 核心特性

- **三层意识场视觉系统** - 通过焦点/候选/后台三层级展现事件优先级
- **意识核心** - 响应式中心球体，支持拖拽与长按语音交互
- **多模块支持** - 任务状态、便签协作、镜子、硬件感知
- **语音界面** - 长按中心球体激活语音对话
- **专注模式** - 按下 `F` 键进入聚焦视图，行星交错显示
- **分离窗口** - 长按面板标题栏拖出独立浮动窗口
- **召唤系统** - AI 主动推送重要事件的通知
- **键盘快捷键** - 快速访问所有功能

---

## 技术栈

### 前端核心

| 技术 | 版本 | 用途 |
|------|------|------|
| **React** | 19.1.2 | 现代化 UI 框架，采用最新特性 |
| **TypeScript** | 5.8.3 | 类型安全保障 |
| **Vite** | 8.0.1 | 高速构建工具与开发服务器 |
| **Tailwind CSS** | 3.4.17 | 原子化 CSS 框架 |
| **React Router** | 7.6.3 | 客户端路由 |

### 状态管理与数据

| 技术 | 版本 | 用途 |
|------|------|------|
| **Recharts** | 3.2.0 | 数据可视化图表 |
| **Firebase** | 12.0.0 | 数据库与身份验证（预留） |
| **Supabase** | 2.57.4 | 备用后端服务（预留） |
| **Stripe** | 4.0.2 | 支付集成（预留） |

### 国际化

| 技术 | 版本 | 用途 |
|------|------|------|
| **i18next** | 25.3.2 | 国际化框架 |
| **react-i18next** | 15.6.0 | React 集成 |
| **i18next-browser-languagedetector** | 8.2.0 | 自动语言检测 |

### 开发工具

| 工具 | 版本 | 用途 |
|------|------|------|
| **ESLint** | 9.30.1 | 代码质量检查 |
| **PostCSS** | 8.5.6 | CSS 处理 |
| **unplugin-auto-import** | 19.3.0 | 自动导入 API |

---

## 交互设计详解

### 意识场视觉系统

仪表盘采用「意识场」隐喻，将不同模块的活动以行星形式围绕中心球体运行，通过三层级视觉权重展现事件优先级：

```
┌─────────────────────────────────────────────────────────────┐
│                                                           │
│                    ┌─────────────┐                        │
│                   ╱               ╲                       │
│                  │   后台行星      │  第三层：弱脉冲，仅信号  │
│                   ╲               ╱                        │
│                    └─────────────┘                         │
│                                                           │
│         ┌─────────────┐       ┌─────────────┐             │
│        ╱               ╲     ╱               ╲            │
│       │   候选行星      │   │   候选行星      │ 第二层      │
│        ╲               ╱     ╲               ╱            │
│         └─────────────┘       └─────────────┘             │
│                                                           │
│                 ┌─────────────┐                          │
│                ╱               ╲                         │
│               │   焦点行星      │  第一层：最亮，当前关注  │
│                ╲               ╱                         │
│                 └─────────────┘                          │
│                                                           │
│              ╱───────────────────╲                       │
│             │    ● 意识核心 ●     │                      │
│              ╲───────────────────╱                       │
│                                                           │
└─────────────────────────────────────────────────────────────┘
```

| 层级 | 视觉特征 | 代表含义 |
|------|----------|----------|
| **焦点行星** | 轨道半径 148px，尺寸 62px，最亮 | AI 当前正在执行的主要任务 |
| **候选行星** | 轨道半径 210-215px，尺寸 42-46px，中等亮度 | 次要任务，需要关注 |
| **后台脉冲** | 轨道半径 248px，尺寸 34px，微弱闪烁 | 后台运行的服务（如硬件监控） |

### 核心交互操作

#### 中心球体（意识核心）

| 操作 | 效果 |
|------|------|
| **拖拽** | 球体跟随鼠标移动，产生倾斜 3D 效果 |
| **松手** | 弹簧动画回弹至中心 |
| **长按 650ms** | 激活语音对话界面，进度环环绕显示 |

#### 行星节点

| 操作 | 效果 |
|------|------|
| **单击** | 打开对应模块的详情面板 |
| **拖拽** | 行星跟随移动 |
| **松手** | 自动吸附回轨道（计算角度） |
| **长按面板标题** | 显示进度条，持续拖拽后分离为独立窗口 |

#### 面板（JarvisPanel）

详情面板采用「JARVIS 风格」设计，支持丰富的手势操作：

| 操作 | 效果 |
|------|------|
| **左右滑动** | 切换当前模块的不同状态（如任务状态：进行中 → 已完成） |
| **上下滑动** | 切换到不同模块（任务 → 便签 → 镜子 → 硬件） |
| **拖拽边缘** | 调整面板尺寸 |
| **双击尺寸指示器** | 重置为默认大小 |

### 键盘快捷键

| 按键 | 功能 |
|------|------|
| `F` | 进入/退出专注模式 |
| `1-4` | 快速切换到对应模块（任务/便签/镜子/硬件） |
| `5` | 打开信任与边界面板 |
| `?` | 显示快捷键帮助面板 |
| `ESC` | 关闭当前面板/退出语音界面 |
| `方向键` | 在面板中导航（←→ 切换状态，↑↓ 切换模块） |

### 专注模式

按下 `F` 键进入专注模式：
- 所有行星淡出至不可见
- 仅保留中心球体与极简环境
- 退出时行星交错淡入，间隔 110ms

### 语音界面

长按中心球体 650ms 激活语音对话，包含 5 个阶段：

1. **就绪** - 显示麦克风图标，提示"直接说出你的想法"
2. **聆听** - 波纹环绕，实时捕获关键词并向上漂移
3. **理解** - 粒子向中心汇聚，提取意图碎片
4. **确认** - 显示 AI 理解的摘要与操作按钮
5. **执行** - 步进式显示执行进度，完成后跳转至对应模块

**语音建议指令：**
- "帮我看今天的重点任务"
- "总结我刚才说的内容"
- "帮我记一下这个想法"
- "最近系统状态怎么样"

### 召唤系统

AI 主动推送通知，以「被召唤的行星」形式出现：

```
┌─────────────────────────────────────┐
│  ● 你有一个 2 小时后到期的任务       │
│  周报整理 · 今日 11:00 截止         │
│  [点击查看并开始处理]               │
└─────────────────────────────────────┘
```

支持三种优先级：`urgent`（紧急）、`normal`（普通）、`low`（低）

---

## 仪表盘承载内容

### 1. 任务状态模块（焦点行星）

展示 AI 正在执行的主要任务，包含 9 种状态：

| 状态 | 标签 | 说明 |
|------|------|------|
| `standby` | 待命 | 等待新指令 |
| `idle_present` | 空闲在场 | 上次任务完成，持续监听 |
| `working` | 推进中 | 正在处理任务，显示进度条 |
| `highlight` | 新进展 | 发现需要关注的风险点 |
| `completing` | 接近完成 | 草稿已就绪，等待确认 |
| `done` | 已完成 | 任务全部完成 |
| `error_permission` | 缺少权限 | 需要用户授权才能继续 |
| `error_blocked` | 步骤阻塞 | 依赖的上游任务未完成 |
| `error_missing_info` | 缺少信息 | 需要用户提供关键信息 |

**面板内容：**
- 任务标题与副标题
- 进度百分比与步骤指示器
- 上下文信息流（实时更新的操作日志）
- 异常警告卡片（如有）

### 2. 便签协作模块（候选行星）

异步任务处理与团队协作功能：

| 状态 | 标签 | 说明 |
|------|------|------|
| `notepad_processing` | 便签处理中 | AI 正在处理用户留下的便签 |
| `notepad_reminder` | 重复任务提醒 | 识别用户习惯，定时提醒 |
| `scheduled_task` | 定时任务 | 自动巡检已完成，发现需关注项 |

**面板内容：**
- 便签列表（处理中/待执行/需确认）
- 标签分类（文档/沟通/优先级）
- 上下文操作日志

### 3. 镜子模块（候选行星）

周期性总结与习惯洞察，帮助用户自我反思：

| 状态 | 标签 | 说明 |
|------|------|------|
| `mirror_summary` | 周期总结 | 总结最近两周的关注重点 |
| `mirror_habit` | 习惯洞察 | 识别用户形成的新工作节奏 |

**面板内容：**
- 洞察周期（如 2025 年 3 月 17 日 — 3 月 31 日）
- 洞察卡片列表（带强调标记）
- 观察到的模式与建议

### 4. 硬件感知模块（后台脉冲）

系统监控与性能感知：

| 状态 | 标签 | 说明 |
|------|------|------|
| `sense_alert` | 系统感知 | CPU/内存负荷过高，建议暂缓任务 |
| `sense_suggestion` | 系统建议 | 系统状态良好，适合启动大任务 |

**面板内容：**
- 实时系统指标（CPU/内存/网络/磁盘）
- 风险等级指示（正常/警告/临界）
- 建议操作

### 5. 信任与边界模块

展示 AI 决策的透明度与风险边界：

**显示信息：**
- 风险等级（低/中/高）
- 是否需要用户授权
- 数据保存路径
- 是否可恢复
- 成本异常提示

---

## 项目结构

```
CialloClaw/
├── src/
│   ├── i18n/                    # 国际化配置
│   │   ├── index.ts
│   │   └── local/               # 翻译文件（自动导入）
│   ├── mocks/                   # Mock 数据与状态定义
│   │   └── agentStates.ts       # 18 种代理状态数据
│   ├── pages/                   # 页面组件
│   │   ├── home/
│   │   │   ├── page.tsx         # 主仪表盘组件
│   │   │   └── components/      # 视觉组件
│   │   │       ├── CenterOrb.tsx        # 中心意识球
│   │   │       ├── PlanetNode.tsx       # 行星节点
│   │   │       ├── JarvisPanel.tsx      # 详情面板
│   │   │       ├── VoiceInterface.tsx   # 语音界面
│   │   │       ├── SummonedPlanet.tsx   # 召唤通知
│   │   │       ├── DetachedWindow.tsx   # 独立窗口
│   │   │       └── ...
│   │   └── NotFound.tsx
│   ├── router/                  # 路由配置
│   ├── App.tsx
│   ├── main.tsx
│   └── index.css
├── .github/workflows/           # GitHub Actions
├── .githooks/                   # Git hooks
├── eslint.config.ts
├── tailwind.config.ts
├── vite.config.ts
└── package.json
```

---

## 实现模式

本项目采用了一套一致且精心设计的交互实现模式，以下展示核心技术细节。

### 拖拽交互模式

#### 1. 中心球拖拽（弹簧回弹）

**位置**: `src/pages/home/components/CenterOrb.tsx:96-161`

使用物理弹簧模型实现平滑回弹效果：

```tsx
// 弹簧回弹核心算法
const spring = () => {
  const stiffness = 0.16;  // 弹簧刚度
  const damping = 0.70;    // 阻尼系数
  velRef.current.x += -offsetRef.current.x * stiffness;
  velRef.current.y += -offsetRef.current.y * stiffness;
  velRef.current.x *= damping;
  velRef.current.y *= damping;
  offsetRef.current.x += velRef.current.x;
  offsetRef.current.y += velRef.current.y;

  // 使用 requestAnimationFrame 实现平滑动画
  if (dist > 0.3 || Math.abs(velRef.current.x) > 0.1) {
    returnAnimRef.current = requestAnimationFrame(spring);
  }
};
```

**特点**：
- 拖拽半径限制为 100px
- 使用 `useRef` 存储速度和位置，避免频繁重新渲染
- 同时支持拖拽和长按（650ms 触发语音）

#### 2. 行星拖拽（自动吸附轨道）

**位置**: `src/pages/home/components/PlanetNode.tsx:122-185`

行星松手后自动计算轨道角度并吸附：

```tsx
// 计算释放点的轨道角度
const rect = containerRef.current.getBoundingClientRect();
const relX = e.clientX - (rect.left + rect.width / 2);
const relY = e.clientY - (rect.top + rect.height / 2);
const dropAngle = (Math.atan2(relY, relX) * 180) / Math.PI;
const normalizedAngle = ((dropAngle % 360) + 360) % 360;
onOrbitAngleChange?.(config.key, normalizedAngle);
```

**特点**：
- 6px 阈值区分点击和拖拽
- 使用 `Math.atan2` 计算极坐标角度
- 角度标准化到 0-360° 范围

#### 3. 面板长按分离窗口

**位置**: `src/pages/home/components/JarvisPanel.tsx:476-560`

长按面板标题栏 800ms 后可拖拽分离：

```tsx
// 使用 setInterval 更新进度条（每 100ms）
longPressProgressRef.current = setInterval(() => {
  setLongPressProgress(prev => {
    const next = prev + 100 / 8;  // 8 步完成
    if (next >= 100) {
      clearInterval(longPressProgressRef.current);
      return 100;
    }
    return next;
  });
}, 100);

// 长按 800ms 后激活拖拽
longPressTimerRef.current = setTimeout(() => {
  isLongPressRef.current = true;
  detachActiveRef.current = true;
}, 800);
```

**特点**：
- 使用 `setInterval` 产生平滑的进度条动画
- 拖拽超过 80px 显示分离预览
- 支持水平/垂直滑动锁定

---

### 动画实现模式

#### 1. 请求动画帧（RAF）

**位置**: `src/pages/home/page.tsx:418-445`

使用 `requestAnimationFrame` 替代 `setInterval` 实现平滑动画：

```tsx
useEffect(() => {
  const animate = (timestamp: number) => {
    // 脉冲动画：正弦波产生 0-1 的值
    const t = Date.now() / 1000;
    setPulse(Math.sin(t * 1.2) * 0.5 + 0.5);

    // 使用 timestamp 计算 delta time
    const dt = lastTimeRef.current ? (timestamp - lastTimeRef.current) / 1000 : 0;
    lastTimeRef.current = timestamp;

    // 更新行星轨道角度（使用 delta time 保证平滑）
    if (dt > 0 && dt < 0.1) {
      setAngles(prev => {
        const next = { ...prev };
        ALL_PLANETS.forEach(p => {
          if (!draggingPlanets.current.has(p.key) && p.orbitSpeed > 0) {
            next[p.key] = (prev[p.key] + p.orbitSpeed * dt) % 360;
          }
        });
        return next;
      });
    }

    animRef.current = requestAnimationFrame(animate);
  };
  animRef.current = requestAnimationFrame(animate);
  return () => cancelAnimationFrame(animRef.current);
}, []);
```

**优势**：
- 与浏览器刷新率同步（通常 60fps）
- 页面不可见时自动暂停，节省资源
- 使用 delta time 保证动画平滑

#### 2. 粒子系统（语音界面）

**位置**: `src/pages/home/components/VoiceInterface.tsx:171-196`

在"理解"阶段，粒子向中心汇聚：

```tsx
setParticles(prev => {
  // 更新现有粒子：向中心移动 + 淡出
  const moved = prev.map(p => ({
    ...p,
    radius: p.radius - p.speed * 1.4,
    opacity: p.radius < 35 ? p.opacity * 0.88 : p.opacity * 0.992,
  })).filter(p => p.radius > 4 && p.opacity > 0.03);

  // 随机生成新粒子
  if (moved.length < 20 && Math.random() < 0.35) {
    const angle = Math.random() * Math.PI * 2;
    const radius = 110 + Math.random() * 55;
    moved.push({
      id: ++particleIdRef.current,
      x: Math.cos(angle) * radius,
      y: Math.sin(angle) * radius,
      radius,
      opacity: 0.5 + Math.random() * 0.35,
      speed: 0.7 + Math.random() * 0.7,
      size: 1.5 + Math.random() * 2,
    });
  }
  return moved;
});
```

**特点**：
- 粒子数量限制为 20 个
- 使用 `Math.random()` 控制生成概率
- 每个粒子有独立的速度和生命周期

#### 3. CSS 过渡效果

**位置**: `src/pages/home/components/CenterOrb.tsx:359-365`

根据交互状态动态切换过渡时长：

```tsx
transform: `perspective(300px) rotateX(${tiltX}deg) rotateY(${tiltY}deg) scale(...)`,
transition: isDragging
  ? 'transform 0.05s ease'                                    // 拖拽时快速响应
  : 'transform 0.9s cubic-bezier(0.34,1.56,0.64,1), ...',     // 松手时弹性缓动
```

**缓动函数说明**：
- `ease` - 标准缓动
- `cubic-bezier(0.34,1.56,0.64,1)` - 弹性效果（超出目标后回弹）

---

### 状态管理模式

#### 1. 组件本地状态

**位置**: 所有组件统一使用 `useState` + `useRef` 组合

```tsx
// useState 用于需要触发重新渲染的状态
const [rotation, setRotation] = useState(0);
const [isDragging, setIsDragging] = useState(false);
const [offset, setOffset] = useState({ x: 0, y: 0 });

// useRef 用于存储不触发渲染的值
const animRef = useRef<number>(0);           // 动画帧 ID
const dragStartRef = useRef({ x: 0, y: 0 }); // 拖拽起始点
const velRef = useRef({ x: 0, y: 0 });        // 速度向量
const isDraggingRef = useRef(false);          // 拖拽状态标志
```

**设计原则**：
- 渲染相关状态用 `useState`
- 临时值和 DOM 引用用 `useRef`
- 事件处理函数用 `useCallback` 缓存

#### 2. 全局状态提升

**位置**: `src/pages/home/page.tsx:312-358`

在父组件中管理子组件共享的状态：

```tsx
// 状态提升到父组件
const [activePlanet, setActivePlanet] = useState<ModuleKey | null>(null);
const [currentState, setCurrentState] = useState<AgentStateKey>('working');
const [focusMode, setFocusMode] = useState(false);

// 派生状态通过条件计算得出
const stateData = agentStates[currentState];
const activePlanetConfig = activePlanet
  ? ALL_PLANETS.find(p => p.key === activePlanet)
  : null;

// 事件处理函数通过 props 传递
const handlePlanetClick = useCallback((key: ModuleKey) => {
  setActivePlanet(key);
  setCurrentState(group.states[0]);
}, [activePlanet]);
```

**优点**：
- 多个组件可以访问同一状态
- 状态变化时同步更新所有相关组件
- 便于实现复杂的交互逻辑

---

### 长按交互模式

**位置**: `src/pages/home/components/CenterOrb.tsx:125-161`

长按检测使用 `setTimeout` + `requestAnimationFrame` 组合：

```tsx
const LONG_PRESS_DURATION = 650; // ms

// 进度环动画（RAF 实现）
const startLongPressAnim = useCallback(() => {
  longPressStartRef.current = Date.now();
  const tick = () => {
    const elapsed = Date.now() - longPressStartRef.current;
    const progress = Math.min(elapsed / LONG_PRESS_DURATION, 1);
    setLongPressProgress(progress);
    if (progress < 1) {
      longPressAnimRef.current = requestAnimationFrame(tick);
    }
  };
  longPressAnimRef.current = requestAnimationFrame(tick);
}, []);

// 长按检测（setTimeout 实现）
longPressRef.current = setTimeout(() => {
  cancelLongPressAnim();
  onLongPress?.();  // 触发回调
}, LONG_PRESS_DURATION);
```

**特点**：
- `requestAnimationFrame` 更新进度环（60fps）
- `setTimeout` 检测长按阈值
- 移动超过 6px 自动取消长按

---

### 过渡效果模式

#### 交错淡入动画

**位置**: `src/pages/home/page.tsx:456-464`

退出专注模式时行星交错显示：

```tsx
setFocusModeExiting(true);
setRevealedPlanets(new Set());

ALL_PLANETS.forEach((p, i) => {
  setTimeout(() => {
    setRevealedPlanets(cur => new Set([...cur, p.key]));
  }, 120 + i * 110);  // 基础延迟 120ms，每个行星额外延迟 110ms
});

setTimeout(() => setFocusModeExiting(false),
  120 + ALL_PLANETS.length * 110 + 400
);
```

**效果**：
- 第一个行星延迟 120ms 出现
- 第二个行星延迟 230ms 出现
- 第三个行星延迟 340ms 出现
- 以此类推...

---

## 快速开始

### 环境要求

- Node.js v18 或更高版本
- npm 或 yarn 包管理器

### 安装步骤

1. 克隆仓库：
```bash
git clone git@github.com:Blackcloudss/CialloClaw.git
cd CialloClaw
```

2. 安装依赖：
```bash
npm install
```

3. 启动开发服务器：
```bash
npm run dev
```

4. 在浏览器中打开：
```
http://localhost:3000
```

### 可用脚本

| 命令 | 说明 |
|------|------|
| `npm run dev` | 在端口 3000 启动开发服务器 |
| `npm run build` | 构建生产版本 |
| `npm run preview` | 预览生产构建 |
| `npm run lint` | 运行 ESLint 检查 |
| `npm run type-check` | 运行 TypeScript 类型检查 |

---

## 设计理念

### 意识场隐喻

将 AI 代理的活动想象为一个「意识场」：

- **中心球体** 代表 AI 的「意识核心」，始终在场，持续呼吸
- **轨道行星** 代表不同的任务流，按优先级分层呈现
- **召唤系统** 模拟 AI 的主动意识，当需要关注时主动浮现
- **拖拽与吸附** 体现意识的流动性与弹性

### 视觉语言

- **颜色编码**：每个模块有专属色（任务-绿/便签-紫/镜子-紫/硬件-蓝）
- **呼吸动画**：所有元素以不同速率呼吸，传达生命力
- **光晕效果**：核心与行星使用多层光晕，营造能量场感
- **层级衰减**：非焦点元素逐渐透明，减少视觉干扰

---

## 开发说明

### 代码质量

- 宽松的 TypeScript 配置以支持快速原型开发
- 自定义 ESLint 规则（含路由验证）
- Git commit 消息验证
- 完善的 Mock 数据系统

### 构建配置

- 输出至 `out/` 目录
- 启用 Source Maps 用于调试
- 自动导入配置（React、Router、i18n）

---

## 贡献指南

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feature/my-feature`
3. 提交更改：`git commit -m 'feat: add my feature'`
4. 推送到分支：`git push origin feature/my-feature`
5. 提交 Pull Request

---

## 许可证

本项目采用 MIT 许可证。

---

> **CialloClaw** - 让 AI 的意识场可视化
