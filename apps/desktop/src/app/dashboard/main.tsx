// This entrypoint mounts the dashboard desktop window.
import { getCurrentWindow } from "@tauri-apps/api/window";
import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { installHideOnCloseRequest } from "@/platform/hideOnCloseRequest";
import {
  readDashboardEscapeCloseSuppressionVersion,
  wasDashboardEscapeCloseSuppressed,
} from "./dashboardEscapeCloseGuard";
import { DashboardRoot } from "./DashboardRoot";
import { DashboardWindowErrorBoundary } from "./DashboardWindowErrorBoundary";

function isDashboardHomeHash(hashValue: string) {
  return hashValue === "" || hashValue === "#" || hashValue === "#/";
}

function installDashboardEscapeClose(windowHandle = getCurrentWindow()) {
  let closing = false;

  window.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") {
      return;
    }

    const suppressionVersion = readDashboardEscapeCloseSuppressionVersion();

    queueMicrotask(() => {
      const target = event.target as HTMLElement | null;
      const tagName = target?.tagName;
      const currentHash = window.location.hash;

      if (
        event.defaultPrevented ||
        wasDashboardEscapeCloseSuppressed(suppressionVersion) ||
        !isDashboardHomeHash(currentHash) ||
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
  // Keep the recovery boundary above AppProviders so provider init failures
  // still land on the dashboard fallback instead of collapsing the window.
  <DashboardWindowErrorBoundary>
    <AppProviders>
      <DashboardRoot />
    </AppProviders>
  </DashboardWindowErrorBoundary>,
);
