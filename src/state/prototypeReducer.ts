import { mockResults } from '../data/mockResults'
import type {
  ActionKind,
  ContentHint,
  ProcessingStage,
  SelectionRect,
  PrototypeAction,
  PrototypeState,
} from '../types/prototype'

export const MIN_SELECTION_SIZE = 48

export const FULL_SCREEN_SELECTION: SelectionRect = {
  x: 24,
  y: 24,
  width: 1392,
  height: 852,
}

export const processingStages: readonly ProcessingStage[] = [
  'Capturing area',
  'Understanding structure',
  'Preparing result',
] as const

function getSuggestedAction(hint: ContentHint | null): ActionKind | null {
  switch (hint) {
    case 'error':
      return 'analyze-error'
    case 'text':
      return 'recognize-text'
    case 'chart':
      return 'explain-chart'
    case 'page':
      return 'summarize-page'
    case 'reply':
      return 'generate-reply'
    case 'content':
      return 'analyze-content'
    default:
      return null
  }
}

export function getContentHint(selection: Pick<SelectionRect, 'width' | 'height'>): ContentHint {
  if (selection.width > 560 && selection.height > 300) {
    return 'chart'
  }

  if (selection.height < 180) {
    return 'text'
  }

  return 'error'
}

export function isValidSelection(selection: SelectionRect) {
  return selection.width >= MIN_SELECTION_SIZE && selection.height >= MIN_SELECTION_SIZE
}

export function isFullScreenSelection(selection: SelectionRect | null) {
  if (selection === null) {
    return false
  }

  return (
    selection.x === FULL_SCREEN_SELECTION.x &&
    selection.y === FULL_SCREEN_SELECTION.y &&
    selection.width === FULL_SCREEN_SELECTION.width &&
    selection.height === FULL_SCREEN_SELECTION.height
  )
}

function clearSelectionNotice(state: PrototypeState) {
  if (state.notice?.source === 'selection') {
    return null
  }

  return state.notice
}

function getNextSuggestedAction(state: PrototypeState, hint: ContentHint | null) {
  return state.suggestedAction ?? getSuggestedAction(hint)
}

function getResultPayload(actionKind: ActionKind) {
  const result = mockResults[actionKind]

  if (actionKind === 'analyze-error') {
    return {
      ...result,
      title: `Error analysis: ${result.title}`,
    }
  }

  return result
}

export const initialState: PrototypeState = {
  surface: 'idle',
  selection: null,
  contentHint: null,
  activeAction: null,
  processingStage: null,
  suggestedAction: null,
  quickMenuOpen: false,
  sidePanelOpen: false,
  backgroundTask: null,
  orbTaskChip: null,
  result: null,
  notice: null,
  derivedFrom: null,
  lastCompletedAt: null,
}

export function reducer(state: PrototypeState, action: PrototypeAction): PrototypeState {
  switch (action.type) {
    case 'openQuickMenu':
      return { ...state, surface: 'quick-menu', quickMenuOpen: true }
    case 'startSelection':
      return {
        ...state,
        surface: 'selecting',
        selection: null,
        contentHint: null,
        activeAction: null,
        processingStage: null,
        notice: clearSelectionNotice(state),
        quickMenuOpen: false,
        sidePanelOpen: false,
      }
    case 'startSelectionWithSuggestedAction':
      return {
        ...state,
        surface: 'selecting',
        selection: null,
        contentHint: null,
        activeAction: null,
        processingStage: null,
        notice: clearSelectionNotice(state),
        quickMenuOpen: false,
        sidePanelOpen: false,
        suggestedAction: action.actionKind,
      }
    case 'openSidePanel':
      return {
        ...state,
        surface: 'idle',
        quickMenuOpen: false,
        sidePanelOpen: true,
        orbTaskChip: state.backgroundTask?.status === 'running' ? state.orbTaskChip : null,
      }
    case 'openRecentResult':
      if (state.result === null) {
        return { ...state, quickMenuOpen: false }
      }

      return {
        ...state,
        surface: 'result',
        quickMenuOpen: false,
        sidePanelOpen: true,
        orbTaskChip: state.backgroundTask?.status === 'running' ? state.orbTaskChip : null,
      }
    case 'cancelSelection':
      return {
        ...state,
        surface: 'idle',
        selection: null,
        contentHint: null,
        activeAction: null,
        processingStage: null,
        suggestedAction: null,
        notice: clearSelectionNotice(state),
      }
    case 'rejectSelection':
      return {
        ...state,
        surface: 'selecting',
        notice: {
          id: 'selection-too-small',
          tone: 'warning',
          message: action.message,
          source: 'selection',
        },
      }
    case 'completeSelection':
      return {
        ...state,
        surface: 'selected',
        selection: action.selection,
        contentHint: action.hint,
        activeAction: null,
        processingStage: null,
        notice: clearSelectionNotice(state),
        suggestedAction: getNextSuggestedAction(state, action.hint),
      }
    case 'captureFullScreen': {
      return {
        ...state,
        surface: 'selected',
        selection: FULL_SCREEN_SELECTION,
        contentHint: getContentHint(FULL_SCREEN_SELECTION),
        activeAction: null,
        processingStage: null,
        notice: clearSelectionNotice(state),
        suggestedAction: getNextSuggestedAction(state, getContentHint(FULL_SCREEN_SELECTION)),
      }
    }
    case 'startProcessing':
      return {
        ...state,
        surface: 'processing',
        activeAction: action.actionKind,
        processingStage: processingStages[0],
        quickMenuOpen: false,
        sidePanelOpen: false,
      }
    case 'setProcessingStage':
      return {
        ...state,
        processingStage: action.stage,
      }
    case 'cancelProcessing':
      return {
        ...state,
        surface: 'selected',
        activeAction: null,
        processingStage: null,
      }
    case 'runInBackground':
      if (state.activeAction === null) {
        return state
      }

      return {
        ...state,
        surface: 'idle',
        processingStage: null,
        quickMenuOpen: false,
        backgroundTask: {
          id: 'task-1',
          actionKind: state.activeAction,
          status: 'running',
          selection: state.selection,
          startedAt: 'mock-start',
          completedAt: null,
        },
        orbTaskChip: '1 task running',
      }
    case 'backgroundTaskComplete':
      if (state.backgroundTask === null) {
        return state
      }

      return {
        ...state,
        backgroundTask: {
          ...state.backgroundTask,
          status: 'complete',
          completedAt: 'mock-complete',
        },
        result: getResultPayload(state.backgroundTask.actionKind),
        derivedFrom: state.backgroundTask.actionKind,
        lastCompletedAt: 'mock-complete',
        orbTaskChip: null,
      }
    case 'resumeCurrentTask':
      if (state.backgroundTask === null) {
        return state
      }

      if (state.backgroundTask.selection) {
        return {
          ...state,
          surface: 'result',
          quickMenuOpen: false,
          selection: state.backgroundTask.selection,
          sidePanelOpen: false,
          result: getResultPayload(state.backgroundTask.actionKind),
          derivedFrom: state.backgroundTask.actionKind,
          lastCompletedAt: state.backgroundTask.completedAt ?? state.lastCompletedAt,
        }
      }

      return {
        ...state,
        surface: 'result',
        quickMenuOpen: false,
        sidePanelOpen: true,
        result: getResultPayload(state.backgroundTask.actionKind),
        derivedFrom: state.backgroundTask.actionKind,
        lastCompletedAt: state.backgroundTask.completedAt ?? state.lastCompletedAt,
      }
    case 'processingComplete':
      return {
        ...state,
        surface: 'result',
        quickMenuOpen: false,
        activeAction: action.actionKind,
        processingStage: null,
        suggestedAction: action.actionKind,
        result: getResultPayload(action.actionKind),
        derivedFrom: action.actionKind,
        lastCompletedAt: 'mock-complete',
      }
    default:
      return state
  }
}
