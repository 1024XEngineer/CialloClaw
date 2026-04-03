# Desktop AI Assistant Prototype

A frontend-only desktop AI assistant prototype that demonstrates floating-orb entry, screen-region selection, action picking, processing feedback, result cards, side-panel details, background-task handling, and repeat processing.

## Quick Start

```bash
npm install
npm run dev
```

## Development

```bash
# Run unit tests
npm run test -- --run

# Run e2e tests (requires installation)
npm run test:e2e:install
npm run test:e2e

# Build for production
npm run build
```

## Project Structure

```
src/
├── app/                  # Main app shell and routing
├── components/
│   ├── capture/         # Screen capture overlay and selection
│   ├── desktop/         # Desktop scene and mock environment
│   ├── feedback/        # Processing card and result display
│   ├── menu/            # Quick menu and action menu
│   ├── orb/              # Floating orb component
│   └── panel/            # Side detail panel
├── data/                 # Mock result fixtures
├── hooks/                # Custom hooks (e.g., mock processing)
├── lib/                  # Utilities (e.g., overlay layout)
├── state/                # Reducer and state management
├── styles/              # CSS tokens and base styles
├── test/                 # Test utilities and setup
└── types/                # TypeScript type definitions
```

## Features Implemented

- Floating orb with hover, drag-to-reposition, and edge snapping
- Quick actions menu anchored to orb position
- Full-screen capture overlay with drag selection
- Space key for full-screen capture
- Content hint labeling (text/chart/error)
- Post-selection action menu with primary and secondary actions
- Reselect and Pin to Side Panel utilities
- Processing card with stage progression
- Cancel and Run in Background actions
- Background task chip on orb
- Resume Current Task and Recent Result in quick menu
- Resume fallback to side panel when selection anchor is lost
- Keyboard support (Esc cancel, Enter confirm)

## Tech Stack

- React + TypeScript
- Vite
- Vitest + React Testing Library
- Playwright (e2e)
- CSS Modules