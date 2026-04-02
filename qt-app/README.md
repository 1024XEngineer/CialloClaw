# CialloClaw Qt 6 原型机

基于 Qt 6 + QML + Go 的桌面桌宠原型机。

## 项目结构

```
qt-app/
├── CMakeLists.txt
├── src/
│   ├── main.cpp
│   ├── PetWindow.h
│   ├── PetWindow.cpp
│   ├── ChatWindow.h
│   └── ChatWindow.cpp
└── resources/
    └── ChatWindow.qml
```

## 依赖

- Qt 6.5+ (Core, Gui, Widgets, Network, Qml, Quick)
- Go 1.20+
- CMake 3.16+

## 构建

### 1. 安装 Qt 6

```bash
# Windows: 使用 Qt Online Installer
# macOS: brew install qt
# Linux: sudo apt install qt6-base-dev qt6-declarative-dev
```

### 2. 构建

```bash
cd qt-app
mkdir build && cd build
cmake ..
cmake --build .
```

### 3. 运行

```bash
# 确保 Go sidecar 在运行
cd ../go-backend
go run ./cmd/sidecar/main.go &

# 运行 Qt 应用
./build/cialloclaw
```

## 窗口说明

### 桌宠窗口 (PetWindow)
- 独立异形窗口，基于 `setMask` 实现轮廓内交互/轮廓外穿透
- 可拖动
- 左键点击打开对话窗口
- 右键弹出菜单（隐藏/退出）
- 系统托盘支持（显示/退出）

### 对话窗口 (ChatWindow)
- QML 实现的独立窗口
- 默认隐藏
- 从 Go sidecar 获取 mock 数据
- 假消息/假输入框/假发送按钮

## Go Mock

Go sidecar 提供:
- `/api/pet` - 桌宠状态
- `/api/chat/init` - 对话初始消息