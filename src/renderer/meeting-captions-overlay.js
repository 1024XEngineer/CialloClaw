const api = window.pixelOrb;

const elements = {
  captionsStatus: document.getElementById("captionsStatus"),
  captionsMeta: document.getElementById("captionsMeta"),
  othersStatus: document.getElementById("othersStatus"),
  selfStatus: document.getElementById("selfStatus"),
  othersFeed: document.getElementById("othersFeed"),
  selfFeed: document.getElementById("selfFeed"),
  captionsUpdatedAt: document.getElementById("captionsUpdatedAt"),
  showSummaryButton: document.getElementById("showSummaryButton"),
  closeButton: document.getElementById("closeButton"),
  resizeHandle: document.getElementById("resizeHandle")
};

const state = {
  snapshot: null,
  resize: null,
  pendingResizeFrame: 0,
  autoScroll: {
    others: true,
    self: true
  }
};

function getMeetingState() {
  return state.snapshot?.state?.meetingSummary || null;
}

function scheduleResize(width, height) {
  if (state.pendingResizeFrame) {
    cancelAnimationFrame(state.pendingResizeFrame);
  }

  state.pendingResizeFrame = requestAnimationFrame(() => {
    state.pendingResizeFrame = 0;
    api.resizeCurrentWindow({
      width,
      height
    });
  });
}

function onResizeMove(event) {
  if (!state.resize) {
    return;
  }

  const width = Math.max(420, state.resize.width + (event.screenX - state.resize.screenX));
  const height = Math.max(260, state.resize.height + (event.screenY - state.resize.screenY));
  scheduleResize(width, height);
}

function stopResize() {
  if (!state.resize) {
    return;
  }

  state.resize = null;
  window.removeEventListener("mousemove", onResizeMove);
  window.removeEventListener("mouseup", stopResize);
}

function startResize(event) {
  event.preventDefault();
  state.resize = {
    screenX: event.screenX,
    screenY: event.screenY,
    width: window.innerWidth,
    height: window.innerHeight
  };
  window.addEventListener("mousemove", onResizeMove);
  window.addEventListener("mouseup", stopResize);
}

function updateAutoScroll(key, container) {
  state.autoScroll[key] = (container.scrollHeight - container.scrollTop - container.clientHeight) < 18;
}

function createCaptionItem(item, className = "caption-item") {
  const article = document.createElement("article");
  article.className = className;

  const time = document.createElement("time");
  time.textContent = item.time || "";
  article.appendChild(time);

  const text = document.createElement("span");
  text.textContent = item.text || "";
  article.appendChild(text);
  return article;
}

function renderLane(kind, source, container, statusElement) {
  const sourceState = getMeetingState()?.sources?.[source];
  const recentItems = [...(sourceState?.recentItems || [])].reverse();
  const latestText = typeof sourceState?.latestText === "string" ? sourceState.latestText.trim() : "";
  const latestFinalizedText = recentItems.length ? (recentItems[recentItems.length - 1].text || "").trim() : "";
  const previewText = latestText && latestText !== latestFinalizedText ? latestText : "";

  statusElement.textContent = sourceState?.statusLabel || "等待输入";
  container.replaceChildren();

  if (!recentItems.length && !previewText) {
    const empty = document.createElement("div");
    empty.className = "caption-empty";
    empty.textContent = sourceState?.error || sourceState?.note || "这一侧的实时字幕会显示在这里。";
    container.appendChild(empty);
    return;
  }

  recentItems.forEach((item) => {
    container.appendChild(createCaptionItem(item));
  });

  if (previewText) {
    container.appendChild(createCaptionItem({
      time: sourceState?.lastUpdatedAt || "实时",
      text: previewText
    }, "caption-live"));
  }

  if (state.autoScroll[kind]) {
    container.scrollTop = container.scrollHeight;
  }
}

function render() {
  const meeting = getMeetingState();
  elements.captionsStatus.textContent = meeting?.statusLabel || "等待开始";
  elements.captionsMeta.textContent = meeting?.error
    || meeting?.note
    || "系统音频和麦克风会分开实时显示。";
  elements.captionsUpdatedAt.textContent = meeting?.lastUpdatedAt
    ? `最近概要更新时间 ${meeting.lastUpdatedAt}`
    : "实时更新中";

  renderLane("others", "system", elements.othersFeed, elements.othersStatus);
  renderLane("self", "microphone", elements.selfFeed, elements.selfStatus);
}

function bindEvents() {
  elements.othersFeed.addEventListener("scroll", () => {
    updateAutoScroll("others", elements.othersFeed);
  });
  elements.selfFeed.addEventListener("scroll", () => {
    updateAutoScroll("self", elements.selfFeed);
  });

  elements.showSummaryButton.addEventListener("click", () => {
    api.showMeetingSummaryOverlay();
  });

  elements.closeButton.addEventListener("click", () => {
    api.hideMeetingCaptionsOverlay();
  });

  elements.resizeHandle.addEventListener("mousedown", startResize);
}

async function init() {
  state.snapshot = await api.bootstrap();
  render();
  api.onSnapshot((snapshot) => {
    state.snapshot = snapshot;
    render();
  });
  bindEvents();
}

init();
