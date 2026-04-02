# CialloClaw Prototype

一个可以直接运行的 Windows 本地桌面原型，覆盖第一阶段文档里的核心入口：

- 悬浮球
- 聊天弹层
- 控制面板
- 复制内容触发
- 视频链接确认
- Markdown 待办巡检
- 本地记忆文件输出
- 审批记录与任务轨迹

## 运行

```bash
python app.py
```

或者双击 `run.bat`。

## 说明

- 原型使用 Python 标准库和 Tkinter，不需要额外依赖。
- 剪贴板、待办巡检、日志、记忆文件都会写入本地工作区 `workspace/.ciallo/`。
- 当前版本把命令执行做成了审批演示，重点先验证交互闭环和本地留痕。

## 测试

```bash
python -m unittest
```

