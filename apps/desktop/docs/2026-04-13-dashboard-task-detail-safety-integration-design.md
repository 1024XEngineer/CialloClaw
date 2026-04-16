# Dashboard 任务详情与安全联动联调设计

## 1. 目标

在不改变当前 dashboard live 主链路的前提下，完成 `apps/desktop` 中 `/tasks` 与 `/safety` 的一次最小但真实的联调补强，使任务详情能够稳定承接后端真实数据，并把“安全详情”从泛跳转升级为“跳到对应安全项”的精确跳转。

本次设计同时冻结一个重要产品边界：

- dashboard 不提供直接修改 task 的正式能力
- task 的后续修改仍由悬浮球承接
- `/tasks` 页只负责浏览、控制、查看安全详情与结果摘要

## 2. 背景与真源

### 2.1 仓库级真源

本设计依据以下真源约束：

- `AGENTS.md`
- `docs/architecture-overview.md`
- `docs/development-guidelines.md`
- `docs/protocol-design.md`
- `docs/data-design.md`
- `docs/module-design.md`
- `docs/work-priority-plan.md`
- `docs/atomic-features.md`

这些真源约束要求本次设计必须保持：

- 对外主对象仍然是 `task`
- 前后端稳定边界仍然是 `JSON-RPC 2.0`
- dashboard 不能平行造链
- 风险治理仍然通过安全链路统一承接

### 2.2 当前前端实现

当前 desktop dashboard 已存在真实页面链路：

- `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx`
- `apps/desktop/src/features/dashboard/tasks/taskPage.service.ts`
- `apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx`
- `apps/desktop/src/features/dashboard/safety/SecurityApp.tsx`

其中：

- `/tasks` 已接入 `agent.task.list`、`agent.task.detail.get`、`agent.task.control`
- `/safety` 已接入 `agent.security.summary.get`、`agent.security.pending.list`、`agent.security.respond`
- `/tasks` 的“安全详情”当前只会跳到安全页首页，不能定位到具体审批项或恢复点
- `/tasks` 详情页仍保留较多 fallback 文案和兜底说明

### 2.3 当前后端实现

当前 local-service 已存在真实接口骨架：

- `agent.task.list`
- `agent.task.detail.get`
- `agent.task.control`
- `agent.security.summary.get`
- `agent.security.pending.list`
- `agent.security.respond`

其中 `agent.task.detail.get` 当前已返回：

- `task`
- `timeline`
- `artifacts`
- `mirror_references`
- `security_summary`

但当前详情返回仍缺少“精确定位到某条安全审批”的关键字段。

## 3. 当前问题

### 3.1 任务页无法精确跳到对应安全项

`TaskDetailPanel` 里的“安全详情”动作当前只能跳到 `/safety` 首页，无法自动展开当前 task 对应的那一条审批卡片，也无法优先定位到当前 task 的恢复点。

### 3.2 任务详情的安全摘要只有数量，没有锚点

当前 `task detail` 返回的 `security_summary` 只有：

- `security_status`
- `risk_level`
- `pending_authorizations`
- `latest_restore_point`

其中 `pending_authorizations` 只是数量，前端无法据此知道当前应跳到哪一条 `approval:${approval_id}`。

### 3.3 dashboard 内直接修改 task 不符合当前产品边界

当前 `TaskActionBar` 保留了 disabled 的“修改任务”动作，但后端并没有设计 dashboard 内直接修改任务的正式 RPC。按当前产品设想，用户应通过悬浮球继续发起“修改这条任务”的新承接，而不是在仪表盘中直接改写任务。

### 3.4 跨页返回时存在旧缓存风险

`/safety` 成功处理授权后，`/tasks` 需要反映最新状态；否则用户回到任务页时可能看到旧的 task status 或旧的 detail。当前实现里 tasks 页 query 配置偏保守，联调时必须明确这条跨页一致性策略。

## 4. 范围与非目标

### 4.1 本次范围

本次联调只覆盖以下内容：

1. 补强 `agent.task.detail.get`，让任务详情能提供精确安全跳转所需信息
2. 从 `/tasks` 精确跳转到 `/safety` 对应审批项或恢复点
3. 在 `/safety` 消费来自 `/tasks` 的定位信息并自动展开对应详情
4. 处理授权后保持 `/tasks` 与 `/safety` 的状态一致
5. 冻结“dashboard 不直接修改 task”的边界

### 4.2 本次明确不做

- 不新增 dashboard 内直接修改 task 的 RPC
- 不新增独立的 task detail 新页面主链路
- 不新增 dashboard 专用聚合详情接口
- 不新增 `agent.task.artifact.list` / `agent.task.artifact.open` 这类 planned 能力
- 不实现完整安全审计列表、恢复点列表或恢复点应用链路
- 不把 `/safety` 做成 URL 深链接系统

## 5. 设计总览

### 5.1 核心思路

本次联调继续沿用现有 live 主路径：

- `/tasks` 负责 task-centric 浏览与控制
- `/safety` 负责风险、授权与恢复点视图
- 两者通过现有稳定 RPC 交互

设计选择为：

- 不新增新的详情主接口
- 在现有 `agent.task.detail.get` 上补最小必要字段
- 使用 route state 在前端页面间传递“一次性定位信息”

这样可以在最小改动下保持主链路统一，不引入 dashboard 私有平行对象。

### 5.2 页面职责分工

`/tasks` 负责：

- 列表浏览
- 详情查看
- pause / resume / cancel / restart
- 从 task 详情跳转到相关安全项

`/safety` 负责：

- 展示待审批授权
- 展示恢复点摘要
- 响应授权 allow / deny
- 回放 task 视角下的安全治理结果

`/tasks` 不负责：

- 直接修改 task
- 直接执行安全审批
- 自己拼装完整安全分页视图

## 6. 协议与返回结构设计

### 6.1 总体原则

本次不新增 JSON-RPC 方法，继续使用：

- `agent.task.detail.get`
- `agent.security.summary.get`
- `agent.security.pending.list`
- `agent.security.respond`

本次只在 `agent.task.detail.get` 的正式返回结构中补最小必要字段。

### 6.2 `AgentTaskDetailGetResult` 的补强

当前：

```ts
type AgentTaskDetailGetResult = {
  task: Task;
  timeline: TaskStep[];
  artifacts: Artifact[];
  mirror_references: MirrorReference[];
  security_summary: SecuritySummary;
};
```

本次建议补强为：

```ts
type AgentTaskDetailGetResult = {
  task: Task;
  timeline: TaskStep[];
  artifacts: Artifact[];
  mirror_references: MirrorReference[];
  security_summary: SecuritySummary;
  approval_request: ApprovalRequest | null;
};
```

新增字段语义：

- `approval_request` 只表示“当前 task 关联的正在等待处理的那一条授权请求”
- 如果当前 task 没有 pending approval，则返回 `null`
- 它不是安全页 pending 列表的镜像，也不是分页结果替代物

当前阶段额外冻结一条约束：

- 单个 `task` 在 P0 主链路下只暴露一个“当前有效的 pending approval anchor”
- 因而 task detail 中的 `pending_authorizations` 对当前 task 视角应收敛为 `0 | 1`
- 如果未来治理链路允许一个 task 同时存在多条未决审批，则 `approval_request` 仍只返回当前主锚点，选择规则固定为“最新创建、仍未决、且与当前 task 直接关联的那一条”
- 多条审批的完整列表仍必须留在安全域接口中处理，而不是回流到 task detail

### 6.3 `security_summary.latest_restore_point` 的收口

本次要求在 task detail 返回里，`security_summary.latest_restore_point` 稳定为：

- `RecoveryPoint | null`

不再接受只返回字符串 ID 的宽松语义。原因是任务页跳转和详情展示都需要稳定读取：

- `recovery_point_id`
- `task_id`
- `summary`
- `created_at`

如果 runengine 当前只有局部信息，则由 orchestrator 负责尽量从 storage 补齐；补不齐时返回 `null`，不要让前端猜测。

### 6.4 本次不补的协议能力

本次明确不补以下字段或接口：

- task 对应的完整 pending approvals 列表
- dashboard 私有安全详情 read model
- dashboard 内 task 编辑协议
- 审计记录列表协议
- 恢复点列表协议

## 7. 后端设计

### 7.1 `agent.task.detail.get` 继续作为唯一详情主接口

后端继续在 `services/local-service/internal/orchestrator/service.go` 的 `TaskDetailGet` 中装配任务详情，不新增新的 dashboard detail handler。

返回结构仍围绕 `task` 组织，不把 `run / event / tool_call` 直接暴露给前端长期消费。

### 7.2 `approval_request` 的装配来源

`approval_request` 的真源是 task 治理链路中的正式 `approval_request` 对象，而不是前端局部状态，也不是临时 UI 推断。

当前阶段的装配规则为：

- 优先读取 task 记录或持久化 task-run 快照中挂接的正式 `approval_request` 对象
- 当前实现若由 runengine 承载该对象，则读取的是 runengine 中 task record 的正式治理字段，而不是其他瞬时状态
- 若 task 当前不在等待授权态，则返回 `null`

这条字段只承担“跳到哪一条审批卡片”的职责，不承担列表查询职责。

### 7.3 `latest_restore_point` 的装配规则

`latest_restore_point` 装配规则保持：

1. 优先使用 task record 内已有恢复点对象
2. 如果 task 内没有完整对象，则尝试从 storage 补齐当前 task 对应的最新恢复点
3. 仍无结果则返回 `null`

### 7.4 task-centric 边界

后端对外仍只返回 task 详情视图所需对象：

- `task`
- `timeline`
- `artifacts`
- `mirror_references`
- `security_summary`
- `approval_request`

本次不允许把内部 run 状态、event 列表或 tool call 列表直接塞进 dashboard detail 长期使用。

## 8. 前端设计

### 8.1 `/tasks` 继续作为详情入口

`apps/desktop/src/features/dashboard/tasks/TaskPage.tsx` 继续负责：

- 拉取 task detail
- 打开详情弹层
- 处理主动作按钮

本次不新增第二个 task 详情入口。

### 8.2 “安全详情”跳转载荷

从 `/tasks` 跳到 `/safety` 时，使用 route state 传递一次性定位信息，而不是新增 query string 或新协议字段。

建议状态结构：

```ts
type SafetyNavigationState = {
  source: "task-detail";
  taskId: string;
  approvalRequest?: ApprovalRequest;
  restorePoint?: RecoveryPoint;
};
```

使用规则：

- 有 `approval_request` 时，直接带完整 `approvalRequest` 快照
- 否则有 `latest_restore_point` 时，直接带完整 `restorePoint` 快照
- 两者都没有时，只带 `taskId` 或直接空 state 跳总览

选择完整对象而不是只带 ID 的原因是：

- `agent.security.pending.list` 当前是分页读取，不能保证目标审批一定在首屏
- `agent.security.summary.get` 当前只稳定返回全局摘要，不能保证 task 对应恢复点就是当前 summary 上那一条
- route state 已经由 task detail 的真实 RPC 数据驱动，带完整对象可以让安全页稳定落到“对应那一条”的详情，而不是 best-effort 猜测

### 8.3 `/safety` 的定位消费逻辑

`apps/desktop/src/features/dashboard/safety/SecurityApp.tsx` 在加载完 `moduleData` 后消费一次 route state：

- 有 `approvalRequest`：优先打开这条审批的详情视图
- 有 `restorePoint`：优先打开这条恢复点的详情视图
- 都没有：停留在安全总览

消费成功或消费结束后应清空 route state，避免重复展开。

这里的“详情视图”采用双层对齐：

1. 先以 route state 中的对象快照作为精确焦点，保证用户一定能看到“对应那一条”
2. 若当前安全页实时数据中存在同 ID 的 live 项，再把焦点绑定到 live 卡片或 live 恢复点摘要上

这样可以保证：

- 精确定位不依赖分页首屏命中
- 精确定位不依赖全局 summary 是否正好等于当前 task 对应恢复点
- 安全页仍然可以在随后用 live 数据覆盖同 ID 的详情表现

### 8.4 未命中的降级策略

#### `approvalRequest` 未命中 live 列表

可能原因：

- 该审批已被处理
- 当前 pending 首屏尚未包含它
- 最新数据尚未刷新

处理策略：

1. 先直接展示 route state 中携带的审批详情快照
2. 在 RPC 模式下继续做一次受保护刷新
3. 若刷新后仍未在 live pending 中命中，则保留这条快照详情，并提示“该授权可能已被处理，当前显示的是进入安全页时的任务详情快照”

#### `restorePoint` 未命中当前 summary

处理策略：

1. 先直接展示 route state 中携带的恢复点详情快照
2. 若当前 summary 的 `latest_restore_point` 与该快照同 ID，则切回 live 恢复点详情
3. 若不同 ID 或 summary 为空，则继续展示该快照详情，并标注“当前全局安全摘要中的恢复点已变化”

这意味着本次联调中的“精确跳到对应那一条”依赖的是 task detail 提供的真实对象快照，而不是要求安全页自己的分页 / summary RPC 必须天然命中同一条对象。

### 8.5 task 详情页 fallback 策略

如果 task detail 还未完成加载或 detail 请求失败：

- 仍允许进入 `/safety`
- 但不再尝试精确跳转到审批项或恢复点
- 页面只落到安全总览，并给出轻量提示

这可以避免用户因为详情暂时失败而失去整个安全入口。

### 8.6 dashboard 内不直接修改 task

当前 `TaskActionBar` 中 disabled 的“修改任务”不应在产品语义上暗示“即将支持 dashboard 直接编辑 task”。

本次边界冻结为：

- 不新增 `agent.task.edit`
- 不新增 dashboard 内 task mutation
- 如后续需要继续保留该入口，应改成“去悬浮球继续修改”这类引导语义，而不是直接编辑语义

## 9. 跨页一致性策略

### 9.1 问题定义

用户在 `/safety` 完成 `allow_once` 或 `deny_once` 后，返回 `/tasks` 时必须看到最新 task status、最新 security summary 和最新详情结果。

### 9.2 一致性要求

本次设计要求：

- `/safety` 成功调用 `agent.security.respond` 后，`/tasks` 相关缓存必须被视为 dirty
- 返回 `/tasks` 时必须触发一次真实刷新，不能长期停留在旧 cache

### 9.3 冻结实现策略

本次联调冻结采用同一套机制，不保留双方案：

1. `/safety` 在 `agent.security.respond` 成功后，主动失效以下 tasks 相关 query：
   - `['dashboard', 'tasks', 'bucket']`
   - `['dashboard', 'tasks', 'detail']`
2. `/tasks` 在 RPC 模式下重新进入页面时，task bucket 与 task detail 查询必须允许 mount refetch，确保失效后的 query 能真正回源
3. 如果未来 tasks 页保持挂载而不是卸载，以上失效动作也应立即触发活跃查询刷新

这条策略的结果必须是：安全响应成功后，无论用户立刻返回 `/tasks`，还是稍后再次进入 `/tasks`，都不会长期停留在旧的 task 列表或旧的详情缓存上。

## 10. 风险与边界

### 10.1 风险一：把安全页列表塞回 task detail

如果为了定位方便把完整 pending 列表塞进 task detail，会让 `agent.task.detail.get` 失去边界，变成安全页分页镜像。本次明确禁止这种扩张。

### 10.2 风险二：重新发明 dashboard 私有任务编辑链路

如果在 dashboard 内引入直接 task edit，会与当前悬浮球承接链路冲突，也会逼迫后端新增尚未设计好的 mutation 能力。本次明确不做。

### 10.3 风险三：跨页状态不同步

如果只在安全页本地更新 UI，而不刷新 task 相关缓存，用户回到任务页时会看到旧状态，破坏联调可信度。本次必须把该一致性作为正式验收项。

## 11. 验收标准

### 11.1 任务页到安全页跳转

- 对于 `waiting_auth` 的 task，点击“安全详情”后可进入 `/safety` 并自动打开当前 task 对应的那条审批详情；若 live pending 中仍存在同 ID 项，则同步高亮到对应 `approval:${approval_id}`
- 对于无 pending approval 但有恢复点的 task，点击“安全详情”后进入 `/safety` 并优先落到当前 task 对应的恢复点详情；若当前安全 summary 恰好是同一恢复点，则切换为 live 恢复点视图
- 对于既无 pending approval 又无恢复点的 task，点击“安全详情”后正常进入安全总览，不报错

### 11.2 安全页行为

- 定位信息命中时自动展开对应详情
- 即使 live pending / summary 没有命中同一条对象，也能先展示来自 task detail 的那条安全对象快照，再给轻提示说明 live 数据已变化或对象已被处理
- `approval.pending` 实时行为不被破坏

### 11.3 跨页一致性

- 在 `/safety` 中 allow / deny 某条授权后，再回 `/tasks`，列表与详情必须反映最新状态
- 不允许持续显示旧的 `waiting_auth`、旧的 pending 数量或旧的 detail 摘要

### 11.4 产品边界

- dashboard 中不出现可用的“直接修改任务”正式能力
- 任务修改语义仍由悬浮球承接

## 12. 受影响文件

本次联调设计预计影响以下位置：

- 协议：`packages/protocol/rpc/methods.ts`
- 后端：`services/local-service/internal/orchestrator/service.go`
- 前端 task service：`apps/desktop/src/features/dashboard/tasks/taskPage.service.ts`
- 前端 task page：`apps/desktop/src/features/dashboard/tasks/TaskPage.tsx`
- 前端 safety page：`apps/desktop/src/features/dashboard/safety/SecurityApp.tsx`

如涉及正式协议字段变更，实施时还需同步回写仓库真源文档，而不能只改代码。

## 13. 结论

本次联调选择的是“沿现有主链路补强，而不是新开一条 dashboard detail 链路”的方案。

具体来说：

- 用 `agent.task.detail.get` 补一个最小但关键的 `approval_request`
- 把 task detail 中的恢复点对象收口为稳定可定位结构
- 让 `/tasks` 通过 route state 精确跳到 `/safety` 对应卡片
- 保持 dashboard 不直接修改 task 的产品边界

这样可以在不破坏 task-centric 主链路的前提下，让任务详情、安全详情和授权处理形成一个真实、可验证、可持续扩展的联调闭环。
