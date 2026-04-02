import { escapeHtml } from '../utils.js';

export function renderResultPopup(state) {
  if (!state.ui.result.open || !state.ui.result.payload) return '';
  const result = state.ui.result.payload;

  return `
    <aside class="result-popup glass-panel" aria-label="窗口3 / 小型回复窗">
      <header class="panel-topline">
        <div>
          <div class="panel-kicker">窗口3 / 小型回复窗</div>
          <h2>${escapeHtml(result.cardTitle)}</h2>
        </div>
        <button type="button" class="icon-btn" data-action="close-result" aria-label="关闭">×</button>
      </header>

      <div class="result-main">
        <div class="result-title">${escapeHtml(result.title)}</div>
        <p class="result-body">${escapeHtml(result.body)}</p>
        <ul class="result-bullets">
          ${result.bullets.map((bullet) => `<li>${escapeHtml(bullet)}</li>`).join('')}
        </ul>
        <div class="result-summary">来源：${escapeHtml(result.sourceNames)}</div>
      </div>

      <div class="result-actions">
        <button type="button" class="primary-btn" data-action="open-workspace-from-result">打开工作台</button>
        <button type="button" class="secondary-btn" data-action="result-to-todo">加入待办</button>
      </div>
    </aside>
  `;
}
