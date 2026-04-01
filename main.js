const { app, BrowserWindow, Menu, ipcMain, screen, Tray, globalShortcut, Notification, nativeImage } = require('electron');
const path = require('path');
const fs = require('fs');

let clawWindow = null;
let chatWindow = null;
let dashboardWindow = null;
let tray = null;

const userDataPath = app.getPath('userData');
const configPath = path.join(userDataPath, 'config.json');
const chatHistoryPath = path.join(userDataPath, 'chat-history.json');
const tasksPath = path.join(userDataPath, 'tasks.json');
const memoryPath = path.join(userDataPath, 'memory.json');

let config = {
    autoStart: false,
    showNotifications: true,
    soundEffects: true,
    alwaysOnTop: true,
    theme: 'dark'
};

function loadConfig() {
    try {
        if (fs.existsSync(configPath)) {
            const data = fs.readFileSync(configPath, 'utf-8');
            config = { ...config, ...JSON.parse(data) };
        }
    } catch (e) {
        console.error('Failed to load config:', e);
    }
}

function saveConfig() {
    try {
        fs.writeFileSync(configPath, JSON.stringify(config, null, 2));
    } catch (e) {
        console.error('Failed to save config:', e);
    }
}

function loadChatHistory() {
    try {
        if (fs.existsSync(chatHistoryPath)) {
            return JSON.parse(fs.readFileSync(chatHistoryPath, 'utf-8'));
        }
    } catch (e) {
        console.error('Failed to load chat history:', e);
    }
    return [];
}

function saveChatHistory(history) {
    try {
        fs.writeFileSync(chatHistoryPath, JSON.stringify(history, null, 2));
    } catch (e) {
        console.error('Failed to save chat history:', e);
    }
}

function loadTasks() {
    try {
        if (fs.existsSync(tasksPath)) {
            return JSON.parse(fs.readFileSync(tasksPath, 'utf-8'));
        }
    } catch (e) {
        console.error('Failed to load tasks:', e);
    }
    return [];
}

function saveTasks(tasks) {
    try {
        fs.writeFileSync(tasksPath, JSON.stringify(tasks, null, 2));
    } catch (e) {
        console.error('Failed to save tasks:', e);
    }
}

function loadMemory() {
    try {
        if (fs.existsSync(memoryPath)) {
            return JSON.parse(fs.readFileSync(memoryPath, 'utf-8'));
        }
    } catch (e) {
        console.error('Failed to load memory:', e);
    }
    return [];
}

function saveMemory(memory) {
    try {
        fs.writeFileSync(memoryPath, JSON.stringify(memory, null, 2));
    } catch (e) {
        console.error('Failed to save memory:', e);
    }
}

function createTray() {
    const iconSize = 16;
    const icon = nativeImage.createEmpty();
    
    const trayIconPath = path.join(__dirname, 'tray-icon.png');
    if (fs.existsSync(trayIconPath)) {
        tray = new Tray(trayIconPath);
    } else {
        tray = new Tray(icon.resize({ width: iconSize, height: iconSize }));
    }
    
    const contextMenu = Menu.buildFromTemplate([
        {
            label: 'Show/Hide Claw',
            click: () => {
                if (clawWindow) {
                    if (clawWindow.isVisible()) {
                        clawWindow.hide();
                    } else {
                        clawWindow.show();
                    }
                }
            }
        },
        {
            label: 'Open Chat',
            click: () => {
                createChatWindow();
            }
        },
        {
            label: 'Open Dashboard',
            click: () => {
                createDashboardWindow();
            }
        },
        { type: 'separator' },
        {
            label: 'Quit',
            click: () => {
                app.quit();
            }
        }
    ]);
    
    tray.setToolTip('CialloClaw');
    tray.setContextMenu(contextMenu);
    
    tray.on('click', () => {
        if (clawWindow) {
            if (clawWindow.isVisible()) {
                clawWindow.hide();
            } else {
                clawWindow.show();
            }
        }
    });
}

function registerShortcuts() {
    globalShortcut.register('CommandOrControl+Shift+C', () => {
        toggleChatWindow();
    });
    
    globalShortcut.register('CommandOrControl+Shift+D', () => {
        createDashboardWindow();
    });
}

function showNotification(title, body) {
    if (config.showNotifications && Notification.isSupported()) {
        const notification = new Notification({
            title: title,
            body: body,
            silent: config.soundEffects
        });
        notification.show();
    }
}

function createClawWindow() {
    clawWindow = new BrowserWindow({
        width: 50,
        height: 50,
        frame: false,
        transparent: true,
        alwaysOnTop: config.alwaysOnTop,
        resizable: false,
        skipTaskbar: true,
        hasShadow: false,
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false
        }
    });

    clawWindow.loadFile('claw.html');
    clawWindow.setVisibleOnAllWorkspaces(true);
    
    clawWindow.webContents.on('did-finish-load', () => {
        const primaryDisplay = screen.getPrimaryDisplay();
        const { width, height } = primaryDisplay.workAreaSize;
        clawWindow.setPosition(width - 70, height - 70);
    });
}

function createChatWindow() {
    if (chatWindow) {
        chatWindow.show();
        chatWindow.focus();
        return;
    }

    const clawBounds = clawWindow.getBounds();
    const chatWidth = 360;
    const chatHeight = 480;
    
    let x = clawBounds.x - chatWidth + 50;
    let y = clawBounds.y - chatHeight - 10;
    
    if (x < 0) x = clawBounds.x + 60;
    if (y < 0) y = clawBounds.y + 60;

    chatWindow = new BrowserWindow({
        width: chatWidth,
        height: chatHeight,
        x: x,
        y: y,
        frame: false,
        transparent: true,
        alwaysOnTop: true,
        resizable: false,
        skipTaskbar: true,
        hasShadow: true,
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false
        }
    });

    chatWindow.loadFile('index.html');

    chatWindow.on('closed', () => {
        chatWindow = null;
    });
}

function toggleChatWindow() {
    if (chatWindow && chatWindow.isVisible()) {
        chatWindow.hide();
    } else {
        createChatWindow();
    }
}

function createDashboardWindow() {
    if (dashboardWindow) {
        dashboardWindow.show();
        dashboardWindow.focus();
        return;
    }

    const primaryDisplay = screen.getPrimaryDisplay();
    const { width: screenWidth, height: screenHeight } = primaryDisplay.workAreaSize;

    dashboardWindow = new BrowserWindow({
        width: Math.min(1000, screenWidth - 100),
        height: Math.min(700, screenHeight - 100),
        frame: false,
        transparent: true,
        minWidth: 600,
        minHeight: 400,
        center: true,
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
            contextIsolation: true,
            nodeIntegration: false
        }
    });

    dashboardWindow.loadFile('dashboard.html');
    dashboardWindow.setMenuBarVisibility(false);

    dashboardWindow.on('closed', () => {
        dashboardWindow = null;
    });
}

function showClawContextMenu() {
    const contextMenu = Menu.buildFromTemplate([
        {
            label: 'Open Dashboard',
            click: () => {
                createDashboardWindow();
            }
        },
        {
            label: 'Open Chat',
            click: () => {
                createChatWindow();
            }
        },
        { type: 'separator' },
        {
            label: 'Settings',
            click: () => {
                createDashboardWindow();
                dashboardWindow.webContents.once('did-finish-load', () => {
                    dashboardWindow.webContents.send('switch-tab', 'settings');
                });
            }
        },
        { type: 'separator' },
        {
            label: 'Quit',
            click: () => {
                app.quit();
            }
        }
    ]);
    contextMenu.popup({ window: clawWindow });
}

ipcMain.on('toggle-chat', () => {
    toggleChatWindow();
});

ipcMain.on('show-context-menu', () => {
    showClawContextMenu();
});

ipcMain.on('close-window', (event) => {
    const win = BrowserWindow.fromWebContents(event.sender);
    if (win) win.close();
});

ipcMain.on('minimize-window', (event) => {
    const win = BrowserWindow.fromWebContents(event.sender);
    if (win) win.minimize();
});

ipcMain.on('open-dashboard', () => {
    createDashboardWindow();
});

ipcMain.on('close-dashboard', () => {
    if (dashboardWindow) {
        dashboardWindow.close();
    }
});

ipcMain.handle('get-config', () => {
    return config;
});

ipcMain.handle('save-config', (event, newConfig) => {
    config = { ...config, ...newConfig };
    saveConfig();
    
    if (clawWindow) {
        clawWindow.setAlwaysOnTop(config.alwaysOnTop);
    }
    
    return config;
});

ipcMain.handle('get-chat-history', () => {
    return loadChatHistory();
});

ipcMain.handle('save-chat-history', (event, history) => {
    saveChatHistory(history);
    return true;
});

ipcMain.handle('get-tasks', () => {
    return loadTasks();
});

ipcMain.handle('save-tasks', (event, tasks) => {
    saveTasks(tasks);
    return true;
});

ipcMain.handle('get-memory', () => {
    return loadMemory();
});

ipcMain.handle('save-memory', (event, memory) => {
    saveMemory(memory);
    return true;
});

ipcMain.on('send-notification', (event, { title, body }) => {
    showNotification(title, body);
});

app.whenReady().then(() => {
    loadConfig();
    createClawWindow();
    createTray();
    registerShortcuts();
    
    console.log('CialloClaw started successfully');
});

app.on('window-all-closed', () => {
});

app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
        createClawWindow();
    }
});

app.on('will-quit', () => {
    globalShortcut.unregisterAll();
});