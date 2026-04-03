const path = require('node:path');
const { app, BrowserWindow, ipcMain, screen } = require('electron');

const {
  FLOATING_SIZE,
  DEFAULT_PANEL_SIZE,
  getFloatingBallBounds,
  getMainPanelBounds,
  buildDesktopPanelUrl
} = require('./desktop-shell.js');
const { createFloatingLauncherLifecycle } = require('./floating-launcher-lifecycle.js');
const { createMainPanelController } = require('./main-controller.js');

let floatingWindow;
let mainPanelController;
let floatingLauncherLifecycle;

function getWorkArea() {
  return screen.getPrimaryDisplay().workArea;
}

function createFloatingWindow() {
  const window = new BrowserWindow({
    width: FLOATING_SIZE.width,
    height: FLOATING_SIZE.height,
    frame: false,
    transparent: true,
    resizable: false,
    skipTaskbar: true,
    alwaysOnTop: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  window.setBounds(getFloatingBallBounds(getWorkArea()));
  window.loadFile(path.join(__dirname, 'floating.html'));

  return window;
}

function createPanelWindow() {
  const window = new BrowserWindow({
    show: false,
    width: DEFAULT_PANEL_SIZE.width,
    height: DEFAULT_PANEL_SIZE.height,
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  window.loadURL(buildDesktopPanelUrl(path.join(__dirname, 'index.html')));
  return window;
}

function positionPanelWindow(panelWindow) {
  const panelBounds = getMainPanelBounds(
    floatingWindow.getBounds(),
    getWorkArea(),
    panelWindow.getBounds()
  );

  panelWindow.setBounds(panelBounds);
}

async function bootstrap() {
  await app.whenReady();

  floatingLauncherLifecycle = createFloatingLauncherLifecycle({
    createFloatingWindow() {
      floatingWindow = createFloatingWindow();
      return floatingWindow;
    }
  });

  floatingWindow = floatingLauncherLifecycle.ensureFloatingWindow();
  const initialPanelWindow = createPanelWindow();
  positionPanelWindow(initialPanelWindow);

  mainPanelController = createMainPanelController({
    initialPanelWindow,
    createPanelWindow,
    positionPanelWindow
  });

  ipcMain.handle('desktop:toggle-main-panel', () => mainPanelController.toggleMainPanel());

  app.on('activate', () => {
    floatingWindow = floatingLauncherLifecycle.ensureFloatingWindow();
  });
}

app.on('window-all-closed', () => {
});

app.on('before-quit', () => {
  if (floatingLauncherLifecycle) {
    floatingLauncherLifecycle.prepareToQuit();
  }

  if (mainPanelController) {
    mainPanelController.prepareToQuit();
  }
});

bootstrap();
