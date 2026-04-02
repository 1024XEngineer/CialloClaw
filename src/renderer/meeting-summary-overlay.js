const api = window.pixelOrb;

const elements = {
  summaryStatus: document.getElementById("summaryStatus"),
  summaryMeta: document.getElementById("summaryMeta"),
  summaryEmptyState: document.getElementById("summaryEmptyState"),
  othersSummary: document.getElementById("othersSummary"),
  selfSummary: document.getElementById("selfSummary"),
  summaryUpdatedAt: document.getElementById("summaryUpdatedAt"),
  summaryWindowHint: document.getElementById("summaryWindowHint"),
  showCaptionsButton: document.getElementById("showCaptionsButton"),
  closeButton: document.getElementById("closeButton"),
  resizeHandle: document.getElementById("resizeHandle")
};

const state = {
  snapshot: null,
  resize: null,
  pendingResizeFrame: 0
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

  const width = Math.max(320, state.resize.width + (event.screenX - state.resize.screenX));
  const height = Math.max(220, state.resize.height + (event.screenY - state.resize.screenY));
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

function render() {
  const meeting = getMeetingState();
  const summary = meeting?.latestSummary || {};
  const othersText = typeof summary.othersText === "string" && summary.othersText.trim()
    ? summary.othersText.trim()
    : "本轮未识别到有效内容";
  const selfText = typeof summary.selfText === "string" && summary.selfText.trim()
    ? summary.selfText.trim()
    : "本轮未识别到有效内容";
  const isEmpty = Boolean(summary.isEmpty) || (othersText === "本轮未识别到有效内容" && selfText === "本轮未识别到有效内容");

  elements.summaryStatus.textContent = meeting?.statusLabel || "等待开始";
  elements.summaryMeta.textContent = meeting?.error
    || summary.note
    || meeting?.note
    || "会议总结启动后，这里会每隔两分钟更新一次。";
  elements.summaryEmptyState.classList.toggle("hidden", !isEmpty);
  elements.summaryEmptyState.textContent = summary.note || "最近两分钟暂无可总结内容";
  elements.othersSummary.textContent = othersText;
  elements.selfSummary.textContent = selfText;
  elements.summaryUpdatedAt.textContent = meeting?.lastUpdatedAt
    ? `更新于 ${meeting.lastUpdatedAt}`
    : "等待第一轮概要";
  elements.summaryWindowHint.textContent = `最近 ${summary.windowSeconds || 120} 秒`;
}

function bindEvents() {
  elements.showCaptionsButton.addEventListener("click", () => {
    api.showMeetingCaptionsOverlay();
  });

  elements.closeButton.addEventListener("click", () => {
    api.hideMeetingSummaryOverlay();
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
