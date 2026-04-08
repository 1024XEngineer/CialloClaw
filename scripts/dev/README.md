# 开发脚本说明

建议的本地联调顺序：

1. `go run ./services/local-service/cmd/server`
2. `pnpm --dir apps/desktop dev`
3. 协议链路稳定后，再补 worker 启动脚本与联调辅助脚本
