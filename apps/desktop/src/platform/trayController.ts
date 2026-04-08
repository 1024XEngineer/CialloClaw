export function openControlPanelFromTray() {
  return openWindowLabel("control-panel");
}

function openWindowLabel(label: string) {
  return Promise.resolve(label);
}
