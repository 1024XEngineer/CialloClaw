import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";
import type {
  AgentDeliveryOpenResult,
  AgentNotepadConvertToTaskParams,
  AgentNotepadConvertToTaskResult,
  AgentNotepadListParams,
  AgentNotepadListResult,
  AgentNotepadUpdateParams,
  AgentNotepadUpdateResult,
  AgentTaskArtifactListResult,
  AgentTaskArtifactOpenResult,
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
  export function convertNotepadToTask(params: AgentNotepadConvertToTaskParams): Promise<AgentNotepadConvertToTaskResult>;
  export function controlTask(params: AgentTaskControlParams): Promise<AgentTaskControlResult>;
  export function getTaskDetail(params: AgentTaskDetailGetParams): Promise<AgentTaskDetailGetResult>;
  export function listNotepad(params: AgentNotepadListParams): Promise<AgentNotepadListResult>;
  export function listTasks(params: AgentTaskListParams): Promise<AgentTaskListResult>;
  export function updateNotepad(params: AgentNotepadUpdateParams): Promise<AgentNotepadUpdateResult>;
}

const desktopRoot = process.cwd();

function loadDashboardSafetyNavigationModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSafetyNavigation.js")) as {
      buildDashboardSafetyCardNavigationState: (focusCard: "status" | "budget" | "governance") => unknown;
      buildDashboardSafetyNavigationState: (detail: AgentTaskDetailGetResult) => unknown;
      buildDashboardSafetyRestorePointNavigationState: (restorePoint: RecoveryPoint) => unknown;
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
      buildDashboardTaskArtifactQueryKey: (dataMode: "rpc" | "mock", taskId: string) => unknown;
      buildDashboardTaskBucketQueryKey: (dataMode: "rpc" | "mock", group: "unfinished" | "finished", limit: number) => unknown;
      buildDashboardTaskDetailQueryKey: (dataMode: "rpc" | "mock", taskId: string) => unknown;
      getDashboardTaskSecurityRefreshPlan: (dataMode: "rpc" | "mock") => unknown;
      resolveDashboardTaskSafetyOpenPlan: (detailSource: "rpc" | "mock" | "fallback") => unknown;
      shouldEnableDashboardTaskDetailQuery: (selectedTaskId: string | null, detailOpen: boolean) => boolean;
      dashboardTaskArtifactQueryPrefix: unknown;
      dashboardTaskBucketQueryPrefix: unknown;
      dashboardTaskDetailQueryPrefix: unknown;
    },
  );
}

function loadNotePageQueryModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/notePage.query.js")) as {
      buildDashboardNoteBucketInvalidateKeys: (dataMode: "rpc" | "mock", groups: ReadonlyArray<"upcoming" | "later" | "recurring_rule" | "closed">) => unknown;
      buildDashboardNoteBucketQueryKey: (dataMode: "rpc" | "mock", group: "upcoming" | "later" | "recurring_rule" | "closed") => unknown;
      getDashboardNoteRefreshPlan: (dataMode: "rpc" | "mock") => unknown;
      dashboardNoteBucketGroups: unknown;
      dashboardNoteBucketQueryPrefix: unknown;
    },
  );
}

function loadNotePageServiceModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/notePage.service.js")) as {
      isAllowedNoteOpenUrl: (url: string) => boolean;
      resolveNoteResourceOpenExecutionPlan: (resource: {
        id: string;
        label: string;
        openAction?: "task_detail" | "open_url" | "copy_path" | null;
        path: string;
        taskId?: string | null;
        type: string;
        url?: string | null;
      }) => {
        mode: "task_detail" | "open_url" | "copy_path";
        taskId: string | null;
        path: string | null;
        url: string | null;
        feedback: string;
      };
    },
  );
}

function loadTaskOutputServiceModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskOutput.service.js")) as {
      describeTaskOpenResultForCurrentTask: (plan: { mode: string; taskId: string | null }, currentTaskId: string | null) => string | null;
      isAllowedTaskOpenUrl: (url: string) => boolean;
      loadTaskArtifactPage: (taskId: string, source: "rpc" | "mock") => Promise<AgentTaskArtifactListResult>;
      openTaskArtifactForTask: (taskId: string, artifactId: string, source: "rpc" | "mock") => Promise<AgentTaskArtifactOpenResult>;
      openTaskDeliveryForTask: (taskId: string, artifactId: string | undefined, source: "rpc" | "mock") => Promise<AgentDeliveryOpenResult>;
      resolveTaskOpenExecutionPlan: (result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult) => {
        mode: "task_detail" | "open_url" | "copy_path";
        taskId: string | null;
        path: string | null;
        url: string | null;
        feedback: string;
      };
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
      loadSettings: () => {
        settings: {
          data_log: {
            provider: string;
            budget_auto_downgrade: boolean;
            provider_api_key_configured: boolean;
          };
          models: {
            provider: string;
            budget_auto_downgrade: boolean;
            provider_api_key_configured: boolean;
            base_url: string;
            model: string;
          };
          general: {
            download: {
              ask_before_save_each_file: boolean;
            };
          };
          memory: {
            enabled: boolean;
            lifecycle: string;
          };
        };
      };
      saveSettings: (settings: unknown) => void;
    },
  );
}

function loadDashboardSettingsMutationModule(rpcMethods?: {
  controlTask?: (params: AgentTaskControlParams) => Promise<AgentTaskControlResult>;
  convertNotepadToTask?: (params: AgentNotepadConvertToTaskParams) => Promise<AgentNotepadConvertToTaskResult>;
  updateSettings?: (params: unknown) => Promise<unknown>;
  getSettingsDetailed?: (params: unknown) => Promise<unknown>;
  getTaskDetail?: (params: AgentTaskDetailGetParams) => Promise<AgentTaskDetailGetResult>;
  listNotepad?: (params: AgentNotepadListParams) => Promise<AgentNotepadListResult>;
  listTasks?: (params: AgentTaskListParams) => Promise<AgentTaskListResult>;
  updateNotepad?: (params: AgentNotepadUpdateParams) => Promise<AgentNotepadUpdateResult>;
}) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsMutation.js");

    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      updateDashboardSettings: (patch: Record<string, unknown>, source?: "rpc" | "mock") => Promise<{
        applyMode: string;
        needRestart: boolean;
        persisted: boolean;
        source: string;
        updatedKeys: string[];
        snapshot: {
          settings: {
            data_log: {
              budget_auto_downgrade: boolean;
            };
            general: {
              download: {
                ask_before_save_each_file: boolean;
              };
            };
            memory: {
              enabled: boolean;
              lifecycle: string;
            };
          };
        };
      }>;
    };
  }, rpcMethods);
}

type DashboardContractRpcMethodOverrides = {
  controlTask?: (params: AgentTaskControlParams) => Promise<AgentTaskControlResult>;
  convertNotepadToTask?: (params: AgentNotepadConvertToTaskParams) => Promise<AgentNotepadConvertToTaskResult>;
  updateSettings?: (params: unknown) => Promise<unknown>;
  getSettingsDetailed?: (params: unknown) => Promise<unknown>;
  getTaskDetail?: (params: AgentTaskDetailGetParams) => Promise<AgentTaskDetailGetResult>;
  listNotepad?: (params: AgentNotepadListParams) => Promise<AgentNotepadListResult>;
  listTasks?: (params: AgentTaskListParams) => Promise<AgentTaskListResult>;
  updateNotepad?: (params: AgentNotepadUpdateParams) => Promise<AgentNotepadUpdateResult>;
};

function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => Promise<T>,
  rpcMethods?: DashboardContractRpcMethodOverrides,
): Promise<T>;
function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => T,
  rpcMethods?: DashboardContractRpcMethodOverrides,
): T;
function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => T | Promise<T>,
  rpcMethods?: {
    controlTask?: (params: AgentTaskControlParams) => Promise<AgentTaskControlResult>;
    convertNotepadToTask?: (params: AgentNotepadConvertToTaskParams) => Promise<AgentNotepadConvertToTaskResult>;
    updateSettings?: (params: unknown) => Promise<unknown>;
    getSettingsDetailed?: (params: unknown) => Promise<unknown>;
    getTaskDetail?: (params: AgentTaskDetailGetParams) => Promise<AgentTaskDetailGetResult>;
    listNotepad?: (params: AgentNotepadListParams) => Promise<AgentNotepadListResult>;
    listTasks?: (params: AgentTaskListParams) => Promise<AgentTaskListResult>;
    updateNotepad?: (params: AgentNotepadUpdateParams) => Promise<AgentNotepadUpdateResult>;
  },
): T | Promise<T> {
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
        controlTask:
          rpcMethods?.controlTask ??
          (() => {
            throw new Error("controlTask should not run in dashboard contract tests");
          }),
        convertNotepadToTask:
          rpcMethods?.convertNotepadToTask ??
          (() => {
            throw new Error("convertNotepadToTask should not run in dashboard contract tests");
          }),
        getTaskDetail:
          rpcMethods?.getTaskDetail ??
          (() => {
            throw new Error("getTaskDetail should not run in dashboard contract tests");
          }),
        listNotepad:
          rpcMethods?.listNotepad ??
          (() => {
            throw new Error("listNotepad should not run in dashboard contract tests");
          }),
        listTaskArtifacts() {
          throw new Error("listTaskArtifacts should not run in dashboard contract tests");
        },
        listTasks:
          rpcMethods?.listTasks ??
          (() => {
            throw new Error("listTasks should not run in dashboard contract tests");
          }),
        openDelivery() {
          throw new Error("openDelivery should not run in dashboard contract tests");
        },
        openTaskArtifact() {
          throw new Error("openTaskArtifact should not run in dashboard contract tests");
        },
        updateNotepad:
          rpcMethods?.updateNotepad ??
          (() => {
            throw new Error("updateNotepad should not run in dashboard contract tests");
          }),
        getSettingsDetailed: rpcMethods?.getSettingsDetailed ?? (() => Promise.reject(new Error("getSettingsDetailed should not run in dashboard contract tests"))),
        updateSettings: rpcMethods?.updateSettings ?? (() => Promise.reject(new Error("updateSettings should not run in dashboard contract tests"))),
      };
    }

    return originalLoad(request, parent, isMain);
  };

  const restoreRuntime = () => {
    if (originalTsLoader === undefined) {
      Reflect.deleteProperty(require.extensions, ".ts");
    } else {
      require.extensions[".ts"] = originalTsLoader;
    }
    NodeModule._load = originalLoad;
    NodeModule._resolveFilename = originalResolveFilename;
  };

  try {
    const result = callback(require);
    if (result && typeof (result as unknown as { then?: unknown }).then === "function") {
      return (result as Promise<T>).finally(restoreRuntime);
    }

    restoreRuntime();
    return result;
  } catch (error) {
    restoreRuntime();
    throw error;
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
    citations: [],
    mirror_references: [],
    runtime_summary: {
      active_steering_count: 0,
      events_count: 0,
      latest_failure_code: null,
      latest_failure_category: null,
      latest_failure_summary: null,
      latest_event_type: null,
      loop_stop_reason: null,
      observation_signals: [],
    },
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

test("buildDashboardSafetyRestorePointNavigationState keeps mirror restore deep links within the safety route contract", () => {
  const { buildDashboardSafetyRestorePointNavigationState, readDashboardSafetyNavigationState } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyRestorePointNavigationState(createRecoveryPoint());

  assert.deepEqual(state, {
    restorePoint: createRecoveryPoint(),
    source: "mirror-detail",
    taskId: "task_dashboard_001",
  });
  assert.deepEqual(readDashboardSafetyNavigationState(state), state);
});

test("buildDashboardSafetyCardNavigationState keeps mirror static-card deep links within the safety route contract", () => {
  const { buildDashboardSafetyCardNavigationState, readDashboardSafetyNavigationState } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyCardNavigationState("budget");

  assert.deepEqual(state, {
    focusCard: "budget",
    source: "mirror-detail",
  });
  assert.deepEqual(readDashboardSafetyNavigationState(state), state);
});

test("readDashboardSafetyNavigationState accepts valid routed state and rejects malformed values", () => {
  const { buildDashboardSafetyCardNavigationState, buildDashboardSafetyNavigationState, readDashboardSafetyNavigationState } = loadDashboardSafetyNavigationModule();
  const state = buildDashboardSafetyNavigationState(createDetail({ approval_request: null }));

  assert.deepEqual(readDashboardSafetyNavigationState(state), state);
  assert.deepEqual(readDashboardSafetyNavigationState(buildDashboardSafetyCardNavigationState("status")), {
    focusCard: "status",
    source: "mirror-detail",
  });
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
      focusCard: "restore",
      source: "mirror-detail",
    }),
    null,
  );
  assert.equal(
    readDashboardSafetyNavigationState({
      focusCard: "budget",
      restorePoint: createRecoveryPoint(),
      source: "mirror-detail",
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

test("resolveDashboardSafetyFocusTarget keeps mirror static-card routes anchored to the requested safety card", () => {
  const { buildDashboardSafetyCardNavigationState, resolveDashboardSafetyFocusTarget } = loadDashboardSafetyNavigationModule();
  const target = resolveDashboardSafetyFocusTarget({
    livePending: [createApprovalRequest()],
    liveRestorePoint: createRecoveryPoint(),
    state: buildDashboardSafetyCardNavigationState("status"),
  });

  assert.deepEqual(target, {
    activeDetailKey: "status",
    approvalSnapshot: null,
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
    buildDashboardTaskArtifactQueryKey,
    buildDashboardTaskBucketQueryKey,
    buildDashboardTaskDetailQueryKey,
    dashboardTaskArtifactQueryPrefix,
    getDashboardTaskSecurityRefreshPlan,
    dashboardTaskBucketQueryPrefix,
    dashboardTaskDetailQueryPrefix,
  } = loadTaskPageQueryModule();
  assert.deepEqual(dashboardTaskArtifactQueryPrefix, ["dashboard", "tasks", "artifacts"]);
  assert.deepEqual(dashboardTaskBucketQueryPrefix, ["dashboard", "tasks", "bucket"]);
  assert.deepEqual(dashboardTaskDetailQueryPrefix, ["dashboard", "tasks", "detail"]);
  assert.deepEqual(buildDashboardTaskArtifactQueryKey("rpc", "task_dashboard_001"), ["dashboard", "tasks", "artifacts", "rpc", "task_dashboard_001"]);
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

test("note page query helpers expose stable prefixes, bucket order, and refresh-key mapping", () => {
  const {
    buildDashboardNoteBucketInvalidateKeys,
    buildDashboardNoteBucketQueryKey,
    getDashboardNoteRefreshPlan,
    dashboardNoteBucketGroups,
    dashboardNoteBucketQueryPrefix,
  } = loadNotePageQueryModule();

  assert.deepEqual(dashboardNoteBucketQueryPrefix, ["dashboard", "notes", "bucket"]);
  assert.deepEqual(dashboardNoteBucketGroups, ["upcoming", "later", "recurring_rule", "closed"]);
  assert.deepEqual(buildDashboardNoteBucketQueryKey("rpc", "upcoming"), ["dashboard", "notes", "bucket", "rpc", "upcoming"]);
  assert.deepEqual(buildDashboardNoteBucketInvalidateKeys("mock", ["upcoming", "closed", "upcoming"]), [
    ["dashboard", "notes", "bucket", "mock", "upcoming"],
    ["dashboard", "notes", "bucket", "mock", "closed"],
  ]);
  assert.deepEqual(getDashboardNoteRefreshPlan("rpc"), {
    invalidatePrefixes: [["dashboard", "notes", "bucket"]],
    refetchOnMount: true,
  });
  assert.deepEqual(getDashboardNoteRefreshPlan("mock"), {
    invalidatePrefixes: [["dashboard", "notes", "bucket"]],
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

test("security board styles stay scoped to the safety feature stylesheet", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");
  const securityBoardSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/securityBoard.css"), "utf8");
  const globalsSource = readFileSync(resolve(desktopRoot, "src/styles/globals.css"), "utf8");

  assert.match(securityAppSource, /import "\.\/securityBoard\.css";/);
  assert.match(securityBoardSource, /\.security-page__canvas\s*\{/);
  assert.match(securityBoardSource, /@media \(max-width: 980px\)[\s\S]*\.security-page__detail-grid\s*\{/);
  assert.doesNotMatch(globalsSource, /\.security-page__canvas\s*\{/);
  assert.doesNotMatch(globalsSource, /\.security-page__draggable\s*\{/);
});

test("SecurityApp keeps task-detail navigation hooks above the module-data early return", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");
  const earlyReturnIndex = securityAppSource.search(/if \(!moduleData\) \{\s*return \(\s*<main className="app-shell security-page">/);
  const openTaskDetailHookIndex = securityAppSource.indexOf("const openTaskDetail = useCallback");

  assert.notEqual(earlyReturnIndex, -1);
  assert.notEqual(openTaskDetailHookIndex, -1);
  assert.ok(openTaskDetailHookIndex < earlyReturnIndex);
});

test("security audit cards and mirror cards stay aligned with the v6 frontend protocol contract", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");
  const mirrorAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorApp.tsx"), "utf8");
  const mirrorDetailSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorDetailContent.tsx"), "utf8");
  const rpcClientSource = readFileSync(resolve(desktopRoot, "src/rpc/client.ts"), "utf8");

  assert.match(securityAppSource, /const \[auditScope, setAuditScope\] = useState<SecurityAuditScope>\("focused_task"\)/);
  assert.match(securityAppSource, /const auditFilterTaskId = auditScope === "focused_task" \? focusedTaskId : null/);
  assert.match(securityAppSource, /const rpcAuditRequiresTaskContext = moduleData\?\.source === "rpc"/);
  assert.match(securityAppSource, /disabled=\{rpcAuditRequiresTaskContext\}/);
  assert.match(securityAppSource, /当前后端仅支持按 task 查看审计记录/);
  assert.match(securityAppSource, /loadSecurityAuditRecords\(moduleData\.source, auditFilterTaskId/);
  assert.match(securityAppSource, /loadSecurityFocusedTaskDetail\(focusedTaskId, moduleData\?\.source \?\? "rpc"\)/);
  assert.match(securityAppSource, /当前屏幕任务治理链/);
  assert.match(securityAppSource, /正式授权锚点/);
  assert.match(securityAppSource, /正式引用/);
  assert.match(securityAppSource, /latest_failure_category/);
  assert.match(securityAppSource, /title: "审计记录"/);
  assert.doesNotMatch(securityAppSource, /decisionHistory/);
  assert.doesNotMatch(securityAppSource, /loadDashboardSettingsSnapshot/);
  assert.match(rpcClientSource, /function readImportMetaEnv\(\)/);
  assert.match(rpcClientSource, /windowEnv\?\.debugEndpoint \?\? importMetaEnv\.debugEndpoint \?\? processEnv\?\.VITE_CIALLOCLAW_DEBUG_RPC_ENDPOINT/);
  assert.match(rpcClientSource, /windowEnv\?\.transport \?\?[\s\S]*importMetaEnv\.transport \?\?/);
  assert.match(mirrorAppSource, /overview\.history_summary\[0\] \?\? latestConversation\?\.user_text/);
  assert.match(mirrorAppSource, /overview\.history_summary\[1\] \?\?[\s\S]*latestConversation\?\.agent_text/);
  assert.match(mirrorAppSource, /latestMemoryReference\?\.summary \|\| latestMemoryReference\?\.reason/);
  assert.match(mirrorDetailSource, /reference\.summary \|\| reference\.reason/);
});

test("task context links back into mirror detail state instead of plain text dead ends", () => {
  const taskContextSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskContextBlock.tsx"), "utf8");
  const mirrorAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorApp.tsx"), "utf8");
  const mirrorDetailSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorDetailContent.tsx"), "utf8");

  assert.match(taskContextSource, /resolveDashboardModuleRoutePath\("memory"\)/);
  assert.match(taskContextSource, /activeDetailKey: "memory"/);
  assert.match(taskContextSource, /focusMemoryId: memoryId/);
  assert.match(taskContextSource, /activeDetailKey: "history"/);
  assert.match(mirrorAppSource, /readMirrorRouteState/);
  assert.match(mirrorAppSource, /focusMemoryId=\{focusedMemoryId\}/);
  assert.match(mirrorAppSource, /latestRestorePoint=\{mirrorData\.latestRestorePoint\}/);
  assert.match(mirrorAppSource, /navigate\(location\.pathname, \{ replace: true, state: null \}\)/);
  assert.match(mirrorDetailSource, /focusMemoryId: string \| null/);
  assert.match(mirrorDetailSource, /highlightedMemoryId/);
  assert.match(mirrorDetailSource, /当前任务引用/);
  assert.match(mirrorDetailSource, /resolveDashboardModuleRoutePath\("safety"\)/);
  assert.match(mirrorDetailSource, /buildDashboardSafetyCardNavigationState/);
  assert.match(mirrorDetailSource, /buildDashboardSafetyRestorePointNavigationState/);
  assert.match(mirrorDetailSource, /前往安全详情/);
  assert.match(mirrorDetailSource, /前往恢复点/);
  assert.match(mirrorDetailSource, /前往预算详情/);
  assert.match(mirrorDetailSource, /activeDetailKey: "history"/);
  assert.match(mirrorDetailSource, /historyDetailView: "conversation"/);
  assert.match(mirrorDetailSource, /前往本地对话/);
  assert.match(mirrorAppSource, /historyDetailView\?: MirrorHistoryDetailView/);
  assert.match(mirrorAppSource, /options\?: \{ focusMemoryId\?: string \| null; historyDetailView\?: MirrorHistoryDetailView \| null \}/);
  assert.match(mirrorAppSource, /setHistoryDetailView\(options\.historyDetailView\)/);
});

test("task page keeps waiting-auth anchors and waiting-input escape hatches", () => {
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

test("settings service keeps RPC data_log fields authoritative over stale desktop model aliases", () => {
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
          data_log: {
            provider: "anthropic",
            budget_auto_downgrade: false,
            provider_api_key_configured: true,
          },
          models: {
            provider: "openai",
            budget_auto_downgrade: true,
            provider_api_key_configured: false,
            base_url: "https://local-router.invalid/v1",
            model: "gpt-local",
          },
        },
      }),
    );

    const loaded = loadSettings();
    assert.equal(loaded.settings.data_log.provider, "anthropic");
    assert.equal(loaded.settings.data_log.budget_auto_downgrade, false);
    assert.equal(loaded.settings.data_log.provider_api_key_configured, true);
    assert.equal(loaded.settings.models.provider, "anthropic");
    assert.equal(loaded.settings.models.budget_auto_downgrade, false);
    assert.equal(loaded.settings.models.provider_api_key_configured, true);
    assert.equal(loaded.settings.models.base_url, "https://local-router.invalid/v1");
    assert.equal(loaded.settings.models.model, "gpt-local");

    saveSettings(loaded as never);

    const persisted = JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}");
    assert.equal(persisted.settings.data_log.provider, "anthropic");
    assert.equal(persisted.settings.data_log.provider_api_key_configured, true);
    assert.equal(persisted.settings.models.provider, "anthropic");
    assert.equal(persisted.settings.models.provider_api_key_configured, true);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("dashboard settings mutation updates the local snapshot in mock mode", async () => {
  const { loadSettings } = loadSettingsServiceModule();
  const { updateDashboardSettings } = loadDashboardSettingsMutationModule();
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
    const result = await updateDashboardSettings(
      {
        data_log: {
          budget_auto_downgrade: false,
        },
        general: {
          download: {
            ask_before_save_each_file: false,
          },
        },
        memory: {
          enabled: false,
          lifecycle: "session",
        },
      },
      "mock",
    );

    assert.equal(result.source, "mock");
    assert.equal(result.applyMode, "immediate");
    assert.equal(result.needRestart, false);
    assert.equal(result.persisted, true);
    assert.deepEqual(result.updatedKeys.sort(), ["data_log", "general", "memory"]);
    assert.equal(result.snapshot.settings.memory.enabled, false);
    assert.equal(result.snapshot.settings.memory.lifecycle, "session");
    assert.equal(result.snapshot.settings.general.download.ask_before_save_each_file, false);
    assert.equal(result.snapshot.settings.data_log.budget_auto_downgrade, false);

    const persisted = loadSettings();

    assert.equal(persisted.settings.memory.enabled, false);
    assert.equal(persisted.settings.memory.lifecycle, "session");
    assert.equal(persisted.settings.general.download.ask_before_save_each_file, false);
    assert.equal(persisted.settings.data_log.budget_auto_downgrade, false);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("dashboard settings mutation keeps fallback snapshots read-only when the RPC transport is unavailable", async () => {
  const { loadSettings } = loadSettingsServiceModule();
  const { updateDashboardSettings } = loadDashboardSettingsMutationModule({
    updateSettings: async () => {
      throw new Error("transport is not wired");
    },
  });
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
    const before = loadSettings();
    const result = await updateDashboardSettings({
      memory: {
        enabled: false,
        lifecycle: "session",
      },
    });
    const after = loadSettings();

    assert.equal(result.source, "mock");
    assert.equal(result.persisted, false);
    assert.deepEqual(result.updatedKeys, []);
    assert.equal(result.snapshot.settings.memory.enabled, before.settings.memory.enabled);
    assert.equal(result.snapshot.settings.memory.lifecycle, before.settings.memory.lifecycle);
    assert.equal(after.settings.memory.enabled, before.settings.memory.enabled);
    assert.equal(after.settings.memory.lifecycle, before.settings.memory.lifecycle);
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

test("task output helpers normalize open actions from existing rpc contracts", async () => {
  const outputService = loadTaskOutputServiceModule();

  assert.deepEqual(
    outputService.resolveTaskOpenExecutionPlan({
      open_action: "task_detail",
      resolved_payload: { path: null, url: null, task_id: "task_dashboard_001" },
      delivery_result: {
        type: "task_detail",
        title: "Task detail",
        preview_text: "回到任务详情",
        payload: { path: null, url: null, task_id: "task_dashboard_001" },
      },
    }),
    {
      mode: "task_detail",
      taskId: "task_dashboard_001",
      path: null,
      url: null,
      feedback: "已定位到任务详情。",
    },
  );

  assert.deepEqual(
    outputService.resolveTaskOpenExecutionPlan({
      open_action: "result_page",
      resolved_payload: { path: null, url: "https://example.test/result", task_id: "task_dashboard_001" },
      delivery_result: {
        type: "result_page",
        title: "Result page",
        preview_text: "打开结果页",
        payload: { path: null, url: "https://example.test/result", task_id: "task_dashboard_001" },
      },
    }),
    {
      mode: "open_url",
      taskId: "task_dashboard_001",
      path: null,
      url: "https://example.test/result",
      feedback: "已打开结果页。",
    },
  );

  assert.deepEqual(
    outputService.resolveTaskOpenExecutionPlan({
      artifact: {
        artifact_id: "artifact_dashboard_001",
        artifact_type: "workspace_document",
        mime_type: "text/tsx",
        path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx",
        task_id: "task_dashboard_001",
        title: "TaskPage.tsx",
      },
      open_action: "open_file",
      resolved_payload: { path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx", url: null, task_id: "task_dashboard_001" },
      delivery_result: {
        type: "open_file",
        title: "TaskPage.tsx",
        preview_text: "打开文件",
        payload: { path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx", url: null, task_id: "task_dashboard_001" },
      },
    }),
    {
      mode: "copy_path",
      taskId: "task_dashboard_001",
      path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx",
      url: null,
      feedback: "当前环境暂不支持直接打开，已准备复制路径。",
    },
  );
});

test("task output service exposes artifact list and open flows in mock mode", async () => {
  const outputService = loadTaskOutputServiceModule();

  const artifactPage = await outputService.loadTaskArtifactPage("task_done_001", "mock");
  assert.ok(artifactPage.items.length > 0);
  assert.equal(artifactPage.page.offset, 0);

  const artifactOpen = await outputService.openTaskArtifactForTask("task_done_001", "artifact_done_003", "mock");
  assert.equal(artifactOpen.open_action, "reveal_in_folder");

  const deliveryOpen = await outputService.openTaskDeliveryForTask("task_done_001", undefined, "mock");
  assert.equal(deliveryOpen.delivery_result.payload.task_id, "task_done_001");

  assert.equal(
    outputService.describeTaskOpenResultForCurrentTask(
      {
        mode: "task_detail",
        taskId: "task_done_001",
      },
      "task_done_001",
    ),
    "当前任务没有独立可打开结果，请先查看成果区。",
  );

  assert.equal(outputService.isAllowedTaskOpenUrl("https://example.test/result"), true);
  assert.equal(outputService.isAllowedTaskOpenUrl("http://example.test/result"), true);
  assert.equal(outputService.isAllowedTaskOpenUrl("javascript:alert(1)"), false);
  assert.equal(outputService.isAllowedTaskOpenUrl("file:///tmp/out.txt"), false);
});

test("note resource open helpers normalize task, url, and copy flows", () => {
  const noteService = loadNotePageServiceModule();

  const taskPlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_001",
    label: "Task detail",
    openAction: "task_detail",
    path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx",
    taskId: "task_dashboard_001",
    type: "task",
    url: null,
  });
  assert.equal(taskPlan.mode, "task_detail");
  assert.equal(taskPlan.taskId, "task_dashboard_001");

  const urlPlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_002",
    label: "Spec",
    openAction: "open_url",
    path: "",
    taskId: null,
    type: "doc",
    url: "https://example.test/spec",
  });
  assert.equal(urlPlan.mode, "open_url");
  assert.equal(urlPlan.url, "https://example.test/spec");

  const copyPlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_003",
    label: "Draft",
    openAction: "copy_path",
    path: "workspace/drafts/spec.md",
    taskId: null,
    type: "draft",
    url: null,
  });
  assert.equal(copyPlan.mode, "copy_path");
  assert.equal(copyPlan.path, "workspace/drafts/spec.md");

  assert.equal(noteService.isAllowedNoteOpenUrl("https://example.test/spec"), true);
  assert.equal(noteService.isAllowedNoteOpenUrl("http://example.test/spec"), true);
  assert.equal(noteService.isAllowedNoteOpenUrl("javascript:alert(1)"), false);
  assert.equal(noteService.isAllowedNoteOpenUrl("file:///tmp/spec.md"), false);
});

test("task page adopts rpc output helpers directly in the task detail panel", () => {
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const taskDetailSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");
  const taskOutputSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskOutput.service.ts"), "utf8");

  assert.match(taskPageSource, /buildDashboardTaskArtifactQueryKey/);
  assert.match(taskPageSource, /loadTaskArtifactPage/);
  assert.match(taskPageSource, /openTaskArtifactForTask/);
  assert.match(taskPageSource, /openTaskDeliveryForTask/);
  assert.match(taskPageSource, /subscribeDeliveryReady\(\(payload\) =>/);
  assert.match(taskPageSource, /payload\.task_id/);
  assert.doesNotMatch(taskPageSource, /\["dashboard", "tasks", "artifacts"/);
  assert.doesNotMatch(taskPageSource, /TaskFilesSheet/);

  assert.doesNotMatch(taskDetailSource, /当前协议尚未提供稳定的 artifact\.open 能力/);
  assert.match(taskDetailSource, /onOpenArtifact/);
  assert.match(taskDetailSource, /onOpenLatestDelivery/);
  assert.doesNotMatch(taskDetailSource, /文件舱门/);
  assert.match(taskDetailSource, /artifactItems/);

  assert.doesNotMatch(taskOutputSource, /isRpcChannelUnavailable/);
  assert.doesNotMatch(taskOutputSource, /logRpcMockFallback/);
  assert.match(taskOutputSource, /isAllowedTaskOpenUrl/);
});

test("note page consumes note query helpers instead of inlining note bucket contracts", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.service.ts"), "utf8");

  assert.match(notePageSource, /buildDashboardNoteBucketQueryKey/);
  assert.match(notePageSource, /buildDashboardNoteBucketInvalidateKeys/);
  assert.match(notePageSource, /getDashboardNoteRefreshPlan/);
  assert.doesNotMatch(notePageSource, /\["dashboard", "notes", "bucket", dataMode/);
  assert.match(noteServiceSource, /isAllowedNoteOpenUrl/);
  assert.match(noteServiceSource, /mode === "open_url"/);
});

test("task fallback copy no longer claims backend output actions are missing", () => {
  const taskServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.service.ts"), "utf8");
  const taskTabsSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskTabsPanel.tsx"), "utf8");

  assert.doesNotMatch(taskServiceSource, /当前协议未返回更多结果摘要/);
  assert.doesNotMatch(taskServiceSource, /后续可把任务修改或产出打开能力接进来/);
  assert.doesNotMatch(taskTabsSource, /当前协议尚未提供稳定的 artifact\.open 能力/);
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
    assert.deepEqual(fallback.detail.runtime_summary, {
      active_steering_count: 0,
      events_count: 0,
      latest_failure_code: null,
      latest_failure_category: null,
      latest_failure_summary: null,
      latest_event_type: null,
      loop_stop_reason: null,
      observation_signals: [],
    });
    assert.equal(fallback.detail.security_summary.pending_authorizations, 0);
    assert.equal(fallback.detail.security_summary.security_status, "normal");
  });
});

test("task detail normalization recovers invalid artifacts and citations but still rejects broken mirrors and timeline steps", () => {
  withDesktopAliasRuntime((requireFn) => {
    const service = requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js")) as {
      normalizeTaskDetailData: (detail: AgentTaskDetailGetResult) => { detailWarningMessage: string | null; detail: AgentTaskDetailGetResult };
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
            runtime_summary: null as never,
          }),
        ),
      /runtime summary/i,
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

    const recovered = service.normalizeTaskDetailData(
      createDetail({
        artifacts: [{ artifact_id: "artifact_1" } as never],
      }),
    );

    assert.equal(recovered.detail.artifacts.length, 0);
    assert.match(recovered.detailWarningMessage ?? "", /成果信息暂时无法完整展示/);

    const recoveredCitation = service.normalizeTaskDetailData(
      createDetail({
        citations: [{ citation_id: "citation_1" } as never],
      }),
    );

    assert.equal(recoveredCitation.detail.citations.length, 0);
    assert.match(recoveredCitation.detailWarningMessage ?? "", /任务引用信息暂时无法完整展示/);

    const recoveredMirror = service.normalizeTaskDetailData(
      createDetail({
        mirror_references: [{ memory_id: "memory_1" } as never],
      }),
    );

    assert.equal(recoveredMirror.detail.mirror_references.length, 0);
    assert.match(recoveredMirror.detailWarningMessage ?? "", /镜子命中信息暂时无法完整展示/);

    const recoveredBoth = service.normalizeTaskDetailData(
      createDetail({
        artifacts: null as never,
        citations: null as never,
        mirror_references: null as never,
      }),
    );

    assert.equal(recoveredBoth.detail.artifacts.length, 0);
    assert.equal(recoveredBoth.detail.citations.length, 0);
    assert.equal(recoveredBoth.detail.mirror_references.length, 0);
    assert.match(recoveredBoth.detailWarningMessage ?? "", /成果信息暂时无法完整展示/);
    assert.match(recoveredBoth.detailWarningMessage ?? "", /任务引用信息暂时无法完整展示/);
    assert.match(recoveredBoth.detailWarningMessage ?? "", /镜子命中信息暂时无法完整展示/);

    const recoveredRuntimeSummary = service.normalizeTaskDetailResult({
      ...createDetail(),
      runtime_summary: undefined as never,
    });

    assert.equal(recoveredRuntimeSummary.runtime_summary.events_count, 0);
    assert.equal(recoveredRuntimeSummary.runtime_summary.active_steering_count, 0);
    assert.equal(recoveredRuntimeSummary.runtime_summary.latest_failure_category, null);
    assert.equal(recoveredRuntimeSummary.runtime_summary.latest_event_type, null);
    assert.equal(recoveredRuntimeSummary.runtime_summary.loop_stop_reason, null);

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

test("task rpc service keeps transport failures visible instead of switching to mock data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        controlTaskByAction: (taskId: string, action: "pause" | "resume" | "cancel" | "restart", source?: "rpc" | "mock") => Promise<unknown>;
        loadTaskBucketPage: (group: "unfinished" | "finished", options?: { limit?: number; offset?: number; source?: "rpc" | "mock" }) => Promise<unknown>;
        loadTaskDetailData: (taskId: string, source?: "rpc" | "mock") => Promise<unknown>;
      };

      await assert.rejects(() => service.loadTaskBucketPage("unfinished", { source: "rpc" }), /transport is not wired/i);
      await assert.rejects(() => service.loadTaskDetailData("task_dashboard_001", "rpc"), /transport is not wired/i);
      await assert.rejects(() => service.controlTaskByAction("task_dashboard_001", "pause", "rpc"), /transport is not wired/i);
    },
    {
      controlTask: () => Promise.reject(transportError),
      getTaskDetail: () => Promise.reject(transportError),
      listTasks: () => Promise.reject(transportError),
    },
  );
});

test("note rpc service keeps transport failures visible instead of switching to mock data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/notePage.service.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        convertNoteToTask: (itemId: string, source?: "rpc" | "mock") => Promise<unknown>;
        loadNoteBucket: (group: "upcoming" | "later" | "recurring_rule" | "closed", source?: "rpc" | "mock") => Promise<unknown>;
        updateNote: (itemId: string, action: "complete" | "cancel" | "move_upcoming" | "toggle_recurring" | "cancel_recurring" | "restore" | "delete", source?: "rpc" | "mock") => Promise<unknown>;
      };

      await assert.rejects(() => service.loadNoteBucket("upcoming", "rpc"), /transport is not wired/i);
      await assert.rejects(() => service.convertNoteToTask("todo_001", "rpc"), /transport is not wired/i);
      await assert.rejects(() => service.updateNote("todo_001", "complete", "rpc"), /transport is not wired/i);
    },
    {
      convertNotepadToTask: () => Promise.reject(transportError),
      listNotepad: () => Promise.reject(transportError),
      updateNotepad: () => Promise.reject(transportError),
    },
  );
});

test("TaskDetailPanel defers the entire fallback security summary until formal detail arrives", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /detailData\.source === "fallback" \|\| detailState !== "ready"/);
  assert.match(panelSource, /等待详情同步后展示风险、授权与恢复点/);
});

test("TaskDetailPanel renders runtime summary fields from the formal detail payload", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /Runtime Summary/);
  assert.match(panelSource, /循环停止原因与调试概览/);
  assert.match(panelSource, /runtimeSummary\.loop_stop_reason \?\? "当前还没有停止原因"/);
  assert.match(panelSource, /runtimeSummary\.latest_event_type \?\? "当前还没有 runtime event"/);
  assert.match(panelSource, /runtimeSummary\.events_count/);
  assert.match(panelSource, /runtimeSummary\.active_steering_count/);
});

test("TaskDetailPanel keeps evidence artifacts scoped to formal citation links", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /const evidenceArtifactRefs = new Set\(evidenceItems\.map\(\(citation\) => citation\.source_ref\)\)/);
  assert.match(panelSource, /const evidenceArtifacts = artifactItems\.filter\(\(artifact\) => evidenceArtifactRefs\.has\(artifact\.artifact_id\) \|\| evidenceArtifactRefs\.has\(artifact\.path\)\)/);
  assert.match(panelSource, /const outputArtifacts = artifactItems\.filter\(\(artifact\) => !evidenceArtifactRefs\.has\(artifact\.artifact_id\) && !evidenceArtifactRefs\.has\(artifact\.path\)\)/);
  assert.match(panelSource, /const formalEvidenceCount = new Set\(/);
  assert.match(panelSource, /return sourceRef\.length > 0 \? sourceRef : citation\.citation_id/);
  assert.doesNotMatch(panelSource, /artifactItems\.map\(\(artifact\) => \(/);
});

test("TaskDetailPanel renders a formal screen governance section only for screen tasks with synced detail", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /const isScreenTask = task\.source_type === "screen_capture" \|\| detail\.task\.intent\?\.name === "screen_analyze"/);
  assert.match(panelSource, /if \(!isScreenTask \|\| shouldDeferSecuritySummary\) \{/);
  assert.match(panelSource, /Screen Governance/);
  assert.match(panelSource, /屏幕授权、恢复与失败收口/);
  assert.match(panelSource, /该区域只消费正式 `approval_request`、`recovery_point` 与 `runtime_summary` 字段/);
  assert.match(panelSource, /runtimeSummary\.latest_failure_category/);
  assert.match(panelSource, /detail\.approval_request/);
  assert.match(panelSource, /detail\.security_summary\.latest_restore_point/);
  assert.match(panelSource, /formalEvidenceCount/);
  assert.doesNotMatch(panelSource, /evidenceItems\.length \+ evidenceArtifacts\.length/);
});

test("TaskDetailPanel keeps runtime sections visible for ended tasks and preserves steering draft until success", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");

  assert.match(panelSource, /if \(!feedback \|\| !\/已记录新的补充要求\/\.test\(feedback\)\)/);
  assert.doesNotMatch(panelSource, /handleSubmitSteering\(\)[\s\S]*setSteeringMessage\(""\)/);
  assert.match(panelSource, /\{renderRuntimeSummarySection\(\)\}/);
  assert.match(panelSource, /\{renderRuntimeEventsSection\(\)\}/);
  assert.match(taskPageSource, /invalidateSelectedTaskDetail\(selectedTaskId\)/);
});

test("TaskDetailPanel exposes formal runtime event filters and applies them explicitly", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /agent\.task\.events\.list/);
  assert.match(panelSource, /事件类型/);
  assert.match(panelSource, /Run ID/);
  assert.match(panelSource, /最近 24 小时/);
  assert.match(panelSource, /应用筛选/);
  assert.match(panelSource, /setEventFilterDraft\(DEFAULT_TASK_EVENT_FILTERS\)/);
  assert.match(panelSource, /typing does not trigger[\s\S]*RPC refetch per keystroke/);
});

test("task runtime event queries key and service include filter dimensions and time bounds", () => {
  const querySource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.query.ts"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const serviceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.service.ts"), "utf8");

  assert.match(querySource, /buildDashboardTaskEventQueryKey/);
  assert.match(taskPageSource, /buildDashboardTaskEventQueryKey\(dataMode, selectedTaskId \?\? "", taskEventFilters\)/);
  assert.match(serviceSource, /created_at_from/);
  assert.match(serviceSource, /created_at_to/);
  assert.match(serviceSource, /timeRange: "all"/);
});

test("dashboard home consumes task module runtime summaries for focus-task visibility", () => {
  const serviceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts"), "utf8");

  assert.match(serviceSource, /focus_runtime_summary/);
  assert.match(serviceSource, /focus_task_id/);
  assert.match(serviceSource, /最近运行事件/);
  assert.match(serviceSource, /待消费追加要求/);
  assert.match(serviceSource, /waiting_auth_tasks/);
  assert.match(serviceSource, /focusTaskId === expectedFocusTaskId/);
  assert.match(serviceSource, /runtimeSummary\.latest_event_type === "loop\.retrying"/);
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
