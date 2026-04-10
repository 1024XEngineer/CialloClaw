import type { ReactNode } from "react";
import { cn } from "@/utils/cn";
import type { NoteListItem } from "../notePage.types";
import { NotePreviewCard } from "./NotePreviewCard";

type NotePreviewSectionProps = {
  activeItemId: string | null;
  description: string;
  items: NoteListItem[];
  onSelect: (itemId: string) => void;
  title: string;
  trailing?: ReactNode;
  variant?: "default" | "hint";
};

export function NotePreviewSection({ activeItemId, description, items, onSelect, title, trailing, variant = "default" }: NotePreviewSectionProps) {
  return (
    <article className={cn("dashboard-card note-preview-shell", variant === "hint" && "note-preview-shell--hint")}>
      <div className="note-preview-shell__header">
        <div>
          <p className="dashboard-card__kicker">{title}</p>
          <p className="note-preview-shell__description">{description}</p>
        </div>
        {trailing}
      </div>

      <div className="note-preview-shell__list">
        {items.map((item) => (
          <NotePreviewCard key={item.item.item_id} isActive={item.item.item_id === activeItemId} item={item} onSelect={onSelect} />
        ))}
      </div>
    </article>
  );
}
