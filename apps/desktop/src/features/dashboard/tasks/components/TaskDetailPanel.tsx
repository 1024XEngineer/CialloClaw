import { AlertTriangle, Clock3, FolderOutput, ShieldAlert, X } from "lucide-react";
import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { getTaskPreviewStatusLabel, getTaskProgress, getTaskStateVoice, getTaskStatusBadgeClass, isTaskEnded } from "../taskPage.mapper";
import type { TaskDetailData } from "../taskPage.types";
import { TaskActionBar } from "./TaskActionBar";
import { TaskContextBlock } from "./TaskContextBlock";
import { TaskProgressTimeline } from "./TaskProgressTimeline";

type TaskDetailPanelProps = {
  detailData: TaskDetailData;
  feedback: string | null;
  onAction: (action: "pause" | "resume" | "cancel" | "restart" | "edit" | "open-safety") => void;
  onClose: () => void;
};

export function TaskDetailPanel({ detailData, feedback, onAction, onClose }: TaskDetailPanelProps) {
  const { detail, experience, task } = detailData;
  const progress = getTaskProgress(detail.timeline);
  const stateVoice = getTaskStateVoice(task, experience, detail.timeline);
  const ended = isTaskEnded(task);
  const waitingCopy = task.status === "waiting_auth" || task.status === "waiting_input" || task.status === "paused" ? experience.waitingReason : task.status === "failed" || task.status === "blocked" ? experience.blockedReason : null;

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
          <strong>{task.source_type}</strong>
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
          <strong>
            {progress.completedCount}/{progress.total}
          </strong>
        </div>
      </div>

      <ScrollArea className="task-detail-shell__scroll">
        <div className="task-detail-shell__body">
          {!ended ? (
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
                      <p className="task-detail-current-card__text">{experience.nextAction}</p>
                    </div>
                  </article>
                </div>
              </section>

              <TaskContextBlock detailData={detailData} />

              <section className="task-detail-card">
                <div className="task-detail-card__header">
                  <p className="task-detail-card__eyebrow">成果区</p>
                  <h3 className="task-detail-card__title">已生成的文件与草稿</h3>
                </div>
                <div className="task-detail-output-list">
                  {detail.artifacts.length > 0 ? (
                    detail.artifacts.map((artifact) => (
                      <article key={artifact.artifact_id} className="task-detail-output-item">
                        <FolderOutput className="h-4 w-4" />
                        <div>
                          <p className="task-detail-output-item__title">{artifact.title}</p>
                          <p className="task-detail-output-item__path">{artifact.path}</p>
                        </div>
                      </article>
                    ))
                  ) : (
                    <p className="task-detail-ended-copy">当前还没有挂载成果物，完成后会出现在这里。</p>
                  )}
                </div>
              </section>

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
                            ? typeof detail.security_summary.latest_restore_point === "string"
                              ? detail.security_summary.latest_restore_point
                              : detail.security_summary.latest_restore_point.summary || detail.security_summary.latest_restore_point.recovery_point_id
                            : "当前没有恢复点"}
                        </p>
                      </div>
                    </article>
                  </div>
                </section>
            </>
          ) : (
            <>
              <section className="task-detail-card task-detail-card--spotlight">
                <div className="task-detail-card__header">
                  <p className="task-detail-card__eyebrow">任务结果</p>
                  <h3 className="task-detail-card__title">这条任务已经结束</h3>
                </div>
                <p className="task-detail-ended-copy">{experience.endedSummary ?? stateVoice.body}</p>
                <p className="task-detail-ended-time">结束时间：{formatTimestamp(task.finished_at)}</p>
              </section>

              <section className="task-detail-card">
                <div className="task-detail-card__header">
                  <p className="task-detail-card__eyebrow">产出内容</p>
                  <h3 className="task-detail-card__title">已生成的结果</h3>
                </div>
                <div className="task-detail-output-list">
                  {detail.artifacts.map((artifact) => (
                    <article key={artifact.artifact_id} className="task-detail-output-item">
                      <FolderOutput className="h-4 w-4" />
                      <div>
                        <p className="task-detail-output-item__title">{artifact.title}</p>
                        <p className="task-detail-output-item__path">{artifact.path}</p>
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            </>
          )}
        </div>
      </ScrollArea>

      <TaskActionBar detailData={detailData} onAction={onAction} />
    </motion.section>
  );
}
