import { motion } from "motion/react";
import { ArrowUpRight, CalendarClock, Link2, Repeat, Sparkles, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { formatNoteDisplayPath, getNoteBucketLabel, getNoteStatusBadgeClass } from "../notePage.mapper";
import type { NoteDetailAction, NoteListItem, NoteResource } from "../notePage.types";
import { NoteActionBar } from "./NoteActionBar";

type NoteDetailPanelProps = {
  feedback: string | null;
  item: NoteListItem;
  onAction: (action: NoteDetailAction) => void;
  onClose: () => void;
  onOpenLinkedTask?: () => void;
  onOpenResource?: (resourceId: string) => void;
  onToggleRecurring?: () => void;
  scheduleActionLabel?: string;
  scheduleDisabledReason?: string | null;
  scheduleDueAt?: string;
  scheduleEditing?: boolean;
  scheduleRepeatRule?: string;
  isSavingSchedule?: boolean;
  onCancelScheduleEdit?: () => void;
  onResetSchedule?: () => void;
  onSaveSchedule?: () => void;
  onScheduleDueAtChange?: (value: string) => void;
  onScheduleRepeatRuleChange?: (value: string) => void;
  onStartScheduleEdit?: () => void;
};

function formatLinkedTaskReference(taskId: string) {
  return taskId.length > 18 ? `${taskId.slice(0, 18)}...` : taskId;
}

function getResourceMeta(resource: NoteResource) {
  if (resource.openAction === "task_detail" && resource.taskId) {
    return "任务详情入口";
  }

  if (resource.openAction === "open_url") {
    return `${resource.type} · 外部链接`;
  }

  if (resource.openAction === "reveal_in_folder") {
    return `${resource.type} · 打开目录`;
  }

  if (resource.openAction === "copy_path") {
    return `${resource.type} · 复制路径`;
  }

  return `${resource.type} · 本地文件`;
}

function getResourceTarget(resource: NoteResource) {
  if (resource.url) {
    return resource.url;
  }

  if (resource.taskId) {
    return `任务 ID · ${formatLinkedTaskReference(resource.taskId)}`;
  }

  return formatNoteDisplayPath(resource.path);
}

/**
 * Renders the detail surface for one note together with follow-up actions.
 *
 * @param props Selected note data, close handlers, and inline schedule controls.
 * @returns The note detail panel shown inside the dashboard modal shell.
 */
export function NoteDetailPanel({
  feedback,
  item,
  onAction,
  onClose,
  onOpenLinkedTask,
  onOpenResource,
  onToggleRecurring,
  scheduleActionLabel = "安排时间",
  scheduleDisabledReason = null,
  scheduleDueAt = "",
  scheduleEditing = false,
  scheduleRepeatRule = "",
  isSavingSchedule = false,
  onCancelScheduleEdit,
  onResetSchedule,
  onSaveSchedule,
  onScheduleDueAtChange,
  onScheduleRepeatRuleChange,
  onStartScheduleEdit,
}: NoteDetailPanelProps) {
  const { experience } = item;
  const hasRelatedEntries = Boolean(item.item.linked_task_id) || experience.relatedResources.length > 0;
  const hasScheduleDraft = scheduleDueAt.trim() !== "" || scheduleRepeatRule.trim() !== "";
  const scheduleRequiresStartTime = scheduleRepeatRule.trim() !== "" && scheduleDueAt.trim() === "";
  const scheduleEditDisabled = scheduleDisabledReason !== null || isSavingSchedule;
  const scheduleSaveDisabled = scheduleEditDisabled || !hasScheduleDraft || scheduleRequiresStartTime;
  const isRecurringRule = item.item.bucket === "recurring_rule";
  const recurringToggleLabel = item.experience.isRecurringEnabled ? "暂停重复" : "开启重复";
  const recurringCollapsedHint = item.experience.isRecurringEnabled
    ? "当前规则正在生效；可以直接暂停，也可以继续修改首次时间和重复规则。"
    : "当前规则已暂停，不会继续生成新的巡检实例；点击“开启重复”可立即恢复。";
  const scheduleHelperText = scheduleDisabledReason
    ?? (scheduleRequiresStartTime
      ? "设置重复规则前请先填写首次时间。"
      : "直接在详情页里设置首次时间和重复规则；正文编辑器仍保持只写内容。");

  return (
    <motion.section animate={{ opacity: 1, x: 0 }} className="note-detail-shell" initial={{ opacity: 0, x: 18 }} transition={{ duration: 0.26, ease: [0.22, 1, 0.36, 1] }}>
      <div className="note-detail-shell__header">
        <div>
          <p className="note-detail-shell__eyebrow">便签详情</p>
          <h2 className="note-detail-shell__title">{item.item.title}</h2>
          <p className="note-detail-shell__subtitle">{experience.agentSuggestion.detail}</p>
        </div>

        <div className="note-detail-shell__status-wrap">
          <Button className="note-detail-shell__close" onClick={onClose} size="icon-sm" variant="ghost">
            <X className="h-4 w-4" />
            <span className="sr-only">关闭便签详情</span>
          </Button>
          <Badge className={cn("border-0 px-3 py-1 text-[0.74rem] ring-1", getNoteStatusBadgeClass(item.item.status))}>{experience.detailStatus}</Badge>
          {feedback ? <span className="note-detail-shell__feedback">{feedback}</span> : null}
        </div>
      </div>

      <div className="note-detail-shell__meta-grid">
        <div className="note-detail-shell__meta-card">
          <span>分类</span>
          <strong>{getNoteBucketLabel(item.item.bucket)}</strong>
        </div>
        <div className="note-detail-shell__meta-card">
          <span>事项类型</span>
          <strong>{experience.typeLabel}</strong>
        </div>
        <div className="note-detail-shell__meta-card">
          <span>时间信息</span>
          <strong>{experience.timeHint}</strong>
        </div>
        <div className="note-detail-shell__meta-card">
          <span>Agent 建议</span>
          <strong>{experience.agentSuggestion.label}</strong>
        </div>
      </div>

      <ScrollArea className="note-detail-shell__scroll">
        <div className="note-detail-shell__body">
          <section className="note-detail-card note-detail-card--spotlight">
            <div className="note-detail-card__header">
              <p className="note-detail-card__eyebrow">备注</p>
              <h3 className="note-detail-card__title">这条事项的背景与说明</h3>
            </div>
            <p className="note-detail-card__copy">{experience.noteText}</p>
          </section>

          <div className="note-detail-grid">
            <section className="note-detail-card">
              <div className="note-detail-card__header">
                <p className="note-detail-card__eyebrow">时间</p>
                <h3 className="note-detail-card__title">时间与状态</h3>
              </div>
              <div className="note-detail-list">
                {experience.plannedAt ? (
                  <article className="note-detail-list__item">
                    <CalendarClock className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">计划时间</p>
                      <p className="note-detail-list__value">{formatTimestamp(experience.plannedAt)}</p>
                    </div>
                  </article>
                ) : null}

                {experience.nextOccurrenceAt ? (
                  <article className="note-detail-list__item">
                    <Repeat className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">下次发生</p>
                      <p className="note-detail-list__value">{formatTimestamp(experience.nextOccurrenceAt)}</p>
                    </div>
                  </article>
                ) : null}

                {experience.endedAt ? (
                  <article className="note-detail-list__item">
                    <CalendarClock className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">结束时间</p>
                      <p className="note-detail-list__value">{formatTimestamp(experience.endedAt)}</p>
                    </div>
                  </article>
                ) : null}

                <article className="note-detail-list__item">
                  <Sparkles className="h-4 w-4" />
                  <div>
                    <p className="note-detail-list__label">状态说明</p>
                    <p className="note-detail-list__value">{experience.detailStatus}</p>
                  </div>
                </article>
              </div>
            </section>

            <section className="note-detail-card">
              <div className="note-detail-card__header">
                <p className="note-detail-card__eyebrow">条件与规则</p>
                <h3 className="note-detail-card__title">前置条件和重复范围</h3>
              </div>
              <div className="note-detail-list">
                {experience.prerequisite ? (
                  <article className="note-detail-list__item">
                    <Sparkles className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">前置条件</p>
                      <p className="note-detail-list__value">{experience.prerequisite}</p>
                    </div>
                  </article>
                ) : null}

                {experience.repeatRule ? (
                  <article className="note-detail-list__item">
                    <Repeat className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">重复规则</p>
                      <p className="note-detail-list__value">{experience.repeatRule}</p>
                    </div>
                  </article>
                ) : null}

                {experience.recentInstanceStatus ? (
                  <article className="note-detail-list__item">
                    <Repeat className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">最近一次状态</p>
                      <p className="note-detail-list__value">{experience.recentInstanceStatus}</p>
                    </div>
                  </article>
                ) : null}

                {experience.effectiveScope ? (
                  <article className="note-detail-list__item">
                    <Sparkles className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">生效范围</p>
                      <p className="note-detail-list__value">{formatNoteDisplayPath(experience.effectiveScope)}</p>
                    </div>
                  </article>
                ) : null}
              </div>

              {onStartScheduleEdit ? (
                <div className="note-detail-card__actions">
                  {scheduleEditing ? (
                    <motion.div
                      animate={{ opacity: 1, y: 0 }}
                      className="note-detail-schedule-editor"
                      initial={{ opacity: 0, y: 8 }}
                      transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
                    >
                      <div className="note-detail-schedule-editor__grid">
                        <label className="note-schedule-modal__field">
                          <span className="note-schedule-modal__label">
                            <CalendarClock className="h-4 w-4" />
                            首次时间
                          </span>
                          <input
                            className="note-schedule-modal__input"
                            disabled={scheduleEditDisabled}
                            onChange={(event) => onScheduleDueAtChange?.(event.target.value)}
                            type="datetime-local"
                            value={scheduleDueAt}
                          />
                        </label>

                        <label className="note-schedule-modal__field">
                          <span className="note-schedule-modal__label">
                            <Repeat className="h-4 w-4" />
                            重复规则
                          </span>
                          <input
                            className="note-schedule-modal__input"
                            disabled={scheduleEditDisabled}
                            onChange={(event) => onScheduleRepeatRuleChange?.(event.target.value)}
                            placeholder="例如：每周、每两周、每天、每月"
                            type="text"
                            value={scheduleRepeatRule}
                          />
                        </label>
                      </div>

                      <p className="note-detail-card__hint">{scheduleHelperText}</p>

                      <div className="note-schedule-modal__actions note-detail-schedule-editor__actions">
                        <Button disabled={isSavingSchedule} onClick={onCancelScheduleEdit} type="button" variant="ghost">
                          收起
                        </Button>
                        <Button disabled={scheduleEditDisabled || !hasScheduleDraft} onClick={onResetSchedule} type="button" variant="ghost">
                          清除时间
                        </Button>
                        <Button className="note-schedule-modal__button--primary" disabled={scheduleSaveDisabled} onClick={onSaveSchedule} type="button">
                          {isSavingSchedule ? "保存中..." : "保存安排"}
                        </Button>
                      </div>
                    </motion.div>
                  ) : (
                    <>
                      <div className="note-detail-card__action-row">
                        <Button className="note-detail-card__action" disabled={scheduleDisabledReason !== null} onClick={onStartScheduleEdit} type="button" variant="outline">
                          <CalendarClock className="h-4 w-4" />
                          {scheduleActionLabel}
                        </Button>
                        {isRecurringRule && onToggleRecurring ? (
                          <Button
                            className={cn(
                              "note-detail-card__action",
                              item.experience.isRecurringEnabled
                                ? "note-detail-card__action--warn"
                                : "note-detail-card__action--accent",
                            )}
                            onClick={onToggleRecurring}
                            type="button"
                            variant="outline"
                          >
                            <Repeat className="h-4 w-4" />
                            {recurringToggleLabel}
                          </Button>
                        ) : null}
                      </div>
                      <p className="note-detail-card__hint">{isRecurringRule ? recurringCollapsedHint : scheduleHelperText}</p>
                    </>
                  )}
                </div>
              ) : null}
            </section>
          </div>

          {hasRelatedEntries ? (
            <section className="note-detail-card">
              <div className="note-detail-card__header">
                <p className="note-detail-card__eyebrow">相关入口</p>
                <h3 className="note-detail-card__title">关联任务与资料</h3>
              </div>

              <div className="note-detail-resource-list">
                {item.item.linked_task_id ? (
                  <button
                    className="note-detail-resource-item note-detail-resource-item--button"
                    disabled={!onOpenLinkedTask}
                    onClick={onOpenLinkedTask}
                    type="button"
                  >
                    <Link2 className="h-4 w-4" />
                    <div>
                      <p className="note-detail-resource-item__title">关联任务</p>
                      <p className="note-detail-resource-item__meta">已关联任务</p>
                      <p className="note-detail-resource-item__path">任务 ID · {formatLinkedTaskReference(item.item.linked_task_id)}</p>
                    </div>
                  </button>
                ) : null}

                {experience.relatedResources.map((resource) => (
                  <button
                    key={resource.id}
                    className="note-detail-resource-item note-detail-resource-item--button"
                    disabled={!onOpenResource}
                    onClick={() => onOpenResource?.(resource.id)}
                    type="button"
                  >
                    <ArrowUpRight className="h-4 w-4" />
                    <div>
                      <p className="note-detail-resource-item__title">{resource.label}</p>
                      <p className="note-detail-resource-item__meta">{getResourceMeta(resource)}</p>
                      <p className="note-detail-resource-item__path">{getResourceTarget(resource)}</p>
                    </div>
                  </button>
                ))}
              </div>
            </section>
          ) : null}

          <section className="note-detail-card">
            <div className="note-detail-card__header">
              <p className="note-detail-card__eyebrow">Agent 建议</p>
              <h3 className="note-detail-card__title">下一步怎么做更合适</h3>
            </div>
            <p className="note-detail-card__copy">{experience.agentSuggestion.detail}</p>
          </section>
        </div>
      </ScrollArea>

      <NoteActionBar item={item} onAction={onAction} />
    </motion.section>
  );
}
