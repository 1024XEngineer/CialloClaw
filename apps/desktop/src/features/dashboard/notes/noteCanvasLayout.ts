import type { TodoBucket } from "@cialloclaw/protocol";
import type { NoteCanvasCardLayout, NoteCanvasLayoutSnapshot, NoteListItem } from "./notePage.types";

export type NoteCanvasBounds = {
  height: number;
  width: number;
};

export type NoteCanvasPoint = {
  x: number;
  y: number;
};

export const NOTE_CANVAS_CARD_WIDTH = 284;
export const NOTE_CANVAS_CARD_HEIGHT = 196;
export const NOTE_CANVAS_GRID_STEP = 24;

const NOTE_CANVAS_SIDE_PADDING = 20;
const NOTE_CANVAS_TOP_PADDING = 96;
const NOTE_CANVAS_BOTTOM_PADDING = 20;
const NOTE_CANVAS_GAP = 20;

/**
 * Snaps a freeform canvas point to the persistent note grid and clamps the
 * card into the available board bounds.
 *
 * @param point Raw canvas point.
 * @param bounds Current canvas bounds.
 * @returns The settled grid-snapped point.
 */
export function snapNoteCanvasPoint(point: NoteCanvasPoint, bounds: NoteCanvasBounds): NoteCanvasPoint {
  const maxX = Math.max(NOTE_CANVAS_SIDE_PADDING, bounds.width - NOTE_CANVAS_CARD_WIDTH - NOTE_CANVAS_SIDE_PADDING);
  const maxY = Math.max(NOTE_CANVAS_TOP_PADDING, bounds.height - NOTE_CANVAS_CARD_HEIGHT - NOTE_CANVAS_BOTTOM_PADDING);

  return {
    x: clampAndSnap(point.x, NOTE_CANVAS_SIDE_PADDING, maxX),
    y: clampAndSnap(point.y, NOTE_CANVAS_TOP_PADDING, maxY),
  };
}

/**
 * Finds the next unoccupied slot for a note card inside the canvas grid.
 *
 * @param snapshot Current pinned note layouts.
 * @param bounds Current canvas bounds.
 * @param ignoredItemId Optional item id to ignore during overlap checks.
 * @returns The next available snapped point.
 */
export function findNextNoteCanvasPoint(
  snapshot: NoteCanvasLayoutSnapshot,
  bounds: NoteCanvasBounds,
  ignoredItemId?: string,
): NoteCanvasPoint {
  const occupiedLayouts = Object.values(snapshot).filter((entry) => entry.itemId !== ignoredItemId);
  const maxX = Math.max(NOTE_CANVAS_SIDE_PADDING, bounds.width - NOTE_CANVAS_CARD_WIDTH - NOTE_CANVAS_SIDE_PADDING);
  const maxY = Math.max(NOTE_CANVAS_TOP_PADDING, bounds.height - NOTE_CANVAS_CARD_HEIGHT - NOTE_CANVAS_BOTTOM_PADDING);

  for (let y = NOTE_CANVAS_TOP_PADDING; y <= maxY; y += NOTE_CANVAS_GRID_STEP) {
    for (let x = NOTE_CANVAS_SIDE_PADDING; x <= maxX; x += NOTE_CANVAS_GRID_STEP) {
      const candidate = { x, y };
      if (!doesNoteCanvasPointOverlap(candidate, occupiedLayouts)) {
        return candidate;
      }
    }
  }

  return snapNoteCanvasPoint({ x: NOTE_CANVAS_SIDE_PADDING, y: NOTE_CANVAS_TOP_PADDING }, bounds);
}

/**
 * Removes stale or invalid pinned-note layouts after note data refreshes.
 *
 * Notes that drifted into the `closed` bucket are automatically removed from
 * the canvas unless the user pinned them from the closed group itself.
 *
 * @param snapshot Current pinned note layouts.
 * @param items Latest note payloads.
 * @returns The pruned snapshot.
 */
export function pruneNoteCanvasLayoutSnapshot(
  snapshot: NoteCanvasLayoutSnapshot,
  items: NoteListItem[],
): NoteCanvasLayoutSnapshot {
  const itemMap = new Map(items.map((item) => [item.item.item_id, item]));
  const nextSnapshot: NoteCanvasLayoutSnapshot = {};

  Object.entries(snapshot).forEach(([itemId, layout]) => {
    const item = itemMap.get(itemId);
    if (!item) {
      return;
    }

    if (item.item.bucket === "closed" && layout.sourceBucket !== "closed") {
      return;
    }

    nextSnapshot[itemId] = {
      itemId,
      sourceBucket: normalizeSnapshotBucket(layout.sourceBucket),
      x: Math.round(layout.x),
      y: Math.round(layout.y),
    };
  });

  return nextSnapshot;
}

/**
 * Returns the pinned note ids in stable snapshot order.
 *
 * @param snapshot Current pinned note layouts.
 * @returns The pinned item ids.
 */
export function getPinnedNoteItemIds(snapshot: NoteCanvasLayoutSnapshot) {
  return Object.keys(snapshot);
}

/**
 * Creates a persisted canvas layout entry for a note card.
 *
 * @param itemId Note identifier.
 * @param sourceBucket Bucket the note was pinned from.
 * @param point Settled canvas point.
 * @returns The snapshot-ready layout entry.
 */
export function createNoteCanvasCardLayout(itemId: string, sourceBucket: TodoBucket, point: NoteCanvasPoint): NoteCanvasCardLayout {
  return {
    itemId,
    sourceBucket,
    x: Math.round(point.x),
    y: Math.round(point.y),
  };
}

function clampAndSnap(value: number, min: number, max: number) {
  const clampedValue = Math.min(Math.max(value, min), max);
  const snappedValue =
    Math.round((clampedValue - min) / NOTE_CANVAS_GRID_STEP) * NOTE_CANVAS_GRID_STEP + min;

  return Math.min(Math.max(snappedValue, min), max);
}

function doesNoteCanvasPointOverlap(point: NoteCanvasPoint, occupiedLayouts: NoteCanvasCardLayout[]) {
  return occupiedLayouts.some((layout) => {
    const separatedHorizontally =
      point.x + NOTE_CANVAS_CARD_WIDTH + NOTE_CANVAS_GAP <= layout.x ||
      layout.x + NOTE_CANVAS_CARD_WIDTH + NOTE_CANVAS_GAP <= point.x;
    const separatedVertically =
      point.y + NOTE_CANVAS_CARD_HEIGHT + NOTE_CANVAS_GAP <= layout.y ||
      layout.y + NOTE_CANVAS_CARD_HEIGHT + NOTE_CANVAS_GAP <= point.y;

    return !(separatedHorizontally || separatedVertically);
  });
}

function normalizeSnapshotBucket(bucket: TodoBucket) {
  return bucket === "closed" || bucket === "later" || bucket === "recurring_rule" ? bucket : "upcoming";
}
