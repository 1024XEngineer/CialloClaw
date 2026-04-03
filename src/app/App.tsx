import {
  useCallback,
  useEffect,
  useReducer,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from 'react'

import { CaptureOverlay } from '../components/capture/CaptureOverlay'
import { DesktopScene } from '../components/desktop/DesktopScene'
import { ProcessingCard } from '../components/feedback/ProcessingCard'
import { ActionMenu } from '../components/menu/ActionMenu'
import type { FloatingOrbPosition } from '../components/orb/FloatingOrb'
import { useMockProcessing } from '../hooks/useMockProcessing'
import {
  getContentHint,
  initialState,
  isFullScreenSelection,
  isValidSelection,
  reducer,
} from '../state/prototypeReducer'
import type { PrototypeState, SelectionRect } from '../types/prototype'
import styles from './App.module.css'

function getNormalizedSelection(startX: number, startY: number, currentX: number, currentY: number): SelectionRect {
  const left = Math.min(startX, currentX)
  const top = Math.min(startY, currentY)
  const right = Math.max(startX, currentX)
  const bottom = Math.max(startY, currentY)

  return {
    x: left,
    y: top,
    width: right - left,
    height: bottom - top,
  }
}

const defaultOrbPosition: FloatingOrbPosition = {
  edge: 'right',
  top: 540,
}

type AppProps = {
  initialState?: PrototypeState
}

export default function App({ initialState: seedState = initialState }: AppProps) {
  const [state, dispatch] = useReducer(reducer, seedState)
  const [orbPosition, setOrbPosition] = useState<FloatingOrbPosition>(defaultOrbPosition)
  const [dragStart, setDragStart] = useState<[number, number] | null>(null)
  const [draftSelection, setDraftSelection] = useState<SelectionRect | null>(null)
  const sceneFrameRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (state.surface !== 'selecting') {
      setDragStart(null)
      setDraftSelection(null)
      return undefined
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        dispatch({ type: 'cancelSelection' })
        return
      }

      if (event.key !== ' ') {
        return
      }

      event.preventDefault()
      setDragStart(null)
      setDraftSelection(null)
      dispatch({ type: 'captureFullScreen' })
    }

    window.addEventListener('keydown', handleKeyDown)

    return () => {
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [state.surface])

  function handlePointerDownCapture(event: ReactPointerEvent<HTMLElement>) {
    if (state.surface !== 'selecting') {
      return
    }

    const target = event.target as HTMLElement

    if (target.closest('[data-testid="desktop-scene"]') === null) {
      return
    }

    const bounds = sceneFrameRef.current?.getBoundingClientRect()

    if (!bounds) {
      return
    }

    if (state.notice?.source === 'selection') {
      dispatch({ type: 'startSelection' })
    }

    setDragStart([event.clientX - bounds.left, event.clientY - bounds.top])
    setDraftSelection({ x: event.clientX - bounds.left, y: event.clientY - bounds.top, width: 0, height: 0 })
  }

  function handlePointerMoveCapture(event: ReactPointerEvent<HTMLElement>) {
    if (state.surface !== 'selecting' || dragStart === null) {
      return
    }

    const bounds = sceneFrameRef.current?.getBoundingClientRect()

    if (!bounds) {
      return
    }

    setDraftSelection(
      getNormalizedSelection(dragStart[0], dragStart[1], event.clientX - bounds.left, event.clientY - bounds.top),
    )
  }

  function handlePointerUpCapture(event: ReactPointerEvent<HTMLElement>) {
    if (state.surface !== 'selecting' || dragStart === null) {
      return
    }

    const bounds = sceneFrameRef.current?.getBoundingClientRect()

    if (!bounds) {
      return
    }

    const selection = getNormalizedSelection(
      dragStart[0],
      dragStart[1],
      event.clientX - bounds.left,
      event.clientY - bounds.top,
    )

    setDragStart(null)

    if (!isValidSelection(selection)) {
      setDraftSelection(selection)
      dispatch({ type: 'rejectSelection', message: 'Selection too small' })
      return
    }

    dispatch({
      type: 'completeSelection',
      selection,
      hint: getContentHint(selection),
    })
  }

  function handleQuickMenuAction(id: string) {
    if (id === 'capture-text') {
      dispatch({ type: 'startSelectionWithSuggestedAction', actionKind: 'recognize-text' })
    }

    if (id === 'resume-current-task') {
      dispatch({ type: 'resumeCurrentTask' })
    }

    if (id === 'open-side-panel') {
      dispatch({ type: 'openSidePanel' })
    }

    if (id === 'recent-result') {
      dispatch({ type: 'openRecentResult' })
    }

    if (id === 'region-analyze') {
      dispatch({ type: 'startSelection' })
    }
  }

  const handleProcessingComplete = useCallback(() => {
    if (state.backgroundTask?.status === 'running') {
      dispatch({ type: 'backgroundTaskComplete' })
      return
    }

    if (state.surface === 'processing' && state.activeAction) {
      dispatch({ type: 'processingComplete', actionKind: state.activeAction })
    }
  }, [state.activeAction, state.backgroundTask, state.surface])

  const handleProcessingStageChange = useCallback((stage: PrototypeState['processingStage']) => {
    if (stage) {
      dispatch({ type: 'setProcessingStage', stage })
    }
  }, [])

  useMockProcessing(
    state.surface === 'processing' || state.backgroundTask?.status === 'running',
    handleProcessingStageChange,
    handleProcessingComplete,
  )

  const showCaptureOverlay = state.surface === 'selecting'
  const selection = draftSelection
  const contentHint = selection ? getContentHint(selection) : null
  const activeSelection = draftSelection ?? state.selection

  return (
    <main
      className={styles.app}
      onPointerDownCapture={handlePointerDownCapture}
      onPointerMoveCapture={handlePointerMoveCapture}
      onPointerUpCapture={handlePointerUpCapture}
    >
      <span data-testid="surface-state">{state.surface}</span>
      <span data-testid="suggested-action">{state.suggestedAction ?? 'none'}</span>
      <div className={styles.sceneFrame} ref={sceneFrameRef}>
        <DesktopScene
          onOpenQuickMenu={() => dispatch({ type: 'openQuickMenu' })}
          onOrbDragEnd={setOrbPosition}
          onQuickMenuAction={handleQuickMenuAction}
          orbPosition={orbPosition}
          state={state}
        />
        {showCaptureOverlay ? (
          <CaptureOverlay
            fullScreen={isFullScreenSelection(selection)}
            hint={contentHint}
            selection={selection}
            warning={state.notice?.source === 'selection' ? state.notice.message : null}
          />
        ) : null}
        {state.surface === 'selected' && state.selection ? (
          <ActionMenu
            selection={state.selection}
            onAction={(actionKind) => dispatch({ type: 'startProcessing', actionKind })}
            onPinToSidePanel={() => dispatch({ type: 'openSidePanel' })}
            onReselect={() => dispatch({ type: 'startSelection' })}
          />
        ) : null}
        {state.surface === 'processing' && activeSelection && state.processingStage ? (
          <ProcessingCard
            currentStage={state.processingStage}
            selection={activeSelection}
            onCancel={() => dispatch({ type: 'cancelProcessing' })}
            onRunInBackground={() => dispatch({ type: 'runInBackground' })}
          />
        ) : null}
      </div>
    </main>
  )
}
