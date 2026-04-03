import { render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { finishAnalyzeErrorFlow, finishSummarizePageFlow } from './prototypeTestUtils'

function TestHarness() {
  return (
    <>
      <div data-testid="desktop-scene" />
      <button aria-label="Open AI assistant" type="button">
        Open
      </button>
      <button role="menuitem" type="button">
        Region Analyze
      </button>
      <button type="button">Analyze Error</button>
      <button type="button">Summarize Page</button>
    </>
  )
}

describe('prototypeTestUtils', () => {
  it('uses an explicit timer advancement callback for analyze error flow', async () => {
    const advanceTimers = vi.fn()

    render(<TestHarness />)

    await finishAnalyzeErrorFlow({ advanceTimers })

    expect(advanceTimers).toHaveBeenCalledTimes(1)
  })

  it('uses an explicit timer advancement callback for summarize page flow', async () => {
    const advanceTimers = vi.fn()

    render(<TestHarness />)

    await finishSummarizePageFlow({ advanceTimers })

    expect(advanceTimers).toHaveBeenCalledTimes(1)
  })
})
