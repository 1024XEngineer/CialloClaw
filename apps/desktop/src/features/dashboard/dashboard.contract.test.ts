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
  AgentSettingsGetParams,
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

function loadDashboardTaskDetailNavigationSource() {
  return readFileSync(resolve(desktopRoot, "src/features/dashboard/shared/dashboardTaskDetailNavigation.ts"), "utf8");
}

function loadConversationSessionServiceModule() {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, "src/services/conversationSessionService.ts");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      getConversationSessionIdForTask: (taskId: string | null | undefined) => string | undefined;
      getCurrentConversationSessionId: () => string | undefined;
      rememberConversationSessionFromTask: (task: Task | null | undefined) => string | null;
    };
  });
}

function loadTaskPageQueryModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.query.js")) as {
      buildDashboardTaskArtifactQueryKey: (dataMode: "rpc" | "mock", taskId: string) => unknown;
      buildDashboardTaskBucketQueryKey: (dataMode: "rpc" | "mock", group: "unfinished" | "finished", limit: number) => unknown;
      buildDashboardTaskDetailQueryKey: (dataMode: "rpc" | "mock", taskId: string) => unknown;
      getDashboardTaskSecurityRefreshPlan: (dataMode: "rpc" | "mock") => unknown;
      resolveDashboardTaskSafetyOpenPlan: (detailState: "loading" | "error" | "ready") => unknown;
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

type DashboardContractDesktopLocalPathOverrides = {
  openDesktopLocalPath?: (path: string) => Promise<void>;
  revealDesktopLocalPath?: (path: string) => Promise<void>;
};

function loadNotePageServiceModule(desktopLocalPath?: DashboardContractDesktopLocalPathOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/notePage.service.js");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      isAllowedNoteOpenUrl: (url: string) => boolean;
      resolveNoteResourceOpenExecutionPlan: (resource: {
        id: string;
        label: string;
        openAction?: "task_detail" | "open_url" | "open_file" | "reveal_in_folder" | "copy_path" | null;
        path: string;
        taskId?: string | null;
        type: string;
        url?: string | null;
      }) => {
        mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
        taskId: string | null;
        path: string | null;
        url: string | null;
        feedback: string;
      };
      performNoteResourceOpenExecution: (plan: {
        mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
        feedback: string;
        path: string | null;
        taskId: string | null;
        url: string | null;
      }, options?: {
        onOpenTaskDetail?: (input: {
          plan: {
            mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
            feedback: string;
            path: string | null;
            taskId: string | null;
            url: string | null;
          };
          taskId: string;
        }) => Promise<string | void> | string | void;
      }) => Promise<string>;
    };
  }, undefined, desktopLocalPath);
}

function loadTaskOutputServiceModule(desktopLocalPath?: DashboardContractDesktopLocalPathOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskOutput.service.js");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      describeTaskOpenResultForCurrentTask: (plan: { mode: string; taskId: string | null }, currentTaskId: string | null) => string | null;
      isAllowedTaskOpenUrl: (url: string) => boolean;
      loadTaskArtifactPage: (taskId: string, source: "rpc" | "mock") => Promise<AgentTaskArtifactListResult>;
      openTaskArtifactForTask: (taskId: string, artifactId: string, source: "rpc" | "mock") => Promise<AgentTaskArtifactOpenResult>;
      openTaskDeliveryForTask: (taskId: string, artifactId: string | undefined, source: "rpc" | "mock") => Promise<AgentDeliveryOpenResult>;
      resolveTaskOpenExecutionPlan: (result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult) => {
        mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
        taskId: string | null;
        path: string | null;
        url: string | null;
        feedback: string;
      };
      performTaskOpenExecution: (plan: {
        mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
        taskId: string | null;
        path: string | null;
        url: string | null;
        feedback: string;
      }, options?: {
        onOpenTaskDetail?: (input: {
          plan: {
            mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
            taskId: string | null;
            path: string | null;
            url: string | null;
            feedback: string;
          };
          taskId: string;
        }) => Promise<string | void> | string | void;
      }) => Promise<string>;
    };
  }, undefined, desktopLocalPath);
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
          models: {
            provider: string;
            budget_auto_downgrade: boolean;
            provider_api_key_configured: boolean;
            base_url: string;
            model: string;
          };
          general: {
            voice_type: string;
            download: {
              ask_before_save_each_file: boolean;
              workspace_path: string;
            };
          };
          floating_ball: {
            auto_snap: boolean;
            idle_translucent: boolean;
            position_mode: string;
            size: string;
          };
          memory: {
            enabled: boolean;
            lifecycle: string;
            work_summary_interval: {
              unit: string;
              value: number;
            };
            profile_refresh_interval: {
              unit: string;
              value: number;
            };
          };
        };
      };
      saveSettings: (settings: unknown) => void;
    },
  );
}

function loadControlPanelServiceModule(rpcMethods?: DashboardContractRpcMethodOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, "src/services/controlPanelService.ts");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      loadControlPanelData: () => Promise<{
        source: "rpc";
        settings: {
          general: {
            voice_type: string;
            download: {
              ask_before_save_each_file: boolean;
              workspace_path: string;
            };
          };
          floating_ball: {
            auto_snap: boolean;
            idle_translucent: boolean;
            position_mode: string;
            size: string;
          };
          memory: {
            work_summary_interval: {
              unit: string;
              value: number;
            };
            profile_refresh_interval: {
              unit: string;
              value: number;
            };
          };
          models: {
            provider: string;
            provider_api_key_configured: boolean;
            budget_auto_downgrade: boolean;
            base_url: string;
            model: string;
          };
        };
        inspector: {
          task_sources: string[];
          inspection_interval: {
            unit: string;
            value: number;
          };
          inspect_on_file_change: boolean;
          inspect_on_startup: boolean;
          remind_before_deadline: boolean;
          remind_when_stale: boolean;
        };
        providerApiKeyInput: string;
        warnings?: string[];
      }>;
      saveControlPanelData: (
        data: unknown,
        options?: {
          saveInspector?: boolean;
          saveSettings?: boolean;
          validateModel?: boolean;
          timeoutMs?: number;
        },
      ) => Promise<{
        source: "rpc";
        applyMode: string;
        needRestart: boolean;
        savedInspector?: boolean;
        savedSettings?: boolean;
        updatedKeys: string[];
        warnings: string[];
        modelValidation?: {
          ok: boolean;
          status: string;
          message: string;
        } | null;
        effectiveSettings: {
          general: {
            voice_type: string;
            download: {
              ask_before_save_each_file: boolean;
              workspace_path: string;
            };
          };
          floating_ball: {
            auto_snap: boolean;
            idle_translucent: boolean;
            position_mode: string;
            size: string;
          };
          memory: {
            work_summary_interval: {
              unit: string;
              value: number;
            };
            profile_refresh_interval: {
              unit: string;
              value: number;
            };
          };
          models: {
            provider: string;
            provider_api_key_configured: boolean;
            budget_auto_downgrade: boolean;
            base_url: string;
            model: string;
          };
        };
      }>;
      validateControlPanelModel: (
        data: unknown,
        options?: {
          timeoutMs?: number;
        },
      ) => Promise<{
        ok: boolean;
        status: string;
        message: string;
        provider: string;
        canonical_provider: string;
        base_url: string;
        model: string;
        text_generation_ready: boolean;
        tool_calling_ready: boolean;
      }>;
    };
  }, rpcMethods);
}

function loadControlPanelAboutServiceModule() {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, "src/services/controlPanelAboutService.ts");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      getControlPanelAboutFeedbackChannels: () => Array<
        | {
            actionLabel: string;
            description: string;
            href: string;
            hrefLabel: string;
            id: string;
            kind: "link";
            title: string;
          }
        | {
            description: string;
            id: string;
            kind: "placeholder";
            note: string;
            placeholderLabel: string;
            title: string;
          }
      >;
      getControlPanelAboutFallbackSnapshot: () => {
        appName: string;
        appVersion: string;
      };
      copyControlPanelAboutValue: (value: string, successMessage: string) => Promise<string>;
      runControlPanelAboutAction: (action: "share") => Promise<string>;
    };
  });
}

function loadDashboardSettingsMutationModule(rpcMethods?: DashboardContractRpcMethodOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsMutation.js");
    const snapshotModulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsSnapshot.js");

    delete requireFn.cache[modulePath];
    delete requireFn.cache[snapshotModulePath];

    return requireFn(modulePath) as {
      updateDashboardSettings: (patch: Record<string, unknown>, source?: "rpc" | "mock") => Promise<{
        applyMode: string;
        needRestart: boolean;
        persisted: boolean;
        source: string;
        updatedKeys: string[];
        snapshot: {
          source: string;
          settings: {
            models: {
              credentials: {
                budget_auto_downgrade: boolean;
              };
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

function loadDashboardSettingsSnapshotModule(rpcMethods?: Pick<DashboardContractRpcMethodOverrides, "getSettingsDetailed">) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsSnapshot.js");

    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      loadDashboardSettingsSnapshot: (
        source?: "rpc" | "mock",
        scope?: AgentSettingsGetParams["scope"],
      ) => Promise<{
        source: string;
        settings: {
          general: {
            download: {
              ask_before_save_each_file: boolean;
            };
          };
          memory: {
            enabled: boolean;
            lifecycle: string;
          };
          models: {
            provider: string;
          };
        };
        rpcContext: {
          serverTime: string | null;
          warnings: string[];
        };
      }>;
    };
  }, rpcMethods);
}

function loadMirrorServiceModule() {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/memory/mirrorService.js");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      applyMirrorSettingsSnapshot: (
        current: {
          overview: {
            history_summary: string[];
          };
          insight: {
            badge: string;
          };
          latestRestorePoint: RecoveryPoint | null;
          rpcContext: {
            serverTime: string | null;
            warnings: string[];
          };
          settingsSnapshot: {
            source: string;
            settings: {
              memory: {
                enabled: boolean;
                lifecycle: string;
              };
              general: {
                download: {
                  ask_before_save_each_file: boolean;
                };
              };
            };
          };
          source: "rpc" | "mock";
          conversations: Array<{ id: string }>;
        },
        settingsSnapshot: {
          source: string;
          settings: {
            memory: {
              enabled: boolean;
              lifecycle: string;
            };
            general: {
              download: {
                ask_before_save_each_file: boolean;
              };
            };
          };
        },
      ) => {
        overview: {
          history_summary: string[];
        };
        insight: {
          badge: string;
        };
        latestRestorePoint: RecoveryPoint | null;
        rpcContext: {
          serverTime: string | null;
          warnings: string[];
        };
        settingsSnapshot: {
          source: string;
          settings: {
            memory: {
              enabled: boolean;
              lifecycle: string;
            };
            general: {
              download: {
                ask_before_save_each_file: boolean;
              };
            };
          };
        };
        source: "rpc" | "mock";
        conversations: Array<{ id: string }>;
      };
    };
  });
}

type DashboardContractRpcMethodOverrides = {
  applySecurityRestoreDetailed?: (params: unknown) => Promise<unknown>;
  controlTask?: (params: AgentTaskControlParams) => Promise<AgentTaskControlResult>;
  convertNotepadToTask?: (params: AgentNotepadConvertToTaskParams) => Promise<AgentNotepadConvertToTaskResult>;
  getMirrorOverviewDetailed?: (params: unknown) => Promise<unknown>;
  getSecuritySummary?: (params: unknown) => Promise<unknown>;
  getSecuritySummaryDetailed?: (params: unknown) => Promise<unknown>;
  getSettings?: (params: unknown) => Promise<unknown>;
  updateSettings?: (params: unknown) => Promise<unknown>;
  getSettingsDetailed?: (params: unknown) => Promise<unknown>;
  getTaskInspectorConfig?: (params: unknown) => Promise<unknown>;
  getTaskDetail?: (params: AgentTaskDetailGetParams) => Promise<AgentTaskDetailGetResult>;
  listSecurityAuditDetailed?: (params: unknown) => Promise<unknown>;
  listSecurityPendingDetailed?: (params: unknown) => Promise<unknown>;
  listSecurityRestorePointsDetailed?: (params: unknown) => Promise<unknown>;
  listNotepad?: (params: AgentNotepadListParams) => Promise<AgentNotepadListResult>;
  listTasks?: (params: AgentTaskListParams) => Promise<AgentTaskListResult>;
  respondSecurityDetailed?: (params: unknown) => Promise<unknown>;
  runTaskInspector?: (params: unknown) => Promise<unknown>;
  validateSettingsModel?: (params: unknown) => Promise<unknown>;
  updateTaskInspectorConfig?: (params: unknown) => Promise<unknown>;
  updateNotepad?: (params: AgentNotepadUpdateParams) => Promise<AgentNotepadUpdateResult>;
};

function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => Promise<T>,
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopLocalPath?: DashboardContractDesktopLocalPathOverrides,
): Promise<T>;
function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => T,
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopLocalPath?: DashboardContractDesktopLocalPathOverrides,
): T;
function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => T | Promise<T>,
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopLocalPath?: DashboardContractDesktopLocalPathOverrides,
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
        getSecuritySummary:
          rpcMethods?.getSecuritySummary ??
          (() => Promise.reject(new Error("getSecuritySummary should not run in dashboard contract tests"))),
        getMirrorOverviewDetailed:
          rpcMethods?.getMirrorOverviewDetailed ??
          (() => Promise.reject(new Error("getMirrorOverviewDetailed should not run in dashboard contract tests"))),
        getSecuritySummaryDetailed:
          rpcMethods?.getSecuritySummaryDetailed ??
          (() => Promise.reject(new Error("getSecuritySummaryDetailed should not run in dashboard contract tests"))),
        getSettings:
          rpcMethods?.getSettings ??
          (() => Promise.reject(new Error("getSettings should not run in dashboard contract tests"))),
        listNotepad:
          rpcMethods?.listNotepad ??
          (() => {
            throw new Error("listNotepad should not run in dashboard contract tests");
          }),
        listSecurityAuditDetailed:
          rpcMethods?.listSecurityAuditDetailed ??
          (() => Promise.reject(new Error("listSecurityAuditDetailed should not run in dashboard contract tests"))),
        listSecurityPendingDetailed:
          rpcMethods?.listSecurityPendingDetailed ??
          (() => Promise.reject(new Error("listSecurityPendingDetailed should not run in dashboard contract tests"))),
        listSecurityRestorePointsDetailed:
          rpcMethods?.listSecurityRestorePointsDetailed ??
          (() => Promise.reject(new Error("listSecurityRestorePointsDetailed should not run in dashboard contract tests"))),
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
        respondSecurityDetailed:
          rpcMethods?.respondSecurityDetailed ??
          (() => Promise.reject(new Error("respondSecurityDetailed should not run in dashboard contract tests"))),
        applySecurityRestoreDetailed:
          rpcMethods?.applySecurityRestoreDetailed ??
          (() => Promise.reject(new Error("applySecurityRestoreDetailed should not run in dashboard contract tests"))),
        updateNotepad:
          rpcMethods?.updateNotepad ??
          (() => {
            throw new Error("updateNotepad should not run in dashboard contract tests");
          }),
        getTaskInspectorConfig:
          rpcMethods?.getTaskInspectorConfig ??
          (() => Promise.reject(new Error("getTaskInspectorConfig should not run in dashboard contract tests"))),
        runTaskInspector:
          rpcMethods?.runTaskInspector ??
          (() => Promise.reject(new Error("runTaskInspector should not run in dashboard contract tests"))),
        updateTaskInspectorConfig:
          rpcMethods?.updateTaskInspectorConfig ??
          (() => Promise.reject(new Error("updateTaskInspectorConfig should not run in dashboard contract tests"))),
        getSettingsDetailed: rpcMethods?.getSettingsDetailed ?? (() => Promise.reject(new Error("getSettingsDetailed should not run in dashboard contract tests"))),
        updateSettings: rpcMethods?.updateSettings ?? (() => Promise.reject(new Error("updateSettings should not run in dashboard contract tests"))),
        validateSettingsModel:
          rpcMethods?.validateSettingsModel ??
          (() => Promise.resolve({
            ok: true,
            status: "valid",
            message: "当前模型配置校验通过，可执行文本生成与工具调用。",
            provider: "openai",
            canonical_provider: "openai_responses",
            base_url: "https://api.openai.com/v1",
            model: "gpt-4.1-mini",
            text_generation_ready: true,
            tool_calling_ready: true,
          })),
      };
    }

    if (request === "@/platform/desktopLocalPath") {
      return {
        openDesktopLocalPath:
          desktopLocalPath?.openDesktopLocalPath ??
          (() => Promise.resolve()),
        revealDesktopLocalPath:
          desktopLocalPath?.revealDesktopLocalPath ??
          (() => Promise.resolve()),
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
  const { session_id = null, ...rest } = overrides;

  return {
    task_id: "task_dashboard_001",
    session_id,
    title: "Review dashboard safety state",
    status: "waiting_auth",
    source_type: "hover_input",
    updated_at: "2026-04-13T09:05:00.000Z",
    started_at: "2026-04-13T09:00:30.000Z",
    finished_at: null,
    intent: null,
    current_step: "Awaiting approval",
    risk_level: "yellow",
    ...rest,
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
    audit_record: null,
    artifacts: [],
    authorization_record: null,
    citations: [],
    delivery_result: null,
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

test("task page stays RPC-only instead of exposing a page-level mock toggle", () => {
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");

  assert.match(taskPageSource, /const dataMode: TaskPageDataMode = "rpc";/);
  assert.doesNotMatch(taskPageSource, /DashboardMockToggle/);
  assert.doesNotMatch(taskPageSource, /loadDashboardDataMode\("tasks"\)/);
  assert.doesNotMatch(taskPageSource, /saveDashboardDataMode\("tasks"\)/);
  assert.doesNotMatch(taskPageSource, /setDataMode\(/);
});

test("note page stays RPC-only instead of exposing a page-level mock toggle", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /const dataMode: NotePageDataMode = "rpc";/);
  assert.doesNotMatch(notePageSource, /DashboardMockToggle/);
  assert.doesNotMatch(notePageSource, /loadDashboardDataMode\("notes"\)/);
  assert.doesNotMatch(notePageSource, /saveDashboardDataMode\("notes"\)/);
  assert.doesNotMatch(notePageSource, /setDataMode\(/);
});

test("mirror page stays RPC-only instead of exposing a page-level mock toggle", () => {
  const mirrorAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorApp.tsx"), "utf8");

  assert.match(mirrorAppSource, /const dataMode: MirrorOverviewSource = "rpc";/);
  assert.doesNotMatch(mirrorAppSource, /DashboardMockToggle/);
  assert.doesNotMatch(mirrorAppSource, /loadDashboardDataMode\("memory"\)/);
  assert.doesNotMatch(mirrorAppSource, /saveDashboardDataMode\("memory"\)/);
  assert.doesNotMatch(mirrorAppSource, /setDataMode\(/);
});

test("safety page stays RPC-only instead of exposing a page-level mock toggle", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");

  assert.match(securityAppSource, /const dataMode = "rpc" as const;/);
  assert.doesNotMatch(securityAppSource, /DashboardMockToggle/);
  assert.doesNotMatch(securityAppSource, /loadDashboardDataMode\("safety"\)/);
  assert.doesNotMatch(securityAppSource, /saveDashboardDataMode\("safety"\)/);
  assert.doesNotMatch(securityAppSource, /setDataMode\(/);
});

test("dashboard home entrance labels stay hidden until hover or focus", () => {
  const dashboardHomeStyleSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.css"), "utf8");
  const entranceOrbSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/components/DashboardEntranceOrb.tsx"), "utf8");

  assert.match(entranceOrbSource, /data-hovered=\{isHovered \? "true" : "false"\}/);
  assert.match(dashboardHomeStyleSource, /\.dashboard-orbit-entrance__label \{[\s\S]*opacity: 0;/);
  assert.match(dashboardHomeStyleSource, /\.dashboard-orbit-entrance:hover \.dashboard-orbit-entrance__label,/);
  assert.match(dashboardHomeStyleSource, /\.dashboard-orbit-entrance:focus-visible \.dashboard-orbit-entrance__label,/);
  assert.match(dashboardHomeStyleSource, /\.dashboard-orbit-entrance\[data-hovered="true"\] \.dashboard-orbit-entrance__label \{/);
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

test("security board cards keep CJK headlines and status badges readable", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");
  const securityBoardSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/securityBoard.css"), "utf8");

  assert.match(securityAppSource, /className="security-page__status-strip"/);
  assert.match(securityAppSource, /className="security-page__status-badge"/);
  assert.match(securityAppSource, /className="security-page__card-badge"/);
  assert.match(securityBoardSource, /--security-font-display: "Noto Serif SC", "Source Han Serif SC", "Songti SC", "STSong", "SimSun"/);
  assert.match(securityBoardSource, /\.security-page__card-line \{[\s\S]*line-height: 1\.18;/);
  assert.match(securityBoardSource, /\.security-page__card-line \{[\s\S]*overflow-wrap: anywhere;/);
  assert.match(securityBoardSource, /\.security-page__status-badge,[\s\S]*white-space: normal;/);
});

test("security board cards reserve a larger readable footprint", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");

  assert.match(securityAppSource, /const DEFAULT_CARD_SIZE: CardSize = \{ width: 316, height: 236 \};/);
  assert.match(securityAppSource, /width: clampValue\(width, 228, DEFAULT_CARD_SIZE\.width\)/);
  assert.match(securityAppSource, /height: clampValue\(height, 172, DEFAULT_CARD_SIZE\.height\)/);
});

test("security board dragging keeps the path free until drop settles collisions", () => {
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");

  assert.match(securityAppSource, /const getClampedCardPosition = useCallback/);
  assert.match(securityAppSource, /Keep the drag path free while the card is moving/);
  assert.match(securityAppSource, /handleCardPointerMove[\s\S]*getClampedCardPosition\(/);
  assert.match(securityAppSource, /handleCardPointerUp[\s\S]*getSettledCardPosition\(key, currentPositions\[key\] \?\? FALLBACK_POSITION, currentPositions\)/);
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

test("mirror cards use CJK-friendly display typography without clipped line clamps", () => {
  const mirrorStyleSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/mirror.css"), "utf8");

  assert.match(mirrorStyleSource, /--mirror-font-display: "Noto Serif SC", "Source Han Serif SC", "Songti SC", "STSong", "SimSun"/);
  assert.match(mirrorStyleSource, /\.mirror-page__card-line \{[\s\S]*line-height: 1\.28;/);
  assert.match(mirrorStyleSource, /\.mirror-page__card-line \{[\s\S]*padding-bottom: 0\.12em;/);
  assert.match(mirrorStyleSource, /\.mirror-page__card-line--memory \{[\s\S]*word-break: break-word;/);
  assert.match(mirrorStyleSource, /\.mirror-page__card-detail \{[\s\S]*overflow-wrap: anywhere;/);
});

test("mirror floating cards reserve a slightly larger readable footprint", () => {
  const mirrorAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorApp.tsx"), "utf8");

  assert.match(mirrorAppSource, /const MIN_COMPACT_CARD_WIDTH = 132;/);
  assert.match(mirrorAppSource, /const MIN_COMPACT_CARD_HEIGHT = 132;/);
  assert.match(mirrorAppSource, /const DEFAULT_CARD_SIZE: ModuleSize = \{ width: 376, height: 252 \};/);
  assert.match(mirrorAppSource, /width: clampValue\(width, 1, DEFAULT_CARD_SIZE\.width\)/);
  assert.match(mirrorAppSource, /height: clampValue\(height, 1, DEFAULT_CARD_SIZE\.height\)/);
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

test("task page keeps waiting-auth anchors and routes follow-up steering through the detail panel", () => {
  const { getTaskPrimaryActions } = loadTaskPageMapperModule();
  const waitingAuthTask = createTask({ status: "waiting_auth" });
  const waitingInputTask = createTask({ status: "waiting_input" });
  const mapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.mapper.ts"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const taskDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.equal(getTaskPrimaryActions(waitingAuthTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: waitingAuthTask })).at(-1)?.label, "安全详情");
  assert.deepEqual(
    getTaskPrimaryActions(waitingInputTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: waitingInputTask })).map((action) => action.action),
    ["cancel", "open-safety"],
  );
  assert.doesNotMatch(mapperSource, /当前任务还在等待补充输入，如需修改或补充，请到悬浮球继续处理。/);
  assert.match(taskPageSource, /onSteerTask=\{handleSteerTask\}/);
  assert.match(taskDetailPanelSource, /placeholder=\{canSteerTask \? "例如：保留现有结果，再额外补一份简短结论。" : "当前任务已结束，不能继续补充要求。"\}/);
});

test("settings service normalizes legacy stored snapshots before returning and saving", () => {
  const { loadSettings, saveSettings } = loadSettingsServiceModule();
  const originalWindow = globalThis.window;
  const legacyModelsAlias = "data" + "_log";
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
          models: {
            provider: "openai",
            budget_auto_downgrade: true,
            base_url: "https://api.openai.com/v1",
            model: "gpt-4.1-mini",
          },
        },
      }),
    );

    const loaded = loadSettings();
    assert.equal(loaded.settings.models.provider_api_key_configured, false);

    saveSettings(loaded as never);

    const persisted = JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}");
    assert.equal(persisted.settings.models.provider_api_key_configured, false);
    assert.equal(Reflect.has(persisted.settings, legacyModelsAlias), false);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service ignores stale legacy settings aliases when models are already stored", () => {
  const { loadSettings, saveSettings } = loadSettingsServiceModule();
  const originalWindow = globalThis.window;
  const legacyModelsAlias = "data" + "_log";
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
          [legacyModelsAlias]: {
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
    assert.equal(Reflect.has(loaded.settings as object, legacyModelsAlias), false);
    assert.equal(loaded.settings.models.provider, "openai");
    assert.equal(loaded.settings.models.budget_auto_downgrade, true);
    assert.equal(loaded.settings.models.provider_api_key_configured, false);
    assert.equal(loaded.settings.models.base_url, "https://local-router.invalid/v1");
    assert.equal(loaded.settings.models.model, "gpt-local");

    saveSettings(loaded as never);

    const persisted = JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}");
    assert.equal(Reflect.has(persisted.settings, legacyModelsAlias), false);
    assert.equal(persisted.settings.models.provider, "openai");
    assert.equal(persisted.settings.models.provider_api_key_configured, false);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("control panel about service exposes fallback metadata and feedback channel config", () => {
  const { getControlPanelAboutFallbackSnapshot, getControlPanelAboutFeedbackChannels } = loadControlPanelAboutServiceModule();
  const fallback = getControlPanelAboutFallbackSnapshot();
  const feedbackChannels = getControlPanelAboutFeedbackChannels();

  assert.deepEqual(fallback, {
    appName: "CialloClaw",
    appVersion: "0.1.0",
  });
  assert.deepEqual(feedbackChannels, [
    {
      actionLabel: "复制链接",
      description: "公开问题反馈、功能建议与版本回归记录。",
      href: "https://github.com/1024XEngineer/CialloClaw/issues",
      hrefLabel: "github.com/1024XEngineer/CialloClaw/issues",
      id: "github_issues",
      kind: "link",
      title: "GitHub Issues",
    },
    {
      description: "预留微信群、QQ群或 Discord 等社群二维码图片。",
      id: "community_qr",
      kind: "placeholder",
      note: "后续放入二维码图片后，会在这里直接显示预览。",
      placeholderLabel: "待放置二维码图片",
      title: "社群二维码",
    },
    {
      description: "预留邮箱、工单表单或其它定向联系入口。",
      id: "contact_form",
      kind: "placeholder",
      note: "支持后续替换成链接、表单地址或其它说明文本。",
      placeholderLabel: "待放置链接或表单",
      title: "邮箱 / 表单",
    },
  ]);
});

test("control panel about helpers copy feedback and share links", async () => {
  const { copyControlPanelAboutValue, runControlPanelAboutAction } = loadControlPanelAboutServiceModule();
  const originalWindowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  const originalNavigatorDescriptor = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  const copiedValues: string[] = [];

  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: {
      clipboard: {
        writeText: async (value: string) => {
          copiedValues.push(value);
        },
      },
    },
  });

  try {
    const feedbackCopy = await copyControlPanelAboutValue("https://github.com/1024XEngineer/CialloClaw/issues", "已复制反馈渠道链接。");
    const shareFeedback = await runControlPanelAboutAction("share");

    assert.equal(feedbackCopy, "已复制反馈渠道链接。");
    assert.equal(shareFeedback, "已复制分享链接。");
    assert.deepEqual(copiedValues, [
      "https://github.com/1024XEngineer/CialloClaw/issues",
      "https://github.com/1024XEngineer/CialloClaw",
    ]);
  } finally {
    if (originalNavigatorDescriptor) {
      Object.defineProperty(globalThis, "navigator", originalNavigatorDescriptor);
    } else {
      Reflect.deleteProperty(globalThis, "navigator");
    }

    if (originalWindowDescriptor) {
      Object.defineProperty(globalThis, "window", originalWindowDescriptor);
    } else {
      Reflect.deleteProperty(globalThis, "window");
    }
  }
});

test("control panel app wires the about navigation without update-only fields", () => {
  const controlPanelAppSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");
  const removedRuntimeCopyPattern = /Tauri\s+Runtime/;

  assert.match(controlPanelAppSource, /type ControlPanelSectionId = .*"about"/);
  assert.match(controlPanelAppSource, /navLabel: "关于"/);
  assert.match(controlPanelAppSource, /case "about":/);
  assert.match(controlPanelAppSource, /title="帮助与反馈"/);
  assert.match(controlPanelAppSource, /title="版本信息"/);
  assert.match(controlPanelAppSource, /应用内新手引导/);
  assert.match(controlPanelAppSource, /反馈渠道/);
  assert.match(controlPanelAppSource, /CONTROL_PANEL_ABOUT_FEEDBACK_CHANNELS/);
  assert.match(controlPanelAppSource, /复制链接/);
  assert.doesNotMatch(controlPanelAppSource, /快捷操作/);
  assert.doesNotMatch(controlPanelAppSource, /打开帮助/);
  assert.doesNotMatch(controlPanelAppSource, /提交反馈/);
  assert.doesNotMatch(controlPanelAppSource, /打开链接/);
  assert.doesNotMatch(controlPanelAppSource, /GitHub 项目主页/);
  assert.doesNotMatch(controlPanelAppSource, /当前反馈/);
  assert.doesNotMatch(controlPanelAppSource, /更多渠道/);
  assert.doesNotMatch(controlPanelAppSource, /应用标识/);
  assert.doesNotMatch(controlPanelAppSource, /元信息来源/);
  assert.doesNotMatch(controlPanelAppSource, /检查更新/);
  assert.doesNotMatch(controlPanelAppSource, removedRuntimeCopyPattern);
});

test("control panel app surfaces about action feedback in local UI state", () => {
  const controlPanelAppSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");

  assert.match(controlPanelAppSource, /const \[aboutActionFeedback, setAboutActionFeedback\] = useState<string \| null>\(null\);/);
  assert.match(controlPanelAppSource, /const feedback = await runControlPanelAboutAction\(action\);[\s\S]*setAboutActionFeedback\(feedback\);/);
  assert.match(controlPanelAppSource, /const feedback = await copyControlPanelAboutValue\(url, "已复制反馈渠道链接。"\);[\s\S]*setAboutActionFeedback\(feedback\);/);
  assert.match(controlPanelAppSource, /aboutActionFeedback \? \([\s\S]*aria-live="polite"[\s\S]*\{aboutActionFeedback\}/);
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
        models: {
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
    assert.deepEqual(result.updatedKeys.sort(), ["general", "memory", "models"]);
    assert.equal(result.snapshot.settings.memory.enabled, false);
    assert.equal(result.snapshot.settings.memory.lifecycle, "session");
    assert.equal(result.snapshot.settings.general.download.ask_before_save_each_file, false);
    assert.equal(result.snapshot.settings.models.credentials.budget_auto_downgrade, false);

    const persisted = loadSettings();

    assert.equal(persisted.settings.memory.enabled, false);
    assert.equal(persisted.settings.memory.lifecycle, "session");
    assert.equal(persisted.settings.general.download.ask_before_save_each_file, false);
    assert.equal(persisted.settings.models.budget_auto_downgrade, false);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("dashboard settings snapshot merges scoped memory payloads onto the local baseline", async () => {
  const requestedScopes: string[] = [];
  const { loadDashboardSettingsSnapshot } = loadDashboardSettingsSnapshotModule({
    getSettingsDetailed: async (params) => {
      const request = params as {
        request_meta?: {
          trace_id?: string;
          client_time?: string;
        };
        scope?: string;
      };
      requestedScopes.push(request.scope ?? "missing");
      assert.match(request.request_meta?.trace_id ?? "", /^trace_dashboard_settings_/);
      assert.match(request.request_meta?.client_time ?? "", /^\d{4}-\d{2}-\d{2}T/);

      return {
        data: {
          settings: {
            memory: {
              enabled: false,
              lifecycle: "session",
              work_summary_interval: {
                unit: "week",
                value: 1,
              },
              profile_refresh_interval: {
                unit: "month",
                value: 1,
              },
            },
          },
        },
        meta: {
          server_time: "2026-04-24T09:30:00Z",
        },
        warnings: [],
      };
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
    const snapshot = await loadDashboardSettingsSnapshot("rpc", "memory");

    assert.deepEqual(requestedScopes, ["memory"]);
    assert.equal(snapshot.source, "rpc");
    assert.equal(snapshot.settings.memory.enabled, false);
    assert.equal(snapshot.settings.memory.lifecycle, "session");
    assert.equal(snapshot.settings.general.download.ask_before_save_each_file, true);
    assert.equal(snapshot.settings.models.provider, "openai");
    assert.equal(snapshot.rpcContext.serverTime, "2026-04-24T09:30:00Z");
    assert.deepEqual(snapshot.rpcContext.warnings, []);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("dashboard settings mutation reloads only the touched memory scope after rpc writes", async () => {
  const requestedScopes: string[] = [];
  const { updateDashboardSettings } = loadDashboardSettingsMutationModule({
    updateSettings: async () => ({
      apply_mode: "immediate",
      need_restart: false,
      updated_keys: ["memory.enabled", "memory.lifecycle"],
      effective_settings: {
        memory: {
          enabled: false,
          lifecycle: "session",
        },
      },
    }),
    getSettingsDetailed: async (params) => {
      requestedScopes.push((params as { scope?: string }).scope ?? "missing");

      return {
        data: {
          settings: {
            memory: {
              enabled: false,
              lifecycle: "session",
              work_summary_interval: {
                unit: "week",
                value: 1,
              },
              profile_refresh_interval: {
                unit: "month",
                value: 1,
              },
            },
          },
        },
        meta: {
          server_time: "2026-04-24T09:35:00Z",
        },
        warnings: [],
      };
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
    const result = await updateDashboardSettings({
      memory: {
        enabled: false,
        lifecycle: "session",
      },
    });

    assert.deepEqual(requestedScopes, ["memory"]);
    assert.equal(result.source, "rpc");
    assert.equal(result.snapshot.settings.memory.enabled, false);
    assert.equal(result.snapshot.settings.general.download.ask_before_save_each_file, true);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("control panel saves full floating-ball ownership through the real rpc settings flow", async () => {
  const { loadSettings } = loadSettingsServiceModule();
  const strongholdStatus = {
    backend: "stronghold",
    available: true,
    fallback: false,
    initialized: true,
    formal_store: true,
  };
  let updateSettingsRequest: Record<string, unknown> | null = null;
  let inspectorUpdateCount = 0;
  let settingsReadCount = 0;
  let inspectorReadCount = 0;
  let remoteSettings = {
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
    models: {
      provider: "openai",
      credentials: {
        budget_auto_downgrade: true,
        provider_api_key_configured: false,
        base_url: "https://api.openai.com/v1",
        model: "gpt-4.1-mini",
        stronghold: strongholdStatus,
      },
    },
  };
  const inspectorConfig = {
    task_sources: ["D:/workspace/todos"],
    inspection_interval: {
      unit: "minute",
      value: 15,
    },
    inspect_on_file_change: true,
    inspect_on_startup: true,
    remind_before_deadline: true,
    remind_when_stale: false,
  };
  const { loadControlPanelData, saveControlPanelData } = loadControlPanelServiceModule({
    getSecuritySummary: async () => ({
      summary: {
        security_status: "normal",
        pending_authorizations: 0,
        latest_restore_point: null,
        token_cost_summary: {
          current_task_tokens: 0,
          current_task_cost: 0,
          today_tokens: 0,
          today_cost: 0,
          single_task_limit: 50000,
          daily_limit: 300000,
          budget_auto_downgrade: true,
        },
      },
    }),
    getSettings: async (params) => {
      const request = params as {
        request_meta?: {
          trace_id?: string;
        };
        scope?: string;
      };

      settingsReadCount += 1;
      assert.equal(request.scope, "all");
      assert.match(request.request_meta?.trace_id ?? "", /^trace_control_panel_/);

      return {
        settings: remoteSettings,
      };
    },
    getTaskInspectorConfig: async () => {
      inspectorReadCount += 1;
      return inspectorConfig;
    },
    updateSettings: async (params) => {
      const request = params as {
        request_meta?: {
          trace_id?: string;
        };
        general: {
          voice_type: string;
          download: {
            ask_before_save_each_file: boolean;
            workspace_path: string;
          };
        };
        floating_ball: {
          auto_snap: boolean;
          idle_translucent: boolean;
          position_mode: string;
          size: string;
        };
        memory: {
          work_summary_interval: {
            unit: string;
            value: number;
          };
          profile_refresh_interval: {
            unit: string;
            value: number;
          };
        };
      };

      updateSettingsRequest = request as unknown as Record<string, unknown>;

      assert.match(request.request_meta?.trace_id ?? "", /^trace_control_panel_/);
      assert.equal(request.general.voice_type, "voice_nebula");
      assert.equal(request.general.download.ask_before_save_each_file, false);
      assert.deepEqual(request.floating_ball, {
        auto_snap: false,
        idle_translucent: false,
        position_mode: "fixed",
        size: "large",
      });
      assert.deepEqual(request.memory.work_summary_interval, {
        unit: "hour",
        value: 12,
      });
      assert.deepEqual(request.memory.profile_refresh_interval, {
        unit: "day",
        value: 5,
      });

      remoteSettings = {
        ...remoteSettings,
        general: {
          ...remoteSettings.general,
          ...request.general,
          download: {
            ...remoteSettings.general.download,
            ...request.general.download,
          },
        },
        floating_ball: {
          ...remoteSettings.floating_ball,
          ...request.floating_ball,
        },
        memory: {
          ...remoteSettings.memory,
          ...request.memory,
          work_summary_interval: {
            ...remoteSettings.memory.work_summary_interval,
            ...request.memory.work_summary_interval,
          },
          profile_refresh_interval: {
            ...remoteSettings.memory.profile_refresh_interval,
            ...request.memory.profile_refresh_interval,
          },
        },
      };

      return {
        apply_mode: "immediate",
        need_restart: false,
        updated_keys: [
          "general.voice_type",
          "general.download.ask_before_save_each_file",
          "floating_ball.auto_snap",
          "floating_ball.idle_translucent",
          "floating_ball.position_mode",
          "floating_ball.size",
          "memory.work_summary_interval",
          "memory.profile_refresh_interval",
        ],
        effective_settings: {
          general: {
            voice_type: request.general.voice_type,
            download: {
              ask_before_save_each_file: request.general.download.ask_before_save_each_file,
              workspace_path: request.general.download.workspace_path,
            },
          },
          floating_ball: request.floating_ball,
          memory: request.memory,
          models: {
            provider: "openai",
            budget_auto_downgrade: true,
            provider_api_key_configured: false,
            base_url: "https://api.openai.com/v1",
            model: "gpt-4.1-mini",
            stronghold: strongholdStatus,
          },
        },
      };
    },
    updateTaskInspectorConfig: async () => {
      inspectorUpdateCount += 1;
      return {
        effective_config: inspectorConfig,
      };
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
    const initialData = await loadControlPanelData();
    const result = await saveControlPanelData(
      {
        ...initialData,
        settings: {
          ...initialData.settings,
          general: {
            ...initialData.settings.general,
            voice_type: "voice_nebula",
            download: {
              ...initialData.settings.general.download,
              ask_before_save_each_file: false,
            },
          },
          floating_ball: {
            ...initialData.settings.floating_ball,
            auto_snap: false,
            idle_translucent: false,
            position_mode: "fixed",
            size: "large",
          },
          memory: {
            ...initialData.settings.memory,
            work_summary_interval: {
              unit: "hour",
              value: 12,
            },
            profile_refresh_interval: {
              unit: "day",
              value: 5,
            },
          },
        },
      },
      {
        saveInspector: false,
        saveSettings: true,
      },
    );

    assert.ok(updateSettingsRequest);
    assert.equal(inspectorUpdateCount, 0);
    assert.equal(settingsReadCount, 1);
    assert.equal(inspectorReadCount, 1);
    assert.equal(result.source, "rpc");
    assert.equal(result.needRestart, false);
    assert.equal(result.effectiveSettings.general.voice_type, "voice_nebula");
    assert.equal(result.effectiveSettings.general.download.ask_before_save_each_file, false);
    assert.equal(result.effectiveSettings.floating_ball.auto_snap, false);
    assert.equal(result.effectiveSettings.floating_ball.idle_translucent, false);
    assert.equal(result.effectiveSettings.floating_ball.position_mode, "fixed");
    assert.equal(result.effectiveSettings.floating_ball.size, "large");
    assert.equal(result.effectiveSettings.memory.work_summary_interval.value, 12);
    assert.equal(result.effectiveSettings.memory.work_summary_interval.unit, "hour");
    assert.equal(result.effectiveSettings.memory.profile_refresh_interval.value, 5);
    assert.equal(result.effectiveSettings.memory.profile_refresh_interval.unit, "day");

    const persisted = loadSettings();
    assert.equal(persisted.settings.general.voice_type, "voice_nebula");
    assert.equal(persisted.settings.general.download.ask_before_save_each_file, false);
    assert.equal(persisted.settings.floating_ball.auto_snap, false);
    assert.equal(persisted.settings.floating_ball.idle_translucent, false);
    assert.equal(persisted.settings.floating_ball.position_mode, "fixed");
    assert.equal(persisted.settings.floating_ball.size, "large");
    assert.equal(persisted.settings.memory.work_summary_interval.value, 12);
    assert.equal(persisted.settings.memory.work_summary_interval.unit, "hour");
    assert.equal(persisted.settings.memory.profile_refresh_interval.value, 5);
    assert.equal(persisted.settings.memory.profile_refresh_interval.unit, "day");

    const reloaded = await loadControlPanelData();
    assert.equal(settingsReadCount, 2);
    assert.equal(inspectorReadCount, 2);
    assert.equal(reloaded.source, "rpc");
    assert.equal(reloaded.settings.general.voice_type, "voice_nebula");
    assert.equal(reloaded.settings.general.download.ask_before_save_each_file, false);
    assert.equal(reloaded.settings.floating_ball.auto_snap, false);
    assert.equal(reloaded.settings.floating_ball.idle_translucent, false);
    assert.equal(reloaded.settings.floating_ball.position_mode, "fixed");
    assert.equal(reloaded.settings.floating_ball.size, "large");
    assert.equal(reloaded.settings.memory.work_summary_interval.value, 12);
    assert.equal(reloaded.settings.memory.work_summary_interval.unit, "hour");
    assert.equal(reloaded.settings.memory.profile_refresh_interval.value, 5);
    assert.equal(reloaded.settings.memory.profile_refresh_interval.unit, "day");
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("control-panel save keeps arbitrary provider aliases on the supported OpenAI-compatible route", async () => {
  const strongholdStatus = {
    backend: "stronghold",
    available: true,
    fallback: false,
    initialized: true,
    formal_store: true,
  };
  let remoteSettings = {
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
    models: {
      provider: "openai",
      credentials: {
        budget_auto_downgrade: true,
        provider_api_key_configured: false,
        base_url: "https://api.openai.com/v1",
        model: "gpt-4.1-mini",
        stronghold: strongholdStatus,
      },
    },
  };
  const inspectorConfig = {
    task_sources: ["D:/workspace/todos"],
    inspection_interval: {
      unit: "minute",
      value: 15,
    },
    inspect_on_file_change: true,
    inspect_on_startup: true,
    remind_before_deadline: true,
    remind_when_stale: false,
  };
  const { loadControlPanelData, saveControlPanelData } = loadControlPanelServiceModule({
    getSecuritySummary: async () => ({
      summary: {
        security_status: "normal",
        pending_authorizations: 0,
        latest_restore_point: null,
        token_cost_summary: {
          current_task_tokens: 0,
          current_task_cost: 0,
          today_tokens: 0,
          today_cost: 0,
          single_task_limit: 50000,
          daily_limit: 300000,
          budget_auto_downgrade: true,
        },
      },
    }),
    getSettings: async () => ({
      settings: remoteSettings,
    }),
    getTaskInspectorConfig: async () => inspectorConfig,
    updateSettings: async (params) => {
      const request = params as {
        models: {
          provider: string;
          budget_auto_downgrade: boolean;
          base_url: string;
          model: string;
          api_key?: string;
        };
      };

      assert.equal(request.models.provider, "anthropic");
      assert.equal(request.models.api_key, "saved-secret-key");

      remoteSettings = {
        ...remoteSettings,
        models: {
          provider: request.models.provider,
          credentials: {
            ...remoteSettings.models.credentials,
            budget_auto_downgrade: request.models.budget_auto_downgrade,
            provider_api_key_configured: true,
            base_url: request.models.base_url,
            model: request.models.model,
          },
        },
      };

      return {
        apply_mode: "next_task_effective",
        need_restart: false,
        updated_keys: ["models.provider", "models.api_key"],
        effective_settings: {
          models: {
            provider: request.models.provider,
            budget_auto_downgrade: request.models.budget_auto_downgrade,
            provider_api_key_configured: true,
            base_url: request.models.base_url,
            model: request.models.model,
            stronghold: strongholdStatus,
          },
        },
      };
    },
    updateTaskInspectorConfig: async () => ({
      effective_config: inspectorConfig,
    }),
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
    const initialData = await loadControlPanelData();
    const result = await saveControlPanelData(
      {
        ...initialData,
        providerApiKeyInput: "saved-secret-key",
        settings: {
          ...initialData.settings,
          models: {
            ...initialData.settings.models,
            provider: "anthropic",
            base_url: "https://api.qnaigc.com/v1",
            model: "claude-3-7-sonnet",
          },
        },
      },
      {
        saveInspector: false,
        saveSettings: true,
      },
    );

    assert.deepEqual(result.warnings, []);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("control-panel save blocks invalid model routes before persisting settings", async () => {
  const strongholdStatus = {
    backend: "stronghold",
    available: true,
    fallback: false,
    initialized: true,
    formal_store: true,
  };
  const remoteSettings = {
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
      work_summary_interval: { unit: "day", value: 7 },
      profile_refresh_interval: { unit: "week", value: 2 },
    },
    models: {
      provider: "openai",
      credentials: {
        budget_auto_downgrade: true,
        provider_api_key_configured: false,
        base_url: "https://api.openai.com/v1",
        model: "gpt-4.1-mini",
        stronghold: strongholdStatus,
      },
    },
  };
  const inspectorConfig = {
    task_sources: ["D:/workspace/todos"],
    inspection_interval: { unit: "minute", value: 15 },
    inspect_on_file_change: true,
    inspect_on_startup: true,
    remind_before_deadline: true,
    remind_when_stale: false,
  };
  let updateSettingsCalled = false;
  const { loadControlPanelData, saveControlPanelData, validateControlPanelModel } = loadControlPanelServiceModule({
    getSecuritySummary: async () => ({
      summary: {
        security_status: "normal",
        pending_authorizations: 0,
        latest_restore_point: null,
        token_cost_summary: {
          current_task_tokens: 0,
          current_task_cost: 0,
          today_tokens: 0,
          today_cost: 0,
          single_task_limit: 50000,
          daily_limit: 300000,
          budget_auto_downgrade: true,
        },
      },
    }),
    getSettings: async () => ({ settings: remoteSettings }),
    getTaskInspectorConfig: async () => inspectorConfig,
    updateSettings: async (params) => {
      updateSettingsCalled = true;
      const request = params as { models: { provider: string; base_url: string; model: string; api_key?: string } };
      assert.equal(request.models.provider, "broken-provider");
      assert.equal(request.models.base_url, "https://broken.example/v1");
      assert.equal(request.models.model, "bad-model");
      assert.equal(request.models.api_key, "bad-secret");

      return {
        apply_mode: "next_task_effective",
        need_restart: false,
        updated_keys: ["models.provider", "models.base_url", "models.model", "models.api_key"],
        effective_settings: {
          models: {
            provider: request.models.provider,
            budget_auto_downgrade: true,
            provider_api_key_configured: true,
            base_url: request.models.base_url,
            model: request.models.model,
            stronghold: strongholdStatus,
          },
        },
      };
    },
    validateSettingsModel: async () => ({
      ok: false,
      status: "auth_failed",
      message: "模型配置校验失败：鉴权失败，请检查 API Key 或访问权限。",
      provider: "broken-provider",
      canonical_provider: "openai_responses",
      base_url: "https://broken.example/v1",
      model: "bad-model",
      text_generation_ready: false,
      tool_calling_ready: false,
    }),
    updateTaskInspectorConfig: async () => ({ effective_config: inspectorConfig }),
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

  Object.assign(globalThis, { window: { localStorage } });

  try {
    const initialData = await loadControlPanelData();
    await assert.rejects(
      saveControlPanelData(
        {
          ...initialData,
          providerApiKeyInput: "bad-secret",
          settings: {
            ...initialData.settings,
            models: {
              ...initialData.settings.models,
              provider: "broken-provider",
              base_url: "https://broken.example/v1",
              model: "bad-model",
            },
          },
        },
        { saveInspector: false, saveSettings: true, validateModel: true },
      ),
      /当前设置未保存。/,
    );
    assert.equal(updateSettingsCalled, false);

    const validation = await validateControlPanelModel(
      {
        ...initialData,
        providerApiKeyInput: "bad-secret",
        settings: {
          ...initialData.settings,
          models: {
            ...initialData.settings.models,
            provider: "broken-provider",
            base_url: "https://broken.example/v1",
            model: "bad-model",
          },
        },
      },
    );
    assert.equal(validation.ok, false);
    assert.equal(validation.status, "auth_failed");

    const controlPanelSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");
    assert.match(controlPanelSource, /测试连接/);
    assert.match(controlPanelSource, /handleValidateModel/);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("shell-ball protocol stub stays aligned with formal settings snapshot shape", () => {
  const protocolStubSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/test-stubs/protocol.ts"), "utf8");

  assert.match(protocolStubSource, /models:\s*\{[\s\S]*credentials:\s*\{/);
  assert.doesNotMatch(protocolStubSource, /data_log\?:/);
});

test("control-panel save persists local settings after model-only saves and keeps validation metadata", async () => {
  const strongholdStatus = {
    backend: "stronghold",
    available: true,
    fallback: false,
    initialized: true,
    formal_store: true,
  };
  let remoteSettings = {
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
      work_summary_interval: { unit: "day", value: 7 },
      profile_refresh_interval: { unit: "week", value: 2 },
    },
    task_automation: {
      inspect_on_startup: true,
      inspect_on_file_change: true,
      inspection_interval: { unit: "minute", value: 15 },
      task_sources: ["D:/workspace/todos"],
      remind_before_deadline: true,
      remind_when_stale: false,
    },
    models: {
      provider: "openai",
      credentials: {
        budget_auto_downgrade: true,
        provider_api_key_configured: false,
        base_url: "https://api.openai.com/v1",
        model: "gpt-4.1-mini",
        stronghold: strongholdStatus,
      },
    },
  };
  const inspectorConfig = {
    task_sources: ["D:/workspace/todos"],
    inspection_interval: { unit: "minute", value: 15 },
    inspect_on_file_change: true,
    inspect_on_startup: true,
    remind_before_deadline: true,
    remind_when_stale: false,
  };
  let validationCount = 0;
  const { loadControlPanelData, saveControlPanelData } = loadControlPanelServiceModule({
    getSecuritySummary: async () => ({
      summary: {
        security_status: "normal",
        pending_authorizations: 0,
        latest_restore_point: null,
        token_cost_summary: {
          current_task_tokens: 0,
          current_task_cost: 0,
          today_tokens: 0,
          today_cost: 0,
          single_task_limit: 50000,
          daily_limit: 300000,
          budget_auto_downgrade: true,
        },
      },
    }),
    getSettings: async () => ({ settings: remoteSettings }),
    getTaskInspectorConfig: async () => inspectorConfig,
    updateSettings: async (params) => {
      const request = params as {
        models: {
          provider: string;
          budget_auto_downgrade: boolean;
          base_url: string;
          model: string;
          api_key?: string;
        };
      };
      remoteSettings = {
        ...remoteSettings,
        models: {
          provider: request.models.provider,
          credentials: {
            ...remoteSettings.models.credentials,
            budget_auto_downgrade: request.models.budget_auto_downgrade,
            provider_api_key_configured: true,
            base_url: request.models.base_url,
            model: request.models.model,
          },
        },
      };

      return {
        apply_mode: "next_task_effective",
        need_restart: false,
        updated_keys: ["models.provider", "models.base_url", "models.model", "models.api_key"],
        effective_settings: {
          models: {
            provider: request.models.provider,
            budget_auto_downgrade: request.models.budget_auto_downgrade,
            provider_api_key_configured: true,
            base_url: request.models.base_url,
            model: request.models.model,
            stronghold: strongholdStatus,
          },
        },
      };
    },
    validateSettingsModel: async () => {
      validationCount += 1;
      return {
        ok: true,
        status: "ok",
        message: "validated",
        provider: "anthropic",
        canonical_provider: "openai_responses",
        base_url: "https://api.qnaigc.com/v1",
        model: "claude-3-7-sonnet",
        text_generation_ready: true,
        tool_calling_ready: true,
      };
    },
    updateTaskInspectorConfig: async () => ({ effective_config: inspectorConfig }),
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

  Object.assign(globalThis, { window: { localStorage } });

  try {
    const initialData = await loadControlPanelData();
    const result = await saveControlPanelData(
      {
        ...initialData,
        providerApiKeyInput: "saved-secret-key",
        settings: {
          ...initialData.settings,
          models: {
            ...initialData.settings.models,
            provider: "anthropic",
            base_url: "https://api.qnaigc.com/v1",
            model: "claude-3-7-sonnet",
          },
        },
      },
      {
        saveInspector: false,
        saveSettings: true,
      },
    );

    assert.equal(validationCount, 1);
    assert.equal(result.savedSettings, true);
    assert.equal(result.savedInspector, false);
    assert.equal(result.modelValidation?.ok, true);
    const persisted = JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}");
    assert.equal(persisted.settings.models.provider, "anthropic");
    assert.equal(persisted.settings.models.base_url, "https://api.qnaigc.com/v1");
    assert.equal(persisted.settings.models.model, "claude-3-7-sonnet");
    assert.equal(persisted.settings.models.provider_api_key_configured, true);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("mirror overview can reuse a refreshed settings snapshot without reloading the page data", async () => {
  const { updateDashboardSettings } = loadDashboardSettingsMutationModule({
    updateSettings: async () => ({
      apply_mode: "immediate",
      need_restart: false,
      updated_keys: ["memory.enabled", "memory.lifecycle"],
      effective_settings: {
        memory: {
          enabled: false,
          lifecycle: "session",
        },
      },
    }),
    getSettingsDetailed: async () => ({
      data: {
        settings: {
          memory: {
            enabled: false,
            lifecycle: "session",
            work_summary_interval: {
              unit: "week",
              value: 1,
            },
            profile_refresh_interval: {
              unit: "month",
              value: 1,
            },
          },
        },
      },
      meta: {
        server_time: "2026-04-24T09:40:00Z",
      },
      warnings: [],
    }),
  });
  const { applyMirrorSettingsSnapshot } = loadMirrorServiceModule();
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
    const result = await updateDashboardSettings({
      memory: {
        enabled: false,
        lifecycle: "session",
      },
    });
    const currentOverview = {
      overview: {
        history_summary: ["recent mirror summary"],
      },
      insight: {
        badge: "mirror ready",
      },
      latestRestorePoint: null,
      rpcContext: {
        serverTime: "2026-04-24T09:00:00Z",
        warnings: [],
      },
      settingsSnapshot: {
        source: "rpc",
        settings: {
          memory: {
            enabled: true,
            lifecycle: "30d",
          },
          general: {
            download: {
              ask_before_save_each_file: true,
            },
          },
        },
      },
      source: "rpc" as const,
      conversations: [{ id: "conv_1" }],
    };

    const nextOverview = applyMirrorSettingsSnapshot(currentOverview, result.snapshot);

    assert.equal(nextOverview.settingsSnapshot.settings.memory.enabled, false);
    assert.equal(nextOverview.settingsSnapshot.settings.memory.lifecycle, "session");
    assert.equal(nextOverview.settingsSnapshot.settings.general.download.ask_before_save_each_file, true);
    assert.deepEqual(nextOverview.overview.history_summary, currentOverview.overview.history_summary);
    assert.deepEqual(nextOverview.conversations, currentOverview.conversations);
    assert.equal(nextOverview.source, "rpc");
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("mirror app reuses the mutation snapshot instead of triggering a second mirror overview reload", () => {
  const mirrorAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorApp.tsx"), "utf8");

  assert.match(mirrorAppSource, /applyMirrorSettingsSnapshot\(current, result\.snapshot\)/);
  assert.doesNotMatch(
    mirrorAppSource,
    /const handleSettingsUpdate = useCallback\([\s\S]*loadMirrorOverviewData\(dataMode\)/,
  );
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

  assert.deepEqual(resolveDashboardTaskSafetyOpenPlan("loading"), {
    shouldRefetchDetail: true,
  });
  assert.deepEqual(resolveDashboardTaskSafetyOpenPlan("error"), {
    shouldRefetchDetail: true,
  });
  assert.deepEqual(resolveDashboardTaskSafetyOpenPlan("ready"), {
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
      mode: "open_local_path",
      taskId: "task_dashboard_001",
      path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx",
      url: null,
      feedback: "已打开本地文件。",
    },
  );

  assert.deepEqual(
    outputService.resolveTaskOpenExecutionPlan({
      artifact: {
        artifact_id: "artifact_dashboard_002",
        artifact_type: "generated_file",
        mime_type: "application/pdf",
        path: "workspace/reports/q3-review.pdf",
        task_id: "task_dashboard_001",
        title: "q3-review.pdf",
      },
      open_action: "reveal_in_folder",
      resolved_payload: { path: "workspace/reports/q3-review.pdf", url: null, task_id: "task_dashboard_001" },
      delivery_result: {
        type: "reveal_in_folder",
        title: "q3-review.pdf",
        preview_text: "定位文件",
        payload: { path: "workspace/reports/q3-review.pdf", url: null, task_id: "task_dashboard_001" },
      },
    }),
    {
      mode: "reveal_local_path",
      taskId: "task_dashboard_001",
      path: "workspace/reports/q3-review.pdf",
      url: null,
      feedback: "已在文件夹中定位结果。",
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

test("note resource open helpers normalize task, url, local open, and copy flows", () => {
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

  const openFilePlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_003",
    label: "Draft",
    openAction: "open_file",
    path: "workspace/drafts/spec.md",
    taskId: null,
    type: "draft",
    url: null,
  });
  assert.equal(openFilePlan.mode, "open_local_path");
  assert.equal(openFilePlan.path, "workspace/drafts/spec.md");

  const copyPlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_003_copy",
    label: "Draft",
    openAction: "copy_path",
    path: "workspace/drafts/spec.md",
    taskId: null,
    type: "draft",
    url: null,
  });
  assert.equal(copyPlan.mode, "copy_path");
  assert.equal(copyPlan.path, "workspace/drafts/spec.md");

  const revealPlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_004",
    label: "Exports",
    openAction: "reveal_in_folder",
    path: "workspace/exports/q3-review.pdf",
    taskId: null,
    type: "artifact",
    url: null,
  });
  assert.equal(revealPlan.mode, "reveal_local_path");
  assert.equal(revealPlan.path, "workspace/exports/q3-review.pdf");

  const missingPlan = noteService.resolveNoteResourceOpenExecutionPlan({
    id: "note_resource_005",
    label: "Missing",
    openAction: "copy_path",
    path: "",
    taskId: null,
    type: "artifact",
    url: null,
  });
  assert.equal(missingPlan.mode, "copy_path");

  assert.equal(noteService.isAllowedNoteOpenUrl("https://example.test/spec"), true);
  assert.equal(noteService.isAllowedNoteOpenUrl("http://example.test/spec"), true);
  assert.equal(noteService.isAllowedNoteOpenUrl("javascript:alert(1)"), false);
  assert.equal(noteService.isAllowedNoteOpenUrl("file:///tmp/spec.md"), false);
});

test("task output execution uses desktop local open handlers and falls back to copying paths on failure", async () => {
  let openedPath: string | null = null;
  const successService = loadTaskOutputServiceModule({
    openDesktopLocalPath: async (path) => {
      openedPath = path;
    },
  });

  const successMessage = await successService.performTaskOpenExecution({
    mode: "open_local_path",
    taskId: "task_dashboard_001",
    path: "workspace/reports/q3-review.pdf",
    url: null,
    feedback: "已打开本地文件。",
  });
  assert.equal(openedPath, "workspace/reports/q3-review.pdf");
  assert.equal(successMessage, "已打开本地文件。");

  const failingService = loadTaskOutputServiceModule({
    revealDesktopLocalPath: async () => {
      throw new Error("target missing");
    },
  });
  const fallbackMessage = await failingService.performTaskOpenExecution({
    mode: "reveal_local_path",
    taskId: "task_dashboard_001",
    path: "workspace/reports/q3-review.pdf",
    url: null,
    feedback: "已在文件夹中定位结果。",
  });

  assert.match(fallbackMessage, /无法在文件夹中定位结果/);
  assert.match(fallbackMessage, /workspace\/reports\/q3-review\.pdf/);
});

test("note resource execution uses desktop local open handlers and keeps copy-path fallback", async () => {
  let revealedPath: string | null = null;
  const successService = loadNotePageServiceModule({
    revealDesktopLocalPath: async (path) => {
      revealedPath = path;
    },
  });

  const revealMessage = await successService.performNoteResourceOpenExecution({
    mode: "reveal_local_path",
    feedback: "已在文件夹中定位 Exports。",
    path: "workspace/exports/q3-review.pdf",
    taskId: null,
    url: null,
  });
  assert.equal(revealedPath, "workspace/exports/q3-review.pdf");
  assert.equal(revealMessage, "已在文件夹中定位 Exports。");

  const failingService = loadNotePageServiceModule({
    openDesktopLocalPath: async () => {
      throw new Error("target missing");
    },
  });
  const fallbackMessage = await failingService.performNoteResourceOpenExecution({
    mode: "open_local_path",
    feedback: "已打开 Draft。",
    path: "workspace/drafts/spec.md",
    taskId: null,
    url: null,
  });

  assert.match(fallbackMessage, /无法直接打开本地资源/);
  assert.match(fallbackMessage, /workspace\/drafts\/spec\.md/);
});

test("task output execution delegates task-detail routing through the shared callback", async () => {
  const outputService = loadTaskOutputServiceModule();
  const openedTaskIds: string[] = [];

  const feedback = await outputService.performTaskOpenExecution({
    mode: "task_detail",
    taskId: "task_dashboard_001",
    path: null,
    url: null,
    feedback: "宸插畾浣嶅埌浠诲姟璇︽儏銆?",
  }, {
    onOpenTaskDetail: ({ taskId }) => {
      openedTaskIds.push(taskId);
      return "宸插湪浠〃鐩樹腑鎵撳紑浠诲姟璇︽儏銆?";
    },
  });

  assert.deepEqual(openedTaskIds, ["task_dashboard_001"]);
  assert.equal(feedback, "宸插湪浠〃鐩樹腑鎵撳紑浠诲姟璇︽儏銆?");
});

test("note resource execution delegates task-detail routing through the shared callback", async () => {
  const noteService = loadNotePageServiceModule();
  const openedTaskIds: string[] = [];

  const feedback = await noteService.performNoteResourceOpenExecution({
    mode: "task_detail",
    feedback: "宸插畾浣嶅埌浠诲姟 Task detail銆?",
    path: null,
    taskId: "task_dashboard_001",
    url: null,
  }, {
    onOpenTaskDetail: ({ taskId }) => {
      openedTaskIds.push(taskId);
      return "宸插湪浠〃鐩樹腑鎵撳紑 Task detail銆?";
    },
  });

  assert.deepEqual(openedTaskIds, ["task_dashboard_001"]);
  assert.equal(feedback, "宸插湪浠〃鐩樹腑鎵撳紑 Task detail銆?");
});

test("task page adopts rpc output helpers directly in the task detail panel", () => {
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const taskDetailSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");
  const taskOutputSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskOutput.service.ts"), "utf8");
  const taskDetailNavigationSource = loadDashboardTaskDetailNavigationSource();

  assert.match(taskPageSource, /buildDashboardTaskArtifactQueryKey/);
  assert.match(taskPageSource, /loadTaskArtifactPage/);
  assert.match(taskPageSource, /openTaskArtifactForTask/);
  assert.match(taskPageSource, /openTaskDeliveryForTask/);
  assert.match(taskPageSource, /readDashboardTaskDetailRouteState/);
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
  assert.match(taskOutputSource, /onOpenTaskDetail/);
  assert.match(taskDetailNavigationSource, /requestDashboardTaskDetailOpen/);
});

test("dashboard task-detail routing deduplicates retry request ids and accepts tasks outside loaded buckets", () => {
  const dashboardRootSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardRoot.tsx"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");

  assert.match(dashboardRootSource, /handledTaskDetailRequestIdsRef = useRef<Map<string, number>>\(new Map\(\)\)/);
  assert.match(dashboardRootSource, /function rememberHandledTaskDetailRequest\(requestId: string\)/);
  assert.match(dashboardRootSource, /if \(!rememberHandledTaskDetailRequest\(payload\.request_id\)\) \{/);
  assert.doesNotMatch(dashboardRootSource, /handledTaskDetailRequestIdRef\.current === payload\.request_id/);

  assert.match(taskPageSource, /const detailRouteState = readDashboardTaskDetailRouteState\(location\.state\);[\s\S]*if \(detailRouteState\) \{[\s\S]*setSelectedTaskId\(detailRouteState\.focusTaskId\);[\s\S]*navigate\(location\.pathname, \{ replace: true, state: null \}\);[\s\S]*return;/);
  assert.doesNotMatch(taskPageSource, /detailRouteState && allTasks\.some\(\(item\) => item\.task\.task_id === detailRouteState\.focusTaskId\)/);
  assert.match(taskPageSource, /if \(selectedTaskId && detailOpen\) \{/);
});

test("conversation session reuse expires after the backend freshness window", () => {
  const originalDate = globalThis.Date;

  class FreshFakeDate extends Date {
    constructor(value?: string | number | Date) {
      super(value ?? FreshFakeDate.now());
    }

    static now() {
      return originalDate.parse("2026-04-23T10:00:00.000Z");
    }
  }

  Object.defineProperty(globalThis, "Date", {
    configurable: true,
    value: FreshFakeDate,
  });

  try {
    const service = loadConversationSessionServiceModule();

    assert.equal(
      service.rememberConversationSessionFromTask(
        createTask({
          session_id: "sess_backend_fresh",
          task_id: "task_dashboard_session",
        }),
      ),
      "sess_backend_fresh",
    );
    assert.equal(service.getCurrentConversationSessionId(), "sess_backend_fresh");
    assert.equal(service.getConversationSessionIdForTask("task_dashboard_session"), "sess_backend_fresh");

    Object.defineProperty(globalThis, "Date", {
      configurable: true,
      value: class ExpiredFakeDate extends Date {
        constructor(value?: string | number | Date) {
          super(value ?? ExpiredFakeDate.now());
        }

        static now() {
          return originalDate.parse("2026-04-23T10:15:00.001Z");
        }
      },
    });

    assert.equal(service.getCurrentConversationSessionId(), undefined);
    assert.equal(service.getConversationSessionIdForTask("task_dashboard_session"), undefined);
  } finally {
    Object.defineProperty(globalThis, "Date", {
      configurable: true,
      value: originalDate,
    });
  }
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

test("note service no longer invents related resources from title keywords", () => {
  const noteServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.service.ts"), "utf8");

  assert.match(noteServiceSource, /function createResourceHints\(item: TodoItem\)/);
  assert.doesNotMatch(noteServiceSource, /normalizedTitle\.includes\("template"\)/);
  assert.doesNotMatch(noteServiceSource, /normalizedTitle\.includes\("report"\)/);
  assert.doesNotMatch(noteServiceSource, /normalizedTitle\.includes\("design"\)/);
  assert.match(noteServiceSource, /return \[\];/);
});

test("task fallback copy no longer claims backend output actions are missing", () => {
  const taskServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.service.ts"), "utf8");
  const taskTabsSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskTabsPanel.tsx"), "utf8");

  assert.doesNotMatch(taskServiceSource, /当前协议未返回更多结果摘要/);
  assert.doesNotMatch(taskServiceSource, /后续可把任务修改或产出打开能力接进来/);
  assert.doesNotMatch(taskTabsSource, /当前协议尚未提供稳定的 artifact\.open 能力/);
});

test("task detail normalization rejects string restore points in rpc mode and keeps runtime summary defaults", () => {
  withDesktopAliasRuntime((requireFn) => {
    const service = requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.service.js")) as {
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

    const taskServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.service.ts"), "utf8");
    assert.doesNotMatch(taskServiceSource, /buildFallbackTaskDetailData/);
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

test("task rpc service builds protocol-only experience instead of reusing mock task fixtures", () => {
  const taskServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.service.ts"), "utf8");

  assert.match(taskServiceSource, /function buildProtocolTaskExperience\(task: Task, detail\?: AgentTaskDetailGetResult\)/);
  assert.doesNotMatch(taskServiceSource, /getTaskExperience\(/);
  assert.doesNotMatch(taskServiceSource, /createFallbackExperience\(/);
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

test("note rpc service derives experience from protocol note data instead of mock fixtures", () => {
  const noteServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.service.ts"), "utf8");

  assert.match(noteServiceSource, /function mapItems\(items: TodoItem\[\]\)/);
  assert.doesNotMatch(noteServiceSource, /getMockNoteExperience\(/);
});

test("security rpc service keeps transport failures visible instead of switching to mock governance data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/safety/securityService.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadSecurityModuleData: (source?: "rpc" | "mock") => Promise<unknown>;
        loadSecurityModuleRpcData: () => Promise<unknown>;
      };

      await assert.rejects(() => service.loadSecurityModuleData("rpc"), /transport is not wired/i);
      await assert.rejects(() => service.loadSecurityModuleRpcData(), /transport is not wired/i);
    },
    {
      getSecuritySummaryDetailed: () => Promise.reject(transportError),
      listSecurityPendingDetailed: () => Promise.reject(transportError),
    },
  );
});

test("mirror rpc service keeps transport failures visible instead of switching to mock overview data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/memory/mirrorService.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadMirrorOverviewData: (source?: "rpc" | "mock") => Promise<unknown>;
      };

      await assert.rejects(() => service.loadMirrorOverviewData("rpc"), /transport is not wired/i);
    },
    {
      getMirrorOverviewDetailed: () => Promise.reject(transportError),
    },
  );
});

test("TaskDetailPanel defers the security summary until formal detail arrives", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /detailState !== "ready" \|\| detail === null/);
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

test("TaskDetailPanel separates formal delivery from structured evidence metadata", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /const formalDeliveryResult = detail\?\.delivery_result \?\? null;/);
  assert.match(panelSource, /Formal Delivery/);
  assert.match(panelSource, /该区域只消费正式 `delivery_result`/);
  assert.match(panelSource, /citation\.evidence_role/);
  assert.match(panelSource, /citation\.artifact_type/);
  assert.match(panelSource, /citation\.excerpt_text/);
});

test("TaskDetailPanel renders a formal screen governance section only for screen tasks with synced detail", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /const isScreenTask = task\?\.source_type === "screen_capture" \|\| detail\?\.task\.intent\?\.name === "screen_analyze"/);
  assert.match(panelSource, /if \(!isScreenTask \|\| shouldDeferSecuritySummary \|\| !runtimeSummary \|\| detail === null\) \{/);
  assert.match(panelSource, /Screen Governance/);
  assert.match(panelSource, /屏幕授权、恢复与失败收口/);
  assert.match(panelSource, /该区域只消费正式 `approval_request`、`authorization_record`、`audit_record`、`recovery_point` 与 `runtime_summary` 字段/);
  assert.match(panelSource, /runtimeSummary\.latest_failure_category/);
  assert.match(panelSource, /detail\.approval_request/);
  assert.match(panelSource, /detail\.authorization_record/);
  assert.match(panelSource, /detail\.audit_record/);
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

