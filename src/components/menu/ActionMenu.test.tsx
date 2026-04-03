import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import App from '../../app/App'
import { initialState } from '../../state/prototypeReducer'
import { dragOrb, selectLargeRegion, startProcessingFlow } from '../../test/prototypeTestUtils'

describe('ActionMenu', () => {
  beforeEach(() => {
    vi.useRealTimers()
  })

  afterEach(() => {
    if (vi.isFakeTimers()) {
      vi.runOnlyPendingTimers()
    }
    vi.useRealTimers()
  })

  it('starts analyze error from the post-selection menu', async () => {
    const user = userEvent.setup()

    render(<App />)

    await selectLargeRegion()
    await user.click(screen.getByRole('button', { name: /analyze error/i }))

    expect(screen.getByText(/capturing area/i)).toBeInTheDocument()
  })

  it('offers reselect and pin to side panel from the post-selection menu', async () => {
    render(<App />)

    await selectLargeRegion()

    expect(screen.getByRole('button', { name: /reselect/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /pin to side panel/i })).toBeInTheDocument()
  })

  it('collapses processing into the orb when run in background is pressed', async () => {
    const user = userEvent.setup()

    render(<App />)

    await startProcessingFlow()
    await user.click(screen.getByRole('button', { name: /run in background/i }))

    expect(screen.getByText(/1 task running/i)).toBeInTheDocument()
    expect(screen.getByTestId('orb-task-chip')).toBeInTheDocument()
  })

  it('returns to the post-selection state when cancel is pressed during processing', async () => {
    const user = userEvent.setup()

    render(<App />)

    await startProcessingFlow()
    await user.click(screen.getByRole('button', { name: /cancel/i }))

    expect(screen.getByRole('toolbar', { name: /selection actions/i })).toBeInTheDocument()
  })

  it('lets the user move the processing card away from the selection when it blocks content', async () => {
    render(<App />)

    await startProcessingFlow()
    await dragOrb(screen.getByTestId('processing-card'), { start: [980, 260], end: [1180, 220] })

    expect(screen.getByTestId('processing-card').style.transform).toContain('translate')
  })

  it('surfaces resume current task and recent result after background completion', async () => {
    vi.useFakeTimers()

    render(
      <App
        initialState={{
          ...initialState,
          backgroundTask: {
            id: 'task-1',
            actionKind: 'analyze-error',
            selection: { x: 180, y: 120, width: 640, height: 420 },
            status: 'running',
            startedAt: 'mock-start',
            completedAt: null,
          },
          orbTaskChip: '1 task running',
        }}
      />,
    )

    act(() => {
      vi.runAllTimers()
    })

    vi.useRealTimers()
    act(() => {
      screen.getByRole('button', { name: /open ai assistant/i }).click()
    })

    expect(screen.getByRole('menuitem', { name: /resume current task/i })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /recent result/i })).toHaveAttribute('aria-disabled', 'false')
  })

  it('falls back to the side panel when a completed background task no longer has an anchor to restore', async () => {
    const user = userEvent.setup()

    render(
      <App
        initialState={{
          ...initialState,
          backgroundTask: {
            id: 'task-1',
            actionKind: 'analyze-error',
            selection: null,
            status: 'complete',
            startedAt: 'mock-start',
            completedAt: 'mock-complete',
          },
        }}
      />,
    )

    await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
    await user.click(screen.getByRole('menuitem', { name: /resume current task/i }))

    expect(screen.getByRole('complementary', { name: /analysis details/i })).toBeInTheDocument()
  })
})
