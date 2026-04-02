import { escapeHtml, groupBy } from '../../utils.js';

function renderTodoCard(todo) {
  return `
    <article class="todo-card ${todo.status === '已完成' ? 'is-done' : ''}">
      <div class="todo-card__head">
        <strong>${escapeHtml(todo.title)}</strong>
        <span class="todo-status">${escapeHtml(todo.status)}</span>
      </div>
      <div class="todo-meta">来源：${escapeHtml(todo.source)}</div>
      <div class="todo-meta">${todo.agentGenerated ? 'Agent 建议生成' : '用户手动新增'}</div>
      <div class="todo-actions">
        <button type="button" class="mini-btn" data-action="todo-toggle" data-todo-id="${todo.id}">${todo.status === '已完成' ? '恢复' : '完成'}</button>
        <button type="button" class="mini-btn danger" data-action="todo-delete" data-todo-id="${todo.id}">删除</button>
      </div>
    </article>
  `;
}

export function renderTodosPanel(state, data) {
  const grouped = groupBy(state.data.todos, (todo) => todo.bucket);

  return `
    <div class="workspace-grid workspace-grid-todos">
      <section class="todo-creator glass-panel">
        <div class="panel-kicker">待办模式</div>
        <h2>用户可手动新增待办</h2>
        <div class="todo-input-row">
          <input type="text" class="text-input" data-action="todo-input" placeholder="输入一个待办标题…" />
          <button type="button" class="primary-btn" data-action="todo-add">添加</button>
        </div>
      </section>

      <section class="todo-columns">
        <div class="todo-column glass-panel">
          <div class="section-title">今日</div>
          ${(grouped.today || []).map(renderTodoCard).join('') || '<div class="empty-note">暂无今日待办</div>'}
        </div>
        <div class="todo-column glass-panel">
          <div class="section-title">稍后</div>
          ${(grouped.later || []).map(renderTodoCard).join('') || '<div class="empty-note">暂无稍后待办</div>'}
        </div>
        <div class="todo-column glass-panel">
          <div class="section-title">已完成</div>
          ${(grouped.done || []).map(renderTodoCard).join('') || '<div class="empty-note">暂无已完成待办</div>'}
        </div>
      </section>

      <section class="todo-link glass-panel">
        <div class="section-title">协作联动</div>
        <p>窗口2、窗口3 和会话结果都可以转成待办；这里用静态 mock 数据模拟这一联动。</p>
      </section>
    </div>
  `;
}
