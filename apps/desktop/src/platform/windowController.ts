import { Window } from "@tauri-apps/api/window";
import { requestShellBallDashboardOpenTransition } from "./dashboardWindowTransition";

export type DesktopWindowLabel = "dashboard" | "control-panel";

const desktopWindowOptions = {
  dashboard: {
    title: "CialloClaw Dashboard",
    width: 1280,
    height: 860,
    decorations: false,
    visible: false,
    url: "dashboard.html",
  },
  "control-panel": {
    title: "CialloClaw Control Panel",
    width: 1080,
    height: 760,
    decorations: false,
    visible: false,
    url: "control-panel.html",
  },
} as const satisfies Record<DesktopWindowLabel, {
  title: string;
  width: number;
  height: number;
  decorations: boolean;
  visible: boolean;
  url: string;
}>;

async function getOrCreateDesktopWindow(label: DesktopWindowLabel) {
  const existingWindow = await Window.getByLabel(label);

  if (existingWindow !== null) {
    return existingWindow;
  }

  return new Window(label, desktopWindowOptions[label]);
}

// 该文件封装桌面窗口控制能力。
export async function openOrFocusDesktopWindow(label: DesktopWindowLabel) {
  const windowHandle = await getOrCreateDesktopWindow(label);

  if (label === "dashboard") {
    await windowHandle.unminimize();
    await windowHandle.setFullscreen(true);
    await requestShellBallDashboardOpenTransition();
  } else {
    await windowHandle.unminimize();
  }

  await windowHandle.show();
  await windowHandle.setFocus();

  return label;
}

// closeDesktopWindow closes an existing desktop window when a flow needs to
// leave the current surface entirely instead of just hiding it.
export async function closeDesktopWindow(label: DesktopWindowLabel) {
  const windowHandle = await Window.getByLabel(label);

  if (windowHandle === null) {
    return;
  }

  await windowHandle.close();
}
