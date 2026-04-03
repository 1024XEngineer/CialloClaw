function registerDesktopBridge(contextBridge, ipcRenderer) {
  const desktopApi = {
    toggleMainPanel() {
      return ipcRenderer.invoke('desktop:toggle-main-panel');
    }
  };

  contextBridge.exposeInMainWorld('CialloDesktop', desktopApi);
  return desktopApi;
}

module.exports = {
  registerDesktopBridge
};

if (typeof process !== 'undefined' && process.type === 'renderer') {
  const { contextBridge, ipcRenderer } = require('electron');
  registerDesktopBridge(contextBridge, ipcRenderer);
}
