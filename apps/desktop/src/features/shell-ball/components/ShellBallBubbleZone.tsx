import type { ShellBallVisualState } from "../shellBall.types";

type ShellBallBubbleZoneProps = {
  visualState: ShellBallVisualState;
};

export function ShellBallBubbleZone({ visualState }: ShellBallBubbleZoneProps) {
  return (
    <section className="shell-ball-bubble-zone" data-state={visualState} aria-hidden="true">
      <div className="shell-ball-bubble-zone__shell" />
      <div className="shell-ball-bubble-zone__shell shell-ball-bubble-zone__shell--secondary" />
    </section>
  );
}
