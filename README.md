# CialloClaw

## Orb Weave Prototype

Orb Weave is a Tauri desktop prototype for a gesture-first floating orb assistant.

It avoids the usual left-click / right-click desktop widget pattern and instead treats the orb like a small edge-docked agent that reacts to motion, hover, voice, and tray behavior.

## What It Tries To Prove

- A floating assistant can stay on the desktop without constantly blocking work
- Gesture semantics can replace traditional mouse-button semantics
- Voice can be a primary input, not just an extra button
- Small, in-place previews can reduce context switching before opening a full chat

## Core Interaction Model

When the orb is idle, it peeks from the nearest screen edge.

- Hover `3s`: show three predicted questions
- Scroll wheel: switch agent mode between `work`, `life`, and `create`
- Light drag: move orb, then auto-snap to nearest edge
- Swipe down: hide to system tray
- Long press: start voice capture
- Long press + drag up: open full chat panel
- Long press + circle: open orbit menu
- Long press + drag away from edge: open orb status panel

## Agent Modes

The orb changes color, copy, suggested questions, and menu emphasis based on its current mode.

- `work`: execution, priority, task breakdown
- `life`: reminders, lists, lightweight companionship
- `create`: ideas, titles, outlines, expansion

## Voice Flow

Voice is designed to be direct.

- Hold the orb to start speech recognition
- Release after speaking to submit directly to the AI flow
- A mini thinking card appears near the orb
- The thinking card turns into a small streaming reply card
- Clicking the reply card opens the full chat panel with details

Current AI replies are mocked in the frontend so the prototype can demonstrate the interaction loop before wiring a real model endpoint.

## Panels In The Prototype

The prototype uses adaptive window sizing instead of mixing unrelated overlays and full windows.

- `idle-peek`: small orb only
- `preview`: hover prediction, voice preview, thinking, mini reply
- `chat`: full conversation surface
- `menu`: orbit actions
- `status`: orb state page

## Tray Behavior

The app integrates with the system tray.

- Swipe down to hide the orb into the tray
- Left-click tray icon to restore the orb
- Right-click tray icon to open a compact control menu

Tray menu items are intentionally system-like and action-first:

- `状态：...`
- `暂停` / `继续`
- `打开控制面板`
- `隐藏`
- `退出程序`

## Tech Stack

- Tauri 2
- Vite
- Vanilla JavaScript
- CSS
- Rust

## Project Structure

- `src/main.js`: interaction logic, gesture handling, state, mock streaming AI flow
- `src/style.css`: visual system, dock behavior, motion, panel styling
- `src-tauri/src/main.rs`: native window layout, tray integration, visibility control
- `src-tauri/tauri.conf.json`: Tauri app configuration

## Run Locally

Install dependencies:

```bash
npm install
```

Run the web preview only:

```bash
npm run dev
```

Run the desktop app:

```bash
npm run tauri dev
```

Build frontend assets:

```bash
npm run build
```

## Requirements

You need a desktop Tauri toolchain available locally.

- Node.js
- npm
- Rust toolchain
- platform-specific Tauri prerequisites for Windows

## Current Prototype Notes

- Speech recognition currently depends on Web Speech API availability in the host WebView
- Mini reply streaming is mocked on the client side
- The product direction is interaction-first, so several panels intentionally optimize for feel over production completeness

## Good Next Steps

- Replace mock AI replies with a real streaming model API
- Persist conversation and status history
- Add multi-monitor aware docking memory
- Add a richer status panel with device, mic, and tray diagnostics
- Turn menu cards into real executable actions
