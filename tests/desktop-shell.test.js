const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const {
  FLOATING_SIZE,
  DEFAULT_PANEL_SIZE,
  getFloatingBallBounds,
  getMainPanelBounds,
  buildDesktopPanelUrl
} = require('../desktop-shell.js');

test('desktop shell exports the expected sizing constants', () => {
  assert.deepEqual(FLOATING_SIZE, { width: 72, height: 72 });
  assert.deepEqual(DEFAULT_PANEL_SIZE, { width: 960, height: 720 });
});

test('getFloatingBallBounds places the ball near the bottom-right corner', () => {
  const bounds = getFloatingBallBounds({ x: 0, y: 0, width: 1440, height: 900 });

  assert.deepEqual(bounds, {
    x: 1344,
    y: 804,
    width: 72,
    height: 72
  });
});

test('getMainPanelBounds places the panel near the floating ball', () => {
  const panelBounds = getMainPanelBounds(
    { x: 1344, y: 804, width: 72, height: 72 },
    { x: 0, y: 0, width: 1440, height: 900 }
  );

  assert.deepEqual(panelBounds, {
    x: 368,
    y: 156,
    width: 960,
    height: 720
  });
});

test('getMainPanelBounds clamps the panel so it stays on screen', () => {
  const panelBounds = getMainPanelBounds(
    { x: 20, y: 20, width: 72, height: 72 },
    { x: 0, y: 0, width: 1000, height: 800 },
    { width: 960, height: 720 }
  );

  assert.deepEqual(panelBounds, {
    x: 0,
    y: 0,
    width: 960,
    height: 720
  });
});

test('buildDesktopPanelUrl returns a file URL with the desktop shell query', () => {
  const indexPath = path.join('D:\\Desktop\\claw\\mvp', 'index.html');
  const expected = pathToFileURL(indexPath).toString() + '?shell=desktop';

  assert.equal(buildDesktopPanelUrl(indexPath), expected);
});
