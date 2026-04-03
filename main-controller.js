function createMainPanelController(deps) {
  const { createPanelWindow, positionPanelWindow } = deps;
  let panelWindow = attachPanelWindow(deps.initialPanelWindow);
  let allowPanelClose = false;

  function attachPanelWindow(window) {
    if (!window) {
      return window;
    }

    window.on('close', (event) => {
      if (allowPanelClose) {
        return;
      }

      event.preventDefault();
      window.hide();
    });

    return window;
  }

  function ensurePanelWindow() {
    if (!panelWindow || panelWindow.isDestroyed()) {
      panelWindow = attachPanelWindow(createPanelWindow());
    }

    return panelWindow;
  }

  function toggleMainPanel() {
    const window = ensurePanelWindow();

    if (window.isVisible()) {
      window.hide();
      return;
    }

    positionPanelWindow(window);
    window.show();
  }

  function prepareToQuit() {
    allowPanelClose = true;
  }

  return {
    toggleMainPanel,
    prepareToQuit
  };
}

module.exports = {
  createMainPanelController
};
