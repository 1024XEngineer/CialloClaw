// 该文件承载仪表盘任务工作台相关的界面逻辑。
import { ShieldCheck, Workflow } from "lucide-react";
import { PanelSurface, StatusBadge } from "@cialloclaw/ui";
import { MetricCard } from "@/components/MetricCard";
import { mapTaskToDetailViewModel } from "@/models/TaskDetailViewModel";
import { useTaskStore } from "@/stores/taskStore";

// DashboardApp 处理当前模块的相关逻辑。
export function DashboardApp() {
  const tasks = useTaskStore((state) => state.tasks).map(mapTaskToDetailViewModel);

  return (
    <main className="app-shell">
      <div className="mx-auto flex max-w-6xl flex-col gap-6">
        <header className="flex flex-col gap-3 rounded-[32px] border border-white/10 bg-slate-950/45 p-8 backdrop-blur-xl md:flex-row md:items-end md:justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.3em] text-cyan-300/75">dashboard</p>
            <h1 className="mt-3 text-4xl font-semibold text-white">P0 任务工作台</h1>
            <p className="mt-3 max-w-3xl text-slate-300">
              任务状态、正式交付、安全摘要与记忆回填会统一落到同一个 task 视角工作台里。
            </p>
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            <MetricCard label="进行中任务" value="1" detail="已预置一个示例 task，用于串接仪表盘主链路。" />
            <MetricCard label="记忆命中" value="2" detail="为 FTS5 + sqlite-vec 记忆命中结果预留了展示位置。" />
            <MetricCard label="风险态势" value="green" detail="授权、审计与恢复点通道已经预留骨架。" />
          </div>
        </header>

        <div className="grid gap-6 lg:grid-cols-[1.4fr_1fr]">
          <PanelSurface title="任务概览" eyebrow="task-state">
            <div className="space-y-4">
              {tasks.map((task) => (
                <article key={task.id} className="rounded-2xl border border-white/10 bg-white/5 p-4">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="text-lg text-white">{task.title}</h2>
                      <p className="mt-1 font-mono text-xs text-slate-400">{task.id}</p>
                    </div>
                    <StatusBadge tone={task.statusTone}>{task.statusLabel}</StatusBadge>
                  </div>
                  <p className="mt-4 text-sm text-slate-300">开始时间：{task.startedAtLabel}</p>
                </article>
              ))}
            </div>
          </PanelSurface>

          <div className="grid gap-6">
            <PanelSurface title="安全治理" eyebrow="risk-and-approval">
              <div className="flex items-start gap-3 text-sm text-slate-300">
                <ShieldCheck className="mt-0.5 h-5 w-5 text-emerald-300" />
                <p>风险、审计、恢复点和工作区边界模块已在 Go harness 骨架中分层隔离。</p>
              </div>
            </PanelSurface>

            <PanelSurface title="Worker 与能力接入" eyebrow="capability-access">
              <div className="flex items-start gap-3 text-sm text-slate-300">
                <Workflow className="mt-0.5 h-5 w-5 text-cyan-300" />
                <p>Playwright、OCR 和 media worker 已隔离在 `workers/*`，仅允许由 harness 编排接入。</p>
              </div>
            </PanelSurface>
          </div>
        </div>
      </div>
    </main>
  );
}
