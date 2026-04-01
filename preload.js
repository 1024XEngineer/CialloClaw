const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
    toggleChat: () => ipcRenderer.send('toggle-chat'),
    showContextMenu: () => ipcRenderer.send('show-context-menu'),
    closeWindow: () => ipcRenderer.send('close-window'),
    minimizeWindow: () => ipcRenderer.send('minimize-window'),
    openDashboard: () => ipcRenderer.send('open-dashboard'),
    closeDashboard: () => ipcRenderer.send('close-dashboard'),
    
    getConfig: () => ipcRenderer.invoke('get-config'),
    saveConfig: (config) => ipcRenderer.invoke('save-config', config),
    
    getChatHistory: () => ipcRenderer.invoke('get-chat-history'),
    saveChatHistory: (history) => ipcRenderer.invoke('save-chat-history', history),
    
    getTasks: () => ipcRenderer.invoke('get-tasks'),
    saveTasks: (tasks) => ipcRenderer.invoke('save-tasks', tasks),
    
    getMemory: () => ipcRenderer.invoke('get-memory'),
    saveMemory: (memory) => ipcRenderer.invoke('save-memory', memory),
    
    sendNotification: (title, body) => ipcRenderer.send('send-notification', { title, body }),
    
    onSwitchTab: (callback) => {
        ipcRenderer.on('switch-tab', (event, tab) => callback(tab));
    }
});