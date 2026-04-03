import type { ContentHint, SelectionRect } from '../../types/prototype'
import { SelectionBox } from './SelectionBox'
import styles from './CaptureOverlay.module.css'

type CaptureOverlayProps = {
  selection: SelectionRect | null
  hint: ContentHint | null
  fullScreen: boolean
  warning: string | null
}

export function CaptureOverlay({ selection, hint, fullScreen, warning }: CaptureOverlayProps) {
  return (
    <div className={styles.overlay}>
      <div className={styles.instructions}>Drag to select · Esc cancel · Space capture full screen</div>
      {selection ? <SelectionBox fullScreen={fullScreen} hint={hint} selection={selection} /> : null}
      {warning ? (
        <div aria-live="polite" className={styles.warning} role="status">
          {warning}
        </div>
      ) : null}
    </div>
  )
}
