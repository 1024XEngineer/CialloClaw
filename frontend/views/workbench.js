import { findWorkbenchScene, workbenchSceneOrder } from '../data.js';

function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function renderSceneTabs(currentSceneId) {
  return workbenchSceneOrder
    .map((sceneId) => {
      const scene = findWorkbenchScene(sceneId);
      return `
        <button
          class="scene-tab ${currentSceneId === sceneId ? 'scene-tab-active' : ''}"
          data-action="select-workbench-scene"
          data-scene-id="${sceneId}"
        >
          <span>${scene.label}</span>
        </button>
      `;
    })
    .join('');
}

function renderModeTabs(currentMode) {
  return [
    { id: 'understand', label: '理解模式' },
    { id: 'advance', label: '推进模式' }
  ]
    .map(
      (mode) => `
        <button
          class="mode-tab ${currentMode === mode.id ? 'mode-tab-active' : ''}"
          data-action="set-workbench-mode"
          data-mode="${mode.id}"
        >
          ${mode.label}
        </button>
      `
    )
    .join('');
}

function renderResultCards(cards) {
  return cards
    .map(
      (card) => `
        <article class="glass-subcard workbench-result-card">
          <span class="mini-title">结果卡</span>
          <h4>${card.title}</h4>
          <p>${card.body}</p>
        </article>
      `
    )
    .join('');
}

function renderActions(actions) {
  return actions
    .map((action) => {
      if (action.kind === 'navigate') {
        return `
          <button
            class="secondary-action workbench-action workbench-action-navigate"
            data-action="workbench-action"
            data-action-id="${action.id}"
          >
            ${action.label}
          </button>
        `;
      }

      return `
        <button
          class="primary-action workbench-action workbench-action-confirm"
          data-action="workbench-action"
          data-action-id="${action.id}"
        >
          ${action.label}
        </button>
      `;
    })
    .join('');
}

function renderFollowUpHistory(log) {
  if (!log || log.length === 0) {
    return '<div class="followup-empty">轻追问条在这里承接补充，不会把它做成完整聊天。</div>';
  }

  return log
    .slice(-2)
    .map(
      (entry) => `
        <div class="followup-log-item glass-subcard">
          <strong>${escapeHtml(entry.question)}</strong>
          <p>${escapeHtml(entry.reply)}</p>
        </div>
      `
    )
    .join('');
}

export function renderWorkbenchView(state, data, scene, context) {
  const activeScene = scene || findWorkbenchScene(state.workbenchSceneId);
  const activeMode = activeScene.modes[state.workbenchMode] || activeScene.modes[activeScene.defaultMode];
  const sourceContext = context || state.lastWorkbenchContext || null;
  const objectTitle = sourceContext?.title || activeScene.objectTitle;
  const objectSummary = sourceContext?.summary || activeScene.objectSummary;
  const objectLabel = sourceContext?.label || activeScene.objectLabel;
  const contextNote = sourceContext?.detail || activeScene.objectSummary;
  const sourceLabel = sourceContext?.sourceLabel || activeScene.label;
  const outcome = state.workbenchOutcome;

  return `
    <section class="panel-shell panel-shell-workbench workbench-shell">
      <div class="panel-topbar workbench-topbar">
        <div>
          <span class="eyebrow">收束台 / Workbench</span>
          <h2>${activeMode.badge} · ${activeScene.label}</h2>
        </div>
        <button class="close-button" data-action="close-layer">关闭</button>
      </div>

      <div class="workbench-switch-row">
        <div class="scene-tab-group">
          ${renderSceneTabs(activeScene.id)}
        </div>
        <div class="mode-tab-group">
          ${renderModeTabs(state.workbenchMode)}
        </div>
      </div>

      <div class="workbench-body">
        <section class="workbench-object glass-panel inset-panel">
          <div class="workbench-object-top">
            <div>
              <span class="mini-title">对象区</span>
              <h3>${objectTitle}</h3>
            </div>
            <div class="meta-pills">
              <span class="glass-chip">${objectLabel}</span>
              <span class="glass-chip">来源：${sourceLabel}</span>
            </div>
          </div>
          <p class="workbench-object-summary">${objectSummary}</p>
          <p class="workbench-object-note">${contextNote}</p>
          <div class="context-list">
            ${(sourceContext?.tags || activeScene.objectTags)
              .map((tag) => `<em>${tag}</em>`)
              .join('')}
          </div>
        </section>

        ${outcome
          ? `
            <section class="workbench-outcome glass-subcard">
              <span class="mini-title">执行回声</span>
              <strong>${outcome.title}</strong>
              <p>${outcome.detail}</p>
            </section>
          `
          : ''}

        <section class="workbench-results-grid">
          ${renderResultCards(activeMode.cards)}
        </section>

        <section class="workbench-actions glass-panel inset-panel">
          <div class="mini-title">动作区</div>
          <div class="workbench-action-row">
            ${renderActions(activeMode.actions)}
          </div>
        </section>
      </div>

      <section class="workbench-followup glass-panel inset-panel">
        <div class="workbench-followup-head">
          <div>
            <span class="mini-title">轻追问条</span>
            <p>它只负责补充一句，不把界面做成聊天应用。</p>
          </div>
          <div class="workbench-followup-chips">
            ${activeMode.followUps
              .map(
                (item) => `
                  <button
                    class="followup-chip"
                    data-action="use-followup-chip"
                    data-value="${item}"
                  >
                    ${item}
                  </button>
                `
              )
              .join('')}
          </div>
        </div>

        <form class="workbench-followup-form" data-action="submit-followup-form">
          <input
            class="followup-input"
            type="text"
            value="${escapeHtml(state.followUpDraft || '')}"
            placeholder="补一句，比如：再简短一点 / 换成日报语气"
            data-role="workbench-followup-input"
          />
          <button class="followup-send" type="submit">送出</button>
        </form>

        <div class="followup-log">
          ${renderFollowUpHistory(state.followUpLog)}
        </div>
      </section>
    </section>
  `;
}

export function renderActionBar(state) {
  if (!state.actionBar) {
    return '';
  }

  const bar = state.actionBar;

  return `
    <section class="action-bar glass-panel ${bar.expanded ? 'action-bar-expanded' : ''}">
      <div class="action-bar-main">
        <span class="eyebrow">顶部执行确认条</span>
        <h3>${bar.title}</h3>
        <p>${bar.impact}</p>
      </div>
      <div class="action-bar-meta">
        <span class="glass-chip">会影响：${bar.scopes.slice(0, 3).join(' / ')}</span>
        <span class="glass-chip">可撤回：${bar.reversible}</span>
      </div>
      <div class="action-bar-actions">
        <button class="primary-action" data-action="confirm-action-bar">${bar.confirmLabel || '确认'}</button>
        <button class="secondary-action" data-action="allow-once-action-bar">${bar.allowOnceLabel || '仅这次允许'}</button>
        <button class="secondary-action" data-action="toggle-action-bar-scope">${bar.expanded ? '收起影响范围' : '查看影响范围'}</button>
        <button class="secondary-action" data-action="cancel-action-bar">取消</button>
      </div>
      ${bar.expanded
        ? `
          <div class="action-bar-scope">
            ${bar.scopes.map((scope) => `<span class="glass-chip">${scope}</span>`).join('')}
          </div>
        `
        : ''}
    </section>
  `;
}
