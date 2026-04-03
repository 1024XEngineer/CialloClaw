# CialloClaw MVP

> A lightweight desktop Agent assistant focused on controllable execution, task inspection, and transparent history.

CialloClaw is an Electron-based desktop application that provides a floating ball launcher for task inspection and active assistance. It helps users complete high-frequency, low-to-medium complexity tasks while maintaining trust through verifiable, rollback-capable execution.

## Features

- **Floating Ball Launcher** - Low-friction desktop entry point that stays resident
- **Task Inspection** - Periodic scanning of markdown task files with reminders and summaries
- **Active Assistance** - Context-aware help triggered by user actions (text selection, file upload, errors)
- **Security-First Execution** - Risk分级, confirmations, audit logs, and rollback capabilities
- **Web Control Panel** - Dashboard for status, logs, rules, and settings

## Getting Started

### Prerequisites

- Node.js 18+
- npm

### Installation

```bash
npm install
```

### Run

```bash
npm start
```

### Test

```bash
npm test
```

## Project Structure

```
.
├── main.js              # Electron main process
├── preload.js            # Preload script for IPC
├── index.html           # Main control panel UI
├── floating.html        # Floating ball UI
├── app.js               # Main panel application logic
├── logic.js             # Business logic
├── desktop-shell.js     # Desktop shell utilities
├── main-controller.js   # Main panel window controller
├── floating-launcher-lifecycle.js  # Floating ball lifecycle
├── style.css            # Main panel styles
├── floating.css         # Floating ball styles
├── PRD.md               # Product Requirements Document
└── tests/               # Test files
```

## Tech Stack

- Electron 36
- Vanilla JavaScript
- CSS

## License

Private
