/**
 * Provides local renderer storage helpers.
 *
 * These helpers must stay SSR-safe because several contract tests render desktop
 * entry points in a Node environment without a browser `window`.
 */
function readLocalStorage() {
  if (typeof window === "undefined" || typeof window.localStorage === "undefined") {
    return null;
  }

  return window.localStorage;
}

export function loadStoredValue<T>(key: string): T | null {
  const localStorage = readLocalStorage();
  if (localStorage === null) {
    return null;
  }

  const rawValue = localStorage.getItem(key);
  if (!rawValue) {
    return null;
  }

  try {
    return JSON.parse(rawValue) as T;
  } catch (error) {
    console.warn("[storage] Failed to parse localStorage value", { key, error });
    return null;
  }
}

/**
 * Persists a JSON-serializable value when browser storage is available.
 */
export function saveStoredValue<T>(key: string, value: T) {
  const localStorage = readLocalStorage();
  if (localStorage === null) {
    return;
  }

  localStorage.setItem(key, JSON.stringify(value));
}

export function removeStoredValue(key: string) {
  const localStorage = readLocalStorage();
  if (localStorage === null) {
    return;
  }

  localStorage.removeItem(key);
}
