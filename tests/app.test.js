const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function createElement(id) {
  return {
    id,
    innerHTML: '',
    textContent: '',
    value: '',
    disabled: false,
    className: '',
    listeners: {},
    scrollCount: 0,
    attributes: {},
    classList: {
      values: new Set(),
      add(...names) {
        names.forEach((name) => this.values.add(name));
      },
      remove(...names) {
        names.forEach((name) => this.values.delete(name));
      },
      toggle(name, force) {
        if (force === undefined) {
          if (this.values.has(name)) {
            this.values.delete(name);
            return false;
          }

          this.values.add(name);
          return true;
        }

        if (force) {
          this.values.add(name);
          return true;
        }

        this.values.delete(name);
        return false;
      },
      contains(name) {
        return this.values.has(name);
      }
    },
    addEventListener(type, handler) {
      this.listeners[type] = handler;
    },
    click() {
      if (this.listeners.click) {
        this.listeners.click({ currentTarget: this, preventDefault() {} });
      }
    },
    setAttribute(name, value) {
      this.attributes[name] = String(value);
    },
    getAttribute(name) {
      return this.attributes[name];
    },
    scrollIntoView() {
      this.scrollCount += 1;
    }
  };
}

function loadApp(options = {}) {
  const logic = require('../logic.js');
  const source = fs.readFileSync(path.join(__dirname, '..', 'app.js'), 'utf8');
  const ids = [
    'floating-ball',
    'assistant-panel',
    'assistant-message',
    'task-list',
    'inspection-summary',
    'draft-output',
    'execution-log',
    'run-inspection',
    'generate-draft',
    'undo-last-action',
    'view-log',
    'chat-panel',
    'chat-messages',
    'chat-action-summarize',
    'chat-action-explain',
    'chat-action-extract-todos',
    'chat-input',
    'chat-send'
  ];

  const elements = Object.fromEntries(ids.map((id) => [id, createElement(id)]));
  elements['assistant-panel'].classList.add('is-hidden');
  elements['floating-ball'].setAttribute('aria-expanded', 'false');

  const document = {
    body: createElement('body'),
    getElementById(id) {
      return elements[id] || null;
    },
    readyState: 'complete',
    addEventListener() {}
  };

  const context = {
    window: {
      CialloLogic: logic,
      location: {
        search: options.search || ''
      }
    },
    document,
    console
  };

  context.globalThis = context.window;
  context.window.window = context.window;
  context.window.document = document;

  vm.runInNewContext(source, context);

  return {
    api: context.window.CialloApp,
    elements,
    document
  };
}

test('app initializes demo tasks and renders them', () => {
  const { api, elements } = loadApp();

  assert.equal(typeof api.init, 'function');
  assert.match(elements['task-list'].innerHTML, /Finish weekly status update/);
  assert.match(elements['task-list'].innerHTML, /Review candidate notes/);
  assert.match(elements['task-list'].innerHTML, /Archive closed ticket notes/);
});

test('run inspection updates summary, highlights relevant tasks, and logs success', () => {
  const { elements } = loadApp();

  elements['run-inspection'].click();

  assert.match(elements['inspection-summary'].innerHTML, /2 tasks need follow-up today\./);
  assert.match(elements['task-list'].innerHTML, /task-card is-relevant/);
  assert.match(elements['execution-log'].innerHTML, /inspection/);
  assert.match(elements['assistant-message'].textContent, /inspection complete/i);
});

test('generate draft requires inspection and then confirmation before drafting', () => {
  const { elements } = loadApp();

  elements['generate-draft'].click();
  assert.match(elements['assistant-message'].textContent, /run inspection first/i);
  assert.match(elements['execution-log'].innerHTML, /blocked/);

  elements['run-inspection'].click();
  elements['generate-draft'].click();
  assert.match(elements['assistant-message'].textContent, /click generate draft again/i);
  assert.doesNotMatch(elements['draft-output'].innerHTML, /Draft Ready/);

  elements['generate-draft'].click();
  assert.match(elements['draft-output'].innerHTML, /Draft Ready/);
  assert.equal(elements['undo-last-action'].disabled, false);
});

test('undo only removes the draft and view log scrolls while keeping log history', () => {
  const { elements } = loadApp();

  elements['run-inspection'].click();
  elements['generate-draft'].click();
  elements['generate-draft'].click();
  const summaryBeforeUndo = elements['inspection-summary'].innerHTML;

  elements['undo-last-action'].click();
  assert.equal(elements['inspection-summary'].innerHTML, summaryBeforeUndo);
  assert.doesNotMatch(elements['draft-output'].innerHTML, /Draft Ready/);
  assert.match(elements['execution-log'].innerHTML, /undo/);

  elements['view-log'].click();
  assert.equal(elements['execution-log'].scrollCount, 1);
  assert.match(elements['execution-log'].innerHTML, /view-log/);
});

test('unexpected action errors show fallback messaging and preserve current results', () => {
  const { api, elements } = loadApp();

  elements['run-inspection'].click();
  const summaryBeforeError = elements['inspection-summary'].innerHTML;
  const logBeforeError = elements['execution-log'].innerHTML;

  api.forceErrorForTesting('generate-draft');
  elements['generate-draft'].click();

  assert.equal(elements['inspection-summary'].innerHTML, summaryBeforeError);
  assert.match(elements['execution-log'].innerHTML, /unexpected error/i);
  assert.notEqual(elements['execution-log'].innerHTML, logBeforeError);
  assert.match(elements['assistant-message'].textContent, /something went wrong/i);
});

test('desktop shell mode hides the in-page floating ball and keeps the assistant panel open', () => {
  const { elements, document } = loadApp({ search: '?shell=desktop' });

  assert.equal(document.body.classList.contains('desktop-shell'), true);
  assert.equal(elements['floating-ball'].classList.contains('is-hidden'), true);
  assert.equal(elements['assistant-panel'].classList.contains('is-hidden'), false);
  assert.equal(elements['floating-ball'].getAttribute('aria-expanded'), 'true');
});

test('app renders one welcome assistant chat message on startup', () => {
  const { elements } = loadApp();

  assert.match(elements['chat-messages'].innerHTML, /chat-message chat-message--assistant/);
  assert.match(elements['chat-messages'].innerHTML, /chat-message__bubble/);
  assert.match(elements['chat-messages'].innerHTML, /ask for a summary, tasks, or an explanation/i);
});

test('sending a chat message appends a user message and deterministic local reply', () => {
  const { elements } = loadApp();

  elements['run-inspection'].click();
  elements['chat-input'].value = 'Please summarize the current work';
  elements['chat-send'].click();

  assert.match(elements['chat-messages'].innerHTML, /chat-message chat-message--user/);
  assert.match(elements['chat-messages'].innerHTML, /chat-message chat-message--assistant/);
  assert.match(elements['chat-messages'].innerHTML, /chat-message__bubble/);
  assert.match(elements['chat-messages'].innerHTML, /Please summarize the current work/);
  assert.match(elements['chat-messages'].innerHTML, /2 tasks need follow-up today\./);
  assert.equal(elements['assistant-message'].textContent, 'Inspection complete. Relevant tasks are highlighted and ready for draft confirmation.');
});

test('quick actions append a prompt and assistant reply without using the workflow status area', () => {
  const { elements } = loadApp();

  elements['chat-action-extract-todos'].click();

  assert.match(elements['chat-messages'].innerHTML, /chat-message chat-message--user/);
  assert.match(elements['chat-messages'].innerHTML, /chat-message chat-message--assistant/);
  assert.match(elements['chat-messages'].innerHTML, /chat-message__bubble/);
  assert.match(elements['chat-messages'].innerHTML, /Extract todos/i);
  assert.match(elements['chat-messages'].innerHTML, /Finish weekly status update/);
  assert.match(elements['chat-messages'].innerHTML, /Review candidate notes/);
  assert.equal(elements['assistant-message'].textContent, 'Ready to inspect tasks and prepare a visible draft.');
});
