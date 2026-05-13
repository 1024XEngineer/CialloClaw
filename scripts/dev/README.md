# 开发脚本说明

建议的本地联调顺序：

1. `pnpm dev:service`
2. `pnpm --dir apps/desktop exec tauri dev`
3. 协议链路稳定后，再补 worker 启动脚本与联调辅助脚本
