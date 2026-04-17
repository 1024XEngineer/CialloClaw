# AGENTS.md

本文件只补充 `services/local-service` 及其子目录的后端专项规则。

进入本目录前，必须先读根目录 `AGENTS.md`。
本文件不重复共享铁律，只补充后端专属边界、阅读路径和注释要求。

------

## 1. 目录范围

本文件覆盖：

- `services/local-service/cmd`
- `services/local-service/internal`

若当前任务会同时修改协议真源、前端承接方式、共享 schema 或跨端时序，必须回到根目录 `AGENTS.md`，并补读相关上位文档，不得仅凭本文件决策。

------

## 2. 后端工作前先确认什么

开始修改前，至少先回答：

1. 当前改动属于哪个后端域：rpc、orchestrator、context、intent、delivery、risk、memory、storage、runengine，还是其他服务？
2. 这次改动影响的是 `task` 对外语义、`run` 内部执行语义，还是两者映射关系？
3. 是否新增或改变了正式协议对象、状态、错误码或数据字段？
4. 是否涉及工具调用、授权、审计、恢复点或正式交付出口？
5. 工具结果会如何回流到 `tool_call / event / delivery_result` 链？

若任一问题回答不清，先补读：

- `docs/module-design.md`
- `docs/development-guidelines.md`
- `docs/work-priority-plan.md`
- 触及协议时补读 `docs/protocol-design.md` 与 `packages/protocol`
- 触及数据时补读 `docs/data-design.md`

------

## 3. 后端边界

后端负责：

- JSON-RPC Server
- Orchestrator
- Context Manager
- Intent / Planning
- RunEngine / Task 状态机
- Delivery
- Memory
- Risk / Audit / Recovery
- Storage
- Trace / Eval
- Tool / Plugin / Sandbox / Adapter

后端不得：

- 感知前端页面组件树
- 依赖 Tauri 细节
- 输出未登记对象给前端长期消费
- 绕过正式交付出口直接把工具原始结果当最终结果

必须坚持：

- 保持 `task` 与 `run` 稳定映射
- 让工具结果先进入 `tool_call / event / delivery_result` 链
- 让风险动作可授权、可审计、可恢复
- 让错误进入正式错误码体系

禁止：

- 绕过编排器直连前端
- 绕过交付内核直接回结果
- 把记忆逻辑侵入运行态状态机
- 把平台细节写死在业务层
- 让 intent 退化成固定关键词分类器并替代 Agent Loop 主执行路径

------

## 4. 后端实现优先级

实现顺序必须优先满足：

1. 保住 `task-centric` 主链路和 `run` 执行兼容链
2. 保住 JSON-RPC 契约、错误码和数据真源的一致性
3. 保住工具调用、风险治理、交付出口和事件回流的完整性
4. 在不破坏以上三项的前提下，再做性能、扩展性和外围增强

如果某个实现需要临时绕开风险、交付或正式对象边界才能跑通，该实现不能进入正式分支。

------

## 5. 后端注释规范

注释在后端不是可选项，尤其以下场景必须写英文注释：

- 状态机迁移、确认语义、重试与恢复逻辑
- 工具调用回流、事件生成、交付对象构建
- 授权、审计、恢复点、风险分级相关逻辑
- 并发控制、锁、队列、超时、重放、幂等处理
- 存储映射、字段语义、兼容边界和错误包装

必须坚持：

- 新增复杂逻辑时同步写英文注释
- 调整复杂逻辑时同步检查并修正旧注释
- 当前改动范围内若发现中文注释，必须改成英文注释

禁止：

- 用“函数名已经很清楚”作为不写注释的理由
- 让关键失败路径、恢复路径和授权路径缺少解释
- 让注释与实际状态迁移、错误处理或数据语义失真

------

## 6. 后端自检清单

提交前至少确认：

- `task` 与 `run` 的映射是否保持稳定
- 是否新增了协议 / 状态 / 错误码 / 字段；若有，是否已同步真源与文档
- 工具结果是否正确回流到 `tool_call / event / delivery_result`
- 风险动作是否经过授权、审计和恢复点链路
- 当前改动是否补齐所需英文注释
- 是否补到了对应的测试或失败路径验证

------

## 7. 一句话总原则

> 后端先守住协议、编排、治理和交付边界，再考虑实现细节的扩展与优化。
