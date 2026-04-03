import { useRef, useState } from 'react'

import type { SelectionRect } from '../../types/prototype'
import styles from './ProcessingCard.module.css'

type ProcessingCardProps = {
  currentStage: string
  selection: SelectionRect
  onCancel: () => void
  onRunInBackground: () => void
}

function supportsPointerCapture(target: EventTarget | null): target is HTMLElement {
  return (
    target instanceof HTMLElement &&
    typeof target.setPointerCapture === 'function' &&
    typeof target.releasePointerCapture === 'function'
  )
}

function getBasePosition(selection: SelectionRect) {
  return {
    left: selection.x + Math.max(selection.width - 220, 24),
    top: selection.y + Math.max(selection.height * 0.15, 12),
  }
}

export function ProcessingCard({ currentStage, selection, onCancel, onRunInBackground }: ProcessingCardProps) {
  const pointerDown = useRef<{ x: number; y: number; offsetX: number; offsetY: number } | null>(null)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [dragging, setDragging] = useState(false)

  const base = getBasePosition(selection)

  function handlePointerDown(event: React.PointerEvent<HTMLElement>) {
    pointerDown.current = {
      x: event.clientX,
      y: event.clientY,
      offsetX: offset.x,
      offsetY: offset.y,
    }

    if (supportsPointerCapture(event.currentTarget)) {
      event.currentTarget.setPointerCapture(event.pointerId)
    }
  }

  function handlePointerMove(event: React.PointerEvent<HTMLElement>) {
    if (pointerDown.current === null) {
      return
    }

    const deltaX = event.clientX - pointerDown.current.x
    const deltaY = event.clientY - pointerDown.current.y

    if (Math.abs(deltaX) > 4 || Math.abs(deltaY) > 4) {
      setDragging(true)
    }

    setOffset({
      x: pointerDown.current.offsetX + deltaX,
      y: pointerDown.current.offsetY + deltaY,
    })
  }

  function handlePointerUp(event: React.PointerEvent<HTMLElement>) {
    pointerDown.current = null
    setDragging(false)

    if (supportsPointerCapture(event.currentTarget)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
  }

  function handlePointerCancel() {
    pointerDown.current = null
    setDragging(false)
  }

  return (
    <section
      className={styles.card}
      data-testid="processing-card"
      style={{
        left: `${base.left}px`,
        top: `${base.top}px`,
        transform: `translate(${offset.x}px, ${offset.y}px)`,
      }}
      onPointerCancel={handlePointerCancel}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
    >
      <header className={styles.dragHandle}>{dragging ? 'Moving processing card' : 'Working on this selection'}</header>
      <p className={styles.stage}>{currentStage}</p>
      <div className={styles.actions}>
        <button className={styles.secondary} type="button" onClick={onCancel}>
          Cancel
        </button>
        <button className={styles.primary} type="button" onClick={onRunInBackground}>
          Run in Background
        </button>
      </div>
    </section>
  )
}
