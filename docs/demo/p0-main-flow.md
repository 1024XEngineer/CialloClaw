# P0 主链路

## 目标链路

1. 用户拖拽文件、选中文本、悬停输入或语音提交。
2. `shell-ball` 承接当前现场并展示意图确认气泡。
3. 前端通过 `agent.task.start`、`agent.task.confirm`，或同 session 的 `agent.input.submit` 续接来发起或更新任务。
4. Go harness 建立 `task` 与 `run` 映射，并推进执行。
5. 工具链执行后生成短结果或正式交付对象 `delivery_result`。
6. artifact、audit、recovery_point 能在 dashboard 中被看到。
7. 至少有一次记忆检索或记忆摘要挂载返回。

## 当前骨架覆盖范围

- `apps/desktop` 已提供三个桌面入口和基础 task 视角界面
- `packages/protocol` 已提供 task-centric 协议骨架与 dot.case 方法名
- `services/local-service/internal/*` 已按 harness 分层落位
- 主前后端通信骨架已明确收敛为 Named Pipe 优先、HTTP 调试兼容
- `workers/*` 已隔离 Playwright、OCR、媒体处理 sidecar

## 下一步实现切片

- 接通 `/rpc` 的 `agent.task.start`、`agent.task.confirm`、`agent.task.list`
- 建立 `task_id` 与 `run_id` 的可追踪映射并写入 SQLite
- 打通 1 条工具执行链路，产出 1 个 artifact 与 1 个 `delivery_result`
