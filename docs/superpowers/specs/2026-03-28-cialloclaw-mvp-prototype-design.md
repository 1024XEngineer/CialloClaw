# CialloClaw MVP Minimal Prototype Design

## 1. Goal

Build the simplest runnable prototype from `PRD.md` that proves the core value loop:

`task detected -> summary shown -> user confirms -> draft generated -> log recorded`

The prototype should be easy to open locally, easy to demo, and intentionally avoid nonessential engineering.

## 2. Prototype Scope

### In scope

- Single-page local web prototype
- Floating ball entry on the page
- Chat-style action panel
- Mock task inspection area with 2-3 built-in tasks
- Draft/result area
- Execution log area
- Minimal confirm-before-execute interaction
- Minimal undo for the last generated result

### Out of scope

- Real desktop shell or always-on-top system floating window
- Real file watching or Markdown parsing
- Real model/API integration
- Real local permissions, workspace sandboxing, or system actions
- Real rollback beyond UI-level undo
- Voice, plugins, multi-agent, or advanced memory

## 3. Success Criteria

The prototype is considered successful if a user can:

1. Open the prototype locally in a browser
2. Click the floating ball to open and close the assistant panel
3. Run task inspection and see a generated summary
4. Confirm execution and generate a draft
5. See execution records appended to the log
6. Undo the last generated result once

## 4. User Flow

### Primary loop

1. User opens the page
2. Floating ball shows there are pending tasks
3. User clicks the floating ball
4. Assistant panel opens with available actions
5. User clicks `Run Inspection`
6. System filters built-in tasks for urgent/follow-up items and generates a summary
7. User clicks `Generate Draft`
8. System first asks for lightweight confirmation
9. User confirms
10. System generates a draft in the result area and appends a log entry
11. User can click `Undo Last Action` to clear the latest result and append another log entry

### Guardrails

- If the user tries to generate a draft before inspection, show a message asking them to run inspection first
- If no pending tasks exist, show an empty-state summary instead of failing
- All actions, including failures or blocked actions, should create log entries

## 5. UI Structure

The page contains five visible regions:

### Floating ball

- Fixed near the bottom-right corner
- Shows a compact status like `2 tasks`
- Toggles the assistant panel open/closed

### Assistant panel

- Main interaction surface
- Contains three primary actions:
  - `Run Inspection`
  - `Generate Draft`
  - `View Log`
- Shows short system hints and confirmation prompts, including the lightweight confirm-before-execute state for draft generation
- Also contains an `Undo Last Action` control that is enabled only after a draft has been generated

### Task inspection area

- Displays the built-in task list
- Each task shows title, due date, status, priority, and suggested action
- Inspection updates task emphasis and creates a summary block

### Result area

- Displays the inspection summary first and then shows the generated draft in a separate block below the summary
- Keeps content readable and demo-friendly

### Execution log

- Shows timestamped records in reverse chronological order
- Includes action name and outcome status

## 6. Technical Approach

### Stack

- `HTML`
- `CSS`
- Vanilla `JavaScript`

### File layout

Preferred implementation:

- `index.html`
- `style.css`
- `app.js`

This keeps the code readable while staying lightweight. If implementation ends up small enough, the assets may be collapsed further, but multiple files are acceptable.

## 7. State Model

The page keeps only the minimum client-side state needed for the demo:

- `isPanelOpen`
- `tasks`
- `inspectionHasRun`
- `inspectionSummary`
- `generatedDraft`
- `pendingConfirmation`
- `logs`

## 8. Data Model

Built-in tasks will be hard-coded in JavaScript as parsed task-like objects, for example:

- title
- dueAt
- status
- priority
- suggestedAction

At least one task should always satisfy the inspection rule during demos. This can be done by using dates relative to the current day or by keeping one high-priority pending task in the built-in data.

This intentionally simulates the post-parse state of a Markdown task file without implementing actual file ingestion.

## 9. Behavior Details

### Run Inspection

- Reviews the built-in task array
- Selects tasks that are still pending and either due today or marked high priority by a simple hard-coded rule
- Generates a plain-language summary
- Updates the UI to highlight relevant tasks
- Appends a success log entry

### Generate Draft

- Requires inspection to have run first
- First click sets a lightweight confirmation state
- Confirmation click generates a fixed-template draft based on the current summary/tasks
- Writes the draft to a draft block below the existing inspection summary
- Appends a success log entry

### Undo Last Action

- Is triggered from the assistant panel after a draft exists
- Clears only the most recent generated draft block
- Keeps the inspection summary and existing log history intact
- Appends an undo log entry

### View Log

- Scrolls or focuses the log section
- Optionally highlights the most recent record

## 10. Error Handling

- `Generate Draft` before inspection: show inline guidance and log a blocked action
- No pending tasks: show `No tasks need follow-up right now` and log a no-op success/info event
- Undo when nothing was generated: show inline guidance and log a blocked action
- Unexpected JS error: show a lightweight fallback message in the panel and preserve the existing log/history where possible

## 11. Visual Direction

The prototype should feel distinct enough to communicate product intent without adding engineering weight:

- Floating ball is visually prominent but compact
- Panel feels like a lightweight assistant, not a full dashboard
- Main page can resemble a control room with clear sections for tasks, result, and log
- Styling should be clean and intentional, but implementation should stay simple and static

## 12. Testing Approach

Manual verification is sufficient for this prototype.

Test checklist:

1. Open the page locally without a build step
2. Toggle the floating ball panel
3. Run inspection and confirm summary text appears
4. Generate draft only after confirmation
5. Verify log entries appear for inspection and draft generation
6. Undo the last result and verify the log updates
7. Try invalid action order and verify helpful inline feedback appears

## 13. Why This Design

This design maps to the MVP intent in `PRD.md` while staying deliberately minimal:

- Preserves the floating-ball entry concept
- Demonstrates task inspection instead of generic chat
- Shows confirm-before-execute behavior to reinforce trust
- Shows traceability through logs
- Avoids heavy platform work so the prototype can be built and demoed quickly

## 14. Implementation Boundary

This spec is only for the minimum prototype. It should not expand into:

- Desktop runtime packaging
- Real filesystem monitoring
- Real policy engine or rollback layer
- Multi-step agent orchestration
- Any feature that prevents same-session completion
