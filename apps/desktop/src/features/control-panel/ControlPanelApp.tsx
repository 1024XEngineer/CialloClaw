import { Cpu, Database, MonitorCog } from "lucide-react";
import { PanelSurface, StatusBadge } from "@cialloclaw/ui";
import { loadSettings } from "@/services/settingsService";

export function ControlPanelApp() {
  const settings = loadSettings();
  const snapshot = settings.settings;

  return (
    <main className="app-shell">
      <div className="mx-auto grid max-w-5xl gap-6 lg:grid-cols-3">
        <PanelSurface title="桌面入口" eyebrow="general">
          <div className="space-y-4 text-sm text-slate-300">
            <div className="flex items-center gap-3 text-white">
              <MonitorCog className="h-5 w-5 text-cyan-300" />
              <span>Windows 优先的 Tauri 桌面宿主</span>
            </div>
            <StatusBadge tone={snapshot.floating_ball.auto_snap ? "processing" : "cancelled"}>
              悬浮球自动贴边 {snapshot.floating_ball.auto_snap ? "开启" : "关闭"}
            </StatusBadge>
            <p>主题模式：{snapshot.general.theme_mode}</p>
            <p>语音通知：{snapshot.general.voice_notification_enabled ? "开启" : "关闭"}</p>
            <p>工作区路径：{snapshot.general.download.workspace_path}</p>
          </div>
        </PanelSurface>

        <PanelSurface title="记忆" eyebrow="local-rag">
          <div className="space-y-4 text-sm text-slate-300">
            <div className="flex items-center gap-3 text-white">
              <Database className="h-5 w-5 text-orange-300" />
              <span>SQLite + FTS5 + sqlite-vec</span>
            </div>
            <StatusBadge tone={snapshot.memory.enabled ? "green" : "cancelled"}>
              记忆 {snapshot.memory.enabled ? "启用" : "停用"}
            </StatusBadge>
            <p>生命周期：{snapshot.memory.lifecycle}</p>
            <p>
              工作总结频率：{snapshot.memory.work_summary_interval.value}
              {snapshot.memory.work_summary_interval.unit}
            </p>
          </div>
        </PanelSurface>

        <PanelSurface title="自动化与日志" eyebrow="task-automation">
          <div className="space-y-4 text-sm text-slate-300">
            <div className="flex items-center gap-3 text-white">
              <Cpu className="h-5 w-5 text-cyan-300" />
              <span>{snapshot.data_log.provider}</span>
            </div>
            <StatusBadge tone={snapshot.data_log.budget_auto_downgrade ? "green" : "cancelled"}>
              预算自动降级 {snapshot.data_log.budget_auto_downgrade ? "开启" : "关闭"}
            </StatusBadge>
            <p>
              巡检频率：{snapshot.task_automation.inspection_interval.value}
              {snapshot.task_automation.inspection_interval.unit}
            </p>
            <p>巡检来源：{snapshot.task_automation.task_sources.join("、")}</p>
            <p>模型调用入口被限制在 Go harness 的 `internal/model` 层，不直接暴露给前端业务层。</p>
          </div>
        </PanelSurface>
      </div>
    </main>
  );
}
