import { PrototypeState, PrototypeAction, SampleDefinition } from './types';
import { samples, actionMap, recognitionMap, resultMap } from './mockData';

function getInitialActions(sample: SampleDefinition): any[] {
  return sample.kind === 'unsupported' ? [] : actionMap[sample.kind] || [];
}

export function createInitialState(sample = samples['product-pdf']): PrototypeState {
  return {
    status: 'idle',
    activeSample: sample,
    suggestedActions: getInitialActions(sample),
    activeActionId: null,
    activeResult: null,
    recognitionLabel: null,
    recognitionSummary: null,
    errorReasonVisible: false,
    railMode: 'gallery',
    highlightedGalleryState: 'gallery-item-idle',
    dragGhost: null
  };
}

export function prototypeReducer(state: PrototypeState, action: PrototypeAction): PrototypeState {
  switch (action.type) {
    case 'selectSample':
      return createInitialState(samples[action.sampleId]);
    case 'startDrag':
      return {
        ...state,
        dragGhost: {
          sourceId: action.sourceId,
          sourceType: action.sourceType,
          x: action.x,
          y: action.y,
          snapped: false
        }
      };
    case 'moveDrag':
      return state.dragGhost
        ? { ...state, dragGhost: { ...state.dragGhost, x: action.x, y: action.y } }
        : state;
    case 'enterOrbRange':
      return state.activeSample.kind === 'unsupported'
        ? { ...state, status: 'unsupported-nearby', highlightedGalleryState: 'gallery-item-nearby' }
        : { ...state, status: 'nearby', highlightedGalleryState: 'gallery-item-nearby' };
    case 'hoverOrb':
      return state.activeSample.kind === 'unsupported'
        ? {
            ...state,
            status: 'unsupported-hover',
            highlightedGalleryState: 'gallery-item-hover',
            dragGhost: state.dragGhost ? { ...state.dragGhost, snapped: true } : null
          }
        : {
            ...state,
            status: 'hover',
            highlightedGalleryState: 'gallery-item-hover',
            dragGhost: state.dragGhost ? { ...state.dragGhost, snapped: true } : null
          };
    case 'leaveOrbRange':
      return { ...state, status: 'idle', dragGhost: null, highlightedGalleryState: 'gallery-item-idle' };
    case 'dropOnOrb':
      if (state.activeSample.kind === 'unsupported') {
        return {
          ...state,
          status: 'unsupported',
          suggestedActions: [],
          highlightedGalleryState: 'gallery-item-error',
          dragGhost: null
        };
      }
      const recMap = recognitionMap[state.activeSample.id];
      return {
        ...state,
        status: 'recognized',
        highlightedGalleryState: 'gallery-item-recognized',
        recognitionLabel: recMap?.title || null,
        recognitionSummary: recMap?.summary || null,
        dragGhost: null
      };
    case 'finishRecognition':
      return {
        ...state,
        status: 'actions',
        suggestedActions: actionMap[state.activeSample.kind] || [],
        highlightedGalleryState: 'gallery-item-actions'
      };
    case 'chooseAction':
      return {
        ...state,
        status: 'processing',
        activeActionId: action.actionId,
        highlightedGalleryState: 'gallery-item-processing'
      };
    case 'finishProcessing': {
      const results = resultMap[state.activeSample.id];
      const result = results?.[state.activeActionId!];
      return {
        ...state,
        status: 'result',
        activeResult: result || null,
        highlightedGalleryState: 'gallery-item-result'
      };
    }
    case 'continueFromResult':
      return {
        ...state,
        status: 'actions',
        activeActionId: null,
        highlightedGalleryState: 'gallery-item-actions'
      };
    case 'openDetail':
      return { ...state, status: 'detail', railMode: 'detail', highlightedGalleryState: 'gallery-item-detail' };
    case 'closeDetail':
      return { ...state, status: 'result', railMode: 'gallery', highlightedGalleryState: 'gallery-item-result' };
    case 'triggerErrorPreview':
      return {
        ...state,
        status: 'error',
        railMode: 'gallery',
        errorReasonVisible: false,
        highlightedGalleryState: 'gallery-item-error'
      };
    case 'retryAfterError':
      return {
        ...state,
        status: 'actions',
        errorReasonVisible: false,
        highlightedGalleryState: 'gallery-item-actions'
      };
    case 'viewErrorReason':
      return { ...state, errorReasonVisible: true };
    case 'resetScene':
      return createInitialState(state.activeSample);
    default:
      return state;
  }
}
