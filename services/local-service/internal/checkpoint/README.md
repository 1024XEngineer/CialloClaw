# checkpoint 模块 README

## 1. 模块定位

`/services/local-service/internal/checkpoint` 是 CialloClaw 后端治理与安全层中的**恢复点模块**。

本模块负责在高风险动作执行前，为上层提供“是否需要创建恢复点、恢复点描述如何组织、恢复点对象如何表达”的统一能力入口，并通过注入的 writer 输出给持久化层。

本模块当前不是回滚编排器，也不是文件恢复执行器，而是恢复点能力的最小收口层。

---

## 2. 本模块负责什么

本模块当前只负责以下内容：

1. 恢复点最小结构定义
2. 恢复点输入整理
3. 恢复点候选生成
4. 为上层提供统一 checkpoint service 入口

---

## 3. 本模块不负责什么

本模块明确不负责：

- 风险判断
- 审批流程
- storage 实现细节本身
- task / run 状态流转
- artifact 正式交付
- 前端恢复点页面展示

一句话说：

> `checkpoint` 只负责“恢复点是什么、什么时候应该准备”，不负责“完整回滚流程如何推进”。

---

## 4. 当前输入输出

### 4.1 输入

当前最小输入建议包含：

- `task_id`
- `summary`
- `objects`

### 4.2 输出

当前输出建议对齐 `RecoveryPoint`：

- `recovery_point_id`
- `task_id`
- `summary`
- `created_at`
- `objects`

说明：

- 本模块已经具备最小 writer + service 能力；
- 不要在这里自行发明协议真源之外的正式字段。

---

## 5. 已冻结规则

### 5.1 协议概念

- `recovery_point`

### 5.2 主链路约束

主链路要求 dashboard 最终可看到：

- `task`
- `artifact`
- `audit`
- `recovery_point`

### 5.3 存储职责

根据架构文档，SQLite + WAL 负责：

- checkpoint
- 审计
- 授权记录

---

## 6. 当前未冻结规则

以下规则当前未冻结，不应擅自定死：

1. 恢复点对象的最小粒度（文件级、目录级、任务级）
2. 恢复点创建时机（执行前统一创建，还是按风险等级条件创建）
3. 恢复点是否支持真正的一键恢复，以及恢复动作归属哪个模块
4. 恢复点与 artifact / audit 的关系是否需要强绑定

---

## 7. 模块内待完成清单

### P0

- 补更清晰的 `BuildCreateInputFromCandidate(...)` 示例样例
- 明确 `required=false` 时在上层执行链中的消费策略
- 明确 storage 读侧结果在 orchestrator / security 视图中的消费规则

### P1

- 增加 checkpoint candidate 构造 helper
- 补更明确的对象分类策略

### P2

- 为真正回滚流程做扩展准备

---

## 8. 跨模块待完成清单

### P0

- `tools` / `execution` 已能产出并消费最小 checkpoint candidate
- `storage` 已具备 recovery point 的写侧和读侧能力
- dashboard / security 页读取 recovery point 列表

### P1

- 与 `audit` 记录联动
- 与授权流和恢复流联动

---

## 9. 模块自检清单

- 有没有把风险判断写进 checkpoint？
- 有没有把真正回滚执行逻辑写进 checkpoint？
- 有没有发明协议真源外字段？
- 有没有越界到 storage / orchestrator 实现细节？

若答案不明确，不要直接开写。
