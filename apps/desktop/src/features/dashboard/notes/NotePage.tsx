/**
 * Note dashboard page now behaves like a mirror-style workbench while keeping
 * the stable notepad RPC boundary unchanged.
 */
import { useEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQueries, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowLeft,
  ArrowUpRight,
  CheckCircle2,
  LayoutGrid,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCcw,
  Repeat2,
  RotateCcw,
  Sparkles,
  X,
  XCircle,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import type { NotepadAction, TodoBucket } from "@cialloclaw/protocol";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { describeNotePreview, getNoteBucketLabel, getNoteStatusBadgeClass, sortClosedNotes, sortNotesByUrgency } from "./notePage.mapper";
import {
  createNoteCanvasCardLayout,
  findNextNoteCanvasPoint,
  NOTE_CANVAS_CARD_HEIGHT,
  NOTE_CANVAS_CARD_WIDTH,
  pruneNoteCanvasLayoutSnapshot,
  snapNoteCanvasPoint,
  type NoteCanvasBounds,
} from "./noteCanvasLayout";
import { buildDashboardNoteBucketInvalidateKeys, buildDashboardNoteBucketQueryKey, dashboardNoteBucketGroups, getDashboardNoteRefreshPlan } from "./notePage.query";
import {
  convertNoteToTask,
  loadNoteBucket,
  performNoteResourceOpenExecution,
  resolveNoteResourceOpenExecutionPlan,
  updateNote,
  type NotePageDataMode,
  type NoteResourceOpenExecutionPlan,
} from "./notePage.service";
import type {
  NoteCanvasLayoutSnapshot,
  NoteDetailLayerState,
  NoteDetailAction,
  NoteDrawerPreferenceSnapshot,
  NoteListItem,
  NoteSelectionState,
} from "./notePage.types";
import { loadNoteCanvasLayoutSnapshot, loadNoteDrawerPreferenceSnapshot, saveNoteCanvasLayoutSnapshot, saveNoteDrawerPreferenceSnapshot } from "./noteWorkspaceStorage";
import "./notePage.css";

type NoteBucketConfig = {
  key: TodoBucket;
  label: string;
  emptyDescription: string;
  pinnedDescription: string;
};

type NoteDragState = {
  itemId: string;
  bucket: TodoBucket;
  moved: boolean;
  offsetX: number;
  offsetY: number;
  originX: number;
  originY: number;
  pointerId: number;
  previewX: number;
  previewY: number;
  source: "canvas" | "drawer";
  startX: number;
  startY: number;
};

const NOTE_BUCKET_CONFIGS: NoteBucketConfig[] = [
  {
    key: "upcoming",
    label: "近期",
    emptyDescription: "没有等待你处理的近期便签。",
    pinnedDescription: "这组便签已经全部放上画布。",
  },
  {
    key: "later",
    label: "后续",
    emptyDescription: "暂时没有后续安排。",
    pinnedDescription: "后续安排已经全部移到画布上。",
  },
  {
    key: "recurring_rule",
    label: "重复",
    emptyDescription: "当前没有重复规则。",
    pinnedDescription: "重复规则都已经在画布中。",
  },
  {
    key: "closed",
    label: "已结束",
    emptyDescription: "完成或取消的便签会显示在这里。",
    pinnedDescription: "已结束便签都已经在画布中。",
  },
];

const NOTE_DRAG_THRESHOLD = 8;
const DEFAULT_DRAWER_PREFERENCES: NoteDrawerPreferenceSnapshot = {
  drawerOpen: true,
  expandedBucket: "upcoming",
};
const DEFAULT_SELECTION_STATE: NoteSelectionState = {
  bucket: null,
  itemId: null,
};
const DEFAULT_DETAIL_LAYER_STATE: NoteDetailLayerState = {
  activeBucket: null,
  openItemIds: [],
};

function buildPrimaryStatusAction(item: NoteListItem): { action: NotepadAction; label: string; tone: "normal" | "warn" | "success" } | null {
  if (item.item.bucket === "upcoming") {
    return { action: "complete", label: "标记完成", tone: "success" };
  }

  if (item.item.bucket === "later") {
    return { action: "move_upcoming", label: "提前到近期", tone: "normal" };
  }

  if (item.item.bucket === "recurring_rule") {
    return {
      action: "toggle_recurring",
      label: item.experience.isRecurringEnabled ? "暂停重复" : "开启重复",
      tone: "normal",
    };
  }

  return { action: "restore", label: "恢复到近期", tone: "warn" };
}

function buildSecondaryActions(item: NoteListItem): Array<{ action: NoteDetailAction; label: string }> {
  if (item.item.bucket === "upcoming") {
    return [{ action: "cancel", label: "取消便签" }];
  }

  if (item.item.bucket === "later") {
    return [{ action: "cancel", label: "取消便签" }];
  }

  if (item.item.bucket === "recurring_rule") {
    return [{ action: "cancel-recurring", label: "结束规则" }];
  }

  return [{ action: "delete", label: "删除记录" }];
}

function buildActionFeedback(action: NotepadAction, recurringEnabled?: boolean) {
  const feedbackByAction: Record<NotepadAction, string> = {
    cancel: "已取消这条便签。",
    cancel_recurring: "已结束整条重复规则。",
    complete: "已将便签标记为完成。",
    delete: "已删除这条便签记录。",
    move_upcoming: "已提前到近期分组。",
    restore: "已恢复到近期分组。",
    toggle_recurring: recurringEnabled === false ? "已暂停重复规则。" : "已重新开启重复规则。",
  };

  return feedbackByAction[action];
}

function buildQueryErrorEntries(queries: Array<{ error: unknown }>) {
  return [
    { error: queries[0].error, label: "近期" },
    { error: queries[1].error, label: "后续" },
    { error: queries[2].error, label: "重复" },
    { error: queries[3].error, label: "已结束" },
  ].filter((entry) => entry.error);
}

function isPointWithinRect(clientX: number, clientY: number, rect: DOMRect) {
  return clientX >= rect.left && clientX <= rect.right && clientY >= rect.top && clientY <= rect.bottom;
}

function areSnapshotsEqual(left: NoteCanvasLayoutSnapshot, right: NoteCanvasLayoutSnapshot) {
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);

  if (leftKeys.length !== rightKeys.length) {
    return false;
  }

  return leftKeys.every((key) => {
    const leftEntry = left[key];
    const rightEntry = right[key];

    return (
      rightEntry &&
      leftEntry.itemId === rightEntry.itemId &&
      leftEntry.sourceBucket === rightEntry.sourceBucket &&
      leftEntry.x === rightEntry.x &&
      leftEntry.y === rightEntry.y
    );
  });
}

function areItemIdListsEqual(left: string[], right: string[]) {
  if (left.length !== right.length) {
    return false;
  }

  return left.every((itemId, index) => itemId === right[index]);
}

function getFirstResource(item: NoteListItem) {
  return item.experience.relatedResources[0] ?? null;
}

function getDrawerPreferencesFromStorage() {
  return loadNoteDrawerPreferenceSnapshot() ?? DEFAULT_DRAWER_PREFERENCES;
}

function getDefaultCanvasBounds(): NoteCanvasBounds {
  return {
    height: NOTE_CANVAS_CARD_HEIGHT + 56,
    width: NOTE_CANVAS_CARD_WIDTH + 56,
  };
}

/**
 * Renders the note dashboard as a mirror-style workbench while keeping the
 * stable notepad RPC boundary unchanged.
 */
export function NotePage() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const storedDrawerPreferences = getDrawerPreferencesFromStorage();
  const [dataMode, setDataMode] = useState<NotePageDataMode>(() => loadDashboardDataMode("notes") as NotePageDataMode);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [pendingConfirmation, setPendingConfirmation] = useState<NoteResourceOpenExecutionPlan | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(storedDrawerPreferences.drawerOpen);
  const [expandedBucket, setExpandedBucket] = useState<TodoBucket>(storedDrawerPreferences.expandedBucket);
  const [selectionState, setSelectionState] = useState<NoteSelectionState>(DEFAULT_SELECTION_STATE);
  const [detailLayerState, setDetailLayerState] = useState<NoteDetailLayerState>({
    ...DEFAULT_DETAIL_LAYER_STATE,
    activeBucket: storedDrawerPreferences.expandedBucket,
  });
  const [canvasSnapshot, setCanvasSnapshot] = useState<NoteCanvasLayoutSnapshot>(() => loadNoteCanvasLayoutSnapshot());
  const [canvasBounds, setCanvasBounds] = useState<NoteCanvasBounds>(getDefaultCanvasBounds());
  const [dragState, setDragState] = useState<NoteDragState | null>(null);
  const dragStateRef = useRef<NoteDragState | null>(null);
  const feedbackTimeoutRef = useRef<number | null>(null);
  const scrollAfterPinBucketRef = useRef<TodoBucket | null>(null);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const drawerRef = useRef<HTMLDivElement | null>(null);
  const drawerItemRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const noteRefreshPlan = useMemo(() => getDashboardNoteRefreshPlan(dataMode), [dataMode]);

  function handleTopbarRouteClick(targetPath: string) {
    if (location.pathname === targetPath) {
      return;
    }

    navigate(targetPath);
  }

  function handleTopbarPointerDown(event: ReactPointerEvent<HTMLElement>) {
    event.stopPropagation();
  }

  function isTopbarRouteActive(targetPath: string) {
    if (targetPath === "/") {
      return location.pathname === "/";
    }

    return location.pathname === targetPath || location.pathname.startsWith(`${targetPath}/`);
  }

  useEffect(() => {
    saveDashboardDataMode("notes", dataMode);
  }, [dataMode]);

  useEffect(() => {
    saveNoteDrawerPreferenceSnapshot({
      drawerOpen,
      expandedBucket,
    });
  }, [drawerOpen, expandedBucket]);

  useEffect(() => {
    saveNoteCanvasLayoutSnapshot(canvasSnapshot);
  }, [canvasSnapshot]);

  useEffect(() => {
    if (!canvasRef.current || typeof ResizeObserver === "undefined") {
      return;
    }

    const canvasElement = canvasRef.current;
    const updateBounds = () => {
      setCanvasBounds({
        height: Math.max(canvasElement.clientHeight, NOTE_CANVAS_CARD_HEIGHT + 56),
        width: Math.max(canvasElement.clientWidth, NOTE_CANVAS_CARD_WIDTH + 56),
      });
    };

    updateBounds();
    const resizeObserver = new ResizeObserver(updateBounds);
    resizeObserver.observe(canvasElement);

    return () => {
      resizeObserver.disconnect();
    };
  }, []);

  const queries = useQueries({
    queries: dashboardNoteBucketGroups.map((group) => ({
      queryKey: buildDashboardNoteBucketQueryKey(dataMode, group),
      queryFn: () => loadNoteBucket(group, dataMode),
      retry: false,
      refetchOnMount: noteRefreshPlan.refetchOnMount,
      refetchOnReconnect: false,
      refetchOnWindowFocus: false,
    })),
  });

  const upcomingItems = sortNotesByUrgency(queries[0].data?.items ?? []);
  const laterItems = sortNotesByUrgency(queries[1].data?.items ?? []);
  const recurringItems = sortNotesByUrgency(queries[2].data?.items ?? []);
  const closedItems = sortClosedNotes(queries[3].data?.items ?? []);
  const itemsByBucket = useMemo(
    () =>
      ({
        upcoming: upcomingItems,
        later: laterItems,
        recurring_rule: recurringItems,
        closed: closedItems,
      }) satisfies Record<TodoBucket, NoteListItem[]>,
    [closedItems, laterItems, recurringItems, upcomingItems],
  );
  const allItems = useMemo(() => [...upcomingItems, ...laterItems, ...recurringItems, ...closedItems], [closedItems, laterItems, recurringItems, upcomingItems]);
  const itemMap = useMemo(() => new Map(allItems.map((item) => [item.item.item_id, item])), [allItems]);
  const pinnedItemIds = useMemo(() => new Set(Object.keys(canvasSnapshot)), [canvasSnapshot]);
  const visibleItemsByBucket = useMemo(
    () =>
      NOTE_BUCKET_CONFIGS.reduce<Record<TodoBucket, NoteListItem[]>>((result, config) => {
        result[config.key] = itemsByBucket[config.key].filter((item) => !pinnedItemIds.has(item.item.item_id));
        return result;
      }, {} as Record<TodoBucket, NoteListItem[]>),
    [itemsByBucket, pinnedItemIds],
  );
  const canvasItems = useMemo(
    () =>
      Object.values(canvasSnapshot)
        .map((layout) => {
          const item = itemMap.get(layout.itemId);
          return item ? { item, layout } : null;
        })
        .filter((entry): entry is { item: NoteListItem; layout: NoteCanvasLayoutSnapshot[string] } => Boolean(entry)),
    [canvasSnapshot, itemMap],
  );
  const queryErrors = buildQueryErrorEntries(queries);
  const expandedBucketLabel = NOTE_BUCKET_CONFIGS.find((config) => config.key === expandedBucket)?.label ?? "近期";
  const pageStyle = {
    "--note-accent": "#d88e63",
    "--note-accent-strong": "#86573b",
    "--note-glow": "rgba(233, 189, 159, 0.42)",
    "--note-line": "rgba(122, 92, 65, 0.18)",
    "--note-copy": "rgba(72, 56, 44, 0.72)",
    "--note-ink": "#312419",
  } as CSSProperties;

  function clearFeedbackTimeout() {
    if (feedbackTimeoutRef.current) {
      window.clearTimeout(feedbackTimeoutRef.current);
      feedbackTimeoutRef.current = null;
    }
  }

  function showFeedback(message: string, autoHide = true) {
    setFeedback(message);
    clearFeedbackTimeout();

    if (!autoHide) {
      return;
    }

    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2600);
  }

  function activateBucket(bucket: TodoBucket) {
    setExpandedBucket((currentBucket) => (currentBucket === bucket ? currentBucket : bucket));
    setDetailLayerState((currentState) => {
      if (currentState.activeBucket === bucket) {
        return currentState;
      }

      return {
        activeBucket: bucket,
        openItemIds: currentState.openItemIds.filter((itemId) => itemMap.get(itemId)?.item.bucket === bucket),
      };
    });
  }

  const convertMutation = useMutation({
    mutationFn: (itemId: string) => convertNoteToTask(itemId, dataMode),
    onSuccess: async (outcome, itemId) => {
      await Promise.all(
        buildDashboardNoteBucketInvalidateKeys(dataMode, outcome.result.refresh_groups).map((queryKey) =>
          queryClient.invalidateQueries({ queryKey }),
        ),
      );

      setSelectionState({
        bucket: outcome.result.notepad_item?.bucket ?? selectionState.bucket,
        itemId,
      });
      showFeedback("已生成关联任务，便签会继续保留在原分组。");
    },
    onError: (error) => {
      showFeedback(error instanceof Error ? `转任务失败：${error.message}` : "转任务失败，请稍后重试。");
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ action, itemId }: { action: NotepadAction; itemId: string }) => updateNote(itemId, action, dataMode),
    onSuccess: async (outcome, variables) => {
      await Promise.all(
        buildDashboardNoteBucketInvalidateKeys(dataMode, outcome.result.refresh_groups).map((queryKey) =>
          queryClient.invalidateQueries({ queryKey }),
        ),
      );

      if (outcome.result.deleted_item_id) {
        setCanvasSnapshot((currentSnapshot) => {
          if (!(variables.itemId in currentSnapshot)) {
            return currentSnapshot;
          }

          const nextSnapshot = { ...currentSnapshot };
          delete nextSnapshot[variables.itemId];
          return nextSnapshot;
        });
        setSelectionState((currentState) => (currentState.itemId === variables.itemId ? DEFAULT_SELECTION_STATE : currentState));
        setDetailLayerState((currentState) => ({
          activeBucket: currentState.activeBucket,
          openItemIds: currentState.openItemIds.filter((itemId) => itemId !== variables.itemId),
        }));
      }

      if (outcome.result.notepad_item) {
        activateBucket(outcome.result.notepad_item.bucket);
        setSelectionState({
          bucket: outcome.result.notepad_item.bucket,
          itemId: outcome.result.notepad_item.item_id,
        });
      }

      showFeedback(buildActionFeedback(variables.action, outcome.result.notepad_item?.recurring_enabled));
    },
    onError: (error, variables) => {
      showFeedback(error instanceof Error ? `更新失败（${variables.action}）：${error.message}` : "便签更新失败，请稍后重试。");
    },
  });

  function mapDetailActionToMutation(action: NoteDetailAction): NotepadAction | null {
    switch (action) {
      case "cancel":
        return "cancel";
      case "cancel-recurring":
        return "cancel_recurring";
      case "complete":
        return "complete";
      case "delete":
        return "delete";
      case "move-upcoming":
        return "move_upcoming";
      case "restore":
        return "restore";
      case "toggle-recurring":
        return "toggle_recurring";
      default:
        return null;
    }
  }

  async function handleOpenPlan(plan: NoteResourceOpenExecutionPlan, approveOutsideWorkspace = false) {
    const result = await performNoteResourceOpenExecution(plan, { approveOutsideWorkspace });

    if (result.type === "confirm_required") {
      setPendingConfirmation(result.plan);
      showFeedback(result.message, false);
      return;
    }

    setPendingConfirmation(null);

    if (result.type === "task_detail") {
      navigate(resolveDashboardModuleRoutePath("tasks"), {
        state: {
          focusTaskId: result.taskId,
          openDetail: true,
        },
      });
      showFeedback(result.message);
      return;
    }

    if (result.type === "opened") {
      showFeedback(result.message);
      return;
    }

    if (result.type === "error") {
      showFeedback(result.message);
      return;
    }

  }

  async function openFirstResource(item: NoteListItem) {
    const resource = getFirstResource(item);
    if (!resource) {
      showFeedback("当前便签没有可打开的资源。");
      return;
    }

    const plan = resolveNoteResourceOpenExecutionPlan(resource);
    await handleOpenPlan(plan);
  }

  function openLinkedTask(taskId: string) {
    navigate(resolveDashboardModuleRoutePath("tasks"), {
      state: {
        focusTaskId: taskId,
        openDetail: true,
      },
    });
  }

  function handleDetailAction(item: NoteListItem, action: NoteDetailAction) {
    if (action === "convert-to-task") {
      convertMutation.mutate(item.item.item_id);
      return;
    }

    if (action === "open-resource") {
      void openFirstResource(item);
      return;
    }

    if (action === "view-task" && item.item.linked_task_id) {
      openLinkedTask(item.item.linked_task_id);
      return;
    }

    const mutationAction = mapDetailActionToMutation(action);
    if (mutationAction) {
      updateMutation.mutate({
        action: mutationAction,
        itemId: item.item.item_id,
      });
      return;
    }

    showFeedback("当前动作会在后续接入。");
  }

  function pinNoteToCanvas(item: NoteListItem, point?: { x: number; y: number }) {
    activateBucket(item.item.bucket);
    setSelectionState({
      bucket: item.item.bucket,
      itemId: item.item.item_id,
    });
    setCanvasSnapshot((currentSnapshot) => {
      const nextPoint = point ? snapNoteCanvasPoint(point, canvasBounds) : findNextNoteCanvasPoint(currentSnapshot, canvasBounds, item.item.item_id);

      return {
        ...currentSnapshot,
        [item.item.item_id]: createNoteCanvasCardLayout(item.item.item_id, item.item.bucket, nextPoint),
      };
    });
    scrollAfterPinBucketRef.current = item.item.bucket;
  }

  function removePinnedNote(itemId: string) {
    setCanvasSnapshot((currentSnapshot) => {
      if (!(itemId in currentSnapshot)) {
        return currentSnapshot;
      }

      const nextSnapshot = { ...currentSnapshot };
      delete nextSnapshot[itemId];
      return nextSnapshot;
    });
    setDetailLayerState((currentState) => ({
      activeBucket: currentState.activeBucket,
      openItemIds: currentState.openItemIds.filter((openItemId) => openItemId !== itemId),
    }));
  }

  function toggleDetailLayer(item: NoteListItem) {
    activateBucket(item.item.bucket);
    setDetailLayerState((currentState) => {
      const isOpen = currentState.openItemIds.includes(item.item.item_id);
      return {
        activeBucket: item.item.bucket,
        openItemIds: isOpen
          ? currentState.openItemIds.filter((itemId) => itemId !== item.item.item_id)
          : [...currentState.openItemIds, item.item.item_id],
      };
    });
  }

  function handleCanvasCardClick(item: NoteListItem) {
    activateBucket(item.item.bucket);

    if (selectionState.itemId !== item.item.item_id) {
      setSelectionState({
        bucket: item.item.bucket,
        itemId: item.item.item_id,
      });
      return;
    }

    toggleDetailLayer(item);
  }

  function computeCanvasPointFromClientPosition(clientX: number, clientY: number, offsetX: number, offsetY: number) {
    const canvasRect = canvasRef.current?.getBoundingClientRect();
    if (!canvasRect) {
      return null;
    }

    return snapNoteCanvasPoint(
      {
        x: clientX - canvasRect.left - offsetX,
        y: clientY - canvasRect.top - offsetY,
      },
      canvasBounds,
    );
  }

  function stopDragging() {
    window.removeEventListener("pointermove", handleWindowPointerMove);
    window.removeEventListener("pointerup", handleWindowPointerUp);
    window.removeEventListener("pointercancel", handleWindowPointerCancel);
    dragStateRef.current = null;
    setDragState(null);
  }

  function handleWindowPointerMove(event: PointerEvent) {
    const currentDragState = dragStateRef.current;
    if (!currentDragState || currentDragState.pointerId !== event.pointerId) {
      return;
    }

    const deltaX = event.clientX - currentDragState.startX;
    const deltaY = event.clientY - currentDragState.startY;
    const moved = currentDragState.moved || Math.hypot(deltaX, deltaY) >= NOTE_DRAG_THRESHOLD;
    const nextPreviewX =
      currentDragState.source === "drawer"
        ? event.clientX - currentDragState.offsetX
        : currentDragState.originX + deltaX;
    const nextPreviewY =
      currentDragState.source === "drawer"
        ? event.clientY - currentDragState.offsetY
        : currentDragState.originY + deltaY;

    const nextDragState: NoteDragState = {
      ...currentDragState,
      moved,
      previewX: nextPreviewX,
      previewY: nextPreviewY,
    };

    dragStateRef.current = nextDragState;
    setDragState(nextDragState);
  }

  function finalizeDrawerDrop(currentDragState: NoteDragState) {
    const item = itemMap.get(currentDragState.itemId);
    if (!item) {
      return;
    }

    const releaseX = currentDragState.previewX + currentDragState.offsetX;
    const releaseY = currentDragState.previewY + currentDragState.offsetY;
    const drawerRect = drawerRef.current?.getBoundingClientRect();
    const canvasRect = canvasRef.current?.getBoundingClientRect();
    if (currentDragState.moved && drawerRect && isPointWithinRect(releaseX, releaseY, drawerRect)) {
      return;
    }

    if (!currentDragState.moved || !canvasRect || !isPointWithinRect(releaseX, releaseY, canvasRect)) {
      pinNoteToCanvas(item);
      return;
    }

    const point = computeCanvasPointFromClientPosition(
      releaseX,
      releaseY,
      currentDragState.offsetX,
      currentDragState.offsetY,
    );

    pinNoteToCanvas(item, point ?? undefined);
  }

  function finalizeCanvasDrop(currentDragState: NoteDragState) {
    const item = itemMap.get(currentDragState.itemId);
    if (!item) {
      return;
    }

    if (!currentDragState.moved) {
      handleCanvasCardClick(item);
      return;
    }

    const drawerRect = drawerRef.current?.getBoundingClientRect();
    if (drawerRect && isPointWithinRect(currentDragState.previewX + currentDragState.offsetX, currentDragState.previewY + currentDragState.offsetY, drawerRect)) {
      removePinnedNote(item.item.item_id);
      return;
    }

    const point = computeCanvasPointFromClientPosition(
      currentDragState.previewX + currentDragState.offsetX,
      currentDragState.previewY + currentDragState.offsetY,
      currentDragState.offsetX,
      currentDragState.offsetY,
    );

    if (!point) {
      return;
    }

    setCanvasSnapshot((currentSnapshot) => ({
      ...currentSnapshot,
      [item.item.item_id]: createNoteCanvasCardLayout(
        item.item.item_id,
        currentSnapshot[item.item.item_id]?.sourceBucket ?? item.item.bucket,
        point,
      ),
    }));
  }

  function handleWindowPointerUp(event: PointerEvent) {
    const currentDragState = dragStateRef.current;
    if (!currentDragState || currentDragState.pointerId !== event.pointerId) {
      return;
    }

    if (currentDragState.source === "drawer") {
      finalizeDrawerDrop(currentDragState);
    } else {
      finalizeCanvasDrop(currentDragState);
    }

    stopDragging();
  }

  function handleWindowPointerCancel() {
    stopDragging();
  }

  function startDrawerDrag(item: NoteListItem, event: ReactPointerEvent<HTMLButtonElement>) {
    if (event.button !== 0) {
      return;
    }

    const rect = event.currentTarget.getBoundingClientRect();
    const nextDragState: NoteDragState = {
      itemId: item.item.item_id,
      bucket: item.item.bucket,
      moved: false,
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
      originX: rect.left,
      originY: rect.top,
      pointerId: event.pointerId,
      previewX: rect.left,
      previewY: rect.top,
      source: "drawer",
      startX: event.clientX,
      startY: event.clientY,
    };

    dragStateRef.current = nextDragState;
    setDragState(nextDragState);
    window.addEventListener("pointermove", handleWindowPointerMove);
    window.addEventListener("pointerup", handleWindowPointerUp);
    window.addEventListener("pointercancel", handleWindowPointerCancel);
  }

  function startCanvasDrag(item: NoteListItem, event: ReactPointerEvent<HTMLDivElement>) {
    if (event.button !== 0) {
      return;
    }

    const layout = canvasSnapshot[item.item.item_id];
    if (!layout) {
      return;
    }

    const rect = event.currentTarget.getBoundingClientRect();
    const nextDragState: NoteDragState = {
      itemId: item.item.item_id,
      bucket: item.item.bucket,
      moved: false,
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
      originX: rect.left,
      originY: rect.top,
      pointerId: event.pointerId,
      previewX: rect.left,
      previewY: rect.top,
      source: "canvas",
      startX: event.clientX,
      startY: event.clientY,
    };

    dragStateRef.current = nextDragState;
    setDragState(nextDragState);
    window.addEventListener("pointermove", handleWindowPointerMove);
    window.addEventListener("pointerup", handleWindowPointerUp);
    window.addEventListener("pointercancel", handleWindowPointerCancel);
  }

  function handleResetLayout() {
    setCanvasSnapshot((currentSnapshot) => {
      let nextSnapshot: NoteCanvasLayoutSnapshot = {};
      Object.values(currentSnapshot).forEach((layout) => {
        const point = findNextNoteCanvasPoint(nextSnapshot, canvasBounds);
        nextSnapshot = {
          ...nextSnapshot,
          [layout.itemId]: createNoteCanvasCardLayout(layout.itemId, layout.sourceBucket, point),
        };
      });
      return nextSnapshot;
    });
    showFeedback("已重置画布局。");
  }

  useEffect(() => {
    const nextSnapshot = pruneNoteCanvasLayoutSnapshot(canvasSnapshot, allItems);

    if (areSnapshotsEqual(canvasSnapshot, nextSnapshot)) {
      return;
    }

    setCanvasSnapshot(nextSnapshot);
  }, [allItems, canvasSnapshot]);

  useEffect(() => {
    setCanvasSnapshot((currentSnapshot) => {
      const nextSnapshot = Object.values(currentSnapshot).reduce<NoteCanvasLayoutSnapshot>((result, layout) => {
        result[layout.itemId] = createNoteCanvasCardLayout(
          layout.itemId,
          layout.sourceBucket,
          snapNoteCanvasPoint({ x: layout.x, y: layout.y }, canvasBounds),
        );
        return result;
      }, {});

      return areSnapshotsEqual(currentSnapshot, nextSnapshot) ? currentSnapshot : nextSnapshot;
    });
  }, [canvasBounds]);

  useEffect(() => {
    if (detailLayerState.activeBucket === expandedBucket) {
      return;
    }

    setDetailLayerState((currentState) => ({
      activeBucket: expandedBucket,
      openItemIds: currentState.openItemIds.filter((itemId) => itemMap.get(itemId)?.item.bucket === expandedBucket),
    }));
  }, [detailLayerState.activeBucket, expandedBucket, itemMap]);

  useEffect(() => {
    setDetailLayerState((currentState) => {
      const nextOpenItemIds = currentState.openItemIds.filter((itemId) => {
        const item = itemMap.get(itemId);
        return item ? item.item.bucket === currentState.activeBucket : false;
      });

      if (areItemIdListsEqual(currentState.openItemIds, nextOpenItemIds)) {
        return currentState;
      }

      return {
        activeBucket: currentState.activeBucket,
        openItemIds: nextOpenItemIds,
      };
    });
  }, [itemMap]);

  useEffect(() => {
    const hasSelection = selectionState.itemId ? itemMap.has(selectionState.itemId) : false;
    if (hasSelection) {
      return;
    }

    const fallbackItem = itemsByBucket[expandedBucket][0] ?? allItems[0] ?? null;
    setSelectionState(
      fallbackItem
        ? {
            bucket: fallbackItem.item.bucket,
            itemId: fallbackItem.item.item_id,
          }
        : DEFAULT_SELECTION_STATE,
    );
  }, [allItems, expandedBucket, itemMap, itemsByBucket, selectionState.itemId]);

  useEffect(() => {
    const bucketToScroll = scrollAfterPinBucketRef.current;
    if (!bucketToScroll) {
      return;
    }

    scrollAfterPinBucketRef.current = null;
    const nextVisibleItem = visibleItemsByBucket[bucketToScroll][0];
    if (!nextVisibleItem) {
      return;
    }

    window.requestAnimationFrame(() => {
      drawerItemRefs.current[nextVisibleItem.item.item_id]?.scrollIntoView({
        block: "nearest",
      });
    });
  }, [visibleItemsByBucket]);

  useEffect(() => {
    return () => {
      window.removeEventListener("pointermove", handleWindowPointerMove);
      window.removeEventListener("pointerup", handleWindowPointerUp);
      window.removeEventListener("pointercancel", handleWindowPointerCancel);
      dragStateRef.current = null;
      clearFeedbackTimeout();
    };
    // Drag listeners are attached and removed explicitly by the drag lifecycle.
    // The cleanup only needs to run once when the page unmounts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const drawerStatusLabel = drawerOpen ? "收起抽屉" : "展开抽屉";

  function renderQuickActions(item: NoteListItem) {
    const primaryStatusAction = buildPrimaryStatusAction(item);
    const resource = getFirstResource(item);

    return (
      <div className="note-workbench__card-actions">
        {item.item.linked_task_id ? (
          <button
            className="note-workbench__card-action"
            onClick={(event) => {
              event.stopPropagation();
              openLinkedTask(item.item.linked_task_id!);
            }}
            onPointerDown={(event) => event.stopPropagation()}
            type="button"
          >
            <ArrowUpRight className="h-4 w-4" />
            查看任务
          </button>
        ) : null}

        {resource ? (
          <button
            className="note-workbench__card-action"
            onClick={(event) => {
              event.stopPropagation();
              void openFirstResource(item);
            }}
            onPointerDown={(event) => event.stopPropagation()}
            type="button"
          >
            <ArrowUpRight className="h-4 w-4" />
            打开资源
          </button>
        ) : null}

        {primaryStatusAction ? (
          <button
            className={cn(
              "note-workbench__card-action",
              primaryStatusAction.tone === "success" && "is-success",
              primaryStatusAction.tone === "warn" && "is-warn",
            )}
            onClick={(event) => {
              event.stopPropagation();
              updateMutation.mutate({
                action: primaryStatusAction.action,
                itemId: item.item.item_id,
              });
            }}
            onPointerDown={(event) => event.stopPropagation()}
            type="button"
          >
            {item.item.bucket === "upcoming" ? <CheckCircle2 className="h-4 w-4" /> : item.item.bucket === "closed" ? <RotateCcw className="h-4 w-4" /> : <Repeat2 className="h-4 w-4" />}
            {primaryStatusAction.label}
          </button>
        ) : null}
      </div>
    );
  }

  function renderDrawerCard(item: NoteListItem) {
    const primaryResource = getFirstResource(item);

    return (
      <button
        key={item.item.item_id}
        className={cn("note-workbench__drawer-card", item.item.linked_task_id && "is-linked")}
        onClick={() => pinNoteToCanvas(item)}
        onPointerDown={(event) => startDrawerDrag(item, event)}
        ref={(element) => {
          drawerItemRefs.current[item.item.item_id] = element;
        }}
        type="button"
      >
        <div className="note-workbench__drawer-card-top">
          <span className={cn("note-workbench__status-pill", getNoteStatusBadgeClass(item.item.status))}>{item.experience.previewStatus}</span>
          <span className="note-workbench__drawer-card-time">{item.experience.timeHint}</span>
        </div>
        <h3 className="note-workbench__drawer-card-title">{item.item.title}</h3>
        <p className="note-workbench__drawer-card-copy">{describeNotePreview(item.item, item.experience)}</p>
        <div className="note-workbench__drawer-card-footer">
          <span>{item.experience.typeLabel}</span>
          <span>{primaryResource?.label ?? "等待补充资源"}</span>
        </div>
      </button>
    );
  }

  function renderCanvasCard(entry: { item: NoteListItem; layout: NoteCanvasLayoutSnapshot[string] }) {
    const { item, layout } = entry;
    const isSelected = selectionState.itemId === item.item.item_id;
    const isDragging = dragState?.itemId === item.item.item_id && dragState.source === "canvas";
    const resource = getFirstResource(item);

    return (
      <div
        key={item.item.item_id}
        className={cn(
          "note-workbench__canvas-card",
          item.item.linked_task_id && "is-linked",
          isSelected && "is-selected",
          isDragging && "is-dragging",
        )}
        onPointerDown={(event) => startCanvasDrag(item, event)}
        role="button"
        style={{
          transform: `translate3d(${layout.x}px, ${layout.y}px, 0)`,
        }}
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            handleCanvasCardClick(item);
          }
        }}
      >
        <section className="note-workbench__canvas-card-surface">
          <div className="note-workbench__canvas-card-top">
            <div>
              <p className="note-workbench__canvas-card-kicker">{getNoteBucketLabel(item.item.bucket)}</p>
              <h3 className="note-workbench__canvas-card-title">{item.item.title}</h3>
            </div>
            <span className={cn("note-workbench__status-pill", getNoteStatusBadgeClass(item.item.status))}>{item.experience.previewStatus}</span>
          </div>

          <p className="note-workbench__canvas-card-copy">{item.experience.noteText}</p>

          <div className="note-workbench__canvas-card-meta">
            <span>{item.experience.timeHint}</span>
            <span>{resource?.label ?? "暂无资源"}</span>
          </div>

          {item.item.linked_task_id ? <p className="note-workbench__canvas-card-link">已关联任务</p> : null}
          {isSelected ? renderQuickActions(item) : <p className="note-workbench__canvas-card-hint">先选中，再次点击展开详情</p>}
        </section>
      </div>
    );
  }

  function renderDetailOverlay(itemId: string) {
    const item = itemMap.get(itemId);
    const layout = canvasSnapshot[itemId];
    if (!item || !layout) {
      return null;
    }

    const overlayWidth = 360;
    const overlayHeight = 360;
    const preferredLeft = layout.x + NOTE_CANVAS_CARD_WIDTH + 18;
    const preferredTop = layout.y - 8;
    const maxLeft = Math.max(24, canvasBounds.width - overlayWidth - 24);
    const maxTop = Math.max(24, canvasBounds.height - overlayHeight - 24);
    const left = preferredLeft > maxLeft ? Math.max(24, layout.x - overlayWidth - 18) : Math.min(preferredLeft, maxLeft);
    const top = Math.min(Math.max(preferredTop, 24), maxTop);
    const primaryStatusAction = buildPrimaryStatusAction(item);
    const secondaryActions = buildSecondaryActions(item);

    return (
      <motion.section
        key={itemId}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        className="note-workbench__detail-layer"
        exit={{ opacity: 0, scale: 0.98, y: 12 }}
        initial={{ opacity: 0, scale: 0.98, y: 16 }}
        style={{ left, top }}
        transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
      >
        <header className="note-workbench__detail-header">
          <div>
            <p className="note-workbench__detail-kicker">{getNoteBucketLabel(item.item.bucket)}</p>
            <h3 className="note-workbench__detail-title">{item.item.title}</h3>
            <p className="note-workbench__detail-subtitle">{item.experience.agentSuggestion.detail}</p>
          </div>

          <div className="note-workbench__detail-header-actions">
            {item.item.linked_task_id ? (
              <button className="note-workbench__detail-primary" onClick={() => openLinkedTask(item.item.linked_task_id!)} type="button">
                <ArrowUpRight className="h-4 w-4" />
                查看任务
              </button>
            ) : null}
            <button
              className="note-workbench__detail-close"
              onClick={() =>
                setDetailLayerState((currentState) => ({
                  activeBucket: currentState.activeBucket,
                  openItemIds: currentState.openItemIds.filter((openItemId) => openItemId !== itemId),
                }))
              }
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="note-workbench__detail-scroll">
          <section className="note-workbench__detail-section">
            <div className="note-workbench__detail-section-head">
              <span className={cn("note-workbench__status-pill", getNoteStatusBadgeClass(item.item.status))}>{item.experience.detailStatus}</span>
              <span>{item.experience.timeHint}</span>
            </div>
            <p className="note-workbench__detail-copy">{item.experience.noteText}</p>
          </section>

          {item.experience.prerequisite || item.experience.repeatRule ? (
            <section className="note-workbench__detail-section">
              {item.experience.prerequisite ? <p className="note-workbench__detail-copy">前置：{item.experience.prerequisite}</p> : null}
              {item.experience.repeatRule ? <p className="note-workbench__detail-copy">规则：{item.experience.repeatRule}</p> : null}
              {item.experience.recentInstanceStatus ? <p className="note-workbench__detail-copy">最近一次：{item.experience.recentInstanceStatus}</p> : null}
            </section>
          ) : null}

          <section className="note-workbench__detail-section">
            <div className="note-workbench__detail-actions">
              {primaryStatusAction ? (
                <button
                  className="note-workbench__detail-action"
                  onClick={() => updateMutation.mutate({ action: primaryStatusAction.action, itemId: item.item.item_id })}
                  type="button"
                >
                  {item.item.bucket === "upcoming" ? <CheckCircle2 className="h-4 w-4" /> : item.item.bucket === "closed" ? <RotateCcw className="h-4 w-4" /> : <Repeat2 className="h-4 w-4" />}
                  {primaryStatusAction.label}
                </button>
              ) : null}

              {item.experience.relatedResources.length > 0 ? (
                <button className="note-workbench__detail-action" onClick={() => void openFirstResource(item)} type="button">
                  <ArrowUpRight className="h-4 w-4" />
                  打开资源
                </button>
              ) : null}

              {item.experience.canConvertToTask ? (
                <button className="note-workbench__detail-action is-strong" onClick={() => convertMutation.mutate(item.item.item_id)} type="button">
                  <Sparkles className="h-4 w-4" />
                  转成任务
                </button>
              ) : null}

              {secondaryActions.map((action) => (
                <button key={action.action} className="note-workbench__detail-action is-muted" onClick={() => handleDetailAction(item, action.action)} type="button">
                  {action.action === "cancel" || action.action === "cancel-recurring" || action.action === "delete" ? <XCircle className="h-4 w-4" /> : <RotateCcw className="h-4 w-4" />}
                  {action.label}
                </button>
              ))}
            </div>
          </section>

          <section className="note-workbench__detail-section">
            <h4 className="note-workbench__detail-section-title">相关资源</h4>
            <div className="note-workbench__resource-list">
              {item.experience.relatedResources.length > 0 ? (
                item.experience.relatedResources.map((resource) => (
                  <article key={resource.id} className="note-workbench__resource-item">
                    <div>
                      <p className="note-workbench__resource-title">{resource.label}</p>
                      <p className="note-workbench__resource-meta">{resource.type}</p>
                      <p className="note-workbench__resource-path">{resource.path ?? resource.url ?? "等待补充路径"}</p>
                    </div>
                    <button
                      className="note-workbench__resource-open"
                      onClick={() => void handleOpenPlan(resolveNoteResourceOpenExecutionPlan(resource))}
                      type="button"
                    >
                      打开
                    </button>
                  </article>
                ))
              ) : (
                <p className="note-workbench__detail-copy">当前没有挂载相关资源。</p>
              )}
            </div>
          </section>
        </div>
      </motion.section>
    );
  }

  const dragGhostItem = dragState ? itemMap.get(dragState.itemId) ?? null : null;

  return (
    <main className="dashboard-page note-workbench" style={pageStyle}>
      <header className="dashboard-page__topbar">
        <button
          className="dashboard-page__home-link"
          onClick={() => handleTopbarRouteClick(resolveDashboardRoutePath("home"))}
          onPointerDown={handleTopbarPointerDown}
          type="button"
        >
          <ArrowLeft className="h-4 w-4" />
          返回首页
        </button>

        <nav aria-label="Dashboard modules" className="dashboard-page__module-nav">
          {dashboardModules.map((item) => {
            const isActive = isTopbarRouteActive(item.path);

            return (
              <button
                key={item.route}
                aria-current={isActive ? "page" : undefined}
                className={cn("dashboard-page__module-link", isActive && "is-active")}
                onClick={() => handleTopbarRouteClick(item.path)}
                onPointerDown={handleTopbarPointerDown}
                type="button"
              >
                {item.title}
              </button>
            );
          })}
        </nav>
      </header>

      <section className="note-workbench__workspace">
        <div className="note-workbench__workspace-toolbar">
          <div className="note-workbench__workspace-status">
            <span className="note-workbench__workspace-chip">{dataMode === "rpc" ? "LIVE" : "MOCK"}</span>
            <p className="note-workbench__workspace-copy">
              画布 {canvasItems.length} 张，抽屉当前展开 {expandedBucketLabel}
            </p>
          </div>

          <div className="note-workbench__workspace-actions">
            <button className="note-workbench__toolbar-button" onClick={handleResetLayout} type="button">
              <LayoutGrid className="h-4 w-4" />
              重置布局
            </button>
            <button
              aria-pressed={drawerOpen}
              className="note-workbench__toolbar-button"
              onClick={() => setDrawerOpen((current) => !current)}
              type="button"
            >
              {drawerOpen ? <PanelLeftClose className="h-4 w-4" /> : <PanelLeftOpen className="h-4 w-4" />}
              {drawerStatusLabel}
            </button>
          </div>
        </div>

        <aside className={cn("note-workbench__drawer", !drawerOpen && "is-collapsed")} ref={drawerRef}>
          {drawerOpen ? (
            NOTE_BUCKET_CONFIGS.map((config) => {
              const visibleItems = visibleItemsByBucket[config.key];
              const totalItems = itemsByBucket[config.key];
              const isExpanded = expandedBucket === config.key;
              const isEmpty = totalItems.length === 0;
              const isPinnedOut = !isEmpty && visibleItems.length === 0;

              return (
                <section key={config.key} className={cn("note-workbench__drawer-group", isExpanded && "is-expanded")}>
                  <button
                    className="note-workbench__drawer-group-toggle"
                    onClick={() => activateBucket(config.key)}
                    type="button"
                  >
                    <div>
                      <p className="note-workbench__drawer-group-title">{config.label}</p>
                    </div>
                    <span className="note-workbench__drawer-group-count">{visibleItems.length}</span>
                  </button>

                  {isExpanded ? (
                    <div className="note-workbench__drawer-group-body">
                      {visibleItems.length > 0 ? (
                        visibleItems.map(renderDrawerCard)
                      ) : (
                        <div className="note-workbench__drawer-empty">
                          <p>{isPinnedOut ? config.pinnedDescription : config.emptyDescription}</p>
                        </div>
                      )}
                    </div>
                  ) : null}
                </section>
              );
            })
          ) : (
            <button className="note-workbench__drawer-rail" onClick={() => setDrawerOpen(true)} type="button">
              <PanelLeftOpen className="h-4 w-4" />
              <span>{NOTE_BUCKET_CONFIGS.find((config) => config.key === expandedBucket)?.label ?? "近期"}</span>
            </button>
          )}
        </aside>

        <section className="note-workbench__canvas-shell">
          <div className="note-workbench__canvas" ref={canvasRef}>
            <div className="note-workbench__canvas-scene" aria-hidden="true">
              <div className="note-workbench__canvas-field" />
              <div className="note-workbench__canvas-glow note-workbench__canvas-glow--north" />
              <div className="note-workbench__canvas-glow note-workbench__canvas-glow--south" />
            </div>

            {canvasItems.length > 0 ? canvasItems.map(renderCanvasCard) : <div className="note-workbench__canvas-empty">从左侧抽屉拖出便签，或点卡片直接放上画布。</div>}

            <AnimatePresence>{detailLayerState.openItemIds.map((itemId) => renderDetailOverlay(itemId))}</AnimatePresence>

            {dragGhostItem && dragState ? (
              <div
                className="note-workbench__drag-ghost"
                style={{
                  left: dragState.previewX,
                  top: dragState.previewY,
                }}
              >
                <div className="note-workbench__canvas-card-surface is-ghost">
                  <div className="note-workbench__canvas-card-top">
                    <div>
                      <p className="note-workbench__canvas-card-kicker">{getNoteBucketLabel(dragGhostItem.item.bucket)}</p>
                      <h3 className="note-workbench__canvas-card-title">{dragGhostItem.item.title}</h3>
                    </div>
                    <span className={cn("note-workbench__status-pill", getNoteStatusBadgeClass(dragGhostItem.item.status))}>{dragGhostItem.experience.previewStatus}</span>
                  </div>
                  <p className="note-workbench__canvas-card-copy">{dragGhostItem.experience.noteText}</p>
                </div>
              </div>
            ) : null}
          </div>
        </section>
      </section>

      <AnimatePresence>
        {pendingConfirmation || feedback || queryErrors.length > 0 ? (
          <motion.aside
            animate={{ opacity: 1, y: 0 }}
            className="note-workbench__floating-card"
            exit={{ opacity: 0, y: 12 }}
            initial={{ opacity: 0, y: 18 }}
            transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
          >
            <div className="note-workbench__floating-card-icon">
              <AlertTriangle className="h-4 w-4" />
            </div>
            <div className="note-workbench__floating-card-copy">
              <p className="note-workbench__floating-card-title">
                {pendingConfirmation ? "工作区外路径确认" : feedback ? "操作提示" : "便签同步失败"}
              </p>
              <p className="note-workbench__floating-card-text">
                {pendingConfirmation
                  ? pendingConfirmation.confirmMessage
                  : feedback ??
                    (queryErrors.length === 1
                      ? `${queryErrors[0].label}：${queryErrors[0].error instanceof Error ? queryErrors[0].error.message : "请求失败"}`
                      : `${queryErrors.length} 个分组加载失败：${queryErrors
                          .map((entry) => `${entry.label}${entry.error instanceof Error ? `（${entry.error.message}）` : ""}`)
                          .join("；")}`)}
              </p>
            </div>

            {pendingConfirmation ? (
              <div className="note-workbench__floating-card-actions">
                <button
                  className="note-workbench__floating-card-action is-strong"
                  onClick={async () => {
                    const currentConfirmation = pendingConfirmation;
                    if (!currentConfirmation) {
                      return;
                    }

                    setPendingConfirmation(null);
                    await handleOpenPlan(currentConfirmation, true);
                  }}
                  type="button"
                >
                  确认打开
                </button>
                <button
                  className="note-workbench__floating-card-action"
                  onClick={() => {
                    setPendingConfirmation(null);
                    setFeedback(null);
                  }}
                  type="button"
                >
                  取消
                </button>
              </div>
            ) : !feedback ? (
              <button
                className="note-workbench__floating-card-action"
                onClick={() => {
                  void queries[0].refetch();
                  void queries[1].refetch();
                  void queries[2].refetch();
                  void queries[3].refetch();
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
          setPendingConfirmation(null);
          setDataMode((current) => (current === "rpc" ? "mock" : "rpc"));
        }}
      />
    </main>
  );
}
