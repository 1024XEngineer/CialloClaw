import type {
  AgentTaskInspectorConfigGetResult,
  AgentTaskInspectorRunResult,
  RequestMeta,
} from "@cialloclaw/protocol";
import {
  canUseDesktopSourceNotes,
  createDesktopSourceNote,
  loadDesktopSourceNoteIndex,
  loadDesktopSourceNotes,
  saveDesktopSourceNote,
  type DesktopSourceNoteDocument,
  type DesktopSourceNoteIndexEntry,
  type DesktopSourceNoteIndexSnapshot,
  type DesktopSourceNoteSnapshot,
} from "@/platform/desktopSourceNotes";
import { syncDesktopSettingsSnapshot } from "@/platform/desktopSettingsSnapshot";
import { isRpcChannelUnavailable } from "@/rpc/fallback";
import { getTaskInspectorConfig, runTaskInspector } from "@/rpc/methods";
import {
  hydrateDesktopRuntimeDefaults,
  loadSettings,
  toProtocolSettingsSnapshot,
} from "@/services/settingsService";
import type {
  SourceNoteDocument,
  SourceNoteIndexEntry,
  SourceNoteIndexSnapshot,
  SourceNoteSnapshot,
} from "./notePage.types";

const NOTE_SOURCE_TIMEOUT_MS = 10_000;
const LEGACY_TASK_SOURCE_PLACEHOLDERS = new Set(["workspace/todos"]);

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
}

function mapSourceNoteDocument(document: DesktopSourceNoteDocument): SourceNoteDocument {
  return {
    content: document.content,
    fileName: document.file_name,
    modifiedAtMs: document.modified_at_ms,
    path: document.path,
    sourceRoot: document.source_root,
    title: document.title,
  };
}

function mapSourceNoteIndexEntry(entry: DesktopSourceNoteIndexEntry): SourceNoteIndexEntry {
  return {
    fileName: entry.file_name,
    modifiedAtMs: entry.modified_at_ms,
    path: entry.path,
    sizeBytes: entry.size_bytes,
    sourceRoot: entry.source_root,
  };
}

function mapSourceNoteIndexSnapshot(snapshot: DesktopSourceNoteIndexSnapshot): SourceNoteIndexSnapshot {
  return {
    defaultSourceRoot: snapshot.default_source_root,
    notes: snapshot.notes.map(mapSourceNoteIndexEntry),
    sourceRoots: snapshot.source_roots,
  };
}

function mapSourceNoteSnapshot(snapshot: DesktopSourceNoteSnapshot): SourceNoteSnapshot {
  return {
    defaultSourceRoot: snapshot.default_source_root,
    notes: snapshot.notes.map(mapSourceNoteDocument),
    sourceRoots: snapshot.source_roots,
  };
}

function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${label}请求超时`)), NOTE_SOURCE_TIMEOUT_MS);
    }),
  ]);
}

function normalizeSourceEntry(source: string) {
  return source.trim().replaceAll("\\", "/").toLowerCase();
}

function isAbsoluteLocalTaskSource(source: string) {
  const trimmed = source.trim();
  return /^(?:[a-z]:[\\/]|\\\\|\/)/i.test(trimmed);
}

function isMissingTaskSourceConfigError(error: unknown) {
  if (!(error instanceof Error)) {
    return false;
  }

  const normalizedMessage = error.message.trim().toLowerCase();
  return normalizedMessage.includes("task inspection source not found")
    || normalizedMessage.includes("inspection_source_not_found");
}

function buildCachedTaskInspectorConfig(): AgentTaskInspectorConfigGetResult {
  const taskAutomation = loadSettings().settings.task_automation;
  return {
    task_sources: taskAutomation.task_sources,
    inspection_interval: taskAutomation.inspection_interval,
    inspect_on_file_change: taskAutomation.inspect_on_file_change,
    inspect_on_startup: taskAutomation.inspect_on_startup,
    remind_before_deadline: taskAutomation.remind_before_deadline,
    remind_when_stale: taskAutomation.remind_when_stale,
  };
}

function resolvePreferredTaskSources(remoteTaskSources: string[]) {
  const cachedTaskSources = loadSettings().settings.task_automation.task_sources
    .map((source) => source.trim())
    .filter((source) => source.length > 0);

  if (cachedTaskSources.length === 0 || !cachedTaskSources.every(isAbsoluteLocalTaskSource)) {
    return remoteTaskSources;
  }

  if (remoteTaskSources.length === 0) {
    return cachedTaskSources;
  }

  const usesLegacyPlaceholderOnly = remoteTaskSources.every((source) =>
    LEGACY_TASK_SOURCE_PLACEHOLDERS.has(normalizeSourceEntry(source)),
  );

  return usesLegacyPlaceholderOnly ? cachedTaskSources : remoteTaskSources;
}

async function syncSourceNoteSettingsSnapshot(taskSources: string[]) {
  const currentSettings = loadSettings();

  await syncDesktopSettingsSnapshot(
    toProtocolSettingsSnapshot({
      ...currentSettings.settings,
      task_automation: {
        ...currentSettings.settings.task_automation,
        task_sources: taskSources,
      },
    }),
  );
}

/**
 * Reports whether the renderer can use the desktop markdown-note bridge.
 */
export function areDesktopSourceNotesAvailable() {
  return canUseDesktopSourceNotes();
}

/**
 * Loads the current task-source configuration used by the note inspector.
 */
export async function loadNoteSourceConfig(): Promise<AgentTaskInspectorConfigGetResult> {
  await hydrateDesktopRuntimeDefaults();
  try {
    const config = await withTimeout(
      getTaskInspectorConfig({ request_meta: createRequestMeta("note_source_config") }),
      "任务来源配置加载",
    );
    const resolvedTaskSources = resolvePreferredTaskSources(config.task_sources);
    await syncSourceNoteSettingsSnapshot(resolvedTaskSources);

    return {
      ...config,
      task_sources: resolvedTaskSources,
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("当前无法读取任务来源配置，请稍后重试。");
    }

    if (isMissingTaskSourceConfigError(error)) {
      const cachedConfig = buildCachedTaskInspectorConfig();
      const resolvedTaskSources = resolvePreferredTaskSources(cachedConfig.task_sources);
      await syncSourceNoteSettingsSnapshot(resolvedTaskSources);
      return {
        ...cachedConfig,
        task_sources: resolvedTaskSources,
      };
    }

    throw error;
  }
}

/**
 * Loads the latest markdown note snapshot from the configured task-source roots.
 *
 * @param taskSources Current task-source directory list.
 */
export async function loadNoteSourceSnapshot(taskSources: string[]): Promise<SourceNoteSnapshot> {
  if (!canUseDesktopSourceNotes()) {
    throw new Error("当前运行环境不支持桌面端 markdown 便签桥接。");
  }

  return mapSourceNoteSnapshot(
    await withTimeout(loadDesktopSourceNotes(taskSources), "markdown 便签加载"),
  );
}

/**
 * Loads lightweight source-note metadata so the notes page can poll for
 * external file changes without rereading every markdown file body.
 *
 * @param taskSources Current task-source directory list.
 */
export async function loadNoteSourceIndex(taskSources: string[]): Promise<SourceNoteIndexSnapshot> {
  if (!canUseDesktopSourceNotes()) {
    throw new Error("当前运行环境不支持桌面端 markdown 便签桥接。");
  }

  return mapSourceNoteIndexSnapshot(
    await withTimeout(loadDesktopSourceNoteIndex(taskSources), "markdown 便签索引加载"),
  );
}

/**
 * Appends a markdown note block into the primary task-source note file.
 *
 * @param taskSources Current task-source directory list.
 * @param content Markdown content that should seed the appended note block.
 */
export async function createNoteSource(
  taskSources: string[],
  content: string,
): Promise<SourceNoteDocument> {
  if (!canUseDesktopSourceNotes()) {
    throw new Error("当前运行环境不支持桌面端 markdown 便签桥接。");
  }

  return mapSourceNoteDocument(
    await withTimeout(createDesktopSourceNote(taskSources, content), "markdown 便签创建"),
  );
}

/**
 * Saves markdown content back into the selected task-source note file.
 *
 * @param taskSources Current task-source directory list.
 * @param path Existing markdown note file path.
 * @param content Markdown content from the editor.
 */
export async function saveNoteSource(
  taskSources: string[],
  path: string,
  content: string,
): Promise<SourceNoteDocument> {
  if (!canUseDesktopSourceNotes()) {
    throw new Error("当前运行环境不支持桌面端 markdown 便签桥接。");
  }

  return mapSourceNoteDocument(
    await withTimeout(saveDesktopSourceNote(taskSources, path, content), "markdown 便签保存"),
  );
}

/**
 * Triggers one manual inspection pass from the notes page using the current
 * task-source directories.
 *
 * @param taskSources Current task-source directory list.
 * @param reason Reason string recorded in the inspection request.
 */
export async function runNoteSourceInspection(
  taskSources: string[],
  reason: string,
): Promise<AgentTaskInspectorRunResult> {
  try {
    return await withTimeout(
      runTaskInspector({
        request_meta: createRequestMeta("note_source_inspection"),
        reason,
        target_sources: taskSources,
      }),
      "便签巡检",
    );
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("当前无法执行便签巡检，请稍后重试。");
    }

    throw error;
  }
}
