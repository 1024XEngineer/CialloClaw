import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";
import type {
  AgentTaskControlParams,
  AgentTaskControlResult,
  AgentTaskDetailGetParams,
  AgentTaskDetailGetResult,
  AgentTaskListParams,
  AgentTaskListResult,
  ApprovalRequest,
  RecoveryPoint,
  Task,
} from "@cialloclaw/protocol";

declare module "@/rpc/methods" {
  export function controlTask(params: AgentTaskControlParams): Promise<AgentTaskControlResult>;
  export function getTaskDetail(params: AgentTaskDetailGetParams): Promise<AgentTaskDetailGetResult>;
  export function listTasks(params: AgentTaskListParams): Promise<AgentTaskListResult>;
}

const desktopRoot = process.cwd();

function loadDashboardSafetyNavigationModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSafetyNavigation.js")) as {
      buildDashboardSafetyNavigationState: (detail: AgentTaskDetailGetResult) => unknown;
      readDashboardSafetyNavigationState: (value: unknown) => unknown;
      resolveDashboardSafetyNavigationRoute: (input: {
        locationState: unknown;
        livePending: ApprovalRequest[];
        liveRestorePoint: RecoveryPoint | null;
      }) => unknown;
      resolveDashboardSafetyFocusTarget: (input: {
        state: unknown;
        livePending: ApprovalRequest[];
        liveRestorePoint: RecoveryPoint | null;
      }) => unknown;
      shouldRetainDashboardSafetyActiveDetail: (input: {
        activeDetailKey: string | null;
        approvalSnapshot: ApprovalRequest | null;
        cardKeys: string[];
      }) => boolean;
      isDashboardSafetyApprovalSnapshotOnly: (input: {
        activeDetailKey: string | null;
        approvalSnapshot: ApprovalRequest | null;
        cardKeys: string[];
      }) => boolean;
      resolveDashboardSafetySnapshotLifecycle: (input: {
        activeDetailKey: string | null;
        routeDrivenDetailKey: string | null;
        approvalSnapshot: ApprovalRequest | null;
        restorePointSnapshot: RecoveryPoint | null;
        subscribedTaskId: string | null;
      }) => {
        approvalSnapshot: ApprovalRequest | null;
        restorePointSnapshot: RecoveryPoint | null;
        routeDrivenDetailKey: string | null;
        subscribedTaskId: string | null;
      };
    },
  );
}

function loadTaskPageQueryModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.query.js")) as {
      buildDashboardTaskBucketQueryKey: (dataMode: "rpc" | "mock", group: "unfinished" | "finished", limit: number) => unknown;
      buildDashboardTaskDetailQueryKey: (dataMode: "rpc" | "mock", taskId: string) => unknown;
      getDashboardTaskSecurityRefreshPlan: (dataMode: "rpc" | "mock") => unknown;
      resolveDashboardTaskSafetyOpenPlan: (detailSource: "rpc" | "mock" | "fallback") => unknown;
      shouldEnableDashboardTaskDetailQuery: (selectedTaskId: string | null, detailOpen: boolean) => boolean;
      dashboardTaskBucketQueryPrefix: unknown;
      dashboardTaskDetailQueryPrefix: unknown;
    },
  );
}

function loadTaskPageMapperModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.mapper.js")) as {
      getTaskPrimaryActions: (task: Task, detail: AgentTaskDetailGetResult) => Array<{ action: string; label: string; tooltip: string }>;
    },
  );
}

function loadSettingsServiceModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/services/settingsService.js")) as {
      loadSettings: () => { settings: { data_log: { provider_api_key_configured: boolean } } };
      saveSettings: (settings: unknown) => void;
    },
  );
}

function withDesktopAliasRuntime<T>(callback: (requireFn: NodeRequire) => T): T {
  const NodeModule = require("node:module") as {
    _load: (request: string, parent: unknown, isMain: boolean) => unknown;
    _resolveFilename: (request: string, parent: unknown, isMain: boolean, options?: unknown) => string;
  };
  const originalTsLoader = require.extensions[".ts"];
  const originalLoad = NodeModule._load;
  const originalResolveFilename = NodeModule._resolveFilename;
  const protocolRoot = resolve(desktopRoot, "..", "..", "packages", "protocol");

  NodeModule._resolveFilename = function resolveDesktopAlias(request: string, parent: unknown, isMain: boolean, options?: unknown) {
    if (request === "@/rpc/fallback") {
      return resolve(desktopRoot, ".cache/dashboard-tests/features/shell-ball/test-stubs/rpcFallback.js");
    }

    if (request.startsWith("@/")) {
      const modulePath = request.slice(2);
      const emittedBasePath = resolve(desktopRoot, ".cache/dashboard-tests", modulePath);
      const emittedCandidates = [`${emittedBasePath}.js`, resolve(emittedBasePath, "index.js")];

      for (const candidate of emittedCandidates) {
        if (existsSync(candidate)) {
          return candidate;
        }
      }
    }

    if (request === "@cialloclaw/protocol") {
      return resolve(protocolRoot, "index.ts");
    }

    return originalResolveFilename.call(this, request, parent, isMain, options);
  };

  require.extensions[".ts"] = (module, filename) => {
    const source = require("node:fs").readFileSync(filename, "utf8") as string;
    const transpiled = ts.transpileModule(source, {
      compilerOptions: {
        esModuleInterop: true,
        module: ts.ModuleKind.CommonJS,
        moduleResolution: ts.ModuleResolutionKind.NodeJs,
        target: ts.ScriptTarget.ES2022,
      },
      fileName: filename,
    });

    (module as unknown as { _compile(code: string, fileName: string): void })._compile(transpiled.outputText, filename);
  };

  NodeModule._load = function loadDesktopRuntime(request: string, parent: unknown, isMain: boolean) {
    if (request === "@cialloclaw/protocol") {
      return originalLoad(resolve(protocolRoot, "types/core.ts"), parent, isMain);
    }

    if (request === "@/rpc/methods") {
      return {
        controlTask() {
          throw new Error("controlTask should not run in dashboard contract tests");
        },
        getTaskDetail() {
          throw new Error("getTaskDetail should not run in dashboard contract tests");
        },
        listTasks() {
          throw new Error("listTasks should not run in dashboard contract tests");
        },
      };
    }

    return originalLoad(request, parent, isMain);
  };

  try {
    return callback(require);
  } finally {
    if (originalTsLoader === undefined) {
      Reflect.deleteProperty(require.extensions, ".ts");
    } else {
      require.extensions[".ts"] = originalTsLoader;
    }
    NodeModule._load = originalLoad;
    NodeModule._resolveFilename = originalResolveFilename;
  }
}

function createTask(overrides: Partial<Task> = {}): Task {
  return {
    task_id: "task_dashboard_001",
    title: "Review dashboard safety state",
    status: "waiting_auth",
    source_type: "hover_input",
    updated_at: "2026-04-13T09:05:00.000Z",
    started_at: "2026-04-13T09:00:30.000Z",
    finished_at: null,
    intent: null,
    current_step: "Awaiting approval",
    risk_level: "yellow",
    ...overrides,
  };
}

function createApprovalRequest(overrides: Partial<ApprovalRequest> = {}): ApprovalRequest {
  return {
    approval_id: "approval_dashboard_001",
    task_id: "task_dashboard_001",
    operation_name: "write_file",
    risk_level: "yellow",
    target_object: "workspace/task.md",
    reason: "Need confirmation before updating the file.",
    status: "pending",
    created_at: "2026-04-13T09:01:00.000Z",
    ...overrides,
  };
}

function createRecoveryPoint(overrides: Partial<RecoveryPoint> = {}): RecoveryPoint {
  return {
    recovery_point_id: "rp_dashboard_001",
    task_id: "task_dashboard_001",
    summary: "Snapshot before file edits",
    created_at: "2026-04-13T09:02:00.000Z",
    objects: ["workspace/task.md"],
    ...overrides,
  };
}

function createDetail(overrides: Partial<AgentTaskDetailGetResult> = {}): AgentTaskDetailGetResult {
  return {
    approval_request: createApprovalRequest(),
    artifacts: [],
    mirror_references: [],
    security_summary: {
      latest_restore_point: createRecoveryPoint(),
      pending_authorizations: 1,
      risk_level: "yellow",
      security_status: "pending_confirmation",
    },
    task: createTask(),
    timeline: [],
    ...overrides,
  };
}

test("buildDashboardSafetyNavigationState follows the approved task-detail route shape", () => {
  const { buildDashboardSafetyNavigationState } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail());

  assert.deepEqual(state, {
    approvalRequest: createApprovalRequest(),
    source: "task-detail",
    taskId: "task_dashboard_001",
  });

  assert.deepEqual(buildDashboardSafetyNavigationState(createDetail({ approval_request: null })), {
    restorePoint: createRecoveryPoint(),
    source: "task-detail",
    taskId: "task_dashboard_001",
  });

  assert.deepEqual(
    buildDashboardSafetyNavigationState(
      createDetail({
        approval_request: null,
        security_summary: {
          latest_restore_point: null,
          pending_authorizations: 0,
          risk_level: "yellow",
          security_status: "normal",
        },
      }),
    ),
    {
      source: "task-detail",
      taskId: "task_dashboard_001",
    },
  );
});

test("readDashboardSafetyNavigationState accepts valid routed state and rejects malformed values", () => {
  const { buildDashboardSafetyNavigationState, readDashboardSafetyNavigationState } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail({ approval_request: null }));

  assert.deepEqual(readDashboardSafetyNavigationState(state), state);
  assert.deepEqual(
    readDashboardSafetyNavigationState({
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    {
      source: "task-detail",
      taskId: "task_dashboard_001",
    },
  );
  assert.equal(readDashboardSafetyNavigationState({ taskId: 42 }), null);
  assert.equal(
    readDashboardSafetyNavigationState({
      approvalRequest: "approval_dashboard_001",
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      approvalRequest: createApprovalRequest({ risk_level: "orange" as never }),
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      approvalRequest: createApprovalRequest({ status: "waiting" as never }),
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      restorePoint: createRecoveryPoint(),
      source: "task-detail",
      taskId: "task_dashboard_001",
      unknown: true,
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      approvalRequest: createApprovalRequest(),
      restorePoint: createRecoveryPoint(),
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      approvalRequest: createApprovalRequest({ task_id: "task_dashboard_999" }),
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      restorePoint: createRecoveryPoint({ task_id: "task_dashboard_999" }),
      source: "task-detail",
      taskId: "task_dashboard_001",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      source: "other",
      taskId: "task_dashboard_001",
    }),
    null,
  );
});

test("resolveDashboardSafetyFocusTarget prefers matching live approval data over restore point", () => {
  const { buildDashboardSafetyNavigationState, resolveDashboardSafetyFocusTarget } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail());
  const liveApproval = createApprovalRequest({ reason: "Live approval state" });

  const target = resolveDashboardSafetyFocusTarget({
    livePending: [liveApproval],
    liveRestorePoint: createRecoveryPoint({ summary: "Live restore point" }),
    state,
  });

  assert.deepEqual(target, {
    activeDetailKey: "approval:approval_dashboard_001",
    approvalSnapshot: liveApproval,
    feedback: null,
    restorePointSnapshot: null,
  });
});

test("resolveDashboardSafetyFocusTarget keeps approval snapshot renderable when live approval changed away", () => {
  const { buildDashboardSafetyNavigationState, resolveDashboardSafetyFocusTarget } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail());

  const target = resolveDashboardSafetyFocusTarget({
    livePending: [createApprovalRequest({ approval_id: "approval_dashboard_999" })],
    liveRestorePoint: createRecoveryPoint(),
    state,
  });

  assert.deepEqual(target, {
    activeDetailKey: "approval:approval_dashboard_001",
    approvalSnapshot: createApprovalRequest(),
    feedback: "实时安全数据已变化，当前展示的是路由携带的快照。",
    restorePointSnapshot: null,
  });
});

test("resolveDashboardSafetyFocusTarget keeps restore snapshot renderable when live restore point changed away", () => {
  const { buildDashboardSafetyNavigationState, resolveDashboardSafetyFocusTarget } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail({ approval_request: null }));

  const target = resolveDashboardSafetyFocusTarget({
    livePending: [],
    liveRestorePoint: createRecoveryPoint({ recovery_point_id: "rp_dashboard_999" }),
    state,
  });

  assert.deepEqual(target, {
    activeDetailKey: "restore",
    approvalSnapshot: null,
    feedback: "实时安全数据已变化，当前展示的是路由携带的快照。",
    restorePointSnapshot: createRecoveryPoint(),
  });
});

test("resolveDashboardSafetyFocusTarget uses live restore point when it matches and no approval is routed", () => {
  const { buildDashboardSafetyNavigationState, resolveDashboardSafetyFocusTarget } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail({ approval_request: null }));
  const liveRestorePoint = createRecoveryPoint({ summary: "Live restore point" });

  const target = resolveDashboardSafetyFocusTarget({
    livePending: [],
    liveRestorePoint,
    state,
  });

  assert.deepEqual(target, {
    activeDetailKey: "restore",
    approvalSnapshot: null,
    feedback: null,
    restorePointSnapshot: liveRestorePoint,
  });
});

test("resolveDashboardSafetyFocusTarget returns empty focus state when no route anchor exists", () => {
  const { buildDashboardSafetyNavigationState, resolveDashboardSafetyFocusTarget } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(
    createDetail({
      approval_request: null,
      security_summary: {
        latest_restore_point: null,
        pending_authorizations: 0,
        risk_level: "yellow",
        security_status: "normal",
      },
    }),
  );

  assert.deepEqual(
    resolveDashboardSafetyFocusTarget({
      livePending: [],
      liveRestorePoint: null,
      state,
    }),
    {
      activeDetailKey: null,
      approvalSnapshot: null,
      feedback: null,
      restorePointSnapshot: null,
    },
  );
});

test("task page query helpers expose stable prefixes and keys", () => {
  const {
    buildDashboardTaskBucketQueryKey,
    buildDashboardTaskDetailQueryKey,
    getDashboardTaskSecurityRefreshPlan,
    dashboardTaskBucketQueryPrefix,
    dashboardTaskDetailQueryPrefix,
  } = loadTaskPageQueryModule();
  assert.deepEqual(dashboardTaskBucketQueryPrefix, ["dashboard", "tasks", "bucket"]);
  assert.deepEqual(dashboardTaskDetailQueryPrefix, ["dashboard", "tasks", "detail"]);
  assert.deepEqual(buildDashboardTaskBucketQueryKey("rpc", "unfinished", 12), ["dashboard", "tasks", "bucket", "rpc", "unfinished", 12]);
  assert.deepEqual(buildDashboardTaskDetailQueryKey("mock", "task_dashboard_001"), ["dashboard", "tasks", "detail", "mock", "task_dashboard_001"]);
  assert.deepEqual(getDashboardTaskSecurityRefreshPlan("rpc"), {
    invalidatePrefixes: [
      ["dashboard", "tasks", "bucket"],
      ["dashboard", "tasks", "detail"],
    ],
    refetchOnMount: true,
  });
  assert.deepEqual(getDashboardTaskSecurityRefreshPlan("mock"), {
    invalidatePrefixes: [
      ["dashboard", "tasks", "bucket"],
      ["dashboard", "tasks", "detail"],
    ],
    refetchOnMount: false,
  });
});

test("task page no longer exposes edit guidance and uses 安全总览 without anchors", () => {
  const mapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.mapper.ts"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");

  assert.doesNotMatch(mapperSource, /action: "edit"/);
  assert.doesNotMatch(mapperSource, /去悬浮球继续/);
  assert.match(mapperSource, /label: hasAnchor \? "安全详情" : "安全总览"/);
  assert.doesNotMatch(taskPageSource, /action === "edit"/);
});

test("task page removes waiting-input shell-ball action and keeps shell-ball guidance in the page notes", () => {
  const { getTaskPrimaryActions } = loadTaskPageMapperModule();
  const waitingAuthTask = createTask({ status: "waiting_auth" });
  const waitingInputTask = createTask({ status: "waiting_input" });
  const mapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.mapper.ts"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");

  assert.equal(getTaskPrimaryActions(waitingAuthTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: waitingAuthTask })).at(-1)?.label, "安全详情");
  assert.deepEqual(
    getTaskPrimaryActions(waitingInputTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: waitingInputTask })).map((action) => action.action),
    ["cancel", "open-safety"],
  );
  assert.doesNotMatch(mapperSource, /当前任务还在等待补充输入，如需修改或补充，请到悬浮球继续处理。/);
  assert.match(taskPageSource, /如需修改或补充当前任务，请到悬浮球继续处理。/);
});

test("settings service normalizes legacy stored snapshots before returning and saving", () => {
  const { loadSettings, saveSettings } = loadSettingsServiceModule();
  const originalWindow = globalThis.window;
  const storage = new Map<string, string>();
  const localStorage = {
    getItem(key: string) {
      return storage.get(key) ?? null;
    },
    setItem(key: string, value: string) {
      storage.set(key, value);
    },
    removeItem(key: string) {
      storage.delete(key);
    },
  };

  Object.assign(globalThis, {
    window: {
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          general: {
            language: "zh-CN",
            auto_launch: true,
            theme_mode: "follow_system",
            voice_notification_enabled: true,
            voice_type: "default_female",
            download: {
              workspace_path: "D:/CialloClawWorkspace",
              ask_before_save_each_file: true,
            },
          },
          floating_ball: {
            auto_snap: true,
            idle_translucent: true,
            position_mode: "draggable",
            size: "medium",
          },
          memory: {
            enabled: true,
            lifecycle: "30d",
            work_summary_interval: {
              unit: "day",
              value: 7,
            },
            profile_refresh_interval: {
              unit: "week",
              value: 2,
            },
          },
          task_automation: {
            inspect_on_startup: true,
            inspect_on_file_change: true,
            inspection_interval: {
              unit: "minute",
              value: 15,
            },
            task_sources: ["D:/workspace/todos"],
            remind_before_deadline: true,
            remind_when_stale: false,
          },
          data_log: {
            provider: "openai",
            budget_auto_downgrade: true,
          },
        },
      }),
    );

    const loaded = loadSettings();
    assert.equal(loaded.settings.data_log.provider_api_key_configured, false);

    saveSettings(loaded as never);

    assert.equal(JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}").settings.data_log.provider_api_key_configured, false);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("SecurityApp route resolution reacts to each new route state and exposes task refresh targets", () => {
  const { resolveDashboardSafetyNavigationRoute, resolveDashboardSafetySnapshotLifecycle } = loadDashboardSafetyNavigationModule();

  assert.deepEqual(
    resolveDashboardSafetyNavigationRoute({
      locationState: {
        approvalRequest: createApprovalRequest(),
        source: "task-detail",
        taskId: "task_dashboard_001",
      },
      livePending: [],
      liveRestorePoint: null,
    }),
    {
      activeDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      feedback: "实时安全数据已变化，当前展示的是路由携带的快照。",
      restorePointSnapshot: null,
      routedTaskId: "task_dashboard_001",
      shouldClearRouteState: true,
    },
  );

  assert.deepEqual(
    resolveDashboardSafetyNavigationRoute({
      locationState: {
        restorePoint: createRecoveryPoint(),
        source: "task-detail",
        taskId: "task_dashboard_001",
      },
      livePending: [],
      liveRestorePoint: createRecoveryPoint(),
    }),
    {
      activeDetailKey: "restore",
      approvalSnapshot: null,
      feedback: null,
      restorePointSnapshot: createRecoveryPoint(),
      routedTaskId: "task_dashboard_001",
      shouldClearRouteState: true,
    },
  );

  assert.deepEqual(
    resolveDashboardSafetyNavigationRoute({
      locationState: {
        source: "task-detail",
        taskId: "task_dashboard_001",
      },
      livePending: [createApprovalRequest()],
      liveRestorePoint: createRecoveryPoint(),
    }),
    {
      activeDetailKey: null,
      approvalSnapshot: null,
      feedback: null,
      restorePointSnapshot: null,
      routedTaskId: "task_dashboard_001",
      shouldClearRouteState: true,
    },
  );

  assert.deepEqual(
    resolveDashboardSafetyNavigationRoute({
      locationState: null,
      livePending: [],
      liveRestorePoint: null,
    }),
    {
      activeDetailKey: null,
      approvalSnapshot: null,
      feedback: null,
      restorePointSnapshot: null,
      routedTaskId: null,
      shouldClearRouteState: false,
    },
  );

  assert.deepEqual(
    resolveDashboardSafetySnapshotLifecycle({
      activeDetailKey: "approval:approval_dashboard_001",
      routeDrivenDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      restorePointSnapshot: null,
      subscribedTaskId: "task_dashboard_001",
    }),
    {
      approvalSnapshot: createApprovalRequest(),
      restorePointSnapshot: null,
      routeDrivenDetailKey: "approval:approval_dashboard_001",
      subscribedTaskId: "task_dashboard_001",
    },
  );
});

test("SecurityApp keeps snapshot-only approval detail renderable when live cards no longer contain it", () => {
  const { isDashboardSafetyApprovalSnapshotOnly, resolveDashboardSafetySnapshotLifecycle, shouldRetainDashboardSafetyActiveDetail } = loadDashboardSafetyNavigationModule();

  assert.equal(
    shouldRetainDashboardSafetyActiveDetail({
      activeDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      cardKeys: ["status", "restore"],
    }),
    true,
  );

  assert.equal(
    shouldRetainDashboardSafetyActiveDetail({
      activeDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest({ approval_id: "approval_dashboard_999" }),
      cardKeys: ["status", "restore"],
    }),
    false,
  );

  assert.equal(
    shouldRetainDashboardSafetyActiveDetail({
      activeDetailKey: "restore",
      approvalSnapshot: null,
      cardKeys: ["status", "restore"],
    }),
    true,
  );

  assert.equal(
    isDashboardSafetyApprovalSnapshotOnly({
      activeDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      cardKeys: ["status", "restore"],
    }),
    true,
  );

  assert.equal(
    isDashboardSafetyApprovalSnapshotOnly({
      activeDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      cardKeys: ["status", "approval:approval_dashboard_001"],
    }),
    false,
  );

  assert.deepEqual(
    resolveDashboardSafetySnapshotLifecycle({
      activeDetailKey: "approval:approval_dashboard_001",
      routeDrivenDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      restorePointSnapshot: null,
      subscribedTaskId: "task_dashboard_001",
    }),
    {
      approvalSnapshot: createApprovalRequest(),
      restorePointSnapshot: null,
      routeDrivenDetailKey: "approval:approval_dashboard_001",
      subscribedTaskId: "task_dashboard_001",
    },
  );

  assert.deepEqual(
    resolveDashboardSafetySnapshotLifecycle({
      activeDetailKey: "status",
      routeDrivenDetailKey: "approval:approval_dashboard_001",
      approvalSnapshot: createApprovalRequest(),
      restorePointSnapshot: null,
      subscribedTaskId: "task_dashboard_001",
    }),
    {
      approvalSnapshot: null,
      restorePointSnapshot: null,
      routeDrivenDetailKey: null,
      subscribedTaskId: null,
    },
  );

  assert.deepEqual(
    resolveDashboardSafetySnapshotLifecycle({
      activeDetailKey: null,
      routeDrivenDetailKey: "restore",
      approvalSnapshot: null,
      restorePointSnapshot: createRecoveryPoint(),
      subscribedTaskId: "task_dashboard_001",
    }),
    {
      approvalSnapshot: null,
      restorePointSnapshot: null,
      routeDrivenDetailKey: null,
      subscribedTaskId: null,
    },
  );
});

test("TaskPage wiring helpers require real detail for safety focus and keep detail query task-id centric", () => {
  const { resolveDashboardTaskSafetyOpenPlan, shouldEnableDashboardTaskDetailQuery } = loadTaskPageQueryModule();

  assert.deepEqual(resolveDashboardTaskSafetyOpenPlan("fallback"), {
    shouldRefetchDetail: true,
  });
  assert.deepEqual(resolveDashboardTaskSafetyOpenPlan("rpc"), {
    shouldRefetchDetail: false,
  });
  assert.equal(shouldEnableDashboardTaskDetailQuery("task_dashboard_001", true), true);
  assert.equal(shouldEnableDashboardTaskDetailQuery("task_dashboard_001", false), false);
  assert.equal(shouldEnableDashboardTaskDetailQuery(null, true), false);
});

test("task detail normalization rejects string restore points in rpc mode and keeps null approval fallback", () => {
  withDesktopAliasRuntime((requireFn) => {
    const service = requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js")) as {
      buildFallbackTaskDetailData: (item: { experience: ReturnType<typeof createFallbackExperience>; task: Task }) => { detail: AgentTaskDetailGetResult };
      normalizeTaskDetailResult: (detail: AgentTaskDetailGetResult) => AgentTaskDetailGetResult;
    };

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            security_summary: {
              latest_restore_point: "rp_dashboard_001" as never,
              pending_authorizations: 1,
              risk_level: "yellow",
              security_status: "pending_confirmation",
            },
          }),
        ),
      /restore point/i,
    );

    const fallback = service.buildFallbackTaskDetailData({
      experience: createFallbackExperience(),
      task: createTask({ status: "waiting_auth" }),
    });

    assert.equal(fallback.detail.approval_request, null);
    assert.equal(fallback.detail.security_summary.pending_authorizations, 0);
    assert.equal(fallback.detail.security_summary.security_status, "normal");
  });
});

test("task detail normalization fails fast on invalid artifacts, mirror references, and timeline steps", () => {
  withDesktopAliasRuntime((requireFn) => {
    const service = requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js")) as {
      normalizeTaskDetailResult: (detail: AgentTaskDetailGetResult) => AgentTaskDetailGetResult;
    };

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            task: { task_id: "task_dashboard_001" } as never,
          }),
        ),
      /task information|task payload/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult({
          ...createDetail(),
          approval_request: undefined as never,
        }),
      /approval_request/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            security_summary: {
              pending_authorizations: 1,
              risk_level: "yellow",
              security_status: "pending_confirmation",
            } as never,
          }),
        ),
      /security summary|restore point/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            artifacts: [{ artifact_id: "artifact_1" } as never],
          }),
        ),
      /artifacts/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            mirror_references: [{ memory_id: "memory_1" } as never],
          }),
        ),
      /mirror/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            timeline: [{ step_id: "step_1" } as never],
          }),
        ),
      /timeline/i,
    );
  });
});

test("task detail normalization rejects pending authorization counts outside the contract", () => {
  withDesktopAliasRuntime((requireFn) => {
    const service = requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js")) as {
      normalizeTaskDetailResult: (detail: AgentTaskDetailGetResult) => AgentTaskDetailGetResult;
    };

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            security_summary: {
              latest_restore_point: createRecoveryPoint(),
              pending_authorizations: 2 as 0 | 1,
              risk_level: "yellow",
              security_status: "pending_confirmation",
            },
          }),
        ),
      /security summary|pending authorization/i,
    );
  });
});

test("task detail normalization enforces approval and restore-point task invariants", () => {
  withDesktopAliasRuntime((requireFn) => {
    const service = requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js")) as {
      normalizeTaskDetailResult: (detail: AgentTaskDetailGetResult) => AgentTaskDetailGetResult;
    };

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            approval_request: null,
            security_summary: {
              latest_restore_point: createRecoveryPoint(),
              pending_authorizations: 1,
              risk_level: "yellow",
              security_status: "pending_confirmation",
            },
          }),
        ),
      /pending authorization|approval/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            security_summary: {
              latest_restore_point: createRecoveryPoint(),
              pending_authorizations: 0,
              risk_level: "yellow",
              security_status: "pending_confirmation",
            },
          }),
        ),
      /pending authorization|approval/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            approval_request: createApprovalRequest({ task_id: "task_dashboard_999" }),
          }),
        ),
      /approval_request|task_id/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            security_summary: {
              latest_restore_point: createRecoveryPoint({ task_id: "task_dashboard_999" }),
              pending_authorizations: 1,
              risk_level: "yellow",
              security_status: "pending_confirmation",
            },
          }),
        ),
      /restore point|task_id/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            task: createTask({ status: "processing" }),
          }),
        ),
      /waiting_auth|approval/i,
    );

    assert.throws(
      () =>
        service.normalizeTaskDetailResult(
          createDetail({
            approval_request: createApprovalRequest({ status: "approved" }),
          }),
        ),
      /active|pending|approval/i,
    );
  });
});

test("TaskDetailPanel defers the entire fallback security summary until formal detail arrives", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /detailData\.source === "fallback" \|\| detailState !== "ready"/);
  assert.match(panelSource, /等待详情同步后展示风险、授权与恢复点/);
});

test("dashboard validators read enum truth sources from protocol exports", () => {
  const validatorSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/shared/dashboardContractValidators.ts"), "utf8");

  assert.match(validatorSource, /import\s*\{[^}]*APPROVAL_STATUSES[^}]*RISK_LEVELS[^}]*\}\s*from\s*"@cialloclaw\/protocol"/);
});

function createFallbackExperience() {
  return {
    acceptance: [],
    assistantState: {
      hint: "fallback",
      label: "fallback",
    },
    background: "fallback",
    constraints: [],
    dueAt: null,
    goal: "fallback",
    nextAction: "fallback",
    noteDraft: "fallback",
    noteEntries: [],
    outputs: [],
    phase: "fallback",
    priority: "steady" as const,
    progressHint: "fallback",
    quickContext: [],
    recentConversation: [],
    relatedFiles: [],
    stepTargets: {},
    suggestedNext: "fallback",
  };
}
