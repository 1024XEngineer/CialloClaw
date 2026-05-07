// Keep desktop storage access SSR-safe so contract tests and non-DOM entry
// points can import the module without crashing on `window`.
function resolveLocalStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

export function loadStoredValue<T>(key: string): T | null {
  const rawValue = resolveLocalStorage()?.getItem(key);
  if (!rawValue) {
    return null;
  }

  return JSON.parse(rawValue) as T;
}

export function saveStoredValue<T>(key: string, value: T) {
  resolveLocalStorage()?.setItem(key, JSON.stringify(value));
}

export function removeStoredValue(key: string) {
  resolveLocalStorage()?.removeItem(key);
}
