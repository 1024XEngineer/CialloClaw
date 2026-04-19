import { getCurrentWindow } from "@tauri-apps/api/window";
import { requestShellBallDashboardCloseTransition } from "./dashboardWindowTransition";

type HideOnCloseWindow = ReturnType<typeof getCurrentWindow>;

const destroyOnCloseLabels = new Set(["control-panel"]);

export function installHideOnCloseRequest(windowHandle: HideOnCloseWindow = getCurrentWindow()) {
  let hiding = false;
  const destroysOnClose = destroyOnCloseLabels.has(windowHandle.label);

  return windowHandle.onCloseRequested(async (event) => {
    if (hiding) {
      // Destroy-on-close windows re-enter the handler through the native destroy path,
      // but hide-on-close windows must keep blocking repeated close requests.
      if (!destroysOnClose) {
        event.preventDefault();
      }
      return;
    }

    event.preventDefault();

    hiding = true;

    try {
      if (windowHandle.label === "dashboard") {
        await requestShellBallDashboardCloseTransition();
      }

      if (destroysOnClose) {
        await windowHandle.destroy();
        return;
      }

      await windowHandle.hide();
    } finally {
      hiding = false;
    }
  });
}
