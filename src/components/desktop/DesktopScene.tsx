import type { PrototypeState } from '../../types/prototype'
import { QuickMenu } from '../menu/QuickMenu'
import { FloatingOrb, type FloatingOrbPosition } from '../orb/FloatingOrb'
import styles from './DesktopScene.module.css'

const menuItems = [
  {
    id: 'region-analyze',
    label: 'Region Analyze',
    icon: '[]',
    description: 'Select an area and let AI inspect it',
  },
  {
    id: 'capture-text',
    label: 'Capture Text',
    icon: 'T',
    description: 'Select a text block for OCR-style output',
  },
  {
    id: 'resume-current-task',
    label: 'Resume Current Task',
    icon: '>',
    description: 'Reopen the task running in the background',
  },
  {
    id: 'recent-result',
    label: 'Recent Result',
    icon: 'R',
    description: 'Return to the latest completed task',
  },
  {
    id: 'open-side-panel',
    label: 'Open Side Panel',
    icon: '||',
    description: 'Pin the current context to the right panel',
  },
] as const

type DesktopSceneProps = {
  state: PrototypeState
  orbPosition: FloatingOrbPosition
  onOpenQuickMenu: () => void
  onQuickMenuAction: (id: string) => void
  onOrbDragEnd: (position: FloatingOrbPosition) => void
}

function getQuickMenuTop(orbTop: number) {
  return Math.max(24, orbTop - 108)
}

export function DesktopScene({
  state,
  orbPosition,
  onOpenQuickMenu,
  onQuickMenuAction,
  onOrbDragEnd,
}: DesktopSceneProps) {
  const quickMenuItems = menuItems
    .filter((item) => item.id !== 'resume-current-task' || state.backgroundTask !== null)
    .map((item) => ({
      ...item,
      description:
        item.id === 'resume-current-task' && state.backgroundTask?.status === 'complete'
          ? 'Restore the latest completed task'
          : item.description,
      disabled:
        (item.id === 'recent-result' && state.lastCompletedAt === null) ||
        (item.id === 'resume-current-task' && state.backgroundTask === null),
    }))

  return (
    <section aria-label="Desktop scene" className={styles.scene} data-testid="desktop-scene">
      <div className={styles.wallpaper} />
      <div className={styles.workspace}>
        <header className={styles.header}>
          <span className={styles.eyebrow}>Desktop overlay prototype</span>
          <h1>Desktop AI Assistant Prototype</h1>
          <p>Screen-aware capture flows stay close to the desktop instead of opening a chat workspace.</p>
        </header>
        <div className={styles.windowRow}>
          <article className={styles.window}>
            <span className={styles.windowLabel}>Code review</span>
            <h2>Release blocker notes</h2>
            <p>Highlight any chart, text block, or error surface to trigger the assistant overlay.</p>
          </article>
          <article className={styles.windowAlt}>
            <span className={styles.windowLabel}>Knowledge base</span>
            <h2>Architecture recap</h2>
            <p>Keep the quick menu lightweight and make region selection the main entry path.</p>
          </article>
        </div>
      </div>

      {state.quickMenuOpen ? (
        <QuickMenu
          anchorTop={getQuickMenuTop(orbPosition.top)}
          edge={orbPosition.edge}
          items={quickMenuItems}
          onAction={onQuickMenuAction}
        />
      ) : null}

      {state.sidePanelOpen ? (
        <aside aria-label="Analysis details" className={styles.sidePanel} role="complementary">
          <h2>Analysis details</h2>
          <p>Select an area to begin analysis.</p>
        </aside>
      ) : null}

      <FloatingOrb
        onDragEnd={onOrbDragEnd}
        onOpen={onOpenQuickMenu}
        position={orbPosition}
        statusLabel={state.orbTaskChip}
      />
    </section>
  )
}
