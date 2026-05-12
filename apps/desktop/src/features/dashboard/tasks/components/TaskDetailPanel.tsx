import { useEffect, useState } from "react";
import { AlertTriangle, ArrowUpRight, Clock3, FolderOutput, RefreshCcw, SendHorizonal, ShieldAlert, X } from "lucide-react";
import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { canTaskAcceptSteering, formatTaskSourceLabel, getTaskPreviewStatusLabel, getTaskProgress, getTaskStateVoice, getTaskStatusBadgeClass, isTaskEnded } from "../taskPage.mapper";
import type { Task } from "@cialloclaw/protocol";
import type { TaskDetailData, TaskExperience, TaskPrimaryAction } from "../taskPage.types";
import { TaskActionBar } from "./TaskActionBar";
import { TaskContextBlock } from "./TaskContextBlock";
import { TaskProgressTimeline } from "./TaskProgressTimeline";
import type { TaskEventFilters, TaskEventTimeRange } from "../taskPage.types";
import { DEFAULT_TASK_EVENT_FILTERS } from "../taskPage.service";
import { canOpenTaskDeliveryResult, getTaskDeliveryOpenLabel } from "../taskOutput.service";

type TaskDetailPanelProps = {
  artifactActionPendingId: string | null;
  artifactErrorMessage: string | null;
  artifactItems: TaskDetailData["detail"]["artifacts"];
  artifactLoading: boolean;
  fallbackOutputAccess?: boolean;
  detailWarningMessage: string | null;
  detailData: TaskDetailData | null;
  detailErrorMessage: string | null;
  eventErrorMessage: string | null;
  eventFilters: TaskEventFilters;
  eventItems: import("../taskPage.types").TaskEventItem[];
  eventLoading: boolean;
  detailState: "loading" | "error" | "ready";
  deliveryActionPending: boolean;
  feedback: string | null;
  fallbackActions?: TaskPrimaryAction[] | null;
  onAction: (action: "pause" | "resume" | "cancel" | "restart" | "open-safety") => void;
  onClose: () => void;
  onOpenArtifact: (artifactId: string) => void;
  onOpenLatestDelivery: () => void;
  onApplyEventFilters: (filters: TaskEventFilters) => void;
  previewExperience: TaskExperience | null;
  previewTask: Task | null;
  onResetEventFilters: () => void;
  onRetryDetail: (() => void) | null;
  onSteerTask: (message: string) => void;
  steeringPending: boolean;
  steeringSuccessVersion: number;
};

/**
 * Shows the full task detail panel while keeping the formal delivery page as a
 * downstream handoff instead of repeating the same result summary in multiple
 * places.
 */
export function TaskDetailPanel({
  artifactActionPendingId,
  artifactErrorMessage,
  artifactItems,
  artifactLoading,
  fallbackOutputAccess = false,
  detailWarningMessage,
  detailData,
  detailErrorMessage,
  eventErrorMessage,
  eventFilters,
  eventItems,
  eventLoading,
  detailState,
  deliveryActionPending,
  feedback,
  fallbackActions = null,
  onAction,
  onClose,
  onOpenArtifact,
  onOpenLatestDelivery,
  onApplyEventFilters,
  previewExperience,
  previewTask,
  onResetEventFilters,
  onRetryDetail,
  onSteerTask,
  steeringPending,
  steeringSuccessVersion,
}: TaskDetailPanelProps) {
  const detail = detailData?.detail ?? null;
  const experience = detailData?.experience ?? previewExperience;
  const task = detailData?.task ?? previewTask;
  const [steeringMessage, setSteeringMessage] = useState("");
  const [eventFilterDraft, setEventFilterDraft] = useState(eventFilters);
  const progress = getTaskProgress(detail?.timeline ?? []);
  const stateVoice = task && experience
    ? getTaskStateVoice(task, experience, detail?.timeline ?? [])
    : {
        body: "正在从本地服务同步当前任务详情。",
        title: "正在打开任务详情",
      };
  const ended = task ? isTaskEnded(task) : false;
  const waitingCopy = task && experience
    ? task.status === "confirming_intent" || task.status === "waiting_auth" || task.status === "waiting_input" || task.status === "paused"
      ? experience.waitingReason
      : task.status === "failed" || task.status === "blocked"
        ? experience.blockedReason
        : null
    : null;
  const isDetailLoading = detailState === "loading";
  const isDetailError = detailState === "error";
  const progressLabel = progress.total > 0 ? `${progress.completedCount}/${progress.total}` : "无";
  const detailNoticeTitle = isDetailLoading ? "正在同步更多详情" : "详情同步失败";
  const detailNoticeBody = isDetailLoading
    ? "当前先展示基础任务信息，时间线、产出和安全摘要正在从本地服务拉取。"
    : `${detailErrorMessage ?? "任务详情请求失败"}。当前先展示基础任务信息，你可以稍后重试。`;
  const shouldDeferSecuritySummary = detailState !== "ready" || detail === null;
  const canSteerTask = task ? canTaskAcceptSteering(task) : false;
  const steeringHint = task?.status === "confirming_intent"
    ? "当前任务仍在等待确认处理方式；确认后才会开放追加要求。"
    : "这会把补充说明排入当前任务后续执行。";
  const steeringPlaceholder = canSteerTask
    ? "例如：保留现有结果，再额外补一份简短结论。"
    : task?.status === "confirming_intent"
      ? "当前任务还在等待确认处理方式，确认后才能继续追加要求。"
      : task && !ended
        ? "当前任务暂不接受追加要求。"
        : "当前任务已结束，不能继续补充要求。";
  const formalDeliveryResult = detail?.delivery_result ?? null;
  const runtimeSummary = detail?.runtime_summary ?? null;
  const evidenceItems = detail?.citations ?? [];
  // Evidence artifacts stay task-centric by following the formal citation links
  // instead of treating every task artifact as screen evidence.
  const evidenceArtifactRefs = new Set(evidenceItems.map((citation) => citation.source_ref));
  const evidenceArtifacts = artifactItems.filter((artifact) => evidenceArtifactRefs.has(artifact.artifact_id) || evidenceArtifactRefs.has(artifact.path));
  const outputArtifacts = artifactItems.filter((artifact) => !evidenceArtifactRefs.has(artifact.artifact_id) && !evidenceArtifactRefs.has(artifact.path));
  const formalDeliveryPath = formalDeliveryResult?.payload.path?.trim() ?? "";
  const formalDeliveryDuplicatesArtifact = Boolean(
    formalDeliveryResult
    && formalDeliveryPath.length > 0
    && (formalDeliveryResult.type === "workspace_document" || formalDeliveryResult.type === "open_file" || formalDeliveryResult.type === "reveal_in_folder")
    && outputArtifacts.some((artifact) => artifact.path.trim() === formalDeliveryPath),
  );
  const formalEvidenceCount = new Set(
    evidenceItems.map((citation) => {
      const sourceRef = citation.source_ref.trim();

      return sourceRef.length > 0 ? sourceRef : citation.citation_id;
    }),
  ).size;
  const isScreenTask = task?.source_type === "screen_capture" || detail?.task.intent?.name === "screen_analyze";
  const hasOutputArtifacts = outputArtifacts.length > 0;
  const hasEvidenceContent = evidenceItems.length > 0 || evidenceArtifacts.length > 0;
  const hasRuntimeSummarySignals = Boolean(
    runtimeSummary &&
    ((runtimeSummary.latest_event_type ?? "").trim().startsWith("loop.") ||
      (runtimeSummary.loop_stop_reason ?? "").trim() !== "" ||
      runtimeSummary.active_steering_count > 0),
  );
  const hasRuntimeProcessContent = hasRuntimeSummarySignals || eventItems.length > 0 || eventLoading || eventErrorMessage !== null;
  const hasFormalOutput = formalDeliveryResult !== null && !formalDeliveryDuplicatesArtifact;
  const hasOutputContent = hasFormalOutput || hasOutputArtifacts;
  const canOpenFormalOutput = canOpenTaskDeliveryResult(formalDeliveryResult, task?.task_id ?? null);
  const canOpenFallbackDelivery = canOpenTaskDeliveryResult(detailData?.detail.delivery_result ?? null, task?.task_id ?? null);
  const isInlineBubbleOutput = formalDeliveryResult?.type === "bubble";
  const shouldHideEndedResultCopy = ended && isInlineBubbleOutput && !hasOutputArtifacts;

  useEffect(() => {
    if (steeringPending || steeringSuccessVersion === 0) {
      return;
    }

    // Clear the local follow-up draft only after the parent mutation confirms a
    // successful steer, so backend wording changes cannot leave stale input behind.
    setSteeringMessage("");
  }, [steeringPending, steeringSuccessVersion]);

  useEffect(() => {
    // Keep runtime event filters as local draft state so typing does not trigger
    // one RPC refetch per keystroke before the user explicitly applies changes.
    setEventFilterDraft(eventFilters);
  }, [eventFilters]);

  if (!task) {
    return (
      <motion.section animate={{ opacity: 1, x: 0 }} className="task-detail-shell" initial={{ opacity: 0, x: 18 }} transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}>
        <div className="task-detail-shell__header">
          <div>
            <p className="task-detail-shell__eyebrow">任务详情</p>
            <h2 className="task-detail-shell__title">正在打开任务详情</h2>
            <p className="task-detail-shell__subtitle">正在等待任务详情返回。</p>
          </div>

          <div className="task-detail-shell__status-wrap">
            <Button className="task-detail-shell__close" onClick={onClose} size="icon-sm" variant="ghost">
              <X className="h-4 w-4" />
              <span className="sr-only">关闭任务详情</span>
            </Button>
          </div>
        </div>

        <ScrollArea className="task-detail-shell__scroll">
          <div className="task-detail-shell__body">
            <section className="task-detail-card task-detail-card--notice">
              <div className="task-detail-card__header task-detail-card__header--actionable">
                <div>
                  <p className="task-detail-card__eyebrow">详情状态</p>
                  <h3 className="task-detail-card__title">{detailNoticeTitle}</h3>
                </div>
                {isDetailError && onRetryDetail ? (
                  <button className="task-detail-card__action" onClick={onRetryDetail} type="button">
                    <RefreshCcw className="h-4 w-4" />
                    重试
                  </button>
                ) : null}
              </div>
              <p className="task-detail-ended-copy">{detailNoticeBody}</p>
            </section>
            {fallbackActions && fallbackActions.length > 0 ? <TaskActionBar actionsOverride={fallbackActions} detail={null} onAction={onAction} task={null} /> : null}
            {fallbackOutputAccess ? (
              <section className="task-detail-card">
                <div className="task-detail-card__header task-detail-card__header--actionable">
                  <div>
                    <p className="task-detail-card__eyebrow">产出内容</p>
                    <h3 className="task-detail-card__title">已生成的结果</h3>
                  </div>
                  {canOpenFallbackDelivery ? (
                    <button className="task-detail-card__action" disabled={deliveryActionPending} onClick={onOpenLatestDelivery} type="button">
                      <ArrowUpRight className="h-4 w-4" />
                      {deliveryActionPending ? "打开中..." : "打开结果"}
                    </button>
                  ) : null}
                </div>
                <div className="task-detail-output-list">
                  {artifactErrorMessage ? <p className="task-detail-card__hint">{artifactErrorMessage}</p> : null}
                  {artifactLoading && artifactItems.length === 0 ? <p className="task-detail-card__empty">正在同步成果列表...</p> : null}
                  {artifactItems.length > 0 ? (
                    artifactItems.map((artifact) => (
                      <article key={artifact.artifact_id} className="task-detail-output-item">
                        <FolderOutput className="h-4 w-4" />
                        <div>
                          <p className="task-detail-output-item__title">{artifact.title}</p>
                          <p className="task-detail-output-item__path">{artifact.path}</p>
                        </div>
                        <button
                          className="task-detail-card__action"
                          disabled={artifactActionPendingId === artifact.artifact_id}
                          onClick={() => onOpenArtifact(artifact.artifact_id)}
                          type="button"
                        >
                          <ArrowUpRight className="h-4 w-4" />
                          {artifactActionPendingId === artifact.artifact_id ? "打开中..." : "打开"}
                        </button>
                      </article>
                    ))
                  ) : !artifactLoading && !artifactErrorMessage ? (
                    <p className="task-detail-card__empty">结果详情仍在同步，稍后可重试详情或直接尝试打开最新结果。</p>
                  ) : null}
                </div>
              </section>
            ) : null}
          </div>
        </ScrollArea>
      </motion.section>
    );
  }

  function handleSubmitSteering() {
    if (!steeringMessage.trim()) {
      return;
    }
    onSteerTask(steeringMessage);
  }

  function handleApplyEventFilters() {
    onApplyEventFilters({
      eventType: eventFilterDraft.eventType.trim(),
      runId: eventFilterDraft.runId.trim(),
      timeRange: eventFilterDraft.timeRange,
    });
  }

  function handleResetEventFilters() {
    setEventFilterDraft(DEFAULT_TASK_EVENT_FILTERS);
    onResetEventFilters();
  }

  function updateEventTimeRange(timeRange: TaskEventTimeRange) {
    setEventFilterDraft((current) => ({
      ...current,
      timeRange,
    }));
  }

  function renderRuntimeEventFilters() {
    return (
      <div className="task-detail-runtime-filters">
        <label className="task-detail-runtime-filters__field">
          <span>事件类型</span>
          <input
            className="task-detail-runtime-filters__input"
            onChange={(event) =>
              setEventFilterDraft((current) => ({
                ...current,
                eventType: event.target.value,
              }))
            }
            placeholder="例如 loop.failed"
            value={eventFilterDraft.eventType}
          />
        </label>
          <label className="task-detail-runtime-filters__field">
            <span>运行编号</span>
            <input
              className="task-detail-runtime-filters__input"
            onChange={(event) =>
              setEventFilterDraft((current) => ({
                ...current,
                runId: event.target.value,
              }))
            }
            placeholder="例如 001"
            value={eventFilterDraft.runId}
          />
        </label>
        <label className="task-detail-runtime-filters__field">
          <span>时间范围</span>
          <select className="task-detail-runtime-filters__input" onChange={(event) => updateEventTimeRange(event.target.value as TaskEventTimeRange)} value={eventFilterDraft.timeRange}>
            <option value="all">全部时间</option>
            <option value="1h">最近 1 小时</option>
            <option value="24h">最近 24 小时</option>
            <option value="7d">最近 7 天</option>
          </select>
        </label>
        <div className="task-detail-runtime-filters__actions">
          <button className="task-detail-card__action" disabled={eventLoading} onClick={handleApplyEventFilters} type="button">
            <RefreshCcw className="h-4 w-4" />
            应用筛选
          </button>
          <button className="task-detail-card__action" disabled={eventLoading} onClick={handleResetEventFilters} type="button">
            重置
          </button>
        </div>
      </div>
    );
  }

  function renderRuntimeSummarySection() {
    if (!runtimeSummary || !hasRuntimeSummarySignals) {
      return null;
    }

    return (
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <p className="task-detail-card__eyebrow">Loop Signals</p>
          <h3 className="task-detail-card__title">循环事件与停止信号</h3>
        </div>
        <div className="task-detail-current-grid">
          <article className="task-detail-current-card">
            <Clock3 className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">Loop stop reason</p>
              <p className="task-detail-current-card__text">{runtimeSummary.loop_stop_reason ?? "当前未返回停止原因"}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <AlertTriangle className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">Latest event</p>
              <p className="task-detail-current-card__text">{runtimeSummary.latest_event_type ?? "当前未返回 runtime event"}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <ShieldAlert className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">Event count</p>
              <p className="task-detail-current-card__text">{runtimeSummary.events_count}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <SendHorizonal className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">Pending steering</p>
              <p className="task-detail-current-card__text">{runtimeSummary.active_steering_count}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <AlertTriangle className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">Latest failure</p>
              <p className="task-detail-current-card__text">{runtimeSummary.latest_failure_summary ?? "当前未返回失败摘要"}</p>
            </div>
          </article>
        </div>
        {runtimeSummary.observation_signals.length > 0 ? <p className="task-detail-card__hint">Observation: {runtimeSummary.observation_signals.join(" / ")}</p> : null}
      </section>
    );
  }

  function renderEvidenceSection() {
    if (!isScreenTask || detail === null || !hasEvidenceContent) {
      return null;
    }

    return (
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <div>
            <p className="task-detail-card__eyebrow">证据链</p>
            <h3 className="task-detail-card__title">截图证据与引用</h3>
          </div>
        </div>
        <p className="task-detail-card__hint">该区域只在屏幕类任务中展示截图、OCR 摘要与引用片段。</p>
        <div className="task-detail-output-list">
          {evidenceItems.length > 0
            ? evidenceItems.map((citation) => (
                <article key={citation.citation_id} className="task-detail-output-item">
                  <AlertTriangle className="h-4 w-4" />
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="task-detail-output-item__title">{citation.label}</p>
                      {citation.evidence_role ? <Badge variant="outline">{citation.evidence_role}</Badge> : null}
                      {citation.artifact_type ? <Badge variant="secondary">{citation.artifact_type}</Badge> : null}
                    </div>
                    {citation.excerpt_text ? <p className="task-detail-card__hint">{citation.excerpt_text}</p> : null}
                    <p className="task-detail-output-item__path">{citation.source_ref}</p>
                  </div>
                </article>
              ))
            : null}
          {evidenceArtifacts.length > 0
            ? evidenceArtifacts.map((artifact) => (
                <article key={`evidence_${artifact.artifact_id}`} className="task-detail-output-item">
                  <FolderOutput className="h-4 w-4" />
                  <div>
                    <p className="task-detail-output-item__title">{artifact.title}</p>
                    <p className="task-detail-output-item__path">{artifact.path}</p>
                  </div>
                  <button
                    className="task-detail-card__action"
                    disabled={artifactActionPendingId === artifact.artifact_id}
                    onClick={() => onOpenArtifact(artifact.artifact_id)}
                    type="button"
                  >
                    <ArrowUpRight className="h-4 w-4" />
                    {artifactActionPendingId === artifact.artifact_id ? "打开中..." : "打开证据"}
                  </button>
                </article>
              ))
            : null}
        </div>
      </section>
    );
  }

  function renderOutputSection(eyebrow: string, title: string) {
    return (
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <div>
            <p className="task-detail-card__eyebrow">{eyebrow}</p>
            <h3 className="task-detail-card__title">{title}</h3>
          </div>
        </div>
        <div className="task-detail-output-list">
          {artifactErrorMessage ? <p className="task-detail-card__hint">{artifactErrorMessage}</p> : null}
          {artifactLoading && !hasOutputContent ? <p className="task-detail-card__empty">正在同步成果列表...</p> : null}
          {hasFormalOutput && formalDeliveryResult ? (
            <article className={cn("task-detail-output-item", isInlineBubbleOutput && "task-detail-output-item--bubble") }>
              {isInlineBubbleOutput ? null : <SendHorizonal className="h-4 w-4" />}
              <div>
                {isInlineBubbleOutput ? (
                  <>
                    <p className="task-detail-output-item__title">{formalDeliveryResult.title}</p>
                    <p className="task-detail-output-item__bubble-copy">{formalDeliveryResult.preview_text}</p>
                  </>
                ) : (
                  <>
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="task-detail-output-item__title">{formalDeliveryResult.title}</p>
                      <Badge variant="outline">{formalDeliveryResult.type}</Badge>
                    </div>
                    <p className="task-detail-card__hint">{formalDeliveryResult.preview_text}</p>
                  </>
                )}
              </div>
              {canOpenFormalOutput ? (
                <button className="task-detail-card__action" disabled={deliveryActionPending} onClick={onOpenLatestDelivery} type="button">
                  <ArrowUpRight className="h-4 w-4" />
                  {deliveryActionPending ? "打开中..." : getTaskDeliveryOpenLabel(formalDeliveryResult)}
                </button>
              ) : null}
            </article>
          ) : null}
          {outputArtifacts.map((artifact) => (
            <article key={artifact.artifact_id} className="task-detail-output-item">
              <FolderOutput className="h-4 w-4" />
              <div>
                <p className="task-detail-output-item__title">{artifact.title}</p>
                <p className="task-detail-output-item__path">{artifact.path}</p>
              </div>
              <button
                className="task-detail-card__action"
                disabled={artifactActionPendingId === artifact.artifact_id}
                onClick={() => onOpenArtifact(artifact.artifact_id)}
                type="button"
              >
                <ArrowUpRight className="h-4 w-4" />
                {artifactActionPendingId === artifact.artifact_id ? "打开中..." : "打开"}
              </button>
            </article>
          ))}
          {!artifactLoading && !hasOutputContent ? <p className="task-detail-card__empty">当前没有可直接打开的产出内容。</p> : null}
        </div>
      </section>
    );
  }

  function renderScreenGovernanceSection() {
    if (!isScreenTask || shouldDeferSecuritySummary || !runtimeSummary || detail === null) {
      return null;
    }

    const latestFailureLabel =
      [runtimeSummary.latest_failure_category, runtimeSummary.latest_failure_code].filter((value): value is string => Boolean(value && value.trim().length > 0)).join(" · ") ||
      "当前没有失败记录";
    const latestFailureSummary = runtimeSummary.latest_failure_summary ?? "当前没有失败摘要";
    const approvalAnchor = detail.approval_request;
    const authorizationRecord = detail.authorization_record;
    const auditRecord = detail.audit_record;
    const restorePoint = detail.security_summary.latest_restore_point;

    return (
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <div>
            <p className="task-detail-card__eyebrow">屏幕安全与恢复</p>
            <h3 className="task-detail-card__title">屏幕安全与恢复</h3>
          </div>
        </div>
        <p className="task-detail-card__hint">该区域只展示授权、恢复点、审计记录与运行摘要，不读取未整理的工作输出。</p>
        <div className="task-detail-current-grid">
          <article className="task-detail-current-card">
            <ShieldAlert className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">授权依据</p>
              <p className="task-detail-current-card__text">{approvalAnchor?.operation_name ?? "当前没有活跃授权"}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <SendHorizonal className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">授权记录</p>
              <p className="task-detail-current-card__text">
                {authorizationRecord ? `${authorizationRecord.decision} · ${authorizationRecord.operator}` : "当前没有授权记录"}
              </p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <Clock3 className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">最近恢复点</p>
              <p className="task-detail-current-card__text">{restorePoint?.summary ?? "当前没有恢复点"}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <AlertTriangle className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">最近失败类别</p>
              <p className="task-detail-current-card__text">{latestFailureLabel}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <FolderOutput className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">审计记录</p>
              <p className="task-detail-current-card__text">{auditRecord ? `${auditRecord.action} · ${auditRecord.result}` : "当前没有审计记录"}</p>
            </div>
          </article>
          <article className="task-detail-current-card">
            <FolderOutput className="h-4 w-4" />
            <div>
              <p className="task-detail-current-card__label">证据数量</p>
              <p className="task-detail-current-card__text">{formalEvidenceCount}</p>
            </div>
          </article>
        </div>
        <p className="task-detail-card__hint">{latestFailureSummary}</p>
      </section>
    );
  }

  function renderRuntimeEventsSection() {
    if (detail === null || !hasRuntimeProcessContent) {
      return null;
    }

    return (
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <p className="task-detail-card__eyebrow">运行事件</p>
          <h3 className="task-detail-card__title">执行事件与循环回流</h3>
        </div>
        <p className="task-detail-card__hint">通过任务运行事件列表查看当前任务的运行时事件，可按事件类型、运行编号与时间范围筛选。</p>
        {renderRuntimeSummarySection()}
        {renderRuntimeEventFilters()}
        {eventErrorMessage ? <p className="task-detail-card__hint">{eventErrorMessage}</p> : null}
        {eventLoading && eventItems.length === 0 ? <p className="task-detail-card__empty">正在同步运行时事件...</p> : null}
        {eventItems.length > 0 ? (
          <div className="task-detail-runtime-list">
            {eventItems.map((event) => (
              <article key={event.event_id} className="task-detail-runtime-item">
                <div className="task-detail-runtime-item__meta">
                  <span className="task-detail-runtime-item__type">{event.type}</span>
                  <span>{formatTimestamp(event.created_at)}</span>
                </div>
                <p className="task-detail-runtime-item__summary">{event.payload?.stop_reason ? `stop_reason: ${String(event.payload.stop_reason)}` : `level: ${event.level}`}</p>
                <p className="task-detail-runtime-item__payload">{event.payload_json}</p>
              </article>
            ))}
          </div>
        ) : !eventLoading ? (
          <p className="task-detail-card__empty">当前没有可展示的运行时事件。</p>
        ) : null}
      </section>
    );
  }

  return (
    <motion.section animate={{ opacity: 1, x: 0 }} className="task-detail-shell" initial={{ opacity: 0, x: 18 }} transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}>
      <div className="task-detail-shell__header">
        <div>
          <p className="task-detail-shell__eyebrow">任务详情</p>
          <h2 className="task-detail-shell__title">{task.title}</h2>
          <p className="task-detail-shell__subtitle">{stateVoice.body}</p>
        </div>

        <div className="task-detail-shell__status-wrap">
          <Button className="task-detail-shell__close" onClick={onClose} size="icon-sm" variant="ghost">
            <X className="h-4 w-4" />
            <span className="sr-only">关闭任务详情</span>
          </Button>
          <Badge className={cn("border-0 px-3 py-1 text-[0.74rem] ring-1", getTaskStatusBadgeClass(task.status))}>
            {getTaskPreviewStatusLabel(task.status)}
          </Badge>
          {feedback ? (
            <span className="task-detail-shell__feedback">
              <AlertTriangle className="h-4 w-4" />
              {feedback}
            </span>
          ) : null}
        </div>
      </div>

        <div className="task-detail-shell__meta-grid">
        <div className="task-detail-shell__meta-card">
          <span>来源</span>
          <strong>{formatTaskSourceLabel(task.source_type)}</strong>
        </div>
        <div className="task-detail-shell__meta-card">
          <span>开始时间</span>
          <strong>{formatTimestamp(task.started_at)}</strong>
        </div>
        <div className="task-detail-shell__meta-card">
          <span>最近更新</span>
          <strong>{formatTimestamp(task.updated_at)}</strong>
        </div>
        <div className="task-detail-shell__meta-card">
          <span>进度</span>
          <strong>{progressLabel}</strong>
        </div>
      </div>

      <ScrollArea className="task-detail-shell__scroll">
        <div className="task-detail-shell__body">
          {detailState !== "ready" ? (
            <section className="task-detail-card task-detail-card--notice">
              <div className="task-detail-card__header task-detail-card__header--actionable">
                <div>
                  <p className="task-detail-card__eyebrow">详情状态</p>
                  <h3 className="task-detail-card__title">{detailNoticeTitle}</h3>
                </div>
                {isDetailError && onRetryDetail ? (
                  <button className="task-detail-card__action" onClick={onRetryDetail} type="button">
                    <RefreshCcw className="h-4 w-4" />
                    重试
                  </button>
                ) : null}
              </div>
              <p className="task-detail-ended-copy">{detailNoticeBody}</p>
            </section>
          ) : null}

          {detailWarningMessage ? (
            <section className="task-detail-card task-detail-card--notice">
              <div className="task-detail-card__header">
                <p className="task-detail-card__eyebrow">详情提示</p>
                <h3 className="task-detail-card__title">部分信息已降级展示</h3>
              </div>
              <p className="task-detail-ended-copy">{detailWarningMessage}</p>
            </section>
          ) : null}

          {!ended ? (
            <>
              {detail ? (
                <>
              {waitingCopy ? (
                <section className="task-detail-card task-detail-card--notice">
                  <div className="task-detail-card__header">
                    <p className="task-detail-card__eyebrow">当前提醒</p>
                    <h3 className="task-detail-card__title">为什么现在停在这里</h3>
                  </div>
                  <p className="task-detail-ended-copy">{waitingCopy}</p>
                </section>
              ) : null}

              <section className="task-detail-card task-detail-card--spotlight">
                <div className="task-detail-card__header">
                  <p className="task-detail-card__eyebrow">当前进展</p>
                  <h3 className="task-detail-card__title">完整任务进展</h3>
                </div>
                <TaskProgressTimeline timeline={detail.timeline} />
              </section>

              <section className="task-detail-card">
                <div className="task-detail-card__header">
                  <p className="task-detail-card__eyebrow">当前阶段</p>
                  <h3 className="task-detail-card__title">现在正在推进什么</h3>
                </div>
                <div className="task-detail-current-grid">
                  <article className="task-detail-current-card">
                    <Clock3 className="h-4 w-4" />
                    <div>
                      <p className="task-detail-current-card__label">执行到哪一步</p>
                      <p className="task-detail-current-card__text">{progress.currentLabel}</p>
                    </div>
                  </article>
                  <article className="task-detail-current-card">
                    <ShieldAlert className="h-4 w-4" />
                    <div>
                      <p className="task-detail-current-card__label">当前提醒</p>
                      <p className="task-detail-current-card__text">{experience?.nextAction ?? "详情返回后补充下一步建议。"}</p>
                    </div>
                  </article>
                </div>
              </section>

              {renderEvidenceSection()}

              {renderScreenGovernanceSection()}

              {detailData ? <TaskContextBlock detailData={detailData} /> : null}

              {renderRuntimeEventsSection()}

              {renderOutputSection("成果区", "已生成的内容")}

              {shouldDeferSecuritySummary ? (
                <section className="task-detail-card task-detail-card--notice">
                  <div className="task-detail-card__header">
                    <p className="task-detail-card__eyebrow">信任摘要</p>
                    <h3 className="task-detail-card__title">等待安全详情</h3>
                  </div>
                  <p className="task-detail-ended-copy">等待详情同步后展示风险、授权与恢复点。</p>
                </section>
              ) : (
                <section className="task-detail-card">
                  <div className="task-detail-card__header">
                    <p className="task-detail-card__eyebrow">信任摘要</p>
                    <h3 className="task-detail-card__title">风险与授权情况</h3>
                  </div>
                  <div className="task-detail-current-grid">
                    <article className="task-detail-current-card">
                      <ShieldAlert className="h-4 w-4" />
                      <div>
                        <p className="task-detail-current-card__label">风险状态</p>
                        <p className="task-detail-current-card__text">{detail.security_summary.risk_level}</p>
                      </div>
                    </article>
                    <article className="task-detail-current-card">
                      <Clock3 className="h-4 w-4" />
                      <div>
                        <p className="task-detail-current-card__label">待授权数量</p>
                        <p className="task-detail-current-card__text">{detail.security_summary.pending_authorizations}</p>
                      </div>
                    </article>
                    <article className="task-detail-current-card">
                      <ShieldAlert className="h-4 w-4" />
                      <div>
                        <p className="task-detail-current-card__label">边界状态</p>
                        <p className="task-detail-current-card__text">{detail.security_summary.security_status}</p>
                      </div>
                    </article>
                    <article className="task-detail-current-card">
                      <FolderOutput className="h-4 w-4" />
                      <div>
                        <p className="task-detail-current-card__label">恢复点</p>
                        <p className="task-detail-current-card__text">
                          {detail.security_summary.latest_restore_point
                            ? detail.security_summary.latest_restore_point.summary || detail.security_summary.latest_restore_point.recovery_point_id
                            : "当前没有恢复点"}
                        </p>
                      </div>
                    </article>
                  </div>
                </section>
              )}
                </>
              ) : null}

              <section className="task-detail-card">
                <div className="task-detail-card__header task-detail-card__header--actionable">
                  <div>
                    <p className="task-detail-card__eyebrow">任务引导</p>
                    <h3 className="task-detail-card__title">补充新的执行要求</h3>
                  </div>
                </div>
                <p className="task-detail-card__hint">{steeringHint}</p>
                <div className="task-detail-steer-box">
                  <textarea
                    className="task-detail-steer-box__input"
                    disabled={!canSteerTask || steeringPending}
                    onChange={(event) => setSteeringMessage(event.target.value)}
                    placeholder={steeringPlaceholder}
                    rows={3}
                    value={steeringMessage}
                  />
                  <button className="task-detail-card__action" disabled={!canSteerTask || steeringPending || !steeringMessage.trim()} onClick={handleSubmitSteering} type="button">
                    <SendHorizonal className="h-4 w-4" />
                    {steeringPending ? "提交中..." : "追加要求"}
                  </button>
                </div>
              </section>
            </>
          ) : (
            <>
              <section className="task-detail-card task-detail-card--spotlight">
                <div className="task-detail-card__header">
                  <div>
                    <p className="task-detail-card__eyebrow">任务结果</p>
                    <h3 className="task-detail-card__title">这条任务已经结束</h3>
                  </div>
                </div>
                {!shouldHideEndedResultCopy ? <p className="task-detail-ended-copy">{experience?.endedSummary ?? stateVoice.body}</p> : null}
                <p className="task-detail-ended-time">结束时间：{formatTimestamp(task.finished_at)}</p>
              </section>

              {renderOutputSection("产出内容", "已生成的结果")}

              {detail ? (
                <>
                  {renderEvidenceSection()}

                  {renderScreenGovernanceSection()}

                  {renderRuntimeEventsSection()}
                </>
              ) : null}
            </>
          )}
        </div>
      </ScrollArea>

      {task ? <TaskActionBar detail={detail} onAction={onAction} task={task} /> : null}
    </motion.section>
  );
}
