/**
 * Reads a JSON-backed value from local storage.
 *
 * Corrupted snapshots are cleared so feature modules can fall back to their
 * default state instead of failing during render.
 */
export function loadStoredValue<T>(key: string): T | null {
  const rawValue = window.localStorage.getItem(key);
  if (!rawValue) {
    return null;
  }

  try {
    return JSON.parse(rawValue) as T;
  } catch {
    window.localStorage.removeItem(key);
    return null;
  }
}

/**
 * Persists a JSON-backed value to local storage.
 */
export function saveStoredValue<T>(key: string, value: T) {
  window.localStorage.setItem(key, JSON.stringify(value));
}

export function removeStoredValue(key: string) {
  window.localStorage.removeItem(key);
}
