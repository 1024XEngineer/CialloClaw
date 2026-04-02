import { getMemoryWorkbenchBinding, memoryLevelOptions } from '../data.js';

function buildManagementEchoes(state, memories) {
  const hiddenEntries = memories
    .filter((memory) => state.hiddenMemoryIds.includes(memory.id))
    .map((memory) => ({ ...memory, managementEcho: '已关闭存档' }));

  const correctedEntries = memories
    .filter((memory) => state.correctedMemoryIds.includes(memory.id))
    .map((memory) => ({ ...memory, managementEcho: '已校正记录' }));

  return [...hiddenEntries, ...correctedEntries];
}

export function getVisibleMemories(state, memories) {
  if (state.memoryLevel === 'management') {
    const managementBase = memories.filter((memory) => memory.level === 'management');
    const echoes = buildManagementEchoes(state, memories);
    const unique = new Map();

    [...managementBase, ...echoes].forEach((memory) => {
      unique.set(memory.id, memory);
    });

    return Array.from(unique.values());
  }

  return memories.filter(
    (memory) => memory.level === state.memoryLevel && !state.hiddenMemoryIds.includes(memory.id)
  );
}

function resolveSelectedMemory(state, memories) {
  return memories.find((memory) => memory.id === state.selectedMemoryId) || memories[0] || null;
}

function typeLabel(type) {
  if (type === 'observation') {
    return '观察卡';
  }
  if (type === 'mirror') {
    return '镜像卡';
  }
  return '阶段折页/时间卡';
}

export function renderMemoryView(state, data) {
  const visibleMemories = getVisibleMemories(state, data.memories);
  const selectedMemory = resolveSelectedMemory(state, visibleMemories.length > 0 ? visibleMemories : data.memories);
  const memoryBinding = selectedMemory ? getMemoryWorkbenchBinding(selectedMemory.id) : null;

  return `
    <section class="panel-shell memory-panel">
      <div class="panel-topbar">
        <div>
          <span class="eyebrow">镜像面板</span>
          <h2>把用户画像与工作记忆做成可读、可校正、可回跳的镜像层</h2>
        </div>
        <button class="close-button" data-action="close-layer">关闭</button>
      </div>

      <div class="panel-body memory-layout">
        <aside class="memory-sidebar glass-panel inset-panel">
          <div class="sidebar-title">镜像层级导航</div>
          <div class="stack-list">
            ${memoryLevelOptions
              .map(
                (option) => `
                  <button
                    class="stack-button ${state.memoryLevel === option.id ? 'stack-button-active' : ''}"
                    data-action="set-memory-level"
                    data-level="${option.id}"
                  >
                    ${option.label}
                  </button>
                `
              )
              .join('')}
          </div>
          <section class="sidebar-footnote glass-subcard">
            <p>镜像不是后台字段，而是为了帮用户看清：最近观察、阶段变化、长期偏好与管理动作之间的关系。</p>
          </section>
        </aside>

        <main class="memory-main glass-panel inset-panel">
          <div class="memory-spotlight glass-subcard ${state.highlightMemoryId === selectedMemory?.id ? 'flash-target' : ''}">
            ${selectedMemory
              ? `
                <div class="memory-spotlight-top">
                  <div>
                    <span class="card-type">${typeLabel(selectedMemory.type)}</span>
                    <h3>${selectedMemory.title}</h3>
                  </div>
                  <div class="meta-pills">
                    <span class="glass-chip">${selectedMemory.updatedAt}</span>
                    ${state.correctedMemoryIds.includes(selectedMemory.id) ? '<span class="glass-chip success-chip">已校正</span>' : ''}
                    ${state.hiddenMemoryIds.includes(selectedMemory.id) ? '<span class="glass-chip muted-chip">已关闭</span>' : ''}
                  </div>
                </div>
                <p class="spotlight-summary">${selectedMemory.summary}</p>
                <p class="spotlight-detail">${selectedMemory.detail}</p>
                <div class="memory-actions">
                  <button class="secondary-action" data-action="correct-memory" data-memory-id="${selectedMemory.id}">
                    ${state.correctedMemoryIds.includes(selectedMemory.id) ? '撤回校正标记' : '校正这条镜像'}
                  </button>
                  <button class="secondary-action" data-action="close-memory" data-memory-id="${selectedMemory.id}">
                    ${state.hiddenMemoryIds.includes(selectedMemory.id) ? '重新打开' : '关闭这条镜像'}
                  </button>
                  ${selectedMemory.relatedTaskId
                    ? `<button class="primary-action" data-action="jump-task" data-task-id="${selectedMemory.relatedTaskId}" data-memory-id="${selectedMemory.id}">查看相关任务</button>`
                    : ''}
                  <button
                    class="secondary-action"
                    data-action="open-workbench"
                    data-memory-id="${selectedMemory.id}"
                    data-scene-id="${memoryBinding?.sceneId || 'doc-digest'}"
                    data-mode="${memoryBinding?.mode || 'understand'}"
                  >
                    按这个模式继续处理
                  </button>
                </div>
              `
              : '<p>当前层级没有可展示的镜像。</p>'}
          </div>

          <div class="memory-grid">
            ${visibleMemories
              .map(
                (memory) => `
                  <article
                    class="memory-card glass-subcard ${selectedMemory?.id === memory.id ? 'memory-card-active' : ''} ${state.highlightMemoryId === memory.id ? 'flash-target' : ''}"
                    data-action="select-memory"
                    data-memory-id="${memory.id}"
                  >
                    <div class="memory-card-top">
                      <span class="card-type">${typeLabel(memory.type)}</span>
                      <span class="memory-time">${memory.updatedAt}</span>
                    </div>
                    <h4>${memory.title}</h4>
                    <p>${memory.summary}</p>
                    <div class="memory-card-foot">
                      ${memory.managementEcho ? `<span class="memory-echo">${memory.managementEcho}</span>` : `<span>${memoryLevelOptions.find((item) => item.id === memory.level)?.label || ''}</span>`}
                      ${memory.relatedTaskId ? '<span>可回跳任务</span>' : '<span>仅查看</span>'}
                    </div>
                    ${memory.relatedTaskId || getMemoryWorkbenchBinding(memory.id)
                      ? `
                        <button
                          class="memory-card-action"
                          data-action="open-workbench"
                          data-memory-id="${memory.id}"
                          data-scene-id="${getMemoryWorkbenchBinding(memory.id).sceneId}"
                          data-mode="${getMemoryWorkbenchBinding(memory.id).mode}"
                        >
                          按这个模式继续处理
                        </button>
                      `
                      : ''}
                  </article>
                `
              )
              .join('')}
          </div>
        </main>
      </div>
    </section>
  `;
}
