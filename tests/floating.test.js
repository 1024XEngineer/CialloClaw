const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function loadFloatingWindow() {
  const source = fs.readFileSync(path.join(__dirname, '..', 'floating.js'), 'utf8');
  const listeners = {};
  const desktopFloatingBall = {
    addEventListener(type, handler) {
      listeners[type] = handler;
    },
    click() {
      if (listeners.click) {
        listeners.click({ currentTarget: this, preventDefault() {} });
      }
    }
  };

  const context = {
    window: {
      CialloDesktop: {
        toggleMainPanel() {}
      }
    },
    document: {
      getElementById(id) {
        return id === 'desktop-floating-ball' ? desktopFloatingBall : null;
      },
      readyState: 'complete',
      addEventListener() {}
    },
    console
  };

  context.globalThis = context.window;
  context.window.window = context.window;
  context.window.document = context.document;

  vm.runInNewContext(source, context);

  return context.window;
}

test('floating renderer click toggles the desktop main panel', () => {
  const window = loadFloatingWindow();
  let toggleCount = 0;

  window.CialloDesktop.toggleMainPanel = () => {
    toggleCount += 1;
  };

  window.document.getElementById('desktop-floating-ball').click();

  assert.equal(toggleCount, 1);
});

test('floating markup loads the dedicated renderer assets', () => {
  const markup = fs.readFileSync(path.join(__dirname, '..', 'floating.html'), 'utf8');

  assert.match(markup, /id="desktop-floating-ball"/);
  assert.match(markup, /<img[^>]+src="assets\/floating-avatar\.png"/);
  assert.match(markup, /href="floating\.css"/);
  assert.match(markup, /src="floating\.js"/);
});

test('desktop main process loads floating.html instead of an embedded data url', () => {
  const source = fs.readFileSync(path.join(__dirname, '..', 'main.js'), 'utf8');

  assert.match(source, /floating\.html/);
  assert.doesNotMatch(source, /data:text\/html/);
});
