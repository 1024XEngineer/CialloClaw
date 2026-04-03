import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import App from '../../app/App'
import { dragOrb } from '../../test/prototypeTestUtils'

it('shows the hover tooltip and snaps back to the nearest edge after drag', async () => {
  const user = userEvent.setup()

  render(<App />)

  const orb = screen.getByRole('button', { name: /open ai assistant/i })
  const tooltipText = /select an area for ai analysis/i

  expect(screen.queryByText(tooltipText)).not.toBeInTheDocument()
  await user.hover(orb)

  expect(screen.getByText(tooltipText)).toBeInTheDocument()

  await user.unhover(orb)

  expect(screen.queryByText(tooltipText)).not.toBeInTheDocument()

  await user.tab()

  expect(screen.getByText(tooltipText)).toBeInTheDocument()

  await dragOrb(orb, { start: [1100, 620], end: [880, 540] })

  expect(orb).toHaveAttribute('data-edge', 'right')
  expect(orb).toHaveAttribute('data-top', '460')
})
