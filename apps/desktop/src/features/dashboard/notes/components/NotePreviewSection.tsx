import type { ReactNode } from "react";
import { cn } from "@/utils/cn";
import type { NoteListItem } from "../notePage.types";
import { NotePreviewCard } from "./NotePreviewCard";

type NotePreviewSectionProps = {
  activeItemId: string | null;
  draggableToCanvas?: boolean;
  emptyLabel?: string;
  errorMessage?: string | null;
  isExpanded: boolean;
  items: NoteListItem[];
  onCanvasDragEnd?: (itemId: string, event: PointerEvent) => void;
  onCanvasDragMove?: (itemId: string, event: PointerEvent) => void;
  onCanvasDragStart?: (
    item: NoteListItem,
    dragSeed: {
      height: number;
      offsetX: number;
      offsetY: number;
      pointerId: number;
      startX: number;
      startY: number;
      width: number;
    },
  ) => void;
  onSelect: (itemId: string) => void;
  onToggle: () => void;
  title: string;
  trailing?: ReactNode;
  variant?: "default" | "hint";
};

export function NotePreviewSection({ activeItemId, draggableToCanvas = false, emptyLabel = "无", errorMessage, isExpanded, items, onCanvasDragEnd, onCanvasDragMove, onCanvasDragStart, onSelect, onToggle, title, trailing, variant = "default" }: NotePreviewSectionProps) {
  return (
    <article className={cn("dashboard-card note-preview-shell", variant === "hint" && "note-preview-shell--hint", isExpanded ? "is-expanded" : "is-collapsed")}>
      <button aria-expanded={isExpanded} className="note-preview-shell__bucket-toggle" onClick={onToggle} type="button">
        <p className="dashboard-card__kicker">{title}</p>
        {trailing}
      </button>

      {isExpanded ? (
        <div className="note-preview-shell__bucket-body">
          <div className="note-preview-shell__list">
            {errorMessage ? (
              <div className="note-preview-shell__empty note-preview-shell__empty--error">{errorMessage}</div>
            ) : items.length > 0 ? (
              items.map((item) => (
                <NotePreviewCard
                  key={item.item.item_id}
                  draggableToCanvas={draggableToCanvas}
                  isActive={item.item.item_id === activeItemId}
                  item={item}
                  onCanvasDragEnd={onCanvasDragEnd}
                  onCanvasDragMove={onCanvasDragMove}
                  onCanvasDragStart={onCanvasDragStart}
                  onSelect={onSelect}
                />
              ))
            ) : (
              <div className="note-preview-shell__empty">{emptyLabel}</div>
            )}
          </div>
        </div>
      ) : null}
    </article>
  );
}
