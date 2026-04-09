import { cn } from "../../../utils/cn";
import { getShellBallDemoViewModel } from "../shellBall.demo";
import { shellBallVisualStates, type ShellBallVisualState } from "../shellBall.types";

type ShellBallDemoSwitcherProps = {
  value: ShellBallVisualState;
  onChange: (value: ShellBallVisualState) => void;
};

export function ShellBallDemoSwitcher({ value, onChange }: ShellBallDemoSwitcherProps) {
  return (
    <section className="shell-ball-switcher" aria-label="Shell-ball demo switcher">
      <div className="shell-ball-switcher__header">
        <div>
          <p className="shell-ball-switcher__eyebrow">demo state switcher</p>
          <h2 className="shell-ball-switcher__title">前端演示切换器</h2>
        </div>
        <p className="shell-ball-switcher__note">只切换冻结的 7 个展示态，不读取真实 task 数据。</p>
      </div>

      <div className="shell-ball-switcher__grid">
        {shellBallVisualStates.map((state) => {
          const option = getShellBallDemoViewModel(state);
          const isActive = state === value;

          return (
            <button
              key={state}
              type="button"
              className={cn("shell-ball-switcher__button", isActive && "shell-ball-switcher__button--active")}
              onClick={() => onChange(state)}
            >
              <span className="shell-ball-switcher__button-badge">{option.badgeLabel}</span>
              <span className="shell-ball-switcher__button-title">{option.title}</span>
              <span className="shell-ball-switcher__button-state">{state}</span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
