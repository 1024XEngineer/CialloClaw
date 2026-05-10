import type { NavigateFunction } from "react-router-dom";
import { resolveDashboardRoutePath } from "./dashboardRouteTargets";

export type DashboardResultPageRouteState = {
  taskId: string | null;
  title: string | null;
  url: string;
};

/**
 * Builds the router state used by dashboard result-page views so task- and
 * note-driven result openings can converge on one renderer entry.
 *
 * @param input Result page location and optional task metadata.
 * @returns Router state understood by the dashboard result-page route.
 */
export function buildDashboardResultPageRouteState(input: DashboardResultPageRouteState): DashboardResultPageRouteState {
  return {
    taskId: input.taskId,
    title: input.title,
    url: input.url,
  };
}

/**
 * Reads dashboard result-page router state from an unknown location payload so
 * unrelated route state does not accidentally drive the result-page view.
 *
 * @param value Unknown router state supplied by another dashboard module.
 * @returns Normalized result-page state or null when the payload does not match.
 */
export function readDashboardResultPageRouteState(value: unknown): DashboardResultPageRouteState | null {
  if (!value || typeof value !== "object") {
    return null;
  }

  const url = typeof (value as { url?: unknown }).url === "string"
    ? (value as { url: string }).url.trim()
    : "";
  const taskId = typeof (value as { taskId?: unknown }).taskId === "string"
    ? (value as { taskId: string }).taskId
    : null;
  const title = typeof (value as { title?: unknown }).title === "string"
    ? (value as { title: string }).title
    : null;

  if (!url) {
    return null;
  }

  return {
    taskId,
    title,
    url,
  };
}

/**
 * Resolves dashboard result-page input from router state so current navigation
 * stays within the browser history entry instead of persisting delivery URLs to
 * renderer-readable storage.
 *
 * @param input The current location route state payload.
 * @returns The recoverable result-page route payload or null when missing.
 */
export function readDashboardResultPageLocation(input: { state: unknown }): DashboardResultPageRouteState | null {
  return readDashboardResultPageRouteState(input.state);
}

/**
 * Navigates inside the dashboard to the dedicated result-page shell while
 * preserving the originating task context when available.
 *
 * @param navigate React Router navigate function from the current dashboard view.
 * @param input Result page location and optional task metadata.
 */
export function navigateToDashboardResultPage(
  navigate: NavigateFunction,
  input: DashboardResultPageRouteState,
) {
  navigate(resolveDashboardRoutePath("result"), {
    state: buildDashboardResultPageRouteState(input),
  });
}
