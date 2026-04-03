# CialloClaw Dashboard And Stitched Chat Layout Design

## 1. Goal

Upgrade the current desktop-shell main panel from a simple split workspace into a more intentional product surface:

- the opened main panel should feel like a dashboard inspired by `仪表盘.png`
- the chat side should feel like a dedicated messaging surface inspired by `聊天界面.png`
- the two areas should read as stitched side-by-side panels inside one main window, not as a cramped side column squeezed into the old layout

This remains a minimal prototype. The goal is visual and structural improvement while preserving existing interaction behavior.

## 2. Scope

### In scope

- Rework the main panel layout into a dashboard-style left area and a dedicated right chat area
- Use `仪表盘.png` as the visual reference for the left-side dashboard area
- Use `聊天界面.png` as the visual reference for the right-side chat area
- Keep the current floating avatar entry point unchanged
- Preserve existing left-side workflow behavior:
  - inspection
  - summary
  - draft generation
  - execution log
  - undo
- Preserve existing right-side mock chat behavior:
  - welcome message
  - send message
  - quick actions

### Out of scope

- pixel-perfect reproduction of the two PNGs
- extra Electron windows
- model integration
- charts backed by real external data
- complex cross-panel orchestration
- animated transitions beyond minimal polish

## 3. Product Shape

The main panel becomes a single stitched workspace with two clear zones:

### Left dashboard zone

- visually references `仪表盘.png`
- looks like the product's main operational dashboard
- organizes current controls and results into a more deliberate dashboard structure

### Right chat zone

- visually references `聊天界面.png`
- feels like a dedicated conversation interface, not a generic card sidebar
- contains the persistent assistant conversation flow

The two zones should feel intentionally paired, with a clean seam between them, rather than appearing as one narrow aside shoved into the existing layout.

## 4. Layout

### Overall shell

- A single horizontal workspace inside the main Electron panel
- Left dashboard zone takes the larger share of width
- Right chat zone takes a smaller but still substantial share of width
- Both zones should have their own visual identity while still belonging to one product surface

### Left dashboard zone

The left side should be reorganized into a dashboard-style composition, for example:

- top status/header area
- workflow control block
- task/summary/draft cards
- execution log area

The exact card arrangement can adapt to implementation needs, but it should feel materially closer to `仪表盘.png` than the current simple stacked card layout.

### Right chat zone

The right side should be a dedicated chat shell with:

- a recognizable chat header
- message area
- quick actions or assistant shortcuts
- compose area at the bottom

It should read as its own panel visually, not as a small utility sidebar.

## 5. Workflow Mapping

### Left side responsibilities

- `assistant-panel` remains the workflow control surface
- `assistant-message` remains the workflow status message
- existing workflow buttons stay available:
  - `run-inspection`
  - `generate-draft`
  - `undo-last-action`
  - `view-log`
- task inspection, summary, draft, and execution log remain on the left side

### Right side responsibilities

- `chat-panel` remains the conversation surface
- `chat-messages`, `chat-input`, `chat-send`, and quick actions remain on the right side
- welcome message, typed send flow, and quick actions continue to use local mock behavior

## 6. Interaction Model

1. User clicks the floating avatar
2. Main panel opens
3. User immediately sees:
   - a dashboard-style left zone
   - a dedicated chat-style right zone
4. User can continue using the existing workflow actions on the left
5. User can chat on the right without collapsing or obscuring the dashboard
6. The two zones may reference shared state, but they remain visually independent and intentionally arranged

## 7. Visual Direction

### Left dashboard inspiration

Use `仪表盘.png` as inspiration for:

- dashboard hierarchy
- card grouping
- spacing rhythm
- visual emphasis areas
- overall “operations console” feeling

### Right chat inspiration

Use `聊天界面.png` as inspiration for:

- chat shell framing
- message list styling
- compose area structure
- separation between header, conversation area, and input area

### Important constraint

These images are references, not background images to paste in as dead UI.

The final layout should be recreated as interactive HTML/CSS so the prototype remains usable.

## 8. Implementation Constraints

- Prefer reworking `index.html` and `style.css` rather than rewriting the app architecture
- Preserve existing DOM ids wherever practical so `app.js` logic can remain mostly intact
- Adjust `app.js` only where the new layout needs small rendering or presentation updates
- Do not open another Electron window for chat

## 9. Verification Criteria

The feature is successful if the following are true:

1. The main panel feels visually closer to `仪表盘.png` on the left and `聊天界面.png` on the right
2. The left and right sides read as stitched adjacent panels rather than a cramped main area plus a narrow sidebar
3. The left-side workflow still works:
   - inspection
   - summary
   - draft generation
   - log updates
   - undo
4. The right-side chat still works:
   - welcome message
   - typed send
   - quick actions
5. The floating avatar still opens and hides the main panel

## 10. Why This Design

This is the smallest design-oriented upgrade that responds to the requested direction:

- it keeps one main panel window
- it preserves existing behavior and IDs
- it upgrades the experience from “functional split layout” to “dashboard + dedicated chat interface”
- it uses the provided image references as style guides without sacrificing interactivity
