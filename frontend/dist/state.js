import { settingsSeed, todoSeed, workspaceConversations, statusSnapshots } from './data.js';

export function createStore(initialState) {
  let state = structuredClone(initialState);
  const listeners = new Set();

  function getState() {
    return state;
  }

  function setState(next) {
    state = next;
    listeners.forEach((listener) => {
      listener(state);
    });
  }

  function update(mutator) {
    const draft = structuredClone(state);
    mutator(draft);
    setState(draft);
  }

  function subscribe(listener) {
    listeners.add(listener);
    listener(state);
    return () => listeners.delete(listener);
  }

  return { getState, setState, update, subscribe };
}

export function createInitialState() {
  return {
    ui: {
      ball: {
        visible: true,
        mode: 'default',
        hovered: false,
        contextMenuOpen: false,
        hoverLatched: false
      },
      quickChat: {
        open: false,
        user: '帮我提炼这段内容的重点。',
        agent: '我先压成 3 个重点：目标、变化、下一步。',
        lastIntent: '提炼重点',
        dragHint: '把材料拖到悬浮球'
      },
      confirm: {
        open: false,
        materials: [],
        guessedIntent: '提炼重点',
        customIntent: '',
        sourceType: 'selection'
      },
      result: {
        open: false,
        payload: null
      },
      workspace: {
        open: false,
        mode: 'conversation',
        menuOpen: false,
        activeConversationId: workspaceConversations[0].id,
        draft: '',
        historyFilter: 'all',
        selectedStatus: 'idle'
      },
      dev: {
        open: false
      }
    },
    data: {
      conversations: structuredClone(workspaceConversations),
      todos: structuredClone(todoSeed),
      settings: structuredClone(settingsSeed),
      status: 'idle',
      statusSnapshots: structuredClone(statusSnapshots),
      memoryNotes: [
        '先提示，再确认。',
        '轻提示和小窗优先。',
        '长期记忆是协作能力的一部分。'
      ],
      recentActions: [
        '默认只显示悬浮球',
        '可协作触发会变成感叹号',
        '拖入材料会变成加号'
      ]
    }
  };
}
