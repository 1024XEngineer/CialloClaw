import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import App from '../../app/App'
import { dragSelection, enterSelectionMode } from '../../test/prototypeTestUtils'

it('shows the instruction bar and selection dimensions while dragging', async () => {
  const user = userEvent.setup()

  render(<App />)

  await enterSelectionMode()

  const canvas = screen.getByTestId('desktop-scene')

  await user.pointer([
    { target: canvas, coords: { x: 120, y: 100 }, keys: '[MouseLeft>]' },
    { target: canvas, coords: { x: 720, y: 420 } },
  ])

  expect(screen.getByText(/drag to select/i)).toBeInTheDocument()
  expect(screen.getByText('600 x 320')).toBeInTheDocument()

  await user.pointer([{ target: canvas, coords: { x: 720, y: 420 }, keys: '[/MouseLeft]' }])
})

it('hides the capture overlay once full-screen capture is confirmed with Space', async () => {
  const user = userEvent.setup()

  render(<App />)

  await enterSelectionMode()
  await user.keyboard(' ')

  expect(screen.getByTestId('surface-state')).toHaveTextContent('selected')
  expect(screen.queryByText(/drag to select/i)).not.toBeInTheDocument()
  expect(screen.queryByTestId('selection-box')).not.toBeInTheDocument()
})

it('preserves the seeded capture-text action after a valid drag selection', async () => {
  const user = userEvent.setup()

  render(<App />)

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /capture text/i }))
  await dragSelection({ start: [120, 100], end: [720, 420] })

  expect(screen.getByTestId('surface-state')).toHaveTextContent('selected')
  expect(screen.getByTestId('suggested-action')).toHaveTextContent('recognize-text')
})

it('hides the capture overlay after a valid drag selection is confirmed', async () => {
  render(<App />)

  await enterSelectionMode()
  await dragSelection({ start: [120, 100], end: [720, 420] })

  expect(screen.getByTestId('surface-state')).toHaveTextContent('selected')
  expect(screen.queryByText(/drag to select/i)).not.toBeInTheDocument()
  expect(screen.queryByTestId('selection-box')).not.toBeInTheDocument()
})

it('keeps the user in selection mode for tiny selections', async () => {
  render(<App />)

  await enterSelectionMode()
  await dragSelection({ start: [100, 100], end: [110, 110] })

  expect(screen.getByTestId('surface-state')).toHaveTextContent('selecting')
  expect(screen.getByText(/selection too small/i)).toBeInTheDocument()
})

it('cancels capture mode when Esc is pressed', async () => {
  const user = userEvent.setup()

  render(<App />)

  await enterSelectionMode()
  await user.keyboard('{Escape}')

  expect(screen.getByTestId('surface-state')).toHaveTextContent('idle')
  expect(screen.queryByText(/drag to select/i)).not.toBeInTheDocument()
  expect(screen.queryByText(/selection too small/i)).not.toBeInTheDocument()
})
