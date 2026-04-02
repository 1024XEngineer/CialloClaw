function symbolForMode(mode) {
  if (mode === 'alert') return '!';
  if (mode === 'plus') return '+';
  return '⋄';
}

export function renderDesktop(state) {
  const ball = state.ui.ball;
  const hiddenBall = state.ui.confirm.open && state.ui.confirm.sourceType === 'click';

  return `
    <section class="desktop-layer" aria-label="桌面常驻入口">
      <div class="desktop-backdrop"></div>
      <div class="desktop-noise"></div>

      <div class="ball-stage ${hiddenBall ? 'is-hidden' : ''}" data-ball-zone="true" data-drop-zone="ball">
        <button
          class="floating-ball ball-${ball.mode} ${ball.hovered ? 'is-hovered' : ''} ${state.ui.quickChat.open ? 'is-paired' : ''}"
          type="button"
          data-action="ball-primary"
          aria-label="悬浮球"
          title="双击打开工作台"
          draggable="false"
        >
          <span class="ball-glow"></span>
          <span class="ball-ear ball-ear-left"></span>
          <span class="ball-ear ball-ear-right"></span>
          <span class="ball-face">
            <span class="ball-eye ball-eye-left"></span>
            <span class="ball-eye ball-eye-right"></span>
            <span class="ball-mouth"></span>
            <span class="ball-symbol">${symbolForMode(ball.mode)}</span>
          </span>
          <span class="ball-caption">CialloClaw</span>
        </button>
      </div>

      <div class="ball-context-menu ${ball.contextMenuOpen ? 'is-open' : ''}" data-context-menu>
        <button type="button" data-action="hide-app">隐藏</button>
        <button type="button" data-action="quit-app" class="danger">退出</button>
      </div>
    </section>
  `;
}
