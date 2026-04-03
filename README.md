# CialloClaw

> An innovative AI-powered dashboard interface that visualizes agent states and system interactions through a dynamic "consciousness field" metaphor.

## [中文文档](#chinese-documentation)

---

## Overview

CialloClaw is a visually stunning dashboard application that presents AI agent activities as orbiting planets around a central orb. This immersive interface provides an intuitive way to monitor system status, manage tasks, and interact with AI assistants.

### Key Features

- **Planetary Visualization System** - Interactive orbiting representations of different modules
- **Consciousness Core** - Central orb that responds to user interactions
- **Multi-Module Support** - Task Status, Notepad Collaboration, Mirror, and Hardware Sense
- **Voice Interface** - Long-press center orb for voice commands
- **Focus Mode** - Press 'F' to enter focused view with staggered planet reveals
- **Detached Windows** - Drag planets to create separate floating windows
- **Keyboard Shortcuts** - Quick access to all functions
- **Internationalization** - Built-in i18n support for multiple languages

## Technology Stack

### Frontend
- **React 19.1.2** - Modern React with latest features
- **TypeScript** - Full TypeScript support
- **Vite 8.0.1** - Fast build tool and development server
- **Tailwind CSS 3.4.17** - Utility-first CSS framework
- **React Router DOM 7.6.3** - Client-side routing

### State Management & Data
- **React Hooks** - Built-in state management
- **Recharts 3.2.0** - Data visualization
- **Firebase 12.0.0** - Database and authentication
- **Supabase 2.57.4** - Alternative backend service
- **Stripe React 4.0.2** - Payment integration

### Internationalization
- **i18next 25.3.2** - Internationalization framework
- **react-i18next 15.6.0** - React integration
- **i18next-browser-languagedetector** - Auto language detection

## Project Structure

```
CialloClaw/
├── src/
│   ├── i18n/                 # Internationalization configuration
│   │   ├── index.ts
│   │   └── local/           # Translation files (auto-imported)
│   ├── mocks/               # Mock data and state definitions
│   │   └── agentStates.ts
│   ├── pages/               # Page components
│   │   ├── home/
│   │   │   ├── page.tsx     # Main dashboard component
│   │   │   └── components/  # Visual components
│   │   └── NotFound.tsx     # 404 page
│   ├── router/              # Routing configuration
│   │   ├── config.tsx
│   │   └── index.ts
│   ├── App.tsx              # Main app component
│   ├── main.tsx             # Application entry point
│   └── index.css            # Global styles
├── .github/workflows/       # GitHub Actions workflows
├── .githooks/               # Git hooks
├── eslint.config.ts         # ESLint configuration
├── tailwind.config.ts       # Tailwind CSS config
└── vite.config.ts           # Vite build configuration
```

## Getting Started

### Prerequisites

- Node.js (v18 or higher recommended)
- npm or yarn package manager

### Installation

1. Clone the repository:
```bash
git clone git@github.com:Blackcloudss/CialloClaw.git
cd CialloClaw
```

2. Install dependencies:
```bash
npm install
```

3. Start the development server:
```bash
npm run dev
```

4. Open your browser and navigate to:
```
http://localhost:3000
```

### Available Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start development server on port 3000 |
| `npm run build` | Build for production |
| `npm run preview` | Preview production build |
| `npm run lint` | Run ESLint |
| `npm run type-check` | Run TypeScript type checking |

## Core Modules

### 1. Task Status (任务状态)
Primary task management with states:
- Working
- Completing
- Done
- Error handling

### 2. Notepad Collaboration (便签协作)
Async task processing and reminders for team collaboration.

### 3. Mirror (镜子)
Periodic insights and habit tracking for self-reflection.

### 4. Hardware Sense (硬件感知)
System monitoring and performance awareness.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `F` | Enter/Exit Focus Mode |
| `1-5` | Quick access to module functions |
| `ESC` | Close panels/windows |
| `?` | Show help and shortcuts |
| Long-click center orb | Activate voice interface |

## Interactive Features

### Focus Mode
Press 'F' to enter a focused view with staggered planet reveals, minimizing distractions.

### Voice Interface
Long-press the center orb to activate voice commands for hands-free interaction.

### Detached Windows
Drag any planet to create a separate floating window for multitasking.

### Summon System
Proactive notifications that appear as summoned planets when events require attention.

## Development

### Code Quality
- Relaxed TypeScript settings for rapid development
- Custom ESLint rules including route validation
- Git commit message validation
- Comprehensive mock data system

### Build Configuration
- Outputs to `out/` directory
- Source maps enabled for debugging
- Auto-import configured for React, Router, and i18n

## Contributing

1. Fork the repository
2. Create your feature branch: `git checkout -b feature/my-feature`
3. Commit your changes: `git commit -m 'feat: add my feature'`
4. Push to the branch: `git push origin feature/my-feature`
5. Submit a pull request

## License

This project is licensed under the MIT License.

---

## Chinese Documentation [中文文档]

> 一个创新的 AI 驱动仪表板界面，通过动态"意识场"隐喻可视化代理状态和系统交互。

## 概述 [Overview]

CialloClaw 是一个视觉震撼的仪表板应用程序，将 AI 代理活动呈现为围绕中心球体运行的行星。这个沉浸式界面提供了监控系统状态、管理任务和与 AI 助手交互的直观方式。

### 核心特性

- **行星可视化系统** - 不同模块的交互式轨道表示
- **意识核心** - 响应用户交互的中心球体
- **多模块支持** - 任务状态、便签协作、镜子和硬件感知
- **语音界面** - 长按中心球体进行语音命令
- **专注模式** - 按 'F' 进入聚焦视图，行星交错显示
- **分离窗口** - 拖动行星创建独立的浮动窗口
- **键盘快捷键** - 快速访问所有功能
- **国际化** - 内置 i18n 支持多种语言

## 技术栈

### 前端
- **React 19.1.2** - 最新功能的现代 React
- **TypeScript** - 完整的 TypeScript 支持
- **Vite 8.0.1** - 快速构建工具和开发服务器
- **Tailwind CSS 3.4.17** - 实用优先的 CSS 框架
- **React Router DOM 7.6.3** - 客户端路由

### 状态管理和数据
- **React Hooks** - 内置状态管理
- **Recharts 3.2.0** - 数据可视化
- **Firebase 12.0.0** - 数据库和身份验证
- **Supabase 2.57.4** - 备用后端服务
- **Stripe React 4.0.2** - 支付集成

### 国际化
- **i18next 25.3.2** - 国际化框架
- **react-i18next 15.6.0** - React 集成
- **i18next-browser-languagedetector** - 自动语言检测

## 快速开始

### 安装

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

| 命令 | 描述 |
|------|------|
| `npm run dev` | 在端口 3000 启动开发服务器 |
| `npm run build` | 构建生产版本 |
| `npm run preview` | 预览生产构建 |
| `npm run lint` | 运行 ESLint |
| `npm run type-check` | 运行 TypeScript 类型检查 |

## 核心模块

### 1. 任务状态
主要任务管理，支持以下状态：
- 进行中 (Working)
- 完成中 (Completing)
- 已完成 (Done)
- 错误处理 (Error handling)

### 2. 便签协作
用于团队协作的异步任务处理和提醒。

### 3. 镜子
用于自我反思的定期洞察和习惯追踪。

### 4. 硬件感知
系统监控和性能感知。

## 键盘快捷键

| 按键 | 操作 |
|------|------|
| `F` | 进入/退出专注模式 |
| `1-5` | 快速访问模块功能 |
| `ESC` | 关闭面板/窗口 |
| `?` | 显示帮助和快捷键 |
| 长按中心球体 | 激活语音界面 |

## 交互功能

### 专注模式
按 'F' 进入聚焦视图，行星交错显示，最大限度减少干扰。

### 语音界面
长按中心球体激活语音命令，实现免提交互。

### 分离窗口
拖动任何行星创建独立的浮动窗口，便于多任务处理。

### 召唤系统
当事件需要关注时，主动通知会显示为被召唤的行星。

## 贡献

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feature/my-feature`
3. 提交更改：`git commit -m 'feat: add my feature'`
4. 推送到分支：`git push origin feature/my-feature`
5. 提交拉取请求

## 许可证

本项目采用 MIT 许可证。
