export type ControlPanelAboutAction = "help" | "feedback" | "share";

export type ControlPanelAboutSnapshot = {
  appName: string;
  appVersion: string;
};

const CONTROL_PANEL_ABOUT_URLS = {
  feedback: "https://github.com/1024XEngineer/CialloClaw/issues",
  help: "https://github.com/1024XEngineer/CialloClaw",
  share: "https://github.com/1024XEngineer/CialloClaw",
} as const;

const CONTROL_PANEL_ABOUT_FALLBACK_SNAPSHOT: ControlPanelAboutSnapshot = {
  appName: "CialloClaw",
  appVersion: "0.1.0",
};

/**
 * Returns the stable fallback metadata used before the desktop runtime replies.
 *
 * @returns The fallback about snapshot for the control panel window.
 */
export function getControlPanelAboutFallbackSnapshot(): ControlPanelAboutSnapshot {
  return { ...CONTROL_PANEL_ABOUT_FALLBACK_SNAPSHOT };
}

/**
 * Resolves the external URL behind each non-copy about action.
 *
 * @param action About action that opens an external page.
 * @returns The URL used by that action.
 */
export function resolveControlPanelAboutActionUrl(action: Exclude<ControlPanelAboutAction, "share">): string {
  return CONTROL_PANEL_ABOUT_URLS[action];
}

/**
 * Loads desktop runtime metadata for the control-panel about surface while
 * preserving a static fallback when the Tauri app API is unavailable.
 *
 * @returns Desktop metadata for the about section.
 */
export async function loadControlPanelAboutSnapshot(): Promise<ControlPanelAboutSnapshot> {
  try {
    const appApi = await import("@tauri-apps/api/app");
    const [appName, appVersion] = await Promise.all([appApi.getName(), appApi.getVersion()]);

    return {
      appName,
      appVersion,
    };
  } catch {
    return getControlPanelAboutFallbackSnapshot();
  }
}

function openControlPanelAboutUrl(url: string, successMessage: string) {
  if (typeof window !== "undefined" && typeof window.open === "function") {
    window.open(url, "_blank", "noopener,noreferrer");
    return successMessage;
  }

  return `当前环境暂不支持直接打开，请访问：${url}`;
}

async function copyControlPanelShareUrl(url: string) {
  if (globalThis.navigator?.clipboard?.writeText) {
    try {
      await globalThis.navigator.clipboard.writeText(url);
      return "已复制分享链接。";
    } catch {
      return `当前环境暂不支持直接复制，请手动分享：${url}`;
    }
  }

  return `当前环境暂不支持直接复制，请手动分享：${url}`;
}

/**
 * Executes a local control-panel about action without touching formal settings
 * state, because these buttons only open external help surfaces or copy links.
 *
 * @param action User-selected about action.
 * @returns User-facing feedback for the control panel surface.
 */
export async function runControlPanelAboutAction(action: ControlPanelAboutAction): Promise<string> {
  switch (action) {
    case "help":
      return openControlPanelAboutUrl(CONTROL_PANEL_ABOUT_URLS.help, "已打开帮助与项目主页。");
    case "feedback":
      return openControlPanelAboutUrl(CONTROL_PANEL_ABOUT_URLS.feedback, "已打开反馈页。");
    case "share":
      return copyControlPanelShareUrl(CONTROL_PANEL_ABOUT_URLS.share);
  }
}
