import { render, screen } from '@testing-library/react'
import App from './App'

it('renders the desktop prototype shell', () => {
  render(<App />)

  expect(screen.getByRole('button', { name: /open ai assistant/i })).toBeInTheDocument()
  expect(screen.getByText(/desktop ai assistant prototype/i)).toBeInTheDocument()
})
