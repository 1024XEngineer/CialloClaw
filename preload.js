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
  getModelConfig: () => ipcRenderer.invoke("settings:model:get"),
  saveModelConfig: (config) => ipcRenderer.invoke("settings:model:save", config),
  startMeetingSummary: (options) => ipcRenderer.invoke("meeting-summary:start", options || {}),
  showMeetingSummaryOverlay: () => ipcRenderer.send("meeting-summary-overlay:show"),
  hideMeetingSummaryOverlay: () => ipcRenderer.send("meeting-summary-overlay:hide"),
  showMeetingCaptionsOverlay: () => ipcRenderer.send("meeting-captions-overlay:show"),
  hideMeetingCaptionsOverlay: () => ipcRenderer.send("meeting-captions-overlay:hide"),
  resizeCurrentWindow: (payload) => ipcRenderer.send("window:resize-current", payload || {}),
  acceptMeetingDetectionPrompt: () => ipcRenderer.invoke("meeting-detection:accept"),
  dismissMeetingDetectionPrompt: () => ipcRenderer.invoke("meeting-detection:dismiss"),
  meetingSummaryCaptureReady: (payload) => ipcRenderer.send("meeting-summary:capture-ready", payload),
  reportMeetingSummarySourceError: (payload) => ipcRenderer.send("meeting-summary:source-error", payload),
  appendMeetingSummaryAudio: (payload) => ipcRenderer.send("meeting-summary:audio-append", payload),
  processMeetingSummaryChunk: (payload) => ipcRenderer.invoke("meeting-summary:chunk", payload),
  finalizeMeetingRealtimeTranscription: () => ipcRenderer.invoke("meeting-summary:finalize-transcription"),
  refreshMeetingSummary: (payload) => ipcRenderer.invoke("meeting-summary:refresh-summary", payload),
  stopMeetingSummary: () => ipcRenderer.invoke("meeting-summary:stop"),
  reportMeetingSummaryError: (message) => ipcRenderer.send("meeting-summary:error", { message }),
  onSnapshot: (handler) => {
    const listener = (_event, payload) => handler(payload);
    ipcRenderer.on("snapshot", listener);
    return () => ipcRenderer.removeListener("snapshot", listener);
  }
});
