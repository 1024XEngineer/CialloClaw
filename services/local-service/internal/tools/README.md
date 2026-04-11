# tools 模块 README

## 1. 模块定位

`/services/local-service/internal/tools` 是 CialloClaw 后端 Harness 的工具能力接入层。

本项目采用 `task-centric` 主链路：

- 对外产品与协议以 `task` 为主对象；
- 后端保留 `run / step / event / tool_call` 作为执行兼容层；
- `tools` 模块不负责主链路编排，只负责“工具如何被注册、适配、执行、接入外部 worker / sidecar”。

本模块属于 5 号负责人范围，职责必须稳定，避免侵入 `intent`、`orchestrator`、`runengine`、`delivery` 等模块。

---

## 2. 本模块负责什么

本模块只负责以下 5 类内容：

1. `tool registry`
   - 维护工具注册表；
   - 维护工具名称、工具能力、执行入口与元数据；
   - 统一暴露可执行工具集合。

2. `tool adapter`
   - 把内置能力、平台能力、外部 worker 能力适配成统一工具接口；
   - 屏蔽具体实现差异，向上只暴露稳定工具契约。

3. `tool executor facade`
   - 作为工具执行统一入口；
   - 统一处理执行前校验、执行结果归一化、错误归一化、超时/不可用等公共逻辑；
   - 保证任何一次工具执行都经过统一出口。

4. `builtin tools`
   - 放置本地内置工具实现；
   - 例如文件读取、工作区内文本处理、简单系统能力桥接等。

5. `worker client / sidecar client`
   - 负责与 Playwright、OCR、媒体处理等独立 worker / sidecar 建立调用客户端；
   - 负责协议适配、请求发送、结果接收、错误映射；
   - 不负责 worker 生命周期的业务编排决策。

---

## 3. 本模块不负责什么

本模块明确不负责以下内容：

- `intent` 识别；
- `orchestrator / runengine` 状态机流转；
- `delivery_result` 编排；
- 前端协议消费；
- task / run / step / event 的主链路状态推进；
- 风险审批、授权恢复、审计摘要；
- 模型调用与 Prompt 编排；
- 平台实现细节的散落式调用。

一句话说：

> `tools` 模块只管“工具能力如何统一接入和执行”，不管“什么时候执行、执行后如何推进任务、如何正式交付”。

---

## 4. 依赖关系与边界

### 4.1 允许依赖

本模块可以依赖：

- `/packages/protocol`
- `/packages/config`
- `/services/local-service/internal/platform`
- `/workers/*` 的受控客户端接入层

### 4.2 被谁依赖

本模块通常被以下模块依赖：

- `orchestrator`
- `model`（如 tool calling 结果适配）
- `plugin`

### 4.3 禁止依赖

本模块不得依赖：

- `apps/desktop/*`
- 前端页面、store、RPC client
- `orchestrator / runengine` 的内部状态实现细节
- `delivery` 的正式交付编排逻辑

---

## 5. 统一约束

### 5.1 tool 名称必须使用 `snake_case`

所有工具名称必须使用 `snake_case`，包括但不限于：

- registry 中的 key
- worker / sidecar 暴露的 tool name
- `ToolCall.tool_name`

禁止：

- `camelCase`
- `PascalCase`
- 混合风格或临时别名

### 5.2 所有输出结构服从 `/packages/protocol`

本模块产生的输入、输出、错误、产物引用、工具执行记录，必须能映射到 `/packages/protocol` 已定义结构。

尤其要遵守：

- 不得自行发明临时 JSON；
- 不得新增未登记字段冒充协议结果；
- 如果协议层没有对应字段，先补 `/packages/protocol`，再改实现。

### 5.3 所有工具执行都必须产生 `ToolCall` 记录

本模块内约定：

- 任何一次工具执行都必须经过统一 executor facade；
- 任何一次工具执行都必须产生一条 `ToolCall` 记录；
- 不能出现“实际执行了工具，但没有 `ToolCall` 记录”的静默路径。

如果当前 `ToolCall` 最终由上层写入运行态，则 `tools` 模块必须返回完整、稳定、可映射的数据，不允许上层靠字符串猜测执行结果。

### 5.4 平台相关逻辑必须通过 `platform adapter` 注入

凡是涉及以下能力，必须通过 `platform` 抽象注入：

- 文件系统
- 路径合法性与 workspace 边界
- 外部命令启动
- sidecar 生命周期管理
- 本地 IPC / 进程能力

禁止在工具实现里直接散落：

- `os/exec` 启动外部进程的业务逻辑
- 写死平台路径
- 写死盘符 / 分隔符
- 直接绕过 `platform` 去启动 worker / sidecar

---

## 6. 推荐目录结构

以下是推荐目录树，用于后续逐步演进，不要求一次全部建完：

```text
/services/local-service/internal/tools
  README.md                    # 模块边界、接入规范、禁止事项
  registry.go                  # tool registry
  executor.go                  # tool executor facade
  types.go                     # 模块内统一接口、执行结果、错误适配契约
  manifest.go                  # 内置工具与 worker 工具元数据定义（可选）

  /builtin
    read_file.go               # builtin tool 示例：读取文件
    write_file.go              # builtin tool 示例：写文件
    command_preview.go         # builtin tool 示例：命令预检查/受控执行

  /adapter
    file_system_adapter.go     # 将 FileSystemAdapter 适配为工具依赖
    execution_adapter.go       # 将 ExecutionBackendAdapter 适配为工具依赖
    worker_payload_mapper.go   # worker 请求/响应映射

  /workerclient
    playwright_client.go       # Playwright worker client
    ocr_client.go              # OCR worker client
    media_client.go            # Media worker client

  /sidecarclient
    playwright_sidecar.go      # Playwright sidecar client

  /builtin_test
    read_file_test.go

  /testdata
    ...                        # 工具测试样例
```

说明：

- `registry.go` 与 `executor.go` 是核心入口；
- `builtin/` 放本地内置工具实现；
- `workerclient/` 与 `sidecarclient/` 放外部能力接入客户端；
- `adapter/` 放平台或协议层的适配，不放业务编排。

---

## 7. 新增一个工具的标准步骤

新增一个工具时，必须按下面顺序推进：

### 第一步：确认职责归属

先判断这个工具属于哪一类：

- builtin tool
- worker client tool
- sidecar client tool

如果它其实是：

- 意图分类逻辑 → 应放 `intent`
- 任务推进逻辑 → 应放 `orchestrator / runengine`
- 正式结果组织逻辑 → 应放 `delivery`

则不应该进入本模块。

### 第二步：检查协议与命名

- 工具名称先定为 `snake_case`；
- 检查 `/packages/protocol` 是否已具备本工具所需输出结构；
- 如果缺少协议字段或结构，先补协议，再写工具。

### 第三步：定义统一接口

- 在模块内定义该工具的执行输入与执行输出映射；
- 输出必须可稳定映射到 `ToolCall` / `Artifact` / `Event` 等协议结构；
- 不允许返回“临时对象 + 上层自己猜”的松散 JSON。

### 第四步：实现 adapter 或 client

- builtin tool：实现本地执行逻辑；
- worker / sidecar 工具：先写 client，再做结果适配；
- 涉及平台能力时，通过 `platform adapter` 注入，不直接写平台调用。

### 第五步：注册到 registry

- 在 registry 中完成注册；
- 暴露唯一工具名；
- 保证不会和其他工具重名或出现同义词别名。

### 第六步：接入 executor facade

- 所有执行路径都通过统一 executor；
- 成功、失败、超时、worker 不可用，都必须归一化；
- 必须能产出 `ToolCall` 记录所需数据。

### 第七步：补测试

至少补以下测试：

- registry 注册测试
- 成功执行测试
- 错误路径测试
- timeout / 不可用测试
- 输出结构映射测试

### 第八步：补文档

- 更新本 README 或模块局部文档；
- 说明工具职责、依赖、错误映射、是否依赖 worker / sidecar。

---

## 8. 工具执行统一流程

推荐统一流程如下：

1. orchestrator / model / plugin 请求执行某个 tool
2. executor facade 根据 tool name 从 registry 查找工具
3. executor 做基础校验与上下文准备
4. tool adapter / builtin / worker client 真正执行
5. 结果被统一归一化
6. 生成 `ToolCall` 记录输入材料
7. 上层消费归一化结果，决定是否生成 `Artifact` / `Event` / 进入后续 delivery

注意：

- 本模块负责第 2~6 步；
- 不负责根据业务意图决定“该不该执行”；
- 不负责执行后的 task 状态推进。

当前主分支本地执行线中，`execution / orchestrator` 已经开始真实调用 `ToolExecutor`，因此 tools 不再只是模块内骨架，而是已经进入后端运行时路径。

---

## 9. 禁止事项

以下行为在本模块中禁止出现：

1. 禁止在工具实现中直接做 `intent` 识别。
2. 禁止在工具实现中直接推进 `task / run / step / event` 状态机。
3. 禁止在工具实现中直接编排 `delivery_result`。
4. 禁止返回未登记的临时 JSON。
5. 禁止绕过 `ToolCall` 记录做静默执行。
6. 禁止使用非 `snake_case` 的工具名称。
7. 禁止把平台相关逻辑散落在工具实现里。
8. 禁止在工具实现中写死 Windows / macOS / Linux 路径。
9. 禁止从前端直接消费本模块内部结构。
10. 禁止在本模块中复制协议类型，替代 `/packages/protocol`。
11. 禁止把 worker 生命周期编排逻辑直接写进某个具体工具实现。
12. 禁止使用 `misc`、`common`、`temp` 之类模糊目录承载正式工具代码。

---

## 10. 模块自检清单

开始为本模块新增或修改工具前，先自检：

- 这个能力真的是工具，而不是意图/编排/交付逻辑吗？
- tool name 是否是 `snake_case`？
- 输出是否能映射到 `/packages/protocol`？
- 是否能产生 `ToolCall` 记录？
- 是否通过 `platform adapter` 注入了平台能力？
- 是否绕开了 worker / sidecar 的统一 client？
- 是否补了成功/失败/超时测试？

只要其中任一项答案不明确，就不要直接开写。
