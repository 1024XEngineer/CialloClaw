import { FileText, FileUp, ListChecks, NotebookPen } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/utils/cn";
import { formatStatusLabel, formatTimestamp } from "@/utils/formatters";
import { getTaskStatusBadgeClass } from "../taskPage.mapper";
import type { AssistantCardKey, TaskDetailData, TaskTabsValue } from "../taskPage.types";

type TaskTabsPanelProps = {
  activeTab: TaskTabsValue;
  detailData: TaskDetailData;
  noteDraft: string;
  onHighlightAssistantCard: (card: AssistantCardKey) => void;
  onNoteDraftChange: (value: string) => void;
  onTabChange: (value: TaskTabsValue) => void;
};

const tabItems = [
  { value: "details", label: "任务详情", icon: FileText },
  { value: "subtasks", label: "子任务", icon: ListChecks },
  { value: "outputs", label: "产出内容", icon: FileUp },
  { value: "notes", label: "笔记记录", icon: NotebookPen },
] as const;

function renderTabBody({
  activeTab,
  detailData,
  noteDraft,
  onHighlightAssistantCard,
  onNoteDraftChange,
}: Omit<TaskTabsPanelProps, "onTabChange">) {
  const { detail, experience, task } = detailData;

  if (activeTab === "details") {
    return (
      <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr]">
        <Card className="task-capsule-subcard rounded-[28px] border-0">
          <CardHeader className="px-5 pt-5">
            <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">任务背景</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5 px-5 pb-5 text-sm leading-7 text-slate-600">
            <p>{experience.background}</p>
            <div className="grid gap-4 md:grid-cols-2">
              <article className="rounded-[24px] bg-white/70 p-4">
                <p className="text-xs uppercase tracking-[0.22em] text-slate-500">当前目标</p>
                <p className="mt-3 text-[0.95rem] leading-7 text-slate-700">{experience.goal}</p>
              </article>
              <article className="rounded-[24px] bg-white/70 p-4">
                <p className="text-xs uppercase tracking-[0.22em] text-slate-500">当前阶段</p>
                <p className="mt-3 text-[0.95rem] leading-7 text-slate-700">{experience.phase}</p>
              </article>
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4">
          <Card className="task-capsule-subcard rounded-[28px] border-0">
            <CardHeader className="px-5 pt-5">
              <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">约束</CardTitle>
            </CardHeader>
            <CardContent className="px-5 pb-5">
              <ul className="space-y-3 text-sm leading-6 text-slate-600">
                {experience.constraints.map((item) => (
                  <li key={item} className="rounded-[20px] bg-white/70 px-4 py-3">
                    {item}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          <Card className="task-capsule-subcard rounded-[28px] border-0">
            <CardHeader className="px-5 pt-5">
              <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">验收标准</CardTitle>
            </CardHeader>
            <CardContent className="px-5 pb-5">
              <ul className="space-y-3 text-sm leading-6 text-slate-600">
                {experience.acceptance.map((item) => (
                  <li key={item} className="rounded-[20px] bg-white/70 px-4 py-3">
                    {item}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>
      </div>
    );
  }

  if (activeTab === "subtasks") {
    return (
      <div className="space-y-3">
        {detail.timeline.map((step) => (
          <motion.button
            key={step.step_id}
            whileHover={{ y: -4 }}
            className="task-capsule-step-card w-full rounded-[26px] border border-white/70 bg-white/68 p-4 text-left shadow-[0_24px_46px_-34px_rgba(67,85,106,0.4)] transition-shadow hover:shadow-[0_28px_54px_-30px_rgba(72,94,122,0.45)]"
            onClick={() => onHighlightAssistantCard(experience.stepTargets[step.step_id] ?? "agent")}
            type="button"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-[0.7rem] uppercase tracking-[0.22em] text-slate-500">步骤 {step.order_index}</p>
                <h3 className="mt-2 text-lg font-semibold tracking-[-0.03em] text-slate-800">{step.name}</h3>
                <p className="mt-3 text-sm leading-6 text-slate-600">{step.input_summary}</p>
                <p className="mt-2 text-sm leading-6 text-slate-500">{step.output_summary}</p>
              </div>
              <Badge className={cn("border-0 px-3 py-1 text-[0.74rem] ring-1", getTaskStatusBadgeClass(step.status as typeof task.status))}>
                {formatStatusLabel(step.status)}
              </Badge>
            </div>
          </motion.button>
        ))}
      </div>
    );
  }

  if (activeTab === "outputs") {
    return (
      <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <div className="grid gap-4">
          {experience.outputs.map((output) => (
            <Card key={output.id} className="task-capsule-subcard rounded-[28px] border-0">
              <CardHeader className="px-5 pt-5">
                <div className="flex items-center justify-between gap-3">
                  <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">{output.label}</CardTitle>
                  <Badge className="border-0 bg-white/70 px-3 py-1 text-[0.72rem] text-slate-600 ring-1 ring-white/80">
                    {output.tone === "draft" ? "draft" : output.tone === "result" ? "result" : "editable"}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="px-5 pb-5 text-sm leading-7 text-slate-600">{output.content}</CardContent>
            </Card>
          ))}
        </div>

        <Card className="task-capsule-subcard rounded-[28px] border-0">
          <CardHeader className="px-5 pt-5">
            <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">成果区</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 px-5 pb-5">
            {detail.artifacts.map((artifact) => (
              <div key={artifact.artifact_id} className="rounded-[22px] bg-white/72 p-4 text-sm text-slate-600">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="font-medium text-slate-800">{artifact.title}</p>
                    <p className="mt-1 text-xs uppercase tracking-[0.18em] text-slate-500">{artifact.artifact_type}</p>
                  </div>
                  <Tooltip>
                    <TooltipTrigger className="inline-flex">
                      <button className="rounded-full border border-slate-200 bg-white/80 px-3 py-1.5 text-xs text-slate-600" type="button">
                        打开
                      </button>
                    </TooltipTrigger>
                    <TooltipContent className="rounded-full bg-slate-900/90 px-3 py-1.5 text-[0.72rem] text-white">
                      当前协议尚未提供稳定的 artifact.open 能力
                    </TooltipContent>
                  </Tooltip>
                </div>
                <p className="mt-3 rounded-[18px] bg-slate-50/90 px-3 py-2 font-mono text-xs text-slate-500">{artifact.path}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
      <Card className="task-capsule-subcard rounded-[28px] border-0">
        <CardHeader className="px-5 pt-5">
          <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">过程记录</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 px-5 pb-5">
          {experience.noteEntries.map((entry) => (
            <div key={entry} className="rounded-[20px] bg-white/72 px-4 py-3 text-sm leading-6 text-slate-600">
              {entry}
            </div>
          ))}
        </CardContent>
      </Card>

      <Card className="task-capsule-subcard rounded-[28px] border-0">
        <CardHeader className="px-5 pt-5">
          <div className="flex items-center justify-between gap-3">
            <CardTitle className="text-lg font-semibold tracking-[-0.03em] text-slate-800">可编辑笔记</CardTitle>
            <Badge className="border-0 bg-white/70 px-3 py-1 text-[0.72rem] text-slate-600 ring-1 ring-white/80">本地草稿</Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-3 px-5 pb-5">
          <Textarea
            className="min-h-[17rem] rounded-[24px] border border-white/80 bg-white/72 px-4 py-4 text-sm leading-7 text-slate-700 shadow-[inset_0_1px_0_rgba(255,255,255,0.7)] placeholder:text-slate-400 focus-visible:border-slate-300 focus-visible:ring-[3px] focus-visible:ring-slate-200/80"
            onChange={(event) => onNoteDraftChange(event.target.value)}
            placeholder="把临时想法、决策点和过程记录留在这里。"
            value={noteDraft}
          />
          <p className="text-xs uppercase tracking-[0.18em] text-slate-500">最近更新 {formatTimestamp(task.updated_at)}</p>
        </CardContent>
      </Card>
    </div>
  );
}

export function TaskTabsPanel(props: TaskTabsPanelProps) {
  return (
    <Card className="task-capsule-card task-capsule-tabs-card min-h-0 rounded-[32px] border-0">
      <CardContent className="flex min-h-0 flex-1 flex-col gap-4 p-5">
        <Tabs className="flex min-h-0 flex-1 flex-col gap-4" onValueChange={(value) => props.onTabChange(value as TaskTabsValue)} value={props.activeTab}>
          <TabsList className="task-capsule-tab-list gap-2 rounded-full bg-white/65 p-1.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.65)]" variant="line">
            {tabItems.map((item) => {
              const Icon = item.icon;

              return (
                <TabsTrigger key={item.value} className="task-capsule-tab-trigger rounded-full px-4 py-2 text-sm text-slate-500 data-active:bg-white data-active:text-slate-800 data-active:shadow-[0_18px_30px_-24px_rgba(72,94,122,0.42)]" value={item.value}>
                  <Icon className="h-4 w-4" />
                  {item.label}
                </TabsTrigger>
              );
            })}
          </TabsList>

          <ScrollArea className="task-capsule-scroll min-h-0 flex-1 rounded-[28px] border border-white/65 bg-white/48 p-1">
            <AnimatePresence mode="wait">
              <motion.div
                key={props.activeTab}
                animate={{ opacity: 1, y: 0 }}
                className="min-h-[28rem] p-3"
                exit={{ opacity: 0, y: -10 }}
                initial={{ opacity: 0, y: 12 }}
                transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
              >
                <TabsContent className="mt-0" value={props.activeTab}>
                  {renderTabBody(props)}
                </TabsContent>
              </motion.div>
            </AnimatePresence>
          </ScrollArea>
        </Tabs>
      </CardContent>
    </Card>
  );
}
