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

> **Front-End Focus**: Ensure data integrity, clear state separation, and accurate delivery paths before refining UI aesthetics or interactions.
