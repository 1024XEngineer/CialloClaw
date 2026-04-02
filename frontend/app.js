import { createInitialState, createStore } from './state.js';
import { mockMaterials, buildMockResult, buildQuickReply, guessIntent } from './data.js';
import { runtimeCall } from './utils.js';
import { renderDesktop } from './views/desktop.js';
import { renderQuickChat } from './views/quickChat.js';
import { renderIntentPanel } from './views/intentPanel.js';
import { renderResultPopup } from './views/resultPopup.js';
import { renderWorkspace } from './views/workspace.js';
import { renderDevPanel } from './views/devPanel.js';

const store = createStore(createInitialState());
const root = document.querySelector('#app');
let lastWindowFrame = '';

const dataView = {
  get materialsForDrag() {
    return mockMaterials;
  }
};

function clamp(value, min, max) {
  return Math.max(min, Math.min(value, max));
}

function getViewportSize() {
  const width = window.screen?.availWidth || window.innerWidth || 1280;
  const height = window.screen?.availHeight || window.innerHeight || 800;
  return { width, height };
}

function syncWindowForState(state) {
  if (!window.runtime) return;

  const viewport = getViewportSize();
  const margin = 24;
  let width = 260;
  let height = 260;
  let center = false;

  if (state.ui.workspace.open) {
    width = 1480;
    height = 940;
    center = true;
  } else if (state.ui.confirm.open) {
    width = 820;
    height = 640;
  } else if (state.ui.result.open) {
    width = 480;
    height = 380;
  } else if (state.ui.quickChat.open) {
    width = 440;
    height = 320;
  }

  width = clamp(width, 220, Math.max(220, viewport.width - margin * 2));
  height = clamp(height, 220, Math.max(220, viewport.height - margin * 2));

  const x = center ? Math.max(0, Math.round((viewport.width - width) / 2)) : Math.max(0, viewport.width - width - margin);
  const y = center ? Math.max(0, Math.round((viewport.height - height) / 2)) : Math.max(0, viewport.height - height - margin);
  const frameKey = `${width}x${height}@${x},${y}`;

  if (frameKey === lastWindowFrame) return;
  lastWindowFrame = frameKey;

  runtimeCall('WindowSetAlwaysOnTop', true);
  runtimeCall('WindowSetSize', width, height);
  runtimeCall('WindowSetPosition', x, y);
}

function renderApp() {
  const state = store.getState();
  document.body.dataset.mode = state.ui.workspace.open ? 'workspace' : state.ui.confirm.open ? 'confirm' : state.ui.result.open ? 'result' : 'desktop';
  root.innerHTML = `
    <div class="app-shell">
      ${renderDesktop(state)}
      ${renderDevPanel(state)}
      ${renderQuickChat(state)}
      ${renderIntentPanel(state, dataView)}
      ${renderResultPopup(state)}
      ${renderWorkspace(state, {
        conversations: state.data.conversations,
        todos: state.data.todos,
        settings: state.data.settings,
        statusSnapshots: state.data.statusSnapshots,
        memoryNotes: state.data.memoryNotes,
        materialsForDrag: mockMaterials
      })}
    </div>
  `;
}

store.subscribe((state) => {
  renderApp();
  syncWindowForState(state);
});

function findMaterialById(id) {
  return mockMaterials.find((item) => item.id === id);
}

function currentConfirmMaterials(state) {
  return state.ui.confirm.materials.length ? state.ui.confirm.materials : [mockMaterials[0]];
}

function resetToBallBase() {
  store.update((draft) => {
    draft.ui.ball.mode = 'default';
    draft.ui.ball.hovered = false;
    draft.ui.ball.contextMenuOpen = false;
    draft.ui.quickChat.open = false;
    draft.ui.confirm.open = false;
    draft.ui.confirm.sourceType = 'selection';
    draft.ui.confirm.materials = [];
    draft.ui.confirm.customIntent = '';
    draft.ui.confirm.guessedIntent = '提炼重点';
    draft.ui.result.open = false;
    draft.ui.result.payload = null;
    draft.ui.workspace.open = false;
    draft.ui.workspace.menuOpen = false;
    draft.ui.dev.open = false;
    draft.data.status = 'idle';
  });
}

function openResultPanel(payload) {
  store.update((draft) => {
    draft.ui.quickChat.open = false;
    draft.ui.confirm.open = false;
    draft.ui.workspace.open = false;
    draft.ui.workspace.menuOpen = false;
    draft.ui.ball.mode = 'default';
    draft.ui.ball.hovered = false;
    draft.ui.result.open = true;
    draft.ui.result.payload = payload;
    draft.ui.dev.open = false;
  });
}

function simulateHoverPanel() {
  store.update((draft) => {
    draft.ui.quickChat.open = true;
    draft.ui.confirm.open = false;
    draft.ui.result.open = false;
    draft.ui.workspace.open = false;
    draft.ui.ball.mode = 'default';
    draft.ui.ball.hovered = true;
    draft.ui.dev.open = true;
  });
}

function simulateConfirmPanel() {
  ensureCooperativeState('click', ['material-selection', 'material-web']);
  store.update((draft) => {
    draft.ui.dev.open = true;
  });
}

function simulateResultPanel() {
  openResultPanel(buildMockResult('提炼重点', [findMaterialById('material-selection'), findMaterialById('material-web')].filter(Boolean), '提炼重点'));
  store.update((draft) => {
    draft.data.status = 'working';
    draft.ui.dev.open = true;
  });
}

function simulateWorkspacePanel() {
  const state = store.getState();
  if (!state.ui.result.payload) {
    store.update((draft) => {
      draft.ui.result.payload = buildMockResult('生成草稿', [findMaterialById('material-clipboard')].filter(Boolean), '生成草稿');
      draft.ui.result.open = false;
    });
  }
  attachResultToConversation();
  openWorkspace();
  store.update((draft) => {
    draft.ui.workspace.mode = 'conversation';
    draft.ui.workspace.activeConversationId = draft.data.conversations[0]?.id || draft.ui.workspace.activeConversationId;
    draft.ui.dev.open = true;
  });
}

function toggleDevPanel() {
  store.update((draft) => {
    draft.ui.dev.open = !draft.ui.dev.open;
  });
}

function openQuickChat() {
  const state = store.getState();
  if (state.ui.workspace.open || state.ui.confirm.open || state.ui.result.open) return;
  store.update((draft) => {
    draft.ui.quickChat.open = true;
    draft.ui.ball.hovered = true;
    draft.ui.dev.open = false;
  });
  requestAnimationFrame(() => {
    const input = document.querySelector('[data-action="quick-chat-input"]');
    input?.focus();
  });
}

function closeQuickChat() {
  store.update((draft) => {
    draft.ui.quickChat.open = false;
    draft.ui.ball.hovered = false;
  });
}

function ensureCooperativeState(sourceType, materialIds = []) {
  const items = materialIds.map(findMaterialById).filter(Boolean);
  const guessedIntent = guessIntent(items.length ? items : [mockMaterials[0]]);
  store.update((draft) => {
    draft.ui.ball.mode = sourceType === 'drag' ? 'plus' : 'alert';
    draft.ui.ball.hovered = false;
    draft.ui.quickChat.open = false;
    draft.ui.confirm.open = sourceType === 'drag' || sourceType === 'click';
    draft.ui.confirm.materials = items.length ? items : [mockMaterials[0]];
    draft.ui.confirm.guessedIntent = guessedIntent;
    draft.ui.confirm.customIntent = '';
    draft.ui.confirm.sourceType = sourceType;
    draft.ui.result.open = false;
    draft.ui.workspace.open = false;
    draft.ui.workspace.menuOpen = false;
    draft.ui.dev.open = false;
  });
}

function removeMaterial(materialId) {
  store.update((draft) => {
    draft.ui.confirm.materials = draft.ui.confirm.materials.filter((item) => item.id !== materialId);
    if (!draft.ui.confirm.materials.length) {
      draft.ui.confirm.materials = [mockMaterials[0]];
    }
    draft.ui.confirm.guessedIntent = guessIntent(draft.ui.confirm.materials);
  });
}

function addMaterial(materialId) {
  const item = findMaterialById(materialId);
  if (!item) return;
  store.update((draft) => {
    const exists = draft.ui.confirm.materials.some((current) => current.id === item.id);
    if (!exists) {
      draft.ui.confirm.materials.push(item);
    }
    draft.ui.confirm.guessedIntent = guessIntent(draft.ui.confirm.materials);
  });
}

function confirmIntent() {
  const state = store.getState();
  const materials = currentConfirmMaterials(state);
  const payload = buildMockResult(state.ui.confirm.guessedIntent, materials, state.ui.confirm.customIntent);
  openResultPanel(payload);
  store.update((draft) => {
    draft.data.status = 'working';
    draft.data.recentActions = [
      `确认了：${payload.intentLabel}`,
      `来源：${materials.map((item) => item.label).join(' / ')}`,
      '窗口3 已打开'
    ];
  });
}

function openWorkspace() {
  store.update((draft) => {
    draft.ui.workspace.open = true;
    draft.ui.workspace.mode = 'conversation';
    draft.ui.workspace.menuOpen = false;
    draft.ui.quickChat.open = false;
    draft.ui.confirm.open = false;
    draft.ui.result.open = false;
    draft.ui.ball.mode = 'default';
    draft.ui.ball.hovered = false;
    draft.ui.dev.open = false;
  });
}

function attachResultToConversation() {
  const state = store.getState();
  if (!state.ui.result.payload) return;
  store.update((draft) => {
    const active = draft.data.conversations.find((conv) => conv.id === draft.ui.workspace.activeConversationId);
    if (active) {
      active.messages.push({
        role: 'result',
        kind: 'result',
        title: '窗口3 接入结果',
        body: state.ui.result.payload.summary,
        time: '刚刚'
      });
      active.summary = state.ui.result.payload.summary;
      active.time = '刚刚';
    }
  });
}

function sendQuickChat() {
  const input = document.querySelector('[data-action="quick-chat-input"]');
  const text = input?.value.trim() || '';
  if (!text) return;
  const reply = buildQuickReply(text);
  store.update((draft) => {
    draft.ui.quickChat.user = text;
    draft.ui.quickChat.agent = reply;
    draft.ui.quickChat.lastIntent = guessIntent([mockMaterials[0]]);
    draft.ui.dev.open = false;
  });
  if (input) input.value = '';
}

function switchConversation(id) {
  store.update((draft) => {
    draft.ui.workspace.activeConversationId = id;
  });
}

function sendWorkspaceMessage() {
  const input = document.querySelector('[data-action="workspace-input"]');
  const text = input?.value.trim() || '';
  if (!text) return;
  store.update((draft) => {
    const active = draft.data.conversations.find((conv) => conv.id === draft.ui.workspace.activeConversationId);
    if (active) {
      active.messages.push({ role: 'user', kind: 'user', title: '你', body: text, time: '刚刚' });
      active.messages.push({ role: 'assistant', kind: 'assistant', title: 'Agent', body: buildQuickReply(text), time: '刚刚' });
      active.summary = text.slice(0, 32);
      active.time = '刚刚';
    }
  });
  if (input) input.value = '';
}

function addTodoFromInput() {
  const input = document.querySelector('[data-action="todo-input"]');
  const title = input?.value.trim();
  if (!title) return;
  store.update((draft) => {
    draft.data.todos.unshift({
      id: `todo-${Date.now()}`,
      title,
      bucket: 'today',
      status: '进行中',
      source: '用户手动新增',
      agentGenerated: false
    });
  });
  if (input) input.value = '';
}

function toggleTodo(id) {
  store.update((draft) => {
    const item = draft.data.todos.find((todo) => todo.id === id);
    if (!item) return;
    item.bucket = item.bucket === 'done' ? 'today' : 'done';
    item.status = item.bucket === 'done' ? '已完成' : '进行中';
  });
}

function deleteTodo(id) {
  store.update((draft) => {
    draft.data.todos = draft.data.todos.filter((todo) => todo.id !== id);
  });
}

function toggleSetting(setting) {
  store.update((draft) => {
    draft.data.settings[setting] = !draft.data.settings[setting];
  });
}

function appendResultToTodo() {
  const state = store.getState();
  if (!state.ui.result.payload) return;
  store.update((draft) => {
    draft.data.todos.unshift({
      id: `todo-${Date.now()}`,
      title: state.ui.result.payload.title,
      bucket: 'today',
      status: '进行中',
      source: '窗口3 / 结果窗',
      agentGenerated: true
    });
  });
}

function closeWorkspace() {
  resetToBallBase();
}

function handleBallPrimaryClick() {
  const state = store.getState();
  if (state.ui.ball.mode === 'alert') {
    ensureCooperativeState('click');
    return;
  }
  if (state.ui.ball.mode === 'plus') {
    ensureCooperativeState('drag');
    return;
  }
}

function handleDoubleClickBall() {
  openWorkspace();
}

function handleContextMenuBall(event) {
  event.preventDefault();
  store.update((draft) => {
    draft.ui.ball.contextMenuOpen = true;
    draft.ui.workspace.menuOpen = false;
  });
}

function closeMenus() {
  store.update((draft) => {
    draft.ui.ball.contextMenuOpen = false;
    draft.ui.workspace.menuOpen = false;
  });
}

function dragMaterialFromEvent(event) {
  const id = event.target?.closest('[data-drag-kind]')?.dataset.dragKind;
  if (!id) return null;
  event.dataTransfer.effectAllowed = 'copy';
  event.dataTransfer.setData('text/plain', id);
  return id;
}

function handleDragOver(event) {
  const dropZone = event.target.closest('[data-drop-zone]');
  if (!dropZone) return;
  event.preventDefault();
  event.dataTransfer.dropEffect = 'copy';
  store.update((draft) => {
    draft.ui.ball.mode = 'plus';
  });
}

function handleDrop(event) {
  const dropZone = event.target.closest('[data-drop-zone]');
  if (!dropZone) return;
  event.preventDefault();
  const id = event.dataTransfer.getData('text/plain');
  if (dropZone.dataset.dropZone === 'quick-chat') return;
  if (dropZone.dataset.dropZone === 'confirm') {
    if (id) addMaterial(id);
    store.update((draft) => {
      draft.ui.ball.mode = 'plus';
      draft.ui.confirm.open = true;
      draft.ui.confirm.sourceType = 'drag';
      draft.ui.dev.open = false;
    });
    return;
  }
  if (dropZone.dataset.ballZone === 'true' || dropZone.closest('[data-ball-zone="true"]')) {
    if (id) {
      ensureCooperativeState('drag', [id]);
    }
  }
}

function updateBallHover(hovered) {
  const state = store.getState();
  if (state.ui.ball.hovered === hovered && (hovered ? state.ui.quickChat.open : true)) {
    return;
  }
  store.update((draft) => {
    draft.ui.ball.hovered = hovered;
    if (hovered && !draft.ui.quickChat.open && !draft.ui.confirm.open && !draft.ui.workspace.open && !draft.ui.result.open) {
      draft.ui.quickChat.open = true;
    }
  });
}

document.addEventListener('click', (event) => {
  const action = event.target.closest('[data-action]');
  if (!action) {
    closeMenus();
    return;
  }

  const name = action.dataset.action;
  if (name === 'ball-primary') {
    handleBallPrimaryClick();
    return;
  }
  if (name === 'hide-app') {
    runtimeCall('Hide');
    return;
  }
  if (name === 'quit-app') {
    runtimeCall('Quit');
    return;
  }
  if (name === 'toggle-dev') {
    toggleDevPanel();
    return;
  }
  if (name === 'simulate-default') {
    resetToBallBase();
    return;
  }
  if (name === 'simulate-alert') {
    store.update((draft) => {
      draft.ui.ball.mode = 'alert';
      draft.ui.ball.hovered = false;
      draft.ui.quickChat.open = false;
      draft.ui.confirm.open = false;
      draft.ui.result.open = false;
      draft.ui.workspace.open = false;
      draft.ui.ball.contextMenuOpen = false;
      draft.ui.workspace.menuOpen = false;
      draft.ui.dev.open = true;
    });
    return;
  }
  if (name === 'simulate-plus') {
    store.update((draft) => {
      draft.ui.ball.mode = 'plus';
      draft.ui.ball.hovered = false;
      draft.ui.quickChat.open = false;
      draft.ui.confirm.open = false;
      draft.ui.result.open = false;
      draft.ui.workspace.open = false;
      draft.ui.ball.contextMenuOpen = false;
      draft.ui.workspace.menuOpen = false;
      draft.ui.dev.open = true;
    });
    return;
  }
  if (name === 'simulate-window2') {
    simulateConfirmPanel();
    return;
  }
  if (name === 'simulate-window3') {
    simulateResultPanel();
    return;
  }
  if (name === 'simulate-hover') {
    simulateHoverPanel();
    return;
  }
  if (name === 'simulate-workspace') {
    simulateWorkspacePanel();
    return;
  }
  if (name === 'close-quick-chat') {
    closeQuickChat();
    return;
  }
  if (name === 'quick-chat-send') {
    sendQuickChat();
    return;
  }
  if (name === 'close-confirm') {
    resetToBallBase();
    return;
  }
  if (name === 'confirm-intent') {
    confirmIntent();
    return;
  }
  if (name === 'remove-material') {
    removeMaterial(action.dataset.materialId);
    return;
  }
  if (name === 'close-result') {
    resetToBallBase();
    return;
  }
  if (name === 'open-workspace-from-result') {
    attachResultToConversation();
    openWorkspace();
    return;
  }
  if (name === 'result-to-todo') {
    appendResultToTodo();
    return;
  }
  if (name === 'toggle-workspace-menu') {
    store.update((draft) => {
      draft.ui.workspace.menuOpen = !draft.ui.workspace.menuOpen;
      draft.ui.ball.contextMenuOpen = false;
    });
    return;
  }
  if (name === 'set-workspace-mode') {
    store.update((draft) => {
      draft.ui.workspace.mode = action.dataset.mode;
      draft.ui.workspace.menuOpen = false;
    });
    return;
  }
  if (name === 'close-workspace') {
    closeWorkspace();
    return;
  }
  if (name === 'switch-conversation') {
    switchConversation(action.dataset.conversationId);
    return;
  }
  if (name === 'workspace-fill') {
    const input = document.querySelector('[data-action="workspace-input"]');
    if (input) {
      input.value = `请帮我${action.dataset.fill}当前内容。`;
      input.focus();
    }
    return;
  }
  if (name === 'workspace-send') {
    sendWorkspaceMessage();
    return;
  }
  if (name === 'todo-add') {
    addTodoFromInput();
    return;
  }
  if (name === 'todo-toggle') {
    toggleTodo(action.dataset.todoId);
    return;
  }
  if (name === 'todo-delete') {
    deleteTodo(action.dataset.todoId);
    return;
  }
  if (name === 'status-set') {
    store.update((draft) => {
      draft.data.status = action.dataset.status;
    });
    return;
  }
  if (name === 'setting-toggle') {
    toggleSetting(action.dataset.setting);
    return;
  }
  if (name === 'open-workspace') {
    openWorkspace();
    return;
  }
});

document.addEventListener('dblclick', (event) => {
  if (event.target.closest('[data-action="ball-primary"]')) {
    handleDoubleClickBall();
  }
});

document.addEventListener('contextmenu', (event) => {
  if (event.target.closest('[data-action="ball-primary"]')) {
    handleContextMenuBall(event);
  }
});

document.addEventListener('pointerover', (event) => {
  if (event.target.closest('[data-ball-zone="true"]')) {
    updateBallHover(true);
  }
});

document.addEventListener('pointerout', (event) => {
  if (event.target.closest('[data-ball-zone="true"]')) {
    const related = event.relatedTarget;
    if (!related || !related.closest || !related.closest('[data-ball-zone="true"]')) {
      store.update((draft) => {
        draft.ui.ball.hovered = false;
      });
    }
  }
});

document.addEventListener('dragstart', (event) => {
  dragMaterialFromEvent(event);
});

document.addEventListener('dragover', handleDragOver);
document.addEventListener('drop', handleDrop);

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') {
    closeMenus();
    closeQuickChat();
    store.update((draft) => {
      draft.ui.dev.open = false;
    });
  }
  if (event.key === 'Enter' && event.target.matches('[data-action="quick-chat-input"]')) {
    event.preventDefault();
    sendQuickChat();
  }
  if (event.key === 'Enter' && event.target.matches('[data-action="workspace-input"]')) {
    event.preventDefault();
    sendWorkspaceMessage();
  }
  if (event.key === 'Enter' && event.target.matches('[data-action="todo-input"]')) {
    event.preventDefault();
    addTodoFromInput();
  }
});

document.addEventListener('input', (event) => {
  if (event.target.matches('[data-action="intent-custom-input"]')) {
    store.update((draft) => {
      draft.ui.confirm.customIntent = event.target.value;
    });
  }
});

renderApp();
