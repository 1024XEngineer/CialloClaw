export type ControlPanelAboutAction = "share";

export type ControlPanelAboutSnapshot = {
  appName: string;
  appVersion: string;
};

export type ControlPanelAboutFeedbackChannel =
  | {
      actionLabel: string;
      description: string;
      href: string;
      hrefLabel: string;
      id: string;
      kind: "link";
      title: string;
    }
  | {
      description: string;
      id: string;
      kind: "image";
      note?: string;
      previewAlt: string;
      previewSrc: string;
      title: string;
    }
  | {
      description: string;
      id: string;
      kind: "placeholder";
      note: string;
      placeholderLabel: string;
      title: string;
    };

const CONTROL_PANEL_ABOUT_URLS = {
  feedback: "https://github.com/1024XEngineer/CialloClaw/issues",
  share: "https://github.com/1024XEngineer/CialloClaw",
} as const;

const CONTROL_PANEL_ABOUT_FEEDBACK_CHANNELS = [
  {
    description: "公开问题反馈、功能建议与版本回归记录。",
    href: CONTROL_PANEL_ABOUT_URLS.feedback,
    hrefLabel: "github.com/1024XEngineer/CialloClaw/issues",
    id: "github_issues",
    kind: "link",
    actionLabel: "复制链接",
    title: "GitHub Issues",
  },
  {
    description: "预留微信群、QQ群或 Discord 等社群二维码图片。",
    id: "community_qr",
    kind: "placeholder",
    note: "后续放入二维码图片后，会在这里直接显示预览。",
    placeholderLabel: "待放置二维码图片",
    title: "社群二维码",
  },
  {
    description: "预留邮箱、工单表单或其它定向联系入口。",
    id: "contact_form",
    kind: "placeholder",
    note: "支持后续替换成链接、表单地址或其它说明文本。",
    placeholderLabel: "待放置链接或表单",
    title: "邮箱 / 表单",
  },
] as const satisfies readonly ControlPanelAboutFeedbackChannel[];

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
 * Returns the feedback channel definitions used by the control-panel about page.
 *
 * Link and image values live in one dedicated place so the React view does not
 * hardcode channel-specific copy, URLs, or future QR image paths.
 *
 * @returns The list of feedback channels shown in the about section.
 */
export function getControlPanelAboutFeedbackChannels(): ControlPanelAboutFeedbackChannel[] {
  return CONTROL_PANEL_ABOUT_FEEDBACK_CHANNELS.map((channel) => ({ ...channel }));
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

/**
 * Copies an about-page value without touching formal settings state.
 *
 * @param value Value copied to the clipboard.
 * @param successMessage User-facing confirmation shown in the control panel.
 * @returns Success or fallback copy for the current runtime.
 */
export async function copyControlPanelAboutValue(value: string, successMessage: string) {
  if (globalThis.navigator?.clipboard?.writeText) {
    try {
      await globalThis.navigator.clipboard.writeText(value);
      return successMessage;
    } catch {
      return `当前环境暂不支持直接复制，请手动处理：${value}`;
    }
  }

  return `当前环境暂不支持直接复制，请手动处理：${value}`;
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
    case "share":
      return copyControlPanelAboutValue(CONTROL_PANEL_ABOUT_URLS.share, "已复制分享链接。");
  }
}
