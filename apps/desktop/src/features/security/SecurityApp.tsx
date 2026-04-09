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
  ApprovalDecision,
  ApprovalPendingNotification,
  ApprovalRequest,
  RiskLevel,
  SecurityStatus,
} from "@cialloclaw/protocol";
import { subscribeApprovalPending } from "@/rpc/subscriptions";
import {
  getInitialSecurityModuleData,
  loadSecurityModuleData,
  loadSecurityModuleRpcData,
  respondToApproval,
  type SecurityModuleData,
  type SecurityModuleSource,
} from "@/services/securityService";
import { openWindow } from "@/platform/windowController";

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

const STATIC_CARD_KEYS: SecurityCardKey[] = ["status", "restore", "budget", "governance"];
const DRAG_THRESHOLD = 8;
const CARD_CLEARANCE = 14;
const CARD_STEP = 18;
const BOARD_INSET_X = 22;
const BOARD_INSET_TOP = 140;
const BOARD_INSET_BOTTOM = 24;
const DEFAULT_CARD_SIZE: CardSize = { width: 248, height: 176 };
const FALLBACK_POSITION: CardPosition = { x: BOARD_INSET_X, y: BOARD_INSET_TOP };

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

function getSourceCopy(source: SecurityModuleSource) {
  if (source === "rpc") {
    return {
      badge: "LIVE",
      title: "当前显示的是 JSON-RPC 实时数据",
      description: "来自后端返回，不是本地 mock。",
      className: "security-page__source-status--rpc",
    };
  }

  return {
    badge: "MOCK",
    title: "当前显示的是本地 mock 数据",
    description: "仅用于前端联调，不是真实后端返回。",
    className: "security-page__source-status--mock",
  };
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

function mergePendingApproval(current: SecurityModuleData, payload: ApprovalPendingNotification): SecurityModuleData {
  const exists = current.pending.some((item) => item.approval_id === payload.approval_request.approval_id);
  const pending = exists ? current.pending : [payload.approval_request, ...current.pending];

  return {
    ...current,
    summary: {
      ...current.summary,
      security_status: "pending_confirmation",
      pending_authorizations: exists ? current.summary.pending_authorizations : current.summary.pending_authorizations + 1,
    },
    pending,
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

function getCardPreview(
  key: SecurityCardKey,
  moduleData: SecurityModuleData,
  approvalLookup: Map<string, ApprovalRequest>,
  sourceCopy: ReturnType<typeof getSourceCopy>,
  feedback: string | null,
): SecurityCardPreview {
  if (key === "status") {
    return {
      eyebrow: "security status",
      title: "安全态势",
      badgeLabel: moduleData.summary.security_status,
      badgeColor: getStatusColor(moduleData.summary.security_status),
      headline: `${moduleData.summary.pending_authorizations} 条待确认`,
      supporting: "approval.pending 的实时推送和序列保护仍保持启用。",
      meta: moduleData.source === "rpc" ? "LIVE refresh queue" : "等待 RPC 首次刷新",
      emphasis: "number",
      icon: ShieldCheck,
    };
  }

  if (key === "restore") {
    const restorePoint = moduleData.summary.latest_restore_point;

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
      supporting: "工作区边界、恢复点和预算治理说明仍聚合在本模块。",
      meta: feedback ?? `${moduleData.pending.length} 条 pending approvals`,
      icon: Siren,
    };
  }

  const approval = approvalLookup.get(key);

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
  const [moduleData, setModuleData] = useState(() => getInitialSecurityModuleData());
  const [activeApprovalId, setActiveApprovalId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [titleMotionTick, setTitleMotionTick] = useState(0);
  const [cardPositions, setCardPositions] = useState<Record<string, CardPosition>>({});
  const [cardStack, setCardStack] = useState<SecurityCardKey[]>([]);
  const [cardSize, setCardSize] = useState<CardSize>(DEFAULT_CARD_SIZE);
  const [draggingKey, setDraggingKey] = useState<SecurityCardKey | null>(null);
  const [activeDetailKey, setActiveDetailKey] = useState<SecurityCardKey | null>(null);
  const [boardReady, setBoardReady] = useState(false);
  const sourceCopy = useMemo(() => getSourceCopy(moduleData.source), [moduleData.source]);
  const approvalLookup = useMemo(
    () => new Map(moduleData.pending.map((approval) => [`approval:${approval.approval_id}`, approval] as const)),
    [moduleData.pending],
  );
  const pendingCardKeys = useMemo(
    () => moduleData.pending.map((approval) => `approval:${approval.approval_id}` as SecurityCardKey),
    [moduleData.pending],
  );
  const cardKeys = useMemo(() => [...STATIC_CARD_KEYS, ...pendingCardKeys], [pendingCardKeys]);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const refreshSequenceRef = useRef(0);

  const queueRpcRefresh = () => {
    const nextSequence = ++refreshSequenceRef.current;

    void loadSecurityModuleRpcData()
      .then((nextData) => {
        if (refreshSequenceRef.current === nextSequence) {
          setModuleData(nextData);
        }
      })
      .catch(() => undefined);
  };

  useEffect(() => {
    const nextSequence = ++refreshSequenceRef.current;
    void loadSecurityModuleData().then((nextData) => {
      if (refreshSequenceRef.current === nextSequence) {
        setModuleData(nextData);
      }
    });

    const unsubscribe = subscribeApprovalPending((payload) => {
      setModuleData((current) => ({
        ...mergePendingApproval(current, payload),
        source: "rpc",
      }));
      setFeedback(`收到新的待确认授权：${payload.approval_request.operation_name}`);
      queueRpcRefresh();
    });

    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    setCardStack((currentStack) => {
      const preserved = currentStack.filter((key) => cardKeys.includes(key));
      const additions = cardKeys.filter((key) => !preserved.includes(key));
      return [...preserved, ...additions];
    });
  }, [cardKeys]);

  useEffect(() => {
    if (activeDetailKey && !cardKeys.includes(activeDetailKey)) {
      setActiveDetailKey(null);
    }
  }, [activeDetailKey, cardKeys]);

  const bringCardToFront = useCallback((key: SecurityCardKey) => {
    setCardStack((currentStack) => [...currentStack.filter((item) => item !== key), key]);
  }, []);

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

  const handleRespond = async (approval: ApprovalRequest, decision: ApprovalDecision) => {
    setActiveApprovalId(approval.approval_id);

    try {
      const result = await respondToApproval(approval, decision, moduleData.source);
      setModuleData((current) => ({
        ...current,
        summary: {
          ...current.summary,
          pending_authorizations: Math.max(0, current.summary.pending_authorizations - 1),
        },
        pending: current.pending.filter((item) => item.approval_id !== approval.approval_id),
      }));
      setFeedback(result.bubble_message?.text ?? "已更新安全审批状态。");
    } catch (error) {
      setFeedback(error instanceof Error ? error.message : "安全审批提交失败");
    } finally {
      setActiveApprovalId(null);
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
          <p className="security-page__detail-value">{sourceCopy.badge}</p>
          <p className="security-page__detail-copy">{sourceCopy.title}</p>
        </article>
      </div>

      <div className="security-page__detail-note">
        approval.pending 推送仍先合并到前端状态，再走顺序保护的 RPC 刷新，避免旧响应覆盖较新的安全视图。
      </div>
    </div>
  );

  const renderRestoreDetail = () => {
    const restorePoint = moduleData.summary.latest_restore_point;

    if (!restorePoint) {
      return <p className="security-page__empty-state">当前没有可展示的恢复点。</p>;
    }

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">恢复点 ID</p>
            <p className="security-page__detail-value security-page__detail-value--mono">{restorePoint.recovery_point_id}</p>
            <p className="security-page__detail-copy">关联 task：{restorePoint.task_id}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">创建时间</p>
            <p className="security-page__detail-value">{formatDateTime(restorePoint.created_at)}</p>
            <p className="security-page__detail-copy">{restorePoint.summary}</p>
          </article>
        </div>

        <div className="security-page__detail-list">
          {restorePoint.objects.map((item) => (
            <article key={item} className="security-page__detail-list-item">
              <p className="security-page__detail-label">影响对象</p>
              <p className="security-page__detail-copy">{item}</p>
            </article>
          ))}
        </div>
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

  const renderGovernanceDetail = () => (
    <div className="security-page__detail-stack">
      <div className="security-page__detail-note">
        工作区边界、影响范围展示、恢复点与预算治理会继续统一承接在这个模块里，但前端当前只稳定使用 summary / pending / respond 三条 RPC 通道。
      </div>

      <div className="security-page__detail-note">
        approval.pending 的实时行为没有移除：新授权会先进入画布，再以顺序保护的方式拉取最新 summary 与 pending，避免界面回退到旧状态。
      </div>

      {feedback ? <div className="security-page__detail-callout">{feedback}</div> : null}

      <Flex align="center" gap="3" wrap="wrap">
        <Button variant="soft" color="gray" onClick={() => void openWindow("dashboard")}>
          返回 Dashboard
          <ArrowUpRight className="h-4 w-4" />
        </Button>
      </Flex>
    </div>
  );

  const renderApprovalDetail = (approval: ApprovalRequest | undefined) => {
    if (!approval) {
      return <p className="security-page__empty-state">该待确认授权已经从当前列表中移除。</p>;
    }

    return (
      <div className="security-page__detail-stack">
        <div className="security-page__detail-grid">
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">操作名称</p>
            <p className="security-page__detail-value">{approval.operation_name}</p>
            <p className="security-page__detail-copy">风险等级：{approval.risk_level}</p>
          </article>
          <article className="security-page__detail-card">
            <p className="security-page__detail-label">创建时间</p>
            <p className="security-page__detail-value">{formatDateTime(approval.created_at)}</p>
            <p className="security-page__detail-copy">task_id：{approval.task_id}</p>
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

        <div className="security-page__approval-actions">
          <Button
            color="gray"
            variant="soft"
            disabled={activeApprovalId === approval.approval_id}
            onClick={() => void handleRespond(approval, "deny_once")}
          >
            拒绝
          </Button>
          <Button
            color="amber"
            variant="solid"
            disabled={activeApprovalId === approval.approval_id}
            onClick={() => void handleRespond(approval, "allow_once")}
          >
            允许一次
          </Button>
        </div>
      </div>
    );
  };

  const renderDetailBody = (key: SecurityCardKey) => {
    if (key === "status") {
      return renderStatusDetail();
    }

    if (key === "restore") {
      return renderRestoreDetail();
    }

    if (key === "budget") {
      return renderBudgetDetail();
    }

    if (key === "governance") {
      return renderGovernanceDetail();
    }

    return renderApprovalDetail(approvalLookup.get(key));
  };

  const renderDetailOverlay = () => {
    if (!activeDetailKey) {
      return null;
    }

    const preview = getCardPreview(activeDetailKey, moduleData, approvalLookup, sourceCopy, feedback);

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
                    {sourceCopy.badge}
                  </Badge>
                </Flex>
                <button type="button" className="security-page__close-button" onClick={closeDetail} aria-label="关闭详情视图">
                  <X className="security-page__close-icon" />
                </button>
              </div>
            </div>

            <div className="security-page__detail-body">{renderDetailBody(activeDetailKey)}</div>
          </section>
        </div>
      </div>
    );
  };

  const renderDraggableCard = (key: SecurityCardKey, index: number) => {
    const preview = getCardPreview(key, moduleData, approvalLookup, sourceCopy, feedback);
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

        <aside className={`security-page__source-status ${sourceCopy.className}`} aria-label="Security 数据来源状态">
          <Badge color={moduleData.source === "rpc" ? "green" : "amber"} variant="soft" highContrast>
            {sourceCopy.badge}
          </Badge>
          <div className="security-page__source-copy">
            <p className="security-page__source-title">{sourceCopy.title}</p>
            <p className="security-page__source-description">{sourceCopy.description}</p>
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
    </main>
  );
}
