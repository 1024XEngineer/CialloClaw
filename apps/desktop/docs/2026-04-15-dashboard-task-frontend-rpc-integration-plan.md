# Dashboard Task Frontend RPC Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the frontend-only `/tasks` RPC integration by consuming the backend's existing artifact and delivery endpoints, wiring those actions into the task UI, and reducing placeholder result handling without changing backend code.

**Architecture:** Reuse the current `task.list/detail/control` frontend stack and add a dedicated task-output service for `agent.task.artifact.list`, `agent.task.artifact.open`, and `agent.delivery.open`. Keep the current task-detail contract strict, keep intent-recognition / `confirming_intent` out of scope, and let the task page own output-open mutations plus artifact-sheet loading while using graceful frontend-only fallbacks when the desktop runtime cannot launch a resolved payload directly.

**Tech Stack:** React 18, TypeScript, TanStack Query, React Router, existing Tauri desktop frontend surface, dashboard node:test contract suite, workspace protocol package.

---

## Scope

### In Scope

- frontend-only task-module integration work under `apps/desktop/src/features/dashboard/tasks`
- existing backend RPC methods only:
  - `agent.task.artifact.list`
  - `agent.task.artifact.open`
  - `agent.delivery.open`
- task outputs tab, task files sheet, and task detail result affordances
- frontend contract tests / typecheck for the new task-output flow

### Explicitly Out of Scope

- backend code changes under `services/local-service/**`
- protocol/schema changes under `packages/protocol/**`
- `confirming_intent` / intent-recognition UX changes
- safety-page audit / restore-point work (belongs to the safety module, not this task-only plan)
- new Tauri backend commands; use current frontend/runtime capabilities only

## File Map

- Modify: `apps/desktop/src/rpc/methods.ts`
- Create: `apps/desktop/src/features/dashboard/tasks/taskOutput.service.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.types.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskFilesSheet.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.service.ts`
- Modify: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`

## Constraints To Preserve

- Do not change any backend handlers, orchestrator code, protocol definitions, or stable RPC names.
- Do not add task editing or intent-confirmation entrypoints in this slice.
- Keep `task` as the external object; do not leak raw backend internals into the task page.
- Prefer smaller focused frontend files instead of continuing to inflate `taskPage.service.ts`.
- Keep mock mode available, but make RPC mode the primary path and remove placeholder copy that claims the backend is missing capabilities that already exist.
- If the desktop runtime cannot directly launch a resolved file/folder payload with current frontend capabilities, degrade to a clear user-facing fallback (for example copy path + feedback) rather than blocking the whole feature.

### Task 1: Add Task Output RPC Surface and Frontend Output Service

**Files:**
- Modify: `apps/desktop/src/rpc/methods.ts`
- Create: `apps/desktop/src/features/dashboard/tasks/taskOutput.service.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.types.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts`
- Modify: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`

- [ ] **Step 1: Write the failing dashboard contract tests**

Extend `apps/desktop/src/features/dashboard/dashboard.contract.test.ts` with pure tests for the new task-output service helpers.

```ts
test("task output helpers normalize open actions from existing RPC contracts", async () => {
  const module = await import("./tasks/taskOutput.service");

  assert.deepEqual(
    module.resolveTaskOpenExecutionPlan({
      open_action: "task_detail",
      resolved_payload: { path: null, url: null, task_id: "task_demo_001" },
      delivery_result: {
        type: "task_detail",
        title: "Task detail",
        preview_text: "回到任务详情",
        payload: { path: null, url: null, task_id: "task_demo_001" },
      },
    }),
    {
      mode: "task_detail",
      taskId: "task_demo_001",
      path: null,
      url: null,
      feedback: "已定位到任务详情。",
    },
  );
});
```

Also add tests that cover:

- artifact-list mock fallback shape
- `open_file` / `reveal_in_folder` execution plans
- `open_url` / `result_page` execution plans
- the graceful fallback path when no direct launcher is available in the frontend runtime

- [ ] **Step 2: Run the dashboard contract test to verify it fails**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: FAIL because `taskOutput.service.ts` and the new RPC wrappers do not exist yet.

- [ ] **Step 3: Implement the minimal RPC wrappers and output service**

Add the missing frontend wrappers in `apps/desktop/src/rpc/methods.ts`:

```ts
export function listTaskArtifacts(params: AgentTaskArtifactListParams) {
  return rpcClient.request<AgentTaskArtifactListResult>(RPC_METHODS.AGENT_TASK_ARTIFACT_LIST, params);
}

export function openTaskArtifact(params: AgentTaskArtifactOpenParams) {
  return rpcClient.request<AgentTaskArtifactOpenResult>(RPC_METHODS.AGENT_TASK_ARTIFACT_OPEN, params);
}

export function openDelivery(params: AgentDeliveryOpenParams) {
  return rpcClient.request<AgentDeliveryOpenResult>(RPC_METHODS.AGENT_DELIVERY_OPEN, params);
}
```

Create `apps/desktop/src/features/dashboard/tasks/taskOutput.service.ts` to own frontend-only output concerns:

- `loadTaskArtifactPage(taskId, source)`
- `openTaskArtifactForTask(taskId, artifactId, source)`
- `openTaskDeliveryForTask(taskId, artifactId | undefined, source)`
- `resolveTaskOpenExecutionPlan(result)`
- `performTaskOpenExecution(plan)`

Target shape:

```ts
export type TaskOpenExecutionPlan = {
  mode: "task_detail" | "open_url" | "copy_path";
  taskId: string | null;
  path: string | null;
  url: string | null;
  feedback: string;
};
```

Use mock support in `taskPage.mock.ts` so dashboard tests can exercise the same surface in mock mode without touching backend code.

- [ ] **Step 4: Run the dashboard contract test and typecheck**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: PASS.

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/rpc/methods.ts apps/desktop/src/features/dashboard/tasks/taskOutput.service.ts apps/desktop/src/features/dashboard/tasks/taskPage.types.ts apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts apps/desktop/src/features/dashboard/dashboard.contract.test.ts
git commit -m "feat(dashboard): add task output rpc services"
```

### Task 2: Wire Task Outputs Tab and Files Sheet to Existing Backend RPC

**Files:**
- Modify: `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskFilesSheet.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.types.ts`
- Modify: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`

- [ ] **Step 1: Write the failing dashboard contract tests for wiring**

Add behavior-first tests around the new output integration helpers, then a small source-adoption check for the task components.

```ts
test("task files sheet prefers artifact.list data over detail-only fallback when available", async () => {
  const module = await import("./tasks/taskOutput.service");

  const page = await module.loadTaskArtifactPage("task_done_001", "mock");

  assert.ok(page.items.length > 0);
  assert.equal(page.page.offset, 0);
});

test("task page adopts the task output service for open actions", () => {
  const taskTabsSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskTabsPanel.tsx"), "utf8");
  const filesSheetSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskFilesSheet.tsx"), "utf8");

  assert.match(taskTabsSource, /onOpenArtifact/);
  assert.match(filesSheetSource, /onOpenArtifact/);
});
```

- [ ] **Step 2: Run the dashboard contract test to verify it fails**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: FAIL because the task page components still show placeholder output actions.

- [ ] **Step 3: Implement the task-page wiring**

Update `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx` so it owns:

- one artifact-list query for the file sheet, enabled only when the sheet is open in RPC mode
- one mutation for artifact open
- one mutation for delivery open
- shared success/error feedback for open actions

Target structure:

```ts
const artifactListQuery = useQuery({
  enabled: filesSheetOpen && detailOpen && dataMode === "rpc" && Boolean(selectedTaskId),
  queryKey: ["dashboard", "tasks", "artifacts", dataMode, selectedTaskId],
  queryFn: () => loadTaskArtifactPage(selectedTaskId!, dataMode),
  retry: false,
});
```

Update `TaskTabsPanel.tsx` and `TaskFilesSheet.tsx` so artifact rows use real callbacks:

- `onOpenArtifact(artifactId)`
- `onOpenLatestDelivery()` for output sections where there is no single artifact choice

Replace the current placeholder tooltip copy with real state-driven UI:

- loading
- action available
- open failed feedback

Update `TaskDetailPanel.tsx` if needed so the detail result card and the outputs tab stay semantically aligned (both should describe real output actions, not “protocol missing” placeholders).

- [ ] **Step 4: Run the dashboard contract test and typecheck**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: PASS.

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/dashboard/tasks/TaskPage.tsx apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx apps/desktop/src/features/dashboard/tasks/components/TaskFilesSheet.tsx apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx apps/desktop/src/features/dashboard/tasks/taskPage.types.ts apps/desktop/src/features/dashboard/dashboard.contract.test.ts
git commit -m "feat(dashboard): wire task outputs to existing rpc"
```

### Task 3: Reduce Task Result Placeholder Semantics and Verify the End-to-End Frontend Flow

**Files:**
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.service.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskFilesSheet.tsx`
- Modify: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`

- [ ] **Step 1: Write the failing dashboard contract tests for fallback/result semantics**

Add tests that lock in the new frontend behavior:

```ts
test("task detail fallback no longer claims artifact.open is missing when rpc wrappers exist", () => {
  const source = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskTabsPanel.tsx"), "utf8");

  assert.doesNotMatch(source, /当前协议尚未提供稳定的 artifact\.open 能力/);
});
```

Also add tests that ensure:

- fallback experience does not pretend result-opening support is unavailable
- artifact list falls back to `detail.artifacts` only when no dedicated list data have been fetched yet
- the task page still stays functional in mock mode

- [ ] **Step 2: Run the dashboard contract test to verify it fails**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: FAIL because the old placeholder copy and fallback assumptions are still present.

- [ ] **Step 3: Implement the minimal cleanup**

Update `taskPage.service.ts` so fallback experience/result copy no longer implies missing backend support. Keep fallback strictly about missing frontend data, not missing protocol support.

Examples to remove or replace:

- `当前协议未返回更多结果摘要，先展示任务轨迹。`
- `后续可把任务修改或产出打开能力接进来。`

Replace with copy that reflects the actual state:

- waiting for more task context
- results available through task outputs when present
- fallback mode is a temporary frontend rendering fallback, not a protocol limitation

- [ ] **Step 4: Run the full verification set**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: PASS.

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/dashboard/tasks/taskPage.service.ts apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx apps/desktop/src/features/dashboard/tasks/components/TaskFilesSheet.tsx apps/desktop/src/features/dashboard/dashboard.contract.test.ts
git commit -m "feat(dashboard): trim task result placeholders"
```

## Final Verification

- [ ] Run `pnpm --dir apps/desktop test:dashboard`
- [ ] Run `pnpm --dir apps/desktop typecheck`

## Notes For Implementation

- Do not expand this slice into the safety module's `audit / restore_points / restore.apply` work. That belongs to a separate plan.
- Do not touch `confirming_intent` or shell-ball logic in this plan.
- If current frontend runtime cannot directly open a resolved local path/folder without backend changes, degrade gracefully inside the frontend-only execution plan (for example by surfacing the resolved path and a clear success/fallback message) instead of inventing new protocol or Tauri commands.
