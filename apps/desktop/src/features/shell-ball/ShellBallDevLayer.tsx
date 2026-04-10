import { ShellBallDemoSwitcher } from "./components/ShellBallDemoSwitcher";
import type { ShellBallVisualState } from "./shellBall.types";

type ShellBallDevLayerProps = {
  value: ShellBallVisualState;
  onChange: (value: ShellBallVisualState) => void;
};

export function ShellBallDevLayer({ value, onChange }: ShellBallDevLayerProps) {
  return (
    <aside className="shell-ball-surface__switcher-shell" aria-label="Shell-ball demo controls">
      <ShellBallDemoSwitcher value={value} onChange={onChange} />
    </aside>
  );
}
