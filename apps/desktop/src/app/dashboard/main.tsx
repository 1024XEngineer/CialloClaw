// 该入口负责挂载仪表盘窗口。
import { getCurrentWindow } from "@tauri-apps/api/window";
import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { installHideOnCloseRequest } from "@/platform/hideOnCloseRequest";
import { DashboardRoot } from "./DashboardRoot";

function installDashboardEscapeClose(windowHandle = getCurrentWindow()) {
  let closing = false;

  window.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") {
      return;
    }

    queueMicrotask(() => {
      const target = event.target as HTMLElement | null;
      const tagName = target?.tagName;

      if (
        event.defaultPrevented ||
        closing ||
        target?.isContentEditable ||
        tagName === "INPUT" ||
        tagName === "TEXTAREA" ||
        tagName === "SELECT"
      ) {
        return;
      }

      closing = true;
      void windowHandle.close().finally(() => {
        closing = false;
      });
    });
  });
}

void installHideOnCloseRequest();
installDashboardEscapeClose();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <AppProviders>
    <DashboardRoot />
  </AppProviders>,
);
