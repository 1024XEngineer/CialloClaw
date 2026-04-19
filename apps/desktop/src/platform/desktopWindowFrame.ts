import { getCurrentWindow } from "@tauri-apps/api/window";

function getDesktopFrameWindow() {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return getCurrentWindow();
  } catch {
    return null;
  }
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

type DesktopCloseHandle = {
  close: () => Promise<void>;
};

// installDesktopEscapeClose keeps frameless desktop windows dismissible
// without stealing Escape from active text inputs or editable regions.
export function installDesktopEscapeClose(windowHandle?: DesktopCloseHandle | null) {
  if (typeof window === "undefined") {
    return;
  }

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
      const currentWindow = windowHandle ?? getDesktopFrameWindow();

      if (!currentWindow) {
        closing = false;
        return;
      }

      void currentWindow.close().finally(() => {
        closing = false;
      });
    });
  });
}
