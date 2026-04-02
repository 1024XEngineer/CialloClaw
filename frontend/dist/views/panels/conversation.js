import { escapeHtml, groupBy } from '../../utils.js';

function renderMessage(message) {
  const cls = message.kind || message.role;
  return `
    <article class="message-card message-${cls}">
      <div class="message-card__top">
        <span>${escapeHtml(message.title || message.role)}</span>
        <span class="message-time">${escapeHtml(message.time || '')}</span>
      </div>
      <div class="message-card__body">${escapeHtml(message.body)}</div>
    </article>
  `;
}

export function renderConversationPanel(state, data) {
  const convs = data.conversations;
  const grouped = groupBy(convs, (item) => item.bucket);
  const active = convs.find((item) => item.id === state.ui.workspace.activeConversationId) || convs[0];

  return `
    <div class="workspace-grid workspace-grid-conversation">
      <aside class="history-rail glass-panel">
        <div class="rail-head">
          <div class="panel-kicker">历史会话区</div>
          <div class="rail-sub">今天 / 更早</div>
        </div>

        <div class="history-group">
          <div class="history-group__label">今天</div>
          ${ (grouped['今天'] || []).map((item) => `
            <button type="button" class="history-item ${item.id === active.id ? 'is-active' : ''}" data-action="switch-conversation" data-conversation-id="${item.id}">
              <strong>${escapeHtml(item.title)}</strong>
              <span>${escapeHtml(item.time)}</span>
              <p>${escapeHtml(item.summary)}</p>
            </button>
          `).join('') }
        </div>

        <div class="history-group">
          <div class="history-group__label">更早</div>
          ${ (grouped['更早'] || []).map((item) => `
            <button type="button" class="history-item ${item.id === active.id ? 'is-active' : ''}" data-action="switch-conversation" data-conversation-id="${item.id}">
              <strong>${escapeHtml(item.title)}</strong>
              <span>${escapeHtml(item.time)}</span>
              <p>${escapeHtml(item.summary)}</p>
            </button>
          `).join('') }
        </div>
      </aside>

      <section class="conversation-main glass-panel">
        <div class="conversation-head">
          <div>
            <div class="panel-kicker">会话模式</div>
            <h2>${escapeHtml(active.title)}</h2>
          </div>
          <div class="conversation-tags">
            ${active.tags.map((tag) => `<span class="pill">${escapeHtml(tag)}</span>`).join('')}
          </div>
        </div>

        <div class="conversation-stream">
          ${active.messages.map(renderMessage).join('')}
          ${state.ui.result.open && state.ui.result.payload ? `
            <article class="message-card message-result message-attached">
              <div class="message-card__top">
                <span>已接入的结果</span>
                <span class="message-time">刚刚</span>
              </div>
              <div class="message-card__body">${escapeHtml(state.ui.result.payload.summary)}</div>
            </article>
          ` : ''}
        </div>

        <div class="composer-shell">
          <div class="composer-actions">
            <button type="button" class="intent-pill" data-action="workspace-fill" data-fill="提炼">提炼</button>
            <button type="button" class="intent-pill" data-action="workspace-fill" data-fill="改写">改写</button>
            <button type="button" class="intent-pill" data-action="workspace-fill" data-fill="推进">推进</button>
            <button type="button" class="intent-pill" data-action="workspace-fill" data-fill="解释">解释</button>
          </div>
          <div class="composer-row">
            <input type="text" class="text-input composer-input" data-action="workspace-input" placeholder="继续推进当前会话…" />
            <button type="button" class="primary-btn" data-action="workspace-send">发送</button>
          </div>
        </div>
      </section>

      <aside class="workspace-side glass-panel">
        <div class="side-card">
          <div class="section-title">长期协作记忆</div>
          ${data.memoryNotes.map((note) => `<p class="side-note">${escapeHtml(note)}</p>`).join('')}
        </div>
        <div class="side-card">
          <div class="section-title">最近形成的偏好</div>
          <div class="side-chip-list">
            ${data.memoryNotes.map((note) => `<span class="pill pill-soft">${escapeHtml(note)}</span>`).join('')}
          </div>
        </div>
      </aside>
    </div>
  `;
}
