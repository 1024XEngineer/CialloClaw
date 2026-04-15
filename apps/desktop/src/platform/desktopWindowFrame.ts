import { getCurrentWindow } from "@tauri-apps/api/window";

function isDesktopWindowEnvironment() {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

function getDesktopFrameWindow() {
  if (!isDesktopWindowEnvironment()) {
    return null;
  }

  return getCurrentWindow();
}

// startCurrentDesktopWindowDragging delegates to the Tauri window drag API
// so frameless desktop windows can still be repositioned by the user.
export async function startCurrentDesktopWindowDragging() {
  const currentWindow = getDesktopFrameWindow();

  if (!currentWindow) {
    return;
  }

  await currentWindow.startDragging();
}

// requestCurrentDesktopWindowClose uses the native window close path so
// existing hide-on-close interception can keep the window lifecycle consistent.
export async function requestCurrentDesktopWindowClose() {
  const currentWindow = getDesktopFrameWindow();

  if (!currentWindow) {
    return;
  }

  await currentWindow.close();
}

// installDesktopEscapeClose keeps frameless desktop windows dismissible
// without stealing Escape from active text inputs or editable regions.
export function installDesktopEscapeClose(windowHandle = getCurrentWindow()) {
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
