import type { ActionKind, SelectionRect } from '../../types/prototype'
import styles from './ActionMenu.module.css'

const actions: { kind: ActionKind; label: string; primary?: boolean }[] = [
  { kind: 'analyze-content', label: 'Analyze Content', primary: true },
  { kind: 'recognize-text', label: 'Recognize Text' },
  { kind: 'explain-chart', label: 'Explain Chart' },
  { kind: 'analyze-error', label: 'Analyze Error' },
  { kind: 'summarize-page', label: 'Summarize Page' },
  { kind: 'generate-reply', label: 'Generate Reply' },
]

type ActionMenuProps = {
  selection: SelectionRect
  onAction: (actionKind: ActionKind) => void
  onReselect: () => void
  onPinToSidePanel: () => void
}

function getAnchorStyle(selection: SelectionRect) {
  return {
    left: `${selection.x + Math.max(selection.width - 300, 16)}px`,
    top: `${selection.y + selection.height + 18}px`,
  }
}

export function ActionMenu({ selection, onAction, onReselect, onPinToSidePanel }: ActionMenuProps) {
  return (
    <div
      aria-label="Selection actions"
      className={styles.menu}
      role="toolbar"
      style={getAnchorStyle(selection)}
    >
      {actions.map((action) => (
        <button
          key={action.kind}
          className={action.primary ? styles.primary : styles.action}
          data-action-kind={action.kind}
          data-primary-action={action.primary ? 'true' : undefined}
          type="button"
          onClick={() => onAction(action.kind)}
        >
          {action.label}
        </button>
      ))}
      <button className={styles.utility} type="button" onClick={onReselect}>
        Reselect
      </button>
      <button className={styles.utility} type="button" onClick={onPinToSidePanel}>
        Pin to Side Panel
      </button>
    </div>
  )
}
