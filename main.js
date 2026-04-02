const { app, BrowserWindow, Menu, Tray, ipcMain, nativeImage, screen, desktopCapturer, session } = require("electron");
const { execFile } = require("child_process");
const fs = require("fs");
const path = require("path");
const WebSocket = require("ws");
const {
  MEETING_SUMMARY_WINDOW_SECONDS,
  MEETING_CAPTION_HISTORY_MS,
  MEETING_CAPTION_MAX_ITEMS,
  MEETING_SOURCE_EMPTY_TEXT,
  MEETING_ALL_EMPTY_TEXT,
  createEmptyStructuredSummary,
  normalizeStructuredSummary,
  hasStructuredSummaryContent,
  trimCaptionItems,
  splitTranscriptBlocksBySource,
  buildSourceTranscriptText,
  parseStructuredSummaryResponse,
  createTranscriptSnippet
} = require("./src/shared/meeting-summary-utils");

const COLLAPSED_SIZE = {
  small: 88,
  default: 96,
  large: 108
};
const RING_WINDOW = {
  width: 420,
  height: 420
};
const MEETING_PROMPT_WINDOW = {
  width: 348,
  height: 208
};
const TASK_RING_RADIUS = 104;
const TASK_BALL_SIZE = 54;
const RING_SAFE_PADDING = 24;
const EXPANDED_SAFE_INSET = TASK_RING_RADIUS + Math.ceil(TASK_BALL_SIZE / 2) + RING_SAFE_PADDING;
const CHAT_WINDOW = {
  width: 392,
  height: 472
};
const MEETING_SUMMARY_WINDOW = {
  width: 388,
  height: 264,
  minWidth: 320,
  minHeight: 220
};
const MEETING_CAPTIONS_WINDOW = {
  width: 640,
  height: 360,
  minWidth: 420,
  minHeight: 260
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
const MEETING_SUMMARY_INTERVAL_MS = 2 * 60 * 1000;
const MEETING_TRANSCRIPT_INTERVAL_MS = 10 * 1000;
const MEETING_REALTIME_SAMPLE_RATE = 16000;
const MEETING_REALTIME_SNAPSHOT_THROTTLE_MS = 120;
const MEETING_REALTIME_RECONNECT_BASE_MS = 900;
const MEETING_REALTIME_MAX_RECONNECTS = 5;
const MEETING_REALTIME_FINALIZE_WAIT_MS = 650;
const MEETING_DETECTION_POLL_MS = 5000;
const MEETING_DETECTION_CANDIDATE_HOLD_MS = 20 * 1000;
const MEETING_DETECTION_EXIT_HOLD_MS = 60 * 1000;
const MEETING_DETECTION_SOURCE_IDLE_STOP_MS = 90 * 1000;
const MEETING_DETECTION_PERMISSION_COOLDOWN_MS = 5 * 60 * 1000;
const SETTINGS_FILE_NAME = "pixel-orb-settings.json";

let mainWindow;
let chatWindow;
let panelWindow;
let meetingSummaryWindow;
let meetingCaptionsWindow;
let tray;
let proximityTimer;
let progressTimer;
let dragState = null;
let appIsQuitting = false;
let meetingSnapshotBroadcastTimer = null;
let meetingDetectionTimer = null;
let foregroundWindowQueryPending = false;
let meetingSummaryOverlayPersistTimer = null;
let meetingCaptionsOverlayPersistTimer = null;
let meetingOverlayLayoutTimer = null;
let meetingOverlayLayoutForcePending = false;
let goBridgeBuildPromise = null;

const overlayWindowState = {
  summary: {
    bounds: null
  },
  captions: {
    bounds: null
  }
};

const modelConfigState = {
  transcription: {
    baseUrl: "",
    apiKey: "",
    model: ""
  },
  summary: {
    baseUrl: "",
    apiKey: "",
    model: ""
  }
};

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
  meetingDetection: createMeetingDetectionState(),
  meetingSummary: createMeetingSummaryState(),
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

function createMeetingSourceState(label, overrides = {}) {
  return {
    label,
    active: false,
    status: "idle",
    note: "\u7b49\u5f85\u8fde\u63a5",
    latestText: "",
    lastUpdatedAt: "",
    lastSignalAt: 0,
    chunkCount: 0,
    error: "",
    recentItems: [],
    ...overrides
  };
}

function createMeetingSummaryState(overrides = {}) {
  return {
    enabled: false,
    status: "idle",
    transcriptionMode: "chunk",
    startTrigger: "manual",
    startedAt: 0,
    captureRequestId: "",
    intervalMs: MEETING_SUMMARY_INTERVAL_MS,
    transcriptIntervalMs: MEETING_TRANSCRIPT_INTERVAL_MS,
    realtimeSampleRate: MEETING_REALTIME_SAMPLE_RATE,
    note: "\u70b9\u51fb\u201c\u4f1a\u8bae\u603b\u7ed3\u201d\u540e\uff0c\u7cfb\u7edf\u4f1a\u5c1d\u8bd5\u540c\u65f6\u6293\u53d6\u7cfb\u7edf\u97f3\u9891\u548c\u9ea6\u514b\u98ce\u3002",
    latestSummary: createEmptyStructuredSummary(),
    latestTranscript: "",
    error: "",
    lastUpdatedAt: "",
    chunkCount: 0,
    pendingTranscriptBlocks: [],
    sources: {
      system: createMeetingSourceState("\u7cfb\u7edf\u97f3\u9891"),
      microphone: createMeetingSourceState("\u9ea6\u514b\u98ce")
    },
    ...overrides
  };
}

function createMeetingDetectionState(overrides = {}) {
  return {
    enabled: true,
    status: "idle",
    candidateApp: "",
    candidateTitle: "",
    matchedSignature: "",
    promptVisible: false,
    suppressedForSession: false,
    cooldownUntil: 0,
    lastSeenAt: 0,
    candidateSince: 0,
    absentSince: 0,
    promptShownAt: 0,
    note: "\u5f00\u542f\u540e\u4f1a\u5728\u68c0\u6d4b\u5230\u4f1a\u8bae\u8f6f\u4ef6\u65f6\u63d0\u9192\u4f60\u542f\u52a8\u4f1a\u8bae\u603b\u7ed3\u3002",
    ...overrides
  };
}

function createMeetingRealtimeConnectionState(source) {
  return {
    source,
    socket: null,
    ready: false,
    initialized: false,
    closedByClient: false,
    pendingAudioChunks: [],
    reconnectAttempts: 0,
    reconnectTimer: null,
    lastError: ""
  };
}

const meetingRealtimeConnections = {
  system: createMeetingRealtimeConnectionState("system"),
  microphone: createMeetingRealtimeConnectionState("microphone")
};

const MEETING_APP_SIGNATURES = [
  {
    id: "tencent-meeting",
    label: "\u817e\u8baf\u4f1a\u8bae",
    processNames: ["wemeetapp", "wemeeting", "voovmeeting", "voovmeetingapp"],
    titleKeywords: ["\u817e\u8baf\u4f1a\u8bae", "voov meeting", "wemeet"]
  },
  {
    id: "lark-meeting",
    label: "\u98de\u4e66\u4f1a\u8bae",
    processNames: ["feishu", "lark", "feishumeeting"],
    titleKeywords: ["\u98de\u4e66\u4f1a\u8bae", "lark meetings", "feishu meetings", "\u98de\u4e66"]
  },
  {
    id: "zoom",
    label: "Zoom",
    processNames: ["zoom", "zoom workplace", "zoomrooms"],
    titleKeywords: ["zoom meeting", "zoom workplace", "zoom"]
  },
  {
    id: "teams",
    label: "Microsoft Teams",
    processNames: ["teams", "ms-teams", "msteams"],
    titleKeywords: ["microsoft teams", "teams meeting", "teams"]
  },
  {
    id: "dingtalk-meeting",
    label: "\u9489\u9489\u4f1a\u8bae",
    processNames: ["dingtalk"],
    titleKeywords: ["\u9489\u9489\u4f1a\u8bae", "\u9489\u9489"]
  },
  {
    id: "wecom-meeting",
    label: "\u4f01\u4e1a\u5fae\u4fe1\u4f1a\u8bae",
    processNames: ["wxwork", "wecom"],
    titleKeywords: ["\u4f01\u4e1a\u5fae\u4fe1\u4f1a\u8bae", "\u4f01\u5fae\u4f1a\u8bae", "wecom"]
  },
  {
    id: "google-meet",
    label: "Google Meet",
    processNames: ["chrome", "msedge", "firefox", "brave", "opera"],
    titleKeywords: ["google meet", "meet.google.com", "google \u4f1a\u8bae"]
  }
];

function formatClockTime() {
  return new Date().toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit"
  });
}

function getMeetingSourceLabel(source) {
  return source === "microphone" ? "\u9ea6\u514b\u98ce" : "\u7cfb\u7edf\u97f3\u9891";
}

function getMeetingSourceStatusLabel(sourceState = {}) {
  if (sourceState.status === "starting") {
    return "\u6b63\u5728\u8fde\u63a5";
  }
  if (sourceState.status === "processing") {
    return "\u6b63\u5728\u8f6c\u5199";
  }
  if (sourceState.status === "listening") {
    return sourceState.active ? "\u5b9e\u65f6\u8f6c\u5199\u4e2d" : "\u5f85\u547d";
  }
  if (sourceState.status === "error") {
    return "\u8f93\u5165\u5931\u8d25";
  }
  if (sourceState.status === "stopped") {
    return "\u5df2\u505c\u6b62";
  }
  return "\u672a\u8fde\u63a5";
}

function getMeetingSourceCount() {
  return Object.values(appState.meetingSummary.sources || {}).filter((sourceState) => sourceState.active).length;
}

function getMeetingListeningNote() {
  const activeLabels = Object.entries(appState.meetingSummary.sources || {})
    .filter(([, sourceState]) => sourceState.active)
    .map(([source]) => getMeetingSourceLabel(source));
  const isRealtime = appState.meetingSummary.transcriptionMode === "realtime";

  if (!activeLabels.length) {
    return "\u6b63\u5728\u7b49\u5f85\u53ef\u7528\u7684\u97f3\u9891\u8f93\u5165\u6e90\u3002";
  }

  if (isRealtime) {
    return `\u5df2\u8fde\u63a5${activeLabels.join("\u3001")}\uff0c\u6b63\u5728\u8fde\u7eed\u6d41\u5f0f\u8f6c\u5199\uff0c\u6bcf 2 \u5206\u949f\u5237\u65b0\u4e00\u6b21\u4f1a\u8bae\u6982\u8981\u3002`;
  }

  return `\u5df2\u8fde\u63a5${activeLabels.join("\u3001")}\uff0c\u6bcf 10 \u79d2\u66f4\u65b0\u5b9e\u65f6\u8f6c\u5199\uff0c\u6bcf 2 \u5206\u949f\u5237\u65b0\u4e00\u6b21\u4f1a\u8bae\u6982\u8981\u3002`;
}

function getMeetingDetectionStatusLabel(detection = appState.meetingDetection) {
  if (!detection.enabled) {
    return "\u5df2\u5173\u95ed";
  }
  if (!appState.abilityToggles.activeApp) {
    return "\u5df2\u6682\u505c";
  }
  if (detection.status === "candidate") {
    return "\u68c0\u6d4b\u4e2d";
  }
  if (detection.status === "prompting") {
    return "\u5f85\u4f60\u786e\u8ba4";
  }
  if (detection.status === "running") {
    return "\u4f1a\u4e2d";
  }
  if (detection.status === "suppressed") {
    return "\u672c\u573a\u5df2\u5ffd\u7565";
  }
  return "\u5f85\u547d";
}

function getForegroundAppLabel(windowInfo = {}) {
  const title = typeof windowInfo.title === "string" ? windowInfo.title.trim() : "";
  const processName = typeof windowInfo.processName === "string" ? windowInfo.processName.trim() : "";
  if (title && processName) {
    return `${title} (${processName})`;
  }
  return title || processName || appState.context.activeApp;
}

function normalizeMatchText(value) {
  return String(value || "").trim().toLowerCase();
}

function matchMeetingApp(windowInfo = {}) {
  const processName = normalizeMatchText(windowInfo.processName);
  const title = normalizeMatchText(windowInfo.title);
  if (!processName && !title) {
    return null;
  }

  return MEETING_APP_SIGNATURES.find((signature) => {
    const processMatched = signature.processNames.some((keyword) => processName.includes(normalizeMatchText(keyword)));
    const titleMatched = signature.titleKeywords.some((keyword) => title.includes(normalizeMatchText(keyword)));
    if (signature.id === "google-meet") {
      return processMatched && titleMatched;
    }
    return processMatched || titleMatched;
  }) || null;
}

function hasMeetingSummaryRunningState() {
  return appState.meetingSummary.enabled || ["starting", "listening", "processing"].includes(appState.meetingSummary.status);
}

function clearMeetingDetectionPrompt(options = {}) {
  appState.meetingDetection = {
    ...appState.meetingDetection,
    promptVisible: false,
    promptShownAt: 0,
    status: options.keepStatus || "idle",
    note: options.note || appState.meetingDetection.note
  };
}

function resetMeetingDetectionSession(options = {}) {
  appState.meetingDetection = createMeetingDetectionState({
    enabled: appState.meetingDetection.enabled,
    cooldownUntil: options.keepCooldown ? appState.meetingDetection.cooldownUntil : 0,
    note: options.note || createMeetingDetectionState().note
  });
}

function suppressMeetingDetectionForCurrentSession(note) {
  appState.meetingDetection = {
    ...appState.meetingDetection,
    status: "suppressed",
    promptVisible: false,
    suppressedForSession: true,
    promptShownAt: 0,
    note: note || "\u672c\u573a\u4f1a\u8bae\u5df2\u5ffd\u7565\uff0c\u4f1a\u5728\u7ed3\u675f\u540e\u6062\u590d\u81ea\u52a8\u68c0\u6d4b\u3002"
  };
}

function setMeetingDetectionCooldown(reason, durationMs = MEETING_DETECTION_PERMISSION_COOLDOWN_MS) {
  const now = Date.now();
  appState.meetingDetection = {
    ...appState.meetingDetection,
    status: "suppressed",
    promptVisible: false,
    suppressedForSession: false,
    cooldownUntil: now + durationMs,
    promptShownAt: 0,
    note: reason || "\u81ea\u52a8\u63d0\u9192\u5df2\u6682\u505c\uff0c\u7a0d\u540e\u4f1a\u518d\u6b21\u5c1d\u8bd5\u3002"
  };
}

function getMeetingDetectionSourceIdleMs(now = Date.now()) {
  const sourceStates = Object.values(appState.meetingSummary.sources || {});
  if (!sourceStates.length) {
    return Number.POSITIVE_INFINITY;
  }

  const lastSignalAt = sourceStates.reduce((latest, sourceState) => {
    const current = Number(sourceState.lastSignalAt || 0);
    return current > latest ? current : latest;
  }, 0);

  if (!lastSignalAt) {
    return Number.POSITIVE_INFINITY;
  }

  return now - lastSignalAt;
}

function shouldCooldownAutoPromptOnFailure() {
  return appState.meetingSummary.startTrigger === "auto-prompt"
    && appState.meetingSummary.startedAt
    && (Date.now() - appState.meetingSummary.startedAt) < 30 * 1000
    && !appState.meetingSummary.chunkCount;
}

function updateMeetingSourceState(source, patch) {
  const current = appState.meetingSummary.sources?.[source] || createMeetingSourceState(getMeetingSourceLabel(source));
  appState.meetingSummary = {
    ...appState.meetingSummary,
    sources: {
      ...appState.meetingSummary.sources,
      [source]: {
        ...current,
        ...patch
      }
    }
  };
}

function sanitizeModelConfig(input = {}) {
  return {
    baseUrl: typeof input.baseUrl === "string" ? input.baseUrl.trim().replace(/\/+$/, "") : "",
    apiKey: typeof input.apiKey === "string" ? input.apiKey.trim() : "",
    model: typeof input.model === "string" ? input.model.trim() : ""
  };
}

function sanitizeModelConfigBundle(input = {}) {
  const source = input && typeof input === "object" ? input : {};

  if (source && (
    Object.prototype.hasOwnProperty.call(source, "baseUrl")
    || Object.prototype.hasOwnProperty.call(source, "apiKey")
    || Object.prototype.hasOwnProperty.call(source, "model")
  )) {
    const legacyConfig = sanitizeModelConfig(source);
    return {
      transcription: {
        ...legacyConfig
      },
      summary: {
        ...legacyConfig
      }
    };
  }

  return {
    transcription: sanitizeModelConfig(source.transcription),
    summary: sanitizeModelConfig(source.summary)
  };
}

function hasCompleteModelConfig(config) {
  return Boolean(config.baseUrl && config.apiKey && config.model);
}

function hasAnyModelConfigValue(config) {
  return Boolean(config.baseUrl || config.apiKey || config.model);
}

function getSettingsFilePath() {
  return path.join(app.getPath("userData"), SETTINGS_FILE_NAME);
}

function getPersistedSettingsPayload() {
  return {
    size: appState.size,
    autoHideMode: appState.autoHideMode,
    lowProfile: appState.lowProfile,
    dnd: appState.dnd,
    agentPaused: appState.agentPaused,
    chatPinned: appState.chatPinned,
    theme: {
      motionScale: appState.theme.motionScale,
      saturation: appState.theme.saturation,
      tooltipDelay: appState.theme.tooltipDelay
    },
    abilityToggles: {
      ...appState.abilityToggles
    },
    meetingAutoDetect: {
      enabled: appState.meetingDetection.enabled
    },
    meetingSummaryOverlay: {
      bounds: overlayWindowState.summary.bounds
    },
    meetingCaptionsOverlay: {
      bounds: overlayWindowState.captions.bounds
    },
    context: {
      selectedTextAvailable: appState.context.selectedTextAvailable,
      clipboardReady: appState.context.clipboardReady
    },
    modelConfig: {
      transcription: {
        ...modelConfigState.transcription
      },
      summary: {
        ...modelConfigState.summary
      }
    }
  };
}

function persistSettings() {
  try {
    const filePath = getSettingsFilePath();
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, JSON.stringify(getPersistedSettingsPayload(), null, 2), "utf8");
  } catch (error) {
    console.error("Failed to persist settings", error);
  }
}

function loadPersistedSettings() {
  try {
    const filePath = getSettingsFilePath();
    if (!fs.existsSync(filePath)) {
      return;
    }

    let shouldPersistMigration = false;
    const parsed = JSON.parse(fs.readFileSync(filePath, "utf8"));
    if (parsed.size && COLLAPSED_SIZE[parsed.size]) {
      appState.size = parsed.size;
    }
    if (["off", "peek", "smart"].includes(parsed.autoHideMode)) {
      appState.autoHideMode = parsed.autoHideMode;
    }
    if (Object.prototype.hasOwnProperty.call(parsed, "lowProfile")) {
      appState.lowProfile = Boolean(parsed.lowProfile);
    }
    if (Object.prototype.hasOwnProperty.call(parsed, "dnd")) {
      appState.dnd = Boolean(parsed.dnd);
    }
    if (Object.prototype.hasOwnProperty.call(parsed, "agentPaused")) {
      appState.agentPaused = Boolean(parsed.agentPaused);
    }
    if (Object.prototype.hasOwnProperty.call(parsed, "chatPinned")) {
      appState.chatPinned = Boolean(parsed.chatPinned);
    }
    if (parsed.theme && typeof parsed.theme === "object") {
      appState.theme = {
        ...appState.theme,
        ...parsed.theme
      };
    }
    if (parsed.abilityToggles && typeof parsed.abilityToggles === "object") {
      appState.abilityToggles = {
        ...appState.abilityToggles,
        ...parsed.abilityToggles
      };
    }
    if (parsed.meetingAutoDetect && typeof parsed.meetingAutoDetect === "object") {
      appState.meetingDetection = {
        ...appState.meetingDetection,
        enabled: !Object.prototype.hasOwnProperty.call(parsed.meetingAutoDetect, "enabled")
          || Boolean(parsed.meetingAutoDetect.enabled)
      };
    }
    if (parsed.meetingSummaryOverlay && typeof parsed.meetingSummaryOverlay === "object") {
      overlayWindowState.summary.bounds = sanitizeStoredOverlayBounds(
        parsed.meetingSummaryOverlay.bounds,
        MEETING_SUMMARY_WINDOW
      );
    }
    if (parsed.meetingCaptionsOverlay && typeof parsed.meetingCaptionsOverlay === "object") {
      overlayWindowState.captions.bounds = sanitizeStoredOverlayBounds(
        parsed.meetingCaptionsOverlay.bounds,
        MEETING_CAPTIONS_WINDOW
      );
    }
    if (appState.meetingDetection.enabled && !appState.abilityToggles.activeApp) {
      appState.abilityToggles = {
        ...appState.abilityToggles,
        activeApp: true
      };
      shouldPersistMigration = true;
    }
    if (parsed.context && typeof parsed.context === "object") {
      appState.context = {
        ...appState.context,
        ...parsed.context
      };
    }

    const normalizedConfig = sanitizeModelConfigBundle(parsed.modelConfig);
    Object.assign(modelConfigState.transcription, normalizedConfig.transcription);
    Object.assign(modelConfigState.summary, normalizedConfig.summary);
    if (shouldPersistMigration) {
      persistSettings();
    }
  } catch (error) {
    console.error("Failed to load settings", error);
  }
}

function getMeetingSummaryStatusLabel(summary = appState.meetingSummary) {
  if (summary.status === "starting") {
    return "\u6b63\u5728\u8fde\u63a5\u4f1a\u8bae\u97f3\u9891";
  }
  if (summary.status === "listening") {
    return "\u6b63\u5728\u8bc6\u522b\u4f1a\u8bae";
  }
  if (summary.status === "processing") {
    return "\u6b63\u5728\u751f\u6210\u6458\u8981";
  }
  if (summary.status === "error") {
    return "\u8bc6\u522b\u5931\u8d25";
  }
  if (summary.status === "stopped") {
    return "\u5df2\u505c\u6b62";
  }
  return "\u672a\u5f00\u59cb";
}

function normalizeRuntimeError(error, fallback = "\u53d1\u751f\u4e86\u672a\u77e5\u9519\u8bef") {
  if (!error) {
    return fallback;
  }
  if (typeof error === "string") {
    return error;
  }
  if (typeof error.message === "string" && error.message.trim()) {
    return error.message.trim();
  }
  return fallback;
}

function getApiErrorMessage(payload) {
  if (!payload) {
    return "";
  }
  if (typeof payload === "string") {
    return payload.trim();
  }
  if (payload.error && typeof payload.error.message === "string") {
    return payload.error.message.trim();
  }
  if (typeof payload.message === "string") {
    return payload.message.trim();
  }
  if (payload.code && typeof payload.code === "string") {
    return payload.code.trim();
  }
  return "";
}

async function parseApiResponse(response) {
  const contentType = response.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    return response.json();
  }

  const text = await response.text();
  try {
    return JSON.parse(text);
  } catch (_error) {
    return text;
  }
}

function buildApiUrl(baseUrl, route) {
  const normalizedBase = baseUrl.replace(/\/+$/, "");
  const normalizedRoute = route.startsWith("/") ? route : `/${route}`;
  return `${normalizedBase}${normalizedRoute}`;
}

function isDashScopeCompatibleBaseUrl(baseUrl) {
  return /dashscope(-intl)?\.aliyuncs\.com\/compatible-mode\/v1\/?$/i.test((baseUrl || "").trim());
}

function shouldUseRealtimeMeetingTranscription(config = modelConfigState.transcription) {
  const resolvedConfig = sanitizeModelConfig(config);
  return isDashScopeCompatibleBaseUrl(resolvedConfig.baseUrl) && /realtime/i.test(resolvedConfig.model);
}

function buildDashScopeRealtimeUrl(baseUrl, model) {
  const url = new URL(baseUrl);
  url.protocol = url.protocol === "http:" ? "ws:" : "wss:";
  url.pathname = "/api-ws/v1/realtime";
  url.search = `model=${encodeURIComponent(model)}`;
  url.hash = "";
  return url.toString();
}

function createRealtimeEventId(prefix) {
  return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
}

function buildInputAudioDataUri(mimeType, audioBase64) {
  return `data:${mimeType || "audio/webm"};base64,${audioBase64}`;
}

function buildHttpError(response, payload, fallback) {
  const detail = getApiErrorMessage(payload);
  const status = response?.status ? `HTTP ${response.status}` : "HTTP error";
  return new Error(detail ? `${fallback} (${status}): ${detail}` : `${fallback} (${status})`);
}

function extractAssistantText(payload) {
  const content = payload?.choices?.[0]?.message?.content;
  if (typeof content === "string") {
    return content.trim();
  }
  if (Array.isArray(content)) {
    return content
      .map((item) => {
        if (typeof item === "string") {
          return item;
        }
        if (typeof item?.text === "string") {
          return item.text;
        }
        return "";
      })
      .join("")
      .trim();
  }
  return "";
}

function extractRealtimeTranscriptText(payload) {
  const transcript = payload?.transcript;
  if (typeof transcript === "string") {
    return transcript.trim();
  }
  if (typeof transcript?.text === "string") {
    return transcript.text.trim();
  }
  if (typeof transcript?.transcript === "string") {
    return transcript.transcript.trim();
  }
  return "";
}

function extractRealtimeTranscriptStash(payload) {
  return typeof payload?.transcript?.stash === "string" ? payload.transcript.stash : "";
}

function getModelConfigError() {
  const missingGroups = [];
  if (!hasCompleteModelConfig(modelConfigState.transcription)) {
    missingGroups.push("\u8bed\u97f3\u8f6c\u5199\u914d\u7f6e");
  }
  if (!hasCompleteModelConfig(modelConfigState.summary)) {
    missingGroups.push("\u6587\u672c\u603b\u7ed3\u914d\u7f6e");
  }

  if (!missingGroups.length) {
    return "";
  }
  return `\u8bf7\u5148\u5728\u63a7\u5236\u9762\u677f\u4fdd\u5b58${missingGroups.join("\u548c")}\u3002`;
}

function getMeetingDetectionBlockReason() {
  if (!appState.meetingDetection.enabled) {
    return "\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\u5df2\u5173\u95ed\u3002";
  }
  if (!appState.abilityToggles.activeApp) {
    return "\u6d3b\u52a8\u5e94\u7528\u611f\u77e5\u5df2\u5173\u95ed\uff0c\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\u5df2\u6682\u505c\u3002";
  }
  const modelConfigError = getModelConfigError();
  if (modelConfigError) {
    return "\u81ea\u52a8\u8bc6\u522b\u5df2\u6682\u505c\uff0c\u9700\u5148\u8865\u5168\u4f1a\u8bae\u603b\u7ed3\u6a21\u578b\u914d\u7f6e\u3002";
  }
  return "";
}

function getGoBridgeBinaryPath() {
  return path.join(app.getPath("userData"), "go-bridge", `pixel-orb-core${process.platform === "win32" ? ".exe" : ""}`);
}

function ensureGoBridgeBinary() {
  if (goBridgeBuildPromise) {
    return goBridgeBuildPromise;
  }

  goBridgeBuildPromise = new Promise((resolve) => {
    const binaryPath = getGoBridgeBinaryPath();
    try {
      fs.mkdirSync(path.dirname(binaryPath), { recursive: true });
    } catch (_error) {
      resolve("");
      return;
    }

    execFile(
      "go",
      ["build", "-o", binaryPath, "./go-backend/cmd/pixel-orb-core"],
      {
        cwd: __dirname,
        windowsHide: true,
        timeout: 15000,
        maxBuffer: 512 * 1024
      },
      (error) => {
        if (error) {
          console.warn("Failed to build Go bridge, falling back to JS implementation.", error.message || error);
          resolve("");
          return;
        }
        resolve(binaryPath);
      }
    );
  });

  return goBridgeBuildPromise;
}

async function invokeGoBridge(command, payload) {
  const binaryPath = await ensureGoBridgeBinary();
  if (!binaryPath) {
    return null;
  }

  return new Promise((resolve) => {
    const child = execFile(
      binaryPath,
      [command],
      {
        cwd: __dirname,
        windowsHide: true,
        timeout: 5000,
        maxBuffer: 512 * 1024
      },
      (error, stdout) => {
        if (error) {
          resolve(null);
          return;
        }

        try {
          resolve(JSON.parse(String(stdout || "").trim() || "{}"));
        } catch (_parseError) {
          resolve(null);
        }
      }
    );

    if (payload === undefined) {
      child.stdin.end();
      return;
    }

    child.stdin.end(JSON.stringify(payload));
  });
}

function buildForegroundWindowScript() {
  return `
Add-Type @"
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class ForegroundWindowReader {
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowText(IntPtr hWnd, StringBuilder text, int count);
  [DllImport("user32.dll")] public static extern int GetWindowTextLength(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);
}
"@
$hwnd = [ForegroundWindowReader]::GetForegroundWindow()
if ($hwnd -eq [IntPtr]::Zero) {
  [pscustomobject]@{ title = ""; processName = ""; processId = 0 } | ConvertTo-Json -Compress
  exit 0
}
$titleLength = [ForegroundWindowReader]::GetWindowTextLength($hwnd)
$builder = New-Object System.Text.StringBuilder ($titleLength + 1)
[void][ForegroundWindowReader]::GetWindowText($hwnd, $builder, $builder.Capacity)
$processId = [uint32]0
[void][ForegroundWindowReader]::GetWindowThreadProcessId($hwnd, [ref]$processId)
$processName = ""
try {
  $process = Get-Process -Id $processId -ErrorAction Stop
  $processName = $process.ProcessName
} catch {
  $processName = ""
}
[pscustomobject]@{
  title = $builder.ToString()
  processName = $processName
  processId = [int]$processId
} | ConvertTo-Json -Compress
`;
}

async function queryForegroundWindow() {
  const goResult = await invokeGoBridge("foreground-window");
  if (goResult && (typeof goResult.title === "string" || typeof goResult.processName === "string")) {
    return {
      title: typeof goResult.title === "string" ? goResult.title.trim() : "",
      processName: typeof goResult.processName === "string" ? goResult.processName.trim() : "",
      processId: Number(goResult.processId || 0)
    };
  }

  const powershellPath = path.join(process.env.SYSTEMROOT || "C:\\Windows", "System32", "WindowsPowerShell", "v1.0", "powershell.exe");
  const encodedCommand = Buffer.from(buildForegroundWindowScript(), "utf16le").toString("base64");
  return new Promise((resolve) => {
    execFile(
      powershellPath,
      ["-NoProfile", "-EncodedCommand", encodedCommand],
      {
        windowsHide: true,
        timeout: 4000,
        maxBuffer: 256 * 1024
      },
      (error, stdout) => {
        if (error) {
          resolve(null);
          return;
        }

        try {
          const parsed = JSON.parse(String(stdout || "").trim() || "{}");
          resolve({
            title: typeof parsed.title === "string" ? parsed.title.trim() : "",
            processName: typeof parsed.processName === "string" ? parsed.processName.trim() : "",
            processId: Number(parsed.processId || 0)
          });
        } catch (_parseError) {
          resolve(null);
        }
      }
    );
  });
}

function showMeetingDetectionPrompt(match, windowInfo, now) {
  const wasPrompting = appState.meetingDetection.promptVisible && appState.meetingDetection.status === "prompting";
  appState.meetingDetection = {
    ...appState.meetingDetection,
    status: "prompting",
    candidateApp: match.label,
    candidateTitle: windowInfo.title || "",
    matchedSignature: match.id,
    promptVisible: true,
    promptShownAt: wasPrompting ? appState.meetingDetection.promptShownAt : now,
    lastSeenAt: now,
    absentSince: 0,
    note: `\u68c0\u6d4b\u5230\u4f60\u53ef\u80fd\u6b63\u5728\u53c2\u4e0e ${match.label}\uff0c\u53ef\u4ee5\u5f00\u59cb\u5f53\u524d\u4f1a\u8bae\u7684\u4e24\u5206\u949f\u6982\u8981\u3002`
  };

  if (!wasPrompting) {
    pushAction("\u68c0\u6d4b\u5230\u4f1a\u8bae\u5019\u9009", `${match.label} \u5df2\u4fdd\u6301\u524d\u53f0\uff0c\u7b49\u4f60\u786e\u8ba4\u542f\u52a8\u4f1a\u8bae\u603b\u7ed3\u3002`);
  }
}

function handleMatchedMeetingCandidate(match, windowInfo, now) {
  const previous = appState.meetingDetection;
  const signatureChanged = previous.matchedSignature !== match.id;
  const candidateSince = signatureChanged || !previous.candidateSince ? now : previous.candidateSince;
  const blockReason = getMeetingDetectionBlockReason();

  if (hasMeetingSummaryRunningState()) {
    appState.meetingDetection = {
      ...previous,
      status: "running",
      candidateApp: match.label,
      candidateTitle: windowInfo.title || "",
      matchedSignature: match.id,
      promptVisible: false,
      lastSeenAt: now,
      candidateSince,
      absentSince: 0,
      note: `\u68c0\u6d4b\u5230 ${match.label} \u6b63\u5728\u524d\u53f0\uff0c\u4f1a\u8bae\u603b\u7ed3\u5df2\u5904\u4e8e\u4f1a\u4e2d\u72b6\u6001\u3002`
    };
    return;
  }

  if (previous.suppressedForSession) {
    appState.meetingDetection = {
      ...previous,
      status: "suppressed",
      candidateApp: match.label,
      candidateTitle: windowInfo.title || "",
      matchedSignature: match.id,
      promptVisible: false,
      lastSeenAt: now,
      candidateSince,
      absentSince: 0
    };
    return;
  }

  if (previous.cooldownUntil && previous.cooldownUntil > now) {
    appState.meetingDetection = {
      ...previous,
      status: "suppressed",
      candidateApp: match.label,
      candidateTitle: windowInfo.title || "",
      matchedSignature: match.id,
      promptVisible: false,
      lastSeenAt: now,
      candidateSince,
      absentSince: 0,
      note: previous.note || "\u81ea\u52a8\u63d0\u9192\u51b7\u5374\u4e2d\uff0c\u7a0d\u540e\u4f1a\u518d\u6b21\u5c1d\u8bd5\u3002"
    };
    return;
  }

  if ((now - candidateSince) >= MEETING_DETECTION_CANDIDATE_HOLD_MS) {
    if (blockReason) {
      appState.meetingDetection = {
        ...previous,
        status: "candidate",
        candidateApp: match.label,
        candidateTitle: windowInfo.title || "",
        matchedSignature: match.id,
        promptVisible: false,
        lastSeenAt: now,
        candidateSince,
        absentSince: 0,
        note: blockReason
      };
      return;
    }

    showMeetingDetectionPrompt(match, windowInfo, now);
    return;
  }

  if (signatureChanged || previous.status !== "candidate") {
    pushAction("\u547d\u4e2d\u4f1a\u8bae\u7ebf\u7d22", `\u5df2\u68c0\u6d4b\u5230 ${match.label}\uff0c\u6b63\u5728\u7ee7\u7eed\u786e\u8ba4\u662f\u5426\u5904\u4e8e\u4f1a\u4e2d\u3002`);
  }

  appState.meetingDetection = {
    ...previous,
    status: "candidate",
    candidateApp: match.label,
    candidateTitle: windowInfo.title || "",
    matchedSignature: match.id,
    promptVisible: false,
    lastSeenAt: now,
    candidateSince,
    absentSince: 0,
    note: `\u5df2\u68c0\u6d4b\u5230 ${match.label}\uff0c\u6b63\u5728\u786e\u8ba4\u662f\u5426\u9700\u8981\u81ea\u52a8\u63d0\u9192\u3002`
  };
}

function handleMeetingNoMatch(now) {
  const detection = appState.meetingDetection;
  const hasTrackingState = Boolean(
    detection.matchedSignature
    || detection.promptVisible
    || detection.suppressedForSession
    || detection.status === "candidate"
    || detection.status === "prompting"
    || detection.status === "running"
  );

  if (!hasTrackingState) {
    if (detection.cooldownUntil && detection.cooldownUntil <= now) {
      appState.meetingDetection = {
        ...detection,
        status: "idle",
        cooldownUntil: 0,
        note: createMeetingDetectionState().note
      };
    }
    return;
  }

  const absentSince = detection.absentSince || now;
  const missingDuration = now - absentSince;

  if (hasMeetingSummaryRunningState()) {
    appState.meetingDetection = {
      ...detection,
      status: "running",
      promptVisible: false,
      absentSince,
      note: detection.candidateApp
        ? `${detection.candidateApp} \u5df2\u79bb\u5f00\u524d\u53f0\uff0c\u6b63\u5728\u7ed3\u5408\u97f3\u9891\u8f93\u5165\u5224\u65ad\u4f1a\u8bae\u662f\u5426\u7ed3\u675f\u3002`
        : "\u6b63\u5728\u7ed3\u5408\u97f3\u9891\u8f93\u5165\u5224\u65ad\u4f1a\u8bae\u662f\u5426\u7ed3\u675f\u3002"
    };

    if (missingDuration >= MEETING_DETECTION_EXIT_HOLD_MS
      && getMeetingDetectionSourceIdleMs(now) >= MEETING_DETECTION_SOURCE_IDLE_STOP_MS) {
      stopMeetingSummarySession({
        clearError: true,
        reason: "auto-end",
        note: "\u68c0\u6d4b\u5230\u4f1a\u8bae\u7a97\u53e3\u5df2\u79bb\u5f00\u4e14\u97f3\u9891\u8f93\u5165\u5df2\u9759\u9ed8\uff0c\u5df2\u81ea\u52a8\u7ed3\u675f\u4f1a\u8bae\u603b\u7ed3\u3002"
      });
      pushAction("\u81ea\u52a8\u505c\u6b62\u4f1a\u8bae\u603b\u7ed3", "\u68c0\u6d4b\u5230\u672c\u573a\u4f1a\u8bae\u5df2\u7ed3\u675f\uff0c\u81ea\u52a8\u6062\u590d\u4e3a\u5f85\u547d\u72b6\u6001\u3002");
    }
    return;
  }

  appState.meetingDetection = {
    ...detection,
    status: detection.suppressedForSession ? "suppressed" : "idle",
    promptVisible: false,
    absentSince
  };

  if (missingDuration >= MEETING_DETECTION_EXIT_HOLD_MS) {
    if (detection.suppressedForSession || detection.matchedSignature) {
      pushAction("\u5224\u5b9a\u4f1a\u8bae\u7ed3\u675f", "\u5df2\u79bb\u5f00\u4f1a\u8bae\u524d\u53f0\u7a97\u53e3\uff0c\u81ea\u52a8\u68c0\u6d4b\u6062\u590d\u5f85\u547d\u3002");
    }
    resetMeetingDetectionSession({
      keepCooldown: detection.cooldownUntil > now,
      note: detection.cooldownUntil > now ? detection.note : createMeetingDetectionState().note
    });
  }
}

async function pollMeetingDetection() {
  if (foregroundWindowQueryPending || appIsQuitting) {
    return;
  }

  foregroundWindowQueryPending = true;
  try {
    const windowInfo = await queryForegroundWindow();
    const now = Date.now();
    if (windowInfo) {
      appState.context = {
        ...appState.context,
        activeApp: getForegroundAppLabel(windowInfo)
      };
    }

    if (!appState.meetingDetection.enabled || !appState.abilityToggles.activeApp) {
      if (appState.meetingDetection.promptVisible) {
        clearMeetingDetectionPrompt({
          keepStatus: "idle",
          note: getMeetingDetectionBlockReason()
        });
      } else {
        appState.meetingDetection = {
          ...appState.meetingDetection,
          status: "idle",
          note: getMeetingDetectionBlockReason()
        };
      }
      broadcastSnapshot();
      return;
    }

    const match = matchMeetingApp(windowInfo || {});
    if (match) {
      handleMatchedMeetingCandidate(match, windowInfo || {}, now);
    } else {
      handleMeetingNoMatch(now);
    }
    broadcastSnapshot();
  } finally {
    foregroundWindowQueryPending = false;
  }
}

function startMeetingDetectionLoop() {
  if (meetingDetectionTimer) {
    return;
  }

  void pollMeetingDetection();
  meetingDetectionTimer = setInterval(() => {
    void pollMeetingDetection();
  }, MEETING_DETECTION_POLL_MS);
}

function stopMeetingDetectionLoop() {
  if (meetingDetectionTimer) {
    clearInterval(meetingDetectionTimer);
    meetingDetectionTimer = null;
  }
}

function registerDisplayMediaHandler() {
  session.defaultSession.setDisplayMediaRequestHandler(async (request, callback) => {
    try {
      const sources = await desktopCapturer.getSources({
        types: ["screen"]
      });

      if (!sources.length) {
        callback({});
        return;
      }

      callback({
        video: sources[0],
        audio: request.audioRequested ? "loopback" : undefined
      });
    } catch (error) {
      console.error("Failed to grant display media request", error);
      callback({});
    }
  });
}

function setMeetingSummaryError(message, options = {}) {
  const resolvedMessage = normalizeRuntimeError(message, "\u4f1a\u8bae\u603b\u7ed3\u5931\u8d25");
  closeAllMeetingRealtimeConnections();
  appState.meetingSummary = {
    ...appState.meetingSummary,
    enabled: false,
    status: "error",
    error: resolvedMessage,
    note: options.note || "\u8bf7\u68c0\u67e5\u7cfb\u7edf\u97f3\u9891\u3001API \u914d\u7f6e\u548c\u6240\u9009\u6a21\u578b\u662f\u5426\u652f\u6301\u8f6c\u5199\u3002"
  };
  if (shouldCooldownAutoPromptOnFailure()) {
    setMeetingDetectionCooldown("\u81ea\u52a8\u542f\u52a8\u4f1a\u8bae\u603b\u7ed3\u672a\u6210\u529f\uff0c\u5df2\u6682\u505c\u81ea\u52a8\u63d0\u9192 5 \u5206\u949f\u3002");
  } else if (appState.meetingDetection.status === "running") {
    appState.meetingDetection = {
      ...appState.meetingDetection,
      status: "idle",
      promptVisible: false,
      note: resolvedMessage
    };
  }
  pushAction("\u4f1a\u8bae\u603b\u7ed3\u5931\u8d25", resolvedMessage, "warn");
  broadcastSnapshot();
  return {
    ok: false,
    error: resolvedMessage,
    handled: true
  };
}

function startMeetingSummarySession(options = {}) {
  const modelConfigError = getModelConfigError();
  if (modelConfigError) {
    return setMeetingSummaryError(modelConfigError, {
      note: "\u8bf7\u5148\u6253\u5f00\u63a7\u5236\u9762\u677f\uff0c\u4fdd\u5b58 OpenAI \u517c\u5bb9\u63a5\u53e3\u914d\u7f6e\u3002"
    });
  }

  const trigger = options.trigger === "auto-prompt" ? "auto-prompt" : "manual";
  const transcriptionMode = shouldUseRealtimeMeetingTranscription() ? "realtime" : "chunk";
  const captureRequestId = createRealtimeEventId("meeting-capture");
  closeAllMeetingRealtimeConnections();
  appState.meetingSummary = createMeetingSummaryState({
    enabled: true,
    status: "starting",
    transcriptionMode,
    startTrigger: trigger,
    startedAt: Date.now(),
    captureRequestId,
    intervalMs: MEETING_SUMMARY_INTERVAL_MS,
    transcriptIntervalMs: MEETING_TRANSCRIPT_INTERVAL_MS,
    realtimeSampleRate: MEETING_REALTIME_SAMPLE_RATE,
    note: transcriptionMode === "realtime"
      ? "\u6b63\u5728\u8fde\u63a5\u7cfb\u7edf\u97f3\u9891\u548c\u9ea6\u514b\u98ce\uff0c\u5b9e\u65f6\u5b57\u5e55\u7a97\u4f1a\u8fde\u7eed\u66f4\u65b0\uff0c\u6982\u8981\u7a97\u6bcf 2 \u5206\u949f\u5237\u65b0\u4e00\u6b21\u3002"
      : "\u6b63\u5728\u8fde\u63a5\u7cfb\u7edf\u97f3\u9891\u548c\u9ea6\u514b\u98ce\uff0c\u5b57\u5e55\u7a97\u4f1a\u663e\u793a\u5b9e\u65f6\u8f6c\u5199\u3002"
  });
  appState.meetingDetection = {
    ...appState.meetingDetection,
    status: "running",
    promptVisible: false,
    promptShownAt: 0,
    note: trigger === "auto-prompt"
      ? `\u5df2\u51c6\u5907\u5bf9 ${appState.meetingDetection.candidateApp || "\u5f53\u524d\u4f1a\u8bae"} \u5f00\u59cb\u4f1a\u8bae\u603b\u7ed3\u3002`
      : "\u4f1a\u8bae\u603b\u7ed3\u5df2\u542f\u52a8\u3002"
  };
  pushAction(
    trigger === "auto-prompt" ? "\u81ea\u52a8\u542f\u52a8\u4f1a\u8bae\u603b\u7ed3" : "\u5f00\u59cb\u4f1a\u8bae\u603b\u7ed3",
    "\u5df2\u5f00\u542f\u7cfb\u7edf\u97f3\u9891\u548c\u9ea6\u514b\u98ce\u8bc6\u522b"
  );
  showMeetingSummaryOverlay({
    forceLayout: true
  });
  showMeetingCaptionsOverlay({
    forceLayout: true
  });
  broadcastSnapshot();
  return {
    ok: true,
    intervalMs: MEETING_SUMMARY_INTERVAL_MS,
    transcriptIntervalMs: MEETING_TRANSCRIPT_INTERVAL_MS,
    transcriptionMode,
    realtimeSampleRate: MEETING_REALTIME_SAMPLE_RATE,
    captureRequestId
  };
}

function acceptMeetingDetectionPrompt() {
  if (!appState.meetingDetection.promptVisible) {
    return {
      ok: false,
      error: "\u5f53\u524d\u6ca1\u6709\u5f85\u786e\u8ba4\u7684\u4f1a\u8bae\u63d0\u9192\u3002"
    };
  }

  ensureChatWindow({ focus: true });
  return startMeetingSummarySession({
    trigger: "auto-prompt"
  });
}

function dismissMeetingDetectionPrompt() {
  suppressMeetingDetectionForCurrentSession(
    appState.meetingDetection.candidateApp
      ? `\u5df2\u5ffd\u7565 ${appState.meetingDetection.candidateApp} \u8fd9\u4e00\u573a\u4f1a\u8bae\uff0c\u76f4\u5230\u4f1a\u8bae\u7ed3\u675f\u524d\u4e0d\u518d\u81ea\u52a8\u63d0\u9192\u3002`
      : "\u672c\u573a\u4f1a\u8bae\u5df2\u5ffd\u7565\uff0c\u76f4\u5230\u4f1a\u8bae\u7ed3\u675f\u524d\u4e0d\u518d\u81ea\u52a8\u63d0\u9192\u3002"
  );
  pushAction("\u5ffd\u7565\u672c\u573a\u4f1a\u8bae", "\u5df2\u505c\u6b62\u8fd9\u4e00\u573a\u4f1a\u8bae\u7684\u81ea\u52a8\u63d0\u9192\u3002");
  broadcastSnapshot();
  return {
    ok: true
  };
}

function markMeetingCaptureReady(source) {
  if (!appState.meetingSummary.enabled) {
    return;
  }

  updateMeetingSourceState(source, {
    active: true,
    status: appState.meetingSummary.transcriptionMode === "realtime" ? "starting" : "listening",
    lastSignalAt: Date.now(),
    note: appState.meetingSummary.transcriptionMode === "realtime"
      ? "\u97f3\u9891\u8f93\u5165\u5df2\u5c31\u7eea\uff0c\u6b63\u5728\u5efa\u7acb\u5b9e\u65f6\u8f6c\u5199\u94fe\u8def..."
      : "\u5b9e\u65f6\u8f6c\u5199\u5df2\u542f\u52a8",
    error: ""
  });
  appState.meetingSummary = {
    ...appState.meetingSummary,
    status: appState.meetingSummary.transcriptionMode === "realtime" ? "starting" : "listening",
    error: "",
    note: getMeetingListeningNote()
  };
  broadcastSnapshot();

  if (appState.meetingSummary.transcriptionMode === "realtime") {
    void ensureMeetingRealtimeConnection(source);
  }
}

function markMeetingSourceError(source, message) {
  const resolvedMessage = normalizeRuntimeError(message, `${getMeetingSourceLabel(source)}\u63a5\u5165\u5931\u8d25`);
  closeMeetingRealtimeSourceConnection(source);
  updateMeetingSourceState(source, {
    active: false,
    status: "error",
    lastSignalAt: 0,
    note: resolvedMessage,
    error: resolvedMessage
  });

  appState.meetingSummary = {
    ...appState.meetingSummary,
    note: getMeetingSourceCount()
      ? `${getMeetingSourceLabel(source)}\u5931\u8d25\uff0c\u5df2\u7ee7\u7eed\u4f7f\u7528\u5176\u4ed6\u97f3\u6e90\u3002`
      : "\u6ca1\u6709\u53ef\u7528\u7684\u97f3\u9891\u8f93\u5165\u6e90\uff0c\u8bf7\u68c0\u67e5\u7cfb\u7edf\u97f3\u9891\u3001\u9ea6\u514b\u98ce\u548c\u6743\u9650\u3002"
  };
  pushAction(`${getMeetingSourceLabel(source)}\u8f93\u5165\u5f02\u5e38`, resolvedMessage, "warn");
  broadcastSnapshot();
  return {
    ok: false,
    error: resolvedMessage,
    handled: true
  };
}

function getMeetingRealtimeConnection(source) {
  return meetingRealtimeConnections[source === "microphone" ? "microphone" : "system"];
}

function clearMeetingRealtimeReconnect(source) {
  const connection = getMeetingRealtimeConnection(source);
  if (connection.reconnectTimer) {
    clearTimeout(connection.reconnectTimer);
    connection.reconnectTimer = null;
  }
}

function sendMeetingRealtimeEvent(connection, payload) {
  if (!connection.socket || connection.socket.readyState !== WebSocket.OPEN) {
    return false;
  }

  try {
    connection.socket.send(JSON.stringify(payload));
    return true;
  } catch (error) {
    connection.lastError = normalizeRuntimeError(error, `${getMeetingSourceLabel(connection.source)}\u5b9e\u65f6\u8f6c\u5199\u53d1\u9001\u5931\u8d25`);
    return false;
  }
}

function queueMeetingRealtimeAudioChunk(source, audioBase64) {
  if (!audioBase64) {
    return;
  }

  const connection = getMeetingRealtimeConnection(source);
  connection.pendingAudioChunks.push(audioBase64);
  if (connection.pendingAudioChunks.length > 160) {
    connection.pendingAudioChunks.splice(0, connection.pendingAudioChunks.length - 160);
  }
}

function flushMeetingRealtimeAudioQueue(source) {
  const connection = getMeetingRealtimeConnection(source);
  if (!connection.ready) {
    return;
  }

  while (connection.pendingAudioChunks.length) {
    const nextAudioChunk = connection.pendingAudioChunks.shift();
    const sent = sendMeetingRealtimeEvent(connection, {
      event_id: createRealtimeEventId(`audio-${source}`),
      type: "input_audio_buffer.append",
      audio: nextAudioChunk
    });
    if (!sent) {
      if (nextAudioChunk) {
        connection.pendingAudioChunks.unshift(nextAudioChunk);
      }
      break;
    }
  }
}

function syncMeetingSummaryToListeningState() {
  if (appState.meetingSummary.status === "processing") {
    return;
  }

  appState.meetingSummary = {
    ...appState.meetingSummary,
    status: getMeetingSourceCount() ? "listening" : "starting",
    error: "",
    note: getMeetingListeningNote()
  };
}

function handleMeetingRealtimePreview(source, payload) {
  const confirmedText = extractRealtimeTranscriptText(payload);
  const stashText = extractRealtimeTranscriptStash(payload);
  const previewText = `${confirmedText}${stashText}`.trim();
  if (!previewText) {
    return;
  }

  updateMeetingSourceState(source, {
    active: true,
    status: "listening",
    note: "\u5b9e\u65f6\u5b57\u5e55\u66f4\u65b0\u4e2d",
    latestText: previewText,
    error: "",
    lastUpdatedAt: formatClockTime(),
    lastSignalAt: Date.now()
  });
  syncMeetingSummaryToListeningState();
  scheduleMeetingSnapshotBroadcast();
}

function handleMeetingRealtimeCompleted(source, payload) {
  const transcript = extractRealtimeTranscriptText(payload);
  if (!transcript) {
    return;
  }

  const now = Date.now();
  const timestamp = formatClockTime();
  const sourceState = appState.meetingSummary.sources[source] || createMeetingSourceState(getMeetingSourceLabel(source));
  updateMeetingSourceState(source, {
    active: true,
    status: "listening",
    note: "\u5b9e\u65f6\u8f6c\u5199\u4e2d",
    latestText: transcript,
    error: "",
    lastUpdatedAt: timestamp,
    lastSignalAt: now,
    chunkCount: (sourceState.chunkCount || 0) + 1,
    recentItems: trimCaptionItems(
      [
        {
          id: `meeting-source-${source}-${now}`,
          text: transcript,
          time: timestamp,
          capturedAt: now
        },
        ...(sourceState.recentItems || [])
      ],
      now,
      {
        historyMs: MEETING_CAPTION_HISTORY_MS,
        maxItems: MEETING_CAPTION_MAX_ITEMS
      }
    )
  });

  appState.meetingSummary = {
    ...appState.meetingSummary,
    pendingTranscriptBlocks: [
      ...(appState.meetingSummary.pendingTranscriptBlocks || []),
      {
        id: `meeting-pending-${source}-${now}`,
        source,
        time: timestamp,
        capturedAt: now,
        text: transcript
      }
    ].slice(-64)
  };
  syncMeetingSummaryToListeningState();
  scheduleMeetingSnapshotBroadcast();
}

function scheduleMeetingRealtimeReconnect(source, reason) {
  const connection = getMeetingRealtimeConnection(source);
  if (connection.closedByClient || !appState.meetingSummary.enabled || appState.meetingSummary.transcriptionMode !== "realtime") {
    return;
  }

  if (!(appState.meetingSummary.sources?.[source]?.active)) {
    return;
  }

  if (connection.reconnectAttempts >= MEETING_REALTIME_MAX_RECONNECTS) {
    markMeetingSourceError(source, reason || `${getMeetingSourceLabel(source)}\u5b9e\u65f6\u8f6c\u5199\u8fde\u63a5\u5df2\u591a\u6b21\u4e2d\u65ad`);
    return;
  }

  clearMeetingRealtimeReconnect(source);
  connection.reconnectAttempts += 1;
  updateMeetingSourceState(source, {
    active: true,
    status: "starting",
    note: `\u5b9e\u65f6\u8f6c\u5199\u8fde\u63a5\u4e2d\u65ad\uff0c\u6b63\u5728\u7b2c ${connection.reconnectAttempts} \u6b21\u91cd\u8fde...`,
    error: ""
  });
  scheduleMeetingSnapshotBroadcast();

  const reconnectDelay = MEETING_REALTIME_RECONNECT_BASE_MS * connection.reconnectAttempts;
  connection.reconnectTimer = setTimeout(() => {
    connection.reconnectTimer = null;
    void ensureMeetingRealtimeConnection(source);
  }, reconnectDelay);
}

function handleMeetingRealtimeSocketMessage(source, rawMessage) {
  const connection = getMeetingRealtimeConnection(source);
  let payload;

  try {
    payload = JSON.parse(rawMessage.toString());
  } catch (_error) {
    return;
  }

  if (payload.type === "session.created") {
    sendMeetingRealtimeEvent(connection, {
      event_id: createRealtimeEventId(`session-${source}`),
      type: "session.update",
      session: {
        modalities: ["text"],
        input_audio_format: "pcm16",
        sample_rate: MEETING_REALTIME_SAMPLE_RATE,
        input_audio_transcription: {
          language: "zh"
        },
        turn_detection: {
          type: "server_vad",
          silence_duration_ms: 400
        }
      }
    });
    return;
  }

  if (payload.type === "session.updated") {
    clearMeetingRealtimeReconnect(source);
    connection.ready = true;
    connection.initialized = true;
    connection.reconnectAttempts = 0;
    connection.lastError = "";
    updateMeetingSourceState(source, {
      active: true,
      status: "listening",
      lastSignalAt: Date.now(),
      note: "\u771f\u5b9e\u65f6\u8f6c\u5199\u5df2\u8fde\u63a5",
      error: ""
    });
    syncMeetingSummaryToListeningState();
    flushMeetingRealtimeAudioQueue(source);
    flushMeetingSnapshotBroadcast();
    return;
  }

  if (payload.type === "conversation.item.input_audio_transcription.text") {
    handleMeetingRealtimePreview(source, payload);
    return;
  }

  if (payload.type === "conversation.item.input_audio_transcription.completed") {
    handleMeetingRealtimeCompleted(source, payload);
    return;
  }

  if (payload.type === "conversation.item.input_audio_transcription.failed") {
    const failureMessage = getApiErrorMessage(payload) || `${getMeetingSourceLabel(source)}\u8fd9\u4e00\u6bb5\u5b9e\u65f6\u8f6c\u5199\u5931\u8d25`;
    updateMeetingSourceState(source, {
      active: true,
      status: "listening",
      note: `${failureMessage}\uff0c\u5df2\u7ee7\u7eed\u76d1\u542c`,
      error: ""
    });
    scheduleMeetingSnapshotBroadcast();
    return;
  }

  if (payload.type === "input_audio_buffer.speech_started") {
    updateMeetingSourceState(source, {
      active: true,
      status: "listening",
      lastSignalAt: Date.now(),
      note: "\u5df2\u68c0\u6d4b\u5230\u8bed\u97f3\uff0c\u6b63\u5728\u5b9e\u65f6\u8bc6\u522b...",
      error: ""
    });
    scheduleMeetingSnapshotBroadcast();
    return;
  }

  if (payload.type === "input_audio_buffer.speech_stopped") {
    updateMeetingSourceState(source, {
      active: true,
      status: "listening",
      note: "\u68c0\u6d4b\u5230\u505c\u987f\uff0c\u6b63\u5728\u6574\u7406\u8fd9\u6bb5\u8f6c\u5199...",
      error: ""
    });
    scheduleMeetingSnapshotBroadcast();
    return;
  }

  if (payload.type === "error") {
    connection.lastError = getApiErrorMessage(payload?.error || payload) || `${getMeetingSourceLabel(source)}\u5b9e\u65f6\u8f6c\u5199\u8fde\u63a5\u5931\u8d25`;
    if (connection.socket && connection.socket.readyState === WebSocket.OPEN) {
      connection.socket.close();
    }
  }
}

async function ensureMeetingRealtimeConnection(source) {
  if (!appState.meetingSummary.enabled || appState.meetingSummary.transcriptionMode !== "realtime") {
    return;
  }

  const connection = getMeetingRealtimeConnection(source);
  if (connection.socket && (connection.socket.readyState === WebSocket.CONNECTING || connection.socket.readyState === WebSocket.OPEN)) {
    return;
  }

  const config = sanitizeModelConfig(modelConfigState.transcription);
  if (!shouldUseRealtimeMeetingTranscription(config)) {
    return;
  }

  clearMeetingRealtimeReconnect(source);
  connection.closedByClient = false;
  connection.ready = false;
  connection.initialized = false;
  connection.lastError = "";

  const socket = new WebSocket(buildDashScopeRealtimeUrl(config.baseUrl, config.model), {
    headers: {
      Authorization: `Bearer ${config.apiKey}`
    }
  });

  connection.socket = socket;
  updateMeetingSourceState(source, {
    active: true,
    status: "starting",
    note: "\u6b63\u5728\u5efa\u7acb\u5b9e\u65f6\u8f6c\u5199\u8fde\u63a5...",
    error: ""
  });
  scheduleMeetingSnapshotBroadcast();

  socket.on("message", (rawMessage) => {
    handleMeetingRealtimeSocketMessage(source, rawMessage);
  });

  socket.on("error", (error) => {
    connection.lastError = normalizeRuntimeError(error, `${getMeetingSourceLabel(source)}\u5b9e\u65f6\u8f6c\u5199\u8fde\u63a5\u5931\u8d25`);
  });

  socket.on("close", (code, reasonBuffer) => {
    const reasonText = Buffer.isBuffer(reasonBuffer) ? reasonBuffer.toString("utf8").trim() : String(reasonBuffer || "").trim();
    const closeMessage = connection.lastError || reasonText || `${getMeetingSourceLabel(source)}\u5b9e\u65f6\u8f6c\u5199\u8fde\u63a5\u5df2\u5173\u95ed (${code})`;
    const closedByClient = connection.closedByClient;
    connection.socket = null;
    connection.ready = false;
    connection.initialized = false;
    connection.closedByClient = false;

    if (closedByClient) {
      connection.pendingAudioChunks = [];
      connection.reconnectAttempts = 0;
      return;
    }

    scheduleMeetingRealtimeReconnect(source, closeMessage);
  });
}

function closeMeetingRealtimeSourceConnection(source, options = {}) {
  const connection = getMeetingRealtimeConnection(source);
  clearMeetingRealtimeReconnect(source);

  if (connection.socket) {
    connection.closedByClient = true;
    connection.socket.removeAllListeners();
    try {
      connection.socket.close();
    } catch (_error) {
      // noop
    }
  }

  connection.socket = null;
  connection.ready = false;
  connection.initialized = false;
  connection.lastError = "";
  connection.reconnectAttempts = 0;
  connection.closedByClient = false;
  if (options.keepPendingAudio !== true) {
    connection.pendingAudioChunks = [];
  }
}

function closeAllMeetingRealtimeConnections() {
  closeMeetingRealtimeSourceConnection("system");
  closeMeetingRealtimeSourceConnection("microphone");
}

async function finalizeMeetingRealtimeTranscription() {
  if (appState.meetingSummary.transcriptionMode !== "realtime") {
    return {
      ok: true,
      skipped: true
    };
  }

  ["system", "microphone"].forEach((source) => {
    const connection = getMeetingRealtimeConnection(source);
    if (!connection.ready) {
      return;
    }

    sendMeetingRealtimeEvent(connection, {
      event_id: createRealtimeEventId(`commit-${source}`),
      type: "input_audio_buffer.commit"
    });
  });

  await new Promise((resolve) => {
    setTimeout(resolve, MEETING_REALTIME_FINALIZE_WAIT_MS);
  });

  return {
    ok: true
  };
}

function appendMeetingRealtimeAudio(payload) {
  if (!appState.meetingSummary.enabled || appState.meetingSummary.transcriptionMode !== "realtime") {
    return;
  }

  const source = payload?.source === "microphone" ? "microphone" : "system";
  const audioBase64 = typeof payload?.audioBase64 === "string" ? payload.audioBase64 : "";
  if (!audioBase64) {
    return;
  }

  const connection = getMeetingRealtimeConnection(source);
  if (!connection.socket || connection.socket.readyState !== WebSocket.OPEN || !connection.ready) {
    queueMeetingRealtimeAudioChunk(source, audioBase64);
    void ensureMeetingRealtimeConnection(source);
    return;
  }

  const sent = sendMeetingRealtimeEvent(connection, {
    event_id: createRealtimeEventId(`audio-${source}`),
    type: "input_audio_buffer.append",
    audio: audioBase64
  });

  if (!sent) {
    queueMeetingRealtimeAudioChunk(source, audioBase64);
    void ensureMeetingRealtimeConnection(source);
  }
}

function stopMeetingSummarySession(options = {}) {
  closeAllMeetingRealtimeConnections();
  const wasActive = appState.meetingSummary.enabled || ["starting", "listening", "processing"].includes(appState.meetingSummary.status);
  const hasSummary = hasStructuredSummaryContent(appState.meetingSummary.latestSummary);
  const nextSources = Object.fromEntries(
    Object.entries(appState.meetingSummary.sources || {}).map(([source, sourceState]) => [
      source,
      {
        ...sourceState,
        active: false,
        status: hasSummary ? "stopped" : "idle",
        note: hasSummary ? "\u5df2\u505c\u6b62\u91c7\u96c6" : "\u7b49\u5f85\u8fde\u63a5"
      }
    ])
  );
  appState.meetingSummary = {
    ...appState.meetingSummary,
    enabled: false,
    status: hasSummary ? "stopped" : "idle",
    error: options.clearError ? "" : appState.meetingSummary.error,
    pendingTranscriptBlocks: [],
    sources: nextSources,
    note: options.note || (hasSummary
      ? "\u5df2\u505c\u6b62\u4f1a\u8bae\u603b\u7ed3\uff0c\u4fdd\u7559\u6700\u540e\u4e00\u7248\u6982\u8981\u3002"
      : "\u5df2\u505c\u6b62\u4f1a\u8bae\u603b\u7ed3\u3002")
  };

  if (options.reason === "manual") {
    suppressMeetingDetectionForCurrentSession(
      "\u4f60\u5df2\u624b\u52a8\u53d6\u6d88\u672c\u573a\u4f1a\u8bae\u603b\u7ed3\uff0c\u672c\u573a\u4f1a\u8bae\u5185\u4e0d\u518d\u81ea\u52a8\u542f\u52a8\u3002"
    );
  } else if (options.reason === "auto-end") {
    resetMeetingDetectionSession({
      note: "\u68c0\u6d4b\u5230\u4f1a\u8bae\u5df2\u7ed3\u675f\uff0c\u81ea\u52a8\u8bc6\u522b\u5df2\u6062\u590d\u5f85\u547d\u3002"
    });
  } else if (!hasMeetingSummaryRunningState() && appState.meetingDetection.status === "running") {
    appState.meetingDetection = {
      ...appState.meetingDetection,
      status: "idle",
      promptVisible: false,
      note: createMeetingDetectionState().note
    };
  }

  if (wasActive && options.pushLog !== false) {
    pushAction("\u505c\u6b62\u4f1a\u8bae\u603b\u7ed3", appState.meetingSummary.note);
  }
  broadcastSnapshot();
  return {
    ok: true
  };
}

async function transcribeMeetingChunk(payload) {
  const config = sanitizeModelConfig(modelConfigState.transcription);
  const mimeType = payload.mimeType || "audio/webm";

  if (isDashScopeCompatibleBaseUrl(config.baseUrl)) {
    if (/realtime/i.test(config.model)) {
      throw new Error("\u5f53\u524d\u914d\u7f6e\u7684 qwen3-asr-flash-realtime \u9700\u8981 WebSocket \u5b9e\u65f6\u534f\u8bae\uff0c\u4e0d\u652f\u6301\u672c\u9879\u76ee\u73b0\u5728\u8fd9\u79cd HTTP \u5206\u6bb5\u4e0a\u4f20\u8f6c\u5199\u3002\u8bf7\u6539\u7528 qwen3-asr-flash\u3002");
    }

    const response = await fetch(buildApiUrl(config.baseUrl, "/chat/completions"), {
      method: "POST",
      headers: {
        Authorization: `Bearer ${config.apiKey}`,
        "Content-Type": "application/json"
      },
      body: JSON.stringify({
        model: config.model,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "input_audio",
                input_audio: {
                  data: buildInputAudioDataUri(mimeType, payload.audioBase64)
                }
              }
            ]
          }
        ],
        stream: false,
        asr_options: {
          enable_itn: false
        }
      })
    });

    const data = await parseApiResponse(response);
    if (!response.ok) {
      throw buildHttpError(response, data, "\u97f3\u9891\u8f6c\u5199\u5931\u8d25");
    }

    return extractAssistantText(data);
  }

  const extension = mimeType.includes("mp4") ? "m4a" : "webm";
  const audioBlob = new Blob([Buffer.from(payload.audioBase64, "base64")], { type: mimeType });
  const formData = new FormData();
  formData.set("file", audioBlob, `meeting-${payload.sequence || Date.now()}.${extension}`);
  formData.set("model", config.model);

  const response = await fetch(buildApiUrl(config.baseUrl, "/audio/transcriptions"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${config.apiKey}`
    },
    body: formData
  });

  const data = await parseApiResponse(response);
  if (!response.ok) {
    throw buildHttpError(response, data, "\u97f3\u9891\u8f6c\u5199\u5931\u8d25");
  }

  if (typeof data === "string") {
    return data.trim();
  }
  return typeof data?.text === "string" ? data.text.trim() : "";
}

function createStructuredMeetingSummary(overrides = {}) {
  return normalizeStructuredSummary({
    updatedAt: formatClockTime(),
    windowSeconds: MEETING_SUMMARY_WINDOW_SECONDS,
    ...overrides
  });
}

function createUnavailableMeetingSummary() {
  return createStructuredMeetingSummary({
    othersText: MEETING_SOURCE_EMPTY_TEXT,
    selfText: MEETING_SOURCE_EMPTY_TEXT,
    isEmpty: true,
    note: MEETING_ALL_EMPTY_TEXT
  });
}

async function summarizeMeetingTranscript(blockGroups) {
  const hasOthers = Boolean(blockGroups?.hasOthers);
  const hasSelf = Boolean(blockGroups?.hasSelf);
  if (!hasOthers && !hasSelf) {
    return createUnavailableMeetingSummary();
  }

  const config = sanitizeModelConfig(modelConfigState.summary);
  const othersTranscript = buildSourceTranscriptText(blockGroups?.recentSystemBlocks, "\u7cfb\u7edf\u97f3\u9891");
  const selfTranscript = buildSourceTranscriptText(blockGroups?.recentMicrophoneBlocks, "\u9ea6\u514b\u98ce");
  const response = await fetch(buildApiUrl(config.baseUrl, "/chat/completions"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${config.apiKey}`,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      model: config.model,
      temperature: 0.2,
      messages: [
        {
          role: "system",
          content: [
            "\u4f60\u662f\u4e00\u4e2a\u4e25\u8c28\u7684\u4e2d\u6587\u4f1a\u8bae\u6982\u8981\u52a9\u624b\u3002",
            "\u53ea\u80fd\u57fa\u4e8e\u201c\u6700\u8fd1\u4e24\u5206\u949f\u201d\u7684\u8f6c\u5199\u5185\u5bb9\u603b\u7ed3\uff0c\u4e0d\u8981\u5f15\u7528\u66f4\u65e9\u5185\u5bb9\uff0c\u4e0d\u8981\u7f16\u9020\u4fe1\u606f\u3002",
            "\u4f60\u5fc5\u987b\u8f93\u51fa\u4e25\u683c JSON \u5bf9\u8c61\uff0c\u53ea\u5305\u542b two fields: othersText \u548c selfText\u3002",
            `\u5176\u4e2d othersText \u53ea\u603b\u7ed3\u201c\u7cfb\u7edf\u97f3\u9891/\u4ed6\u4eba\u8bb2\u8bdd\u201d\uff0cselfText \u53ea\u603b\u7ed3\u201c\u9ea6\u514b\u98ce/\u6211\u8bb2\u8bdd\u201d\u3002`,
            `\u67d0\u4e00\u8def\u6ca1\u6709\u6709\u6548\u5185\u5bb9\u65f6\uff0c\u5bf9\u5e94\u5b57\u6bb5\u586b\u5199\u201c${MEETING_SOURCE_EMPTY_TEXT}\u201d\u3002`,
            "\u6bcf\u4e2a\u5b57\u6bb5\u90fd\u662f\u4e00\u5c0f\u6bb5\u7b80\u4f53\u4e2d\u6587\uff0c\u4e0d\u8981\u5217\u8868\u3001\u4e0d\u8981 Markdown\u3001\u4e0d\u8981\u4ee3\u7801\u5757\u3002"
          ].join("")
        },
        {
          role: "user",
          content: [
            "\u8fd9\u662f\u6700\u65b0 2 \u5206\u949f\u5185\u7684\u4e24\u8def\u8f6c\u5199\uff1a",
            "",
            "[\u4ed6\u4eba\u8bb2\u8bdd / \u7cfb\u7edf\u97f3\u9891]",
            othersTranscript || `(${MEETING_SOURCE_EMPTY_TEXT})`,
            "",
            "[\u6211\u8bb2\u8bdd / \u9ea6\u514b\u98ce]",
            selfTranscript || `(${MEETING_SOURCE_EMPTY_TEXT})`,
            "",
            "\u8bf7\u8fd4\u56de JSON\uff1a{\"othersText\":\"...\",\"selfText\":\"...\"}"
          ].join("\n")
        }
      ]
    })
  });

  const data = await parseApiResponse(response);
  if (!response.ok) {
    throw buildHttpError(response, data, "\u4f1a\u8bae\u6458\u8981\u751f\u6210\u5931\u8d25");
  }

  const rawSummary = extractAssistantText(data);
  if (!rawSummary) {
    return createStructuredMeetingSummary({
      othersText: hasOthers ? createTranscriptSnippet(othersTranscript) : MEETING_SOURCE_EMPTY_TEXT,
      selfText: hasSelf ? createTranscriptSnippet(selfTranscript) : MEETING_SOURCE_EMPTY_TEXT
    });
  }

  const parsed = parseStructuredSummaryResponse(rawSummary);
  return createStructuredMeetingSummary({
    othersText: parsed.othersText || (hasOthers ? createTranscriptSnippet(othersTranscript) : MEETING_SOURCE_EMPTY_TEXT),
    selfText: parsed.selfText || (hasSelf ? createTranscriptSnippet(selfTranscript) : MEETING_SOURCE_EMPTY_TEXT)
  });
}

async function processMeetingSummaryChunk(payload) {
  if (!appState.meetingSummary.enabled) {
    return {
      ok: false,
      error: "\u4f1a\u8bae\u603b\u7ed3\u5f53\u524d\u672a\u8fd0\u884c"
    };
  }

  const source = payload?.source === "microphone" ? "microphone" : "system";
  try {
    updateMeetingSourceState(source, {
      status: "processing",
      note: "\u6b63\u5728\u8f6c\u5199\u8fd9\u8def\u97f3\u9891...",
      error: ""
    });
    broadcastSnapshot();

    const transcript = await transcribeMeetingChunk(payload);
    if (!transcript) {
      updateMeetingSourceState(source, {
        active: true,
        status: "listening",
        note: "\u672c\u6b21\u672a\u8bc6\u522b\u5230\u6e05\u6670\u8bed\u97f3",
        lastUpdatedAt: formatClockTime()
      });
      syncMeetingSummaryToListeningState();
      broadcastSnapshot();
      return {
        ok: true,
        transcript: ""
      };
    }

    const now = Date.now();
    const timestamp = formatClockTime();
    const sourceState = appState.meetingSummary.sources[source] || createMeetingSourceState(getMeetingSourceLabel(source));
    updateMeetingSourceState(source, {
      active: true,
      status: "listening",
      note: "\u5b9e\u65f6\u8f6c\u5199\u4e2d",
      latestText: transcript,
      error: "",
      lastUpdatedAt: timestamp,
      lastSignalAt: now,
      chunkCount: (sourceState.chunkCount || 0) + 1,
      recentItems: trimCaptionItems(
        [
          {
            id: `meeting-source-${source}-${now}`,
            text: transcript,
            time: timestamp,
            capturedAt: now
          },
          ...(sourceState.recentItems || [])
        ],
        now,
        {
          historyMs: MEETING_CAPTION_HISTORY_MS,
          maxItems: MEETING_CAPTION_MAX_ITEMS
        }
      )
    });
    appState.meetingSummary = {
      ...appState.meetingSummary,
      pendingTranscriptBlocks: [
        ...(appState.meetingSummary.pendingTranscriptBlocks || []),
        {
          id: `meeting-pending-${source}-${now}`,
          source,
          time: timestamp,
          capturedAt: now,
          text: transcript
        }
      ].slice(-32)
    };
    syncMeetingSummaryToListeningState();
    broadcastSnapshot();
    return {
      ok: true,
      transcript
    };
  } catch (error) {
    return markMeetingSourceError(source, error);
  }
}

async function refreshMeetingSummary(options = {}) {
  const pendingTranscriptBlocks = [...(appState.meetingSummary.pendingTranscriptBlocks || [])];
  const now = Date.now();
  const blockGroups = splitTranscriptBlocksBySource(pendingTranscriptBlocks, now, {
    windowSeconds: MEETING_SUMMARY_WINDOW_SECONDS
  });
  const transcript = blockGroups.recentBlocks
    .map((entry) => `[${getMeetingSourceLabel(entry.source)}${entry.time ? ` ${entry.time}` : ""}] ${entry.text}`)
    .join("\n");
  const processedIds = new Set(pendingTranscriptBlocks.map((entry) => entry.id));
  const remainingTranscriptBlocks = (appState.meetingSummary.pendingTranscriptBlocks || []).filter(
    (entry) => !processedIds.has(entry.id)
  );

  if (!blockGroups.hasAny) {
    if (options.force) {
      return {
        ok: true,
        skipped: true,
        summary: appState.meetingSummary.latestSummary,
        transcript: appState.meetingSummary.latestTranscript || ""
      };
    }

    const emptySummary = createUnavailableMeetingSummary();
    appState.meetingSummary = {
      ...appState.meetingSummary,
      status: getMeetingSourceCount() ? "listening" : "starting",
      note: getMeetingSourceCount()
        ? getMeetingListeningNote()
        : "\u7b49\u5f85\u97f3\u9891\u6e90\u7ee7\u7eed\u8f93\u5165\u3002",
      latestTranscript: "",
      latestSummary: emptySummary,
      error: "",
      lastUpdatedAt: emptySummary.updatedAt,
      pendingTranscriptBlocks: remainingTranscriptBlocks
    };
    broadcastSnapshot();
    return {
      ok: true,
      skipped: true,
      summary: emptySummary,
      transcript: ""
    };
  }

  try {
    appState.meetingSummary = {
      ...appState.meetingSummary,
      status: "processing",
      error: "",
      note: "\u6b63\u5728\u57fa\u4e8e\u6700\u65b0\u8f6c\u5199\u751f\u6210\u4f1a\u8bae\u6982\u8981..."
    };
    broadcastSnapshot();

    const summary = await summarizeMeetingTranscript(blockGroups);
    const nextChunkCount = appState.meetingSummary.chunkCount + 1;
    const activeSourceCount = getMeetingSourceCount();
    appState.meetingSummary = {
      ...appState.meetingSummary,
      status: activeSourceCount ? "listening" : "starting",
      note: activeSourceCount ? getMeetingListeningNote() : "\u7b49\u5f85\u97f3\u9891\u6e90\u7ee7\u7eed\u8f93\u5165\u3002",
      latestTranscript: transcript,
      latestSummary: summary,
      error: "",
      lastUpdatedAt: summary.updatedAt || formatClockTime(),
      chunkCount: nextChunkCount,
      pendingTranscriptBlocks: remainingTranscriptBlocks
    };
    pushAction("\u4f1a\u8bae\u6982\u8981\u5df2\u66f4\u65b0", `\u7b2c ${nextChunkCount} \u6b21\u6982\u8981\u5df2\u751f\u6210`);
    broadcastSnapshot();
    return {
      ok: true,
      summary,
      transcript
    };
  } catch (error) {
    return setMeetingSummaryError(error);
  }
}

function saveModelConfig(payload) {
  const sanitized = sanitizeModelConfigBundle(payload);
  const invalidGroups = [
    {
      key: "transcription",
      label: "\u8bed\u97f3\u8f6c\u5199"
    },
    {
      key: "summary",
      label: "\u6587\u672c\u603b\u7ed3"
    }
  ].filter((group) => {
    const config = sanitized[group.key];
    return hasAnyModelConfigValue(config) && !hasCompleteModelConfig(config);
  });

  if (invalidGroups.length) {
    return {
      ok: false,
      error: `${invalidGroups.map((group) => `${group.label}\u914d\u7f6e`).join("\u3001")}\u8bf7\u5b8c\u6574\u586b\u5199 baseUrl\u3001apiKey \u548c model\uff0c\u6216\u8005\u5c06\u8be5\u7ec4\u5168\u90e8\u6e05\u7a7a\u3002`
    };
  }

  modelConfigState.transcription = {
    ...sanitized.transcription
  };
  modelConfigState.summary = {
    ...sanitized.summary
  };
  persistSettings();
  pushAction(
    "\u5df2\u4fdd\u5b58\u6a21\u578b\u914d\u7f6e",
    [
      sanitized.transcription.model
        ? `\u8f6c\u5199\uff1a${sanitized.transcription.model}`
        : "\u8f6c\u5199\uff1a\u672a\u914d\u7f6e",
      sanitized.summary.model
        ? `\u603b\u7ed3\uff1a${sanitized.summary.model}`
        : "\u603b\u7ed3\uff1a\u672a\u914d\u7f6e"
    ].join(" / ")
  );
  broadcastSnapshot();
  return {
    ok: true
  };
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

function sanitizeStoredOverlayBounds(bounds, fallbackSize) {
  if (!bounds || typeof bounds !== "object") {
    return null;
  }

  const parsed = {
    x: Number(bounds.x),
    y: Number(bounds.y),
    width: Number(bounds.width),
    height: Number(bounds.height)
  };

  if (!Number.isFinite(parsed.x) || !Number.isFinite(parsed.y) || !Number.isFinite(parsed.width) || !Number.isFinite(parsed.height)) {
    return null;
  }

  return {
    x: Math.round(parsed.x),
    y: Math.round(parsed.y),
    width: Math.max(fallbackSize.minWidth || 240, Math.round(parsed.width)),
    height: Math.max(fallbackSize.minHeight || 180, Math.round(parsed.height))
  };
}

function getOverlayBoundsState(kind) {
  return kind === "captions" ? overlayWindowState.captions : overlayWindowState.summary;
}

function getOverlayWindowDefaults(kind) {
  return kind === "captions" ? MEETING_CAPTIONS_WINDOW : MEETING_SUMMARY_WINDOW;
}

function clampOverlayBounds(bounds) {
  const display = screen.getDisplayMatching(bounds).workArea;
  const clampedWidth = Math.min(bounds.width, display.width - 24);
  const clampedHeight = Math.min(bounds.height, display.height - 24);
  return clampBounds(
    {
      ...bounds,
      width: Math.max(260, Math.round(clampedWidth)),
      height: Math.max(180, Math.round(clampedHeight))
    },
    display
  );
}

function getDefaultOverlayBounds(kind) {
  const workArea = getNearestDisplay(getCollapsedAnchorBounds()).workArea;
  if (kind === "captions") {
    return clampOverlayBounds({
      x: workArea.x + workArea.width - MEETING_CAPTIONS_WINDOW.width - 28,
      y: workArea.y + 304,
      width: MEETING_CAPTIONS_WINDOW.width,
      height: MEETING_CAPTIONS_WINDOW.height
    });
  }

  return clampOverlayBounds({
    x: workArea.x + workArea.width - MEETING_SUMMARY_WINDOW.width - 28,
    y: workArea.y + 34,
    width: MEETING_SUMMARY_WINDOW.width,
    height: MEETING_SUMMARY_WINDOW.height
  });
}

function getOverlayBounds(kind) {
  const storedBounds = getOverlayBoundsState(kind).bounds;
  return clampOverlayBounds(storedBounds || getDefaultOverlayBounds(kind));
}

function rectToPlain(bounds) {
  if (!bounds) {
    return null;
  }

  return {
    x: Math.round(bounds.x),
    y: Math.round(bounds.y),
    width: Math.round(bounds.width),
    height: Math.round(bounds.height)
  };
}

function rectOverlapArea(first, second) {
  if (!first || !second) {
    return 0;
  }

  const width = Math.max(0, Math.min(first.x + first.width, second.x + second.width) - Math.max(first.x, second.x));
  const height = Math.max(0, Math.min(first.y + first.height, second.y + second.height) - Math.max(first.y, second.y));
  return width * height;
}

function isRectOutsideWorkArea(bounds, workArea) {
  if (!bounds || !workArea) {
    return false;
  }

  return (
    bounds.x < workArea.x
    || bounds.y < workArea.y
    || (bounds.x + bounds.width) > (workArea.x + workArea.width)
    || (bounds.y + bounds.height) > (workArea.y + workArea.height)
  );
}

function getMeetingOverlayLayoutInput() {
  const referenceBounds = chatWindow && !chatWindow.isDestroyed() && chatWindow.isVisible()
    ? chatWindow.getBounds()
    : getCollapsedAnchorBounds();
  const workArea = getNearestDisplay(referenceBounds).workArea;
  const summaryBounds = meetingSummaryWindow && !meetingSummaryWindow.isDestroyed()
    ? meetingSummaryWindow.getBounds()
    : getOverlayBounds("summary");
  const captionsBounds = meetingCaptionsWindow && !meetingCaptionsWindow.isDestroyed()
    ? meetingCaptionsWindow.getBounds()
    : getOverlayBounds("captions");

  return {
    workArea: rectToPlain(workArea),
    chatVisible: Boolean(chatWindow && !chatWindow.isDestroyed() && chatWindow.isVisible()),
    chatBounds: chatWindow && !chatWindow.isDestroyed() && chatWindow.isVisible()
      ? rectToPlain(chatWindow.getBounds())
      : null,
    orbBounds: rectToPlain(getCollapsedAnchorBounds()),
    summaryBounds: rectToPlain(summaryBounds),
    captionsBounds: rectToPlain(captionsBounds),
    summaryMinWidth: MEETING_SUMMARY_WINDOW.minWidth,
    summaryMinHeight: MEETING_SUMMARY_WINDOW.minHeight,
    captionsMinWidth: MEETING_CAPTIONS_WINDOW.minWidth,
    captionsMinHeight: MEETING_CAPTIONS_WINDOW.minHeight,
    margin: 24,
    gap: 16
  };
}

function computeMeetingOverlayLayoutFallback(input) {
  const workArea = input.workArea;
  const margin = Math.max(24, Number(input.margin || 0));
  const gap = Math.max(16, Number(input.gap || 0));
  const chatBounds = input.chatVisible ? input.chatBounds : null;
  const summaryWidth = clampValue(input.summaryBounds.width, input.summaryMinWidth, workArea.width - (margin * 2));
  const captionsWidth = clampValue(input.captionsBounds.width, input.captionsMinWidth, workArea.width - (margin * 2));
  const requiredWidth = Math.max(summaryWidth, captionsWidth);
  const leftSpace = chatBounds ? chatBounds.x - workArea.x - (margin * 2) : 0;
  const rightSpace = chatBounds ? (workArea.x + workArea.width) - (chatBounds.x + chatBounds.width) - (margin * 2) : 0;
  let side = "right";

  if (chatBounds) {
    if (leftSpace >= requiredWidth && leftSpace >= rightSpace) {
      side = "left";
    } else if (rightSpace >= requiredWidth) {
      side = "right";
    } else {
      side = leftSpace >= rightSpace ? "left" : "right";
    }
  } else {
    const orbBounds = input.orbBounds;
    const orbCenter = orbBounds ? orbBounds.x + (orbBounds.width / 2) : workArea.x + workArea.width;
    side = orbCenter >= (workArea.x + (workArea.width / 2)) ? "left" : "right";
  }

  const availableWidth = chatBounds
    ? Math.max(
      side === "left"
        ? leftSpace
        : rightSpace,
      Math.max(input.summaryMinWidth, input.captionsMinWidth)
    )
    : workArea.width - (margin * 2);
  const finalSummaryWidth = clampValue(summaryWidth, input.summaryMinWidth, availableWidth);
  const finalCaptionsWidth = clampValue(captionsWidth, input.captionsMinWidth, availableWidth);
  let finalSummaryHeight = clampValue(input.summaryBounds.height, input.summaryMinHeight, workArea.height - (margin * 2));
  let finalCaptionsHeight = clampValue(input.captionsBounds.height, input.captionsMinHeight, workArea.height - (margin * 2));
  const availableHeight = workArea.height - (margin * 2);

  if ((finalSummaryHeight + gap + finalCaptionsHeight) > availableHeight) {
    let overflow = (finalSummaryHeight + gap + finalCaptionsHeight) - availableHeight;
    const captionsReducible = Math.max(0, finalCaptionsHeight - input.captionsMinHeight);
    const reduceCaptions = Math.min(overflow, captionsReducible);
    finalCaptionsHeight -= reduceCaptions;
    overflow -= reduceCaptions;
    const summaryReducible = Math.max(0, finalSummaryHeight - input.summaryMinHeight);
    const reduceSummary = Math.min(overflow, summaryReducible);
    finalSummaryHeight -= reduceSummary;
  }

  const summaryX = side === "left"
    ? workArea.x + margin
    : workArea.x + workArea.width - margin - finalSummaryWidth;
  const captionsX = side === "left"
    ? workArea.x + margin
    : workArea.x + workArea.width - margin - finalCaptionsWidth;
  const summaryY = workArea.y + margin;
  const captionsY = summaryY + finalSummaryHeight + gap;

  return {
    summary: clampOverlayBounds({
      x: summaryX,
      y: summaryY,
      width: finalSummaryWidth,
      height: finalSummaryHeight
    }),
    captions: clampOverlayBounds({
      x: captionsX,
      y: captionsY,
      width: finalCaptionsWidth,
      height: finalCaptionsHeight
    })
  };
}

function shouldReflowMeetingOverlays(input) {
  const summaryBounds = input.summaryBounds;
  const captionsBounds = input.captionsBounds;
  const chatBounds = input.chatVisible ? input.chatBounds : null;
  return (
    rectOverlapArea(summaryBounds, captionsBounds) > 0
    || rectOverlapArea(summaryBounds, chatBounds) > 0
    || rectOverlapArea(captionsBounds, chatBounds) > 0
    || isRectOutsideWorkArea(summaryBounds, input.workArea)
    || isRectOutsideWorkArea(captionsBounds, input.workArea)
  );
}

function applyMeetingOverlayLayout(layout) {
  if (meetingSummaryWindow && !meetingSummaryWindow.isDestroyed() && layout?.summary) {
    meetingSummaryWindow.setBounds(clampOverlayBounds(layout.summary), false);
    updateOverlayWindowBoundsState("summary", meetingSummaryWindow);
  }
  if (meetingCaptionsWindow && !meetingCaptionsWindow.isDestroyed() && layout?.captions) {
    meetingCaptionsWindow.setBounds(clampOverlayBounds(layout.captions), false);
    updateOverlayWindowBoundsState("captions", meetingCaptionsWindow);
  }
  scheduleOverlayBoundsPersist("summary");
  scheduleOverlayBoundsPersist("captions");
}

async function arrangeMeetingOverlays(options = {}) {
  const hasSummaryVisible = Boolean(meetingSummaryWindow && !meetingSummaryWindow.isDestroyed() && meetingSummaryWindow.isVisible());
  const hasCaptionsVisible = Boolean(meetingCaptionsWindow && !meetingCaptionsWindow.isDestroyed() && meetingCaptionsWindow.isVisible());
  if (!hasSummaryVisible && !hasCaptionsVisible) {
    return;
  }

  const input = getMeetingOverlayLayoutInput();
  if (!options.force && !shouldReflowMeetingOverlays(input)) {
    return;
  }

  const goLayout = await invokeGoBridge("layout-overlays", input);
  applyMeetingOverlayLayout(goLayout && goLayout.summary && goLayout.captions
    ? goLayout
    : computeMeetingOverlayLayoutFallback(input));
  broadcastSnapshot();
}

function scheduleMeetingOverlayArrangement(force = false) {
  meetingOverlayLayoutForcePending = meetingOverlayLayoutForcePending || force;
  if (meetingOverlayLayoutTimer) {
    return;
  }

  meetingOverlayLayoutTimer = setTimeout(() => {
    const shouldForce = meetingOverlayLayoutForcePending;
    meetingOverlayLayoutForcePending = false;
    meetingOverlayLayoutTimer = null;
    void arrangeMeetingOverlays({
      force: shouldForce
    });
  }, 80);
}

function scheduleOverlayBoundsPersist(kind) {
  const timerName = kind === "captions" ? "meetingCaptionsOverlayPersistTimer" : "meetingSummaryOverlayPersistTimer";
  const timer = timerName === "meetingCaptionsOverlayPersistTimer"
    ? meetingCaptionsOverlayPersistTimer
    : meetingSummaryOverlayPersistTimer;

  if (timer) {
    clearTimeout(timer);
  }

  const nextTimer = setTimeout(() => {
    if (kind === "captions") {
      meetingCaptionsOverlayPersistTimer = null;
    } else {
      meetingSummaryOverlayPersistTimer = null;
    }
    persistSettings();
  }, 160);

  if (kind === "captions") {
    meetingCaptionsOverlayPersistTimer = nextTimer;
  } else {
    meetingSummaryOverlayPersistTimer = nextTimer;
  }
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

function hasVisibleOrbMeetingPrompt() {
  return Boolean(
    appState.meetingDetection.promptVisible
    && !orbState.hidden
    && orbState.mode !== "ring"
  );
}

function getMeetingPromptBounds() {
  const collapsed = getCollapsedAnchorBounds();
  const display = getNearestDisplay(collapsed).workArea;
  const width = MEETING_PROMPT_WINDOW.width;
  const height = MEETING_PROMPT_WINDOW.height;
  const collapsedCenterX = collapsed.x + Math.round(collapsed.width / 2);
  const collapsedCenterY = collapsed.y + Math.round(collapsed.height / 2);
  let x = collapsed.x;
  let y = collapsed.y;
  let anchorX = Math.round(collapsed.width / 2) + 18;
  let anchorY = Math.round(collapsed.height / 2);

  if (orbState.dockSide === "right") {
    x = collapsed.x + collapsed.width - width;
    y = Math.round(collapsedCenterY - height / 2);
    anchorX = width - Math.round(collapsed.width / 2) - 18;
    anchorY = Math.round(height / 2);
  } else if (orbState.dockSide === "left") {
    x = collapsed.x;
    y = Math.round(collapsedCenterY - height / 2);
    anchorX = Math.round(collapsed.width / 2) + 18;
    anchorY = Math.round(height / 2);
  } else if (orbState.dockSide === "top") {
    x = Math.round(collapsedCenterX - width / 2);
    y = collapsed.y;
    anchorX = Math.round(width / 2);
    anchorY = Math.round(collapsed.height / 2) + 18;
  } else {
    x = Math.round(collapsedCenterX - width / 2);
    y = collapsed.y + collapsed.height - height;
    anchorX = Math.round(width / 2);
    anchorY = height - Math.round(collapsed.height / 2) - 18;
  }

  const clamped = clampBounds(
    {
      x,
      y,
      width,
      height
    },
    display
  );

  return {
    ...clamped,
    anchorX,
    anchorY
  };
}

function getPeekBounds() {
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

  return peekBounds;
}

function getOrbLayout() {
  const collapsed = getCollapsedAnchorBounds();
  const size = getCollapsedSize();
  const currentBounds = orbState.mode === "ring"
    ? orbState.expandedBounds || getExpandedBounds()
    : hasVisibleOrbMeetingPrompt()
      ? getMeetingPromptBounds()
      : collapsed;
  const isExpanded = orbState.mode === "ring";
  const anchorX = isExpanded || hasVisibleOrbMeetingPrompt()
    ? currentBounds.anchorX
    : Math.round(currentBounds.width / 2);
  const anchorY = isExpanded || hasVisibleOrbMeetingPrompt()
    ? currentBounds.anchorY
    : Math.round(currentBounds.height / 2);

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
  const meetingSummary = appState.meetingSummary;
  const meetingDetection = appState.meetingDetection;
  const sourceEntries = Object.entries(meetingSummary.sources || {}).map(([source, sourceState]) => [
    source,
    {
      ...sourceState,
      statusLabel: getMeetingSourceStatusLabel(sourceState)
    }
  ]);

  return {
    state: {
      ...appState,
      meetingDetection: {
        ...meetingDetection,
        statusLabel: getMeetingDetectionStatusLabel()
      },
      meetingSummary: {
        ...meetingSummary,
        statusLabel: getMeetingSummaryStatusLabel(),
        latestSummary: normalizeStructuredSummary(meetingSummary.latestSummary),
        summaryOverlayVisible: Boolean(meetingSummaryWindow && meetingSummaryWindow.isVisible()),
        captionsOverlayVisible: Boolean(meetingCaptionsWindow && meetingCaptionsWindow.isVisible()),
        sources: Object.fromEntries(sourceEntries)
      },
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
  syncMainWindowPresentation();
  sendSnapshot(mainWindow);
  sendSnapshot(chatWindow);
  sendSnapshot(panelWindow);
  sendSnapshot(meetingSummaryWindow);
  sendSnapshot(meetingCaptionsWindow);
}

function scheduleMeetingSnapshotBroadcast() {
  if (meetingSnapshotBroadcastTimer) {
    return;
  }

  meetingSnapshotBroadcastTimer = setTimeout(() => {
    meetingSnapshotBroadcastTimer = null;
    broadcastSnapshot();
  }, MEETING_REALTIME_SNAPSHOT_THROTTLE_MS);
}

function flushMeetingSnapshotBroadcast() {
  if (meetingSnapshotBroadcastTimer) {
    clearTimeout(meetingSnapshotBroadcastTimer);
    meetingSnapshotBroadcastTimer = null;
  }
  broadcastSnapshot();
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

  const current = mainWindow.getBounds();
  if (
    Math.round(current.x) === Math.round(bounds.x)
    && Math.round(current.y) === Math.round(bounds.y)
    && Math.round(current.width) === Math.round(bounds.width)
    && Math.round(current.height) === Math.round(bounds.height)
  ) {
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

function syncMainWindowPresentation() {
  if (!mainWindow || mainWindow.isDestroyed() || orbState.hidden) {
    return;
  }

  if (orbState.mode === "ring") {
    setMainWindowBounds(orbState.expandedBounds || getExpandedBounds());
    return;
  }

  if (hasVisibleOrbMeetingPrompt()) {
    setMainWindowBounds(getMeetingPromptBounds());
    return;
  }

  if (orbState.peeked) {
    setMainWindowBounds(getPeekBounds());
    return;
  }

  setMainWindowBounds(getCollapsedAnchorBounds());
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
  if (appState.meetingDetection.promptVisible) {
    return;
  }
  if (chatWindow && chatWindow.isVisible()) {
    return;
  }

  orbState.peeked = true;
  setMainWindowBounds(getPeekBounds());
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
  const currentBounds = chatWindow.getBounds();
  const chatWidth = currentBounds.width || CHAT_WINDOW.width;
  const chatHeight = currentBounds.height || CHAT_WINDOW.height;
  let x;
  let y = Math.round(anchor.y + anchor.height / 2 - chatHeight / 2);

  if (orbState.dockSide === "left") {
    x = anchor.x + anchor.width + 14;
  } else if (orbState.dockSide === "right") {
    x = anchor.x - chatWidth - 14;
  } else {
    const prefersRight = anchor.x + anchor.width / 2 < display.x + display.width / 2;
    x = prefersRight ? anchor.x + anchor.width + 14 : anchor.x - chatWidth - 14;
  }

  if (orbState.dockSide === "top") {
    y = anchor.y + anchor.height + 12;
  }
  if (orbState.dockSide === "bottom") {
    y = anchor.y - chatHeight - 12;
  }

  x = Math.min(Math.max(x, display.x + 12), display.x + display.width - chatWidth - 12);
  y = Math.min(Math.max(y, display.y + 12), display.y + display.height - chatHeight - 12);

  chatWindow.setBounds(
    {
      x: Math.round(x),
      y: Math.round(y),
      width: chatWidth,
      height: chatHeight
    },
    false
  );
  if (appState.meetingSummary.enabled) {
    scheduleMeetingOverlayArrangement(false);
  }
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
    minWidth: 392,
    minHeight: 472,
    show: false,
    frame: false,
    transparent: false,
    backgroundColor: "#08141e",
    resizable: true,
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

function updateOverlayWindowBoundsState(kind, targetWindow) {
  if (!targetWindow || targetWindow.isDestroyed()) {
    return;
  }

  getOverlayBoundsState(kind).bounds = clampOverlayBounds(targetWindow.getBounds());
}

function attachOverlayWindowPersistence(kind, targetWindow) {
  const persistBounds = () => {
    updateOverlayWindowBoundsState(kind, targetWindow);
    scheduleOverlayBoundsPersist(kind);
  };

  targetWindow.on("move", persistBounds);
  targetWindow.on("resize", persistBounds);
  targetWindow.on("hide", () => {
    updateOverlayWindowBoundsState(kind, targetWindow);
    persistSettings();
    broadcastSnapshot();
  });
  targetWindow.on("show", () => {
    updateOverlayWindowBoundsState(kind, targetWindow);
    broadcastSnapshot();
  });
}

function createMeetingOverlayWindow(kind) {
  const defaults = getOverlayWindowDefaults(kind);
  const bounds = getOverlayBounds(kind);
  const windowName = kind === "captions" ? "meeting-captions-overlay" : "meeting-summary-overlay";
  const htmlFile = kind === "captions" ? "meeting-captions-overlay.html" : "meeting-summary-overlay.html";
  const targetWindow = new BrowserWindow({
    x: bounds.x,
    y: bounds.y,
    width: bounds.width,
    height: bounds.height,
    minWidth: defaults.minWidth,
    minHeight: defaults.minHeight,
    show: false,
    frame: false,
    transparent: true,
    backgroundColor: "#00000000",
    resizable: true,
    minimizable: false,
    maximizable: false,
    skipTaskbar: true,
    alwaysOnTop: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      additionalArguments: [`--window=${windowName}`],
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  targetWindow.setAlwaysOnTop(true, "screen-saver");
  targetWindow.loadFile(path.join(__dirname, "src", "renderer", htmlFile));
  targetWindow.on("close", (event) => {
    if (appIsQuitting) {
      return;
    }
    event.preventDefault();
    targetWindow.hide();
  });
  targetWindow.on("closed", () => {
    if (kind === "captions") {
      meetingCaptionsWindow = null;
    } else {
      meetingSummaryWindow = null;
    }
    broadcastSnapshot();
  });
  attachOverlayWindowPersistence(kind, targetWindow);

  return targetWindow;
}

function ensureMeetingSummaryWindow() {
  if (meetingSummaryWindow && !meetingSummaryWindow.isDestroyed()) {
    return meetingSummaryWindow;
  }

  meetingSummaryWindow = createMeetingOverlayWindow("summary");
  return meetingSummaryWindow;
}

function ensureMeetingCaptionsWindow() {
  if (meetingCaptionsWindow && !meetingCaptionsWindow.isDestroyed()) {
    return meetingCaptionsWindow;
  }

  meetingCaptionsWindow = createMeetingOverlayWindow("captions");
  return meetingCaptionsWindow;
}

function showMeetingSummaryOverlay(options = {}) {
  const target = ensureMeetingSummaryWindow();
  if (!target.isVisible()) {
    target.showInactive();
  }
  if (options.focus === true) {
    target.focus();
  }
  scheduleMeetingOverlayArrangement(Boolean(options.forceLayout));
  broadcastSnapshot();
}

function showMeetingCaptionsOverlay(options = {}) {
  const target = ensureMeetingCaptionsWindow();
  if (!target.isVisible()) {
    target.showInactive();
  }
  if (options.focus === true) {
    target.focus();
  }
  scheduleMeetingOverlayArrangement(Boolean(options.forceLayout));
  broadcastSnapshot();
}

function hideMeetingSummaryOverlay() {
  if (meetingSummaryWindow && !meetingSummaryWindow.isDestroyed()) {
    meetingSummaryWindow.hide();
  }
}

function hideMeetingCaptionsOverlay() {
  if (meetingCaptionsWindow && !meetingCaptionsWindow.isDestroyed()) {
    meetingCaptionsWindow.hide();
  }
}

function resizeOverlayWindow(senderWindow, nextBounds = {}) {
  if (!senderWindow || senderWindow.isDestroyed()) {
    return;
  }

  const isSummaryWindow = senderWindow === meetingSummaryWindow;
  const isCaptionsWindow = senderWindow === meetingCaptionsWindow;
  if (!isSummaryWindow && !isCaptionsWindow) {
    return;
  }

  const defaults = isCaptionsWindow ? MEETING_CAPTIONS_WINDOW : MEETING_SUMMARY_WINDOW;
  const currentBounds = senderWindow.getBounds();
  const width = Math.max(defaults.minWidth, Math.round(Number(nextBounds.width) || currentBounds.width));
  const height = Math.max(defaults.minHeight, Math.round(Number(nextBounds.height) || currentBounds.height));
  const clamped = clampOverlayBounds({
    ...currentBounds,
    width,
    height
  });
  senderWindow.setBounds(clamped, false);
  updateOverlayWindowBoundsState(isCaptionsWindow ? "captions" : "summary", senderWindow);
  scheduleOverlayBoundsPersist(isCaptionsWindow ? "captions" : "summary");
  broadcastSnapshot();
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
  if (meetingSummaryWindow && !meetingSummaryWindow.isDestroyed()) {
    meetingSummaryWindow.hide();
  }
  if (meetingCaptionsWindow && !meetingCaptionsWindow.isDestroyed()) {
    meetingCaptionsWindow.hide();
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

  if (patch.meetingAutoDetect && typeof patch.meetingAutoDetect === "object") {
    appState.meetingDetection = {
      ...appState.meetingDetection,
      enabled: !Object.prototype.hasOwnProperty.call(patch.meetingAutoDetect, "enabled")
        ? appState.meetingDetection.enabled
        : Boolean(patch.meetingAutoDetect.enabled),
      promptVisible: false
    };
  }
}

function updatePreferences(patch) {
  const previousMeetingAutoDetectEnabled = appState.meetingDetection.enabled;
  const requestedMeetingAutoDetectEnabled = patch.meetingAutoDetect && Object.prototype.hasOwnProperty.call(patch.meetingAutoDetect, "enabled")
    ? Boolean(patch.meetingAutoDetect.enabled)
    : null;
  const requestedActiveAppEnabled = patch.abilityToggles && Object.prototype.hasOwnProperty.call(patch.abilityToggles, "activeApp")
    ? Boolean(patch.abilityToggles.activeApp)
    : null;
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

  if (requestedMeetingAutoDetectEnabled === true && !appState.abilityToggles.activeApp) {
    appState.abilityToggles = {
      ...appState.abilityToggles,
      activeApp: true
    };
  }

  if (requestedActiveAppEnabled === false && appState.meetingDetection.enabled) {
    appState.meetingDetection = {
      ...appState.meetingDetection,
      enabled: false,
      status: "idle",
      promptVisible: false,
      note: "\u5df2\u5173\u95ed\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\uff0c\u56e0\u4e3a\u8be5\u529f\u80fd\u4f9d\u8d56\u6d3b\u52a8\u5e94\u7528\u611f\u77e5\u3002"
    };
  }

  if (previousMeetingAutoDetectEnabled && !appState.meetingDetection.enabled) {
    resetMeetingDetectionSession({
      note: requestedActiveAppEnabled === false
        ? "\u5df2\u5173\u95ed\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\uff0c\u56e0\u4e3a\u4f60\u624b\u52a8\u5173\u95ed\u4e86\u6d3b\u52a8\u5e94\u7528\u611f\u77e5\u3002"
        : "\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\u5df2\u5173\u95ed\u3002"
    });
  } else if (!previousMeetingAutoDetectEnabled && appState.meetingDetection.enabled) {
    appState.meetingDetection = {
      ...appState.meetingDetection,
      status: "idle",
      note: "\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\u5df2\u5f00\u542f\uff0c\u547d\u4e2d\u4f1a\u8bae\u8f6f\u4ef6\u540e\u4f1a\u63d0\u9192\u4f60\u542f\u52a8\u4f1a\u8bae\u603b\u7ed3\u3002"
    };
  }

  if (!appState.abilityToggles.activeApp) {
    appState.meetingDetection = {
      ...appState.meetingDetection,
      status: "idle",
      promptVisible: false,
      note: "\u6d3b\u52a8\u5e94\u7528\u611f\u77e5\u5df2\u5173\u95ed\uff0c\u4f1a\u8bae\u81ea\u52a8\u8bc6\u522b\u5df2\u6682\u505c\u3002"
    };
  }

  const targetBounds = {
    ...getCollapsedAnchorBounds(),
    width: getCollapsedSize(),
    height: getCollapsedSize()
  };
  applyCollapsedBounds(targetBounds);
  persistSettings();
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
    const windowType = senderWindow === mainWindow
      ? "orb"
      : senderWindow === chatWindow
        ? "chat"
        : senderWindow === panelWindow
          ? "panel"
          : senderWindow === meetingSummaryWindow
            ? "meeting-summary-overlay"
            : senderWindow === meetingCaptionsWindow
              ? "meeting-captions-overlay"
              : "unknown";
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

  ipcMain.handle("settings:model:get", () => ({
    transcription: {
      ...modelConfigState.transcription
    },
    summary: {
      ...modelConfigState.summary
    }
  }));

  ipcMain.handle("settings:model:save", (_event, payload) => {
    markActivity();
    return saveModelConfig(payload || {});
  });

  ipcMain.handle("meeting-summary:start", (_event, payload) => {
    markActivity();
    return startMeetingSummarySession(payload || {});
  });

  ipcMain.on("meeting-summary-overlay:show", () => {
    markActivity();
    showMeetingSummaryOverlay();
  });

  ipcMain.on("meeting-summary-overlay:hide", () => {
    hideMeetingSummaryOverlay();
  });

  ipcMain.on("meeting-captions-overlay:show", () => {
    markActivity();
    showMeetingCaptionsOverlay();
  });

  ipcMain.on("meeting-captions-overlay:hide", () => {
    hideMeetingCaptionsOverlay();
  });

  ipcMain.on("window:resize-current", (event, payload) => {
    resizeOverlayWindow(BrowserWindow.fromWebContents(event.sender), payload || {});
  });

  ipcMain.handle("meeting-detection:accept", () => {
    markActivity();
    return acceptMeetingDetectionPrompt();
  });

  ipcMain.handle("meeting-detection:dismiss", () => {
    markActivity();
    return dismissMeetingDetectionPrompt();
  });

  ipcMain.on("meeting-summary:capture-ready", (_event, payload) => {
    markActivity();
    markMeetingCaptureReady(payload?.source === "microphone" ? "microphone" : "system");
  });

  ipcMain.on("meeting-summary:source-error", (_event, payload) => {
    markActivity();
    markMeetingSourceError(payload?.source === "microphone" ? "microphone" : "system", payload?.message);
  });

  ipcMain.on("meeting-summary:audio-append", (_event, payload) => {
    appendMeetingRealtimeAudio(payload || {});
  });

  ipcMain.handle("meeting-summary:chunk", async (_event, payload) => {
    markActivity();
    return processMeetingSummaryChunk(payload);
  });

  ipcMain.handle("meeting-summary:finalize-transcription", async () => {
    markActivity();
    return finalizeMeetingRealtimeTranscription();
  });

  ipcMain.handle("meeting-summary:refresh-summary", async (_event, payload) => {
    markActivity();
    return refreshMeetingSummary(payload || {});
  });

  ipcMain.handle("meeting-summary:stop", () => {
    markActivity();
    return stopMeetingSummarySession({
      clearError: true,
      reason: "manual"
    });
  });

  ipcMain.on("meeting-summary:error", (_event, payload) => {
    markActivity();
    setMeetingSummaryError(payload?.message || "\u4f1a\u8bae\u603b\u7ed3\u88ab\u4e2d\u65ad");
  });

  ipcMain.on("panel:close", () => {
    if (panelWindow && !panelWindow.isDestroyed()) {
      panelWindow.close();
    }
  });
}

app.whenReady().then(() => {
  loadPersistedSettings();
  void ensureGoBridgeBinary();
  registerDisplayMediaHandler();
  Menu.setApplicationMenu(null);
  registerIpc();
  createMainWindow();
  createTray();
  startProximityLoop();
  startMeetingDetectionLoop();
});

app.on("before-quit", () => {
  appIsQuitting = true;
  stopMeetingDetectionLoop();
});

app.on("window-all-closed", (event) => {
  if (!appIsQuitting) {
    event.preventDefault();
  }
});
