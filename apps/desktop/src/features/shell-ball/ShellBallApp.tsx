import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";
import { ShellBallDemoSwitcher } from "./components/ShellBallDemoSwitcher";
import { ShellBallInputBar } from "./components/ShellBallInputBar";
import { ShellBallMascot } from "./components/ShellBallMascot";
import "./shellBall.css";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";

export function ShellBallApp() {
  const {
    visualState,
    inputValue,
    setInputValue,
    inputBarMode,
    handlePrimaryClick,
    handleRegionEnter,
    handleRegionLeave,
    handleSubmitText,
    handleAttachFile,
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handleForceState,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);

  return (
    <main className="app-shell shell-ball-page">
      <div className="shell-ball-page__frame">
        <section className="shell-ball-page__hero">
          <div className="shell-ball-page__hero-copy">
            <p className="shell-ball-page__eyebrow">shell-ball phase 1</p>
            <div className="shell-ball-page__headline-copy">
              <h1 className="shell-ball-page__title">小胖啾近场承接</h1>
              <p className="shell-ball-page__lede">同一只鸟球根据 7 个冻结展示态切换姿态、节奏和承接区，保持 demo-only 第一阶段边界。</p>
            </div>
          </div>

          <div className="shell-ball-page__interaction-stack">
            <ShellBallBubbleZone visualState={visualState} />
            <div className="shell-ball-page__interaction-region" onPointerEnter={handleRegionEnter} onPointerLeave={handleRegionLeave}>
              <div className="shell-ball-page__interaction-core">
                <div className="shell-ball-page__mascot-shell">
                  <ShellBallMascot
                    visualState={visualState}
                    motionConfig={motionConfig}
                    onPrimaryClick={handlePrimaryClick}
                    onPressStart={(event) => handlePressStart(event.clientY)}
                    onPressMove={(event) => handlePressMove(event.clientY)}
                    onPressEnd={handlePressEnd}
                  />
                </div>
                <ShellBallInputBar
                  mode={inputBarMode}
                  value={inputValue}
                  onValueChange={setInputValue}
                  onAttachFile={handleAttachFile}
                  onSubmit={handleSubmitText}
                />
              </div>
            </div>
          </div>
        </section>

        <div className="shell-ball-page__switcher-shell">
          <ShellBallDemoSwitcher value={visualState} onChange={handleForceState} />
        </div>
      </div>
    </main>
  );
}
