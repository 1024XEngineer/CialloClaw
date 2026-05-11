import type { ComponentType } from "react";
import { ArrowUpRight, CalendarClock, CheckCircle2, Clock3, Pencil, Repeat, RotateCcw, Trash2, WandSparkles, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { NoteDetailAction, NoteListItem } from "../notePage.types";

type NoteActionBarProps = {
  item: NoteListItem;
  onAction: (action: NoteDetailAction) => void;
};

type NoteActionDescriptor = {
  action: NoteDetailAction;
  label: string;
  tooltip: string;
};

const actionIcons: Record<NoteDetailAction, ComponentType<{ className?: string }>> = {
  cancel: XCircle,
  "cancel-recurring": XCircle,
  complete: CheckCircle2,
  "convert-to-task": WandSparkles,
  delete: Trash2,
  edit: Pencil,
  "move-upcoming": CalendarClock,
  "open-linked-task": ArrowUpRight,
  "open-resource": ArrowUpRight,
  restore: RotateCcw,
  "skip-once": Clock3,
  "toggle-recurring": Repeat,
};

function createResourceActions(item: NoteListItem): NoteActionDescriptor[] {
  const resourceCount = item.experience.relatedResources.length;
  if (resourceCount === 0) {
    return [];
  }

  if (item.sourceNote?.localOnly) {
    return [
      {
        action: "open-resource",
        label: "打开源文件",
        tooltip: "直接打开这张源便签所在的 markdown 文件。",
      },
    ];
  }

  if (resourceCount === 1) {
    return [
      {
        action: "open-resource",
        label: "打开相关资料",
        tooltip: "直接打开这条便签当前关联的资料入口。",
      },
    ];
  }

  return [
    {
      action: "open-resource",
      label: "查看资料列表",
      tooltip: "在这条便签关联的多份资料里选择要打开的目标。",
    },
  ];
}

function createTaskActions(item: NoteListItem): NoteActionDescriptor[] {
  if (item.item.linked_task_id) {
    return [
      {
        action: "open-linked-task",
        label: "打开关联任务",
        tooltip: "跳转到这条便签已经关联的正式任务详情。",
      },
    ];
  }

  if (!item.experience.canConvertToTask) {
    return [];
  }

  return [
    {
      action: "convert-to-task",
      label: "转交给 Agent",
      tooltip: "会按这条便签生成正式任务并跳转到任务页。",
    },
  ];
}

function getActions(item: NoteListItem): NoteActionDescriptor[] {
  const resourceActions = createResourceActions(item);
  const taskActions = createTaskActions(item);

  if (item.sourceNote?.localOnly) {
    return [
      {
        action: "edit",
        label: "编辑源便签",
        tooltip: "继续修改这条源 markdown 便签的正文内容。",
      },
      ...resourceActions,
    ];
  }

  if (item.item.bucket === "upcoming") {
    return [
      { action: "complete", label: "标记完成", tooltip: "把这条事项标记为已完成。" },
      { action: "cancel", label: "取消/跳过", tooltip: "取消或跳过本次事项。" },
      { action: "edit", label: "编辑源便签", tooltip: "打开源便签编辑器并修改正文内容。" },
      ...resourceActions,
      ...taskActions,
    ];
  }

  if (item.item.bucket === "later") {
    return [
      { action: "move-upcoming", label: "提前到近期", tooltip: "把这条后续安排提前到近期执行。" },
      { action: "edit", label: "编辑源便签", tooltip: "打开源便签编辑器并修改正文内容。" },
      { action: "cancel", label: "取消", tooltip: "取消这条后续安排。" },
      ...resourceActions,
      ...taskActions,
    ];
  }

  if (item.item.bucket === "recurring_rule") {
    return [
      { action: "cancel-recurring", label: "取消规则", tooltip: "取消整条重复规则。" },
      ...resourceActions,
      ...taskActions,
    ];
  }

  return [
    { action: "restore", label: "恢复未完成", tooltip: "把这条事项恢复到未完成列表。" },
    { action: "delete", label: "删除记录", tooltip: "删除这条记录并从便签页移除。" },
    ...resourceActions,
    ...taskActions,
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
