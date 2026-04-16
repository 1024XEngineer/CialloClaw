# Dashboard Task Detail Safety Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/tasks` task detail jump into the exact `/safety` object it refers to, while extending `agent.task.detail.get` with the minimum formal safety anchor data and keeping dashboard task editing out of scope.

**Architecture:** Keep `agent.task.detail.get` as the only task detail read path and extend it with one formal anchor field, `approval_request`, plus a strict `RecoveryPoint | null` shape in `security_summary.latest_restore_point`. On the frontend, define one shared dashboard safety-navigation contract plus one shared task-query-key contract, let `/tasks` hand off an exact safety snapshot through route state, and let `/safety` bind that snapshot to live pending/summary data when possible while still rendering the routed snapshot if live data have already moved on.

**Tech Stack:** React 18, TypeScript, TanStack Query, React Router, dedicated dashboard node:test contract suite, Go 1.26, local-service orchestrator/RPC tests, workspace protocol package.

---

## File Map

- Modify: `packages/protocol/rpc/methods.ts`
- Modify: `services/local-service/internal/orchestrator/service.go`
- Modify: `services/local-service/internal/orchestrator/service_test.go`
- Modify: `services/local-service/internal/rpc/server_test.go`
- Modify: `docs/protocol-design.md`
- Create: `apps/desktop/src/features/dashboard/shared/dashboardSafetyNavigation.ts`
- Create: `apps/desktop/src/features/dashboard/tasks/taskPage.query.ts`
- Create: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`
- Create: `apps/desktop/tsconfig.dashboard-test.json`
- Modify: `apps/desktop/package.json`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.service.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.mapper.ts`
- Modify: `apps/desktop/src/features/dashboard/safety/SecurityApp.tsx`

## Constraints To Preserve

- Do not add a new dashboard-only detail RPC.
- Do not add dashboard task-edit RPC or mutation flow.
- Keep `task` as the external object; do not expose raw `run / event / tool_call` lists to the task page.
- Keep full pending-approval listing in the safety domain; task detail gets one formal anchor only.
- Ship the formal protocol/doc change in the same slice as the contract change.
- For task detail, `pending_authorizations` must remain coherent with the single active anchor and therefore collapse to `0 | 1`.
- Keep every completed code slice committed with `feat(dashboard): ...`; use `docs: ...` only for docs-only work.

### Task 1: Extend Task Detail With Formal Safety Anchors

**Files:**
- Modify: `packages/protocol/rpc/methods.ts`
- Modify: `services/local-service/internal/orchestrator/service.go`
- Modify: `services/local-service/internal/orchestrator/service_test.go`
- Modify: `services/local-service/internal/rpc/server_test.go`
- Modify: `docs/protocol-design.md`

- [ ] **Step 1: Write the failing backend tests**

Add one orchestrator test for a waiting-auth task and one RPC dispatch test for `agent.task.detail.get`.

```go
func TestServiceTaskDetailGetIncludesApprovalRequestAnchor(t *testing.T) {
    service := newTestService()

    startResult, _ := service.StartTask(map[string]any{
        "session_id": "sess_demo",
        "source":     "floating_ball",
        "trigger":    "text_selected_click",
        "input": map[string]any{
            "type": "text_selection",
            "text": "needs authorization",
        },
    })

    taskID := startResult["task"].(map[string]any)["task_id"].(string)
    _, _ = service.ConfirmTask(map[string]any{
        "task_id": taskID,
        "corrected_intent": map[string]any{
            "name": "write_file",
            "arguments": map[string]any{
                "require_authorization": true,
                "target_path":           "workspace_document",
            },
        },
    })

    detail, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
    if err != nil {
        t.Fatalf("task detail get failed: %v", err)
    }

    approvalRequest := detail["approval_request"].(map[string]any)
    if approvalRequest["task_id"] != taskID {
        t.Fatalf("expected approval anchor for %s, got %#v", taskID, approvalRequest)
    }
    if detail["security_summary"].(map[string]any)["pending_authorizations"] != 1 {
        t.Fatalf("expected pending_authorizations to collapse to 1 for anchored waiting_auth task")
    }
}
```

Also add an RPC test that asserts a `waiting_auth` task returns `approval_request`, a normal task returns `approval_request: nil`, and task-detail `pending_authorizations` never rises above `1`.

Also add a negative backend test for the stale-anchor case: if a task record still carries an `approval_request` payload but its status is no longer `waiting_auth`, then task detail must return both `approval_request: nil` and `pending_authorizations: 0`.

- [ ] **Step 2: Run the backend tests to verify they fail**

Run: `go test ./services/local-service/internal/orchestrator ./services/local-service/internal/rpc`
Expected: FAIL because `AgentTaskDetailGetResult` and `TaskDetailGet` do not expose `approval_request` yet.

- [ ] **Step 3: Implement the minimal protocol and backend changes**

Update `packages/protocol/rpc/methods.ts` so `AgentTaskDetailGetResult` carries one formal anchor:

```ts
export interface SecuritySummary {
  security_status: SecurityStatus;
  risk_level: RiskLevel;
  pending_authorizations: number;
  latest_restore_point: RecoveryPoint | null;
}

export interface AgentTaskDetailGetResult {
  task: Task;
  timeline: TaskStep[];
  artifacts: Artifact[];
  mirror_references: MirrorReference[];
  security_summary: SecuritySummary;
  approval_request: ApprovalRequest | null;
}
```

Update `services/local-service/internal/orchestrator/service.go` so `TaskDetailGet`:

- reads the formal task-level approval object from the runtime/storage-backed task record
- returns `approval_request` when the task is in `waiting_auth`
- returns `nil` otherwise
- normalizes task-detail `pending_authorizations` to `1` when an anchor exists and `0` otherwise
- keeps `security_summary.latest_restore_point` as a full object or `nil`, never a loose string identifier

Target structure:

```go
approvalRequest := map[string]any(nil)
if task.Status == "waiting_auth" && len(task.ApprovalRequest) != 0 {
    securitySummary["pending_authorizations"] = 1
    approvalRequest = cloneMap(task.ApprovalRequest)
} else {
    securitySummary["pending_authorizations"] = 0
}

return map[string]any{
    "task":              taskMap(task),
    "timeline":          timelineMap(task.Timeline),
    "artifacts":         cloneMapSlice(task.Artifacts),
    "mirror_references": cloneMapSlice(task.MirrorReferences),
    "security_summary":  securitySummary,
    "approval_request":  approvalRequest,
}, nil
```

Update `docs/protocol-design.md` in the same slice to record that:

- `agent.task.detail.get` now returns `approval_request`
- `approval_request` is one task-detail safety anchor, not a replacement for `agent.security.pending.list`
- task-detail `pending_authorizations` is the anchored task-detail count and therefore `0 | 1`
- `security_summary.latest_restore_point` in task detail is `RecoveryPoint | null`

- [ ] **Step 4: Run the backend tests again**

Run: `go test ./services/local-service/internal/orchestrator ./services/local-service/internal/rpc`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/protocol/rpc/methods.ts services/local-service/internal/orchestrator/service.go services/local-service/internal/orchestrator/service_test.go services/local-service/internal/rpc/server_test.go docs/protocol-design.md
git commit -m "feat(dashboard): expose task detail safety anchors"
```

### Task 2: Add Shared Helper Contracts and a Dedicated Dashboard Contract Test Runner

**Files:**
- Create: `apps/desktop/src/features/dashboard/shared/dashboardSafetyNavigation.ts`
- Create: `apps/desktop/src/features/dashboard/tasks/taskPage.query.ts`
- Create: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`
- Create: `apps/desktop/tsconfig.dashboard-test.json`
- Modify: `apps/desktop/package.json`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.service.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx`

- [ ] **Step 1: Write the failing dedicated dashboard contract tests**

Create `apps/desktop/src/features/dashboard/dashboard.contract.test.ts` and test pure helper behavior directly.

```ts
import test from "node:test";
import assert from "node:assert/strict";
import {
  buildDashboardSafetyNavigationState,
  readDashboardSafetyNavigationState,
  resolveDashboardSafetyFocusTarget,
} from "./shared/dashboardSafetyNavigation";
import {
  buildDashboardTaskBucketQueryKey,
  buildDashboardTaskDetailQueryKey,
  dashboardTaskBucketQueryPrefix,
  dashboardTaskDetailQueryPrefix,
} from "./tasks/taskPage.query";

test("dashboard safety navigation prefers approval anchors over restore points", () => {
  // approval_request wins
});

test("dashboard safety focus falls back to routed snapshots when live data moved on", () => {
  // snapshot approval / snapshot restore point remain renderable
});

test("dashboard task query keys stay stable across task and safety pages", () => {
  assert.deepEqual(dashboardTaskBucketQueryPrefix, ["dashboard", "tasks", "bucket"]);
  assert.deepEqual(buildDashboardTaskDetailQueryKey("rpc", "task_demo_001"), ["dashboard", "tasks", "detail", "rpc", "task_demo_001"]);
});
```

The test file must cover:

- `buildDashboardSafetyNavigationState(...)`
- `readDashboardSafetyNavigationState(...)`
- `resolveDashboardSafetyFocusTarget(...)`
- query prefix/key builders
- malformed route state rejection
- live-match vs snapshot-fallback behavior for both approval and restore-point cases

- [ ] **Step 2: Add the dashboard test script and run it to verify failure**

Add `test:dashboard` to `apps/desktop/package.json` using a dedicated tsconfig/emitted cache, mirroring the existing node:test flow used elsewhere.

```json
{
  "scripts": {
    "test:dashboard": "node -e \"require('node:fs').rmSync('.cache/dashboard-tests', { recursive: true, force: true });\" && tsc -p tsconfig.dashboard-test.json && node -e \"require('node:fs').mkdirSync('.cache/dashboard-tests', { recursive: true }); require('node:fs').writeFileSync('.cache/dashboard-tests/package.json', '{\\\"type\\\":\\\"commonjs\\\"}');\" && node --test .cache/dashboard-tests/features/dashboard/dashboard.contract.test.js"
  }
}
```

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: FAIL because the helper modules and tsconfig do not exist yet.

- [ ] **Step 3: Implement the helper contracts and strict detail normalization**

Create `apps/desktop/src/features/dashboard/shared/dashboardSafetyNavigation.ts` with:

```ts
export type DashboardSafetyNavigationState = {
  source: "task-detail";
  taskId: string;
  approvalRequest?: ApprovalRequest;
  restorePoint?: RecoveryPoint;
};

export type DashboardSafetyFocusTarget = {
  activeDetailKey: `approval:${string}` | "restore" | null;
  approvalSnapshot: ApprovalRequest | null;
  restorePointSnapshot: RecoveryPoint | null;
  feedback: string | null;
};

export function buildDashboardSafetyNavigationState(detail: AgentTaskDetailGetResult): DashboardSafetyNavigationState | null { /* ... */ }
export function readDashboardSafetyNavigationState(value: unknown): DashboardSafetyNavigationState | null { /* ... */ }
export function resolveDashboardSafetyFocusTarget(args: {
  state: DashboardSafetyNavigationState | null;
  livePending: ApprovalRequest[];
  liveRestorePoint: RecoveryPoint | null;
}): DashboardSafetyFocusTarget | null { /* ... */ }
```

The focus resolver must follow the approved spec exactly:

- approval anchor wins over restore point
- live object wins when IDs match
- routed snapshot stays renderable when live data miss or have already changed
- snapshot fallback returns user-facing feedback

Create `apps/desktop/src/features/dashboard/tasks/taskPage.query.ts` with stable query prefixes and key builders:

```ts
export const dashboardTaskBucketQueryPrefix = ["dashboard", "tasks", "bucket"] as const;
export const dashboardTaskDetailQueryPrefix = ["dashboard", "tasks", "detail"] as const;

export function buildDashboardTaskBucketQueryKey(dataMode: "rpc" | "mock", group: "unfinished" | "finished", limit: number) {
  return [...dashboardTaskBucketQueryPrefix, dataMode, group, limit] as const;
}

export function buildDashboardTaskDetailQueryKey(dataMode: "rpc" | "mock", taskId: string) {
  return [...dashboardTaskDetailQueryPrefix, dataMode, taskId] as const;
}
```

Task 3 will extend this same file with the cross-page refresh-plan helper so `/tasks` and `/safety` share one source of truth for invalidation + remount refetch behavior.

Then tighten `apps/desktop/src/features/dashboard/tasks/taskPage.service.ts` so the detail normalizer:

- accepts `approval_request` as `ApprovalRequest | null`
- rejects string restore points for RPC mode
- keeps fallback detail data returning `approval_request: null`

If `TaskDetailPanel.tsx` still branches on string restore-point values, remove that dead branch so the UI follows the formal contract.

- [ ] **Step 4: Run the dashboard contract test and typecheck**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: PASS.

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/dashboard/shared/dashboardSafetyNavigation.ts apps/desktop/src/features/dashboard/tasks/taskPage.query.ts apps/desktop/src/features/dashboard/dashboard.contract.test.ts apps/desktop/tsconfig.dashboard-test.json apps/desktop/package.json apps/desktop/src/features/dashboard/tasks/taskPage.service.ts apps/desktop/src/features/dashboard/tasks/components/TaskDetailPanel.tsx
git commit -m "feat(dashboard): add safety navigation helpers"
```

### Task 3: Wire `/tasks` and `/safety` to the Shared Contracts

**Files:**
- Modify: `apps/desktop/src/features/dashboard/dashboard.contract.test.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.query.ts`
- Modify: `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx`
- Modify: `apps/desktop/src/features/dashboard/tasks/taskPage.mapper.ts`
- Modify: `apps/desktop/src/features/dashboard/safety/SecurityApp.tsx`

- [ ] **Step 1: Extend the dashboard contract test with refresh-plan behavior assertions**

Add a pure helper contract for the cross-page refresh semantics instead of regexing component source.

```ts
test("dashboard security refresh plan invalidates both task query families", () => {
  assert.deepEqual(getDashboardTaskSecurityRefreshPlan("rpc"), {
    invalidatePrefixes: [
      dashboardTaskBucketQueryPrefix,
      dashboardTaskDetailQueryPrefix,
    ],
    refetchOnMount: true,
  });

  assert.deepEqual(getDashboardTaskSecurityRefreshPlan("mock"), {
    invalidatePrefixes: [
      dashboardTaskBucketQueryPrefix,
      dashboardTaskDetailQueryPrefix,
    ],
    refetchOnMount: false,
  });
});
```

- [ ] **Step 2: Run the dashboard contract test to verify it fails**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: FAIL because `getDashboardTaskSecurityRefreshPlan(...)` does not exist yet.

- [ ] **Step 3: Implement the page wiring**

Update `apps/desktop/src/features/dashboard/tasks/TaskPage.tsx` to:

- use `buildDashboardTaskBucketQueryKey(...)` and `buildDashboardTaskDetailQueryKey(...)`
- use `getDashboardTaskSecurityRefreshPlan(dataMode).refetchOnMount` so invalidated task queries really re-fetch when the page remounts
- hand `buildDashboardSafetyNavigationState(detailData.detail)` into `navigate(resolveDashboardRoutePath("safety"), { state })`
- leave the edit CTA disabled in the page flow; do not add a fake dashboard edit handler

Target structure:

```ts
const unfinishedQuery = useQuery({
  queryKey: buildDashboardTaskBucketQueryKey(dataMode, "unfinished", unfinishedLimit),
  refetchOnMount: getDashboardTaskSecurityRefreshPlan(dataMode).refetchOnMount,
  // ...
});

if (action === "open-safety") {
  navigate(resolveDashboardRoutePath("safety"), {
    state: buildDashboardSafetyNavigationState(detailData.detail),
  });
  return;
}
```

Update `apps/desktop/src/features/dashboard/tasks/taskPage.mapper.ts` explicitly so every disabled edit CTA stops promising direct dashboard editing. Keep it tooltip-only and disabled. Target copy:

```ts
{ action: "edit", label: "去悬浮球继续", tooltip: "如需修改这条任务，请回到悬浮球继续补充或修正。" }
```

Update `apps/desktop/src/features/dashboard/safety/SecurityApp.tsx` to:

- use `useLocation()` and `useQueryClient()`
- read and clear route state once
- compute focus through `resolveDashboardSafetyFocusTarget(...)`
- store `approvalSnapshot` / `restorePointSnapshot` from the helper result
- render snapshot data when live `pending` / `summary` do not contain the same object yet
- make the active detail header/preview use the same live-or-snapshot object as the detail body, so the chrome and body never disagree during fallback cases
- invalidate both task query families using `getDashboardTaskSecurityRefreshPlan(...)`

Target structure:

```ts
const location = useLocation();
const queryClient = useQueryClient();
const [approvalSnapshot, setApprovalSnapshot] = useState<ApprovalRequest | null>(null);
const [restorePointSnapshot, setRestorePointSnapshot] = useState<RecoveryPoint | null>(null);

useEffect(() => {
  const state = readDashboardSafetyNavigationState(location.state);
  const target = resolveDashboardSafetyFocusTarget({
    state,
    livePending: moduleData?.pending ?? [],
    liveRestorePoint: moduleData?.summary.latest_restore_point ?? null,
  });
  if (!target) return;

  setActiveDetailKey(target.activeDetailKey);
  setApprovalSnapshot(target.approvalSnapshot);
  setRestorePointSnapshot(target.restorePointSnapshot);
  setFeedback(target.feedback);
  navigate(location.pathname, { replace: true, state: null });
}, [location.pathname, location.state, moduleData, navigate]);
```

For rendering:

- approval detail should only bind a snapshot when `activeDetailKey` matches that same routed `approval_id`
- restore detail should prefer the live restore point when it is the same `recovery_point_id`, and only fall back to `restorePointSnapshot` when live data have changed away from the routed object

Target structure:

```ts
const activeApproval =
  approvalSnapshot && activeDetailKey === `approval:${approvalSnapshot.approval_id}`
    ? approvalLookup.get(`approval:${approvalSnapshot.approval_id}`) ?? approvalSnapshot
    : activeDetailKey && activeDetailKey.startsWith("approval:")
      ? approvalLookup.get(activeDetailKey)
      : undefined;

const activeRestorePoint =
  restorePointSnapshot && activeDetailKey === "restore"
    ? moduleData?.summary.latest_restore_point?.recovery_point_id === restorePointSnapshot.recovery_point_id
      ? moduleData.summary.latest_restore_point
      : restorePointSnapshot
    : moduleData?.summary.latest_restore_point ?? null;

const activePreview = getCardPreviewFromResolvedDetail({
  activeDetailKey,
  moduleData,
  activeApproval,
  activeRestorePoint,
  feedback,
  lastResolvedApproval,
});
```

If `getCardPreview(...)` stays as the preview builder, refactor it so it accepts the resolved live-or-snapshot approval/restore-point object instead of reading only from the current live `moduleData` collections.

After successful approval response:

```ts
const refreshPlan = getDashboardTaskSecurityRefreshPlan(dataMode);
await Promise.all(
  refreshPlan.invalidatePrefixes.map((queryKey) =>
    queryClient.invalidateQueries({ queryKey }),
  ),
);
```

- [ ] **Step 4: Run the full verification set**

Run: `pnpm --dir apps/desktop test:dashboard`
Expected: PASS.

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

Run: `go test ./services/local-service/internal/orchestrator ./services/local-service/internal/rpc`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/dashboard/dashboard.contract.test.ts apps/desktop/src/features/dashboard/tasks/taskPage.query.ts apps/desktop/src/features/dashboard/tasks/TaskPage.tsx apps/desktop/src/features/dashboard/tasks/taskPage.mapper.ts apps/desktop/src/features/dashboard/safety/SecurityApp.tsx
git commit -m "feat(dashboard): wire task safety focus into security page"
```

## Execution Notes

- Use @superpowers:test-driven-development before each implementation task.
- Use @superpowers:verification-before-completion before claiming the feature is done.
- Do not merge the three tasks into one commit; keep the commit boundaries exactly aligned with the slices above.
- If implementation reveals that task-level approval truth is not actually recoverable from the runtime/storage-backed task record, stop and repair the truth-source selection before wiring frontend code.
