import { useRef, useState } from 'react'

import styles from './FloatingOrb.module.css'

export type FloatingOrbEdge = 'left' | 'right'

export type FloatingOrbPosition = {
  edge: FloatingOrbEdge
  top: number
}

const ORB_MARGIN = 32
const ORB_SIZE = 56
const MIN_TOP = 96

function supportsPointerCapture(target: EventTarget | null): target is HTMLButtonElement {
  return (
    target instanceof HTMLButtonElement &&
    typeof target.setPointerCapture === 'function' &&
    typeof target.releasePointerCapture === 'function'
  )
}

function clampTop(top: number) {
  const maxTop = Math.max(MIN_TOP, window.innerHeight - ORB_SIZE - ORB_MARGIN)

  return Math.min(Math.max(top, MIN_TOP), maxTop)
}

export type FloatingOrbProps = {
  onOpen: () => void
  statusLabel?: string | null
  position: FloatingOrbPosition
  onDragEnd?: (position: FloatingOrbPosition) => void
}

export function FloatingOrb({ onOpen, statusLabel, position, onDragEnd }: FloatingOrbProps) {
  const pointerDown = useRef<{ x: number; y: number; position: FloatingOrbPosition } | null>(null)
  const [dragging, setDragging] = useState(false)
  const [activePosition, setActivePosition] = useState<FloatingOrbPosition | null>(null)
  const [showTooltip, setShowTooltip] = useState(false)

  const resolvedPosition = activePosition ?? position

  function handlePointerDown(event: React.PointerEvent<HTMLButtonElement>) {
    pointerDown.current = { x: event.clientX, y: event.clientY, position }
    setDragging(false)
    setShowTooltip(false)

    if (supportsPointerCapture(event.currentTarget)) {
      event.currentTarget.setPointerCapture(event.pointerId)
    }
  }

  function handlePointerMove(event: React.PointerEvent<HTMLButtonElement>) {
    if (pointerDown.current === null) {
      return
    }

    const deltaX = event.clientX - pointerDown.current.x
    const deltaY = event.clientY - pointerDown.current.y

    if (Math.abs(deltaX) > 4 || Math.abs(deltaY) > 4) {
      setDragging(true)
    }

    const nextTop = clampTop(pointerDown.current.position.top + deltaY)
    const nextEdge: FloatingOrbEdge = event.clientX >= window.innerWidth / 2 ? 'right' : 'left'

    setActivePosition({ edge: nextEdge, top: nextTop })
  }

  function handlePointerUp(event: React.PointerEvent<HTMLButtonElement>) {
    if (pointerDown.current !== null && dragging && activePosition !== null) {
      onDragEnd?.(activePosition)
    }

    pointerDown.current = null
    setActivePosition(null)
    setDragging(false)

    if (supportsPointerCapture(event.currentTarget)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
  }

  function handlePointerCancel() {
    pointerDown.current = null
    setActivePosition(null)
    setDragging(false)
  }

  function handleClick() {
    if (!dragging) {
      onOpen()
    }
  }

  return (
    <div className={styles.orbWrap} data-edge={resolvedPosition.edge} style={{ top: `${resolvedPosition.top}px` }}>
      <button
        aria-label="Open AI assistant"
        className={styles.orb}
        data-edge={resolvedPosition.edge}
        data-top={String(Math.round(resolvedPosition.top))}
        type="button"
        onClick={handleClick}
        onBlur={() => setShowTooltip(false)}
        onFocus={() => setShowTooltip(true)}
        onPointerCancel={handlePointerCancel}
        onPointerDown={handlePointerDown}
        onPointerEnter={() => setShowTooltip(true)}
        onPointerLeave={() => setShowTooltip(false)}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
      >
        <span>AI</span>
        {statusLabel ? (
          <span className={styles.badge} data-testid="orb-task-chip">
            {statusLabel}
          </span>
        ) : null}
      </button>
      {showTooltip ? <span className={styles.tooltip}>Select an area for AI analysis</span> : null}
    </div>
  )
}
