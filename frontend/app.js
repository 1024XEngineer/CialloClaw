import {
  appData,
  findHintCard,
  findMemory,
  findTask,
  findWorkbenchScene,
  getMemoryWorkbenchBinding,
  getPrimaryMemoryForTask,
  getTaskWorkbenchBinding
} from './data.js';
import { createRouter, initialState } from './router.js';
import { renderDesktop } from './views/desktop.js';
import { renderTasksView } from './views/tasks.js';
import { renderMemoryView } from './views/memory.js';
import { renderUnderstandView } from './views/understand.js';
import { renderAdvanceView } from './views/advance.js';
import { renderActionBar, renderWorkbenchView } from './views/workbench.js';

const root = document.querySelector('#app');
const router = createRouter(initialState);
let toastTimer = null;

function buildDesktopContext(scene) {
  return {
    sourceType: 'desktop',
    sourceLabel: '桌面岛',
    label: scene.objectLabel,
    title: scene.objectTitle,
    summary: scene.objectSummary,
    detail: scene.objectSummary,
    sceneId: scene.id,
    tags: scene.objectTags
  };
}

function buildHintContext(hint) {
  const scene = findWorkbenchScene(hint.sceneId);
  return {
    sourceType: 'hint',
    sourceLabel: hint.sourceLabel,
    label: '轻提示承接卡',
    title: hint.title,
    summary: hint.body,
    detail: `${hint.triggerLabel} · ${hint.sourceLabel}`,
    hintId: hint.id,
    sceneId: scene.id,
    tags: [hint.triggerLabel, hint.sourceLabel, scene.label]
  };
}

function buildTaskContext(task, binding) {
  return {
    sourceType: 'task',
    sourceLabel: task.project,
    label: '当前任务',
    title: task.title,
    summary: task.whyNow,
    detail: task.nextStep,
    taskId: task.id,
    sceneId: binding.sceneId,
    tags: [task.actionState, task.timeBucket, task.project]
  };
}

function buildMemoryContext(memory, binding) {
  const levelLabel = {
    recent: '最近记忆',
    phase: '阶段镜像',
    preference: '长期偏好',
    management: '记忆管理'
  }[memory.level] || '记忆';

  return {
    sourceType: 'memory',
    sourceLabel: levelLabel,
    label: '当前镜像',
    title: memory.title,
    summary: memory.summary,
    detail: memory.detail,
    memoryId: memory.id,
    sceneId: binding.sceneId,
    tags: [levelLabel, memory.type, memory.updatedAt]
  };
}

function buildDefaultWorkbenchContext(scene) {
  return buildDesktopContext(scene);
}

function renderActivePanel(state) {
  if (state.activeView === 'tasks') {
    return renderTasksView(state, appData);
  }
  if (state.activeView === 'memory') {
    return renderMemoryView(state, appData);
  }
  if (state.activeView === 'workbench') {
    const scene = findWorkbenchScene(state.workbenchSceneId);
    return renderWorkbenchView(state, appData, scene, state.workbenchContext || state.lastWorkbenchContext || buildDefaultWorkbenchContext(scene));
  }
  if (state.activeView === 'understand') {
    return renderUnderstandView(appData);
  }
  if (state.activeView === 'advance') {
    return renderAdvanceView(appData);
  }
  return '';
}

function shouldShowScrim(state) {
  return state.activeView !== 'desktop' || state.quickCardOpen;
}

function getScrimClass(state) {
  if (state.activeView !== 'desktop') {
    return 'screen-scrim';
  }

  if (state.quickCardOpen) {
    return 'screen-scrim screen-scrim-quick';
  }

  return '';
}

function renderApp(state) {
  root.innerHTML = `
    <div class="app-shell">
      ${renderDesktop(state, appData)}
      ${shouldShowScrim(state) ? `<div class="${getScrimClass(state)}" data-action="dismiss-layer"></div>` : ''}
      ${renderActionBar(state)}
      <div class="panel-host ${state.activeView !== 'desktop' ? 'panel-host-active' : ''} ${state.activeView === 'workbench' ? 'panel-host-workbench' : ''}">
        ${renderActivePanel(state)}
      </div>
      ${state.toast ? `<div class="toast">${state.toast.message}</div>` : ''}
    </div>
  `;

  if (state.highlightTaskId || state.highlightMemoryId) {
    requestAnimationFrame(() => {
      const targetId = state.highlightTaskId || state.highlightMemoryId;
      const target = document.querySelector(`[data-task-id="${targetId}"], [data-memory-id="${targetId}"]`);
      if (target) {
        target.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      setTimeout(() => {
        router.dispatch({ type: 'CLEAR_HIGHLIGHTS' });
      }, 1100);
    });
  }

  if (toastTimer) {
    window.clearTimeout(toastTimer);
    toastTimer = null;
  }

  if (state.toast) {
    const toastId = state.toast.id;
    toastTimer = window.setTimeout(() => {
      router.dispatch({ type: 'CLEAR_TOAST', toastId });
    }, 1800);
  }
}

function openWorkbench(trigger) {
  const state = router.getState();

  if (trigger.dataset.hintId) {
    const hint = findHintCard(trigger.dataset.hintId);
    if (!hint) {
      router.dispatch({ type: 'SHOW_TOAST', message: '这条轻提示没有找到。' });
      return;
    }

    const scene = findWorkbenchScene(hint.sceneId);
    router.dispatch({
      type: 'OPEN_WORKBENCH',
      sceneId: scene.id,
      mode: hint.mode,
      context: buildHintContext(hint)
    });
    return;
  }

  if (trigger.dataset.taskId) {
    const task = findTask(trigger.dataset.taskId);
    if (!task) {
      router.dispatch({ type: 'SHOW_TOAST', message: '这条任务没有找到。' });
      return;
    }

    const binding = getTaskWorkbenchBinding(task.id);
    router.dispatch({
      type: 'OPEN_WORKBENCH',
      sceneId: binding.sceneId,
      mode: binding.mode,
      context: buildTaskContext(task, binding)
    });
    return;
  }

  if (trigger.dataset.memoryId) {
    const memory = findMemory(trigger.dataset.memoryId);
    if (!memory) {
      router.dispatch({ type: 'SHOW_TOAST', message: '这条镜像没有找到。' });
      return;
    }

    const binding = getMemoryWorkbenchBinding(memory.id);
    router.dispatch({
      type: 'OPEN_WORKBENCH',
      sceneId: binding.sceneId,
      mode: binding.mode,
      context: buildMemoryContext(memory, binding)
    });
    return;
  }

  const sceneId = trigger.dataset.sceneId || state.lastWorkbenchSceneId || 'doc-digest';
  const scene = findWorkbenchScene(sceneId);
  const mode = trigger.dataset.mode || state.lastWorkbenchMode || scene.defaultMode || 'understand';

  const context =
    trigger.dataset.purpose === 'continue-current' && state.lastWorkbenchContext
      ? state.lastWorkbenchContext
      : buildDefaultWorkbenchContext(scene);

  router.dispatch({
    type: 'OPEN_WORKBENCH',
    sceneId: scene.id,
    mode,
    context
  });
}

function openTasks(trigger) {
  router.dispatch({ type: 'OPEN_VIEW', view: 'tasks' });

  if (trigger.dataset.filter) {
    router.dispatch({ type: 'SET_TASK_FILTER', filterId: trigger.dataset.filter });
  }

  if (trigger.dataset.taskId) {
    router.dispatch({ type: 'SELECT_TASK', taskId: trigger.dataset.taskId });
  }
}

function openMemory(trigger) {
  router.dispatch({ type: 'OPEN_VIEW', view: 'memory' });

  if (trigger.dataset.level) {
    router.dispatch({ type: 'SET_MEMORY_LEVEL', level: trigger.dataset.level });
  }

  if (trigger.dataset.memoryId) {
    router.dispatch({ type: 'SELECT_MEMORY', memoryId: trigger.dataset.memoryId });
  }
}

function executeWorkbenchAction(trigger) {
  const state = router.getState();
  const scene = findWorkbenchScene(state.workbenchSceneId);
  const modePane = scene.modes[state.workbenchMode] || scene.modes[scene.defaultMode];
  const action = modePane.actions.find((item) => item.id === trigger.dataset.actionId);

  if (!action) {
    return;
  }

  if (action.kind === 'bar') {
    router.dispatch({ type: 'OPEN_ACTION_BAR', bar: action.bar });
    return;
  }

  if (action.kind === 'navigate') {
    if (action.targetView === 'tasks') {
      const taskId = state.workbenchContext?.taskId || scene.relatedTaskId || state.selectedTaskId;
      router.dispatch({ type: 'OPEN_VIEW', view: 'tasks' });
      router.dispatch({ type: 'SET_TASK_FILTER', filterId: 'all' });
      if (taskId) {
        router.dispatch({ type: 'SELECT_TASK', taskId });
      }
      router.dispatch({ type: 'SHOW_TOAST', message: '已回到任务控制台。' });
      return;
    }

    if (action.targetView === 'memory') {
      const memoryId = state.workbenchContext?.memoryId || scene.relatedMemoryId || state.selectedMemoryId;
      const memory = memoryId ? findMemory(memoryId) : null;
      router.dispatch({ type: 'OPEN_VIEW', view: 'memory' });
      if (memory?.level) {
        router.dispatch({ type: 'SET_MEMORY_LEVEL', level: memory.level });
      }
      if (memoryId) {
        router.dispatch({ type: 'SELECT_MEMORY', memoryId });
      }
      router.dispatch({ type: 'SHOW_TOAST', message: '已回到镜像面板。' });
    }
  }
}

function buildFollowUpReply(question) {
  const q = question.trim();
  if (!q) {
    return '这句还没有内容。';
  }

  if (q.includes('日报')) {
    return '可以，已换成日报语气：进展 / 阻塞 / 下一步。';
  }

  if (q.includes('简短')) {
    return '可以，再压成一句话摘要。';
  }

  if (q.includes('重要')) {
    return '重要性在于它会决定下一步怎么挂回任务和镜像。';
  }

  if (q.includes('重写')) {
    return '可以，已按这个模式重写成更短的一版。';
  }

  if (q.includes('晨会')) {
    return '可以，已切到晨会语气。';
  }

  return '收到，已按这个补充继续收束。';
}

document.addEventListener('click', (event) => {
  const trigger = event.target.closest('[data-action]');
  if (!trigger) {
    return;
  }

  const action = trigger.dataset.action;

  if (action === 'open-view') {
    if (trigger.dataset.view === 'tasks') {
      openTasks(trigger);
      return;
    }

    if (trigger.dataset.view === 'memory') {
      openMemory(trigger);
      return;
    }

    router.dispatch({ type: 'OPEN_VIEW', view: trigger.dataset.view });
    return;
  }

  if (action === 'open-workbench') {
    openWorkbench(trigger);
    return;
  }

  if (action === 'show-hint-card') {
    router.dispatch({ type: 'SHOW_HINT_CARD', hintId: trigger.dataset.hintId });
    return;
  }

  if (action === 'hide-hint-card') {
    router.dispatch({ type: 'HIDE_HINT_CARD' });
    return;
  }

  if (action === 'dismiss-layer' || action === 'close-layer') {
    router.dispatch({ type: 'CLOSE_LAYER' });
    return;
  }

  if (action === 'toggle-quick-card') {
    router.dispatch({ type: 'TOGGLE_QUICK_CARD' });
    return;
  }

  if (action === 'toggle-orb') {
    router.dispatch({ type: 'TOGGLE_ORB' });
    return;
  }

  if (action === 'select-workbench-scene') {
    router.dispatch({ type: 'SET_WORKBENCH_SCENE', sceneId: trigger.dataset.sceneId });
    return;
  }

  if (action === 'set-workbench-mode') {
    router.dispatch({ type: 'SET_WORKBENCH_MODE', mode: trigger.dataset.mode });
    return;
  }

  if (action === 'workbench-action') {
    executeWorkbenchAction(trigger);
    return;
  }

  if (action === 'confirm-action-bar') {
    router.dispatch({ type: 'CONFIRM_ACTION_BAR' });
    return;
  }

  if (action === 'allow-once-action-bar') {
    router.dispatch({ type: 'ALLOW_ONCE_ACTION_BAR' });
    return;
  }

  if (action === 'cancel-action-bar') {
    router.dispatch({ type: 'CANCEL_ACTION_BAR' });
    return;
  }

  if (action === 'toggle-action-bar-scope') {
    router.dispatch({ type: 'TOGGLE_ACTION_BAR_SCOPE' });
    return;
  }

  if (action === 'set-task-browse') {
    router.dispatch({ type: 'SET_TASK_BROWSE_MODE', mode: trigger.dataset.mode });
    return;
  }

  if (action === 'set-task-filter') {
    router.dispatch({ type: 'SET_TASK_FILTER', filterId: trigger.dataset.filter });
    return;
  }

  if (action === 'select-task') {
    router.dispatch({ type: 'SELECT_TASK', taskId: trigger.dataset.taskId });
    return;
  }

  if (action === 'set-memory-level') {
    router.dispatch({ type: 'SET_MEMORY_LEVEL', level: trigger.dataset.level });
    return;
  }

  if (action === 'select-memory') {
    router.dispatch({ type: 'SELECT_MEMORY', memoryId: trigger.dataset.memoryId });
    return;
  }

  if (action === 'jump-memory') {
    const memoryId = trigger.dataset.memoryId || getPrimaryMemoryForTask(trigger.dataset.taskId)?.id;
    if (!memoryId) {
      router.dispatch({ type: 'SHOW_TOAST', message: '这条任务暂时没有挂接镜像。' });
      return;
    }

    const memory = findMemory(memoryId);
    if (!memory) {
      router.dispatch({ type: 'SHOW_TOAST', message: '相关镜像没有找到。' });
      return;
    }

    router.dispatch({ type: 'OPEN_MEMORY_FROM_TASK', memoryId, level: memory.level });
    return;
  }

  if (action === 'jump-task') {
    const taskId = trigger.dataset.taskId;
    if (!taskId) {
      return;
    }
    router.dispatch({ type: 'OPEN_TASK_FROM_MEMORY', taskId });
    return;
  }

  if (action === 'correct-memory') {
    const memoryId = trigger.dataset.memoryId;
    router.dispatch({ type: 'TOGGLE_MEMORY_CORRECTED', memoryId });
    const nextState = router.getState();
    const corrected = nextState.correctedMemoryIds.includes(memoryId);
    router.dispatch({ type: 'SHOW_TOAST', message: corrected ? '已标记为校正完成。' : '已撤回校正标记。' });
    return;
  }

  if (action === 'close-memory') {
    const memoryId = trigger.dataset.memoryId;
    router.dispatch({ type: 'TOGGLE_MEMORY_HIDDEN', memoryId });
    const nextState = router.getState();
    const hidden = nextState.hiddenMemoryIds.includes(memoryId);
    router.dispatch({ type: 'SHOW_TOAST', message: hidden ? '这条镜像已关闭，可在记忆管理里找回。' : '这条镜像已重新打开。' });
    return;
  }

  if (action === 'use-followup-chip') {
    const question = trigger.dataset.value || '';
    router.dispatch({
      type: 'SUBMIT_WORKBENCH_FOLLOW_UP',
      question,
      reply: buildFollowUpReply(question)
    });
    return;
  }
});

document.addEventListener('submit', (event) => {
  const form = event.target.closest('[data-action="submit-followup-form"]');
  if (!form) {
    return;
  }

  event.preventDefault();

  const input = form.querySelector('[data-role="workbench-followup-input"]');
  const question = input?.value.trim() || '';

  if (!question) {
    router.dispatch({ type: 'SHOW_TOAST', message: '先补一句追问。' });
    return;
  }

  router.dispatch({
    type: 'SUBMIT_WORKBENCH_FOLLOW_UP',
    question,
    reply: buildFollowUpReply(question)
  });
});

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') {
    router.dispatch({ type: 'CLOSE_LAYER' });
  }
});

router.subscribe(renderApp);
renderApp(router.getState());
