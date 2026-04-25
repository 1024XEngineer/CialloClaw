import type { ReactNode } from "react";
import { cn } from "@/utils/cn";
import type { NoteListItem, NotePreviewGroupKey } from "../notePage.types";
import { NotePreviewCard } from "./NotePreviewCard";

type NotePreviewSectionProps = {
  activeItemId: string | null;
  bucketKey: NotePreviewGroupKey;
  draggableToCanvas?: boolean;
  emptyLabel?: string;
  errorMessage?: string | null;
  isDropTarget?: boolean;
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
  stackCards?: boolean;
  title: string;
  trailing?: ReactNode;
  variant?: "default" | "hint";
};

/**
 * Renders one drawer bucket and highlights the matching group when a board
 * card is dragged back toward the sidebar.
 */
export function NotePreviewSection({
  activeItemId,
  bucketKey,
  draggableToCanvas = false,
  emptyLabel = "暂无便签",
  errorMessage,
  isDropTarget = false,
  isExpanded,
  items,
  onCanvasDragEnd,
  onCanvasDragMove,
  onCanvasDragStart,
  onSelect,
  onToggle,
  stackCards = false,
  title,
  trailing,
  variant = "default",
}: NotePreviewSectionProps) {
  return (
    <article
      className={cn(
        "dashboard-card note-preview-shell",
        variant === "hint" && "note-preview-shell--hint",
        isDropTarget && "is-drop-target",
        isExpanded ? "is-expanded" : "is-collapsed",
      )}
      data-note-bucket={bucketKey}
    >
      <button aria-expanded={isExpanded} className="note-preview-shell__bucket-toggle" onClick={onToggle} type="button">
        <p className="dashboard-card__kicker">{title}</p>
        {trailing}
      </button>

      {isExpanded ? (
        <div className="note-preview-shell__bucket-body">
          <div className={cn("note-preview-shell__list", stackCards && items.length > 1 && "note-preview-shell__list--stacked")}>
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
