import type { NavigateFunction } from "react-router-dom";
import { resolveDashboardRoutePath } from "./dashboardRouteTargets";

export type DashboardResultPageRouteState = {
  taskId: string | null;
  title: string | null;
  url: string;
};

type DashboardResultPageLocationInput = {
  search: string;
  state: unknown;
};

type StoredDashboardResultPageRouteState = DashboardResultPageRouteState & {
  storedAt: number;
};

const dashboardResultPageStoragePrefix = "dashboard.result-page.";
const dashboardResultPageStorageMaxAgeMs = 1000 * 60 * 5;
const dashboardResultPageStorageMaxEntries = 4;

function getDashboardResultPageStorage() {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function listDashboardResultPageStorageKeys(storage: Storage) {
  const keys: string[] = [];

  for (let index = 0; index < storage.length; index += 1) {
    const key = storage.key(index);
    if (key && key.startsWith(dashboardResultPageStoragePrefix)) {
      keys.push(key);
    }
  }

  return keys.sort();
}

function pruneDashboardResultPageStorage(storage: Storage, now: number) {
  const keys = listDashboardResultPageStorageKeys(storage);

  for (const key of keys) {
    const raw = storage.getItem(key);
    if (!raw) {
      storage.removeItem(key);
      continue;
    }

    try {
      const parsed = JSON.parse(raw) as Partial<StoredDashboardResultPageRouteState>;
      if (typeof parsed.storedAt !== "number" || now - parsed.storedAt > dashboardResultPageStorageMaxAgeMs) {
        storage.removeItem(key);
      }
    } catch {
      storage.removeItem(key);
    }
  }

  const remainingKeys = listDashboardResultPageStorageKeys(storage);

  while (remainingKeys.length > dashboardResultPageStorageMaxEntries) {
    const oldestKey = remainingKeys.shift();
    if (!oldestKey) {
      break;
    }
    storage.removeItem(oldestKey);
  }
}

function storeDashboardResultPageRouteState(input: DashboardResultPageRouteState) {
  const storage = getDashboardResultPageStorage();
  if (!storage) {
    return null;
  }

  const now = Date.now();
  const token = `${now.toString(36)}${Math.random().toString(36).slice(2, 10)}`;
  const key = `${dashboardResultPageStoragePrefix}${token}`;
  const value: StoredDashboardResultPageRouteState = {
    ...input,
    storedAt: now,
  };

  pruneDashboardResultPageStorage(storage, now);
  storage.setItem(key, JSON.stringify(value));
  return token;
}

function readStoredDashboardResultPageRouteState(token: string): DashboardResultPageRouteState | null {
  const storage = getDashboardResultPageStorage();
  if (!storage) {
    return null;
  }

  const now = Date.now();
  pruneDashboardResultPageStorage(storage, now);
  const storageKey = `${dashboardResultPageStoragePrefix}${token}`;
  const raw = storage.getItem(storageKey);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<StoredDashboardResultPageRouteState>;
    if (typeof parsed.url !== "string" || parsed.url.trim() === "") {
      storage.removeItem(storageKey);
      return null;
    }

    return {
      taskId: typeof parsed.taskId === "string" ? parsed.taskId : null,
      title: typeof parsed.title === "string" ? parsed.title : null,
      url: parsed.url.trim(),
    };
  } catch {
    storage.removeItem(storageKey);
    return null;
  }
}

function readDashboardResultPageSearch(search: string): DashboardResultPageRouteState | null {
  const params = new URLSearchParams(search);
  const token = (params.get("result_id") ?? "").trim();
  return token ? readStoredDashboardResultPageRouteState(token) : null;
}

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
 * Resolves dashboard result-page input from both router search params and route
 * state so refreshes can recover the formal delivery address while the cached
 * recovery token remains valid in the current dashboard session.
 *
 * @param input The current location search string and route state payload.
 * @returns The recoverable result-page route payload or null when missing.
 */
export function readDashboardResultPageLocation(input: DashboardResultPageLocationInput): DashboardResultPageRouteState | null {
  const searchState = readDashboardResultPageSearch(input.search);
  const routedState = readDashboardResultPageRouteState(input.state);

  if (!searchState) {
    return routedState;
  }

  return {
    taskId: searchState.taskId ?? routedState?.taskId ?? null,
    title: searchState.title ?? routedState?.title ?? null,
    url: searchState.url,
  };
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
  const token = storeDashboardResultPageRouteState(input);
  const search = token ? `?result_id=${encodeURIComponent(token)}` : "";
  navigate(`${resolveDashboardRoutePath("result")}${search}`, {
    state: buildDashboardResultPageRouteState(input),
  });
}
