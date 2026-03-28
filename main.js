const { app, BrowserWindow, Menu, Tray, ipcMain, nativeImage, screen } = require("electron");
const path = require("path");

const COLLAPSED_SIZE = {
  small: 88,
  default: 96,
  large: 108
};
const RING_WINDOW = {
  width: 420,
  height: 420
};
const TASK_RING_RADIUS = 104;
const TASK_BALL_SIZE = 54;
const RING_SAFE_PADDING = 24;
const EXPANDED_SAFE_INSET = TASK_RING_RADIUS + Math.ceil(TASK_BALL_SIZE / 2) + RING_SAFE_PADDING;
const CHAT_WINDOW = {
  width: 392,
  height: 472
};
const PANEL_WINDOW = {
  width: 1040,
  height: 720
};
const SNAP_THRESHOLD = 28;
const PEEK_VISIBLE = 26;
const INACTIVITY_TO_PEEK = 9500;
const PEEK_REVEAL_DISTANCE = 72;
const FLOAT_GAP = 16;

let mainWindow;
let chatWindow;
let panelWindow;
let tray;
let proximityTimer;
let progressTimer;
let dragState = null;
let appIsQuitting = false;

const baseTasks = [
  {
    id: "ask",
    slot: 0,
    shortLabel: "问",
    title: "快速提问",
    description: "用一句话发起一个短问题，不离开当前上下文。",
    availability: "始终可用",
    hint: "点击运行",
    group: "fixed"
  },
  {
    id: "summarize",
    slot: 1,
    shortLabel: "总",
    title: "总结选中文本",
    description: "把当前选中的内容压缩为重点摘要或行动项。",
    availability: "适用于选中文本",
    hint: "点击运行",
    group: "fixed"
  },
  {
    id: "translate",
    slot: 2,
    shortLabel: "译",
    title: "翻译内容",
    description: "保留原意和语气，在中英之间快速转换。",
    availability: "适用于选中文本",
    hint: "点击运行",
    group: "fixed"
  },
  {
    id: "explain",
    slot: 3,
    shortLabel: "解",
    title: "解释错误",
    description: "将报错、日志或概念解释成更容易理解的话。",
    availability: "适用于错误信息",
    hint: "点击运行",
    group: "fixed"
  },
  {
    id: "clipboard",
    slot: 4,
    shortLabel: "夹",
    title: "整理剪贴板",
    description: "从剪贴板里抽取待办、链接和关键片段。",
    availability: "适用于剪贴板内容",
    hint: "点击运行",
    group: "fixed"
  },
  {
    id: "compare",
    slot: 5,
    shortLabel: "比",
    title: "快速比较",
    description: "快速比较两个方案、两个版本或两个选择。",
    availability: "始终可用",
    hint: "点击运行",
    group: "fixed"
  },
  {
    id: "context",
    slot: 6,
    shortLabel: "待",
    title: "提取待办",
    description: "从当前选中的片段里抽出可执行的行动项。",
    availability: "适用于选中文本",
    hint: "点击运行",
    group: "context"
  },
  {
    id: "more",
    slot: 7,
    shortLabel: "更",
    title: "更多任务",
    description: "打开紧凑任务库，查看更多轻量动作。",
    availability: "始终可用",
    hint: "点击展开",
    group: "fixed"
  }
];

const taskLibrary = [
  {
    id: "rename",
    title: "批量重命名",
    description: "生成命名规则并预览替换结果。",
    availability: "适用于文件列表",
    hint: "点击运行"
  },
  {
    id: "convert",
    title: "格式转换",
    description: "将片段改写成 Markdown、表格或邮件格式。",
    availability: "适用于文本内容",
    hint: "点击运行"
  },
  {
    id: "reminder",
    title: "创建提醒",
    description: "把一句话变成一条简单提醒或后续动作。",
    availability: "始终可用",
    hint: "点击运行"
  },
  {
    id: "search",
    title: "快速搜索",
    description: "把关键词整理成更清晰的搜索入口。",
    availability: "始终可用",
    hint: "点击运行"
  }
];

const appState = {
  status: "idle",
  statusNote: "待命中",
  agentPaused: false,
  dnd: false,
  lowProfile: true,
  size: "default",
  autoHideMode: "peek",
  progress: 0,
  progressLabel: "",
  currentTask: null,
  chatPinned: false,
  quickChips: [
    "总结选中文本",
    "解释这个错误",
    "帮我起草回复",
    "翻译为英文"
  ],
  context: {
    selectedTextAvailable: true,
    clipboardReady: true,
    activeApp: "Visual Studio Code",
    currentSelection: "请把这段需求压缩为 3 个重点，并保留产品语气。",
    clipboardText: "待办: 更新首页文案 / 检查导出逻辑 / 回复设计评审",
    errorSnippet: "TypeError: Cannot read properties of undefined (reading 'status')",
    activeFile: "floating-orb-spec.md"
  },
  abilityToggles: {
    selection: true,
    clipboard: true,
    activeApp: true,
    plugins: false
  },
  theme: {
    motionScale: 1,
    saturation: 1,
    tooltipDelay: 220
  },
  recentActions: [
    {
      id: "action-boot",
      title: "悬浮球已就绪",
      detail: "桌面像素伙伴已加载到边缘停靠位。",
      time: "刚刚",
      level: "info"
    }
  ],
  chatHistory: [
    {
      id: "msg-1",
      role: "assistant",
      text: "我会尽量待在视觉边缘，需要的时候再靠近一点。你可以直接问我一个短问题，或者悬停打开任务环。",
      timestamp: "09:20"
    }
  ]
};

const orbState = {
  mode: "collapsed",
  dockSide: "right",
  collapsedBounds: null,
  expandedBounds: null,
  peeked: false,
  hidden: false,
  lastInteractionAt: Date.now(),
  previewDockSide: null
};

function createOrbIconDataUrl() {
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
      <rect width="64" height="64" rx="20" fill="#0d1a24"/>
      <circle cx="32" cy="32" r="22" fill="#1b2d3b" stroke="#8fd8d0" stroke-width="2"/>
      <rect x="22" y="21" width="20" height="18" rx="5" fill="#d7f2f6"/>
      <rect x="26" y="28" width="4" height="4" fill="#10222d"/>
      <rect x="34" y="28" width="4" height="4" fill="#10222d"/>
      <rect x="28" y="35" width="8" height="2" fill="#2f4e60"/>
      <rect x="19" y="17" width="8" height="4" fill="#5bd2c3"/>
      <rect x="37" y="17" width="8" height="4" fill="#5bd2c3"/>
    </svg>
  `;

  return `data:image/svg+xml;base64,${Buffer.from(svg).toString("base64")}`;
}

function pushAction(title, detail, level = "info") {
  appState.recentActions = [
    {
      id: `action-${Date.now()}`,
      title,
      detail,
      time: new Date().toLocaleTimeString("zh-CN", {
        hour: "2-digit",
        minute: "2-digit"
      }),
      level
    },
    ...appState.recentActions
  ].slice(0, 18);
}

function addMessage(role, text) {
  appState.chatHistory = [
    ...appState.chatHistory,
    {
      id: `msg-${Date.now()}-${Math.random().toString(16).slice(2, 7)}`,
      role,
      text,
      timestamp: new Date().toLocaleTimeString("zh-CN", {
        hour: "2-digit",
        minute: "2-digit"
      })
    }
  ];
}

function getCollapsedSize() {
  return COLLAPSED_SIZE[appState.size] || COLLAPSED_SIZE.default;
}

function getNearestDisplay(bounds) {
  return screen.getDisplayMatching(bounds);
}

function getDefaultCollapsedBounds() {
  const size = getCollapsedSize();
  const primary = screen.getPrimaryDisplay().workArea;
  const x = Math.round(primary.x + primary.width - size - FLOAT_GAP);
  const y = Math.round(primary.y + primary.height * 0.28);
  return {
    x,
    y,
    width: size,
    height: size
  };
}

function getCollapsedAnchorBounds() {
  return orbState.collapsedBounds || getDefaultCollapsedBounds();
}

function clampBounds(bounds, workArea) {
  const x = Math.min(Math.max(bounds.x, workArea.x), workArea.x + workArea.width - bounds.width);
  const y = Math.min(Math.max(bounds.y, workArea.y), workArea.y + workArea.height - bounds.height);
  return {
    ...bounds,
    x: Math.round(x),
    y: Math.round(y)
  };
}

function clampValue(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function getDistanceToEdges(bounds, workArea) {
  return {
    left: Math.abs(bounds.x - workArea.x),
    right: Math.abs(workArea.x + workArea.width - (bounds.x + bounds.width)),
    top: Math.abs(bounds.y - workArea.y),
    bottom: Math.abs(workArea.y + workArea.height - (bounds.y + bounds.height))
  };
}

function inferDockSide(bounds) {
  const display = getNearestDisplay(bounds);
  const workArea = display.workArea;
  const distances = getDistanceToEdges(bounds, workArea);
  return Object.entries(distances).sort((a, b) => a[1] - b[1])[0][0];
}

function snapBounds(bounds) {
  const display = getNearestDisplay(bounds);
  const workArea = display.workArea;
  const distances = getDistanceToEdges(bounds, workArea);
  const nearest = Object.entries(distances).sort((a, b) => a[1] - b[1])[0];
  const snapped = {
    ...bounds
  };

  if (nearest[1] <= SNAP_THRESHOLD) {
    const [side] = nearest;
    orbState.dockSide = side;
    if (side === "left") {
      snapped.x = workArea.x;
    }
    if (side === "right") {
      snapped.x = workArea.x + workArea.width - bounds.width;
    }
    if (side === "top") {
      snapped.y = workArea.y;
    }
    if (side === "bottom") {
      snapped.y = workArea.y + workArea.height - bounds.height;
    }
  } else {
    orbState.dockSide = inferDockSide(bounds);
  }

  return clampBounds(snapped, workArea);
}

function getOrbCenter() {
  const bounds = getCollapsedAnchorBounds();
  return {
    x: bounds.x + bounds.width / 2,
    y: bounds.y + bounds.height / 2
  };
}

function getExpandedBounds() {
  const anchor = getOrbCenter();
  const width = RING_WINDOW.width;
  const height = RING_WINDOW.height;
  let anchorX = Math.round(width / 2);
  let anchorY = Math.round(height / 2);

  if (orbState.dockSide === "left") {
    anchorX = EXPANDED_SAFE_INSET;
  }
  if (orbState.dockSide === "right") {
    anchorX = width - EXPANDED_SAFE_INSET;
  }
  if (orbState.dockSide === "top") {
    anchorY = EXPANDED_SAFE_INSET;
  }
  if (orbState.dockSide === "bottom") {
    anchorY = height - EXPANDED_SAFE_INSET;
  }

  const x = Math.round(anchor.x - anchorX);
  const y = Math.round(anchor.y - anchorY);

  return {
    x,
    y,
    width,
    height,
    anchorX,
    anchorY
  };
}

function getOrbLayout() {
  const collapsed = getCollapsedAnchorBounds();
  const size = getCollapsedSize();
  const currentBounds = orbState.mode === "ring" ? orbState.expandedBounds || getExpandedBounds() : collapsed;
  const isExpanded = orbState.mode === "ring";
  const anchorX = isExpanded ? currentBounds.anchorX : Math.round(currentBounds.width / 2);
  const anchorY = isExpanded ? currentBounds.anchorY : Math.round(currentBounds.height / 2);

  return {
    mode: orbState.mode,
    dockSide: orbState.dockSide,
    peeked: orbState.peeked,
    hidden: orbState.hidden,
    previewDockSide: orbState.previewDockSide,
    collapsedSize: size,
    anchorX,
    anchorY,
    width: currentBounds.width,
    height: currentBounds.height
  };
}

function getStatusLabel() {
  if (appState.agentPaused) {
    return "暂停";
  }
  if (appState.dnd) {
    return "请勿打扰";
  }
  if (appState.status === "running") {
    return "运行中";
  }
  if (appState.status === "thinking") {
    return "思考中";
  }
  if (appState.status === "needs-user") {
    return "等待确认";
  }
  if (appState.status === "error") {
    return "错误";
  }
  return "空闲";
}

function getOrbTasks() {
  return baseTasks.map((task) => {
    if (task.id !== "context") {
      return {
        ...task,
        enabled: task.id === "summarize" || task.id === "translate"
          ? appState.context.selectedTextAvailable && appState.abilityToggles.selection
          : task.id === "clipboard"
            ? appState.context.clipboardReady && appState.abilityToggles.clipboard
            : true
      };
    }

    const enabled = appState.context.selectedTextAvailable && appState.abilityToggles.selection;
    return {
      ...task,
      enabled,
      description: enabled
        ? "从选中的片段里抽出可执行的行动项。"
        : "当前没有可用选区，暂时无法抽取待办。",
      availability: enabled ? "检测到文本选区" : "未检测到选区",
      hint: enabled ? "点击运行" : "不可用"
    };
  });
}

function getSnapshot() {
  return {
    state: {
      ...appState,
      statusLabel: getStatusLabel()
    },
    tasks: getOrbTasks(),
    taskLibrary,
    layout: getOrbLayout(),
    chatVisible: Boolean(chatWindow && chatWindow.isVisible()),
    panelVisible: Boolean(panelWindow && panelWindow.isVisible())
  };
}

function sendSnapshot(targetWindow) {
  if (!targetWindow || targetWindow.isDestroyed()) {
    return;
  }

  targetWindow.webContents.send("snapshot", getSnapshot());
}

function broadcastSnapshot() {
  sendSnapshot(mainWindow);
  sendSnapshot(chatWindow);
  sendSnapshot(panelWindow);
}

function markActivity() {
  orbState.lastInteractionAt = Date.now();
  if (orbState.peeked) {
    exitPeekMode();
  }
}

function setMainWindowBounds(bounds) {
  if (!mainWindow || mainWindow.isDestroyed()) {
    return;
  }

  mainWindow.setBounds(
    {
      x: Math.round(bounds.x),
      y: Math.round(bounds.y),
      width: Math.round(bounds.width),
      height: Math.round(bounds.height)
    },
    true
  );
}

function applyCollapsedBounds(bounds) {
  const snapped = snapBounds(bounds);
  orbState.collapsedBounds = snapped;
  orbState.expandedBounds = null;
  orbState.mode = "collapsed";
  orbState.previewDockSide = null;
  setMainWindowBounds(snapped);
  positionChatWindow();
  broadcastSnapshot();
}

function expandToRing() {
  if (!mainWindow || orbState.hidden) {
    return;
  }

  markActivity();
  const expanded = getExpandedBounds();
  orbState.mode = "ring";
  orbState.expandedBounds = expanded;
  setMainWindowBounds(expanded);
  broadcastSnapshot();
}

function collapseOrb() {
  if (!mainWindow || orbState.hidden) {
    return;
  }

  markActivity();
  orbState.mode = "collapsed";
  orbState.expandedBounds = null;
  setMainWindowBounds(getCollapsedAnchorBounds());
  broadcastSnapshot();
}

function enterPeekMode() {
  if (!mainWindow || orbState.peeked || orbState.hidden) {
    return;
  }
  if (appState.autoHideMode === "off" || orbState.mode !== "collapsed") {
    return;
  }
  if (chatWindow && chatWindow.isVisible()) {
    return;
  }

  const bounds = getCollapsedAnchorBounds();
  const peekBounds = {
    ...bounds
  };

  if (orbState.dockSide === "left") {
    peekBounds.x = bounds.x - bounds.width + PEEK_VISIBLE;
  }
  if (orbState.dockSide === "right") {
    peekBounds.x = bounds.x + bounds.width - PEEK_VISIBLE;
  }
  if (orbState.dockSide === "top") {
    peekBounds.y = bounds.y - bounds.height + PEEK_VISIBLE;
  }
  if (orbState.dockSide === "bottom") {
    peekBounds.y = bounds.y + bounds.height - PEEK_VISIBLE;
  }

  orbState.peeked = true;
  setMainWindowBounds(peekBounds);
  broadcastSnapshot();
}

function exitPeekMode() {
  if (!mainWindow || !orbState.peeked) {
    return;
  }

  orbState.peeked = false;
  setMainWindowBounds(getCollapsedAnchorBounds());
  broadcastSnapshot();
}

function positionChatWindow() {
  if (!chatWindow || chatWindow.isDestroyed()) {
    return;
  }

  const anchor = getCollapsedAnchorBounds();
  const display = getNearestDisplay(anchor).workArea;
  let x;
  let y = Math.round(anchor.y + anchor.height / 2 - CHAT_WINDOW.height / 2);

  if (orbState.dockSide === "left") {
    x = anchor.x + anchor.width + 14;
  } else if (orbState.dockSide === "right") {
    x = anchor.x - CHAT_WINDOW.width - 14;
  } else {
    const prefersRight = anchor.x + anchor.width / 2 < display.x + display.width / 2;
    x = prefersRight ? anchor.x + anchor.width + 14 : anchor.x - CHAT_WINDOW.width - 14;
  }

  if (orbState.dockSide === "top") {
    y = anchor.y + anchor.height + 12;
  }
  if (orbState.dockSide === "bottom") {
    y = anchor.y - CHAT_WINDOW.height - 12;
  }

  x = Math.min(Math.max(x, display.x + 12), display.x + display.width - CHAT_WINDOW.width - 12);
  y = Math.min(Math.max(y, display.y + 12), display.y + display.height - CHAT_WINDOW.height - 12);

  chatWindow.setBounds(
    {
      x: Math.round(x),
      y: Math.round(y),
      width: CHAT_WINDOW.width,
      height: CHAT_WINDOW.height
    },
    false
  );
}

function createMainWindow() {
  const bounds = getDefaultCollapsedBounds();
  orbState.collapsedBounds = bounds;

  mainWindow = new BrowserWindow({
    ...bounds,
    show: false,
    frame: false,
    transparent: true,
    backgroundColor: "#00000000",
    resizable: false,
    minimizable: false,
    maximizable: false,
    hasShadow: false,
    skipTaskbar: true,
    roundedCorners: false,
    alwaysOnTop: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      additionalArguments: ["--window=orb"],
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  mainWindow.setAlwaysOnTop(true, "screen-saver");
  mainWindow.loadFile(path.join(__dirname, "src", "renderer", "orb.html"));
  mainWindow.once("ready-to-show", () => {
    if (!orbState.hidden) {
      mainWindow.showInactive();
      broadcastSnapshot();
    }
  });
  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

function createChatWindow() {
  if (chatWindow && !chatWindow.isDestroyed()) {
    return chatWindow;
  }

  chatWindow = new BrowserWindow({
    width: CHAT_WINDOW.width,
    height: CHAT_WINDOW.height,
    show: false,
    frame: false,
    transparent: false,
    backgroundColor: "#08141e",
    resizable: false,
    minimizable: false,
    maximizable: false,
    skipTaskbar: true,
    alwaysOnTop: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      additionalArguments: ["--window=chat"],
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  chatWindow.setAlwaysOnTop(true, "floating");
  chatWindow.loadFile(path.join(__dirname, "src", "renderer", "chat.html"));
  chatWindow.on("close", (event) => {
    if (appIsQuitting) {
      return;
    }
    event.preventDefault();
    chatWindow.hide();
    broadcastSnapshot();
  });
  chatWindow.on("closed", () => {
    chatWindow = null;
  });
  chatWindow.on("show", () => {
    positionChatWindow();
    broadcastSnapshot();
  });
  chatWindow.on("hide", () => {
    broadcastSnapshot();
  });

  return chatWindow;
}

function createPanelWindow() {
  if (panelWindow && !panelWindow.isDestroyed()) {
    return panelWindow;
  }

  panelWindow = new BrowserWindow({
    width: PANEL_WINDOW.width,
    height: PANEL_WINDOW.height,
    minWidth: 920,
    minHeight: 640,
    show: false,
    frame: false,
    backgroundColor: "#0b1520",
    titleBarStyle: "hidden",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      additionalArguments: ["--window=panel"],
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  panelWindow.loadFile(path.join(__dirname, "src", "renderer", "panel.html"));
  panelWindow.on("closed", () => {
    panelWindow = null;
    broadcastSnapshot();
  });

  return panelWindow;
}

function ensureChatWindow(options = {}) {
  const target = createChatWindow();
  positionChatWindow();
  target.show();
  if (options.focus !== false) {
    target.focus();
  }
  broadcastSnapshot();
}

function openPanel() {
  const target = createPanelWindow();
  target.center();
  target.show();
  target.focus();
  broadcastSnapshot();
}

function hideOrbWindow() {
  orbState.hidden = true;
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.hide();
  }
  if (chatWindow && !chatWindow.isDestroyed()) {
    chatWindow.hide();
  }
  broadcastSnapshot();
}

function showOrbWindow() {
  orbState.hidden = false;
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.showInactive();
  }
  broadcastSnapshot();
}

function normalizePreferencePatch(patch) {
  if (Object.prototype.hasOwnProperty.call(patch, "chatPinned")) {
    appState.chatPinned = Boolean(patch.chatPinned);
    if (chatWindow && !chatWindow.isDestroyed()) {
      chatWindow.setAlwaysOnTop(appState.chatPinned, appState.chatPinned ? "screen-saver" : "floating");
    }
  }
}

function updatePreferences(patch) {
  normalizePreferencePatch(patch);
  if (patch.size) {
    appState.size = patch.size;
  }
  if (patch.autoHideMode) {
    appState.autoHideMode = patch.autoHideMode;
    if (patch.autoHideMode === "off") {
      exitPeekMode();
    }
  }
  if (Object.prototype.hasOwnProperty.call(patch, "lowProfile")) {
    appState.lowProfile = Boolean(patch.lowProfile);
  }
  if (Object.prototype.hasOwnProperty.call(patch, "dnd")) {
    appState.dnd = Boolean(patch.dnd);
  }
  if (Object.prototype.hasOwnProperty.call(patch, "agentPaused")) {
    appState.agentPaused = Boolean(patch.agentPaused);
  }
  if (patch.theme) {
    appState.theme = {
      ...appState.theme,
      ...patch.theme
    };
  }
  if (patch.abilityToggles) {
    appState.abilityToggles = {
      ...appState.abilityToggles,
      ...patch.abilityToggles
    };
  }
  if (patch.context) {
    appState.context = {
      ...appState.context,
      ...patch.context
    };
  }

  const targetBounds = {
    ...getCollapsedAnchorBounds(),
    width: getCollapsedSize(),
    height: getCollapsedSize()
  };
  applyCollapsedBounds(targetBounds);
  pushAction("更新设置", "面板中的偏好已应用到当前演示。");
  broadcastSnapshot();
}

function createContextMenu(position) {
  const menu = Menu.buildFromTemplate([
    {
      label: `状态：${getStatusLabel()}`,
      enabled: false
    },
    {
      type: "separator"
    },
    {
      label: "暂停 Agent",
      type: "checkbox",
      checked: appState.agentPaused,
      click: () => {
        appState.agentPaused = !appState.agentPaused;
        appState.status = appState.agentPaused ? "idle" : appState.currentTask ? "running" : "idle";
        appState.statusNote = appState.agentPaused ? "已暂停所有主动动作" : "已恢复响应";
        pushAction(appState.agentPaused ? "暂停 Agent" : "恢复 Agent", appState.statusNote);
        broadcastSnapshot();
      }
    },
    {
      label: "请勿打扰",
      type: "checkbox",
      checked: appState.dnd,
      click: () => {
        appState.dnd = !appState.dnd;
        appState.statusNote = appState.dnd ? "进入低打扰状态" : "恢复正常可见度";
        pushAction(appState.dnd ? "开启 DND" : "关闭 DND", appState.statusNote);
        broadcastSnapshot();
      }
    },
    {
      label: "低存在感模式",
      type: "checkbox",
      checked: appState.lowProfile,
      click: () => {
        appState.lowProfile = !appState.lowProfile;
        appState.statusNote = appState.lowProfile ? "已降低亮度与动效" : "已恢复标准存在感";
        pushAction("切换低存在感模式", appState.statusNote);
        broadcastSnapshot();
      }
    },
    {
      type: "separator"
    },
    {
      label: "打开聊天",
      click: () => ensureChatWindow()
    },
    {
      label: "打开控制面板",
      click: () => openPanel()
    },
    {
      label: "状态",
      submenu: [
        {
          label: "设为：空闲",
          click: () => {
            appState.status = "idle";
            appState.statusNote = "等待下一次触发";
            appState.currentTask = null;
            appState.progress = 0;
            appState.progressLabel = "";
            broadcastSnapshot();
          }
        },
        {
          label: "设为：请勿打扰",
          click: () => {
            appState.dnd = true;
            appState.statusNote = "由你手动切换为低打扰状态";
            broadcastSnapshot();
          }
        },
        {
          label: "设为：暂停",
          click: () => {
            appState.agentPaused = true;
            appState.statusNote = "等待你重新恢复";
            broadcastSnapshot();
          }
        }
      ]
    },
    {
      label: "外观",
      submenu: [
        {
          label: "尺寸：小",
          type: "radio",
          checked: appState.size === "small",
          click: () => updatePreferences({ size: "small" })
        },
        {
          label: "尺寸：默认",
          type: "radio",
          checked: appState.size === "default",
          click: () => updatePreferences({ size: "default" })
        },
        {
          label: "尺寸：大",
          type: "radio",
          checked: appState.size === "large",
          click: () => updatePreferences({ size: "large" })
        },
        {
          type: "separator"
        },
        {
          label: "自动隐藏：关闭",
          type: "radio",
          checked: appState.autoHideMode === "off",
          click: () => updatePreferences({ autoHideMode: "off" })
        },
        {
          label: "自动隐藏：探出",
          type: "radio",
          checked: appState.autoHideMode === "peek",
          click: () => updatePreferences({ autoHideMode: "peek" })
        },
        {
          label: "自动隐藏：智能",
          type: "radio",
          checked: appState.autoHideMode === "smart",
          click: () => updatePreferences({ autoHideMode: "smart" })
        }
      ]
    },
    {
      type: "separator"
    },
    {
      label: "隐藏悬浮球",
      click: () => hideOrbWindow()
    },
    {
      label: "退出程序",
      click: () => {
        appIsQuitting = true;
        app.quit();
      }
    }
  ]);

  menu.popup({
    window: mainWindow,
    x: Math.round(position.x),
    y: Math.round(position.y)
  });
}

function createTray() {
  const icon = nativeImage
    .createFromDataURL(createOrbIconDataUrl())
    .resize({ width: 16, height: 16 });
  tray = new Tray(icon);
  tray.setToolTip("像素悬浮球");
  tray.on("click", () => {
    showOrbWindow();
    ensureChatWindow({ focus: false });
  });
  tray.on("right-click", () => {
    const trayMenu = Menu.buildFromTemplate([
      {
        label: "显示悬浮球",
        click: () => showOrbWindow()
      },
      {
        label: "打开聊天",
        click: () => ensureChatWindow()
      },
      {
        label: "打开控制面板",
        click: () => openPanel()
      },
      {
        type: "separator"
      },
      {
        label: "退出程序",
        click: () => {
          appIsQuitting = true;
          app.quit();
        }
      }
    ]);
    tray.popUpContextMenu(trayMenu);
  });
}

function getTaskById(taskId) {
  return [...getOrbTasks(), ...taskLibrary].find((task) => task.id === taskId);
}

function finishTask(task, resultText, options = {}) {
  clearInterval(progressTimer);
  progressTimer = null;
  appState.progress = 1;
  appState.progressLabel = options.completeLabel || "已完成";
  appState.status = options.finalStatus || "needs-user";
  appState.currentTask = null;
  appState.statusNote = options.note || `${task.title} 已准备好`;
  addMessage("assistant", resultText);
  pushAction("任务完成", appState.statusNote);
  broadcastSnapshot();

  setTimeout(() => {
    if (appState.currentTask || appState.agentPaused) {
      return;
    }
    appState.status = "idle";
    appState.progress = 0;
    appState.progressLabel = "";
    appState.statusNote = "一次提醒后已停止打扰";
    broadcastSnapshot();
  }, 2400);
}

function simulateTask(taskId, source = "orb") {
  const task = getTaskById(taskId);
  if (!task) {
    return;
  }

  if (appState.agentPaused) {
    ensureChatWindow({ focus: false });
    addMessage("assistant", "当前处于暂停状态。你可以先从右键菜单或控制面板恢复我，再继续执行任务。");
    pushAction("任务被拦截", "当前是暂停状态，未启动新的动作。", "warn");
    broadcastSnapshot();
    return;
  }

  if (task.enabled === false) {
    ensureChatWindow({ focus: false });
    addMessage("assistant", `${task.title} 现在还不可用，因为 ${task.availability}。`);
    pushAction("任务不可用", `${task.title} 缺少上下文条件。`, "warn");
    broadcastSnapshot();
    return;
  }

  if (taskId === "ask") {
    ensureChatWindow();
    addMessage("assistant", "我在这儿。你可以直接问一个短问题，也可以点下方的快捷动作继续。");
    pushAction("打开聊天", `${source === "orb" ? "从悬浮球" : "从其他入口"}进入简短对话。`);
    broadcastSnapshot();
    return;
  }

  const scripts = {
    summarize: {
      progressLabel: "正在总结选中文本",
      intro: "收到，我先把选中的内容压成 3 个重点，尽量保持原来的产品语气。",
      result: "演示摘要:\n1. 悬浮球以像素角色承载状态表达，核心是低存在感而不是抢焦点。\n2. 悬停会展开固定方向的任务环，帮助用户在 1 秒内触发常用动作。\n3. 左键聊天、右键菜单、控制面板共同构成轻量但完整的系统入口。",
      note: "总结结果已准备好"
    },
    translate: {
      progressLabel: "正在翻译当前内容",
      intro: "我先按自然语气处理一下，避免翻得太硬。",
      result: "Demo translation:\nThe floating orb acts like a calm pixel companion. It stays at the edge of your attention, surfaces progress softly, and only expands when you intentionally engage with it.",
      note: "翻译结果已准备好"
    },
    explain: {
      progressLabel: "正在解释错误信息",
      intro: `我先把这条报错拆开讲:\n${appState.context.errorSnippet}`,
      result: "这类错误通常意味着某个对象本该存在，但实际拿到的是 undefined。演示里最可能的原因是状态还没初始化完成，就提前读取了 status 字段。可以先检查数据注入时机，再补一层可选链或默认值。",
      note: "错误解释已整理好"
    },
    clipboard: {
      progressLabel: "正在整理剪贴板",
      intro: `我看到了剪贴板里的内容:\n${appState.context.clipboardText}`,
      result: "我从剪贴板里提取到 3 个动作:\n- 更新首页文案\n- 检查导出逻辑\n- 回复设计评审\n如果你愿意，我可以接着把它改成今日计划或提醒列表。",
      note: "剪贴板整理完成"
    },
    compare: {
      progressLabel: "正在整理比较项",
      intro: "我先按常见的决策维度帮你搭一个比较框架。",
      result: "快速比较:\n- 方案 A: 上手快，适合现在直接演示。\n- 方案 B: 扩展性更强，但首版实现更重。\n建议先用 A 跑通体验，再把 B 需要的可配置能力放进控制面板。",
      note: "比较结果已准备好"
    },
    context: {
      progressLabel: "正在提取待办",
      intro: `我会从这段选区中抽取行动项:\n${appState.context.currentSelection}`,
      result: "提取到的待办:\n- 定义悬浮球的状态切换\n- 固化任务环的方向记忆\n- 保持聊天弹层轻量而可关闭",
      note: "待办列表已提取"
    },
    rename: {
      progressLabel: "正在生成重命名规则",
      intro: "我先给你一个安全的预览方案，不直接改任何文件。",
      result: "演示重命名方案:\n- meeting-notes-01.md\n- meeting-notes-02.md\n- meeting-notes-03.md\n你可以再决定是否按日期、项目名或序号规则继续。",
      note: "重命名预览已准备好"
    },
    convert: {
      progressLabel: "正在转换格式",
      intro: "我会先保留原始信息，再换一个更适合分享的格式。",
      result: "演示输出已改成 Markdown 卡片格式，适合贴到文档或 IM 里。接下来也可以转换成邮件摘要、表格或日报格式。",
      note: "格式转换已完成"
    },
    reminder: {
      progressLabel: "正在创建提醒",
      intro: "我先把这件事压成一条简短提醒。",
      result: "提醒已创建（演示）:\n今天 18:00 前回看导出流程，并确认悬浮球的自动隐藏节奏。",
      note: "提醒已生成"
    },
    search: {
      progressLabel: "正在整理搜索入口",
      intro: "我先把搜索词改写得更利于命中结果。",
      result: "推荐搜索串:\nElectron transparent BrowserWindow draggable floating widget radial menu demo\n如果你愿意，我还可以继续拆成中文关键词版本。",
      note: "搜索入口已整理"
    }
  };

  const script = scripts[taskId] || scripts.ask;

  ensureChatWindow({ focus: false });
  addMessage("assistant", script.intro);
  pushAction(task.title, `${source === "chat" ? "从聊天触发" : "从任务环触发"}，${script.progressLabel}`);
  appState.status = "running";
  appState.statusNote = script.progressLabel;
  appState.currentTask = taskId;
  appState.progress = 0.12;
  appState.progressLabel = script.progressLabel;
  broadcastSnapshot();

  const steps = [0.24, 0.41, 0.63, 0.82, 1];
  let stepIndex = 0;
  clearInterval(progressTimer);
  progressTimer = setInterval(() => {
    if (stepIndex >= steps.length) {
      finishTask(task, script.result, {
        note: script.note,
        completeLabel: "已完成",
        finalStatus: "needs-user"
      });
      return;
    }
    appState.progress = steps[stepIndex];
    appState.statusNote = steps[stepIndex] < 1 ? script.progressLabel : script.note;
    stepIndex += 1;
    broadcastSnapshot();
  }, 520);
}

function cancelCurrentTask() {
  if (!appState.currentTask) {
    return;
  }

  clearInterval(progressTimer);
  progressTimer = null;
  addMessage("assistant", "当前演示任务已取消，我会停在旁边，等你下一次明确触发。");
  pushAction("取消任务", "长任务状态已被用户终止。", "warn");
  appState.currentTask = null;
  appState.progress = 0;
  appState.progressLabel = "";
  appState.status = "idle";
  appState.statusNote = "已取消并停止继续提醒";
  broadcastSnapshot();
}

function generateReply(text) {
  if (/你好|hello|hi/i.test(text)) {
    return "你好，我会尽量用短句回答，避免把你的工作流拖走。";
  }
  if (/低存在感|打扰|安静/.test(text)) {
    return "可以把我理解成一个会呼吸的桌面入口。我会在边缘待命，只在你悬停、点击或明确召唤时升高存在感。";
  }
  if (/怎么用|使用方式|操作/.test(text)) {
    return "悬停打开任务环，左键进聊天，右键看系统菜单，控制面板里可以继续调尺寸、自动隐藏和任务配置。";
  }
  return "这是一条前端演示回复。我现在没有接后端，所以会优先给你结构化建议，或者把意图导向一个已做好的快捷任务。";
}

function handleChatMessage(text) {
  const content = text.trim();
  if (!content) {
    return;
  }

  markActivity();
  addMessage("user", content);
  pushAction("发送消息", content.slice(0, 28));
  broadcastSnapshot();

  if (/总结|摘要/.test(content)) {
    simulateTask("summarize", "chat");
    return;
  }
  if (/翻译|英文|中文/.test(content)) {
    simulateTask("translate", "chat");
    return;
  }
  if (/错误|报错|exception|bug/i.test(content)) {
    simulateTask("explain", "chat");
    return;
  }
  if (/待办|todo|行动项/i.test(content)) {
    simulateTask("context", "chat");
    return;
  }
  if (/比较|哪个好|区别/.test(content)) {
    simulateTask("compare", "chat");
    return;
  }

  appState.status = "thinking";
  appState.statusNote = "正在组织一句简短回复";
  broadcastSnapshot();

  setTimeout(() => {
    appState.status = "idle";
    appState.statusNote = "一次提醒后已停止打扰";
    addMessage("assistant", generateReply(content));
    broadcastSnapshot();
  }, 760);
}

function startProximityLoop() {
  clearInterval(proximityTimer);
  proximityTimer = setInterval(() => {
    if (!mainWindow || mainWindow.isDestroyed() || orbState.hidden) {
      return;
    }

    const now = Date.now();
    if (!orbState.peeked && appState.autoHideMode !== "off" && now - orbState.lastInteractionAt > INACTIVITY_TO_PEEK) {
      enterPeekMode();
    }

    if (!orbState.peeked) {
      return;
    }

    const cursor = screen.getCursorScreenPoint();
    const bounds = getCollapsedAnchorBounds();
    const distanceX = Math.max(bounds.x - cursor.x, 0, cursor.x - (bounds.x + bounds.width));
    const distanceY = Math.max(bounds.y - cursor.y, 0, cursor.y - (bounds.y + bounds.height));
    const distance = Math.sqrt(distanceX * distanceX + distanceY * distanceY);

    if (distance <= PEEK_REVEAL_DISTANCE) {
      exitPeekMode();
    }
  }, 360);
}

function registerIpc() {
  ipcMain.handle("app:bootstrap", (event) => {
    const senderWindow = BrowserWindow.fromWebContents(event.sender);
    const windowType = senderWindow === mainWindow ? "orb" : senderWindow === chatWindow ? "chat" : "panel";
    return {
      ...getSnapshot(),
      windowType
    };
  });

  ipcMain.on("presence:activity", () => {
    markActivity();
    broadcastSnapshot();
  });

  ipcMain.on("orb:expand", () => {
    expandToRing();
  });

  ipcMain.on("orb:collapse", () => {
    collapseOrb();
  });

  ipcMain.on("orb:drag-start", (_event, payload) => {
    markActivity();
    exitPeekMode();
    dragState = {
      pointerX: payload.screenX,
      pointerY: payload.screenY,
      bounds: getCollapsedAnchorBounds()
    };
  });

  ipcMain.on("orb:drag-move", (_event, payload) => {
    if (!dragState || !mainWindow) {
      return;
    }

    const width = getCollapsedSize();
    const height = getCollapsedSize();
    const candidate = {
      x: Math.round(dragState.bounds.x + payload.screenX - dragState.pointerX),
      y: Math.round(dragState.bounds.y + payload.screenY - dragState.pointerY),
      width,
      height
    };
    const clamped = clampBounds(candidate, getNearestDisplay(candidate).workArea);
    orbState.collapsedBounds = clamped;
    orbState.mode = "collapsed";
    setMainWindowBounds(clamped);
    const distances = getDistanceToEdges(clamped, getNearestDisplay(clamped).workArea);
    const nearest = Object.entries(distances).sort((a, b) => a[1] - b[1])[0];
    orbState.previewDockSide = nearest[1] <= SNAP_THRESHOLD ? nearest[0] : null;
    positionChatWindow();
    broadcastSnapshot();
  });

  ipcMain.on("orb:drag-end", () => {
    if (!dragState) {
      return;
    }
    dragState = null;
    applyCollapsedBounds(getCollapsedAnchorBounds());
  });

  ipcMain.on("orb:show-menu", (_event, payload) => {
    markActivity();
    createContextMenu(payload);
  });

  ipcMain.on("chat:open", (_event, payload) => {
    markActivity();
    ensureChatWindow({ focus: !payload || payload.focus !== false });
  });

  ipcMain.on("chat:close", () => {
    if (chatWindow && !chatWindow.isDestroyed()) {
      chatWindow.hide();
      broadcastSnapshot();
    }
  });

  ipcMain.on("panel:open", () => {
    openPanel();
  });

  ipcMain.on("task:run", (_event, payload) => {
    markActivity();
    simulateTask(payload.taskId, payload.source || "orb");
  });

  ipcMain.on("task:cancel", () => {
    cancelCurrentTask();
  });

  ipcMain.on("chat:send", (_event, payload) => {
    handleChatMessage(payload.text);
  });

  ipcMain.on("prefs:update", (_event, patch) => {
    markActivity();
    updatePreferences(patch);
  });

  ipcMain.on("panel:close", () => {
    if (panelWindow && !panelWindow.isDestroyed()) {
      panelWindow.close();
    }
  });
}

app.whenReady().then(() => {
  Menu.setApplicationMenu(null);
  registerIpc();
  createMainWindow();
  createTray();
  startProximityLoop();
});

app.on("before-quit", () => {
  appIsQuitting = true;
});

app.on("window-all-closed", (event) => {
  if (!appIsQuitting) {
    event.preventDefault();
  }
});
