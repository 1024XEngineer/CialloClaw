import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import type { MirrorOverviewUpdatedNotification } from "@cialloclaw/protocol";
import { X } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { PanelSurface, StatusBadge } from "@cialloclaw/ui";
import { subscribeMirrorOverviewUpdated } from "@/rpc/subscriptions";
import { useDashboardEscapeHandler } from "@/features/dashboard/shared/dashboardEscapeCoordinator";
import {
  formatDashboardSettingsMutationFeedback,
  updateDashboardSettings,
  type DashboardSettingsPatch,
} from "@/features/dashboard/shared/dashboardSettingsMutation";
import {
  applyMirrorSettingsSnapshot,
  loadMirrorOverviewData,
  type MirrorOverviewData,
  type MirrorOverviewSource,
} from "./mirrorService";
import { MirrorDetailContent, type MirrorHistoryDetailView } from "./MirrorDetailContent";
import { loadMirrorFloatingPositions, saveMirrorFloatingPositions } from "./mirrorLayoutStorage";
import { MirrorDecorativeBirds } from "./MirrorDecorativeBirds";
import {
  DEFAULT_MIRROR_DIRECTION_STACK,
  FLOATING_MIRROR_DIRECTION_KEYS,
  MIRROR_ORBITAL_TARGETS,
  getMirrorDirectionMeta,
  type FloatingMirrorDirectionKey,
  type MirrorCardAccent,
  type MirrorDirectionKey,
} from "./mirrorDirections";
import { buildMirrorProfileView } from "./mirrorViewModel";
import "./mirror.css";

type ModulePosition = { x: number; y: number };
type ModulePositions = Record<MirrorDirectionKey, ModulePosition>;
type ModuleSize = { width: number; height: number };
type ModuleSizes = Record<MirrorDirectionKey, ModuleSize>;
type BoardBounds = { minX: number; minY: number; maxX: number; maxY: number };
type BoardGrid = { columns: number; rows: number };
type OccupiedModule = { position: ModulePosition; size: ModuleSize };
type LayoutMode = "default" | "compact";
type BoardLayout = {
  canvasWidth: number;
  canvasHeight: number;
  mode: LayoutMode;
  bounds: BoardBounds;
  memoryBounds: BoardBounds;
  regularSize: ModuleSize;
  moduleSizes: ModuleSizes;
  grid: BoardGrid;
  candidates: ModulePosition[];
};
type CardSummary = {
  badge: string;
  tone: string;
  mainLine: string;
  detailLine: string;
  accent: MirrorCardAccent;
  emphasis?: "memory";
};
type DetailBadge = {
  label: string;
  tone: string;
};
type DragState = {
  key: FloatingMirrorDirectionKey;
  pointerId: number;
  startX: number;
  startY: number;
  originX: number;
  originY: number;
  moved: boolean;
};
type MirrorRouteState = {
  activeDetailKey?: MirrorDirectionKey;
  focusMemoryId?: string;
  historyDetailView?: MirrorHistoryDetailView;
};

const INITIAL_MODULE_STACK: MirrorDirectionKey[] = DEFAULT_MIRROR_DIRECTION_STACK;
const DRAG_THRESHOLD = 8;
const BOARD_PADDING = 12;
const CARD_CLEARANCE = 10;
const CARD_STEP = 16;
const COMPACT_MEMORY_GAP = 14;
/* Keep floating mirror cards large enough to show two readable headline lines. */
const MIN_COMPACT_CARD_WIDTH = 132;
const MIN_COMPACT_CARD_HEIGHT = 132;
const MIN_COMPACT_MEMORY_HEIGHT = 132;
const DEFAULT_CARD_SIZE: ModuleSize = { width: 376, height: 252 };
const DEFAULT_MEMORY_CARD_SIZE: ModuleSize = { width: 480, height: 320 };
const PINNED_MEMORY_CARD_OFFSET = { x: 20, y: 104 };
const DEFAULT_MODULE_SIZES: ModuleSizes = {
  dailyStage: DEFAULT_CARD_SIZE,
  profile: DEFAULT_CARD_SIZE,
  memory: DEFAULT_MEMORY_CARD_SIZE,
  history: DEFAULT_CARD_SIZE,
};
const DEFAULT_MODULE_POSITIONS: ModulePositions = {
  dailyStage: { x: 0, y: 0 },
  profile: { x: 0, y: 0 },
  memory: { x: 0, y: 0 },
  history: { x: 0, y: 0 },
};

function isMirrorDirectionKey(value: string): value is MirrorDirectionKey {
  return INITIAL_MODULE_STACK.includes(value as MirrorDirectionKey);
}
function readMirrorRouteState(value: unknown) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }

  const state = value as MirrorRouteState;
  const focusMemoryId = typeof state.focusMemoryId === "string" && state.focusMemoryId.trim().length > 0 ? state.focusMemoryId : null;
  const activeDetailKey =
    typeof state.activeDetailKey === "string" && isMirrorDirectionKey(state.activeDetailKey)
      ? state.activeDetailKey
      : focusMemoryId
        ? "memory"
        : null;

  if (!activeDetailKey) {
    return null;
  }

  return {
    activeDetailKey,
    focusMemoryId,
    historyDetailView:
      activeDetailKey === "history" &&
      (state.historyDetailView === "summary" || state.historyDetailView === "conversation")
        ? state.historyDetailView
        : null,
  };
}

function formatShortMirrorDate(value: string) {
  return new Date(value).toLocaleDateString("zh-CN", {
    month: "short",
    day: "numeric",
  });
}

function formatMirrorDateTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
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

function clampDimension(value: number, min: number, max: number) {
  if (max <= 1) {
    return 1;
  }

  const effectiveMin = Math.min(min, max);
  return Math.min(Math.max(value, effectiveMin), max);
}

function getBoardGrid(canvasWidth: number, canvasHeight: number): BoardGrid {
  let bestGrid: BoardGrid = { columns: FLOATING_MIRROR_DIRECTION_KEYS.length, rows: 1 };
  let bestScore = Number.NEGATIVE_INFINITY;

  for (let columns = 1; columns <= FLOATING_MIRROR_DIRECTION_KEYS.length; columns += 1) {
    const rows = Math.ceil(FLOATING_MIRROR_DIRECTION_KEYS.length / columns);
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
    width: clampValue(width, 1, DEFAULT_CARD_SIZE.width),
    height: clampValue(height, 1, DEFAULT_CARD_SIZE.height),
  } satisfies ModuleSize;
}

function getMemoryCardSize(canvasWidth: number, canvasHeight: number) {
  const maxWidth = Math.max(1, canvasWidth - BOARD_PADDING * 2);
  const maxHeight = Math.max(1, canvasHeight - BOARD_PADDING * 2);

  return {
    width: clampDimension(Math.floor(canvasWidth * 0.42), 360, Math.min(640, maxWidth)),
    height: clampDimension(Math.floor(canvasHeight * 0.38), 272, Math.min(420, maxHeight)),
  } satisfies ModuleSize;
}

function getModuleSizes(regularSize: ModuleSize, memorySize: ModuleSize): ModuleSizes {
  return {
    dailyStage: regularSize,
    profile: regularSize,
    memory: memorySize,
    history: regularSize,
  } satisfies ModuleSizes;
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

function overlapsOccupied(position: ModulePosition, size: ModuleSize, occupied: OccupiedModule[]) {
  return occupied.some((item) => {
    const separatedHorizontally =
      position.x + size.width + CARD_CLEARANCE <= item.position.x ||
      item.position.x + item.size.width + CARD_CLEARANCE <= position.x;
    const separatedVertically =
      position.y + size.height + CARD_CLEARANCE <= item.position.y ||
      item.position.y + item.size.height + CARD_CLEARANCE <= position.y;

    return !(separatedHorizontally || separatedVertically);
  });
}

function resolveSettledPosition(target: ModulePosition, size: ModuleSize, occupied: OccupiedModule[], layout: BoardLayout) {
  const clampedTarget = clampPosition(target, layout.bounds);

  if (!overlapsOccupied(clampedTarget, size, occupied)) {
    return clampedTarget;
  }

  let bestCandidate = clampedTarget;
  let bestDistance = Number.POSITIVE_INFINITY;

  for (const candidate of layout.candidates) {
    if (overlapsOccupied(candidate, size, occupied)) {
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

function getGridModuleTargets(bounds: BoardBounds, grid: BoardGrid, size: ModuleSize): ModulePositions {
  const availableWidth = Math.max(size.width, bounds.maxX - bounds.minX + size.width);
  const availableHeight = Math.max(size.height, bounds.maxY - bounds.minY + size.height);
  const gridHeight = grid.rows * size.height + Math.max(0, grid.rows - 1) * CARD_CLEARANCE;
  const gridStartY = bounds.minY + Math.max(0, (availableHeight - gridHeight) / 2);
  const positions = { ...DEFAULT_MODULE_POSITIONS };

  FLOATING_MIRROR_DIRECTION_KEYS.forEach((key, index) => {
    const row = Math.floor(index / grid.columns);
    const indexInRow = index % grid.columns;
    const remainingCards = FLOATING_MIRROR_DIRECTION_KEYS.length - row * grid.columns;
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

function getOrbitalModuleTargets(bounds: BoardBounds) {
  const positions = { ...DEFAULT_MODULE_POSITIONS };
  const travelX = Math.max(0, bounds.maxX - bounds.minX);
  const travelY = Math.max(0, bounds.maxY - bounds.minY);

  FLOATING_MIRROR_DIRECTION_KEYS.forEach((key) => {
    const target = MIRROR_ORBITAL_TARGETS[key];
    positions[key] = {
      x: Math.round(bounds.minX + travelX * target.x),
      y: Math.round(bounds.minY + travelY * target.y),
    };
  });

  return positions;
}

function getDefaultModuleTargets(bounds: BoardBounds, grid: BoardGrid, size: ModuleSize): ModulePositions {
  const availableWidth = bounds.maxX - bounds.minX + size.width;
  const availableHeight = bounds.maxY - bounds.minY + size.height;
  const canUseOrbitalLayout = availableWidth >= size.width * 3.15 && availableHeight >= size.height * 2.6;

  if (!canUseOrbitalLayout) {
    return getGridModuleTargets(bounds, grid, size);
  }

  return getOrbitalModuleTargets(bounds);
}

function getPinnedMemoryTarget(bounds: BoardBounds) {
  return clampPosition(
    {
      x: bounds.minX + PINNED_MEMORY_CARD_OFFSET.x,
      y: bounds.minY + PINNED_MEMORY_CARD_OFFSET.y,
    },
    bounds,
  );
}

function getCompactLayout(canvasWidth: number, canvasHeight: number): BoardLayout {
  const canvasInnerWidth = Math.max(1, canvasWidth - BOARD_PADDING * 2);
  const canvasInnerHeight = Math.max(1, canvasHeight - BOARD_PADDING * 2);
  let bestLayout: BoardLayout | null = null;
  let bestScore = Number.NEGATIVE_INFINITY;

  for (let columns = 1; columns <= FLOATING_MIRROR_DIRECTION_KEYS.length; columns += 1) {
    const rows = Math.ceil(FLOATING_MIRROR_DIRECTION_KEYS.length / columns);
    const regularWidth = Math.floor((canvasInnerWidth - CARD_CLEARANCE * (columns - 1)) / columns);

    if (regularWidth < MIN_COMPACT_CARD_WIDTH) {
      continue;
    }

    const maxMemoryHeight =
      canvasInnerHeight -
      COMPACT_MEMORY_GAP -
      rows * MIN_COMPACT_CARD_HEIGHT -
      CARD_CLEARANCE * Math.max(0, rows - 1);

    if (maxMemoryHeight < MIN_COMPACT_MEMORY_HEIGHT) {
      continue;
    }

    const memoryHeight = clampValue(Math.floor(canvasHeight * 0.28), MIN_COMPACT_MEMORY_HEIGHT, Math.min(248, maxMemoryHeight));
    const availableRegularHeight =
      canvasInnerHeight - memoryHeight - COMPACT_MEMORY_GAP - CARD_CLEARANCE * Math.max(0, rows - 1);
    const regularHeight = clampValue(
      Math.floor(Math.min(availableRegularHeight / rows, regularWidth * 0.72)),
      MIN_COMPACT_CARD_HEIGHT,
      224,
    );
    const score = regularWidth * regularHeight;

    if (score <= bestScore) {
      continue;
    }

    const regularSize = {
      width: regularWidth,
      height: regularHeight,
    } satisfies ModuleSize;
    const memorySize = {
      width: canvasInnerWidth,
      height: memoryHeight,
    } satisfies ModuleSize;
    const bounds = getBoardBounds(canvasWidth, canvasHeight, regularSize);
    const memoryBounds = getBoardBounds(canvasWidth, canvasHeight, memorySize);

    bestLayout = {
      canvasWidth,
      canvasHeight,
      mode: "compact",
      bounds,
      memoryBounds,
      regularSize,
      moduleSizes: getModuleSizes(regularSize, memorySize),
      grid: { columns, rows },
      candidates: buildBoardCandidates(bounds),
    };
    bestScore = score;
  }

  if (bestLayout) {
    return bestLayout;
  }

  const fallbackGrid = { columns: FLOATING_MIRROR_DIRECTION_KEYS.length, rows: 1 } satisfies BoardGrid;
  const regularSize = {
    width: Math.max(1, Math.floor((canvasInnerWidth - CARD_CLEARANCE * (fallbackGrid.columns - 1)) / fallbackGrid.columns)),
    height: clampValue(Math.floor(canvasInnerHeight * 0.32), 1, 180),
  } satisfies ModuleSize;
  const memoryHeight = Math.max(1, canvasInnerHeight - regularSize.height - COMPACT_MEMORY_GAP);
  const memorySize = {
    width: canvasInnerWidth,
    height: memoryHeight,
  } satisfies ModuleSize;

  return {
    canvasWidth,
    canvasHeight,
    mode: "compact",
    bounds: getBoardBounds(canvasWidth, canvasHeight, regularSize),
    memoryBounds: getBoardBounds(canvasWidth, canvasHeight, memorySize),
    regularSize,
    moduleSizes: getModuleSizes(regularSize, memorySize),
    grid: fallbackGrid,
    candidates: buildBoardCandidates(getBoardBounds(canvasWidth, canvasHeight, regularSize)),
  };
}

function getCompactModuleTargets(layout: BoardLayout): ModulePositions {
  const positions = { ...DEFAULT_MODULE_POSITIONS };
  const memoryWidth = layout.moduleSizes.memory.width;
  const memoryHeight = layout.moduleSizes.memory.height;
  const regularSize = layout.regularSize;
  const horizontalGap = CARD_CLEARANCE;
  const verticalGap = CARD_CLEARANCE;
  const memoryPosition = clampPosition(
    {
      x: Math.round((layout.memoryBounds.minX + layout.memoryBounds.maxX) / 2),
      y: layout.memoryBounds.minY,
    },
    layout.memoryBounds,
  );
  const rows = layout.grid.rows;
  const contentHeight =
    rows * regularSize.height + Math.max(0, rows - 1) * verticalGap;
  const startY = clampValue(
    memoryPosition.y + memoryHeight + COMPACT_MEMORY_GAP,
    layout.bounds.minY,
    Math.max(layout.bounds.minY, layout.bounds.maxY - contentHeight + regularSize.height),
  );

  positions.memory = memoryPosition;

  FLOATING_MIRROR_DIRECTION_KEYS.forEach((key, index) => {
    const row = Math.floor(index / layout.grid.columns);
    const remainingCards = FLOATING_MIRROR_DIRECTION_KEYS.length - row * layout.grid.columns;
    const cardsInRow = Math.min(layout.grid.columns, remainingCards);
    const rowWidth = cardsInRow * regularSize.width + Math.max(0, cardsInRow - 1) * horizontalGap;
    const rowStartX = layout.bounds.minX + Math.max(0, (memoryWidth - rowWidth) / 2);

    positions[key] = clampPosition(
      {
        x: rowStartX + (index % layout.grid.columns) * (regularSize.width + horizontalGap),
        y: startY + row * (regularSize.height + verticalGap),
      },
      layout.bounds,
    );
  });

  return positions;
}

function normalizeModulePositions(targets: ModulePositions, layout: BoardLayout) {
  if (layout.mode === "compact") {
    return getCompactModuleTargets(layout);
  }

  const nextPositions = { ...DEFAULT_MODULE_POSITIONS };
  const pinnedMemoryPosition = getPinnedMemoryTarget(layout.memoryBounds);
  const occupied: OccupiedModule[] = [{ position: pinnedMemoryPosition, size: layout.moduleSizes.memory }];

  nextPositions.memory = pinnedMemoryPosition;

  for (const key of FLOATING_MIRROR_DIRECTION_KEYS) {
    const settledPosition = resolveSettledPosition(targets[key], layout.moduleSizes[key], occupied, layout);

    if (!settledPosition) {
      return getCompactModuleTargets(getCompactLayout(layout.canvasWidth, layout.canvasHeight));
    }

    nextPositions[key] = settledPosition;
    occupied.push({ position: settledPosition, size: layout.moduleSizes[key] });
  }

  return nextPositions;
}

function getDirectionTitle(key: MirrorDirectionKey) {
  return getMirrorDirectionMeta(key).title;
}

function getDirectionEyebrow(key: MirrorDirectionKey) {
  return getMirrorDirectionMeta(key).eyebrow;
}

function getDirectionPlateCode(key: MirrorDirectionKey) {
  const codes: Record<MirrorDirectionKey, string> = {
    dailyStage: "SL-02",
    profile: "SL-03",
    memory: "MS-01",
    history: "SL-04",
  };

  return codes[key];
}

function pickFloatingModulePositions(positions: ModulePositions): Record<FloatingMirrorDirectionKey, ModulePosition> {
  return {
    dailyStage: positions.dailyStage,
    profile: positions.profile,
    history: positions.history,
  };
}

export function MirrorApp() {
  const location = useLocation();
  const navigate = useNavigate();
  const storedFloatingPositionsRef = useRef(loadMirrorFloatingPositions());
  const hasStoredFloatingPositionsRef = useRef(storedFloatingPositionsRef.current !== null);
  const [mirrorData, setMirrorData] = useState<MirrorOverviewData | null>(null);
  const dataMode: MirrorOverviewSource = "rpc";
  const [loadError, setLoadError] = useState<string | null>(null);
  const [modulePositions, setModulePositions] = useState<ModulePositions>(() => ({
    ...DEFAULT_MODULE_POSITIONS,
    ...(storedFloatingPositionsRef.current ?? {}),
  }));
  const [moduleStack, setModuleStack] = useState<MirrorDirectionKey[]>(INITIAL_MODULE_STACK);
  const [moduleSizes, setModuleSizes] = useState<ModuleSizes>(DEFAULT_MODULE_SIZES);
  const [layoutMode, setLayoutMode] = useState<LayoutMode>("default");
  const [draggingKey, setDraggingKey] = useState<FloatingMirrorDirectionKey | null>(null);
  const [activeDetailKey, setActiveDetailKey] = useState<MirrorDirectionKey | null>(null);
  const [focusedMemoryId, setFocusedMemoryId] = useState<string | null>(null);
  const [historyDetailView, setHistoryDetailView] = useState<MirrorHistoryDetailView>("conversation");
  const [boardReady, setBoardReady] = useState(false);
  const [lastMirrorUpdate, setLastMirrorUpdate] = useState<MirrorOverviewUpdatedNotification | null>(null);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const modulePositionsRef = useRef<ModulePositions>(DEFAULT_MODULE_POSITIONS);
  const hasPlacedModulesRef = useRef(false);
  const isMountedRef = useRef(true);
  const dataModeRef = useRef<MirrorOverviewSource>(dataMode);
  const fetchInFlightRef = useRef(false);
  const pendingRefreshRef = useRef(false);
  const refreshSequenceRef = useRef(0);
  const lastSavedFloatingPositionsRef = useRef<string | null>(null);

  dataModeRef.current = dataMode;

  const openDetail = useCallback((key: MirrorDirectionKey, options?: { focusMemoryId?: string | null; historyDetailView?: MirrorHistoryDetailView | null }) => {
    setActiveDetailKey(key);
    setFocusedMemoryId(key === "memory" ? options?.focusMemoryId ?? null : null);
    if (key === "history" && options?.historyDetailView) {
      setHistoryDetailView(options.historyDetailView);
    }
  }, []);

  const closeDetail = useCallback(() => {
    setActiveDetailKey(null);
    setFocusedMemoryId(null);
  }, []);

  useDashboardEscapeHandler({
    enabled: activeDetailKey !== null,
    handleEscape: closeDetail,
    priority: 220,
  });

  useEffect(() => {
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    const routeState = readMirrorRouteState(location.state);

    if (!routeState) {
      return;
    }

    openDetail(routeState.activeDetailKey, {
      focusMemoryId: routeState.focusMemoryId,
      historyDetailView: routeState.historyDetailView,
    });
    navigate(location.pathname, { replace: true, state: null });
  }, [location.pathname, location.state, navigate, openDetail]);

  useEffect(() => {
    if (!mirrorData || mirrorData.conversations.length > 0 || historyDetailView === "summary") {
      return;
    }

    setHistoryDetailView("summary");
  }, [historyDetailView, mirrorData]);

  const refreshMirrorData = useCallback(() => {
    if (fetchInFlightRef.current) {
      pendingRefreshRef.current = true;
      return;
    }

    fetchInFlightRef.current = true;
    let nextSequence = ++refreshSequenceRef.current;

    void (async () => {
      try {
        do {
          pendingRefreshRef.current = false;
          const nextData = await loadMirrorOverviewData("rpc");

          if (!isMountedRef.current || refreshSequenceRef.current !== nextSequence) {
            return;
          }

          setLoadError(null);
          setMirrorData(nextData);

          if (pendingRefreshRef.current) {
            nextSequence = ++refreshSequenceRef.current;
          }
        } while (pendingRefreshRef.current);
      } catch (error) {
        if (!isMountedRef.current || refreshSequenceRef.current !== nextSequence) {
          return;
        }

        setLoadError(error instanceof Error ? error.message : "镜子数据请求失败");
      } finally {
        fetchInFlightRef.current = false;

        if (pendingRefreshRef.current && isMountedRef.current && dataModeRef.current === "rpc") {
          refreshMirrorData();
        }
      }
    })();
  }, []);

  useEffect(() => {
    setMirrorData(null);

    const unsubscribe = subscribeMirrorOverviewUpdated((notification) => {
      setLastMirrorUpdate(notification);
      refreshMirrorData();
    });

    refreshMirrorData();

    return () => {
      unsubscribe();
    };
  }, [dataMode, refreshMirrorData]);

  const bringModuleToFront = useCallback((key: MirrorDirectionKey) => {
    setModuleStack((currentStack) => [...currentStack.filter((item) => item !== key), key]);
  }, []);

  const persistFloatingModulePositions = useCallback((positions: ModulePositions) => {
    const floatingPositions = pickFloatingModulePositions(positions);
    const serializedFloatingPositions = JSON.stringify(floatingPositions);

    if (lastSavedFloatingPositionsRef.current === serializedFloatingPositions) {
      return;
    }

    saveMirrorFloatingPositions(floatingPositions);
    lastSavedFloatingPositionsRef.current = serializedFloatingPositions;
  }, []);

  const getBoardLayout = useCallback(() => {
    const canvas = canvasRef.current;

    if (!canvas) {
      return null;
    }

    if (canvas.clientWidth <= 760 || canvas.clientHeight <= 560) {
      return getCompactLayout(canvas.clientWidth, canvas.clientHeight);
    }

    const grid = getBoardGrid(canvas.clientWidth, canvas.clientHeight);
    const regularSize = getBoardCardSize(canvas.clientWidth, canvas.clientHeight, grid);
    const memorySize = getMemoryCardSize(canvas.clientWidth, canvas.clientHeight);
    const bounds = getBoardBounds(canvas.clientWidth, canvas.clientHeight, regularSize);
    const memoryBounds = getBoardBounds(canvas.clientWidth, canvas.clientHeight, memorySize);

    return {
      canvasWidth: canvas.clientWidth,
      canvasHeight: canvas.clientHeight,
      mode: "default",
      bounds,
      memoryBounds,
      regularSize,
      moduleSizes: getModuleSizes(regularSize, memorySize),
      grid,
      candidates: buildBoardCandidates(bounds),
    } satisfies BoardLayout;
  }, []);

  const getSettledModulePosition = useCallback(
    (key: MirrorDirectionKey, target: ModulePosition, positions: ModulePositions) => {
      const layout = getBoardLayout();

      if (!layout) {
        return target;
      }

      if (layout.mode === "compact") {
        return positions[key];
      }

      const occupied = INITIAL_MODULE_STACK.filter((item) => item !== key).map((item) => ({
        position: positions[item],
        size: layout.moduleSizes[item],
      }));
      return resolveSettledPosition(target, layout.moduleSizes[key], occupied, layout) ?? positions[key];
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

      setLayoutMode(layout.mode);
      setModuleSizes(layout.moduleSizes);
      setModulePositions((currentPositions) => {
        const targets = hasPlacedModulesRef.current
          ? currentPositions
          : {
              ...getDefaultModuleTargets(layout.bounds, layout.grid, layout.regularSize),
              ...(hasStoredFloatingPositionsRef.current ? pickFloatingModulePositions(currentPositions) : {}),
            };
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
    modulePositionsRef.current = modulePositions;
  }, [modulePositions]);

  useEffect(() => {
    if (!boardReady || draggingKey) {
      return;
    }

    persistFloatingModulePositions(modulePositions);
  }, [boardReady, draggingKey, modulePositions, persistFloatingModulePositions]);

  const handleSettingsUpdate = useCallback(
    async (subject: string, patch: DashboardSettingsPatch) => {
      const result = await updateDashboardSettings(patch, dataMode);

      if (isMountedRef.current) {
        setLoadError(null);
        setMirrorData((current) => (current ? applyMirrorSettingsSnapshot(current, result.snapshot) : current));
      }

      return formatDashboardSettingsMutationFeedback(result, subject);
    },
    [dataMode],
  );

  if (!mirrorData) {
    return (
      <main className="app-shell mirror-page">
        <div className="mirror-page__canvas mirror-page__canvas--full mirror-page__canvas--loading">
          <p className="mirror-page__loading-copy">{loadError ? `镜子页同步失败：${loadError}` : "正在点亮检片台…"}</p>
        </div>
      </main>
    );
  }

  const profileView = buildMirrorProfileView(mirrorData.profileItems);

  const { overview } = mirrorData;
  const dataSourceDetails = [
    mirrorData.source === "rpc"
      ? "当前展示来自本地服务。"
      : "当前展示的是本地缓存的概览，用于在服务不可用时保持页面可读性。",
  ];

  if (mirrorData.rpcContext.serverTime) {
    dataSourceDetails.push(`服务端时间 ${formatMirrorDateTime(mirrorData.rpcContext.serverTime)}`);
  }

  if (lastMirrorUpdate) {
    dataSourceDetails.push(
      lastMirrorUpdate.source
        ? `最近通知 revision #${lastMirrorUpdate.revision} · ${lastMirrorUpdate.source}`
        : `最近通知 revision #${lastMirrorUpdate.revision}`,
    );
  }

  if (mirrorData.rpcContext.warnings.length) {
    dataSourceDetails.push(`warnings：${mirrorData.rpcContext.warnings.join("；")}`);
  }

  if (loadError && dataMode === "rpc") {
    dataSourceDetails.push(`error：${loadError}`);
  }

  const dataSourceBadge =
    mirrorData.source === "rpc"
      ? { label: "LIVE", tone: "green" as const, copy: dataSourceDetails.join(" · ") }
      : { label: "本地", tone: "processing" as const, copy: dataSourceDetails.join(" · ") };
  const latestMemoryReference = overview.memory_references[0] ?? null;
  const latestConversation = mirrorData.conversations[0] ?? null;

  const releaseDrag = () => {
    dragStateRef.current = null;
    setDraggingKey(null);
  };

  const handleModulePointerDown = (key: FloatingMirrorDirectionKey) => (event: PointerEvent<HTMLDivElement>) => {
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

  const handleModulePointerMove = (key: FloatingMirrorDirectionKey) => (event: PointerEvent<HTMLDivElement>) => {
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

  const handleModulePointerUp = (key: FloatingMirrorDirectionKey) => (event: PointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;

    if (!dragState || dragState.key !== key || dragState.pointerId !== event.pointerId) {
      return;
    }

    const travelled = Math.hypot(event.clientX - dragState.startX, event.clientY - dragState.startY);

    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    releaseDrag();

    if (dragState.moved) {
      persistFloatingModulePositions(modulePositionsRef.current);
    }

    if (!dragState.moved && travelled < DRAG_THRESHOLD) {
      openDetail(key);
    }
  };

  const handleModulePointerCancel = (_key: FloatingMirrorDirectionKey) => (event: PointerEvent<HTMLDivElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    releaseDrag();
  };

  const handleModuleKeyDown = (key: MirrorDirectionKey) => (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      bringModuleToFront(key);
      openDetail(key);
    }
  };

  const getDetailBadge = (key: MirrorDirectionKey): DetailBadge => {
    if (key === "dailyStage") {
      return {
        label: formatShortMirrorDate(mirrorData.dailyDigest.date),
        tone: "processing",
      };
    }

    if (key === "profile") {
      return {
        label: profileView.total_items > 0 ? `${profileView.total_items} 项资料` : "暂无资料",
        tone: "green",
      };
    }

    if (key === "history") {
      return {
        label: mirrorData.conversationSummary.total_records ? `${mirrorData.conversationSummary.total_records} 条对话` : "暂无对话",
        tone: "processing",
      };
    }

    return {
      label: overview.memory_references.length ? `${overview.memory_references.length} 条引用` : "暂无引用",
      tone: "processing",
    };
  };

  const renderDetailOverlay = () => {
    if (!activeDetailKey) {
      return null;
    }

    const detailBadge = getDetailBadge(activeDetailKey);
    const detailTitleAccessory =
      activeDetailKey === "history" ? (
        <div className="mirror-page__detail-tab-list mirror-page__detail-tab-list--title" role="tablist" aria-label="历史详情视图" data-testid="mirror-history-tabs">
          <button
            type="button"
            role="tab"
            aria-selected={historyDetailView === "summary"}
            className="mirror-page__detail-tab-trigger"
            data-active={historyDetailView === "summary" ? "" : undefined}
            onClick={() => setHistoryDetailView("summary")}
          >
            历史概要
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={historyDetailView === "conversation"}
            className="mirror-page__detail-tab-trigger"
            data-active={historyDetailView === "conversation" ? "" : undefined}
            onClick={() => setHistoryDetailView("conversation")}
          >
            最近 100 条本地对话
          </button>
        </div>
      ) : null;

    return (
      <div className="mirror-page__detail-layer" onClick={closeDetail}>
        <div
          className="mirror-page__detail-panel"
          role="dialog"
          aria-modal="true"
          aria-label={`${getDirectionTitle(activeDetailKey)}详情`}
          data-testid={`mirror-detail-${activeDetailKey}`}
          onClick={(event) => event.stopPropagation()}
        >
          <PanelSurface
            title={getDirectionTitle(activeDetailKey)}
            eyebrow={getDirectionEyebrow(activeDetailKey)}
            titleAccessory={detailTitleAccessory}
          >
            <div className="mirror-page__detail-topbar">
              <div className="mirror-page__detail-meta">
                <StatusBadge tone={detailBadge.tone}>{detailBadge.label}</StatusBadge>
              </div>
              <button type="button" className="mirror-page__close-button" onClick={closeDetail} aria-label="关闭详情视图">
                <X className="mirror-page__close-icon" />
              </button>
            </div>
            <div className="mirror-page__detail-body">
              <MirrorDetailContent
                activeDetailKey={activeDetailKey}
                conversationSummary={mirrorData.conversationSummary}
                conversations={mirrorData.conversations}
                dailyDigest={mirrorData.dailyDigest}
                focusMemoryId={focusedMemoryId}
                historyDetailView={historyDetailView}
                latestRestorePoint={mirrorData.latestRestorePoint}
                overview={overview}
                onUpdateSettings={handleSettingsUpdate}
                profileView={profileView}
                rpcContext={mirrorData.rpcContext}
                settingsSnapshot={mirrorData.settingsSnapshot}
              />
            </div>
          </PanelSurface>
        </div>
      </div>
    );
  };

  const getCardSummary = (key: MirrorDirectionKey): CardSummary => {
    if (key === "dailyStage") {
      return {
        badge: formatShortMirrorDate(mirrorData.dailyDigest.date),
        tone: "processing",
        detailLine: mirrorData.dailyDigest.lede,
        accent: getMirrorDirectionMeta(key).accent,
        mainLine: mirrorData.dailyDigest.headline,
      };
    }

    if (key === "profile") {
      const primaryProfileItem = profileView.backend_items[0] ?? profileView.local_stat_items[0] ?? null;
      return {
        badge: profileView.backend_items.length > 0 ? `${profileView.backend_items.length} 个画像项` : "暂无画像项",
        tone: "green",
        detailLine: profileView.local_stat_items.length > 0 ? `${profileView.local_stat_items.length} 条最近本地统计。` : "仅显示你的画像信息。",
        accent: getMirrorDirectionMeta(key).accent,
        mainLine: primaryProfileItem?.value ?? "暂无画像资料",
      };
    }

    if (key === "history") {
      const historyHeadline = overview.history_summary[0] ?? latestConversation?.user_text ?? "暂无历史概要";
      const historyDetail =
        overview.history_summary[1] ??
        latestConversation?.agent_text ??
        latestConversation?.user_text ??
        (mirrorData.conversationSummary.total_records > 0
          ? `本地仍保留 ${mirrorData.conversationSummary.total_records} 条最近对话。`
          : "轻触查看历史概要与本地最近 100 条对话记录。");

      return {
        badge:
          overview.history_summary.length > 0
            ? `${overview.history_summary.length} 条概要`
            : mirrorData.conversationSummary.total_records
              ? `${mirrorData.conversationSummary.total_records} 条本地对话`
              : "暂无历史概要",
        tone: "processing",
        detailLine: historyDetail,
        accent: getMirrorDirectionMeta(key).accent,
        mainLine: historyHeadline,
      };
    }

    const memorySummary = latestMemoryReference?.summary || latestMemoryReference?.reason;

    return {
      badge: `${overview.memory_references.length} 条引用`,
      tone: "processing",
      detailLine:
        memorySummary ??
        (mirrorData.conversationSummary.total_records > 0
          ? `暂无新引用；本地仍保留 ${mirrorData.conversationSummary.total_records} 条最近对话统计。`
          : "等待新的记忆引用记录。"),
      accent: getMirrorDirectionMeta(key).accent,
      mainLine: latestMemoryReference?.memory_id ?? "暂无近期被调用记忆",
      emphasis: "memory",
    };
  };

  const renderDraggableModule = (key: MirrorDirectionKey) => {
    const isDragging = draggingKey === key;
    const isExpanded = activeDetailKey === key;
    const isPinnedMemoryCard = key === "memory";
    const moduleSize = moduleSizes[key];
    const summary = getCardSummary(key);
    const directionMeta = getMirrorDirectionMeta(key);
    const inspectionCode = getDirectionPlateCode(key);
    const summaryClassName = [
      "mirror-page__card-line",
      summary.emphasis ? `mirror-page__card-line--${summary.emphasis}` : null,
    ]
      .filter(Boolean)
      .join(" ");

    const pointerHandlers = isPinnedMemoryCard || layoutMode === "compact"
      ? {
          onClick: () => {
            bringModuleToFront(key);
            openDetail(key);
          },
        }
      : {
          onPointerDown: handleModulePointerDown(key),
          onPointerMove: handleModulePointerMove(key),
          onPointerUp: handleModulePointerUp(key),
          onPointerCancel: handleModulePointerCancel(key),
        };

    return (
      <div
        key={key}
        className={`mirror-page__draggable mirror-page__draggable--${key}${isPinnedMemoryCard ? " mirror-page__draggable--pinned" : ""}${isDragging ? " is-dragging" : ""}${isExpanded ? " is-active" : ""}${boardReady ? " is-ready" : ""}`}
        data-accent={summary.accent}
        data-surface-kind={isPinnedMemoryCard ? "master" : "slide"}
        data-testid={`mirror-card-${key}`}
        style={{
          height: `${moduleSize.height}px`,
          transform: `translate3d(${modulePositions[key].x}px, ${modulePositions[key].y}px, 0)`,
          width: `${moduleSize.width}px`,
        }}
        role="button"
        tabIndex={0}
        aria-haspopup="dialog"
        aria-expanded={isExpanded}
        aria-label={`${getDirectionTitle(key)}，${isPinnedMemoryCard || layoutMode === "compact" ? "可打开详情" : "可拖动并打开详情"}`}
        onKeyDown={handleModuleKeyDown(key)}
        {...pointerHandlers}
      >
        <section className={`mirror-page__card-surface mirror-page__card-surface--${isPinnedMemoryCard ? "master" : "slide"}`} aria-hidden="true">
          {isPinnedMemoryCard ? <span className="mirror-page__master-clip" aria-hidden="true" /> : <span className="mirror-page__slide-tab" aria-hidden="true" />}
          <div className="mirror-page__surface-registers" aria-hidden="true">
            <span className="mirror-page__surface-register mirror-page__surface-register--top-left" />
            <span className="mirror-page__surface-register mirror-page__surface-register--top-right" />
            <span className="mirror-page__surface-register mirror-page__surface-register--bottom-left" />
            <span className="mirror-page__surface-register mirror-page__surface-register--bottom-right" />
          </div>
          <div className="mirror-page__card-shell">
            <div className="mirror-page__card-top">
              <div className="mirror-page__card-heading">
                <p className="mirror-page__card-kicker">{directionMeta.eyebrow}</p>
                <p className="mirror-page__card-title">{directionMeta.title}</p>
              </div>
              <div className="mirror-page__card-top-meta">
                <p className="mirror-page__surface-code">{inspectionCode}</p>
                <StatusBadge tone={summary.tone}>{summary.badge}</StatusBadge>
              </div>
            </div>
            <p className={summaryClassName}>{summary.mainLine}</p>
            <p className="mirror-page__card-detail">{summary.detailLine}</p>
            <p className="mirror-page__module-hint">{directionMeta.hint}</p>
          </div>
        </section>
      </div>
    );
  };

  return (
    <main className="app-shell mirror-page">
      <div className="mirror-page__canvas mirror-page__canvas--full" ref={canvasRef} aria-label="Mirror 检片台" data-testid="mirror-canvas">
        <div className="mirror-page__source-status" aria-live="polite">
          <StatusBadge tone={dataSourceBadge.tone}>{dataSourceBadge.label}</StatusBadge>
          <p className="mirror-page__source-copy">{dataSourceBadge.copy}</p>
        </div>
        <section className="mirror-page__scene" aria-hidden="true">
          <div className="mirror-page__desk-glow mirror-page__desk-glow--north" />
          <div className="mirror-page__desk-glow mirror-page__desk-glow--east" />
          <div className="mirror-page__desk-shadow-band" />
          <div className="mirror-page__inspection-field">
            <div className="mirror-page__inspection-field-core" />
            <div className="mirror-page__inspection-grid" />
            <div className="mirror-page__inspection-register mirror-page__inspection-register--horizontal" />
            <div className="mirror-page__inspection-register mirror-page__inspection-register--vertical" />
            <span className="mirror-page__inspection-corner mirror-page__inspection-corner--top-left" />
            <span className="mirror-page__inspection-corner mirror-page__inspection-corner--top-right" />
            <span className="mirror-page__inspection-corner mirror-page__inspection-corner--bottom-left" />
            <span className="mirror-page__inspection-corner mirror-page__inspection-corner--bottom-right" />
            <span className="mirror-page__inspection-pin mirror-page__inspection-pin--top" />
            <span className="mirror-page__inspection-pin mirror-page__inspection-pin--right" />
          </div>
          <MirrorDecorativeBirds />
        </section>
        {moduleStack.map(renderDraggableModule)}
        {activeDetailKey ? <div data-testid="mirror-detail-overlay">{renderDetailOverlay()}</div> : null}
      </div>
    </main>
  );
}
