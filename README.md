# CialloClaw 桌面原型机

基于 **Tauri 2 + Vue 3 + TypeScript + Tailwind CSS + Go sidecar** 的多窗口桌面原型机。

目标：把“桌面常驻悬浮球 Agent”做成可运行、可打包、可继续演进的产品前身，而不是一次性网页 demo。

---

## 已完成能力

- 四个独立窗口
  - 悬浮球窗口 `orb`
  - 主对话窗口 `chat`
  - 设置窗口 `settings`
  - 轻提示窗口 `nudge`
- Tauri 2 Rust 壳负责
  - 多窗口创建与关闭语义控制
  - 托盘菜单
  - 原生右键菜单弹出
  - Go sidecar 启停
- Vue 前端负责
  - 路由化窗口视图
  - 像素风 UI 呈现
  - 统一 store / service 数据访问
- Go sidecar 负责
  - 首页状态 mock
  - 轻提示 mock
  - 对话场景 mock
  - 设置数据 mock
  - mock 动作确认

---

## 项目结构

```text
Ciallo-Demo/
├─ src/                         # Vue + Vite 前端
│  ├─ index.html
│  ├─ package.json
│  ├─ vite.config.ts
│  ├─ tailwind.config.cjs
│  └─ src/
│     ├─ main.ts
│     ├─ App.vue
│     ├─ router/
│     ├─ windows/
│     │  ├─ OrbWindow.vue
│     │  ├─ ChatWindow.vue
│     │  ├─ SettingsWindow.vue
│     │  └─ NudgeWindow.vue
│     ├─ components/
│     ├─ stores/
│     ├─ services/
│     ├─ styles/
│     └─ types/
├─ src-tauri/                   # Tauri 2 Rust 桌面壳
│  ├─ src/main.rs
│  ├─ build.rs
│  ├─ Cargo.toml
│  ├─ tauri.conf.json
│  └─ icons/
├─ go-backend/                  # Go mock sidecar
│  ├─ go.mod
│  ├─ bin/
│  └─ cmd/sidecar/main.go
└─ README.md
```

---

## 各层职责

### 1. Rust / Tauri 层

只负责桌面壳：

- 创建 4 个原生窗口
- 维护“关闭 = 隐藏”“退出 = 真退出”的语义
- 托盘菜单
- 悬浮球右键原生菜单
- 启动 Go sidecar
- 提供必要 IPC（如打开设置、隐藏应用、显示聊天窗）

### 2. Vue 前端层

只负责表现与交互：

- 窗口 UI
- 路由视图
- store 状态管理
- service 访问 sidecar
- 统一像素风视觉

### 3. Go sidecar 层

负责未来 Agent 边界的 mock 承接：

- `/api/home`
- `/api/nudges`
- `/api/scenarios`
- `/api/settings`
- `/api/actions/confirm`

目前全部为 mock 数据，但数据源真实来自 Go，而不是散落在窗口组件里。

---

## 当前哪些是 mock

以下内容都还是 mock：

- Agent 推理结果
- 记忆系统
- 权限系统
- 执行系统
- 工作流
- 系统感知/OCR/真实文件写入

### 未来如何接入真实 Agent

后续只需要替换 `go-backend/cmd/sidecar/main.go` 背后的实现：

- 把静态 JSON 替换为真实 workflow / memory / policy 服务
- 保持前端 `services/api.ts` 的接口不变
- 保持窗口组件层不承载业务逻辑

---

## 开发环境要求

- Node.js 18+
- Rust stable
- Go 1.20+
- Windows 下建议已安装 WebView2 Runtime

---

## 安装依赖

### 前端

```powershell
Set-Location E:\code\opencode\Ciallo-Demo\src
npm install
```

### Go sidecar

```powershell
Set-Location E:\code\opencode\Ciallo-Demo\go-backend
go build -o .\bin\cialloclaw-sidecar.exe .\cmd\sidecar
```

### Rust / Tauri

Rust 依赖通过 Cargo 自动处理。

---

## 启动开发环境

```powershell
Set-Location E:\code\opencode\Ciallo-Demo
& ".\src\node_modules\.bin\tauri.cmd" dev
```

说明：

- Vite dev server 会跑在 `http://127.0.0.1:5173`
- Tauri 会创建 4 个原生窗口
- Rust 会尝试启动 `go-backend/bin/cialloclaw-sidecar.exe`

---

## 构建

### 前端构建

```powershell
Set-Location E:\code\opencode\Ciallo-Demo\src
npm run build
```

### Go sidecar 构建

```powershell
Set-Location E:\code\opencode\Ciallo-Demo\go-backend
if (!(Test-Path .\bin)) { New-Item -ItemType Directory -Path .\bin | Out-Null }
go build -o .\bin\cialloclaw-sidecar.exe .\cmd\sidecar
```

### Windows 桌面应用构建

```powershell
Set-Location E:\code\opencode\Ciallo-Demo
& ".\src\node_modules\.bin\tauri.cmd" build --debug
```

---

## 当前已产出的 Windows 可运行成品

本次环境内已实际构建成功：

- 调试可执行文件：
  - `src-tauri/target/debug/cialloclaw.exe`
- Windows 安装包：
  - `src-tauri/target/debug/bundle/nsis/CialloClaw_0.0.0_x64-setup.exe`
- Windows MSI：
  - `src-tauri/target/debug/bundle/msi/CialloClaw_0.0.0_x64_en-US.msi`

其中：

- **可双击运行的 exe 安装成品**：
  - `src-tauri/target/debug/bundle/nsis/CialloClaw_0.0.0_x64-setup.exe`

---

## macOS 打包方式

当前环境是 Windows，无法直接产出 macOS `.app`，但工程已具备 Tauri 2 标准构建链路。

在 **macOS 主机** 上执行：

```bash
cd /path/to/Ciallo-Demo/src
npm install
npm run build
npx tauri build --target universal-apple-darwin
```

典型产物位置：

- `.app`：`src-tauri/target/universal-apple-darwin/release/bundle/macos/`
- `.dmg`：`src-tauri/target/universal-apple-darwin/release/bundle/dmg/`

如需签名/公证，需要在 macOS 环境补充 Apple 开发者证书配置。

---

## Linux 打包方式

当前环境是 Windows，无法直接产出 Linux 成品，但工程已具备标准构建链路。

在 **Linux 主机** 上执行：

```bash
cd /path/to/Ciallo-Demo/src
npm install
npm run build
npx tauri build
```

典型产物位置：

- AppImage：`src-tauri/target/release/bundle/appimage/`
- deb：`src-tauri/target/release/bundle/deb/`
- rpm：`src-tauri/target/release/bundle/rpm/`

Linux 下推荐交付：

- `AppImage`（双击可运行）
- 或 `deb/rpm`（标准桌面分发）

---

## 窗口职责

### 悬浮球窗口 `orb`

- 常驻桌面主入口
- 左键打开/聚焦主对话窗
- 右键弹原生菜单
- 始终存在，除非明确退出应用

### 主对话窗口 `chat`

- 承接核心 mock 场景
- 展示“先提示，再确认，后执行”
- 关闭只隐藏，不退出应用

### 设置窗口 `settings`

- 仅通过悬浮球右键菜单或托盘菜单打开
- 展示外观、提醒、mock 模式、安全说明等

### 轻提示窗口 `nudge`

- 低打扰提示
- 可查看 / 稍后 / 忽略
- 查看会导向主对话窗口

---

## 多窗口与生命周期语义

- 悬浮球是锚点窗口
- 主对话关闭仅隐藏
- 设置窗口关闭仅隐藏
- 轻提示关闭仅隐藏
- “隐藏” = 所有窗口隐藏，但进程仍在、托盘仍在
- “退出” = 结束窗口、结束 sidecar、结束应用进程

---

## 当前验证结果

已验证：

- `cargo check` 通过
- `cargo build` 通过
- `go build ./cmd/sidecar` 通过
- `npm run build` 通过
- `npx tauri build --debug` 通过（Windows）
- Go sidecar HTTP 接口可返回数据

---

## 剩余风险

- macOS `.app` 与 Linux AppImage/deb/rpm 需要在对应原生系统上实际执行打包
- 当前图标仍为占位资产，后续可替换为正式品牌资产
- Go sidecar 当前走固定 localhost 端口，未来可升级为更强的 sidecar 发现/健康检查机制

---

## 默认产物位置速查

### Windows

- 可执行文件：`src-tauri/target/debug/cialloclaw.exe`
- NSIS 安装包：`src-tauri/target/debug/bundle/nsis/CialloClaw_0.0.0_x64-setup.exe`
- MSI：`src-tauri/target/debug/bundle/msi/CialloClaw_0.0.0_x64_en-US.msi`

### macOS

- `.app`：`src-tauri/target/<target>/release/bundle/macos/`
- `.dmg`：`src-tauri/target/<target>/release/bundle/dmg/`

### Linux

- `AppImage`：`src-tauri/target/release/bundle/appimage/`
- `deb`：`src-tauri/target/release/bundle/deb/`
- `rpm`：`src-tauri/target/release/bundle/rpm/`
