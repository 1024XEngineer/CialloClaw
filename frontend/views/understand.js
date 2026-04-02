export function renderUnderstandView(data) {
  return `
    <section class="panel-shell standalone-panel">
      <div class="panel-topbar">
        <div>
          <span class="eyebrow">理解面</span>
          <h2>把上下文压成阶段判断，而不是聊天摘要</h2>
        </div>
        <button class="close-button" data-action="close-layer">关闭</button>
      </div>

      <div class="panel-body standalone-layout glass-panel inset-panel">
        <section class="standalone-hero glass-subcard">
          <span class="mini-title">本轮理解</span>
          <h3>用户更需要一个随手可抽出的工作总览层</h3>
          <p>理解面的角色是把任务、镜像与最近观察收束成一句清楚判断，再给出少量证据与下一步建议。</p>
        </section>

        <div class="standalone-card-grid">
          ${data.understandCards
            .map(
              (card) => `
                <article class="glass-subcard result-card">
                  <span class="mini-title">结果卡</span>
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
