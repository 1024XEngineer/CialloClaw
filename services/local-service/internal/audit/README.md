# audit 模块 README

## 1. 模块定位

`/services/local-service/internal/audit` 是 CialloClaw 后端治理与安全层中的**审计记录模块**。

本模块的职责是把已经发生、已经判定完成、已经执行完成的操作，整理成统一、稳定、可追踪的审计记录输入，并通过注入的 writer 输出给持久化层。

在 `task-centric` 架构中：

- 对外界面围绕 `task_id` 组织；
- `audit` 只负责“记录什么”，不负责“何时执行”和“如何推进任务状态”。

---

## 2. 本模块负责什么

本模块当前只负责以下内容：

1. 审计记录输入收口
   - 对文件、命令、工具执行等已完成动作生成统一审计输入

2. 审计记录最小结构定义
   - 对齐 `audit_record` 概念，保证后续存储层和 RPC 层可稳定消费

3. 审计摘要标准化
   - 给上层提供一致的 `type / action / summary / target / result` 语义

---

## 3. 本模块不负责什么

本模块明确不负责：

- 风险判断
- 审批流转
- checkpoint 创建
- artifact 正式生成
- task / run / step 状态机推进
- 前端安全页展示
- storage 实现细节本身

一句话说：

> `audit` 只负责整理“要写什么审计记录”，不负责“何时写”和“写到哪里”。

---

## 4. 当前输入输出

### 4.1 输入

当前建议的最小输入应围绕以下已冻结概念组织：

- `task_id`
- `type`
- `action`
- `summary`
- `target`
- `result`

### 4.2 输出

当前建议输出对齐 `AuditRecord`：

- `audit_id`
- `task_id`
- `type`
- `action`
- `summary`
- `target`
- `result`
- `created_at`

说明：

- 当前模块已经具备最小 writer + service 能力；
- 不能在这里自行发明协议层不存在的正式字段。

---

## 5. 已冻结规则

### 5.1 协议概念

- `audit_record`

### 5.2 协议返回位置

安全类接口可以返回：

- `approval_request`
- `authorization_record`
- `audit_record`
- `recovery_point`
- 必要时附带 `impact_scope`

### 5.3 存储职责

根据架构文档，SQLite + WAL 负责：

- 结构化运行态
- 审计
- 授权记录
- checkpoint

---

## 6. 当前未冻结规则

以下规则当前未冻结，不应擅自定死：

1. `audit_id` 的生成方式
2. 同一 task 下审计记录的粒度（按工具、按动作、按阶段）
3. 审计写入时机（执行中、执行后、失败后）
4. 审计记录与 ToolCall / Event 的最终映射关系

---

## 7. 模块内待完成清单

### P0

- 补更清晰的 `BuildRecordInputFromCandidate(...)` 示例样例
- 明确 writer 错误在上层执行链中的统一处理方式
- 明确 storage 读侧结果在 orchestrator / security 视图中的消费规则

### P1

- 增加 audit record 构造 helper
- 对齐与 ToolCall / risk / checkpoint 的关系说明

### P2

- 根据主链路需要扩展更多审计分类

---

## 8. 跨模块待完成清单

### P0

- `tools` / `execution` / `orchestrator` 已能产出并消费最小 audit candidate
- `storage` 已具备 audit 的写侧和读侧能力
- `rpc` / dashboard 安全页读取 audit 数据

### P1

- 与 `checkpoint`、`risk` 结果做联动视图
- 与 `ToolCall`、`Event` 做统一关联

---

## 9. 模块自检清单

- 有没有把风险判断写进 audit？
- 有没有把 checkpoint 创建写进 audit？
- 有没有擅自发明协议字段？
- 有没有越界到 orchestrator / storage 实现细节？

若答案不明确，不要直接开写。
