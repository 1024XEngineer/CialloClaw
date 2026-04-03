const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { inspectTasks, buildDraft, createLogEntry } = require('../logic.js');

const today = '2026-03-28';
const tasks = [
  {
    title: 'Finish weekly status update',
    dueAt: today,
    status: 'pending',
    priority: 'high',
    suggestedAction: 'Generate a progress update draft'
  },
  {
    title: 'Review candidate notes',
    dueAt: '2026-03-29',
    status: 'pending',
    priority: 'high',
    suggestedAction: 'Summarize follow-up items'
  },
  {
    title: 'Archive closed ticket notes',
    dueAt: '2026-03-27',
    status: 'done',
    priority: 'low',
    suggestedAction: 'Move notes to archive'
  }
];

test('inspectTasks returns only pending tasks due today or high priority', () => {
  const result = inspectTasks(tasks, today);

  assert.equal(result.relevantTasks.length, 2);
  assert.deepEqual(
    result.relevantTasks.map((task) => task.title),
    ['Finish weekly status update', 'Review candidate notes']
  );
  assert.match(result.summary, /2 tasks need follow-up/i);
});

test('buildDraft produces a readable draft from the summary and tasks', () => {
  const inspection = inspectTasks(tasks, today);
  const draft = buildDraft(inspection.summary, inspection.relevantTasks);

  assert.match(draft, /Finish weekly status update/);
  assert.match(draft, /Generate a progress update draft/);
  assert.match(draft, /Review candidate notes/);
  assert.match(draft, /Summarize follow-up items/);
});

test('buildDraft handles the no-task case', () => {
  const draft = buildDraft('No tasks need follow-up right now.', []);

  assert.match(draft, /No tasks need follow-up right now\./);
  assert.match(draft, /No action is needed right now\./);
});

test('createLogEntry normalizes action metadata', () => {
  const entry = createLogEntry('inspection', 'success', 'Inspection completed', '2026-03-28T09:30:00.000Z');

  assert.deepEqual(Object.keys(entry), ['id', 'action', 'status', 'detail', 'timestamp']);
  assert.match(entry.id, /^inspection-2026-03-28T09:30:00\.000Z-\d+$/);
  assert.equal(entry.action, 'inspection');
  assert.equal(entry.status, 'success');
  assert.equal(entry.detail, 'Inspection completed');
  assert.match(entry.timestamp, /2026-03-28/);
});

test('createLogEntry generates unique ids for repeated action and timestamp values', () => {
  const firstEntry = createLogEntry('inspection', 'success', 'Inspection completed', '2026-03-28T09:30:00.000Z');
  const secondEntry = createLogEntry('inspection', 'success', 'Inspection completed', '2026-03-28T09:30:00.000Z');

  assert.notEqual(firstEntry.id, secondEntry.id);
  assert.equal(firstEntry.timestamp, '2026-03-28T09:30:00.000Z');
  assert.equal(secondEntry.timestamp, '2026-03-28T09:30:00.000Z');
});

test('createLogEntry uses a string timestamp by default', () => {
  const entry = createLogEntry('inspection', 'success', 'Inspection completed');

  assert.equal(typeof entry.timestamp, 'string');
});

test('logic.js exposes browser globals through globalThis and window', () => {
  const source = fs.readFileSync(path.join(__dirname, '..', 'logic.js'), 'utf8');
  const context = {
    globalThis: {},
    window: {}
  };

  vm.runInNewContext(source, context);

  assert.equal(typeof context.globalThis.CialloLogic.inspectTasks, 'function');
  assert.equal(context.window.CialloLogic, context.globalThis.CialloLogic);
});
