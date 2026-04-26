import { invoke } from "@tauri-apps/api/core";

export type DesktopSourceNoteDocument = {
  content: string;
  file_name: string;
  modified_at_ms: number | null;
  path: string;
  source_root: string;
  title: string;
};

export type DesktopSourceNoteIndexEntry = {
  file_name: string;
  modified_at_ms: number | null;
  path: string;
  size_bytes: number;
  source_root: string;
};

export type DesktopSourceNoteIndexSnapshot = {
  default_source_root: string | null;
  notes: DesktopSourceNoteIndexEntry[];
  source_roots: string[];
};

export type DesktopSourceNoteSnapshot = {
  default_source_root: string | null;
  notes: DesktopSourceNoteDocument[];
  source_roots: string[];
};

/**
 * Reports whether the current renderer is attached to the desktop Tauri host.
 */
export function canUseDesktopSourceNotes() {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

/**
 * Loads markdown note files from the configured task-source directories.
 *
 * @param sources Configured task-source directories from the inspector settings.
 */
export async function loadDesktopSourceNotes(sources: string[]) {
  return invoke<DesktopSourceNoteSnapshot>("desktop_load_source_notes", { sources });
}

/**
 * Loads lightweight source-note metadata so the renderer can detect external
 * file changes without rereading every markdown file body.
 *
 * @param sources Configured task-source directories from the inspector settings.
 */
export async function loadDesktopSourceNoteIndex(sources: string[]) {
  return invoke<DesktopSourceNoteIndexSnapshot>("desktop_load_source_note_index", { sources });
}

/**
 * Creates one markdown note under the first configured task-source directory.
 *
 * @param sources Configured task-source directories from the inspector settings.
 * @param content Markdown content to write into the new note file.
 */
export async function createDesktopSourceNote(sources: string[], content: string) {
  return invoke<DesktopSourceNoteDocument>("desktop_create_source_note", { content, sources });
}

/**
 * Saves markdown content back into an existing task-source note file.
 *
 * @param sources Configured task-source directories from the inspector settings.
 * @param path Existing markdown note file path.
 * @param content Latest markdown content from the note editor.
 */
export async function saveDesktopSourceNote(sources: string[], path: string, content: string) {
  return invoke<DesktopSourceNoteDocument>("desktop_save_source_note", { content, path, sources });
}
