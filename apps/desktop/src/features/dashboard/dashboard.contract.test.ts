import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";
import type {
  AgentDeliveryOpenParams,
  AgentDeliveryOpenResult,
  AgentNotepadConvertToTaskParams,
  AgentNotepadConvertToTaskResult,
  AgentNotepadListParams,
  AgentNotepadListResult,
  AgentSettingsGetParams,
  AgentNotepadUpdateParams,
  AgentNotepadUpdateResult,
  AgentTaskArtifactListParams,
  AgentTaskArtifactListResult,
  AgentTaskArtifactOpenParams,
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

function loadDashboardOpeningTransitionModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/app/dashboard/dashboardOpeningTransition.js")) as {
      DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS: number;
      createDashboardOpeningTransitionController: (environment: {
        cancelAnimationFrame: (handle: number) => void;
        clearTimeout: (handle: number) => void;
        hasFocus: () => boolean;
        getVisibilityState: () => DocumentVisibilityState;
        requestAnimationFrame: (callback: FrameRequestCallback) => number;
        setIsOpening: (value: boolean) => void;
        setTimeout: (callback: () => void, timeoutMs: number) => number;
      }) => {
        dispose: () => void;
        handleVisibilityChange: () => boolean;
        handleWindowFocusChanged: (focused: boolean) => boolean;
        restoreIfNeeded: () => boolean;
        trigger: () => void;
      };
    },
  );
}

function loadDashboardWindowErrorBoundaryModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/app/dashboard/DashboardWindowErrorBoundary.js")) as {
      DashboardWindowErrorBoundary: (props: { children: unknown }) => {
        props: { children: unknown };
        type: {
          new (props: { children: unknown }): {
            componentDidCatch: (error: Error, errorInfo: { componentStack: string }) => void;
            props: { children: unknown };
            render: () => unknown;
            state: { hasError: boolean };
          };
          getDerivedStateFromError: () => { hasError: boolean };
        };
      };
    },
  );
}

function instantiateDashboardWindowErrorBoundary(
  DashboardWindowErrorBoundary: (props: { children: unknown }) => {
    props: { children: unknown };
    type: {
      new (props: { children: unknown }): {
          componentDidCatch: (error: Error, errorInfo: { componentStack: string }) => void;
          props: { children: unknown };
          render: () => unknown;
          state: { hasError: boolean };
        };
        getDerivedStateFromError: () => { hasError: boolean };
      };
  },
) {
  const renderedBoundary = DashboardWindowErrorBoundary({ children: null });
  const BoundaryImplementation = renderedBoundary.type;

  return {
    BoundaryImplementation,
    create(props: { children: unknown }) {
      const element = DashboardWindowErrorBoundary(props);
      return new BoundaryImplementation(element.props);
    },
  };
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
      buildDashboardTaskArtifactQueryKey: (dataMode: "rpc", taskId: string) => unknown;
      buildDashboardTaskBucketQueryKey: (dataMode: "rpc", group: "unfinished" | "finished", limit: number) => unknown;
      buildDashboardTaskDetailQueryKey: (dataMode: "rpc", taskId: string) => unknown;
      getDashboardTaskSecurityRefreshPlan: (dataMode: "rpc") => unknown;
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
      buildDashboardNoteBucketInvalidateKeys: (dataMode: "rpc", groups: ReadonlyArray<"upcoming" | "later" | "recurring_rule" | "closed">) => unknown;
      buildDashboardNoteBucketQueryKey: (dataMode: "rpc", group: "upcoming" | "later" | "recurring_rule" | "closed") => unknown;
      getDashboardNoteRefreshPlan: (dataMode: "rpc") => unknown;
      dashboardNoteBucketGroups: unknown;
      dashboardNoteBucketQueryPrefix: unknown;
    },
  );
}

function loadSourceNoteEditorModule() {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/sourceNoteEditor.js");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      createEmptySourceNoteEditorDraft: (sourcePath?: string | null) => {
        agentSuggestion: string;
        bucket: "upcoming" | "later" | "recurring_rule" | "closed";
        checked: boolean;
        createdAt: string;
        dueAt: string;
        effectiveScope: string;
        endedAt: string;
        extraMetadata: Array<{ key: string; value: string }>;
        nextOccurrenceAt: string;
        noteText: string;
        prerequisite: string;
        recentInstanceStatus: string;
        repeatRule: string;
        sourceLine: number | null;
        sourcePath: string | null;
        title: string;
        updatedAt: string;
      };
      formatSourceNoteEditorContent: (draft: {
        title: string;
        noteText: string;
      }) => string;
      formatSourceNoteScheduleInputValue: (value: string | null | undefined) => string;
      sanitizeSourceNoteBodyText: (
        value: string | null | undefined,
        options?: {
          title?: string | null;
        },
      ) => string;
      parseSourceNoteEditorBlocks: (note: {
        content: string;
        fileName: string;
        modifiedAtMs: number | null;
        path: string;
        sourceRoot: string;
        title: string;
      }) => Array<{
        agentSuggestion: string;
        bucket: "upcoming" | "later" | "recurring_rule" | "closed";
        checked: boolean;
        createdAt: string;
        dueAt: string;
        effectiveScope: string;
        endedAt: string;
        extraMetadata: Array<{ key: string; value: string }>;
        nextOccurrenceAt: string;
        noteText: string;
        prerequisite: string;
        recentInstanceStatus: string;
        repeatRule: string;
        sourceLine: number | null;
        sourcePath: string | null;
        title: string;
        updatedAt: string;
      }>;
      removeSourceNoteEditorBlock: (
        note: {
          content: string;
          fileName: string;
          modifiedAtMs: number | null;
          path: string;
          sourceRoot: string;
          title: string;
        },
        draft: {
          sourceLine: number | null;
          title: string;
        },
      ) => {
        content: string;
        removed: boolean;
      };
      serializeSourceNoteEditorDraft: (draft: {
        agentSuggestion: string;
        bucket: "upcoming" | "later" | "recurring_rule" | "closed";
        checked: boolean;
        createdAt: string;
        dueAt: string;
        effectiveScope: string;
        endedAt: string;
        extraMetadata: Array<{ key: string; value: string }>;
        nextOccurrenceAt: string;
        noteText: string;
        prerequisite: string;
        recentInstanceStatus: string;
        repeatRule: string;
        sourceLine: number | null;
        sourcePath: string | null;
        title: string;
        updatedAt: string;
      }, now?: Date) => {
        blockContent: string;
        normalizedDraft: {
          noteText: string;
          title: string;
        };
      };
      buildSourceNoteEditorDraftFromNote: (note: {
        content: string;
        fileName: string;
        modifiedAtMs: number | null;
        path: string;
        sourceRoot: string;
        title: string;
      }, item: unknown) => {
        noteText: string;
        title: string;
      };
      updateSourceNoteEditorDraftContent: <TDraft extends {
        checked: boolean;
        noteText: string;
        title: string;
      }>(draft: TDraft, content: string) => TDraft;
      resolveSourceNoteDraftBucketForSchedule: (schedule: {
        dueAt: string;
        repeatRule: string;
      }) => "upcoming" | "later" | "recurring_rule" | "closed";
      serializeSourceNoteScheduleInputValue: (value: string) => string;
    };
  });
}

type DashboardContractDesktopLocalPathOverrides = {
  openDesktopLocalPath?: (path: string) => Promise<void>;
  revealDesktopLocalPath?: (path: string) => Promise<void>;
};

type DashboardContractDesktopHostOverrides = {
  invoke?: (command: string, args?: Record<string, unknown>) => Promise<unknown> | unknown;
};

type DashboardContractWindowControllerOverrides = {
  openOrFocusDesktopWindow?: (label: "dashboard" | "control-panel") => Promise<string> | string;
};

type DashboardContractWindowApiOverrides = {
  getCurrentWindow?: () => {
    emit: (eventName: string, payload?: unknown) => Promise<void> | void;
    emitTo: (label: string, eventName: string, payload?: unknown) => Promise<void> | void;
    label: string;
  };
};

function loadNotePageServiceModule(desktopLocalPath?: DashboardContractDesktopLocalPathOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/notePage.service.js");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      buildVisibleNoteText: (
        value: string | null | undefined,
        options?: {
          title?: string | null;
        },
      ) => string;
      buildSourceNoteFallbackItems: (note: {
        content: string;
        fileName: string;
        modifiedAtMs: number | null;
        path: string;
        sourceRoot: string;
        title: string;
      }) => Array<{
        experience: {
          canConvertToTask: boolean;
          detailStatus: string;
          previewStatus: string;
          repeatRule: string | null;
        };
        item: {
          bucket: string;
          status: string;
        };
      }>;
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

function loadTaskOutputServiceModule(
  desktopLocalPath?: DashboardContractDesktopLocalPathOverrides,
  windowController?: DashboardContractWindowControllerOverrides,
  windowApi?: DashboardContractWindowApiOverrides,
) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskOutput.service.js");
    delete requireFn.cache[modulePath];

    return requireFn(modulePath) as {
      describeTaskOpenResultForCurrentTask: (plan: { mode: string; taskId: string | null }, currentTaskId: string | null) => string | null;
      isAllowedTaskOpenUrl: (url: string) => boolean;
      loadTaskArtifactPage: (taskId: string, source: "rpc") => Promise<AgentTaskArtifactListResult>;
      openTaskArtifactForTask: (taskId: string, artifactId: string, source: "rpc") => Promise<AgentTaskArtifactOpenResult>;
      openTaskDeliveryForTask: (taskId: string, artifactId: string | undefined, source: "rpc") => Promise<AgentDeliveryOpenResult>;
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
        onOpenTaskDelivery?: (input: {
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
  }, undefined, desktopLocalPath, undefined, windowController, windowApi);
}

function loadTaskPageMapperModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskPage.mapper.js")) as {
      canTaskAcceptSteering: (task: Task) => boolean;
      getTaskRunwayTone: (status: Task["status"]) => "departure" | "holding" | "irregular" | "archive";
      getTaskPrimaryActions: (task: Task, detail: AgentTaskDetailGetResult) => Array<{ action: string; label: string; tooltip: string }>;
    },
  );
}

function loadNotePageMapperModule() {
  return withDesktopAliasRuntime((requireFn) =>
    requireFn(resolve(desktopRoot, "src/features/dashboard/notes/notePage.mapper.ts")) as {
      describeNotePreview: (
        item: { bucket: "upcoming" | "later" | "recurring_rule" | "closed" },
        experience: { isRecurringEnabled?: boolean; repeatRule?: string; previewStatus?: string; timeHint: string },
      ) => string;
      formatNoteBoardTimeHint: (
        item: { bucket: "upcoming" | "later" | "recurring_rule" | "closed" },
        experience: { isRecurringEnabled?: boolean; timeHint: string },
      ) => string;
    },
  );
}

function loadSettingsServiceModule(desktopHost?: DashboardContractDesktopHostOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/services/settingsService.js");
    const runtimeDefaultsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/platform/desktopRuntimeDefaults.js");
    delete requireFn.cache[modulePath];
    delete requireFn.cache[runtimeDefaultsModulePath];

    return requireFn(modulePath) as {
      loadDesktopRuntimeDefaultsSnapshot: () => Promise<{
        data_path: string;
        workspace_path: string;
        task_sources: string[];
      } | null>;
      loadHydratedSettings: () => Promise<{
        settings: {
          general: {
            download: {
              workspace_path: string;
            };
          };
          task_automation: {
            task_sources: string[];
          };
        };
      }>;
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
          task_automation: {
            task_sources: string[];
          };
        };
      };
      saveSettings: (settings: unknown) => void;
    };
  },
    undefined,
    undefined,
    desktopHost,
  );
}

function loadNoteSourceServiceModule(
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopHost?: DashboardContractDesktopHostOverrides,
) {
  return withDesktopAliasRuntime(
    (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/notes/noteSource.service.ts");
      const settingsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/services/settingsService.js");
      const settingsSnapshotModulePath = resolve(desktopRoot, ".cache/dashboard-tests/platform/desktopSettingsSnapshot.js");
      const runtimeDefaultsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/platform/desktopRuntimeDefaults.js");
      const sourceNotesModulePath = resolve(desktopRoot, "src/platform/desktopSourceNotes.ts");
      delete requireFn.cache[modulePath];
      delete requireFn.cache[settingsModulePath];
      delete requireFn.cache[settingsSnapshotModulePath];
      delete requireFn.cache[runtimeDefaultsModulePath];
      delete requireFn.cache[sourceNotesModulePath];

      return requireFn(modulePath) as {
        loadNoteSourceConfig: () => Promise<{
          task_sources: string[];
        }>;
        loadNoteSourceSnapshot: (taskSources: string[]) => Promise<{
          defaultSourceRoot: string | null;
          notes: Array<{
            content: string;
            fileName: string;
            modifiedAtMs: number | null;
            path: string;
            sourceRoot: string;
            title: string;
          }>;
          sourceRoots: string[];
        }>;
        loadNoteSourceIndex: (taskSources: string[]) => Promise<{
          defaultSourceRoot: string | null;
          notes: Array<{
            fileName: string;
            modifiedAtMs: number | null;
            path: string;
            sizeBytes: number;
            sourceRoot: string;
          }>;
          sourceRoots: string[];
        }>;
        createNoteSource: (taskSources: string[], content: string) => Promise<{
          content: string;
          fileName: string;
          modifiedAtMs: number | null;
          path: string;
          sourceRoot: string;
          title: string;
        }>;
        saveNoteSource: (taskSources: string[], path: string, content: string) => Promise<{
          content: string;
          fileName: string;
          modifiedAtMs: number | null;
          path: string;
          sourceRoot: string;
          title: string;
        }>;
        runNoteSourceInspection: (taskSources: string[], reason: string) => Promise<{
          accepted_sources?: string[];
          ok?: boolean;
          reason?: string;
        }>;
      };
    },
    rpcMethods,
    undefined,
    desktopHost,
  );
}

function loadControlPanelServiceModule(
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopHost?: DashboardContractDesktopHostOverrides,
) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, "src/services/controlPanelService.ts");
    const settingsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/services/settingsService.js");
    const runtimeDefaultsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/platform/desktopRuntimeDefaults.js");
    delete requireFn.cache[modulePath];
    delete requireFn.cache[settingsModulePath];
    delete requireFn.cache[runtimeDefaultsModulePath];

    return requireFn(modulePath) as {
      buildControlPanelRestoreDefaultsData: (data: {
        source: "rpc";
        settings: {
          general: {
            language: string;
            auto_launch: boolean;
            theme_mode: string;
            voice_notification_enabled: boolean;
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
          task_automation: {
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
          models: {
            provider: string;
            provider_api_key_configured: boolean;
            budget_auto_downgrade: boolean;
            base_url: string;
            model: string;
            stronghold: {
              backend: string;
              available: boolean;
              fallback: boolean;
              initialized: boolean;
              formal_store: boolean;
            };
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
        securitySummary: {
          security_status: string;
          pending_authorizations: number;
          latest_restore_point: null;
          token_cost_summary: {
            current_task_tokens: number;
            current_task_cost: number;
            today_tokens: number;
            today_cost: number;
            single_task_limit: number;
            daily_limit: number;
            budget_auto_downgrade: boolean;
          };
        };
        warnings?: string[];
      }, persisted: {
        source: "rpc";
        providerApiKeyInput: string;
        settings: {
          general: {
            language: string;
            auto_launch: boolean;
            theme_mode: string;
            voice_notification_enabled: boolean;
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
          task_automation: {
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
          models: {
            provider: string;
            provider_api_key_configured: boolean;
            budget_auto_downgrade: boolean;
            base_url: string;
            model: string;
            stronghold: {
              backend: string;
              available: boolean;
              fallback: boolean;
              initialized: boolean;
              formal_store: boolean;
            };
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
        securitySummary: {
          security_status: string;
          pending_authorizations: number;
          latest_restore_point: null;
          token_cost_summary: {
            current_task_tokens: number;
            current_task_cost: number;
            today_tokens: number;
            today_cost: number;
            single_task_limit: number;
            daily_limit: number;
            budget_auto_downgrade: boolean;
          };
        };
        warnings?: string[];
      }) => {
        source: "rpc";
        providerApiKeyInput: string;
        settings: {
          general: {
            language: string;
            auto_launch: boolean;
            theme_mode: string;
            voice_notification_enabled: boolean;
            voice_type: string;
            download: {
              ask_before_save_each_file: boolean;
              workspace_path: string;
            };
          };
          task_automation: {
            task_sources: string[];
            inspect_on_file_change: boolean;
            inspect_on_startup: boolean;
            remind_before_deadline: boolean;
            remind_when_stale: boolean;
            inspection_interval: {
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
            stronghold: {
              backend: string;
              available: boolean;
              fallback: boolean;
              initialized: boolean;
              formal_store: boolean;
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
        warnings?: string[];
      };
      loadControlPanelData: () => Promise<{
        source: "rpc";
        runtimeWorkspacePath: string | null;
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
  }, rpcMethods, undefined, desktopHost);
}

function loadControlPanelAboutServiceModule(desktopHost?: DashboardContractDesktopHostOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, "src/services/controlPanelAboutService.ts");
    const settingsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/services/settingsService.js");
    const runtimeDefaultsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/platform/desktopRuntimeDefaults.js");
    delete requireFn.cache[modulePath];
    delete requireFn.cache[settingsModulePath];
    delete requireFn.cache[runtimeDefaultsModulePath];

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
        localDataPath: string | null;
      };
      loadControlPanelAboutSnapshot: () => Promise<{
        appName: string;
        appVersion: string;
        localDataPath: string | null;
      }>;
      copyControlPanelAboutValue: (value: string, successMessage: string) => Promise<string>;
      runControlPanelAboutAction: (action: "open_data_directory" | "share") => Promise<string>;
    };
  }, undefined, undefined, desktopHost);
}

function loadDashboardSettingsMutationModule(rpcMethods?: DashboardContractRpcMethodOverrides) {
  return withDesktopAliasRuntime((requireFn) => {
    const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsMutation.js");
    const snapshotModulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsSnapshot.js");

    delete requireFn.cache[modulePath];
    delete requireFn.cache[snapshotModulePath];

    return requireFn(modulePath) as {
      formatDashboardSettingsMutationFeedback: (result: {
        applyMode: string;
        needRestart: boolean;
        persisted: boolean;
        readbackWarning: string | null;
      }, subject: string) => string;
      updateDashboardSettings: (patch: Record<string, unknown>, source?: "rpc") => Promise<{
        applyMode: string;
        needRestart: boolean;
        persisted: boolean;
        readbackWarning: string | null;
        source: string;
        updatedKeys: string[];
        snapshot: {
          rpcContext: {
            serverTime: string | null;
            warnings: string[];
          };
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
        source?: "rpc",
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
          source: "rpc";
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
          source: "rpc";
        conversations: Array<{ id: string }>;
      };
    };
  });
}

function findRenderedElement(
  node: unknown,
  predicate: (element: { props: Record<string, unknown>; type: unknown }) => boolean,
): { props: Record<string, unknown>; type: unknown } | null {
  if (Array.isArray(node)) {
    for (const item of node) {
      const match = findRenderedElement(item, predicate);
      if (match) {
        return match;
      }
    }
    return null;
  }

  if (!node || typeof node !== "object") {
    return null;
  }

  const maybeElement = node as { props?: Record<string, unknown>; type?: unknown };
  if (!maybeElement.props || !("type" in maybeElement)) {
    return null;
  }

  const element = {
    props: maybeElement.props,
    type: maybeElement.type,
  };
  if (predicate(element)) {
    return element;
  }

  return findRenderedElement(element.props.children, predicate);
}

type DashboardContractRpcMethodOverrides = {
  applySecurityRestoreDetailed?: (params: unknown) => Promise<unknown>;
  controlTask?: (params: AgentTaskControlParams) => Promise<AgentTaskControlResult>;
  convertNotepadToTask?: (params: AgentNotepadConvertToTaskParams) => Promise<AgentNotepadConvertToTaskResult>;
  getDashboardModule?: (params: unknown) => Promise<unknown>;
  getDashboardOverview?: (params: unknown) => Promise<unknown>;
  getMirrorOverview?: (params: unknown) => Promise<unknown>;
  getRecommendations?: (params: unknown) => Promise<unknown>;
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
  listTaskArtifacts?: (params: AgentTaskArtifactListParams) => Promise<AgentTaskArtifactListResult>;
  listNotepad?: (params: AgentNotepadListParams) => Promise<AgentNotepadListResult>;
  listTasks?: (params: AgentTaskListParams) => Promise<AgentTaskListResult>;
  openDelivery?: (params: AgentDeliveryOpenParams) => Promise<AgentDeliveryOpenResult>;
  openTaskArtifact?: (params: AgentTaskArtifactOpenParams) => Promise<AgentTaskArtifactOpenResult>;
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
  desktopHost?: DashboardContractDesktopHostOverrides,
  windowController?: DashboardContractWindowControllerOverrides,
  windowApi?: DashboardContractWindowApiOverrides,
): Promise<T>;
function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => T,
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopLocalPath?: DashboardContractDesktopLocalPathOverrides,
  desktopHost?: DashboardContractDesktopHostOverrides,
  windowController?: DashboardContractWindowControllerOverrides,
  windowApi?: DashboardContractWindowApiOverrides,
): T;
function withDesktopAliasRuntime<T>(
  callback: (requireFn: NodeRequire) => T | Promise<T>,
  rpcMethods?: DashboardContractRpcMethodOverrides,
  desktopLocalPath?: DashboardContractDesktopLocalPathOverrides,
  desktopHost?: DashboardContractDesktopHostOverrides,
  windowController?: DashboardContractWindowControllerOverrides,
  windowApi?: DashboardContractWindowApiOverrides,
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

      const sourceBasePath = resolve(desktopRoot, "src", modulePath);
      const sourceCandidates = [
        `${sourceBasePath}.ts`,
        `${sourceBasePath}.tsx`,
        resolve(sourceBasePath, "index.ts"),
        resolve(sourceBasePath, "index.tsx"),
      ];

      for (const candidate of sourceCandidates) {
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

    if (request === "@tauri-apps/api/core") {
      return {
        invoke:
          desktopHost?.invoke ??
          (() => Promise.reject(new Error("invoke should not run in dashboard contract tests"))),
      };
    }

    if (request === "@tauri-apps/api/window") {
      return {
        getCurrentWindow:
          windowApi?.getCurrentWindow ??
          (() => ({
            label: "dashboard",
            emit: () => Promise.resolve(),
            emitTo: () => Promise.resolve(),
          })),
      };
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
        getDashboardModule:
          rpcMethods?.getDashboardModule ??
          (() => Promise.reject(new Error("getDashboardModule should not run in dashboard contract tests"))),
        getDashboardOverview:
          rpcMethods?.getDashboardOverview ??
          (() => Promise.reject(new Error("getDashboardOverview should not run in dashboard contract tests"))),
        getMirrorOverview:
          rpcMethods?.getMirrorOverview ??
          (() => Promise.reject(new Error("getMirrorOverview should not run in dashboard contract tests"))),
        getRecommendations:
          rpcMethods?.getRecommendations ??
          (() => Promise.reject(new Error("getRecommendations should not run in dashboard contract tests"))),
        getMirrorOverviewDetailed:
          rpcMethods?.getMirrorOverviewDetailed ??
          (() => Promise.reject(new Error("getMirrorOverviewDetailed should not run in dashboard contract tests"))),
        getSecuritySummaryDetailed:
          rpcMethods?.getSecuritySummaryDetailed ??
          (() => Promise.reject(new Error("getSecuritySummaryDetailed should not run in dashboard contract tests"))),
        getSettings:
          rpcMethods?.getSettings ??
          (() => Promise.reject(new Error("getSettings should not run in dashboard contract tests"))),
        listSecurityPendingDetailed:
          rpcMethods?.listSecurityPendingDetailed ??
          (() => Promise.reject(new Error("listSecurityPendingDetailed should not run in dashboard contract tests"))),
        listNotepad:
          rpcMethods?.listNotepad ??
          (() => {
            throw new Error("listNotepad should not run in dashboard contract tests");
          }),
        listSecurityAuditDetailed:
          rpcMethods?.listSecurityAuditDetailed ??
          (() => Promise.reject(new Error("listSecurityAuditDetailed should not run in dashboard contract tests"))),
        listSecurityRestorePointsDetailed:
          rpcMethods?.listSecurityRestorePointsDetailed ??
          (() => Promise.reject(new Error("listSecurityRestorePointsDetailed should not run in dashboard contract tests"))),
        listTaskArtifacts:
          rpcMethods?.listTaskArtifacts ??
          (() => {
            throw new Error("listTaskArtifacts should not run in dashboard contract tests");
          }),
        listTasks:
          rpcMethods?.listTasks ??
          (() => {
            throw new Error("listTasks should not run in dashboard contract tests");
          }),
        openDelivery:
          rpcMethods?.openDelivery ??
          (() => {
            throw new Error("openDelivery should not run in dashboard contract tests");
          }),
        openTaskArtifact:
          rpcMethods?.openTaskArtifact ??
          (() => {
            throw new Error("openTaskArtifact should not run in dashboard contract tests");
          }),
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

    if (request === "@/platform/windowController") {
      return {
        openOrFocusDesktopWindow:
          windowController?.openOrFocusDesktopWindow ??
          (() => Promise.resolve("dashboard")),
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
  assert.deepEqual(buildDashboardTaskDetailQueryKey("rpc", "task_dashboard_001"), ["dashboard", "tasks", "detail", "rpc", "task_dashboard_001"]);
  assert.deepEqual(getDashboardTaskSecurityRefreshPlan("rpc"), {
    invalidatePrefixes: [
      ["dashboard", "tasks", "bucket"],
      ["dashboard", "tasks", "detail"],
    ],
    refetchOnMount: true,
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
  assert.deepEqual(buildDashboardNoteBucketInvalidateKeys("rpc", ["upcoming", "closed", "upcoming"]), [
    ["dashboard", "notes", "bucket", "rpc", "upcoming"],
    ["dashboard", "notes", "bucket", "rpc", "closed"],
  ]);
  assert.deepEqual(getDashboardNoteRefreshPlan("rpc"), {
    invalidatePrefixes: [["dashboard", "notes", "bucket"]],
    refetchOnMount: true,
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

test("dashboard root no longer falls back to mock home data when the live query is unavailable", () => {
  const dashboardRootSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardRoot.tsx"), "utf8");
  const dashboardHomeServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts"), "utf8");

  assert.doesNotMatch(dashboardRootSource, /getDashboardHomeFallbackData/);
  assert.match(dashboardRootSource, /const dashboardHomeData = dashboardHomeQuery\.data \?\? null;/);
  assert.match(dashboardRootSource, /DashboardHomeStatusShell/);
  assert.match(dashboardRootSource, /sequences=\{dashboardHomeData\?\.voiceSequences \?\? \[\]\}/);
  assert.match(dashboardRootSource, /dashboardHomeStatusShellModules/);
  assert.match(dashboardRootSource, /to=\{module\.route\}/);
  assert.doesNotMatch(dashboardRootSource, /clearDashboardResultPageRecoveryForSearch/);
  assert.doesNotMatch(dashboardHomeServiceSource, /export function getDashboardHomeFallbackData/);
  assert.match(dashboardHomeServiceSource, /Promise\.allSettled/);
});

test("dashboard home no longer replays mock summon or voice presets when live recommendations are empty", () => {
  const dashboardHomeSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardHome.tsx"), "utf8");
  const dashboardHomeServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts"), "utf8");

  assert.doesNotMatch(dashboardHomeServiceSource, /dashboardHome\.mocks/);
  assert.doesNotMatch(dashboardHomeServiceSource, /return templates.length > 0 \? templates : dashboardSummonTemplates\.map/);
  assert.doesNotMatch(dashboardHomeServiceSource, /return sequences.length > 0 \? sequences : dashboardVoiceSequences\.map/);
  assert.match(dashboardHomeSource, /if \(data\.summonTemplates\.length === 0\) \{/);
  assert.match(dashboardHomeSource, /data\.loadWarnings\.length > 0/);
});

test("dashboard event panel routes task-detail actions through the shared navigation helper", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/components/DashboardEventPanel.tsx"), "utf8");

  assert.match(panelSource, /navigateToDashboardTaskDetail/);
  assert.match(panelSource, /target\?\.kind === "task_detail"/);
  assert.match(panelSource, /target\?\.kind === "mirror_detail"/);
  assert.match(panelSource, /activeDetailKey: target\.activeDetailKey/);
  assert.match(panelSource, /resolvePrimaryActionLabel/);
  assert.match(panelSource, /activeState\.navigationTarget\?\.label/);
  assert.match(panelSource, /filterDistinctContextItems/);
  assert.match(panelSource, /filterDistinctSignals/);
  assert.match(panelSource, /buildMetaPills/);
  assert.doesNotMatch(panelSource, /这是首页事件舱/);
});

test("dashboard home randomizes summons while preferring a different module when alternatives exist", () => {
  const dashboardHomeSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardHome.tsx"), "utf8");

  assert.match(dashboardHomeSource, /function pickNextSummonIndex\(/);
  assert.match(dashboardHomeSource, /function buildSummonTemplateSignature\(/);
  assert.match(dashboardHomeSource, /function buildNavigationTargetSignature/);
  assert.match(dashboardHomeSource, /if \(previousIndex < 0 \|\| previousModule === null\) \{/);
  assert.match(dashboardHomeSource, /return 0;/);
  assert.match(dashboardHomeSource, /candidate\.module !== previousModule/);
  assert.match(dashboardHomeSource, /const pool = candidateIndexes\.length > 0 \? candidateIndexes : fallbackIndexes/);
  assert.match(dashboardHomeSource, /Math\.floor\(Math\.random\(\) \* pool\.length\)/);
  assert.match(dashboardHomeSource, /lastSummonModuleRef\.current = template\.module/);
  assert.match(dashboardHomeSource, /const summonTemplatesRef = useRef\(data\.summonTemplates\)/);
  assert.match(dashboardHomeSource, /const summonTemplateSignature = buildSummonTemplateSignature\(data\.summonTemplates\)/);
  assert.match(dashboardHomeSource, /const templates = summonTemplatesRef\.current/);
  assert.match(dashboardHomeSource, /summonTemplatesRef\.current = data\.summonTemplates/);
  assert.match(dashboardHomeSource, /target\.taskId/);
  assert.match(dashboardHomeSource, /target\.focusMemoryId \?\? ""/);
  assert.match(dashboardHomeSource, /buildNavigationTargetSignature\(template\.expandedState\?\.navigationTarget\)/);
  assert.match(dashboardHomeSource, /\}, \[data\.summonTemplates\.length, scheduleSummon, summonTemplateSignature\]\);/);
  assert.match(dashboardHomeSource, /const closeActiveOverlay = useCallback\(\(\) => \{/);
  assert.match(dashboardHomeSource, /if \(event\.key === "Escape" && \(activeStateKey \|\| activeExpandedState\)\) \{/);
  assert.match(dashboardHomeSource, /onClose=\{closeActiveOverlay\}/);
});

test("dashboard result page only embeds trusted loopback shell origins and auto-opens browser fallback", () => {
  const resultPageSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardResultPage.tsx"), "utf8");

  assert.match(resultPageSource, /function isTrustedDashboardResultPageOrigin/);
  assert.match(resultPageSource, /const trustedDashboardResultPageHosts = new Set\(/);
  assert.match(resultPageSource, /"desktop\.local"/);
  assert.match(resultPageSource, /trustedDashboardResultPageHosts\.has\(currentOriginUrl\.hostname\)/);
  assert.match(resultPageSource, /return url\.origin === currentOriginUrl\.origin/);
  assert.match(resultPageSource, /window\.open\(resultUrl, "_blank", "noopener,noreferrer"\)/);
  assert.match(resultPageSource, /browserFallbackOpenedRef/);
  assert.match(resultPageSource, /不在站内可信嵌入白名单内，已切换为浏览器承接模式/);
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
  const { canTaskAcceptSteering, getTaskPrimaryActions, getTaskRunwayTone } = loadTaskPageMapperModule();
  const confirmingIntentTask = createTask({ status: "confirming_intent", current_step: "intent_confirmation", intent: { name: "summarize", arguments: {} } });
  const waitingAuthTask = createTask({ status: "waiting_auth" });
  const waitingInputTask = createTask({ status: "waiting_input" });
  const processingPromptTask = createTask({ status: "processing", current_step: "generate_output", intent: { name: "agent_loop", arguments: {} } });
  const processingLoopTask = createTask({ status: "processing", current_step: "agent_loop", intent: { name: "agent_loop", arguments: {} } });
  const mapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.mapper.ts"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const taskServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.service.ts"), "utf8");
  const taskDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.equal(canTaskAcceptSteering(confirmingIntentTask), false);
  assert.equal(canTaskAcceptSteering(waitingAuthTask), true);
  assert.equal(canTaskAcceptSteering(waitingInputTask), false);
  assert.equal(canTaskAcceptSteering(processingPromptTask), false);
  assert.equal(canTaskAcceptSteering(processingLoopTask), true);
  assert.equal(getTaskRunwayTone(confirmingIntentTask.status), "holding");
  assert.equal(getTaskRunwayTone(waitingAuthTask.status), "holding");
  assert.equal(getTaskRunwayTone(processingPromptTask.status), "departure");
  assert.deepEqual(
    getTaskPrimaryActions(confirmingIntentTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: confirmingIntentTask })).map((action) => action.action),
    ["cancel", "open-safety"],
  );
  assert.equal(getTaskPrimaryActions(waitingAuthTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: waitingAuthTask })).at(-1)?.label, "安全详情");
  assert.deepEqual(
    getTaskPrimaryActions(waitingInputTask, createDetail({ approval_request: null, security_summary: { latest_restore_point: null, pending_authorizations: 0, risk_level: "yellow", security_status: "normal" }, task: waitingInputTask })).map((action) => action.action),
    ["cancel", "open-safety"],
  );
  assert.match(mapperSource, /title: "等待确认"/);
  assert.match(taskServiceSource, /等待确认当前处理方式后继续执行。/);
  assert.doesNotMatch(mapperSource, /当前任务还在等待补充输入，如需修改或补充，请到悬浮球继续处理。/);
  assert.match(taskPageSource, /getTaskRunwayTone\(item\.task\.status\) === "departure"/);
  assert.match(taskPageSource, /getTaskRunwayTone\(item\.task\.status\) === "holding"/);
  assert.doesNotMatch(taskPageSource, /item\.task\.status === "confirming_intent" \|\| item\.task\.status === "processing"/);
  assert.match(taskPageSource, /onSteerTask=\{handleSteerTask\}/);
  assert.match(taskDetailPanelSource, /const canSteerTask = task \? canTaskAcceptSteering\(task\) : false;/);
  assert.match(taskDetailPanelSource, /当前任务仍在等待确认处理方式；确认后才会开放正式 `agent\.task\.steer` 追加要求。/);
  assert.match(taskDetailPanelSource, /当前任务还在等待确认处理方式，确认后才能继续追加要求。/);
  assert.match(taskDetailPanelSource, /placeholder=\{steeringPlaceholder\}/);
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

test("settings service falls back to neutral placeholders before runtime hydration", () => {
  const { loadSettings } = loadSettingsServiceModule();
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
    const loaded = loadSettings();
    assert.equal(loaded.settings.general.download.workspace_path, "workspace");
    assert.deepEqual(loaded.settings.task_automation.task_sources, ["workspace/todos"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service hydrates runtime defaults before loading fallback snapshots", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const settingsService = loadSettingsServiceModule({
      invoke: async (command) => {
        assert.equal(command, "desktop_get_runtime_defaults");
        return {
          workspace_path: "/Users/runtime/CialloClaw/workspace",
          task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
        };
      },
    });
    const hydrated = await settingsService.loadHydratedSettings();

    assert.equal(hydrated.settings.general.download.workspace_path, "/Users/runtime/CialloClaw/workspace");
    assert.deepEqual(hydrated.settings.task_automation.task_sources, ["/Users/runtime/CialloClaw/workspace/todos"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service exposes the trusted runtime data directory snapshot", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const settingsService = loadSettingsServiceModule({
      invoke: async (command) => {
        assert.equal(command, "desktop_get_runtime_defaults");
        return {
          data_path: "/Users/runtime/CialloClaw/data",
          workspace_path: "/Users/runtime/CialloClaw/workspace",
          task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
        };
      },
    });

    const runtimeDefaults = await settingsService.loadDesktopRuntimeDefaultsSnapshot();

    assert.deepEqual(runtimeDefaults, {
      data_path: "/Users/runtime/CialloClaw/data",
      workspace_path: "/Users/runtime/CialloClaw/workspace",
      task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
    });
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service rejects cached runtime workspace snapshots when host hydration fails", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.runtime-defaults",
      JSON.stringify({
        workspace_path: "/cached/runtime/workspace",
        task_sources: ["/cached/runtime/workspace/todos"],
      }),
    );
    const settingsService = loadSettingsServiceModule({
      invoke: async () => {
        throw new Error("desktop runtime defaults unavailable");
      },
    });

    const runtimeDefaults = await settingsService.loadDesktopRuntimeDefaultsSnapshot();

    assert.equal(runtimeDefaults, null);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service loadHydratedSettings keeps existing snapshot when host hydration fails", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          general: {
            download: {
              workspace_path: "/cached/workspace",
            },
          },
          task_automation: {
            task_sources: ["/cached/workspace/todos"],
          },
        },
      }),
    );
    const settingsService = loadSettingsServiceModule({
      invoke: async () => {
        throw new Error("desktop runtime defaults unavailable");
      },
    });

    const hydrated = await settingsService.loadHydratedSettings();
    assert.equal(hydrated.settings.general.download.workspace_path, "/cached/workspace");
    assert.deepEqual(hydrated.settings.task_automation.task_sources, ["/cached/workspace/todos"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service preserves user-owned workspace-relative task sources during runtime hydration", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          general: {
            download: {
              workspace_path: "workspace",
            },
          },
          task_automation: {
            task_sources: ["workspace/review"],
          },
        },
      }),
    );
    const settingsService = loadSettingsServiceModule({
      invoke: async () => ({
        workspace_path: "/Users/runtime/CialloClaw/workspace",
        task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
      }),
    });

    const hydrated = await settingsService.loadHydratedSettings();
    assert.equal(hydrated.settings.general.download.workspace_path, "/Users/runtime/CialloClaw/workspace");
    assert.deepEqual(hydrated.settings.task_automation.task_sources, ["workspace/review"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service preserves multi-root workspace-relative task sources during runtime hydration", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          general: {
            download: {
              workspace_path: "workspace",
            },
          },
          task_automation: {
            task_sources: ["workspace/backlog", "workspace/review"],
          },
        },
      }),
    );
    const settingsService = loadSettingsServiceModule({
      invoke: async () => ({
        workspace_path: "/Users/runtime/CialloClaw/workspace",
        task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
      }),
    });

    const hydrated = await settingsService.loadHydratedSettings();
    assert.deepEqual(hydrated.settings.task_automation.task_sources, ["workspace/backlog", "workspace/review"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("settings service rewrites only the legacy single-root task source placeholder during runtime hydration", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          task_automation: {
            task_sources: ["workspace/todos"],
          },
        },
      }),
    );
    const settingsService = loadSettingsServiceModule({
      invoke: async () => ({
        workspace_path: "/Users/runtime/CialloClaw/workspace",
        task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
      }),
    });

    const hydrated = await settingsService.loadHydratedSettings();
    assert.deepEqual(hydrated.settings.task_automation.task_sources, ["/Users/runtime/CialloClaw/workspace/todos"]);

    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          task_automation: {
            task_sources: ["d:/workspace/todos"],
          },
        },
      }),
    );
    const rewrittenWindowsLegacy = await settingsService.loadHydratedSettings();
    assert.deepEqual(rewrittenWindowsLegacy.settings.task_automation.task_sources, ["/Users/runtime/CialloClaw/workspace/todos"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config prefers hydrated unix task sources over legacy workspace snapshots", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const syncedTaskSources: string[][] = [];
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule(
      {
        getTaskInspectorConfig: async () => ({
          task_sources: ["workspace/todos"],
        }),
      },
      {
        invoke: async (command, args) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "/Users/runtime/CialloClaw/workspace",
              task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            const settings = args?.settings as { task_automation?: { task_sources?: string[] } } | undefined;
            syncedTaskSources.push(settings?.task_automation?.task_sources ?? []);
            return undefined;
          }

          throw new Error(`unexpected desktop command: ${command}`);
        },
      },
    );

    const config = await loadNoteSourceConfig();
    assert.deepEqual(config.task_sources, ["/Users/runtime/CialloClaw/workspace/todos"]);
    assert.deepEqual(syncedTaskSources, [["/Users/runtime/CialloClaw/workspace/todos"]]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config keeps remote task sources when cached settings are not absolute", async () => {
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
          task_automation: {
            task_sources: ["workspace/todos"],
          },
        },
      }),
    );
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule({
      getTaskInspectorConfig: async () => ({
        task_sources: ["workspace/review"],
      }),
    });

    const config = await loadNoteSourceConfig();
    assert.deepEqual(config.task_sources, ["workspace/review"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config keeps remote task sources when cached settings are explicitly empty", async () => {
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
          task_automation: {
            task_sources: [],
          },
        },
      }),
    );
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule({
      getTaskInspectorConfig: async () => ({
        task_sources: ["workspace/review"],
      }),
    });

    const config = await loadNoteSourceConfig();
    assert.deepEqual(config.task_sources, ["workspace/review"]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config syncs resolved task sources into the desktop host cache", async () => {
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
  const syncedTaskSources: string[][] = [];

  Object.assign(globalThis, {
    window: {
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule(
      {
        getTaskInspectorConfig: async () => ({
          task_sources: ["workspace/review"],
        }),
      },
      {
        invoke: async (command, args) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "/Users/runtime/CialloClaw/workspace",
              task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            const settings = args?.settings as { task_automation?: { task_sources?: string[] } } | undefined;
            syncedTaskSources.push(settings?.task_automation?.task_sources ?? []);
            return undefined;
          }

          throw new Error(`unexpected desktop command: ${command}`);
        },
      },
    );

    const config = await loadNoteSourceConfig();
    assert.deepEqual(config.task_sources, ["workspace/review"]);
    assert.deepEqual(syncedTaskSources, [["workspace/review"]]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config surfaces rpc transport failures with the localized retry copy", async () => {
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
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule({
      getTaskInspectorConfig: async () => {
        throw new Error("transport is not wired");
      },
    });

    await assert.rejects(loadNoteSourceConfig(), /当前无法读取任务来源配置，请稍后重试。/);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config falls back to cached task sources when the backend rejects a missing default source", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const syncedTaskSources: string[][] = [];
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule(
      {
        getTaskInspectorConfig: async () => {
          throw new Error("task inspection source not found: /Users/runtime/CialloClaw/workspace/todos");
        },
      },
      {
        invoke: async (command, args) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "/Users/runtime/CialloClaw/workspace",
              task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            const settings = args?.settings as { task_automation?: { task_sources?: string[] } } | undefined;
            syncedTaskSources.push(settings?.task_automation?.task_sources ?? []);
            return undefined;
          }

          throw new Error(`unexpected desktop command: ${command}`);
        },
      },
    );

    const config = await loadNoteSourceConfig();
    assert.deepEqual(config.task_sources, ["/Users/runtime/CialloClaw/workspace/todos"]);
    assert.deepEqual(syncedTaskSources, [["/Users/runtime/CialloClaw/workspace/todos"]]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source config prefers cached task sources when the backend returns an empty list", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const syncedTaskSources: string[][] = [];
    const { loadNoteSourceConfig } = loadNoteSourceServiceModule(
      {
        getTaskInspectorConfig: async () => ({
          task_sources: [],
        }),
      },
      {
        invoke: async (command, args) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "/Users/runtime/CialloClaw/workspace",
              task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            const settings = args?.settings as { task_automation?: { task_sources?: string[] } } | undefined;
            syncedTaskSources.push(settings?.task_automation?.task_sources ?? []);
            return undefined;
          }

          throw new Error(`unexpected desktop command: ${command}`);
        },
      },
    );

    const config = await loadNoteSourceConfig();
    assert.deepEqual(config.task_sources, ["/Users/runtime/CialloClaw/workspace/todos"]);
    assert.deepEqual(syncedTaskSources, [["/Users/runtime/CialloClaw/workspace/todos"]]);
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source snapshot and index retry with runtime defaults after stale source path failures", async () => {
  const originalWindow = globalThis.window;
  const storage = new Map<string, string>();
  const staleSource = "C:/Users/33721/AppData/Local/CialloClaw/workspace/todos";
  const runtimeSource = "D:/runtime/CialloClaw/workspace/todos";
  const syncedTaskSources: string[][] = [];
  const snapshotCalls: string[][] = [];
  const indexCalls: string[][] = [];
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          task_automation: {
            task_sources: [staleSource],
          },
        },
      }),
    );

    const { loadNoteSourceIndex, loadNoteSourceSnapshot } = loadNoteSourceServiceModule(
      undefined,
      {
        invoke: async (command, args) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "D:/runtime/CialloClaw/workspace",
              task_sources: [runtimeSource],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            const settings = args?.settings as { task_automation?: { task_sources?: string[] } } | undefined;
            syncedTaskSources.push(settings?.task_automation?.task_sources ?? []);
            return undefined;
          }

          if (command === "desktop_load_source_notes") {
            const sources = (args?.sources as string[] | undefined) ?? [];
            snapshotCalls.push(sources);
            if (sources[0] === staleSource) {
              throw new Error(`task inspection source not found: ${staleSource}`);
            }

            return {
              default_source_root: runtimeSource,
              notes: [
                {
                  content: "Runtime note",
                  file_name: "notes.md",
                  modified_at_ms: 123,
                  path: `${runtimeSource}/notes.md`,
                  source_root: runtimeSource,
                  title: "notes",
                },
              ],
              source_roots: sources,
            };
          }

          if (command === "desktop_load_source_note_index") {
            const sources = (args?.sources as string[] | undefined) ?? [];
            indexCalls.push(sources);
            if (sources[0] === staleSource) {
              throw new Error(`task inspection source not found: ${staleSource}`);
            }

            return {
              default_source_root: runtimeSource,
              notes: [
                {
                  file_name: "notes.md",
                  modified_at_ms: 123,
                  path: `${runtimeSource}/notes.md`,
                  size_bytes: 64,
                  source_root: runtimeSource,
                },
              ],
              source_roots: sources,
            };
          }

          throw new Error(`unexpected desktop command: ${command}`);
        },
      },
    );

    const snapshot = await loadNoteSourceSnapshot([staleSource]);
    const index = await loadNoteSourceIndex([staleSource]);
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.deepEqual(snapshotCalls, [[staleSource], [runtimeSource]]);
    assert.deepEqual(indexCalls, [[staleSource], [runtimeSource]]);
    assert.equal(snapshot.defaultSourceRoot, runtimeSource);
    assert.deepEqual(snapshot.sourceRoots, [runtimeSource]);
    assert.equal(snapshot.notes[0]?.path, `${runtimeSource}/notes.md`);
    assert.equal(index.defaultSourceRoot, runtimeSource);
    assert.deepEqual(index.sourceRoots, [runtimeSource]);
    assert.equal(index.notes[0]?.sourceRoot, runtimeSource);

    const savedSettings = JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}") as {
      settings?: { task_automation?: { task_sources?: string[] } };
    };
    assert.deepEqual(savedSettings.settings?.task_automation?.task_sources, [runtimeSource]);
    assert.equal(
      syncedTaskSources.some((sources) => JSON.stringify(sources) === JSON.stringify([runtimeSource])),
      true,
    );
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("note source create, save, and inspection retry with runtime defaults after stale source path failures", async () => {
  const originalWindow = globalThis.window;
  const storage = new Map<string, string>();
  const staleSource = "C:/Users/33721/AppData/Local/CialloClaw/workspace/todos";
  const runtimeSource = "D:/runtime/CialloClaw/workspace/todos";
  const createCalls: string[][] = [];
  const saveCalls: string[][] = [];
  const inspectionCalls: string[][] = [];
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          task_automation: {
            task_sources: [staleSource],
          },
        },
      }),
    );

    const { createNoteSource, runNoteSourceInspection, saveNoteSource } = loadNoteSourceServiceModule(
      {
        runTaskInspector: async (params) => {
          const request = params as { reason: string; target_sources: string[] };
          inspectionCalls.push(request.target_sources);
          if (request.target_sources[0] === staleSource) {
            throw new Error(`task inspection source not found: ${staleSource}`);
          }

          return {
            accepted_sources: request.target_sources,
            ok: true,
            reason: request.reason,
          };
        },
      },
      {
        invoke: async (command, args) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "D:/runtime/CialloClaw/workspace",
              task_sources: [runtimeSource],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            return undefined;
          }

          if (command === "desktop_create_source_note") {
            const sources = (args?.sources as string[] | undefined) ?? [];
            createCalls.push(sources);
            if (sources[0] === staleSource) {
              throw new Error(`task inspection source not found: ${staleSource}`);
            }

            return {
              content: args?.content,
              file_name: "notes.md",
              modified_at_ms: 456,
              path: `${runtimeSource}/notes.md`,
              source_root: runtimeSource,
              title: "notes",
            };
          }

          if (command === "desktop_save_source_note") {
            const sources = (args?.sources as string[] | undefined) ?? [];
            saveCalls.push(sources);
            if (sources[0] === staleSource) {
              throw new Error(`task inspection source not found: ${staleSource}`);
            }

            return {
              content: args?.content,
              file_name: "notes.md",
              modified_at_ms: 789,
              path: args?.path,
              source_root: runtimeSource,
              title: "notes",
            };
          }

          throw new Error(`unexpected desktop command: ${command}`);
        },
      },
    );

    const createdNote = await createNoteSource([staleSource], "New note");
    const savedNote = await saveNoteSource([staleSource], `${runtimeSource}/notes.md`, "Updated note");
    const inspectionResult = await runNoteSourceInspection([staleSource], "manual refresh");

    assert.deepEqual(createCalls, [[staleSource], [runtimeSource]]);
    assert.deepEqual(saveCalls, [[staleSource], [runtimeSource]]);
    assert.deepEqual(inspectionCalls, [[staleSource], [runtimeSource]]);
    assert.equal(createdNote.sourceRoot, runtimeSource);
    assert.equal(createdNote.content, "New note");
    assert.equal(savedNote.path, `${runtimeSource}/notes.md`);
    assert.equal(savedNote.content, "Updated note");
    assert.deepEqual(inspectionResult, {
      accepted_sources: [runtimeSource],
      ok: true,
      reason: "manual refresh",
    });

    const savedSettings = JSON.parse(localStorage.getItem("cialloclaw.settings") ?? "{}") as {
      settings?: { task_automation?: { task_sources?: string[] } };
    };
    assert.deepEqual(savedSettings.settings?.task_automation?.task_sources, [runtimeSource]);
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
    localDataPath: null,
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

test("control panel about snapshot reuses the trusted runtime data directory", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const { loadControlPanelAboutSnapshot } = loadControlPanelAboutServiceModule({
      invoke: async (command) => {
        assert.equal(command, "desktop_get_runtime_defaults");
        return {
          data_path: "/Users/runtime/CialloClaw/data",
          workspace_path: "/Users/runtime/CialloClaw/workspace",
          task_sources: ["/Users/runtime/CialloClaw/workspace/todos"],
        };
      },
    });

    const snapshot = await loadControlPanelAboutSnapshot();

    assert.equal(snapshot.appName, "CialloClaw");
    assert.equal(snapshot.appVersion, "0.1.0");
    assert.equal(snapshot.localDataPath, "/Users/runtime/CialloClaw/data");
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("control panel about helpers open the data directory and share links", async () => {
  const originalWindowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  const originalNavigatorDescriptor = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  const copiedValues: string[] = [];
  const invokedCommands: string[] = [];

  const { copyControlPanelAboutValue, runControlPanelAboutAction } = loadControlPanelAboutServiceModule({
    invoke: async (command) => {
      invokedCommands.push(command);
      return undefined;
    },
  });

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

  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      __TAURI_INTERNALS__: {},
    },
  });

  try {
    const feedbackCopy = await copyControlPanelAboutValue("https://github.com/1024XEngineer/CialloClaw/issues", "已复制反馈渠道链接。");
    const openFeedback = await runControlPanelAboutAction("open_data_directory");
    const shareFeedback = await runControlPanelAboutAction("share");

    assert.equal(feedbackCopy, "已复制反馈渠道链接。");
    assert.equal(openFeedback, "已在系统中打开本地存储目录。");
    assert.equal(shareFeedback, "已复制分享链接。");
    assert.deepEqual(invokedCommands, ["desktop_open_runtime_data_path"]);
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
  assert.match(controlPanelAppSource, /title="本地存储位置"/);
  assert.match(controlPanelAppSource, /title="帮助与反馈"/);
  assert.match(controlPanelAppSource, /title="版本信息"/);
  assert.match(controlPanelAppSource, /title="恢复默认设置"/);
  assert.match(controlPanelAppSource, /数据目录/);
  assert.match(controlPanelAppSource, /打开目录/);
  assert.match(controlPanelAppSource, /恢复默认设置/);
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
  assert.match(controlPanelAppSource, /const localDataPath = normalizeDisplayPath\(aboutSnapshot\.localDataPath \?\? ""\);/);
  assert.match(controlPanelAppSource, /handleAboutAction\("open_data_directory"\)/);
  assert.match(controlPanelAppSource, /const \[isRestoreDefaultsConfirming, setIsRestoreDefaultsConfirming\] = useState\(false\);/);
  assert.match(controlPanelAppSource, /const restoreDraft = buildControlPanelRestoreDefaultsData\(draft, persistedPanelData\);/);
  assert.match(controlPanelAppSource, /validateModel: false/);
  assert.match(controlPanelAppSource, /不会删除任务历史、记忆内容、本地文件/);
  assert.match(controlPanelAppSource, /恢复默认设置/);
  assert.match(controlPanelAppSource, /aboutActionFeedback \? \([\s\S]*aria-live="polite"[\s\S]*\{aboutActionFeedback\}/);
  assert.match(controlPanelAppSource, /const settings = \(await loadHydratedSettings\(\)\)\.settings;/);
  assert.match(controlPanelAppSource, /const fallbackData = await buildLocalControlPanelSnapshot\(\);/);
});

test("dashboard settings mutation persists rpc-effective settings into the local snapshot", async () => {
  const { loadSettings } = loadSettingsServiceModule();
  const { updateDashboardSettings } = loadDashboardSettingsMutationModule({
    updateSettings: async () => ({
      apply_mode: "immediate",
      need_restart: false,
      updated_keys: ["general.download.ask_before_save_each_file", "memory.enabled", "memory.lifecycle", "models.budget_auto_downgrade"],
      effective_settings: {
        general: {
          download: {
            ask_before_save_each_file: false,
          },
        },
        memory: {
          enabled: false,
          lifecycle: "session",
        },
        models: {
          budget_auto_downgrade: false,
        },
      },
    }),
    getSettingsDetailed: async () => ({
      data: {
        settings: {
          general: {
            download: {
              ask_before_save_each_file: false,
            },
          },
          memory: {
            enabled: false,
            lifecycle: "session",
          },
          models: {
            budget_auto_downgrade: false,
          },
        },
      },
      meta: {
        server_time: "2026-04-28T09:30:00Z",
      },
      warnings: [],
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
    const result = await updateDashboardSettings({
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
    });
    assert.equal(result.source, "rpc");
    assert.equal(result.applyMode, "immediate");
    assert.equal(result.needRestart, false);
    assert.equal(result.persisted, true);
    assert.equal(result.readbackWarning, null);
    assert.deepEqual(result.updatedKeys.sort(), [
      "general.download.ask_before_save_each_file",
      "memory.enabled",
      "memory.lifecycle",
      "models.budget_auto_downgrade",
    ]);
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
test("control panel workspace section opens the trusted runtime directory instead of editing draft paths", () => {
  const controlPanelAppSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");
  assert.match(controlPanelAppSource, /loadDesktopRuntimeDefaultsSnapshot/);
  assert.match(controlPanelAppSource, /openDesktopRuntimeWorkspaceDirectory/);
  assert.match(controlPanelAppSource, /const handleOpenCurrentWorkspaceDirectory = async \(\) =>/);
  assert.match(controlPanelAppSource, /await openDesktopRuntimeWorkspaceDirectory\(\);/);
  assert.match(controlPanelAppSource, /runtimeWorkspacePathLabel/);
  assert.match(controlPanelAppSource, /打开当前目录/);
  assert.doesNotMatch(controlPanelAppSource, /value=\{draft\.settings\.general\.download\.workspace_path\}/);
  assert.doesNotMatch(controlPanelAppSource, /workspace_path: event\.target\.value/);
});
test("control panel keeps budget rows in the safety page instead of duplicating them", () => {
  const controlPanelAppSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");
  assert.match(controlPanelAppSource, /title="模型与安全摘要"/);
  assert.match(controlPanelAppSource, /label="安全状态"/);
  assert.match(controlPanelAppSource, /label="待确认授权"/);
  assert.doesNotMatch(controlPanelAppSource, /label="今日成本"/);
  assert.doesNotMatch(controlPanelAppSource, /label="单任务上限"/);
  assert.doesNotMatch(controlPanelAppSource, /label="当日上限"/);
});
test("control panel restore-default helper preserves the persisted workspace, task-source, and model-route boundaries", () => {
  const { buildControlPanelRestoreDefaultsData } = loadControlPanelServiceModule();
  const persisted: Parameters<typeof buildControlPanelRestoreDefaultsData>[1] = {
    source: "rpc",
    providerApiKeyInput: "",
    settings: {
      general: {
        language: "en-US",
        auto_launch: false,
        theme_mode: "dark",
        voice_notification_enabled: false,
        voice_type: "custom_voice",
        download: {
          workspace_path: "D:/SavedWorkspace",
          ask_before_save_each_file: false,
        },
      },
      floating_ball: {
        auto_snap: false,
        idle_translucent: false,
        position_mode: "fixed",
        size: "large",
      },
      memory: {
        enabled: false,
        lifecycle: "7d",
        work_summary_interval: {
          unit: "hour",
          value: 4,
        },
        profile_refresh_interval: {
          unit: "day",
          value: 3,
        },
      },
      task_automation: {
        task_sources: ["D:/saved-todos"],
        inspection_interval: {
          unit: "hour",
          value: 2,
        },
        inspect_on_file_change: false,
        inspect_on_startup: false,
        remind_before_deadline: false,
        remind_when_stale: true,
      },
      models: {
        provider: "anthropic",
        provider_api_key_configured: true,
        budget_auto_downgrade: false,
        base_url: "https://api.anthropic.com",
        model: "claude-3-7-sonnet",
        stronghold: {
          backend: "stronghold",
          available: true,
          fallback: false,
          initialized: true,
          formal_store: true,
        },
      },
    },
    inspector: {
      task_sources: ["D:/saved-todos"],
      inspection_interval: {
        unit: "hour",
        value: 2,
      },
      inspect_on_file_change: false,
      inspect_on_startup: false,
      remind_before_deadline: false,
      remind_when_stale: true,
    },
    securitySummary: {
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
        budget_auto_downgrade: false,
      },
    },
    warnings: ["stale warning"],
  };
  const draft: Parameters<typeof buildControlPanelRestoreDefaultsData>[0] = {
    ...persisted,
    providerApiKeyInput: "sk-unsaved-secret",
    settings: {
      ...persisted.settings,
      general: {
        ...persisted.settings.general,
        download: {
          ...persisted.settings.general.download,
          workspace_path: "D:/UnsavedWorkspace",
        },
      },
      task_automation: {
        ...persisted.settings.task_automation,
        task_sources: ["D:/unsaved-todos"],
      },
      models: {
        ...persisted.settings.models,
        provider: "openai-compatible",
        base_url: "https://draft.example.com/v1",
        model: "draft-model",
      },
    },
    inspector: {
      ...persisted.inspector,
      task_sources: ["D:/unsaved-todos"],
    },
  };
  const restored = buildControlPanelRestoreDefaultsData(
    draft,
    persisted,
  );
  assert.equal(restored.providerApiKeyInput, "");
  assert.equal(restored.settings.general.language, "zh-CN");
  assert.equal(restored.settings.general.download.workspace_path, "D:/SavedWorkspace");
  assert.equal(restored.settings.general.download.ask_before_save_each_file, true);
  assert.equal(restored.settings.floating_ball.size, "medium");
  assert.equal(restored.settings.memory.enabled, true);
  assert.equal(restored.settings.memory.lifecycle, "30d");
  assert.equal(restored.settings.models.provider, "anthropic");
  assert.equal(restored.settings.models.base_url, "https://api.anthropic.com");
  assert.equal(restored.settings.models.model, "claude-3-7-sonnet");
  assert.equal(restored.settings.models.budget_auto_downgrade, true);
  assert.equal(restored.settings.models.provider_api_key_configured, true);
  assert.deepEqual(restored.settings.models.stronghold, {
    backend: "stronghold",
    available: true,
    fallback: false,
    initialized: true,
    formal_store: true,
  });
  assert.deepEqual(restored.settings.task_automation.task_sources, ["D:/saved-todos"]);
  assert.deepEqual(restored.inspector.task_sources, ["D:/saved-todos"]);
  assert.deepEqual(restored.inspector.inspection_interval, { unit: "minute", value: 15 });
  assert.equal(restored.inspector.inspect_on_startup, true);
  assert.equal(restored.inspector.inspect_on_file_change, true);
  assert.equal(restored.inspector.remind_before_deadline, true);
  assert.equal(restored.inspector.remind_when_stale, false);
  assert.deepEqual(restored.warnings, []);
});

test("dashboard settings mutation keeps successful writes visible when settings readback fails", async () => {
  const { loadSettings } = loadSettingsServiceModule();
  const { formatDashboardSettingsMutationFeedback, updateDashboardSettings } = loadDashboardSettingsMutationModule({
    updateSettings: async () => ({
      apply_mode: "immediate",
      need_restart: false,
      updated_keys: ["memory.enabled"],
      effective_settings: {
        memory: {
          enabled: false,
        },
      },
    }),
    getSettingsDetailed: async () => {
      throw new Error("settings readback timed out");
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
      },
    });

    assert.equal(result.persisted, true);
    assert.equal(result.source, "rpc");
    assert.equal(result.readbackWarning, "settings readback timed out");
    assert.equal(result.snapshot.settings.memory.enabled, false);
    assert.deepEqual(result.snapshot.rpcContext.warnings, ["settings readback timed out"]);
    assert.equal(loadSettings().settings.memory.enabled, false);
    assert.match(
      formatDashboardSettingsMutationFeedback(result, "记忆开关"),
      /设置已写入，但 settings\.get 回读失败：settings readback timed out。当前先展示刚保存的本地快照。/,
    );
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

test("dashboard settings snapshot hydrates runtime defaults before merging scoped rpc payloads", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.settings",
      JSON.stringify({
        settings: {
          general: {
            download: {
              workspace_path: "workspace",
            },
          },
          task_automation: {
            task_sources: ["workspace/todos"],
          },
        },
      }),
    );

    const snapshot = await withDesktopAliasRuntime(
      async (requireFn) => {
        const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsSnapshot.js");
        const runtimeDefaultsModulePath = resolve(desktopRoot, ".cache/dashboard-tests/platform/desktopRuntimeDefaults.js");
        delete requireFn.cache[modulePath];
        delete requireFn.cache[runtimeDefaultsModulePath];

        const moduleExports = requireFn(modulePath) as {
          loadDashboardSettingsSnapshot: (source?: "rpc", scope?: AgentSettingsGetParams["scope"]) => Promise<{
            settings: {
              general: { download: { workspace_path: string } };
              memory: { enabled: boolean; lifecycle: string };
              task_automation: { task_sources: string[] };
            };
          }>;
        };

        return moduleExports.loadDashboardSettingsSnapshot("rpc", "memory");
      },
      {
        getSettingsDetailed: async () => ({
          data: {
            settings: {
              memory: {
                enabled: false,
                lifecycle: "session",
              },
            },
          },
          meta: {
            server_time: "2026-04-28T09:30:00Z",
          },
          warnings: [],
        }),
      },
      undefined,
      {
        invoke: async () => ({
          workspace_path: "/runtime/workspace",
          task_sources: ["/runtime/workspace/todos"],
        }),
      },
    ) as {
      settings: {
        general: { download: { workspace_path: string } };
        memory: { enabled: boolean; lifecycle: string };
        task_automation: { task_sources: string[] };
      };
    };

    assert.equal(snapshot.settings.general.download.workspace_path, "/runtime/workspace");
    assert.deepEqual(snapshot.settings.task_automation.task_sources, ["/runtime/workspace/todos"]);
    assert.equal(snapshot.settings.memory.enabled, false);
    assert.equal(snapshot.settings.memory.lifecycle, "session");
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

test("control panel data keeps the trusted runtime workspace separate from the formal settings snapshot", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    const { loadControlPanelData } = loadControlPanelServiceModule(
      {
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
          settings: {
            general: {
              language: "zh-CN",
              auto_launch: true,
              theme_mode: "follow_system",
              voice_notification_enabled: true,
              voice_type: "default_female",
              download: {
                workspace_path: "D:/pending-workspace",
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
              task_sources: ["D:/pending-workspace/todos"],
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
                stronghold: {
                  backend: "stronghold",
                  available: true,
                  fallback: false,
                  initialized: true,
                  formal_store: true,
                },
              },
            },
          },
        }),
        getTaskInspectorConfig: async () => ({
          task_sources: ["D:/pending-workspace/todos"],
          inspection_interval: { unit: "minute", value: 15 },
          inspect_on_file_change: true,
          inspect_on_startup: true,
          remind_before_deadline: true,
          remind_when_stale: false,
        }),
      },
      {
        invoke: async (command) => {
          if (command === "desktop_get_runtime_defaults") {
            return {
              workspace_path: "D:/runtime-workspace",
              task_sources: ["D:/runtime-workspace/todos"],
            };
          }

          if (command === "desktop_sync_settings_snapshot") {
            return undefined;
          }

          throw new Error(`unexpected desktop host command: ${command}`);
        },
      },
    );

    const loaded = await loadControlPanelData();

    assert.equal(loaded.runtimeWorkspacePath, "D:/runtime-workspace");
    assert.equal(loaded.settings.general.download.workspace_path, "D:/pending-workspace");
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("control panel data clears stale runtime workspace paths when host verification fails", async () => {
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
      __TAURI_INTERNALS__: {},
      localStorage,
    },
  });

  try {
    localStorage.setItem(
      "cialloclaw.runtime-defaults",
      JSON.stringify({
        workspace_path: "D:/stale-runtime-workspace",
        task_sources: ["D:/stale-runtime-workspace/todos"],
      }),
    );

    const { loadControlPanelData } = loadControlPanelServiceModule(
      {
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
          settings: {
            general: {
              language: "zh-CN",
              auto_launch: true,
              theme_mode: "follow_system",
              voice_notification_enabled: true,
              voice_type: "default_female",
              download: {
                workspace_path: "D:/pending-workspace",
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
              task_sources: ["D:/pending-workspace/todos"],
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
                stronghold: {
                  backend: "stronghold",
                  available: true,
                  fallback: false,
                  initialized: true,
                  formal_store: true,
                },
              },
            },
          },
        }),
        getTaskInspectorConfig: async () => ({
          task_sources: ["D:/pending-workspace/todos"],
          inspection_interval: { unit: "minute", value: 15 },
          inspect_on_file_change: true,
          inspect_on_startup: true,
          remind_before_deadline: true,
          remind_when_stale: false,
        }),
      },
      {
        invoke: async (command) => {
          if (command === "desktop_get_runtime_defaults") {
            throw new Error("desktop runtime defaults unavailable");
          }

          if (command === "desktop_sync_settings_snapshot") {
            return undefined;
          }

          throw new Error(`unexpected desktop host command: ${command}`);
        },
      },
    );

    const loaded = await loadControlPanelData();

    assert.equal(loaded.runtimeWorkspacePath, null);
    assert.equal(loaded.settings.general.download.workspace_path, "D:/pending-workspace");
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
  const mirrorDetailSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/MirrorDetailContent.tsx"), "utf8");

  assert.match(mirrorAppSource, /applyMirrorSettingsSnapshot\(current, result\.snapshot\)/);
  assert.doesNotMatch(
    mirrorAppSource,
    /const handleSettingsUpdate = useCallback\([\s\S]*loadMirrorOverviewData\(dataMode\)/,
  );
  assert.match(mirrorDetailSource, /settingsSnapshotUsesWarningBaseline/);
  assert.match(mirrorDetailSource, /本地回退快照/);
});

test("dashboard settings mutation keeps transport failures visible and does not mutate local settings", async () => {
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
    await assert.rejects(() => updateDashboardSettings({
      memory: {
        enabled: false,
        lifecycle: "session",
      },
    }), /transport is not wired/i);
    const after = loadSettings();

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
      mode: "open_result_page",
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

test("task delivery navigation helpers keep dashboard result-page hrefs stable", async () => {
  await withDesktopAliasRuntime((requireFn) => {
    const navigationModule = requireFn(resolve(desktopRoot, "src/features/dashboard/tasks/taskDeliveryNavigation.ts")) as {
      isDashboardTaskDeliveryHref: (url: string) => boolean;
      resolveDashboardTaskDeliveryRouteHref: (taskId: string) => string;
      resolveDashboardTaskDeliveryRoutePath: (taskId: string) => string;
    };

    assert.equal(
      navigationModule.resolveDashboardTaskDeliveryRoutePath("task result/001"),
      "/tasks/delivery/task%20result%2F001",
    );
    assert.equal(
      navigationModule.resolveDashboardTaskDeliveryRouteHref("task result/001"),
      "./dashboard.html#/tasks/delivery/task%20result%2F001",
    );
    assert.equal(
      navigationModule.isDashboardTaskDeliveryHref("./dashboard.html#/tasks/delivery/task%20result%2F001"),
      true,
    );
    assert.equal(navigationModule.isDashboardTaskDeliveryHref("https://example.test/result"), false);
  });
});

test("task output service exposes artifact list and open flows through formal RPC payloads", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/tasks/taskOutput.service.js");
      delete requireFn.cache[modulePath];

      const outputService = requireFn(modulePath) as {
        describeTaskOpenResultForCurrentTask: (plan: { mode: string; taskId: string | null }, currentTaskId: string | null) => string | null;
        isAllowedTaskOpenUrl: (url: string) => boolean;
        loadTaskArtifactPage: (taskId: string) => Promise<AgentTaskArtifactListResult>;
        openTaskArtifactForTask: (taskId: string, artifactId: string) => Promise<AgentTaskArtifactOpenResult>;
        openTaskDeliveryForTask: (taskId: string, artifactId?: string) => Promise<AgentDeliveryOpenResult>;
      };

      const artifactPage = await outputService.loadTaskArtifactPage("task_done_001");
      assert.ok(artifactPage.items.length > 0);
      assert.equal(artifactPage.page.offset, 0);

      const artifactOpen = await outputService.openTaskArtifactForTask("task_done_001", "artifact_done_003");
      assert.equal(artifactOpen.open_action, "reveal_in_folder");

      const deliveryOpen = await outputService.openTaskDeliveryForTask("task_done_001");
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
    },
    {
      listTaskArtifacts: async () => ({
        items: [
          {
            artifact_id: "artifact_done_003",
            artifact_type: "reveal_in_folder",
            created_at: "2026-04-28T08:00:00.000Z",
            mime_type: "application/pdf",
            path: "workspace/reports/q3-review.pdf",
            task_id: "task_done_001",
            title: "q3-review.pdf",
          },
        ],
        page: {
          has_more: false,
          limit: 1,
          offset: 0,
          total: 1,
        },
      }),
      openDelivery: async () => ({
        delivery_result: {
          payload: {
            path: null,
            task_id: "task_done_001",
            url: "https://example.test/result",
          },
          preview_text: "结果页",
          title: "结果页",
          type: "result_page",
        },
        open_action: "result_page",
        resolved_payload: {
          path: null,
          task_id: "task_done_001",
          url: "https://example.test/result",
        },
      }),
      openTaskArtifact: async () => ({
        artifact: {
          artifact_id: "artifact_done_003",
          artifact_type: "reveal_in_folder",
          created_at: "2026-04-28T08:00:00.000Z",
          mime_type: "application/pdf",
          path: "workspace/reports/q3-review.pdf",
          task_id: "task_done_001",
          title: "q3-review.pdf",
        },
        delivery_result: {
          payload: {
            path: "workspace/reports/q3-review.pdf",
            task_id: "task_done_001",
            url: null,
          },
          preview_text: "定位文件",
          title: "q3-review.pdf",
          type: "reveal_in_folder",
        },
        open_action: "reveal_in_folder",
        resolved_payload: {
          path: "workspace/reports/q3-review.pdf",
          task_id: "task_done_001",
          url: null,
        },
      }),
    },
  );
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
    feedback: "已定位到任务详情。",
  }, {
    onOpenTaskDetail: ({ taskId }) => {
      openedTaskIds.push(taskId);
      return "已在仪表盘中打开任务详情。";
    },
  });

  assert.deepEqual(openedTaskIds, ["task_dashboard_001"]);
  assert.equal(feedback, "已在仪表盘中打开任务详情。");
});

test("task output execution routes dashboard result pages through the formal delivery window path", async () => {
  const openedWindowLabels: string[] = [];
  const emittedRequests: Array<{ eventName: string; label: string; payload: unknown }> = [];
  const outputService = loadTaskOutputServiceModule(
    undefined,
    {
      openOrFocusDesktopWindow: async (label) => {
        openedWindowLabels.push(label);
        return label;
      },
    },
    {
      getCurrentWindow: () => ({
        label: "shell-ball",
        emit: () => Promise.resolve(),
        emitTo: (label, eventName, payload) => {
          emittedRequests.push({ eventName, label, payload });
          return Promise.resolve();
        },
      }),
    },
  );

  const feedback = await outputService.performTaskOpenExecution({
    mode: "open_url",
    taskId: "task_dashboard_001",
    path: null,
    url: "./dashboard.html#/tasks/delivery/task_dashboard_001",
    feedback: "已打开结果页。",
  });

  assert.deepEqual(openedWindowLabels, ["dashboard"]);
  assert.equal(feedback, "已打开结果页。");
  assert.equal(emittedRequests.length, 1);
  assert.equal(emittedRequests[0]?.label, "dashboard");
  assert.match(emittedRequests[0]?.eventName ?? "", /task-delivery-open/);
  assert.equal((emittedRequests[0]?.payload as { task_id?: string } | undefined)?.task_id, "task_dashboard_001");
});

test("task output execution delegates dashboard result pages through the shared callback when provided", async () => {
  const outputService = loadTaskOutputServiceModule();
  const openedTaskIds: string[] = [];

  const feedback = await outputService.performTaskOpenExecution({
    mode: "open_url",
    taskId: "task_dashboard_001",
    path: null,
    url: "./dashboard.html#/tasks/delivery/task_dashboard_001",
    feedback: "已打开结果页。",
  }, {
    onOpenTaskDelivery: ({ taskId }) => {
      openedTaskIds.push(taskId);
      return "已在仪表盘中打开结果页。";
    },
  });

  assert.deepEqual(openedTaskIds, ["task_dashboard_001"]);
  assert.equal(feedback, "已在仪表盘中打开结果页。");
});

test("note resource execution delegates task-detail routing through the shared callback", async () => {
  const noteService = loadNotePageServiceModule();
  const openedTaskIds: string[] = [];

  const feedback = await noteService.performNoteResourceOpenExecution({
    mode: "task_detail",
    feedback: "已定位到任务 Task detail。",
    path: null,
    taskId: "task_dashboard_001",
    url: null,
  }, {
    onOpenTaskDetail: ({ taskId }) => {
      openedTaskIds.push(taskId);
      return "已在仪表盘中打开 Task detail。";
    },
  });

  assert.deepEqual(openedTaskIds, ["task_dashboard_001"]);
  assert.equal(feedback, "已在仪表盘中打开 Task detail。");
});

test("task workspace routes formal delivery through a dedicated page and keeps list refresh task-updated aware", () => {
  const dashboardRootSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardRoot.tsx"), "utf8");
  const tasksPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TasksPage.tsx"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const taskDeliverySource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskDeliveryPage.tsx"), "utf8");
  const taskDetailSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");
  const taskDeliveryNavigationSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskDeliveryNavigation.ts"), "utf8");
  const taskOutputSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskOutput.service.ts"), "utf8");
  const taskDetailNavigationSource = loadDashboardTaskDetailNavigationSource();

  assert.match(tasksPageSource, /dashboardTaskDeliveryRoutePattern/);
  assert.match(tasksPageSource, /TaskDeliveryPage/);
  assert.match(taskPageSource, /buildDashboardTaskArtifactQueryKey/);
  assert.match(taskPageSource, /loadTaskArtifactPage/);
  assert.match(taskPageSource, /openTaskArtifactForTask/);
  assert.match(taskPageSource, /resolveDashboardTaskDeliveryRoutePath/);
  assert.match(taskPageSource, /readDashboardTaskDetailRouteState/);
  assert.match(taskPageSource, /subscribeTaskUpdated\(\(payload\) =>/);
  assert.match(taskPageSource, /subscribeDeliveryReady\(\(payload\) =>/);
  assert.match(taskPageSource, /scheduleBucketQueryRefresh/);
  assert.match(taskPageSource, /TASK_BUCKET_REFRESH_DEBOUNCE_MS/);
  assert.match(taskPageSource, /dashboardTaskEventQueryPrefix/);
  assert.match(taskPageSource, /payload\.task_id/);
  assert.doesNotMatch(taskPageSource, /subscribeTask\(/);
  assert.doesNotMatch(taskPageSource, /\["dashboard", "tasks", "artifacts"/);
  assert.doesNotMatch(taskPageSource, /TaskFilesSheet/);

  assert.match(taskDeliverySource, /openTaskDeliveryForTask/);
  assert.match(taskDeliverySource, /loadTaskArtifactPage/);
  assert.match(taskDeliverySource, /subscribeTaskUpdated/);
  assert.match(taskDeliverySource, /subscribeDeliveryReady/);
  assert.match(taskDeliverySource, /subscribeTaskRuntime/);
  assert.match(taskDeliverySource, /TASK_DELIVERY_DETAIL_REFRESH_DEBOUNCE_MS/);
  assert.match(taskDeliverySource, /scheduleTaskDetailRefresh/);
  assert.match(taskDeliverySource, /invalidateCurrentTaskArtifacts/);
  assert.match(taskDeliverySource, /subscribeTaskUpdated\(\(payload\) => \{[\s\S]*scheduleTaskDetailRefresh\(\);/);
  assert.match(taskDeliverySource, /subscribeDeliveryReady\(\(payload\) => \{[\s\S]*invalidateCurrentTaskDelivery\(\);/);
  assert.match(taskDeliverySource, /subscribeTaskRuntime\(taskId, \(\) => \{[\s\S]*scheduleTaskDetailRefresh\(\);/);
  assert.match(taskDeliverySource, /const taskDetailArtifacts = useMemo\(\(\) => detailData\?\.detail\.artifacts \?\? \[\], \[detailData\?\.detail\.artifacts\]\);/);
  assert.match(taskDeliverySource, /const artifactItems = useMemo\(\(\) => \{/);
  assert.match(taskDeliverySource, /const listedArtifacts = artifactListQuery\.data\?\.items \?\? \[\];/);
  assert.match(taskDeliverySource, /mergedArtifacts\.push\(artifact\);/);
  assert.doesNotMatch(taskDeliverySource, /const artifactItems = artifactListQuery\.data\?\.items \?\? detailData\?\.detail\.artifacts \?\? \[\];/);
  assert.match(taskDeliverySource, /buildDashboardTaskDetailRouteState/);
  assert.match(taskDeliverySource, /isAllowedTaskOpenUrl/);
  assert.match(taskDeliverySource, /formalDeliveryUrlIsAllowed/);
  assert.doesNotMatch(taskDeliverySource, /href=\{formalDeliveryResult\.payload\.url\}/);
  assert.match(taskDeliverySource, /当前正式结果已经在交付页中展示/);

  assert.doesNotMatch(taskDetailSource, /当前协议尚未提供稳定的 artifact\.open 能力/);
  assert.match(taskDetailSource, /onOpenArtifact/);
  assert.match(taskDetailSource, /onOpenLatestDelivery/);
  assert.match(taskDetailSource, /查看结果页/);
  assert.doesNotMatch(taskDetailSource, /文件舱门/);
  assert.match(taskDetailSource, /artifactItems/);

  assert.match(taskDeliveryNavigationSource, /dashboardTaskDeliveryRoutePattern = "delivery\/:taskId"/);
  assert.match(taskDeliveryNavigationSource, /encodeURIComponent\(taskId\)/);
  assert.match(taskDeliveryNavigationSource, /dashboardTaskDeliveryNavigationEvent/);
  assert.match(taskDeliveryNavigationSource, /requestDashboardTaskDeliveryOpen/);
  assert.doesNotMatch(taskOutputSource, /isRpcChannelUnavailable/);
  assert.doesNotMatch(taskOutputSource, /logRpcMockFallback/);
  assert.match(taskOutputSource, /isAllowedTaskOpenUrl/);
  assert.match(taskOutputSource, /onOpenTaskDetail/);
  assert.match(taskOutputSource, /requestDashboardTaskDeliveryOpen/);
  assert.match(taskDetailNavigationSource, /requestDashboardTaskDetailOpen/);
  assert.match(dashboardRootSource, /dashboardTaskDeliveryNavigationEvent/);
  assert.match(dashboardRootSource, /navigateToDashboardTaskDelivery/);
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

test("dashboard opening mask replays after Tauri window focus returns from hidden desktop sessions", () => {
  const dashboardRootSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardRoot.tsx"), "utf8");

  assert.match(dashboardRootSource, /createDashboardOpeningTransitionController/);
  assert.match(dashboardRootSource, /const handleVisibilityChange = \(\) => \{/);
  assert.match(dashboardRootSource, /\.onFocusChanged\(\(\{ payload: focused \}\) => \{/);
  assert.match(dashboardRootSource, /openingTransitionController\.handleWindowFocusChanged\(focused\);/);
});

test("dashboard opening transition controller replays focus and visibility recovery at runtime", () => {
  const {
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    createDashboardOpeningTransitionController,
  } = loadDashboardOpeningTransitionModule();
  const openingStates: boolean[] = [];
  const timeoutDurations: number[] = [];
  const cancelledFrames: number[] = [];
  const clearedTimeouts: number[] = [];
  const frameCallbacks = new Map<number, FrameRequestCallback>();
  const timeoutCallbacks = new Map<number, () => void>();
  let nextHandle = 1;
  let visibilityState: DocumentVisibilityState = "visible";
  let hasFocus = true;

  const controller = createDashboardOpeningTransitionController({
    cancelAnimationFrame: (handle) => {
      if (handle > 0) {
        cancelledFrames.push(handle);
        frameCallbacks.delete(handle);
      }
    },
    clearTimeout: (handle) => {
      if (handle > 0) {
        clearedTimeouts.push(handle);
        timeoutCallbacks.delete(handle);
      }
    },
    hasFocus: () => hasFocus,
    getVisibilityState: () => visibilityState,
    requestAnimationFrame: (callback) => {
      const handle = nextHandle++;
      frameCallbacks.set(handle, callback);
      return handle;
    },
    setIsOpening: (value) => {
      openingStates.push(value);
    },
    setTimeout: (callback, timeoutMs) => {
      const handle = nextHandle++;
      timeoutDurations.push(timeoutMs);
      timeoutCallbacks.set(handle, callback);
      return handle;
    },
  });

  controller.trigger();
  assert.deepEqual(openingStates, [true]);
  assert.deepEqual(timeoutDurations, [DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS]);

  controller.handleWindowFocusChanged(false);
  assert.equal(cancelledFrames.length, 1);
  assert.equal(clearedTimeouts.length, 1);

  controller.handleWindowFocusChanged(true);
  assert.deepEqual(openingStates, [true, true]);
  assert.deepEqual(timeoutDurations, [
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
  ]);
  assert.equal(frameCallbacks.size, 1);
  Array.from(frameCallbacks.values()).at(-1)?.(16.7);
  assert.deepEqual(openingStates, [true, true, false]);

  controller.handleWindowFocusChanged(false);
  visibilityState = "hidden";
  controller.handleWindowFocusChanged(true);
  assert.deepEqual(openingStates, [true, true, false]);

  visibilityState = "visible";
  controller.handleVisibilityChange();
  assert.deepEqual(openingStates, [true, true, false, true]);
  assert.deepEqual(timeoutDurations, [
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
  ]);
  Array.from(timeoutCallbacks.values()).at(-1)?.();
  assert.deepEqual(openingStates, [true, true, false, true, false]);

  controller.dispose();
  assert.equal(cancelledFrames.length, 3);
  assert.equal(clearedTimeouts.length, 3);
});

test("dashboard opening transition controller replays the opening mask for windows mounted while hidden", () => {
  const {
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    createDashboardOpeningTransitionController,
  } = loadDashboardOpeningTransitionModule();
  const openingStates: boolean[] = [];
  const timeoutDurations: number[] = [];
  const frameCallbacks = new Map<number, FrameRequestCallback>();
  let nextHandle = 1;
  let visibilityState: DocumentVisibilityState = "hidden";
  let hasFocus = false;

  const controller = createDashboardOpeningTransitionController({
    cancelAnimationFrame: (handle) => {
      frameCallbacks.delete(handle);
    },
    clearTimeout: () => {},
    hasFocus: () => hasFocus,
    getVisibilityState: () => visibilityState,
    requestAnimationFrame: (callback) => {
      const handle = nextHandle++;
      frameCallbacks.set(handle, callback);
      return handle;
    },
    setIsOpening: (value) => {
      openingStates.push(value);
    },
    setTimeout: (_callback, timeoutMs) => {
      timeoutDurations.push(timeoutMs);
      return nextHandle++;
    },
  });

  controller.trigger();
  assert.deepEqual(openingStates, [true]);
  assert.deepEqual(timeoutDurations, [DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS]);

  visibilityState = "visible";
  assert.equal(controller.handleVisibilityChange(), true);
  assert.deepEqual(openingStates, [true, true]);
  assert.deepEqual(timeoutDurations, [
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
  ]);
});

test("dashboard opening transition controller replays the opening mask for windows mounted while unfocused", () => {
  const {
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    createDashboardOpeningTransitionController,
  } = loadDashboardOpeningTransitionModule();
  const openingStates: boolean[] = [];
  const timeoutDurations: number[] = [];
  let nextHandle = 1;
  let visibilityState: DocumentVisibilityState = "visible";
  let hasFocus = false;

  const controller = createDashboardOpeningTransitionController({
    cancelAnimationFrame: () => {},
    clearTimeout: () => {},
    hasFocus: () => hasFocus,
    getVisibilityState: () => visibilityState,
    requestAnimationFrame: () => nextHandle++,
    setIsOpening: (value) => {
      openingStates.push(value);
    },
    setTimeout: (_callback, timeoutMs) => {
      timeoutDurations.push(timeoutMs);
      return nextHandle++;
    },
  });

  controller.trigger();
  assert.deepEqual(openingStates, [true]);
  assert.deepEqual(timeoutDurations, [DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS]);

  hasFocus = true;
  assert.equal(controller.handleWindowFocusChanged(true), true);
  assert.deepEqual(openingStates, [true, true]);
  assert.deepEqual(timeoutDurations, [
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
    DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS,
  ]);
});

test("dashboard entry keeps a window-level error boundary so runtime faults do not collapse into a blank shell", () => {
  const dashboardMainSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/main.tsx"), "utf8");
  const dashboardErrorBoundarySource = readFileSync(
    resolve(desktopRoot, "src/app/dashboard/DashboardWindowErrorBoundary.tsx"),
    "utf8",
  );

  assert.match(dashboardMainSource, /DashboardWindowErrorBoundary/);
  assert.match(
    dashboardMainSource,
    /<DashboardWindowErrorBoundary>[\s\S]*<AppProviders>[\s\S]*<DashboardRoot \/>[\s\S]*<\/AppProviders>[\s\S]*<\/DashboardWindowErrorBoundary>/,
  );
  assert.match(dashboardErrorBoundarySource, /export function DashboardWindowErrorBoundary/);
  assert.match(dashboardErrorBoundarySource, /class DashboardWindowErrorBoundaryImpl extends Component/);
  assert.match(dashboardErrorBoundarySource, /static getDerivedStateFromError/);
  assert.match(dashboardErrorBoundarySource, /window\.location\.reload\(\)/);
  assert.match(dashboardErrorBoundarySource, /dashboard window render failed/);
});

test("dashboard window error boundary renders a recovery fallback and reload action after runtime faults", () => {
  const { DashboardWindowErrorBoundary } = loadDashboardWindowErrorBoundaryModule();
  const child = { props: { id: "child" }, type: "mock-child" };
  const { BoundaryImplementation, create } = instantiateDashboardWindowErrorBoundary(DashboardWindowErrorBoundary);
  const boundary = create({ children: child });
  const originalConsoleError = console.error;
  const originalWindowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  const consoleMessages: unknown[][] = [];
  let reloadCalls = 0;

  try {
    console.error = (...args: unknown[]) => {
      consoleMessages.push(args);
    };
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: {
        location: {
          reload: () => {
            reloadCalls += 1;
          },
        },
      },
      writable: true,
    });

    assert.equal(boundary.render(), child);

    boundary.componentDidCatch(new Error("dashboard exploded"), {
      componentStack: "\n    at DashboardRoot",
    });
    assert.equal(consoleMessages.length, 1);
    assert.match(String(consoleMessages[0][0]), /dashboard window render failed/);

    boundary.state = {
      ...boundary.state,
      ...BoundaryImplementation.getDerivedStateFromError(),
    };

    const fallbackTree = boundary.render();
    const title = findRenderedElement(
      fallbackTree,
      (element) => element.type === "h1" && element.props.children === "仪表盘需要恢复",
    );
    const reloadButton = findRenderedElement(
      fallbackTree,
      (element) => element.props.type === "button" && typeof element.props.onClick === "function",
    );

    assert.ok(title);
    assert.ok(reloadButton);
    (reloadButton.props.onClick as () => void)();
    assert.equal(reloadCalls, 1);
  } finally {
    console.error = originalConsoleError;
    if (originalWindowDescriptor) {
      Object.defineProperty(globalThis, "window", originalWindowDescriptor);
    } else {
      delete (globalThis as { window?: unknown }).window;
    }
  }
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
  assert.match(noteServiceSource, /if \(payload\?\.url\) \{/);
  assert.match(noteServiceSource, /mode === "open_url"/);
});

test("source-note fallback cards stay local instead of inferring formal todo bucket and due status", () => {
  const noteService = loadNotePageServiceModule();
  const items = noteService.buildSourceNoteFallbackItems({
    content: [
      "- [ ] 复查仪表盘文案",
      "due: 2024-04-30T10:00:00.000Z",
      "note: 保留这一条给巡检同步。",
    ].join("\n"),
    fileName: "review.md",
    modifiedAtMs: 1714300000000,
    path: "D:/notes/review.md",
    sourceRoot: "D:/notes",
    title: "review",
  });

  assert.equal(items.length, 1);
  assert.equal(items[0].item.bucket, "later");
  assert.equal(items[0].item.status, "normal");
  assert.equal(items[0].experience.canConvertToTask, false);
  assert.equal(items[0].experience.detailStatus, "等待巡检同步");
  assert.equal(items[0].experience.previewStatus, "待巡检");
  assert.equal(items[0].experience.repeatRule, null);
});

test("note page no longer guesses source-note paths from duplicated titles", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function resolveNoteItemSourceNotePath\(/);
  assert.match(notePageSource, /function buildSourceNotePathLookup\(sourceNotes: SourceNoteDocument\[\]\)/);
  assert.match(notePageSource, /registerSourceNoteLookupKey\(lookup, `workspace\/\$\{relativePath\}`, note\);/);
  assert.doesNotMatch(notePageSource, /sourceNotesByTitle\.get\(item\.item\.title/);
  assert.match(notePageSource, /return null;/);
});

test("note page disambiguates duplicated relative source-note paths within the same lookup key instead of dropping them", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /type SourceNotePathLookup = Map<string, SourceNoteDocument\[\]>;/);
  assert.match(notePageSource, /function resolveAmbiguousSourceNoteCandidate\(/);
  assert.match(notePageSource, /const candidates = getSourceNoteLookupCandidates\(lookup, key\);/);
  assert.match(notePageSource, /return resolveAmbiguousSourceNoteCandidate\(item, candidates, sourceNoteBlocksByPath\) \?\? null;/);
  assert.match(notePageSource, /const sourceNoteBlocksByPath = useMemo\(\s*\(\) => new Map\(sourceNotes\.map\(\(note\) => \[note\.path, parseSourceNoteEditorBlocks\(note\)\]\)\),/);
});

test("note page keeps scanning related resources after null source-path misses", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /\.map\(\(resource\) => resolveSourceNoteLookupMatch\(item, sourceNotesByPath, sourceNoteBlocksByPath, resource\.path\)\)/);
  assert.match(notePageSource, /\.find\(\(note\): note is SourceNoteDocument => note !== null\)/);
  assert.doesNotMatch(notePageSource, /\.find\(\(note\) => note !== undefined\)/);
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
        controlTaskByAction: (taskId: string, action: "pause" | "resume" | "cancel" | "restart", source?: "rpc") => Promise<unknown>;
        loadTaskBucketPage: (group: "unfinished" | "finished", options?: { limit?: number; offset?: number; source?: "rpc" }) => Promise<unknown>;
        loadTaskDetailData: (taskId: string, source?: "rpc") => Promise<unknown>;
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
  const taskOutputSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskOutput.service.ts"), "utf8");

  assert.match(taskServiceSource, /function buildProtocolTaskExperience\(task: Task, detail\?: AgentTaskDetailGetResult\)/);
  assert.doesNotMatch(taskServiceSource, /getTaskExperience\(/);
  assert.doesNotMatch(taskServiceSource, /createFallbackExperience\(/);
  assert.doesNotMatch(taskServiceSource, /getMockTaskBuckets\(/);
  assert.doesNotMatch(taskServiceSource, /getMockTaskDetail\(/);
  assert.doesNotMatch(taskServiceSource, /runMockTaskControl\(/);
  assert.doesNotMatch(taskOutputSource, /getMockTaskDetail\(/);
});

test("note rpc service keeps transport failures visible instead of switching to mock data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/notes/notePage.service.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        convertNoteToTask: (itemId: string, source?: "rpc") => Promise<unknown>;
        loadNoteBucket: (group: "upcoming" | "later" | "recurring_rule" | "closed", source?: "rpc") => Promise<unknown>;
        updateNote: (itemId: string, action: "complete" | "cancel" | "move_upcoming" | "toggle_recurring" | "cancel_recurring" | "restore" | "delete", source?: "rpc") => Promise<unknown>;
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

test("note conversion and confirming-intent surfaces use direct task handoff wording instead of stale confirm copy", () => {
  const noteActionBarSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteActionBar.tsx"), "utf8");
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.service.ts"), "utf8");
  const taskMapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.mapper.ts"), "utf8");
  const voiceFieldSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/components/DashboardVoiceField.tsx"), "utf8");
  const homeServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts"), "utf8");

  assert.match(noteActionBarSource, /会按这条便签生成正式任务并跳转到任务页。/);
  assert.doesNotMatch(noteActionBarSource, /会直接生成任务并跳转到任务页。/);
  assert.match(notePageSource, /function getNoteConvertSuccessFeedback\(status: Task\["status"\]\)/);
  assert.match(notePageSource, /已按这条便签生成任务，正在打开任务详情。/);
  assert.doesNotMatch(notePageSource, /等待你确认处理方式。/);
  assert.match(notePageSource, /后续还需要处理授权。/);
  assert.match(noteServiceSource, /这条便签会按当前正文直接生成任务；如果还想补充路径、时间或说明，可以继续写在正文里后再转交给 Agent。/);
  assert.doesNotMatch(noteServiceSource, /基础便签数据/);
  assert.match(taskMapperSource, /todo: "便签转入"/);
  assert.match(voiceFieldSource, /return "正在等待确认处理方式";/);
  assert.doesNotMatch(voiceFieldSource, /已进入意图确认/);
  assert.match(homeServiceSource, /actionLabel: "前往处理"/);
  assert.match(homeServiceSource, /当前任务仍在等待确认处理方式/);
  assert.doesNotMatch(homeServiceSource, /actionLabel: "确认继续"/);
});

test("note visible content strips hidden header metadata before rendering preview copy", () => {
  const noteService = loadNotePageServiceModule();

  assert.equal(
    noteService.buildVisibleNoteText([
      "created_at: 2026-05-07T13:41:26.242Z",
      "updated_at: 2026-05-07T16:51:39.202Z",
      "",
      "今晚之前完成计算机作业",
      "作业在 C:/workspace/homework",
    ].join("\n")),
    [
      "今晚之前完成计算机作业",
      "作业在 C:/workspace/homework",
    ].join("\n"),
  );
  assert.equal(
    noteService.buildVisibleNoteText([
      "今晚之前完成计算机作业",
      "updated_at: 2026-05-07T16:51:39.202Z",
      "recurring_enabled: false",
    ].join("\n"), { title: "今晚之前完成计算机作业" }),
    "",
  );
  assert.equal(
    noteService.buildVisibleNoteText([
      "整理今天的会议纪要",
      "updated_at: 2026-05-07T16:51:39.202Z",
      "recurring_enabled: false",
    ].join("\n")),
    "整理今天的会议纪要",
  );
  assert.equal(noteService.buildVisibleNoteText("note: 只展示这一句"), "只展示这一句");
});

test("source note editor keeps a content-only input while preserving hidden markdown metadata", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const seededDraft = {
    ...sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    agentSuggestion: "把问题拆成前端回归点。",
    bucket: "upcoming" as const,
    createdAt: "2026-04-20T09:00:00.000Z",
    dueAt: "2026-05-01 18:30",
    endedAt: "2026-04-22T12:00:00.000Z",
    updatedAt: "2026-04-28T15:45:00.000Z",
  };

  const editedDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    seededDraft,
    "整理 PR365 的便签体验\n前端只让用户输入内容。\n其余元数据继续由系统维护。",
  );

  assert.equal(editedDraft.title, "整理 PR365 的便签体验");
  assert.equal(editedDraft.noteText, "前端只让用户输入内容。\n其余元数据继续由系统维护。");
  assert.equal(editedDraft.bucket, "upcoming");
  assert.equal(editedDraft.dueAt, "2026-05-01 18:30");
  assert.equal(editedDraft.agentSuggestion, "把问题拆成前端回归点。");
  assert.equal(
    sourceNoteEditor.formatSourceNoteEditorContent(editedDraft),
    "整理 PR365 的便签体验\n前端只让用户输入内容。\n其余元数据继续由系统维护。",
  );

  const serialized = sourceNoteEditor.serializeSourceNoteEditorDraft(editedDraft, new Date("2026-04-30T08:00:00.000Z"));
  assert.match(
    serialized.blockContent,
    /^- \[ \] 整理 PR365 的便签体验\nbucket: upcoming\ncreated_at: 2026-04-20T09:00:00.000Z\ndue: 2026-05-01 18:30\nagent: 把问题拆成前端回归点。\nended_at: 2026-04-22T12:00:00.000Z\nupdated_at: 2026-04-30T08:00:00.000Z\n\n前端只让用户输入内容。/,
  );
  assert.equal(serialized.normalizedDraft.title, "整理 PR365 的便签体验");
  assert.equal(serialized.normalizedDraft.noteText, "前端只让用户输入内容。\n其余元数据继续由系统维护。");
});

test("source note editor keeps completed notes completed while the textarea is temporarily empty", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const completedDraft = {
    ...sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    checked: true,
    noteText: "旧正文",
    title: "已完成便签",
  };

  const clearedDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(completedDraft, "");
  assert.equal(clearedDraft.checked, true);
  assert.equal(clearedDraft.title, "");
  assert.equal(clearedDraft.noteText, "");

  const replacedDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    clearedDraft,
    "重写后的标题\n重写后的正文",
  );
  assert.equal(replacedDraft.checked, true);
  assert.equal(replacedDraft.title, "重写后的标题");
  assert.equal(replacedDraft.noteText, "重写后的正文");
});

test("source note editor preserves an intentional blank line between the title and body", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const draft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    "标题\n\n第二段正文",
  );

  assert.equal(draft.title, "标题");
  assert.equal(draft.noteText, "\n第二段正文");
  assert.equal(
    sourceNoteEditor.formatSourceNoteEditorContent(draft),
    "标题\n\n第二段正文",
  );
});

test("source note editor keeps matched markdown blocks content-only without leaking hidden metadata back from the item fallback", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const draft = sourceNoteEditor.buildSourceNoteEditorDraftFromNote(
    {
      content: [
        "- [ ] 不好",
        "bucket: later",
        "created_at: 2026-05-02T11:35:52.489Z",
        "updated_at: 2026-05-02T11:35:52.489Z",
      ].join("\n"),
      fileName: "notes.md",
      modifiedAtMs: null,
      path: "workspace/notes/notes.md",
      sourceRoot: "workspace/notes",
      title: "notes",
    },
    {
      experience: {
        agentSuggestion: { detail: "", label: "" },
        canConvertToTask: false,
        detailStatus: "",
        detailStatusTone: "normal",
        effectiveScope: null,
        endedAt: null,
        isRecurringEnabled: false,
        nextOccurrenceAt: null,
        noteText: "created_at: 2026-05-02T11:35:52.489Z\n\nupdated_at: 2026-05-02T11:35:52.489Z",
        noteType: "follow-up",
        plannedAt: null,
        prerequisite: null,
        previewStatus: "",
        recentInstanceStatus: null,
        relatedResources: [],
        repeatRule: null,
        summaryLabel: "",
        timeHint: "",
        title: "",
        typeLabel: "",
      },
      item: {
        agent_suggestion: null,
        bucket: "later",
        due_at: null,
        effective_scope: null,
        item_id: "note_001",
        next_occurrence_at: null,
        note_text: "created_at: 2026-05-02T11:35:52.489Z\n\nupdated_at: 2026-05-02T11:35:52.489Z",
        prerequisite: null,
        recent_instance_status: null,
        repeat_rule: null,
        status: "pending",
        title: "不好",
      },
      sourceNote: {
        localOnly: false,
        path: "workspace/notes/notes.md",
        sourceLine: 1,
        title: "不好",
      },
    },
  );

  assert.equal(draft.title, "不好");
  assert.equal(draft.noteText, "");
});

test("source note editor strips backend-only metadata pollution from content-only note bodies", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();

  assert.equal(
    sourceNoteEditor.sanitizeSourceNoteBodyText([
      "完成计算机作业",
      "created_at: 2026-05-07T18:37:21.131Z",
      "updated_at: 2026-05-07T18:40:43.265Z",
      "recurring_enabled: false",
    ].join("\n"), { title: "完成计算机作业" }),
    "",
  );
  assert.equal(
    sourceNoteEditor.sanitizeSourceNoteBodyText([
      "第一行正文",
      "updated_at: 2026-05-07T18:40:43.265Z",
      "recurring_enabled: false",
    ].join("\n")),
    "第一行正文",
  );
  assert.equal(
    sourceNoteEditor.sanitizeSourceNoteBodyText([
      "status: 这一行现在是正文",
      "updated_at: 2026-05-07T18:40:43.265Z",
    ].join("\n")),
    "status: 这一行现在是正文",
  );
});

test("source note editor keeps custom header metadata hidden from the content editor", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const draft = sourceNoteEditor.buildSourceNoteEditorDraftFromNote(
    {
      content: [
        "- [ ] 只显示正文",
        "priority: p1",
        "owner: desk-team",
        "",
        "真正给用户编辑的正文",
      ].join("\n"),
      fileName: "notes.md",
      modifiedAtMs: null,
      path: "workspace/notes/notes.md",
      sourceRoot: "workspace/notes",
      title: "notes",
    },
    {
      experience: {
        agentSuggestion: { detail: "", label: "" },
        canConvertToTask: false,
        detailStatus: "",
        detailStatusTone: "normal",
        effectiveScope: null,
        endedAt: null,
        isRecurringEnabled: false,
        nextOccurrenceAt: null,
        noteText: "priority: p1\nowner: desk-team\n\n真正给用户编辑的正文",
        noteType: "follow-up",
        plannedAt: null,
        prerequisite: null,
        previewStatus: "",
        recentInstanceStatus: null,
        relatedResources: [],
        repeatRule: null,
        summaryLabel: "",
        timeHint: "",
        title: "",
        typeLabel: "",
      },
      item: {
        agent_suggestion: null,
        bucket: "later",
        due_at: null,
        effective_scope: null,
        item_id: "note_002",
        next_occurrence_at: null,
        note_text: "priority: p1\nowner: desk-team\n\n真正给用户编辑的正文",
        prerequisite: null,
        recent_instance_status: null,
        repeat_rule: null,
        status: "pending",
        title: "只显示正文",
      },
      sourceNote: {
        localOnly: false,
        path: "workspace/notes/notes.md",
        sourceLine: 1,
        title: "只显示正文",
      },
    },
  );

  assert.equal(draft.title, "只显示正文");
  assert.equal(draft.noteText, "真正给用户编辑的正文");
});

test("source note editor strips pasted checklist markers from the title without changing hidden completion state", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const completedDraft = {
    ...sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    checked: true,
  };

  const nextDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    completedDraft,
    "- [ ] 重写后的标题\n正文保持普通文本。",
  );
  assert.equal(nextDraft.checked, true);
  assert.equal(nextDraft.title, "重写后的标题");
  assert.equal(nextDraft.noteText, "正文保持普通文本。");
});

test("source note editor stops parsing hidden metadata after the body starts", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const blocks = sourceNoteEditor.parseSourceNoteEditorBlocks({
    content: [
      "- [ ] 保留正文里的保留前缀",
      "bucket: upcoming",
      "status: waiting_review",
      "",
      "status: 这一行现在是正文",
      "resource: https://example.com/as-body-text",
      "note: 这一行也应继续留在正文里",
    ].join("\n"),
    fileName: "tasks.md",
    modifiedAtMs: null,
    path: "workspace/notes/tasks.md",
    sourceRoot: "workspace/notes",
    title: "tasks",
  });

  assert.equal(blocks.length, 1);
  assert.equal(blocks[0]?.recentInstanceStatus, "waiting_review");
  assert.deepEqual(blocks[0]?.extraMetadata, []);
  assert.equal(
    blocks[0]?.noteText,
    "status: 这一行现在是正文\nresource: https://example.com/as-body-text\nnote: 这一行也应继续留在正文里",
  );
});

test("source note schedule helpers round-trip hidden time metadata and derive the persisted bucket", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();

  assert.equal(sourceNoteEditor.formatSourceNoteScheduleInputValue("2026-05-07T20:30"), "2026-05-07T20:30");
  assert.equal(sourceNoteEditor.serializeSourceNoteScheduleInputValue("2026-05-07T20:30"), new Date("2026-05-07T20:30").toISOString());
  assert.equal(sourceNoteEditor.resolveSourceNoteDraftBucketForSchedule({ dueAt: "2026-05-07T20:30:00.000Z", repeatRule: "" }), "upcoming");
  assert.equal(sourceNoteEditor.resolveSourceNoteDraftBucketForSchedule({ dueAt: "2026-05-07T20:30:00.000Z", repeatRule: "每周" }), "recurring_rule");
  assert.equal(sourceNoteEditor.resolveSourceNoteDraftBucketForSchedule({ dueAt: "", repeatRule: "" }), "later");
});

test("source note editor removes the matched markdown block when a note record is deleted", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const removal = sourceNoteEditor.removeSourceNoteEditorBlock(
    {
      content: [
        "- [ ] 第一条",
        "bucket: later",
        "",
        "- [x] 第二条",
        "bucket: closed",
        "ended_at: 2026-05-07T20:30:00.000Z",
      ].join("\n"),
      fileName: "notes.md",
      modifiedAtMs: null,
      path: "workspace/notes/notes.md",
      sourceRoot: "workspace/notes",
      title: "notes",
    },
    {
      sourceLine: 1,
      title: "第一条",
    },
  );

  assert.equal(removal.removed, true);
  assert.equal(
    removal.content,
    [
      "- [x] 第二条",
      "bucket: closed",
      "ended_at: 2026-05-07T20:30:00.000Z",
      "",
    ].join("\n"),
  );
});

test("note page resolves newly created source notes from the appended tail block instead of matching by mutable metadata", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /const createdBlocks = parseSourceNoteEditorBlocks\(savedNote\);/);
  assert.match(notePageSource, /const createdBlock = createdBlocks\[createdBlocks\.length - 1\] \?\? null;/);
  assert.doesNotMatch(notePageSource, /find\(\(block\) => block\.updatedAt === normalizedDraft\.updatedAt\)/);
});

test("note page prefers formal notepad items over local source fallback cards when matching a newly created source note", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /const formalCandidateIds = candidateIds\.filter\(\(itemId\) => \{/);
  assert.match(notePageSource, /return item \? !item\.sourceNote\?\.localOnly : false;/);
  assert.match(notePageSource, /const exactLineCandidate = formalCandidateIds\.find\(\(itemId\) => \{/);
  assert.match(notePageSource, /const exactCandidate = formalCandidateIds\.find\(\(itemId\) => \{/);
  assert.match(notePageSource, /return formalCandidateIds\.length === 1 \? formalCandidateIds\[0\] : null;/);
});

test("note page can pin a pending created source note before the formal bucket item finishes syncing", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function findPreferredItemIdForSourceNote\(/);
  assert.match(notePageSource, /const nextItemId = replacementItemId \?\? findPreferredItemIdForSourceNote\(/);
  assert.match(notePageSource, /if \(nextItem\.sourceNote\?\.localOnly\) \{/);
  assert.match(notePageSource, /showFeedback\("新便签已放到网格里，正在同步正式分组。"\);/);
});

test("note page upgrades canvas and selected source-note cards to their formal items once sync completes", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function findFormalReplacementItemIdForSourceNoteEntry\(/);
  assert.match(notePageSource, /if \(replacementItemId && \(selectedItem\?\.sourceNote\?\.localOnly \|\| !selectedItem\)\) \{/);
  assert.match(notePageSource, /if \(replacementItemId && \(currentItem\?\.sourceNote\?\.localOnly \|\| !currentItem\) && !seenItemIds\.has\(replacementItemId\)\) \{/);
  assert.match(notePageSource, /next\.push\(\{ \.\.\.entry, itemId: replacementItemId \}\);/);
});

test("note page syncs newly created source notes onto the board instead of auto-converting them into tasks", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /async function syncCreatedSourceNoteToBoard\(/);
  assert.match(notePageSource, /pinNoteToCanvasRef\.current\(matchedItem\.item\.item_id\);/);
  assert.match(notePageSource, /showFeedback\("新便签已同步到便签页，并放到了网格里。"\);/);
  assert.doesNotMatch(notePageSource, /const outcome = await convertNoteToTask\(matchedItem\.item\.item_id, dataMode\);/);
});

test("source note studio removes direct metadata form inputs from the user-facing editor", () => {
  const sourceNoteStudioSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/SourceNoteStudio.tsx"), "utf8");
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(sourceNoteStudioSource, /内容式便签编辑/);
  assert.match(sourceNoteStudioSource, /第一行会作为标题/);
  assert.match(sourceNoteStudioSource, /开始新便签/);
  assert.match(sourceNoteStudioSource, /点击“保存便签”后/);
  assert.match(sourceNoteStudioSource, /value=\{editorContent\}/);
  assert.doesNotMatch(sourceNoteStudioSource, /formatSourceNoteEditorContent/);
  assert.doesNotMatch(sourceNoteStudioSource, /updateSourceNoteEditorDraftContent/);
  assert.match(notePageSource, /const \[sourceNoteEditorContent, setSourceNoteEditorContent\] = useState/);
  assert.match(notePageSource, /const nextDraft = updateSourceNoteEditorDraftContent\(sourceNoteDraft, sourceNoteEditorContent\);/);
  assert.match(notePageSource, /editorContent=\{sourceNoteEditorContent\}/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>标题<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>分组<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>计划时间<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>Agent 建议<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /写入分组/);
  assert.doesNotMatch(sourceNoteStudioSource, /最近写回/);
});

test("note page deduplicates source-note fallback cards and canvas cards by source block identity", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function buildSourceNoteBlockAliases\(/);
  assert.match(notePageSource, /function resolveSourceNoteBlockAliases\(/);
  assert.match(notePageSource, /resolveSourceNoteBlockAliases\(item, sourceNotesByPath, sourceNoteBlocksByPath\)\.forEach\(\(alias\) => \{/);
  assert.match(notePageSource, /resolveSourceNoteBlockAliases\(item, sourceNotesByPath, sourceNoteBlocksByPath\)\.some\(\(alias\) => representedSourceNoteBlocks\.has\(alias\)\)/);
  assert.match(notePageSource, /const targetAliases = targetItem \? resolveSourceNoteBlockAliases\(targetItem, sourceNotesByPath, sourceNoteBlocksByPath\) : \[\];/);
  assert.match(notePageSource, /next\[replacementIndex\] = \{ \.\.\.next\[replacementIndex\], itemId \};/);
});

test("note page files overdue upcoming notes into the closed sidebar bucket without rewriting the formal bucket", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function resolveRailBucketForItem\(/);
  assert.match(notePageSource, /function resolveOverdueCanvasAutoReturnKeys\(/);
  assert.match(notePageSource, /if \(displayedBucket === "upcoming" && item\.item\.status === "overdue"\) \{/);
  assert.match(notePageSource, /return "closed";/);
  assert.match(notePageSource, /nextGroups\[resolveRailBucketForItem\(item, displayedBucket\)\]\.push\(item\);/);
  assert.match(notePageSource, /const activeOverdueKeys = new Set<string>\(\);/);
  assert.match(notePageSource, /if \(railBucket !== displayedBucket\) \{\s*resolveOverdueCanvasAutoReturnKeys\(item, sourceNotesByPath, sourceNoteBlocksByPath\)\.forEach\(\(key\) => activeOverdueKeys\.add\(key\)\);/);
  assert.match(notePageSource, /const autoReturnKeys = resolveOverdueCanvasAutoReturnKeys\(currentItem, sourceNotesByPath, sourceNoteBlocksByPath\);/);
  assert.match(notePageSource, /if \(railBucket !== displayedBucket && autoReturnKeys\.some\(\(key\) => !overdueCanvasAutoReturnedKeysRef\.current\.has\(key\)\)\) \{/);
  assert.match(notePageSource, /resolveOverdueCanvasAutoReturnKeys\(targetItem, sourceNotesByPath, sourceNoteBlocksByPath\)\.forEach\(\(key\) => \{\s*overdueCanvasAutoReturnedKeysRef\.current\.add\(key\);/);
  assert.match(notePageSource, /if \(!removedForRailBucket && next\.length === 0 && current\.length > 0 && defaultBoardItemIds\.length > 0 && boardBounds\) \{/);
});

test("note page persists local board layout and overdue auto-return markers across refreshes", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /import \{ loadStoredValue, removeStoredValue, saveStoredValue \} from "@\/platform\/storage";/);
  assert.match(notePageSource, /const NOTE_BOARD_STORAGE_KEY = "cialloclaw\.dashboard\.notes\.board";/);
  assert.match(notePageSource, /const persistedBoardStateRef = useRef<PersistedNoteBoardState \| null>\(loadPersistedNoteBoardState\(\)\);/);
  assert.match(notePageSource, /const \[boardStateHydrated, setBoardStateHydrated\] = useState\(\(\) => persistedBoardStateRef\.current === null\);/);
  assert.match(notePageSource, /if \(boardStateHydrated \|\| !boardLayerSize \|\| !noteBucketsResolved\) \{/);
  assert.match(notePageSource, /saveStoredValue<PersistedNoteBoardState>\(NOTE_BOARD_STORAGE_KEY, \{/);
  assert.match(notePageSource, /overdueAutoReturnedKeys: \[\.\.\.overdueCanvasAutoReturnedKeysRef\.current\],/);
  assert.match(notePageSource, /removeStoredValue\(NOTE_BOARD_STORAGE_KEY\);/);
});

test("note preview stacks assign increasing sidebar z-order so later cards cover earlier cards", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const notePreviewSectionSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NotePreviewSection.tsx"), "utf8");
  const notePreviewCardSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NotePreviewCard.tsx"), "utf8");
  const notePageStyleSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.css"), "utf8");

  assert.match(notePreviewSectionSource, /stackOrder=\{stackCards && items\.length > 1 \? index \+ 1 : undefined\}/);
  assert.match(notePageSource, /stackOrder=\{group\.items\.length > 1 \? index \+ 1 : undefined\}/);
  assert.match(notePreviewCardSource, /"--note-stack-order": String\(stackOrder\)/);
  assert.match(notePageStyleSource, /z-index: var\(--note-stack-order, 1\);/);
});

test("note page keeps markdown source blocks out of the rendered note buckets so new notes only show as formal cards", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.doesNotMatch(notePageSource, /buildSourceNoteFallbackItems/);
  assert.doesNotMatch(notePageSource, /const sourceFallbackItemsByBucket = useMemo\(\(\) => \{/);
  assert.match(notePageSource, /const upcomingItems = rpcUpcomingItems;/);
  assert.match(notePageSource, /const laterItems = rpcLaterItems;/);
  assert.match(notePageSource, /const recurringItems = rpcRecurringItems;/);
  assert.match(notePageSource, /const closedItems = rpcClosedItems;/);
  assert.match(notePageSource, /function isNoteItemRepresentedOnCanvas\(/);
  assert.match(notePageSource, /const canvasRepresentedSourceNoteBlocks = useMemo\(\(\) => \{/);
  assert.match(notePageSource, /resolveSourceNoteBlockAliases\(item, sourceNotesByPath, sourceNoteBlocksByPath\)\.some\(\(alias\) => canvasRepresentedSourceNoteBlocks\.has\(alias\)\)/);
  assert.match(notePageSource, /const visibleUpcomingItems = useMemo\([\s\S]*!isNoteItemRepresentedOnCanvas\(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath\)/);
  assert.match(notePageSource, /const visibleLaterItems = useMemo\([\s\S]*!isNoteItemRepresentedOnCanvas\(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath\)/);
  assert.match(notePageSource, /const visibleRecurringItems = useMemo\([\s\S]*!isNoteItemRepresentedOnCanvas\(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath\)/);
  assert.match(notePageSource, /const visibleClosedItems = useMemo\([\s\S]*!isNoteItemRepresentedOnCanvas\(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath\)/);
  assert.match(notePageSource, /items=\{railUpcomingItems\}/);
  assert.match(notePageSource, /items=\{railLaterItems\}/);
  assert.match(notePageSource, /items=\{railRecurringItems\}/);
  assert.match(notePageSource, /railUpcomingItems\.length/);
  assert.match(notePageSource, /railLaterItems\.length/);
  assert.match(notePageSource, /railRecurringItems\.length/);
  assert.match(notePageSource, /railClosedItems\.length/);
});

test("note page writes full markdown state back after note actions and removes the source block on delete", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function buildSourceNoteDraftFromFormalItem\(/);
  assert.match(notePageSource, /sanitizeSourceNoteBodyText\(nextItem\.note_text, \{ title: nextItem\.title \}\)/);
  assert.match(notePageSource, /async function persistSourceNoteMutationForItem\(/);
  assert.match(notePageSource, /const nextSourceFile = removeSourceNoteEditorBlock\(context\.note, context\.draft\);/);
  assert.match(notePageSource, /const nextDraft = buildSourceNoteDraftFromFormalItem\(context, nextItem\);/);
  assert.match(notePageSource, /await saveNoteSource\(taskSourceRoots, context\.note\.path, nextSourceFile\.content\);/);
  assert.match(notePageSource, /await persistSourceNoteMutationForItem\(\s*updatedItem,\s*outcome\.result\.notepad_item,\s*outcome\.result\.deleted_item_id \?\? null,\s*\);/);
  assert.match(notePageSource, /appendSourceBucketSyncFailure\(/);
});

test("recurring rule edit stays inside the detail-page schedule editor instead of jumping to the source-note editor", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /if \(action === "edit"\) \{\s*if \(selectedItem\.item\.bucket === "recurring_rule"\) \{\s*startScheduleEditingForItem\(selectedItem\);/);
});

test("paused recurring rules persist a hidden markdown override and reapply it after inspection refreshes", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /key\) !== "recurring_enabled"/);
  assert.match(notePageSource, /key: "recurring_enabled",\s*value: "false"/);
  assert.match(notePageSource, /const displayRpcItems = useMemo\(\s*\(\) => rawRpcItems\.map\(\(item\) => applySourceNoteDisplayOverrides\(item, sourceNotesByPath, sourceNoteBlocksByPath\)\)/);
  assert.match(notePageSource, /const recurringEnabledOverride = readSourceNoteRecurringEnabledOverride\(matchedBlock\);/);
  assert.match(notePageSource, /isRecurringEnabled: false,/);
  assert.match(notePageSource, /previewStatus: "规则已暂停",/);
});

test("note path display strips Windows extended prefixes without changing the underlying open path flow", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteDetailPanel.tsx"), "utf8");
  const noteMapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.mapper.ts"), "utf8");

  assert.match(noteMapperSource, /export function formatNoteDisplayPath\(value: string \| null \| undefined\)/);
  assert.match(noteMapperSource, /value\.startsWith\("\\\\\\\\\?\\\\UNC\\\\"/);
  assert.match(noteMapperSource, /value\.startsWith\("\\\\\\\\\?\\\\"/);
  assert.match(noteDetailPanelSource, /formatNoteDisplayPath\(experience\.effectiveScope\)/);
  assert.match(noteDetailPanelSource, /return formatNoteDisplayPath\(resource\.path\);/);
  assert.match(notePageSource, /formatNoteDisplayPath\(primarySourceNote\.path\)/);
  assert.match(notePageSource, /formatNoteDisplayPath\(resolvedSourceRoots\[0\]\)/);
  assert.match(notePageSource, /resource\.url \?\? formatNoteDisplayPath\(resource\.path\)/);
});

test("note page keeps formal source-note buckets stable across inspection refreshes", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /const rememberedFormalBucketByAliasRef = useRef\(new Map<string, NotePreviewGroupKey>\(\)\);/);
  assert.match(notePageSource, /const rawRpcItems = useMemo\(\s*\(\) => \[/);
  assert.match(notePageSource, /updateRememberedFormalBucketForItem\(\s*rememberedFormalBucketByAliasRef\.current,\s*item,\s*item\.item\.bucket,/);
  assert.match(notePageSource, /function resolveRememberedFormalBucket\(/);
  assert.match(notePageSource, /const displayedBucket = resolveRememberedFormalBucket\(/);
  assert.match(notePageSource, /if \(nextBucket === "later"\) \{\s*if \(options\.allowLaterReset\) \{\s*rememberedBucketByAlias\.delete\(alias\);/);
  assert.match(notePageSource, /updateRememberedFormalBucketForItem\([\s\S]*allowLaterReset: true/);
  assert.match(notePageSource, /nextGroups\[displayedBucket\]\.push\(item\);/);
  assert.doesNotMatch(notePageSource, /bucket:\s*rememberedBucketByAlias\.get\(rememberedBucket\)/);
});

test("note sidebar keeps single preview cards compact instead of stretching to fill the whole bucket", () => {
  const notePageStyleSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.css"), "utf8");

  assert.match(notePageStyleSource, /\.note-preview-shell__list,[\s\S]*align-content: start;/);
  assert.match(notePageStyleSource, /\.note-preview-card \{[\s\S]*align-self: start;/);
});

test.skip("note detail panel hides source scope and resource cards while keeping the action-bar open flow", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteDetailPanel.tsx"), "utf8");
  const noteActionBarSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteActionBar.tsx"), "utf8");

  assert.doesNotMatch(noteDetailPanelSource, /生效范围/);
  assert.doesNotMatch(noteDetailPanelSource, /当前事项关联的入口/);
  assert.doesNotMatch(noteDetailPanelSource, /note-detail-resource-list/);
  assert.doesNotMatch(noteDetailPanelSource, /onResourceOpen/);
  assert.doesNotMatch(notePageSource, /onResourceOpen=\{handleResourceOpen\}/);
  assert.match(noteActionBarSource, /"open-resource"/);
  assert.match(notePageSource, /if \(action === "open-resource"\)/);
  assert.match(notePageSource, /void handleResourceOpen\(firstResource\.id\);/);
});
test("note detail panel surfaces scope, linked tasks, and related resources without losing the action-bar flow", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteDetailPanel.tsx"), "utf8");
  const noteActionBarSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteActionBar.tsx"), "utf8");

  assert.match(noteDetailPanelSource, /生效范围/);
  assert.match(noteDetailPanelSource, /关联任务与资料/);
  assert.match(noteDetailPanelSource, /note-detail-resource-list/);
  assert.match(noteDetailPanelSource, /onOpenLinkedTask\?: \(\) => void;/);
  assert.match(noteDetailPanelSource, /onOpenResource\?: \(resourceId: string\) => void;/);
  assert.match(noteActionBarSource, /"open-linked-task"/);
  assert.match(noteActionBarSource, /查看资料列表/);
  assert.match(noteActionBarSource, /"open-resource"/);
  assert.match(notePageSource, /const \[noteResourcePickerOpen, setNoteResourcePickerOpen\] = useState\(false\);/);
  assert.match(notePageSource, /if \(action === "open-linked-task"\)/);
  assert.match(notePageSource, /if \(action === "open-resource"\)/);
  assert.match(notePageSource, /setNoteResourcePickerOpen\(true\);/);
  assert.match(notePageSource, /noteResourcePickerOpen && selectedItem/);
  assert.match(notePageSource, /onOpenResource=\{\(resourceId\) => \{/);
});

test("note detail schedule flow keeps time metadata outside the content-only editor via inline detail editing", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteDetailPanel.tsx"), "utf8");
  const noteScheduleDialogPath = resolve(desktopRoot, "src/features/dashboard/notes/components/NoteScheduleDialog.tsx");

  assert.match(notePageSource, /const \[noteScheduleEditing, setNoteScheduleEditing\] = useState\(false\);/);
  assert.match(notePageSource, /function startScheduleEditingForItem\(item: NoteListItem\) \{/);
  assert.match(notePageSource, /async function handleSaveNoteSchedule\(\) \{/);
  assert.match(notePageSource, /resolveSourceNoteDraftBucketForSchedule/);
  assert.match(notePageSource, /"notes_schedule_saved"/);
  assert.doesNotMatch(notePageSource, /<NoteScheduleDialog/);
  assert.match(noteDetailPanelSource, /scheduleEditing\?: boolean;/);
  assert.match(noteDetailPanelSource, /onStartScheduleEdit\?: \(\) => void;/);
  assert.match(noteDetailPanelSource, /scheduleActionLabel = "安排时间"/);
  assert.match(noteDetailPanelSource, /note-detail-schedule-editor/);
  assert.match(noteDetailPanelSource, /type="datetime-local"/);
  assert.match(noteDetailPanelSource, /placeholder="例如：每周、每两周、每天、每月"/);
  assert.match(noteDetailPanelSource, /保存安排/);
  assert.match(noteDetailPanelSource, /直接在详情页里设置首次时间和重复规则；正文编辑器仍保持只写内容/);
  assert.equal(existsSync(noteScheduleDialogPath), false);
});
test("recurring rule detail panel exposes a direct pause-resume button beside schedule editing instead of burying it in the footer action bar", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteDetailPanel.tsx"), "utf8");
  const noteActionBarSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteActionBar.tsx"), "utf8");

  assert.match(noteDetailPanelSource, /onToggleRecurring\?: \(\) => void;/);
  assert.match(noteDetailPanelSource, /const isRecurringRule = item\.item\.bucket === "recurring_rule";/);
  assert.match(noteDetailPanelSource, /const recurringToggleLabel = item\.experience\.isRecurringEnabled \? "暂停重复" : "开启重复";/);
  assert.match(noteDetailPanelSource, /点击“开启重复”可立即恢复/);
  assert.match(noteDetailPanelSource, /className="note-detail-card__action-row"/);
  assert.match(noteDetailPanelSource, /onClick=\{onToggleRecurring\}/);
  assert.match(notePageSource, /onToggleRecurring=\{selectedItem\.item\.bucket === "recurring_rule" \? \(\) => handleDetailAction\("toggle-recurring"\) : undefined\}/);
  assert.doesNotMatch(noteActionBarSource, /label: item\.experience\.isRecurringEnabled \? "暂停重复" : "开启重复"/);
  assert.doesNotMatch(noteActionBarSource, /label: "修改规则"/);
});

test("note board cards label footer time by start or next execution semantics", () => {
  const noteMapper = loadNotePageMapperModule();
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.equal(
    noteMapper.formatNoteBoardTimeHint({ bucket: "upcoming" }, { timeHint: "5/10 00:51" }),
    "开始时间 5/10 00:51",
  );
  assert.equal(
    noteMapper.formatNoteBoardTimeHint({ bucket: "later" }, { timeHint: "5/11 08:30" }),
    "开始时间 5/11 08:30",
  );
  assert.equal(
    noteMapper.formatNoteBoardTimeHint({ bucket: "recurring_rule" }, { timeHint: "5/10 00:51" }),
    "下次执行 5/10 00:51",
  );
  assert.equal(
    noteMapper.formatNoteBoardTimeHint({ bucket: "recurring_rule" }, { isRecurringEnabled: false, timeHint: "已暂停" }),
    "重复已暂停",
  );
  assert.equal(
    noteMapper.describeNotePreview({ bucket: "recurring_rule" }, { isRecurringEnabled: false, timeHint: "已暂停" }),
    "重复规则 · 已暂停",
  );
  assert.equal(
    noteMapper.formatNoteBoardTimeHint({ bucket: "closed" }, { timeHint: "5/12 18:20" }),
    "结束时间 5/12 18:20",
  );
  assert.match(notePageSource, /formatNoteBoardTimeHint\(item\.item, item\.experience\)/);
});

test("note board cards hide duplicate preview copy when a note has no visible body content", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const notePageStyleSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.css"), "utf8");

  assert.match(notePageSource, /const boardCardCopy = item\.experience\.noteText\.trim\(\);/);
  assert.match(notePageSource, /const hasBoardCardCopy = boardCardCopy !== "";/);
  assert.match(notePageSource, /!hasBoardCardCopy && "note-preview-page__board-card-title--spacious"/);
  assert.match(notePageSource, /hasBoardCardCopy \? <p className="note-preview-page__board-card-copy">\{boardCardCopy\}<\/p> : null/);
  assert.match(notePageStyleSource, /\.note-preview-page__board-card-title--spacious \{/);
});
test("note rpc service derives experience from protocol note data instead of mock fixtures", () => {
  const noteServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.service.ts"), "utf8");

  assert.match(noteServiceSource, /function mapItems\(items: TodoItem\[\]\)/);
  assert.doesNotMatch(noteServiceSource, /getMockNoteExperience\(/);
  assert.doesNotMatch(noteServiceSource, /getMockNoteBuckets\(/);
  assert.doesNotMatch(noteServiceSource, /runMockConvertNoteToTask\(/);
  assert.doesNotMatch(noteServiceSource, /runMockUpdateNote\(/);
});

test("security rpc service keeps transport failures visible instead of switching to mock governance data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/safety/securityService.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadSecurityModuleData: (source?: "rpc") => Promise<unknown>;
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

test("security detail rpc reads keep transport failures visible instead of switching to mock detail lists", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/safety/securityService.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadSecurityAuditRecords: (source: "rpc", taskId?: string | null, options?: { limit?: number; offset?: number }) => Promise<unknown>;
        loadSecurityPendingApprovals: (source: "rpc", options?: { limit?: number; offset?: number }) => Promise<unknown>;
        loadSecurityRestorePoints: (source: "rpc", options?: { limit?: number; offset?: number; taskId?: string | null }) => Promise<unknown>;
      };

      await assert.rejects(() => service.loadSecurityPendingApprovals("rpc"), /transport is not wired/i);
      await assert.rejects(() => service.loadSecurityRestorePoints("rpc", { taskId: "task_dashboard_001" }), /transport is not wired/i);
      await assert.rejects(() => service.loadSecurityAuditRecords("rpc", "task_dashboard_001"), /transport is not wired/i);
    },
    {
      listSecurityAuditDetailed: () => Promise.reject(transportError),
      listSecurityPendingDetailed: () => Promise.reject(transportError),
      listSecurityRestorePointsDetailed: () => Promise.reject(transportError),
    },
  );
});

test("dashboard home rpc service keeps transport failures visible instead of switching to mock orbit data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<unknown>;
      };

      await assert.rejects(() => service.loadDashboardHomeData(), /transport is not wired/i);
    },
    {
      getDashboardModule: () => Promise.reject(transportError),
      getDashboardOverview: () => Promise.reject(transportError),
      getRecommendations: () => Promise.reject(transportError),
    },
  );
});

test("mirror overview keeps rendering when memory settings snapshot falls back to a warning snapshot", async () => {
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
    await withDesktopAliasRuntime(
      async (requireFn) => {
        const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/memory/mirrorService.js");
        const snapshotModulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/shared/dashboardSettingsSnapshot.js");
        delete requireFn.cache[modulePath];
        delete requireFn.cache[snapshotModulePath];

        const service = requireFn(modulePath) as {
          loadMirrorOverviewData: () => Promise<{
            overview: { history_summary: string[] };
            rpcContext: { warnings: string[] };
            settingsSnapshot: {
              rpcContext: { warnings: string[] };
              settings: { memory: { enabled: boolean } };
              source: string;
            };
          }>;
        };

        const result = await service.loadMirrorOverviewData();

        assert.equal(result.overview.history_summary[0], "memory overview");
        assert.equal(result.settingsSnapshot.source, "rpc");
        assert.equal(result.settingsSnapshot.settings.memory.enabled, true);
        assert.deepEqual(result.settingsSnapshot.rpcContext.warnings, ["settings-context: memory settings unavailable"]);
        assert.ok(result.rpcContext.warnings.includes("settings-context: memory settings unavailable"));
      },
      {
        getMirrorOverviewDetailed: async () => ({
          data: {
            daily_summary: null,
            history_summary: ["memory overview"],
            memory_references: [],
            profile: null,
          },
          meta: {
            server_time: "2026-04-28T10:00:00Z",
          },
          warnings: [],
        }),
        getSettingsDetailed: async () => {
          throw new Error("memory settings unavailable");
        },
        getSecuritySummaryDetailed: async () => ({
          data: {
            summary: {
              latest_restore_point: null,
              pending_authorizations: 0,
              risk_level: "green",
              security_status: "normal",
            },
          },
          meta: {
            server_time: "2026-04-28T10:00:00Z",
          },
          warnings: [],
        }),
        listSecurityPendingDetailed: async () => ({
          data: {
            items: [],
            page: {
              has_more: false,
              limit: 20,
              offset: 0,
              total: 0,
            },
          },
          meta: {
            server_time: "2026-04-28T10:00:00Z",
          },
          warnings: [],
        }),
        listTasks: async () => ({
          items: [],
          page: {
            has_more: false,
            limit: 20,
            offset: 0,
            total: 0,
          },
        }),
      },
    );
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.assign(globalThis, { window: originalWindow });
    }
  }
});

test("dashboard home keeps module and recommendation failures local instead of blanking the full orbit", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          focusLine: { headline: string; reason: string };
          loadWarnings: string[];
          stateGroups: Array<{ key: string; states: string[] }>;
          summonTemplates: Array<unknown>;
          voiceSequences: Array<unknown>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.stateGroups.length, 4);
      assert.equal(data.loadWarnings.length, 3);
      assert.match(data.loadWarnings[0], /便签摘要同步失败：notes module unavailable/);
      assert.match(data.loadWarnings[1], /建议流同步失败：recommendations unavailable/);
      assert.match(data.loadWarnings[2], /镜子概览同步失败：mirror overview unavailable/);
      assert.equal(data.focusLine.headline, "当前整体风险等级为 低");
      assert.equal(data.summonTemplates.length, 1);
      assert.equal(data.voiceSequences.length, 0);
    },
    {
      getDashboardModule: async (params) => {
        const moduleName = (params as { module?: string }).module;
        if (moduleName === "notes") {
          throw new Error("notes module unavailable");
        }

        return {
          highlights: moduleName === "tasks" ? ["继续处理 task focus"] : [],
          module: moduleName ?? "unknown",
          summary: {},
          tab: "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => {
        throw new Error("recommendations unavailable");
      },
      getMirrorOverview: async () => {
        throw new Error("mirror overview unavailable");
      },
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home prioritizes live overview summons and task-detail targets over recommendation-only fallback copy", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { navigationTarget?: { kind: string; label: string; module: string; taskId?: string } }>;
          summonTemplates: Array<{ message: string; nextStep?: string; reason: string; stateKey: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.ok(data.summonTemplates.length >= 1);
      assert.equal(data.summonTemplates[0]?.message, "整理 Q3 复盘要点");
      assert.equal(data.summonTemplates[0]?.nextStep, "打开任务详情");
      assert.match(data.summonTemplates[0]?.reason ?? "", /刚生成了新的摘要草稿/);
      assert.equal(data.summonTemplates[0]?.stateKey, "task_working");
      assert.equal(data.stateMap.task_working?.navigationTarget?.kind, "task_detail");
      assert.equal(data.stateMap.task_working?.navigationTarget?.taskId, "task_focus_001");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";

        if (moduleName === "tasks") {
          return {
            highlights: ["继续推进当前摘要任务"],
            module: moduleName,
            summary: {
              blocked_tasks: 0,
              focus_runtime_summary: {
                active_steering_count: 0,
                events_count: 1,
                latest_event_type: null,
                loop_stop_reason: null,
                observation_signals: [],
              },
              focus_task_id: "task_focus_001",
              processing_tasks: 1,
              waiting_auth_tasks: 0,
            },
            tab: "focus",
          };
        }

        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: {
            current_step: "生成摘要",
            next_action: "等待处理完成",
            status: "processing",
            task_id: "task_focus_001",
            title: "整理 Q3 复盘要点",
            updated_at: "2026-04-07T10:40:00+08:00",
          },
          high_value_signal: ["刚生成了新的摘要草稿。"],
          quick_actions: ["打开任务详情"],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home prioritizes overview and module signals before recommendation-only fallback copy", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { navigationTarget?: { kind: string; label: string; module: string; taskId?: string } }>;
          summonTemplates: Array<{ message: string; module: string; nextStep?: string; reason: string; stateKey: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.ok(data.summonTemplates.length >= 3);
      assert.deepEqual(data.summonTemplates.slice(0, 3).map((item) => item.module), ["safety", "tasks", "memory"]);
      assert.equal(data.summonTemplates[0]?.message, "当前有 2 项操作等待授权");
      assert.equal(data.summonTemplates[1]?.nextStep, "打开任务详情");
      assert.equal(data.stateMap.task_working?.navigationTarget?.kind, "task_detail");
      assert.equal(data.stateMap.task_working?.navigationTarget?.taskId, "task_focus_001");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";

        if (moduleName === "tasks") {
          return {
            highlights: ["继续推进当前摘要任务"],
            module: moduleName,
            summary: {
              blocked_tasks: 0,
              focus_runtime_summary: {
                active_steering_count: 0,
                events_count: 1,
                latest_event_type: null,
                loop_stop_reason: null,
                observation_signals: [],
              },
              focus_task_id: "task_focus_001",
              processing_tasks: 1,
              waiting_auth_tasks: 1,
            },
            tab: "focus",
          };
        }

        if (moduleName === "notes") {
          return {
            highlights: ["两条便签接近执行窗口", "建议先整理今日提醒"],
            module: moduleName,
            summary: {
              completed_tasks: 3,
              exceptions: 1,
            },
            tab: "queue",
          };
        }

        if (moduleName === "memory") {
          return {
            highlights: ["本周复盘已经形成初稿", "最近三次协作都提到了同一风险边界"],
            module: moduleName,
            summary: {},
            tab: "overview",
          };
        }

        if (moduleName === "safety") {
          return {
            highlights: ["建议先处理待授权操作，再继续推进其它任务。"],
            module: moduleName,
            summary: {},
            tab: "guard",
          };
        }

        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: {
            current_step: "生成摘要",
            next_action: "等待处理完成",
            status: "processing",
            task_id: "task_focus_001",
            title: "整理 Q3 复盘要点",
            updated_at: "2026-04-07T10:40:00+08:00",
          },
          high_value_signal: ["刚生成了新的摘要草稿。"],
          quick_actions: ["打开任务详情"],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 2,
            risk_level: "yellow",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [
          {
            feedback_score: 0.8,
            intent: { confidence: 0.8, name: "task_follow_up" },
            recommendation_id: "rec_001",
            text: "继续推进当前任务。",
          },
        ],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: ["本周复盘已经形成初稿", "最近三次协作都提到了同一风险边界"],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async (params: unknown) => {
        const group = (params as { group?: string }).group;
        if (group === "upcoming") {
          return {
            items: [
              {
                agent_suggestion: "先处理这个事项。",
                bucket: "upcoming",
                due_at: "2026-04-07T18:00:00+08:00",
                item_id: "todo_home_001",
                status: "due_today",
                title: "重要客户邮件回复",
                type: "note",
              },
            ],
            page: {
              has_more: false,
              limit: 12,
              offset: 0,
              total: 1,
            },
          };
        }

        return {
          items: [],
          page: {
            has_more: false,
            limit: group === "closed" ? 24 : 12,
            offset: 0,
            total: 0,
          },
        };
      },
    },
  );
});

test("dashboard home keeps urgent safety summons aligned with safety copy instead of global task signals", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          summonTemplates: Array<{ message: string; module: string; nextStep?: string; reason: string; stateKey: string }>;
          stateMap: Record<string, { headline: string; navigationTarget?: { kind: string; label: string } }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      const safetySummon = data.summonTemplates.find((item) => item.stateKey === "safety_alert");
      assert.equal(safetySummon?.module, "safety");
      assert.equal(safetySummon?.message, "当前有 1 项操作等待授权");
      assert.equal(safetySummon?.reason, "建议先处理待授权操作，再继续推进其它任务。");
      assert.equal(safetySummon?.nextStep, "处理待授权操作");
      assert.equal(data.stateMap.safety_alert?.headline, "当前有 1 项操作等待授权");
      assert.equal(data.stateMap.safety_alert?.navigationTarget?.kind, "module");
      assert.equal(data.stateMap.safety_alert?.navigationTarget?.label, "处理待授权操作");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";
        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: moduleName === "tasks" ? "focus" : "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: {
            current_step: "生成摘要",
            next_action: "等待处理完成",
            status: "processing",
            task_id: "task_focus_001",
            title: "整理 Q3 复盘要点",
            updated_at: "2026-04-07T10:40:00+08:00",
          },
          high_value_signal: ["刚生成了新的摘要草稿。"],
          quick_actions: ["处理待授权操作", "打开任务详情"],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 1,
            risk_level: "yellow",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home reuses formal mirror profile fields for memory copy", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { headline: string; subline: string; context?: Array<{ text: string }>; navigationTarget?: { kind: string; activeDetailKey?: string } }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.stateMap.memory_summary?.headline, "用户画像");
      assert.equal(data.stateMap.memory_summary?.subline, "工作风格：偏好即时结果回显");
      assert.equal(data.stateMap.memory_summary?.context?.[0]?.text, "偏好交付：bubble");
      assert.equal(data.stateMap.memory_summary?.context?.[1]?.text, "活跃时段：16-21h");
      assert.equal(data.stateMap.memory_summary?.navigationTarget?.kind, "mirror_detail");
      assert.equal(data.stateMap.memory_summary?.navigationTarget?.activeDetailKey, "profile");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";

        if (moduleName === "notes") {
          return {
            highlights: ["最近恢复点 rp_1777961976151255500 已可用于安全回显。", "最近审计动作：generate_text -> openai_responses:deepseek-v4-flas..."],
            module: moduleName,
            summary: {
              completed_tasks: 2,
              exceptions: 1,
            },
            tab: "queue",
          };
        }

        if (moduleName === "memory") {
          return {
            highlights: ["最近恢复点 rp_1777961976151255500 已可用于安全回显。", "最近审计动作：generate_text -> openai_responses:deepseek-v4-flas..."],
            module: moduleName,
            summary: {},
            tab: "overview",
          };
        }

        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: moduleName === "tasks" ? "focus" : "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: [],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: ["这里是历史概要，不该覆盖用户画像文案。"],
        memory_references: [],
        profile: {
          active_hours: "16-21h",
          preferred_output: "bubble",
          work_style: "偏好即时结果回显",
        },
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home keeps a low-priority safety summon available when the formal trust chain is green but recoverable", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          summonTemplates: Array<{ message: string; module: string; nextStep?: string; priority: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.summonTemplates[0]?.module, "safety");
      assert.equal(data.summonTemplates[0]?.message, "最近恢复点可用");
      assert.equal(data.summonTemplates[0]?.nextStep, "查看安全详情");
      assert.equal(data.summonTemplates[0]?.priority, "low");
    },
    {
      getDashboardModule: async (params: unknown) => ({
        highlights: [],
        module: (params as { module?: string }).module ?? "unknown",
        summary: {},
        tab: "overview",
      }),
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: ["打开任务详情"],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home only uses quick actions for task summons that can truly deep-link to task detail", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          summonTemplates: Array<{ module: string; nextStep?: string; stateKey: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      const taskSummon = data.summonTemplates.find((item) => item.stateKey === "task_working");
      assert.equal(taskSummon?.module, "tasks");
      assert.equal(taskSummon?.nextStep, "打开任务页");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";
        if (moduleName === "tasks") {
          return {
            highlights: ["继续推进当前摘要任务"],
            module: moduleName,
            summary: {
              blocked_tasks: 0,
              processing_tasks: 0,
              waiting_auth_tasks: 0,
            },
            tab: "focus",
          };
        }

        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: ["当前任务轨道已有新的系统摘要。"],
          quick_actions: ["打开任务详情"],
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home keeps task-detail CTA copy when the first quick action targets a different route", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { navigationTarget?: { kind: string; label: string; taskId?: string } }>;
          summonTemplates: Array<{ message: string; module: string; nextStep?: string; stateKey: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      const taskSummon = data.summonTemplates.find((item) => item.stateKey === "task_working");
      assert.equal(taskSummon?.module, "tasks");
      assert.equal(taskSummon?.message, "整理 Q3 复盘要点");
      assert.equal(taskSummon?.nextStep, "打开任务详情");
      assert.equal(data.stateMap.task_working?.navigationTarget?.kind, "task_detail");
      assert.equal(data.stateMap.task_working?.navigationTarget?.taskId, "task_focus_001");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";

        if (moduleName === "tasks") {
          return {
            highlights: ["继续推进当前摘要任务"],
            module: moduleName,
            summary: {
              blocked_tasks: 0,
              focus_runtime_summary: {
                active_steering_count: 0,
                events_count: 1,
                latest_event_type: null,
                loop_stop_reason: null,
                observation_signals: [],
              },
              focus_task_id: "task_focus_001",
              processing_tasks: 1,
              waiting_auth_tasks: 1,
            },
            tab: "focus",
          };
        }

        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: {
            current_step: "生成摘要",
            next_action: "等待处理完成",
            status: "processing",
            task_id: "task_focus_001",
            title: "整理 Q3 复盘要点",
            updated_at: "2026-04-07T10:40:00+08:00",
          },
          high_value_signal: ["当前有 1 项操作等待授权"],
          quick_actions: ["处理待授权操作", "打开任务详情"],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 1,
            risk_level: "yellow",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home routes overview fallback signals to the inferred module instead of defaulting to tasks", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          summonTemplates: Array<{ message: string; module: string; nextStep?: string; stateKey: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.summonTemplates[0]?.message, "镜子里新增了一条历史概要");
      assert.equal(data.summonTemplates[0]?.module, "memory");
      assert.equal(data.summonTemplates[0]?.stateKey, "memory_summary");
      assert.equal(data.summonTemplates[0]?.nextStep, "打开镜子页");
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";
        return {
          highlights: moduleName === "memory" ? ["本周复盘已经形成初稿"] : [],
          module: moduleName,
          summary: {},
          tab: "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: ["镜子里新增了一条历史概要"],
          quick_actions: ["打开任务详情"],
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: ["这里是最近一条历史概要。", "第二条历史概要。"],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home prefers formal mirror references over profile copy when both exist", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { headline: string; subline: string; context?: Array<{ text: string }> }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.stateMap.memory_habit?.headline, "近期被调用记忆");
      assert.equal(data.stateMap.memory_habit?.subline, "本周战略复盘已被近期任务再次引用。");
      assert.equal(data.stateMap.memory_habit?.context?.[0]?.text, "来源：近期长期记忆命中");
      assert.equal(data.stateMap.memory_habit?.context?.[1]?.text, "近期任务再次命中这段长期记忆。");
    },
    {
      getDashboardModule: async (params: unknown) => ({
        highlights: [],
        module: (params as { module?: string }).module ?? "unknown",
        summary: {},
        tab: "overview",
      }),
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: [],
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: ["这里有历史概要，但不该覆盖近期记忆引用。"],
        memory_references: [
          {
            memory_id: "memory_strategy_weekly",
            reason: "近期任务再次命中这段长期记忆。",
            summary: "本周战略复盘已被近期任务再次引用。",
          },
        ],
        profile: {
          active_hours: "16-21h",
          preferred_output: "bubble",
          work_style: "偏好即时结果回显",
        },
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home sanitizes mirror reference copy before surfacing it on the home orb", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { subline: string; context?: Array<{ text: string }> }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.stateMap.memory_habit?.subline, "任务完成，意图=agent_loop，输入=你知道我现在在么...");
      assert.equal(data.stateMap.memory_habit?.context?.[0]?.text, "来源：近期长期记忆命中");
      assert.equal(data.stateMap.memory_habit?.context?.[1]?.text, "这条长期记忆再次命中了当前协作。");
    },
    {
      getDashboardModule: async (params: unknown) => ({
        highlights: [],
        module: (params as { module?: string }).module ?? "unknown",
        summary: {},
        tab: "overview",
      }),
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: [],
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [
          {
            memory_id: "memory_strategy_weekly",
            reason: "这条长期记忆再次命中了当前协作。",
            summary: " 任务完成，意图=agent_loop，输入=你知道我现在在么�... ",
          },
        ],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home rotates mirror summons across formal memory, profile, and history sections", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          summonTemplates: Array<{ module: string; message: string; nextStep?: string; expandedState?: { headline: string; navigationTarget?: { kind: string; activeDetailKey?: string } } }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();
      const mirrorSummons = data.summonTemplates.filter((item) => item.module === "memory");

      assert.deepEqual(
        mirrorSummons.slice(0, 3).map((item) => item.expandedState?.headline),
        ["近期被调用记忆", "用户画像", "历史概要"],
      );
      assert.deepEqual(
        mirrorSummons.slice(0, 3).map((item) => item.expandedState?.navigationTarget?.activeDetailKey),
        ["memory", "profile", "history"],
      );
      assert.equal(mirrorSummons[0]?.message, "近期被调用记忆");
      assert.equal(mirrorSummons[1]?.message, "用户画像");
      assert.equal(mirrorSummons[2]?.message, "历史概要");
    },
    {
      getDashboardModule: async (params: unknown) => ({
        highlights: [],
        module: (params as { module?: string }).module ?? "unknown",
        summary: {},
        tab: "overview",
      }),
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: [],
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: ["这里是最近一条历史概要。", "第二条历史概要。"],
        memory_references: [
          {
            memory_id: "memory_strategy_weekly",
            reason: "近期任务再次命中这段长期记忆。",
            summary: "本周战略复盘已被近期任务再次引用。",
          },
        ],
        profile: {
          active_hours: "16-21h",
          preferred_output: "bubble",
          work_style: "偏好即时结果回显",
        },
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home keeps notes copy module-native and skips fake empty-note summons", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { headline: string; subline: string; context?: Array<{ text: string }> }>;
          summonTemplates: Array<{ module: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.stateMap.notes_scheduled?.headline, "这里还没有可协作的事项");
      assert.equal(data.stateMap.notes_scheduled?.subline, "当前例外项 1 条，建议优先整理最接近执行窗口的事项。");
      assert.equal(data.stateMap.notes_scheduled?.context?.[0]?.text, "暂无便签");
      assert.equal(data.summonTemplates.some((item) => item.module === "notes"), false);
    },
    {
      getDashboardModule: async (params: unknown) => {
        const moduleName = (params as { module?: string }).module ?? "unknown";

        if (moduleName === "notes") {
          return {
            highlights: ["最近恢复点 rp_1777961976151255500 已可用于安全回显。", "最近审计动作：generate_text -> openai_responses:deepseek-v4-flas..."],
            module: moduleName,
            summary: {
              completed_tasks: 2,
              exceptions: 1,
            },
            tab: "queue",
          };
        }

        return {
          highlights: [],
          module: moduleName,
          summary: {},
          tab: moduleName === "tasks" ? "focus" : "overview",
        };
      },
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: [],
          trust_summary: {
            has_restore_point: true,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async () => ({
        items: [],
        page: {
          has_more: false,
          limit: 12,
          offset: 0,
          total: 0,
        },
      }),
    },
  );
});

test("dashboard home does not promote closed-only notes into the active note summon path", async () => {
  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<{
          stateMap: Record<string, { headline: string; subline: string }>;
          summonTemplates: Array<{ module: string; message: string }>;
        }>;
      };

      const data = await service.loadDashboardHomeData();

      assert.equal(data.stateMap.notes_scheduled?.headline, "这里还没有可协作的事项");
      assert.equal(data.summonTemplates.some((item) => item.module === "notes"), false);
    },
    {
      getDashboardModule: async (params: unknown) => ({
        highlights: [],
        module: (params as { module?: string }).module ?? "unknown",
        summary: {},
        tab: "overview",
      }),
      getDashboardOverview: async () => ({
        overview: {
          focus_summary: null,
          high_value_signal: [],
          quick_actions: [],
          trust_summary: {
            has_restore_point: false,
            pending_authorizations: 0,
            risk_level: "green",
            workspace_path: "workspace",
          },
        },
      }),
      getRecommendations: async () => ({
        cooldown_hit: false,
        items: [],
      }),
      getMirrorOverview: async () => ({
        daily_summary: null,
        history_summary: [],
        memory_references: [],
        profile: null,
      }),
      listNotepad: async (params: unknown) => {
        const group = (params as { group?: string }).group;
        if (group === "closed") {
          return {
            items: [
              {
                agent_suggestion: null,
                bucket: "closed",
                due_at: null,
                item_id: "todo_closed_001",
                status: "completed",
                title: "历史已结束事项",
                type: "archive",
              },
            ],
            page: {
              has_more: false,
              limit: 24,
              offset: 0,
              total: 1,
            },
          };
        }

        return {
          items: [],
          page: {
            has_more: false,
            limit: group === "closed" ? 24 : 12,
            offset: 0,
            total: 0,
          },
        };
      },
    },
  );
});

test("security service no longer imports governance mocks into product runtime", () => {
  const securityServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/securityService.ts"), "utf8");

  assert.doesNotMatch(securityServiceSource, /securitySummaryMock/);
  assert.doesNotMatch(securityServiceSource, /securityPendingMock/);
  assert.doesNotMatch(securityServiceSource, /securityRestorePointsMock/);
  assert.doesNotMatch(securityServiceSource, /securityAuditMock/);
  assert.doesNotMatch(securityServiceSource, /buildMockRespondResult/);
  assert.doesNotMatch(securityServiceSource, /buildMockRestoreApplyResult/);
  assert.doesNotMatch(securityServiceSource, /getInitialSecurityModuleData/);
});

test("security detail rpc reads keep transport failures visible instead of switching to mock detail lists", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, ".cache/dashboard-tests/features/dashboard/safety/securityService.js");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadSecurityAuditRecords: (source: "rpc", taskId?: string | null, options?: { limit?: number; offset?: number }) => Promise<unknown>;
        loadSecurityPendingApprovals: (source: "rpc", options?: { limit?: number; offset?: number }) => Promise<unknown>;
        loadSecurityRestorePoints: (source: "rpc", options?: { limit?: number; offset?: number; taskId?: string | null }) => Promise<unknown>;
      };

      await assert.rejects(() => service.loadSecurityPendingApprovals("rpc"), /transport is not wired/i);
      await assert.rejects(() => service.loadSecurityRestorePoints("rpc", { taskId: "task_dashboard_001" }), /transport is not wired/i);
      await assert.rejects(() => service.loadSecurityAuditRecords("rpc", "task_dashboard_001"), /transport is not wired/i);
    },
    {
      listSecurityAuditDetailed: () => Promise.reject(transportError),
      listSecurityPendingDetailed: () => Promise.reject(transportError),
      listSecurityRestorePointsDetailed: () => Promise.reject(transportError),
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
        loadMirrorOverviewData: (source?: "rpc") => Promise<unknown>;
      };

      await assert.rejects(() => service.loadMirrorOverviewData("rpc"), /transport is not wired/i);
    },
    {
      getMirrorOverviewDetailed: () => Promise.reject(transportError),
    },
  );
});

test("mirror service no longer imports overview mock data into product runtime", () => {
  const mirrorServiceSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/memory/mirrorService.ts"), "utf8");

  assert.doesNotMatch(mirrorServiceSource, /mirrorOverviewMock/);
  assert.doesNotMatch(mirrorServiceSource, /buildFallbackOverview/);
  assert.doesNotMatch(mirrorServiceSource, /getInitialMirrorOverviewData/);
});

test("source note editor keeps a content-only input while preserving hidden markdown metadata", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const seededDraft = {
    ...sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    agentSuggestion: "把问题拆成前端回归点。",
    bucket: "upcoming" as const,
    createdAt: "2026-04-20T09:00:00.000Z",
    dueAt: "2026-05-01 18:30",
    endedAt: "2026-04-22T12:00:00.000Z",
    updatedAt: "2026-04-28T15:45:00.000Z",
  };

  const editedDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    seededDraft,
    "整理 PR365 的便签体验\n前端只让用户输入内容。\n其余元数据继续由系统维护。",
  );

  assert.equal(editedDraft.title, "整理 PR365 的便签体验");
  assert.equal(editedDraft.noteText, "前端只让用户输入内容。\n其余元数据继续由系统维护。");
  assert.equal(editedDraft.bucket, "upcoming");
  assert.equal(editedDraft.dueAt, "2026-05-01 18:30");
  assert.equal(editedDraft.agentSuggestion, "把问题拆成前端回归点。");
  assert.equal(
    sourceNoteEditor.formatSourceNoteEditorContent(editedDraft),
    "整理 PR365 的便签体验\n前端只让用户输入内容。\n其余元数据继续由系统维护。",
  );

  const serialized = sourceNoteEditor.serializeSourceNoteEditorDraft(editedDraft, new Date("2026-04-30T08:00:00.000Z"));
  assert.match(
    serialized.blockContent,
    /^- \[ \] 整理 PR365 的便签体验\nbucket: upcoming\ncreated_at: 2026-04-20T09:00:00.000Z\ndue: 2026-05-01 18:30\nagent: 把问题拆成前端回归点。\nended_at: 2026-04-22T12:00:00.000Z\nupdated_at: 2026-04-30T08:00:00.000Z\n\n前端只让用户输入内容。/,
  );
  assert.equal(serialized.normalizedDraft.title, "整理 PR365 的便签体验");
  assert.equal(serialized.normalizedDraft.noteText, "前端只让用户输入内容。\n其余元数据继续由系统维护。");
});

test("source note editor keeps completed notes completed while the textarea is temporarily empty", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const completedDraft = {
    ...sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    checked: true,
    noteText: "旧正文",
    title: "已完成便签",
  };

  const clearedDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(completedDraft, "");
  assert.equal(clearedDraft.checked, true);
  assert.equal(clearedDraft.title, "");
  assert.equal(clearedDraft.noteText, "");

  const replacedDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    clearedDraft,
    "重写后的标题\n重写后的正文",
  );
  assert.equal(replacedDraft.checked, true);
  assert.equal(replacedDraft.title, "重写后的标题");
  assert.equal(replacedDraft.noteText, "重写后的正文");
});

test("source note editor preserves an intentional blank line between the title and body", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const draft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    "标题\n\n第二段正文",
  );

  assert.equal(draft.title, "标题");
  assert.equal(draft.noteText, "\n第二段正文");
  assert.equal(
    sourceNoteEditor.formatSourceNoteEditorContent(draft),
    "标题\n\n第二段正文",
  );
});

test("source note editor keeps matched markdown blocks content-only without leaking hidden metadata back from the item fallback", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const draft = sourceNoteEditor.buildSourceNoteEditorDraftFromNote(
    {
      content: [
        "- [ ] 不好",
        "bucket: later",
        "created_at: 2026-05-02T11:35:52.489Z",
        "updated_at: 2026-05-02T11:35:52.489Z",
      ].join("\n"),
      fileName: "notes.md",
      modifiedAtMs: null,
      path: "workspace/notes/notes.md",
      sourceRoot: "workspace/notes",
      title: "notes",
    },
    {
      experience: {
        agentSuggestion: { detail: "", label: "" },
        canConvertToTask: false,
        detailStatus: "",
        detailStatusTone: "normal",
        effectiveScope: null,
        endedAt: null,
        isRecurringEnabled: false,
        nextOccurrenceAt: null,
        noteText: "created_at: 2026-05-02T11:35:52.489Z\n\nupdated_at: 2026-05-02T11:35:52.489Z",
        noteType: "follow-up",
        plannedAt: null,
        prerequisite: null,
        previewStatus: "",
        recentInstanceStatus: null,
        relatedResources: [],
        repeatRule: null,
        summaryLabel: "",
        timeHint: "",
        title: "",
        typeLabel: "",
      },
      item: {
        agent_suggestion: null,
        bucket: "later",
        due_at: null,
        effective_scope: null,
        item_id: "note_001",
        next_occurrence_at: null,
        note_text: "created_at: 2026-05-02T11:35:52.489Z\n\nupdated_at: 2026-05-02T11:35:52.489Z",
        prerequisite: null,
        recent_instance_status: null,
        repeat_rule: null,
        status: "pending",
        title: "不好",
      },
      sourceNote: {
        localOnly: false,
        path: "workspace/notes/notes.md",
        sourceLine: 1,
        title: "不好",
      },
    },
  );

  assert.equal(draft.title, "不好");
  assert.equal(draft.noteText, "");
});

test("source note editor keeps custom header metadata hidden from the content editor", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const draft = sourceNoteEditor.buildSourceNoteEditorDraftFromNote(
    {
      content: [
        "- [ ] 只显示正文",
        "priority: p1",
        "owner: desk-team",
        "",
        "真正给用户编辑的正文",
      ].join("\n"),
      fileName: "notes.md",
      modifiedAtMs: null,
      path: "workspace/notes/notes.md",
      sourceRoot: "workspace/notes",
      title: "notes",
    },
    {
      experience: {
        agentSuggestion: { detail: "", label: "" },
        canConvertToTask: false,
        detailStatus: "",
        detailStatusTone: "normal",
        effectiveScope: null,
        endedAt: null,
        isRecurringEnabled: false,
        nextOccurrenceAt: null,
        noteText: "priority: p1\nowner: desk-team\n\n真正给用户编辑的正文",
        noteType: "follow-up",
        plannedAt: null,
        prerequisite: null,
        previewStatus: "",
        recentInstanceStatus: null,
        relatedResources: [],
        repeatRule: null,
        summaryLabel: "",
        timeHint: "",
        title: "",
        typeLabel: "",
      },
      item: {
        agent_suggestion: null,
        bucket: "later",
        due_at: null,
        effective_scope: null,
        item_id: "note_002",
        next_occurrence_at: null,
        note_text: "priority: p1\nowner: desk-team\n\n真正给用户编辑的正文",
        prerequisite: null,
        recent_instance_status: null,
        repeat_rule: null,
        status: "pending",
        title: "只显示正文",
      },
      sourceNote: {
        localOnly: false,
        path: "workspace/notes/notes.md",
        sourceLine: 1,
        title: "只显示正文",
      },
    },
  );

  assert.equal(draft.title, "只显示正文");
  assert.equal(draft.noteText, "真正给用户编辑的正文");
});

test("source note editor strips pasted checklist markers from the title without changing hidden completion state", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const completedDraft = {
    ...sourceNoteEditor.createEmptySourceNoteEditorDraft("workspace/notes/tasks.md"),
    checked: true,
  };

  const nextDraft = sourceNoteEditor.updateSourceNoteEditorDraftContent(
    completedDraft,
    "- [ ] 重写后的标题\n正文保持普通文本。",
  );
  assert.equal(nextDraft.checked, true);
  assert.equal(nextDraft.title, "重写后的标题");
  assert.equal(nextDraft.noteText, "正文保持普通文本。");
});

test("source note editor stops parsing hidden metadata after the body starts", () => {
  const sourceNoteEditor = loadSourceNoteEditorModule();
  const blocks = sourceNoteEditor.parseSourceNoteEditorBlocks({
    content: [
      "- [ ] 保留正文里的保留前缀",
      "bucket: upcoming",
      "status: waiting_review",
      "",
      "status: 这一行现在是正文",
      "resource: https://example.com/as-body-text",
      "note: 这一行也应继续留在正文里",
    ].join("\n"),
    fileName: "tasks.md",
    modifiedAtMs: null,
    path: "workspace/notes/tasks.md",
    sourceRoot: "workspace/notes",
    title: "tasks",
  });

  assert.equal(blocks.length, 1);
  assert.equal(blocks[0]?.recentInstanceStatus, "waiting_review");
  assert.deepEqual(blocks[0]?.extraMetadata, []);
  assert.equal(
    blocks[0]?.noteText,
    "status: 这一行现在是正文\nresource: https://example.com/as-body-text\nnote: 这一行也应继续留在正文里",
  );
});

test("note page resolves newly created source notes from the appended tail block instead of matching by mutable metadata", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /const createdBlocks = parseSourceNoteEditorBlocks\(savedNote\);/);
  assert.match(notePageSource, /const createdBlock = createdBlocks\[createdBlocks\.length - 1\] \?\? null;/);
  assert.doesNotMatch(notePageSource, /find\(\(block\) => block\.updatedAt === normalizedDraft\.updatedAt\)/);
});

test("source note studio removes direct metadata form inputs from the user-facing editor", () => {
  const sourceNoteStudioSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/SourceNoteStudio.tsx"), "utf8");
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(sourceNoteStudioSource, /内容式便签编辑/);
  assert.match(sourceNoteStudioSource, /第一行会作为标题/);
  assert.match(sourceNoteStudioSource, /开始新便签/);
  assert.match(sourceNoteStudioSource, /点击“保存便签”后/);
  assert.match(sourceNoteStudioSource, /value=\{editorContent\}/);
  assert.doesNotMatch(sourceNoteStudioSource, /formatSourceNoteEditorContent/);
  assert.doesNotMatch(sourceNoteStudioSource, /updateSourceNoteEditorDraftContent/);
  assert.match(notePageSource, /const \[sourceNoteEditorContent, setSourceNoteEditorContent\] = useState/);
  assert.match(notePageSource, /const nextDraft = updateSourceNoteEditorDraftContent\(sourceNoteDraft, sourceNoteEditorContent\);/);
  assert.match(notePageSource, /editorContent=\{sourceNoteEditorContent\}/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>标题<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>分组<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>计划时间<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /<span>Agent 建议<\/span>/);
  assert.doesNotMatch(sourceNoteStudioSource, /写入分组/);
  assert.doesNotMatch(sourceNoteStudioSource, /最近写回/);
});

test("dashboard home rpc service keeps transport failures visible instead of switching to mock orbit data", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");

  await withDesktopAliasRuntime(
    async (requireFn) => {
      const modulePath = resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.service.ts");
      delete requireFn.cache[modulePath];

      const service = requireFn(modulePath) as {
        loadDashboardHomeData: () => Promise<unknown>;
      };

      await assert.rejects(() => service.loadDashboardHomeData(), /transport is not wired/i);
    },
    {
      getDashboardModule: () => Promise.reject(transportError),
      getDashboardOverview: () => Promise.reject(transportError),
      getRecommendations: () => Promise.reject(transportError),
    },
  );
});

test("note page deduplicates source-note fallback cards and canvas cards by source block identity", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");

  assert.match(notePageSource, /function buildSourceNoteBlockAliases\(/);
  assert.match(notePageSource, /function resolveSourceNoteBlockAliases\(/);
  assert.match(notePageSource, /resolveSourceNoteBlockAliases\(item, sourceNotesByPath, sourceNotesByTitle\)\.forEach\(\(alias\) => \{/);
  assert.match(notePageSource, /resolveSourceNoteBlockAliases\(item, sourceNotesByPath, sourceNotesByTitle\)\.some\(\(alias\) => representedSourceNoteBlocks\.has\(alias\)\)/);
  assert.match(notePageSource, /const targetAliases = targetItem \? resolveSourceNoteBlockAliases\(targetItem, sourceNotesByPath, sourceNotesByTitle\) : \[\];/);
  assert.match(notePageSource, /next\[replacementIndex\] = \{ \.\.\.next\[replacementIndex\], itemId \};/);
});

test("note preview stacks assign increasing sidebar z-order so later cards cover earlier cards", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const notePreviewSectionSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NotePreviewSection.tsx"), "utf8");
  const notePreviewCardSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NotePreviewCard.tsx"), "utf8");
  const notePageStyleSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/notePage.css"), "utf8");

  assert.match(notePreviewSectionSource, /stackOrder=\{stackCards && items\.length > 1 \? index \+ 1 : undefined\}/);
  assert.match(notePageSource, /stackOrder=\{group\.items\.length > 1 \? index \+ 1 : undefined\}/);
  assert.match(notePreviewCardSource, /"--note-stack-order": String\(stackOrder\)/);
  assert.match(notePageStyleSource, /z-index: var\(--note-stack-order, 1\);/);
});
test("note detail panel hides source scope and resource cards while keeping the action-bar open flow", () => {
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const noteDetailPanelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteDetailPanel.tsx"), "utf8");
  const noteActionBarSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/components/NoteActionBar.tsx"), "utf8");

  assert.doesNotMatch(noteDetailPanelSource, /生效范围/);
  assert.doesNotMatch(noteDetailPanelSource, /当前事项关联的入口/);
  assert.doesNotMatch(noteDetailPanelSource, /note-detail-resource-list/);
  assert.doesNotMatch(noteDetailPanelSource, /onResourceOpen/);
  assert.doesNotMatch(notePageSource, /onResourceOpen=\{handleResourceOpen\}/);
  assert.match(noteActionBarSource, /"open-resource"/);
  assert.match(notePageSource, /if \(action === "open-resource"\)/);
  assert.match(notePageSource, /void handleResourceOpen\(firstResource\.id\);/);
});
test("TaskDetailPanel defers the entire fallback security summary until formal detail arrives", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");

  assert.match(panelSource, /detailState !== "ready" \|\| detail === null/);
  assert.match(panelSource, /等待详情同步后展示风险、授权与恢复点/);
});

test("task detail fallback keeps operator controls available from preview tasks and routed task ids", () => {
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");
  const actionBarSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskActionBar.tsx"), "utf8");
  const mapperSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/taskPage.mapper.ts"), "utf8");

  assert.match(taskPageSource, /const selectedTask = selectedTaskPreview\?\.task \?\? null;/);
  assert.match(taskPageSource, /const selectedTaskControlTargetId = selectedTask\?\.task_id \?\? selectedTaskId;/);
  assert.match(taskPageSource, /taskControlMutation\.mutate\(\{ action, taskId: selectedTaskControlTargetId \}\)/);
  assert.match(taskPageSource, /taskSteerMutation\.mutate\(\{ message, taskId: selectedTaskControlTargetId \}\)/);
  assert.match(taskPageSource, /taskId: selectedTaskControlTargetId/);
  assert.match(taskPageSource, /fallbackDetailActions: TaskPrimaryAction\[\] \| null/);
  assert.match(taskPageSource, /const fallbackOutputAccess = !selectedTaskPreview && Boolean\(selectedTaskId\);/);
  assert.doesNotMatch(taskPageSource, /detailData && artifactListQuery\.isError/);
  assert.match(panelSource, /task \? <TaskActionBar detail=\{detail\} onAction=\{onAction\} task=\{task\} \/> : null/);
  assert.match(panelSource, /fallbackActions && fallbackActions.length > 0 \? <TaskActionBar actionsOverride=\{fallbackActions\} detail=\{null\} onAction=\{onAction\} task=\{null\} \/> : null/);
  assert.match(panelSource, /fallbackOutputAccess \? \(/);
  assert.doesNotMatch(panelSource, /detailData \? <TaskActionBar/);
  assert.match(panelSource, /<h3 className="task-detail-card__title">已生成的结果<\/h3>/);
  assert.match(panelSource, /结果详情仍在同步，稍后可重试详情或直接尝试打开最新结果。/);
  assert.match(actionBarSource, /actionsOverride\?: TaskPrimaryAction\[\] \| null;/);
  assert.match(actionBarSource, /task: Task \| null;/);
  assert.match(mapperSource, /export function getTaskPrimaryActions\(task: Task, detail: AgentTaskDetailGetResult \| null\)/);
  assert.match(mapperSource, /const hasAnchor = detail !== null/);
  assert.doesNotMatch(mapperSource, /detail\?\.approval_request !== null \|\| detail\?\.security_summary\.latest_restore_point !== null/);
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

test("TaskDetailPanel keeps runtime sections visible for ended tasks and clears steering draft from explicit success state", () => {
  const panelSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/components/TaskDetailPanel.tsx"), "utf8");
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");

  assert.match(panelSource, /steeringSuccessVersion: number/);
  assert.match(panelSource, /if \(steeringPending \|\| steeringSuccessVersion === 0\)/);
  assert.doesNotMatch(panelSource, /handleSubmitSteering\(\)[\s\S]*setSteeringMessage\(""\)/);
  assert.match(panelSource, /\{renderRuntimeSummarySection\(\)\}/);
  assert.match(panelSource, /\{renderRuntimeEventsSection\(\)\}/);
  assert.match(taskPageSource, /const \[steeringSuccessVersion, setSteeringSuccessVersion\] = useState\(0\);/);
  assert.match(taskPageSource, /setSteeringSuccessVersion\(\(current\) => current \+ 1\);/);
  assert.match(taskPageSource, /steeringSuccessVersion=\{steeringSuccessVersion\}/);
  assert.match(taskPageSource, /invalidateTaskRuntimeQueries\(selectedTaskId\)/);
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

test("dashboard voice submit only reuses browser page context when a real URL is available", () => {
  const voiceFieldSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/home/components/DashboardVoiceField.tsx"), "utf8");

  assert.match(voiceFieldSource, /trigger: "voice_commit",[\s\S]*includeForegroundBrowserPageContext: true,/);
  assert.doesNotMatch(voiceFieldSource, /includeForegroundWindowContext: true/);
});
