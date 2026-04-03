# Desktop AI Assistant Prototype Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a frontend-only desktop AI assistant prototype that demonstrates floating-orb entry, screen-region selection, action picking, processing feedback, result cards, side-panel details, background-task handling, and repeat processing without any backend integration.

**Architecture:** Build a single-page React + TypeScript prototype that simulates a desktop overlay system on top of a mock desktop workspace. Drive all UI states through a pure reducer so transitions are deterministic, testable, and easy to extend while result content comes from local mock fixtures instead of backend APIs.

**Tech Stack:** Vite, React, TypeScript, CSS Modules, Vitest, React Testing Library, Playwright

---

## Implementation Notes

- This repository is currently empty and not a git repo, so the first task initializes the workspace and version control.
- The prototype should simulate desktop behavior inside the browser. Do not implement OS-level screenshot APIs, native permissions, or backend logic.
- Keep all copy and layout aligned to `docs/superpowers/specs/2026-04-02-desktop-ai-assistant-prototype-design.md`.
- Prefer one focused component per file. Keep state transitions in the reducer, not hidden inside many components.
- Any unit-test helper names used later in this plan (`dragOrb`, `enterSelectionMode`, `dragSelection`, `selectLargeRegion`, `startProcessingFlow`, `finishAnalyzeErrorFlow`, `finishSummarizePageFlow`) should be implemented once in `src/test/prototypeTestUtils.tsx` and imported into the named test files.

## File Structure

### App Shell and Tooling

- Create: `package.json`
- Create: `tsconfig.json`
- Create: `tsconfig.node.json`
- Create: `vite.config.ts`
- Create: `vitest.config.ts`
- Create: `playwright.config.ts`
- Create: `index.html`
- Create: `src/main.tsx`
- Create: `src/vite-env.d.ts`

### Shared Styling and Types

- Create: `src/styles/tokens.css`
- Create: `src/styles/base.css`
- Create: `src/types/prototype.ts`

### State and Mock Data

- Create: `src/state/prototypeReducer.ts`
- Create: `src/state/prototypeReducer.test.ts`
- Create: `src/data/mockResults.ts`
- Create: `src/hooks/useMockProcessing.ts`
- Create: `src/lib/overlayLayout.ts`
- Create: `src/lib/overlayLayout.test.ts`

### UI Components

- Create: `src/app/App.tsx`
- Create: `src/app/App.module.css`
- Create: `src/app/App.test.tsx`
- Create: `src/components/desktop/DesktopScene.tsx`
- Create: `src/components/desktop/DesktopScene.module.css`
- Create: `src/components/orb/FloatingOrb.tsx`
- Create: `src/components/orb/FloatingOrb.module.css`
- Create: `src/components/menu/QuickMenu.tsx`
- Create: `src/components/menu/QuickMenu.module.css`
- Create: `src/components/capture/CaptureOverlay.tsx`
- Create: `src/components/capture/CaptureOverlay.module.css`
- Create: `src/components/capture/SelectionBox.tsx`
- Create: `src/components/capture/SelectionBox.module.css`
- Create: `src/components/menu/ActionMenu.tsx`
- Create: `src/components/menu/ActionMenu.module.css`
- Create: `src/components/feedback/ProcessingCard.tsx`
- Create: `src/components/feedback/ProcessingCard.module.css`
- Create: `src/components/feedback/ResultCard.tsx`
- Create: `src/components/feedback/ResultCard.module.css`
- Create: `src/components/panel/SidePanel.tsx`
- Create: `src/components/panel/SidePanel.module.css`
- Create: `src/components/feedback/StatusNotice.tsx`
- Create: `src/components/feedback/StatusNotice.module.css`

### UI Tests

- Create: `src/test/prototypeTestUtils.tsx`
- Create: `src/components/orb/FloatingOrb.test.tsx`
- Create: `src/components/menu/QuickMenu.test.tsx`
- Create: `src/components/capture/CaptureOverlay.test.tsx`
- Create: `src/components/menu/ActionMenu.test.tsx`
- Create: `src/components/feedback/ResultCard.test.tsx`
- Create: `src/components/panel/SidePanel.test.tsx`
- Create: `src/app/App.keyboard.test.tsx`

### End-to-End and Handoff

- Create: `tests/e2e/helpers/dragMockSelection.ts`
- Create: `tests/e2e/prototype.spec.ts`
- Create: `README.md`

## Task 1: Initialize the Prototype Workspace

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `tsconfig.node.json`
- Create: `vite.config.ts`
- Create: `vitest.config.ts`
- Create: `playwright.config.ts`
- Create: `index.html`
- Create: `src/main.tsx`
- Create: `src/vite-env.d.ts`
- Create: `src/app/App.tsx`
- Create: `src/app/App.module.css`
- Create: `src/app/App.test.tsx`
- Create: `src/styles/tokens.css`
- Create: `src/styles/base.css`
- Test: `src/app/App.test.tsx`

- [ ] **Step 1: Initialize git and scaffold the Vite React TypeScript app in a temporary folder, then copy it into the current root so the existing `docs/` directory is preserved**

```bash
git init
npm create vite@latest .tmp-vite -- --template react-ts
node -e "const fs=require('fs');const path=require('path');for(const entry of fs.readdirSync('.tmp-vite')){fs.cpSync(path.join('.tmp-vite', entry), entry, { recursive: true, force: true });}fs.rmSync('.tmp-vite',{ recursive: true, force: true });"
```

- [ ] **Step 2: Install the test dependencies and browser test runner**

```bash
npm install
npm install -D vitest jsdom @testing-library/react @testing-library/jest-dom @testing-library/user-event playwright
npx playwright install
```

- [ ] **Step 3: Add a failing smoke test for the app shell**

```tsx
import { render, screen } from '@testing-library/react'
import App from './App'

it('renders the desktop prototype shell', () => {
  render(<App />)
  expect(screen.getByRole('button', { name: /open ai assistant/i })).toBeInTheDocument()
  expect(screen.getByText(/desktop ai assistant prototype/i)).toBeInTheDocument()
})
```

- [ ] **Step 4: Configure the base app, scripts, and minimal shell to satisfy the test**

```tsx
import styles from './App.module.css'

export default function App() {
  return (
    <main className={styles.app}>
      <h1>Desktop AI Assistant Prototype</h1>
      <button aria-label="Open AI assistant">AI</button>
    </main>
  )
}
```

- [ ] **Step 5: Run the smoke test**

Run: `npm run test -- --run src/app/App.test.tsx`
Expected: PASS with `1 passed`

- [ ] **Step 6: Commit the scaffold**

```bash
git add .
git commit -m "chore: scaffold desktop prototype app"
```

## Task 2: Define the Prototype State Model and Mock Content

**Files:**
- Create: `src/types/prototype.ts`
- Create: `src/state/prototypeReducer.ts`
- Create: `src/state/prototypeReducer.test.ts`
- Create: `src/data/mockResults.ts`
- Create: `src/test/prototypeTestUtils.tsx`
- Modify: `src/app/App.tsx`
- Test: `src/state/prototypeReducer.test.ts`

- [ ] **Step 1: Write failing reducer tests for the core happy-path transitions**

```ts
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

it('moves from processing to result using local mock payloads', () => {
  const state = reducer(initialState, { type: 'processingComplete', actionKind: 'analyze-error' })
  expect(state.surface).toBe('result')
  expect(state.result?.title).toMatch(/error/i)
})
```

- [ ] **Step 2: Run the reducer test to confirm it fails**

Run: `npm run test -- --run src/state/prototypeReducer.test.ts`
Expected: FAIL because `prototypeReducer` and related types do not exist yet

- [ ] **Step 3: Implement the shared types and reducer**

```ts
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

export const initialState: PrototypeState = {
  surface: 'idle',
  selection: null,
  contentHint: null,
  activeAction: null,
  suggestedAction: null,
  quickMenuOpen: false,
  sidePanelOpen: false,
  backgroundTask: null,
  result: null,
  notice: null,
  derivedFrom: null,
  lastCompletedAt: null,
}

export function reducer(state: PrototypeState, action: PrototypeAction): PrototypeState {
  switch (action.type) {
    case 'openQuickMenu':
      return { ...state, surface: 'quick-menu' }
    case 'startSelection':
      return { ...state, surface: 'selecting', quickMenuOpen: false }
    case 'completeSelection':
      return { ...state, surface: 'selected', selection: action.selection, contentHint: action.hint }
    default:
      return state
  }
}
```

- [ ] **Step 4: Add deterministic mock results for each action kind**

```ts
export const mockResults: Record<ActionKind, ResultPayload> = {
  'analyze-error': {
    title: 'Possible null reference in render path',
    summary: ['Cause is likely missing API payload guard', 'Check the state before mapping items'],
    detail: ['The selected stack trace points to Dashboard.tsx:84', 'The failing path reads items[0].name before loading completes'],
  },
  'explain-chart': {
    title: 'Revenue trend rises after Q2 dip',
    summary: ['Growth resumes in Q3', 'April remains the weakest month'],
    detail: ['The highlighted region shows a 22% climb from June to September'],
  },
  'summarize-page': {
    title: 'Release note summary ready',
    summary: ['Three visible product changes', 'One migration warning'],
    detail: ['The page emphasizes search speed, admin filters, and onboarding updates'],
  },
  'generate-reply': {
    title: 'Reply draft prepared',
    summary: ['Acknowledges the request', 'Calls out the next action clearly'],
    detail: ['The mock reply stays concise and professional for work chat or email'],
  },
  'recognize-text': {
    title: 'Text extracted from selection',
    summary: ['Headline and body copied into structured output'],
    detail: ['The result preserves paragraph order and line breaks'],
  },
  'analyze-content': {
    title: 'Selection analyzed',
    summary: ['Detected mixed text and UI elements', 'Best next step is task-specific refinement'],
    detail: ['The area contains enough structure to continue with a specialized action'],
  },
}
```

- [ ] **Step 5: Wire `useReducer` into `src/app/App.tsx` and render the current surface label plus suggested action for debugging**

```tsx
type AppProps = {
  initialState?: PrototypeState
}

export default function App({ initialState: seedState = initialState }: AppProps) {
  const [state, dispatch] = useReducer(reducer, seedState)

  return (
    <main className={styles.app}>
      <span data-testid="surface-state">{state.surface}</span>
      <span data-testid="suggested-action">{state.suggestedAction ?? 'none'}</span>
      <DesktopScene />
    </main>
  )
}
```

- [ ] **Step 6: Create `src/test/prototypeTestUtils.tsx` so later test files share one source of truth for interaction helpers**

```tsx
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'

export async function dragSelection({ start, end }) {
  const user = userEvent.setup()
  const canvas = screen.getByTestId('desktop-scene')
  await user.pointer([
    { target: canvas, coords: { x: start[0], y: start[1] }, keys: '[MouseLeft>]' },
    { target: canvas, coords: { x: end[0], y: end[1] } },
    { target: canvas, coords: { x: end[0], y: end[1] }, keys: '[/MouseLeft]' },
  ])
}

export async function dragOrb(orb: HTMLElement, { start, end }) {
  const user = userEvent.setup()
  await user.pointer([
    { target: orb, coords: { x: start[0], y: start[1] }, keys: '[MouseLeft>]' },
    { target: orb, coords: { x: end[0], y: end[1] } },
    { target: orb, coords: { x: end[0], y: end[1] }, keys: '[/MouseLeft]' },
  ])
}

export async function enterSelectionMode() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /region analyze/i }))
}

export async function selectLargeRegion() {
  await enterSelectionMode()
  await dragSelection({ start: [120, 100], end: [720, 420] })
}

export async function startProcessingFlow() {
  const user = userEvent.setup()
  await selectLargeRegion()
  await user.click(screen.getByRole('button', { name: /analyze error/i }))
}

export async function finishAnalyzeErrorFlow() {
  await startProcessingFlow()
  vi.runAllTimers()
}

export async function finishSummarizePageFlow() {
  const user = userEvent.setup()
  await selectLargeRegion()
  await user.click(screen.getByRole('button', { name: /summarize page/i }))
  vi.runAllTimers()
}
```

- [ ] **Step 7: Run the reducer test again**

Run: `npm run test -- --run src/state/prototypeReducer.test.ts`
Expected: PASS with all core transitions green

- [ ] **Step 8: Commit the state model and test helpers**

```bash
git add src/types/prototype.ts src/state/prototypeReducer.ts src/state/prototypeReducer.test.ts src/data/mockResults.ts src/test/prototypeTestUtils.tsx src/app/App.tsx
git commit -m "feat: add prototype state model"
```

## Task 3: Build the Desktop Scene, Floating Orb, and Quick Menu

**Files:**
- Create: `src/components/desktop/DesktopScene.tsx`
- Create: `src/components/desktop/DesktopScene.module.css`
- Create: `src/components/orb/FloatingOrb.tsx`
- Create: `src/components/orb/FloatingOrb.module.css`
- Create: `src/components/menu/QuickMenu.tsx`
- Create: `src/components/menu/QuickMenu.module.css`
- Create: `src/components/orb/FloatingOrb.test.tsx`
- Create: `src/components/menu/QuickMenu.test.tsx`
- Modify: `src/app/App.tsx`
- Test: `src/components/orb/FloatingOrb.test.tsx`
- Test: `src/components/menu/QuickMenu.test.tsx`

- [ ] **Step 1: Write failing tests for the orb hover state and quick menu actions**

```tsx
it('shows the hover tooltip and snaps back to the nearest edge after drag', async () => {
  render(<App />)
  const orb = screen.getByRole('button', { name: /open ai assistant/i })
  await user.hover(orb)
  expect(screen.getByText(/select an area for ai analysis/i)).toBeInTheDocument()
  await dragOrb(orb, { start: [1100, 620], end: [880, 540] })
  expect(orb).toHaveAttribute('data-edge', 'right')
})

it('opens the quick menu from the orb', async () => {
  render(<App />)
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  expect(screen.getByRole('menu')).toBeInTheDocument()
  expect(screen.getByRole('menuitem', { name: /region analyze/i })).toBeInTheDocument()
})

it('shows recent result as disabled until a completed task exists', async () => {
  render(<App />)
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  expect(screen.getByRole('menuitem', { name: /recent result/i })).toHaveAttribute('aria-disabled', 'true')
})

it('starts selection with recognize-text suggested when capture text is chosen', async () => {
  render(<App />)
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /capture text/i }))
  expect(screen.getByTestId('surface-state')).toHaveTextContent('selecting')
  expect(screen.getByTestId('suggested-action')).toHaveTextContent('recognize-text')
})

it('opens an empty side panel when open side panel is chosen from the quick menu', async () => {
  render(<App />)
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /open side panel/i }))
  expect(screen.getByRole('complementary', { name: /analysis details/i })).toBeInTheDocument()
  expect(screen.getByText(/select an area to begin analysis/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the orb and menu tests to confirm they fail**

Run: `npm run test -- --run src/components/orb/FloatingOrb.test.tsx src/components/menu/QuickMenu.test.tsx`
Expected: FAIL because the components do not exist yet

- [ ] **Step 3: Implement the mock desktop scene and floating orb with hover tooltip, glow, drag repositioning, and edge snapping**

```tsx
export function FloatingOrb({ onOpen, statusLabel, edge, onDragEnd }: FloatingOrbProps) {
  return (
    <div className={styles.orbWrap} data-edge={edge}>
      <button className={styles.orb} aria-label="Open AI assistant" onClick={onOpen}>
        <span>AI</span>
        {statusLabel ? <span className={styles.badge}>{statusLabel}</span> : null}
      </button>
      <span className={styles.tooltip}>Select an area for AI analysis</span>
    </div>
  )
}
```

- [ ] **Step 4: Implement the anchored quick menu with concrete flows for `Region Analyze`, `Capture Text`, `Recent Result`, and `Open Side Panel`**

```tsx
const menuItems = [
  { id: 'region-analyze', label: 'Region Analyze', description: 'Select an area and let AI inspect it' },
  { id: 'capture-text', label: 'Capture Text', description: 'Select a text block for OCR-style output' },
  { id: 'recent-result', label: 'Recent Result', description: 'Return to the latest completed task' },
  { id: 'open-side-panel', label: 'Open Side Panel', description: 'Pin the current context to the right panel' },
]

function handleQuickMenuAction(id: string) {
  if (id === 'capture-text') dispatch({ type: 'startSelectionWithSuggestedAction', actionKind: 'recognize-text' })
  if (id === 'open-side-panel') dispatch({ type: 'openSidePanel' })
  if (id === 'recent-result') dispatch({ type: 'openRecentResult' })
  if (id === 'region-analyze') dispatch({ type: 'startSelection' })
}
```

- [ ] **Step 5: Replace the placeholder app shell with `DesktopScene`, `FloatingOrb`, and `QuickMenu` wiring**

Run: `npm run test -- --run src/components/orb/FloatingOrb.test.tsx src/components/menu/QuickMenu.test.tsx src/app/App.test.tsx`
Expected: PASS with menu behavior and shell still rendering correctly

- [ ] **Step 6: Commit the orb and menu UI**

```bash
git add src/components/desktop src/components/orb src/components/menu src/app/App.tsx
git commit -m "feat: add floating orb and quick menu"
```

## Task 4: Build the Capture Overlay and Selection Interaction

**Files:**
- Create: `src/components/capture/CaptureOverlay.tsx`
- Create: `src/components/capture/CaptureOverlay.module.css`
- Create: `src/components/capture/SelectionBox.tsx`
- Create: `src/components/capture/SelectionBox.module.css`
- Create: `src/components/capture/CaptureOverlay.test.tsx`
- Modify: `src/app/App.tsx`
- Modify: `src/state/prototypeReducer.ts`
- Test: `src/components/capture/CaptureOverlay.test.tsx`

- [ ] **Step 1: Write failing tests for selection mode, drag feedback, and too-small selection handling**

```tsx
it('shows the instruction bar and selection dimensions while dragging', async () => {
  render(<App />)
  await enterSelectionMode()
  await dragSelection({ start: [120, 100], end: [720, 420] })
  expect(screen.getByText(/drag to select/i)).toBeInTheDocument()
  expect(screen.getByText('600 x 320')).toBeInTheDocument()
})

it('converts the entire desktop scene into a selection when Space is pressed in capture mode', async () => {
  render(<App />)
  await enterSelectionMode()
  await user.keyboard(' ')
  expect(screen.getByTestId('selection-box')).toHaveAttribute('data-fullscreen', 'true')
})

it('keeps the user in selection mode for tiny selections', async () => {
  render(<App />)
  await enterSelectionMode()
  await dragSelection({ start: [100, 100], end: [110, 110] })
  expect(screen.getByText(/selection too small/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the capture overlay test and verify it fails**

Run: `npm run test -- --run src/components/capture/CaptureOverlay.test.tsx`
Expected: FAIL because capture components and pointer behavior are not implemented yet

- [ ] **Step 3: Implement the full-screen overlay, top instruction bar, and pointer-driven selection box**

```tsx
<div className={styles.overlay}>
  <div className={styles.instructions}>Drag to select · Esc cancel · Space capture full screen</div>
  <SelectionBox selection={draftSelection} hint={contentHint} />
</div>
```

- [ ] **Step 4: Add the selection threshold and mock content-hint labeling**

```ts
const MIN_SELECTION_SIZE = 48

function getContentHint(selection: SelectionRect): ContentHint {
  if (selection.width > 560 && selection.height > 300) return 'chart'
  if (selection.height < 180) return 'text'
  return 'error'
}
```

- [ ] **Step 5: Wire selection completion and `Space` full-screen capture into the reducer so valid selection always opens the post-selection state**

```ts
if (action.type === 'captureFullScreen') {
  const fullScreenSelection = { x: 24, y: 24, width: 1392, height: 852 }
  return {
    ...state,
    surface: 'selected',
    selection: fullScreenSelection,
    contentHint: getContentHint(fullScreenSelection),
  }
}
```

Run: `npm run test -- --run src/components/capture/CaptureOverlay.test.tsx src/state/prototypeReducer.test.ts`
Expected: PASS with both capture behavior and state transitions working

- [ ] **Step 6: Commit the capture overlay**

```bash
git add src/components/capture src/app/App.tsx src/state/prototypeReducer.ts
git commit -m "feat: add capture overlay and selection feedback"
```

## Task 5: Add the Post-Selection Action Menu and Processing Card

**Files:**
- Create: `src/components/menu/ActionMenu.tsx`
- Create: `src/components/menu/ActionMenu.module.css`
- Create: `src/components/feedback/ProcessingCard.tsx`
- Create: `src/components/feedback/ProcessingCard.module.css`
- Create: `src/components/menu/ActionMenu.test.tsx`
- Create: `src/hooks/useMockProcessing.ts`
- Modify: `src/app/App.tsx`
- Modify: `src/components/menu/QuickMenu.tsx`
- Modify: `src/components/menu/QuickMenu.test.tsx`
- Modify: `src/state/prototypeReducer.ts`
- Test: `src/components/menu/ActionMenu.test.tsx`

- [ ] **Step 1: Write failing tests for utility actions, processing stages, and the full background-task flow**

```tsx
it('starts analyze error from the post-selection menu', async () => {
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
  render(<App />)
  await startProcessingFlow()
  await user.click(screen.getByRole('button', { name: /run in background/i }))
  expect(screen.getByText(/1 task running/i)).toBeInTheDocument()
  expect(screen.getByTestId('orb-task-chip')).toBeInTheDocument()
})

it('returns to the post-selection state when cancel is pressed during processing', async () => {
  render(<App />)
  await startProcessingFlow()
  await user.click(screen.getByRole('button', { name: /cancel/i }))
  expect(screen.getByRole('toolbar', { name: /selection actions/i })).toBeInTheDocument()
})

it('lets the user move the processing card away from the selection when it blocks content', async () => {
  render(<App />)
  await startProcessingFlow()
  await dragOrb(screen.getByTestId('processing-card'), { start: [980, 260], end: [1180, 220] })
  expect(screen.getByTestId('processing-card')).toHaveStyle({ transform: expect.stringContaining('translate') })
})

it('surfaces resume current task and recent result after background completion', async () => {
  vi.useFakeTimers()
  render(<App />)
  await startProcessingFlow()
  await user.click(screen.getByRole('button', { name: /run in background/i }))
  vi.runAllTimers()
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  expect(screen.getByRole('menuitem', { name: /resume current task/i })).toBeInTheDocument()
  expect(screen.getByRole('menuitem', { name: /recent result/i })).toHaveAttribute('aria-disabled', 'false')
})

it('falls back to the side panel when a completed background task no longer has an anchor to restore', async () => {
  vi.useFakeTimers()
  render(
    <App
      initialState={{
        ...initialState,
        backgroundTask: { actionKind: 'analyze-error', selection: null, status: 'complete' },
      }}
    />,
  )
  await user.click(screen.getByRole('button', { name: /open ai assistant/i }))
  await user.click(screen.getByRole('menuitem', { name: /resume current task/i }))
  expect(screen.getByRole('complementary', { name: /analysis details/i })).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the action menu test to confirm it fails**

Run: `npm run test -- --run src/components/menu/ActionMenu.test.tsx`
Expected: FAIL because the action menu and processing hook do not exist yet

- [ ] **Step 3: Implement the action menu with one primary action, secondary task pills, and `Reselect` / `Pin to Side Panel` utility actions**

```tsx
<div className={styles.menu} role="toolbar" aria-label="Selection actions">
  <button className={styles.primary} data-primary-action="true" data-action-kind="analyze-content">Analyze Content</button>
  <button data-action-kind="recognize-text">Recognize Text</button>
  <button data-action-kind="explain-chart">Explain Chart</button>
  <button data-action-kind="analyze-error">Analyze Error</button>
  <button data-action-kind="summarize-page">Summarize Page</button>
  <button data-action-kind="generate-reply">Generate Reply</button>
  <button>Reselect</button>
  <button>Pin to Side Panel</button>
</div>
```

- [ ] **Step 4: Implement the mock processing hook and processing card stages**

```ts
const stages = ['Capturing area', 'Understanding structure', 'Preparing result'] as const

export function useMockProcessing(active: boolean, onComplete: () => void) {
  useEffect(() => {
    if (!active) return
    const timeouts = [300, 700, 1100].map((delay, index) =>
      window.setTimeout(() => {
        if (index === stages.length - 1) onComplete()
      }, delay),
    )
    return () => timeouts.forEach(window.clearTimeout)
  }, [active, onComplete])
}
```

```tsx
<section data-testid="processing-card" className={styles.card}>
  <header className={styles.dragHandle}>Working on this selection</header>
  <p>{currentStage}</p>
  <button>Cancel</button>
  <button>Run in Background</button>
</section>
```

- [ ] **Step 5: Add `Cancel`, `Run in Background`, `Resume Current Task`, completed-badge, prior-anchor restore, and `Recent Result` behavior**

```ts
if (action.type === 'cancelProcessing') {
  return {
    ...state,
    surface: 'selected',
    processingStage: null,
  }
}

if (action.type === 'runInBackground') {
  return {
    ...state,
    surface: 'idle',
    backgroundTask: {
      actionKind: state.activeAction,
      selection: state.selection,
      status: 'running',
    },
    orbTaskChip: '1 task running',
  }
}

if (action.type === 'resumeCurrentTask') {
  if (state.backgroundTask?.selection) {
    return {
      ...state,
      surface: 'result',
      result: mockResults[state.backgroundTask.actionKind],
      selection: state.backgroundTask.selection,
      sidePanelOpen: false,
    }
  }

  return {
    ...state,
    surface: 'result',
    result: mockResults[state.backgroundTask!.actionKind],
    sidePanelOpen: true,
  }
}
```

Run: `npm run test -- --run src/components/menu/ActionMenu.test.tsx src/components/menu/QuickMenu.test.tsx src/state/prototypeReducer.test.ts`
Expected: PASS with stage changes, background-task state reflected in the orb, and quick-menu recovery actions working

- [ ] **Step 6: Commit the action and processing states**

```bash
git add src/components/menu/ActionMenu* src/components/feedback/ProcessingCard* src/hooks/useMockProcessing.ts src/app/App.tsx src/state/prototypeReducer.ts
git commit -m "feat: add action menu and processing states"
```

## Task 6: Build the Result Card and Right-Side Detail Panel

**Files:**
- Create: `src/components/feedback/ResultCard.tsx`
- Create: `src/components/feedback/ResultCard.module.css`
- Create: `src/components/feedback/ResultCard.test.tsx`
- Create: `src/components/panel/SidePanel.tsx`
- Create: `src/components/panel/SidePanel.module.css`
- Create: `src/components/panel/SidePanel.test.tsx`
- Modify: `src/app/App.tsx`
- Modify: `src/data/mockResults.ts`
- Test: `src/components/feedback/ResultCard.test.tsx`
- Test: `src/components/panel/SidePanel.test.tsx`

- [ ] **Step 1: Write failing tests for concise result output and side-panel expansion**

```tsx
it('renders a short result card near the selected source', async () => {
  render(<App />)
  await finishAnalyzeErrorFlow()
  expect(screen.getByRole('heading', { name: /possible null reference/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /open details/i })).toBeInTheDocument()
})

it('opens a side panel with summary, breakdown, actions, and history', async () => {
  render(<App />)
  await finishAnalyzeErrorFlow()
  await user.click(screen.getByRole('button', { name: /open details/i }))
  expect(screen.getByRole('complementary', { name: /analysis details/i })).toBeInTheDocument()
  expect(screen.getByText(/detailed breakdown/i)).toBeInTheDocument()
  expect(screen.getByTestId('side-panel-header')).toBeInTheDocument()
})

it('pins the current selection into the side panel before processing', async () => {
  render(<App />)
  await selectLargeRegion()
  await user.click(screen.getByRole('button', { name: /pin to side panel/i }))
  expect(screen.getByRole('complementary', { name: /analysis details/i })).toBeInTheDocument()
  expect(screen.getByText(/ready to analyze this selection/i)).toBeInTheDocument()
})

it('offers continue-processing actions from the result card and side panel', async () => {
  render(<App />)
  await finishSummarizePageFlow()
  expect(screen.getByRole('button', { name: /generate reply from this/i })).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /open details/i }))
  expect(screen.getByRole('button', { name: /turn into checklist/i })).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the result and panel tests to verify they fail**

Run: `npm run test -- --run src/components/feedback/ResultCard.test.tsx src/components/panel/SidePanel.test.tsx`
Expected: FAIL because result and panel components do not exist yet

- [ ] **Step 3: Implement the concise result card with structured follow-up buttons and visible continue-processing triggers**

```tsx
export function ResultCard({ result, onOpenDetails }: ResultCardProps) {
  return (
    <section className={styles.card} aria-label="AI result">
      <h2>{result.title}</h2>
      <ul>{result.summary.map((item) => <li key={item}>{item}</li>)}</ul>
      <div className={styles.actions}>
        <button>Explain More</button>
        <button>Change Angle</button>
        <button>Generate Steps</button>
        <button>Generate Reply from This</button>
        <button>Copy</button>
        <button onClick={onOpenDetails}>Open Details</button>
      </div>
    </section>
  )
}
```

- [ ] **Step 4: Implement the right-side panel with selection thumbnail, timestamp, pinned-selection placeholder, and follow-up controls**

```tsx
<aside className={styles.panel} aria-label="Analysis details">
  <header className={styles.stickyHeader} data-testid="side-panel-header">
    <div className={styles.thumbnail} />
    <div>
      <strong>{result.title}</strong>
      <span>{state.lastCompletedAt}</span>
      <small>{formatAction(state.activeAction)}</small>
    </div>
  </header>
  <div className={styles.followUps}>
    <button>Generate Reply from This</button>
    <button>Turn into Checklist</button>
    <button>Reselect Area</button>
  </div>
  <div className={styles.scrollRegion}>
    <section><h3>Summary</h3></section>
    <section><h3>Detailed Breakdown</h3></section>
    <section><h3>Suggested Actions</h3></section>
    <section><h3>Related History</h3></section>
  </div>
</aside>
```

- [ ] **Step 5: Wire result and detail actions into `App.tsx` and rerun the tests**

Run: `npm run test -- --run src/components/feedback/ResultCard.test.tsx src/components/panel/SidePanel.test.tsx src/app/App.test.tsx`
Expected: PASS with the detail surface available from the result state

- [ ] **Step 6: Commit the result and detail UI**

```bash
git add src/components/feedback/ResultCard* src/components/panel src/app/App.tsx src/data/mockResults.ts
git commit -m "feat: add result card and detail panel"
```

## Task 7: Support Reselect, Continue Processing, and Edge States

**Files:**
- Create: `src/components/feedback/StatusNotice.tsx`
- Create: `src/components/feedback/StatusNotice.module.css`
- Modify: `src/components/feedback/ResultCard.tsx`
- Modify: `src/components/feedback/ResultCard.module.css`
- Modify: `src/components/feedback/ResultCard.test.tsx`
- Modify: `src/hooks/useMockProcessing.ts`
- Modify: `src/state/prototypeReducer.ts`
- Modify: `src/state/prototypeReducer.test.ts`
- Modify: `src/app/App.tsx`
- Modify: `src/app/App.test.tsx`
- Modify: `src/data/mockResults.ts`
- Test: `src/state/prototypeReducer.test.ts`

- [ ] **Step 1: Write failing tests for context-preserving reselect and derived follow-up actions**

```ts
it('preserves analyze-error as the suggested action when reselecting from an error result', () => {
  const state = reducer(resultState('analyze-error'), { type: 'reselectRequested' })
  expect(state.surface).toBe('selecting')
  expect(state.suggestedAction).toBe('analyze-error')
})

it('labels continue processing as a derived action', () => {
  const state = reducer(resultState('summarize-page'), {
    type: 'continueProcessing',
    nextAction: 'generate-reply',
  })
  expect(state.derivedFrom).toBe('summarize-page')
})
```

- [ ] **Step 2: Add failing tests for unsupported content, no-result, and failure states**

```ts
it('shows an unsupported-content notice inside the normal result surface when the task does not match the selected area', () => {
  const state = reducer(selectedState('text'), { type: 'processingUnsupported', actionKind: 'explain-chart' })
  expect(state.surface).toBe('result')
  expect(state.notice?.kind).toBe('unsupported')
})

it('shows a no-result notice inside the normal result surface when the chosen task has weak confidence', () => {
  const state = reducer(selectedState('chart'), { type: 'processingEmpty', actionKind: 'analyze-error' })
  expect(state.surface).toBe('result')
  expect(state.notice?.kind).toBe('no-result')
})

it('shows a failure notice inside the result surface when mock processing throws', () => {
  const state = reducer(selectedState('error'), { type: 'processingFailed', message: 'Mock timeout' })
  expect(state.surface).toBe('result')
  expect(state.notice?.kind).toBe('failure')
})
```

```tsx
it('reaches unsupported content through the live UI with a text-shaped selection and Explain Chart', async () => {
  render(<App />)
  const user = userEvent.setup()
  await enterSelectionMode()
  await dragSelection({ start: [100, 120], end: [500, 220] })
  await user.click(screen.getByRole('button', { name: /explain chart/i }))
  expect(screen.getByRole('button', { name: /try recognize text/i })).toBeInTheDocument()
})

it('reaches no-result through the live UI with a chart-shaped selection and Analyze Error', async () => {
  render(<App />)
  const user = userEvent.setup()
  await enterSelectionMode()
  await dragSelection({ start: [120, 100], end: [760, 460] })
  await user.click(screen.getByRole('button', { name: /analyze error/i }))
  expect(screen.getByRole('button', { name: /select larger area/i })).toBeInTheDocument()
})

it('reaches failure through the live UI with an error-shaped selection and Generate Reply', async () => {
  render(<App />)
  const user = userEvent.setup()
  await enterSelectionMode()
  await dragSelection({ start: [120, 100], end: [720, 300] })
  await user.click(screen.getByRole('button', { name: /generate reply/i }))
  expect(screen.getByRole('button', { name: /run in background/i })).toBeInTheDocument()
})
```

- [ ] **Step 3: Implement reducer branches and `StatusNotice` variants for all edge states, but render them inside the standard result-card surface to match the spec**

```tsx
export function StatusNotice({ notice }: StatusNoticeProps) {
  const actionsByKind = {
    unsupported: ['Try Recognize Text', 'Reselect Area', 'Open Details'],
    'no-result': ['Select Larger Area', 'Choose Another Action', 'Open Side Panel'],
    failure: ['Retry', 'Run in Background', 'Reselect'],
  } as const

  return (
    <section className={styles.notice} aria-live="polite" data-variant={notice.kind}>
      <h2>{notice.title}</h2>
      <p>{notice.message}</p>
      <div>
        {actionsByKind[notice.kind].map((label) => (
          <button key={label}>{label}</button>
        ))}
      </div>
    </section>
  )
}

// ResultCard.tsx
if (result.notice) {
  return (
    <section className={styles.card} aria-label="AI result notice">
      <StatusNotice notice={result.notice} />
    </section>
  )
}
```

- [ ] **Step 4: Add deterministic mock outcome rules so each non-happy-path state is reachable on demand in the running prototype**

```ts
export function resolveMockOutcome(contentHint: ContentHint, actionKind: ActionKind) {
  if (contentHint === 'text' && actionKind === 'explain-chart') return 'unsupported'
  if (contentHint === 'chart' && actionKind === 'analyze-error') return 'no-result'
  if (contentHint === 'error' && actionKind === 'generate-reply') return 'failure'
  return 'success'
}
```

- [ ] **Step 5: Add the visible derived-action label and suggested-action badge in the app shell**

```tsx
{state.derivedFrom ? (
  <span className={styles.derivedLabel}>
    Based on: {formatAction(state.derivedFrom)} -&gt; {formatAction(state.activeAction)}
  </span>
) : null}
```

- [ ] **Step 6: Run the reducer and UI suites and confirm all follow-up and edge-state behavior**

Run: `npm run test -- --run src/state/prototypeReducer.test.ts src/app/App.test.tsx src/components/feedback/ResultCard.test.tsx`
Expected: PASS with happy path, continue path, and edge path coverage

- [ ] **Step 7: Commit the follow-up flows and notices**

```bash
git add src/components/feedback/StatusNotice* src/components/feedback/ResultCard* src/hooks/useMockProcessing.ts src/state/prototypeReducer.ts src/state/prototypeReducer.test.ts src/app/App.tsx src/app/App.test.tsx src/data/mockResults.ts
git commit -m "feat: add reselect flows and edge states"
```

## Task 8: Apply Visual System and Overlay Layout Rules

**Files:**
- Create: `src/lib/overlayLayout.ts`
- Create: `src/lib/overlayLayout.test.ts`
- Modify: `src/styles/tokens.css`
- Modify: `src/styles/base.css`
- Modify: `src/app/App.module.css`
- Modify: `src/components/orb/FloatingOrb.module.css`
- Modify: `src/components/menu/QuickMenu.module.css`
- Modify: `src/components/capture/CaptureOverlay.module.css`
- Modify: `src/components/capture/SelectionBox.module.css`
- Modify: `src/components/menu/ActionMenu.module.css`
- Modify: `src/components/feedback/ProcessingCard.module.css`
- Modify: `src/components/feedback/ResultCard.module.css`
- Modify: `src/components/panel/SidePanel.module.css`
- Modify: `src/app/App.tsx`
- Test: `src/lib/overlayLayout.test.ts`

- [ ] **Step 1: Write failing tests for anchored overlay placement, overlap avoidance, and docked panel sizing**

```ts
it('places the action menu adjacent to the selected area', () => {
  const position = placeOverlay({
    anchor: { x: 640, y: 360, width: 300, height: 180 },
    size: { width: 320, height: 120 },
    viewport: { width: 1440, height: 900 },
    preferred: 'below-right',
  })
  expect(position.left).toBeGreaterThan(640)
  expect(position.top).toBeGreaterThan(540)
})

it('moves the result card away when the preferred position would overlap the selection or screen edge', () => {
  const position = placeOverlay({
    anchor: { x: 1120, y: 640, width: 220, height: 140 },
    size: { width: 360, height: 220 },
    viewport: { width: 1440, height: 900 },
    preferred: 'below-right',
  })
  expect(position.left + 360).toBeLessThanOrEqual(1440)
})

it('uses a fixed docked width for the side panel', () => {
  expect(getSidePanelWidth(1440)).toBe(400)
})
```

- [ ] **Step 2: Run the layout test to confirm it fails**

Run: `npm run test -- --run src/lib/overlayLayout.test.ts`
Expected: FAIL because the layout utility does not exist yet

- [ ] **Step 3: Implement the visual token system from the spec in `src/styles/tokens.css` and `src/styles/base.css`**

```css
:root {
  --color-surface: #f7f8fa;
  --color-text: #111827;
  --color-muted: #6b7280;
  --color-accent: #3b82f6;
  --color-accent-hover: #2563eb;
  --color-overlay: rgba(15, 23, 42, 0.42);
  --font-ui: 'PingFang SC', 'HarmonyOS Sans SC', 'MiSans', sans-serif;
  --radius-panel: 20px;
  --shadow-float: 0 18px 50px rgba(15, 23, 42, 0.14);
  --blur-frosted: 18px;
  --panel-width: 400px;
  --motion-fast: 160ms;
}
```

- [ ] **Step 4: Implement `overlayLayout.ts` and apply it so menus and cards stay anchored to the orb or selection while avoiding overlap**

```ts
export function placeOverlay({ anchor, size, viewport }: PlaceOverlayArgs) {
  const candidates = [
    { left: anchor.x + anchor.width + 12, top: anchor.y + anchor.height + 12 },
    { left: anchor.x - size.width - 12, top: anchor.y },
    { left: anchor.x, top: anchor.y - size.height - 12 },
  ]

  return candidates.find((candidate) => fitsViewport(candidate, size, viewport)) ?? clampToViewport(candidates[0], size, viewport)
}

export function getSidePanelWidth() {
  return 400
}
```

- [ ] **Step 5: Update component CSS modules so the UI matches the spec’s utility look and overlay behavior instead of a generic in-app page**

```css
.card {
  background: rgba(247, 248, 250, 0.86);
  backdrop-filter: blur(var(--blur-frosted));
  border: 1px solid rgba(255, 255, 255, 0.7);
  border-radius: var(--radius-panel);
  box-shadow: var(--shadow-float);
}

.panel {
  position: fixed;
  top: 24px;
  right: 24px;
  width: var(--panel-width);
  height: calc(100vh - 48px);
}
```

- [ ] **Step 6: Run layout tests and manually verify the overlay system looks and behaves like one coherent desktop utility**

Run: `npm run test -- --run src/lib/overlayLayout.test.ts`
Expected: PASS with anchored placement and side-panel sizing green

- [ ] **Step 7: Commit the visual system and overlay layout rules**

```bash
git add src/lib/overlayLayout.ts src/lib/overlayLayout.test.ts src/styles/tokens.css src/styles/base.css src/app/App.module.css src/components/orb/FloatingOrb.module.css src/components/menu/QuickMenu.module.css src/components/capture/CaptureOverlay.module.css src/components/capture/SelectionBox.module.css src/components/menu/ActionMenu.module.css src/components/feedback/ProcessingCard.module.css src/components/feedback/ResultCard.module.css src/components/panel/SidePanel.module.css src/app/App.tsx
git commit -m "feat: add desktop overlay layout and visual system"
```

## Task 9: Add Keyboard Handling, End-to-End Tests, and Usage Docs

**Files:**
- Create: `src/app/App.keyboard.test.tsx`
- Create: `tests/e2e/helpers/dragMockSelection.ts`
- Create: `tests/e2e/prototype.spec.ts`
- Create: `README.md`
- Modify: `src/app/App.tsx`
- Test: `src/app/App.keyboard.test.tsx`
- Test: `tests/e2e/prototype.spec.ts`

- [ ] **Step 1: Write failing keyboard tests for `Esc` cancel and focus-aware `Enter` primary-action confirm**

```tsx
it('cancels the current lightweight surface with Escape', async () => {
  render(<App />)
  await enterSelectionMode()
  await user.keyboard('{Escape}')
  expect(screen.queryByText(/drag to select/i)).not.toBeInTheDocument()
})

it('confirms the primary selected action only when that control has focus', async () => {
  render(<App />)
  await selectLargeRegion()
  screen.getByRole('button', { name: /analyze content/i }).focus()
  await user.keyboard('{Enter}')
  expect(screen.getByText(/capturing area/i)).toBeInTheDocument()
})

it('does not start processing when Enter is pressed without a focused action control', async () => {
  render(<App />)
  await selectLargeRegion()
  await user.keyboard('{Enter}')
  expect(screen.queryByText(/capturing area/i)).not.toBeInTheDocument()
})
```

- [ ] **Step 2: Write a failing Playwright test for the main prototype workflow and the background resume flow**

```ts
import { dragMockSelection } from './helpers/dragMockSelection'

test('completes the desktop prototype happy path', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Open AI assistant' }).click()
  await page.getByRole('menuitem', { name: 'Region Analyze' }).click()
  await dragMockSelection(page, { start: { x: 120, y: 120 }, end: { x: 700, y: 420 } })
  await page.getByRole('button', { name: 'Analyze Error' }).click()
  await expect(page.getByRole('heading', { name: /possible null reference/i })).toBeVisible()
})
```

- [ ] **Step 3: Create the Playwright drag helper and implement the keyboard event handling plus any missing stable labels or `data-testid` hooks**

```tsx
// tests/e2e/helpers/dragMockSelection.ts
export async function dragMockSelection(page, { start, end }) {
  await page.mouse.move(start.x, start.y)
  await page.mouse.down()
  await page.mouse.move(end.x, end.y, { steps: 12 })
  await page.mouse.up()
}

// src/app/App.tsx
useEffect(() => {
  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Escape') dispatch({ type: 'cancelSurface' })
    if (
      event.key === 'Enter' &&
      document.activeElement?.getAttribute('data-primary-action') === 'true'
    ) {
      dispatch({
        type: 'chooseAction',
        actionKind: document.activeElement.getAttribute('data-action-kind') as ActionKind,
      })
    }
  }
  window.addEventListener('keydown', handleKeyDown)
  return () => window.removeEventListener('keydown', handleKeyDown)
}, [state.surface])
```

- [ ] **Step 4: Document how to run the prototype and tests in `README.md`**

```md
## Run

`npm install`

`npm run dev`

## Test

`npm run test -- --run`

`npx playwright test`
```

- [ ] **Step 5: Run the full validation suite**

Run: `npm run test -- --run && npx playwright test`
Expected: PASS with all unit tests and the end-to-end workflow green

- [ ] **Step 6: Commit the verification and docs**

```bash
git add src/app/App.keyboard.test.tsx tests/e2e/prototype.spec.ts README.md src/app/App.tsx
git commit -m "test: add workflow coverage for desktop prototype"
```

## Final Verification Checklist

- [ ] Run `npm run dev` and manually inspect the orb, quick menu, overlay, result card, and side panel in the browser
- [ ] Confirm the UI never reads like a chat transcript
- [ ] Confirm the overlay, result card, and side panel all feel like the same desktop overlay system
- [ ] Confirm the palette, typography, frosted materials, and shadow/motion values match the spec’s modern utility style
- [ ] Confirm `Run in Background`, `Recent Result`, `Reselect`, and `Continue Processing` all work from the main flow
- [ ] Confirm invalid selection, no-result, and failure states are reachable in the prototype

## Handoff Notes

- Treat `src/state/prototypeReducer.ts` as the source of truth for all interaction-state changes
- Keep mock payloads in `src/data/mockResults.ts`; do not hardcode result text in components
- If you add motion later, prefer CSS transitions first and keep timings in the 120-220ms range from the spec
- Do not add backend or native OS code in this prototype branch
