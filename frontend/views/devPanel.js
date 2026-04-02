export function renderDevPanel(state) {
  return `
    <section class="dev-rail ${state.ui.dev.open ? 'is-open' : ''}" aria-label="开发调试入口">
      <button type="button" class="dev-toggle" data-action="toggle-dev">DEV</button>
      <div class="dev-panel glass-panel">
        <button type="button" class="dev-btn" data-action="simulate-default">默认态</button>
        <button type="button" class="dev-btn" data-action="simulate-alert">感叹号</button>
        <button type="button" class="dev-btn" data-action="simulate-plus">加号</button>
        <button type="button" class="dev-btn" data-action="simulate-window2">打开窗口2</button>
        <button type="button" class="dev-btn" data-action="simulate-window3">打开窗口3</button>
        <button type="button" class="dev-btn" data-action="simulate-hover">Hover 轻窗</button>
        <button type="button" class="dev-btn" data-action="simulate-workspace">工作台</button>
      </div>
    </section>
  `;
}
