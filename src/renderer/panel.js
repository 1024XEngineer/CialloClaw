const api = window.pixelOrb;

const elements = {
  tabButtons: Array.from(document.querySelectorAll(".tab-button")),
  panels: Array.from(document.querySelectorAll(".tab-panel")),
  openChatButton: document.getElementById("openChatButton"),
  closePanelButton: document.getElementById("closePanelButton"),
  pausedToggle: document.getElementById("pausedToggle"),
  dndToggle: document.getElementById("dndToggle"),
  lowProfileToggle: document.getElementById("lowProfileToggle"),
  autoHideRadios: Array.from(document.querySelectorAll('input[name="autoHide"]')),
  sizeRadios: Array.from(document.querySelectorAll('input[name="size"]')),
  motionRange: document.getElementById("motionRange"),
  motionValue: document.getElementById("motionValue"),
  saturationRange: document.getElementById("saturationRange"),
  saturationValue: document.getElementById("saturationValue"),
  recentActions: document.getElementById("recentActions"),
  taskSlots: document.getElementById("taskSlots"),
  libraryPreview: document.getElementById("libraryPreview"),
  selectionToggle: document.getElementById("selectionToggle"),
  clipboardToggle: document.getElementById("clipboardToggle"),
  activeAppToggle: document.getElementById("activeAppToggle"),
  pluginToggle: document.getElementById("pluginToggle"),
  contextSummary: document.getElementById("contextSummary"),
  runSummaryButton: document.getElementById("runSummaryButton"),
  runExplainButton: document.getElementById("runExplainButton"),
  statusSummary: document.getElementById("statusSummary"),
  advancedLogs: document.getElementById("advancedLogs")
};

const state = {
  snapshot: null,
  activeTab: "general",
  syncing: false
};

function switchTab(name) {
  state.activeTab = name;
  elements.tabButtons.forEach((button) => {
    button.classList.toggle("active", button.dataset.tab === name);
  });
  elements.panels.forEach((panel) => {
    panel.classList.toggle("active", panel.dataset.panel === name);
  });
}

function renderLogs(target, items) {
  target.replaceChildren();
  items.forEach((item) => {
    const row = document.createElement("div");
    row.className = "log-item";
    row.innerHTML = `<strong>${item.title}</strong><span>${item.detail}</span><span>${item.time}</span>`;
    target.appendChild(row);
  });
}

function renderTaskRows(target, tasks, compact = false) {
  target.replaceChildren();
  tasks.forEach((task, index) => {
    const row = document.createElement("div");
    row.className = "slot-row";
    const availability = compact ? task.availability : `${task.availability} · ${task.enabled === false ? "不可用" : "可运行"}`;
    row.innerHTML = `
      <div class="slot-index">${compact ? "库" : index + 1}</div>
      <div class="slot-copy">
        <strong>${task.title}</strong>
        <span>${availability}</span>
      </div>
      <button class="ghost-button" type="button">${compact ? "运行" : "试跑"}</button>
    `;
    row.querySelector("button").addEventListener("click", () => {
      api.runTask(task.id === "more" ? "search" : task.id, "panel");
    });
    target.appendChild(row);
  });
}

function syncInputs() {
  if (!state.snapshot) {
    return;
  }

  state.syncing = true;
  const current = state.snapshot.state;
  elements.pausedToggle.checked = Boolean(current.agentPaused);
  elements.dndToggle.checked = Boolean(current.dnd);
  elements.lowProfileToggle.checked = Boolean(current.lowProfile);
  elements.autoHideRadios.forEach((radio) => {
    radio.checked = radio.value === current.autoHideMode;
  });
  elements.sizeRadios.forEach((radio) => {
    radio.checked = radio.value === current.size;
  });
  elements.motionRange.value = current.theme.motionScale;
  elements.motionValue.textContent = `${Number(current.theme.motionScale).toFixed(1)}x`;
  elements.saturationRange.value = current.theme.saturation;
  elements.saturationValue.textContent = `${Number(current.theme.saturation).toFixed(2)}x`;
  elements.selectionToggle.checked = Boolean(current.abilityToggles.selection);
  elements.clipboardToggle.checked = Boolean(current.abilityToggles.clipboard);
  elements.activeAppToggle.checked = Boolean(current.abilityToggles.activeApp);
  elements.pluginToggle.checked = Boolean(current.abilityToggles.plugins);
  state.syncing = false;
}

function renderSummary() {
  if (!state.snapshot) {
    return;
  }
  const current = state.snapshot.state;
  elements.contextSummary.textContent = `活动应用：${current.context.activeApp}。选区：${current.context.currentSelection}。剪贴板：${current.context.clipboardText}。`;
  elements.statusSummary.textContent = `当前为“${current.statusLabel}”，自动隐藏模式为“${current.autoHideMode}”，尺寸为“${current.size}”。`;
}

function applySnapshot(snapshot) {
  state.snapshot = snapshot;
  document.title = `Pixel Panel · ${snapshot.state.statusLabel}`;
  syncInputs();
  renderLogs(elements.recentActions, snapshot.state.recentActions);
  renderLogs(elements.advancedLogs, snapshot.state.recentActions);
  renderTaskRows(elements.taskSlots, snapshot.tasks.filter((task) => task.id !== "more"));
  renderTaskRows(elements.libraryPreview, snapshot.taskLibrary, true);
  renderSummary();
}

function maybeUpdate(patch) {
  if (state.syncing) {
    return;
  }
  api.updatePreferences(patch);
}

function bindEvents() {
  elements.tabButtons.forEach((button) => {
    button.addEventListener("click", () => switchTab(button.dataset.tab));
  });

  elements.openChatButton.addEventListener("click", () => api.openChat({ focus: true }));
  elements.closePanelButton.addEventListener("click", () => api.closePanel());
  elements.runSummaryButton.addEventListener("click", () => api.runTask("summarize", "panel"));
  elements.runExplainButton.addEventListener("click", () => api.runTask("explain", "panel"));

  elements.pausedToggle.addEventListener("change", () => maybeUpdate({ agentPaused: elements.pausedToggle.checked }));
  elements.dndToggle.addEventListener("change", () => maybeUpdate({ dnd: elements.dndToggle.checked }));
  elements.lowProfileToggle.addEventListener("change", () => maybeUpdate({ lowProfile: elements.lowProfileToggle.checked }));

  elements.autoHideRadios.forEach((radio) => {
    radio.addEventListener("change", () => {
      if (radio.checked) {
        maybeUpdate({ autoHideMode: radio.value });
      }
    });
  });

  elements.sizeRadios.forEach((radio) => {
    radio.addEventListener("change", () => {
      if (radio.checked) {
        maybeUpdate({ size: radio.value });
      }
    });
  });

  elements.motionRange.addEventListener("input", () => {
    elements.motionValue.textContent = `${Number(elements.motionRange.value).toFixed(1)}x`;
  });
  elements.motionRange.addEventListener("change", () => {
    maybeUpdate({ theme: { motionScale: Number(elements.motionRange.value) } });
  });

  elements.saturationRange.addEventListener("input", () => {
    elements.saturationValue.textContent = `${Number(elements.saturationRange.value).toFixed(2)}x`;
  });
  elements.saturationRange.addEventListener("change", () => {
    maybeUpdate({ theme: { saturation: Number(elements.saturationRange.value) } });
  });

  const abilityPatch = () => ({
    abilityToggles: {
      selection: elements.selectionToggle.checked,
      clipboard: elements.clipboardToggle.checked,
      activeApp: elements.activeAppToggle.checked,
      plugins: elements.pluginToggle.checked
    },
    context: {
      selectedTextAvailable: elements.selectionToggle.checked,
      clipboardReady: elements.clipboardToggle.checked
    }
  });

  elements.selectionToggle.addEventListener("change", () => maybeUpdate(abilityPatch()));
  elements.clipboardToggle.addEventListener("change", () => maybeUpdate(abilityPatch()));
  elements.activeAppToggle.addEventListener("change", () => maybeUpdate(abilityPatch()));
  elements.pluginToggle.addEventListener("change", () => maybeUpdate(abilityPatch()));
}

async function init() {
  const snapshot = await api.bootstrap();
  applySnapshot(snapshot);
  switchTab(state.activeTab);
  api.onSnapshot((next) => applySnapshot(next));
  bindEvents();
}

init();
