import { desktopSignalCards, findHintCard } from '../data.js';

export function renderDesktop(state, data) {
  const quickCard = state.quickCardOpen
    ? `
      <div class="desktop-island-card glass-panel">
        <div class="mini-title">桌面岛捷径</div>
        <button
          class="quick-link"
          data-action="open-workbench"
          data-scene-id="${state.lastWorkbenchSceneId || 'doc-digest'}"
          data-mode="${state.lastWorkbenchMode || 'understand'}"
          data-purpose="continue-current"
        >
          继续当前处理
        </button>
        <button class="quick-link" data-action="open-view" data-view="tasks">任务控制台</button>
        <button class="quick-link" data-action="open-view" data-view="memory">镜像面板</button>
        <button
          class="quick-link"
          data-action="show-hint-card"
          data-hint-id="hint-doc-digest"
        >
          轻提示
        </button>
      </div>
    `
    : '';

  const activeHintCard = state.hintCardId ? findHintCard(state.hintCardId) : null;
  const hintCard = activeHintCard
    ? `
      <aside class="light-hint-card glass-panel">
        <div class="light-hint-top">
          <div>
            <span class="mini-title">轻提示承接卡</span>
            <h4>${activeHintCard.title}</h4>
          </div>
          <button class="hint-close" data-action="hide-hint-card">×</button>
        </div>
        <p>${activeHintCard.body}</p>
        <div class="hint-meta">
          <span class="glass-chip">来源：${activeHintCard.sourceLabel}</span>
          <span class="glass-chip">模式：${activeHintCard.mode === 'advance' ? '推进' : '理解'}</span>
        </div>
        <div class="hint-actions">
          <button
            class="primary-action hint-primary"
            data-action="open-workbench"
            data-hint-id="${activeHintCard.id}"
          >
            ${activeHintCard.actionLabel}
          </button>
        </div>
      </aside>
    `
    : '';

  const orbDirections = state.orbExpanded
    ? `
      <button class="orb-direction orb-up" data-action="open-workbench" data-scene-id="doc-digest" data-mode="understand">理解</button>
      <button class="orb-direction orb-right" data-action="open-workbench" data-scene-id="daily-report" data-mode="advance">推进</button>
      <button class="orb-direction orb-left" data-action="open-view" data-view="tasks">巡检</button>
      <button class="orb-direction orb-down" data-action="open-view" data-view="memory">记忆</button>
    `
    : '';

  return `
    <section class="desktop-scene ${state.activeView !== 'desktop' ? 'desktop-scene-dim' : ''}">
      <div class="desktop-backdrop"></div>
      <div class="desktop-grid"></div>
      <header class="desktop-header">
        <div>
          <span class="eyebrow">CialloClaw / 桌面常驻 UX mock</span>
          <h1>把任务、理解与镜像留在桌面边上</h1>
        </div>
        <div class="desktop-header-note glass-chip">重点是交互闭环，不是聊天入口</div>
      </header>

      <div class="mock-window-layer">
        ${data.desktopWindows
          .map(
            (window, index) => `
              <article class="mock-window glass-panel mock-window-${index + 1}">
                <div class="mock-window-head">
                  <span>${window.title}</span>
                  <span class="window-tag">${window.tag}</span>
                </div>
                ${window.lines.map((line) => `<p>${line}</p>`).join('')}
                <div class="window-signal-row">
                  ${desktopSignalCards.filter((signal) => signal.windowId === window.id).map((signal) => {
                    const card = findHintCard(signal.hintId);
                    return `
                      <button
                        class="window-signal"
                        data-action="show-hint-card"
                        data-hint-id="${signal.hintId}"
                      >
                        ${card?.triggerLabel || '可接住'}
                      </button>
                    `;
                  }).join('')}
                </div>
              </article>
            `
          )
          .join('')}
      </div>

      <aside class="side-beacon glass-panel">
        <button class="beacon-main" data-action="open-view" data-view="tasks">
          <span class="beacon-kicker">侧边信标</span>
          <strong>打开任务控制台</strong>
          <small>直接看全貌，不走聊天框</small>
        </button>
        <div class="beacon-links">
          <button class="beacon-secondary" data-action="open-view" data-view="tasks" data-filter="现在要推进">看现在要推进</button>
          <button class="beacon-secondary" data-action="show-hint-card" data-hint-id="hint-daily-report">打开轻提示</button>
        </div>
      </aside>

      <div class="floating-orb-zone">
        ${orbDirections}
        <button class="floating-orb glass-panel ${state.orbExpanded ? 'floating-orb-active' : ''}" data-action="toggle-orb">
          <span class="orb-core">C</span>
          <span class="orb-label">四向悬浮球</span>
        </button>
      </div>

      <div class="desktop-island-zone">
        ${quickCard}
        <button class="desktop-island glass-panel" data-action="toggle-quick-card">
          <span class="island-title">桌面岛</span>
          <span class="island-subtitle">继续当前处理 / 任务 / 镜像</span>
        </button>
      </div>

      ${hintCard}
    </section>
  `;
}
