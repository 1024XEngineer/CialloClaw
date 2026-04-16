import type { AgentTaskDetailGetResult, ApprovalRequest, RecoveryPoint } from "@cialloclaw/protocol";
import { isApprovalRequest, isRecoveryPoint } from "./dashboardContractValidators";

const dashboardSafetySnapshotFeedback = "实时安全数据已变化，当前展示的是路由携带的快照。";

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

export type DashboardSafetyRouteResolution = DashboardSafetyFocusTarget & {
  routedTaskId: string | null;
  shouldClearRouteState: boolean;
};

export function buildDashboardSafetyNavigationState(detail: AgentTaskDetailGetResult): DashboardSafetyNavigationState {
  const approvalRequest = detail.approval_request ?? null;
  const latestRestorePoint = detail.security_summary.latest_restore_point ?? null;

  if (approvalRequest) {
    return {
      approvalRequest,
      source: "task-detail",
      taskId: detail.task.task_id,
    };
  }

  if (latestRestorePoint) {
    return {
      restorePoint: latestRestorePoint,
      source: "task-detail",
      taskId: detail.task.task_id,
    };
  }

  return {
    source: "task-detail",
    taskId: detail.task.task_id,
  };
}

export function readDashboardSafetyNavigationState(value: unknown): DashboardSafetyNavigationState | null {
  if (!value || typeof value !== "object") {
    return null;
  }

  const candidate = value as Partial<DashboardSafetyNavigationState>;
  const approvalRequest = candidate.approvalRequest;
  const restorePoint = candidate.restorePoint;

  for (const key of Object.keys(candidate)) {
    if (key !== "approvalRequest" && key !== "restorePoint" && key !== "source" && key !== "taskId") {
      return null;
    }
  }

  if (candidate.source !== "task-detail") {
    return null;
  }

  if (typeof candidate.taskId !== "string") {
    return null;
  }

  if (approvalRequest !== undefined && !isApprovalRequest(approvalRequest)) {
    return null;
  }

  if (approvalRequest !== undefined && approvalRequest.task_id !== candidate.taskId) {
    return null;
  }

  if (restorePoint !== undefined && !isRecoveryPoint(restorePoint)) {
    return null;
  }

  if (restorePoint !== undefined && restorePoint.task_id !== candidate.taskId) {
    return null;
  }

  if (approvalRequest !== undefined && restorePoint !== undefined) {
    return null;
  }

  return {
    ...(approvalRequest ? { approvalRequest } : {}),
    ...(restorePoint ? { restorePoint } : {}),
    source: candidate.source,
    taskId: candidate.taskId,
  };
}

export function resolveDashboardSafetyFocusTarget({
  state,
  livePending,
  liveRestorePoint,
}: {
  state: DashboardSafetyNavigationState | null;
  livePending: ApprovalRequest[];
  liveRestorePoint: RecoveryPoint | null;
}): DashboardSafetyFocusTarget {
  if (!state || !state.approvalRequest && !state.restorePoint) {
    return {
      activeDetailKey: null,
      approvalSnapshot: null,
      feedback: null,
      restorePointSnapshot: null,
    };
  }

  if (state.approvalRequest) {
    const liveApproval = livePending.find((item) => item.approval_id === state.approvalRequest?.approval_id) ?? null;

    if (liveApproval) {
      return {
        activeDetailKey: `approval:${liveApproval.approval_id}`,
        approvalSnapshot: liveApproval,
        feedback: null,
        restorePointSnapshot: null,
      };
    }

    return {
      activeDetailKey: `approval:${state.approvalRequest.approval_id}`,
      approvalSnapshot: state.approvalRequest,
      feedback: dashboardSafetySnapshotFeedback,
      restorePointSnapshot: null,
    };
  }

  if (state.restorePoint) {
    if (liveRestorePoint?.recovery_point_id === state.restorePoint.recovery_point_id) {
      return {
        activeDetailKey: "restore",
        approvalSnapshot: null,
        feedback: null,
        restorePointSnapshot: liveRestorePoint,
      };
    }

    return {
      activeDetailKey: "restore",
      approvalSnapshot: null,
      feedback: dashboardSafetySnapshotFeedback,
      restorePointSnapshot: state.restorePoint,
    };
  }

  return {
    activeDetailKey: null,
    approvalSnapshot: null,
    feedback: null,
    restorePointSnapshot: null,
  };
}

export function resolveDashboardSafetyNavigationRoute({
  locationState,
  livePending,
  liveRestorePoint,
}: {
  locationState: unknown;
  livePending: ApprovalRequest[];
  liveRestorePoint: RecoveryPoint | null;
}): DashboardSafetyRouteResolution {
  const state = readDashboardSafetyNavigationState(locationState);
  const focusTarget = resolveDashboardSafetyFocusTarget({
    livePending,
    liveRestorePoint,
    state,
  });

  return {
    ...focusTarget,
    routedTaskId: state?.taskId ?? null,
    shouldClearRouteState: state !== null,
  };
}

export function shouldRetainDashboardSafetyActiveDetail({
  activeDetailKey,
  approvalSnapshot,
  cardKeys,
}: {
  activeDetailKey: string | null;
  approvalSnapshot: ApprovalRequest | null;
  cardKeys: string[];
}) {
  if (!activeDetailKey) {
    return false;
  }

  if (cardKeys.includes(activeDetailKey)) {
    return true;
  }

  return activeDetailKey.startsWith("approval:") && approvalSnapshot?.approval_id === activeDetailKey.slice("approval:".length);
}

export function isDashboardSafetyApprovalSnapshotOnly({
  activeDetailKey,
  approvalSnapshot,
  cardKeys,
}: {
  activeDetailKey: string | null;
  approvalSnapshot: ApprovalRequest | null;
  cardKeys: string[];
}) {
  if (!activeDetailKey?.startsWith("approval:") || !approvalSnapshot) {
    return false;
  }

  return !cardKeys.includes(activeDetailKey);
}

export function resolveDashboardSafetySnapshotLifecycle({
  activeDetailKey,
  routeDrivenDetailKey,
  approvalSnapshot,
  restorePointSnapshot,
  subscribedTaskId,
}: {
  activeDetailKey: string | null;
  routeDrivenDetailKey: string | null;
  approvalSnapshot: ApprovalRequest | null;
  restorePointSnapshot: RecoveryPoint | null;
  subscribedTaskId: string | null;
}) {
  if (routeDrivenDetailKey && activeDetailKey === routeDrivenDetailKey) {
    return {
      approvalSnapshot,
      restorePointSnapshot,
      routeDrivenDetailKey,
      subscribedTaskId,
    };
  }

  return {
    approvalSnapshot: null,
    restorePointSnapshot: null,
    routeDrivenDetailKey: null,
    subscribedTaskId: null,
  };
}
