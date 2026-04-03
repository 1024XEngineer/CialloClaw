const { pathToFileURL } = require('node:url');

const FLOATING_SIZE = {
  width: 72,
  height: 72
};

const DEFAULT_PANEL_SIZE = {
  width: 960,
  height: 720
};

const FLOATING_MARGIN = 24;
const PANEL_GAP = 16;

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function getFloatingBallBounds(workArea) {
  return {
    x: workArea.x + workArea.width - FLOATING_SIZE.width - FLOATING_MARGIN,
    y: workArea.y + workArea.height - FLOATING_SIZE.height - FLOATING_MARGIN,
    width: FLOATING_SIZE.width,
    height: FLOATING_SIZE.height
  };
}

function getMainPanelBounds(floatingBounds, workArea, panelSize = DEFAULT_PANEL_SIZE) {
  const x = floatingBounds.x - panelSize.width - PANEL_GAP;
  const y = floatingBounds.y + floatingBounds.height - panelSize.height;

  return {
    x: clamp(x, workArea.x, workArea.x + workArea.width - panelSize.width),
    y: clamp(y, workArea.y, workArea.y + workArea.height - panelSize.height),
    width: panelSize.width,
    height: panelSize.height
  };
}

function buildDesktopPanelUrl(indexPath) {
  return pathToFileURL(indexPath).toString() + '?shell=desktop';
}

module.exports = {
  FLOATING_SIZE,
  DEFAULT_PANEL_SIZE,
  getFloatingBallBounds,
  getMainPanelBounds,
  buildDesktopPanelUrl
};
