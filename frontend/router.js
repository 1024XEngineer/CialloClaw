export const initialState = {
  activeView: 'desktop',
  quickCardOpen: false,
  orbExpanded: false,
  taskBrowseMode: 'status',
  taskFilter: 'all',
  memoryLevel: 'recent',
  selectedTaskId: 'task-ux-route',
  selectedMemoryId: 'mem-desk-entry',
  hiddenMemoryIds: [],
  correctedMemoryIds: [],
  highlightTaskId: '',
  highlightMemoryId: '',
  toast: null,
  hintCardId: '',
  workbenchSceneId: 'doc-digest',
  workbenchMode: 'understand',
  workbenchContext: null,
  lastWorkbenchSceneId: 'doc-digest',
  lastWorkbenchMode: 'understand',
  lastWorkbenchContext: null,
  workbenchOutcome: null,
  followUpDraft: '',
  followUpLog: [],
  actionBar: null
};

function clearTransientLayers(state) {
  return {
    ...state,
    quickCardOpen: false,
    orbExpanded: false,
    hintCardId: '',
    actionBar: null
  };
}

function setWorkbenchOutcome(state, outcome, toastMessage) {
  return {
    ...state,
    workbenchOutcome: outcome,
    actionBar: null,
    toast: toastMessage
      ? {
          id: Date.now(),
          message: toastMessage
        }
      : state.toast
  };
}

function reducer(state, action) {
  switch (action.type) {
    case 'OPEN_VIEW':
      return {
        ...clearTransientLayers(state),
        activeView: action.view
      };
    case 'CLOSE_LAYER':
      return {
        ...state,
        activeView: 'desktop',
        quickCardOpen: false,
        orbExpanded: false,
        hintCardId: '',
        actionBar: null,
        workbenchContext: null,
        workbenchOutcome: null,
        followUpDraft: '',
        followUpLog: []
      };
    case 'TOGGLE_QUICK_CARD':
      return {
        ...state,
        quickCardOpen: !state.quickCardOpen,
        orbExpanded: false,
        hintCardId: '',
        actionBar: null
      };
    case 'TOGGLE_ORB':
      return {
        ...state,
        orbExpanded: !state.orbExpanded,
        quickCardOpen: false,
        hintCardId: '',
        actionBar: null
      };
    case 'SHOW_HINT_CARD':
      return {
        ...state,
        quickCardOpen: false,
        orbExpanded: false,
        hintCardId: action.hintId,
        actionBar: null
      };
    case 'HIDE_HINT_CARD':
      return {
        ...state,
        hintCardId: ''
      };
    case 'OPEN_WORKBENCH':
      return {
        ...clearTransientLayers(state),
        activeView: 'workbench',
        workbenchSceneId: action.sceneId,
        workbenchMode: action.mode,
        workbenchContext: action.context,
        lastWorkbenchSceneId: action.sceneId,
        lastWorkbenchMode: action.mode,
        lastWorkbenchContext: action.context,
        workbenchOutcome: null,
        followUpDraft: '',
        followUpLog: []
      };
    case 'SET_WORKBENCH_SCENE':
      return {
        ...state,
        workbenchSceneId: action.sceneId,
        lastWorkbenchSceneId: action.sceneId,
        actionBar: null,
        workbenchOutcome: null,
        followUpDraft: '',
        followUpLog: []
      };
    case 'SET_WORKBENCH_MODE':
      return {
        ...state,
        workbenchMode: action.mode,
        lastWorkbenchMode: action.mode,
        actionBar: null,
        workbenchOutcome: null
      };
    case 'SET_WORKBENCH_DRAFT':
      return {
        ...state,
        followUpDraft: action.value
      };
    case 'SUBMIT_WORKBENCH_FOLLOW_UP': {
      const nextEntry = {
        id: Date.now(),
        question: action.question,
        reply: action.reply
      };

      return {
        ...state,
        followUpDraft: '',
        followUpLog: [...state.followUpLog, nextEntry].slice(-3)
      };
    }
    case 'OPEN_ACTION_BAR':
      return {
        ...state,
        quickCardOpen: false,
        orbExpanded: false,
        hintCardId: '',
        workbenchOutcome: null,
        actionBar: {
          ...action.bar,
          id: Date.now(),
          expanded: false
        }
      };
    case 'TOGGLE_ACTION_BAR_SCOPE':
      if (!state.actionBar) {
        return state;
      }
      return {
        ...state,
        actionBar: {
          ...state.actionBar,
          expanded: !state.actionBar.expanded
        }
      };
    case 'CANCEL_ACTION_BAR':
      return {
        ...state,
        actionBar: null
      };
    case 'CONFIRM_ACTION_BAR':
      if (!state.actionBar) {
        return state;
      }
      return setWorkbenchOutcome(
        state,
        {
          kind: 'confirmed',
          title: state.actionBar.title,
          detail: state.actionBar.resultNote || state.actionBar.title
        },
        state.actionBar.toastMessage || state.actionBar.title
      );
    case 'ALLOW_ONCE_ACTION_BAR':
      if (!state.actionBar) {
        return state;
      }
      return setWorkbenchOutcome(
        state,
        {
          kind: 'allow-once',
          title: state.actionBar.title,
          detail: state.actionBar.resultNote || state.actionBar.title
        },
        `仅本次允许：${state.actionBar.title}`
      );
    case 'SELECT_TASK':
      return {
        ...state,
        selectedTaskId: action.taskId
      };
    case 'SELECT_MEMORY':
      return {
        ...state,
        selectedMemoryId: action.memoryId
      };
    case 'SET_TASK_BROWSE_MODE':
      return {
        ...state,
        taskBrowseMode: action.mode
      };
    case 'SET_TASK_FILTER':
      return {
        ...state,
        taskFilter: action.filterId
      };
    case 'SET_MEMORY_LEVEL':
      return {
        ...state,
        memoryLevel: action.level
      };
    case 'OPEN_TASK_FROM_MEMORY':
      return {
        ...clearTransientLayers(state),
        activeView: 'tasks',
        selectedTaskId: action.taskId,
        highlightTaskId: action.taskId
      };
    case 'OPEN_MEMORY_FROM_TASK':
      return {
        ...clearTransientLayers(state),
        activeView: 'memory',
        memoryLevel: action.level,
        selectedMemoryId: action.memoryId,
        highlightMemoryId: action.memoryId
      };
    case 'TOGGLE_MEMORY_HIDDEN': {
      const exists = state.hiddenMemoryIds.includes(action.memoryId);
      return {
        ...state,
        hiddenMemoryIds: exists
          ? state.hiddenMemoryIds.filter((id) => id !== action.memoryId)
          : [...state.hiddenMemoryIds, action.memoryId]
      };
    }
    case 'TOGGLE_MEMORY_CORRECTED': {
      const exists = state.correctedMemoryIds.includes(action.memoryId);
      return {
        ...state,
        correctedMemoryIds: exists
          ? state.correctedMemoryIds.filter((id) => id !== action.memoryId)
          : [...state.correctedMemoryIds, action.memoryId]
      };
    }
    case 'SHOW_TOAST':
      return {
        ...state,
        toast: {
          id: Date.now(),
          message: action.message
        }
      };
    case 'CLEAR_TOAST':
      if (!state.toast || state.toast.id !== action.toastId) {
        return state;
      }
      return {
        ...state,
        toast: null
      };
    case 'CLEAR_HIGHLIGHTS':
      return {
        ...state,
        highlightTaskId: '',
        highlightMemoryId: ''
      };
    default:
      return state;
  }
}

export function createRouter(seedState = initialState) {
  let currentState = { ...seedState };
  const listeners = new Set();

  return {
    getState() {
      return currentState;
    },
    dispatch(action) {
      currentState = reducer(currentState, action);
      listeners.forEach((listener) => {
        listener(currentState, action);
      });
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    }
  };
}
