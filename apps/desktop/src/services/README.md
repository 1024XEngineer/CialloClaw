# 前端服务层

这里封装前端的协议访问与平台适配能力。

- 可以依赖 `rpc`、`platform`、`packages/protocol`
- 不能直接引入 Go `internal/*` 实现
- 对外产品主对象统一使用 `task`
