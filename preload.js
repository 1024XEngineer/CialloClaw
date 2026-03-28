const { contextBridge, ipcRenderer } = require("electron");

function getWindowType() {
  const value = process.argv.find((entry) => entry.startsWith("--window="));
  return value ? value.replace("--window=", "") : "unknown";
}

contextBridge.exposeInMainWorld("pixelOrb", {
  windowType: getWindowType(),
  bootstrap: () => ipcRenderer.invoke("app:bootstrap"),
  markActivity: () => ipcRenderer.send("presence:activity"),
  expandOrb: () => ipcRenderer.send("orb:expand"),
  collapseOrb: () => ipcRenderer.send("orb:collapse"),
  dragStart: (payload) => ipcRenderer.send("orb:drag-start", payload),
  dragMove: (payload) => ipcRenderer.send("orb:drag-move", payload),
  dragEnd: () => ipcRenderer.send("orb:drag-end"),
  showContextMenu: (payload) => ipcRenderer.send("orb:show-menu", payload),
  openChat: (options) => ipcRenderer.send("chat:open", options || {}),
  closeChat: () => ipcRenderer.send("chat:close"),
  openPanel: () => ipcRenderer.send("panel:open"),
  closePanel: () => ipcRenderer.send("panel:close"),
  runTask: (taskId, source) => ipcRenderer.send("task:run", { taskId, source }),
  cancelTask: () => ipcRenderer.send("task:cancel"),
  sendChat: (text) => ipcRenderer.send("chat:send", { text }),
  updatePreferences: (patch) => ipcRenderer.send("prefs:update", patch),
  onSnapshot: (handler) => {
    const listener = (_event, payload) => handler(payload);
    ipcRenderer.on("snapshot", listener);
    return () => ipcRenderer.removeListener("snapshot", listener);
  }
});
