// 该文件封装托盘入口控制能力。 
export function openControlPanelFromTray() {
  return openWindowLabel("control-panel");
}

function openWindowLabel(label: string) {
  return Promise.resolve(label);
}
