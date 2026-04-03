# Desktop AI Assistant Frontend Interaction Prototype Design

## Overview

This document defines a desktop-first AI assistant interaction prototype focused on using screen capture and region selection as the primary input. The product is designed as a cross-application productivity tool rather than a chat interface.

The prototype only covers frontend interface and interaction behavior. It does not include backend logic, model orchestration, permission handling, or implementation details for OCR, vision, or system APIs.

## Design Goals

- Emphasize desktop efficiency and cross-application usage
- Make screen region selection the core input method
- Avoid chat-product mental models and message-stream layouts
- Provide clear visual hierarchy and state transitions
- Keep the style simple, modern, and professional

## Core Product Concept

The assistant behaves like a screen-aware desktop tool:

1. A floating orb stays available across applications
2. The user activates capture or region-selection mode
3. The user selects any area of the screen
4. The assistant offers contextual AI actions for that selected area
5. Results first appear as concise, actionable cards near the source
6. A side panel provides expanded detail and follow-up actions when needed

The main workflow is:

`Floating orb -> Quick actions -> Full-screen selection -> Action menu -> Processing -> Result card -> Side panel -> Re-select / Continue`

## Recommended Design Direction

Recommended approach: workflow-first interaction design.

Why this direction:

- Best fits a desktop utility product
- Makes screen selection the center of the experience
- Demonstrates the full state chain clearly in prototype reviews
- Supports both fast answers and deeper follow-up handling

Alternative approaches considered but not chosen:

- Command-palette-first: strong for power users, but weakens the screen-input narrative
- Lightweight toolbar-first: efficient, but too limited for multi-step analysis and follow-up actions

## Information Architecture

The interface is organized into four layers:

### 1. Persistent Entry Layer

Contains the always-available floating orb.

### 2. Ephemeral Action Layer

Contains quick menus and contextual action menus close to the trigger point or selected region.

### 3. Task Feedback Layer

Contains processing indicators and result cards for immediate feedback.

### 4. Deep Reading Layer

Contains the right-side detail panel for expanded analysis, history, and continued actions.

## Surface Model and Windowing Rules

To keep the prototype unambiguous during review, all primary UI surfaces are defined as desktop overlays rather than elements inside a standalone application window.

Surface rules:

- The floating orb is a persistent always-on-top desktop overlay
- The quick menu, capture overlay, post-selection action menu, processing card, and result card are all transient desktop overlays anchored to the orb or selected region
- The side panel is also an always-on-top desktop overlay docked to the right edge, not a separate application window
- The side panel may visually read like a utility drawer, but it should still feel connected to the same overlay system as the orb and result cards

Persistence rules:

- The capture overlay disappears after selection is confirmed or cancelled
- Transient menus and cards may auto-dismiss when their task is complete or when the user clicks away
- The side panel is the only long-lived surface and may stay open while the user continues working in other apps

This model ensures the experience remains consistently cross-application from entry through follow-up reading.

## Primary Screens and States

### 1. Floating Orb Default State

Purpose: Provide a low-friction, always-available desktop entry point.

Behavior:

- Stays docked near the lower-right area by default
- Supports drag-and-drop repositioning with edge snapping
- Maintains low visual prominence when idle
- On hover, shows a subtle glow and a tooltip such as `Select an area for AI analysis`

Visual guidance:

- Size: 52-56px
- Shape: circle
- Material: lightly frosted surface with subtle shadow
- Content: brand mark or simple `AI` label

### 2. Activated Quick Actions Menu

Purpose: Expose the highest-frequency entry actions without forcing the user into a larger workspace.

Behavior:

- Opens from the floating orb as a compact anchored menu
- Keeps the main recommendation as the first item
- Avoids broad navigation and keeps focus on capture workflows

Recommended items:

- `Region Analyze`
- `Capture Text`
- `Recent Result`
- `Open Side Panel`

Menu rules:

- Compact vertical stack
- Each item includes icon, label, and short helper text
- No search box by default

### 3. Screen Capture / Region Selection Overlay

Purpose: Move the user into a cross-application capture mode while preserving awareness of the current desktop context.

Behavior:

- Applies a darkened full-screen overlay while keeping the underlying screen visible
- Switches cursor to crosshair
- Shows a slim top instruction bar with guidance such as `Drag to select · Esc cancel · Space capture full screen`
- Hides unnecessary product UI while in capture mode

Overlay rules:

- The product should feel like it is operating above all apps
- The user should never feel taken into a separate app canvas

### 4. Selection In-Progress Feedback

Purpose: Make the selected area visually precise and easy to confirm.

Behavior:

- The selected region remains fully visible at normal brightness
- The surrounding area darkens
- A highlighted border outlines the active region
- Corner markers improve precision
- A compact size label appears in the region, e.g. `824 x 516`

Optional smart hints:

- `Text-heavy`
- `Chart detected`
- `Code / error likely`

These hints are only used as assistive cues and should not compete with the primary selection feedback.

### 5. Post-Selection Action Menu

Purpose: Let the user choose the AI job immediately after selection, without typing.

Behavior:

- Appears adjacent to the selected region after mouse release
- Does not auto-run analysis without explicit user intent
- Promotes one primary action and several secondary task shortcuts

Recommended actions:

- Primary: `Analyze Content`
- Secondary: `Recognize Text`, `Explain Chart`, `Analyze Error`, `Summarize Page`, `Generate Reply`
- Utility: `Reselect`, `Pin to Side Panel`

Layout rules:

- Primary action should be visually distinct
- Secondary actions should be quick-scan pills or compact buttons
- Menu should avoid covering the selection target when possible

### 6. Processing State

Purpose: Confirm task takeover and reduce uncertainty while analysis is running.

Behavior:

- Replaces the action menu with a lightweight processing card near the selected region
- Shows short stage-based progress instead of a fake percentage bar
- Supports `Cancel` and `Run in Background`

Recommended processing stages:

- `Capturing area`
- `Understanding structure`
- `Preparing result`

Interaction rules:

- The card may be moved aside if it blocks relevant content
- The UI should clearly differentiate in-progress work from completed results

`Run in Background` behavior:

- Selecting `Run in Background` collapses the processing card into a compact task chip attached to the floating orb
- The orb gains a subtle active badge and a tooltip such as `1 task running`
- Clicking the orb while a background task exists exposes a `Resume Current Task` entry at the top of the quick menu
- When processing completes, the orb shows a completed badge until the user opens the result
- Opening the completed task restores the result card near the previous selection anchor when possible; otherwise it opens directly in the side panel

### 7. Result Card State

Purpose: Deliver a concise, actionable answer near the source region.

Behavior:

- Appears near the selected area, automatically avoiding overlap where possible
- Prioritizes short conclusions and next actions over long prose
- Acts as a fast-read layer, not the full workspace

Recommended structure:

- Title
- 3-5 lines of core output
- Suggested next actions

Examples by task:

- For `Analyze Error`: likely cause, first checks, suggested fix direction
- For `Explain Chart`: main trend, anomaly, likely interpretation
- For `Summarize Page`: page gist, key points, optional takeaways
- For `Generate Reply`: one or more ready-to-use response suggestions

Recommended footer actions:

- `Explain More`
- `Change Angle`
- `Generate Steps`
- `Copy`
- `Open Details`

### 8. Side Panel Detail State

Purpose: Support deeper reading, structured detail, history, and continued processing.

Behavior:

- Opens as a fixed right-side panel
- Shows the current selection thumbnail and timestamp at the top
- Uses sectioned content instead of a chat transcript
- Supports follow-up transformations without requiring a text conversation layout

Recommended panel sections:

- `Summary`
- `Detailed Breakdown`
- `Suggested Actions`
- `Related History`

Panel principles:

- Current task stays pinned at the top
- Scrolling happens inside the panel only
- The original selected region remains visually connected to the detail result through thumbnail and task label

### 9. Re-Selection and Continue-Processing State

Purpose: Support iterative screen-based workflows without collapsing back to a blank start.

Behavior:

- Available from both the result card and the side panel
- Lets the user re-select a new region while preserving task context when appropriate
- Supports second-step transformations of the current result

Examples:

- Re-select another error region while staying in `Analyze Error`
- Turn `Summarize Page` into `Generate Reply`
- Turn `Analyze Error` into `Step-by-step Checklist`

Rule:

The system should preserve context when it reduces friction, but always make the active task label visible so the user understands what is being continued.

Context persistence rules:

- Preserve the task context on `Reselect` when the user is repeating the same analysis intent, such as `Analyze Error` on a different error block or `Recognize Text` on another snippet
- Reset the task context when the user explicitly chooses a different top-level action from the post-selection action menu
- Preserve the source result context on `Continue Processing`, but label the new step as a derived action, for example `Based on: Summarize Page -> Generate Reply`
- If the prior context would be confusing on a newly selected area, the UI should show the old task label as a suggestion rather than auto-apply it

## Non-Happy-Path States

The prototype should include lightweight failure and edge-case states so the workflow feels complete during review.

### Invalid or Too-Small Selection

- If the selected area is below a practical threshold, show a compact warning near the selection: `Selection too small - choose a larger area`
- Keep the user in selection mode and allow immediate retry without resetting the full overlay

### Unsupported or Low-Confidence Content

- If the content is visually ambiguous or not suitable for the selected task, show a neutral notice in the result card, such as `This area is hard to interpret for chart analysis`
- Offer recovery actions: `Try Recognize Text`, `Reselect Area`, `Open Details`

### No Useful Result

- If the output is weak, the card should say so directly instead of generating filler text
- Example: `No clear error information found in this selection`
- Provide next steps: `Select Larger Area`, `Choose Another Action`, `Open Side Panel`

### Analysis Failure

- If analysis fails, replace the processing card or result card with an error state that explains the task could not complete
- Include recovery actions: `Retry`, `Run in Background`, `Reselect`
- Use a calm, utility-style tone rather than alarming language

These non-happy-path states should be represented in review artifacts even if they are only shown as secondary frames.

## End-to-End Interaction Flow

### Step 1. Idle

The floating orb remains available with minimal interruption.

### Step 2. Activate

The user clicks the orb and opens the quick menu.

### Step 3. Select

The user chooses region analysis and enters the full-screen overlay to drag-select content.

### Step 4. Choose Intent

The user picks a task from the contextual post-selection action menu.

### Step 5. Process

The UI shows a lightweight processing card close to the selected source.

### Step 6. Read Fast Result

The result card appears near the source and prioritizes a short, useful answer.

### Step 7. Expand if Needed

The user opens the side panel for deeper detail.

### Step 8. Continue

The user either re-selects another screen area or transforms the current result into another useful output.

## Visual System

### Style Direction

The product should feel like a modern desktop utility, not a social or chat product.

Keywords:

- concise
- professional
- modern
- efficient
- cross-application

### Color Strategy

- Neutral surfaces: light gray-white cards
- Text: deep gray instead of pure black
- Overlay: deep translucent slate/navy
- Accent: single cool blue or blue-cyan highlight

Suggested palette:

- Background card: `#F7F8FA`
- Primary text: `#111827`
- Secondary text: `#6B7280`
- Accent: `#3B82F6`
- Accent hover: `#2563EB`
- Overlay: `rgba(15, 23, 42, 0.42)`

### Typography

Recommended direction:

- Clean desktop-oriented sans-serif
- Chinese UI fonts such as `PingFang SC`, `HarmonyOS Sans SC`, or `MiSans`
- Strong distinction between title, action label, body, and auxiliary hint text

### Material and Elevation

- Light frosted surfaces for floating components
- Thin borders to keep elements crisp
- Soft shadows, never heavy or decorative
- Minimal blur and minimal glossy effects

## Layout and Sizing Guidance

- Floating orb: 56px diameter
- Quick actions menu: 220-240px width
- Menu row height: 44-48px
- Processing card: 220-260px width
- Result card: 320-380px width
- Side panel: 380-420px width

Layout rules:

- Small UI stays near the trigger or selected area
- Only the side panel serves as the deep-reading container
- Only one primary result card should remain visible at a time
- All temporary overlays should avoid excessive screen occupation

## Motion Principles

The motion language should be restrained and purposeful.

Recommended transitions:

- Quick menu: short anchored expand
- Action menu: pop near selection edge
- Result card: fade in with slight upward motion
- Side panel: slide in from the right

Timing:

- 120-220ms for most transitions

Avoid:

- Typing animations
- Chat bubble streaming
- Decorative bounce effects

## Interaction Rules and Shortcuts

- `Esc`: cancel the current lightweight state
- `Enter`: confirm the primary action when focused
- Global shortcut may be supported, such as `Alt + Space`
- The system may remember the user’s last action type when repeating similar tasks
- `Recent Result` should be available without forcing a new capture flow

## Content Strategy

The UI should prioritize actionability over explanation.

Preferred content style:

- Short labels
- Immediate conclusions
- Clear next steps

Preferred action labels:

- `Recognize Text`
- `Explain Chart`
- `Analyze Error`
- `Summarize Page`
- `Generate Reply`

Avoid vague or over-general labels.

## Prototype Scope

Included in prototype:

- Visual states and transitions
- Layering model
- Key interaction paths
- Action menu logic at the UI level
- Re-selection and continued processing flows

Explicitly out of scope:

- Backend or model behavior
- Permission prompts and OS integration details
- OCR and vision implementation
- Real progress estimation
- User account, sync, or history persistence logic

## Success Criteria for Review

The prototype is successful if reviewers can immediately understand:

- this is a desktop-native, cross-application assistant
- screen region selection is the primary input method
- the product is not organized like a chat app
- the workflow supports both quick answers and deeper follow-up handling
- each state has clear visual hierarchy and user feedback

## Suggested Prototype Frames

For a review-ready high-fidelity prototype, create at least these frames:

1. Floating orb default state
2. Floating orb hover state
3. Quick actions menu
4. Full-screen selection overlay
5. Selection dragging state
6. Post-selection action menu
7. Processing state
8. Result card near source
9. Side panel open state
10. Re-select flow with preserved task context
11. Continue-processing flow from an existing result
12. Background processing collapsed into orb state
13. Invalid or too-small selection state
14. No useful result or analysis failure state

## Final Recommendation

Prototype the experience as a screen-native workflow tool. The fastest path to clarity is to make the selected region the source of truth for every subsequent UI state: action choice, processing feedback, result presentation, detail expansion, and follow-up handling.
