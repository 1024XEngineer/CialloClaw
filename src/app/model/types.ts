export type PrototypeStatus =
  | 'idle'
  | 'nearby'
  | 'hover'
  | 'unsupported-nearby'
  | 'unsupported-hover'
  | 'recognized'
  | 'actions'
  | 'processing'
  | 'result'
  | 'detail'
  | 'unsupported'
  | 'error';

export type SampleKind = 'pdf' | 'image' | 'text' | 'link' | 'unsupported';

export interface SampleDefinition {
  id: string;
  label: string;
  kind: SampleKind;
  meta: string;
  preview?: string;
}

export interface ActionDefinition {
  id: string;
  label: string;
}

export interface ResultDefinition {
  title: string;
  body: string;
  actions: { label: string; id: string }[];
}

export interface GalleryItem {
  id: string;
  title: string;
  description: string;
}

export interface PrototypeState {
  status: PrototypeStatus;
  railMode: 'gallery' | 'detail';
  activeSample: SampleDefinition;
  suggestedActions: ActionDefinition[];
  activeActionId: string | null;
  activeResult: ResultDefinition | null;
  recognitionLabel: string | null;
  recognitionSummary: string | null;
  errorReasonVisible: boolean;
  highlightedGalleryState: string;
  dragGhost: {
    sourceId: string;
    sourceType: 'tray' | 'stage';
    x: number;
    y: number;
    snapped: boolean;
  } | null;
}

export type PrototypeAction =
  | { type: 'selectSample'; sampleId: string }
  | { type: 'startDrag'; sourceId: string; sourceType: 'tray' | 'stage'; x: number; y: number }
  | { type: 'moveDrag'; x: number; y: number }
  | { type: 'enterOrbRange' }
  | { type: 'hoverOrb' }
  | { type: 'leaveOrbRange' }
  | { type: 'dropOnOrb' }
  | { type: 'finishRecognition' }
  | { type: 'chooseAction'; actionId: string }
  | { type: 'finishProcessing' }
  | { type: 'continueFromResult' }
  | { type: 'openDetail' }
  | { type: 'closeDetail' }
  | { type: 'triggerErrorPreview' }
  | { type: 'retryAfterError' }
  | { type: 'viewErrorReason' }
  | { type: 'resetScene' };
