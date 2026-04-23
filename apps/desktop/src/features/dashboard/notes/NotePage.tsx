/**
 * Note dashboard page keeps nearby notes, future arrangements, and recurring
 * reminders grouped for quick conversion into formal tasks.
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { useUnmount } from "ahooks";
import type { CSSProperties, PointerEvent as ReactPointerEvent, UIEvent } from "react";
import { Link, NavLink, useNavigate } from "react-router-dom";
import { useMutation, useQueries, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, ArrowLeft, CircleDashed, NotebookPen, PanelLeftClose, PanelLeftOpen, RefreshCcw } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import type { NotepadAction } from "@cialloclaw/protocol";
import { Badge } from "@/components/ui/badge";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { buildNoteSummary, describeNotePreview, getNoteBucketLabel, getNoteStatusBadgeClass, groupClosedNotes, sortClosedNotes, sortNotesByUrgency } from "./notePage.mapper";
import { buildDashboardNoteBucketInvalidateKeys, buildDashboardNoteBucketQueryKey, dashboardNoteBucketGroups, getDashboardNoteRefreshPlan } from "./notePage.query";
import { convertNoteToTask, loadNoteBucket, performNoteResourceOpenExecution, resolveNoteResourceOpenExecutionPlan, updateNote, type NotePageDataMode } from "./notePage.service";
import type { NoteDetailAction, NoteListItem } from "./notePage.types";
import { NoteDetailPanel } from "./components/NoteDetailPanel";
import { NoteEmptyState } from "./components/NoteEmptyState";
import { NotePreviewCard } from "./components/NotePreviewCard";
import { NotePreviewSection } from "./components/NotePreviewSection";
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

const NOTE_CANVAS_CARD_WIDTH = 360;
const NOTE_CANVAS_CARD_HEIGHT = 280;
const NOTE_CANVAS_SEED_POSITIONS = [
  { x: 150, y: 110 },
  { x: 540, y: 120 },
  { x: 920, y: 140 },
  { x: 330, y: 360 },
];

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
  const [expandedBucket, setExpandedBucket] = useState<"upcoming" | "later" | "recurring_rule" | "closed" | null>("upcoming");
  const [showMoreClosed, setShowMoreClosed] = useState(false);
  const [canvasCards, setCanvasCards] = useState<NoteCanvasCard[]>([]);
  const [boardSeeded, setBoardSeeded] = useState(false);
  const [isBoardDropTarget, setIsBoardDropTarget] = useState(false);
  const [isRailDropTarget, setIsRailDropTarget] = useState(false);
  const [isCompactBoard, setIsCompactBoard] = useState<boolean>(() => (typeof window !== "undefined" ? window.matchMedia("(max-width: 720px)").matches : false));
  const [drawerDragPreview, setDrawerDragPreview] = useState<NoteDrawerDragPreview | null>(null);
  const [draggingBoardItemId, setDraggingBoardItemId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [boardLayerSize, setBoardLayerSize] = useState<{ height: number; width: number } | null>(null);
  const [dataMode, setDataMode] = useState<NotePageDataMode>(() => loadDashboardDataMode("notes") as NotePageDataMode);
  const feedbackTimeoutRef = useRef<number | null>(null);
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

  const upcomingItems = sortNotesByUrgency(upcomingQuery.data?.items ?? []);
  const laterItems = sortNotesByUrgency(laterQuery.data?.items ?? []);
  const recurringItems = sortNotesByUrgency(recurringQuery.data?.items ?? []);
  const closedItems = sortClosedNotes(closedQuery.data?.items ?? []);
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
  const selectedItem = useMemo(
    () => allItems.find((entry) => entry.item.item_id === selectedItemId) ?? upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0] ?? null,
    [allItems, closedItems, laterItems, recurringItems, selectedItemId, upcomingItems],
  );

  const pageStyle = {
    "--note-accent": "#d88e63",
    "--note-accent-strong": "#86573b",
    "--note-paper": "rgba(255, 250, 243, 0.9)",
    "--note-paper-strong": "rgba(255, 252, 247, 0.96)",
    "--note-line": "rgba(122, 92, 65, 0.18)",
    "--note-ink": "#35271b",
    "--note-copy": "rgba(72, 56, 44, 0.72)",
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

  function getNextCanvasZIndex(cards: NoteCanvasCard[]) {
    return cards.reduce((max, entry) => Math.max(max, entry.zIndex), 0) + 1;
  }

  function clampCanvasPlacement(
    placement: { x: number; y: number },
    bounds: { height: number; width: number },
    cardSize = { height: NOTE_CANVAS_CARD_HEIGHT, width: NOTE_CANVAS_CARD_WIDTH },
  ) {
    const maxX = Math.max(0, bounds.width - cardSize.width);
    const maxY = Math.max(0, bounds.height - cardSize.height);

    return {
      x: Math.min(maxX, Math.max(0, placement.x)),
      y: Math.min(maxY, Math.max(0, placement.y)),
    };
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
      await Promise.all(
        buildDashboardNoteBucketInvalidateKeys(dataMode, outcome.result.refresh_groups).map((queryKey) =>
          queryClient.invalidateQueries({
            queryKey,
          }),
        ),
      );
      showFeedback("已为这条事项生成任务，正在跳转到任务页。");
      navigate(resolveDashboardModuleRoutePath("tasks"), { state: { focusTaskId: outcome.result.task.task_id, openDetail: true } });
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : "转交给 Agent 失败，请稍后再试。";
      showFeedback(`转交给 Agent 失败：${message}`);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ action, itemId }: { action: NotepadAction; itemId: string }) => updateNote(itemId, action, dataMode),
    onSuccess: async (outcome, variables) => {
      await Promise.all(
        buildDashboardNoteBucketInvalidateKeys(dataMode, outcome.result.refresh_groups).map((queryKey) =>
          queryClient.invalidateQueries({
            queryKey,
          }),
        ),
      );

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

    const mutationAction = mapActionToMutation(action);
    if (mutationAction) {
      updateMutation.mutate({
        action: mutationAction,
        itemId: selectedItem.item.item_id,
      });
      return;
    }

    const placeholderMessage = action === "edit" ? "编辑能力稍后接入。" : "跳过本次的真实动作稍后接入。";
    showFeedback(placeholderMessage);
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
    if (plan.mode === "task_detail" && plan.taskId) {
      navigate(resolveDashboardModuleRoutePath("tasks"), { state: { focusTaskId: plan.taskId, openDetail: true } });
      showFeedback(plan.feedback);
      return;
    }

    showFeedback(await performNoteResourceOpenExecution(plan));
  }

  useEffect(() => {
    if (allItems.length === 0) {
      return;
    }

    const selectedExists = selectedItemId ? allItems.some((entry) => entry.item.item_id === selectedItemId) : false;
    if (selectedExists) {
      return;
    }

    const nextItem = upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0];
    if (nextItem) {
      setSelectedItemId(nextItem.item.item_id);
    }
  }, [allItems, closedItems, laterItems, recurringItems, selectedItemId, upcomingItems]);

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
      const next = current.filter((entry) => noteItemsById.has(entry.itemId));
      if (next.length !== current.length) {
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
      setDraggingBoardItemId(null);
      dragStateRef.current = null;
    }
  }, [defaultBoardItemIds, draggingBoardItemId, noteItemsById]);

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

  function unpinNoteFromCanvas(itemId: string) {
    setCanvasCards((current) => current.filter((entry) => entry.itemId !== itemId));
    setIsRailDropTarget(false);
  }

  function toggleBucket(bucket: "upcoming" | "later" | "recurring_rule" | "closed") {
    setExpandedBucket((current) => (current === bucket ? null : bucket));
    if (!drawerOpen) {
      setDrawerOpen(true);
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
      const droppedOverBoard = event.clientX >= boardBounds.left && event.clientX <= boardBounds.right && event.clientY >= boardBounds.top && event.clientY <= boardBounds.bottom;
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
        showFeedback("已放到画布。拖回左栏即可收回。");
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

    const deltaX = event.clientX - dragState.startClientX;
    const deltaY = event.clientY - dragState.startClientY;
    const nextX = Math.min(dragState.maxX, Math.max(dragState.minX, dragState.originX + deltaX));
    const nextY = Math.min(dragState.maxY, Math.max(dragState.minY, dragState.originY + deltaY));

    if (railRef.current) {
      const railRect = railRef.current.getBoundingClientRect();
      const overRail = event.clientX >= railRect.left && event.clientX <= railRect.right && event.clientY >= railRect.top && event.clientY <= railRect.bottom;
      setIsRailDropTarget(overRail);
    }

    if (!dragState.moved && Math.hypot(deltaX, deltaY) > 4) {
      dragState.moved = true;
    }

    setCanvasCards((current) => current.map((entry) => (entry.itemId === itemId ? { ...entry, x: nextX, y: nextY } : entry)));
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
      const droppedOverRail = event.clientX >= railRect.left && event.clientX <= railRect.right && event.clientY >= railRect.top && event.clientY <= railRect.bottom;
      if (droppedOverRail) {
        unpinNoteFromCanvas(itemId);
        showFeedback("已放回左侧抽屉。");
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
        <header className="dashboard-page__topbar">
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
        </header>

        <section className="note-preview-page__frame">
          <section className="note-preview-page__summary-shell">
            <div className="note-preview-page__summary-copy">
              <p className="note-preview-page__eyebrow">Notepad Collaboration</p>
              <div className="note-preview-page__title-row">
                <NotebookPen className="note-preview-page__title-icon" />
                <div>
                  <h1>便签</h1>
                  <p>便签协作负责整理未来安排、重复规则与尚未开始但需要记住的事情。正式进入执行后，再转交给 Agent 生成任务。</p>
                </div>
              </div>
            </div>

            <div className="note-preview-page__summary-grid">
              <div className="note-preview-page__summary-item">
                <span>今天待处理</span>
                <strong>{summary.dueToday}</strong>
              </div>
              <div className="note-preview-page__summary-item">
                <span>已逾期</span>
                <strong>{summary.overdue}</strong>
              </div>
              <div className="note-preview-page__summary-item">
                <span>重复事项今日落地</span>
                <strong>{summary.recurringToday}</strong>
              </div>
              <div className="note-preview-page__summary-item">
                <span>适合交给 Agent</span>
                <strong>{summary.readyForAgent}</strong>
              </div>
            </div>

            <div className="note-preview-page__summary-notice">
              <CircleDashed className="h-4 w-4" />
              <span>{pageNotice}</span>
            </div>
          </section>

          <section className={cn("note-preview-page__workspace", !drawerOpen && "is-drawer-collapsed")}>
            <aside className={cn("note-preview-page__rail", !drawerOpen && "is-collapsed", isRailDropTarget && "is-drop-target")} ref={railRef}>
              {drawerOpen ? (
                <>
                  <NotePreviewSection
                    activeItemId={selectedItem?.item.item_id ?? null}
                    draggableToCanvas
                    emptyLabel={upcomingQuery.isPending && !upcomingQuery.data ? "加载中" : "这组便签已全部放到画布。"}
                    isExpanded={expandedBucket === "upcoming"}
                    items={visibleUpcomingItems}
                    onCanvasDragEnd={handleDrawerCardDragEnd}
                    onCanvasDragMove={handleDrawerCardDragMove}
                    onCanvasDragStart={handleDrawerCardDragStart}
                    onSelect={openNoteDetail}
                    onToggle={() => toggleBucket("upcoming")}
                    title="近期"
                    trailing={<span className="note-preview-shell__count">{upcomingQuery.isPending && !upcomingQuery.data ? "..." : visibleUpcomingItems.length}</span>}
                  />

                  <NotePreviewSection
                    activeItemId={selectedItem?.item.item_id ?? null}
                    draggableToCanvas
                    emptyLabel={laterQuery.isPending && !laterQuery.data ? "加载中" : "这组便签已全部放到画布。"}
                    isExpanded={expandedBucket === "later"}
                    items={visibleLaterItems}
                    onCanvasDragEnd={handleDrawerCardDragEnd}
                    onCanvasDragMove={handleDrawerCardDragMove}
                    onCanvasDragStart={handleDrawerCardDragStart}
                    onSelect={openNoteDetail}
                    onToggle={() => toggleBucket("later")}
                    title="后续"
                    trailing={<span className="note-preview-shell__count">{laterQuery.isPending && !laterQuery.data ? "..." : visibleLaterItems.length}</span>}
                  />

                  <NotePreviewSection
                    activeItemId={selectedItem?.item.item_id ?? null}
                    draggableToCanvas
                    emptyLabel={recurringQuery.isPending && !recurringQuery.data ? "加载中" : "这组便签已全部放到画布。"}
                    isExpanded={expandedBucket === "recurring_rule"}
                    items={visibleRecurringItems}
                    onCanvasDragEnd={handleDrawerCardDragEnd}
                    onCanvasDragMove={handleDrawerCardDragMove}
                    onCanvasDragStart={handleDrawerCardDragStart}
                    onSelect={openNoteDetail}
                    onToggle={() => toggleBucket("recurring_rule")}
                    title="重复"
                    trailing={<span className="note-preview-shell__count">{recurringQuery.isPending && !recurringQuery.data ? "..." : visibleRecurringItems.length}</span>}
                  />

                  <article className={cn("dashboard-card note-preview-shell", expandedBucket === "closed" ? "is-expanded" : "is-collapsed")}>
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
                                <div className="note-preview-shell__list">
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
                            <div className="note-preview-shell__empty">无</div>
                          )}

                          {!showMoreClosed && hasOlderClosedItems && closedGroups.length > 0 ? <div className="note-preview-finished-groups__sentinel" aria-hidden="true" /> : null}
                        </div>
                      </div>
                    ) : null}
                  </article>
                </>
              ) : (
                <div className="note-preview-page__rail-dropzone">
                  <span className="note-preview-page__rail-dropzone-kicker">抽屉已收起</span>
                  <p className="note-preview-page__rail-dropzone-title">拖回这里</p>
                  <p className="note-preview-page__rail-dropzone-copy">把画布便签拖到这里，就能收回左侧抽屉。</p>
                </div>
              )}
            </aside>

            <button className={cn("note-preview-page__drawer-handle", !drawerOpen && "is-collapsed")} onClick={() => setDrawerOpen((current) => !current)} type="button">
              {drawerOpen ? <PanelLeftClose className="h-4 w-4" /> : <PanelLeftOpen className="h-4 w-4" />}
              <span>{drawerOpen ? "收起抽屉" : "展开抽屉"}</span>
            </button>

            <section className={cn("note-preview-page__board", isBoardDropTarget && "is-drop-target")}>
              <div aria-hidden="true" className="note-preview-page__board-scene" />
              <div className="note-preview-page__board-heading">
                <div className="note-preview-page__board-heading-copy">
                  <span className="note-preview-page__board-chip">画布</span>
                  <p>画布 {boardItems.length} 张，抽取当前展开分组与近期便签做展示。</p>
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
            setDataMode((current) => (current === "rpc" ? "mock" : "rpc"));
          }}
        />
      </>
    </main>
  );
}
