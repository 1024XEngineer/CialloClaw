const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('index.html exposes the stitched dashboard and chat shell structure', () => {
  const markup = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');

  assert.match(markup, /id="floating-ball"/);
  assert.match(markup, /id="floating-ball"[^>]*aria-expanded="false"/);
  assert.match(markup, /class="dashboard-shell"/);
  assert.match(markup, /class="dashboard-main"/);
  assert.match(markup, /class="dashboard-grid"/);
  assert.match(markup, /class="chat-shell(?:\s+[^"]*)?"|class="[^"]*chat-shell(?:\s+[^"]*)?"/);
  assert.match(markup, /id="assistant-panel"/);
  assert.match(
    markup,
    /<section class="dashboard-shell"[^>]*>[\s\S]*<div class="dashboard-main">[\s\S]*<aside id="assistant-panel"/
  );
  assert.match(
    markup,
    /<div class="dashboard-main">[\s\S]*<aside id="assistant-panel"[\s\S]*<section class="dashboard-grid"/
  );
  assert.match(markup, /id="task-list"/);
  assert.match(markup, /id="inspection-summary"/);
  assert.match(markup, /id="draft-output"/);
  assert.match(markup, /id="execution-log"/);
  assert.match(markup, /id="chat-panel"/);
  assert.match(
    markup,
    /<section class="dashboard-shell"[^>]*>[\s\S]*<div class="dashboard-main">[\s\S]*<\/div>[\s\S]*<aside id="chat-panel"[^>]*class="[^"]*chat-shell/
  );
  assert.match(markup, /id="chat-messages"/);
  assert.match(markup, /id="chat-input"/);
  assert.match(markup, /id="chat-input"[^>]*aria-label="[^"]+"/);
  assert.match(markup, /id="chat-send"/);
  assert.match(markup, /id="chat-action-summarize"/);
  assert.match(markup, /id="chat-action-explain"/);
  assert.match(markup, /id="chat-action-extract-todos"/);
  assert.match(markup, /id="run-inspection"/);
  assert.match(markup, /id="generate-draft"/);
  assert.match(markup, /id="undo-last-action"/);
  assert.match(markup, /id="view-log"/);
  assert.match(markup, /src="logic\.js"/);
  assert.match(markup, /src="app\.js"/);
});

test('style.css defines a stitched dashboard/chat desktop layout', () => {
  const css = fs.readFileSync(path.join(__dirname, '..', 'style.css'), 'utf8');
  const stitchedCollapseRule = css.match(/@media\s*\(max-width:\s*(\d+)px\)\s*\{[\s\S]*?\.dashboard-shell\s*\{[\s\S]*?grid-template-columns\s*:\s*1fr;/);

  assert.match(
    css,
    /\.dashboard-shell\s*\{[\s\S]*display\s*:\s*grid;[\s\S]*grid-template-columns\s*:\s*[^;]+\s+[^;]+;/
  );
  assert.match(css, /\.dashboard-main\s*\{/);
  assert.match(css, /\.dashboard-grid\s*\{/);
  assert.match(css, /\.chat-shell\s*\{[\s\S]*display\s*:\s*(grid|flex);/);
  assert.ok(stitchedCollapseRule, 'expected a responsive stitched-layout collapse rule for .dashboard-shell');
  assert.ok(
    Number(stitchedCollapseRule[1]) < 930,
    `expected stitched dashboard/chat collapse breakpoint below 930px, got ${stitchedCollapseRule[1]}px`
  );
});
