const test = require('node:test');
const assert = require('node:assert/strict');

const { createMainPanelController } = require('../main-controller.js');

function createFakeWindow(options = {}) {
  const events = new Map();
  const operations = [];

  const state = {
    destroyed: options.destroyed || false,
    visible: options.visible || false
  };

  return {
    operations,
    isDestroyed() {
      return state.destroyed;
    },
    isVisible() {
      return state.visible;
    },
    hide() {
      operations.push('hide');
      state.visible = false;
    },
    show() {
      operations.push('show');
      state.visible = true;
    },
    on(eventName, handler) {
      events.set(eventName, handler);
    },
    emit(eventName, event) {
      const handler = events.get(eventName);
      if (handler) {
        handler(event);
      }
    }
  };
}

test('toggleMainPanel positions before showing and hides on second toggle', () => {
  const panelWindow = createFakeWindow();
  const callOrder = [];
  const controller = createMainPanelController({
    initialPanelWindow: panelWindow,
    createPanelWindow() {
      throw new Error('should not create a replacement window');
    },
    positionPanelWindow(window) {
      callOrder.push('position');
      assert.equal(window, panelWindow);
    }
  });

  controller.toggleMainPanel();

  assert.deepEqual(callOrder, ['position']);
  assert.deepEqual(panelWindow.operations, ['show']);

  controller.toggleMainPanel();

  assert.deepEqual(panelWindow.operations, ['show', 'hide']);
});

test('toggleMainPanel recreates the panel window when the old one is gone', () => {
  const replacementWindow = createFakeWindow();
  const controller = createMainPanelController({
    initialPanelWindow: createFakeWindow({ destroyed: true }),
    createPanelWindow() {
      return replacementWindow;
    },
    positionPanelWindow(window) {
      window.operations.push('position');
    }
  });

  controller.toggleMainPanel();

  assert.deepEqual(replacementWindow.operations, ['position', 'show']);
});

test('panel close is intercepted and turned into hide', () => {
  const panelWindow = createFakeWindow({ visible: true });
  const controller = createMainPanelController({
    initialPanelWindow: panelWindow,
    createPanelWindow() {
      throw new Error('should not create a replacement window');
    },
    positionPanelWindow() {}
  });

  const closeEvent = {
    defaultPrevented: false,
    preventDefault() {
      this.defaultPrevented = true;
    }
  };

  panelWindow.emit('close', closeEvent);

  assert.equal(closeEvent.defaultPrevented, true);
  assert.deepEqual(panelWindow.operations, ['hide']);

  controller.toggleMainPanel();
  assert.deepEqual(panelWindow.operations, ['hide', 'show']);
});

test('prepareToQuit allows the panel to close for real during shutdown', () => {
  const panelWindow = createFakeWindow({ visible: true });
  const controller = createMainPanelController({
    initialPanelWindow: panelWindow,
    createPanelWindow() {
      throw new Error('should not create a replacement window');
    },
    positionPanelWindow() {}
  });

  controller.prepareToQuit();

  const closeEvent = {
    defaultPrevented: false,
    preventDefault() {
      this.defaultPrevented = true;
    }
  };

  panelWindow.emit('close', closeEvent);

  assert.equal(closeEvent.defaultPrevented, false);
  assert.deepEqual(panelWindow.operations, []);
});
