import type { ComponentType } from "react";
import { ArrowUpRight, CalendarClock, CheckCircle2, Clock3, Pencil, Repeat, RotateCcw, Trash2, WandSparkles, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { NoteDetailAction, NoteListItem } from "../notePage.types";

type NoteActionBarProps = {
  item: NoteListItem;
  onAction: (action: NoteDetailAction) => void;
};

const actionIcons: Record<NoteDetailAction, ComponentType<{ className?: string }>> = {
  cancel: XCircle,
  "cancel-recurring": XCircle,
  complete: CheckCircle2,
  "convert-to-task": WandSparkles,
  delete: Trash2,
  edit: Pencil,
  "move-upcoming": CalendarClock,
  "open-resource": ArrowUpRight,
  restore: RotateCcw,
  "skip-once": Clock3,
  "toggle-recurring": Repeat,
};

function getActions(item: NoteListItem) {
  if (item.sourceNote?.localOnly) {
    return [
      { action: "edit" as const, label: "编辑源便签", tooltip: "继续编辑任务来源目录里的 markdown 便签。" },
      { action: "open-resource" as const, label: "打开源文件", tooltip: "直接打开这张源便签对应的 markdown 文件。" },
    ];
  }

  if (item.item.bucket === "upcoming") {
    return [
      { action: "complete" as const, label: "标记完成", tooltip: "把这条事项标记为已完成。" },
      { action: "cancel" as const, label: "取消/跳过", tooltip: "取消或跳过本次事项。" },
      { action: "edit" as const, label: "编辑", tooltip: "编辑能力稍后接入。" },
      { action: "open-resource" as const, label: "相关资料", tooltip: "先展示资料入口，暂不直接打开。" },
      ...(item.experience.canConvertToTask ? [{ action: "convert-to-task" as const, label: "转交给 Agent", tooltip: "确认后会直接生成任务并跳转到任务页。" }] : []),
    ];
  }

  if (item.item.bucket === "later") {
    return [
      { action: "move-upcoming" as const, label: "提前到近期", tooltip: "将这条事项提前到近期要做。" },
      { action: "edit" as const, label: "编辑", tooltip: "编辑能力稍后接入。" },
      { action: "cancel" as const, label: "取消", tooltip: "取消这条后续安排。" },
      { action: "open-resource" as const, label: "相关资料", tooltip: "先展示资料入口，暂不直接打开。" },
      ...(item.experience.canConvertToTask ? [{ action: "convert-to-task" as const, label: "转交给 Agent", tooltip: "确认后会直接生成任务并跳转到任务页。" }] : []),
    ];
  }

  if (item.item.bucket === "recurring_rule") {
    return [
      { action: "toggle-recurring" as const, label: item.experience.isRecurringEnabled ? "暂停重复" : "开启重复", tooltip: "开关重复规则的真实动作稍后接入。" },
      { action: "edit" as const, label: "修改规则", tooltip: "规则编辑能力稍后接入。" },
      { action: "cancel-recurring" as const, label: "取消规则", tooltip: "取消整个重复事项。" },
      { action: "open-resource" as const, label: "相关资料", tooltip: "先展示资料入口，暂不直接打开。" },
    ];
  }

  return [
    { action: "restore" as const, label: "恢复未完成", tooltip: "把这条事项恢复到未完成列表。" },
    { action: "delete" as const, label: "删除记录", tooltip: "删除记录能力稍后接入。" },
    { action: "open-resource" as const, label: "相关资料", tooltip: "先展示资料入口，暂不直接打开。" },
    ...(item.experience.canConvertToTask ? [{ action: "convert-to-task" as const, label: "转交给 Agent", tooltip: "确认后会直接生成任务并跳转到任务页。" }] : []),
  ];
}

export function NoteActionBar({ item, onAction }: NoteActionBarProps) {
  const actions = getActions(item);

  return (
    <div className="note-detail-actions">
      {actions.map((action) => {
        const Icon = actionIcons[action.action];

        return (
          <Tooltip key={action.label}>
            <TooltipTrigger>
              <Button className="note-detail-actions__button" onClick={() => onAction(action.action)} variant="ghost">
                <Icon className="h-4 w-4" />
                {action.label}
              </Button>
            </TooltipTrigger>
            <TooltipContent className="rounded-full bg-slate-900/90 px-3 py-1.5 text-[0.72rem] text-white">{action.tooltip}</TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
