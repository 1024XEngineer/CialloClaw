import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/utils/cn";
import { describeNotePreview, getNoteStatusBadgeClass } from "../notePage.mapper";
import type { NoteListItem } from "../notePage.types";

type NotePreviewCardProps = {
  isActive: boolean;
  item: NoteListItem;
  onSelect: (itemId: string) => void;
};

export function NotePreviewCard({ isActive, item, onSelect }: NotePreviewCardProps) {
  return (
    <motion.button
      className={cn("note-preview-card", isActive && "note-preview-card--active", item.item.bucket === "closed" && "note-preview-card--closed")}
      onClick={() => onSelect(item.item.item_id)}
      type="button"
      whileHover={{ y: -2 }}
      whileTap={{ scale: 0.995 }}
    >
      <div className="note-preview-card__top">
        <div>
          <h3 className="note-preview-card__title">{item.item.title}</h3>
          <p className="note-preview-card__subtitle">{describeNotePreview(item.item, item.experience)}</p>
        </div>
        <Badge className={cn("border-0 px-3 py-1 text-[0.72rem] ring-1", getNoteStatusBadgeClass(item.item.status))}>{item.experience.previewStatus}</Badge>
      </div>
      <div className="note-preview-card__footer">
        <span>{item.experience.typeLabel}</span>
        <span>{item.experience.summaryLabel}</span>
      </div>
      {item.item.agent_suggestion ? <p className="note-preview-card__suggestion">{item.item.agent_suggestion}</p> : null}
    </motion.button>
  );
}
