const test = require('node:test');
const assert = require('node:assert/strict');

const { createFloatingLauncherLifecycle } = require('../floating-launcher-lifecycle.js');

function createFakeWindow() {
  const events = new Map();
  let destroyed = false;

  return {
    isDestroyed() {
      return destroyed;
    },
    on(eventName, handler) {
      events.set(eventName, handler);
    },
    emit(eventName) {
      if (eventName === 'closed') {
        destroyed = true;
      }

      const handler = events.get(eventName);
      if (handler) {
        handler();
      }
    }
  };
}

test('recreates the floating launcher after it is closed during normal operation', () => {
  const createdWindows = [];
  const lifecycle = createFloatingLauncherLifecycle({
    createFloatingWindow() {
      const window = createFakeWindow();
      createdWindows.push(window);
      return window;
    }
  });

  const firstWindow = lifecycle.ensureFloatingWindow();

  firstWindow.emit('closed');

  assert.equal(createdWindows.length, 2);
  assert.equal(lifecycle.ensureFloatingWindow(), createdWindows[1]);
});

test('does not recreate the floating launcher while the app is quitting', () => {
  const createdWindows = [];
  const lifecycle = createFloatingLauncherLifecycle({
    createFloatingWindow() {
      const window = createFakeWindow();
      createdWindows.push(window);
      return window;
    }
  });

  const firstWindow = lifecycle.ensureFloatingWindow();

  lifecycle.prepareToQuit();
  firstWindow.emit('closed');

  assert.equal(createdWindows.length, 1);
});
