const api = window.pixelOrb;

const elements = {
  avatar: document.getElementById("chatAvatar"),
  statusLabel: document.getElementById("statusLabel"),
  pinButton: document.getElementById("pinButton"),
  historyButton: document.getElementById("historyButton"),
  closeButton: document.getElementById("closeButton"),
  historyPanel: document.getElementById("historyPanel"),
  taskStatus: document.getElementById("taskStatus"),
  taskStatusLabel: document.getElementById("taskStatusLabel"),
  taskStatusMeta: document.getElementById("taskStatusMeta"),
  taskProgressBar: document.getElementById("taskProgressBar"),
  cancelTaskButton: document.getElementById("cancelTaskButton"),
  messageList: document.getElementById("messageList"),
  quickChips: document.getElementById("quickChips"),
  selectionButton: document.getElementById("selectionButton"),
  clipboardButton: document.getElementById("clipboardButton"),
  composerInput: document.getElementById("composerInput"),
  sendButton: document.getElementById("sendButton")
};

const avatarContext = elements.avatar.getContext("2d");
avatarContext.imageSmoothingEnabled = false;

const state = {
  snapshot: null,
  historyOpen: false
};

function applySnapshot(snapshot) {
  state.snapshot = snapshot;
  document.title = `Pixel Chat · ${snapshot.state.statusLabel}`;
  renderHeader();
  renderTaskStatus();
  renderMessages();
  renderQuickChips();
  renderHistory();
}

function renderHeader() {
  if (!state.snapshot) {
    return;
  }
  elements.statusLabel.textContent = state.snapshot.state.statusLabel;
  elements.pinButton.classList.toggle("active", Boolean(state.snapshot.state.chatPinned));
}

function renderTaskStatus() {
  if (!state.snapshot) {
    return;
  }
  const current = state.snapshot.state;
  const active = Boolean(current.currentTask);
  elements.taskStatus.classList.toggle("hidden", !active);
  if (!active) {
    return;
  }
  elements.taskStatusLabel.textContent = current.progressLabel || "正在执行";
  elements.taskStatusMeta.textContent = current.statusNote || current.statusLabel;
  elements.taskProgressBar.style.width = `${Math.round((current.progress || 0) * 100)}%`;
}

function renderMessages() {
  if (!state.snapshot) {
    return;
  }

  elements.messageList.replaceChildren();
  state.snapshot.state.chatHistory.forEach((message) => {
    const article = document.createElement("article");
    article.className = `message ${message.role}`;
    article.textContent = message.text;
    const meta = document.createElement("div");
    meta.className = "message-meta";
    meta.textContent = message.timestamp;
    article.appendChild(meta);
    elements.messageList.appendChild(article);
  });
  elements.messageList.scrollTop = elements.messageList.scrollHeight;
}

function renderQuickChips() {
  if (!state.snapshot) {
    return;
  }
  const chipActions = {
    "总结选中文本": () => api.runTask("summarize", "chat"),
    "解释这个错误": () => api.runTask("explain", "chat"),
    "帮我起草回复": () => api.sendChat("帮我起草一条简短回复"),
    "翻译为英文": () => api.runTask("translate", "chat")
  };

  elements.quickChips.replaceChildren();
  state.snapshot.state.quickChips.forEach((label) => {
    const chip = document.createElement("button");
    chip.type = "button";
    chip.className = "chip-button";
    chip.textContent = label;
    chip.addEventListener("click", () => {
      const action = chipActions[label];
      if (action) {
        action();
      } else {
        api.sendChat(label);
      }
    });
    elements.quickChips.appendChild(chip);
  });
}

function renderHistory() {
  if (!state.snapshot || !state.historyOpen) {
    elements.historyPanel.classList.add("hidden");
    return;
  }

  elements.historyPanel.replaceChildren();
  elements.historyPanel.classList.remove("hidden");
  state.snapshot.state.recentActions.forEach((item) => {
    const row = document.createElement("div");
    row.className = "history-item";
    row.innerHTML = `<strong>${item.title}</strong><span>${item.detail}</span><span>${item.time}</span>`;
    elements.historyPanel.appendChild(row);
  });
}

function autoresize() {
  elements.composerInput.style.height = "44px";
  elements.composerInput.style.height = `${Math.min(elements.composerInput.scrollHeight, 92)}px`;
}

function sendInput() {
  const value = elements.composerInput.value.trim();
  if (!value) {
    return;
  }
  api.sendChat(value);
  elements.composerInput.value = "";
  autoresize();
}

function drawCell(x, y, width, height, color) {
  avatarContext.fillStyle = color;
  avatarContext.fillRect(x * 2, y * 2, width * 2, height * 2);
}

function drawAvatar() {
  avatarContext.clearRect(0, 0, 48, 48);
  const current = state.snapshot?.state;
  if (!current) {
    return;
  }

  const accent = current.status === "thinking" || current.status === "needs-user" ? "#efbc77" : "#67d8cb";
  drawCell(8, 8, 8, 1, "#22313b");
  drawCell(7, 9, 10, 5, "#22313b");
  drawCell(8, 10, 8, 4, "#d8f1f5");
  drawCell(8, 9, 2, 2, accent);
  drawCell(14, 9, 2, 2, accent);
  drawCell(10, 11, 1, 2, "#13222c");
  drawCell(13, 11, 1, 2, "#13222c");
  drawCell(11, 13, 2, 1, current.agentPaused ? "#8ca3b6" : current.status === "error" ? "#ee9174" : "#355466");
  if (current.status === "running") {
    drawCell(6, 11, 1, 3, accent);
    drawCell(17, 11, 1, 3, accent);
  }
  if (current.agentPaused) {
    drawCell(9, 7, 7, 2, "#6c85a3");
    drawCell(15, 8, 2, 3, "#6c85a3");
  }
}

function bindEvents() {
  elements.pinButton.addEventListener("click", () => {
    api.updatePreferences({
      chatPinned: !(state.snapshot?.state.chatPinned)
    });
  });

  elements.historyButton.addEventListener("click", () => {
    state.historyOpen = !state.historyOpen;
    renderHistory();
    elements.historyButton.classList.toggle("active", state.historyOpen);
  });

  elements.closeButton.addEventListener("click", () => {
    api.closeChat();
  });

  elements.cancelTaskButton.addEventListener("click", () => {
    api.cancelTask();
  });

  elements.selectionButton.addEventListener("click", () => {
    api.sendChat("请基于当前选区帮我继续");
  });

  elements.clipboardButton.addEventListener("click", () => {
    api.runTask("clipboard", "chat");
  });

  elements.sendButton.addEventListener("click", sendInput);
  elements.composerInput.addEventListener("input", autoresize);
  elements.composerInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      sendInput();
    }
  });
}

async function init() {
  const snapshot = await api.bootstrap();
  applySnapshot(snapshot);
  drawAvatar();
  api.onSnapshot((next) => {
    applySnapshot(next);
    drawAvatar();
  });
  bindEvents();
  autoresize();
}

init();
