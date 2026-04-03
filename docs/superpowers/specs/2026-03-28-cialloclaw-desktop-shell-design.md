# CialloClaw Desktop Shell Minimal Design

## 1. Goal

Upgrade the existing local web prototype into the smallest runnable desktop version by adding:

- a real system-level floating ball window
- a real desktop main panel window
- the existing prototype UI inside the main panel

The target is not a full desktop product. The target is a minimal desktop shell that proves the current prototype can live on the desktop as a persistent entry point.

## 2. Scope

### In scope

- Electron-based desktop shell
- Two windows only:
  - floating ball window
  - main panel window
- Floating ball always visible after app launch
- Clicking the floating ball toggles the main panel window
- Main panel loads the existing prototype page
- Existing prototype loop continues to work inside the panel:
  - inspection
  - summary
  - draft confirmation
  - draft output
  - execution log
  - undo
- Minimal preload bridge for window toggle communication
- Basic desktop-oriented panel sizing and positioning

### Out of scope

- Tray icon
- Right-click context menu
- Edge snapping
- auto-hide
- drag persistence
- multi-display memory
- workspace permissions or native file access
- real background task scheduling
- packaging/installer work beyond local run

## 3. Product Shape

### Floating ball window

- Small standalone Electron `BrowserWindow`
- Transparent and frameless
- Always on top
- Positioned near the bottom-right of the primary display on launch
- Contains only the floating ball UI and minimal click affordance

### Main panel window

- Separate Electron `BrowserWindow`
- Hidden by default
- Opens near the floating ball when toggled
- Hosts the existing `index.html` prototype
- Can be hidden and shown repeatedly without losing the app process

## 4. User Flow

1. User runs `npm start`
2. App launches a floating ball window
3. Main panel window is created but hidden
4. User clicks the floating ball
5. Main panel appears near the floating ball
6. User interacts with the existing prototype inside the panel
7. User clicks the floating ball again
8. Main panel hides
9. Floating ball remains available on the desktop

## 5. Architecture

The desktop shell uses Electron with a strict minimum of responsibilities.

### Main process

The main process is responsible for:

- creating the floating ball window
- creating the main panel window
- positioning both windows
- responding to toggle requests from the floating window
- hiding the panel instead of closing the whole app when the panel is dismissed

### Preload layer

The preload layer exposes only the smallest possible bridge to the renderer. For MVP this is a single desktop control surface, such as:

- `toggleMainPanel()`

No broad Electron APIs should be exposed to page code.

### Renderer layers

- `floating.html` and its script own the floating ball UI only
- `index.html`, `app.js`, `logic.js`, and `style.css` remain the main panel experience

This keeps the existing prototype mostly intact while giving desktop behavior a separate boundary.

## 6. File Structure

Expected new or modified files:

- `package.json` - Electron dependency and start script
- `main.js` - Electron main process window creation and IPC handling
- `preload.js` - minimal secure bridge for the floating window
- `floating.html` - floating ball renderer
- `floating.js` - floating ball click handling
- `index.html` - minor adjustments if needed for panel framing
- `style.css` - minor adjustments so the current page feels like a desktop panel instead of a full browser page
- `app.js` - only small changes if desktop-specific assumptions require them

## 7. Window Behavior

### Floating ball

- Starts visible
- Uses a compact circular visual treatment
- Sends a toggle request to the main process on click
- Does not attempt to run the prototype workflow itself

### Main panel

- Starts hidden
- Loads the existing prototype UI from local files
- Uses a fixed MVP size large enough for the current sections, with an initial target around `960 x 720`
- Appears adjacent to the floating ball, preferably to the left and slightly above it
- Hides when toggled off
- Keeps renderer state across hide/show cycles during the same app session

For this MVP, window position can be computed from the floating ball window bounds and clamped to stay on screen.

## 8. Security And Simplicity Constraints

- `contextIsolation` should stay enabled
- `nodeIntegration` should remain disabled in renderers
- All renderer-to-main communication should go through preload
- The desktop shell should not introduce file-system or shell execution features

These constraints keep the shell minimal and aligned with the prototype goal.

## 9. Error Handling

- If the main panel window does not exist at toggle time, recreate it and then show it
- If a renderer preload bridge is unavailable, the floating ball may fail quietly for MVP; a visible fallback label is optional and should not block implementation
- If the main panel is manually closed, the app should recreate or re-show it on the next floating ball click instead of exiting entirely

## 10. Verification Criteria

The desktop shell is successful if the following are true:

1. `npm install` completes
2. `npm start` launches the Electron app
3. The floating ball appears as a separate desktop window
4. Clicking the floating ball shows the main panel window
5. Clicking the floating ball again hides the main panel window
6. The main panel still runs the existing prototype workflow correctly
7. Closing or hiding the panel does not remove the floating ball

## 11. Explicit Non-Goals

This design intentionally avoids product-complete desktop behavior. It should not expand into:

- tray lifecycle management
- drag memory
- sticky edges
- DND modes
- hover expansion
- system startup registration
- native task inspection or native automation

## 12. Why This Design

This is the smallest path from the current web prototype to a believable desktop MVP:

- it creates a real desktop entry point
- it reuses the working prototype instead of rewriting it
- it keeps Electron scope narrow to window management
- it preserves the current core loop while making the product feel materially closer to the PRD
