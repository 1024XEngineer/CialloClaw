export type SurfaceState =
  | 'idle'
  | 'quick-menu'
  | 'selecting'
  | 'selected'
  | 'processing'
  | 'result'

export type ActionKind =
  | 'analyze-content'
  | 'recognize-text'
  | 'explain-chart'
  | 'analyze-error'
  | 'summarize-page'
  | 'generate-reply'

export type ContentHint = 'content' | 'error' | 'text' | 'chart' | 'page' | 'reply'

export type SelectionRect = {
  x: number
  y: number
  width: number
  height: number
}

export type ResultPayload = {
  title: string
  summary: string[]
  detail: string[]
}

export type BackgroundTaskStatus = 'running' | 'complete'

export type ProcessingStage = 'Capturing area' | 'Understanding structure' | 'Preparing result'

export type BackgroundTask = {
  id: string
  actionKind: ActionKind
  status: BackgroundTaskStatus
  selection: SelectionRect | null
  startedAt: string
  completedAt: string | null
}

export type NoticeTone = 'info' | 'success' | 'warning'

export type NoticeSource = 'quick-menu' | 'selection' | 'processing' | 'background-task' | 'result'

export type PrototypeNotice = {
  id: string
  tone: NoticeTone
  message: string
  source: NoticeSource
}

export type PrototypeState = {
  surface: SurfaceState
  selection: SelectionRect | null
  contentHint: ContentHint | null
  activeAction: ActionKind | null
  processingStage: ProcessingStage | null
  suggestedAction: ActionKind | null
  quickMenuOpen: boolean
  sidePanelOpen: boolean
  backgroundTask: BackgroundTask | null
  orbTaskChip: string | null
  result: ResultPayload | null
  notice: PrototypeNotice | null
  derivedFrom: ActionKind | null
  lastCompletedAt: string | null
}

export type PrototypeAction =
  | { type: 'openQuickMenu' }
  | { type: 'startSelection' }
  | { type: 'startSelectionWithSuggestedAction'; actionKind: ActionKind }
  | { type: 'openSidePanel' }
  | { type: 'openRecentResult' }
  | { type: 'cancelSelection' }
  | { type: 'rejectSelection'; message: string }
  | { type: 'completeSelection'; selection: SelectionRect; hint: ContentHint | null }
  | { type: 'captureFullScreen' }
  | { type: 'startProcessing'; actionKind: ActionKind }
  | { type: 'setProcessingStage'; stage: ProcessingStage }
  | { type: 'cancelProcessing' }
  | { type: 'runInBackground' }
  | { type: 'backgroundTaskComplete' }
  | { type: 'resumeCurrentTask' }
  | { type: 'processingComplete'; actionKind: ActionKind }
