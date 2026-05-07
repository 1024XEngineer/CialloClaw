import { ShellBallMascot } from "@/features/shell-ball/components/ShellBallMascot";
import { getShellBallMotionConfig } from "@/features/shell-ball/shellBall.motion";
import type { ShellBallMotionConfig, ShellBallVisualState } from "@/features/shell-ball/shellBall.types";
import "@/features/shell-ball/shellBall.css";

type DecorativeBird = {
  key: string;
  role: "lookout" | "dozer" | "tinkerer";
  cornerClassName: string;
  visualState: ShellBallVisualState;
  motionConfig: ShellBallMotionConfig;
};

const noop = () => {};
const noopPressEnd = () => false;

function createDecorativeMotionConfig(
  visualState: ShellBallVisualState,
  overrides: Partial<ShellBallMotionConfig>,
): ShellBallMotionConfig {
  return {
    ...getShellBallMotionConfig(visualState),
    ringMode: "hidden",
    showAuthMarker: false,
    ...overrides,
  };
}

const DECORATIVE_BIRDS: DecorativeBird[] = [
  {
    key: "top-right-lookout",
    role: "lookout",
    cornerClassName: "mirror-page__decor-bird--top-right",
    visualState: "hover_input",
    motionConfig: createDecorativeMotionConfig("hover_input", {
      accentTone: "sky",
      eyeMode: "curious",
      wingMode: "lift",
      bodyScale: 0.88,
      bodyTiltDeg: -12,
      floatOffsetY: -14,
      floatDurationMs: 3200,
      breatheScale: 1.022,
      breatheDurationMs: 2800,
      wingLiftDeg: 15,
      wingSpreadPx: 7,
      wingDurationMs: 1700,
      tailSwingDeg: 9,
      tailDurationMs: 2200,
      crestLiftPx: 4,
      blinkDelayMs: 120,
    }),
  },
  {
    key: "bottom-right-dozer",
    role: "dozer",
    cornerClassName: "mirror-page__decor-bird--bottom-right",
    visualState: "idle",
    motionConfig: createDecorativeMotionConfig("idle", {
      accentTone: "amber",
      eyeMode: "careful",
      wingMode: "tucked",
      bodyScale: 1.02,
      bodyTiltDeg: 10,
      floatOffsetY: -5,
      floatDurationMs: 7200,
      breatheScale: 1.016,
      breatheDurationMs: 5600,
      wingLiftDeg: 2,
      wingSpreadPx: 1,
      wingDurationMs: 3600,
      tailSwingDeg: 3,
      tailDurationMs: 5200,
      crestLiftPx: 0,
      blinkDelayMs: 920,
    }),
  },
  {
    key: "bottom-left-tinkerer",
    role: "tinkerer",
    cornerClassName: "mirror-page__decor-bird--bottom-left",
    visualState: "processing",
    motionConfig: createDecorativeMotionConfig("processing", {
      accentTone: "teal",
      eyeMode: "focus",
      wingMode: "flutter",
      bodyScale: 0.88,
      bodyTiltDeg: 13,
      floatOffsetY: -7,
      floatDurationMs: 1900,
      breatheScale: 1.028,
      breatheDurationMs: 1450,
      wingLiftDeg: 24,
      wingSpreadPx: 10,
      wingDurationMs: 560,
      tailSwingDeg: 22,
      tailDurationMs: 980,
      crestLiftPx: 7,
      blinkDelayMs: 40,
    }),
  },
];

export function MirrorDecorativeBirds() {
  return (
    <div className="mirror-page__decor-birds" aria-hidden="true">
      {DECORATIVE_BIRDS.map((bird) => (
        <div key={bird.key} className={`mirror-page__decor-bird ${bird.cornerClassName}`} data-role={bird.role}>
          <div className="mirror-page__decor-bird-shell">
            <ShellBallMascot
              floatingBallSize="large"
              visualState={bird.visualState}
              voicePreview={null}
              motionConfig={bird.motionConfig}
              onPrimaryClick={noop}
              onPressStart={noop}
              onPressMove={noop}
              onPressEnd={noopPressEnd}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
