import type { CSSProperties, ReactNode } from "react";

// Shared badge tones read the desktop theme tokens so formal statuses stay visually consistent across modules.
const toneStyles: Record<string, CSSProperties> = {
  confirming_intent: {
    background: "color-mix(in srgb, var(--cc-module-memory) 22%, white)",
    color: "var(--cc-module-memory-strong)",
  },
  processing: {
    background: "color-mix(in srgb, var(--cc-module-task) 22%, white)",
    color: "var(--cc-module-task-strong)",
  },
  waiting_auth: {
    background: "color-mix(in srgb, var(--cc-honey) 24%, white)",
    color: "#8a5a20",
  },
  waiting_input: {
    background: "color-mix(in srgb, var(--cc-module-notes) 22%, white)",
    color: "var(--cc-module-notes-strong)",
  },
  paused: {
    background: "rgba(255, 255, 255, 0.58)",
    color: "var(--cc-ink-muted)",
  },
  blocked: {
    background: "color-mix(in srgb, var(--cc-peach) 24%, white)",
    color: "#9a5d47",
  },
  completed: {
    background: "color-mix(in srgb, var(--cc-sage) 25%, white)",
    color: "var(--cc-module-safety-strong)",
  },
  failed: {
    background: "color-mix(in srgb, var(--cc-rose) 24%, white)",
    color: "#9c5360",
  },
  cancelled: {
    background: "color-mix(in srgb, var(--cc-cream-3) 55%, white)",
    color: "var(--cc-ink-muted)",
  },
  ended_unfinished: {
    background: "color-mix(in srgb, var(--cc-cream-3) 45%, white)",
    color: "var(--cc-ink-soft)",
  },
  status: {
    background: "rgba(255, 255, 255, 0.58)",
    color: "var(--cc-ink-soft)",
  },
};

export function StatusBadge({ tone, children }: { tone: string; children: ReactNode }) {
  // Protocol and view-model aliases map to the same compact visual language without redefining formal statuses.
  const aliases: Record<string, keyof typeof toneStyles> = {
    approved: "completed",
    denied: "failed",
    execution_error: "failed",
    green: "completed",
    intent_confirm: "confirming_intent",
    intercepted: "failed",
    pending: "waiting_auth",
    pending_confirmation: "waiting_auth",
    recovered: "completed",
    recoverable: "confirming_intent",
    red: "failed",
    result: "completed",
    yellow: "waiting_auth",
  };
  const normalizedTone = aliases[tone] ?? tone;
  const style = toneStyles[normalizedTone] ?? toneStyles.status;

  return (
    <span className="inline-flex rounded-full border px-3 py-1 text-xs font-semibold" style={{ borderColor: "var(--cc-line)", ...style }}>
      {children}
    </span>
  );
}
