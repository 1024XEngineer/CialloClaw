import { loadStoredValue, saveStoredValue } from "@/platform/storage";
import type { NoteCanvasLayoutSnapshot, NoteDrawerPreferenceSnapshot } from "./notePage.types";

const NOTE_CANVAS_LAYOUT_STORAGE_KEY = "dashboard.notes.canvas-layout";
const NOTE_DRAWER_PREFERENCES_STORAGE_KEY = "dashboard.notes.drawer-preferences";

/**
 * Loads the persisted pinned-note canvas layout snapshot.
 *
 * @returns The stored snapshot, or an empty record when unavailable.
 */
export function loadNoteCanvasLayoutSnapshot(): NoteCanvasLayoutSnapshot {
  if (typeof window === "undefined" || !("localStorage" in window)) {
    return {};
  }

  return loadStoredValue<NoteCanvasLayoutSnapshot>(NOTE_CANVAS_LAYOUT_STORAGE_KEY) ?? {};
}

/**
 * Persists the pinned-note canvas layout snapshot.
 *
 * @param snapshot Snapshot to persist.
 */
export function saveNoteCanvasLayoutSnapshot(snapshot: NoteCanvasLayoutSnapshot) {
  if (typeof window === "undefined" || !("localStorage" in window)) {
    return;
  }

  saveStoredValue(NOTE_CANVAS_LAYOUT_STORAGE_KEY, snapshot);
}

/**
 * Loads the persisted drawer visibility and expanded-bucket preferences.
 *
 * @returns The stored drawer preferences, or `null` when unavailable.
 */
export function loadNoteDrawerPreferenceSnapshot(): NoteDrawerPreferenceSnapshot | null {
  if (typeof window === "undefined" || !("localStorage" in window)) {
    return null;
  }

  return loadStoredValue<NoteDrawerPreferenceSnapshot>(NOTE_DRAWER_PREFERENCES_STORAGE_KEY);
}

/**
 * Persists the drawer visibility and expanded-bucket preferences.
 *
 * @param snapshot Preferences to persist.
 */
export function saveNoteDrawerPreferenceSnapshot(snapshot: NoteDrawerPreferenceSnapshot) {
  if (typeof window === "undefined" || !("localStorage" in window)) {
    return;
  }

  saveStoredValue(NOTE_DRAWER_PREFERENCES_STORAGE_KEY, snapshot);
}
