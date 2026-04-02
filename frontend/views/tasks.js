import { getTaskWorkbenchBinding, taskBrowseOptions, taskFilterOptions } from '../data.js';

export function getVisibleTasks(state, tasks) {
  return tasks.filter((task) => state.taskFilter === 'all' || task.actionState === state.taskFilter);
}

function groupTasks(tasks, mode) {
  const groupMap = new Map();

  tasks.forEach((task) => {
    let key = task.actionState;
    if (mode === 'time') {
      key = task.timeBucket;
    }
    if (mode === 'project') {
      key = task.project;
    }

    if (!groupMap.has(key)) {
      groupMap.set(key, []);
    }

    groupMap.get(key).push(task);
  });

  return Array.from(groupMap.entries()).map(([title, items]) => ({ title, items }));
}

function resolveSelectedTask(state, tasks) {
  return tasks.find((task) => task.id === state.selectedTaskId) || tasks[0] || null;
}

export function renderTasksView(state, data) {
  const visibleTasks = getVisibleTasks(state, data.tasks);
  const selectedTask = resolveSelectedTask(state, visibleTasks.length > 0 ? visibleTasks : data.tasks);
  const groups = groupTasks(visibleTasks, state.taskBrowseMode);
  const totalNow = data.tasks.filter((task) => task.actionState === '现在要推进').length;
  const totalStalled = data.tasks.filter((task) => task.actionState === '已停滞').length;
  const taskBinding = selectedTask ? getTaskWorkbenchBinding(selectedTask.id) : null;

  return `
    <section class="panel-shell panel-shell-wide task-panel">
      <div class="panel-topbar">
        <div>
          <span class="eyebrow">任务控制台</span>
          <h2>把“现在该动什么”放到桌面正面</h2>
        </div>
        <button class="close-button" data-action="close-layer">关闭</button>
      </div>

      <div class="panel-body task-layout">
        <aside class="task-sidebar glass-panel inset-panel">
          <section>
            <div class="sidebar-title">浏览方式</div>
            <div class="stack-list">
              ${taskBrowseOptions
                .map(
                  (option) => `
                    <button
                      class="stack-button ${state.taskBrowseMode === option.id ? 'stack-button-active' : ''}"
                      data-action="set-task-browse"
                      data-mode="${option.id}"
                    >
                      ${option.label}
                    </button>
                  `
                )
                .join('')}
            </div>
          </section>

          <section>
            <div class="sidebar-title">筛选分组</div>
            <div class="stack-list">
              ${taskFilterOptions
                .map(
                  (option) => `
                    <button
                      class="stack-button ${state.taskFilter === option.id ? 'stack-button-active' : ''}"
                      data-action="set-task-filter"
                      data-filter="${option.id}"
                    >
                      ${option.label}
                    </button>
                  `
                )
                .join('')}
            </div>
          </section>

          <section class="sidebar-footnote glass-subcard">
            <div class="meta-line"><span>现在要推进</span><strong>${totalNow}</strong></div>
            <div class="meta-line"><span>已停滞</span><strong>${totalStalled}</strong></div>
            <p>左侧负责决定“怎么看”，中间负责看全貌，右侧负责把下一步说清楚。</p>
          </section>
        </aside>

        <main class="task-main glass-panel inset-panel">
          <div class="task-main-head">
            <div>
              <div class="mini-title">任务主内容区</div>
              <h3>${state.taskFilter === 'all' ? '全部任务分区' : `${state.taskFilter} · 分区浏览`}</h3>
            </div>
            <div class="meta-pills">
              <span class="glass-chip">共 ${visibleTasks.length} 条</span>
              <span class="glass-chip">当前视角：${taskBrowseOptions.find((option) => option.id === state.taskBrowseMode)?.label}</span>
            </div>
          </div>

          <div class="task-sections">
            ${groups
              .map(
                (group) => `
                  <section class="task-group">
                    <div class="task-group-head">
                      <h4>${group.title}</h4>
                      <span>${group.items.length} 条</span>
                    </div>
                    <div class="task-card-grid">
                      ${group.items
                        .map(
                          (task) => `
                            <article
                              class="task-card glass-subcard ${selectedTask?.id === task.id ? 'task-card-active' : ''} ${state.highlightTaskId === task.id ? 'flash-target' : ''}"
                              data-action="select-task"
                              data-task-id="${task.id}"
                            >
                              <div class="task-card-top">
                                <span class="task-project">${task.project}</span>
                                <span class="task-badge">${task.importance}优先</span>
                              </div>
                              <h5>${task.title}</h5>
                              <p>${task.lastStop}</p>
                              <div class="task-card-foot">
                                <span>${task.timeBucket}</span>
                                <span>${task.actionState}</span>
                              </div>
                            </article>
                          `
                        )
                        .join('')}
                    </div>
                  </section>
                `
              )
              .join('')}
          </div>
        </main>

        <aside class="task-detail glass-panel inset-panel">
          ${selectedTask
            ? `
              <div class="mini-title">任务详情抽屉</div>
              <h3>${selectedTask.title}</h3>
              <div class="detail-block">
                <span>为什么它现在重要</span>
                <p>${selectedTask.whyNow}</p>
              </div>
              <div class="detail-block">
                <span>上次停在哪里</span>
                <p>${selectedTask.lastStop}</p>
              </div>
              <div class="detail-block">
                <span>从这里最顺的一步</span>
                <p>${selectedTask.nextStep}</p>
              </div>
              <div class="detail-block">
                <span>相关上下文</span>
                <div class="context-list">
                  ${selectedTask.context.map((item) => `<em>${item}</em>`).join('')}
                </div>
              </div>
              <div class="task-detail-actions">
                <button
                  class="primary-action"
                  data-action="open-workbench"
                  data-task-id="${selectedTask.id}"
                  data-scene-id="${taskBinding?.sceneId || 'doc-digest'}"
                  data-mode="${taskBinding?.mode || 'advance'}"
                >
                  推进这件事
                </button>
                <button
                  class="secondary-action"
                  data-action="jump-memory"
                  data-task-id="${selectedTask.id}"
                  data-memory-id="${selectedTask.relatedMemoryIds[0] || ''}"
                >
                  查看相关镜像
                </button>
              </div>
            `
            : '<p>当前筛选下没有任务。</p>'}
        </aside>
      </div>
    </section>
  `;
}
