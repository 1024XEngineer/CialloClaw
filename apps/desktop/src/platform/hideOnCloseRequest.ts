import { getCurrentWindow } from "@tauri-apps/api/window";
import { requestShellBallDashboardCloseTransition } from "./dashboardWindowTransition";

type HideOnCloseWindow = ReturnType<typeof getCurrentWindow>;

export function installHideOnCloseRequest(windowHandle: HideOnCloseWindow = getCurrentWindow()) {
  let hiding = false;

  return windowHandle.onCloseRequested(async (event) => {
    event.preventDefault();
    if (hiding) {
      return;
    }

    hiding = true;

    try {
      if (windowHandle.label === "dashboard") {
        await requestShellBallDashboardCloseTransition();
      }

      await windowHandle.hide();
    } finally {
      hiding = false;
    }
  });
}
