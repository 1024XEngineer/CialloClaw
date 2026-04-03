import type { ContentHint, SelectionRect } from '../../types/prototype'
import styles from './SelectionBox.module.css'

const hintLabels: Record<ContentHint, string> = {
  content: 'Content detected',
  error: 'Code / error likely',
  text: 'Text-heavy',
  chart: 'Chart detected',
  page: 'Page layout',
  reply: 'Reply candidate',
}

type SelectionBoxProps = {
  selection: SelectionRect
  hint: ContentHint | null
  fullScreen?: boolean
}

export function SelectionBox({ selection, hint, fullScreen = false }: SelectionBoxProps) {
  return (
    <div
      className={styles.selectionBox}
      data-fullscreen={fullScreen ? 'true' : 'false'}
      data-testid="selection-box"
      style={{
        left: `${selection.x}px`,
        top: `${selection.y}px`,
        width: `${selection.width}px`,
        height: `${selection.height}px`,
      }}
    >
      <div className={styles.meta}>
        <span className={styles.dimensions}>{`${selection.width} x ${selection.height}`}</span>
        {hint ? <span className={styles.hint}>{hintLabels[hint]}</span> : null}
      </div>
      <span className={styles.corner} data-corner="top-left" />
      <span className={styles.corner} data-corner="top-right" />
      <span className={styles.corner} data-corner="bottom-left" />
      <span className={styles.corner} data-corner="bottom-right" />
    </div>
  )
}
