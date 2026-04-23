import { motion } from "motion/react";
import { CalendarClock, FolderOpen, Repeat, Sparkles, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { getNoteBucketLabel, getNoteStatusBadgeClass } from "../notePage.mapper";
import type { NoteDetailAction, NoteListItem } from "../notePage.types";
import { NoteActionBar } from "./NoteActionBar";

type NoteDetailPanelProps = {
  feedback: string | null;
  item: NoteListItem;
  onAction: (action: NoteDetailAction) => void;
  onClose: () => void;
  onResourceOpen: (resourceId: string) => void;
};

export function NoteDetailPanel({ feedback, item, onAction, onClose, onResourceOpen }: NoteDetailPanelProps) {
  const { experience } = item;

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
                    <Repeat className="h-4 w-4" />
                    <div>
                      <p className="note-detail-list__label">生效范围</p>
                      <p className="note-detail-list__value">{experience.effectiveScope}</p>
                    </div>
                  </article>
                ) : null}
              </div>
            </section>
          </div>

          <section className="note-detail-card">
            <div className="note-detail-card__header">
              <p className="note-detail-card__eyebrow">Agent 建议</p>
              <h3 className="note-detail-card__title">下一步怎么做更合适</h3>
            </div>
            <p className="note-detail-card__copy">{experience.agentSuggestion.detail}</p>
          </section>

          <section className="note-detail-card">
            <div className="note-detail-card__header">
              <p className="note-detail-card__eyebrow">相关资料</p>
              <h3 className="note-detail-card__title">当前事项关联的入口</h3>
            </div>
            <div className="note-detail-resource-list">
              {experience.relatedResources.length > 0 ? (
                experience.relatedResources.map((resource) => (
                  <article key={resource.id} className="note-detail-resource-item">
                    <FolderOpen className="h-4 w-4" />
                    <div>
                      <p className="note-detail-resource-item__title">{resource.label}</p>
                      <p className="note-detail-resource-item__meta">{resource.type}</p>
                      <p className="note-detail-resource-item__path">{resource.path}</p>
                    </div>
                    <Button className="note-detail-resource-item__open" onClick={() => onResourceOpen(resource.id)} variant="ghost">
                      打开
                    </Button>
                  </article>
                ))
              ) : (
                <p className="note-detail-card__copy">当前没有挂载相关资料，后续可以补充到这里。</p>
              )}
            </div>
          </section>
        </div>
      </ScrollArea>

      <NoteActionBar item={item} onAction={onAction} />
    </motion.section>
  );
}
