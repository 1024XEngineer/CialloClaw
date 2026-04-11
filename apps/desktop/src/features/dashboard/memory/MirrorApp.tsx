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
import { BookMarked, BrainCircuit, CalendarDays, Clock3, X } from "lucide-react";
import { PanelSurface, StatusBadge } from "@cialloclaw/ui";
import { subscribeMirrorOverviewUpdated } from "@/rpc/subscriptions";
import { loadMirrorOverviewData, type MirrorOverviewData } from "./mirrorService";
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

const INITIAL_MODULE_STACK: MirrorDirectionKey[] = DEFAULT_MIRROR_DIRECTION_STACK;
const DRAG_THRESHOLD = 8;
const BOARD_PADDING = 12;
const CARD_CLEARANCE = 10;
const CARD_STEP = 16;
const COMPACT_MEMORY_GAP = 14;
const MIN_COMPACT_CARD_WIDTH = 92;
const MIN_COMPACT_CARD_HEIGHT = 92;
const MIN_COMPACT_MEMORY_HEIGHT = 132;
const DEFAULT_CARD_SIZE: ModuleSize = { width: 260, height: 168 };
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
    width: clampValue(width, 1, 264),
    height: clampValue(height, 1, 172),
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
      172,
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
    height: clampValue(Math.floor(canvasInnerHeight * 0.26), 1, 136),
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
  const storedFloatingPositionsRef = useRef(loadMirrorFloatingPositions());
  const hasStoredFloatingPositionsRef = useRef(storedFloatingPositionsRef.current !== null);
  const [mirrorData, setMirrorData] = useState<MirrorOverviewData | null>(null);
  const [modulePositions, setModulePositions] = useState<ModulePositions>(() => ({
    ...DEFAULT_MODULE_POSITIONS,
    ...(storedFloatingPositionsRef.current ?? {}),
  }));
  const [moduleStack, setModuleStack] = useState<MirrorDirectionKey[]>(INITIAL_MODULE_STACK);
  const [moduleSizes, setModuleSizes] = useState<ModuleSizes>(DEFAULT_MODULE_SIZES);
  const [layoutMode, setLayoutMode] = useState<LayoutMode>("default");
  const [draggingKey, setDraggingKey] = useState<FloatingMirrorDirectionKey | null>(null);
  const [activeDetailKey, setActiveDetailKey] = useState<MirrorDirectionKey | null>(null);
  const [boardReady, setBoardReady] = useState(false);
  const [lastMirrorUpdate, setLastMirrorUpdate] = useState<MirrorOverviewUpdatedNotification | null>(null);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<DragState | null>(null);
  const modulePositionsRef = useRef<ModulePositions>(DEFAULT_MODULE_POSITIONS);
  const hasPlacedModulesRef = useRef(false);
  const isMountedRef = useRef(true);
  const fetchInFlightRef = useRef(false);
  const pendingRefreshRef = useRef(false);
  const lastSavedFloatingPositionsRef = useRef<string | null>(null);
  const latestMirrorRevisionRef = useRef<number | null>(null);

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
    const unsubscribe = subscribeMirrorOverviewUpdated((notification) => {
      const lastRevision = latestMirrorRevisionRef.current;

      if (lastRevision !== null && notification.revision <= lastRevision) {
        return;
      }

      latestMirrorRevisionRef.current = notification.revision;
      setLastMirrorUpdate(notification);
      refreshMirrorData();
    });

    refreshMirrorData();

    return () => {
      isMountedRef.current = false;
      unsubscribe();
    };
  }, [refreshMirrorData]);

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
    return (
      <main className="app-shell mirror-page">
        <div className="mirror-page__canvas mirror-page__canvas--full mirror-page__canvas--loading">
          <p className="mirror-page__loading-copy">正在点亮检片台…</p>
        </div>
      </main>
    );
  }

  const { overview, insight } = mirrorData;
  const dataSourceDetails = [
    mirrorData.source === "rpc"
      ? "当前展示来自本地 JSON-RPC 服务。"
      : "JSON-RPC 不可用，当前展示本地回退数据。",
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

  const dataSourceBadge =
    mirrorData.source === "rpc"
      ? { label: "LIVE", tone: "green" as const, copy: dataSourceDetails.join(" · ") }
      : { label: "MOCK", tone: "processing" as const, copy: dataSourceDetails.join(" · ") };
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
      setActiveDetailKey(key);
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
              <p className="mirror-page__history-label">概要片段 {index + 1}</p>
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

        <article className="mirror-page__profile-card mirror-page__profile-card--accent">
          <div className="mirror-page__profile-heading">
            <BookMarked className="mirror-page__profile-icon" />
            <span>偏好交付</span>
          </div>
          <p className="mirror-page__profile-copy">{profile.preferred_output}</p>
        </article>

        <article className="mirror-page__profile-card">
          <div className="mirror-page__profile-heading">
            <Clock3 className="mirror-page__profile-icon" />
            <span>活跃时段</span>
          </div>
          <p className="mirror-page__hours-value">{profile.active_hours}</p>
        </article>
      </div>
    );
  };

  const renderDailyDetail = () => {
    if (!dailySummary) {
      return <p className="mirror-page__empty-state">暂无日报与阶段总结。</p>;
    }

    return (
      <div className="mirror-page__daily-stack mirror-page__daily-stack--expanded">
        <div className="mirror-page__date-card">
          <CalendarDays className="mirror-page__date-icon" />
          <div>
            <p className="mirror-page__micro-label">回看日期</p>
            <p className="mirror-page__date-value">{formatMirrorDate(dailySummary.date)}</p>
          </div>
        </div>

        <div className="mirror-page__summary-grid">
          <article className="mirror-page__summary-card">
            <p className="mirror-page__micro-label">当天完成事项</p>
            <p className="mirror-page__summary-value">{dailySummary.completed_tasks}</p>
            <p className="mirror-page__summary-copy">已完成任务数量</p>
          </article>
          <article className="mirror-page__summary-card mirror-page__summary-card--accent">
            <p className="mirror-page__micro-label">沉淀成果</p>
            <p className="mirror-page__summary-value">{dailySummary.generated_outputs}</p>
            <p className="mirror-page__summary-copy">已生成输出数量</p>
          </article>
        </div>

        <div className="mirror-page__detail-note-shell mirror-page__detail-note-shell--stage">
          <p className="mirror-page__micro-label">阶段总结</p>
          <p className="mirror-page__stage-headline">{insight.title}</p>
          <p className="mirror-page__note">{insight.description}</p>
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
              <StatusBadge tone="processing">引用记录</StatusBadge>
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

    if (activeDetailKey === "history") {
      return renderHistoryDetail();
    }

    if (activeDetailKey === "profile") {
      return renderProfileDetail();
    }

    if (activeDetailKey === "dailyStage") {
      return renderDailyDetail();
    }

    return renderMemoryDetail();
  };

  const getDetailBadge = (key: MirrorDirectionKey): DetailBadge => {
    if (key === "dailyStage") {
      return {
        label: dailySummary ? formatShortMirrorDate(dailySummary.date) : "暂无日报",
        tone: "processing",
      };
    }

    if (key === "profile") {
      return {
        label: profile ? "画像已整理" : "暂无画像",
        tone: "green",
      };
    }

    if (key === "history") {
      return {
        label: overview.history_summary.length ? `${overview.history_summary.length} 条概要` : "暂无概要",
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

    return (
      <div className="mirror-page__detail-layer" onClick={closeDetail}>
        <div className="mirror-page__detail-panel" role="dialog" aria-modal="true" aria-label={`${getDirectionTitle(activeDetailKey)}详情`} onClick={(event) => event.stopPropagation()}>
          <PanelSurface title={getDirectionTitle(activeDetailKey)} eyebrow={getDirectionEyebrow(activeDetailKey)}>
            <div className="mirror-page__detail-topbar">
              <div className="mirror-page__detail-meta">
                <StatusBadge tone={detailBadge.tone}>{detailBadge.label}</StatusBadge>
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

  const getCardSummary = (key: MirrorDirectionKey): CardSummary => {
    if (key === "dailyStage") {
      return {
        badge: dailySummary ? formatShortMirrorDate(dailySummary.date) : "待同步",
        tone: "processing",
        detailLine: dailySummary ? insight.description : "等待新的日报同步后生成阶段总结。",
        accent: getMirrorDirectionMeta(key).accent,
        mainLine: dailySummary ? insight.title : "暂无日报与阶段总结",
      };
    }

    if (key === "profile") {
      return {
        badge: profile ? "画像侧写" : "暂无画像",
        tone: "green",
        detailLine: profile
          ? `偏好 ${profile.preferred_output} · 活跃于 ${profile.active_hours}`
          : "等待新的用户画像同步。",
        accent: getMirrorDirectionMeta(key).accent,
        mainLine: profile?.work_style ?? "暂无用户画像",
      };
    }

    if (key === "history") {
      return {
        badge: overview.history_summary.length ? `${overview.history_summary.length} 条概要` : "暂无概要",
        tone: "processing",
        detailLine: overview.history_summary[1] ?? "轻触查看按片段整理的历史摘要。",
        accent: getMirrorDirectionMeta(key).accent,
        mainLine: overview.history_summary[0] ?? "暂无历史概要",
      };
    }

    return {
      badge: `${overview.memory_references.length} 条引用`,
      tone: "processing",
      detailLine: latestMemoryReference?.reason ?? "等待新的记忆调用记录。",
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
            setActiveDetailKey(key);
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
      <div className="mirror-page__canvas mirror-page__canvas--full" ref={canvasRef} aria-label="Mirror 检片台">
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
        {renderDetailOverlay()}
      </div>
    </main>
  );
}
