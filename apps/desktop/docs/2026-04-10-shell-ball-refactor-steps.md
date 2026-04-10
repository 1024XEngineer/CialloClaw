# Shell-Ball 桌面悬浮球重构步骤清单

## 1. 文档目的

本文用于把 `shell-ball` 从当前“带页面展示壳的前端演示入口”逐步收敛为“桌面悬浮球核心表面”，并明确每一步的小功能边界、预期产出和提交粒度。

本文与《Shell-Ball 桌面悬浮球化补充设计》配套使用，重点回答“下一步怎么拆、怎么做、怎么分批提交”。

## 2. 当前现状

当前 `shell-ball` 已具备：

- 冻结的 7 个展示态
- 本地交互控制器
- 鸟球主体
- 上方气泡区预留
- 下方 input bar
- 独立窗口入口
- 基础交互测试与构建验证

当前仍存在的问题：

- 结构上仍带有页面展示壳
- 交互触发偏敏感
- dev 调试能力仍较靠前
- 尚未收敛成真正的桌面悬浮球表面结构

## 3. 总体重构目标

将 `shell-ball` 后续重构为以下分层：

- `ShellBallSurface`
  - 纯悬浮球核心表面
- `ShellBallDesktopWindow`
  - 纯桌面窗口容器（后置，待纯前端表面稳定后再实现）
- dev-only 调试层
  - 只用于开发态切换展示态

明确不再长期维护完整 `DemoPage` 概念。

## 4. 小功能拆分建议

### 小功能 1：降低交互敏感度

目标：

- 为 `hover_input` 增加 hover intent
- 为离开交互区增加 leave grace
- 提高语音长按阈值
- 将锁定 / 取消改为“松手提交”

建议内容：

- 新增 hover 延迟常量
- 新增 leave 缓冲常量
- 新增语音手势死区
- 拆分“手势预览”和“最终提交”逻辑
- 保持 7 个 `visualState` 不变

建议提交：

- `feat(shell-ball): soften hover and voice trigger pacing`

### 小功能 2：移除页面展示壳

目标：

- 去掉标题、说明文案、大页面展示结构
- 让 `shell-ball` 主体只剩：
  - 气泡区预留
  - 鸟球主体
  - input bar

建议内容：

- 将当前 `ShellBallApp` 改为更纯粹的表面容器
- 删除页面式 hero 文案结构
- 去除“网页展示感”的布局包装

建议提交：

- `feat(shell-ball): remove demo page shell layout`

### 小功能 3：抽出 `ShellBallSurface`

目标：

- 将悬浮球视觉表面从宿主入口中单独抽出
- 为后续桌面窗口宿主和 dev-only 调试壳共用同一表面

建议内容：

- 新建 `ShellBallSurface`
- `ShellBallApp` 仅暂时做组合层，最终可继续瘦身
- 保持交互控制器复用，不复制逻辑

建议提交：

- `feat(shell-ball): extract reusable shell-ball surface`

### 小功能 4：dev-only 调试层收敛

目标：

- `ShellBallDemoSwitcher` 不再作为主结构展示
- 只在开发环境下显示

建议内容：

- 用 `import.meta.env.DEV` 控制显隐
- 将 switcher 移到次要调试区或浮层
- 正式结构不再依赖 switcher

建议提交：

- `feat(shell-ball): move demo switcher to dev-only layer`

### 小功能 5：桌面窗口宿主收敛

当前执行策略：该步骤后置。在纯前端 `shell-ball` 表面、交互节奏、调试层收敛和拖拽边界都满意之前，不进入这一阶段。

目标：

- 明确 `shell-ball.html` 挂载的是桌面悬浮球窗口宿主
- 让窗口入口与表面组件职责分离

建议内容：

- 增加 `ShellBallDesktopWindow`
- 让入口文件只负责挂载桌面窗口容器
- 不在宿主层继续堆页面展示元素

建议提交：

- `feat(shell-ball): add desktop window shell for floating ball`

### 小功能 6：为未来拖拽预留边界

目标：

- 为后续桌面拖拽和吸边能力预留结构
- 不在当前阶段直接实现真实拖拽能力

建议内容：

- 明确拖拽热点区域
- 明确交互层与窗口层边界
- 保持手势纵向优先，避免与未来拖拽冲突

建议提交：

- `feat(shell-ball): prepare drag-safe interaction boundaries`

## 5. 推荐实施顺序

推荐按以下顺序推进：

1. 降低交互敏感度
2. 移除页面展示壳
3. 抽出 `ShellBallSurface`
4. 收敛 dev-only 调试层
5. 预留拖拽边界
6. 增加桌面窗口宿主层（后置）

原因：

- 先稳住交互手感
- 再收结构
- 先把纯前端表面边界做清楚
- 最后再处理 Tauri 宿主与未来平台扩展边界

## 6. 每一步的验收要求

每个小功能完成后至少验证：

- `pnpm --dir apps/desktop test:shell-ball`
- `pnpm --dir apps/desktop typecheck`
- `pnpm --dir apps/desktop build`

必要时再补：

- 手动交互检查
- 悬浮球窗口显示检查
- 触发阈值实际体验检查

## 7. 本轮不做的事

以下内容继续明确排除：

- 完整气泡区系统
- 真实文件拖拽承接
- 真实文本选中承接
- 真实语音识别
- dashboard 联动
- `ShellBallDesktopWindow` / Tauri 宿主适配（在当前纯前端阶段后置）
- 吸边 / 贴边 / 自动停靠
- 完整桌宠行为系统

## 8. 提交约定

后续每完成一个小功能，单独提交一次。

提交格式统一为：

- `feat(shell-ball): <具体动作>`

示例：

- `feat(shell-ball): soften hover and voice trigger pacing`
- `feat(shell-ball): extract reusable shell-ball surface`
- `feat(shell-ball): move demo switcher to dev-only layer`

## 9. 一句话执行策略

先把 `shell-ball` 的交互节奏调稳，再移除页面展示壳，随后抽出桌面悬浮球核心表面，最后收敛到真正的桌面窗口结构。
