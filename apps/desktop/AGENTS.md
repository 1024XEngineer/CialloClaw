### **Desktop AGENTS Guidelines**

#### **Scope**

* This file applies to the entire `apps/desktop` directory, including:

  * `src`, `src-tauri`, and relevant config files (e.g., `package.json`, `vite.config.ts`, `tailwind.config.ts`, `tsconfig*.json`, `eslint.config.js`, `components.json`).
* If changes affect protocol, backend orchestration, or shared schema, refer back to the root `AGENTS.md`.

#### **Coding Rules**

* **Comments**: Use English for all code comments.
* **Documentation**: Use **TSDoc** or **JSDoc** for exported functions, hooks, React components, and notable helpers.
* **Change Approach**: Favor small, incremental changes and avoid modifying backend or protocol behavior unless necessary for the desktop-only solution.
* **Formal Boundaries**: Maintain clear boundaries between formal task, RPC, and delivery processes. Avoid using desktop-only solutions as substitutes for backend logic.

#### **Verification Rules**

* **Type Checking**: Run `pnpm --dir apps/desktop typecheck` after any desktop-related code changes.
* **Linting**: Run `pnpm --dir apps/desktop lint` after code changes.
* **Rust Checks**: Run `cargo check` in `apps/desktop/src-tauri` after any Rust or plugin changes.

#### **Commit Rules**

* Commit one feature or refactor per change.
* Avoid mixing unrelated changes into the same commit.
* Stage only the files required for the current task when the worktree is dirty.

#### **Common Mistakes to Avoid**

* Forgetting to type event or payload parameters.
* Mixing local state for helper windows with formal business state.
* Adding **Chinese** comments in files within the desktop scope.
* Refactoring hooks in ways that inadvertently change behavior.
* Editing `useShellBallInteraction` when a safer, smaller change in the coordinator or helper window suffices.
* Introducing desktop shortcuts that silently trigger backend processes.
* Forgetting **Tauri capability permissions** when adding official plugins.
* For bone-driven PNG animation, keep **visual rest rotation** separate from **bone rest rotation**. Do not assume `bone.rotation + layer.rotation` is always the final static angle, and prefer explicit SVG transform attributes over `motion.g` rotate shorthands when pivot accuracy matters.

---

### **AGENTS.md Front-End Specific Guidelines**

#### **Scope**

* The guidelines in this file supplement the root `AGENTS.md` and focus on front-end development in `apps/desktop`.

#### **Front-End Documentation Paths**

* Before making changes, answer the following questions:

  1. Which **main entry point** does the change address (e.g., floating ball, bubble, input field, dashboard, control panel)?
  2. Is the data **already defined** in the backend (e.g., `packages/protocol`)?
  3. Is the state you're modifying **formal business state** or just a **local state**?
  4. Will the user action trigger any **risky actions**, **authorization waits**, or **formal delivery splits**?
  5. Will the outcome be a **bubble short delivery** or a **formal artifact/citation** output?

* If unclear, review relevant documents based on the task type:

  * **Floating Ball, Bubble, Input Fields, Light Interactions**: `docs/product-interaction-design.md`, `docs/work-priority-plan.md`.
  * **Dashboard, Task Details, Safety Summary**: `docs/dashboard-design.md`, `docs/work-priority-plan.md`.
  * **Control Panel, Settings, Low-Frequency Config Pages**: `docs/control-panel-settings.md`, `docs/work-priority-plan.md`.
  * **Adding Formal Fields, State, Notifications, Error Handling**: `docs/development-guidelines.md`, `docs/protocol-design.md`.
  * **Unclear Domain or Main Entry Point**: `docs/architecture-overview.md`, `docs/development-guidelines.md`, `docs/work-priority-plan.md`.

#### **Front-End Boundaries**

* **Front-End Responsibility**:

  * Interaction handling.
  * Rendering views.
  * Local state management.
  * ViewModel, Query, and Store.
  * Platform capability bridging.
  * RPC client interaction.

* **Not Front-End's Responsibility**:

  * Backend orchestration.
  * Model decision making.
  * Final risk assessments.
  * Defining true data sources.
  * Direct database, worker, or internal backend access.

* **Key Guidelines**:

  * Organize UI components around **tasks** (e.g., floating balls, bubbles, dashboards).
  * Ensure **local states** are clearly named and separated from **formal business states**.
  * Short results (e.g., bubble outcomes) should not be treated as formal outputs.

#### **Implementation Order**

1. First, integrate **official backend data** and **RPC**.
2. Correctly handle states like **confirmation**, **processing**, **authorization**, **failure**, and **completion**.
3. Handle short vs. formal delivery splits.
4. Only refine **style** and **animations** once the above steps are implemented.

* Avoid forcing a visual solution that introduces unregistered objects or fields.
* Don’t rely on **mock data** for functionality that should be connected to official data sources.

#### **Pre- and Post-Modification Checks**

* **Before**: Ensure the protocol fields, error codes, and notification sources are well-understood.
* **After**: Verify field names align with protocol sources, local states are correctly separated from formal business states, and any required documentation is updated.

#### **Commenting Standards**

* **English comments** are mandatory, especially for:

  * Complex interaction state machines (e.g., drag, hover, long press).
  * Platform bridging and window synchronization.
  * Logic differentiating **local** and **formal business states**.
* Comments should explain:

  * The logic’s role within the main front-end flow.
  * Why it's **local state** and not formal business state.
  * Timing, synchronization, and failure conditions.

#### **Documentation Sync Rules**

* **Update Documentation** when:

  * Interaction semantics (floating ball, dashboard, etc.) change.
  * New formal fields, states, or error-handling protocols are introduced.
  * Page structure or assumptions affect **task completion criteria** in `docs/work-priority-plan.md`.

#### **One-Liner Guideline**

- 调整悬浮球、气泡、输入框、仪表盘或控制面板的正式交互语义
- 调整主链路入口、确认链路、短交付与正式交付分流方式
- 页面需要新增正式字段、正式状态或正式错误处理约定
- 页面结构或交互假设影响 `docs/work-priority-plan.md` 中的任务完成判断

回写时遵循：

- 交互语义优先回写产品 / 仪表盘 / 设置文档
- 协议字段、事件、状态变化优先回写协议与开发规范文档
- 若代码已真实完成文档中的任务项，不要让文档长期落后于代码

------

## 7. 前端注释规范

注释在前端不是可选项，尤其以下场景必须写英文注释：

- 多窗口协调、悬浮球与气泡生命周期切换
- 悬停、拖拽、长按、语音等复杂交互状态机
- 与平台桥接、窗口同步、托盘或 Named Pipe 交互相关的逻辑
- 任何需要区分“局部状态”和“正式状态”的代码
- 失败回退、去抖、竞态规避、延迟同步等不直观逻辑

注释至少要说明：

- 这段逻辑在前端主链路里承担什么作用
- 为什么这里是局部状态而不是正式状态
- 为什么需要当前时序、延迟、同步或回退处理
- 失败路径和边界条件是什么

必须坚持：

- 新增复杂逻辑时同步写英文注释
- 调整复杂逻辑时同步检查并修正旧注释
- 当前改动范围内若发现中文注释，必须改成英文注释
- 新增或调整正式 RPC 方法、正式字段、正式通知、任务路由状态时，必须在同一次工作中同步回写协议或产品真源文档

禁止：

- 用“组件很短”作为不写注释的理由
- 让注释只重复 JSX 或变量名表面含义
- 在状态机切换、窗口同步和副作用边界上省略解释

------

## 8. 建议的前端工作模式

推荐按以下顺序工作：

1. 先读根目录 `AGENTS.md` 与本文件
2. 再按任务类型补读产品、仪表盘、设置或协议文档
3. 再看当前页面、服务层、RPC client 和状态管理实现
4. 先确认正式对象、正式出口和失败路径，再动手改 UI
5. 改完后补注释、自检并回写相关文档

------

## 9. 一句话总原则

> 前端先保证对象真实、状态清楚、交付路径正确，再追求页面表现和交互质感。
