import type { ComponentType } from "react";
import {
  ArrowUpRight,
  CalendarClock,
  CheckCircle2,
  Clock3,
  Pencil,
  Repeat,
  RotateCcw,
  Sparkles,
  Trash2,
  XCircle,
} from "lucide-react";
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
  "convert-to-task": Sparkles,
  delete: Trash2,
  edit: Pencil,
  "move-upcoming": CalendarClock,
  "open-resource": ArrowUpRight,
  restore: RotateCcw,
  "skip-once": Clock3,
  "toggle-recurring": Repeat,
  "view-task": ArrowUpRight,
};

function getActions(item: NoteListItem) {
  if (item.item.bucket === "upcoming") {
    return [
      { action: "complete" as const, label: "标记完成", tooltip: "把这条便签结束并归入已结束。" },
      { action: "cancel" as const, label: "取消", tooltip: "取消这条近期便签。" },
      { action: "open-resource" as const, label: "打开资源", tooltip: "打开这条便签关联的首个资源。" },
      ...(item.item.linked_task_id ? [{ action: "view-task" as const, label: "查看任务", tooltip: "打开这条便签对应的关联任务。" }] : []),
      ...(item.experience.canConvertToTask ? [{ action: "convert-to-task" as const, label: "转成任务", tooltip: "基于当前便签生成关联任务。" }] : []),
    ];
  }

  if (item.item.bucket === "later") {
    return [
      { action: "move-upcoming" as const, label: "提前到近期", tooltip: "把这条后续便签移到近期分组。" },
      { action: "open-resource" as const, label: "打开资源", tooltip: "打开这条便签关联的首个资源。" },
      ...(item.item.linked_task_id ? [{ action: "view-task" as const, label: "查看任务", tooltip: "打开这条便签对应的关联任务。" }] : []),
      ...(item.experience.canConvertToTask ? [{ action: "convert-to-task" as const, label: "转成任务", tooltip: "基于当前便签生成关联任务。" }] : []),
    ];
  }

  if (item.item.bucket === "recurring_rule") {
    return [
      { action: "toggle-recurring" as const, label: item.experience.isRecurringEnabled ? "暂停重复" : "开启重复", tooltip: "切换这条重复规则的状态。" },
      { action: "cancel-recurring" as const, label: "取消规则", tooltip: "结束整条重复规则。" },
      { action: "open-resource" as const, label: "打开资源", tooltip: "打开这条便签关联的首个资源。" },
    ];
  }

  return [
    { action: "restore" as const, label: "恢复到近期", tooltip: "把这条记录恢复到近期分组。" },
    { action: "delete" as const, label: "删除记录", tooltip: "删除这条便签记录。" },
    { action: "open-resource" as const, label: "打开资源", tooltip: "打开这条便签关联的首个资源。" },
    ...(item.item.linked_task_id ? [{ action: "view-task" as const, label: "查看任务", tooltip: "打开这条便签对应的关联任务。" }] : []),
    ...(item.experience.canConvertToTask ? [{ action: "convert-to-task" as const, label: "转成任务", tooltip: "基于当前便签生成关联任务。" }] : []),
  ];
}

/**
 * Renders the legacy note detail action row for modal-based note surfaces.
 *
 * @param props Component props.
 * @returns The action bar element.
 */
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
