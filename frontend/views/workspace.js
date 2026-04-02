import { escapeHtml } from '../utils.js';
import { renderConversationPanel } from './panels/conversation.js';
import { renderStatusPanel } from './panels/status.js';
import { renderTodosPanel } from './panels/todos.js';
import { renderSettingsPanel } from './panels/settings.js';

const modeLabels = {
  conversation: '会话',
  status: '状态',
  todos: '待办',
  settings: '设置'
};

export function renderWorkspace(state, data) {
  if (!state.ui.workspace.open) return '';

  const mode = state.ui.workspace.mode;
  const body = mode === 'status'
    ? renderStatusPanel(state, data)
    : mode === 'todos'
      ? renderTodosPanel(state, data)
      : mode === 'settings'
        ? renderSettingsPanel(state, data)
        : renderConversationPanel(state, data);

  return `
    <section class="workspace-shell glass-panel ${state.ui.workspace.open ? 'is-open' : ''}" aria-label="工作台主窗口">
      <header class="workspace-topbar">
        <div class="workspace-title-block">
          <div class="panel-kicker">工作台 / 主窗口</div>
          <h2>默认进入会话模式</h2>
        </div>

        <div class="workspace-menu-wrap">
          <button type="button" class="menu-trigger" data-action="toggle-workspace-menu">
            ${escapeHtml(modeLabels[mode])} ▾
          </button>
          <div class="workspace-menu ${state.ui.workspace.menuOpen ? 'is-open' : ''}">
            ${Object.entries(modeLabels).map(([key, label]) => `
              <button type="button" class="menu-item ${mode === key ? 'is-active' : ''}" data-action="set-workspace-mode" data-mode="${key}">${escapeHtml(label)}</button>
            `).join('')}
          </div>
        </div>

        <button type="button" class="icon-btn" data-action="close-workspace" aria-label="关闭工作台">×</button>
      </header>

      <div class="workspace-body">
        ${body}
      </div>
    </section>
  `;
}
