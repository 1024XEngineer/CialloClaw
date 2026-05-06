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
  saveSettings,
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

function normalizeTaskSources(taskSources: string[]) {
  return taskSources
    .map((source) => source.trim())
    .filter((source) => source.length > 0);
}

function isAbsoluteLocalTaskSource(source: string) {
  const trimmed = source.trim();
  return /^(?:[a-z]:[\\/]|\\\\|\/)/i.test(trimmed);
}

function isRecoverableTaskSourceError(error: unknown) {
  if (!(error instanceof Error)) {
    return false;
  }

  const normalizedMessage = error.message.trim().toLowerCase();
  return normalizedMessage.includes("task inspection source not found")
    || normalizedMessage.includes("inspection_source_not_found")
    || normalizedMessage.includes("task inspection source unreadable");
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
  const cachedTaskSources = normalizeTaskSources(loadSettings().settings.task_automation.task_sources);

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

function areEquivalentTaskSources(left: string[], right: string[]) {
  const normalizedLeft = normalizeTaskSources(left).map(normalizeSourceEntry);
  const normalizedRight = normalizeTaskSources(right).map(normalizeSourceEntry);
  if (normalizedLeft.length !== normalizedRight.length) {
    return false;
  }

  return normalizedLeft.every((source, index) => source === normalizedRight[index]);
}

async function persistResolvedTaskSources(taskSources: string[]) {
  const normalizedTaskSources = normalizeTaskSources(taskSources);
  if (normalizedTaskSources.length === 0) {
    return;
  }

  const currentSettings = loadSettings();
  if (areEquivalentTaskSources(currentSettings.settings.task_automation.task_sources, normalizedTaskSources)) {
    await syncSourceNoteSettingsSnapshot(normalizedTaskSources);
    return;
  }

  saveSettings({
    settings: {
      ...currentSettings.settings,
      task_automation: {
        ...currentSettings.settings.task_automation,
        task_sources: normalizedTaskSources,
      },
    },
  });
}

async function resolveRuntimeDefaultTaskSources() {
  const runtimeDefaults = await hydrateDesktopRuntimeDefaults();
  return normalizeTaskSources(runtimeDefaults?.task_sources ?? []);
}

async function withRecoveredTaskSources<T>(
  taskSources: string[],
  operation: (nextTaskSources: string[]) => Promise<T>,
) {
  const normalizedTaskSources = normalizeTaskSources(taskSources);
  try {
    return await operation(normalizedTaskSources);
  } catch (error) {
    if (!isRecoverableTaskSourceError(error)) {
      throw error;
    }

    const runtimeTaskSources = await resolveRuntimeDefaultTaskSources();
    if (runtimeTaskSources.length === 0 || areEquivalentTaskSources(runtimeTaskSources, normalizedTaskSources)) {
      throw error;
    }

    await persistResolvedTaskSources(runtimeTaskSources);
    return operation(runtimeTaskSources);
  }
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
    await persistResolvedTaskSources(resolvedTaskSources);

    return {
      ...config,
      task_sources: resolvedTaskSources,
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("当前无法读取任务来源配置，请稍后重试。");
    }

    if (isRecoverableTaskSourceError(error)) {
      const cachedConfig = buildCachedTaskInspectorConfig();
      const resolvedTaskSources = resolvePreferredTaskSources(cachedConfig.task_sources);
      await persistResolvedTaskSources(resolvedTaskSources);
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

  return withRecoveredTaskSources(taskSources, async (nextTaskSources) => mapSourceNoteSnapshot(
    await withTimeout(loadDesktopSourceNotes(nextTaskSources), "markdown 便签加载"),
  ));
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

  return withRecoveredTaskSources(taskSources, async (nextTaskSources) => mapSourceNoteIndexSnapshot(
    await withTimeout(loadDesktopSourceNoteIndex(nextTaskSources), "markdown 便签索引加载"),
  ));
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

  return withRecoveredTaskSources(taskSources, async (nextTaskSources) => mapSourceNoteDocument(
    await withTimeout(createDesktopSourceNote(nextTaskSources, content), "markdown 便签创建"),
  ));
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

  return withRecoveredTaskSources(taskSources, async (nextTaskSources) => mapSourceNoteDocument(
    await withTimeout(saveDesktopSourceNote(nextTaskSources, path, content), "markdown 便签保存"),
  ));
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
    return await withRecoveredTaskSources(taskSources, (nextTaskSources) => withTimeout(
      runTaskInspector({
        request_meta: createRequestMeta("note_source_inspection"),
        reason,
        target_sources: nextTaskSources,
      }),
      "便签巡检",
    ));
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("当前无法执行便签巡检，请稍后重试。");
    }

    throw error;
  }
}
