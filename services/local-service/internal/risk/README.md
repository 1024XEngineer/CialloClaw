# risk 模块 README

## 1. 模块定位

`/services/local-service/internal/risk` 是 CialloClaw 后端治理与安全层中的**风险判断模块**。

本模块的职责不是执行工具，也不是推进主链路状态，而是对“某个操作是否安全、是否需要人工确认、是否应直接拦截”给出统一判断结果，供上层编排层消费。

在 `task-centric` 架构中：

- 对外主对象是 `task`；
- 后端保留 `run / step / event / tool_call` 兼容执行层；
- `risk` 模块只负责**风险评估**，不直接接管状态机。

---

## 2. 本模块负责什么

本模块当前只负责以下内容：

1. 风险等级判断
   - 按统一风险等级输出：`green` / `yellow` / `red`

2. 风险原因归一化
   - 对 workspace 越界、覆盖/删除风险、危险命令、能力缺失等场景给出统一 reason

3. 最小影响面收口
   - 用 `ImpactScope` 表示本次操作的文件、网页、应用及是否越界、是否存在覆盖/删除风险

4. 审批建议输出
   - 给出 `approval_required` / `deny` 这样的判断结果
   - 由上层决定是否真正进入审批流

---

## 3. 本模块不负责什么

本模块明确不负责：

- `intent` 识别
- `orchestrator / runengine` 状态机流转
- `delivery_result` 编排
- `ApprovalRequest` / `AuthorizationRecord` 的真实持久化写入
- audit / checkpoint / artifact 的写入与消费
- 前端协议消费与 UI 提示
- 直接阻断或执行工具调用本身

一句话说：

> `risk` 只负责“判断风险”，不负责“推进流程”。

---

## 4. 依赖关系与边界

### 4.1 允许依赖

本模块可以依赖：

- `/packages/protocol`（概念对齐，不复制协议真源）
- `/packages/config`
- 必要的标准库

### 4.2 被谁依赖

本模块通常会被以下模块依赖：

- `orchestrator`
- `tools`（如工具执行前预判断）
- 后续可能的 `security` / `audit` 聚合层

### 4.3 禁止依赖

本模块不得依赖：

- `apps/desktop/*`
- 前端页面、store、RPC client
- `delivery` 的正式交付逻辑
- `runengine` 的内部持久化实现细节

---

## 5. 当前输入输出

## 5.1 输入

当前主输入是 `AssessmentInput`：

- `operation_name`
- `target_object`
- `capability_available`
- `command_preview`
- `impact_scope`

其中 `impact_scope` 当前包含：

- `files`
- `webpages`
- `apps`
- `out_of_workspace`
- `overwrite_or_delete_risk`

说明：

- 当前输入只保留风险判断真正需要的信息；
- 不直接传整套 task/run 状态，避免把业务状态机侵入 risk 模块。

## 5.2 输出

当前主输出是 `AssessmentResult`：

- `risk_level`
- `approval_required`
- `deny`
- `reason`
- `impact_scope`

说明：

- 这是 risk 模块内部的最小判断结果；
- 不直接替代正式协议对象；
- 上层可基于它构造统一错误、审批请求或安全摘要。

---

## 6. 当前已实现规则

当前 `Service.Assess(...)` 的最小规则如下：

1. `capability_available = false`
   - 输出：`red + deny`
   - `reason = capability_denied`

2. `command_preview` 命中危险命令模式
   - 输出：`red + deny`
   - `reason = command_not_allowed`

3. `impact_scope.out_of_workspace = true`
   - 输出：`red + approval_required`
   - `reason = out_of_workspace`

4. `impact_scope.overwrite_or_delete_risk = true`
   - 输出：`yellow + approval_required`
   - `reason = overwrite_or_delete_risk`

5. 其他情况
   - 输出：`green`
   - `reason = normal`

---

## 7. 已冻结规则

以下内容当前已冻结，应严格遵守：

### 7.1 风险等级

- `green`
- `yellow`
- `red`

### 7.2 相关错误码

- `1004001` `APPROVAL_REQUIRED`
- `1004002` `APPROVAL_REJECTED`
- `1004003` `WORKSPACE_BOUNDARY_DENIED`
- `1004004` `COMMAND_NOT_ALLOWED`
- `1004005` `CAPABILITY_DENIED`

### 7.3 相关协议概念

- `ApprovalRequest`
- `AuthorizationRecord`
- `ImpactScope`

注意：

- 本模块内部类型可以对齐这些概念；
- 但不能在本模块里重新发明一套协议真源。

---

## 8. 当前未冻结规则

以下规则目前**未冻结**，后续实现不得擅自编造为最终产品逻辑：

1. workspace 外操作最终是：
   - 一律直接拒绝；
   - 还是 `red` 后允许人工审批；
   - 还是按工具类别区分；

2. 覆盖/删除风险最终统一是 `yellow` 还是在某些场景升为 `red`

3. 危险命令黑名单的最终范围

4. `risk` 是否最终直接产出 `ApprovalRequest`，还是只输出 assessment 结果供上层转换

5. `ImpactScope` 是否还要扩展网页、应用、覆盖对象更多细节

---

## 9. 模块内待完成清单

### P0

- 增加 `README` 对应的示例输入输出样例，便于后续 AI 与人工统一理解
- 为危险命令判断补更完整的 table-driven tests
- 增加对 `ImpactScope.Files` / `TargetObject` 的一致性校验

### P1

- 抽离更明确的规则函数，如：
  - `assessCapabilityRisk(...)`
  - `assessCommandRisk(...)`
  - `assessWorkspaceRisk(...)`
- 增加更清晰的 reason 常量与注释文档
- 为后续 `ApprovalRequest` 映射准备更明确的内部转换辅助函数

### P2

- 如果规则继续增长，再考虑拆成 `rules/` 子目录
- 增加更多命令模式与能力不可用场景覆盖

---

## 10. 跨模块待完成清单

### P0

- `orchestrator` 真实接入 `risk.Service.Assess(...)`，不再只依赖临时或分散判断
- `tools` 与 `risk` 的规则收口，避免各自维护一套命令风险与 workspace 风险逻辑
- 上层将 `AssessmentResult` 正式映射到：
  - `APPROVAL_REQUIRED`
  - `WORKSPACE_BOUNDARY_DENIED`
  - `COMMAND_NOT_ALLOWED`
  - `CAPABILITY_DENIED`

### P1

- `audit` / `checkpoint` / `security` 聚合层消费 risk 输出
- `storage` 或协议层决定是否记录风险评估快照
- `rpc` 层按统一协议返回 `approval_request` / `impact_scope`

### P2

- 与更完整的审批流、恢复流、安全摘要页做联动
- 评估是否把风险评估规则配置化

---

## 11. 模块自检清单

开始修改 `risk` 模块前，先检查：

- 这是风险判断逻辑，还是状态机逻辑？
- 有没有擅自生成协议真源中未登记的字段？
- 有没有把审批流或持久化逻辑写进 risk 模块？
- 有没有写死平台路径或命令平台差异？
- 有没有越界到 `orchestrator` / `audit` / `checkpoint`？

只要其中任一项答案不明确，就不要直接开写。
