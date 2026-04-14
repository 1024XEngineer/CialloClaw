// 该文件封装前端本地持久化能力。 
export function loadStoredValue<T>(key: string): T | null {
  const rawValue = window.localStorage.getItem(key);
  if (!rawValue) {
    return null;
  }

  return JSON.parse(rawValue) as T;
}

// saveStoredValue 处理当前模块的相关逻辑。
export function saveStoredValue<T>(key: string, value: T) {
  window.localStorage.setItem(key, JSON.stringify(value));
}

export function removeStoredValue(key: string) {
  window.localStorage.removeItem(key);
}
