import { useCallback, useEffect, useMemo, useState } from "react";
import type { AgentMirrorOverviewGetResult, RecoveryPoint } from "@cialloclaw/protocol";
import { BookMarked, BrainCircuit, CalendarDays } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { StatusBadge } from "@cialloclaw/ui";
import { Button, SegmentedControl, Switch } from "@radix-ui/themes";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { buildDashboardSafetyRestorePointNavigationState } from "@/features/dashboard/shared/dashboardSafetyNavigation";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import type { DashboardSettingsPatch } from "@/features/dashboard/shared/dashboardSettingsMutation";
import {
  formatDashboardMemoryLifecycle,
  formatDashboardTimeInterval,
  type DashboardSettingsSnapshotData,
} from "@/features/dashboard/shared/dashboardSettingsSnapshot";
import {
  buildMirrorConversationSummary,
  buildMirrorConversationTaskOptions,
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
import {
  applyMirrorMemoryDrafts,
  applyMirrorProfileDrafts,
  clearMirrorProfileDraft,
  loadMirrorGovernanceDraftSnapshot,
  saveMirrorGovernanceDraftSnapshot,
  setMirrorMemoryHidden,
  upsertMirrorProfileDraft,
  type MirrorGovernanceDraftSnapshot,
  type MirrorGovernedMemoryReference,
  type MirrorGovernedProfileItem,
} from "./mirrorGovernanceDraftService";

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
  profileView: MirrorProfileView;
};

function MirrorEmptyState({ children }: { children: string }) {
  return <p className="mirror-page__empty-state">{children}</p>;
}

function MirrorHistoryDetail({
  overview,
  conversations,
  conversationSummary,
  onOpenTaskDetail,
}: Pick<MirrorDetailContentProps, "overview" | "conversations" | "conversationSummary"> & {
  onOpenTaskDetail: (taskId: string) => void;
}) {
  const [conversationScopeFilter, setConversationScopeFilter] = useState<MirrorConversationScopeFilter>("all");
  const [conversationSourceFilter, setConversationSourceFilter] = useState<MirrorConversationRecord["source"] | "all">("all");
  const [conversationInputModeFilter, setConversationInputModeFilter] = useState<MirrorConversationInputModeFilter>("all");
  const [conversationTaskFilter, setConversationTaskFilter] = useState<string | "all">("all");
  const conversationFilters = useMemo(
    () =>
      ({
        scope: conversationScopeFilter,
        source: conversationSourceFilter,
        input_mode: conversationInputModeFilter,
        task_id: conversationTaskFilter,
      } satisfies MirrorConversationFilters),
    [conversationInputModeFilter, conversationScopeFilter, conversationSourceFilter, conversationTaskFilter],
  );
  const conversationTaskOptions = useMemo(() => buildMirrorConversationTaskOptions(conversations), [conversations]);
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
    if (conversationTaskFilter !== "all" && !conversationTaskOptions.some((option) => option.task_id === conversationTaskFilter)) {
      setConversationTaskFilter("all");
    }
  }, [conversationTaskFilter, conversationTaskOptions]);

  return (
    <Tabs className="mirror-page__detail-tabs" defaultValue={conversations.length > 0 ? "conversation" : "summary"}>
      <TabsList className="mirror-page__detail-tab-list" variant="line" data-testid="mirror-history-tabs">
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="summary">
          历史概要
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="conversation">
          最近 100 条本地对话
        </TabsTrigger>
      </TabsList>

      <TabsContent className="mirror-page__detail-tab-panel" value="summary">
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
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="conversation">
        <div className="mirror-page__conversation-filter-shell">
          <div className="mirror-page__profile-local-note">
            <BrainCircuit className="mirror-page__profile-icon" />
            <p className="mirror-page__summary-copy">这一栏只统计最近 100 条本地输入与前端可见回应，用来按时间和 task 回看，不等于后端历史概要真源。</p>
          </div>

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

          {conversationTaskOptions.length > 0 ? (
            <div className="mirror-page__conversation-filter-bar">
              <button
                type="button"
                className={`mirror-page__conversation-filter${conversationTaskFilter === "all" ? " is-active" : ""}`}
                onClick={() => setConversationTaskFilter("all")}
              >
                全部 task
              </button>
              {conversationTaskOptions.map((option) => (
                <button
                  key={option.task_id}
                  type="button"
                  className={`mirror-page__conversation-filter${conversationTaskFilter === option.task_id ? " is-active" : ""}`}
                  onClick={() => setConversationTaskFilter(option.task_id)}
                >
                  {option.task_id} · {option.count}
                </button>
              ))}
            </div>
          ) : null}
        </div>

        {groupedConversations.length === 0 ? (
          <MirrorEmptyState>{conversationScopeFilter === "all" ? "最近 100 条本地对话还没有记录。" : "当前筛选条件下没有命中的本地记录。"}</MirrorEmptyState>
        ) : (
          <ScrollArea className="mirror-page__conversation-scroll" data-testid="mirror-conversation-list">
            <div className="mirror-page__conversation-days">
              {groupedConversations.map((group) => (
                <section key={group.date_key} className="mirror-page__conversation-day">
                  <div className="mirror-page__conversation-day-header">
                    <p className="mirror-page__micro-label">{group.label}</p>
                    <StatusBadge tone="processing">{group.items.length} 条</StatusBadge>
                  </div>

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
              ))}
            </div>
          </ScrollArea>
        )}
      </TabsContent>
    </Tabs>
  );
}

function MirrorDailyDetail({
  dailyDigest,
  latestRestorePoint,
  onOpenRestorePoint,
  onOpenTaskDetail,
}: Pick<MirrorDetailContentProps, "dailyDigest" | "latestRestorePoint"> & {
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
              {note.id === "restore-point" && latestRestorePoint ? (
                <div className="mirror-page__conversation-actions">
                  <button type="button" className="mirror-page__task-link" onClick={() => onOpenRestorePoint(latestRestorePoint)}>
                    前往恢复点
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

function getMirrorProfileDraftLabel(mode: MirrorGovernedProfileItem["draft_mode"]) {
  if (mode === "edited") {
    return "本地纠正";
  }

  if (mode === "hidden") {
    return "本地隐藏";
  }

  if (mode === "deleted") {
    return "本地删除";
  }

  return null;
}

type MirrorProfileGovernanceHandlers = {
  editingItemId: string | null;
  editingValue: string;
  onStartEdit: (item: MirrorGovernedProfileItem) => void;
  onEditValueChange: (value: string) => void;
  onSaveEdit: (item: MirrorGovernedProfileItem) => void;
  onCancelEdit: () => void;
  onHide: (item: MirrorGovernedProfileItem) => void;
  onDelete: (item: MirrorGovernedProfileItem) => void;
  onRestore: (item: MirrorGovernedProfileItem) => void;
};

function MirrorGovernedProfileGrid({
  items,
  emptyState,
  badgeTone,
  governance,
}: {
  items: MirrorGovernedProfileItem[];
  emptyState: string;
  badgeTone: "green" | "processing";
  governance: MirrorProfileGovernanceHandlers;
}) {
  if (items.length === 0) {
    return <MirrorEmptyState>{emptyState}</MirrorEmptyState>;
  }

  return (
    <div className="mirror-page__profile-grid">
      {items.map((item) => {
        const draftLabel = getMirrorProfileDraftLabel(item.draft_mode);
        const isEditing = governance.editingItemId === item.id;
        const isArchived = item.draft_mode === "hidden" || item.draft_mode === "deleted";

        return (
          <article
            key={item.id}
            className={`mirror-page__profile-item${isArchived ? " mirror-page__profile-item--archived" : ""}`}
            data-testid={`mirror-profile-item-${item.id}`}
          >
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">{item.label}</p>
                <p className="mirror-page__stage-headline">{item.display_value}</p>
              </div>
              <div className="mirror-page__profile-badges">
                <StatusBadge tone={badgeTone}>{item.source_label}</StatusBadge>
                {draftLabel ? <StatusBadge tone={isArchived ? "yellow" : "processing"}>{draftLabel}</StatusBadge> : null}
              </div>
            </div>
            <p className="mirror-page__summary-copy">
              {item.hint}
              {item.display_value !== item.original_value ? ` 当前原值：${item.original_value}。` : ""}
              {item.draft_updated_at
                ? ` 草稿更新时间 ${new Date(item.draft_updated_at).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })}。`
                : ""}
            </p>

            {isEditing ? (
              <div className="mirror-page__profile-edit-shell">
                <textarea
                  className="mirror-page__profile-edit-input"
                  value={governance.editingValue}
                  onChange={(event) => governance.onEditValueChange(event.currentTarget.value)}
                />
                <div className="mirror-page__profile-actions">
                  <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => governance.onCancelEdit()}>
                    取消
                  </Button>
                  <Button className="mirror-page__action-button" variant="soft" color="green" onClick={() => governance.onSaveEdit(item)}>
                    保存本地草稿
                  </Button>
                </div>
              </div>
            ) : (
              <div className="mirror-page__profile-actions">
                {item.draft_mode === "hidden" || item.draft_mode === "deleted" ? (
                  <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => governance.onRestore(item)}>
                    恢复显示
                  </Button>
                ) : (
                  <>
                    <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => governance.onStartEdit(item)}>
                      纠正
                    </Button>
                    <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => governance.onHide(item)}>
                      隐藏
                    </Button>
                    <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => governance.onDelete(item)}>
                      删除
                    </Button>
                    {item.draft_mode === "edited" ? (
                      <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => governance.onRestore(item)}>
                        撤回草稿
                      </Button>
                    ) : null}
                  </>
                )}
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
}

function MirrorProfileDetail({
  backendItems,
  localItems,
  draftedItems,
  governance,
}: {
  backendItems: MirrorGovernedProfileItem[];
  localItems: MirrorGovernedProfileItem[];
  draftedItems: MirrorGovernedProfileItem[];
  governance: MirrorProfileGovernanceHandlers;
}) {
  const defaultTab = backendItems.length > 0 ? "backend" : localItems.length > 0 ? "local" : "drafts";

  return (
    <Tabs className="mirror-page__detail-tabs" defaultValue={defaultTab}>
      <TabsList className="mirror-page__detail-tab-list" variant="line">
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="backend">
          后端画像字段
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="local">
          最近本地统计
        </TabsTrigger>
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="drafts">
          本地治理草稿
        </TabsTrigger>
      </TabsList>

      <TabsContent className="mirror-page__detail-tab-panel" value="backend">
        <div className="mirror-page__profile-local-note">
          <BrainCircuit className="mirror-page__profile-icon" />
          <p className="mirror-page__summary-copy">这里仍以后端 mirror overview 返回的 profile 字段为底；下方“纠正 / 隐藏 / 删除”只会生成本地治理草稿，不会伪装成正式后端写入。</p>
        </div>

        <MirrorGovernedProfileGrid badgeTone="green" emptyState="当前没有后端画像字段。" items={backendItems} governance={governance} />
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="local">
        <div className="mirror-page__profile-local-note">
          <BrainCircuit className="mirror-page__profile-icon" />
          <p className="mirror-page__summary-copy">这里的条目只按最近 100 条本地对话机械统计，用于展示近期使用情况；本地治理草稿同样只影响当前前端视图。</p>
        </div>

        <MirrorGovernedProfileGrid badgeTone="processing" emptyState="当前没有可展示的最近本地统计。" items={localItems} governance={governance} />
      </TabsContent>

      <TabsContent className="mirror-page__detail-tab-panel" value="drafts">
        <div className="mirror-page__profile-local-note">
          <BrainCircuit className="mirror-page__profile-icon" />
          <p className="mirror-page__summary-copy">这里汇总前端本地治理草稿，用来提前承接 issue #187 里 planned 的 `agent.mirror.memory.manage` 交互位；当前不会回写后端。</p>
        </div>

        {draftedItems.length === 0 ? (
          <MirrorEmptyState>当前还没有画像治理草稿。</MirrorEmptyState>
        ) : (
          <ScrollArea className="mirror-page__profile-archive-scroll">
            <div className="mirror-page__profile-archive-list">
              <MirrorGovernedProfileGrid badgeTone="processing" emptyState="当前还没有画像治理草稿。" items={draftedItems} governance={governance} />
            </div>
          </ScrollArea>
        )}
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
  visibleReferences,
  hiddenReferences,
  onHideReference,
  onRestoreReference,
  onOpenTaskDetail,
}: Pick<MirrorDetailContentProps, "overview" | "onUpdateSettings" | "settingsSnapshot" | "rpcContext" | "conversations" | "focusMemoryId"> & {
  visibleReferences: MirrorGovernedMemoryReference[];
  hiddenReferences: MirrorGovernedMemoryReference[];
  onHideReference: (memoryId: string) => void;
  onRestoreReference: (memoryId: string) => void;
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

    return visibleReferences[0]?.memory_id ?? hiddenReferences[0]?.memory_id ?? null;
  }, [focusMemoryId, hiddenReferences, overview.memory_references, visibleReferences]);
  const defaultTab = visibleReferences.length > 0 ? "references" : "policy";
  const runSettingsUpdate = useCallback(
    async (actionKey: string, subject: string, patch: DashboardSettingsPatch) => {
      // Only stable settings keys are written through here; planned memory
      // governance actions still stay in local drafts until the RPC is formalized.
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
        <TabsTrigger className="mirror-page__detail-tab-trigger" value="drafts">
          本地治理草稿
        </TabsTrigger>
      </TabsList>

      <TabsContent className="mirror-page__detail-tab-panel" value="references">
        {visibleReferences.length === 0 ? (
          <MirrorEmptyState>暂无近期记忆引用。</MirrorEmptyState>
        ) : (
          <div className="mirror-page__memory-list mirror-page__memory-list--expanded">
            <div className="mirror-page__profile-local-note">
              <BookMarked className="mirror-page__memory-icon" />
              <p className="mirror-page__summary-copy">当前协议只返回 `memory_id / reason / summary`，还没有时间、来源 task 或命中场景明细，所以这里不会伪造引用来源；“本地隐藏”只影响前端视图。</p>
            </div>

            {visibleReferences.map((reference, index) => (
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
                <div className="mirror-page__memory-summary">{reference.summary}</div>
                <div className="mirror-page__profile-actions">
                  <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => onHideReference(reference.memory_id)}>
                    本地隐藏
                  </Button>
                </div>
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

          <article className="mirror-page__risk-card">
            <div className="mirror-page__stage-card-top">
              <div>
                <p className="mirror-page__micro-label">本地治理草稿</p>
                <p className="mirror-page__stage-headline">{hiddenReferences.length} 条隐藏引用</p>
              </div>
              <StatusBadge tone={hiddenReferences.length > 0 ? "yellow" : "processing"}>{hiddenReferences.length > 0 ? "已生效" : "空白"}</StatusBadge>
            </div>
            <p className="mirror-page__summary-copy">这些草稿只影响镜子详情视图，用来提前承接 planned 的记忆治理动作，不会改写后端真实记忆。</p>
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
            这里展示 `agent.settings.get` 或本地设置回退快照中的镜子记忆策略；已登记到真源的开关会直接写回 `agent.settings.update`，planned 的记忆治理动作仍只保留本地草稿。
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

      <TabsContent className="mirror-page__detail-tab-panel" value="drafts">
        <div className="mirror-page__profile-local-note">
          <BookMarked className="mirror-page__memory-icon" />
          <p className="mirror-page__summary-copy">这里汇总本地隐藏的记忆引用。它们来自前端治理草稿，不是后端已删除或已关闭的正式记忆状态。</p>
        </div>

        {hiddenReferences.length === 0 ? (
          <MirrorEmptyState>当前还没有记忆治理草稿。</MirrorEmptyState>
        ) : (
          <div className="mirror-page__memory-list mirror-page__memory-list--expanded">
            {hiddenReferences.map((reference, index) => (
              <article key={reference.memory_id} className={`mirror-page__memory-card mirror-page__profile-item--archived${reference.memory_id === highlightedMemoryId ? " is-active" : ""}`}>
                <div className="mirror-page__memory-header">
                  <div className="mirror-page__memory-meta">
                    <p className="mirror-page__memory-index">草稿 {index + 1}</p>
                    <div className="mirror-page__memory-title-row">
                      <BookMarked className="mirror-page__memory-icon" />
                      <h3 className="mirror-page__memory-title">{reference.memory_id}</h3>
                    </div>
                  </div>
                  <StatusBadge tone="yellow">本地隐藏</StatusBadge>
                </div>

                <p className="mirror-page__memory-reason">{reference.reason}</p>
                <div className="mirror-page__memory-summary">{reference.summary}</div>
                <div className="mirror-page__profile-actions">
                  <Button className="mirror-page__action-button" variant="soft" color="gray" onClick={() => onRestoreReference(reference.memory_id)}>
                    恢复显示
                  </Button>
                </div>
              </article>
            ))}
          </div>
        )}
      </TabsContent>
    </Tabs>
  );
}

export function MirrorDetailContent(props: MirrorDetailContentProps) {
  const navigate = useNavigate();
  const [governanceDrafts, setGovernanceDrafts] = useState<MirrorGovernanceDraftSnapshot>(() => loadMirrorGovernanceDraftSnapshot());
  const [editingProfileItemId, setEditingProfileItemId] = useState<string | null>(null);
  const [editingProfileValue, setEditingProfileValue] = useState("");
  const openTaskDetail = useMemo(
    () => (taskId: string) => {
      navigate(resolveDashboardModuleRoutePath("tasks"), {
        state: {
          focusTaskId: taskId,
          openDetail: true,
        },
      });
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
  const updateGovernanceDrafts = useMemo(
    () => (updater: (current: MirrorGovernanceDraftSnapshot) => MirrorGovernanceDraftSnapshot) => {
      // Governance drafts intentionally stay local until the planned mirror
      // management RPC becomes stable, so every mutation is persisted locally only.
      setGovernanceDrafts((current) => {
        const nextSnapshot = updater(current);
        saveMirrorGovernanceDraftSnapshot(nextSnapshot);
        return nextSnapshot;
      });
    },
    [],
  );
  const governedBackendProfiles = useMemo(
    () => applyMirrorProfileDrafts(props.profileView.backend_items, governanceDrafts),
    [governanceDrafts, props.profileView.backend_items],
  );
  const governedLocalProfiles = useMemo(
    () => applyMirrorProfileDrafts(props.profileView.local_stat_items, governanceDrafts),
    [governanceDrafts, props.profileView.local_stat_items],
  );
  const draftedProfileItems = useMemo(
    () =>
      [...governedBackendProfiles.drafted_items, ...governedLocalProfiles.drafted_items].sort((left, right) =>
        (right.draft_updated_at ?? "").localeCompare(left.draft_updated_at ?? ""),
      ),
    [governedBackendProfiles.drafted_items, governedLocalProfiles.drafted_items],
  );
  const governedMemoryReferences = useMemo(
    () => applyMirrorMemoryDrafts(props.overview.memory_references, governanceDrafts),
    [governanceDrafts, props.overview.memory_references],
  );
  const startProfileEdit = useMemo(
    () => (item: MirrorGovernedProfileItem) => {
      setEditingProfileItemId(item.id);
      setEditingProfileValue(item.display_value);
    },
    [],
  );
  const cancelProfileEdit = useMemo(
    () => () => {
      setEditingProfileItemId(null);
      setEditingProfileValue("");
    },
    [],
  );
  const saveProfileEdit = useMemo(
    () => (item: MirrorGovernedProfileItem) => {
      updateGovernanceDrafts((current) => upsertMirrorProfileDraft(current, item.id, "edited", editingProfileValue));
      setEditingProfileItemId(null);
      setEditingProfileValue("");
    },
    [editingProfileValue, updateGovernanceDrafts],
  );
  const hideProfileItem = useMemo(
    () => (item: MirrorGovernedProfileItem) => {
      updateGovernanceDrafts((current) => upsertMirrorProfileDraft(current, item.id, "hidden"));
      if (editingProfileItemId === item.id) {
        setEditingProfileItemId(null);
        setEditingProfileValue("");
      }
    },
    [editingProfileItemId, updateGovernanceDrafts],
  );
  const deleteProfileItem = useMemo(
    () => (item: MirrorGovernedProfileItem) => {
      updateGovernanceDrafts((current) => upsertMirrorProfileDraft(current, item.id, "deleted"));
      if (editingProfileItemId === item.id) {
        setEditingProfileItemId(null);
        setEditingProfileValue("");
      }
    },
    [editingProfileItemId, updateGovernanceDrafts],
  );
  const restoreProfileItem = useMemo(
    () => (item: MirrorGovernedProfileItem) => {
      updateGovernanceDrafts((current) => clearMirrorProfileDraft(current, item.id));
      if (editingProfileItemId === item.id) {
        setEditingProfileItemId(null);
        setEditingProfileValue("");
      }
    },
    [editingProfileItemId, updateGovernanceDrafts],
  );
  const hideMemoryReference = useMemo(
    () => (memoryId: string) => {
      updateGovernanceDrafts((current) => setMirrorMemoryHidden(current, memoryId, true));
    },
    [updateGovernanceDrafts],
  );
  const restoreMemoryReference = useMemo(
    () => (memoryId: string) => {
      updateGovernanceDrafts((current) => setMirrorMemoryHidden(current, memoryId, false));
    },
    [updateGovernanceDrafts],
  );

  if (props.activeDetailKey === "history") {
    return <MirrorHistoryDetail conversationSummary={props.conversationSummary} conversations={props.conversations} onOpenTaskDetail={openTaskDetail} overview={props.overview} />;
  }

  if (props.activeDetailKey === "dailyStage") {
    return <MirrorDailyDetail dailyDigest={props.dailyDigest} latestRestorePoint={props.latestRestorePoint} onOpenRestorePoint={openSafetyRestorePoint} onOpenTaskDetail={openTaskDetail} />;
  }

  if (props.activeDetailKey === "profile") {
    return (
      <MirrorProfileDetail
        backendItems={governedBackendProfiles.active_items}
        localItems={governedLocalProfiles.active_items}
        draftedItems={draftedProfileItems}
        governance={{
          editingItemId: editingProfileItemId,
          editingValue: editingProfileValue,
          onStartEdit: startProfileEdit,
          onEditValueChange: setEditingProfileValue,
          onSaveEdit: saveProfileEdit,
          onCancelEdit: cancelProfileEdit,
          onHide: hideProfileItem,
          onDelete: deleteProfileItem,
          onRestore: restoreProfileItem,
        }}
      />
    );
  }

  return (
    <MirrorMemoryDetail
      conversations={props.conversations}
      focusMemoryId={props.focusMemoryId}
      hiddenReferences={governedMemoryReferences.hidden_references}
      onHideReference={hideMemoryReference}
      onOpenTaskDetail={openTaskDetail}
      onUpdateSettings={props.onUpdateSettings}
      onRestoreReference={restoreMemoryReference}
      overview={props.overview}
      rpcContext={props.rpcContext}
      settingsSnapshot={props.settingsSnapshot}
      visibleReferences={governedMemoryReferences.visible_references}
    />
  );
}
