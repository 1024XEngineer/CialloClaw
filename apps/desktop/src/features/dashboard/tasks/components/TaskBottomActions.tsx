import { FilePlus2, FolderPlus, Play, ScissorsLineDashed, Sparkles } from "lucide-react";
import { motion } from "motion/react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { TaskActionShortcut } from "../taskPage.types";

type TaskBottomActionsProps = {
  actions: TaskActionShortcut[];
  onAction: (actionId: string) => void;
};

const actionIcons: Record<string, typeof Play> = {
  attach: FolderPlus,
  continue: Play,
  note: FilePlus2,
  split: ScissorsLineDashed,
  summarize: Sparkles,
};

export function TaskBottomActions({ actions, onAction }: TaskBottomActionsProps) {
  return (
    <div className="task-capsule-bottom flex flex-wrap items-center gap-3">
      {actions.map((action) => {
        const Icon = actionIcons[action.id] ?? Sparkles;

        return (
          <Tooltip key={action.id}>
            <TooltipTrigger>
              <motion.div whileHover={{ y: -3 }} whileTap={{ y: 1 }}>
                <Button className="task-capsule-soft-button h-11 rounded-full px-5 text-sm font-medium" onClick={() => onAction(action.id)} variant="ghost">
                  <Icon className="h-4 w-4" />
                  {action.label}
                </Button>
              </motion.div>
            </TooltipTrigger>
            <TooltipContent className="rounded-full bg-slate-900/90 px-3 py-1.5 text-[0.72rem] text-white">
              {action.tooltip}
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
