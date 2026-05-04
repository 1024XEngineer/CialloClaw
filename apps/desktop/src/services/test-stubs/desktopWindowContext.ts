export type DesktopWindowContextPayload = {
  app_name: string;
  browser_kind: "chrome" | "edge" | "other_browser" | "non_browser";
  process_path: string | null;
  process_id: number | null;
  title: string | null;
  url: string | null;
  visible_text?: string | null;
  hover_target?: string | null;
  error_text?: string | null;
  window_switch_count?: number | null;
  page_switch_count?: number | null;
};

export async function getActiveWindowContext(..._args: any[]): Promise<DesktopWindowContextPayload | null> {
  return null;
}
