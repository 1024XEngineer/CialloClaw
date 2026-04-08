// 该文件定义共享状态徽章组件。 
import type { ReactNode } from "react";

const tones: Record<string, string> = {
  confirming_intent: "bg-sky-400/20 text-sky-100",
  processing: "bg-cyan-400/20 text-cyan-100",
  waiting_auth: "bg-amber-400/20 text-amber-100",
  waiting_input: "bg-violet-400/20 text-violet-100",
  paused: "bg-slate-500/20 text-slate-100",
  blocked: "bg-orange-400/20 text-orange-100",
  completed: "bg-emerald-400/20 text-emerald-100",
  failed: "bg-rose-400/20 text-rose-100",
  cancelled: "bg-slate-700/40 text-slate-200",
  ended_unfinished: "bg-zinc-500/30 text-zinc-100",
  pending_confirmation: "bg-amber-400/20 text-amber-100",
  intercepted: "bg-rose-400/20 text-rose-100",
  execution_error: "bg-rose-400/20 text-rose-100",
  recoverable: "bg-sky-400/20 text-sky-100",
  recovered: "bg-emerald-400/20 text-emerald-100",
  approved: "bg-emerald-400/20 text-emerald-100",
  denied: "bg-rose-400/20 text-rose-100",
  pending: "bg-amber-400/20 text-amber-100",
  status: "bg-slate-500/20 text-slate-100",
  intent_confirm: "bg-sky-400/20 text-sky-100",
  result: "bg-emerald-400/20 text-emerald-100",
  green: "bg-emerald-400/20 text-emerald-100",
  yellow: "bg-amber-400/20 text-amber-100",
  red: "bg-rose-400/20 text-rose-100",
};

export function StatusBadge({ tone, children }: { tone: string; children: ReactNode }) {
  return (
    <span className={`inline-flex rounded-full px-3 py-1 text-xs font-medium ${tones[tone] ?? tones.status}`}>
      {children}
    </span>
  );
}
