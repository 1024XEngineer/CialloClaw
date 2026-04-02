export function renderAdvanceView(data) {
  return `
    <section class="panel-shell standalone-panel">
      <div class="panel-topbar">
        <div>
          <span class="eyebrow">推进面</span>
          <h2>把接下来的动作分层摆开，像工作板而不是输入框</h2>
        </div>
        <button class="close-button" data-action="close-layer">关闭</button>
      </div>

      <div class="panel-body standalone-layout glass-panel inset-panel">
        <section class="standalone-hero glass-subcard">
          <span class="mini-title">推进视角</span>
          <h3>先把最顺的一步推出来，再处理等待与风险</h3>
          <p>推进面负责帮用户决定“下一步往哪动”，因此卡片表达要短、稳、可执行。</p>
        </section>

        <div class="standalone-card-grid">
          ${data.advanceCards
            .map(
              (card) => `
                <article class="glass-subcard result-card">
                  <span class="mini-title">推进卡</span>
                  <h4>${card.title}</h4>
                  <p>${card.body}</p>
                </article>
              `
            )
            .join('')}
        </div>
      </div>
    </section>
  `;
}
