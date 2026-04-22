import { useEffect, useRef } from "react";
import type { MouseEvent, PointerEvent as ReactPointerEvent } from "react";
import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/utils/cn";
import { describeNotePreview, getNoteStatusBadgeClass } from "../notePage.mapper";
import type { NoteListItem } from "../notePage.types";

type NotePreviewCardProps = {
  draggableToCanvas?: boolean;
  isActive: boolean;
  item: NoteListItem;
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
};

export function NotePreviewCard({ draggableToCanvas = false, isActive, item, onCanvasDragEnd, onCanvasDragMove, onCanvasDragStart, onSelect }: NotePreviewCardProps) {
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const cleanupPointerListenersRef = useRef<(() => void) | null>(null);
  const suppressClickRef = useRef(false);
  const pointerDragRef = useRef<{ dragging: boolean; pointerId: number | null; startX: number; startY: number } | null>(null);

  useEffect(() => {
    return () => {
      cleanupPointerListenersRef.current?.();
      suppressClickRef.current = false;
      pointerDragRef.current = null;
    };
  }, []);

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    if (suppressClickRef.current) {
      event.preventDefault();
      suppressClickRef.current = false;
      return;
    }

    onSelect(item.item.item_id);
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLButtonElement>) {
    if (!draggableToCanvas || !event.isPrimary || event.button !== 0) {
      return;
    }

    cleanupPointerListenersRef.current?.();
    pointerDragRef.current = {
      dragging: false,
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
    };

    const handleWindowPointerMove = (windowEvent: PointerEvent) => {
      const dragState = pointerDragRef.current;
      if (!dragState || dragState.pointerId !== windowEvent.pointerId) {
        return;
      }

      const movedEnough = Math.hypot(windowEvent.clientX - dragState.startX, windowEvent.clientY - dragState.startY) > 4;
      if (!dragState.dragging && movedEnough) {
        dragState.dragging = true;
        suppressClickRef.current = true;
        buttonRef.current?.classList.add("is-dragging");
        const rect = buttonRef.current?.getBoundingClientRect();
        if (rect) {
          onCanvasDragStart?.(item, {
            height: rect.height,
            offsetX: windowEvent.clientX - rect.left,
            offsetY: windowEvent.clientY - rect.top,
            pointerId: windowEvent.pointerId,
            startX: windowEvent.clientX,
            startY: windowEvent.clientY,
            width: rect.width,
          });
        }
      }

      if (dragState.dragging) {
        onCanvasDragMove?.(item.item.item_id, windowEvent);
      }
    };

    const finishWindowDrag = (windowEvent: PointerEvent) => {
      const dragState = pointerDragRef.current;
      if (!dragState || dragState.pointerId !== windowEvent.pointerId) {
        return;
      }

      if (dragState.dragging) {
        onCanvasDragEnd?.(item.item.item_id, windowEvent);
        buttonRef.current?.classList.remove("is-dragging");
        window.setTimeout(() => {
          suppressClickRef.current = false;
        }, 0);
      }

      pointerDragRef.current = null;
      cleanupPointerListenersRef.current?.();
      cleanupPointerListenersRef.current = null;
    };

    const cleanup = () => {
      window.removeEventListener("pointermove", handleWindowPointerMove);
      window.removeEventListener("pointerup", finishWindowDrag);
      window.removeEventListener("pointercancel", finishWindowDrag);
    };

    cleanupPointerListenersRef.current = cleanup;
    window.addEventListener("pointermove", handleWindowPointerMove);
    window.addEventListener("pointerup", finishWindowDrag);
    window.addEventListener("pointercancel", finishWindowDrag);
  }

  return (
    <motion.button
      ref={buttonRef}
      className={cn("note-preview-card", draggableToCanvas && "note-preview-card--draggable", isActive && "note-preview-card--active", item.item.bucket === "closed" && "note-preview-card--closed")}
      onClick={handleClick}
      onPointerDown={handlePointerDown}
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
