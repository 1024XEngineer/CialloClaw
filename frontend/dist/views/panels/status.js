import { escapeHtml } from '../../utils.js';

export function renderStatusPanel(state, data) {
  const snapshot = data.statusSnapshots[state.data.status] || data.statusSnapshots.idle;

  return `
    <div class="workspace-grid workspace-grid-status">
      <section class="status-hero glass-panel">
        <div class="panel-kicker">状态模式</div>
        <h2>${escapeHtml(snapshot.title)}</h2>
        <p>${escapeHtml(snapshot.detail)}</p>

        <div class="status-row">
          <span class="status-chip status-chip--${state.data.status}">${escapeHtml(snapshot.label)}</span>
          <span class="status-chip status-chip--soft">${escapeHtml(snapshot.task)}</span>
        </div>

        <div class="state-switch-row">
          <button type="button" class="secondary-btn" data-action="status-set" data-status="idle">空闲</button>
          <button type="button" class="secondary-btn" data-action="status-set" data-status="working">工作中</button>
          <button type="button" class="secondary-btn" data-action="status-set" data-status="error">故障</button>
        </div>
      </section>

      <section class="status-grid">
        <article class="status-card glass-panel">
          <div class="section-title">当前工作区状态</div>
          <p>${escapeHtml(snapshot.task)}</p>
        </article>
        <article class="status-card glass-panel">
          <div class="section-title">最近一次成功</div>
          <p>把窗口3里的结果接入工作台会话。</p>
        </article>
        <article class="status-card glass-panel">
          <div class="section-title">最近一次失败</div>
          <p>${escapeHtml(snapshot.failure)}</p>
        </article>
      </section>

      <section class="status-log glass-panel">
        <div class="section-title">最近动作记录</div>
        ${snapshot.recent.map((item) => `<div class="log-item">${escapeHtml(item)}</div>`).join('')}
      </section>
    </div>
  `;
}
