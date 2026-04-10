import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { BookMarked, BrainCircuit, CalendarDays, Sparkles, X } from "lucide-react";
import { PanelSurface, StatusBadge } from "@cialloclaw/ui";
import { subscribeMirrorOverviewUpdated } from "@/rpc/subscriptions";
import {
  loadMirrorOverviewData,
  type MirrorOverviewData,
  type MirrorOverviewSource,
} from "./mirrorService";

const MODULE_KEYS = [
  "history",
  "profile",
  "activeHours",
  "preferredOutput",
  "daily",
  "completedTasks",
  "generatedOutputs",
  "memory",
] as const;

type DraggableModuleKey = (typeof MODULE_KEYS)[number];
type ModulePosition = { x: number; y: number };
type ModulePositions = Record<DraggableModuleKey, ModulePosition>;
type ModuleSize = { width: number; height: number };
type BoardBounds = { minX: number; minY: number; maxX: number; maxY: number };
type BoardGrid = { columns: number; rows: number };
type BoardLayout = { bounds: BoardBounds; size: ModuleSize; grid: BoardGrid; candidates: ModulePosition[] };
type CardSummary = {
  badge: string;
  tone: string;
  mainLine: string;
  emphasis?: "default" | "number" | "memory";
};
type DragState = {
  key: DraggableModuleKey;
  pointerId: number;
  startX: number;
  startY: number;
  originX: number;
  originY: number;
  moved: boolean;
};

const INITIAL_MODULE_STACK: DraggableModuleKey[] = [...MODULE_KEYS];
const DRAG_THRESHOLD = 8;
const BOARD_PADDING = 12;
const CARD_CLEARANCE = 10;
const CARD_STEP = 16;
const DEFAULT_CARD_SIZE: ModuleSize = { width: 260, height: 168 };
const DEFAULT_MODULE_POSITIONS: ModulePositions = {
  history: { x: 0, y: 0 },
  profile: { x: 0, y: 0 },
  activeHours: { x: 0, y: 0 },
  preferredOutput: { x: 0, y: 0 },
  daily: { x: 0, y: 0 },
  completedTasks: { x: 0, y: 0 },
  generatedOutputs: { x: 0, y: 0 },
  memory: { x: 0, y: 0 },
};

function formatMirrorDate(value: string) {
  return new Date(value).toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

function formatShortMirrorDate(value: string) {
  return new Date(value).toLocaleDateString("zh-CN", {
    month: "short",
    day: "numeric",
  });
}

function formatInsightBadgeLabel(value: string) {
  if (value === "mirror ready") {
    return "镜像已生成";
  }

  if (value === "mirror empty") {
    return "暂无镜像";
  }

  return value;
}

function formatMirrorSourceLabel(source: MirrorOverviewSource) {
  return source === "rpc" ? "JSON-RPC" : "前端 mock";
}

function getMirrorSourceStatus(source: MirrorOverviewSource) {
  if (source === "rpc") {
    return {
      badge: "LIVE",
      title: "当前显示的是 JSON-RPC 实时数据",
      description: "来自后端返回，不是本地 mock。",
      className: "mirror-page__source-status--rpc",
    };
  }

  return {
    badge: "MOCK",
    title: "当前显示的是本地 mock 数据",
    description: "仅用于前端联调，不是真实后端返回。",
    className: "mirror-page__source-status--mock",
  };
}

function getMirrorLoadingStatus() {
  return {
    badge: "LOADING",
    title: "正在连接 JSON-RPC 数据源",
    description: "尚未确认是否为真实后端返回，正在等待首个 overview 响应。",
    className: "mirror-page__source-status--loading",
  };
}

function clampValue(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function clampPosition(value: ModulePosition, bounds: BoardBounds) {
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

function getBoardGrid(canvasWidth: number, canvasHeight: number): BoardGrid {
  let bestGrid: BoardGrid = { columns: MODULE_KEYS.length, rows: 1 };
  let bestScore = Number.NEGATIVE_INFINITY;

  for (let columns = 1; columns <= MODULE_KEYS.length; columns += 1) {
    const rows = Math.ceil(MODULE_KEYS.length / columns);
    const width = (canvasWidth - BOARD_PADDING * 2 - CARD_CLEARANCE * (columns - 1)) / columns;
    const height = (canvasHeight - BOARD_PADDING * 2 - CARD_CLEARANCE * (rows - 1)) / rows;
    const score = Math.min(width, height);

    if (score > bestScore) {
      bestGrid = { columns, rows };
      bestScore = score;
    }
  }

  return bestGrid;
}

function getBoardCardSize(canvasWidth: number, canvasHeight: number, grid: BoardGrid) {
  const width = Math.floor((canvasWidth - BOARD_PADDING * 2 - CARD_CLEARANCE * (grid.columns - 1)) / grid.columns);
  const height = Math.floor((canvasHeight - BOARD_PADDING * 2 - CARD_CLEARANCE * (grid.rows - 1)) / grid.rows);

  return {
    width: clampValue(width, 1, 264),
    height: clampValue(height, 1, 172),
  } satisfies ModuleSize;
}

function getBoardBounds(canvasWidth: number, canvasHeight: number, size: ModuleSize) {
  const horizontalInset = Math.min(BOARD_PADDING, Math.max(0, (canvasWidth - size.width) * 0.5));
  const verticalInset = Math.min(BOARD_PADDING, Math.max(0, (canvasHeight - size.height) * 0.5));

  return {
    minX: horizontalInset,
    minY: verticalInset,
    maxX: Math.max(horizontalInset, canvasWidth - size.width - horizontalInset),
    maxY: Math.max(verticalInset, canvasHeight - size.height - verticalInset),
  } satisfies BoardBounds;
}

function buildBoardCandidates(bounds: BoardBounds) {
  const positions: ModulePosition[] = [];
  const xs = buildAxisPositions(bounds.minX, bounds.maxX);
  const ys = buildAxisPositions(bounds.minY, bounds.maxY);

  for (const y of ys) {
    for (const x of xs) {
      positions.push({ x, y });
    }
  }

  return positions;
}

function overlapsOccupied(position: ModulePosition, occupied: ModulePosition[], size: ModuleSize) {
  return occupied.some((item) => {
    const separatedHorizontally = position.x + size.width + CARD_CLEARANCE <= item.x || item.x + size.width + CARD_CLEARANCE <= position.x;
    const separatedVertically = position.y + size.height + CARD_CLEARANCE <= item.y || item.y + size.height + CARD_CLEARANCE <= position.y;

    return !(separatedHorizontally || separatedVertically);
  });
}

function resolveSettledPosition(target: ModulePosition, occupied: ModulePosition[], layout: BoardLayout) {
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

function getDefaultModuleTargets(bounds: BoardBounds, grid: BoardGrid, size: ModuleSize): ModulePositions {
  const availableWidth = Math.max(size.width, bounds.maxX - bounds.minX + size.width);
  const availableHeight = Math.max(size.height, bounds.maxY - bounds.minY + size.height);
  const gridHeight = grid.rows * size.height + Math.max(0, grid.rows - 1) * CARD_CLEARANCE;
  const gridStartY = bounds.minY + Math.max(0, (availableHeight - gridHeight) / 2);
  const positions = { ...DEFAULT_MODULE_POSITIONS };

  MODULE_KEYS.forEach((key, index) => {
    const row = Math.floor(index / grid.columns);
    const indexInRow = index % grid.columns;
    const remainingCards = MODULE_KEYS.length - row * grid.columns;
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

function normalizeModulePositions(targets: ModulePositions, layout: BoardLayout) {
  const nextPositions = { ...DEFAULT_MODULE_POSITIONS };
  const occupied: ModulePosition[] = [];

  for (const key of MODULE_KEYS) {
    const settledPosition = resolveSettledPosition(targets[key], occupied, layout);

    if (!settledPosition) {
      throw new Error("Mirror board could not find a non-overlapping position for every card.");
    }

    nextPositions[key] = settledPosition;
    occupied.push(settledPosition);
  }

  return nextPositions;
}

function getModuleTitle(key: DraggableModuleKey) {
  if (key === "history") {
    return "历史概要";
  }

  if (key === "profile") {
    return "用户画像";
  }

  if (key === "activeHours") {
    return "活跃时段";
  }

  if (key === "preferredOutput") {
    return "偏好交付";
  }

  if (key === "daily") {
    return "日报";
  }

  if (key === "completedTasks") {
    return "今日完成";
  }

  if (key === "generatedOutputs") {
    return "输出数量";
  }

  return "近期被调用记忆";
}

function getModuleEyebrow(key: DraggableModuleKey) {
  if (key === "history") {
    return "历史概要";
  }

  if (key === "profile") {
    return "用户画像";
  }

  if (key === "activeHours") {
    return "用户画像";
  }

  if (key === "preferredOutput") {
    return "用户画像";
  }

  if (key === "daily") {
    return "日报";
  }

  if (key === "completedTasks") {
    return "日报";
  }

  if (key === "generatedOutputs") {
    return "日报";
  }

  return "记忆引用";
}

function getDetailKey(key: DraggableModuleKey) {
  if (key === "history") {
    return "history";
  }

  if (key === "profile" || key === "activeHours" || key === "preferredOutput") {
    return "profile";
  }

  if (key === "daily" || key === "completedTasks" || key === "generatedOutputs") {
    return "daily";
  }

  return "memory";
}

export function MirrorApp() {
  const [mirrorData, setMirrorData] = useState<MirrorOverviewData | null>(null);
  const [modulePositions, setModulePositions] = useState<ModulePositions>(DEFAULT_MODULE_POSITIONS);
  const [moduleStack, setModuleStack] = useState<DraggableModuleKey[]>(INITIAL_MODULE_STACK);
  const [cardSize, setCardSize] = useState<ModuleSize>(DEFAULT_CARD_SIZE);
  const [draggingKey, setDraggingKey] = useState<DraggableModuleKey | null>(null);
  const [activeDetailKey, setActiveDetailKey] = useState<DraggableModuleKey | null>(null);
  const [boardReady, setBoardReady] = useState(false);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const hasPlacedModulesRef = useRef(false);
  const isMountedRef = useRef(true);
  const fetchInFlightRef = useRef(false);
  const pendingRefreshRef = useRef(false);

  const refreshMirrorData = useCallback(() => {
    if (fetchInFlightRef.current) {
      pendingRefreshRef.current = true;
      return;
    }

    fetchInFlightRef.current = true;

    void (async () => {
      try {
        do {
          pendingRefreshRef.current = false;
          const nextData = await loadMirrorOverviewData();

          if (!isMountedRef.current) {
            return;
          }

          setMirrorData(nextData);
        } while (pendingRefreshRef.current);
      } finally {
        fetchInFlightRef.current = false;

        if (pendingRefreshRef.current && isMountedRef.current) {
          refreshMirrorData();
        }
      }
    })();
  }, []);

  useEffect(() => {
    isMountedRef.current = true;
    const unsubscribe = subscribeMirrorOverviewUpdated(() => {
      refreshMirrorData();
    });

    refreshMirrorData();

    return () => {
      isMountedRef.current = false;
      unsubscribe();
    };
  }, [refreshMirrorData]);

  const bringModuleToFront = useCallback((key: DraggableModuleKey) => {
    setModuleStack((currentStack) => [...currentStack.filter((item) => item !== key), key]);
  }, []);

  const getBoardLayout = useCallback(() => {
    const canvas = canvasRef.current;

    if (!canvas) {
      return null;
    }

    const grid = getBoardGrid(canvas.clientWidth, canvas.clientHeight);
    const nextSize = getBoardCardSize(canvas.clientWidth, canvas.clientHeight, grid);
    const bounds = getBoardBounds(canvas.clientWidth, canvas.clientHeight, nextSize);

    return {
      bounds,
      size: nextSize,
      grid,
      candidates: buildBoardCandidates(bounds),
    } satisfies BoardLayout;
  }, []);

  const getSettledModulePosition = useCallback(
    (key: DraggableModuleKey, target: ModulePosition, positions: ModulePositions) => {
      const layout = getBoardLayout();

      if (!layout) {
        return target;
      }

      const occupied = MODULE_KEYS.filter((item) => item !== key).map((item) => positions[item]);
      return resolveSettledPosition(target, occupied, layout) ?? positions[key];
    },
    [getBoardLayout],
  );

  useLayoutEffect(() => {
    if (!mirrorData) {
      return;
    }

    const syncBoardLayout = () => {
      const layout = getBoardLayout();

      if (!layout) {
        return;
      }

      setCardSize(layout.size);
      setModulePositions((currentPositions) => {
        const targets = hasPlacedModulesRef.current
          ? currentPositions
          : getDefaultModuleTargets(layout.bounds, layout.grid, layout.size);
        return normalizeModulePositions(targets, layout);
      });
      hasPlacedModulesRef.current = true;
      setBoardReady(true);
    };

    syncBoardLayout();
    window.addEventListener("resize", syncBoardLayout);

    return () => {
      window.removeEventListener("resize", syncBoardLayout);
    };
  }, [getBoardLayout, mirrorData]);

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

  if (!mirrorData) {
    const loadingStatus = getMirrorLoadingStatus();

    return (
      <main className="app-shell mirror-page">
        <div className="mirror-page__canvas mirror-page__canvas--full mirror-page__canvas--loading">
          <aside className={`mirror-page__source-status ${loadingStatus.className}`} aria-label="Mirror 数据来源状态">
            <StatusBadge tone="processing">{loadingStatus.badge}</StatusBadge>
            <div className="mirror-page__source-copy">
              <p className="mirror-page__source-title">{loadingStatus.title}</p>
              <p className="mirror-page__source-description">{loadingStatus.description}</p>
            </div>
          </aside>
          <p className="mirror-page__loading-copy">正在加载镜子卡片…</p>
        </div>
      </main>
    );
  }

  const { overview, insight, source } = mirrorData;
  const sourceStatus = getMirrorSourceStatus(source);
  const dailySummary = overview.daily_summary;
  const profile = overview.profile;
  const latestMemoryReference = overview.memory_references[0] ?? null;

  const closeDetail = () => {
    setActiveDetailKey(null);
  };

  const releaseDrag = () => {
    dragStateRef.current = null;
    setDraggingKey(null);
  };

  const handleModulePointerDown = (key: DraggableModuleKey) => (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return;
    }

    bringModuleToFront(key);
    setDraggingKey(key);
    dragStateRef.current = {
      key,
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      originX: modulePositions[key].x,
      originY: modulePositions[key].y,
      moved: false,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handleModulePointerMove = (key: DraggableModuleKey) => (event: PointerEvent<HTMLDivElement>) => {
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

    setModulePositions((currentPositions) => ({
      ...currentPositions,
      [key]: getSettledModulePosition(
        key,
        {
          x: dragState.originX + deltaX,
          y: dragState.originY + deltaY,
        },
        currentPositions,
      ),
    }));
  };

  const handleModulePointerUp = (key: DraggableModuleKey) => (event: PointerEvent<HTMLDivElement>) => {
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

  const handleModulePointerCancel = (_key: DraggableModuleKey) => (event: PointerEvent<HTMLDivElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    releaseDrag();
  };

  const handleModuleKeyDown = (key: DraggableModuleKey) => (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      bringModuleToFront(key);
      setActiveDetailKey(key);
    }
  };

  const renderHistoryDetail = () => {
    if (!overview.history_summary.length) {
      return <p className="mirror-page__empty-state">暂无历史概要。</p>;
    }

    return (
      <div className="mirror-page__history-list">
        {overview.history_summary.map((item, index) => (
          <article key={`${item}-${index}`} className="mirror-page__history-item">
            <div className="mirror-page__history-index">0{index + 1}</div>
            <div className="mirror-page__history-copy">
              <p className="mirror-page__history-label">历史片段 {index + 1}</p>
              <p className="mirror-page__history-text">{item}</p>
            </div>
          </article>
        ))}
      </div>
    );
  };

  const renderProfileDetail = () => {
    if (!profile) {
      return <p className="mirror-page__empty-state">暂无用户画像。</p>;
    }

    return (
      <div className="mirror-page__profile-list mirror-page__profile-list--expanded">
        <article className="mirror-page__profile-card">
          <div className="mirror-page__profile-heading">
            <BrainCircuit className="mirror-page__profile-icon" />
            <span>工作风格</span>
          </div>
          <p className="mirror-page__profile-copy">{profile.work_style}</p>
        </article>

        <article className="mirror-page__profile-card mirror-page__profile-card--warm">
          <div className="mirror-page__profile-heading">
            <Sparkles className="mirror-page__profile-icon mirror-page__profile-icon--warm" />
            <span>偏好交付</span>
          </div>
          <p className="mirror-page__profile-copy">{profile.preferred_output}</p>
        </article>

        <article className="mirror-page__profile-card mirror-page__profile-card--hours">
          <StatusBadge tone="green">活跃时段</StatusBadge>
          <p className="mirror-page__hours-value">{profile.active_hours}</p>
        </article>

        <div className="mirror-page__detail-note-shell">
          <p className="mirror-page__micro-label">镜像摘要</p>
          <p className="mirror-page__note">{insight.description}</p>
        </div>
      </div>
    );
  };

  const renderDailyDetail = () => {
    if (!dailySummary) {
      return <p className="mirror-page__empty-state">暂无每日摘要。</p>;
    }

    const perTaskOutput = dailySummary.completed_tasks
      ? (dailySummary.generated_outputs / dailySummary.completed_tasks).toFixed(1)
      : "0.0";

    return (
      <div className="mirror-page__daily-stack mirror-page__daily-stack--expanded">
        <div className="mirror-page__date-card">
          <CalendarDays className="mirror-page__date-icon" />
          <div>
            <p className="mirror-page__micro-label">记录日期</p>
            <p className="mirror-page__date-value">{formatMirrorDate(dailySummary.date)}</p>
          </div>
        </div>

        <div className="mirror-page__summary-grid">
          <article className="mirror-page__summary-card">
            <p className="mirror-page__micro-label">完成任务</p>
            <p className="mirror-page__summary-value">{dailySummary.completed_tasks}</p>
            <p className="mirror-page__summary-copy">已完成任务</p>
          </article>
          <article className="mirror-page__summary-card mirror-page__summary-card--accent">
            <p className="mirror-page__micro-label">输出数量</p>
            <p className="mirror-page__summary-value">{dailySummary.generated_outputs}</p>
            <p className="mirror-page__summary-copy">已记录输出</p>
          </article>
        </div>

        <div className="mirror-page__note">
          平均每个已完成任务沉淀 {perTaskOutput} 份输出线索，可继续作为后续镜像整理和回看依据。
        </div>
      </div>
    );
  };

  const renderMemoryDetail = () => {
    if (!overview.memory_references.length) {
      return <p className="mirror-page__empty-state">暂无近期记忆引用。</p>;
    }

    return (
      <div className="mirror-page__memory-list mirror-page__memory-list--expanded">
        {overview.memory_references.map((reference, index) => (
          <article key={reference.memory_id} className="mirror-page__memory-card">
            <div className="mirror-page__memory-header">
              <div className="mirror-page__memory-meta">
                <p className="mirror-page__memory-index">记录 {index + 1}</p>
                <div className="mirror-page__memory-title-row">
                  <BookMarked className="mirror-page__memory-icon" />
                  <h3 className="mirror-page__memory-title">{reference.memory_id}</h3>
                </div>
              </div>
              <StatusBadge tone="processing">最近调用</StatusBadge>
            </div>

            <p className="mirror-page__memory-reason">{reference.reason}</p>
            <div className="mirror-page__memory-summary">{reference.summary}</div>
          </article>
        ))}
      </div>
    );
  };

  const renderDetailContent = () => {
    if (!activeDetailKey) {
      return null;
    }

    const detailKey = getDetailKey(activeDetailKey);

    if (detailKey === "history") {
      return renderHistoryDetail();
    }

    if (detailKey === "profile") {
      return renderProfileDetail();
    }

    if (detailKey === "daily") {
      return renderDailyDetail();
    }

    return renderMemoryDetail();
  };

  const renderDetailOverlay = () => {
    if (!activeDetailKey) {
      return null;
    }

    return (
      <div className="mirror-page__detail-layer" onClick={closeDetail}>
        <div className="mirror-page__detail-panel" role="dialog" aria-modal="true" aria-label={`${getModuleTitle(activeDetailKey)}详情`} onClick={(event) => event.stopPropagation()}>
          <PanelSurface title={getModuleTitle(activeDetailKey)} eyebrow={getModuleEyebrow(activeDetailKey)}>
            <div className="mirror-page__detail-topbar">
              <div className="mirror-page__detail-meta">
                <StatusBadge tone="processing">{formatInsightBadgeLabel(insight.badge)}</StatusBadge>
                <span className="mirror-page__mono">{sourceStatus.title}</span>
              </div>
              <button type="button" className="mirror-page__close-button" onClick={closeDetail} aria-label="关闭详情视图">
                <X className="mirror-page__close-icon" />
              </button>
            </div>
            <div className="mirror-page__detail-body">{renderDetailContent()}</div>
          </PanelSurface>
        </div>
      </div>
    );
  };

  const getCardSummary = (key: DraggableModuleKey): CardSummary => {
    if (key === "history") {
      return {
        badge: `${overview.history_summary.length} 条片段`,
        tone: "processing",
        mainLine: overview.history_summary[0] ?? "暂无历史概要",
      };
    }

    if (key === "profile") {
      return {
        badge: "用户画像",
        tone: "green",
        mainLine: profile?.work_style ?? "暂无用户画像",
      };
    }

    if (key === "activeHours") {
      return {
        badge: "活跃时段",
        tone: "green",
        mainLine: profile?.active_hours ?? "未记录",
      };
    }

    if (key === "preferredOutput") {
      return {
        badge: "偏好交付",
        tone: "processing",
        mainLine: profile?.preferred_output ?? "未记录",
      };
    }

    if (key === "daily") {
      return {
        badge: dailySummary ? formatShortMirrorDate(dailySummary.date) : "暂无记录",
        tone: "processing",
        mainLine: dailySummary ? formatMirrorDate(dailySummary.date) : "暂无日报",
      };
    }

    if (key === "completedTasks") {
      return {
        badge: "今日完成",
        tone: "processing",
        mainLine: `${dailySummary?.completed_tasks ?? 0} 项任务`,
        emphasis: "number",
      };
    }

    if (key === "generatedOutputs") {
      return {
        badge: "输出数量",
        tone: "processing",
        mainLine: `${dailySummary?.generated_outputs ?? 0} 份输出`,
        emphasis: "number",
      };
    }

    return {
      badge: `${overview.memory_references.length} 条记忆`,
      tone: "processing",
      mainLine: latestMemoryReference?.memory_id ?? "暂无记忆",
      emphasis: "memory",
    };
  };

  const renderDraggableModule = (key: DraggableModuleKey) => {
    const isDragging = draggingKey === key;
    const isExpanded = activeDetailKey === key;
    const summary = getCardSummary(key);
    const summaryClassName = [
      "mirror-page__card-line",
      summary.emphasis ? `mirror-page__card-line--${summary.emphasis}` : null,
    ]
      .filter(Boolean)
      .join(" ");

    return (
      <div
        key={key}
        className={`mirror-page__draggable mirror-page__draggable--${key}${isDragging ? " is-dragging" : ""}${isExpanded ? " is-active" : ""}${boardReady ? " is-ready" : ""}`}
        style={{
          height: `${cardSize.height}px`,
          transform: `translate3d(${modulePositions[key].x}px, ${modulePositions[key].y}px, 0)`,
          width: `${cardSize.width}px`,
        }}
        role="button"
        tabIndex={0}
        aria-haspopup="dialog"
        aria-expanded={isExpanded}
        aria-label={`${getModuleTitle(key)}，可拖动并打开详情`}
        onPointerDown={handleModulePointerDown(key)}
        onPointerMove={handleModulePointerMove(key)}
        onPointerUp={handleModulePointerUp(key)}
        onPointerCancel={handleModulePointerCancel(key)}
        onKeyDown={handleModuleKeyDown(key)}
      >
        <section className="mirror-page__card-surface" aria-hidden="true">
          <div className="mirror-page__card-shell">
            <StatusBadge tone={summary.tone}>{summary.badge}</StatusBadge>
            <p className={summaryClassName}>{summary.mainLine}</p>
          </div>
        </section>
      </div>
    );
  };

  return (
    <main className="app-shell mirror-page">
      <div className="mirror-page__canvas mirror-page__canvas--full" ref={canvasRef} aria-label="Mirror 卡片工作板">
        <aside className={`mirror-page__source-status ${sourceStatus.className}`} aria-label="Mirror 数据来源状态">
          <StatusBadge tone={source === "rpc" ? "green" : "processing"}>{sourceStatus.badge}</StatusBadge>
          <div className="mirror-page__source-copy">
            <p className="mirror-page__source-title">{sourceStatus.title}</p>
            <p className="mirror-page__source-description">{sourceStatus.description}</p>
          </div>
        </aside>
        {moduleStack.map(renderDraggableModule)}
        {renderDetailOverlay()}
      </div>
    </main>
  );
}
