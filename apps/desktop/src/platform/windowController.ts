// 该文件封装桌面窗口控制能力。 
export function focusWindow(label: string) {
  return openWindow(label);
}

// openWindow 处理当前模块的相关逻辑。
export function openWindow(label: string) {
  if (typeof window !== "undefined") {
    window.location.assign(`./${label}.html`);
  }

  return Promise.resolve(label);
}
