import { SecurityApp } from "./SecurityApp";

/**
 * Keeps the safety route aligned with the shared dashboard shell while
 * delegating all visual treatment to the page-level security styles.
 */
export function SecurityPageShell() {
  return <SecurityApp />;
}
