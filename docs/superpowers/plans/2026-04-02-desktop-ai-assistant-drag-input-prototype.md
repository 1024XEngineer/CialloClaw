# Desktop AI Assistant Drag Input Prototype Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a frontend-only desktop-style AI assistant prototype that demonstrates dragging objects into a floating orb, contextual actions, structured results, detail viewing, and unsupported/error states without any backend.

**Architecture:** Build a small Vite + React + TypeScript single-page app. Keep a single reducer-driven UI state machine for the drag-input flow, mock object fixtures, action mappings, result templates, and the right-side context rail so the prototype stays deterministic and easy to demo. Prioritize the PDF happy path first, then generalize the same shell to image, text, link, unsupported, and error previews.

**Tech Stack:** Vite, React, TypeScript, CSS variables, Vitest, React Testing Library, Playwright

---

## Scope and Delivery Order

Implement in this order so the prototype keeps its main selling point front and center:

1. Ship the first-drag PDF happy path on the left desktop stage.
2. Add the floating-orb state changes and contextual action panel.
3. Add result cards and the right-side rail mode switch.
4. Generalize to image/text/link fixtures and add unsupported/error coverage.
5. Add verification and motion polish only after the drag-input loop is clear.

Do not add a chat transcript, freeform composer, history sidebar, account settings, or backend stubs.

## Workspace Placement Contract

Treat this prototype as a new standalone app rooted in the current directory while preserving the existing docs tree already under `docs/`:

- create `package.json`, `tsconfig.json`, `vite.config.ts`, `playwright.config.ts`, and `index.html` at the workspace root
- place all app source code under `src/`
- leave `docs/` in place and only add/update the spec and plan files there
- do not nest the prototype under another subdirectory unless a human explicitly changes this contract later

## Git Checkpoint Note

The current directory is not a git repo. Treat every commit step in this plan as:

- if `git status` works, run the listed commit commands
- if `git status` fails, skip the commit command and record a short local checkpoint note instead
- do not initialize a new git repo unless a human explicitly asks

## Layout and Visual Guardrails

Lock these requirements before implementation so the prototype does not drift from the design spec:

- Use a two-column shell at desktop widths `>= 1280px` with an approximate `65 / 35` split. A good default is `grid-template-columns: minmax(0, 1.86fr) minmax(360px, 1fr)`.
- Keep the orb fixed inside the left stage at `right: 56px` and `bottom: 112px`, which reads as `右下偏中` instead of a browser chat badge.
- Set the orb to a default size between `56px` and `64px`; use `60px` unless a later accessibility review requires an increase.
- Keep the result card inside the left stage; the right rail is only for gallery and detail modes.
- Build the desktop background from a light linear gradient plus a soft blur layer so the scene reads like a desktop work surface, not a flat browser canvas.
- When the drag ghost enters `nearby`, `hover`, `unsupported-nearby`, or `unsupported-hover`, add a subtle stage-dimming overlay and reduce contrast on non-active objects so the orb becomes the clear drop target.
- Use neutral system tokens only: graphite/stone text, cool white panels, gray-blue highlights, muted amber warnings. Do not introduce purple brand gradients, chat bubbles, mascot avatars, or full-bleed neon glows.
- Use medium radii (`18px` to `24px`), 1px borders, and soft shadows so the UI reads like a desktop productivity tool.

Add final verification for these in Playwright and manual review; do not treat them as subjective polish.

## Desktop Layout Placement Contract

Use a single relative-positioned stage in `src/app/components/DesktopStage.tsx` with these default desktop coordinates at `1440px` width:

- `product-pdf` -> `top: 88px; left: 72px`
- `whiteboard-image` -> `top: 132px; left: 336px`
- `meeting-note` -> `top: 322px; left: 112px`
- `research-link` -> `top: 472px; left: 300px`
- `archive-zip` -> `top: 248px; left: 560px`
- result card anchor -> `right: 148px; bottom: 208px`

Keep these placements proportional when the stage scales down; at `1024px`, preserve the same relative ordering even if absolute values shrink.

## File Structure Map

- `package.json` - scripts and dependencies for Vite, tests, and Playwright
- `tsconfig.json` - TypeScript compiler configuration
- `vite.config.ts` - Vite + Vitest configuration
- `playwright.config.ts` - end-to-end test setup
- `index.html` - app mount node
- `src/main.tsx` - React bootstrap
- `src/app/App.tsx` - top-level layout and reducer wiring
- `src/app/model/types.ts` - shared types for samples, actions, card templates, and UI states
- `src/app/model/mockData.ts` - fixed sample objects, action lists, result fixtures, state gallery metadata
- `src/app/model/motion.ts` - shared motion timing constants used by timers and CSS variables
- `src/app/model/reducer.ts` - prototype state machine and transitions
- `src/app/components/ObjectTray.tsx` - fixed sample object tray and explicit demo controls
- `src/app/components/DesktopStage.tsx` - desktop scene with all 5 visible sample objects, drag ghost, and result card placement
- `src/app/components/FloatingOrb.tsx` - orb visuals and state-dependent copy
- `src/app/components/ActionPanel.tsx` - recognized object panel, suggested actions, unsupported copy
- `src/app/components/ResultCard.tsx` - structured output card variants
- `src/app/components/ContextRail.tsx` - right-side container that swaps gallery/detail modes
- `src/app/components/StateGallery.tsx` - status list synced to the live stage state
- `src/app/components/DetailView.tsx` - expanded result reader
- `src/app/components/ErrorNotice.tsx` - reusable error and unsupported helper block
- `src/styles/tokens.css` - color, radius, spacing, and shadow tokens
- `src/styles/global.css` - resets and page frame
- `src/styles/prototype.css` - component styling and motion rules
- `src/test/setup.ts` - Vitest DOM setup
- `src/app/App.test.tsx` - smoke test for the shell
- `src/app/model/reducer.test.ts` - state machine coverage
- `src/app/components/DesktopStage.test.tsx` - tray and stage behavior tests
- `src/app/components/FloatingOrb.test.tsx` - orb and action panel state tests
- `src/app/components/ContextRail.test.tsx` - processing/result/detail rail tests
- `src/app/components/ActionPanel.test.tsx` - per-object actions and unsupported/error tests
- `tests/e2e/drag-input.spec.ts` - PDF happy path and unsupported drag flow

## UI Label and Selector Contract

Implement these labels and selectors exactly so tests and implementation stay aligned:

- Tray labels: `PDF`, `图片`, `文本`, `链接`, `压缩包`
- Review controls: `一键演示`, `模拟识别失败`
- Landmark labels: `桌面交互主舞台`, `右侧上下文栏`, `悬浮球`
- Action-panel header copy: `已识别：<对象名>` on the first line, object meta such as `12 页 PDF` on the second line
- Test ids: `prototype-shell`, `desktop-active-sample`, `action-panel`, `result-card`
- Desktop object ids: `sample-product-pdf`, `sample-whiteboard-image`, `sample-meeting-note`, `sample-research-link`, `sample-archive-zip`
- Gallery ids: `gallery-item-idle`, `gallery-item-nearby`, `gallery-item-hover`, `gallery-item-recognized`, `gallery-item-actions`, `gallery-item-processing`, `gallery-item-result`, `gallery-item-detail`, `gallery-item-error`

Mounting visibility contract:

- `action-panel` stays mounted but uses `hidden` when status is not `actions`, `processing`, `unsupported`, or `error`
- `result-card` stays mounted but uses `hidden` when status is not `result` or `detail`

## Demo Button Contract

Lock the `一键演示` behavior in `src/app/App.tsx` so tests do not guess:

- It always runs against the current `activeSample`.
- For supported samples, it simulates `nearby -> hover -> recognized -> actions` and then stops.
- For unsupported samples, it simulates `unsupported-nearby -> unsupported-hover -> unsupported` and then stops.
- It never auto-runs `chooseAction`, `processing`, `result`, or `detail`.
- It does not switch samples, rewrite copy, or bypass the same reducer events used by real drag input.

## App State Shape Contract

Define `PrototypeState` in `src/app/model/types.ts` with these exact fields so `src/app/App.tsx` wiring is not guessed:

```ts
type PrototypeState = {
  status: PrototypeStatus;
  railMode: 'gallery' | 'detail';
  activeSample: SampleDefinition;
  suggestedActions: ActionDefinition[];
  activeActionId: string | null;
  activeResult: ResultDefinition | null;
  recognitionLabel: string | null;
  recognitionSummary: string | null;
  errorReasonVisible: boolean;
  highlightedGalleryState:
    | 'gallery-item-idle'
    | 'gallery-item-nearby'
    | 'gallery-item-hover'
    | 'gallery-item-recognized'
    | 'gallery-item-actions'
    | 'gallery-item-processing'
    | 'gallery-item-result'
    | 'gallery-item-detail'
    | 'gallery-item-error';
  dragGhost: {
    sourceId: string;
    sourceType: 'tray' | 'stage';
    x: number;
    y: number;
    snapped: boolean;
  } | null;
};
```

Define `PrototypeAction` with at least these events: `selectSample`, `startDrag`, `moveDrag`, `enterOrbRange`, `hoverOrb`, `leaveOrbRange`, `dropOnOrb`, `finishRecognition`, `chooseAction`, `finishProcessing`, `continueFromResult`, `openDetail`, `closeDetail`, `triggerErrorPreview`, `retryAfterError`, `resetScene`.

Add one more explicit recovery event: `viewErrorReason`.

Use these action ids everywhere in `ActionDefinition[]`, reducer transitions, and result maps: `summary`, `extract`, `qa`, `organize`, `ocr`, `translate`, `brief`, `link-summary`.

Use a helper like `getInitialActions(sample)` so `createInitialState(sample)` and `selectSample` both preload the mapped actions for the current sample kind while the UI stays visually idle.

## Component Contract Summary

Lock these prop/render contracts before implementation so later tasks stay bite-sized:

- `src/app/components/ObjectTray.tsx`
  - props: `samples`, `activeSampleId`, `onSelect(sampleId)`, `onStartDrag(sampleId, sourceType, point)`, `onRunDemo()`, `onTriggerErrorPreview()`
  - renders: five object labels + `一键演示` + `模拟识别失败`
- `src/app/components/DesktopStage.tsx`
  - props: `samples`, `activeSampleId`, `dragGhost`, `status`, `activeResult`, `onStartDrag(sampleId, sourceType, point)`, `onPointerMove(point)`, `onPointerEnd(point)`
  - renders: all five visible object cards, one active card with `data-testid="desktop-active-sample"`, stage overlay, orb mount slot, result-card slot
- `src/app/components/ActionPanel.tsx`
  - props: `sample`, `status`, `actions`, `activeActionId`, `contextMode: 'fresh' | 'from-result'`, `onChooseAction(actionId)`, `onRetry()`, `onViewReason()`
  - renders: recognized-object header, object meta line, action buttons, unsupported helper copy, or error recovery actions
- `src/app/components/ResultCard.tsx`
  - props: `result`, `sample`, `onOpenDetail()`, `onContinue()`, `onCopy()`
  - renders: template-specific content; always anchored inside the stage
- `src/app/components/ContextRail.tsx`
  - props: `mode`, `highlightedState`, `galleryItems`, `detailPayload`, `onCloseDetail()`, `onTriggerErrorPreview()`, `onContinueFromDetail()`, `onCopyResult()`
  - renders: gallery mode or detail mode only; switches internally based on `mode`
- `src/app/components/StateGallery.tsx`
  - props: `items`, `activeItemId`, `onTriggerErrorPreview()`
  - renders: exact title/description pairs from the gallery contract plus footer trigger `模拟识别失败`
- `src/app/components/DetailView.tsx`
  - props: `sample`, `result`, `onClose()`, `onContinue()`, `onCopy()`
  - renders: header, original object info, full result, follow-up actions in that order

## State Gallery Highlight Map

Map every runtime state to exactly one gallery item:

- `idle` -> `gallery-item-idle` (`默认态`)
- `nearby` and `unsupported-nearby` -> `gallery-item-nearby` (`靠近高亮态`)
- `hover` and `unsupported-hover` -> `gallery-item-hover` (`拖拽悬停态`)
- `recognized` -> `gallery-item-recognized` (`对象识别态`)
- `actions` -> `gallery-item-actions` (`动作选择面板`)
- `processing` -> `gallery-item-processing` (`处理中态`)
- `result` -> `gallery-item-result` (`结果卡片态`)
- `detail` -> `gallery-item-detail` (`侧边详情态`)
- `unsupported` and `error` -> `gallery-item-error` (`错误 / 不支持`)

## Motion Timing Contract

Define these exact values in `src/app/model/motion.ts` and mirror them into CSS variables in `src/styles/tokens.css`:

- `nearbyFeedbackMs = 140` (`120-160ms` band)
- `snapScaleMs = 200` (`180-220ms` band)
- `recognitionBeatMs = 750` (`600-900ms` band)
- `processingBeatMs = 900`
- `resultExpandMs = 220` (`200-260ms` band)

Use the motion constants in both JS timers and CSS custom properties so tests can verify them without guessing.

## Fixed Mock Fixtures

Define these exact fixtures in `src/app/model/mockData.ts` so every state uses the same copy and density:

- `product-pdf`
  - label: `产品方案.pdf`
  - kind: `pdf`
  - meta: `12 页 PDF`, `4.8 MB`
  - summary card: 3 条重点 + 1 段摘要 + `展开详情` / `复制结果` / `继续处理`
- `whiteboard-image`
  - label: `白板拍照.png`
  - kind: `image`
  - meta: `2480 x 1640`, `2.1 MB`
  - OCR card: 识别文本预览 + 关键信息块 + `复制结果` / `整理内容`
- `meeting-note`
  - label: `会议摘录.txt`
  - kind: `text`
  - preview lines: `下周一完成首页方案评审` / `确认 OCR 体验文案`
  - translation card: 原文/译文对照 + 语言标签 + `复制译文` / `提取重点`
- `research-link`
  - label: `https://example.com/ai-desktop-workflow`
  - kind: `link`
  - meta: `example.com`, `网页链接`
  - brief card: 3 条网页要点 + `打开详情` / `整理内容`
- `archive-zip`
  - label: `项目资料.zip`
  - kind: `unsupported`
  - meta: `18.2 MB`, `压缩文件`
  - drop result: `暂不支持该格式` + `可尝试拖入 PDF、图片、文本或链接`
- `error-preview`
  - tied sample: `product-pdf`
  - message: `未能完成文档识别，请重试`
  - actions: `重试` / `查看原因`
  - reason: `原因：当前原型使用固定 mock 错误说明`

## Authoritative Action Map

Implement these exact action sets in `src/app/model/mockData.ts` and reuse them everywhere the UI renders actions:

- `pdf` -> `summary: 总结 PDF` / `extract: 提取重点` / `qa: 生成问答` / `organize: 整理内容`
- `image` -> `ocr: OCR 图片` / `extract: 提取重点` / `organize: 整理内容`
- `text` -> `translate: 翻译文本` / `extract: 提取重点` / `organize: 整理内容`
- `link` -> `brief: 提炼网页要点` / `link-summary: 生成摘要` / `organize: 整理内容`
- `unsupported` -> no action buttons; only helper copy and recovery guidance

## Recognition Copy Contract

Use these exact recognition labels and summary lines during the `recognized` beat:

- `product-pdf` -> `识别为 PDF` + `产品方案.pdf · 12 页 PDF`
- `whiteboard-image` -> `识别为图片` + `白板拍照.png · 2480 x 1640`
- `meeting-note` -> `识别为文本` + `下周一完成首页方案评审 / 确认 OCR 体验文案`
- `research-link` -> `识别为链接` + `example.com · 网页链接`
- `archive-zip` -> `暂不支持该格式` + `项目资料.zip · 压缩文件`

## Result Template Catalog

Define these exact mock outputs in `src/app/model/mockData.ts` so every advertised action has a concrete fixture:

- `总结 PDF` -> title `PDF 总结`; body `3 条重点 + 1 段摘要 + 按钮: 展开详情 / 复制结果 / 继续处理`
- `提取重点` -> title `重点提取`; body `重点 1 / 重点 2 / 重点 3 + 按钮: 复制结果 / 整理内容`
- `生成问答` -> title `示例问答`; body `3 组问答卡片 + 按钮: 展开详情 / 继续处理`
- `整理内容` -> title `整理内容`; body `分组小节 + 待办清单 + 按钮: 复制结果 / 展开详情`
- `OCR 图片` -> title `图片文字提取`; body `识别文本预览 + 关键信息块 + 按钮: 复制结果 / 整理内容`
- `翻译文本` -> title `文本翻译`; body `原文 / 译文 / 语言标签 + 按钮: 复制译文 / 提取重点`
- `提炼网页要点` -> title `网页要点`; body `3 条网页要点 + 来源域名 + 按钮: 展开详情 / 整理内容`
- `生成摘要` -> title `网页摘要`; body `1 段摘要 + 2 条重点 + 按钮: 展开详情 / 整理内容`

## Detail View Contract

`src/app/components/DetailView.tsx` must render these sections in order:

1. Header row: `完整结果` title + close button `关闭详情`
2. Original object info: object label, object meta, object type pill
3. Full result content: render the full mock content that matches the current action template
4. Follow-up actions row: at least `继续处理` and `复制结果`, plus one template-specific action when available

## State Gallery Copy

Define these exact gallery titles and one-line descriptions in `src/app/model/mockData.ts` and render them in `src/app/components/StateGallery.tsx`:

- `默认态` - `悬浮球等待对象输入`
- `靠近高亮态` - `对象进入接收范围`
- `拖拽悬停态` - `松手即可开始识别`
- `对象识别态` - `展示类型与对象摘要`
- `动作选择面板` - `按对象类型给出快捷动作`
- `处理中态` - `当前动作正在执行`
- `结果卡片态` - `先看结构化结果`
- `侧边详情态` - `查看完整结果与后续动作`
- `错误 / 不支持` - `提示原因与可恢复操作`

## State Contract

Use one state enum in `src/app/model/types.ts`:

```ts
export type PrototypeStatus =
  | 'idle'
  | 'nearby'
  | 'hover'
  | 'unsupported-nearby'
  | 'unsupported-hover'
  | 'recognized'
  | 'actions'
  | 'processing'
  | 'result'
  | 'detail'
  | 'unsupported'
  | 'error';
```

State rules:

- Clicking an object in the tray only changes the selected sample and resets to `idle`.
- Pointer-down on a tray item must also be able to start a drag directly from the tray; the tray is both a switcher and a valid drag source.
- Switching samples must also return the right rail to gallery mode, reactivate the first gallery card (`默认态`), clear any result card, and refresh the mapped actions for the newly selected sample.
- Dragging a supported object into orb range moves `idle -> nearby -> hover`.
- Dragging `archive-zip` into orb range moves `idle -> unsupported-nearby -> unsupported-hover`; the orb should show warning copy and amber styling instead of the success highlight.
- Releasing a supported object on the orb moves `hover -> recognized` for `600-900ms`, shows the object type plus object summary, then advances to `actions`.
- Releasing `archive-zip` from `unsupported-hover` moves to `unsupported` and never shows action buttons.
- Leaving the orb range at any time moves `nearby`, `hover`, `unsupported-nearby`, or `unsupported-hover` back to `idle` and removes snap styling.
- Choosing an action moves `actions -> processing -> result`.
- Clicking `继续处理` on a result card reopens `actions` with the same sample and current result context; do not open a chat composer.
- Opening detail moves the right rail to detail mode while the root status becomes `detail`.
- Triggering the review-only failure preview moves to `error` without introducing backend logic.

Interaction geometry rules:

- Compute orb proximity from the distance between the drag ghost center and orb center.
- Use a `nearbyRadius` of `140px` and a `hoverRadius` of `76px`.
- If distance `> 140px`, return to `idle`; if `<= 140px` and `> 76px`, move to `nearby` or `unsupported-nearby`; if `<= 76px`, move to `hover` or `unsupported-hover`.
- Represent `吸附` by offsetting the drag ghost `12px` toward the orb center and adding a `data-snapped="true"` attribute once the ghost enters `hover` or `unsupported-hover`.

Timer ownership rules:

- Keep the reducer pure. Do not place timers inside `src/app/model/reducer.ts`.
- In `src/app/App.tsx`, add one `useEffect` that watches `state.status === 'recognized'` and dispatches `finishRecognition` after `750ms`.
- In `src/app/App.tsx`, add another `useEffect` that watches `state.status === 'processing'` and dispatches `finishProcessing` after `MOTION.processingBeatMs` (`900ms`).
- Clear both timers on cleanup so switching samples or resetting the scene cannot leak stale transitions.

## Task 1: Bootstrap the App Shell

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `vite.config.ts`
- Create: `playwright.config.ts`
- Create: `index.html`
- Create: `src/main.tsx`
- Create: `src/app/App.tsx`
- Create: `src/styles/tokens.css`
- Create: `src/styles/global.css`
- Create: `src/styles/prototype.css`
- Create: `src/test/setup.ts`
- Test: `src/app/App.test.tsx`

- [ ] **Step 1: Create the project manifest, test config, and a failing shell test**

```json
{
  "name": "desktop-ai-assistant-prototype",
  "private": true,
  "type": "module",
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@playwright/test": "^1.52.0",
    "@testing-library/jest-dom": "^6.6.3",
    "@testing-library/react": "^16.3.0",
    "@testing-library/user-event": "^14.6.1",
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.4.1",
    "jsdom": "^26.0.0",
    "typescript": "^5.8.3",
    "vite": "^6.3.5",
    "vitest": "^3.1.3"
  },
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:e2e": "playwright test"
  }
}
```

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src"]
}
```

```ts
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts'
  }
});
```

```ts
export default defineConfig({
  use: { baseURL: 'http://127.0.0.1:4173' },
  webServer: {
    command: 'npm run dev -- --host 127.0.0.1 --port 4173',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: !process.env.CI
  }
});
```

```tsx
import { render, screen } from '@testing-library/react';
import App from './App';

it('renders the desktop stage shell', () => {
  render(<App />);
  expect(screen.getByRole('heading', { name: '对象即输入' })).toBeInTheDocument();
  expect(screen.getByLabelText('桌面交互主舞台')).toBeInTheDocument();
  expect(screen.getByLabelText('右侧上下文栏')).toBeInTheDocument();
});
```

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>对象即输入原型</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './app/App';
import './styles/tokens.css';
import './styles/global.css';
import './styles/prototype.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

```ts
import '@testing-library/jest-dom/vitest';
```

```css
:root {
  --motion-nearby: 140ms;
  --motion-snap: 200ms;
  --motion-recognition: 750ms;
  --motion-processing: 900ms;
  --motion-result: 220ms;
  --stage-blur: 18px;
}
```

```css
* { box-sizing: border-box; }
html, body, #root { height: 100%; }
body { margin: 0; font-family: "Segoe UI", "PingFang SC", sans-serif; }
button, input, textarea { font: inherit; }
```

```css
.app-shell { min-height: 100%; }
```

- [ ] **Step 2: Install dependencies and run the test to verify it fails**

Run: `npm install && npm run test -- src/app/App.test.tsx`
Expected: FAIL with a missing shell heading or missing labeled regions.

- [ ] **Step 3: Implement the minimal app shell**

```tsx
export default function App() {
  return (
    <main className="app-shell">
      <section aria-label="桌面交互主舞台">
        <h1>对象即输入</h1>
      </section>
      <aside aria-label="右侧上下文栏" />
    </main>
  );
}
```

- [ ] **Step 4: Run the shell test and a production build**

Run: `npm run test -- src/app/App.test.tsx && npm run build`
Expected: PASS for the test and a successful Vite build.

- [ ] **Step 5: Commit**

```bash
git add package.json tsconfig.json vite.config.ts playwright.config.ts index.html src/main.tsx src/app/App.tsx src/styles/tokens.css src/styles/global.css src/styles/prototype.css src/test/setup.ts src/app/App.test.tsx
git commit -m "chore: bootstrap desktop prototype shell"
```

## Task 2: Define Mock Data and the Reducer State Machine

**Files:**
- Create: `src/app/model/types.ts`
- Create: `src/app/model/mockData.ts`
- Create: `src/app/model/motion.ts`
- Create: `src/app/model/reducer.ts`
- Test: `src/app/model/reducer.test.ts`

- [ ] **Step 1: Write failing reducer tests for the main state rules**

```ts
import { samples } from './mockData';
import { createInitialState, prototypeReducer } from './reducer';

it('moves a pdf sample from drop to actions', () => {
  let state = createInitialState();
  state = prototypeReducer(state, { type: 'selectSample', sampleId: 'product-pdf' });
  state = prototypeReducer(state, { type: 'enterOrbRange' });
  state = prototypeReducer(state, { type: 'hoverOrb' });
  state = prototypeReducer(state, { type: 'dropOnOrb' });

  expect(state.status).toBe('recognized');
  expect(state.activeSample.id).toBe('product-pdf');
  expect(state.recognitionLabel).toBe('识别为 PDF');
});

it('advances from recognized to actions after the recognition beat', () => {
  let state = createInitialState();
  state = prototypeReducer(state, { type: 'selectSample', sampleId: 'product-pdf' });
  state = prototypeReducer(state, { type: 'enterOrbRange' });
  state = prototypeReducer(state, { type: 'hoverOrb' });
  state = prototypeReducer(state, { type: 'dropOnOrb' });
  state = prototypeReducer(state, { type: 'finishRecognition' });

  expect(state.status).toBe('actions');
  expect(state.suggestedActions[0].label).toBe('总结 PDF');
});

it('moves the zip sample to unsupported on release', () => {
  let state = createInitialState();
  state = prototypeReducer(state, { type: 'selectSample', sampleId: 'archive-zip' });
  state = prototypeReducer(state, { type: 'enterOrbRange' });

  expect(state.status).toBe('unsupported-nearby');

  state = prototypeReducer(state, { type: 'hoverOrb' });
  state = prototypeReducer(state, { type: 'dropOnOrb' });

  expect(state.status).toBe('unsupported');
  expect(state.suggestedActions).toHaveLength(0);
});

it('returns to idle when the drag leaves the orb range', () => {
  let state = createInitialState();
  state = prototypeReducer(state, { type: 'enterOrbRange' });
  state = prototypeReducer(state, { type: 'hoverOrb' });
  state = prototypeReducer(state, { type: 'leaveOrbRange' });

  expect(state.status).toBe('idle');
});

it('resets the scene while preserving the current sample contract', () => {
  let state = createInitialState(samples['whiteboard-image']);
  state = prototypeReducer(state, { type: 'enterOrbRange' });
  state = prototypeReducer(state, { type: 'resetScene' });

  expect(state.status).toBe('idle');
  expect(state.activeSample.id).toBe('whiteboard-image');
  expect(state.suggestedActions[0].id).toBe('ocr');
});

it('switches the rail into detail mode and back', () => {
  let state = createInitialState();
  state = prototypeReducer(state, { type: 'selectSample', sampleId: 'product-pdf' });
  state = prototypeReducer(state, { type: 'enterOrbRange' });
  state = prototypeReducer(state, { type: 'hoverOrb' });
  state = prototypeReducer(state, { type: 'dropOnOrb' });
  state = prototypeReducer(state, { type: 'finishRecognition' });
  state = prototypeReducer(state, { type: 'chooseAction', actionId: 'summary' });
  state = prototypeReducer(state, { type: 'finishProcessing' });
  state = prototypeReducer(state, { type: 'openDetail' });

  expect(state.status).toBe('detail');
  expect(state.railMode).toBe('detail');

  state = prototypeReducer(state, { type: 'closeDetail' });

  expect(state.status).toBe('result');
  expect(state.railMode).toBe('gallery');
});

it('returns to actions with result context when continuing from a result card', () => {
  let state = createInitialState(samples['product-pdf']);
  state = prototypeReducer(state, { type: 'chooseAction', actionId: 'summary' });
  state = prototypeReducer(state, { type: 'finishProcessing' });
  state = prototypeReducer(state, { type: 'continueFromResult' });

  expect(state.status).toBe('actions');
  expect(state.activeResult?.title).toBe('PDF 总结');
  expect(state.highlightedGalleryState).toBe('gallery-item-actions');
});
```

- [ ] **Step 2: Run the reducer tests to verify they fail**

Run: `npm run test -- src/app/model/reducer.test.ts`
Expected: FAIL because the reducer and fixtures do not exist yet.

- [ ] **Step 3: Implement sample fixtures in `src/app/model/mockData.ts`**

Acceptance checks:

- define all five sample objects plus `error-preview`
- fixture copy matches the fixed mock fixtures section exactly

- [ ] **Step 4: Implement action maps, recognition copy, result templates, and gallery metadata in `src/app/model/mockData.ts`**

Acceptance checks:

- include the authoritative action map ids and labels
- include the recognition copy contract and gallery titles/descriptions
- include result templates for every advertised action

- [ ] **Step 5: Implement motion constants in `src/app/model/motion.ts` and `createInitialState(sample)` in `src/app/model/reducer.ts`**

Acceptance checks:

- export all motion constants from the timing contract
- `createInitialState(sample)` fills every `PrototypeState` field explicitly

- [ ] **Step 6: Implement reducer transitions in `src/app/model/reducer.ts`**

Acceptance checks:

- include drag, recognition, processing, continue-from-result, error preview, and error-reason transitions
- keep the reducer pure with no timers inside it

```ts
export const MOTION = {
  nearbyFeedbackMs: 140,
  snapScaleMs: 200,
  recognitionBeatMs: 750,
  processingBeatMs: 900,
  resultExpandMs: 220
} as const;

function getInitialActions(sample: SampleDefinition): ActionDefinition[] {
  return sample.kind === 'unsupported' ? [] : actionMap[sample.kind];
}

export function createInitialState(sample = samples['product-pdf']): PrototypeState {
  return {
    status: 'idle',
    activeSample: sample,
    suggestedActions: getInitialActions(sample),
    activeActionId: null,
    activeResult: null,
    recognitionLabel: null,
    recognitionSummary: null,
    errorReasonVisible: false,
    railMode: 'gallery',
    highlightedGalleryState: 'gallery-item-idle',
    dragGhost: null
  };
}

export function prototypeReducer(state: PrototypeState, action: PrototypeAction): PrototypeState {
  switch (action.type) {
    case 'selectSample':
      return createInitialState(samples[action.sampleId]);
    case 'startDrag':
      return {
        ...state,
        dragGhost: {
          sourceId: action.sourceId,
          sourceType: action.sourceType,
          x: action.x,
          y: action.y,
          snapped: false
        }
      };
    case 'moveDrag':
      return state.dragGhost
        ? { ...state, dragGhost: { ...state.dragGhost, x: action.x, y: action.y } }
        : state;
    case 'enterOrbRange':
      return state.activeSample.kind === 'unsupported'
        ? { ...state, status: 'unsupported-nearby', highlightedGalleryState: 'gallery-item-nearby' }
        : { ...state, status: 'nearby', highlightedGalleryState: 'gallery-item-nearby' };
    case 'hoverOrb':
      return state.activeSample.kind === 'unsupported'
        ? {
            ...state,
            status: 'unsupported-hover',
            highlightedGalleryState: 'gallery-item-hover',
            dragGhost: state.dragGhost ? { ...state.dragGhost, snapped: true } : null
          }
        : {
            ...state,
            status: 'hover',
            highlightedGalleryState: 'gallery-item-hover',
            dragGhost: state.dragGhost ? { ...state.dragGhost, snapped: true } : null
          };
    case 'leaveOrbRange':
      return { ...state, status: 'idle', dragGhost: null, highlightedGalleryState: 'gallery-item-idle' };
    case 'dropOnOrb':
      return state.activeSample.kind === 'unsupported'
        ? { ...state, status: 'unsupported', suggestedActions: [], highlightedGalleryState: 'gallery-item-error' }
        : {
            ...state,
            status: 'recognized',
            highlightedGalleryState: 'gallery-item-recognized',
            recognitionLabel: recognitionMap[state.activeSample.id].title,
            recognitionSummary: recognitionMap[state.activeSample.id].summary
          };
    case 'finishRecognition':
      return {
        ...state,
        status: 'actions',
        suggestedActions: actionMap[state.activeSample.kind],
        highlightedGalleryState: 'gallery-item-actions'
      };
    case 'chooseAction':
      return {
        ...state,
        status: 'processing',
        activeActionId: action.actionId,
        highlightedGalleryState: 'gallery-item-processing'
      };
    case 'finishProcessing':
      return {
        ...state,
        status: 'result',
        activeResult: resultMap[state.activeSample.id][state.activeActionId!],
        highlightedGalleryState: 'gallery-item-result'
      };
    case 'continueFromResult':
      return {
        ...state,
        status: 'actions',
        activeActionId: null,
        highlightedGalleryState: 'gallery-item-actions'
      };
    case 'openDetail':
      return { ...state, status: 'detail', railMode: 'detail', highlightedGalleryState: 'gallery-item-detail' };
    case 'closeDetail':
      return { ...state, status: 'result', railMode: 'gallery', highlightedGalleryState: 'gallery-item-result' };
    case 'triggerErrorPreview':
      return {
        ...state,
        status: 'error',
        railMode: 'gallery',
        errorReasonVisible: false,
        highlightedGalleryState: 'gallery-item-error'
      };
    case 'retryAfterError':
      return {
        ...state,
        status: 'actions',
        errorReasonVisible: false,
        highlightedGalleryState: 'gallery-item-actions'
      };
    case 'viewErrorReason':
      return { ...state, errorReasonVisible: true };
    case 'resetScene':
      return createInitialState(state.activeSample);
    default:
      return state;
  }
}
```

- [ ] **Step 7: Run the reducer tests again**

Run: `npm run test -- src/app/model/reducer.test.ts`
Expected: PASS for PDF, unsupported, and detail-mode transition tests.

- [ ] **Step 8: Commit**

```bash
git add src/app/model/types.ts src/app/model/mockData.ts src/app/model/motion.ts src/app/model/reducer.ts src/app/model/reducer.test.ts
git commit -m "feat: add prototype state machine and fixtures"
```

## Task 3: Build the Tray and First-Drag Stage Flow

**Files:**
- Create: `src/app/components/ObjectTray.tsx`
- Create: `src/app/components/DesktopStage.tsx`
- Modify: `src/styles/prototype.css`
- Modify: `src/app/App.tsx`
- Test: `src/app/components/DesktopStage.test.tsx`

- [ ] **Step 1: Write a failing stage test for tray selection and explicit demo triggering**

```tsx
import { render, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from '../App';

it('changes the selected sample without auto-running the flow', async () => {
  const user = userEvent.setup();
  render(<App />);

  await user.click(screen.getByRole('button', { name: '图片' }));

  expect(screen.getByText('白板拍照.png')).toBeInTheDocument();
  expect(screen.getByText('待拖入对象')).toBeInTheDocument();
  expect(screen.queryByText('已识别：白板拍照.png')).not.toBeInTheDocument();
});

it('switches every sample without auto-running recognition or results', async () => {
  const user = userEvent.setup();
  render(<App />);

  for (const label of ['PDF', '图片', '文本', '链接', '压缩包']) {
    await user.click(screen.getByRole('button', { name: label }));
    expect(screen.getByTestId('gallery-item-idle')).toHaveAttribute('data-active', 'true');
    expect(screen.queryByText(/已识别：/)).not.toBeInTheDocument();
    expect(screen.queryByText('PDF 总结')).not.toBeInTheDocument();
  }
});

it('shows all five sample objects and resets the rail when switching samples', async () => {
  const user = userEvent.setup();
  render(<App />);

  expect(screen.getByText('产品方案.pdf')).toBeInTheDocument();
  expect(screen.getByText('白板拍照.png')).toBeInTheDocument();
  expect(screen.getByText('会议摘录.txt')).toBeInTheDocument();
  expect(screen.getByText('https://example.com/ai-desktop-workflow')).toBeInTheDocument();
  expect(screen.getByText('项目资料.zip')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '一键演示' }));
  await user.click(screen.getByRole('button', { name: '图片' }));

  expect(screen.getByTestId('gallery-item-idle')).toHaveAttribute('data-active', 'true');
  expect(screen.queryByText('PDF 总结')).not.toBeInTheDocument();
});

it('can start a drag directly from the tray', () => {
  render(<App />);

  fireEvent.pointerDown(screen.getByRole('button', { name: 'PDF' }), { clientX: 96, clientY: 88 });

  expect(screen.getByTestId('desktop-active-sample')).toHaveAttribute('data-drag-source', 'tray');
});

it('advances the pdf happy path through real drag events', () => {
  render(<App />);

  const sample = screen.getByTestId('desktop-active-sample');
  const orb = screen.getByLabelText('悬浮球');

  fireEvent.pointerDown(sample, { clientX: 220, clientY: 260 });
  fireEvent.pointerMove(document, { clientX: 980, clientY: 580 });
  expect(screen.getByText('松手处理')).toBeInTheDocument();
  fireEvent.pointerUp(orb, { clientX: 1012, clientY: 612 });

  expect(screen.getByText('识别为 PDF')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the stage test to verify it fails**

Run: `npm run test -- src/app/components/DesktopStage.test.tsx`
Expected: FAIL because the tray and stage components do not exist yet.

- [ ] **Step 3: Implement `src/app/components/ObjectTray.tsx` with switch + drag-source behavior**

Build the fixed tray first:

- Render the exact five labels from the selector contract
- Support click-to-select and pointer-down-to-start-drag on the same control
- Keep `一键演示` separate from the object labels

Acceptance checks:

- tray buttons use the exact visible labels from the selector contract
- `一键演示` and `模拟识别失败` are not mixed into the sample label list

- [ ] **Step 4: Implement `src/app/components/DesktopStage.tsx` with all five visible desktop objects**

Build the stage object layer next:

- Render all five fixtures in desktop positions
- Mark the current active object with `待拖入对象`
- Keep the inactive four visible but lower emphasis

Acceptance checks:

- each object renders with its exact sample test id
- the active object also renders `data-testid="desktop-active-sample"`

- [ ] **Step 5: Implement `handleSelectSample` and `handleRunDemo` in `src/app/App.tsx`**

Lock these handlers exactly:

- `handleSelectSample(sampleId)` dispatches `selectSample`
- `handleRunDemo()` follows the demo-button contract and stops at `actions` or `unsupported`

Acceptance checks:

- selecting a sample resets the rail highlight to `gallery-item-idle`
- demo flow does not auto-run processing or result states

- [ ] **Step 6: Implement drag-start and drag-move handlers in `src/app/App.tsx`**

Lock these handlers exactly:

- `handleStartDrag(sampleId, sourceType, point)` dispatches `startDrag`
- `handleMoveDrag(point)` dispatches `moveDrag` and proximity events
- `handleEndDrag(point)` dispatches `dropOnOrb` or `leaveOrbRange`

Acceptance checks:

- drag geometry uses the nearby/hover radii from the state contract
- tray drags set `sourceType: 'tray'`; stage drags set `sourceType: 'stage'`

- [ ] **Step 7: Wire the final prop names in `src/app/App.tsx` and match the component contracts exactly**

```tsx
<ObjectTray
  samples={traySamples}
  activeSampleId={state.activeSample.id}
  onSelect={handleSelectSample}
  onStartDrag={handleStartDrag}
  onRunDemo={handleRunDemo}
  onTriggerErrorPreview={handleTriggerErrorPreview}
/>

<DesktopStage
  samples={traySamples}
  activeSampleId={state.activeSample.id}
  status={state.status}
  dragGhost={state.dragGhost}
  activeResult={state.activeResult}
  onStartDrag={handleStartDrag}
  onPointerMove={handleMoveDrag}
  onPointerEnd={handleEndDrag}
/>
```

Implementation notes:

- Use a `示例对象托盘` pinned to the top-left of the stage.
- Render all 5 sample objects directly on the desktop stage at once so the stage itself communicates `对象即输入`; use the tray as a second, fixed switcher instead of the only source of objects.
- Mark one sample as the active `待拖入对象`, but keep the other 4 visible as dormant desktop objects.
- Clicking a tray item swaps the active object and resets to `idle`; pointer-down on that same tray item starts a drag ghost with `sourceType: 'tray'`.
- Add a separate `一键演示` button that follows the demo-button contract for the current active sample.
- Render a page-level drag ghost card on pointer down so the interaction feels like a desktop drop, not a form upload.
- Detect orb entry by comparing the drag ghost center against the orb bounding box center; dispatch `enterOrbRange`, `hoverOrb`, and `leaveOrbRange` from those thresholds.
- When the drag ghost enters `hover`, translate it slightly toward the orb center and set `data-snapped="true"` so CSS can show the吸附 effect without changing the underlying sample source.
- Apply a light gradient + blur background treatment on the stage and add a dimming overlay plus reduced opacity on non-active objects during nearby/hover states.

- [ ] **Step 8: Add the light gradient + blur desktop background in `src/styles/prototype.css`**

- [ ] **Step 9: Add the stage dimming, non-active object de-emphasis, and drag-snap styling in `src/styles/prototype.css`**

- [ ] **Step 10: Run the stage test again**

Run: `npm run test -- src/app/components/DesktopStage.test.tsx`
Expected: PASS with the tray selection behavior locked in.

- [ ] **Step 11: Commit**

```bash
git add src/app/components/ObjectTray.tsx src/app/components/DesktopStage.tsx src/styles/prototype.css src/app/App.tsx src/app/components/DesktopStage.test.tsx
git commit -m "feat: add tray and desktop stage scaffold"
```

## Task 4: Add Floating Orb States and the Action Panel

**Files:**
- Create: `src/app/components/FloatingOrb.tsx`
- Create: `src/app/components/ActionPanel.tsx`
- Modify: `src/app/components/DesktopStage.tsx`
- Modify: `src/app/App.tsx`
- Modify: `src/styles/prototype.css`
- Test: `src/app/components/FloatingOrb.test.tsx`

- [ ] **Step 1: Write a failing test for orb feedback and PDF action suggestions**

```tsx
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';
import App from '../App';

it('shows the recognition beat before the full pdf action set', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: 'PDF' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));

  expect(screen.getByText('识别为 PDF')).toBeInTheDocument();
  expect(screen.getByText('产品方案.pdf · 12 页 PDF')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '总结 PDF' })).not.toBeInTheDocument();

  vi.advanceTimersByTime(900);

  expect(screen.getByText('已识别：产品方案.pdf')).toBeInTheDocument();
  expect(screen.getByText('12 页 PDF')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '总结 PDF' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '提取重点' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '生成问答' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '整理内容' })).toBeInTheDocument();
  expect(screen.getByTestId('action-panel')).toHaveAttribute('data-side', 'right');
});

it('warns on unsupported files before release', () => {
  render(<App />);

  fireEvent.pointerDown(screen.getByText('项目资料.zip'));
  fireEvent.pointerMove(screen.getByLabelText('悬浮球'));

  expect(screen.getByText('暂不支持该格式')).toBeInTheDocument();
  expect(screen.getByLabelText('悬浮球')).toHaveAttribute('data-status', 'unsupported-hover');
  expect(screen.getByLabelText('悬浮球')).toHaveAttribute('data-eligibility', 'unsupported');
  expect(screen.getByLabelText('悬浮球')).not.toHaveAttribute('data-status', 'hover');
});
```

- [ ] **Step 2: Run the orb test to verify it fails**

Run: `npm run test -- src/app/components/FloatingOrb.test.tsx`
Expected: FAIL because the orb states and action panel are not implemented.

- [ ] **Step 3: Implement `src/app/components/FloatingOrb.tsx` for supported and unsupported orb visuals**

Acceptance checks:

- expose `data-status` and `data-eligibility`
- show `松手处理` on hover and warning copy on unsupported hover

- [ ] **Step 4: Implement `src/app/components/ActionPanel.tsx` for recognized header + supported actions**

Acceptance checks:

- render `data-testid="action-panel"`
- render `已识别：<对象名>` and the meta line
- render the exact PDF action list in the right-side panel

- [ ] **Step 5: Wire `src/app/App.tsx` and `src/app/components/DesktopStage.tsx` to mount the orb and panel together**

```tsx
<FloatingOrb
  status={state.status}
  sample={state.activeSample}
  activeActionId={state.activeActionId}
/>

<ActionPanel
  status={state.status}
  sample={state.activeSample}
  actions={state.suggestedActions}
  activeActionId={state.activeActionId}
  contextMode={state.activeResult ? 'from-result' : 'fresh'}
  onChooseAction={handleChooseAction}
  onRetry={handleRetry}
  onViewReason={handleViewReason}
/>
```

Implementation notes:

- Drive visuals from `data-status` attributes: `idle`, `nearby`, `hover`, `unsupported-nearby`, `unsupported-hover`, `recognized`, `actions`, `processing`, `result`, `unsupported`, `error`.
- Add `data-eligibility="supported|unsupported"` on the orb so tests can verify unsupported flows never borrow the supported treatment.
- Include `unsupported-nearby` and `unsupported-hover` so unsupported objects warn early and never borrow the supported glow treatment.
- Show `松手处理` while hovering, then a distinct recognition beat with object type and summary before actions appear.
- For PDF, always render the exact four actions from the spec: `总结 PDF` / `提取重点` / `生成问答` / `整理内容`.
- For `archive-zip`, move to `unsupported` on release and render helper copy instead of action buttons.
- Mount the action panel in the orb wrapper and tag it with `data-side="right"`; place it with a right-side anchor offset such as `left: calc(100% + 16px)` so implementation and tests agree on the panel origin.

- [ ] **Step 6: Run the orb test again**

Run: `npm run test -- src/app/components/FloatingOrb.test.tsx`
Expected: PASS with the orb and PDF action panel working.

- [ ] **Step 7: Commit**

```bash
git add src/app/components/FloatingOrb.tsx src/app/components/ActionPanel.tsx src/app/components/DesktopStage.tsx src/app/App.tsx src/styles/prototype.css src/app/components/FloatingOrb.test.tsx
git commit -m "feat: add orb states and action panel"
```

## Task 5: Add Processing, Result Cards, and the Rail Mode Switch

**Files:**
- Create: `src/app/components/ResultCard.tsx`
- Create: `src/app/components/ContextRail.tsx`
- Create: `src/app/components/StateGallery.tsx`
- Create: `src/app/components/DetailView.tsx`
- Modify: `src/app/App.tsx`
- Modify: `src/styles/prototype.css`
- Test: `src/app/components/ContextRail.test.tsx`

- [ ] **Step 1: Write a failing test for the PDF processing-to-detail flow**

```tsx
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';
import App from '../App';

it('shows processing, then a result card, then detail mode', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: 'PDF' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '总结 PDF' }));

  expect(screen.getByText('已识别：产品方案.pdf')).toBeInTheDocument();
  expect(screen.getByText('正在总结 PDF')).toBeInTheDocument();
  expect(screen.getByText('当前动作：总结 PDF')).toBeInTheDocument();

  vi.advanceTimersByTime(900);

  expect(await screen.findByText('PDF 总结')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '展开详情' }));
  expect(screen.getByRole('heading', { name: '完整结果' })).toBeInTheDocument();
  expect(screen.getByText('产品方案.pdf')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '继续处理' })).toBeInTheDocument();
});

it('renders the full state gallery copy', () => {
  render(<App />);

  expect(screen.getByText('默认态')).toBeInTheDocument();
  expect(screen.getByText('悬浮球等待对象输入')).toBeInTheDocument();
  expect(screen.getByText('动作选择面板')).toBeInTheDocument();
  expect(screen.getByText('按对象类型给出快捷动作')).toBeInTheDocument();
  expect(screen.getByText('错误 / 不支持')).toBeInTheDocument();
});

it('keeps the highlighted gallery state and result card placement in sync', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: 'PDF' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '总结 PDF' }));
  vi.advanceTimersByTime(900);

  const stage = screen.getByLabelText('桌面交互主舞台');
  expect(within(stage).getByText('PDF 总结')).toBeInTheDocument();
  expect(screen.getByTestId('gallery-item-result')).toHaveAttribute('data-active', 'true');
});

it('reopens the action panel when continuing processing', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: 'PDF' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '总结 PDF' }));
  vi.advanceTimersByTime(900);
  await user.click(await screen.findByRole('button', { name: '继续处理' }));

  expect(screen.getByRole('button', { name: '生成问答' })).toBeInTheDocument();
  expect(screen.getByText('基于当前结果继续处理')).toBeInTheDocument();
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
});

it('restores the result gallery highlight after closing detail', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: 'PDF' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '总结 PDF' }));
  vi.advanceTimersByTime(900);
  await user.click(await screen.findByRole('button', { name: '展开详情' }));
  await user.click(screen.getByRole('button', { name: '关闭详情' }));

  expect(screen.getByTestId('gallery-item-result')).toHaveAttribute('data-active', 'true');
});

```

- [ ] **Step 2: Run the context-rail test to verify it fails**

Run: `npm run test -- src/app/components/ContextRail.test.tsx`
Expected: FAIL because processing, result cards, and detail mode do not exist yet.

- [ ] **Step 3: Implement the processing header and timer effects in `src/app/App.tsx`**

Wire the timed transitions first:

- `recognized -> actions` after `750ms`
- `processing -> result` after `900ms`
- preserve the recognized object header and selected action text during processing

- [ ] **Step 4: Implement `src/app/components/ResultCard.tsx` and keep it inside the left stage**

Acceptance checks:

- render `data-testid="result-card"`
- keep the anchor above-left of the orb
- expose `onOpenDetail`, `onContinue`, and `onCopy`

- [ ] **Step 5: Implement `src/app/components/ContextRail.tsx` and keep rail mode switching isolated there**

Acceptance checks:

- accept `mode`, `highlightedState`, `galleryItems`, `detailPayload`, `onCloseDetail`, `onTriggerErrorPreview`
- render gallery mode or detail mode only, never both together

- [ ] **Step 6: Implement `src/app/components/StateGallery.tsx` with the exact id/title/description mapping contract**

Acceptance checks:

- each item uses the exact gallery ids from the selector contract
- active item exposes `data-active="true"`
- footer renders `模拟识别失败`

- [ ] **Step 7: Implement `src/app/components/DetailView.tsx` with header, object info, full result, and follow-up actions**

Acceptance checks:

- header shows `完整结果` and `关闭详情`
- body shows object label + object meta + full result
- footer shows `继续处理` and `复制结果`

```tsx
<ContextRail
  mode={state.railMode}
  highlightedState={state.highlightedGalleryState}
  galleryItems={galleryItems}
  detailPayload={{ sample: state.activeSample, result: state.activeResult }}
  onCloseDetail={handleCloseDetail}
  onTriggerErrorPreview={handleTriggerErrorPreview}
  onContinueFromDetail={handleContinueFromResult}
  onCopyResult={handleCopyResult}
/>
```

Implementation notes:

- When an action is chosen, set `processing` immediately and complete it with `MOTION.processingBeatMs` (`900ms`).
- Put the recognition and processing timers in `src/app/App.tsx` `useEffect` hooks, not in the reducer or in individual leaf components.
- Keep the result card inside the left stage, anchored above-left of the orb.
- Default rail mode is `gallery`; opening detail switches to `detail`; closing detail restores `gallery` and preserves the highlighted state.
- Add one-line state descriptions from the spec to every gallery card and test that the gallery always renders them.
- Summary cards use `3 条重点 + 1 段摘要 + 2 个后续动作`.
- `继续处理` must reopen the action panel with the current sample and result context; do not introduce a chat transcript or freeform composer.

- [ ] **Step 8: Run the context-rail test again**

Run: `npm run test -- src/app/components/ContextRail.test.tsx`
Expected: PASS for processing, result, and detail transitions.

- [ ] **Step 9: Commit**

```bash
git add src/app/components/ResultCard.tsx src/app/components/ContextRail.tsx src/app/components/StateGallery.tsx src/app/components/DetailView.tsx src/app/App.tsx src/styles/prototype.css src/app/components/ContextRail.test.tsx
git commit -m "feat: add results and detail rail"
```

## Task 6: Generalize the Prototype to All Fixture Types and Review States

**Files:**
- Create: `src/app/components/ErrorNotice.tsx`
- Modify: `src/app/model/mockData.ts`
- Modify: `src/app/model/reducer.ts`
- Modify: `src/app/components/ActionPanel.tsx`
- Modify: `src/app/components/ResultCard.tsx`
- Modify: `src/app/components/StateGallery.tsx`
- Modify: `src/styles/prototype.css`
- Test: `src/app/components/ActionPanel.test.tsx`

- [ ] **Step 1: Write failing tests for image, text, link, unsupported, and error-preview states**

```tsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';
import App from '../App';

it('renders object-specific outputs instead of one shared card layout', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: '图片' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: 'OCR 图片' }));
  vi.advanceTimersByTime(900);

  expect(await screen.findByText('识别文本预览')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '文本' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '翻译文本' }));
  vi.advanceTimersByTime(900);

  expect(await screen.findByText('译文')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '链接' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '提炼网页要点' }));
  vi.advanceTimersByTime(900);

  expect(await screen.findByText('网页要点')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '压缩包' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));

  expect(screen.getByText('暂不支持该格式')).toBeInTheDocument();
  expect(screen.getByText('可尝试拖入 PDF、图片、文本或链接')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '模拟识别失败' }));

  expect(screen.getByText('未能完成文档识别，请重试')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '查看原因' })).toBeInTheDocument();
});

it('keeps utility and recovery actions wired in the visible UI', async () => {
  vi.useFakeTimers();
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  render(<App />);

  await user.click(screen.getByRole('button', { name: 'PDF' }));
  await user.click(screen.getByRole('button', { name: '一键演示' }));
  vi.advanceTimersByTime(750);
  await user.click(screen.getByRole('button', { name: '总结 PDF' }));
  vi.advanceTimersByTime(900);

  expect(screen.getByRole('button', { name: '复制结果' })).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '模拟识别失败' }));

  expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '查看原因' })).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '查看原因' }));
  expect(screen.getByText('原因：当前原型使用固定 mock 错误说明')).toBeInTheDocument();
  expect(screen.getByText('未能完成文档识别，请重试')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: '重试' }));
  expect(screen.getByRole('button', { name: '总结 PDF' })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the fixture-coverage tests to verify they fail**

Run: `npm run test -- src/app/components/ActionPanel.test.tsx`
Expected: FAIL because non-PDF fixtures, unsupported copy, and the review-only error state are not implemented.

- [ ] **Step 3: Add image action definitions in `src/app/model/mockData.ts`**

Acceptance checks:

- include `ocr`, `extract`, `organize`
- labels match the authoritative action map exactly

- [ ] **Step 4: Add text and link action definitions in `src/app/model/mockData.ts`**

Acceptance checks:

- text includes `translate`, `extract`, `organize`
- link includes `brief`, `link-summary`, `organize`

- [ ] **Step 5: Add image/text/link result fixtures in `src/app/model/mockData.ts`**

Acceptance checks:

- include OCR, translation, webpage brief, extract, and organize result payloads
- use the exact titles from the result template catalog

- [ ] **Step 6: Implement object-specific branches in `src/app/components/ResultCard.tsx` for OCR, translation, webpage brief, and shared extract/organize layouts**

Acceptance checks:

- each branch renders the correct section heading
- shared layouts are reused only where the catalog says they should be

- [ ] **Step 7: Implement unsupported helper UI in `src/app/components/ErrorNotice.tsx`**

Acceptance checks:

- title is `暂不支持该格式`
- body is `可尝试拖入 PDF、图片、文本或链接`

- [ ] **Step 8: Wire unsupported rendering in `src/app/components/ActionPanel.tsx`**

Acceptance checks:

- unsupported state shows helper UI instead of action buttons
- supported states keep action buttons

- [ ] **Step 9: Implement the review-only `模拟识别失败` trigger in `src/app/components/StateGallery.tsx`**

- [ ] **Step 10: Handle `triggerErrorPreview` in `src/app/model/reducer.ts`**

Acceptance checks:

- move to `error`
- keep `gallery-item-error` highlighted

- [ ] **Step 11: Render the error view with `重试` and `查看原因` in `src/app/components/ActionPanel.tsx`**

Acceptance checks:

- `重试` dispatches `retryAfterError` and returns to the current sample's `actions` state with its mapped buttons visible again
- `查看原因` reveals a short mock reason block without leaving the right-side model

```tsx
{status === 'unsupported' ? (
  <ErrorNotice title="暂不支持该格式" body="可尝试拖入 PDF、图片、文本或链接" />
) : (
  actions.map((action) => <button key={action.id}>{action.label}</button>)
)}
```

Implementation notes:

- `whiteboard-image` uses the OCR template: `识别文本预览` + `关键信息块`.
- `meeting-note` uses the translation template: `原文` + `译文` + language pill.
- `research-link` uses the webpage brief template: `网页要点` + source domain.
- `archive-zip` shows the unsupported helper immediately after release.
- Add a review-only trigger in the state gallery footer labeled `模拟识别失败`; it should move the app to `error` and show `未能完成文档识别，请重试` with `重试` and `查看原因`.

- [ ] **Step 12: Run the fixture-coverage tests again**

Run: `npm run test -- src/app/components/ActionPanel.test.tsx`
Expected: PASS for image, text, link, unsupported, and error-preview coverage.

- [ ] **Step 13: Commit**

```bash
git add src/app/components/ErrorNotice.tsx src/app/model/mockData.ts src/app/model/reducer.ts src/app/components/ActionPanel.tsx src/app/components/ResultCard.tsx src/app/components/StateGallery.tsx src/styles/prototype.css src/app/components/ActionPanel.test.tsx
git commit -m "feat: generalize prototype states and outputs"
```

## Task 7: Verify the End-to-End Flows and Polish Motion

**Files:**
- Modify: `src/app/App.tsx`
- Modify: `src/app/components/DesktopStage.tsx`
- Modify: `src/app/components/FloatingOrb.tsx`
- Modify: `src/app/components/ResultCard.tsx`
- Modify: `playwright.config.ts`
- Modify: `src/styles/tokens.css`
- Modify: `src/styles/prototype.css`
- Test: `tests/e2e/drag-input.spec.ts`

- [ ] **Step 1: Write failing Playwright tests for the PDF and unsupported flows**

```ts
import { test, expect } from '@playwright/test';

test('pdf happy path reaches result and detail', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'PDF' }).click();

  const sample = page.getByTestId('desktop-active-sample');
  const orb = page.getByLabel('悬浮球');
  const sampleBox = await sample.boundingBox();
  const orbBox = await orb.boundingBox();

  await page.mouse.move(sampleBox!.x + 40, sampleBox!.y + 40);
  await page.mouse.down();
  await page.mouse.move(orbBox!.x + 24, orbBox!.y + 24);
  await expect(page.getByText('松手处理')).toBeVisible();
  await page.mouse.move(orbBox!.x + orbBox!.width / 2, orbBox!.y + orbBox!.height / 2);
  await page.mouse.up();

  await expect(page.getByText('识别为 PDF')).toBeVisible();
  await expect(page.getByRole('button', { name: '总结 PDF' })).toBeVisible();
  await page.getByRole('button', { name: '总结 PDF' }).click();

  const stage = page.getByLabel('桌面交互主舞台');
  const resultCard = page.getByTestId('result-card');

  await expect(page.getByText('PDF 总结')).toBeVisible();
  await expect(page.getByTestId('gallery-item-result')).toHaveAttribute('data-active', 'true');

  const stageBox = await stage.boundingBox();
  const resultBox = await resultCard.boundingBox();
  expect(resultBox!.x).toBeGreaterThanOrEqual(stageBox!.x);
  expect(resultBox!.x + resultBox!.width).toBeLessThanOrEqual(stageBox!.x + stageBox!.width);
  expect(resultBox!.x).toBeLessThan(orbBox!.x);
  expect(resultBox!.y).toBeLessThan(orbBox!.y);

  await page.getByRole('button', { name: '展开详情' }).click();
  await expect(page.getByRole('heading', { name: '完整结果' })).toBeVisible();
});

test('unsupported files warn before release and end in the helper state', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: '压缩包' }).click();

  const sample = page.getByTestId('desktop-active-sample');
  const orb = page.getByLabel('悬浮球');
  const sampleBox = await sample.boundingBox();
  const orbBox = await orb.boundingBox();

  await page.mouse.move(sampleBox!.x + 40, sampleBox!.y + 40);
  await page.mouse.down();
  await page.mouse.move(orbBox!.x + 20, orbBox!.y + 20);

  await expect(page.getByText('暂不支持该格式')).toBeVisible();
  await expect(orb).toHaveAttribute('data-status', 'unsupported-hover');

  await page.mouse.move(orbBox!.x + orbBox!.width / 2, orbBox!.y + orbBox!.height / 2);
  await page.mouse.up();

  await expect(page.getByText('可尝试拖入 PDF、图片、文本或链接')).toBeVisible();
});

test('layout keeps the stage dominant and the orb anchored lower-right', async ({ page }) => {
  await page.goto('/');

  const stageBox = await page.getByLabel('桌面交互主舞台').boundingBox();
  const railBox = await page.getByLabel('右侧上下文栏').boundingBox();
  const orbBox = await page.getByLabel('悬浮球').boundingBox();

  const total = stageBox!.width + railBox!.width;
  expect(stageBox!.width / total).toBeGreaterThan(0.6);
  expect(stageBox!.width / total).toBeLessThan(0.7);
  expect(orbBox!.width).toBeGreaterThanOrEqual(56);
  expect(orbBox!.width).toBeLessThanOrEqual(64);
  expect(orbBox!.x).toBeGreaterThan(stageBox!.x + stageBox!.width * 0.72);
  expect(orbBox!.y).toBeGreaterThan(stageBox!.y + stageBox!.height * 0.58);
});

test('desktop object placement keeps the designed order at 1440px and 1024px', async ({ page }) => {
  for (const width of [1440, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/');

    const pdf = await page.getByTestId('sample-product-pdf').boundingBox();
    const image = await page.getByTestId('sample-whiteboard-image').boundingBox();
    const text = await page.getByTestId('sample-meeting-note').boundingBox();
    const link = await page.getByTestId('sample-research-link').boundingBox();
    const zip = await page.getByTestId('sample-archive-zip').boundingBox();

    expect(pdf!.x).toBeLessThan(image!.x);
    expect(text!.y).toBeGreaterThan(pdf!.y);
    expect(link!.y).toBeGreaterThan(image!.y);
    expect(zip!.x).toBeGreaterThan(text!.x);
  }
});

test('desktop object and result anchors match the default coordinate contract at 1440px', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');

  const stageBox = await page.getByLabel('桌面交互主舞台').boundingBox();
  const pdf = await page.getByTestId('sample-product-pdf').boundingBox();
  const image = await page.getByTestId('sample-whiteboard-image').boundingBox();
  const text = await page.getByTestId('sample-meeting-note').boundingBox();
  const link = await page.getByTestId('sample-research-link').boundingBox();
  const zip = await page.getByTestId('sample-archive-zip').boundingBox();

  expect(Math.abs((pdf!.x - stageBox!.x) - 72)).toBeLessThan(28);
  expect(Math.abs((pdf!.y - stageBox!.y) - 88)).toBeLessThan(28);
  expect(Math.abs((image!.x - stageBox!.x) - 336)).toBeLessThan(32);
  expect(Math.abs((text!.y - stageBox!.y) - 322)).toBeLessThan(32);
  expect(Math.abs((link!.x - stageBox!.x) - 300)).toBeLessThan(32);
  expect(Math.abs((zip!.x - stageBox!.x) - 560)).toBeLessThan(32);

  await page.getByRole('button', { name: 'PDF' }).click();
  const sample = page.getByTestId('desktop-active-sample');
  const orb = page.getByLabel('悬浮球');
  const sampleBox = await sample.boundingBox();
  const orbBox = await orb.boundingBox();
  await page.mouse.move(sampleBox!.x + 40, sampleBox!.y + 40);
  await page.mouse.down();
  await page.mouse.move(orbBox!.x + 24, orbBox!.y + 24);
  await page.mouse.move(orbBox!.x + orbBox!.width / 2, orbBox!.y + orbBox!.height / 2);
  await page.mouse.up();
  await page.getByRole('button', { name: '总结 PDF' }).click();

  const resultCard = await page.getByTestId('result-card').boundingBox();
  expect(Math.abs((stageBox!.x + stageBox!.width) - (resultCard!.x + resultCard!.width) - 148)).toBeLessThan(32);
  expect(Math.abs((stageBox!.y + stageBox!.height) - (resultCard!.y + resultCard!.height) - 208)).toBeLessThan(36);
});

test('first impression keeps the left stage primary', async ({ page }) => {
  await page.goto('/');

  const stageBox = await page.getByLabel('桌面交互主舞台').boundingBox();
  const railBox = await page.getByLabel('右侧上下文栏').boundingBox();

  await expect(page.getByText('产品方案.pdf')).toBeVisible();
  await expect(page.getByText('白板拍照.png')).toBeVisible();
  await expect(page.getByText('会议摘录.txt')).toBeVisible();
  await expect(page.getByText('https://example.com/ai-desktop-workflow')).toBeVisible();
  await expect(page.getByText('项目资料.zip')).toBeVisible();
  await expect(page.getByTestId('action-panel')).toBeHidden();
  await expect(page.getByTestId('result-card')).toBeHidden();
  expect(stageBox!.width).toBeGreaterThan(railBox!.width);
});

test('all primary UI copy stays in Chinese', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByRole('heading', { name: '对象即输入' })).toBeVisible();
  await expect(page.getByRole('button', { name: '一键演示' })).toBeVisible();
  await expect(page.locator('body')).not.toContainText(/Drop here|Ask anything|Summarize PDF|Translate Text/i);
});

test('motion tokens stay inside the spec timing bands', async ({ page }) => {
  await page.goto('/');

  const shell = page.getByTestId('prototype-shell');
  const nearby = await shell.evaluate((node) => getComputedStyle(node).getPropertyValue('--motion-nearby').trim());
  const snap = await shell.evaluate((node) => getComputedStyle(node).getPropertyValue('--motion-snap').trim());
  const recognition = await shell.evaluate((node) => getComputedStyle(node).getPropertyValue('--motion-recognition').trim());
  const processing = await shell.evaluate((node) => getComputedStyle(node).getPropertyValue('--motion-processing').trim());
  const result = await shell.evaluate((node) => getComputedStyle(node).getPropertyValue('--motion-result').trim());

  expect(nearby).toBe('140ms');
  expect(snap).toBe('200ms');
  expect(recognition).toBe('750ms');
  expect(processing).toBe('900ms');
  expect(result).toBe('220ms');
});

test('desktop background and hover overlay keep the system-tool treatment', async ({ page }) => {
  await page.goto('/');

  const stage = page.getByLabel('桌面交互主舞台');
  const shell = page.getByTestId('prototype-shell');
  const orb = page.getByLabel('悬浮球');
  const sample = page.getByTestId('desktop-active-sample');
  const secondary = page.getByTestId('sample-meeting-note');

  const background = await stage.evaluate((node) => getComputedStyle(node).backgroundImage);
  const blur = await shell.evaluate((node) => getComputedStyle(node).getPropertyValue('--stage-blur').trim());

  expect(background).toMatch(/gradient/i);
  expect(blur).not.toBe('0px');

  const sampleBox = await sample.boundingBox();
  const orbBox = await orb.boundingBox();
  await page.mouse.move(sampleBox!.x + 40, sampleBox!.y + 40);
  await page.mouse.down();
  await page.mouse.move(orbBox!.x + 24, orbBox!.y + 24);

  await expect(stage).toHaveAttribute('data-overlay', 'active');
  await expect(secondary).toHaveAttribute('data-muted', 'true');
});
```

- [ ] **Step 2: Run the end-to-end test to verify it fails**

Run: `npx playwright test tests/e2e/drag-input.spec.ts --project=chromium`
Expected: FAIL until the full UI flow and stable selectors exist.

- [ ] **Step 3: Add stable test hooks in `src/app/App.tsx`, `src/app/components/DesktopStage.tsx`, `src/app/components/FloatingOrb.tsx`, and `src/app/components/ResultCard.tsx`**

Acceptance checks:

- expose all test ids from the selector contract
- expose `data-status`, `data-eligibility`, and gallery `data-active`
- expose stage `data-overlay` and non-active object `data-muted` during nearby/hover states

- [ ] **Step 4: Add motion variables in `src/styles/tokens.css` and bind them to `src/app/model/motion.ts` values**

- [ ] **Step 5: Add shell split and orb placement rules in `src/styles/prototype.css`**

Acceptance checks:

- preserve the `65 / 35` shell split
- keep the orb in the lower-right stage zone
- keep the initial viewport visually led by the left stage, with hidden action-panel and result-card elements on load

- [ ] **Step 6: Add desktop object coordinates and result-card anchor rules in `src/styles/prototype.css`**

Acceptance checks:

- match the default coordinate contract at `1440px`
- preserve relative order at `1024px`

- [ ] **Step 7: Add the stage background, blur, dimming overlay, and non-active object de-emphasis in `src/styles/prototype.css`**

Acceptance checks:

- apply the light gradient + blur stage treatment
- activate the dimming overlay and non-active object de-emphasis during nearby/hover states

Implementation checklist:

- Add stable `data-testid` and `data-status` attributes used by tests and CSS.
- Respect `prefers-reduced-motion` while keeping the default hover/recognition timing from the spec.
- Keep the main layout readable at `1440px` and `1024px` widths without hiding the orb or the rail.
- Verify the highlighted gallery card always matches the live stage state.
- Verify every gallery card still shows its one-line explanation in both idle and result flows.
- Add a Playwright assertion that the stage and rail stay within an approximate `65 / 35` split and that the orb remains anchored in the lower-right zone of the stage.
- Add a Playwright assertion that the result card bounding box stays inside the stage bounding box during the result state.
- Add a Playwright assertion that the gallery item for `结果卡片态` becomes active when the result card is visible.

- [ ] **Step 8: Run the full verification suite**

Run: `npm run test && npx playwright test tests/e2e/drag-input.spec.ts --project=chromium && npm run build`
Expected: All unit tests pass, the Playwright spec passes, and the production build succeeds.

- [ ] **Step 9: Commit**

```bash
git add src/app/App.tsx src/app/components/DesktopStage.tsx src/app/components/FloatingOrb.tsx src/app/components/ResultCard.tsx playwright.config.ts src/styles/tokens.css src/styles/prototype.css tests/e2e/drag-input.spec.ts
git commit -m "test: verify drag-input prototype flows"
```

## Final Verification Checklist

- Run `npm run test`
- Run `npx playwright test tests/e2e/drag-input.spec.ts --project=chromium`
- Run `npm run build`
- Manually verify the first impression: the stage makes drag-input feel primary before the rail, result card, or detail view draw attention
- Manually verify the overall look stays like a restrained system tool: medium radii, light borders, soft shadows, no chat bubbles, no mascot styling, no neon glow
- Manually verify unsupported files warn before release and never show the supported glow state
- Manually verify the stage uses a light gradient + blur background and that nearby/hover states dim the rest of the desktop slightly
- Manually verify `继续处理` returns to the action panel instead of opening any chat-style input
- Manually verify `复制结果`, `重试`, and `查看原因` all produce the expected mock behavior
- Manually verify all exposed UI copy remains fully Chinese in idle, result, detail, unsupported, and error states
- Use `@superpowers:verification-before-completion` before claiming the implementation is done
