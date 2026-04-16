import { Files, MessageCircleMore, Sparkles, WandSparkles } from "lucide-react";
import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/utils/cn";
import type { AssistantCardKey, TaskDetailData } from "../taskPage.types";

type TaskAssistantPanelProps = {
  detailData: TaskDetailData;
  highlightedCard: AssistantCardKey | null;
};

const assistantCards = [
  { key: "agent", label: "Agent 状态", icon: Sparkles },
  { key: "files", label: "相关文件", icon: Files },
  { key: "context", label: "快速上下文", icon: MessageCircleMore },
] as const;

export function TaskAssistantPanel({ detailData, highlightedCard }: TaskAssistantPanelProps) {
  const { detail, experience } = detailData;

  return (
    <aside className="task-capsule-sidebar grid min-h-0 gap-4">
      {assistantCards.map((item) => {
        const Icon = item.icon;
        const isHighlighted = highlightedCard === item.key;

        return (
          <motion.div key={item.key} animate={isHighlighted ? { scale: 1.015 } : { scale: 1 }} transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}>
            <Card className={cn("task-capsule-card rounded-[30px] border-0", isHighlighted && "task-capsule-card--highlighted")}>
              <CardHeader className="px-5 pt-5">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-white/70 text-slate-700 shadow-[inset_0_1px_0_rgba(255,255,255,0.7)]">
                      <Icon className="h-4.5 w-4.5" />
                    </div>
                    <CardTitle className="text-base font-semibold tracking-[-0.03em] text-slate-800">{item.label}</CardTitle>
                  </div>
                  {item.key === "agent" ? (
                    <Badge className="border-0 bg-blue-100 px-3 py-1 text-[0.72rem] text-blue-700 ring-1 ring-blue-200/80">
                      {experience.assistantState.label}
                    </Badge>
                  ) : null}
                </div>
              </CardHeader>

              <CardContent className="space-y-4 px-5 pb-5 text-sm leading-6 text-slate-600">
                {item.key === "agent" ? (
                  <>
                    <p>{experience.assistantState.hint}</p>
                    <div className="rounded-[22px] bg-white/72 p-4">
                      <p className="text-xs uppercase tracking-[0.2em] text-slate-500">一句轻提示</p>
                      <p className="mt-2 text-sm text-slate-700">{experience.suggestedNext}</p>
                    </div>
                  </>
                ) : null}

                {item.key === "files" ? (
                  <>
                    <div className="space-y-3">
                      {experience.relatedFiles.slice(0, 3).map((file) => (
                        <div key={file.id} className="rounded-[22px] bg-white/72 p-4">
                          <p className="font-medium text-slate-800">{file.title}</p>
                          <p className="mt-1 text-xs uppercase tracking-[0.18em] text-slate-500">{file.kind}</p>
                          <p className="mt-2 text-sm leading-6 text-slate-600">{file.note}</p>
                        </div>
                      ))}
                    </div>
                  </>
                ) : null}

                {item.key === "context" ? (
                  <>
                    <div className="space-y-3">
                      {experience.quickContext.map((snippet) => (
                        <div key={snippet.id} className="rounded-[22px] bg-white/72 p-4">
                          <p className="text-xs uppercase tracking-[0.2em] text-slate-500">{snippet.label}</p>
                          <p className="mt-2 text-sm leading-6 text-slate-700">{snippet.content}</p>
                        </div>
                      ))}
                    </div>
                    <div className="rounded-[22px] bg-[linear-gradient(135deg,rgba(238,244,252,0.86),rgba(255,249,242,0.82))] p-4 ring-1 ring-white/70">
                      <div className="flex items-center gap-2 text-sm font-medium text-slate-800">
                        <WandSparkles className="h-4 w-4 text-blue-600" />
                        推荐下一步
                      </div>
                      <p className="mt-2 text-sm leading-6 text-slate-600">{experience.suggestedNext}</p>
                    </div>
                    <div className="rounded-[22px] bg-white/72 p-4">
                      <p className="text-xs uppercase tracking-[0.2em] text-slate-500">最近相关对话</p>
                      <ul className="mt-3 space-y-2">
                        {experience.recentConversation.map((itemText) => (
                          <li key={itemText} className="text-sm leading-6 text-slate-600">
                            {itemText}
                          </li>
                        ))}
                      </ul>
                    </div>
                  </>
                ) : null}

                {item.key === "files" ? (
                  <p className="text-xs uppercase tracking-[0.18em] text-slate-500">本次详情已挂载 {detail.artifacts.length} 项成果记录</p>
                ) : null}
              </CardContent>
            </Card>
          </motion.div>
        );
      })}
    </aside>
  );
}
