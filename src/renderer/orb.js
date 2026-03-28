const api = window.pixelOrb;

const elements = {
  body: document.body,
  shell: document.getElementById("orbShell"),
  ringLayer: document.getElementById("ringLayer"),
  tooltip: document.getElementById("taskTooltip"),
  tooltipTitle: document.getElementById("tooltipTitle"),
  tooltipDesc: document.getElementById("tooltipDesc"),
  tooltipAvailability: document.getElementById("tooltipAvailability"),
  tooltipHint: document.getElementById("tooltipHint"),
  microTip: document.getElementById("microTip"),
  dockHint: document.getElementById("dockHint"),
  library: document.getElementById("taskLibrary"),
  libraryList: document.getElementById("libraryList"),
  libraryClose: document.getElementById("libraryClose"),
  canvas: document.getElementById("spriteCanvas"),
  statusDot: document.getElementById("statusDot")
};

const canvasContext = elements.canvas.getContext("2d");
canvasContext.imageSmoothingEnabled = false;

const TASK_RING_RADIUS = 104;
const TASK_BALL_SIZE = 54;
const RING_INTERACTION_RADIUS = TASK_RING_RADIUS + TASK_BALL_SIZE;
const TOOLTIP_GAP = 18;
const TOOLTIP_MARGIN = 12;
const TOOLTIP_FALLBACK_HEIGHT = 104;

const state = {
  snapshot: null,
  hoverTaskId: null,
  ringItems: [],
  pointer: {
    x: 0,
    y: 0
  },
  pressOrigin: null,
  dragStarted: false,
  longPressTriggered: false,
  libraryOpen: false,
  hoverOpenTimer: null,
  hoverCloseTimer: null,
  longPressTimer: null,
  activityTick: 0
};

function clearTimer(name) {
  if (state[name]) {
    clearTimeout(state[name]);
    state[name] = null;
  }
}

function markActivity() {
  const now = Date.now();
  if (now - state.activityTick > 180) {
    api.markActivity();
    state.activityTick = now;
  }
}

function getLayout() {
  return state.snapshot?.layout || {
    anchorX: 48,
    anchorY: 48,
    collapsedSize: 78,
    mode: "collapsed",
    dockSide: "right"
  };
}

function getTasks() {
  return state.snapshot?.tasks || [];
}

function getOrbSize() {
  const collapsedSize = getLayout().collapsedSize || 96;
  return Math.max(64, collapsedSize - 18);
}

function isRingOpen() {
  return getLayout().mode === "ring";
}

function clampValue(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function getTaskById(taskId) {
  return getTasks().find((task) => task.id === taskId) || null;
}

function applySnapshot(snapshot) {
  state.snapshot = snapshot;
  document.title = `Pixel Orb · ${snapshot.state.statusLabel}`;
  renderLayout();
  renderTasks();
  renderLibrary();
  renderMicroTip();
  renderDockHint();
  renderStatusDot();
}

function renderLayout() {
  const layout = getLayout();
  const orbSize = getOrbSize();
  elements.body.style.setProperty("--anchor-x", `${layout.anchorX}px`);
  elements.body.style.setProperty("--anchor-y", `${layout.anchorY}px`);
  elements.body.style.setProperty("--orb-size", `${orbSize}px`);
  elements.body.style.setProperty("--progress", `${state.snapshot?.state.progress || 0}`);
  elements.body.classList.toggle("ring-open", isRingOpen());
  elements.body.classList.toggle("low-profile", Boolean(state.snapshot?.state.lowProfile));
  elements.body.classList.toggle("dnd", Boolean(state.snapshot?.state.dnd));
}

function renderStatusDot() {
  const current = state.snapshot?.state;
  if (!current) {
    return;
  }

  let color = "#67d8cb";
  if (current.agentPaused) {
    color = "#88a6bf";
  } else if (current.dnd) {
    color = "#7ca4a0";
  } else if (current.status === "running") {
    color = "#67d8cb";
  } else if (current.status === "thinking") {
    color = "#efbc77";
  } else if (current.status === "needs-user") {
    color = "#efbc77";
  } else if (current.status === "error") {
    color = "#ee9174";
  }

  elements.statusDot.style.background = color;
}

function renderMicroTip() {
  if (!state.snapshot) {
    return;
  }

  const current = state.snapshot.state;
  const text = current.currentTask
    ? `执行中：${current.progressLabel || current.statusNote}`
    : current.statusNote || current.statusLabel;
  elements.microTip.textContent = text;
  elements.microTip.classList.toggle("visible", isRingOpen() && Boolean(text));
}

function renderDockHint() {
  if (!state.snapshot) {
    return;
  }

  const preview = state.snapshot.layout.previewDockSide;
  let text = "";
  if (preview) {
    const labels = {
      left: "释放后停靠到左边缘",
      right: "释放后停靠到右边缘",
      top: "释放后停靠到上边缘",
      bottom: "释放后停靠到下边缘"
    };
    text = labels[preview] || "";
  } else if (state.snapshot.layout.peeked) {
    text = "低存在感：探出隐藏中";
  }
  elements.dockHint.textContent = text;
  elements.dockHint.classList.toggle("visible", Boolean(text));
}

function taskAngle(slot) {
  return -90 + slot * 45;
}

function taskPosition(slot) {
  const layout = getLayout();
  const radius = TASK_RING_RADIUS;
  const radians = (taskAngle(slot) * Math.PI) / 180;
  return {
    x: layout.anchorX + Math.cos(radians) * radius,
    y: layout.anchorY + Math.sin(radians) * radius
  };
}

function rectOverlapArea(a, b) {
  const width = Math.max(0, Math.min(a.right, b.right) - Math.max(a.left, b.left));
  const height = Math.max(0, Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top));
  return width * height;
}

function tooltipRect(x, y, width, height) {
  return {
    left: x,
    top: y,
    right: x + width,
    bottom: y + height
  };
}

function getTooltipCandidates(point, width, height) {
  const layout = getLayout();
  const anchor = {
    x: layout.anchorX,
    y: layout.anchorY
  };
  const dx = point.x - anchor.x;
  const dy = point.y - anchor.y;
  const orbSafeGap = Math.round(getOrbSize() / 2 + 34);
  const candidates = [];

  const pushCandidate = (x, y) => {
    const key = `${Math.round(x)}:${Math.round(y)}`;
    if (!candidates.some((item) => item.key === key)) {
      candidates.push({
        key,
        x,
        y
      });
    }
  };

  if (layout.dockSide === "right") {
    pushCandidate(anchor.x - width - orbSafeGap, point.y - height / 2);
  } else if (layout.dockSide === "left") {
    pushCandidate(anchor.x + orbSafeGap, point.y - height / 2);
  } else if (layout.dockSide === "top") {
    pushCandidate(point.x - width / 2, anchor.y + orbSafeGap);
  } else if (layout.dockSide === "bottom") {
    pushCandidate(point.x - width / 2, anchor.y - height - orbSafeGap);
  }

  pushCandidate(dx >= 0 ? point.x + TOOLTIP_GAP : point.x - width - TOOLTIP_GAP, point.y - height / 2);
  pushCandidate(point.x - width / 2, dy >= 0 ? point.y + TOOLTIP_GAP : point.y - height - TOOLTIP_GAP);
  pushCandidate(dx >= 0 ? point.x - width - TOOLTIP_GAP : point.x + TOOLTIP_GAP, point.y - height / 2);
  pushCandidate(point.x - width / 2, dy >= 0 ? point.y - height - TOOLTIP_GAP : point.y + TOOLTIP_GAP);

  return candidates;
}

function pickTooltipPosition(point, width, height) {
  const layout = getLayout();
  const minX = TOOLTIP_MARGIN;
  const minY = TOOLTIP_MARGIN;
  const maxX = Math.max(minX, window.innerWidth - width - TOOLTIP_MARGIN);
  const maxY = Math.max(minY, window.innerHeight - height - TOOLTIP_MARGIN);
  const orbSafeRadius = Math.round(getOrbSize() / 2 + 38);
  const orbRect = {
    left: layout.anchorX - orbSafeRadius,
    top: layout.anchorY - orbSafeRadius,
    right: layout.anchorX + orbSafeRadius,
    bottom: layout.anchorY + orbSafeRadius
  };

  let bestCandidate = null;

  getTooltipCandidates(point, width, height).forEach((candidate, index) => {
    const x = clampValue(candidate.x, minX, maxX);
    const y = clampValue(candidate.y, minY, maxY);
    const rect = tooltipRect(x, y, width, height);
    const overlap = rectOverlapArea(rect, orbRect);
    const distance = Math.abs(point.x - x) + Math.abs(point.y - y);
    const score = overlap * 1000 + distance + index;

    if (!bestCandidate || score < bestCandidate.score) {
      bestCandidate = { x, y, score, overlap };
    }
  });

  return bestCandidate || { x: minX, y: minY };
}

function showTooltip(task, point) {
  elements.tooltipTitle.textContent = task.title;
  elements.tooltipDesc.textContent = task.description;
  elements.tooltipAvailability.textContent = task.availability;
  elements.tooltipHint.textContent = task.hint;

  const width = elements.tooltip.offsetWidth || 220;
  const height = elements.tooltip.offsetHeight || TOOLTIP_FALLBACK_HEIGHT;
  const position = pickTooltipPosition(point, width, height);

  elements.tooltip.style.left = `${Math.round(position.x)}px`;
  elements.tooltip.style.top = `${Math.round(position.y)}px`;
  elements.tooltip.classList.remove("hidden");
}

function hideTooltip() {
  elements.tooltip.classList.add("hidden");
}

function renderTasks() {
  const tasks = getTasks();
  const previous = new Map(state.ringItems.map((item) => [item.task.id, item.element]));
  const nextItems = [];

  tasks.forEach((task) => {
    let button = previous.get(task.id);
    if (!button) {
      button = document.createElement("button");
      button.className = "task-ball";
      button.type = "button";
      button.addEventListener("mouseenter", () => {
        const currentTask = getTaskById(button.dataset.taskId);
        if (!currentTask) {
          return;
        }
        clearTimer("hoverCloseTimer");
        state.hoverTaskId = currentTask.id;
        const position = taskPosition(currentTask.slot);
        showTooltip(currentTask, position);
        markActivity();
      });
      button.addEventListener("mouseleave", () => {
        state.hoverTaskId = null;
        if (!state.libraryOpen) {
          hideTooltip();
        }
      });
      button.addEventListener("click", (event) => {
        const currentTask = getTaskById(button.dataset.taskId);
        if (!currentTask) {
          return;
        }
        event.stopPropagation();
        markActivity();
        if (currentTask.id === "more") {
          toggleLibrary();
          return;
        }
        state.libraryOpen = false;
        hideLibrary();
        api.runTask(currentTask.id, "orb");
        scheduleCollapse(120);
      });
      elements.ringLayer.appendChild(button);
    }

    const position = taskPosition(task.slot);
    button.dataset.taskId = task.id;
    button.textContent = task.shortLabel;
    button.style.left = `${position.x}px`;
    button.style.top = `${position.y}px`;
    button.classList.toggle("disabled", task.enabled === false);
    nextItems.push({ task, element: button });
    previous.delete(task.id);
  });

  previous.forEach((element) => element.remove());
  state.ringItems = nextItems;
}

function renderLibrary() {
  if (!state.snapshot) {
    return;
  }

  elements.libraryList.replaceChildren();
  state.snapshot.taskLibrary.forEach((task) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "library-item";
    button.innerHTML = `<strong>${task.title}</strong><span>${task.description}</span>`;
    button.addEventListener("click", () => {
      markActivity();
      state.libraryOpen = false;
      hideLibrary();
      api.runTask(task.id, "orb");
      scheduleCollapse(120);
    });
    elements.libraryList.appendChild(button);
  });
}

function positionLibrary() {
  const layout = getLayout();
  const width = 248;
  let x = layout.anchorX + 62;
  let y = layout.anchorY - 104;

  if (layout.dockSide === "right") {
    x = layout.anchorX - width - 62;
  }
  if (layout.dockSide === "top") {
    y = layout.anchorY + 56;
  }
  if (layout.dockSide === "bottom") {
    y = layout.anchorY - 220;
  }

  x = Math.min(Math.max(x, 12), window.innerWidth - width - 12);
  y = Math.min(Math.max(y, 12), window.innerHeight - 220);

  elements.library.style.left = `${Math.round(x)}px`;
  elements.library.style.top = `${Math.round(y)}px`;
}

function showLibrary() {
  state.libraryOpen = true;
  positionLibrary();
  elements.library.classList.remove("hidden");
}

function hideLibrary() {
  elements.library.classList.add("hidden");
}

function toggleLibrary() {
  state.libraryOpen = !state.libraryOpen;
  if (state.libraryOpen) {
    showLibrary();
  } else {
    hideLibrary();
  }
}

function scheduleExpand() {
  clearTimer("hoverOpenTimer");
  state.hoverOpenTimer = setTimeout(() => {
    if (!isRingOpen()) {
      api.expandOrb();
    }
  }, state.snapshot?.state.theme?.tooltipDelay || 220);
}

function scheduleCollapse(delay = 320) {
  clearTimer("hoverCloseTimer");
  state.hoverCloseTimer = setTimeout(() => {
    if (state.libraryOpen) {
      return;
    }
    api.collapseOrb();
    hideTooltip();
  }, delay);
}

function pointerDistance(event) {
  if (!state.pressOrigin) {
    return 0;
  }
  const dx = event.screenX - state.pressOrigin.screenX;
  const dy = event.screenY - state.pressOrigin.screenY;
  return Math.hypot(dx, dy);
}

function handlePrimaryClick() {
  hideLibrary();
  state.libraryOpen = false;
  hideTooltip();
  if (isRingOpen()) {
    api.collapseOrb();
    return;
  }
  api.expandOrb();
}

function bindEvents() {
  elements.shell.addEventListener("mouseenter", () => {
    markActivity();
    if (!state.dragStarted) {
      scheduleExpand();
    }
  });

  elements.shell.addEventListener("mouseleave", () => {
    clearTimer("hoverOpenTimer");
    if (isRingOpen()) {
      scheduleCollapse(280);
    }
  });

  elements.shell.addEventListener("contextmenu", (event) => {
    event.preventDefault();
    markActivity();
    api.showContextMenu({
      x: event.clientX,
      y: event.clientY
    });
  });

  elements.shell.addEventListener("pointerdown", (event) => {
    if (event.button !== 0) {
      return;
    }
    markActivity();
    elements.shell.setPointerCapture(event.pointerId);
    state.pressOrigin = {
      screenX: event.screenX,
      screenY: event.screenY
    };
    state.dragStarted = false;
    state.longPressTriggered = false;
    clearTimer("longPressTimer");
    state.longPressTimer = setTimeout(() => {
      state.longPressTriggered = true;
      api.expandOrb();
    }, 350);
  });

  elements.shell.addEventListener("pointermove", (event) => {
    state.pointer.x = event.clientX;
    state.pointer.y = event.clientY;
    if (!state.pressOrigin) {
      return;
    }

    if (!state.dragStarted && pointerDistance(event) > 6) {
      state.dragStarted = true;
      clearTimer("longPressTimer");
      api.collapseOrb();
      api.dragStart({
        screenX: state.pressOrigin.screenX,
        screenY: state.pressOrigin.screenY
      });
    }

    if (state.dragStarted) {
      api.dragMove({
        screenX: event.screenX,
        screenY: event.screenY
      });
    }
  });

  elements.shell.addEventListener("pointerup", (event) => {
    clearTimer("longPressTimer");
    if (state.dragStarted) {
      api.dragEnd();
    } else if (!state.longPressTriggered) {
      handlePrimaryClick();
    }
    state.pressOrigin = null;
    state.dragStarted = false;
    state.longPressTriggered = false;
    elements.shell.releasePointerCapture(event.pointerId);
  });

  elements.shell.addEventListener("pointercancel", () => {
    clearTimer("longPressTimer");
    if (state.dragStarted) {
      api.dragEnd();
    }
    state.pressOrigin = null;
    state.dragStarted = false;
    state.longPressTriggered = false;
  });

  elements.ringLayer.addEventListener("mouseenter", () => {
    clearTimer("hoverCloseTimer");
  });

  elements.ringLayer.addEventListener("mouseleave", () => {
    if (isRingOpen()) {
      scheduleCollapse(300);
    }
  });

  elements.libraryClose.addEventListener("click", () => {
    state.libraryOpen = false;
    hideLibrary();
  });

  window.addEventListener("mousemove", (event) => {
    state.pointer.x = event.clientX;
    state.pointer.y = event.clientY;
    if (!isRingOpen()) {
      return;
    }
    const layout = getLayout();
    const dx = event.clientX - layout.anchorX;
    const dy = event.clientY - layout.anchorY;
    const distance = Math.hypot(dx, dy);
    if (distance <= RING_INTERACTION_RADIUS) {
      clearTimer("hoverCloseTimer");
    } else if (!state.libraryOpen) {
      scheduleCollapse(300);
    }
  });

  window.addEventListener("mousedown", (event) => {
    if (elements.library.contains(event.target) || elements.shell.contains(event.target)) {
      return;
    }
    if (state.libraryOpen) {
      state.libraryOpen = false;
      hideLibrary();
    }
  });

  window.addEventListener("resize", () => {
    if (state.libraryOpen) {
      positionLibrary();
    }
  });
}

function drawCell(x, y, width, height, color, offsetY = 0) {
  canvasContext.fillStyle = color;
  canvasContext.fillRect(x * 3, (y + offsetY) * 3, width * 3, height * 3);
}

function spritePalette() {
  return {
    outline: "#20303a",
    face: "#d8f1f5",
    accent: state.snapshot?.state.status === "thinking" || state.snapshot?.state.status === "needs-user" ? "#efbc77" : "#67d8cb",
    trim: "#527188",
    mouth: "#355466",
    error: "#ee9174",
    muted: "#86a1b3",
    cap: "#6c85a3"
  };
}

function currentMood() {
  const current = state.snapshot?.state;
  if (!current) {
    return "idle";
  }
  if (current.agentPaused) {
    return "paused";
  }
  if (current.status === "error") {
    return "error";
  }
  if (current.status === "running") {
    return "running";
  }
  if (current.status === "thinking") {
    return "thinking";
  }
  if (current.status === "needs-user") {
    return "needs-user";
  }
  return current.dnd ? "dnd" : "idle";
}

function drawSprite(time) {
  canvasContext.clearRect(0, 0, 72, 72);
  const palette = spritePalette();
  const mood = currentMood();
  const layout = getLayout();
  const pointerDeltaX = state.pointer.x - layout.anchorX;
  const eyeShift = pointerDeltaX > 12 ? 1 : pointerDeltaX < -12 ? -1 : 0;
  const blink = time % 3800 < 150;
  const floatSpeed = state.snapshot?.state.lowProfile ? 900 : 520;
  const bob = Math.round(Math.sin(time / floatSpeed) * 1);

  drawCell(7, 18, 10, 2, "rgba(9,16,24,0.16)", bob);
  drawCell(8, 6, 8, 1, palette.outline, bob);
  drawCell(7, 7, 10, 1, palette.outline, bob);
  drawCell(6, 8, 12, 7, palette.outline, bob);
  drawCell(7, 9, 10, 5, palette.face, bob);
  drawCell(8, 14, 8, 2, palette.trim, bob);
  drawCell(9, 16, 6, 2, palette.outline, bob);
  drawCell(10, 17, 4, 1, palette.trim, bob);
  drawCell(7, 7, 2, 2, palette.accent, bob);
  drawCell(15, 7, 2, 2, palette.accent, bob);

  if (blink) {
    drawCell(10 + eyeShift, 10, 2, 1, palette.outline, bob);
    drawCell(13 + eyeShift, 10, 2, 1, palette.outline, bob);
  } else {
    drawCell(10 + eyeShift, 10, 1, 2, palette.outline, bob);
    drawCell(14 + eyeShift, 10, 1, 2, palette.outline, bob);
  }

  if (mood === "thinking") {
    drawCell(11, 13, 3, 1, palette.mouth, bob);
    drawCell(17, 5, 2, 2, palette.accent, bob - 1);
    drawCell(19, 3, 1, 1, palette.accent, bob - 2);
    drawCell(21, 2, 1, 1, palette.accent, bob - 3);
  } else if (mood === "running") {
    drawCell(11, 13, 3, 1, palette.mouth, bob);
    drawCell(5, 9, 1, 4, palette.accent, bob);
    drawCell(18, 9, 1, 4, palette.accent, bob);
    drawCell(18, 12, 2, 1, palette.outline, bob);
  } else if (mood === "needs-user") {
    drawCell(11, 13, 3, 1, palette.mouth, bob);
    drawCell(18, 5, 1, 4, palette.outline, bob);
    drawCell(19, 5, 2, 2, palette.accent, bob);
  } else if (mood === "paused") {
    drawCell(9, 4, 7, 2, palette.cap, bob);
    drawCell(15, 5, 2, 4, palette.cap, bob);
    drawCell(12, 13, 2, 1, palette.mouth, bob);
  } else if (mood === "error") {
    drawCell(12, 13, 2, 1, palette.error, bob);
    drawCell(16, 8, 3, 1, "#f5d7bc", bob);
    drawCell(17, 7, 1, 3, "#e5af7f", bob);
  } else if (mood === "dnd") {
    drawCell(11, 13, 3, 1, palette.muted, bob);
    drawCell(18, 5, 2, 2, palette.muted, bob);
    drawCell(19, 4, 1, 3, "#0f1c28", bob);
  } else {
    drawCell(11, 13, 3, 1, palette.mouth, bob);
  }
}

function animationLoop(time) {
  if (state.snapshot) {
    drawSprite(time);
  }
  window.requestAnimationFrame(animationLoop);
}

async function init() {
  const initial = await api.bootstrap();
  applySnapshot(initial);
  api.onSnapshot((snapshot) => {
    applySnapshot(snapshot);
    if (!state.libraryOpen) {
      hideLibrary();
    } else {
      positionLibrary();
    }
  });
  bindEvents();
  window.requestAnimationFrame(animationLoop);
}

init();
