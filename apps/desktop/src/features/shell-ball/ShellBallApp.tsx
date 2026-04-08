// 该文件承载悬浮球近场承接相关的界面逻辑。
import { Sparkles, WandSparkles } from "lucide-react";
import { StatusBadge, PanelSurface } from "@cialloclaw/ui";
import { useTaskStream } from "@/hooks/useTaskStream";
import { useShellBallStore } from "@/stores/shellBallStore";
import { useTaskStore } from "@/stores/taskStore";
import { formatStatusLabel } from "@/utils/formatters";

// ShellBallApp 处理当前模块的相关逻辑。
export function ShellBallApp() {
  const activeTaskId = useTaskStore((state) => state.activeTaskId);
  const activeTask = useTaskStore((state) => state.tasks.find((task) => task.task_id === state.activeTaskId) ?? null);
  const status = useShellBallStore((state) => state.status);

  useTaskStream(activeTaskId);

  return (
    <main className="app-shell glass-grid flex items-center justify-center">
      <div className="flex max-w-3xl items-center gap-10 rounded-[32px] border border-white/10 bg-slate-950/35 px-10 py-8 shadow-glow backdrop-blur-xl">
        <div className="relative flex h-44 w-44 items-center justify-center rounded-full border border-cyan-300/40 bg-[radial-gradient(circle_at_30%_30%,rgba(34,211,238,0.32),rgba(15,23,42,0.92))] shadow-[0_0_80px_rgba(34,211,238,0.35)]">
          <div className="absolute inset-4 rounded-full border border-white/10" />
          <WandSparkles className="h-10 w-10 text-cyan-200" />
        </div>
        <PanelSurface title="近场承接" eyebrow="shell-ball">
          <div className="space-y-4 text-sm text-slate-200">
            <p className="max-w-xl text-balance leading-6 text-slate-300">
              当前近场承接入口已经切换为 task-centric 骨架，可继续接选中文本、文件拖拽、悬停输入和语音提交链路。
            </p>
            <div className="flex items-center gap-3">
              <StatusBadge tone={activeTask?.status ?? "status"}>
                {activeTask ? formatStatusLabel(activeTask.status) : status}
              </StatusBadge>
              <span className="font-mono text-xs text-slate-400">task: {activeTaskId ?? "pending"}</span>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div className="mb-2 flex items-center gap-2 text-cyan-200">
                <Sparkles className="h-4 w-4" />
                <span className="text-xs uppercase tracking-[0.24em]">p0 chain</span>
              </div>
              <p>入口触发 - 意图确认 - 创建或更新 task - 工具执行 - 正式交付 - 仪表盘回显。</p>
            </div>
          </div>
        </PanelSurface>
      </div>
    </main>
  );
}
