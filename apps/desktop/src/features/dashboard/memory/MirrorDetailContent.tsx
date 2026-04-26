import { useCallback, useEffect, useMemo, useState } from "react";
import type { AgentMirrorOverviewGetResult, RecoveryPoint } from "@cialloclaw/protocol";
import { BookMarked, BrainCircuit, CalendarDays } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { StatusBadge } from "@cialloclaw/ui";
import { SegmentedControl, Switch } from "@radix-ui/themes";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  buildDashboardSafetyCardNavigationState,
  buildDashboardSafetyRestorePointNavigationState,
} from "@/features/dashboard/shared/dashboardSafetyNavigation";
import { navigateToDashboardTaskDetail } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import type { DashboardSettingsPatch } from "@/features/dashboard/shared/dashboardSettingsMutation";
import {
  formatDashboardMemoryLifecycle,
  formatDashboardTimeInterval,
  type DashboardSettingsSnapshotData,
} from "@/features/dashboard/shared/dashboardSettingsSnapshot";
import {
  buildMirrorConversationSummary,
  buildMirrorConversationDateOptions,
  buildMirrorConversationTaskMoments,
  filterMirrorConversationRecords,
  formatMirrorConversationRecordMoment,
  getMirrorConversationInputModeLabel,
  getMirrorConversationSourceLabel,
  getMirrorConversationTriggerLabel,
  groupMirrorConversationRecords,
  type MirrorConversationFilters,
  type MirrorConversationInputModeFilter,
  type MirrorConversationScopeFilter,
  type MirrorConversationSummary,
  type MirrorDailyDigest,
  type MirrorProfileItemView,
  type MirrorProfileView,
} from "./mirrorViewModel";
import type { MirrorDirectionKey } from "./mirrorDirections";
import type { MirrorConversationRecord } from "@/services/mirrorMemoryService";

type MirrorDetailContentProps = {
  activeDetailKey: MirrorDirectionKey;
  overview: AgentMirrorOverviewGetResult;
  latestRestorePoint: RecoveryPoint | null;
  onUpdateSettings: (subject: string, patch: DashboardSettingsPatch) => Promise<string>;
  settingsSnapshot: DashboardSettingsSnapshotData;
  rpcContext: {
    serverTime: string | null;
    warnings: string[];
  };
  conversations: MirrorConversationRecord[];
  conversationSummary: MirrorConversationSummary;
  dailyDigest: MirrorDailyDigest;
  focusMemoryId: string | null;
  historyDetailView: MirrorHistoryDetailView;
  profileView: MirrorProfileView;
};

export type MirrorHistoryDetailView = "summary" | "conversation";

function MirrorEmptyState({ children }: { children: string }) {
  return <p className="mirror-page__empty-state">{children}</p>;
}

function MirrorHistoryDetail({
  overview,
  conversations,
  conversationSummary,
  historyDetailView,
  onOpenTaskDetail,
}: Pick<MirrorDetailContentProps, "overview" | "conversations" | "conversationSummary"> & {
  historyDetailView: MirrorHistoryDetailView;
  onOpenTaskDetail: (taskId: string) => void;
}) {
  const [conversationScopeFilter, setConversationScopeFilter] = useState<MirrorConversationScopeFilter>("all");
  const [conversationSourceFilter, setConversationSourceFilter] = useState<MirrorConversationRecord["source"] | "all">("all");
  const [conversationInputModeFilter, setConversationInputModeFilter] = useState<MirrorConversationInputModeFilter>("all");
  const [conversationDateFilter, setConversationDateFilter] = useState<string | "all" | null>(null);
  const conversationDateOptions = useMemo(() => buildMirrorConversationDateOptions(conversations), [conversations]);
  const oldestConversationDateKey = conversationDateOptions.at(-1)?.date_key ?? null;
  const newestConversationDateKey = conversationDateOptions[0]?.date_key ?? null;
  const effectiveConversationDateFilter = conversationDateFilter ?? newestConversationDateKey ?? "all";
  const conversationFilters = useMemo(
    () =>
      ({
        scope: conversationScopeFilter,
        source: conversationSourceFilter,
        input_mode: conversationInputModeFilter,
        date_key: effectiveConversationDateFilter,
      } satisfies MirrorConversationFilters),
    [conversationInputModeFilter, conversationScopeFilter, conversationSourceFilter, effectiveConversationDateFilter],
  );
  const filteredConversations = useMemo(
    () => filterMirrorConversationRecords(conversations, conversationFilters),
    [conversationFilters, conversations],
  );
  const groupedConversations = useMemo(() => groupMirrorConversationRecords(filteredConversations), [filteredConversations]);
  const dominantSource = conversationSummary.dominant_source ? getMirrorConversationSourceLabel(conversationSummary.dominant_source) : "等待新记录";
  const dominantMode = conversationSummary.dominant_input_mode ? getMirrorConversationInputModeLabel(conversationSummary.dominant_input_mode) : "等待新记录";
  const taskLinkedConversationCount = conversations.filter((record) => record.task_id).length;
  const failedConversationCount = conversations.filter((record) => record.status === "failed").length;
  const dashboardConversationCount = conversations.filter((record) => record.source === "dashboard").length;
  const floatingBallConversationCount = conversations.filter((record) => record.source === "floating_ball").length;
  const trayPanelConversationCount = conversations.filter((record) => record.source === "tray_panel").length;
  const voiceConversationCount = conversations.filter((record) => record.input_mode === "voice").length;
  const textConversationCount = conversations.filter((record) => record.input_mode === "text").length;

  useEffect(() => {
    if (conversationDateFilter && !conversationDateOptions.some((option) => option.date_key === conversationDateFilter)) {
      setConversationDateFilter(newestConversationDateKey ?? "all");
    }
  }, [conversationDateFilter, conversationDateOptions, newestConversationDateKey]);

  return (
    <div className="mirror-page__detail-tabs">
      {historyDetailView === "summary" ? (
        <>
        <div className="mirror-page__history-summary-grid">
          <article className="mirror-page__continuity-card">
            <p className="mirror-page__micro-label">本地完整记录</p>
            <p className="mirror-page__summary-value">{conversationSummary.total_records}</p>
            <p className="mirror-page__summary-copy">统计最近 100 条本地输入与前端可见回应记录。</p>
          </article>
          <article className="mirror-page__continuity-card">
            <p className="mirror-page__micro-label">最近常见入口</p>
            <p className="mirror-page__stage-headline">{dominantSource}</p>
            <p className="mirror-page__summary-copy">按最近本地记录统计，常见输入方式为 {dominantMode}。</p>
          </article>
          <article className="mirror-page__continuity-card">
            <p className="mirror-page__micro-label">最近一条本地记录</p>
            <p className="mirror-page__stage-headline">
              {conversationSummary.latest_at ? formatMirrorConversationRecordMoment(conversations[0]) : "暂无本地会话"}
            </p>
            <p className="mirror-page__summary-copy">{conversationSummary.latest_agent_text ?? conversationSummary.latest_user_text ?? "下一条本地记录会显示在这里。"}</p>
          </article>
          <article className="mirror-page__continuity-card">
            <p className="mirror-page__micro-label">挂载任务记录</p>
            <p className="mirror-page__stage-headline">{taskLinkedConversationCount} 条</p>
            <p className="mirror-page__summary-copy">这些记录可以直接回跳到对应任务详情，不需要把镜子当成聊天历史翻页。</p>
          </article>
        </div>

        {overview.history_summary.length > 0 ? (
          <div className="mirror-page__history-list">
            {overview.history_summary.map((item, index) => (
              <article key={`${item}-${index}`} className="mirror-page__history-item">
                <div className="mirror-page__history-index">0{index + 1}</div>
                <div className="mirror-page__history-copy">
                  <p className="mirror-page__history-label">后端历史概要 {index + 1}</p>
                  <p className="mirror-page__history-text">{item}</p>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <MirrorEmptyState>暂无历史概要。</MirrorEmptyState>
        )}
        </>
      ) : (
        <>
        <div className="mirror-page__conversation-filter-shell">
          <div className="mirror-page__conversation-filter-bar">
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationScopeFilter === "all" ? " is-active" : ""}`}
              onClick={() => setConversationScopeFilter("all")}
            >
              全部 {conversations.length}
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationScopeFilter === "with_task" ? " is-active" : ""}`}
              onClick={() => setConversationScopeFilter("with_task")}
            >
              已挂任务 {taskLinkedConversationCount}
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationScopeFilter === "failed" ? " is-active" : ""}`}
              onClick={() => setConversationScopeFilter("failed")}
            >
              失败记录 {failedConversationCount}
            </button>
          </div>

          <div className="mirror-page__conversation-filter-bar">
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationSourceFilter === "all" ? " is-active" : ""}`}
              onClick={() => setConversationSourceFilter("all")}
            >
              全部入口
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationSourceFilter === "dashboard" ? " is-active" : ""}`}
              onClick={() => setConversationSourceFilter("dashboard")}
            >
              仪表盘 {dashboardConversationCount}
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationSourceFilter === "floating_ball" ? " is-active" : ""}`}
              onClick={() => setConversationSourceFilter("floating_ball")}
            >
              悬浮球 {floatingBallConversationCount}
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationSourceFilter === "tray_panel" ? " is-active" : ""}`}
              onClick={() => setConversationSourceFilter("tray_panel")}
            >
              托盘面板 {trayPanelConversationCount}
            </button>
          </div>

          <div className="mirror-page__conversation-filter-bar">
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationInputModeFilter === "all" ? " is-active" : ""}`}
              onClick={() => setConversationInputModeFilter("all")}
            >
              全部输入
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationInputModeFilter === "voice" ? " is-active" : ""}`}
              onClick={() => setConversationInputModeFilter("voice")}
            >
              语音 {voiceConversationCount}
            </button>
            <button
              type="button"
              className={`mirror-page__conversation-filter${conversationInputModeFilter === "text" ? " is-active" : ""}`}
              onClick={() => setConversationInputModeFilter("text")}
            >
              文本 {textConversationCount}
            </button>
          </div>

          {conversationDateOptions.length > 0 ? (
            <div className="mirror-page__conversation-date-shell">
              <label className="mirror-page__conversation-date-control">
                <span className="mirror-page__micro-label">按日期回看</span>
                <input
                  type="date"
                  className="mirror-page__conversation-date-input"
                  value={effectiveConversationDateFilter === "all" ? "" : effectiveConversationDateFilter}
                  min={oldestConversationDateKey ?? undefined}
                  max={newestConversationDateKey ?? undefined}
                  onChange={(event) => {
                    const nextValue = event.currentTarget.value.trim();
                    setConversationDateFilter(nextValue.length > 0 ? nextValue : "all");
                  }}
                />
              </label>
              <div className="mirror-page__conversation-date-meta">
                <p className="mirror-page__summary-copy">
                  {oldestConversationDateKey && newestConversationDateKey
                    ? `留空显示全部日期，可修改范围 ${oldestConversationDateKey} ~ ${newestConversationDateKey}。`
                    : "留空显示全部日期。"}
                </p>
                {conversationDateFilter === "all" ? (
                  <button type="button" className="mirror-page__task-link" onClick={() => setConversationDateFilter(newestConversationDateKey ?? "all")}>
                    回到最新日期
                  </button>
                ) : null}
                {effectiveConversationDateFilter !== "all" ? (
                  <button type="button" className="mirror-page__task-link" onClick={() => setConversationDateFilter("all")}>
                    清除日期
                  </button>
                ) : null}
              </div>
            </div>
          ) : null}
        </div>

        {groupedConversations.length === 0 ? (
          <MirrorEmptyState>{conversationScopeFilter === "all" ? "最近 100 条本地对话还没有记录。" : "当前筛选条件下没有命中的本地记录。"}</MirrorEmptyState>
        ) : (
          <ScrollArea className="mirror-page__conversation-scroll" data-testid="mirror-conversation-list">
            <div className="mirror-page__conversation-days">
              {groupedConversations.map((group) => (
                (() => {
                  const orderedTasks = buildMirrorConversationTaskMoments(group.items);

                  return (
                    <section key={group.date_key} className="mirror-page__conversation-day">
                      <div className="mirror-page__conversation-day-header">
                        <p className="mirror-page__micro-label">{group.label}</p>
                        <StatusBadge tone="processing">{group.items.length} 条</StatusBadge>
                      </div>

                      {orderedTasks.length > 0 ? (
                        <div className="mirror-page__stage-task-list">
                          {orderedTasks.map((task) => (
                            <div key={`${group.date_key}-${task.task_id}`} className="mirror-page__stage-task">
                              <div>
                                <p className="mirror-page__history-label">{task.task_id}</p>
                                <p className="mirror-page__summary-copy">
                                  {new Date(task.latest_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })} · {task.count} 条相关记录
                                </p>
                              </div>
                              <div className="mirror-page__stage-task-actions">
                                <button type="button" className="mirror-page__task-link" onClick={() => onOpenTaskDetail(task.task_id)}>
                                  打开任务
                                </button>
                              </div>
                            </div>
                          ))}
                        </div>
                      ) : null}

                      <div className="mirror-page__conversation-records">
                        {group.items.map((record) => (
                          <article
                            key={record.record_id}
                            className="mirror-page__conversation-record"
                            data-testid={`mirror-conversation-record-${record.record_id}`}
                          >
                            <div className="mirror-page__conversation-meta">
                              <div className="mirror-page__conversation-meta-copy">
                                <p className="mirror-page__micro-label">{formatMirrorConversationRecordMoment(record)}</p>
                                <p className="mirror-page__summary-copy">
                                  {getMirrorConversationSourceLabel(record.source)} · {getMirrorConversationInputModeLabel(record.input_mode)} · {getMirrorConversationTriggerLabel(record.trigger)}
                                  {record.task_id ? ` · ${record.task_id}` : ""}
                                </p>
                              </div>
                              <StatusBadge tone={record.status === "failed" ? "red" : record.agent_bubble_type ?? "processing"}>
                                {record.status === "failed" ? "失败" : record.agent_text ? "已回应" : "等待回应"}
                              </StatusBadge>
                            </div>
                            {record.task_id ? (
                              <div className="mirror-page__conversation-actions">
                                <button type="button" className="mirror-page__task-link" onClick={() => onOpenTaskDetail(record.task_id!)}>
                                  查看关联任务
                                </button>
                              </div>
                            ) : null}

                            <div className="mirror-page__conversation-bubble mirror-page__conversation-bubble--user">
                              <p className="mirror-page__history-label">用户输入</p>
                              <p className="mirror-page__history-text">{record.user_text}</p>
                            </div>

                            <div
                              className={`mirror-page__conversation-bubble ${record.status === "failed" ? "mirror-page__conversation-bubble--failed" : "mirror-page__conversation-bubble--agent"}`}
                            >
                              <p className="mirror-page__history-label">前端可见回应</p>
                              <p className="mirror-page__history-text">{record.agent_text ?? record.error_message ?? "当前没有前端可见回应。"}</p>
                            </div>
                          </article>
                        ))}
                      </div>
                    </section>
                  );
                })()
              ))}
            </div>
          </ScrollArea>
        )}
        </>
      )}
    </div>
  );
}

function MirrorDailyDetail({
  dailyDigest,
  latestRestorePoint,
  onOpenHistoryDetail,
  onOpenSafetyCardDetail,
  onOpenRestorePoint,
  onOpenTaskDetail,
}: Pick<MirrorDetailContentProps, "dailyDigest" | "latestRestorePoint"> & {
  onOpenHistoryDetail: () => void;
  onOpenSafetyCardDetail: (detailKey: "status" | "budget") => void;
  onOpenRestorePoint: (restorePoint: RecoveryPoint) => void;
  onOpenTaskDetail: (taskId: string) => void;
}) {
  return (
    <Tabs className="mirror-page__detail-tabs" defaultValue="today">
      <TabsList className="mirror-page__detail-tab-list" variant="line">
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="today">
          今日汇总
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="stages">
          阶段与进展
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="context">
          权限 / 风险 / 结果
        </TabsTrigger>
      </TabsList>

      <TabsContent className="mirror-page__detail-tab-panel" value="today">
        <div className="mirror-page__daily-stack mirror-page__daily-stack--expanded">
          <div className="mirror-page__date-card">
            <CalendarDays className="mirror-page__date-icon" />
            <div>
              <p className="mirror-page__micro-label">统计日期</p>
              <p className="mirror-page__date-value">{new Date(dailyDigest.date).toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric" })}</p>
            </div>
          </div>

          <div className="mirror-page__summary-grid">
            {dailyDigest.stats.map((stat) => (
              <article key={stat.id} className="mirror-page__summary-card">
                <p className="mirror-page__micro-label">{stat.label}</p>
                <p className="mirror-page__summary-value">{stat.value}</p>
                <p className="mirror-page__summary-copy">{stat.detail}</p>
              </article>
            ))}
          </div>

          <div className="mirror-page__detail-note-shell mirror-page__detail-note-shell--stage">
            <p className="mirror-page__micro-label">今日概览</p>
            <p className="mirror-page__stage-headline">{dailyDigest.headline}</p>
            <p className="mirror-page__note">{dailyDigest.lede}</p>
          </div>
        </div>
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="stages">
        <div className="mirror-page__stage-grid">
          {dailyDigest.stage_snapshots.map((snapshot) => (
            <article key={snapshot.id} className="mirror-page__stage-card">
              <div className="mirror-page__stage-card-top">
                <div>
                  <p className="mirror-page__micro-label">{snapshot.label}</p>
                  <p className="mirror-page__stage-headline">{snapshot.count} 条</p>
                </div>
                <StatusBadge tone={snapshot.tone}>{snapshot.count > 0 ? "已命中" : "平静"}</StatusBadge>
              </div>
              <p className="mirror-page__summary-copy">{snapshot.description}</p>
              {snapshot.tasks.length > 0 ? (
                <div className="mirror-page__stage-task-list">
                  {snapshot.tasks.map((task) => (
                    <div key={task.task_id} className="mirror-page__stage-task">
                      <div>
                        <p className="mirror-page__history-label">{task.title}</p>
                        <p className="mirror-page__summary-copy">{task.moment ? new Date(task.moment).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }) : "等待时间戳"}</p>
                      </div>
                      <div className="mirror-page__stage-task-actions">
                        <StatusBadge tone={task.status}>{task.status}</StatusBadge>
                        <button type="button" className="mirror-page__task-link" onClick={() => onOpenTaskDetail(task.task_id)}>
                          打开任务
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : null}
            </article>
          ))}
        </div>
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="context">
        <div className="mirror-page__risk-list">
          {dailyDigest.context_notes.map((note) => (
            <article key={note.id} className="mirror-page__risk-card">
              <div className="mirror-page__stage-card-top">
                <div>
                  <p className="mirror-page__micro-label">{note.label}</p>
                  <p className="mirror-page__stage-headline">{note.value}</p>
                </div>
                <StatusBadge tone={note.tone}>{note.label}</StatusBadge>
              </div>
              <p className="mirror-page__summary-copy">{note.detail}</p>
              {note.id === "approvals" ? (
                <div className="mirror-page__conversation-actions">
                  <button type="button" className="mirror-page__task-link" onClick={() => onOpenSafetyCardDetail("status")}>
                    前往安全详情
                  </button>
                </div>
              ) : null}
              {note.id === "restore-point" && latestRestorePoint ? (
                <div className="mirror-page__conversation-actions">
                  <button type="button" className="mirror-page__task-link" onClick={() => onOpenRestorePoint(latestRestorePoint)}>
                    前往恢复点
                  </button>
                </div>
              ) : null}
              {note.id === "cost" ? (
                <div className="mirror-page__conversation-actions">
                  <button type="button" className="mirror-page__task-link" onClick={() => onOpenSafetyCardDetail("budget")}>
                    前往预算详情
                  </button>
                </div>
              ) : null}
              {note.id === "continuity" ? (
                <div className="mirror-page__conversation-actions">
                  <button type="button" className="mirror-page__task-link" onClick={() => onOpenHistoryDetail()}>
                    前往本地对话
                  </button>
                </div>
              ) : null}
            </article>
          ))}
        </div>
      </TabsContent>
    </Tabs>
  );
}

function MirrorProfileGrid({
  items,
  emptyState,
  badgeTone,
}: {
  items: MirrorProfileItemView[];
  emptyState: string;
  badgeTone: "green" | "processing";
}) {
  if (items.length === 0) {
    return <MirrorEmptyState>{emptyState}</MirrorEmptyState>;
  }

  return (
    <div className="mirror-page__profile-grid">
      {items.map((item) => (
        <article key={item.id} className="mirror-page__profile-item" data-testid={`mirror-profile-item-${item.id}`}>
          <div className="mirror-page__stage-card-top">
            <div>
              <p className="mirror-page__micro-label">{item.label}</p>
              <p className="mirror-page__stage-headline">{item.value}</p>
            </div>
            <div className="mirror-page__profile-badges">
              <StatusBadge tone={badgeTone}>{item.source_label}</StatusBadge>
            </div>
          </div>
          <p className="mirror-page__summary-copy">{item.hint}</p>
        </article>
      ))}
    </div>
  );
}

function MirrorProfileDetail({
  backendItems,
  localItems,
}: {
  backendItems: MirrorProfileItemView[];
  localItems: MirrorProfileItemView[];
}) {
  const defaultTab = backendItems.length > 0 ? "backend" : "local";

  return (
    <Tabs className="mirror-page__detail-tabs" defaultValue={defaultTab}>
      <TabsList className="mirror-page__detail-tab-list" variant="line">
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="backend">
          后端画像字段
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="local">
          最近本地统计
        </TabsTrigger>
      </TabsList>

      <TabsContent className="mirror-page__detail-tab-panel" value="backend">
        <div className="mirror-page__profile-local-note">
          <BrainCircuit className="mirror-page__profile-icon" />
          <p className="mirror-page__summary-copy">这里直接展示后端 `mirror overview` 返回的 profile 字段。</p>
        </div>

        <MirrorProfileGrid badgeTone="green" emptyState="当前没有后端画像字段。" items={backendItems} />
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="local">
        <div className="mirror-page__profile-local-note">
          <BrainCircuit className="mirror-page__profile-icon" />
          <p className="mirror-page__summary-copy">这里的条目只按最近 100 条本地对话机械统计，用于展示近期使用情况，并与后端画像字段分层展示。</p>
        </div>

        <MirrorProfileGrid badgeTone="processing" emptyState="当前没有可展示的最近本地统计。" items={localItems} />
      </TabsContent>
    </Tabs>
  );
}

function MirrorMemoryDetail({
  overview,
  onUpdateSettings,
  settingsSnapshot,
  rpcContext,
  conversations,
  focusMemoryId,
  onOpenTaskDetail,
}: Pick<MirrorDetailContentProps, "overview" | "onUpdateSettings" | "settingsSnapshot" | "rpcContext" | "conversations" | "focusMemoryId"> & {
  onOpenTaskDetail: (taskId: string) => void;
}) {
  const conversationSummary = buildMirrorConversationSummary(conversations);
  const memorySettings = settingsSnapshot.settings.memory;
  const [settingsActionKey, setSettingsActionKey] = useState<string | null>(null);
  const [settingsFeedback, setSettingsFeedback] = useState<string | null>(null);
  // Current mirror references do not carry task identifiers, so local conversation
  // history is the only honest source for task back-links inside this detail view.
  const recentTaskLinkedConversations = useMemo(() => {
    const seenTaskIds = new Set<string>();

    return conversations.filter((record) => {
      if (!record.task_id || seenTaskIds.has(record.task_id)) {
        return false;
      }

      seenTaskIds.add(record.task_id);
      return true;
    });
  }, [conversations]);
  const highlightedMemoryId = useMemo(() => {
    if (focusMemoryId && overview.memory_references.some((reference) => reference.memory_id === focusMemoryId)) {
      return focusMemoryId;
    }

    return overview.memory_references[0]?.memory_id ?? null;
  }, [focusMemoryId, overview.memory_references]);
  const defaultTab = overview.memory_references.length > 0 ? "references" : "policy";
  const runSettingsUpdate = useCallback(
    async (actionKey: string, subject: string, patch: DashboardSettingsPatch) => {
      // Only stable settings keys are written through here.
      setSettingsActionKey(actionKey);

      try {
        const nextFeedback = await onUpdateSettings(subject, patch);
        setSettingsFeedback(nextFeedback);
      } catch (error) {
        setSettingsFeedback(error instanceof Error ? error.message : "镜子设置更新失败。");
      } finally {
        setSettingsActionKey(null);
      }
    },
    [onUpdateSettings],
  );

  return (
    <Tabs className="mirror-page__detail-tabs" defaultValue={defaultTab}>
      <TabsList className="mirror-page__detail-tab-list" variant="line">
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="references">
          记忆引用
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="context">
          数据上下文
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="policy">
          记忆策略
        </TabsTrigger>
      </TabsList>

      <TabsContent className="mirror-page__detail-tab-panel" value="references">
        {overview.memory_references.length === 0 ? (
          <MirrorEmptyState>暂无近期记忆引用。</MirrorEmptyState>
        ) : (
          <div className="mirror-page__memory-list mirror-page__memory-list--expanded">
            <div className="mirror-page__profile-local-note">
              <BookMarked className="mirror-page__memory-icon" />
              <p className="mirror-page__summary-copy">当前协议只返回 `memory_id / reason / summary`，还没有时间、来源 task 或命中场景明细，所以这里按后端真源直出，不伪造额外来源字段。</p>
            </div>

            {overview.memory_references.map((reference, index) => (
              <article key={reference.memory_id} className={`mirror-page__memory-card${reference.memory_id === highlightedMemoryId ? " is-active" : ""}`}>
                <div className="mirror-page__memory-header">
                  <div className="mirror-page__memory-meta">
                    <p className="mirror-page__memory-index">记录 {index + 1}</p>
                    <div className="mirror-page__memory-title-row">
                      <BookMarked className="mirror-page__memory-icon" />
                      <h3 className="mirror-page__memory-title">{reference.memory_id}</h3>
                    </div>
                  </div>
                  <StatusBadge tone={reference.memory_id === highlightedMemoryId ? "green" : "processing"}>
                    {reference.memory_id === highlightedMemoryId ? "当前任务引用" : "引用记录"}
                  </StatusBadge>
                </div>

                <p className="mirror-page__memory-reason">{reference.reason}</p>
                <div className="mirror-page__memory-summary">{reference.summary || reference.reason}</div>
              </article>
            ))}
          </div>
        )}
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="context">
        <div className="mirror-page__risk-list">
          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">本地连续对话</p>
                <p className="mirror-page__stage-headline">{conversationSummary.total_records} 条</p>
              </div>
              <StatusBadge tone="processing">local</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">最近 100 条本地输入与前端可见回应会保存在本地，仅用于本地记录和统计展示。</p>
          </article>

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">最近后端记忆引用</p>
                <p className="mirror-page__stage-headline">{overview.memory_references[0]?.memory_id ?? "暂无"}</p>
              </div>
              <StatusBadge tone="green">backend</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">{overview.memory_references[0]?.reason ?? "当前还没有新的记忆命中说明。"}</p>
          </article>

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">RPC 同步状态</p>
                <p className="mirror-page__stage-headline">{rpcContext.serverTime ? "已同步" : "本地视图"}</p>
              </div>
              <StatusBadge tone={rpcContext.warnings.length > 0 ? "yellow" : "processing"}>{rpcContext.warnings.length > 0 ? "带提醒" : "稳定"}</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">{rpcContext.warnings.length > 0 ? rpcContext.warnings.join("；") : "当前没有额外 RPC warnings。"}</p>
          </article>

          {recentTaskLinkedConversations.length > 0 ? (
            <article className="mirror-page__risk-card">
              <div className="mirror-page__stage-card-top">
                <div>
                  <p className="mirror-page__micro-label">近期可回跳任务</p>
                  <p className="mirror-page__stage-headline">{recentTaskLinkedConversations.length} 条 task 入口</p>
                </div>
                <StatusBadge tone="processing">task</StatusBadge>
              </div>
              <p className="mirror-page__summary-copy">这些 task 来自本地连续记录，可用于回跳任务详情；它们不代表后端记忆引用的正式来源字段。</p>
              <div className="mirror-page__conversation-actions">
                {recentTaskLinkedConversations.map((record) => (
                  <button key={record.task_id} type="button" className="mirror-page__task-link" onClick={() => onOpenTaskDetail(record.task_id!)}>
                    {record.task_id}
                  </button>
                ))}
              </div>
            </article>
          ) : null}
        </div>
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="policy">
        <div className="mirror-page__profile-local-note">
          <BookMarked className="mirror-page__memory-icon" />
          <p className="mirror-page__summary-copy">
            这里展示 `agent.settings.get` 或本地设置回退快照中的镜子记忆策略；已登记到真源的开关会直接写回 `agent.settings.update`。
          </p>
        </div>
        {settingsFeedback ? <div className="mirror-page__profile-local-note mirror-page__settings-feedback">{settingsFeedback}</div> : null}

        <div className="mirror-page__risk-list">
          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">记忆开关</p>
                <p className="mirror-page__stage-headline">{memorySettings.enabled ? "已开启" : "已关闭"}</p>
              </div>
              <StatusBadge tone={memorySettings.enabled ? "green" : "yellow"}>{memorySettings.enabled ? "enabled" : "disabled"}</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">当前卡片只负责说明 `settings.memory.enabled` 的状态，用来解释镜子是否应继续沉淀长期记忆。</p>
            <div className="mirror-page__settings-controls">
              <Switch
                checked={memorySettings.enabled}
                disabled={settingsActionKey !== null}
                onCheckedChange={(checked) => {
                  void runSettingsUpdate("memory-enabled", "记忆开关", {
                    memory: {
                      enabled: checked,
                    },
                  });
                }}
              />
              <span className="mirror-page__summary-copy">{settingsActionKey === "memory-enabled" ? "正在写入 settings.update…" : "直接写入记忆启停设置。"}</span>
            </div>
          </article>

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">保留周期</p>
                <p className="mirror-page__stage-headline">{formatDashboardMemoryLifecycle(memorySettings.lifecycle)}</p>
              </div>
              <StatusBadge tone="processing">lifecycle</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">这里直接读取 `settings.memory.lifecycle`，用于说明镜子记忆当前按什么周期保留。</p>
            <div className="mirror-page__settings-controls">
              <SegmentedControl.Root
                className="mirror-page__settings-segmented"
                value={memorySettings.lifecycle}
                onValueChange={(value) => {
                  if (!value || value === memorySettings.lifecycle) {
                    return;
                  }

                  void runSettingsUpdate("memory-lifecycle", "记忆生命周期", {
                    memory: {
                      lifecycle: value,
                    },
                  });
                }}
                disabled={settingsActionKey !== null}
              >
                <SegmentedControl.Item value="session">仅本轮</SegmentedControl.Item>
                <SegmentedControl.Item value="7d">7 天</SegmentedControl.Item>
                <SegmentedControl.Item value="30d">30 天</SegmentedControl.Item>
                <SegmentedControl.Item value="long_term">长期</SegmentedControl.Item>
              </SegmentedControl.Root>
            </div>
          </article>

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">工作总结刷新</p>
                <p className="mirror-page__stage-headline">{formatDashboardTimeInterval(memorySettings.work_summary_interval)}</p>
              </div>
              <StatusBadge tone="processing">summary</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">这里展示 `settings.memory.work_summary_interval`，用于解释工作总结类镜像的刷新节奏。</p>
          </article>

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">画像刷新</p>
                <p className="mirror-page__stage-headline">{formatDashboardTimeInterval(memorySettings.profile_refresh_interval)}</p>
              </div>
              <StatusBadge tone="processing">profile</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">这里展示 `settings.memory.profile_refresh_interval`，用于解释画像字段多久刷新一次。</p>
          </article>

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">设置来源</p>
                <p className="mirror-page__stage-headline">{settingsSnapshot.source === "rpc" ? "settings.get" : "local fallback"}</p>
              </div>
              <StatusBadge tone={settingsSnapshot.source === "rpc" ? "green" : "yellow"}>{settingsSnapshot.source}</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">
              {settingsSnapshot.rpcContext.serverTime
                ? `服务端快照时间：${settingsSnapshot.rpcContext.serverTime}`
                : "当前展示的是本地设置回退快照。"}
            </p>
            {settingsSnapshot.rpcContext.warnings.length > 0 ? (
              <div className="mirror-page__conversation-actions">
                {settingsSnapshot.rpcContext.warnings.map((warning) => (
                  <span key={warning} className="mirror-page__task-link">
                    {warning}
                  </span>
                ))}
              </div>
            ) : null}
          </article>
        </div>
      </TabsContent>
    </Tabs>
  );
}

export function MirrorDetailContent(props: MirrorDetailContentProps) {
  const navigate = useNavigate();
  const openTaskDetail = useMemo(
    () => (taskId: string) => {
      navigateToDashboardTaskDetail(navigate, taskId);
    },
    [navigate],
  );
  const openSafetyRestorePoint = useMemo(
    () => (restorePoint: RecoveryPoint) => {
      navigate(resolveDashboardModuleRoutePath("safety"), {
        state: buildDashboardSafetyRestorePointNavigationState(restorePoint),
      });
    },
    [navigate],
  );
  const openSafetyCardDetail = useMemo(
    () => (detailKey: "status" | "budget") => {
      navigate(resolveDashboardModuleRoutePath("safety"), {
        state: buildDashboardSafetyCardNavigationState(detailKey),
      });
    },
    [navigate],
  );
  const openHistoryDetail = useMemo(
    () => () => {
      navigate(resolveDashboardModuleRoutePath("memory"), {
        state: {
          activeDetailKey: "history",
          historyDetailView: "conversation",
        },
      });
    },
    [navigate],
  );

  if (props.activeDetailKey === "history") {
    return (
      <MirrorHistoryDetail
        conversationSummary={props.conversationSummary}
        conversations={props.conversations}
        historyDetailView={props.historyDetailView}
        onOpenTaskDetail={openTaskDetail}
        overview={props.overview}
      />
    );
  }

  if (props.activeDetailKey === "dailyStage") {
    return (
      <MirrorDailyDetail
        dailyDigest={props.dailyDigest}
        latestRestorePoint={props.latestRestorePoint}
        onOpenHistoryDetail={openHistoryDetail}
        onOpenRestorePoint={openSafetyRestorePoint}
        onOpenSafetyCardDetail={openSafetyCardDetail}
        onOpenTaskDetail={openTaskDetail}
      />
    );
  }

  if (props.activeDetailKey === "profile") {
    return <MirrorProfileDetail backendItems={props.profileView.backend_items} localItems={props.profileView.local_stat_items} />;
  }

  return (
    <MirrorMemoryDetail
      conversations={props.conversations}
      focusMemoryId={props.focusMemoryId}
      onOpenTaskDetail={openTaskDetail}
      onUpdateSettings={props.onUpdateSettings}
      overview={props.overview}
      rpcContext={props.rpcContext}
      settingsSnapshot={props.settingsSnapshot}
    />
  );
}
