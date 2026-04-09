import { ShellBallDemoSwitcher } from "./components/ShellBallDemoSwitcher";
import { ShellBallMascot } from "./components/ShellBallMascot";
import { ShellBallPanel } from "./components/ShellBallPanel";
import { getShellBallDemoViewModel } from "./shellBall.demo";
import { getShellBallMotionConfig } from "./shellBall.motion";
import "./shellBall.css";
import { useShellBallStore } from "../../stores/shellBallStore";
import { cn } from "../../utils/cn";

export function ShellBallApp() {
  const visualState = useShellBallStore((state) => state.visualState);
  const setVisualState = useShellBallStore((state) => state.setVisualState);
  const viewModel = getShellBallDemoViewModel(visualState);
  const motionConfig = getShellBallMotionConfig(visualState);

  return (
    <main className="app-shell shell-ball-page">
      <div className="shell-ball-page__frame">
        <section className="shell-ball-page__hero">
          <div className="shell-ball-page__hero-copy">
            <p className="shell-ball-page__eyebrow">shell-ball phase 1</p>
            <div className="shell-ball-page__headline-copy">
              <h1 className="shell-ball-page__title">小胖啾近场承接</h1>
              <p className="shell-ball-page__lede">同一只鸟球根据 7 个冻结展示态切换姿态、节奏和承接面板，保持 demo-only 第一阶段边界。</p>
            </div>
          </div>

          <div className={cn("shell-ball-page__stage", viewModel.panelMode === "hidden" && "shell-ball-page__stage--solo")}>
            <div className="shell-ball-page__mascot-shell">
              <ShellBallMascot visualState={visualState} motionConfig={motionConfig} />
            </div>
            {viewModel.panelMode === "hidden" ? null : (
              <div className="shell-ball-page__panel-shell">
                <ShellBallPanel viewModel={viewModel} accentTone={motionConfig.accentTone} />
              </div>
            )}
          </div>
        </section>

        <ShellBallDemoSwitcher value={visualState} onChange={setVisualState} />
      </div>
    </main>
  );
}
