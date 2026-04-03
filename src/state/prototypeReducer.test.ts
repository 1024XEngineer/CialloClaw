import { describe, expect, it } from 'vitest'

import { initialState, reducer } from './prototypeReducer'

describe('prototypeReducer', () => {
  it('moves from quick menu to selecting to selected', () => {
    const menuState = reducer(initialState, { type: 'openQuickMenu' })
    const selectingState = reducer(menuState, { type: 'startSelection' })
    const selectedState = reducer(selectingState, {
      type: 'completeSelection',
      selection: { x: 180, y: 120, width: 640, height: 420 },
      hint: 'error',
    })

    expect(selectingState.surface).toBe('selecting')
    expect(selectedState.surface).toBe('selected')
  })

  it('preserves a seeded suggested action when selection is confirmed', () => {
    const selectingState = reducer(initialState, {
      type: 'startSelectionWithSuggestedAction',
      actionKind: 'recognize-text',
    })
    const selectedState = reducer(selectingState, {
      type: 'completeSelection',
      selection: { x: 180, y: 120, width: 640, height: 420 },
      hint: 'chart',
    })

    expect(selectedState.surface).toBe('selected')
    expect(selectedState.contentHint).toBe('chart')
    expect(selectedState.suggestedAction).toBe('recognize-text')
  })

  it('moves from processing to result using local mock payloads', () => {
    const state = reducer(initialState, {
      type: 'processingComplete',
      actionKind: 'analyze-error',
    })

    expect(state.surface).toBe('result')
    expect(state.result?.title).toMatch(/error/i)
  })

  it('captures the full screen into the selected state', () => {
    const state = reducer(initialState, { type: 'captureFullScreen' })

    expect(state.surface).toBe('selected')
    expect(state.selection).toEqual({ x: 24, y: 24, width: 1392, height: 852 })
    expect(state.contentHint).toBe('chart')
  })

  it('keeps capture mode active and sets a selection notice for tiny selections', () => {
    const state = reducer(initialState, { type: 'rejectSelection', message: 'Selection too small' })

    expect(state.surface).toBe('selecting')
    expect(state.notice).toEqual({
      id: 'selection-too-small',
      tone: 'warning',
      message: 'Selection too small',
      source: 'selection',
    })
  })

  it('cancels capture mode back to idle', () => {
    const state = reducer(
      {
        ...initialState,
        surface: 'selecting',
        notice: {
          id: 'selection-too-small',
          tone: 'warning',
          message: 'Selection too small',
          source: 'selection',
        },
        suggestedAction: 'recognize-text',
      },
      { type: 'cancelSelection' },
    )

    expect(state.surface).toBe('idle')
    expect(state.notice).toBeNull()
    expect(state.selection).toBeNull()
    expect(state.contentHint).toBeNull()
    expect(state.suggestedAction).toBeNull()
  })

  it('preserves structured background task and notice metadata', () => {
    const state = reducer(
      {
        ...initialState,
        backgroundTask: {
          id: 'task-1',
          actionKind: 'summarize-page',
          status: 'running',
          selection: null,
          startedAt: 'mock-start',
          completedAt: null,
        },
        notice: {
          id: 'notice-1',
          tone: 'info',
          message: 'Background summary is still running',
          source: 'background-task',
        },
      },
      {
        type: 'completeSelection',
        selection: { x: 40, y: 60, width: 240, height: 160 },
        hint: 'chart',
      },
    )

    expect(state.backgroundTask?.status).toBe('running')
    expect(state.notice?.source).toBe('background-task')
  })

  it('returns to selected when processing is cancelled', () => {
    const state = reducer(
      {
        ...initialState,
        surface: 'processing',
        selection: { x: 180, y: 120, width: 640, height: 420 },
        activeAction: 'analyze-error',
      },
      { type: 'cancelProcessing' },
    )

    expect(state.surface).toBe('selected')
    expect(state.selection).toEqual({ x: 180, y: 120, width: 640, height: 420 })
  })

  it('moves processing into a running background task', () => {
    const state = reducer(
      {
        ...initialState,
        surface: 'processing',
        selection: { x: 180, y: 120, width: 640, height: 420 },
        activeAction: 'analyze-error',
      },
      { type: 'runInBackground' },
    )

    expect(state.surface).toBe('idle')
    expect(state.backgroundTask?.status).toBe('running')
    expect(state.orbTaskChip).toBe('1 task running')
  })

  it('restores a completed background task near its prior selection anchor', () => {
    const selection = { x: 180, y: 120, width: 640, height: 420 }
    const state = reducer(
      {
        ...initialState,
        backgroundTask: {
          id: 'task-1',
          actionKind: 'analyze-error',
          status: 'complete',
          selection,
          startedAt: 'mock-start',
          completedAt: 'mock-complete',
        },
      },
      { type: 'resumeCurrentTask' },
    )

    expect(state.surface).toBe('result')
    expect(state.selection).toEqual(selection)
    expect(state.sidePanelOpen).toBe(false)
    expect(state.result?.title).toMatch(/error/i)
  })

  it('falls back to the side panel when a completed background task has no selection anchor', () => {
    const state = reducer(
      {
        ...initialState,
        backgroundTask: {
          id: 'task-1',
          actionKind: 'analyze-error',
          status: 'complete',
          selection: null,
          startedAt: 'mock-start',
          completedAt: 'mock-complete',
        },
      },
      { type: 'resumeCurrentTask' },
    )

    expect(state.surface).toBe('result')
    expect(state.selection).toBeNull()
    expect(state.sidePanelOpen).toBe(true)
    expect(state.result?.title).toMatch(/error/i)
  })
})
