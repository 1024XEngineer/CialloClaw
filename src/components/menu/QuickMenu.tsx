import type { FloatingOrbEdge } from '../orb/FloatingOrb'
import styles from './QuickMenu.module.css'

type QuickMenuItem = {
  id: string
  label: string
  description: string
  icon: string
  disabled?: boolean
}

export type QuickMenuProps = {
  anchorTop: number
  edge: FloatingOrbEdge
  items: QuickMenuItem[]
  onAction: (id: string) => void
}

export function QuickMenu({ anchorTop, edge, items, onAction }: QuickMenuProps) {
  return (
    <div
      aria-label="Assistant quick menu"
      className={styles.menu}
      data-edge={edge}
      data-top={String(Math.round(anchorTop))}
      role="menu"
      style={{ top: `${anchorTop}px` }}
    >
      {items.map((item) => (
        <button
          key={item.id}
          aria-disabled={item.disabled ? 'true' : 'false'}
          className={styles.menuItem}
          role="menuitem"
          type="button"
          onClick={() => {
            if (!item.disabled) {
              onAction(item.id)
            }
          }}
        >
          <span aria-hidden="true" className={styles.icon} data-testid={`quick-menu-icon-${item.id}`}>
            {item.icon}
          </span>
          <span className={styles.copy}>
            <span className={styles.label}>{item.label}</span>
            <span className={styles.description}>{item.description}</span>
          </span>
        </button>
      ))}
    </div>
  )
}
