/**
 * Note dashboard page keeps nearby notes, future arrangements, and recurring
 * reminders grouped for quick conversion into formal tasks.
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { useUnmount } from "ahooks";
import type { CSSProperties, PointerEvent as ReactPointerEvent, UIEvent } from "react";
import { Link, NavLink, useNavigate } from "react-router-dom";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, ArrowLeft, FilePlus2, PanelLeftClose, PanelLeftOpen, RefreshCcw, ScanSearch, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import type { NotepadAction } from "@cialloclaw/protocol";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import { navigateToDashboardTaskDetail } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { buildNoteSummary, describeNotePreview, getNoteBucketLabel, getNoteStatusBadgeClass, groupClosedNotes, sortClosedNotes, sortNotesByUrgency } from "./notePage.mapper";
import { buildDashboardNoteBucketInvalidateKeys, buildDashboardNoteBucketQueryKey, dashboardNoteBucketGroups, getDashboardNoteRefreshPlan } from "./notePage.query";
import {
  areDesktopSourceNotesAvailable,
  createNoteSource,
  loadNoteSourceConfig,
  loadNoteSourceIndex,
  loadNoteSourceSnapshot,
  runNoteSourceInspection,
  saveNoteSource,
} from "./noteSource.service";
import { buildSourceNoteFallbackItems, convertNoteToTask, loadNoteBucket, partitionNoteItemsByBucket, performNoteResourceOpenExecution, resolveNoteResourceOpenExecutionPlan, updateNote, type NotePageDataMode } from "./notePage.service";
import {
  buildSourceNoteEditorDraftFromNote,
  createEmptySourceNoteEditorDraft,
  createSourceNoteEditorDraftSignature,
  parseSourceNoteEditorBlocks,
  serializeSourceNoteEditorDraft,
  upsertSourceNoteEditorBlock,
} from "./sourceNoteEditor";
import type { NoteDetailAction, NoteListItem, NotePreviewGroupKey, SourceNoteDocument, SourceNoteEditorDraft } from "./notePage.types";
import { NoteDetailPanel } from "./components/NoteDetailPanel";
import { NoteEmptyState } from "./components/NoteEmptyState";
import { NotePreviewCard } from "./components/NotePreviewCard";
import { NotePreviewSection } from "./components/NotePreviewSection";
import { SourceNoteStudio } from "./components/SourceNoteStudio";
import "./notePage.css";

type NoteCanvasCard = {
  itemId: string;
  x: number;
  y: number;
  zIndex: number;
};

type NoteDrawerDragPreview = {
  height: number;
  item: NoteListItem;
  width: number;
  x: number;
  y: number;
};

type PendingCreatedSourceNote = {
  path: string;
  sourceLine?: number | null;
  title: string;
};

type SourceNoteIdentity = {
  path: string;
  sourceLine?: number | null;
  title: string;
};

const NOTE_CANVAS_CARD_WIDTH = 360;
const NOTE_CANVAS_CARD_HEIGHT = 280;
const NOTE_CANVAS_GRID_SIZE = 28;
const SOURCE_NOTE_INDEX_POLL_INTERVAL_MS = 2_500;
const NOTE_CANVAS_SEED_POSITIONS = [
  { x: 56, y: 48 },
  { x: 448, y: 56 },
  { x: 840, y: 84 },
  { x: 280, y: 320 },
];

function snapCanvasCoordinate(value: number, max: number) {
  return Math.min(max, Math.max(0, Math.round(value / NOTE_CANVAS_GRID_SIZE) * NOTE_CANVAS_GRID_SIZE));
}

function clampCanvasPlacement(
  placement: { x: number; y: number },
  bounds: { height: number; width: number },
  cardSize = { height: NOTE_CANVAS_CARD_HEIGHT, width: NOTE_CANVAS_CARD_WIDTH },
) {
  const maxX = Math.max(0, bounds.width - cardSize.width);
  const maxY = Math.max(0, bounds.height - cardSize.height);
  const clampedX = Math.min(maxX, Math.max(0, placement.x));
  const clampedY = Math.min(maxY, Math.max(0, placement.y));

  return {
    x: snapCanvasCoordinate(clampedX, maxX),
    y: snapCanvasCoordinate(clampedY, maxY),
  };
}

function normalizeSourceNoteKey(value: string) {
  return value.trim().replace(/\\/g, "/").toLowerCase();
}

function normalizeSourceNoteTitleKey(value: string) {
  return value.trim().toLowerCase();
}

function resolveKnownSourceNotePath(
  rawPath: string,
  sourceNotesByPath: Map<string, SourceNoteDocument>,
) {
  const normalizedPath = normalizeSourceNoteKey(rawPath);
  const directMatch = sourceNotesByPath.get(normalizedPath);
  if (directMatch) {
    return directMatch.path;
  }

  const workspaceRelativePath = normalizedPath.startsWith("workspace/") ? normalizedPath.slice("workspace/".length) : null;
  if (!workspaceRelativePath) {
    return null;
  }

  // Inspector items can still point at workspace-relative source paths while
  // the desktop bridge exposes absolute file paths for the same note file.
  const suffix = `/${workspaceRelativePath}`;
  const matchedNotes = [...sourceNotesByPath.values()].filter((note) => {
    const normalizedNotePath = normalizeSourceNoteKey(note.path);
    return normalizedNotePath === workspaceRelativePath || normalizedNotePath.endsWith(suffix);
  });

  return matchedNotes.length === 1 ? matchedNotes[0]?.path ?? null : null;
}

function buildSourceNoteBlockKey(path: string, title: string, sourceLine?: number | null) {
  return typeof sourceLine === "number" && sourceLine > 0
    ? `${normalizeSourceNoteKey(path)}::line:${sourceLine}`
    : `${normalizeSourceNoteKey(path)}::title:${normalizeSourceNoteTitleKey(title)}`;
}

function readTodoSourcePath(item: NoteListItem["item"]) {
  const sourcePath = (item as NoteListItem["item"] & { source_path?: unknown }).source_path;
  return typeof sourcePath === "string" && sourcePath.trim() !== "" ? sourcePath : null;
}

function readTodoSourceLine(item: NoteListItem["item"]) {
  const sourceLine = (item as NoteListItem["item"] & { source_line?: unknown }).source_line;
  return typeof sourceLine === "number" && Number.isFinite(sourceLine) && sourceLine > 0 ? sourceLine : null;
}

function findReplacementItemIdForSourceNote(
  noteItemsById: Map<string, NoteListItem>,
  noteItemIdsBySourcePath: Map<string, string[]>,
  sourceIdentity: SourceNoteIdentity | PendingCreatedSourceNote | null,
) {
  if (!sourceIdentity) {
    return null;
  }

  const candidateIds = noteItemIdsBySourcePath.get(normalizeSourceNoteKey(sourceIdentity.path)) ?? [];
  if (candidateIds.length === 0) {
    return null;
  }

  if (typeof sourceIdentity.sourceLine === "number" && sourceIdentity.sourceLine > 0) {
    const exactLineCandidate = candidateIds.find((itemId) => {
      const item = noteItemsById.get(itemId);
      return item ? readTodoSourceLine(item.item) === sourceIdentity.sourceLine : false;
    });
    if (exactLineCandidate) {
      return exactLineCandidate;
    }
  }

  const normalizedTitle = normalizeSourceNoteTitleKey(sourceIdentity.title);
  const exactCandidate = candidateIds.find((itemId) => {
    const item = noteItemsById.get(itemId);
    return item ? normalizeSourceNoteTitleKey(item.item.title) === normalizedTitle : false;
  });
  if (exactCandidate) {
    return exactCandidate;
  }

  return candidateIds.length === 1 ? candidateIds[0] : null;
}

function resolveNoteItemSourceNotePath(
  item: NoteListItem,
  sourceNotesByPath: Map<string, SourceNoteDocument>,
  sourceNotesByTitle: Map<string, SourceNoteDocument>,
) {
  const sourcePath = readTodoSourcePath(item.item);
  if (sourcePath) {
    const matchedPath = resolveKnownSourceNotePath(sourcePath, sourceNotesByPath);
    if (matchedPath) {
      return matchedPath;
    }
  }

  if (item.sourceNote?.path) {
    const matchedPath = resolveKnownSourceNotePath(item.sourceNote.path, sourceNotesByPath);
    return matchedPath ?? item.sourceNote.path;
  }

  const resourceMatch = item.experience.relatedResources
    .map((resource) => resolveKnownSourceNotePath(resource.path, sourceNotesByPath))
    .filter((path): path is string => path !== null)
    .map((path) => sourceNotesByPath.get(normalizeSourceNoteKey(path)))
    .find((note) => note !== undefined);
  if (resourceMatch) {
    return resourceMatch.path;
  }

  return sourceNotesByTitle.get(item.item.title.trim().toLowerCase())?.path ?? null;
}

function matchesSourceNotePath(
  item: NoteListItem,
  sourceNotePath: string,
  sourceNotesByPath: Map<string, SourceNoteDocument>,
  sourceNotesByTitle: Map<string, SourceNoteDocument>,
) {
  const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNotesByTitle);
  return matchedPath !== null && normalizeSourceNoteKey(matchedPath) === normalizeSourceNoteKey(sourceNotePath);
}

/**
 * Renders the note dashboard page and coordinates note selection, feedback, and
 * lightweight conversion actions.
 *
 * @returns The note dashboard route content.
 */
export function NotePage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const boardLayerRef = useRef<HTMLDivElement | null>(null);
  const railRef = useRef<HTMLElement | null>(null);
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(true);
  const [expandedBucket, setExpandedBucket] = useState<NotePreviewGroupKey>("upcoming");
  const [showMoreClosed, setShowMoreClosed] = useState(false);
  const [canvasCards, setCanvasCards] = useState<NoteCanvasCard[]>([]);
  const [boardSeeded, setBoardSeeded] = useState(false);
  const [isBoardDropTarget, setIsBoardDropTarget] = useState(false);
  const [isRailDropTarget, setIsRailDropTarget] = useState(false);
  const [activeRailDropBucket, setActiveRailDropBucket] = useState<NotePreviewGroupKey | null>(null);
  const [isCompactBoard, setIsCompactBoard] = useState<boolean>(() => (typeof window !== "undefined" ? window.matchMedia("(max-width: 720px)").matches : false));
  const [drawerDragPreview, setDrawerDragPreview] = useState<NoteDrawerDragPreview | null>(null);
  const [draggingBoardItemId, setDraggingBoardItemId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [boardLayerSize, setBoardLayerSize] = useState<{ height: number; width: number } | null>(null);
  const [dataMode, setDataMode] = useState<NotePageDataMode>(() => loadDashboardDataMode("notes") as NotePageDataMode);
  const [selectedSourceNotePath, setSelectedSourceNotePath] = useState<string | null>(null);
  const [sourceNoteDraft, setSourceNoteDraft] = useState<SourceNoteEditorDraft>(() => createEmptySourceNoteEditorDraft());
  const [sourceNoteBaseline, setSourceNoteBaseline] = useState(() => createSourceNoteEditorDraftSignature(createEmptySourceNoteEditorDraft()));
  const [sourceNoteBaselineContent, setSourceNoteBaselineContent] = useState("");
  const [sourceEditorItemId, setSourceEditorItemId] = useState<string | null>(null);
  const [sourceNoteSyncMessage, setSourceNoteSyncMessage] = useState<string | null>(null);
  const [isCreatingSourceNote, setIsCreatingSourceNote] = useState(false);
  const [sourceStudioOpen, setSourceStudioOpen] = useState(false);
  const [isSavingSourceNote, setIsSavingSourceNote] = useState(false);
  const [isRunningInspection, setIsRunningInspection] = useState(false);
  const feedbackTimeoutRef = useRef<number | null>(null);
  const sourceNoteIndexFingerprintRef = useRef<string | null>(null);
  const pendingCreatedSourceNoteRef = useRef<PendingCreatedSourceNote | null>(null);
  const noteSourceIdentityByItemIdRef = useRef(new Map<string, SourceNoteIdentity>());
  const pinNoteToCanvasRef = useRef<(itemId: string, placement?: { x: number; y: number }) => void>(() => {});
  const skipNextSourceNoteRefreshRef = useRef(false);
  const dragStateRef = useRef<{
    itemId: string;
    pointerId: number;
    startClientX: number;
    startClientY: number;
    minX: number;
    maxX: number;
    minY: number;
    maxY: number;
    moved: boolean;
    originX: number;
    originY: number;
  } | null>(null);
  const suppressBoardClickItemIdRef = useRef<string | null>(null);
  const drawerDragStateRef = useRef<{
    height: number;
    item: NoteListItem;
    offsetX: number;
    offsetY: number;
    pointerId: number;
    started: boolean;
    startX: number;
    startY: number;
    width: number;
  } | null>(null);
  const noteRefreshPlan = useMemo(() => getDashboardNoteRefreshPlan(dataMode), [dataMode]);
  const desktopSourceNotesAvailable = useMemo(() => areDesktopSourceNotesAvailable(), []);

  useEffect(() => {
    saveDashboardDataMode("notes", dataMode);
  }, [dataMode]);

  const [upcomingQuery, laterQuery, recurringQuery, closedQuery] = useQueries({
    queries: [
        {
          queryKey: buildDashboardNoteBucketQueryKey(dataMode, dashboardNoteBucketGroups[0]),
          queryFn: () => loadNoteBucket("upcoming", dataMode),
        retry: false,
        refetchOnMount: noteRefreshPlan.refetchOnMount,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
        {
          queryKey: buildDashboardNoteBucketQueryKey(dataMode, dashboardNoteBucketGroups[1]),
          queryFn: () => loadNoteBucket("later", dataMode),
        retry: false,
        refetchOnMount: noteRefreshPlan.refetchOnMount,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
        {
          queryKey: buildDashboardNoteBucketQueryKey(dataMode, dashboardNoteBucketGroups[2]),
          queryFn: () => loadNoteBucket("recurring_rule", dataMode),
        retry: false,
        refetchOnMount: noteRefreshPlan.refetchOnMount,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
        {
          queryKey: buildDashboardNoteBucketQueryKey(dataMode, dashboardNoteBucketGroups[3]),
          queryFn: () => loadNoteBucket("closed", dataMode),
        retry: false,
        refetchOnMount: noteRefreshPlan.refetchOnMount,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
    ],
  });

  const sourceConfigQuery = useQuery({
    enabled: dataMode === "rpc",
    queryFn: loadNoteSourceConfig,
    queryKey: ["note-source-config", dataMode],
    refetchOnMount: noteRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const configuredTaskSourceRoots = sourceConfigQuery.data?.task_sources;
  const taskSourceRoots = useMemo(() => configuredTaskSourceRoots ?? [], [configuredTaskSourceRoots]);
  const sourceNotesBridgeEnabled = dataMode === "rpc" && desktopSourceNotesAvailable && taskSourceRoots.length > 0;
  const sourceNotesQuery = useQuery({
    enabled: sourceNotesBridgeEnabled,
    queryFn: () => loadNoteSourceSnapshot(taskSourceRoots),
    queryKey: ["note-source-snapshot", dataMode, taskSourceRoots],
    refetchOnMount: noteRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });
  const sourceNoteIndexQuery = useQuery({
    enabled: sourceNotesBridgeEnabled,
    queryFn: () => loadNoteSourceIndex(taskSourceRoots),
    queryKey: ["note-source-index", dataMode, taskSourceRoots],
    refetchInterval: sourceNotesBridgeEnabled ? SOURCE_NOTE_INDEX_POLL_INTERVAL_MS : false,
    refetchOnMount: noteRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const rpcUpcomingItems = sortNotesByUrgency(upcomingQuery.data?.items ?? []);
  const rpcLaterItems = sortNotesByUrgency(laterQuery.data?.items ?? []);
  const rpcRecurringItems = sortNotesByUrgency(recurringQuery.data?.items ?? []);
  const rpcClosedItems = sortClosedNotes(closedQuery.data?.items ?? []);
  const sourceNotesData = sourceNotesQuery.data?.notes;
  const sourceNoteIndexData = sourceNoteIndexQuery.data?.notes;
  const sourceRootsData = sourceNotesQuery.data?.sourceRoots;
  const sourceNotes = useMemo(() => sourceNotesData ?? [], [sourceNotesData]);
  const resolvedSourceRoots = useMemo(() => sourceRootsData ?? taskSourceRoots, [sourceRootsData, taskSourceRoots]);
  const sourceNotesByPath = useMemo(
    () => new Map(sourceNotes.map((note) => [normalizeSourceNoteKey(note.path), note])),
    [sourceNotes],
  );
  const sourceNotesByTitle = useMemo(
    () => new Map(sourceNotes.map((note) => [note.title.trim().toLowerCase(), note])),
    [sourceNotes],
  );
  const representedSourceNoteBlocks = useMemo(() => {
    const representedBlocks = new Set<string>();

    [...rpcUpcomingItems, ...rpcLaterItems, ...rpcRecurringItems, ...rpcClosedItems].forEach((item) => {
      const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNotesByTitle);
      if (!matchedPath) {
        return;
      }

      representedBlocks.add(
        buildSourceNoteBlockKey(
          matchedPath,
          item.item.title,
          readTodoSourceLine(item.item),
        ),
      );
    });

    return representedBlocks;
  }, [rpcClosedItems, rpcLaterItems, rpcRecurringItems, rpcUpcomingItems, sourceNotesByPath, sourceNotesByTitle]);
  const sourceNoteFallbackItems = useMemo(
    () =>
      sourceNotes
        .sort((left, right) => (right.modifiedAtMs ?? 0) - (left.modifiedAtMs ?? 0))
        .flatMap((note) => buildSourceNoteFallbackItems(note))
        .filter((item) => {
          const path = item.sourceNote?.path ?? readTodoSourcePath(item.item);
          if (!path) {
            return true;
          }

          return !representedSourceNoteBlocks.has(
            buildSourceNoteBlockKey(
              path,
              item.sourceNote?.title ?? item.item.title,
              item.sourceNote?.sourceLine ?? readTodoSourceLine(item.item),
            ),
          );
        }),
    [representedSourceNoteBlocks, sourceNotes],
  );
  const sourceNoteFallbackBuckets = useMemo(() => partitionNoteItemsByBucket(sourceNoteFallbackItems), [sourceNoteFallbackItems]);
  const upcomingItems = useMemo(() => sortNotesByUrgency([...sourceNoteFallbackBuckets.upcoming, ...rpcUpcomingItems]), [rpcUpcomingItems, sourceNoteFallbackBuckets]);
  const laterItems = useMemo(() => sortNotesByUrgency([...sourceNoteFallbackBuckets.later, ...rpcLaterItems]), [rpcLaterItems, sourceNoteFallbackBuckets]);
  const recurringItems = useMemo(() => sortNotesByUrgency([...sourceNoteFallbackBuckets.recurring_rule, ...rpcRecurringItems]), [rpcRecurringItems, sourceNoteFallbackBuckets]);
  const closedItems = useMemo(() => sortClosedNotes([...sourceNoteFallbackBuckets.closed, ...rpcClosedItems]), [rpcClosedItems, sourceNoteFallbackBuckets]);
  const canvasItemIdSet = useMemo(() => new Set(canvasCards.map((entry) => entry.itemId)), [canvasCards]);
  const visibleUpcomingItems = useMemo(() => upcomingItems.filter((item) => !canvasItemIdSet.has(item.item.item_id)), [canvasItemIdSet, upcomingItems]);
  const visibleLaterItems = useMemo(() => laterItems.filter((item) => !canvasItemIdSet.has(item.item.item_id)), [canvasItemIdSet, laterItems]);
  const visibleRecurringItems = useMemo(() => recurringItems.filter((item) => !canvasItemIdSet.has(item.item.item_id)), [canvasItemIdSet, recurringItems]);
  const visibleClosedItems = useMemo(() => closedItems.filter((item) => !canvasItemIdSet.has(item.item.item_id)), [canvasItemIdSet, closedItems]);
  const closedGroups = useMemo(() => groupClosedNotes(visibleClosedItems, showMoreClosed), [showMoreClosed, visibleClosedItems]);
  const hasOlderClosedItems = useMemo(() => {
    const now = Date.now();

    return visibleClosedItems.some((item) => {
      const endedAt = item.experience.endedAt ? new Date(item.experience.endedAt).getTime() : now;
      const diffDays = (now - endedAt) / (1000 * 60 * 60 * 24);
      return diffDays > 7;
    });
  }, [visibleClosedItems]);
  const summary = useMemo(() => buildNoteSummary({ recurring_rule: recurringItems, upcoming: upcomingItems }), [recurringItems, upcomingItems]);
  const allItems = useMemo(() => [...upcomingItems, ...laterItems, ...recurringItems, ...closedItems], [upcomingItems, laterItems, recurringItems, closedItems]);
  const noteItemsById = useMemo(() => new Map(allItems.map((item) => [item.item.item_id, item])), [allItems]);
  const noteItemSourcePathById = useMemo(() => {
    const itemPathMap = new Map<string, string>();

    allItems.forEach((item) => {
      const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNotesByTitle);
      if (matchedPath) {
        itemPathMap.set(item.item.item_id, matchedPath);
      }
    });

    return itemPathMap;
  }, [allItems, sourceNotesByPath, sourceNotesByTitle]);
  const noteItemIdsBySourcePath = useMemo(() => {
    const pathItemMap = new Map<string, string[]>();

    noteItemSourcePathById.forEach((path, itemId) => {
      const normalizedPath = normalizeSourceNoteKey(path);
      const nextItemIds = pathItemMap.get(normalizedPath) ?? [];
      nextItemIds.push(itemId);
      pathItemMap.set(normalizedPath, nextItemIds);
    });

    return pathItemMap;
  }, [noteItemSourcePathById]);
  const selectedItem = useMemo(
    () => allItems.find((entry) => entry.item.item_id === selectedItemId) ?? upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0] ?? null,
    [allItems, closedItems, laterItems, recurringItems, selectedItemId, upcomingItems],
  );
  const sourceStudioItem = useMemo(
    () => (sourceEditorItemId ? noteItemsById.get(sourceEditorItemId) ?? null : null),
    [noteItemsById, sourceEditorItemId],
  );
  const primarySourceNote = useMemo(() => sourceNotes[0] ?? null, [sourceNotes]);
  const draftSourcePathHint = primarySourceNote?.path ?? resolvedSourceRoots[0] ?? null;
  const selectedSourceNote = useMemo(
    () => sourceNotes.find((note) => note.path === selectedSourceNotePath) ?? primarySourceNote,
    [primarySourceNote, selectedSourceNotePath, sourceNotes],
  );
  const sourceStudioFileLabel = selectedSourceNote?.fileName ?? primarySourceNote?.fileName ?? null;
  const sourceEditorDirty = createSourceNoteEditorDraftSignature(sourceNoteDraft) !== sourceNoteBaseline;
  const sourceNoteIndexFingerprint = useMemo(
    () => (sourceNoteIndexData ?? []).map((note) => `${note.path}:${note.modifiedAtMs ?? 0}:${note.sizeBytes}`).join("|"),
    [sourceNoteIndexData],
  );
  const sourceNoteAvailabilityMessage = useMemo(() => {
    if (dataMode !== "rpc") {
      return "Mock 模式下不会读写真实任务来源。";
    }

    if (!desktopSourceNotesAvailable) {
      return "当前运行环境不支持桌面端便签来源桥接。";
    }

    if (sourceConfigQuery.error) {
      return sourceConfigQuery.error instanceof Error ? sourceConfigQuery.error.message : "任务来源配置读取失败。";
    }

    if (sourceConfigQuery.isPending) {
      return "正在读取任务来源配置…";
    }

    if (taskSourceRoots.length === 0) {
      return "请先在设置面板的任务来源列表里配置至少一个目录。";
    }

    return null;
  }, [dataMode, desktopSourceNotesAvailable, sourceConfigQuery.error, sourceConfigQuery.isPending, taskSourceRoots.length]);
  const sourceNotesLoading = sourceConfigQuery.isFetching || sourceNotesQuery.isFetching;

  const pageStyle = {
    "--note-accent": "var(--cc-module-notes)",
    "--note-accent-strong": "var(--cc-module-notes-strong)",
    "--note-paper": "var(--cc-paper)",
    "--note-paper-strong": "var(--cc-paper-strong)",
    "--note-line": "var(--cc-line)",
    "--note-ink": "var(--cc-ink)",
    "--note-copy": "var(--cc-ink-muted)",
  } as CSSProperties;

  /**
   * Shows short-lived feedback after placeholder actions and conversion flows.
   *
   * @param message User-facing feedback copy to render in the page chrome.
   */
  function showFeedback(message: string) {
    setFeedback(message);
    if (feedbackTimeoutRef.current) {
      window.clearTimeout(feedbackTimeoutRef.current);
    }
    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2600);
  }

  function buildInspectionSummary(parsedFiles: number, identifiedItems: number, overdue: number, prefix?: string) {
    const summaryCopy = `本次巡检解析 ${parsedFiles} 个文件，识别 ${identifiedItems} 条事项，逾期 ${overdue} 条。`;
    return prefix ? `${prefix}。${summaryCopy}` : summaryCopy;
  }

  async function invalidateAllNoteBuckets() {
    await Promise.all(
      dashboardNoteBucketGroups.map((group) =>
        queryClient.invalidateQueries({
          queryKey: buildDashboardNoteBucketQueryKey(dataMode, group),
        }),
      ),
    );
  }

  async function invalidateNoteBuckets(groups: readonly NotePreviewGroupKey[]) {
    await Promise.all(
      buildDashboardNoteBucketInvalidateKeys(dataMode, groups).map((queryKey) =>
        queryClient.invalidateQueries({
          queryKey,
        }),
      ),
    );
  }

  async function refetchAllNoteBuckets() {
    const [upcomingResult, laterResult, recurringResult, closedResult] = await Promise.all([
      upcomingQuery.refetch(),
      laterQuery.refetch(),
      recurringQuery.refetch(),
      closedQuery.refetch(),
    ]);

    return [
      ...(upcomingResult.data?.items ?? []),
      ...(laterResult.data?.items ?? []),
      ...(recurringResult.data?.items ?? []),
      ...(closedResult.data?.items ?? []),
    ];
  }

  async function triggerAutoTaskExecutionForSourceNote(
    savedNote: SourceNoteDocument,
    sourceIdentity: PendingCreatedSourceNote,
  ) {
    const latestSourceNotesResult = await sourceNotesQuery.refetch();
    const latestSourceNotes = latestSourceNotesResult.data?.notes ?? sourceNotes;
    const latestSourceNotesByPath = new Map(latestSourceNotes.map((note) => [normalizeSourceNoteKey(note.path), note]));
    const latestSourceNotesByTitle = new Map(latestSourceNotes.map((note) => [note.title.trim().toLowerCase(), note]));
    const normalizedExpectedPath = normalizeSourceNoteKey(sourceIdentity.path);
    const normalizedExpectedTitle = normalizeSourceNoteTitleKey(sourceIdentity.title);
    const refetchedItems = await refetchAllNoteBuckets();
    const matchedItem = refetchedItems.find((item) => {
      if (item.sourceNote?.localOnly) {
        return false;
      }

      const matchedPath = resolveNoteItemSourceNotePath(item, latestSourceNotesByPath, latestSourceNotesByTitle);
      if (!matchedPath || normalizeSourceNoteKey(matchedPath) !== normalizedExpectedPath) {
        return false;
      }

      if (typeof sourceIdentity.sourceLine === "number" && sourceIdentity.sourceLine > 0) {
        return readTodoSourceLine(item.item) === sourceIdentity.sourceLine;
      }

      return normalizeSourceNoteTitleKey(item.item.title) === normalizedExpectedTitle;
    })
      ?? refetchedItems.find(
        (item) =>
          !item.sourceNote?.localOnly
          && normalizeSourceNoteTitleKey(item.item.title) === normalizedExpectedTitle
          && matchesSourceNotePath(item, savedNote.path, latestSourceNotesByPath, latestSourceNotesByTitle),
      );

    if (!matchedItem) {
      showFeedback("新便签已保存并完成巡检，但暂时还没有识别成可执行事项。");
      return;
    }

    try {
      const outcome = await convertNoteToTask(matchedItem.item.item_id, dataMode);
      await invalidateNoteBuckets(outcome.result.refresh_groups);
      await refetchAllNoteBuckets();
      setSelectedItemId(matchedItem.item.item_id);
      showFeedback("新便签已保存，并已触发后端开始执行。");
    } catch (error) {
      const message = error instanceof Error ? error.message : "触发后端执行失败，请稍后重试。";
      showFeedback(`新便签已保存，但触发后端执行失败：${message}`);
    }
  }

  async function refreshInspection(reason: string, prefix?: string) {
    if (dataMode !== "rpc") {
      showFeedback("Mock 模式下不会执行真实巡检。");
      return;
    }

    if (taskSourceRoots.length === 0) {
      const message = "请先在设置面板里配置任务来源目录。";
      setSourceNoteSyncMessage(message);
      showFeedback(message);
      return;
    }

    if (isRunningInspection) {
      return;
    }

    setIsRunningInspection(true);
    try {
      const result = await runNoteSourceInspection(taskSourceRoots, reason);
      await invalidateAllNoteBuckets();
      const message = buildInspectionSummary(
        result.summary.parsed_files,
        result.summary.identified_items,
        result.summary.overdue,
        prefix,
      );
      setSourceNoteSyncMessage(message);
      showFeedback(message);
    } catch (error) {
      const message = error instanceof Error ? error.message : "便签巡检失败。";
      setSourceNoteSyncMessage(message);
      showFeedback(message);
    } finally {
      setIsRunningInspection(false);
    }
  }

  function applySourceNoteDraft(nextDraft: SourceNoteEditorDraft, fileContent: string) {
    setSourceNoteDraft(nextDraft);
    setSourceNoteBaseline(createSourceNoteEditorDraftSignature(nextDraft));
    setSourceNoteBaselineContent(fileContent);
  }

  function startCreatingSourceNote() {
    const nextDraft = createEmptySourceNoteEditorDraft(draftSourcePathHint);

    setIsCreatingSourceNote(true);
    setSourceEditorItemId(null);
    setSelectedSourceNotePath(primarySourceNote?.path ?? null);
    applySourceNoteDraft(nextDraft, primarySourceNote?.content ?? "");
    setSourceNoteSyncMessage(
      primarySourceNote?.path
        ? `新便签会追加到 ${primarySourceNote.path}`
        : resolvedSourceRoots[0]
          ? `新便签会追加到 ${resolvedSourceRoots[0]} 下的主便签来源`
          : "新便签会追加到第一个任务来源目录下的主便签来源。",
    );
  }

  function openCreateSourceNoteStudio() {
    startCreatingSourceNote();
    setDetailOpen(false);
    setSourceStudioOpen(true);
  }

  function resolveSourceNotePathForItem(item: NoteListItem) {
    return resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNotesByTitle);
  }

  function openSourceStudioForItem(item: NoteListItem) {
    const matchedPath = resolveSourceNotePathForItem(item) ?? primarySourceNote?.path ?? null;
    const matchedNote = matchedPath
      ? sourceNotesByPath.get(normalizeSourceNoteKey(matchedPath)) ?? primarySourceNote
      : primarySourceNote;

    if (matchedNote) {
      setIsCreatingSourceNote(false);
      setSourceEditorItemId(item.item.item_id);
      setSelectedSourceNotePath(matchedNote.path);
      applySourceNoteDraft(buildSourceNoteEditorDraftFromNote(matchedNote, item), matchedNote.content);
      setSourceNoteSyncMessage("正在编辑当前便签内容，保存后会自动同步巡检。");
      setDetailOpen(false);
      setSourceStudioOpen(true);
      if (sourceNoteAvailabilityMessage) {
        showFeedback(sourceNoteAvailabilityMessage);
      }
      return;
    }

    openCreateSourceNoteStudio();
    showFeedback(sourceNoteAvailabilityMessage ?? "还没有主便签来源，先为你打开空白便签。");
  }

  async function handleSaveSourceNote() {
    if (sourceNoteAvailabilityMessage !== null || taskSourceRoots.length === 0) {
      const message = sourceNoteAvailabilityMessage ?? "请先配置任务来源目录。";
      setSourceNoteSyncMessage(message);
      showFeedback(message);
      return;
    }

    if (isSavingSourceNote || (sourceNoteDraft.title.trim() === "" && sourceNoteDraft.noteText.trim() === "")) {
      return;
    }

    setIsSavingSourceNote(true);
    try {
      const createdSourceNote = isCreatingSourceNote;
      const sourceNoteTarget = selectedSourceNote ?? primarySourceNote;
      const { blockContent, normalizedDraft } = serializeSourceNoteEditorDraft(sourceNoteDraft);
      let savedNote: SourceNoteDocument;
      let resolvedSourceLine: number | null = null;
      let createdSourceNoteIdentity: PendingCreatedSourceNote | null = null;

      if (createdSourceNote || !sourceNoteTarget) {
        savedNote = await createNoteSource(taskSourceRoots, blockContent);
        const createdBlock = parseSourceNoteEditorBlocks(savedNote)
          .filter((block) => normalizeSourceNoteTitleKey(block.title) === normalizeSourceNoteTitleKey(normalizedDraft.title))
          .reverse()
          .find((block) => block.updatedAt === normalizedDraft.updatedAt)
          ?? null;
        resolvedSourceLine = createdBlock?.sourceLine ?? null;
        createdSourceNoteIdentity = {
          path: savedNote.path,
          sourceLine: resolvedSourceLine,
          title: normalizedDraft.title,
        };
      } else {
        const nextSourceFile = upsertSourceNoteEditorBlock(sourceNoteTarget, sourceNoteDraft);
        resolvedSourceLine = nextSourceFile.sourceLine;
        savedNote = await saveNoteSource(taskSourceRoots, sourceNoteTarget.path, nextSourceFile.content);
      }

      pendingCreatedSourceNoteRef.current = createdSourceNoteIdentity;
      skipNextSourceNoteRefreshRef.current = true;
      await Promise.all([sourceNotesQuery.refetch(), sourceNoteIndexQuery.refetch()]);
      setIsCreatingSourceNote(false);
      setSelectedSourceNotePath(savedNote.path);
      applySourceNoteDraft(
        {
          ...normalizedDraft,
          sourceLine: resolvedSourceLine,
          sourcePath: savedNote.path,
        },
        savedNote.content,
      );
      setSourceNoteSyncMessage(`${savedNote.fileName} 已保存，正在同步巡检结果。`);
      await refreshInspection(
        createdSourceNote ? "notes_markdown_created" : "notes_markdown_saved",
        createdSourceNote ? `已创建 ${savedNote.fileName}` : `已保存 ${savedNote.fileName}`,
      );
      if (createdSourceNote && createdSourceNoteIdentity) {
        await triggerAutoTaskExecutionForSourceNote(savedNote, createdSourceNoteIdentity);
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "便签保存失败。";
      setSourceNoteSyncMessage(message);
      showFeedback(message);
    } finally {
      setIsSavingSourceNote(false);
    }
  }

  const refreshInspectionRef = useRef(refreshInspection);
  refreshInspectionRef.current = refreshInspection;
  const sourceNotesRefetchRef = useRef(sourceNotesQuery.refetch);
  sourceNotesRefetchRef.current = sourceNotesQuery.refetch;

  function getNextCanvasZIndex(cards: NoteCanvasCard[]) {
    return cards.reduce((max, entry) => Math.max(max, entry.zIndex), 0) + 1;
  }

  /**
   * The note board cards are positioned relative to the dedicated board layer,
   * not the outer board shell that also contains the heading. All drag math and
   * seed placement must use this layer to avoid cursor offsets and clipping.
   */
  function getBoardLayerBounds() {
    const layer = boardLayerRef.current;
    if (!layer) {
      return null;
    }

    const rect = layer.getBoundingClientRect();
    return {
      height: rect.height,
      left: rect.left,
      right: rect.right,
      top: rect.top,
      bottom: rect.bottom,
      width: rect.width,
    };
  }

  const convertMutation = useMutation({
    mutationFn: (itemId: string) => convertNoteToTask(itemId, dataMode),
    onSuccess: async (outcome) => {
      await invalidateNoteBuckets(outcome.result.refresh_groups);
      showFeedback("已为这条事项生成任务，正在跳转到任务页。");
      navigateToDashboardTaskDetail(navigate, outcome.result.task.task_id);
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : "转交给 Agent 失败，请稍后再试。";
      showFeedback(`转交给 Agent 失败：${message}`);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ action, itemId }: { action: NotepadAction; itemId: string }) => updateNote(itemId, action, dataMode),
    onSuccess: async (outcome, variables) => {
      await invalidateNoteBuckets(outcome.result.refresh_groups);

      const feedbackByAction: Record<NotepadAction, string> = {
        cancel: "已取消这条事项。",
        cancel_recurring: "已取消整个重复规则。",
        complete: "已将事项标记为完成。",
        delete: "已删除这条记录。",
        move_upcoming: "已提前到近期要做。",
        restore: "已恢复为未完成事项。",
        toggle_recurring:
          outcome.result.notepad_item?.recurring_enabled === false ? "已暂停重复规则。" : "已重新开启重复规则。",
      };

      showFeedback(feedbackByAction[variables.action]);
      if (!outcome.result.notepad_item && outcome.result.deleted_item_id === selectedItem?.item.item_id) {
        setDetailOpen(false);
      }
    },
    onError: (error, variables) => {
      const message = error instanceof Error ? error.message : "事项更新失败，请稍后再试。";
      showFeedback(`事项更新失败（${variables.action}）：${message}`);
    },
  });

  function mapActionToMutation(action: NoteDetailAction): NotepadAction | null {
    switch (action) {
      case "complete":
        return "complete";
      case "cancel":
        return "cancel";
      case "move-upcoming":
        return "move_upcoming";
      case "toggle-recurring":
        return "toggle_recurring";
      case "cancel-recurring":
        return "cancel_recurring";
      case "restore":
        return "restore";
      case "delete":
        return "delete";
      default:
        return null;
    }
  }

  function handleDetailAction(action: NoteDetailAction) {
    if (!selectedItem) {
      return;
    }

    if (selectedItem.sourceNote?.localOnly && action !== "edit" && action !== "open-resource") {
      showFeedback("这张源便签还没进入正式事项流，先编辑源文件或等待巡检同步。");
      return;
    }

    if (action === "convert-to-task") {
      convertMutation.mutate(selectedItem.item.item_id);
      return;
    }

    if (action === "open-resource") {
      const firstResource = selectedItem.experience.relatedResources[0];
      if (!firstResource) {
        showFeedback("当前没有可打开的相关资料。");
        return;
      }
      void handleResourceOpen(firstResource.id);
      return;
    }

    if (action === "edit") {
      openSourceStudioForItem(selectedItem);
      return;
    }

    const mutationAction = mapActionToMutation(action);
    if (mutationAction) {
      updateMutation.mutate({
        action: mutationAction,
        itemId: selectedItem.item.item_id,
      });
      return;
    }

    showFeedback("跳过本次真实动作，后续再接入。");
  }

  async function handleResourceOpen(resourceId: string) {
    if (!selectedItem) {
      return;
    }

    const resource = selectedItem.experience.relatedResources.find((item) => item.id === resourceId);
    if (!resource) {
      showFeedback("未找到对应的相关资料。");
      return;
    }

    const plan = resolveNoteResourceOpenExecutionPlan(resource);
    showFeedback(await performNoteResourceOpenExecution(plan, {
      onOpenTaskDetail: ({ taskId }) => {
        navigateToDashboardTaskDetail(navigate, taskId);
        return plan.feedback;
      },
    }));
  }

  useEffect(() => {
    sourceNoteIndexFingerprintRef.current = null;
  }, [dataMode, taskSourceRoots]);

  useEffect(() => {
    noteItemSourcePathById.forEach((path, itemId) => {
      const noteItem = noteItemsById.get(itemId);
      noteSourceIdentityByItemIdRef.current.set(itemId, {
        path,
        sourceLine: noteItem ? readTodoSourceLine(noteItem.item) : null,
        title: noteItem?.item.title ?? "",
      });
    });
  }, [noteItemSourcePathById, noteItemsById]);

  useEffect(() => {
    const nextSourceNote = selectedSourceNotePath
      ? sourceNotes.find((note) => note.path === selectedSourceNotePath) ?? primarySourceNote
      : primarySourceNote;

    if (!nextSourceNote) {
      setSelectedSourceNotePath(null);
      if (!sourceEditorDirty) {
        const emptyDraft = createEmptySourceNoteEditorDraft(draftSourcePathHint);
        const emptySignature = createSourceNoteEditorDraftSignature(emptyDraft);
        if (sourceNoteBaseline !== emptySignature || sourceNoteBaselineContent !== "") {
          applySourceNoteDraft(emptyDraft, "");
        }
        setSourceNoteSyncMessage(null);
      }
      return;
    }

    if (selectedSourceNotePath !== nextSourceNote.path) {
      setSelectedSourceNotePath(nextSourceNote.path);
    }

    if (sourceEditorDirty) {
      return;
    }

    const nextDraft = isCreatingSourceNote
      ? createEmptySourceNoteEditorDraft(nextSourceNote.path)
      : sourceStudioItem
        ? buildSourceNoteEditorDraftFromNote(nextSourceNote, sourceStudioItem)
        : createEmptySourceNoteEditorDraft(nextSourceNote.path);
    const nextSignature = createSourceNoteEditorDraftSignature(nextDraft);

    if (sourceNoteBaseline !== nextSignature || sourceNoteBaselineContent !== nextSourceNote.content) {
      applySourceNoteDraft(nextDraft, nextSourceNote.content);
    }

    setSourceNoteSyncMessage(null);
  }, [
    draftSourcePathHint,
    isCreatingSourceNote,
    primarySourceNote,
    selectedSourceNotePath,
    sourceEditorDirty,
    sourceNoteBaseline,
    sourceNoteBaselineContent,
    sourceNotes,
    sourceStudioItem,
  ]);

  useEffect(() => {
    const currentSourceNote = selectedSourceNote ?? primarySourceNote;
    if (!currentSourceNote || !sourceEditorDirty) {
      return;
    }

    if (currentSourceNote.content !== sourceNoteBaselineContent) {
      setSourceNoteSyncMessage("检测到源文件已在外部变更。当前编辑器保留未保存内容，请确认后再保存。");
    }
  }, [primarySourceNote, selectedSourceNote, sourceEditorDirty, sourceNoteBaselineContent]);

  useEffect(() => {
    if (!sourceNoteIndexQuery.data || dataMode !== "rpc") {
      return;
    }

    if (sourceNoteIndexFingerprintRef.current === null) {
      sourceNoteIndexFingerprintRef.current = sourceNoteIndexFingerprint;
      return;
    }

    if (sourceNoteIndexFingerprint === sourceNoteIndexFingerprintRef.current) {
      return;
    }

    sourceNoteIndexFingerprintRef.current = sourceNoteIndexFingerprint;
    if (skipNextSourceNoteRefreshRef.current) {
      skipNextSourceNoteRefreshRef.current = false;
      return;
    }

    setSourceNoteSyncMessage("检测到任务来源发生变化，正在同步巡检结果。");
    void (async () => {
      try {
        await sourceNotesRefetchRef.current();
      } catch {
        // The full snapshot fetch already surfaces its own query error state.
      }
      await refreshInspectionRef.current("notes_source_polled_change", "检测到任务来源文件变更");
    })();
  }, [dataMode, sourceNoteIndexFingerprint, sourceNoteIndexQuery.data]);

  useEffect(() => {
    if (allItems.length === 0) {
      return;
    }

    const selectedExists = selectedItemId ? allItems.some((entry) => entry.item.item_id === selectedItemId) : false;
    if (selectedExists) {
      return;
    }

    if (selectedItemId) {
      const selectedSourceIdentity = noteSourceIdentityByItemIdRef.current.get(selectedItemId) ?? null;
      const replacementItemId = findReplacementItemIdForSourceNote(
        noteItemsById,
        noteItemIdsBySourcePath,
        selectedSourceIdentity,
      );
      if (replacementItemId) {
        setSelectedItemId(replacementItemId);
        return;
      }
    }

    const nextItem = upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0];
    if (nextItem) {
      setSelectedItemId(nextItem.item.item_id);
    }
  }, [allItems, closedItems, laterItems, noteItemIdsBySourcePath, noteItemsById, recurringItems, selectedItemId, upcomingItems]);

  useUnmount(() => {
    if (feedbackTimeoutRef.current) {
      window.clearTimeout(feedbackTimeoutRef.current);
    }
  });

  const queryErrors = [
    { label: "近期要做", error: upcomingQuery.error },
    { label: "后续安排", error: laterQuery.error },
    { label: "重复事项", error: recurringQuery.error },
    { label: "已结束", error: closedQuery.error },
    { label: "任务来源配置", error: sourceConfigQuery.error },
    { label: "便签来源", error: sourceNotesQuery.error },
    { label: "便签来源索引", error: sourceNoteIndexQuery.error },
  ].filter((item) => item.error);

  const pageNotice =
    selectedItem
      ? `${selectedItem.item.title} · ${describeNotePreview(selectedItem.item, selectedItem.experience)}`
      : "便签协作会把近期要做、后续安排、重复事项和已结束事项整理在这里。";

  const defaultBoardItemIds = useMemo(() => {
    const picked: NoteListItem[] = [];
    const seen = new Set<string>();

    function append(item: NoteListItem | null | undefined) {
      if (!item || seen.has(item.item.item_id)) {
        return;
      }

      seen.add(item.item.item_id);
      picked.push(item);
    }

    append(selectedItem);
    append(upcomingItems[0]);
    append(laterItems[0]);
    append(recurringItems[0]);
    append(closedItems[0]);

    return picked.slice(0, NOTE_CANVAS_SEED_POSITIONS.length).map((item) => item.item.item_id);
  }, [closedItems, laterItems, recurringItems, selectedItem, upcomingItems]);

  const boardItems = useMemo(
    () =>
      canvasCards
        .map((entry) => {
          const item = noteItemsById.get(entry.itemId);
          return item ? { item, x: entry.x, y: entry.y, zIndex: entry.zIndex } : null;
        })
        .filter((entry): entry is { item: NoteListItem; x: number; y: number; zIndex: number } => entry !== null)
        .sort((left, right) => left.zIndex - right.zIndex),
    [canvasCards, noteItemsById],
  );

  useEffect(() => {
    const layer = boardLayerRef.current;
    if (!layer) {
      return;
    }

    const updateBoardLayerSize = () => {
      const { height, width } = layer.getBoundingClientRect();
      setBoardLayerSize((current) => (current && current.height === height && current.width === width ? current : { height, width }));
    };

    updateBoardLayerSize();

    const resizeObserver = typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => updateBoardLayerSize()) : null;
    resizeObserver?.observe(layer);
    window.addEventListener("resize", updateBoardLayerSize);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateBoardLayerSize);
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const mediaQuery = window.matchMedia("(max-width: 720px)");
    const updateCompactBoard = () => setIsCompactBoard(mediaQuery.matches);
    updateCompactBoard();

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", updateCompactBoard);
      return () => mediaQuery.removeEventListener("change", updateCompactBoard);
    }

    mediaQuery.addListener(updateCompactBoard);
    return () => mediaQuery.removeListener(updateCompactBoard);
  }, []);

  useEffect(() => {
    if (boardSeeded || defaultBoardItemIds.length === 0 || !boardLayerSize) {
      return;
    }

    setCanvasCards(
      defaultBoardItemIds.map((itemId, index) => ({
        itemId,
        ...clampCanvasPlacement(
          {
            x: NOTE_CANVAS_SEED_POSITIONS[index]?.x ?? 120 + index * 36,
            y: NOTE_CANVAS_SEED_POSITIONS[index]?.y ?? 120 + index * 28,
          },
          boardLayerSize,
        ),
        zIndex: index + 1,
      })),
    );
    setBoardSeeded(true);
  }, [boardLayerSize, boardSeeded, defaultBoardItemIds]);

  useEffect(() => {
    // Keep the canvas purely local to this page. Once a card is placed, detail
    // toggles and bucket changes must not reshuffle the board order.
    const boardBounds = getBoardLayerBounds();
    setCanvasCards((current) => {
      let changed = false;
      const seenItemIds = new Set<string>();
      const next: NoteCanvasCard[] = [];

      current.forEach((entry) => {
        if (noteItemsById.has(entry.itemId)) {
          seenItemIds.add(entry.itemId);
          next.push(entry);
          return;
        }

        const sourceIdentity = noteSourceIdentityByItemIdRef.current.get(entry.itemId) ?? null;
        const replacementItemId = findReplacementItemIdForSourceNote(
          noteItemsById,
          noteItemIdsBySourcePath,
          sourceIdentity,
        );

        if (replacementItemId && !seenItemIds.has(replacementItemId)) {
          changed = true;
          seenItemIds.add(replacementItemId);
          next.push({ ...entry, itemId: replacementItemId });
          return;
        }

        changed = true;
      });

      if (changed) {
        if (next.length === 0 && current.length > 0 && defaultBoardItemIds.length > 0 && boardBounds) {
          return defaultBoardItemIds.map((itemId, index) => ({
            itemId,
            ...clampCanvasPlacement(
              {
                x: NOTE_CANVAS_SEED_POSITIONS[index]?.x ?? 120 + index * 36,
                y: NOTE_CANVAS_SEED_POSITIONS[index]?.y ?? 120 + index * 28,
              },
              { height: boardBounds.height, width: boardBounds.width },
            ),
            zIndex: index + 1,
          }));
        }

        return next;
      }

      return current;
    });

    if (draggingBoardItemId && !noteItemsById.has(draggingBoardItemId)) {
      const sourceIdentity = noteSourceIdentityByItemIdRef.current.get(draggingBoardItemId) ?? null;
      const replacementItemId = findReplacementItemIdForSourceNote(
        noteItemsById,
        noteItemIdsBySourcePath,
        sourceIdentity,
      );
      if (replacementItemId) {
        if (dragStateRef.current?.itemId === draggingBoardItemId) {
          dragStateRef.current = { ...dragStateRef.current, itemId: replacementItemId };
        }
        setDraggingBoardItemId(replacementItemId);
      } else {
        setDraggingBoardItemId(null);
        dragStateRef.current = null;
      }
    }
  }, [defaultBoardItemIds, draggingBoardItemId, noteItemIdsBySourcePath, noteItemsById]);

  useEffect(() => {
    if (!boardLayerSize) {
      return;
    }

    // Drawer collapse and responsive breakpoints shrink the board after cards
    // were already placed. Re-clamp local card positions so none become
    // unreachable outside the visible canvas.
    setCanvasCards((current) => {
      let changed = false;
      const next = current.map((entry) => {
        const placement = clampCanvasPlacement({ x: entry.x, y: entry.y }, boardLayerSize);
        if (placement.x === entry.x && placement.y === entry.y) {
          return entry;
        }

        changed = true;
        return { ...entry, x: placement.x, y: placement.y };
      });

      return changed ? next : current;
    });
  }, [boardLayerSize]);

  function openNoteDetail(itemId: string) {
    setSelectedItemId(itemId);
    setDetailOpen(true);
  }

  function pinNoteToCanvas(itemId: string, placement?: { x: number; y: number }) {
    setCanvasCards((current) => {
      if (current.some((entry) => entry.itemId === itemId)) {
        return current;
      }

      const seedIndex = current.length % NOTE_CANVAS_SEED_POSITIONS.length;
      const nextPlacement = placement ?? clampCanvasPlacement(
        {
          x: NOTE_CANVAS_SEED_POSITIONS[seedIndex]?.x ?? 120 + current.length * 28,
          y: NOTE_CANVAS_SEED_POSITIONS[seedIndex]?.y ?? 110 + current.length * 24,
        },
        getBoardLayerBounds() ?? { height: NOTE_CANVAS_CARD_HEIGHT * 2, width: NOTE_CANVAS_CARD_WIDTH * 2 },
      );

      return [...current, { itemId, x: nextPlacement.x, y: nextPlacement.y, zIndex: getNextCanvasZIndex(current) }];
    });
  }
  pinNoteToCanvasRef.current = pinNoteToCanvas;

  useEffect(() => {
    const pendingSourceNote = pendingCreatedSourceNoteRef.current;
    if (!pendingSourceNote) {
      return;
    }

    const nextItemId = findReplacementItemIdForSourceNote(
      noteItemsById,
      noteItemIdsBySourcePath,
      pendingSourceNote,
    );
    if (!nextItemId) {
      return;
    }

    const nextItem = noteItemsById.get(nextItemId);
    if (!nextItem) {
      return;
    }

    setDrawerOpen(true);
    setExpandedBucket(nextItem.item.bucket);
    setSelectedItemId(nextItemId);
    pinNoteToCanvasRef.current(nextItemId);
    pendingCreatedSourceNoteRef.current = null;
    showFeedback("新便签已同步到便签页，并放到了网格里。");
  }, [noteItemIdsBySourcePath, noteItemsById]);

  function unpinNoteFromCanvas(itemId: string) {
    setCanvasCards((current) => current.filter((entry) => entry.itemId !== itemId));
    setIsRailDropTarget(false);
  }

  function toggleBucket(bucket: NotePreviewGroupKey) {
    setExpandedBucket(bucket);
    if (!drawerOpen) {
      setDrawerOpen(true);
    }
  }

  function resolveRailDropBucket(itemId: string): NotePreviewGroupKey | null {
    if (!drawerOpen) {
      return noteItemsById.get(itemId)?.item.bucket ?? null;
    }

    return expandedBucket;
  }

  async function runNoteUpdateForRailDrop(itemId: string, action: NotepadAction) {
    const outcome = await updateNote(itemId, action, dataMode);
    await invalidateNoteBuckets(outcome.result.refresh_groups);
    return outcome.result.notepad_item;
  }

  /**
   * Returning a card from the board into the sidebar keeps the board layout
   * local-only, while still using formal note actions when the protocol
   * already supports the requested bucket transition.
   */
  async function syncBoardCardToRailBucket(item: NoteListItem, targetBucket: NotePreviewGroupKey) {
    const sourceBucket = item.item.bucket;
    const sourceLabel = getNoteBucketLabel(sourceBucket);
    const targetLabel = getNoteBucketLabel(targetBucket);
    const presentRailFeedback = (message: string) => {
      window.setTimeout(() => showFeedback(message), 0);
    };

    try {
      if (item.sourceNote?.localOnly && targetBucket !== sourceBucket) {
        if (drawerOpen) {
          setExpandedBucket(sourceBucket);
        }
        presentRailFeedback(`这张源便签还没进入正式事项流，先放回${sourceLabel}。`);
        return;
      }

      if (targetBucket === sourceBucket) {
        if (drawerOpen) {
          setExpandedBucket(targetBucket);
        }
        presentRailFeedback(drawerOpen ? `已放回${targetLabel}分组。` : "已收回侧边栏，可继续在原分组查看。");
        return;
      }

      if (sourceBucket === "later" && targetBucket === "upcoming") {
        await runNoteUpdateForRailDrop(item.item.item_id, "move_upcoming");
        if (drawerOpen) {
          setExpandedBucket("upcoming");
        }
        presentRailFeedback("已放进近期分组，并同步更新便签状态。");
        return;
      }

      if (sourceBucket === "recurring_rule" && targetBucket === "closed") {
        await runNoteUpdateForRailDrop(item.item.item_id, "cancel_recurring");
        if (drawerOpen) {
          setExpandedBucket("closed");
        }
        presentRailFeedback("已放进已结束分组，并结束这条重复规则。");
        return;
      }

      if (sourceBucket === "closed") {
        const restoredItem = await runNoteUpdateForRailDrop(item.item.item_id, "restore");
        const restoredBucket = restoredItem?.bucket ?? sourceBucket;

        if (restoredBucket === targetBucket) {
          if (drawerOpen) {
            setExpandedBucket(restoredBucket);
          }
          presentRailFeedback(`已恢复到${targetLabel}分组。`);
          return;
        }

        if (restoredBucket === "later" && targetBucket === "upcoming") {
          await runNoteUpdateForRailDrop(item.item.item_id, "move_upcoming");
          if (drawerOpen) {
            setExpandedBucket("upcoming");
          }
          presentRailFeedback("已恢复并提前到近期分组。");
          return;
        }

        if (drawerOpen) {
          setExpandedBucket(restoredBucket);
        }
        presentRailFeedback(`当前正式状态还不能直接拖到${targetLabel}，已恢复到${getNoteBucketLabel(restoredBucket)}。`);
        return;
      }

      if (drawerOpen) {
        setExpandedBucket(sourceBucket);
      }
      presentRailFeedback(`当前正式状态还不能直接拖到${targetLabel}，已放回${sourceLabel}。`);
    } catch (error) {
      if (drawerOpen) {
        setExpandedBucket(sourceBucket);
      }
      const message = error instanceof Error ? error.message : "请稍后再试。";
      presentRailFeedback(`便签状态同步失败：${message}`);
    }
  }

  /**
   * The closed-note drawer keeps older records in local UI state and reveals
   * them only after users intentionally scroll to the bottom of the finished
   * history. This stays view-local and does not alter the formal note payload.
   */
  function handleClosedGroupsScroll(event: UIEvent<HTMLDivElement>) {
    if (showMoreClosed || !hasOlderClosedItems) {
      return;
    }

    const { clientHeight, scrollHeight, scrollTop } = event.currentTarget;
    if (scrollHeight - scrollTop - clientHeight <= 28) {
      setShowMoreClosed(true);
    }
  }

  function handleDrawerCardDragStart(
    item: NoteListItem,
    dragSeed: {
      height: number;
      offsetX: number;
      offsetY: number;
      pointerId: number;
      startX: number;
      startY: number;
      width: number;
    },
  ) {
    drawerDragStateRef.current = {
      height: dragSeed.height,
      item,
      offsetX: dragSeed.offsetX,
      offsetY: dragSeed.offsetY,
      pointerId: dragSeed.pointerId,
      started: false,
      startX: dragSeed.startX,
      startY: dragSeed.startY,
      width: dragSeed.width,
    };
  }

  function handleDrawerCardDragMove(itemId: string, event: PointerEvent) {
    const dragState = drawerDragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId || dragState.item.item.item_id !== itemId) {
      return;
    }

    const movedEnough = Math.hypot(event.clientX - dragState.startX, event.clientY - dragState.startY) > 4;
    if (!dragState.started && movedEnough) {
      dragState.started = true;
    }

    if (!dragState.started) {
      return;
    }

    const boardBounds = getBoardLayerBounds();
    if (boardBounds) {
      const overBoard = event.clientX >= boardBounds.left && event.clientX <= boardBounds.right && event.clientY >= boardBounds.top && event.clientY <= boardBounds.bottom;
      setIsBoardDropTarget(overBoard);
    }

    setDrawerDragPreview({
      height: dragState.height,
      item: dragState.item,
      width: dragState.width,
      x: event.clientX - dragState.offsetX,
      y: event.clientY - dragState.offsetY,
    });
  }

  function handleDrawerCardDragEnd(itemId: string, event: PointerEvent) {
    const dragState = drawerDragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId || dragState.item.item.item_id !== itemId) {
      return;
    }

    const boardBounds = getBoardLayerBounds();
    if (boardBounds) {
      const droppedOverBoard =
        event.clientX >= boardBounds.left &&
        event.clientX <= boardBounds.right &&
        event.clientY >= boardBounds.top &&
        event.clientY <= boardBounds.bottom;
      if (droppedOverBoard) {
        pinNoteToCanvas(
          itemId,
          clampCanvasPlacement(
            {
              x: event.clientX - boardBounds.left - dragState.offsetX,
              y: event.clientY - boardBounds.top - dragState.offsetY,
            },
            { height: boardBounds.height, width: boardBounds.width },
            { height: dragState.height, width: dragState.width },
          ),
        );
        showFeedback("已放到网格里，可以继续拖动调整位置。");
      }
    }

    drawerDragStateRef.current = null;
    setDrawerDragPreview(null);
    setIsBoardDropTarget(false);
  }

  /**
   * Starts a board-card drag inside the local canvas only. The offset is view
   * state for arranging preview cards and must not mutate formal note data.
   */
  function handleBoardCardPointerDown(itemId: string, event: ReactPointerEvent<HTMLButtonElement>) {
    const boardBounds = getBoardLayerBounds();
    if (!event.isPrimary || event.button !== 0 || !boardBounds) {
      return;
    }

    const cardRect = event.currentTarget.getBoundingClientRect();
    const currentCard = canvasCards.find((entry) => entry.itemId === itemId);
    const currentOffset = currentCard ? { x: currentCard.x, y: currentCard.y } : { x: 0, y: 0 };

    event.currentTarget.setPointerCapture(event.pointerId);
    setCanvasCards((current) => current.map((entry) => (entry.itemId === itemId ? { ...entry, zIndex: getNextCanvasZIndex(current) } : entry)));
    dragStateRef.current = {
      itemId,
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      originX: currentOffset.x,
      originY: currentOffset.y,
      minX: currentOffset.x + (boardBounds.left - cardRect.left),
      maxX: currentOffset.x + (boardBounds.right - cardRect.right),
      minY: currentOffset.y + (boardBounds.top - cardRect.top),
      maxY: currentOffset.y + (boardBounds.bottom - cardRect.bottom),
      moved: false,
    };
    setDraggingBoardItemId(itemId);
  }

  function handleBoardCardPointerMove(itemId: string, event: ReactPointerEvent<HTMLButtonElement>) {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.itemId !== itemId || dragState.pointerId !== event.pointerId) {
      return;
    }

    const boardBounds = getBoardLayerBounds();
    const deltaX = event.clientX - dragState.startClientX;
    const deltaY = event.clientY - dragState.startClientY;
    const nextX = Math.min(dragState.maxX, Math.max(dragState.minX, dragState.originX + deltaX));
    const nextY = Math.min(dragState.maxY, Math.max(dragState.minY, dragState.originY + deltaY));
    const nextPlacement =
      boardBounds
        ? clampCanvasPlacement(
            { x: nextX, y: nextY },
            { height: boardBounds.height, width: boardBounds.width },
          )
        : { x: nextX, y: nextY };

    if (railRef.current) {
      const railRect = railRef.current.getBoundingClientRect();
      const overRail =
        event.clientX >= railRect.left &&
        event.clientX <= railRect.right &&
        event.clientY >= railRect.top &&
        event.clientY <= railRect.bottom;
      setIsRailDropTarget(overRail);
      // The open drawer treats its currently expanded bucket as the active drop
      // target so users can steer cards back into one visible group at a time.
      setActiveRailDropBucket(overRail ? resolveRailDropBucket(itemId) : null);
    }

    if (!dragState.moved && Math.hypot(deltaX, deltaY) > 4) {
      dragState.moved = true;
    }

    setCanvasCards((current) => current.map((entry) => (entry.itemId === itemId ? { ...entry, x: nextPlacement.x, y: nextPlacement.y } : entry)));
  }

  function finishBoardCardDrag(itemId: string, event: ReactPointerEvent<HTMLButtonElement>) {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.itemId !== itemId || dragState.pointerId !== event.pointerId) {
      return;
    }

    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    if (railRef.current) {
      const railRect = railRef.current.getBoundingClientRect();
      const droppedOverRail =
        event.clientX >= railRect.left &&
        event.clientX <= railRect.right &&
        event.clientY >= railRect.top &&
        event.clientY <= railRect.bottom;
      if (droppedOverRail) {
        const returnedItem = noteItemsById.get(itemId);
        const railDropBucket = resolveRailDropBucket(itemId);
        unpinNoteFromCanvas(itemId);
        if (returnedItem && railDropBucket) {
          void syncBoardCardToRailBucket(returnedItem, railDropBucket);
        } else {
          showFeedback("已收回侧边栏。");
        }
      }
    }

    if (dragState.moved) {
      suppressBoardClickItemIdRef.current = itemId;
      window.setTimeout(() => {
        if (suppressBoardClickItemIdRef.current === itemId) {
          suppressBoardClickItemIdRef.current = null;
        }
      }, 0);
    }

    dragStateRef.current = null;
    setDraggingBoardItemId((current) => (current === itemId ? null : current));
    setIsRailDropTarget(false);
    setActiveRailDropBucket(null);
  }

  function handleBoardCardClick(itemId: string) {
    if (suppressBoardClickItemIdRef.current === itemId) {
      suppressBoardClickItemIdRef.current = null;
      return;
    }

    openNoteDetail(itemId);
  }

  function renderBoardCard(item: NoteListItem, placement: { x: number; y: number; zIndex: number }) {
    return (
      <button
        key={item.item.item_id}
        className={cn(
          "note-preview-page__board-card",
          item.item.item_id === selectedItem?.item.item_id && "is-active",
          draggingBoardItemId === item.item.item_id && "is-dragging",
        )}
        onClick={() => handleBoardCardClick(item.item.item_id)}
        onPointerCancel={(event) => finishBoardCardDrag(item.item.item_id, event)}
        onPointerDown={(event) => handleBoardCardPointerDown(item.item.item_id, event)}
        onPointerMove={(event) => handleBoardCardPointerMove(item.item.item_id, event)}
        onPointerUp={(event) => finishBoardCardDrag(item.item.item_id, event)}
        style={isCompactBoard ? { zIndex: placement.zIndex } : { left: placement.x, top: placement.y, zIndex: placement.zIndex }}
        type="button"
      >
        <div className="note-preview-page__board-card-top">
          <div>
            <p className="note-preview-page__board-kicker">{getNoteBucketLabel(item.item.bucket)}</p>
            <h3 className="note-preview-page__board-card-title">{item.item.title}</h3>
          </div>
          <Badge className={cn("border-0 px-3 py-1 text-[0.72rem] ring-1", getNoteStatusBadgeClass(item.item.status))}>{item.experience.previewStatus}</Badge>
        </div>

        <p className="note-preview-page__board-card-copy">{item.experience.noteText || describeNotePreview(item.item, item.experience)}</p>

        <div className="note-preview-page__board-card-footer">
          <span>{item.experience.timeHint}</span>
          <span>{item.experience.typeLabel}</span>
        </div>

        {item.item.agent_suggestion ? <p className="note-preview-page__board-card-hint">{item.item.agent_suggestion}</p> : null}
      </button>
    );
  }

  return (
    <main className="dashboard-page note-preview-page" style={pageStyle}>
      <>
        <section className="note-preview-page__frame">
          <div className="note-preview-page__page-nav">
            <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
              <ArrowLeft className="h-4 w-4" />
              返回首页
            </Link>

            <nav aria-label="Dashboard modules" className="dashboard-page__module-nav">
              {dashboardModules.map((item) => (
                <NavLink key={item.route} className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")} to={item.path}>
                  {item.title}
                </NavLink>
              ))}
            </nav>
          </div>
          <section className={cn("note-preview-page__workspace", !drawerOpen && "is-drawer-collapsed")}>
            <section className={cn("note-preview-page__board", isBoardDropTarget && "is-drop-target")}>
              <div aria-hidden="true" className="note-preview-page__board-scene" />

              <div className="note-preview-page__board-topbar">
                <div className="note-preview-page__board-heading">
                  <div className="note-preview-page__board-heading-copy">
                    <span className="note-preview-page__board-chip">便签桌面</span>
                    <p>{pageNotice}</p>
                  </div>

                  <div className="note-preview-page__board-summary">
                    <span className="note-preview-page__board-stat">
                      <strong>{summary.dueToday}</strong>
                      <span>今日待处理</span>
                    </span>
                    <span className="note-preview-page__board-stat">
                      <strong>{summary.overdue}</strong>
                      <span>已逾期</span>
                    </span>
                    <span className="note-preview-page__board-stat">
                      <strong>{summary.recurringToday}</strong>
                      <span>今日重复</span>
                    </span>
                    <span className="note-preview-page__board-stat">
                      <strong>{summary.readyForAgent}</strong>
                      <span>适合转任务</span>
                    </span>
                  </div>
                </div>

                <div className="note-preview-page__board-actions">
                  <Button className="note-preview-page__board-action note-preview-page__board-action--primary" onClick={openCreateSourceNoteStudio} size="sm" type="button" variant="ghost">
                    <FilePlus2 className="h-4 w-4" />
                    新建便签
                  </Button>
                  <Button
                    className="note-preview-page__board-action"
                    disabled={isRunningInspection}
                    onClick={() => void refreshInspection("notes_page_manual_run")}
                    size="sm"
                    type="button"
                    variant="ghost"
                  >
                    <ScanSearch className="h-4 w-4" />
                    {isRunningInspection ? "巡检中..." : "立即巡检"}
                  </Button>
                </div>
              </div>

              <div className="note-preview-page__board-layer" ref={boardLayerRef}>
                {boardItems.length > 0
                  ? boardItems.map((entry) => renderBoardCard(entry.item, { x: entry.x, y: entry.y, zIndex: entry.zIndex }))
                  : (
                    <div className="note-preview-page__board-empty">
                      <NoteEmptyState />
                    </div>
                  )}
              </div>
            </section>

            <aside className={cn("note-preview-page__rail-shell", !drawerOpen && "is-collapsed")}>
              <aside className={cn("note-preview-page__rail", !drawerOpen && "is-collapsed", isRailDropTarget && "is-drop-target")} ref={railRef}>
                {drawerOpen ? (
                  <>
                    <NotePreviewSection
                      activeItemId={selectedItem?.item.item_id ?? null}
                      bucketKey="upcoming"
                      draggableToCanvas
                      emptyLabel={upcomingQuery.isPending && !upcomingQuery.data ? "加载中" : "这组便签已全部放到网格。"}
                      isDropTarget={activeRailDropBucket === "upcoming"}
                      isExpanded={expandedBucket === "upcoming"}
                      items={visibleUpcomingItems}
                      onCanvasDragEnd={handleDrawerCardDragEnd}
                      onCanvasDragMove={handleDrawerCardDragMove}
                      onCanvasDragStart={handleDrawerCardDragStart}
                      onSelect={openNoteDetail}
                      onToggle={() => toggleBucket("upcoming")}
                      stackCards
                      title="近期"
                      trailing={<span className="note-preview-shell__count">{upcomingQuery.isPending && !upcomingQuery.data ? "..." : visibleUpcomingItems.length}</span>}
                    />

                    <NotePreviewSection
                      activeItemId={selectedItem?.item.item_id ?? null}
                      bucketKey="later"
                      draggableToCanvas
                      emptyLabel={laterQuery.isPending && !laterQuery.data ? "加载中" : "这组便签已全部放到网格。"}
                      isDropTarget={activeRailDropBucket === "later"}
                      isExpanded={expandedBucket === "later"}
                      items={visibleLaterItems}
                      onCanvasDragEnd={handleDrawerCardDragEnd}
                      onCanvasDragMove={handleDrawerCardDragMove}
                      onCanvasDragStart={handleDrawerCardDragStart}
                      onSelect={openNoteDetail}
                      onToggle={() => toggleBucket("later")}
                      stackCards
                      title="后续"
                      trailing={<span className="note-preview-shell__count">{laterQuery.isPending && !laterQuery.data ? "..." : visibleLaterItems.length}</span>}
                    />

                    <NotePreviewSection
                      activeItemId={selectedItem?.item.item_id ?? null}
                      bucketKey="recurring_rule"
                      draggableToCanvas
                      emptyLabel={recurringQuery.isPending && !recurringQuery.data ? "加载中" : "这组便签已全部放到网格。"}
                      isDropTarget={activeRailDropBucket === "recurring_rule"}
                      isExpanded={expandedBucket === "recurring_rule"}
                      items={visibleRecurringItems}
                      onCanvasDragEnd={handleDrawerCardDragEnd}
                      onCanvasDragMove={handleDrawerCardDragMove}
                      onCanvasDragStart={handleDrawerCardDragStart}
                      onSelect={openNoteDetail}
                      onToggle={() => toggleBucket("recurring_rule")}
                      stackCards
                      title="重复"
                      trailing={<span className="note-preview-shell__count">{recurringQuery.isPending && !recurringQuery.data ? "..." : visibleRecurringItems.length}</span>}
                    />

                    <article className={cn("dashboard-card note-preview-shell", activeRailDropBucket === "closed" && "is-drop-target", expandedBucket === "closed" ? "is-expanded" : "is-collapsed")}>
                      <button aria-expanded={expandedBucket === "closed"} className="note-preview-shell__bucket-toggle" onClick={() => toggleBucket("closed")} type="button">
                        <p className="dashboard-card__kicker">已结束</p>
                        <span className="note-preview-shell__count">{closedQuery.isPending && !closedQuery.data ? "..." : visibleClosedItems.length}</span>
                      </button>

                      {expandedBucket === "closed" ? (
                        <div className="note-preview-shell__bucket-body">
                          <div className="note-preview-shell__body-toolbar">
                            <p className="note-preview-shell__body-copy">默认展示近 3 天；滚到最底部时，会继续补出更早记录。</p>
                          </div>

                          <div className="note-preview-finished-groups" onScroll={handleClosedGroupsScroll}>
                            {closedGroups.length > 0 ? (
                              closedGroups.map((group) => (
                                <section key={group.key} className="note-preview-finished-group">
                                  <div>
                                    <p className="note-preview-finished-group__title">{group.title}</p>
                                    <p className="note-preview-finished-group__description">{group.description}</p>
                                  </div>
                                  <div className={cn("note-preview-shell__list", group.items.length > 1 && "note-preview-shell__list--stacked")}>
                                    {group.items.map((entry) => (
                                      <NotePreviewCard
                                        draggableToCanvas
                                        key={entry.item.item_id}
                                        isActive={entry.item.item_id === selectedItem?.item.item_id}
                                        item={entry}
                                        onCanvasDragEnd={handleDrawerCardDragEnd}
                                        onCanvasDragMove={handleDrawerCardDragMove}
                                        onCanvasDragStart={handleDrawerCardDragStart}
                                        onSelect={openNoteDetail}
                                      />
                                    ))}
                                  </div>
                                </section>
                              ))
                            ) : closedQuery.isPending && !closedQuery.data ? (
                              <div className="note-preview-shell__empty">加载中</div>
                            ) : !showMoreClosed && hasOlderClosedItems ? (
                              <div className="note-preview-shell__empty-stack">
                                <div className="note-preview-shell__empty">当前只有更早时间的已结束记录。</div>
                                <button className="note-preview-shell__toggle" onClick={() => setShowMoreClosed(true)} type="button">
                                  加载更早记录
                                </button>
                              </div>
                            ) : (
                              <div className="note-preview-shell__empty">暂无记录</div>
                            )}

                            {!showMoreClosed && hasOlderClosedItems && closedGroups.length > 0 ? <div className="note-preview-finished-groups__sentinel" aria-hidden="true" /> : null}
                          </div>
                        </div>
                      ) : null}
                    </article>
                  </>
                ) : (
                  <div className="note-preview-page__rail-dropzone">
                    <span className="note-preview-page__rail-dropzone-kicker">侧边栏已收起</span>
                    <p className="note-preview-page__rail-dropzone-title">拖回这里</p>
                    <p className="note-preview-page__rail-dropzone-copy">放手后会回到原本分组。</p>
                  </div>
                )}
              </aside>

              <button className={cn("note-preview-page__drawer-handle", !drawerOpen && "is-collapsed")} onClick={() => setDrawerOpen((current) => !current)} type="button">
                {drawerOpen ? <PanelLeftClose className="h-4 w-4" /> : <PanelLeftOpen className="h-4 w-4" />}
                <span>{drawerOpen ? "收起侧边栏" : "展开侧边栏"}</span>
              </button>
            </aside>
          </section>
        </section>

        {drawerDragPreview ? (
          <div
            aria-hidden="true"
            className="note-preview-page__drag-ghost"
            style={{ height: drawerDragPreview.height, left: drawerDragPreview.x, top: drawerDragPreview.y, width: drawerDragPreview.width }}
          >
            <div className="note-preview-page__drag-ghost-kicker">{getNoteBucketLabel(drawerDragPreview.item.item.bucket)}</div>
            <p className="note-preview-page__drag-ghost-title">{drawerDragPreview.item.item.title}</p>
          </div>
        ) : null}

        <AnimatePresence>
            {detailOpen && selectedItem ? (
              <>
                <motion.button
                  animate={{ opacity: 1 }}
                  className="note-detail-modal__backdrop"
                  exit={{ opacity: 0 }}
                  initial={{ opacity: 0 }}
                  onClick={() => setDetailOpen(false)}
                  type="button"
                />
                <motion.div
                  animate={{ opacity: 1, scale: 1, y: 0 }}
                  className="note-detail-modal"
                  exit={{ opacity: 0, scale: 0.98, y: 20 }}
                  initial={{ opacity: 0, scale: 0.98, y: 16 }}
                  transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
                >
                  <NoteDetailPanel feedback={feedback} item={selectedItem} onAction={handleDetailAction} onClose={() => setDetailOpen(false)} onResourceOpen={handleResourceOpen} />
                </motion.div>
              </>
            ) : null}
        </AnimatePresence>

        <AnimatePresence>
            {sourceStudioOpen ? (
              <>
                <motion.button
                  animate={{ opacity: 1 }}
                  className="note-detail-modal__backdrop note-source-modal__backdrop"
                  exit={{ opacity: 0 }}
                  initial={{ opacity: 0 }}
                  onClick={() => setSourceStudioOpen(false)}
                  type="button"
                />
                <motion.div
                  animate={{ opacity: 1, scale: 1, y: 0 }}
                  className="note-detail-modal note-detail-modal--source"
                  exit={{ opacity: 0, scale: 0.98, y: 20 }}
                  initial={{ opacity: 0, scale: 0.98, y: 16 }}
                  transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
                >
                  <section className="note-source-modal">
                    <div className="note-source-modal__header">
                      <div>
                        <p className="note-preview-page__eyebrow">Source Notes</p>
                        <h2 className="note-source-modal__title">{isCreatingSourceNote ? "空白便签" : "任务来源便签"}</h2>
                      </div>
                      <Button className="note-source-modal__close" onClick={() => setSourceStudioOpen(false)} size="icon-sm" type="button" variant="ghost">
                        <X className="h-4 w-4" />
                        <span className="sr-only">关闭源便签编辑器</span>
                      </Button>
                    </div>

                    <SourceNoteStudio
                      availabilityMessage={sourceNoteAvailabilityMessage}
                      draft={sourceNoteDraft}
                      editingItem={sourceStudioItem}
                      fileLabel={sourceStudioFileLabel}
                      isCreating={isCreatingSourceNote}
                      isDirty={sourceEditorDirty}
                      isInspecting={isRunningInspection}
                      isLoading={sourceNotesLoading}
                      isSaving={isSavingSourceNote}
                      onChange={(value) => setSourceNoteDraft(value)}
                      onClose={() => setSourceStudioOpen(false)}
                      onCreate={openCreateSourceNoteStudio}
                      onInspect={() => void refreshInspection("notes_page_manual_run")}
                      onReload={() => {
                        void sourceConfigQuery.refetch();
                        void sourceNotesQuery.refetch();
                        void sourceNoteIndexQuery.refetch();
                      }}
                      onSave={() => void handleSaveSourceNote()}
                      sourceRoots={resolvedSourceRoots}
                      syncMessage={sourceNoteSyncMessage}
                    />
                  </section>
                </motion.div>
              </>
            ) : null}
        </AnimatePresence>

        <AnimatePresence>
            {(feedback || queryErrors.length > 0) ? (
              <motion.aside
                animate={{ opacity: 1, y: 0 }}
                className="note-preview-page__floating-card"
                exit={{ opacity: 0, y: 12 }}
                initial={{ opacity: 0, y: 16 }}
                transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
              >
                <div className="note-preview-page__floating-card-icon">
                  <AlertTriangle className="h-4 w-4" />
                </div>
                <div className="note-preview-page__floating-card-copy">
                  <p className="note-preview-page__floating-card-title">{feedback ? "操作提示" : "便签同步失败"}</p>
                  <p className="note-preview-page__floating-card-text">
                    {feedback ??
                      (queryErrors.length === 1
                        ? `${queryErrors[0].label}：${queryErrors[0].error instanceof Error ? queryErrors[0].error.message : "请求失败"}`
                        : `${queryErrors.length} 个分区加载失败：${queryErrors
                            .map((item) => `${item.label}${item.error instanceof Error ? `(${item.error.message})` : ""}`)
                            .join("、")}`)}
                  </p>
                </div>
                {!feedback ? (
                  <button
                    className="note-preview-page__floating-card-action"
                    onClick={() => {
                      void upcomingQuery.refetch();
                      void laterQuery.refetch();
                      void recurringQuery.refetch();
                      void closedQuery.refetch();
                    }}
                    type="button"
                  >
                    <RefreshCcw className="h-4 w-4" />
                    重试
                  </button>
                ) : null}
              </motion.aside>
            ) : null}
        </AnimatePresence>

        <DashboardMockToggle
          enabled={dataMode === "mock"}
          onToggle={() => {
            setFeedback(null);
            setSourceNoteSyncMessage(null);
            setDataMode((current) => (current === "rpc" ? "mock" : "rpc"));
          }}
        />
      </>
    </main>
  );
}
