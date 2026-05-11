import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { Badge, Button, Flex, Heading, Text } from "@radix-ui/themes";
import { useQueryClient } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router-dom";
import {
  ArrowUpRight,
  History,
  ShieldCheck,
  Siren,
  Wallet,
  X,
  type LucideIcon,
} from "lucide-react";
import type {
  AgentSecurityApprovalRespondResult,
  AgentTaskDetailGetResult,
  AuditRecord,
  ApprovalDecision,
  ApprovalPendingNotification,
  ApprovalRequest,
  RecoveryPoint,
  RiskLevel,
  SecurityStatus,
} from "@cialloclaw/protocol";
import { JsonRpcClientError } from "@/rpc/client";
import { subscribeApprovalPending, subscribeTask } from "@/rpc/subscriptions";
import { navigateToDashboardTaskDetail } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { useDashboardEscapeHandler } from "@/features/dashboard/shared/dashboardEscapeCoordinator";
import {
  isDashboardSafetyApprovalSnapshotOnly,
  resolveDashboardSafetyNavigationRoute,
  resolveDashboardSafetySnapshotLifecycle,
  shouldRetainDashboardSafetyActiveDetail,
} from "@/features/dashboard/shared/dashboardSafetyNavigation";
import {
  loadSecurityPendingApprovals,
  applySecurityRestorePoint,
  isSecurityApprovalRespondResult,
  isSecurityRestoreRespondResult,
  loadSecurityAuditRecords,
  loadSecurityFocusedTaskDetail,
  loadSecurityModuleData,
  loadSecurityModuleRpcData,
  loadSecurityRestorePoints,
  respondToApproval,
  type SecurityAuditRecordListData,
  type SecurityModuleData,
  type SecurityPendingListData,
  type SecurityRestorePointListData,
  type SecurityRespondOutcome,
} from "./securityService";
import { getDashboardTaskSecurityRefreshPlan } from "../tasks/taskPage.query";
import "./securityPage.css";
import "./securityBoard.css";

type SecurityCardKey = "status" | "restore" | "budget" | "governance" | `approval:${string}`;
type CardPosition = { x: number; y: number };
type CardSize = { width: number; height: number };
type BoardBounds = { minX: number; minY: number; maxX: number; maxY: number };
type BoardGrid = { columns: number; rows: number };
type BoardLayout = { bounds: BoardBounds; size: CardSize; grid: BoardGrid; candidates: CardPosition[] };
type DragState = {
  key: SecurityCardKey;
  pointerId: number;
  startX: number;
  startY: number;
  originX: number;
  originY: number;
  moved: boolean;
};
type ThemeColor = "gray" | "amber" | "orange" | "blue" | "green" | "red";
type SecurityCardPreview = {
  eyebrow: string;
  title: string;
  badgeLabel: string;
  badgeColor: ThemeColor;
  headline: string;
  supporting: string;
  meta: string;
  emphasis?: "number";
  icon: LucideIcon;
};
type SecurityRestoreScope = "focused_task" | "all";
type SecurityAuditScope = "focused_task" | "all";
type ImpactScopeDetails = NonNullable<AgentSecurityApprovalRespondResult["impact_scope"]>;

const STATIC_CARD_KEYS: SecurityCardKey[] = ["status", "restore", "budget", "governance"];
const DRAG_THRESHOLD = 8;
const CARD_CLEARANCE = 14;
const CARD_STEP = 18;
const BOARD_INSET_X = 22;
const BOARD_INSET_TOP = 140;
const BOARD_INSET_BOTTOM = 24;
const DEFAULT_CARD_SIZE: CardSize = { width: 316, height: 236 };
const FALLBACK_POSITION: CardPosition = { x: BOARD_INSET_X, y: BOARD_INSET_TOP };
const SECURITY_DETAIL_PAGE_SIZE = 8;
const ALL_AUDIT_TYPES = "__all__";

function getRiskColor(risk: RiskLevel) {
  if (risk === "red") return "red" as const;
  if (risk === "yellow") return "amber" as const;
  return "green" as const;
}

function getStatusColor(status: SecurityStatus) {
  switch (status) {
    case "pending_confirmation":
      return "amber" as const;
    case "intercepted":
    case "execution_error":
      return "red" as const;
    case "recovered":
    case "recoverable":
      return "green" as const;
    default:
      return "gray" as const;
  }
}

function formatRpcError(error: unknown) {
  if (error instanceof JsonRpcClientError) {
    const details = [error.message];

    if (error.code !== null) {
      details.push(`code ${error.code}`);
    }

    if (error.traceId) {
      details.push(`trace ${error.traceId}`);
    }

    return details.join(" · ");
  }

  return error instanceof Error ? error.message : "安全审批提交失败";
}

function formatCurrency(value: number) {
  return `¥${value.toFixed(2)}`;
}

function formatTokenCount(value: number) {
  return value.toLocaleString("zh-CN");
}

function formatDateTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatImpactScopeSummary(impactScope: AgentSecurityApprovalRespondResult["impact_scope"] | undefined) {
  if (!impactScope) {
    return "影响范围待确认";
  }

  return `${impactScope.files.length} 文件 / ${impactScope.webpages.length} 网页 / ${impactScope.apps.length} 应用`;
}

function formatBooleanLabel(value: boolean) {
  return value ? "是" : "否";
}

function getPendingSecurityStatus(pendingCount: number, fallbackStatus: SecurityStatus) {
  if (pendingCount > 0) {
    return "pending_confirmation";
  }

  return fallbackStatus === "pending_confirmation" ? "normal" : fallbackStatus;
}

function getVisiblePendingItems(items: ApprovalRequest[], limit: number) {
  return items.slice(0, Math.max(0, limit));
}

function renderDetailEntryList(items: string[], emptyCopy: string, keyPrefix: string) {
  if (!items.length) {
    return <p className="security-page__detail-copy">{emptyCopy}</p>;
  }

  return (
    <ul className="security-page__detail-group-list">
      {items.map((item, index) => (
        <li key={`${keyPrefix}-${index}-${item}`} className="security-page__detail-group-item">
          {item}
        </li>
      ))}
    </ul>
  );
}

function getAuditTypeKey(record: AuditRecord) {
  return record.type.trim() || "unknown";
}

function groupAuditRecordsByType(records: AuditRecord[]) {
  return records.reduce<Record<string, AuditRecord[]>>((groups, record) => {
    const key = getAuditTypeKey(record);
    const nextItems = groups[key] ?? [];
    groups[key] = [...nextItems, record];
    return groups;
  }, {});
}

function filterAuditRecordsByType(records: AuditRecord[], typeFilter: string) {
  if (typeFilter === ALL_AUDIT_TYPES) {
    return records;
  }

  return records.filter((record) => getAuditTypeKey(record) === typeFilter);
}

function formatPageWindow(page: { offset: number; total: number }, itemCount: number) {
  if (page.total <= 0 || itemCount <= 0) {
    return "0 / 0";
  }

  const start = page.offset + 1;
  const end = Math.min(page.offset + itemCount, page.total);
  return `${start}-${end} / ${page.total}`;
}

function isFocusedScreenTask(detail: AgentTaskDetailGetResult | null) {
  return detail?.task.source_type === "screen_capture" || detail?.task.intent?.name === "screen_analyze";
}

function formatFocusedTaskFailure(detail: AgentTaskDetailGetResult | null) {
  if (!detail) {
    return "当前 task 还没有失败记录。";
  }

  const parts = [
    detail.runtime_summary.latest_failure_category,
    detail.runtime_summary.latest_failure_code,
    detail.runtime_summary.latest_failure_summary,
  ].filter((value): value is string => typeof value === "string" && value.trim().length > 0);

  return parts.length > 0 ? parts.join(" · ") : "当前 task 还没有失败记录。";
}

function formatCitationEvidence(citation: AgentTaskDetailGetResult["citations"][number]) {
  return [citation.label, citation.source_type, citation.source_ref].filter((value) => value.trim().length > 0).join(" · ");
}

function formatArtifactEvidence(artifact: AgentTaskDetailGetResult["artifacts"][number]) {
  const headline = artifact.title.trim() || artifact.artifact_type;
  const location = artifact.path.trim() || artifact.artifact_id;
  return [headline, location].filter((value) => value.trim().length > 0).join(" · ");
}

function resolveSecurityDetailTaskId(args: {
  activeDetailKey: SecurityCardKey | null;
  approvalLookup: Map<string, ApprovalRequest>;
  approvalSnapshot: ApprovalRequest | null;
  lastResolvedApproval: SecurityRespondOutcome | null;
  latestRestorePoint: RecoveryPoint | null;
  restorePointSnapshot: RecoveryPoint | null;
  subscribedTaskId: string | null;
}) {
  const { activeDetailKey, approvalLookup, approvalSnapshot, lastResolvedApproval, latestRestorePoint, restorePointSnapshot, subscribedTaskId } = args;

  if (activeDetailKey === "restore") {
    return restorePointSnapshot?.task_id ?? latestRestorePoint?.task_id ?? lastResolvedApproval?.response.task.task_id ?? subscribedTaskId ?? null;
  }

  if (activeDetailKey?.startsWith("approval:")) {
    return approvalLookup.get(activeDetailKey)?.task_id ?? approvalSnapshot?.task_id ?? subscribedTaskId ?? null;
  }

  return subscribedTaskId ?? lastResolvedApproval?.response.task.task_id ?? latestRestorePoint?.task_id ?? null;
}

function mergePendingApproval(current: SecurityModuleData, payload: ApprovalPendingNotification): SecurityModuleData {
  const exists = current.pending.some((item) => item.approval_id === payload.approval_request.approval_id);
  const nextPendingItems = exists
    ? current.pending.map((item) => (item.approval_id === payload.approval_request.approval_id ? payload.approval_request : item))
    : [payload.approval_request, ...current.pending];
  const nextTotal = exists ? current.pendingPage.total : current.pendingPage.total + 1;
  const pending = getVisiblePendingItems(nextPendingItems, current.pendingPage.limit);

  return {
    ...current,
    summary: {
      ...current.summary,
      security_status: getPendingSecurityStatus(exists ? current.summary.pending_authorizations : current.summary.pending_authorizations + 1, current.summary.security_status),
      pending_authorizations: exists ? current.summary.pending_authorizations : current.summary.pending_authorizations + 1,
    },
    pending,
    pendingPage: {
      ...current.pendingPage,
      total: nextTotal,
      has_more: current.pendingPage.offset + pending.length < nextTotal,
    },
  };
}

function clampValue(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function clampPosition(value: CardPosition, bounds: BoardBounds) {
  return {
    x: clampValue(value.x, bounds.minX, bounds.maxX),
    y: clampValue(value.y, bounds.minY, bounds.maxY),
  };
}

function buildAxisPositions(min: number, max: number) {
  if (max <= min) {
    return [Math.round(min)];
  }

  const values: number[] = [];

  for (let value = min; value <= max; value += CARD_STEP) {
    values.push(Math.round(value));
  }

  if (values[values.length - 1] !== Math.round(max)) {
    values.push(Math.round(max));
  }

  return Array.from(new Set(values));
}

function getBoardGrid(cardCount: number, canvasWidth: number, canvasHeight: number): BoardGrid {
  let bestGrid: BoardGrid = { columns: cardCount, rows: 1 };
  let bestScore = Number.NEGATIVE_INFINITY;

  for (let columns = 1; columns <= cardCount; columns += 1) {
    const rows = Math.ceil(cardCount / columns);
    const width = (canvasWidth - BOARD_INSET_X * 2 - CARD_CLEARANCE * (columns - 1)) / columns;
    const height = (canvasHeight - BOARD_INSET_TOP - BOARD_INSET_BOTTOM - CARD_CLEARANCE * (rows - 1)) / rows;
    const score = Math.min(width, height);

    if (score > bestScore) {
      bestGrid = { columns, rows };
      bestScore = score;
    }
  }

  return bestGrid;
}

function getBoardCardSize(canvasWidth: number, canvasHeight: number, grid: BoardGrid): CardSize {
  const width = Math.floor((canvasWidth - BOARD_INSET_X * 2 - CARD_CLEARANCE * (grid.columns - 1)) / grid.columns);
  const height = Math.floor((canvasHeight - BOARD_INSET_TOP - BOARD_INSET_BOTTOM - CARD_CLEARANCE * (grid.rows - 1)) / grid.rows);

  return {
    width: clampValue(width, 228, DEFAULT_CARD_SIZE.width),
    height: clampValue(height, 172, DEFAULT_CARD_SIZE.height),
  } satisfies CardSize;
}

function getBoardBounds(canvasWidth: number, canvasHeight: number, size: CardSize): BoardBounds {
  return {
    minX: Math.min(BOARD_INSET_X, Math.max(0, canvasWidth - size.width)),
    minY: Math.min(BOARD_INSET_TOP, Math.max(0, canvasHeight - size.height)),
    maxX: Math.max(BOARD_INSET_X, canvasWidth - size.width - BOARD_INSET_X),
    maxY: Math.max(BOARD_INSET_TOP, canvasHeight - size.height - BOARD_INSET_BOTTOM),
  } satisfies BoardBounds;
}

function buildBoardCandidates(bounds: BoardBounds) {
  const positions: CardPosition[] = [];
  const xs = buildAxisPositions(bounds.minX, bounds.maxX);
  const ys = buildAxisPositions(bounds.minY, bounds.maxY);

  for (const y of ys) {
    for (const x of xs) {
      positions.push({ x, y });
    }
  }

  return positions;
}

function overlapsOccupied(position: CardPosition, occupied: CardPosition[], size: CardSize) {
  return occupied.some((item) => {
    const separatedHorizontally = position.x + size.width + CARD_CLEARANCE <= item.x || item.x + size.width + CARD_CLEARANCE <= position.x;
    const separatedVertically = position.y + size.height + CARD_CLEARANCE <= item.y || item.y + size.height + CARD_CLEARANCE <= position.y;

    return !(separatedHorizontally || separatedVertically);
  });
}

function resolveSettledPosition(target: CardPosition, occupied: CardPosition[], layout: BoardLayout) {
  const clampedTarget = clampPosition(target, layout.bounds);

  if (!overlapsOccupied(clampedTarget, occupied, layout.size)) {
    return clampedTarget;
  }

  let bestCandidate = clampedTarget;
  let bestDistance = Number.POSITIVE_INFINITY;

  for (const candidate of layout.candidates) {
    if (overlapsOccupied(candidate, occupied, layout.size)) {
      continue;
    }

    const distance = Math.hypot(candidate.x - clampedTarget.x, candidate.y - clampedTarget.y);

    if (distance < bestDistance) {
      bestCandidate = candidate;
      bestDistance = distance;
    }
  }

  return bestDistance === Number.POSITIVE_INFINITY ? null : bestCandidate;
}

function getDefaultCardTargets(keys: SecurityCardKey[], bounds: BoardBounds, grid: BoardGrid, size: CardSize) {
  const availableWidth = Math.max(size.width, bounds.maxX - bounds.minX + size.width);
  const availableHeight = Math.max(size.height, bounds.maxY - bounds.minY + size.height);
  const gridHeight = grid.rows * size.height + Math.max(0, grid.rows - 1) * CARD_CLEARANCE;
  const gridStartY = bounds.minY + Math.max(0, (availableHeight - gridHeight) / 2);
  const positions: Record<string, CardPosition> = {};

  keys.forEach((key, index) => {
    const row = Math.floor(index / grid.columns);
    const indexInRow = index % grid.columns;
    const remainingCards = keys.length - row * grid.columns;
    const cardsInRow = Math.min(grid.columns, remainingCards);
    const rowWidth = cardsInRow * size.width + Math.max(0, cardsInRow - 1) * CARD_CLEARANCE;
    const gridStartX = bounds.minX + Math.max(0, (availableWidth - rowWidth) / 2);

    positions[key] = {
      x: gridStartX + indexInRow * (size.width + CARD_CLEARANCE),
      y: gridStartY + row * (size.height + CARD_CLEARANCE),
    };
  });

  return positions;
}

function normalizeCardPositions(keys: SecurityCardKey[], targets: Record<string, CardPosition>, layout: BoardLayout) {
  const nextPositions: Record<string, CardPosition> = {};
  const occupied: CardPosition[] = [];

  for (const key of keys) {
    const baseTarget = targets[key] ?? { x: layout.bounds.minX, y: layout.bounds.minY };
    const settledPosition = resolveSettledPosition(baseTarget, occupied, layout) ?? clampPosition(baseTarget, layout.bounds);

    nextPositions[key] = settledPosition;
    occupied.push(settledPosition);
  }

  return nextPositions;
}

function resolveActiveSafetyDetail(args: {
  activeDetailKey: SecurityCardKey | null;
  approvalLookup: Map<string, ApprovalRequest>;
  approvalSnapshot: ApprovalRequest | null;
  restorePointSnapshot: RecoveryPoint | null;
  moduleData: SecurityModuleData;
}): {
  approval: ApprovalRequest | null;
  restorePoint: RecoveryPoint | null;
} {
  const { activeDetailKey, approvalLookup, approvalSnapshot, restorePointSnapshot, moduleData } = args;

  if (activeDetailKey === "restore") {
    const liveRestorePoint = moduleData.summary.latest_restore_point;
    const resolvedRestorePoint =
      restorePointSnapshot
        ? liveRestorePoint && liveRestorePoint.recovery_point_id === restorePointSnapshot.recovery_point_id
          ? liveRestorePoint
          : restorePointSnapshot
        : liveRestorePoint;

    return {
      approval: null,
      restorePoint: resolvedRestorePoint,
    };
  }

  if (!activeDetailKey?.startsWith("approval:")) {
    return {
      approval: null,
      restorePoint: null,
    };
  }

  const liveApproval = approvalLookup.get(activeDetailKey) ?? null;
  const resolvedApproval =
    liveApproval && approvalSnapshot && liveApproval.approval_id === approvalSnapshot.approval_id
      ? liveApproval
      : liveApproval ?? approvalSnapshot;

  return {
    approval: resolvedApproval,
    restorePoint: null,
  };
}

function getCardPreview(
  key: SecurityCardKey,
  moduleData: SecurityModuleData,
  approvalLookup: Map<string, ApprovalRequest>,
  activeApprovalOverride?: ApprovalRequest | null,
  activeRestorePointOverride?: RecoveryPoint | null,
): SecurityCardPreview {
  if (key === "status") {
    return {
      eyebrow: "security status",
      title: "安全态势",
      badgeLabel: moduleData.summary.security_status,
      badgeColor: getStatusColor(moduleData.summary.security_status),
      headline: `${moduleData.summary.pending_authorizations} 条待确认`,
      supporting: moduleData.pending.length > 0 ? moduleData.pending[0]!.operation_name : "当前没有待处理审批",
      meta: `已加载 ${moduleData.pending.length} / ${moduleData.pendingPage.total} 条审批`,
      emphasis: "number",
      icon: ShieldCheck,
    };
  }

  if (key === "restore") {
    const restorePoint = activeRestorePointOverride ?? moduleData.summary.latest_restore_point;

    return {
      eyebrow: "restore point",
      title: "恢复点",
      badgeLabel: restorePoint ? "restore point" : "no restore point",
      badgeColor: "orange",
      headline: restorePoint?.summary ?? "当前无可用恢复点",
      supporting: restorePoint ? `task ${restorePoint.task_id}` : "等待新的恢复点快照",
      meta: restorePoint ? `${formatDateTime(restorePoint.created_at)} · ${restorePoint.objects.length} 个对象` : "restore timeline pending",
      icon: History,
    };
  }

  if (key === "budget") {
    const tokenCost = moduleData.summary.token_cost_summary;

    return {
      eyebrow: "token / cost",
      title: "Token / 成本",
      badgeLabel: formatCurrency(tokenCost.current_task_cost),
      badgeColor: "blue",
      headline: `${formatTokenCount(tokenCost.current_task_tokens)} tokens`,
      supporting: `当前任务 ${formatCurrency(tokenCost.current_task_cost)} · 当日 ${formatCurrency(tokenCost.today_cost)}`,
      meta: `单任务上限 ${formatTokenCount(tokenCost.single_task_limit)} tokens`,
      icon: Wallet,
    };
  }

  if (key === "governance") {
    const latestRestorePoint = moduleData.summary.latest_restore_point;

    return {
      eyebrow: "audit trail",
      title: "审计记录",
      badgeLabel: latestRestorePoint ? "restore ready" : "audit",
      badgeColor: latestRestorePoint ? "orange" : "gray",
      headline: latestRestorePoint?.summary ?? "查看任务与全局审计",
      supporting: latestRestorePoint ? `最近恢复点 ${formatDateTime(latestRestorePoint.created_at)}` : "只保留后端真实返回的审计与恢复信息",
      meta: moduleData.summary.pending_authorizations > 0 ? `${moduleData.summary.pending_authorizations} 条待确认` : "无待确认授权",
      icon: Siren,
    };
  }

  const approval = activeApprovalOverride ?? approvalLookup.get(key);

  if (!approval) {
    return {
      eyebrow: "pending approval",
      title: "待确认授权",
      badgeLabel: "missing",
      badgeColor: "gray",
      headline: "授权记录已移除",
      supporting: "对应待确认授权已经被处理或刷新覆盖。",
      meta: "card no longer available",
      icon: Siren,
    };
  }

  return {
    eyebrow: "pending approval",
    title: approval.operation_name,
    badgeLabel: approval.risk_level,
    badgeColor: getRiskColor(approval.risk_level),
    headline: approval.target_object,
    supporting: approval.reason,
    meta: formatDateTime(approval.created_at),
    icon: Siren,
  };
}

export function SecurityApp() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const dataMode = "rpc" as const;
  const [moduleData, setModuleData] = useState<SecurityModuleData | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [activeApprovalId, setActiveApprovalId] = useState<string | null>(null);
  const [approvalSnapshot, setApprovalSnapshot] = useState<ApprovalRequest | null>(null);
  const [restorePointSnapshot, setRestorePointSnapshot] = useState<RecoveryPoint | null>(null);
  const [routeDrivenDetailKey, setRouteDrivenDetailKey] = useState<SecurityCardKey | null>(null);
  const [subscribedTaskId, setSubscribedTaskId] = useState<string | null>(null);
  const [lastResolvedApproval, setLastResolvedApproval] = useState<SecurityRespondOutcome | null>(null);
  const [rememberRuleByApprovalId, setRememberRuleByApprovalId] = useState<Record<string, boolean>>({});
  const [pendingListData, setPendingListData] = useState<SecurityPendingListData | null>(null);
  const [pendingListError, setPendingListError] = useState<string | null>(null);
  const [pendingListLoading, setPendingListLoading] = useState(false);
  const [pendingOffset, setPendingOffset] = useState(0);
  const [restorePointsData, setRestorePointsData] = useState<SecurityRestorePointListData | null>(null);
  const [restorePointsError, setRestorePointsError] = useState<string | null>(null);
  const [restorePointsLoading, setRestorePointsLoading] = useState(false);
  const [restoreScope, setRestoreScope] = useState<SecurityRestoreScope>("focused_task");
  const [restoreOffset, setRestoreOffset] = useState(0);
  const [auditRecordsData, setAuditRecordsData] = useState<SecurityAuditRecordListData | null>(null);
  const [auditRecordsError, setAuditRecordsError] = useState<string | null>(null);
  const [auditRecordsLoading, setAuditRecordsLoading] = useState(false);
  const [focusedTaskDetail, setFocusedTaskDetail] = useState<AgentTaskDetailGetResult | null>(null);
  const [focusedTaskDetailError, setFocusedTaskDetailError] = useState<string | null>(null);
  const [auditScope, setAuditScope] = useState<SecurityAuditScope>("focused_task");
  const [auditOffset, setAuditOffset] = useState(0);
  const [auditTypeFilter, setAuditTypeFilter] = useState<string>(ALL_AUDIT_TYPES);
  const [activeRestorePointId, setActiveRestorePointId] = useState<string | null>(null);
  const [activeRestoreRequestId, setActiveRestoreRequestId] = useState<string | null>(null);
  const [cardPositions, setCardPositions] = useState<Record<string, CardPosition>>({});
  const [cardStack, setCardStack] = useState<SecurityCardKey[]>([]);
  const [cardSize, setCardSize] = useState<CardSize>(DEFAULT_CARD_SIZE);
  const [draggingKey, setDraggingKey] = useState<SecurityCardKey | null>(null);
  const [activeDetailKey, setActiveDetailKey] = useState<SecurityCardKey | null>(null);
  const [boardReady, setBoardReady] = useState(false);
  const approvalLookup = useMemo(
    () => new Map((moduleData?.pending ?? []).map((approval) => [`approval:${approval.approval_id}`, approval] as const)),
    [moduleData?.pending],
  );
  const pendingCardKeys = useMemo(
    () => (moduleData?.pending ?? []).map((approval) => `approval:${approval.approval_id}` as SecurityCardKey),
    [moduleData?.pending],
  );
  const cardKeys = useMemo(() => [...STATIC_CARD_KEYS, ...pendingCardKeys], [pendingCardKeys]);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const refreshSequenceRef = useRef(0);
  const taskRefreshPlan = getDashboardTaskSecurityRefreshPlan(dataMode);
  const activeSnapshotState = useMemo(
    () =>
      resolveDashboardSafetySnapshotLifecycle({
        activeDetailKey,
        approvalSnapshot,
        restorePointSnapshot,
        routeDrivenDetailKey,
        subscribedTaskId,
      }),
    [activeDetailKey, approvalSnapshot, restorePointSnapshot, routeDrivenDetailKey, subscribedTaskId],
  );
  const focusedTaskId = useMemo(
    () =>
      resolveSecurityDetailTaskId({
        activeDetailKey,
        approvalLookup,
        approvalSnapshot: activeSnapshotState.approvalSnapshot,
        lastResolvedApproval,
        latestRestorePoint: moduleData?.summary.latest_restore_point ?? null,
        restorePointSnapshot: activeSnapshotState.restorePointSnapshot,
        subscribedTaskId: activeSnapshotState.subscribedTaskId,
      }),
    [
      activeDetailKey,
      activeSnapshotState.approvalSnapshot,
      activeSnapshotState.restorePointSnapshot,
      activeSnapshotState.subscribedTaskId,
      approvalLookup,
      lastResolvedApproval,
      moduleData?.summary.latest_restore_point,
    ],
  );
  const restoreFilterTaskId = restoreScope === "focused_task" ? focusedTaskId : null;
  const auditFilterTaskId = auditScope === "focused_task" ? focusedTaskId : null;
  const rpcAuditRequiresTaskContext = moduleData?.source === "rpc";
  const canLoadAuditRecords = Boolean(auditFilterTaskId) || !rpcAuditRequiresTaskContext;
  const activeAuditRecordsData = useMemo(
    () => (auditRecordsData?.taskId === auditFilterTaskId ? auditRecordsData : null),
    [auditFilterTaskId, auditRecordsData],
  );
  const auditFilterOptions = useMemo(
    () => Array.from(new Set((activeAuditRecordsData?.items ?? []).map((record) => getAuditTypeKey(record)))),
    [activeAuditRecordsData?.items],
  );
  const filteredAuditRecords = useMemo(
    () => filterAuditRecordsByType(activeAuditRecordsData?.items ?? [], auditTypeFilter),
    [activeAuditRecordsData?.items, auditTypeFilter],
  );

  const queueRpcRefresh = useCallback(() => {
    const nextSequence = ++refreshSequenceRef.current;

    void loadSecurityModuleRpcData()
      .then((nextData) => {
        if (refreshSequenceRef.current === nextSequence) {
          setLoadError(null);
          setModuleData(nextData);
        }
      })
      .catch((error) => {
        if (refreshSequenceRef.current === nextSequence) {
          setLoadError(formatRpcError(error));
        }
      });
  }, []);

  useEffect(() => {
    const nextSequence = ++refreshSequenceRef.current;
    setLoadError(null);

    setModuleData(null);
    void loadSecurityModuleData("rpc")
      .then((nextData) => {
        if (refreshSequenceRef.current === nextSequence) {
          setModuleData(nextData);
        }
      })
      .catch((error) => {
        if (refreshSequenceRef.current === nextSequence) {
          setLoadError(formatRpcError(error));
        }
      });

    const unsubscribe = subscribeApprovalPending((payload) => {
      setModuleData((current) => (current ? mergePendingApproval(current, payload) : current));
      queueRpcRefresh();
    });

    return () => {
      unsubscribe();
    };
  }, [dataMode, queueRpcRefresh]);

  useEffect(() => {
    setCardStack((currentStack) => {
      const preserved = currentStack.filter((key) => cardKeys.includes(key));
      const additions = cardKeys.filter((key) => !preserved.includes(key));
      return [...preserved, ...additions];
    });
  }, [cardKeys]);

  useEffect(() => {
    if (!moduleData) {
      return;
    }

    setRememberRuleByApprovalId((current) =>
      moduleData.pending.reduce<Record<string, boolean>>((nextState, approval) => {
        nextState[approval.approval_id] = current[approval.approval_id] ?? false;
        return nextState;
      }, {}),
    );
  }, [moduleData]);

  useEffect(() => {
    if (
      routeDrivenDetailKey !== activeSnapshotState.routeDrivenDetailKey ||
      approvalSnapshot !== activeSnapshotState.approvalSnapshot ||
      restorePointSnapshot !== activeSnapshotState.restorePointSnapshot ||
      subscribedTaskId !== activeSnapshotState.subscribedTaskId
    ) {
      setRouteDrivenDetailKey(activeSnapshotState.routeDrivenDetailKey as SecurityCardKey | null);
      setApprovalSnapshot(activeSnapshotState.approvalSnapshot);
      setRestorePointSnapshot(activeSnapshotState.restorePointSnapshot);
      setSubscribedTaskId(activeSnapshotState.subscribedTaskId);
    }
  }, [activeSnapshotState, approvalSnapshot, restorePointSnapshot, routeDrivenDetailKey, subscribedTaskId]);

  useEffect(() => {
    if (
      activeDetailKey &&
      !shouldRetainDashboardSafetyActiveDetail({
        activeDetailKey,
        approvalSnapshot: activeSnapshotState.approvalSnapshot,
        cardKeys,
      })
    ) {
      setActiveDetailKey(null);
    }
  }, [activeDetailKey, activeSnapshotState.approvalSnapshot, cardKeys]);

  useEffect(() => {
    if (activeDetailKey === "status") {
      setPendingOffset(0);
    }
  }, [activeDetailKey]);

  useEffect(() => {
    if (!focusedTaskId && restoreScope === "focused_task") {
      setRestoreScope("all");
    }
  }, [focusedTaskId, restoreScope]);

  useEffect(() => {
    if (!focusedTaskId && auditScope === "focused_task" && !rpcAuditRequiresTaskContext) {
      setAuditScope("all");
    }
  }, [auditScope, focusedTaskId, rpcAuditRequiresTaskContext]);

  useEffect(() => {
    if (rpcAuditRequiresTaskContext && auditScope === "all") {
      setAuditScope("focused_task");
    }
  }, [auditScope, rpcAuditRequiresTaskContext]);

  useEffect(() => {
    if (activeDetailKey === "restore") {
      setRestoreOffset(0);
    }
  }, [activeDetailKey, restoreScope, focusedTaskId]);

  useEffect(() => {
    if (activeDetailKey === "governance") {
      setAuditOffset(0);
      setAuditTypeFilter(ALL_AUDIT_TYPES);
    }
  }, [activeDetailKey, auditScope, focusedTaskId]);

  useEffect(() => {
    if (!auditFilterOptions.length && auditTypeFilter !== ALL_AUDIT_TYPES) {
      setAuditTypeFilter(ALL_AUDIT_TYPES);
      return;
    }

    if (auditTypeFilter !== ALL_AUDIT_TYPES && !auditFilterOptions.includes(auditTypeFilter)) {
      setAuditTypeFilter(ALL_AUDIT_TYPES);
    }
  }, [auditFilterOptions, auditTypeFilter]);

  useEffect(() => {
    setActionError(null);
  }, [activeDetailKey]);

  const bringCardToFront = useCallback((key: SecurityCardKey) => {
    setCardStack((currentStack) => [...currentStack.filter((item) => item !== key), key]);
  }, []);

  useEffect(() => {
    if (!moduleData) {
      return;
    }

    const routeResolution = resolveDashboardSafetyNavigationRoute({
      locationState: location.state,
      livePending: moduleData.pending,
      liveRestorePoint: moduleData.summary.latest_restore_point,
    });

    if (!routeResolution.shouldClearRouteState) {
      return;
    }

    setApprovalSnapshot(routeResolution.approvalSnapshot);
    setRestorePointSnapshot(routeResolution.restorePointSnapshot);
    setRouteDrivenDetailKey(routeResolution.activeDetailKey);
    setSubscribedTaskId(routeResolution.routedTaskId);

    if (routeResolution.activeDetailKey) {
      setActiveDetailKey(routeResolution.activeDetailKey);
      bringCardToFront(routeResolution.activeDetailKey);
    } else {
      setActiveDetailKey(null);
    }

    navigate(location.pathname, { replace: true, state: null });
  }, [bringCardToFront, location.pathname, location.state, moduleData, navigate]);

  useEffect(() => {
    if (dataMode !== "rpc" || !subscribedTaskId) {
      return;
    }

    return subscribeTask(subscribedTaskId, () => {
      queueRpcRefresh();
    });
  }, [dataMode, queueRpcRefresh, subscribedTaskId]);

  useEffect(() => {
    if (!focusedTaskId) {
      setFocusedTaskDetail(null);
      setFocusedTaskDetailError(null);
      return;
    }

    let disposed = false;
    setFocusedTaskDetail(null);
    setFocusedTaskDetailError(null);
    void loadSecurityFocusedTaskDetail(focusedTaskId, moduleData?.source ?? "rpc")
      .then((detail) => {
        if (!disposed) {
          setFocusedTaskDetail(detail);
          setFocusedTaskDetailError(null);
        }
      })
      .catch((error) => {
        if (!disposed) {
          setFocusedTaskDetail(null);
          setFocusedTaskDetailError(formatRpcError(error));
        }
      });

    return () => {
      disposed = true;
    };
  }, [focusedTaskId, moduleData?.source]);

  useEffect(() => {
    if (!moduleData || activeDetailKey !== "status") {
      return;
    }

    let disposed = false;
    setPendingListLoading(true);
    setPendingListError(null);

    void loadSecurityPendingApprovals(moduleData.source, {
      limit: SECURITY_DETAIL_PAGE_SIZE,
      offset: pendingOffset,
    })
      .then((nextData) => {
        if (!disposed) {
          setPendingListData(nextData);
          setPendingListError(null);
        }
      })
      .catch((error) => {
        if (!disposed) {
          setPendingListError(formatRpcError(error));
        }
      })
      .finally(() => {
        if (!disposed) {
          setPendingListLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [activeDetailKey, moduleData, pendingOffset]);

  useEffect(() => {
    if (!moduleData || (activeDetailKey !== "restore" && activeDetailKey !== "governance")) {
      return;
    }

    let disposed = false;
    setRestorePointsLoading(true);
    setRestorePointsError(null);

    void loadSecurityRestorePoints(moduleData.source, {
      limit: SECURITY_DETAIL_PAGE_SIZE,
      offset: restoreOffset,
      taskId: restoreFilterTaskId,
    })
      .then((nextData) => {
        if (disposed) {
          return;
        }

        setRestorePointsData(nextData);
        setRestorePointsError(null);
        setActiveRestorePointId((current) => {
          if (current && nextData.items.some((item) => item.recovery_point_id === current)) {
            return current;
          }

          const preferredId =
            activeSnapshotState.restorePointSnapshot?.recovery_point_id ??
            moduleData.summary.latest_restore_point?.recovery_point_id ??
            nextData.items[0]?.recovery_point_id ??
            null;
          return preferredId;
        });
      })
      .catch((error) => {
        if (!disposed) {
          setRestorePointsError(formatRpcError(error));
        }
      })
      .finally(() => {
        if (!disposed) {
          setRestorePointsLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [activeDetailKey, activeSnapshotState.restorePointSnapshot, moduleData, restoreFilterTaskId, restoreOffset]);

  useEffect(() => {
    if (!moduleData || activeDetailKey !== "governance") {
      return;
    }

    if (!canLoadAuditRecords) {
      setAuditRecordsData(null);
      setAuditRecordsError(null);
      setAuditRecordsLoading(false);
      return;
    }

    let disposed = false;
    setAuditRecordsLoading(true);
    setAuditRecordsError(null);

    void loadSecurityAuditRecords(moduleData.source, auditFilterTaskId, {
      limit: SECURITY_DETAIL_PAGE_SIZE,
      offset: auditOffset,
    })
      .then((nextData) => {
        if (!disposed) {
          setAuditRecordsData(nextData);
          setAuditRecordsError(null);
        }
      })
      .catch((error) => {
        if (!disposed) {
          setAuditRecordsError(formatRpcError(error));
        }
      })
      .finally(() => {
        if (!disposed) {
          setAuditRecordsLoading(false);
        }
      });

    return () => {
      disposed = true;
    };
  }, [activeDetailKey, auditFilterTaskId, auditOffset, canLoadAuditRecords, moduleData]);

  const getBoardLayout = useCallback(() => {
    const canvas = canvasRef.current;

    if (!canvas || !cardKeys.length) {
      return null;
    }

    const grid = getBoardGrid(cardKeys.length, canvas.clientWidth, canvas.clientHeight);
    const size = getBoardCardSize(canvas.clientWidth, canvas.clientHeight, grid);
    const bounds = getBoardBounds(canvas.clientWidth, canvas.clientHeight, size);

    return {
      bounds,
      size,
      grid,
      candidates: buildBoardCandidates(bounds),
    } satisfies BoardLayout;
  }, [cardKeys.length]);

  const getSettledCardPosition = useCallback(
    (key: SecurityCardKey, target: CardPosition, positions: Record<string, CardPosition>) => {
      const layout = getBoardLayout();

      if (!layout) {
        return target;
      }

      const occupied = cardKeys.filter((item) => item !== key).map((item) => positions[item]).filter(Boolean);
      return resolveSettledPosition(target, occupied, layout) ?? positions[key] ?? clampPosition(target, layout.bounds);
    },
    [cardKeys, getBoardLayout],
  );

  const getClampedCardPosition = useCallback(
    (target: CardPosition) => {
      const layout = getBoardLayout();

      if (!layout) {
        return target;
      }

      return clampPosition(target, layout.bounds);
    },
    [getBoardLayout],
  );

  useLayoutEffect(() => {
    const syncBoardLayout = () => {
      const layout = getBoardLayout();

      if (!layout) {
        return;
      }

      setCardSize(layout.size);
      setCardPositions((currentPositions) => {
        const defaults = getDefaultCardTargets(cardKeys, layout.bounds, layout.grid, layout.size);
        const targets = cardKeys.reduce<Record<string, CardPosition>>((accumulator, key) => {
          accumulator[key] = currentPositions[key] ?? defaults[key];
          return accumulator;
        }, {});
        return normalizeCardPositions(cardKeys, targets, layout);
      });
      setBoardReady(true);
    };

    syncBoardLayout();
    window.addEventListener("resize", syncBoardLayout);

    return () => {
      window.removeEventListener("resize", syncBoardLayout);
    };
  }, [cardKeys, getBoardLayout]);

  useDashboardEscapeHandler({
    enabled: activeDetailKey !== null,
    handleEscape: () => {
      setActionError(null);
      setActiveDetailKey(null);
    },
    priority: 220,
  });

  const openTaskDetail = useCallback(
    (taskId: string) => {
      navigateToDashboardTaskDetail(navigate, taskId);
    },
    [navigate],
  );

  const openPendingApprovalDetail = useCallback(
    (approval: ApprovalRequest) => {
      const approvalKey = `approval:${approval.approval_id}` as SecurityCardKey;

      setActionError(null);
      setApprovalSnapshot(approval);
      setRestorePointSnapshot(null);
      setRouteDrivenDetailKey(approvalKey);
      setSubscribedTaskId(approval.task_id);
      setActiveDetailKey(approvalKey);

      if (cardKeys.includes(approvalKey)) {
        bringCardToFront(approvalKey);
      }
    },
    [bringCardToFront, cardKeys],
  );

  if (!moduleData) {
    return (
      <main className="app-shell security-page">
        <div className="security-page__frame">
          <div className="security-surface security-page__topbar">
            <Text>{loadError ? `安全页同步失败：${loadError}` : "正在同步安全数据..."}</Text>
          </div>
        </div>
      </main>
    );
  }

  const sourceBadgeLabel = moduleData.source === "rpc" ? "RPC" : "MOCK";
  const sourceBadgeColor = moduleData.source === "rpc" ? "green" : "amber";

  const handleRespond = async (approval: ApprovalRequest, decision: ApprovalDecision, rememberRule: boolean) => {
    setActionError(null);
    setActiveApprovalId(approval.approval_id);

    try {
      const result = await respondToApproval(approval, decision, rememberRule, moduleData.source);
      const approvalRouteKey = `approval:${approval.approval_id}` as const;

      setModuleData((current) => {
        if (!current) {
          return current;
        }

        const pending = current.pending.filter((item) => item.approval_id !== approval.approval_id);
        const nextTotal = Math.max(0, current.pendingPage.total - 1);
        const nextPendingCount = Math.max(0, current.summary.pending_authorizations - 1);

        return {
          ...current,
          summary: {
            ...current.summary,
            pending_authorizations: nextPendingCount,
            security_status: getPendingSecurityStatus(nextPendingCount, current.summary.security_status),
          },
          pending: getVisiblePendingItems(pending, current.pendingPage.limit),
          pendingPage: {
            ...current.pendingPage,
            total: nextTotal,
            has_more: current.pendingPage.offset + pending.length < nextTotal,
          },
          rpcContext: {
            serverTime: result.rpcContext.serverTime ?? current.rpcContext.serverTime,
            warnings: Array.from(new Set([...current.rpcContext.warnings, ...result.rpcContext.warnings])),
          },
        };
      });
      setRememberRuleByApprovalId((current) => {
        const nextState = { ...current };
        delete nextState[approval.approval_id];
        return nextState;
      });
      setSubscribedTaskId(result.response.task.task_id);
      if (isSecurityRestoreRespondResult(result.response)) {
        if (routeDrivenDetailKey === approvalRouteKey) {
          setApprovalSnapshot(null);
          setRouteDrivenDetailKey(null);
        }
        setRestorePointSnapshot(result.response.recovery_point);
        setApprovalSnapshot(null);
        setRouteDrivenDetailKey("restore");
        setActiveDetailKey("restore");
        setActiveRestorePointId(result.response.recovery_point.recovery_point_id);
        bringCardToFront("restore");
      }
      setLastResolvedApproval(result);
      for (const queryKey of taskRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }

      if (moduleData.source === "rpc") {
        queueRpcRefresh();
      }
    } catch (error) {
      setActionError(formatRpcError(error));
    } finally {
      setActiveApprovalId(null);
    }
  };

  const handleApplyRestore = async (restorePoint: RecoveryPoint) => {
    setActionError(null);
    setActiveRestoreRequestId(restorePoint.recovery_point_id);

    try {
      const result = await applySecurityRestorePoint(restorePoint, moduleData.source);
      setRestorePointSnapshot(result.response.recovery_point);
      setSubscribedTaskId(result.response.task.task_id);
      for (const queryKey of taskRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }

      if (moduleData.source === "rpc") {
        queueRpcRefresh();
      }
    } catch (error) {
      setActionError(formatRpcError(error));
    } finally {
      setActiveRestoreRequestId(null);
    }
  };

  const closeDetail = () => {
    setActionError(null);
    setActiveDetailKey(null);
  };

  const releaseDrag = () => {
    dragStateRef.current = null;
    setDraggingKey(null);
  };

  const handleCardPointerDown = (key: SecurityCardKey) => (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return;
    }

    bringCardToFront(key);
    setDraggingKey(key);

    const position = cardPositions[key] ?? FALLBACK_POSITION;

    dragStateRef.current = {
      key,
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      originX: position.x,
      originY: position.y,
      moved: false,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handleCardPointerMove = (key: SecurityCardKey) => (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;

    if (!dragState || dragState.key !== key || dragState.pointerId !== event.pointerId) {
      return;
    }

    const deltaX = event.clientX - dragState.startX;
    const deltaY = event.clientY - dragState.startY;

    if (!dragState.moved) {
      if (Math.hypot(deltaX, deltaY) < DRAG_THRESHOLD) {
        return;
      }

      dragStateRef.current = {
        ...dragState,
        moved: true,
      };
    }

    setCardPositions((currentPositions) => ({
      ...currentPositions,
      // Keep the drag path free while the card is moving so neighboring cards do
      // not block the pointer. Collision avoidance still runs on release.
      [key]: getClampedCardPosition({
        x: dragState.originX + deltaX,
        y: dragState.originY + deltaY,
      }),
    }));
  };

  const handleCardPointerUp = (key: SecurityCardKey) => (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;

    if (!dragState || dragState.key !== key || dragState.pointerId !== event.pointerId) {
      return;
    }

    const travelled = Math.hypot(event.clientX - dragState.startX, event.clientY - dragState.startY);

    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    if (dragState.moved) {
      setCardPositions((currentPositions) => ({
        ...currentPositions,
        [key]: getSettledCardPosition(key, currentPositions[key] ?? FALLBACK_POSITION, currentPositions),
      }));
    }

    releaseDrag();

    if (!dragState.moved && travelled < DRAG_THRESHOLD) {
      setActiveDetailKey(key);
    }
  };

  const handleCardPointerCancel = (_key: SecurityCardKey) => (event: PointerEvent<HTMLDivElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    releaseDrag();
  };

  const handleCardKeyDown = (key: SecurityCardKey) => (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      bringCardToFront(key);
      setActiveDetailKey(key);
    }
  };

  const renderStatusDetail = () => {
    const latestRestorePoint = moduleData.summary.latest_restore_point;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">安全状态</p>
            <p className="security-page__detail-value">{moduleData.summary.security_status}</p>
            <p className="security-page__detail-copy">待确认授权 {moduleData.summary.pending_authorizations} 条</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当前页审批</p>
            <p className="security-page__detail-value">{moduleData.pending.length}</p>
            <p className="security-page__detail-copy">
              total {moduleData.pendingPage.total} · has_more {String(moduleData.pendingPage.has_more)}
            </p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">最新恢复点</p>
            <p className="security-page__detail-value">{latestRestorePoint ? formatDateTime(latestRestorePoint.created_at) : "暂无"}</p>
            <p className="security-page__detail-copy">{latestRestorePoint?.summary ?? "当前没有恢复点"}</p>
          </article>
        </div>

        <div className="security-page__detail-section">
          <div className="security-page__detail-toolbar">
            <p className="security-page__detail-label">待确认授权</p>
            {pendingListData ? <p className="security-page__detail-copy">当前页 {formatPageWindow(pendingListData.page, pendingListData.items.length)}</p> : null}
          </div>

          {pendingListLoading ? <div className="security-page__detail-note">正在同步待确认授权…</div> : null}
          {pendingListError ? <div className="security-page__detail-callout">待确认授权同步失败：{pendingListError}</div> : null}
          {!pendingListLoading && !pendingListError && (pendingListData?.items.length ?? 0) === 0 ? (
            <p className="security-page__empty-state">当前没有待确认授权。</p>
          ) : null}

          {(pendingListData?.items.length ?? 0) > 0 ? (
            <div className="security-page__detail-selection-list">
              {pendingListData!.items.map((approval) => (
                <button
                  key={approval.approval_id}
                  type="button"
                  className="security-page__detail-selection-item"
                  onClick={() => openPendingApprovalDetail(approval)}
                >
                  <span className="security-page__detail-label">{formatDateTime(approval.created_at)}</span>
                  <strong className="security-page__detail-selection-title">{approval.operation_name}</strong>
                  <span className="security-page__detail-copy">
                    risk {approval.risk_level} · task {approval.task_id} · {approval.target_object}
                  </span>
                </button>
              ))}
            </div>
          ) : null}

          {pendingListData ? (
            <div className="security-page__detail-pagination">
              <div className="security-page__detail-pagination-actions">
                <Button
                  variant="soft"
                  color="gray"
                  disabled={pendingListLoading || pendingListData.page.offset <= 0}
                  onClick={() => setPendingOffset((current) => Math.max(0, current - pendingListData.page.limit))}
                >
                  上一页
                </Button>
                <Button
                  variant="soft"
                  color="gray"
                  disabled={pendingListLoading || !pendingListData.page.has_more}
                  onClick={() => setPendingOffset((current) => current + pendingListData.page.limit)}
                >
                  下一页
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    );
  };

  const renderRestoreDetail = (restorePoint: RecoveryPoint | null) => {
    const restorePoints = restorePointsData?.items ?? (restorePoint ? [restorePoint] : []);
    const restorePageWindow = restorePointsData ? formatPageWindow(restorePointsData.page, restorePoints.length) : null;
    const restorePageStep = restorePointsData?.page.limit ?? SECURITY_DETAIL_PAGE_SIZE;
    const selectedRestorePoint =
      restorePoints.find((item) => item.recovery_point_id === activeRestorePointId) ??
      restorePoint ??
      restorePoints[0] ??
      null;
    const resolvedRestoreOutcome =
      selectedRestorePoint &&
      lastResolvedApproval &&
      isSecurityRestoreRespondResult(lastResolvedApproval.response) &&
      lastResolvedApproval.response.recovery_point.recovery_point_id === selectedRestorePoint.recovery_point_id
        ? lastResolvedApproval.response
        : null;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-section">
          <div className="security-page__detail-toolbar">
            <p className="security-page__detail-label">恢复点</p>
            {restorePageWindow ? <p className="security-page__detail-copy">当前页 {restorePageWindow}</p> : null}
          </div>

          <div className="security-page__detail-filter-row">
            <div className="security-page__detail-filter-group">
              {focusedTaskId ? (
                <button
                  type="button"
                  className={`security-page__detail-filter-chip${restoreScope === "focused_task" ? " is-active" : ""}`}
                  onClick={() => setRestoreScope("focused_task")}
                >
                  当前 task
                </button>
              ) : null}
              <button
                type="button"
                className={`security-page__detail-filter-chip${restoreScope === "all" ? " is-active" : ""}`}
                onClick={() => setRestoreScope("all")}
                >
                  全部恢复点
                </button>
              </div>
            </div>

          {restorePointsLoading ? <div className="security-page__detail-note">正在同步恢复点…</div> : null}
          {restorePointsError ? <div className="security-page__detail-callout">恢复点同步失败：{restorePointsError}</div> : null}
          {!restorePointsLoading && !restorePointsError && restorePoints.length === 0 ? (
            <p className="security-page__empty-state">
              {restoreFilterTaskId ? `当前 task ${restoreFilterTaskId} 还没有恢复点。` : "当前没有恢复点。"}
            </p>
          ) : null}
        </div>

        {selectedRestorePoint ? (
          <>
            <div className="security-page__detail-grid">
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">恢复点 ID</p>
                <p className="security-page__detail-value security-page__detail-value--mono">{selectedRestorePoint.recovery_point_id}</p>
                <p className="security-page__detail-copy">task {selectedRestorePoint.task_id}</p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">创建时间</p>
                <p className="security-page__detail-value">{formatDateTime(selectedRestorePoint.created_at)}</p>
                <p className="security-page__detail-copy">{selectedRestorePoint.summary}</p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">对象数量</p>
                <p className="security-page__detail-value">{selectedRestorePoint.objects.length}</p>
                <p className="security-page__detail-copy">
                  当前页 {restorePoints.length}
                  {restorePointsData ? ` / ${restorePointsData.page.total}` : ""}
                </p>
              </article>
            </div>

            <article className="security-page__detail-list-item">
              <p className="security-page__detail-label">影响对象</p>
              {renderDetailEntryList(selectedRestorePoint.objects, "该恢复点没有对象清单。", "restore-objects")}
            </article>

            {resolvedRestoreOutcome ? (
              <>
                <div className="security-page__detail-grid">
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">恢复结果</p>
                    <p className="security-page__detail-value">{resolvedRestoreOutcome.applied ? "completed" : "pending"}</p>
                    <p className="security-page__detail-copy">task {resolvedRestoreOutcome.task.status}</p>
                  </article>
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">审计记录</p>
                    <p className="security-page__detail-value security-page__detail-value--mono">
                      {resolvedRestoreOutcome.audit_record?.audit_id ?? "未生成"}
                    </p>
                    <p className="security-page__detail-copy">
                      {resolvedRestoreOutcome.audit_record
                        ? `${resolvedRestoreOutcome.audit_record.result} · ${formatDateTime(resolvedRestoreOutcome.audit_record.created_at)}`
                        : "最近一次恢复响应没有返回 audit_record。"}
                    </p>
                  </article>
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">状态气泡</p>
                    <p className="security-page__detail-value">
                      {resolvedRestoreOutcome.bubble_message ? "已返回" : "未返回"}
                    </p>
                    <p className="security-page__detail-copy">
                      {resolvedRestoreOutcome.bubble_message?.text ?? "最近一次恢复响应没有返回 bubble_message。"}
                    </p>
                  </article>
                </div>

                {resolvedRestoreOutcome.audit_record ? (
                  <article className="security-page__detail-list-item">
                    <p className="security-page__detail-label">审计摘要</p>
                    <p className="security-page__detail-copy">
                      {resolvedRestoreOutcome.audit_record.action} · {resolvedRestoreOutcome.audit_record.summary}
                    </p>
                    <p className="security-page__detail-copy">
                      target {resolvedRestoreOutcome.audit_record.target} · result {resolvedRestoreOutcome.audit_record.result}
                    </p>
                  </article>
                ) : null}
              </>
            ) : null}

            <Flex align="center" gap="3" wrap="wrap">
              <Button variant="soft" color="gray" onClick={() => openTaskDetail(selectedRestorePoint.task_id)}>
                查看关联任务
                <ArrowUpRight className="h-4 w-4" />
              </Button>
              <Button
                color="amber"
                variant="solid"
                disabled={activeRestoreRequestId === selectedRestorePoint.recovery_point_id}
                onClick={() => void handleApplyRestore(selectedRestorePoint)}
              >
                {activeRestoreRequestId === selectedRestorePoint.recovery_point_id ? "提交中..." : "申请恢复"}
              </Button>
            </Flex>
          </>
        ) : null}

        {restorePoints.length > 1 ? (
          <div className="security-page__detail-section">
            <div className="security-page__detail-selection-list">
              {restorePoints.map((item) => (
                <button
                  key={item.recovery_point_id}
                  type="button"
                  className={`security-page__detail-selection-item${item.recovery_point_id === selectedRestorePoint?.recovery_point_id ? " is-active" : ""}`}
                  onClick={() => setActiveRestorePointId(item.recovery_point_id)}
                >
                  <span className="security-page__detail-label">{formatDateTime(item.created_at)}</span>
                  <strong className="security-page__detail-selection-title">{item.summary}</strong>
                  <span className="security-page__detail-copy">
                    {item.recovery_point_id} · {item.objects.length} 个对象 · task {item.task_id}
                  </span>
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {restorePointsData ? (
          <div className="security-page__detail-pagination">
            <div className="security-page__detail-pagination-actions">
              <Button
                variant="soft"
                color="gray"
                disabled={restorePointsLoading || restorePointsData.page.offset <= 0}
                onClick={() => setRestoreOffset((current) => Math.max(0, current - restorePageStep))}
              >
                上一页
              </Button>
              <Button
                variant="soft"
                color="gray"
                disabled={restorePointsLoading || !restorePointsData.page.has_more}
                onClick={() => setRestoreOffset((current) => current + restorePageStep)}
              >
                下一页
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    );
  };

  const renderBudgetDetail = () => {
    const tokenCost = moduleData.summary.token_cost_summary;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当前任务 Tokens</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.current_task_tokens)}</p>
            <p className="security-page__detail-copy">当前成本 {formatCurrency(tokenCost.current_task_cost)}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当日 Tokens</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.today_tokens)}</p>
            <p className="security-page__detail-copy">当日成本 {formatCurrency(tokenCost.today_cost)}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">单任务上限</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.single_task_limit)}</p>
            <p className="security-page__detail-copy">tokens</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当日上限</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.daily_limit)}</p>
            <p className="security-page__detail-copy">tokens</p>
          </article>
        </div>
      </div>
    );
  };

  const renderGovernanceDetail = () => {
    const latestRestorePoint = moduleData.summary.latest_restore_point;
    const auditGroups = groupAuditRecordsByType(filteredAuditRecords);
    const auditPageWindow = activeAuditRecordsData ? formatPageWindow(activeAuditRecordsData.page, activeAuditRecordsData.items.length) : null;
    const auditPageStep = activeAuditRecordsData?.page.limit ?? SECURITY_DETAIL_PAGE_SIZE;
    const focusedTaskApproval = focusedTaskDetail?.approval_request ?? null;
    const focusedTaskRestorePoint = focusedTaskDetail?.security_summary.latest_restore_point ?? null;
    const focusedTaskIsScreenTask = isFocusedScreenTask(focusedTaskDetail);
    const focusedTaskEvidence = focusedTaskDetail?.citations.map(formatCitationEvidence) ?? [];
    const focusedTaskArtifacts = focusedTaskDetail?.artifacts.map(formatArtifactEvidence) ?? [];

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">审计范围</p>
            <p className="security-page__detail-value">{auditFilterTaskId ? "当前 task" : "全局"}</p>
            <p className="security-page__detail-copy">{auditFilterTaskId ? `task ${auditFilterTaskId}` : "全部任务"}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">待确认授权</p>
            <p className="security-page__detail-value">{moduleData.summary.pending_authorizations}</p>
            <p className="security-page__detail-copy">{moduleData.summary.security_status}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">最新恢复点</p>
            <p className="security-page__detail-value">{latestRestorePoint ? formatDateTime(latestRestorePoint.created_at) : "暂无"}</p>
            <p className="security-page__detail-copy">{latestRestorePoint?.summary ?? "当前没有恢复点"}</p>
          </article>
        </div>

        {focusedTaskId ? (
          <div className="security-page__detail-section">
            <div className="security-page__detail-toolbar">
              <p className="security-page__detail-label">{focusedTaskIsScreenTask ? "当前屏幕任务治理链" : "当前 task 治理链"}</p>
              <Button variant="soft" color="gray" onClick={() => openTaskDetail(focusedTaskId)}>
                查看关联任务
                <ArrowUpRight className="h-4 w-4" />
              </Button>
            </div>

            {focusedTaskDetailError ? <div className="security-page__detail-callout">任务详情同步失败：{focusedTaskDetailError}</div> : null}
            {!focusedTaskDetail && !focusedTaskDetailError ? <div className="security-page__detail-note">正在同步当前 task 的正式治理对象…</div> : null}

            {focusedTaskDetail ? (
              <>
                <div className="security-page__detail-grid">
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">当前 task</p>
                    <p className="security-page__detail-value">{focusedTaskDetail.task.title}</p>
                    <p className="security-page__detail-copy">
                      task {focusedTaskDetail.task.task_id} · {focusedTaskDetail.task.intent?.name ?? focusedTaskDetail.task.source_type} · {focusedTaskDetail.task.status}
                    </p>
                  </article>
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">正式授权锚点</p>
                    <p className="security-page__detail-value">{focusedTaskApproval ? focusedTaskApproval.operation_name : "无活跃授权"}</p>
                    <p className="security-page__detail-copy">
                      {focusedTaskApproval
                        ? `${focusedTaskApproval.risk_level} · ${focusedTaskApproval.status} · ${focusedTaskApproval.target_object}`
                        : "当前 task 没有活跃 approval_request。"}
                    </p>
                  </article>
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">最近失败</p>
                    <p className="security-page__detail-value">
                      {focusedTaskDetail.runtime_summary.latest_failure_category ?? focusedTaskDetail.runtime_summary.latest_failure_code ?? "暂无"}
                    </p>
                    <p className="security-page__detail-copy">{formatFocusedTaskFailure(focusedTaskDetail)}</p>
                  </article>
                  <article className="security-page__detail-card">
                    <p className="security-page__detail-label">恢复锚点</p>
                    <p className="security-page__detail-value">
                      {focusedTaskRestorePoint ? formatDateTime(focusedTaskRestorePoint.created_at) : "暂无"}
                    </p>
                    <p className="security-page__detail-copy">{focusedTaskRestorePoint?.summary ?? "当前 task 没有 recovery_point。"}</p>
                  </article>
                </div>

                <article className="security-page__detail-list-item">
                  <p className="security-page__detail-label">正式引用</p>
                  {renderDetailEntryList(
                    focusedTaskEvidence,
                    focusedTaskIsScreenTask ? "当前屏幕任务还没有 formal citation。" : "当前 task 还没有 formal citation。",
                    "focused-task-citation",
                  )}
                </article>

                <article className="security-page__detail-list-item">
                  <p className="security-page__detail-label">正式产物</p>
                  {renderDetailEntryList(focusedTaskArtifacts, "当前 task 还没有可回看的正式产物。", "focused-task-artifact")}
                </article>
              </>
            ) : null}
          </div>
        ) : null}

        <div className="security-page__detail-section">
          <div className="security-page__detail-toolbar">
            <p className="security-page__detail-label">审计记录</p>
            {focusedTaskId ? (
              <Button variant="soft" color="gray" onClick={() => openTaskDetail(focusedTaskId)}>
                查看关联任务
                <ArrowUpRight className="h-4 w-4" />
              </Button>
            ) : null}
          </div>

          <div className="security-page__detail-filter-row">
            <div className="security-page__detail-filter-group">
              <button
                type="button"
                className={`security-page__detail-filter-chip${auditScope === "focused_task" ? " is-active" : ""}`}
                disabled={!focusedTaskId}
                onClick={() => setAuditScope("focused_task")}
              >
                当前 task
              </button>
              <button
                type="button"
                className={`security-page__detail-filter-chip${auditScope === "all" ? " is-active" : ""}`}
                disabled={rpcAuditRequiresTaskContext}
                onClick={() => setAuditScope("all")}
              >
                全部审计
              </button>
            </div>
            <div className="security-page__detail-filter-group">
              <button
                type="button"
                className={`security-page__detail-filter-chip${auditTypeFilter === ALL_AUDIT_TYPES ? " is-active" : ""}`}
                onClick={() => setAuditTypeFilter(ALL_AUDIT_TYPES)}
              >
                全部类型
              </button>
              {auditFilterOptions.map((type) => (
                <button
                  key={type}
                  type="button"
                  className={`security-page__detail-filter-chip${auditTypeFilter === type ? " is-active" : ""}`}
                  onClick={() => setAuditTypeFilter(type)}
                >
                  {type}
                </button>
              ))}
            </div>
            {auditPageWindow ? <p className="security-page__detail-copy">当前页 {auditPageWindow}</p> : null}
          </div>

          {!canLoadAuditRecords ? (
            <div className="security-page__detail-callout">
              当前后端仅支持按 task 查看审计记录。请从带 task 上下文的安全入口或任务详情进入。
            </div>
          ) : null}
          {auditRecordsLoading ? <div className="security-page__detail-note">正在同步审计记录…</div> : null}
          {auditRecordsError ? <div className="security-page__detail-callout">审计记录同步失败：{auditRecordsError}</div> : null}
          {!auditRecordsLoading && !auditRecordsError && activeAuditRecordsData && activeAuditRecordsData.items.length === 0 ? (
            <p className="security-page__empty-state">
              {auditFilterTaskId ? `当前 task ${auditFilterTaskId} 还没有审计记录。` : "当前没有审计记录。"}
            </p>
          ) : null}
          {!auditRecordsLoading &&
          !auditRecordsError &&
          activeAuditRecordsData &&
          activeAuditRecordsData.items.length > 0 &&
          filteredAuditRecords.length === 0 ? (
            <p className="security-page__empty-state">当前筛选条件下没有命中的审计记录。</p>
          ) : null}

          {Object.entries(auditGroups).map(([groupKey, records]) => (
            <div key={groupKey} className="security-page__detail-section">
              <div className="security-page__detail-toolbar">
                <p className="security-page__detail-label">
                  {groupKey} · {records.length} 条
                </p>
              </div>
              <div className="security-page__detail-list">
                {records.map((record) => (
                  <article key={record.audit_id} className="security-page__detail-list-item">
                    <p className="security-page__detail-label">
                      {formatDateTime(record.created_at)} · {record.action}
                    </p>
                    <p className="security-page__detail-value security-page__detail-value--mono">{record.target}</p>
                    <p className="security-page__detail-copy">{record.summary}</p>
                    <p className="security-page__detail-copy">result {record.result} · audit_id {record.audit_id}</p>
                  </article>
                ))}
              </div>
            </div>
          ))}

          {activeAuditRecordsData ? (
            <div className="security-page__detail-pagination">
              <div className="security-page__detail-pagination-actions">
                <Button
                  variant="soft"
                  color="gray"
                  disabled={auditRecordsLoading || activeAuditRecordsData.page.offset <= 0}
                  onClick={() => setAuditOffset((current) => Math.max(0, current - auditPageStep))}
                >
                  上一页
                </Button>
                <Button
                  variant="soft"
                  color="gray"
                  disabled={auditRecordsLoading || !activeAuditRecordsData.page.has_more}
                  onClick={() => setAuditOffset((current) => current + auditPageStep)}
                >
                  下一页
                </Button>
              </div>
            </div>
          ) : null}
        </div>

        {moduleData.rpcContext.warnings.length > 0 ? (
          <div className="security-page__detail-callout">warnings：{moduleData.rpcContext.warnings.join("；")}</div>
        ) : null}
      </div>
    );
  };

  const renderApprovalDetail = (approval: ApprovalRequest | undefined, snapshotOnly = false) => {
    if (!approval) {
      return <p className="security-page__empty-state">该待确认授权已经从当前列表中移除。</p>;
    }

    const rememberRule = rememberRuleByApprovalId[approval.approval_id] ?? false;
    // Preserve the last approval payload on the same detail view so operators can
    // verify the final decision after the pending card disappears from the list.
    const resolvedApprovalOutcome =
      lastResolvedApproval &&
      isSecurityApprovalRespondResult(lastResolvedApproval.response) &&
      lastResolvedApproval.response.authorization_record.approval_id === approval.approval_id
        ? lastResolvedApproval
        : null;
    const resolvedApprovalResponse =
      resolvedApprovalOutcome && isSecurityApprovalRespondResult(resolvedApprovalOutcome.response)
        ? resolvedApprovalOutcome.response
        : null;
    const authorizationRecord = resolvedApprovalResponse?.authorization_record;
    const responseTask = resolvedApprovalResponse?.task;
    const bubbleMessage = resolvedApprovalResponse?.bubble_message;
    const impactScope = resolvedApprovalResponse?.impact_scope as ImpactScopeDetails | undefined;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">操作名称</p>
            <p className="security-page__detail-value">{approval.operation_name}</p>
            <p className="security-page__detail-copy">风险 {approval.risk_level} · 状态 {approval.status}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">目标对象</p>
            <p className="security-page__detail-value">{approval.target_object}</p>
            <p className="security-page__detail-copy">task {approval.task_id}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">审批标识</p>
            <p className="security-page__detail-value security-page__detail-value--mono">{approval.approval_id}</p>
            <p className="security-page__detail-copy">{formatDateTime(approval.created_at)}</p>
          </article>
        </div>

        <article className="security-page__detail-list-item">
          <p className="security-page__detail-label">原因说明</p>
          <p className="security-page__detail-copy">{approval.reason}</p>
        </article>

        {snapshotOnly ? <div className="security-page__detail-callout">该审批快照已脱离实时待处理列表，当前不能继续提交决策。</div> : null}

        {resolvedApprovalOutcome && authorizationRecord && responseTask ? (
          <>
            <div className="security-page__detail-grid">
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">响应结果</p>
                <p className="security-page__detail-value">{authorizationRecord.decision}</p>
                <p className="security-page__detail-copy">
                  remember_rule {formatBooleanLabel(authorizationRecord.remember_rule)} · task {responseTask.status}
                </p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">授权记录</p>
                <p className="security-page__detail-value security-page__detail-value--mono">{authorizationRecord.authorization_record_id}</p>
                <p className="security-page__detail-copy">{formatDateTime(authorizationRecord.created_at)} · {authorizationRecord.operator}</p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">影响范围</p>
                <p className="security-page__detail-value">{formatImpactScopeSummary(impactScope)}</p>
                <p className="security-page__detail-copy">
                  工作区外 {formatBooleanLabel(impactScope?.out_of_workspace ?? false)} · 覆盖/删除风险{" "}
                  {formatBooleanLabel(impactScope?.overwrite_or_delete_risk ?? false)}
                </p>
              </article>
            </div>

            <article className="security-page__detail-list-item">
              <p className="security-page__detail-label">Bubble message</p>
              <p className="security-page__detail-copy">{bubbleMessage?.text ?? "最近一次响应没有返回 bubble_message。"}</p>
            </article>

            <article className="security-page__detail-list-item">
              <p className="security-page__detail-label">影响对象</p>
              {renderDetailEntryList(
                [...(impactScope?.files ?? []), ...(impactScope?.webpages ?? []), ...(impactScope?.apps ?? [])],
                "当前没有记录影响对象。",
                "approval-impact",
              )}
            </article>
          </>
        ) : null}

        <label className="security-page__approval-remember">
          <input
            className="security-page__approval-remember-checkbox"
            type="checkbox"
            checked={rememberRule}
            disabled={snapshotOnly || activeApprovalId === approval.approval_id}
            onChange={(event) => {
              const checked = event.currentTarget.checked;
              setRememberRuleByApprovalId((current) => ({
                ...current,
                [approval.approval_id]: checked,
              }));
            }}
          />
          <span className="security-page__approval-remember-copy">
            <span className="security-page__detail-label">remember_rule</span>
            <span className="security-page__detail-copy">后续同类请求沿用这次决策。</span>
          </span>
        </label>

        <div className="security-page__approval-actions">
          <Button color="gray" variant="soft" onClick={() => openTaskDetail(approval.task_id)}>
            查看关联任务
          </Button>
          <Button
            color="gray"
            variant="soft"
            disabled={snapshotOnly || activeApprovalId === approval.approval_id}
            onClick={() => void handleRespond(approval, "deny_once", rememberRule)}
          >
            拒绝
          </Button>
          <Button
            color="amber"
            variant="solid"
            disabled={snapshotOnly || activeApprovalId === approval.approval_id}
            onClick={() => void handleRespond(approval, "allow_once", rememberRule)}
          >
            允许一次
          </Button>
        </div>
      </div>
    );
  };

  const renderDetailBody = (
    key: SecurityCardKey,
    resolvedApproval: ApprovalRequest | null,
    resolvedRestorePoint: RecoveryPoint | null,
    snapshotOnlyApproval: boolean,
  ) => {
    if (key === "status") {
      return renderStatusDetail();
    }

    if (key === "restore") {
      return renderRestoreDetail(resolvedRestorePoint);
    }

    if (key === "budget") {
      return renderBudgetDetail();
    }

    if (key === "governance") {
      return renderGovernanceDetail();
    }

    return renderApprovalDetail(resolvedApproval ?? approvalLookup.get(key) ?? undefined, snapshotOnlyApproval);
  };

  const renderDetailOverlay = () => {
    if (!activeDetailKey) {
      return null;
    }

    const resolvedDetail = resolveActiveSafetyDetail({
      activeDetailKey,
      approvalLookup,
      approvalSnapshot: activeSnapshotState.approvalSnapshot,
      moduleData,
      restorePointSnapshot: activeSnapshotState.restorePointSnapshot,
    });
    const snapshotOnlyApproval = isDashboardSafetyApprovalSnapshotOnly({
      activeDetailKey,
      approvalSnapshot: activeSnapshotState.approvalSnapshot,
      cardKeys,
    });
    const preview = getCardPreview(
      activeDetailKey,
      moduleData,
      approvalLookup,
      resolvedDetail.approval,
      resolvedDetail.restorePoint,
    );

    return (
      <div className="security-page__detail-layer" onClick={closeDetail}>
        <div className="security-page__detail-panel" role="dialog" aria-modal="true" aria-label={`${preview.title}详情`} onClick={(event) => event.stopPropagation()}>
          <section className="security-page__detail-surface">
            <div className="security-page__detail-header">
              <div className="security-page__detail-heading">
                <p className="security-page__eyebrow security-page__eyebrow--detail">{preview.eyebrow}</p>
                <Heading size="8" className="security-page__detail-title">
                  {preview.title}
                </Heading>
                <Text as="p" size="2" className="security-page__detail-description">
                  {preview.supporting}
                </Text>
              </div>

              <div className="security-page__detail-actions">
                <Flex align="center" gap="2" wrap="wrap" justify="end">
                  <Badge color={preview.badgeColor} variant="soft" highContrast className="security-page__detail-badge">
                    {preview.badgeLabel}
                  </Badge>
                  <Badge color={sourceBadgeColor} variant="soft" highContrast className="security-page__detail-badge">
                    {sourceBadgeLabel}
                  </Badge>
                </Flex>
                <button type="button" className="security-page__close-button" onClick={closeDetail} aria-label="关闭详情视图">
                  <X className="security-page__close-icon" />
                </button>
              </div>
            </div>

            <div className="security-page__detail-body">
              {actionError ? <div className="security-page__detail-callout">请求失败：{actionError}</div> : null}
              {renderDetailBody(activeDetailKey, resolvedDetail.approval, resolvedDetail.restorePoint, snapshotOnlyApproval)}
            </div>
          </section>
        </div>
      </div>
    );
  };

  const renderDraggableCard = (key: SecurityCardKey, index: number) => {
    const preview = getCardPreview(key, moduleData, approvalLookup);
    const Icon = preview.icon;
    const isDragging = draggingKey === key;
    const isExpanded = activeDetailKey === key;
    const position = cardPositions[key] ?? FALLBACK_POSITION;
    const headlineClassName = [
      "security-page__card-line",
      preview.emphasis ? `security-page__card-line--${preview.emphasis}` : null,
    ]
      .filter(Boolean)
      .join(" ");

    return (
      <div
        key={key}
        className={`security-page__draggable${isDragging ? " is-dragging" : ""}${isExpanded ? " is-active" : ""}${boardReady ? " is-ready" : ""}`}
        style={{
          height: `${cardSize.height}px`,
          transform: `translate3d(${position.x}px, ${position.y}px, 0)`,
          width: `${cardSize.width}px`,
          zIndex: index + 2,
        }}
        role="button"
        tabIndex={0}
        aria-haspopup="dialog"
        aria-expanded={isExpanded}
        aria-label={`${preview.title}，可拖动并打开详情`}
        onPointerDown={handleCardPointerDown(key)}
        onPointerMove={handleCardPointerMove(key)}
        onPointerUp={handleCardPointerUp(key)}
        onPointerCancel={handleCardPointerCancel(key)}
        onKeyDown={handleCardKeyDown(key)}
      >
        <section className="security-page__card-surface" aria-hidden="true">
          <div className="security-page__card-shell">
            <div className="security-page__card-top">
              <div className="security-page__card-heading">
                <p className="security-page__card-eyebrow">{preview.eyebrow}</p>
                <p className="security-page__card-title">{preview.title}</p>
              </div>
              <Icon className="security-page__card-icon" />
            </div>

            <Badge color={preview.badgeColor} variant="soft" highContrast className="security-page__card-badge">
              {preview.badgeLabel}
            </Badge>

            <p className={headlineClassName}>{preview.headline}</p>
            <p className="security-page__card-copy">{preview.supporting}</p>
            <p className="security-page__card-meta">{preview.meta}</p>
          </div>
        </section>
      </div>
    );
  };

  return (
    <main className="app-shell security-page">
      <div className="security-page__canvas" ref={canvasRef} aria-label="Security 卡片画布">
        <div className="security-page__hero">
          <Text as="p" size="1" className="security-page__eyebrow">
            security
          </Text>
          <Heading size="9" className="security-page__title">
            安全卫士
          </Heading>
          <Flex align="center" gap="2" wrap="wrap" className="security-page__status-strip">
            <Badge color={sourceBadgeColor} variant="soft" highContrast className="security-page__status-badge">
              {sourceBadgeLabel}
            </Badge>
            <Badge color={getStatusColor(moduleData.summary.security_status)} variant="soft" highContrast className="security-page__status-badge">
              {moduleData.summary.security_status}
            </Badge>
            <Badge color={moduleData.summary.pending_authorizations > 0 ? "amber" : "gray"} variant="soft" highContrast className="security-page__status-badge">
              {moduleData.summary.pending_authorizations} pending
            </Badge>
            <Badge color={moduleData.summary.latest_restore_point ? "orange" : "gray"} variant="soft" highContrast className="security-page__status-badge">
              {moduleData.summary.latest_restore_point ? "restore ready" : "no restore"}
            </Badge>
          </Flex>
          {loadError ? <div className="security-page__detail-callout">同步失败：{loadError}</div> : null}
        </div>

        {cardStack.map(renderDraggableCard)}
        {renderDetailOverlay()}
      </div>
    </main>
  );
}
