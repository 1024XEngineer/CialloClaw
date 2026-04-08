// 该文件封装托盘入口控制能力。 
export function openControlPanelFromTray() {
  return openWindowLabel("control-panel");
}

// openWindowLabel 处理当前模块的相关逻辑。
function openWindowLabel(label: string) {
  return Promise.resolve(label);
}
