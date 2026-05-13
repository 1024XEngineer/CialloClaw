/**
 * Note dashboard page keeps nearby notes, future arrangements, and recurring
 * reminders grouped for quick conversion into formal tasks.
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { useUnmount } from "ahooks";
import type { CSSProperties, PointerEvent as ReactPointerEvent, UIEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Archive, ArrowUpRight, FilePlus2, Info, PanelLeftClose, PanelLeftOpen, RefreshCcw, ScanSearch, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import type { NotepadAction, Task, TodoItem } from "@cialloclaw/protocol";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useDashboardEscapeHandler } from "@/features/dashboard/shared/dashboardEscapeCoordinator";
import { navigateToDashboardTaskDetail } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import {
  openTaskDeliveryForTask,
  performTaskOpenExecution,
  resolveTaskOpenExecutionPlan,
  shouldAutoOpenTaskDeliveryResult,
} from "@/features/dashboard/tasks/taskOutput.service";
import { openDesktopExternalUrl } from "@/platform/desktopExternalUrl";
import { cn } from "@/utils/cn";
import { buildNoteSummary, describeNotePreview, formatNoteBoardTimeHint, formatNoteDisplayPath, getNoteBucketLabel, getNoteStatusBadgeClass, groupClosedNotes, sortClosedNotes, sortNotesByUrgency } from "./notePage.mapper";
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
import { convertNoteToTask, loadNoteBucket, performNoteResourceOpenExecution, resolveNoteResourceOpenExecutionPlan, updateNote, type NotePageDataMode } from "./notePage.service";
import { isDashboardTaskDeliveryHref, navigateToDashboardTaskDelivery, readDashboardTaskDeliveryTaskId } from "../tasks/taskDeliveryNavigation";
import {
  buildSourceNoteEditorDraftFromNote,
  createEmptySourceNoteEditorDraft,
  createSourceNoteEditorDraftSignature,
  formatSourceNoteEditorContent,
  formatSourceNoteScheduleInputValue,
  parseSourceNoteEditorBlocks,
  removeSourceNoteEditorBlock,
  resolveSourceNoteDraftBucketForSchedule,
  sanitizeSourceNoteBodyText,
  serializeSourceNoteEditorDraft,
  serializeSourceNoteScheduleInputValue,
  updateSourceNoteEditorDraftContent,
  upsertSourceNoteEditorBlock,
} from "./sourceNoteEditor";
import type { NoteDetailAction, NoteListItem, NotePreviewGroupKey, SourceNoteDocument, SourceNoteEditorBlock, SourceNoteEditorDraft } from "./notePage.types";
import { NoteDetailPanel } from "./components/NoteDetailPanel";
import { NoteEmptyState } from "./components/NoteEmptyState";
import { NotePreviewCard } from "./components/NotePreviewCard";
import { NotePreviewSection } from "./components/NotePreviewSection";
import { SourceNoteStudio } from "./components/SourceNoteStudio";
import { loadStoredValue, removeStoredValue, saveStoredValue } from "@/platform/storage";
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

type PersistedNoteBoardState = {
  boardSeeded: boolean;
  canvasCards: NoteCanvasCard[];
  overdueAutoReturnedKeys: string[];
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

type SourceNotePathLookup = Map<string, SourceNoteDocument[]>;

const NOTE_CANVAS_CARD_WIDTH = 360;
const NOTE_CANVAS_CARD_HEIGHT = 280;
const NOTE_CANVAS_GRID_SIZE = 28;
const SOURCE_NOTE_INDEX_POLL_INTERVAL_MS = 2_500;
const NOTE_BOARD_STORAGE_KEY = "cialloclaw.dashboard.notes.board";
const NOTE_CANVAS_SEED_POSITIONS = [
  { x: 56, y: 48 },
  { x: 448, y: 56 },
  { x: 840, y: 84 },
  { x: 280, y: 320 },
];

function isPersistedNoteCanvasCard(value: unknown): value is NoteCanvasCard {
  if (!value || typeof value !== "object") {
    return false;
  }

  const itemId = Reflect.get(value, "itemId");
  const x = Reflect.get(value, "x");
  const y = Reflect.get(value, "y");
  const zIndex = Reflect.get(value, "zIndex");

  return typeof itemId === "string"
    && Number.isFinite(x)
    && Number.isFinite(y)
    && Number.isFinite(zIndex);
}

function loadPersistedNoteBoardState(): PersistedNoteBoardState | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    const storedState = loadStoredValue<PersistedNoteBoardState>(NOTE_BOARD_STORAGE_KEY);
    if (!storedState) {
      return null;
    }

    return {
      boardSeeded: storedState.boardSeeded === true,
      canvasCards: Array.isArray(storedState.canvasCards) ? storedState.canvasCards.filter(isPersistedNoteCanvasCard) : [],
      overdueAutoReturnedKeys: Array.isArray(storedState.overdueAutoReturnedKeys)
        ? storedState.overdueAutoReturnedKeys.filter((value): value is string => typeof value === "string" && value.length > 0)
        : [],
    };
  } catch {
    removeStoredValue(NOTE_BOARD_STORAGE_KEY);
    return null;
  }
}

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

function normalizeSourceNoteRelativeKey(value: string) {
  return normalizeSourceNoteKey(value).replace(/^\.?\//, "");
}

function normalizeSourceNoteTitleKey(value: string) {
  return value.trim().toLowerCase();
}

function normalizeSourceNoteBodyKey(value: string | null | undefined) {
  return value ? value.trim().replace(/\r\n/g, "\n").toLowerCase() : "";
}

function getNoteConvertSuccessFeedback(status: Task["status"]) {
  if (status === "waiting_auth") {
    return "已按这条便签生成任务，正在打开任务详情；后续还需要处理授权。";
  }

  return "已按这条便签生成任务，正在打开任务详情。";
}

async function openNoteConvertDelivery(taskId: string, source: NotePageDataMode, navigate: ReturnType<typeof useNavigate>) {
  const result = await openTaskDeliveryForTask(taskId, undefined, source);
  const plan = resolveTaskOpenExecutionPlan(result);
  return performTaskOpenExecution(plan, {
    onOpenTaskDelivery: ({ taskId: resolvedTaskId }) => {
      navigateToDashboardTaskDelivery(navigate, resolvedTaskId);
      return plan.feedback;
    },
    onOpenTaskDetail: ({ taskId: resolvedTaskId }) => {
      navigateToDashboardTaskDetail(navigate, resolvedTaskId);
      return plan.feedback;
    },
  });
}

function registerSourceNoteLookupKey(
  lookup: SourceNotePathLookup,
  key: string,
  note: SourceNoteDocument,
) {
  const normalizedKey = normalizeSourceNoteKey(key);
  if (normalizedKey === "") {
    return;
  }

  const existing = lookup.get(normalizedKey) ?? [];
  if (existing.some((candidate) => candidate.path === note.path)) {
    return;
  }

  lookup.set(normalizedKey, [...existing, note]);
}

function getSourceNoteLookupCandidates(lookup: SourceNotePathLookup, key: string) {
  return lookup.get(normalizeSourceNoteKey(key)) ?? [];
}

function scoreSourceNoteBlockCandidate(item: NoteListItem, block: SourceNoteEditorBlock) {
  const sourceLine = item.sourceNote?.sourceLine ?? readTodoSourceLine(item.item);
  const itemTitle = item.sourceNote?.title ?? item.item.title;
  const normalizedTitle = normalizeSourceNoteTitleKey(itemTitle);
  const normalizedNoteText = normalizeSourceNoteBodyKey(
    sanitizeSourceNoteBodyText(item.item.note_text ?? item.experience.noteText, { title: itemTitle }),
  );
  const normalizedRepeatRule = normalizeSourceNoteBodyKey(item.item.repeat_rule ?? item.experience.repeatRule);
  const normalizedRecentInstanceStatus = normalizeSourceNoteBodyKey(item.item.recent_instance_status ?? item.experience.recentInstanceStatus);
  const normalizedDueAt = (item.item.due_at ?? item.experience.plannedAt ?? "").trim();
  let score = 0;

  if (typeof sourceLine === "number" && sourceLine > 0) {
    if (block.sourceLine !== sourceLine) {
      return -1;
    }

    score += 8;
  }

  if (normalizeSourceNoteTitleKey(block.title) === normalizedTitle) {
    score += 4;
  } else if (!sourceLine) {
    return -1;
  }

  if (
    normalizedNoteText !== ""
    && normalizeSourceNoteBodyKey(sanitizeSourceNoteBodyText(block.noteText, { title: block.title || itemTitle })) === normalizedNoteText
  ) {
    score += 2;
  }

  if (normalizedRepeatRule !== "" && normalizeSourceNoteBodyKey(block.repeatRule) === normalizedRepeatRule) {
    score += 1;
  }

  if (normalizedRecentInstanceStatus !== "" && normalizeSourceNoteBodyKey(block.recentInstanceStatus) === normalizedRecentInstanceStatus) {
    score += 1;
  }

  if (normalizedDueAt !== "" && block.dueAt === normalizedDueAt) {
    score += 1;
  }

  return score;
}

/**
 * When multiple task roots expose the same relative file path, the backend can
 * still point formal note items at `workspace/<relative>`. Resolve those
 * collisions locally by matching only against note blocks from that relative
 * path instead of falling back to unrelated titles across the whole workspace.
 */
function resolveAmbiguousSourceNoteCandidate(
  item: NoteListItem,
  candidates: SourceNoteDocument[],
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  let bestMatch: SourceNoteDocument | null = null;
  let bestScore = -1;
  let tied = false;

  candidates.forEach((candidate) => {
    const blocks = sourceNoteBlocksByPath.get(candidate.path) ?? [];
    const candidateScore = blocks.reduce((maxScore, block) => Math.max(maxScore, scoreSourceNoteBlockCandidate(item, block)), -1);
    if (candidateScore < 0) {
      return;
    }

    if (candidateScore > bestScore) {
      bestMatch = candidate;
      bestScore = candidateScore;
      tied = false;
      return;
    }

    if (candidateScore === bestScore) {
      tied = true;
    }
  });

  if (!bestMatch || tied) {
    return;
  }

  return bestMatch;
}

function resolveSourceNoteLookupMatch(
  item: NoteListItem,
  lookup: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
  key: string,
) {
  const candidates = getSourceNoteLookupCandidates(lookup, key);
  if (candidates.length === 0) {
    return null;
  }

  if (candidates.length === 1) {
    return candidates[0];
  }

  return resolveAmbiguousSourceNoteCandidate(item, candidates, sourceNoteBlocksByPath) ?? null;
}

function buildSourceNotePathLookup(sourceNotes: SourceNoteDocument[]) {
  const lookup: SourceNotePathLookup = new Map();

  sourceNotes.forEach((note) => {
    registerSourceNoteLookupKey(lookup, note.path, note);

    const normalizedNotePath = normalizeSourceNoteKey(note.path);
    const normalizedSourceRoot = normalizeSourceNoteKey(note.sourceRoot);
    const rootPrefix = normalizedSourceRoot.endsWith("/") ? normalizedSourceRoot : `${normalizedSourceRoot}/`;
    if (!normalizedSourceRoot || !normalizedNotePath.startsWith(rootPrefix)) {
      return;
    }

    const relativePath = normalizeSourceNoteRelativeKey(normalizedNotePath.slice(rootPrefix.length));
    if (!relativePath) {
      return;
    }

    registerSourceNoteLookupKey(lookup, relativePath, note);
    registerSourceNoteLookupKey(lookup, `workspace/${relativePath}`, note);
  });

  return lookup;
}

function buildSourceNoteBlockKey(path: string, title: string, sourceLine?: number | null) {
  return typeof sourceLine === "number" && sourceLine > 0
    ? `${normalizeSourceNoteKey(path)}::line:${sourceLine}`
    : `${normalizeSourceNoteKey(path)}::title:${normalizeSourceNoteTitleKey(title)}`;
}

/**
 * A source note can temporarily exist as both a renderer-local fallback card
 * and a formal notepad item. Keep both the exact block key and the title alias
 * so the page can collapse them into one visible note during inspector syncs.
 */
function buildSourceNoteBlockAliases(path: string, title: string, sourceLine?: number | null) {
  const exactKey = buildSourceNoteBlockKey(path, title, sourceLine);
  const titleKey = buildSourceNoteBlockKey(path, title, null);
  return exactKey === titleKey ? [exactKey] : [exactKey, titleKey];
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
  const formalCandidateIds = candidateIds.filter((itemId) => {
    const item = noteItemsById.get(itemId);
    return item ? !item.sourceNote?.localOnly : false;
  });
  if (formalCandidateIds.length === 0) {
    return null;
  }

  if (typeof sourceIdentity.sourceLine === "number" && sourceIdentity.sourceLine > 0) {
    const exactLineCandidate = formalCandidateIds.find((itemId) => {
      const item = noteItemsById.get(itemId);
      return item ? readTodoSourceLine(item.item) === sourceIdentity.sourceLine : false;
    });
    if (exactLineCandidate) {
      return exactLineCandidate;
    }
  }

  const normalizedTitle = normalizeSourceNoteTitleKey(sourceIdentity.title);
  const exactCandidate = formalCandidateIds.find((itemId) => {
    const item = noteItemsById.get(itemId);
    return item ? normalizeSourceNoteTitleKey(item.item.title) === normalizedTitle : false;
  });
  if (exactCandidate) {
    return exactCandidate;
  }

  return formalCandidateIds.length === 1 ? formalCandidateIds[0] : null;
}

function findPreferredItemIdForSourceNote(
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

  const formalCandidateIds = candidateIds.filter((itemId) => {
    const item = noteItemsById.get(itemId);
    return item ? !item.sourceNote?.localOnly : false;
  });
  const fallbackCandidateIds = candidateIds.filter((itemId) => !formalCandidateIds.includes(itemId));
  const orderedCandidateIds = [...formalCandidateIds, ...fallbackCandidateIds];

  if (typeof sourceIdentity.sourceLine === "number" && sourceIdentity.sourceLine > 0) {
    const exactLineCandidate = orderedCandidateIds.find((itemId) => {
      const item = noteItemsById.get(itemId);
      return item ? readTodoSourceLine(item.item) === sourceIdentity.sourceLine : false;
    });
    if (exactLineCandidate) {
      return exactLineCandidate;
    }
  }

  const normalizedTitle = normalizeSourceNoteTitleKey(sourceIdentity.title);
  const exactCandidate = orderedCandidateIds.find((itemId) => {
    const item = noteItemsById.get(itemId);
    return item ? normalizeSourceNoteTitleKey(item.item.title) === normalizedTitle : false;
  });
  if (exactCandidate) {
    return exactCandidate;
  }

  return orderedCandidateIds.length === 1 ? orderedCandidateIds[0] : null;
}

function resolveNoteItemSourceNotePath(
  item: NoteListItem,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  const sourcePath = readTodoSourcePath(item.item);
  if (sourcePath) {
    return resolveSourceNoteLookupMatch(item, sourceNotesByPath, sourceNoteBlocksByPath, sourcePath)?.path ?? null;
  }

  if (item.sourceNote?.path) {
    return resolveSourceNoteLookupMatch(item, sourceNotesByPath, sourceNoteBlocksByPath, item.sourceNote.path)?.path ?? null;
  }

  const resourceMatch = item.experience.relatedResources
    .map((resource) => resolveSourceNoteLookupMatch(item, sourceNotesByPath, sourceNoteBlocksByPath, resource.path))
    .find((note): note is SourceNoteDocument => note !== null);
  if (resourceMatch) {
    return resourceMatch.path;
  }

  return null;
}

function matchesSourceNotePath(
  item: NoteListItem,
  sourceNotePath: string,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNoteBlocksByPath);
  return matchedPath !== null && normalizeSourceNoteKey(matchedPath) === normalizeSourceNoteKey(sourceNotePath);
}

function resolveSourceNoteBlockAliases(
  item: NoteListItem,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNoteBlocksByPath);
  if (!matchedPath) {
    return [];
  }

  return buildSourceNoteBlockAliases(
    matchedPath,
    item.sourceNote?.title ?? item.item.title,
    item.sourceNote?.sourceLine ?? readTodoSourceLine(item.item),
  );
}

function resolveSourceNoteBlockForItem(
  item: NoteListItem,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
): SourceNoteEditorBlock | null {
  const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNoteBlocksByPath);
  if (!matchedPath) {
    return null;
  }

  const blocks = sourceNoteBlocksByPath.get(matchedPath) ?? [];
  let bestBlock: SourceNoteEditorBlock | null = null;
  let bestScore = -1;
  let tied = false;

  blocks.forEach((block) => {
    const candidateScore = scoreSourceNoteBlockCandidate(item, block);
    if (candidateScore < 0) {
      return;
    }

    if (candidateScore > bestScore) {
      bestBlock = block;
      bestScore = candidateScore;
      tied = false;
      return;
    }

    if (candidateScore === bestScore) {
      tied = true;
    }
  });

  if (!bestBlock || tied) {
    return null;
  }

  return bestBlock;
}

function readSourceNoteRecurringEnabledOverride(block: SourceNoteEditorBlock | null) {
  if (!block) {
    return null;
  }

  const metadataValue = block.extraMetadata
    .find((entry) => normalizeSourceNoteTitleKey(entry.key) === "recurring_enabled")
    ?.value.trim()
    .toLowerCase();

  if (metadataValue === "false" || metadataValue === "0" || metadataValue === "paused") {
    return false;
  }

  if (metadataValue === "true" || metadataValue === "1") {
    return true;
  }

  const normalizedRecentStatus = normalizeSourceNoteBodyKey(block.recentInstanceStatus);
  if (normalizedRecentStatus === "paused" || normalizedRecentStatus.includes("暂停")) {
    return false;
  }

  return null;
}

function applySourceNoteDisplayOverrides(
  item: NoteListItem,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
): NoteListItem {
  if (item.sourceNote?.localOnly || item.item.bucket !== "recurring_rule") {
    return item;
  }

  const matchedBlock = resolveSourceNoteBlockForItem(item, sourceNotesByPath, sourceNoteBlocksByPath);
  const recurringEnabledOverride = readSourceNoteRecurringEnabledOverride(matchedBlock);
  if (recurringEnabledOverride !== false) {
    return item;
  }

  const recurringStatusText = matchedBlock?.recentInstanceStatus?.trim() || item.experience.recentInstanceStatus || "paused";

  return {
    ...item,
    experience: {
      ...item.experience,
      detailStatus: "重复规则已暂停",
      detailStatusTone: "warn",
      isRecurringEnabled: false,
      nextOccurrenceAt: null,
      plannedAt: null,
      previewStatus: "规则已暂停",
      recentInstanceStatus: recurringStatusText,
      summaryLabel: "重复规则已暂停",
      timeHint: "已暂停",
    },
  };
}

/**
 * A board card can temporarily stay on the canvas as a renderer-local fallback
 * while the formal bucket item for the same source block has already arrived.
 * Treat either representation as one visible note so the rail never shows a
 * duplicate beside the board.
 */
function isNoteItemRepresentedOnCanvas(
  item: NoteListItem,
  canvasItemIdSet: Set<string>,
  canvasRepresentedSourceNoteBlocks: Set<string>,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  return canvasItemIdSet.has(item.item.item_id)
    || resolveSourceNoteBlockAliases(item, sourceNotesByPath, sourceNoteBlocksByPath).some((alias) => canvasRepresentedSourceNoteBlocks.has(alias));
}

function createEmptyBucketGroups(): Record<NotePreviewGroupKey, NoteListItem[]> {
  return {
    closed: [],
    later: [],
    recurring_rule: [],
    upcoming: [],
  };
}

/**
 * Overdue upcoming notes should leave the active planning rail and appear under
 * the closed sidebar section without rewriting the formal bucket value.
 */
function resolveRailBucketForItem(item: NoteListItem, displayedBucket: NotePreviewGroupKey): NotePreviewGroupKey {
  if (displayedBucket === "upcoming" && item.item.status === "overdue") {
    return "closed";
  }

  return displayedBucket;
}

function resolveOverdueCanvasAutoReturnKeys(
  item: NoteListItem,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  const aliases = resolveSourceNoteBlockAliases(item, sourceNotesByPath, sourceNoteBlocksByPath);
  return aliases.length > 0 ? aliases : [`item:${item.item.item_id}`];
}

function findFormalReplacementItemIdForSourceNoteEntry(
  itemId: string,
  noteItemsById: Map<string, NoteListItem>,
  noteItemIdsBySourcePath: Map<string, string[]>,
  sourceNoteIdentityByItemId: Map<string, SourceNoteIdentity>,
) {
  const sourceIdentity = sourceNoteIdentityByItemId.get(itemId) ?? null;
  if (!sourceIdentity) {
    return null;
  }

  const replacementItemId = findReplacementItemIdForSourceNote(noteItemsById, noteItemIdsBySourcePath, sourceIdentity);
  return replacementItemId && replacementItemId !== itemId ? replacementItemId : null;
}

function updateRememberedFormalBucketForItem(
  rememberedBucketByAlias: Map<string, NotePreviewGroupKey>,
  item: NoteListItem,
  nextBucket: NotePreviewGroupKey | null,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
  options: {
    allowLaterReset?: boolean;
  } = {},
) {
  if (item.sourceNote?.localOnly) {
    return;
  }

  resolveSourceNoteBlockAliases(item, sourceNotesByPath, sourceNoteBlocksByPath).forEach((alias) => {
    if (!nextBucket) {
      rememberedBucketByAlias.delete(alias);
      return;
    }

    if (nextBucket === "later") {
      if (options.allowLaterReset) {
        rememberedBucketByAlias.delete(alias);
      }
      return;
    }

    rememberedBucketByAlias.set(alias, nextBucket);
  });
}

function resolveRememberedFormalBucket(
  rememberedBucketByAlias: Map<string, NotePreviewGroupKey>,
  item: NoteListItem,
  sourceNotesByPath: SourceNotePathLookup,
  sourceNoteBlocksByPath: Map<string, SourceNoteEditorBlock[]>,
) {
  if (item.sourceNote?.localOnly || item.item.bucket !== "later") {
    return null;
  }

  const rememberedAlias = resolveSourceNoteBlockAliases(item, sourceNotesByPath, sourceNoteBlocksByPath).find((alias) => {
    const bucket = rememberedBucketByAlias.get(alias);
    return bucket !== undefined && bucket !== "later";
  });
  return rememberedAlias ? rememberedBucketByAlias.get(rememberedAlias) ?? null : null;
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
  const persistedBoardStateRef = useRef<PersistedNoteBoardState | null>(loadPersistedNoteBoardState());
  const boardLayerRef = useRef<HTMLDivElement | null>(null);
  const railRef = useRef<HTMLElement | null>(null);
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(true);
  const [expandedBucket, setExpandedBucket] = useState<NotePreviewGroupKey>("upcoming");
  const [showMoreClosed, setShowMoreClosed] = useState(false);
  const [canvasCards, setCanvasCards] = useState<NoteCanvasCard[]>([]);
  const [boardSeeded, setBoardSeeded] = useState(() => persistedBoardStateRef.current?.boardSeeded ?? false);
  const [boardStateHydrated, setBoardStateHydrated] = useState(() => persistedBoardStateRef.current === null);
  const [isBoardDropTarget, setIsBoardDropTarget] = useState(false);
  const [isRailDropTarget, setIsRailDropTarget] = useState(false);
  const [activeRailDropBucket, setActiveRailDropBucket] = useState<NotePreviewGroupKey | null>(null);
  const [isCompactBoard, setIsCompactBoard] = useState<boolean>(() => (typeof window !== "undefined" ? window.matchMedia("(max-width: 720px)").matches : false));
  const [drawerDragPreview, setDrawerDragPreview] = useState<NoteDrawerDragPreview | null>(null);
  const [draggingBoardItemId, setDraggingBoardItemId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [boardLayerSize, setBoardLayerSize] = useState<{ height: number; width: number } | null>(null);
  const dataMode: NotePageDataMode = "rpc";
  const [selectedSourceNotePath, setSelectedSourceNotePath] = useState<string | null>(null);
  const [sourceNoteDraft, setSourceNoteDraft] = useState<SourceNoteEditorDraft>(() => createEmptySourceNoteEditorDraft());
  const [sourceNoteBaseline, setSourceNoteBaseline] = useState(() => createSourceNoteEditorDraftSignature(createEmptySourceNoteEditorDraft()));
  const [sourceNoteEditorContent, setSourceNoteEditorContent] = useState(() => formatSourceNoteEditorContent(createEmptySourceNoteEditorDraft()));
  const [sourceNoteEditorBaselineContent, setSourceNoteEditorBaselineContent] = useState(() => formatSourceNoteEditorContent(createEmptySourceNoteEditorDraft()));
  const [sourceNoteBaselineContent, setSourceNoteBaselineContent] = useState("");
  const [sourceEditorItemId, setSourceEditorItemId] = useState<string | null>(null);
  const [sourceNoteSyncMessage, setSourceNoteSyncMessage] = useState<string | null>(null);
  const [isCreatingSourceNote, setIsCreatingSourceNote] = useState(false);
  const [sourceStudioOpen, setSourceStudioOpen] = useState(false);
  const [isSavingSourceNote, setIsSavingSourceNote] = useState(false);
  const [noteScheduleEditing, setNoteScheduleEditing] = useState(false);
  const [noteResourcePickerOpen, setNoteResourcePickerOpen] = useState(false);
  const [noteScheduleDueAt, setNoteScheduleDueAt] = useState("");
  const [noteScheduleRepeatRule, setNoteScheduleRepeatRule] = useState("");
  const [isSavingNoteSchedule, setIsSavingNoteSchedule] = useState(false);
  const [isRunningInspection, setIsRunningInspection] = useState(false);
  const [overdueCanvasAutoReturnedKeysVersion, setOverdueCanvasAutoReturnedKeysVersion] = useState(0);
  const feedbackTimeoutRef = useRef<number | null>(null);
  const rememberedFormalBucketByAliasRef = useRef(new Map<string, NotePreviewGroupKey>());
  const overdueCanvasAutoReturnedKeysRef = useRef(new Set<string>());
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

  useDashboardEscapeHandler({
    enabled: sourceStudioOpen,
    handleEscape: () => setSourceStudioOpen(false),
    priority: 240,
  });

  useDashboardEscapeHandler({
    enabled: detailOpen,
    handleEscape: () => setDetailOpen(false),
    priority: 220,
  });

  const noteRefreshPlan = getDashboardNoteRefreshPlan(dataMode);
  const desktopSourceNotesAvailable = useMemo(() => areDesktopSourceNotesAvailable(), []);

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

  const sourceNotesData = sourceNotesQuery.data?.notes;
  const sourceNoteIndexData = sourceNoteIndexQuery.data?.notes;
  const sourceRootsData = sourceNotesQuery.data?.sourceRoots;
  const noteBucketsResolved = !upcomingQuery.isPending && !laterQuery.isPending && !recurringQuery.isPending && !closedQuery.isPending;
  const sourceNotes = useMemo(() => sourceNotesData ?? [], [sourceNotesData]);
  const resolvedSourceRoots = useMemo(() => sourceRootsData ?? taskSourceRoots, [sourceRootsData, taskSourceRoots]);
  const sourceNotesByPath = useMemo(() => buildSourceNotePathLookup(sourceNotes), [sourceNotes]);
  const sourceNoteBlocksByPath = useMemo(
    () => new Map(sourceNotes.map((note) => [note.path, parseSourceNoteEditorBlocks(note)])),
    [sourceNotes],
  );
  const rawRpcItems = useMemo(
    () => [
      ...(upcomingQuery.data?.items ?? []),
      ...(laterQuery.data?.items ?? []),
      ...(recurringQuery.data?.items ?? []),
      ...(closedQuery.data?.items ?? []),
    ],
    [closedQuery.data?.items, laterQuery.data?.items, recurringQuery.data?.items, upcomingQuery.data?.items],
  );
  useEffect(() => {
    rawRpcItems.forEach((item) => {
      updateRememberedFormalBucketForItem(
        rememberedFormalBucketByAliasRef.current,
        item,
        item.item.bucket,
        sourceNotesByPath,
        sourceNoteBlocksByPath,
      );
    });
  }, [rawRpcItems, sourceNoteBlocksByPath, sourceNotesByPath]);
  useEffect(() => {
    if (boardStateHydrated || !boardLayerSize || !noteBucketsResolved) {
      return;
    }

    const persistedBoardState = persistedBoardStateRef.current;
    if (!persistedBoardState) {
      setBoardStateHydrated(true);
      return;
    }

    overdueCanvasAutoReturnedKeysRef.current = new Set(persistedBoardState.overdueAutoReturnedKeys);
    setCanvasCards(
      persistedBoardState.canvasCards.map((entry) => ({
        ...entry,
        ...clampCanvasPlacement({ x: entry.x, y: entry.y }, boardLayerSize),
      })),
    );
    setBoardSeeded(persistedBoardState.boardSeeded);
    setBoardStateHydrated(true);
  }, [boardLayerSize, boardStateHydrated, noteBucketsResolved]);
  useEffect(() => {
    if (!boardStateHydrated) {
      return;
    }

    const activeOverdueKeys = new Set<string>();

    rawRpcItems.forEach((item) => {
      const displayedBucket = resolveRememberedFormalBucket(
        rememberedFormalBucketByAliasRef.current,
        item,
        sourceNotesByPath,
        sourceNoteBlocksByPath,
      ) ?? item.item.bucket;
      const railBucket = resolveRailBucketForItem(item, displayedBucket);
      if (railBucket !== displayedBucket) {
        resolveOverdueCanvasAutoReturnKeys(item, sourceNotesByPath, sourceNoteBlocksByPath).forEach((key) => activeOverdueKeys.add(key));
      }
    });
    replaceOverdueCanvasAutoReturnedKeys(
      [...overdueCanvasAutoReturnedKeysRef.current].filter((key) => activeOverdueKeys.has(key)),
    );
  }, [boardStateHydrated, rawRpcItems, sourceNoteBlocksByPath, sourceNotesByPath]);
  const displayRpcItems = useMemo(
    () => rawRpcItems.map((item) => applySourceNoteDisplayOverrides(item, sourceNotesByPath, sourceNoteBlocksByPath)),
    [rawRpcItems, sourceNoteBlocksByPath, sourceNotesByPath],
  );
  const rpcItemsByBucket = useMemo(() => {
    const nextGroups = createEmptyBucketGroups();

    displayRpcItems
      .forEach((item) => {
        const displayedBucket = resolveRememberedFormalBucket(
          rememberedFormalBucketByAliasRef.current,
          item,
          sourceNotesByPath,
          sourceNoteBlocksByPath,
        ) ?? item.item.bucket;
        nextGroups[resolveRailBucketForItem(item, displayedBucket)].push(item);
      });

    nextGroups.upcoming = sortNotesByUrgency(nextGroups.upcoming);
    nextGroups.later = sortNotesByUrgency(nextGroups.later);
    nextGroups.recurring_rule = sortNotesByUrgency(nextGroups.recurring_rule);
    nextGroups.closed = sortClosedNotes(nextGroups.closed);

    return nextGroups;
  }, [displayRpcItems, sourceNoteBlocksByPath, sourceNotesByPath]);
  const rpcUpcomingItems = rpcItemsByBucket.upcoming;
  const rpcLaterItems = rpcItemsByBucket.later;
  const rpcRecurringItems = rpcItemsByBucket.recurring_rule;
  const rpcClosedItems = rpcItemsByBucket.closed;
  const upcomingItems = rpcUpcomingItems;
  const laterItems = rpcLaterItems;
  const recurringItems = rpcRecurringItems;
  const closedItems = rpcClosedItems;
  const formalUpcomingItems = rpcUpcomingItems;
  const formalLaterItems = rpcLaterItems;
  const formalRecurringItems = rpcRecurringItems;
  const formalClosedItems = rpcClosedItems;
  const allItems = useMemo(() => [...upcomingItems, ...laterItems, ...recurringItems, ...closedItems], [upcomingItems, laterItems, recurringItems, closedItems]);
  const noteItemsById = useMemo(() => new Map(allItems.map((item) => [item.item.item_id, item])), [allItems]);
  const canvasItemIdSet = useMemo(() => new Set(canvasCards.map((entry) => entry.itemId)), [canvasCards]);
  const canvasRepresentedSourceNoteBlocks = useMemo(() => {
    const representedBlocks = new Set<string>();

    canvasCards.forEach((entry) => {
      const item = noteItemsById.get(entry.itemId);
      if (!item) {
        return;
      }

      resolveSourceNoteBlockAliases(item, sourceNotesByPath, sourceNoteBlocksByPath).forEach((alias) => {
        representedBlocks.add(alias);
      });
    });

    return representedBlocks;
  }, [canvasCards, noteItemsById, sourceNoteBlocksByPath, sourceNotesByPath]);
  const visibleUpcomingItems = useMemo(
    () =>
      upcomingItems.filter(
        (item) => !isNoteItemRepresentedOnCanvas(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath),
      ),
    [canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNoteBlocksByPath, sourceNotesByPath, upcomingItems],
  );
  const visibleLaterItems = useMemo(
    () =>
      laterItems.filter(
        (item) => !isNoteItemRepresentedOnCanvas(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath),
      ),
    [canvasItemIdSet, canvasRepresentedSourceNoteBlocks, laterItems, sourceNoteBlocksByPath, sourceNotesByPath],
  );
  const visibleRecurringItems = useMemo(
    () =>
      recurringItems.filter(
        (item) => !isNoteItemRepresentedOnCanvas(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath),
      ),
    [canvasItemIdSet, canvasRepresentedSourceNoteBlocks, recurringItems, sourceNoteBlocksByPath, sourceNotesByPath],
  );
  const visibleClosedItems = useMemo(
    () =>
      closedItems.filter(
        (item) => !isNoteItemRepresentedOnCanvas(item, canvasItemIdSet, canvasRepresentedSourceNoteBlocks, sourceNotesByPath, sourceNoteBlocksByPath),
      ),
    [canvasItemIdSet, canvasRepresentedSourceNoteBlocks, closedItems, sourceNoteBlocksByPath, sourceNotesByPath],
  );
  /**
   * Keep renderer-local source note cards on the canvas only. The sidebar
   * should list formal note items instead of duplicating local markdown
   * fallback cards beside the board.
   */
  const railUpcomingItems = useMemo(() => visibleUpcomingItems.filter((item) => !item.sourceNote?.localOnly), [visibleUpcomingItems]);
  const railLaterItems = useMemo(() => visibleLaterItems.filter((item) => !item.sourceNote?.localOnly), [visibleLaterItems]);
  const railRecurringItems = useMemo(() => visibleRecurringItems.filter((item) => !item.sourceNote?.localOnly), [visibleRecurringItems]);
  const railClosedItems = useMemo(() => visibleClosedItems.filter((item) => !item.sourceNote?.localOnly), [visibleClosedItems]);
  const preferredUpcomingItem = useMemo(() => railUpcomingItems[0] ?? formalUpcomingItems[0] ?? null, [formalUpcomingItems, railUpcomingItems]);
  const preferredLaterItem = useMemo(() => railLaterItems[0] ?? formalLaterItems[0] ?? null, [formalLaterItems, railLaterItems]);
  const preferredRecurringItem = useMemo(() => railRecurringItems[0] ?? formalRecurringItems[0] ?? null, [formalRecurringItems, railRecurringItems]);
  const preferredClosedItem = useMemo(() => railClosedItems[0] ?? formalClosedItems[0] ?? null, [formalClosedItems, railClosedItems]);
  const closedGroups = useMemo(() => groupClosedNotes(railClosedItems, showMoreClosed), [railClosedItems, showMoreClosed]);
  const hasOlderClosedItems = useMemo(() => {
    const now = Date.now();

    return railClosedItems.some((item) => {
      const endedAt = item.experience.endedAt ? new Date(item.experience.endedAt).getTime() : now;
      const diffDays = (now - endedAt) / (1000 * 60 * 60 * 24);
      return diffDays > 7;
    });
  }, [railClosedItems]);
  const summary = useMemo(
    () => buildNoteSummary({ recurring_rule: formalRecurringItems, upcoming: formalUpcomingItems }),
    [formalRecurringItems, formalUpcomingItems],
  );
  const noteItemSourcePathById = useMemo(() => {
    const itemPathMap = new Map<string, string>();

    allItems.forEach((item) => {
      const matchedPath = resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNoteBlocksByPath);
      if (matchedPath) {
        itemPathMap.set(item.item.item_id, matchedPath);
      }
    });

    return itemPathMap;
  }, [allItems, sourceNoteBlocksByPath, sourceNotesByPath]);
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
    () =>
      allItems.find((entry) => entry.item.item_id === selectedItemId)
      ?? preferredUpcomingItem
      ?? preferredLaterItem
      ?? preferredRecurringItem
      ?? preferredClosedItem
      ?? null,
    [allItems, preferredClosedItem, preferredLaterItem, preferredRecurringItem, preferredUpcomingItem, selectedItemId],
  );
  const canScheduleSelectedItem = selectedItem !== null && selectedItem.item.bucket !== "closed";
  const scheduleActionLabel = useMemo(() => {
    if (!selectedItem) {
      return "安排时间";
    }

    if (selectedItem.experience.repeatRule) {
      return "修改时间/规则";
    }

    if (selectedItem.experience.plannedAt) {
      return "修改时间";
    }

    return "安排时间";
  }, [selectedItem]);
  const sourceStudioItem = useMemo(
    () => (sourceEditorItemId ? noteItemsById.get(sourceEditorItemId) ?? null : null),
    [noteItemsById, sourceEditorItemId],
  );
  const primarySourceNote = useMemo(() => sourceNotes[0] ?? null, [sourceNotes]);
  const selectedSourceNote = useMemo(
    () => sourceNotes.find((note) => note.path === selectedSourceNotePath) ?? primarySourceNote,
    [primarySourceNote, selectedSourceNotePath, sourceNotes],
  );
  const sourceEditorDirty = sourceNoteEditorContent !== sourceNoteEditorBaselineContent
    || createSourceNoteEditorDraftSignature(sourceNoteDraft) !== sourceNoteBaseline;
  const sourceNoteIndexFingerprint = useMemo(
    () => (sourceNoteIndexData ?? []).map((note) => `${note.path}:${note.modifiedAtMs ?? 0}:${note.sizeBytes}`).join("|"),
    [sourceNoteIndexData],
  );
  const sourceNoteAvailabilityMessage = useMemo(() => {
    if (dataMode !== "rpc") {
      return "Mock 模式下不会读写真实 markdown 便签。";
    }

    if (!desktopSourceNotesAvailable) {
      return "当前运行环境不支持桌面端 markdown 便签桥接。";
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

  async function syncCreatedSourceNoteToBoard(
    savedNote: SourceNoteDocument,
    sourceIdentity: PendingCreatedSourceNote,
  ) {
    const latestSourceNotesResult = await sourceNotesQuery.refetch();
    const latestSourceNotes = latestSourceNotesResult.data?.notes ?? sourceNotes;
    const latestSourceNotesByPath = buildSourceNotePathLookup(latestSourceNotes);
    const latestSourceNoteBlocksByPath = new Map(latestSourceNotes.map((note) => [note.path, parseSourceNoteEditorBlocks(note)]));
    const normalizedExpectedPath = normalizeSourceNoteKey(sourceIdentity.path);
    const normalizedExpectedTitle = normalizeSourceNoteTitleKey(sourceIdentity.title);
    const refetchedItems = await refetchAllNoteBuckets();
    const matchedItem = refetchedItems.find((item) => {
      if (item.sourceNote?.localOnly) {
        return false;
      }

      const matchedPath = resolveNoteItemSourceNotePath(item, latestSourceNotesByPath, latestSourceNoteBlocksByPath);
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
          && matchesSourceNotePath(item, savedNote.path, latestSourceNotesByPath, latestSourceNoteBlocksByPath),
      );

    if (!matchedItem) {
      showFeedback("新便签已保存并完成巡检，但暂时还没有识别成可执行事项。");
      return;
    }

    pendingCreatedSourceNoteRef.current = null;
    setDrawerOpen(true);
    setExpandedBucket(matchedItem.item.bucket);
    setSelectedItemId(matchedItem.item.item_id);
    pinNoteToCanvasRef.current(matchedItem.item.item_id);
    showFeedback("新便签已同步到便签页，并放到了网格里。");
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
    const nextEditorContent = formatSourceNoteEditorContent(nextDraft);
    setSourceNoteDraft(nextDraft);
    setSourceNoteBaseline(createSourceNoteEditorDraftSignature(nextDraft));
    setSourceNoteEditorContent(nextEditorContent);
    setSourceNoteEditorBaselineContent(nextEditorContent);
    setSourceNoteBaselineContent(fileContent);
  }

  function handleSourceNoteEditorChange(content: string) {
    setSourceNoteEditorContent(content);
    setSourceNoteDraft((currentDraft) => updateSourceNoteEditorDraftContent(currentDraft, content));
  }

  function startCreatingSourceNote() {
    const nextDraft = createEmptySourceNoteEditorDraft(primarySourceNote?.path ?? null);

    setIsCreatingSourceNote(true);
    setSourceEditorItemId(null);
    setSelectedSourceNotePath(primarySourceNote?.path ?? null);
    applySourceNoteDraft(nextDraft, primarySourceNote?.content ?? "");
    setSourceNoteSyncMessage(
      primarySourceNote?.path
        ? `开始新便签后，点击“保存便签”会追加到 ${formatNoteDisplayPath(primarySourceNote.path)}`
        : resolvedSourceRoots[0]
          ? `开始新便签后，点击“保存便签”会追加到 ${formatNoteDisplayPath(resolvedSourceRoots[0])} 下的主 markdown 便签文件`
          : "开始新便签后，点击“保存便签”会追加到第一个任务来源目录下的主 markdown 便签文件。",
    );
  }

  function openCreateSourceNoteStudio() {
    startCreatingSourceNote();
    setDetailOpen(false);
    setSourceStudioOpen(true);
  }

  function resolveSourceNotePathForItem(item: NoteListItem) {
    return resolveNoteItemSourceNotePath(item, sourceNotesByPath, sourceNoteBlocksByPath);
  }

  function resolveSourceNoteDocumentForItem(item: NoteListItem) {
    const matchedPath = resolveSourceNotePathForItem(item) ?? item.sourceNote?.path ?? null;
    if (matchedPath) {
      return getSourceNoteLookupCandidates(sourceNotesByPath, matchedPath)[0] ?? null;
    }

    return sourceNotes.length === 1 ? sourceNotes[0] ?? null : null;
  }

  function resolveSourceNoteEditorContextForItem(item: NoteListItem) {
    const matchedNote = resolveSourceNoteDocumentForItem(item);
    if (!matchedNote) {
      return null;
    }

    return {
      draft: buildSourceNoteEditorDraftFromNote(matchedNote, item),
      note: matchedNote,
    };
  }

  /**
   * Formal note actions can change more than just the visible bucket. Keep the
   * markdown source block aligned with the returned formal item while preserving
   * hidden metadata that only exists in the source file.
   */
  function buildSourceNoteDraftFromFormalItem(
    context: {
      draft: SourceNoteEditorDraft;
      note: SourceNoteDocument;
    },
    nextItem: TodoItem,
  ): SourceNoteEditorDraft {
    const nextNoteText = nextItem.note_text === null || nextItem.note_text === undefined
      ? context.draft.noteText
      : sanitizeSourceNoteBodyText(nextItem.note_text, { title: nextItem.title });
    const nextExtraMetadata = context.draft.extraMetadata.filter(
      (entry) => normalizeSourceNoteTitleKey(entry.key) !== "recurring_enabled",
    );

    if (nextItem.bucket === "recurring_rule" && nextItem.recurring_enabled === false) {
      nextExtraMetadata.push({
        key: "recurring_enabled",
        value: "false",
      });
    }

    return {
      ...context.draft,
      agentSuggestion: nextItem.agent_suggestion?.trim() ?? "",
      bucket: nextItem.bucket,
      checked: nextItem.status === "completed",
      dueAt: nextItem.due_at?.trim() ?? "",
      effectiveScope: nextItem.effective_scope?.trim() ?? "",
      endedAt: nextItem.ended_at?.trim() ?? "",
      extraMetadata: nextExtraMetadata,
      nextOccurrenceAt: nextItem.next_occurrence_at?.trim() ?? "",
      noteText: nextNoteText,
      prerequisite: nextItem.prerequisite?.trim() ?? "",
      recentInstanceStatus:
        nextItem.recent_instance_status?.trim()
        ?? (nextItem.bucket === "recurring_rule" && nextItem.recurring_enabled === false ? "paused" : ""),
      repeatRule: nextItem.repeat_rule?.trim() ?? "",
      sourceLine: context.draft.sourceLine,
      sourcePath: context.draft.sourcePath ?? context.note.path,
      title: nextItem.title.trim() || context.draft.title,
    };
  }

  async function persistSourceNoteMutationForItem(
    item: NoteListItem,
    nextItem: TodoItem | null,
    deletedItemId: string | null,
  ) {
    if (sourceNoteAvailabilityMessage !== null || taskSourceRoots.length === 0) {
      return false;
    }

    const context = resolveSourceNoteEditorContextForItem(item);
    if (!context) {
      return false;
    }

    if (!nextItem && deletedItemId === item.item.item_id) {
      const nextSourceFile = removeSourceNoteEditorBlock(context.note, context.draft);
      if (!nextSourceFile.removed) {
        return false;
      }
      skipNextSourceNoteRefreshRef.current = true;
      await saveNoteSource(taskSourceRoots, context.note.path, nextSourceFile.content);
      await Promise.all([sourceNotesQuery.refetch(), sourceNoteIndexQuery.refetch()]);
      return true;
    }

    if (!nextItem) {
      return false;
    }

    const nextDraft = buildSourceNoteDraftFromFormalItem(context, nextItem);
    const nextSourceFile = upsertSourceNoteEditorBlock(context.note, nextDraft);
    skipNextSourceNoteRefreshRef.current = true;
    await saveNoteSource(taskSourceRoots, context.note.path, nextSourceFile.content);
    await Promise.all([sourceNotesQuery.refetch(), sourceNoteIndexQuery.refetch()]);
    return true;
  }

  function openSourceStudioForItem(item: NoteListItem) {
    const matchedNote = resolveSourceNoteDocumentForItem(item) ?? primarySourceNote;

    if (matchedNote) {
      setIsCreatingSourceNote(false);
      setSourceEditorItemId(item.item.item_id);
      setSelectedSourceNotePath(matchedNote.path);
      applySourceNoteDraft(buildSourceNoteEditorDraftFromNote(matchedNote, item), matchedNote.content);
      setSourceNoteSyncMessage("正在编辑当前便签内容，系统会保留既有 markdown 元数据。");
      setDetailOpen(false);
      setSourceStudioOpen(true);
      if (sourceNoteAvailabilityMessage) {
        showFeedback(sourceNoteAvailabilityMessage);
      }
      return;
    }

    openCreateSourceNoteStudio();
    showFeedback(sourceNoteAvailabilityMessage ?? "还没有主 markdown 便签文件，先为你打开空白便签。");
  }

  function startScheduleEditingForItem(item: NoteListItem) {
    if (sourceNoteAvailabilityMessage !== null) {
      showFeedback(sourceNoteAvailabilityMessage);
      return;
    }

    const context = resolveSourceNoteEditorContextForItem(item);
    if (!context) {
      showFeedback("还没定位到这条便签对应的 markdown 源块，请先执行一次巡检。");
      return;
    }

    const plannedAt = context.draft.dueAt || context.draft.nextOccurrenceAt;
    setNoteScheduleDueAt(formatSourceNoteScheduleInputValue(plannedAt));
    setNoteScheduleRepeatRule(context.draft.repeatRule);
    setNoteScheduleEditing(true);
  }

  async function handleSaveSourceNote() {
    if (sourceNoteAvailabilityMessage !== null || taskSourceRoots.length === 0) {
      const message = sourceNoteAvailabilityMessage ?? "请先配置任务来源目录。";
      setSourceNoteSyncMessage(message);
      showFeedback(message);
      return;
    }

    if (isSavingSourceNote || sourceNoteEditorContent.trim() === "") {
      return;
    }

    setIsSavingSourceNote(true);
    try {
      const createdSourceNote = isCreatingSourceNote;
      const sourceNoteTarget = selectedSourceNote ?? primarySourceNote;
      const nextDraft = updateSourceNoteEditorDraftContent(sourceNoteDraft, sourceNoteEditorContent);
      const { blockContent, normalizedDraft } = serializeSourceNoteEditorDraft(nextDraft);
      let savedNote: SourceNoteDocument;
      let resolvedSourceLine: number | null = null;
      let createdSourceNoteIdentity: PendingCreatedSourceNote | null = null;

      if (createdSourceNote || !sourceNoteTarget) {
        savedNote = await createNoteSource(taskSourceRoots, blockContent);
        const createdBlocks = parseSourceNoteEditorBlocks(savedNote);
        const createdBlock = createdBlocks[createdBlocks.length - 1] ?? null;
        resolvedSourceLine = createdBlock?.sourceLine ?? null;
        createdSourceNoteIdentity = {
          path: savedNote.path,
          sourceLine: resolvedSourceLine,
          title: normalizedDraft.title,
        };
      } else {
        const nextSourceFile = upsertSourceNoteEditorBlock(sourceNoteTarget, nextDraft);
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
        await syncCreatedSourceNoteToBoard(savedNote, createdSourceNoteIdentity);
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "markdown 便签保存失败。";
      setSourceNoteSyncMessage(message);
      showFeedback(message);
    } finally {
      setIsSavingSourceNote(false);
    }
  }

  async function handleSaveNoteSchedule() {
    if (!selectedItem) {
      return;
    }

    if (sourceNoteAvailabilityMessage !== null || taskSourceRoots.length === 0) {
      showFeedback(sourceNoteAvailabilityMessage ?? "请先配置任务来源目录。");
      return;
    }

    const context = resolveSourceNoteEditorContextForItem(selectedItem);
    if (!context) {
      showFeedback("还没定位到这条便签对应的 markdown 源块，请先执行一次巡检。");
      return;
    }

    const nextDueAt = serializeSourceNoteScheduleInputValue(noteScheduleDueAt);
    const nextRepeatRule = noteScheduleRepeatRule.trim();
    if (nextRepeatRule !== "" && nextDueAt === "") {
      showFeedback("设置重复规则前请先填写首次时间。");
      return;
    }

    setIsSavingNoteSchedule(true);
    try {
      const scheduleChanged = nextDueAt !== context.draft.dueAt.trim() || nextRepeatRule !== context.draft.repeatRule.trim();
      const nextDraft: SourceNoteEditorDraft = {
        ...context.draft,
        bucket: resolveSourceNoteDraftBucketForSchedule({
          dueAt: nextDueAt,
          repeatRule: nextRepeatRule,
        }),
        dueAt: nextDueAt,
        endedAt: "",
        nextOccurrenceAt: scheduleChanged ? "" : context.draft.nextOccurrenceAt,
        recentInstanceStatus: scheduleChanged ? "" : context.draft.recentInstanceStatus,
        repeatRule: nextRepeatRule,
      };
      const nextSourceFile = upsertSourceNoteEditorBlock(context.note, nextDraft);
      skipNextSourceNoteRefreshRef.current = true;
      await saveNoteSource(taskSourceRoots, context.note.path, nextSourceFile.content);
      await Promise.all([sourceNotesQuery.refetch(), sourceNoteIndexQuery.refetch()]);
      setNoteScheduleEditing(false);
      setSourceNoteSyncMessage(nextRepeatRule !== "" ? "重复规则已保存，正在同步巡检结果。" : nextDueAt !== "" ? "计划时间已保存，正在同步巡检结果。" : "已清除时间安排，正在同步巡检结果。");
      await refreshInspection(
        "notes_schedule_saved",
        nextRepeatRule !== "" ? "已更新重复规则" : nextDueAt !== "" ? "已安排时间" : "已清除时间安排",
      );
    } catch (error) {
      showFeedback(error instanceof Error ? error.message : "便签时间安排保存失败。");
    } finally {
      setIsSavingNoteSchedule(false);
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
      if (shouldAutoOpenTaskDeliveryResult(outcome.result.delivery_result)) {
        try {
          showFeedback(await openNoteConvertDelivery(outcome.result.task.task_id, dataMode, navigate));
          return;
        } catch (error) {
          showFeedback(error instanceof Error ? `结果已生成，但打开交付失败：${error.message}` : "结果已生成，但打开交付失败，请稍后再试。");
          navigateToDashboardTaskDetail(navigate, outcome.result.task.task_id);
          return;
        }
      }

      showFeedback(outcome.result.bubble_message?.text ?? getNoteConvertSuccessFeedback(outcome.result.task.status));
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
      const updatedBucket = outcome.result.notepad_item?.bucket ?? null;
      const updatedItem = noteItemsById.get(variables.itemId);
      let sourceNoteSyncError: string | null = null;
      if (updatedItem) {
        updateRememberedFormalBucketForItem(
          rememberedFormalBucketByAliasRef.current,
          updatedItem,
          updatedBucket,
          sourceNotesByPath,
          sourceNoteBlocksByPath,
          { allowLaterReset: true },
        );

        try {
          await persistSourceNoteMutationForItem(
            updatedItem,
            outcome.result.notepad_item,
            outcome.result.deleted_item_id ?? null,
          );
        } catch (error) {
          sourceNoteSyncError = error instanceof Error ? error.message : "请稍后再试。";
        }
      }

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

      showFeedback(
        sourceNoteSyncError
          ? `${feedbackByAction[variables.action]} 但 markdown 同步失败：${sourceNoteSyncError}`
          : feedbackByAction[variables.action],
      );
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

  function openLinkedTaskDetail(taskId: string) {
    navigateToDashboardTaskDetail(navigate, taskId);
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

    if (action === "open-linked-task") {
      if (!selectedItem.item.linked_task_id) {
        showFeedback("当前还没有关联任务。");
        return;
      }

      showFeedback("正在打开关联任务详情。");
      openLinkedTaskDetail(selectedItem.item.linked_task_id);
      return;
    }

    if (action === "open-resource") {
      const firstResource = selectedItem.experience.relatedResources[0];
      if (!firstResource) {
        showFeedback("当前没有可打开的相关资料。");
        return;
      }

      if (selectedItem.experience.relatedResources.length > 1) {
        setNoteResourcePickerOpen(true);
        return;
      }

      void handleResourceOpen(firstResource.id);
      return;
    }

    if (action === "edit") {
      if (selectedItem.item.bucket === "recurring_rule") {
        startScheduleEditingForItem(selectedItem);
        return;
      }

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

    /* Legacy placeholder kept commented out after edit now opens source notes.
    const placeholderMessage =
      false
        ? sourceNoteAvailabilityMessage ?? "请在上方 markdown 便签区编辑源文件。"
        : "跳过本次真实动作，后续再接入。";
    showFeedback(placeholderMessage);
    */
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

    setNoteResourcePickerOpen(false);
    const plan = resolveNoteResourceOpenExecutionPlan(resource);
    showFeedback(await performNoteResourceOpenExecution(plan, {
      onOpenTaskDetail: ({ taskId }) => {
        openLinkedTaskDetail(taskId);
        return plan.feedback;
      },
      onOpenResultPage: ({ taskId, url }) => {
        const deliveryTaskId = taskId ?? readDashboardTaskDeliveryTaskId(url);
        if (deliveryTaskId && isDashboardTaskDeliveryHref(url)) {
          navigateToDashboardTaskDelivery(navigate, deliveryTaskId);
          return plan.feedback;
        }

        return openDesktopExternalUrl(url)
          .then(() => plan.feedback)
          .catch((error: unknown) => {
            const detail = error instanceof Error ? error.message.trim() : "";
            return detail ? `无法通过系统浏览器打开便签结果页链接（${detail}）` : "无法通过系统浏览器打开便签结果页链接";
          });
      },
    }));
  }

  useEffect(() => {
    sourceNoteIndexFingerprintRef.current = null;
  }, [dataMode, taskSourceRoots]);

  useEffect(() => {
    if (!detailOpen) {
      setNoteScheduleEditing(false);
      setNoteResourcePickerOpen(false);
    }
  }, [detailOpen]);

  useEffect(() => {
    setNoteScheduleEditing(false);
  }, [selectedItemId]);

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
        const emptyDraft = createEmptySourceNoteEditorDraft();
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

    setSourceNoteSyncMessage("检测到任务来源 markdown 发生变化，正在同步巡检结果。");
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

    if (selectedItemId) {
      const selectedItem = noteItemsById.get(selectedItemId);
      const replacementItemId = findFormalReplacementItemIdForSourceNoteEntry(
        selectedItemId,
        noteItemsById,
        noteItemIdsBySourcePath,
        noteSourceIdentityByItemIdRef.current,
      );
      if (replacementItemId && (selectedItem?.sourceNote?.localOnly || !selectedItem)) {
        setSelectedItemId(replacementItemId);
        return;
      }
    }

    const selectedExists = selectedItemId ? allItems.some((entry) => entry.item.item_id === selectedItemId) : false;
    if (selectedExists) {
      return;
    }

    const nextItem = preferredUpcomingItem ?? preferredLaterItem ?? preferredRecurringItem ?? preferredClosedItem;
    if (nextItem) {
      setSelectedItemId(nextItem.item.item_id);
    }
  }, [allItems, noteItemIdsBySourcePath, noteItemsById, preferredClosedItem, preferredLaterItem, preferredRecurringItem, preferredUpcomingItem, selectedItemId]);

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
    { label: "markdown 便签", error: sourceNotesQuery.error },
    { label: "markdown 便签索引", error: sourceNoteIndexQuery.error },
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
    append(preferredUpcomingItem);
    append(preferredLaterItem);
    append(preferredRecurringItem);
    append(preferredClosedItem);

    return picked.slice(0, NOTE_CANVAS_SEED_POSITIONS.length).map((item) => item.item.item_id);
  }, [preferredClosedItem, preferredLaterItem, preferredRecurringItem, preferredUpcomingItem, selectedItem]);

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
    if (!boardStateHydrated || boardSeeded || defaultBoardItemIds.length === 0 || !boardLayerSize) {
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
  }, [boardLayerSize, boardSeeded, boardStateHydrated, defaultBoardItemIds]);

  useEffect(() => {
    // Keep the canvas purely local to this page. Once a card is placed, detail
    // toggles and bucket changes must not reshuffle the board order.
    const boardBounds = getBoardLayerBounds();
    setCanvasCards((current) => {
      let changed = false;
      let removedForRailBucket = false;
      const seenItemIds = new Set<string>();
      const next: NoteCanvasCard[] = [];

      current.forEach((entry) => {
        const currentItem = noteItemsById.get(entry.itemId);
        const replacementItemId = findFormalReplacementItemIdForSourceNoteEntry(
          entry.itemId,
          noteItemsById,
          noteItemIdsBySourcePath,
          noteSourceIdentityByItemIdRef.current,
        );

        if (replacementItemId && (currentItem?.sourceNote?.localOnly || !currentItem) && !seenItemIds.has(replacementItemId)) {
          changed = true;
          seenItemIds.add(replacementItemId);
          next.push({ ...entry, itemId: replacementItemId });
          return;
        }

        if (currentItem) {
          const displayedBucket = resolveRememberedFormalBucket(
            rememberedFormalBucketByAliasRef.current,
            currentItem,
            sourceNotesByPath,
            sourceNoteBlocksByPath,
          ) ?? currentItem.item.bucket;
          const railBucket = resolveRailBucketForItem(currentItem, displayedBucket);
          const autoReturnKeys = resolveOverdueCanvasAutoReturnKeys(currentItem, sourceNotesByPath, sourceNoteBlocksByPath);
          if (railBucket !== displayedBucket && autoReturnKeys.some((key) => !overdueCanvasAutoReturnedKeysRef.current.has(key))) {
            changed = true;
            removedForRailBucket = true;
            addOverdueCanvasAutoReturnedKeys(autoReturnKeys);
            return;
          }

          seenItemIds.add(entry.itemId);
          next.push(entry);
          return;
        }

        changed = true;
      });

      if (changed) {
        if (!removedForRailBucket && next.length === 0 && current.length > 0 && defaultBoardItemIds.length > 0 && boardBounds) {
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

    if (draggingBoardItemId) {
      const draggingItem = noteItemsById.get(draggingBoardItemId);
      const replacementItemId = findFormalReplacementItemIdForSourceNoteEntry(
        draggingBoardItemId,
        noteItemsById,
        noteItemIdsBySourcePath,
        noteSourceIdentityByItemIdRef.current,
      );
      if (replacementItemId && (draggingItem?.sourceNote?.localOnly || !draggingItem)) {
        if (dragStateRef.current?.itemId === draggingBoardItemId) {
          dragStateRef.current = { ...dragStateRef.current, itemId: replacementItemId };
        }
        setDraggingBoardItemId(replacementItemId);
      } else if (!draggingItem) {
        setDraggingBoardItemId(null);
        dragStateRef.current = null;
      }
    }
  }, [defaultBoardItemIds, draggingBoardItemId, noteItemIdsBySourcePath, noteItemsById, sourceNoteBlocksByPath, sourceNotesByPath]);

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
  useEffect(() => {
    if (!boardStateHydrated) {
      return;
    }

    if (!boardSeeded && canvasCards.length === 0 && overdueCanvasAutoReturnedKeysRef.current.size === 0) {
      removeStoredValue(NOTE_BOARD_STORAGE_KEY);
      return;
    }

    saveStoredValue<PersistedNoteBoardState>(NOTE_BOARD_STORAGE_KEY, {
      boardSeeded,
      canvasCards,
      overdueAutoReturnedKeys: [...overdueCanvasAutoReturnedKeysRef.current],
    });
  }, [boardSeeded, boardStateHydrated, canvasCards, overdueCanvasAutoReturnedKeysVersion]);

  function openNoteDetail(itemId: string) {
    setSelectedItemId(itemId);
    setDetailOpen(true);
  }

  function pinNoteToCanvas(itemId: string, placement?: { x: number; y: number }) {
    setCanvasCards((current) => {
      if (current.some((entry) => entry.itemId === itemId)) {
        return current;
      }

      const targetItem = noteItemsById.get(itemId);
      if (targetItem) {
        const displayedBucket = resolveRememberedFormalBucket(
          rememberedFormalBucketByAliasRef.current,
          targetItem,
          sourceNotesByPath,
          sourceNoteBlocksByPath,
        ) ?? targetItem.item.bucket;
        const railBucket = resolveRailBucketForItem(targetItem, displayedBucket);
        if (railBucket !== displayedBucket) {
          addOverdueCanvasAutoReturnedKeys(resolveOverdueCanvasAutoReturnKeys(targetItem, sourceNotesByPath, sourceNoteBlocksByPath));
        }
      }

      const targetAliases = targetItem ? resolveSourceNoteBlockAliases(targetItem, sourceNotesByPath, sourceNoteBlocksByPath) : [];
      if (targetAliases.length > 0) {
        const replacementIndex = current.findIndex((entry) => {
          const currentItem = noteItemsById.get(entry.itemId);
          if (!currentItem) {
            return false;
          }

          return resolveSourceNoteBlockAliases(currentItem, sourceNotesByPath, sourceNoteBlocksByPath).some((alias) => targetAliases.includes(alias));
        });

        if (replacementIndex >= 0) {
          const next = [...current];
          next[replacementIndex] = { ...next[replacementIndex], itemId };
          return next;
        }
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

  function replaceOverdueCanvasAutoReturnedKeys(keys: Iterable<string>) {
    const nextKeys = new Set(keys);
    const currentKeys = overdueCanvasAutoReturnedKeysRef.current;
    if (currentKeys.size === nextKeys.size && [...nextKeys].every((key) => currentKeys.has(key))) {
      return;
    }

    overdueCanvasAutoReturnedKeysRef.current = nextKeys;
    setOverdueCanvasAutoReturnedKeysVersion((current) => current + 1);
  }

  function addOverdueCanvasAutoReturnedKeys(keys: Iterable<string>) {
    const nextKeys = new Set(overdueCanvasAutoReturnedKeysRef.current);
    let changed = false;

    Array.from(keys).forEach((key) => {
      if (!nextKeys.has(key)) {
        nextKeys.add(key);
        changed = true;
      }
    });

    if (!changed) {
      return;
    }

    overdueCanvasAutoReturnedKeysRef.current = nextKeys;
  }

  useEffect(() => {
    const pendingSourceNote = pendingCreatedSourceNoteRef.current;
    if (!pendingSourceNote) {
      return;
    }

    const replacementItemId = findReplacementItemIdForSourceNote(
      noteItemsById,
      noteItemIdsBySourcePath,
      pendingSourceNote,
    );
    const nextItemId = replacementItemId ?? findPreferredItemIdForSourceNote(
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
    if (nextItem.sourceNote?.localOnly) {
      showFeedback("新便签已放到网格里，正在同步正式分组。");
      return;
    }
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
    const updatedItem = noteItemsById.get(itemId);
    let sourceNoteSyncError: string | null = null;
    if (updatedItem) {
      updateRememberedFormalBucketForItem(
        rememberedFormalBucketByAliasRef.current,
        updatedItem,
        outcome.result.notepad_item?.bucket ?? null,
        sourceNotesByPath,
        sourceNoteBlocksByPath,
        { allowLaterReset: true },
      );

      try {
        await persistSourceNoteMutationForItem(
          updatedItem,
          outcome.result.notepad_item,
          outcome.result.deleted_item_id ?? null,
        );
      } catch (error) {
        sourceNoteSyncError = error instanceof Error ? error.message : "请稍后再试。";
      }
    }
    await invalidateNoteBuckets(outcome.result.refresh_groups);
    return {
      notepadItem: outcome.result.notepad_item,
      sourceBucketSyncError: sourceNoteSyncError,
    };
  }

  function appendSourceBucketSyncFailure(message: string, sourceBucketSyncError: string | null) {
    return sourceBucketSyncError ? `${message} 但 markdown 分组回写失败：${sourceBucketSyncError}` : message;
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
        const moveOutcome = await runNoteUpdateForRailDrop(item.item.item_id, "move_upcoming");
        if (drawerOpen) {
          setExpandedBucket("upcoming");
        }
        presentRailFeedback(appendSourceBucketSyncFailure("已放进近期分组，并同步更新便签状态。", moveOutcome.sourceBucketSyncError));
        return;
      }

      if (sourceBucket === "recurring_rule" && targetBucket === "closed") {
        const cancelRecurringOutcome = await runNoteUpdateForRailDrop(item.item.item_id, "cancel_recurring");
        if (drawerOpen) {
          setExpandedBucket("closed");
        }
        presentRailFeedback(appendSourceBucketSyncFailure("已放进已结束分组，并结束这条重复规则。", cancelRecurringOutcome.sourceBucketSyncError));
        return;
      }

      if (sourceBucket === "closed") {
        const restoreOutcome = await runNoteUpdateForRailDrop(item.item.item_id, "restore");
        const restoredBucket = restoreOutcome.notepadItem?.bucket ?? sourceBucket;

        if (restoredBucket === targetBucket) {
          if (drawerOpen) {
            setExpandedBucket(restoredBucket);
          }
          presentRailFeedback(appendSourceBucketSyncFailure(`已恢复到${targetLabel}分组。`, restoreOutcome.sourceBucketSyncError));
          return;
        }

        if (restoredBucket === "later" && targetBucket === "upcoming") {
          const moveOutcome = await runNoteUpdateForRailDrop(item.item.item_id, "move_upcoming");
          if (drawerOpen) {
            setExpandedBucket("upcoming");
          }
          presentRailFeedback(appendSourceBucketSyncFailure("已恢复并提前到近期分组。", moveOutcome.sourceBucketSyncError));
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
    const boardCardCopy = item.experience.noteText.trim();
    const hasBoardCardCopy = boardCardCopy !== "";

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
            <h3
              className={cn(
                "note-preview-page__board-card-title",
                !hasBoardCardCopy && "note-preview-page__board-card-title--spacious",
              )}
            >
              {item.item.title}
            </h3>
          </div>
          <Badge className={cn("border-0 px-3 py-1 text-[0.72rem] ring-1", getNoteStatusBadgeClass(item.item.status))}>{item.experience.previewStatus}</Badge>
        </div>

        {hasBoardCardCopy ? <p className="note-preview-page__board-card-copy">{boardCardCopy}</p> : null}

        <div className="note-preview-page__board-card-footer">
          <span>{formatNoteBoardTimeHint(item.item, item.experience)}</span>
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
          <div aria-hidden="true" className="dashboard-page__topbar-spacer note-preview-page__page-nav" />
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
                      items={railUpcomingItems}
                      onCanvasDragEnd={handleDrawerCardDragEnd}
                      onCanvasDragMove={handleDrawerCardDragMove}
                      onCanvasDragStart={handleDrawerCardDragStart}
                      onSelect={openNoteDetail}
                      onToggle={() => toggleBucket("upcoming")}
                      stackCards
                      title="近期"
                      trailing={<span className="note-preview-shell__count">{upcomingQuery.isPending && !upcomingQuery.data ? "..." : railUpcomingItems.length}</span>}
                    />

                    <NotePreviewSection
                      activeItemId={selectedItem?.item.item_id ?? null}
                      bucketKey="later"
                      draggableToCanvas
                      emptyLabel={laterQuery.isPending && !laterQuery.data ? "加载中" : "这组便签已全部放到网格。"}
                      isDropTarget={activeRailDropBucket === "later"}
                      isExpanded={expandedBucket === "later"}
                      items={railLaterItems}
                      onCanvasDragEnd={handleDrawerCardDragEnd}
                      onCanvasDragMove={handleDrawerCardDragMove}
                      onCanvasDragStart={handleDrawerCardDragStart}
                      onSelect={openNoteDetail}
                      onToggle={() => toggleBucket("later")}
                      stackCards
                      title="后续"
                      trailing={<span className="note-preview-shell__count">{laterQuery.isPending && !laterQuery.data ? "..." : railLaterItems.length}</span>}
                    />

                    <NotePreviewSection
                      activeItemId={selectedItem?.item.item_id ?? null}
                      bucketKey="recurring_rule"
                      draggableToCanvas
                      emptyLabel={recurringQuery.isPending && !recurringQuery.data ? "加载中" : "这组便签已全部放到网格。"}
                      isDropTarget={activeRailDropBucket === "recurring_rule"}
                      isExpanded={expandedBucket === "recurring_rule"}
                      items={railRecurringItems}
                      onCanvasDragEnd={handleDrawerCardDragEnd}
                      onCanvasDragMove={handleDrawerCardDragMove}
                      onCanvasDragStart={handleDrawerCardDragStart}
                      onSelect={openNoteDetail}
                      onToggle={() => toggleBucket("recurring_rule")}
                      stackCards
                      title="重复"
                      trailing={<span className="note-preview-shell__count">{recurringQuery.isPending && !recurringQuery.data ? "..." : railRecurringItems.length}</span>}
                    />

                    <article className={cn("dashboard-card note-preview-shell", activeRailDropBucket === "closed" && "is-drop-target", expandedBucket === "closed" ? "is-expanded" : "is-collapsed")}>
                      <button aria-expanded={expandedBucket === "closed"} className="note-preview-shell__bucket-toggle" onClick={() => toggleBucket("closed")} type="button">
                        <span className="note-preview-shell__bucket-title">
                          <Archive className="note-preview-shell__bucket-title-icon" />
                          <p className="dashboard-card__kicker">已结束</p>
                        </span>
                        <span className="note-preview-shell__count">{closedQuery.isPending && !closedQuery.data ? "..." : railClosedItems.length}</span>
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
                                    {group.items.map((entry, index) => (
                                      <NotePreviewCard
                                        draggableToCanvas
                                        key={entry.item.item_id}
                                        isActive={entry.item.item_id === selectedItem?.item.item_id}
                                        item={entry}
                                        onCanvasDragEnd={handleDrawerCardDragEnd}
                                        onCanvasDragMove={handleDrawerCardDragMove}
                                        onCanvasDragStart={handleDrawerCardDragStart}
                                        onSelect={openNoteDetail}
                                        stackOrder={group.items.length > 1 ? index + 1 : undefined}
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
                  <NoteDetailPanel
                    feedback={feedback}
                    item={selectedItem}
                    onAction={handleDetailAction}
                    onClose={() => setDetailOpen(false)}
                    onOpenLinkedTask={selectedItem.item.linked_task_id ? () => {
                      showFeedback("正在打开关联任务详情。");
                      openLinkedTaskDetail(selectedItem.item.linked_task_id!);
                    } : undefined}
                    onOpenResource={(resourceId) => {
                      void handleResourceOpen(resourceId);
                    }}
                    onToggleRecurring={selectedItem.item.bucket === "recurring_rule" ? () => handleDetailAction("toggle-recurring") : undefined}
                    scheduleActionLabel={scheduleActionLabel}
                    scheduleDisabledReason={sourceNoteAvailabilityMessage}
                    scheduleDueAt={noteScheduleDueAt}
                    scheduleEditing={noteScheduleEditing}
                    scheduleRepeatRule={noteScheduleRepeatRule}
                    isSavingSchedule={isSavingNoteSchedule}
                    onCancelScheduleEdit={() => setNoteScheduleEditing(false)}
                    onResetSchedule={() => {
                      setNoteScheduleDueAt("");
                      setNoteScheduleRepeatRule("");
                    }}
                    onSaveSchedule={() => void handleSaveNoteSchedule()}
                    onScheduleDueAtChange={setNoteScheduleDueAt}
                    onScheduleRepeatRuleChange={setNoteScheduleRepeatRule}
                    onStartScheduleEdit={canScheduleSelectedItem ? () => startScheduleEditingForItem(selectedItem) : undefined}
                  />
                </motion.div>
              </>
            ) : null}
        </AnimatePresence>

        <AnimatePresence>
          {noteResourcePickerOpen && selectedItem ? (
            <>
              <motion.button
                animate={{ opacity: 1 }}
                className="note-detail-modal__backdrop note-resource-modal__backdrop"
                exit={{ opacity: 0 }}
                initial={{ opacity: 0 }}
                onClick={() => setNoteResourcePickerOpen(false)}
                type="button"
              />
              <motion.div
                animate={{ opacity: 1, scale: 1, y: 0 }}
                className="note-detail-modal note-detail-modal--resource"
                exit={{ opacity: 0, scale: 0.98, y: 20 }}
                initial={{ opacity: 0, scale: 0.98, y: 16 }}
                transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
              >
                <section className="note-schedule-modal note-resource-modal">
                  <div className="note-schedule-modal__header">
                    <div>
                      <p className="note-preview-page__eyebrow">Related Resources</p>
                      <h2 className="note-schedule-modal__title">选择相关资料</h2>
                      <p className="note-schedule-modal__subtitle">{selectedItem.item.title}</p>
                    </div>
                    <Button className="note-schedule-modal__close" onClick={() => setNoteResourcePickerOpen(false)} size="icon-sm" type="button" variant="ghost">
                      <X className="h-4 w-4" />
                      <span className="sr-only">关闭相关资料列表</span>
                    </Button>
                  </div>

                  <div className="note-schedule-modal__body note-resource-modal__body">
                    <p className="note-schedule-modal__hint">这条便签关联了多份资料，选择其中一份继续打开。</p>
                    <div className="note-detail-resource-list">
                      {selectedItem.experience.relatedResources.map((resource) => (
                        <button
                          key={resource.id}
                          className="note-detail-resource-item note-detail-resource-item--button"
                          onClick={() => {
                            void handleResourceOpen(resource.id);
                          }}
                          type="button"
                        >
                          <ArrowUpRight className="h-4 w-4" />
                          <div>
                            <p className="note-detail-resource-item__title">{resource.label}</p>
                            <p className="note-detail-resource-item__meta">{resource.type}</p>
                            <p className="note-detail-resource-item__path">{resource.url ?? formatNoteDisplayPath(resource.path)}</p>
                          </div>
                        </button>
                      ))}
                    </div>
                  </div>
                </section>
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
                        <h2 className="note-source-modal__title">{isCreatingSourceNote ? "新建便签" : "编辑便签"}</h2>
                      </div>
                      <Button className="note-source-modal__close" onClick={() => setSourceStudioOpen(false)} size="icon-sm" type="button" variant="ghost">
                        <X className="h-4 w-4" />
                        <span className="sr-only">关闭便签编辑器</span>
                      </Button>
                    </div>

                    <SourceNoteStudio
                      availabilityMessage={sourceNoteAvailabilityMessage}
                      draft={sourceNoteDraft}
                      editorContent={sourceNoteEditorContent}
                      editingItem={sourceStudioItem}
                      isCreating={isCreatingSourceNote}
                      isDirty={sourceEditorDirty}
                      isInspecting={isRunningInspection}
                      isLoading={sourceNotesLoading}
                      isSaving={isSavingSourceNote}
                      onChange={handleSourceNoteEditorChange}
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
                data-tone={feedback ? "success" : "warn"}
                exit={{ opacity: 0, y: 12 }}
                initial={{ opacity: 0, y: 16 }}
                transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
              >
                <div className="note-preview-page__floating-card-icon" data-tone={feedback ? "success" : "warn"}>
                  {feedback ? <Info className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
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

      </>
    </main>
  );
}
