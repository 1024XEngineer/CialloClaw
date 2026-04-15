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
import { Badge, Box, Button, Flex, Heading, Text } from "@radix-ui/themes";
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
  AuditRecord,
  AgentSecurityRespondResult,
  ApprovalDecision,
  ApprovalPendingNotification,
  ApprovalRequest,
  RecoveryPoint,
  RiskLevel,
  SecurityStatus,
  Task,
} from "@cialloclaw/protocol";
import { JsonRpcClientError } from "@/rpc/client";
import { subscribeApprovalPending, subscribeTask } from "@/rpc/subscriptions";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import {
  isDashboardSafetyApprovalSnapshotOnly,
  resolveDashboardSafetyNavigationRoute,
  resolveDashboardSafetyFocusTarget,
  resolveDashboardSafetySnapshotLifecycle,
  shouldRetainDashboardSafetyActiveDetail,
} from "@/features/dashboard/shared/dashboardSafetyNavigation";
import {
  getInitialDashboardSettingsSnapshot,
  loadDashboardSettingsSnapshot,
} from "@/features/dashboard/shared/dashboardSettingsSnapshot";
import {
  getInitialSecurityModuleData,
  loadSecurityPendingApprovals,
  applySecurityRestorePoint,
  loadSecurityAuditRecords,
  loadSecurityModuleData,
  loadSecurityModuleRpcData,
  loadSecurityRestorePoints,
  respondToApproval,
  type SecurityAuditRecordListData,
  type SecurityModuleData,
  type SecurityPendingListData,
  type SecurityRestoreApplyOutcome,
  type SecurityRestorePointListData,
  type SecurityRespondOutcome,
} from "./securityService";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
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

type ImpactScopeDetails = NonNullable<AgentSecurityRespondResult["impact_scope"]>;

const STATIC_CARD_KEYS: SecurityCardKey[] = ["status", "restore", "budget", "governance"];
const DRAG_THRESHOLD = 8;
const CARD_CLEARANCE = 14;
const CARD_STEP = 18;
const BOARD_INSET_X = 22;
const BOARD_INSET_TOP = 140;
const BOARD_INSET_BOTTOM = 24;
const DEFAULT_CARD_SIZE: CardSize = { width: 248, height: 176 };
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

function getSourceCopy(moduleData: SecurityModuleData) {
  const copySegments: string[] = [];

  if (moduleData.source === "rpc") {
    copySegments.push("来自后端返回，不是本地 mock。");

    if (moduleData.rpcContext.serverTime) {
      copySegments.push(`服务端时间 ${formatDateTime(moduleData.rpcContext.serverTime)}`);
    }

    if (moduleData.rpcContext.warnings.length) {
      copySegments.push(`warnings：${moduleData.rpcContext.warnings.join("；")}`);
    }

    return {
      badge: "LIVE",
      title: "当前显示的是 JSON-RPC 实时数据",
      description: copySegments.join(" · "),
      className: "security-page__source-status--rpc",
    };
  }

  copySegments.push("仅用于前端联调，不是真实后端返回。");

  return {
    badge: "MOCK",
    title: "当前显示的是本地 mock 数据",
    description: copySegments.join(" · "),
    className: "security-page__source-status--mock",
  };
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

function formatImpactScopeSummary(impactScope: AgentSecurityRespondResult["impact_scope"] | undefined) {
  if (!impactScope) {
    return "影响范围待确认";
  }

  return `${impactScope.files.length} 文件 / ${impactScope.webpages.length} 网页 / ${impactScope.apps.length} 应用`;
}

function formatBooleanLabel(value: boolean) {
  return value ? "是" : "否";
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

function formatOptionalDateTime(value: string | null | undefined) {
  return value ? formatDateTime(value) : "—";
}

function formatTaskIntent(intent: Task["intent"]) {
  if (!intent) {
    return "当前没有挂载 intent。";
  }

  const argumentKeys = Object.keys(intent.arguments);

  return [
    `name: ${intent.name}`,
    argumentKeys.length ? `argument_keys: ${argumentKeys.join(", ")}` : "argument_keys: none",
    `argument_count: ${argumentKeys.length}`,
  ].join("\n");
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
    width: clampValue(width, 208, 260),
    height: clampValue(height, 152, 184),
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
}) {
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
  sourceCopy: ReturnType<typeof getSourceCopy>,
  feedback: string | null,
  lastResolvedApproval: SecurityRespondOutcome | null,
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
      supporting: "approval.pending 的实时推送和序列保护仍保持启用。",
      meta: moduleData.source === "rpc"
        ? `已加载 ${moduleData.pending.length} / ${moduleData.pendingPage.total} 条 pending`
        : "等待 RPC 首次刷新",
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
      supporting: restorePoint ? `创建时间 ${formatDateTime(restorePoint.created_at)}` : "等待新的恢复点快照进入安全视图。",
      meta: restorePoint ? `${restorePoint.objects.length} 个影响对象` : "restore timeline pending",
      icon: History,
    };
  }

  if (key === "budget") {
    const tokenCost = moduleData.summary.token_cost_summary;

    return {
      eyebrow: "token / cost",
      title: "预算治理",
      badgeLabel: formatCurrency(tokenCost.current_task_cost),
      badgeColor: "blue",
      headline: `${formatTokenCount(tokenCost.current_task_tokens)} tokens`,
      supporting: `当前任务 ${formatCurrency(tokenCost.current_task_cost)} · 当日 ${formatCurrency(tokenCost.today_cost)}`,
      meta: `单任务上限 ${formatTokenCount(tokenCost.single_task_limit)} tokens`,
      icon: Wallet,
    };
  }

  if (key === "governance") {
    return {
      eyebrow: "impact scope",
      title: "治理说明",
      badgeLabel: sourceCopy.badge,
      badgeColor: moduleData.source === "rpc" ? "green" : "amber",
      headline: moduleData.source === "rpc" ? "实时治理链路在线" : "前端联调视图运行中",
      supporting: lastResolvedApproval
        ? `最近一次审批 ${lastResolvedApproval.response.authorization_record.decision}，任务状态 ${lastResolvedApproval.response.task.status}。`
        : "工作区边界、恢复点和预算治理说明仍聚合在本模块。",
      meta: lastResolvedApproval
        ? formatImpactScopeSummary(lastResolvedApproval.response.impact_scope)
        : feedback ?? `${moduleData.pending.length} 条 pending approvals`,
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
  const [dataMode, setDataMode] = useState<"rpc" | "mock">(() => loadDashboardDataMode("safety"));
  const [moduleData, setModuleData] = useState<SecurityModuleData | null>(null);
  const [settingsSnapshot, setSettingsSnapshot] = useState(() => getInitialDashboardSettingsSnapshot());
  const [loadError, setLoadError] = useState<string | null>(null);
  const [activeApprovalId, setActiveApprovalId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [approvalSnapshot, setApprovalSnapshot] = useState<ApprovalRequest | null>(null);
  const [restorePointSnapshot, setRestorePointSnapshot] = useState<RecoveryPoint | null>(null);
  const [routeDrivenDetailKey, setRouteDrivenDetailKey] = useState<SecurityCardKey | null>(null);
  const [subscribedTaskId, setSubscribedTaskId] = useState<string | null>(null);
  const [lastResolvedApproval, setLastResolvedApproval] = useState<SecurityRespondOutcome | null>(null);
  const [lastRestoreApply, setLastRestoreApply] = useState<SecurityRestoreApplyOutcome | null>(null);
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
  const [auditOffset, setAuditOffset] = useState(0);
  const [auditTypeFilter, setAuditTypeFilter] = useState<string>(ALL_AUDIT_TYPES);
  const [activeRestorePointId, setActiveRestorePointId] = useState<string | null>(null);
  const [activeRestoreRequestId, setActiveRestoreRequestId] = useState<string | null>(null);
  const [decisionHistory, setDecisionHistory] = useState<SecurityRespondOutcome[]>([]);
  const [activeDecisionRecordId, setActiveDecisionRecordId] = useState<string | null>(null);
  const [titleMotionTick, setTitleMotionTick] = useState(0);
  const [cardPositions, setCardPositions] = useState<Record<string, CardPosition>>({});
  const [cardStack, setCardStack] = useState<SecurityCardKey[]>([]);
  const [cardSize, setCardSize] = useState<CardSize>(DEFAULT_CARD_SIZE);
  const [draggingKey, setDraggingKey] = useState<SecurityCardKey | null>(null);
  const [activeDetailKey, setActiveDetailKey] = useState<SecurityCardKey | null>(null);
  const [boardReady, setBoardReady] = useState(false);
  const sourceCopy = useMemo(() => (moduleData ? getSourceCopy(moduleData) : null), [moduleData]);
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
  const taskRefreshPlan = useMemo(() => getDashboardTaskSecurityRefreshPlan(dataMode), [dataMode]);
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
  const activeAuditRecordsData = useMemo(
    () => (focusedTaskId && auditRecordsData?.taskId === focusedTaskId ? auditRecordsData : null),
    [auditRecordsData, focusedTaskId],
  );
  const auditFilterOptions = useMemo(
    () => Array.from(new Set((activeAuditRecordsData?.items ?? []).map((record) => getAuditTypeKey(record)))),
    [activeAuditRecordsData?.items],
  );
  const filteredAuditRecords = useMemo(
    () => filterAuditRecordsByType(activeAuditRecordsData?.items ?? [], auditTypeFilter),
    [activeAuditRecordsData?.items, auditTypeFilter],
  );
  const governanceRecords = useMemo(() => {
    const nextRecords = [...decisionHistory];

    if (lastResolvedApproval) {
      const nextRecordId = lastResolvedApproval.response.authorization_record.authorization_record_id;

      if (!nextRecords.some((item) => item.response.authorization_record.authorization_record_id === nextRecordId)) {
        nextRecords.unshift(lastResolvedApproval);
      }
    }

    return nextRecords;
  }, [decisionHistory, lastResolvedApproval]);
  const activeGovernanceOutcome = useMemo(() => {
    if (!governanceRecords.length) {
      return null;
    }

    if (!activeDecisionRecordId) {
      return governanceRecords[0] ?? null;
    }

    return (
      governanceRecords.find((item) => item.response.authorization_record.authorization_record_id === activeDecisionRecordId) ??
      governanceRecords[0] ??
      null
    );
  }, [activeDecisionRecordId, governanceRecords]);

  const queueRpcRefresh = useCallback(() => {
    if (dataMode !== "rpc") {
      return;
    }

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
  }, [dataMode]);

  useEffect(() => {
    saveDashboardDataMode("safety", dataMode);
  }, [dataMode]);

  useEffect(() => {
    // The safety board needs a stable settings snapshot even before the detailed
    // governance panel opens, so load the same shared snapshot used by mirror.
    let disposed = false;

    if (dataMode === "mock") {
      setSettingsSnapshot(getInitialDashboardSettingsSnapshot());
      return () => {
        disposed = true;
      };
    }

    void loadDashboardSettingsSnapshot("rpc").then((nextSnapshot) => {
      if (!disposed) {
        setSettingsSnapshot(nextSnapshot);
      }
    });

    return () => {
      disposed = true;
    };
  }, [dataMode]);

  useEffect(() => {
    const nextSequence = ++refreshSequenceRef.current;
    setLoadError(null);

    if (dataMode === "mock") {
      setModuleData(getInitialSecurityModuleData());
      return;
    }

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
      setFeedback(`收到新的待确认授权：${payload.approval_request.operation_name} · task ${payload.task_id}`);
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
    if (activeDetailKey === "restore") {
      setRestoreOffset(0);
    }
  }, [activeDetailKey, restoreScope, focusedTaskId]);

  useEffect(() => {
    if (activeDetailKey === "governance") {
      setAuditOffset(0);
      setAuditTypeFilter(ALL_AUDIT_TYPES);
    }
  }, [activeDetailKey, focusedTaskId]);

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
    if (!governanceRecords.length) {
      if (activeDecisionRecordId !== null) {
        setActiveDecisionRecordId(null);
      }
      return;
    }

    if (!activeDecisionRecordId || !governanceRecords.some((item) => item.response.authorization_record.authorization_record_id === activeDecisionRecordId)) {
      setActiveDecisionRecordId(governanceRecords[0]?.response.authorization_record.authorization_record_id ?? null);
    }
  }, [activeDecisionRecordId, governanceRecords]);

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

    if (routeResolution.feedback) {
      setFeedback((current) => current ?? routeResolution.feedback);
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
    if (!moduleData || activeDetailKey !== "governance" || !focusedTaskId) {
      return;
    }

    let disposed = false;
    setAuditRecordsLoading(true);
    setAuditRecordsError(null);

    void loadSecurityAuditRecords(moduleData.source, focusedTaskId, {
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
  }, [activeDetailKey, auditOffset, focusedTaskId, moduleData]);

  const handleTitleClick = useCallback(() => {
    setTitleMotionTick((currentTick) => currentTick + 1);
  }, []);

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

  useEffect(() => {
    if (!activeDetailKey) {
      return;
    }

    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        setActiveDetailKey(null);
      }
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [activeDetailKey]);

  const openTaskDetail = useCallback(
    (taskId: string) => {
      navigate(resolveDashboardModuleRoutePath("tasks"), {
        state: {
          focusTaskId: taskId,
          openDetail: true,
        },
      });
    },
    [navigate],
  );

  const openPendingApprovalDetail = useCallback(
    (approval: ApprovalRequest) => {
      const approvalKey = `approval:${approval.approval_id}` as SecurityCardKey;

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
        <DashboardMockToggle enabled={dataMode === "mock"} onToggle={() => setDataMode((current) => (current === "rpc" ? "mock" : "rpc"))} />
      </main>
    );
  }

  const resolvedSourceCopy = sourceCopy ?? getSourceCopy(moduleData);

  const handleRespond = async (approval: ApprovalRequest, decision: ApprovalDecision, rememberRule: boolean) => {
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
      if (routeDrivenDetailKey === approvalRouteKey) {
        setApprovalSnapshot(null);
        setRouteDrivenDetailKey(null);
      }
      setLastResolvedApproval(result);
      setDecisionHistory((current) => {
        const nextRecordId = result.response.authorization_record.authorization_record_id;
        const deduped = current.filter((item) => item.response.authorization_record.authorization_record_id !== nextRecordId);
        return [result, ...deduped].slice(0, 10);
      });
      setActiveDecisionRecordId(result.response.authorization_record.authorization_record_id);
      setFeedback(
        `${result.response.bubble_message?.text ?? "已更新安全审批状态。"} · ${result.response.authorization_record.decision} · remember_rule ${result.response.authorization_record.remember_rule ? "on" : "off"} · task ${result.response.task.task_id} / ${result.response.task.status} · ${formatImpactScopeSummary(result.response.impact_scope)}`,
      );
      for (const queryKey of taskRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }

      if (moduleData.source === "rpc") {
        queueRpcRefresh();
      }
    } catch (error) {
      setFeedback(formatRpcError(error));
    } finally {
      setActiveApprovalId(null);
    }
  };

  const handleApplyRestore = async (restorePoint: RecoveryPoint) => {
    setActiveRestoreRequestId(restorePoint.recovery_point_id);

    try {
      const result = await applySecurityRestorePoint(restorePoint, moduleData.source);
      setLastRestoreApply(result);
      setRestorePointSnapshot(result.response.recovery_point);
      setSubscribedTaskId(result.response.task.task_id);
      setFeedback(
        `${result.response.bubble_message?.text ?? "恢复点操作已提交。"} · task ${result.response.task.task_id} / ${result.response.task.status} · restore ${result.response.recovery_point.recovery_point_id}`,
      );
      for (const queryKey of taskRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }

      if (moduleData.source === "rpc") {
        queueRpcRefresh();
      }
    } catch (error) {
      setFeedback(formatRpcError(error));
    } finally {
      setActiveRestoreRequestId(null);
    }
  };

  const closeDetail = () => {
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
      [key]: getSettledCardPosition(
        key,
        {
          x: dragState.originX + deltaX,
          y: dragState.originY + deltaY,
        },
        currentPositions,
      ),
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

  const renderStatusDetail = () => (
    <div className="security-page__detail-stack">
      <div className="security-page__detail-grid">
        <article className="security-page__detail-card">
          <p className="security-page__detail-label">安全状态</p>
          <p className="security-page__detail-value">{moduleData.summary.security_status}</p>
          <p className="security-page__detail-copy">当前待确认授权 {moduleData.summary.pending_authorizations} 条，仍按 task 视角统一治理。</p>
        </article>
        <article className="security-page__detail-card">
          <p className="security-page__detail-label">数据来源</p>
          <p className="security-page__detail-value">{resolvedSourceCopy.badge}</p>
          <p className="security-page__detail-copy">{resolvedSourceCopy.description}</p>
        </article>
        <article className="security-page__detail-card">
          <p className="security-page__detail-label">分页状态</p>
          <p className="security-page__detail-value">{moduleData.pendingPage.has_more ? "还有更多" : "当前页完整"}</p>
          <p className="security-page__detail-copy">
            已加载 {moduleData.pending.length} 条 · total {moduleData.pendingPage.total} · has_more {String(moduleData.pendingPage.has_more)}
          </p>
        </article>
      </div>

      <div className="security-page__detail-note">
        approval.pending 推送仍先合并到前端状态，再走顺序保护的 RPC 刷新，避免旧响应覆盖较新的安全视图。
      </div>

      <div className="security-page__detail-section">
        <div className="security-page__detail-toolbar">
          <div>
            <p className="security-page__detail-label">待确认授权列表</p>
            <p className="security-page__detail-copy">点击任一授权可进入该条审批详情，再继续执行允许/拒绝或打开关联 task。</p>
          </div>
          {pendingListData ? (
            <p className="security-page__detail-copy">当前页 {formatPageWindow(pendingListData.page, pendingListData.items.length)}</p>
          ) : null}
        </div>

        {pendingListLoading ? <div className="security-page__detail-note">正在同步待确认授权列表…</div> : null}
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
            <p className="security-page__detail-copy">当前页用于独立浏览待确认授权，不依赖画布里已经铺开的卡片数量。</p>
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

      <div className="security-page__detail-note">
        当前摘要分页：limit {moduleData.pendingPage.limit} · offset {moduleData.pendingPage.offset} · total {moduleData.pendingPage.total} · has_more {String(moduleData.pendingPage.has_more)}
      </div>
    </div>
  );

  const renderRestoreDetail = (restorePoint: RecoveryPoint | null) => {
    const restorePoints = restorePointsData?.items ?? (restorePoint ? [restorePoint] : []);
    const restorePageWindow = restorePointsData ? formatPageWindow(restorePointsData.page, restorePoints.length) : null;
    const canFilterRestoreByTask = Boolean(focusedTaskId);
    const restorePageStep = restorePointsData?.page.limit ?? SECURITY_DETAIL_PAGE_SIZE;
    const restoreScopeDescription =
      restoreScope === "focused_task" && focusedTaskId
        ? `当前仅显示 task ${focusedTaskId} 的恢复点；切到“全部恢复点”后会跨 task 浏览。`
        : "当前显示全局可见恢复点；如有 task 上下文，可切回当前 task 视图。";
    const selectedRestorePoint =
      restorePoints.find((item) => item.recovery_point_id === activeRestorePointId) ??
      restorePoint ??
      restorePoints[0] ??
      null;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-note">
          {focusedTaskId
            ? `当前安全上下文已挂到 task ${focusedTaskId}。恢复操作会先进入授权，再继续执行回滚。`
            : "当前安全上下文没有绑定到具体 task，因此恢复点默认按全局范围查看。"}
        </div>

        <div className="security-page__detail-section">
          <div className="security-page__detail-toolbar">
            <div>
              <p className="security-page__detail-label">恢复点范围</p>
              <p className="security-page__detail-copy">{restoreScopeDescription}</p>
            </div>
          </div>

          <div className="security-page__detail-filter-row">
            <div className="security-page__detail-filter-group">
              {canFilterRestoreByTask ? (
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
            {restorePageWindow ? <p className="security-page__detail-copy">当前页 {restorePageWindow}</p> : null}
          </div>

          {restorePointsLoading ? <div className="security-page__detail-note">正在同步恢复点列表…</div> : null}
          {restorePointsError ? <div className="security-page__detail-callout">恢复点列表同步失败：{restorePointsError}</div> : null}
          {!restorePointsLoading && !restorePointsError && restorePoints.length === 0 ? (
            <p className="security-page__empty-state">
              {restoreScope === "focused_task" && focusedTaskId
                ? `当前 task ${focusedTaskId} 还没有可展示的恢复点。`
                : "当前还没有可展示的恢复点。"}
            </p>
          ) : null}
        </div>

        {selectedRestorePoint ? (
          <>
            <div className="security-page__detail-grid">
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">恢复点 ID</p>
                <p className="security-page__detail-value security-page__detail-value--mono">{selectedRestorePoint.recovery_point_id}</p>
                <p className="security-page__detail-copy">关联 task：{selectedRestorePoint.task_id}</p>
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
                  当前页已加载 {restorePoints.length} 条
                  {restorePointsData ? ` · total ${restorePointsData.page.total}` : ""}
                </p>
              </article>
            </div>

            <div className="security-page__detail-list">
              <article className="security-page__detail-list-item">
                <p className="security-page__detail-label">影响对象</p>
                {renderDetailEntryList(selectedRestorePoint.objects, "该恢复点当前没有对象清单。", "restore-objects")}
              </article>
            </div>

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

        {lastRestoreApply ? (
          <div className="security-page__detail-callout">
            最近一次恢复申请：task {lastRestoreApply.response.task.task_id} / {lastRestoreApply.response.task.status} · restore{" "}
            {lastRestoreApply.response.recovery_point.recovery_point_id}
          </div>
        ) : null}

        {restorePointsData ? (
          <div className="security-page__detail-pagination">
            <p className="security-page__detail-copy">
              {restoreScope === "focused_task" && focusedTaskId
                ? `当前页仅统计 task ${focusedTaskId} 下的恢复点。`
                : "当前页统计的是全局可见恢复点。"}
            </p>
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

        {restorePoints.length > 1 ? (
          <div className="security-page__detail-section">
            <div className="security-page__detail-toolbar">
              <div>
                <p className="security-page__detail-label">恢复点列表</p>
                <p className="security-page__detail-copy">
                  选择要查看或回滚的恢复点；当前列表
                  {restoreScope === "focused_task" && focusedTaskId ? `按 task ${focusedTaskId} 过滤。` : "按全局范围展示。"}
                </p>
              </div>
            </div>
            <div className="security-page__detail-selection-list">
              {restorePoints.map((item) => (
                <button
                  key={item.recovery_point_id}
                  type="button"
                  className={`security-page__detail-selection-item${item.recovery_point_id === selectedRestorePoint.recovery_point_id ? " is-active" : ""}`}
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
      </div>
    );
  };

  const renderBudgetDetail = () => {
    const tokenCost = moduleData.summary.token_cost_summary;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当前任务</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.current_task_tokens)} tokens</p>
            <p className="security-page__detail-copy">成本 {formatCurrency(tokenCost.current_task_cost)}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当日累计</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.today_tokens)} tokens</p>
            <p className="security-page__detail-copy">成本 {formatCurrency(tokenCost.today_cost)}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">单任务上限</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.single_task_limit)} tokens</p>
            <p className="security-page__detail-copy">超限任务应继续走安全治理判断。</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">当日上限</p>
            <p className="security-page__detail-value">{formatTokenCount(tokenCost.daily_limit)} tokens</p>
            <p className="security-page__detail-copy">自动降级：{tokenCost.budget_auto_downgrade ? "开启" : "关闭"}</p>
          </article>
        </div>
      </div>
    );
  };

  const renderGovernanceDetail = () => {
    const response = activeGovernanceOutcome?.response;
    const authorizationRecord = response?.authorization_record;
    const task = response?.task;
    const bubbleMessage = response?.bubble_message;
    const impactScope = response?.impact_scope as ImpactScopeDetails | undefined;
    const auditGroups = groupAuditRecordsByType(filteredAuditRecords);
    const auditPageWindow = activeAuditRecordsData ? formatPageWindow(activeAuditRecordsData.page, activeAuditRecordsData.items.length) : null;
    const auditPageStep = activeAuditRecordsData?.page.limit ?? SECURITY_DETAIL_PAGE_SIZE;
    const downloadSettings = settingsSnapshot.settings.general.download;
    const dataLogSettings = settingsSnapshot.settings.data_log;
    const settingsWarnings = settingsSnapshot.rpcContext.warnings;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-note">
          这里把当前前端会话内的授权结果、当前 task 的审计记录和恢复入口放在一起浏览；不是后端完整历史归档。
        </div>

        <div className="security-page__detail-note">
          approval.pending 的实时行为没有移除：新授权会先进入画布，再以顺序保护的方式拉取最新 summary 与 pending，避免界面回退到旧状态。
        </div>

        <div className="security-page__detail-note">
          这里只展示已经登记到设置快照里的治理信号，例如工作区下载路径、逐文件确认和预算降级；不会伪造尚未进入真源的命令白名单或工作区外写入开关。
        </div>

        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">工作区下载路径</p>
            <p className="security-page__detail-value security-page__detail-value--mono">{downloadSettings.workspace_path}</p>
            <p className="security-page__detail-copy">当前展示的是 `settings.general.download.workspace_path`，用于说明下载类产物默认落盘到哪里。</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">逐文件保存确认</p>
            <p className="security-page__detail-value">{formatBooleanLabel(downloadSettings.ask_before_save_each_file)}</p>
            <p className="security-page__detail-copy">该开关来自 `settings.general.download.ask_before_save_each_file`，用于说明保存文件前是否逐个确认。</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">数据日志提供商</p>
            <p className="security-page__detail-value">{dataLogSettings.provider}</p>
            <p className="security-page__detail-copy">这里只展示 `settings.data_log.provider`，不额外扩展未登记的提供商策略对象。</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">预算自动降级</p>
            <p className="security-page__detail-value">{formatBooleanLabel(dataLogSettings.budget_auto_downgrade)}</p>
            <p className="security-page__detail-copy">该卡片只说明 `settings.data_log.budget_auto_downgrade` 的当前状态，不伪装成已经完成的后端执行策略。</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">提供商密钥状态</p>
            <p className="security-page__detail-value">{formatBooleanLabel(dataLogSettings.provider_api_key_configured)}</p>
            <p className="security-page__detail-copy">设置真源只回传是否已配置，不会在这里泄露任何真实 secret。</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">设置快照来源</p>
            <p className="security-page__detail-value">{settingsSnapshot.source === "rpc" ? "settings.get" : "local fallback"}</p>
            <p className="security-page__detail-copy">
              {settingsSnapshot.rpcContext.serverTime
                ? `服务端快照时间：${settingsSnapshot.rpcContext.serverTime}`
                : "当前展示的是本地设置回退快照。"}
            </p>
            {settingsWarnings.length > 0 ? (
              <p className="security-page__detail-copy">warnings：{settingsWarnings.join("；")}</p>
            ) : null}
          </article>
        </div>

        <div className="security-page__detail-section">
          <div className="security-page__detail-toolbar">
            <div>
              <p className="security-page__detail-label">当前会话授权结果</p>
              <p className="security-page__detail-copy">
                这里只保留当前前端会话内最近 10 条已提交决策，用于快速回看，不代替正式授权历史接口。
              </p>
            </div>
            {task?.task_id ? (
              <Button variant="soft" color="gray" onClick={() => openTaskDetail(task.task_id)}>
                查看当前决策任务
                <ArrowUpRight className="h-4 w-4" />
              </Button>
            ) : null}
          </div>

          {governanceRecords.length > 0 ? (
            <div className="security-page__detail-selection-list">
              {governanceRecords.map((outcome) => {
                const outcomeRecord = outcome.response.authorization_record;
                const outcomeTask = outcome.response.task;
                const recordId = outcomeRecord.authorization_record_id;

                return (
                  <button
                    key={recordId}
                    type="button"
                    className={`security-page__detail-selection-item${recordId === activeDecisionRecordId ? " is-active" : ""}`}
                    onClick={() => setActiveDecisionRecordId(recordId)}
                  >
                    <span className="security-page__detail-label">{formatDateTime(outcomeRecord.created_at)}</span>
                    <strong className="security-page__detail-selection-title">
                      {outcomeRecord.decision} · {outcomeTask.status}
                    </strong>
                    <span className="security-page__detail-copy">
                      task {outcomeTask.task_id} · {formatImpactScopeSummary(outcome.response.impact_scope)}
                    </span>
                  </button>
                );
              })}
            </div>
          ) : (
            <p className="security-page__empty-state">当前会话还没有已提交的授权决策。</p>
          )}
        </div>

        {response && authorizationRecord && task ? (
          <>
            <div className="security-page__detail-grid">
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">当前授权记录</p>
                <p className="security-page__detail-value">{authorizationRecord.decision}</p>
                <p className="security-page__detail-copy">
                  remember_rule：{formatBooleanLabel(authorizationRecord.remember_rule)} · operator：{authorizationRecord.operator}
                </p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">授权记录标识</p>
                <p className="security-page__detail-value security-page__detail-value--mono">
                  {authorizationRecord.authorization_record_id}
                </p>
                <p className="security-page__detail-copy">
                  approval_id：{authorizationRecord.approval_id} · task_id：{authorizationRecord.task_id} · created_at：
                  {formatDateTime(authorizationRecord.created_at)}
                </p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">最近任务状态</p>
                <p className="security-page__detail-value">{task.status}</p>
                <p className="security-page__detail-copy">
                  {task.title} · current_step：{task.current_step}
                </p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">任务来源与风险</p>
                <p className="security-page__detail-value">{task.source_type}</p>
                <p className="security-page__detail-copy">
                  risk_level：{task.risk_level} · updated_at：{formatDateTime(task.updated_at)}
                </p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">Bubble message</p>
                <p className="security-page__detail-value">{bubbleMessage?.type ?? "未返回"}</p>
                <p className="security-page__detail-copy">
                  pinned：{formatBooleanLabel(bubbleMessage?.pinned ?? false)} · hidden：
                  {formatBooleanLabel(bubbleMessage?.hidden ?? false)}
                </p>
              </article>
              <article className="security-page__detail-card">
                <p className="security-page__detail-label">影响范围</p>
                <p className="security-page__detail-value">{formatImpactScopeSummary(impactScope)}</p>
                <p className="security-page__detail-copy">
                  工作区外：{formatBooleanLabel(impactScope?.out_of_workspace ?? false)} · 覆盖/删除风险：
                  {formatBooleanLabel(impactScope?.overwrite_or_delete_risk ?? false)}
                </p>
              </article>
            </div>

            <div className="security-page__detail-list">
              <article className="security-page__detail-list-item">
                <p className="security-page__detail-label">任务元数据</p>
                {renderDetailEntryList(
                  [
                    `task_id：${task.task_id}`,
                    `started_at：${formatOptionalDateTime(task.started_at)}`,
                    `updated_at：${formatDateTime(task.updated_at)}`,
                    `finished_at：${formatOptionalDateTime(task.finished_at)}`,
                  ],
                  "当前没有任务元数据。",
                  "task-meta",
                )}
              </article>

              <article className="security-page__detail-list-item">
                <p className="security-page__detail-label">Bubble 元数据</p>
                {renderDetailEntryList(
                  bubbleMessage
                    ? [
                        `bubble_id：${bubbleMessage.bubble_id}`,
                        `task_id：${bubbleMessage.task_id}`,
                        `created_at：${formatDateTime(bubbleMessage.created_at)}`,
                        `text：${bubbleMessage.text}`,
                      ]
                    : [],
                  "最近一次响应没有返回 bubble_message。",
                  "bubble-meta",
                )}
              </article>

              <article className="security-page__detail-list-item">
                <p className="security-page__detail-label">影响文件</p>
                {renderDetailEntryList(impactScope?.files ?? [], "当前没有文件影响。", "impact-files")}
              </article>

              <article className="security-page__detail-list-item">
                <p className="security-page__detail-label">影响网页</p>
                {renderDetailEntryList(impactScope?.webpages ?? [], "当前没有网页影响。", "impact-webpages")}
              </article>

              <article className="security-page__detail-list-item">
                <p className="security-page__detail-label">影响应用</p>
                {renderDetailEntryList(impactScope?.apps ?? [], "当前没有应用影响。", "impact-apps")}
              </article>
            </div>

            <div className="security-page__detail-callout">
              <p className="security-page__detail-label">task intent</p>
              <pre className="security-page__detail-code">{formatTaskIntent(task.intent)}</pre>
            </div>
          </>
        ) : null}

        <div className="security-page__detail-section">
          <div className="security-page__detail-toolbar">
            <div>
              <p className="security-page__detail-label">审计记录</p>
              <p className="security-page__detail-copy">
                {focusedTaskId ? `当前聚焦 task ${focusedTaskId}。` : "当前没有绑定到具体 task，审计记录将等待任务上下文。"}
              </p>
            </div>
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

          {auditRecordsLoading ? <div className="security-page__detail-note">正在同步审计记录…</div> : null}
          {auditRecordsError ? <div className="security-page__detail-callout">审计记录同步失败：{auditRecordsError}</div> : null}
          {!auditRecordsLoading && !auditRecordsError && focusedTaskId && (activeAuditRecordsData?.items.length ?? 0) === 0 ? (
            <p className="security-page__empty-state">当前 task 还没有可展示的审计记录。</p>
          ) : null}
          {!auditRecordsLoading &&
          !auditRecordsError &&
          focusedTaskId &&
          (activeAuditRecordsData?.items.length ?? 0) > 0 &&
          filteredAuditRecords.length === 0 ? (
            <p className="security-page__empty-state">当前筛选条件下没有命中的审计记录。</p>
          ) : null}

          {Object.entries(auditGroups).length > 0 ? (
            Object.entries(auditGroups).map(([groupKey, records]) => (
              <div key={groupKey} className="security-page__detail-section">
                <div className="security-page__detail-toolbar">
                  <div>
                    <p className="security-page__detail-label">审计分组</p>
                    <p className="security-page__detail-copy">
                      {groupKey} · {records.length} 条
                    </p>
                  </div>
                </div>
                <div className="security-page__detail-list">
                  {records.map((record) => (
                    <article key={record.audit_id} className="security-page__detail-list-item">
                      <p className="security-page__detail-label">
                        {formatDateTime(record.created_at)} · {record.action}
                      </p>
                      <p className="security-page__detail-value security-page__detail-value--mono">{record.target}</p>
                      <p className="security-page__detail-copy">{record.summary}</p>
                      <p className="security-page__detail-copy">
                        result：{record.result} · audit_id：{record.audit_id}
                      </p>
                    </article>
                  ))}
                </div>
              </div>
            ))
          ) : null}

          {activeAuditRecordsData ? (
            <div className="security-page__detail-pagination">
              <p className="security-page__detail-copy">
                {auditTypeFilter === ALL_AUDIT_TYPES
                  ? `当前页共 ${activeAuditRecordsData.items.length} 条审计记录。`
                  : `筛选后命中 ${filteredAuditRecords.length} / 当前页 ${activeAuditRecordsData.items.length} 条。`}
              </p>
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

          {restorePointsData?.items.length ? (
            <div className="security-page__detail-callout">
              当前任务可见恢复点 {restorePointsData.items.length} 条；最近一条为 {restorePointsData.items[0].recovery_point_id}。
            </div>
          ) : null}
        </div>

        {moduleData.rpcContext.warnings.length ? (
          <div className="security-page__detail-callout">warnings：{moduleData.rpcContext.warnings.join("；")}</div>
        ) : null}

        {feedback ? <div className="security-page__detail-callout">{feedback}</div> : null}

        <Flex align="center" gap="3" wrap="wrap">
          <Button variant="soft" color="gray" onClick={() => navigate(resolveDashboardRoutePath("home"))}>
            返回 Dashboard
            <ArrowUpRight className="h-4 w-4" />
          </Button>
        </Flex>
      </div>
    );
  };
  
  const renderApprovalDetail = (approval: ApprovalRequest | undefined, snapshotOnly = false) => {
    if (!approval) {
      return <p className="security-page__empty-state">该待确认授权已经从当前列表中移除。</p>;
    }

    const rememberRule = rememberRuleByApprovalId[approval.approval_id] ?? false;

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">操作名称</p>
            <p className="security-page__detail-value">{approval.operation_name}</p>
            <p className="security-page__detail-copy">风险等级：{approval.risk_level} · 状态：{approval.status}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">创建时间</p>
            <p className="security-page__detail-value">{formatDateTime(approval.created_at)}</p>
            <p className="security-page__detail-copy">task_id：{approval.task_id}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">审批标识</p>
            <p className="security-page__detail-value security-page__detail-value--mono">{approval.approval_id}</p>
            <p className="security-page__detail-copy">approval.pending 顶层 task_id：{approval.task_id}</p>
          </article>
        </div>

        <article className="security-page__detail-list-item">
          <p className="security-page__detail-label">目标对象</p>
          <p className="security-page__detail-copy">{approval.target_object}</p>
        </article>

        <article className="security-page__detail-list-item">
          <p className="security-page__detail-label">原因说明</p>
          <p className="security-page__detail-copy">{approval.reason}</p>
        </article>

        <div className="security-page__detail-callout">
          {snapshotOnly ? (
            <div className="security-page__detail-copy">该审批已不在当前实时待处理列表中，详情仅保留快照展示，不能继续提交授权决策。</div>
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
              <span className="security-page__detail-copy">记住这次授权规则；提交到 `agent.security.respond` 时将发送 {String(rememberRule)}。</span>
            </span>
          </label>
        </div>

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
      resolvedSourceCopy,
      feedback,
      lastResolvedApproval,
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
                  <Badge color={preview.badgeColor} variant="soft" highContrast>
                    {preview.badgeLabel}
                  </Badge>
                  <Badge color={moduleData.source === "rpc" ? "green" : "amber"} variant="soft" highContrast>
                    {resolvedSourceCopy.badge}
                  </Badge>
                </Flex>
                <button type="button" className="security-page__close-button" onClick={closeDetail} aria-label="关闭详情视图">
                  <X className="security-page__close-icon" />
                </button>
              </div>
            </div>

            <div className="security-page__detail-body">{renderDetailBody(activeDetailKey, resolvedDetail.approval, resolvedDetail.restorePoint, snapshotOnlyApproval)}</div>
          </section>
        </div>
      </div>
    );
  };

  const renderDraggableCard = (key: SecurityCardKey, index: number) => {
    const preview = getCardPreview(key, moduleData, approvalLookup, resolvedSourceCopy, feedback, lastResolvedApproval);
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

            <Badge color={preview.badgeColor} variant="soft" highContrast>
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
        <Box className="security-page__hero">
          <Text as="p" size="1" className="security-page__eyebrow">
            security / governance
          </Text>
          <button
            type="button"
            className="security-page__title-button"
            onClick={handleTitleClick}
            aria-label="播放安全卫士标题动效"
          >
            <Heading size="9" className="security-page__title">
              <span
                key={titleMotionTick}
                className={`security-page__title-text${titleMotionTick > 0 ? " security-page__title-text--animate" : ""}`}
              >
                安全卫士
              </span>
            </Heading>
          </button>
        </Box>

        <aside className={`security-page__source-status ${resolvedSourceCopy.className}`} aria-label="Security 数据来源状态">
          <Badge color={moduleData.source === "rpc" ? "green" : "amber"} variant="soft" highContrast>
            {resolvedSourceCopy.badge}
          </Badge>
          <div className="security-page__source-copy">
            <p className="security-page__source-title">{resolvedSourceCopy.title}</p>
            <p className="security-page__source-description">{loadError && dataMode === "rpc" ? `${resolvedSourceCopy.description} · error：${loadError}` : resolvedSourceCopy.description}</p>
          </div>
        </aside>

        {feedback ? (
          <div className="security-page__feedback" aria-live="polite">
            <p className="security-page__feedback-label">latest update</p>
            <p className="security-page__feedback-copy">{feedback}</p>
          </div>
        ) : null}

        {cardStack.map(renderDraggableCard)}
        {renderDetailOverlay()}
      </div>
      <DashboardMockToggle enabled={dataMode === "mock"} onToggle={() => setDataMode((current) => (current === "rpc" ? "mock" : "rpc"))} />
    </main>
  );
}
