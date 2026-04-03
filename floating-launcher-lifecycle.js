function createFloatingLauncherLifecycle(deps) {
  const { createFloatingWindow } = deps;
  let floatingWindow;
  let allowLauncherClose = false;

  function attachFloatingWindow(window) {
    if (!window) {
      return window;
    }

    window.on('closed', () => {
      if (floatingWindow === window) {
        floatingWindow = undefined;
      }

      if (allowLauncherClose) {
        return;
      }

      floatingWindow = attachFloatingWindow(createFloatingWindow());
    });

    return window;
  }

  function ensureFloatingWindow() {
    if (!floatingWindow || floatingWindow.isDestroyed()) {
      floatingWindow = attachFloatingWindow(createFloatingWindow());
    }

    return floatingWindow;
  }

  function prepareToQuit() {
    allowLauncherClose = true;
  }

  return {
    ensureFloatingWindow,
    prepareToQuit
  };
}

module.exports = {
  createFloatingLauncherLifecycle
};
