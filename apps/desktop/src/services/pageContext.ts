import type { PageContext } from "@cialloclaw/protocol";

export type DesktopWindowPageContextSnapshot = {
  app_name: string;
  browser_kind: "chrome" | "edge" | "other_browser" | "non_browser";
  process_path: string | null;
  process_id: number | null;
  title: string | null;
  url: string | null;
};

function normalizeOptionalPageContextText(value: string | undefined) {
  const trimmed = value?.trim() ?? "";
  return trimmed === "" ? undefined : trimmed;
}

/**
 * Removes volatile and sensitive URL parts before desktop window context enters
 * the formal task context payload.
 *
 * @param rawUrl Raw browser URL reported by the desktop host.
 * @returns A stable page URL without credentials, query, or fragment details.
 */
export function sanitizePageContextUrl(rawUrl: string | null | undefined): string | undefined {
  const normalizedUrl = rawUrl?.trim() ?? "";

  if (normalizedUrl === "") {
    return undefined;
  }

  try {
    const parsedUrl = new URL(normalizedUrl);
    parsedUrl.username = "";
    parsedUrl.password = "";
    parsedUrl.search = "";
    parsedUrl.hash = "";
    return parsedUrl.toString();
  } catch {
    return normalizedUrl.split(/[?#]/u, 1)[0]?.trim() || undefined;
  }
}

/**
 * Drops empty or invalid page-context fields while preserving attach hints that
 * downstream browser flows need for real-window takeover.
 *
 * @param pageContext Candidate page-context payload.
 * @returns A compact formal page context, or `undefined` when no usable fields remain.
 */
export function compactPageContext(pageContext: PageContext | undefined): PageContext | undefined {
  if (!pageContext) {
    return undefined;
  }

  const appName = normalizeOptionalPageContextText(pageContext.app_name);
  const title = normalizeOptionalPageContextText(pageContext.title);
  const url = sanitizePageContextUrl(pageContext.url);
  const processPath = normalizeOptionalPageContextText(pageContext.process_path);
  const windowTitle = normalizeOptionalPageContextText(pageContext.window_title);
  const visibleText = normalizeOptionalPageContextText(pageContext.visible_text);
  const hoverTarget = normalizeOptionalPageContextText(pageContext.hover_target);
  const compacted: PageContext = {
    ...(appName ? { app_name: appName } : {}),
    ...(title ? { title } : {}),
    ...(url ? { url } : {}),
    ...(pageContext.browser_kind ? { browser_kind: pageContext.browser_kind } : {}),
    ...(processPath ? { process_path: processPath } : {}),
    ...(Number.isInteger(pageContext.process_id) && (pageContext.process_id ?? 0) > 0
      ? { process_id: pageContext.process_id }
      : {}),
    ...(windowTitle ? { window_title: windowTitle } : {}),
    ...(visibleText ? { visible_text: visibleText } : {}),
    ...(hoverTarget ? { hover_target: hoverTarget } : {}),
  };

  return Object.keys(compacted).length > 0 ? compacted : undefined;
}

/**
 * Converts the latest desktop browser snapshot into the formal page-context
 * shape consumed by task entrypoints.
 *
 * @param snapshot Raw desktop window snapshot captured from the host platform.
 * @returns A sanitized page-context payload, or `undefined` when the snapshot is absent.
 */
export function mapDesktopWindowSnapshotToPageContext(
  snapshot: DesktopWindowPageContextSnapshot | null,
): PageContext | undefined {
  if (!snapshot) {
    return undefined;
  }

  return compactPageContext({
    app_name: snapshot.app_name,
    browser_kind: snapshot.browser_kind,
    process_path: snapshot.process_path ?? undefined,
    process_id: snapshot.process_id ?? undefined,
    title: snapshot.title ?? undefined,
    url: sanitizePageContextUrl(snapshot.url),
    window_title: snapshot.title ?? undefined,
  });
}

/**
 * Keeps task-start page context aligned with formal inputs while retaining the
 * default shell-ball intake context for sources that do not carry any page hints.
 *
 * @param pageContext Candidate page-context payload.
 * @param fallback Default page-context payload for desktop-only entrypoints.
 * @returns The compact page context, or the provided fallback when none remains.
 */
export function resolveTaskPageContext(pageContext: PageContext | undefined, fallback: PageContext): PageContext {
  return compactPageContext(pageContext) ?? fallback;
}
