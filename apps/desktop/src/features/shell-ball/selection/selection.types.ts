import type { PageContext } from "@cialloclaw/protocol";

/**
 * Describes a concrete text selection captured by a platform-specific adapter
 * that can be routed into the shell-ball near-field entry flow.
 */
export type ShellBallSelectionSnapshot = {
  text: string;
  page_context: PageContext;
  source: "windows_uia";
  updated_at: string;
};
