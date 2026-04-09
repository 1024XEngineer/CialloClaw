// 该文件定义共享状态徽章组件。 
import type { ReactNode } from "react";

// tones 维护状态徽章的颜色映射。
const tones: Record<string, string> = {
  confirming_intent: "bg-status-confirming-intent/20 text-status-confirming-intent-foreground",
  processing: "bg-status-processing/20 text-status-processing-foreground",
  waiting_auth: "bg-status-waiting-auth/20 text-status-waiting-auth-foreground",
  waiting_input: "bg-status-waiting-input/20 text-status-waiting-input-foreground",
  paused: "bg-status-paused/20 text-status-paused-foreground",
  blocked: "bg-status-blocked/20 text-status-blocked-foreground",
  completed: "bg-status-completed/20 text-status-completed-foreground",
  failed: "bg-status-failed/20 text-status-failed-foreground",
  cancelled: "bg-status-cancelled/40 text-status-cancelled-foreground",
  ended_unfinished: "bg-status-ended-unfinished/30 text-status-ended-unfinished-foreground",
  pending_confirmation: "bg-status-waiting-auth/20 text-status-waiting-auth-foreground",
  intercepted: "bg-status-failed/20 text-status-failed-foreground",
  execution_error: "bg-status-failed/20 text-status-failed-foreground",
  recoverable: "bg-status-confirming-intent/20 text-status-confirming-intent-foreground",
  recovered: "bg-status-completed/20 text-status-completed-foreground",
  approved: "bg-status-completed/20 text-status-completed-foreground",
  denied: "bg-status-failed/20 text-status-failed-foreground",
  pending: "bg-status-waiting-auth/20 text-status-waiting-auth-foreground",
  status: "bg-status-paused/20 text-status-paused-foreground",
  intent_confirm: "bg-status-confirming-intent/20 text-status-confirming-intent-foreground",
  result: "bg-status-completed/20 text-status-completed-foreground",
  green: "bg-status-completed/20 text-status-completed-foreground",
  yellow: "bg-status-waiting-auth/20 text-status-waiting-auth-foreground",
  red: "bg-status-failed/20 text-status-failed-foreground",
};

// StatusBadge 处理当前模块的相关逻辑。
export function StatusBadge({ tone, children }: { tone: string; children: ReactNode }) {
  return (
    <span className={`inline-flex rounded-full px-3 py-1 text-xs font-medium ${tones[tone] ?? tones.status}`}>
      {children}
    </span>
  );
}
