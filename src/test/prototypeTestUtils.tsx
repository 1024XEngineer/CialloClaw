import { screen } from '@testing-library/react'
import userEvent, { type UserEvent } from '@testing-library/user-event'

type PointerRange = {
  start: [number, number]
  end: [number, number]
}

type FinishFlowOptions = {
  advanceTimers: () => void
}

export async function dragSelection({ start, end }: PointerRange, user: UserEvent = userEvent.setup()) {
  const canvas = screen.getByTestId('desktop-scene')

  await user.pointer([
    { target: canvas, coords: { x: start[0], y: start[1] }, keys: '[MouseLeft>]' },
    { target: canvas, coords: { x: end[0], y: end[1] } },
    { target: canvas, coords: { x: end[0], y: end[1] }, keys: '[/MouseLeft]' },
  ])
}

export async function dragOrb(orb: HTMLElement, { start, end }: PointerRange, user: UserEvent = userEvent.setup()) {

  await user.pointer([
    { target: orb, coords: { x: start[0], y: start[1] }, keys: '[MouseLeft>]' },
    { target: orb, coords: { x: end[0], y: end[1] } },
    { target: orb, coords: { x: end[0], y: end[1] }, keys: '[/MouseLeft]' },
  ])
}

export async function enterSelectionMode(user: UserEvent = userEvent.setup()) {

  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /region analyze/i }))
}

export async function selectLargeRegion() {
  await enterSelectionMode()
  await dragSelection({ start: [120, 100], end: [720, 420] })
}

export async function startProcessingFlow(user: UserEvent = userEvent.setup()) {
  await enterSelectionMode(user)
  await dragSelection({ start: [120, 100], end: [720, 420] }, user)

  await user.click(screen.getByRole('button', { name: /analyze error/i }))
}

export async function finishAnalyzeErrorFlow({ advanceTimers }: FinishFlowOptions) {
  await startProcessingFlow()
  advanceTimers()
}

export async function finishSummarizePageFlow({ advanceTimers }: FinishFlowOptions) {
  const user = userEvent.setup()

  await selectLargeRegion()
  await user.click(screen.getByRole('button', { name: /summarize page/i }))
  advanceTimers()
}
