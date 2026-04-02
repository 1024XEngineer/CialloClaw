import { escapeHtml } from '../utils.js';

export function renderIntentPanel(state, data) {
  if (!state.ui.confirm.open) return '';
  const guessed = state.ui.confirm.guessedIntent;
  const custom = state.ui.confirm.customIntent;

  return `
    <aside class="intent-panel glass-panel" aria-label="承接确认窗" data-drop-zone="confirm">
      <header class="panel-topline">
        <div>
          <div class="panel-kicker">窗口2 / 承接确认窗</div>
          <h2>先判断，再让你确认</h2>
        </div>
        <div class="intent-top-actions">
          <div class="mini-status">${state.ui.confirm.sourceType === 'drag' ? '拖入承接' : '轻提示承接'}</div>
          <button type="button" class="icon-btn" data-action="close-confirm" aria-label="关闭">×</button>
        </div>
      </header>

      <section class="intent-section intent-section-materials">
        <div class="section-title">A. 输入材料区</div>
        <div class="material-grid">
          ${state.ui.confirm.materials.map((item) => `
            <article class="material-card ${item.kind}">
              <button type="button" class="material-delete" data-action="remove-material" data-material-id="${item.id}" aria-label="删除">×</button>
              <div class="material-card__label">${escapeHtml(item.label)}</div>
              <h3>${escapeHtml(item.title)}</h3>
              <p>${escapeHtml(item.summary)}</p>
              <div class="material-card__meta">${escapeHtml(item.kind.toUpperCase())}</div>
            </article>
          `).join('')}
        </div>
        <div class="add-source-row">
          ${data.materialsForDrag.map((item) => `
            <button
              type="button"
              class="source-chip source-chip--add"
              draggable="true"
              data-drag-kind="${item.id}"
            >再拖入：${escapeHtml(item.label)}</button>
          `).join('')}
        </div>
      </section>

      <section class="intent-section intent-section-judge">
        <div class="section-title">B. 意图判断区</div>
        <div class="judge-line">判断意图为：<strong>${escapeHtml(guessed)}</strong></div>
        <div class="judge-note">系统先看材料，再让你确认；这里可以把它改成更贴近现场的说法。</div>
        <label class="field-block">
          <span>修改：</span>
          <input type="text" class="text-input" value="${escapeHtml(custom)}" placeholder="例如：改写成日报语气 / 提炼重点" data-action="intent-custom-input" />
        </label>
      </section>

      <section class="intent-section intent-section-confirm">
        <div class="section-title">C. 确认区</div>
        <button type="button" class="confirm-circle" data-action="confirm-intent" aria-label="确认">
          <span>✓</span>
        </button>
      </section>
    </aside>
  `;
}
