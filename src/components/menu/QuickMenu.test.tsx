import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import App from '../../app/App'
import { dragOrb } from '../../test/prototypeTestUtils'
import { initialState } from '../../state/prototypeReducer'

it('opens the quick menu from the orb', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))

  expect(screen.getByRole('menu')).toHaveAttribute('data-edge', 'right')
  expect(screen.getByRole('menu')).toHaveAttribute('data-top', '432')
  expect(screen.getByRole('menuitem', { name: /region analyze/i })).toBeInTheDocument()
})

it('reanchors the quick menu to the orb edge after the orb is dragged', async () => {
  const user = userEvent.setup()

  render(<App />)

  const orb = screen.getByRole('button', { name: /open ai assistant/i })

  await dragOrb(orb, { start: [180, 620], end: [140, 540] })
  await user.click(orb)

  expect(screen.getByRole('menu')).toHaveAttribute('data-edge', 'left')
  expect(screen.getByRole('menu')).toHaveAttribute('data-top', '352')
})

it('renders icons for each quick menu action', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))

  expect(screen.getByTestId('quick-menu-icon-region-analyze')).toBeInTheDocument()
  expect(screen.getByTestId('quick-menu-icon-capture-text')).toBeInTheDocument()
  expect(screen.getByTestId('quick-menu-icon-recent-result')).toBeInTheDocument()
  expect(screen.getByTestId('quick-menu-icon-open-side-panel')).toBeInTheDocument()
})

it('shows recent result as disabled until a completed task exists', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))

  expect(screen.getByRole('menuitem', { name: /recent result/i })).toHaveAttribute('aria-disabled', 'true')
})

it('starts selection with recognize-text suggested when capture text is chosen', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /capture text/i }))

  expect(screen.getByTestId('surface-state')).toHaveTextContent('selecting')
  expect(screen.getByTestId('suggested-action')).toHaveTextContent('recognize-text')
})

it('opens an empty side panel when open side panel is chosen from the quick menu', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /open side panel/i }))

  expect(screen.getByRole('complementary', { name: /analysis details/i })).toBeInTheDocument()
  expect(screen.getByText(/select an area to begin analysis/i)).toBeInTheDocument()
})

it('shows resume current task when a background task exists', async () => {
  const user = userEvent.setup()

  render(
    <App
      initialState={{
        ...initialState,
        backgroundTask: {
          id: 'task-1',
          actionKind: 'analyze-error',
          status: 'running',
          selection: { x: 180, y: 120, width: 640, height: 420 },
          startedAt: 'mock-start',
          completedAt: null,
        },
      }}
    />,
  )

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))

  expect(screen.getByRole('menuitem', { name: /resume current task/i })).toBeInTheDocument()
})

it('restores a completed background task from the quick menu', async () => {
  const user = userEvent.setup()

  render(
    <App
      initialState={{
        ...initialState,
        backgroundTask: {
          id: 'task-1',
          actionKind: 'analyze-error',
          status: 'complete',
          selection: { x: 180, y: 120, width: 640, height: 420 },
          startedAt: 'mock-start',
          completedAt: 'mock-complete',
        },
      }}
    />,
  )

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /resume current task/i }))

  expect(screen.getByTestId('surface-state')).toHaveTextContent('result')
})
