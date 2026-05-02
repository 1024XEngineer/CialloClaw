import type {
  AgentInputSubmitParams,
  AgentInputSubmitResult,
  BehaviorContext,
  InputContext,
  PageContext,
  ScreenContext,
} from "@cialloclaw/protocol";
import {
  recordMirrorConversationFailure,
  recordMirrorConversationStart,
  recordMirrorConversationSuccess,
} from "./mirrorMemoryService";
import { getCurrentConversationSessionId, rememberConversationSessionFromTask } from "./conversationSessionService";
import {
  mapDesktopWindowSnapshotToPageContext,
  sanitizePageContextUrl,
  type DesktopWindowPageContextSnapshot,
} from "./pageContext";

type DesktopWindowContextSnapshot = DesktopWindowPageContextSnapshot & {
  window_switch_count?: number | null;
  page_switch_count?: number | null;
};

type DesktopMouseActivitySnapshot = {
  updated_at: string;
};

export type SubmitTextInputParams = {
  text: string;
  source: AgentInputSubmitParams["source"];
  trigger: AgentInputSubmitParams["trigger"];
  inputMode: AgentInputSubmitParams["input"]["input_mode"];
  context?: InputContext;
  pageContext?: PageContext;
  sessionId?: string;
  options?: {
    confirm_required?: boolean;
    preferred_delivery?: "bubble" | "workspace_document" | "result_page" | "open_file" | "reveal_in_folder" | "task_detail";
  };
};

function createRequestMeta(): AgentInputSubmitParams["request_meta"] {
  const now = new Date().toISOString();
  const traceId = typeof globalThis.crypto?.randomUUID === "function"
    ? globalThis.crypto.randomUUID()
    : `trace_${Date.now()}_${Math.random().toString(16).slice(2)}`;

  return {
    trace_id: traceId,
    client_time: now,
  };
}

function compactContextRecord<T extends object>(value: T | undefined): T | undefined {
  if (!value) {
    return undefined;
  }

  const entries = Object.entries(value as Record<string, unknown>).filter(([, entry]) => {
    if (entry === undefined || entry === null) {
      return false;
    }

    if (typeof entry === "string") {
      return entry.trim() !== "";
    }

    return true;
  });

  return entries.length > 0 ? Object.fromEntries(entries) as T : undefined;
}

function mergeContextRecord<T extends object>(primary: T | undefined, fallback: T | undefined): T | undefined {
  const normalizedPrimary = compactContextRecord(primary);
  const normalizedFallback = compactContextRecord(fallback);

  if (!normalizedPrimary && !normalizedFallback) {
    return undefined;
  }

  return {
    ...(normalizedFallback ?? {}),
    ...(normalizedPrimary ?? {}),
  } as T;
}

function createBaseInputContext(input: SubmitTextInputParams): InputContext {
  const mergedPageContext = mergeContextRecord<PageContext>(input.pageContext, input.context?.page);

  return {
    ...(input.context ?? {}),
    ...(mergedPageContext ? { page: mergedPageContext } : {}),
    files: input.context?.files ?? [],
  };
}

function mapDesktopWindowPageContext(snapshot: DesktopWindowContextSnapshot | null): PageContext | undefined {
  return mapDesktopWindowSnapshotToPageContext(snapshot);
}

function mapDesktopWindowScreenContext(snapshot: DesktopWindowContextSnapshot | null): ScreenContext | undefined {
  if (!snapshot) {
    return undefined;
  }

  const summary = createDesktopScreenSummary(snapshot);

  return compactContextRecord<ScreenContext>({
    summary,
    screen_summary: summary,
    window_title: snapshot.title ?? undefined,
  });
}

function resolveDesktopDwellMillis(updatedAt: string | undefined): number | undefined {
  if (!updatedAt) {
    return undefined;
  }

  const parsed = Number(updatedAt);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }

  return Math.max(0, Date.now() - parsed);
}

function createFallbackBehaviorContext(
  trigger: AgentInputSubmitParams["trigger"],
  mouseSnapshot: DesktopMouseActivitySnapshot | null,
  windowSnapshot: DesktopWindowContextSnapshot | null,
): BehaviorContext | undefined {
  const dwellMillis = resolveDesktopDwellMillis(mouseSnapshot?.updated_at);

  return compactContextRecord<BehaviorContext>({
    last_action: trigger,
    dwell_millis: dwellMillis,
    window_switch_count: normalizeSwitchCount(windowSnapshot?.window_switch_count),
    page_switch_count: normalizeSwitchCount(windowSnapshot?.page_switch_count),
  });
}

function normalizeSwitchCount(value: number | null | undefined): number | undefined {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return undefined;
  }

  return Math.max(0, Math.trunc(value));
}

function createDesktopScreenSummary(snapshot: DesktopWindowContextSnapshot | null): string | undefined {
  if (!snapshot) {
    return undefined;
  }

  const appName = snapshot.app_name.trim();
  const title = snapshot.title?.trim() ?? "";
  const url = sanitizePageContextUrl(snapshot.url) ?? "";

  if (title !== "" && url !== "") {
    return `Foreground ${appName || "desktop"} page "${title}" is active at ${url}.`;
  }

  if (title !== "" && appName !== "") {
    return `Foreground window "${title}" from ${appName} is active.`;
  }

  if (title !== "") {
    return `Foreground window "${title}" is active.`;
  }

  if (appName !== "") {
    return `Foreground app ${appName} is active.`;
  }

  return undefined;
}

function shouldEnrichVisualContext(params: AgentInputSubmitParams): boolean {
  return compactContextRecord(params.context.page) !== undefined || compactContextRecord(params.context.screen) !== undefined;
}

function shouldAttachForegroundPageContext(params: AgentInputSubmitParams): boolean {
  return params.source === "floating_ball"
    && (params.trigger === "hover_text_input" || params.trigger === "voice_commit");
}

async function readDesktopWindowContext(): Promise<DesktopWindowContextSnapshot | null> {
  try {
    const desktopWindowContextModule = await import("@/platform/desktopWindowContext");
    return await desktopWindowContextModule.getActiveWindowContext();
  } catch {
    return null;
  }
}

async function readDesktopMouseActivitySnapshot(): Promise<DesktopMouseActivitySnapshot | null> {
  try {
    const desktopActivityModule = await import("@/platform/desktopActivity");
    return await desktopActivityModule.getDesktopMouseActivitySnapshot();
  } catch {
    return null;
  }
}

/**
 * Builds the stable `agent.input.submit` payload shared by shell-ball and
 * dashboard text-entry surfaces.
 *
 * @param input Submission metadata and any explicit formal context overrides.
 * @returns The normalized JSON-RPC payload, or `null` when the draft is empty.
 */
export function createTextInputSubmitParams(input: SubmitTextInputParams): AgentInputSubmitParams | null {
  const normalizedText = input.text.trim();
  const normalizedSessionId = input.sessionId?.trim() || getCurrentConversationSessionId();

  if (normalizedText === "") {
    return null;
  }

  return {
    request_meta: createRequestMeta(),
    ...(normalizedSessionId ? { session_id: normalizedSessionId } : {}),
    source: input.source,
    trigger: input.trigger,
    input: {
      type: "text",
      text: normalizedText,
      input_mode: input.inputMode,
    },
    context: createBaseInputContext(input),
    ...(input.options ? { options: input.options } : {}),
  };
}

export type SubmitTextInputResult = AgentInputSubmitResult;

async function enrichTextInputSubmitParams(params: AgentInputSubmitParams): Promise<AgentInputSubmitParams> {
  const enrichVisualContext = shouldEnrichVisualContext(params);
  const attachForegroundPageContext = shouldAttachForegroundPageContext(params);
  const shouldReadForegroundWindowContext = enrichVisualContext || attachForegroundPageContext;
  const [windowContext, mouseActivitySnapshot] = await Promise.all([
    // Explicit visual requests still need both page and screen fallbacks, while
    // shell-ball near-field text/voice submits should also inherit the current
    // foreground page attach hints for real-browser takeover planning.
    shouldReadForegroundWindowContext ? readDesktopWindowContext() : Promise.resolve(null),
    readDesktopMouseActivitySnapshot(),
  ]);
  const fallbackPageContext = shouldReadForegroundWindowContext
    ? mapDesktopWindowPageContext(windowContext)
    : undefined;
  const fallbackScreenContext = enrichVisualContext ? mapDesktopWindowScreenContext(windowContext) : undefined;
  const fallbackBehaviorContext = createFallbackBehaviorContext(params.trigger, mouseActivitySnapshot, windowContext);
  const mergedPageContext = mergeContextRecord<PageContext>(params.context.page, fallbackPageContext);
  const mergedScreenContext = mergeContextRecord<ScreenContext>(params.context.screen, fallbackScreenContext);
  const mergedBehaviorContext = mergeContextRecord<BehaviorContext>(params.context.behavior, fallbackBehaviorContext);

  return {
    ...params,
    context: {
      ...params.context,
      files: params.context.files ?? [],
      ...(mergedPageContext ? {
        page: mergedPageContext,
      } : {}),
      ...(mergedScreenContext ? {
        screen: mergedScreenContext,
      } : {}),
      ...(mergedBehaviorContext ? {
        behavior: mergedBehaviorContext,
      } : {}),
    },
  };
}

/**
 * Submits a lightweight text input through the formal desktop task pipeline.
 * The renderer enriches the request with best-effort desktop context before
 * sending it over JSON-RPC.
 *
 * @param input Submission metadata and optional explicit context overrides.
 * @returns The formal submit result, or `null` when the draft is empty.
 */
export async function submitTextInput(input: SubmitTextInputParams) {
  const params = createTextInputSubmitParams(input);

  if (params === null) {
    return null;
  }

  const enrichedParams = await enrichTextInputSubmitParams(params);
  recordMirrorConversationStart(enrichedParams);
  const rpcMethods = await import("@/rpc/methods");

  try {
    const result = await rpcMethods.submitInput(enrichedParams);
    rememberConversationSessionFromTask(result.task);
    recordMirrorConversationSuccess(enrichedParams, result);
    return result;
  } catch (error) {
    recordMirrorConversationFailure(enrichedParams, error);
    throw error;
  }
}
