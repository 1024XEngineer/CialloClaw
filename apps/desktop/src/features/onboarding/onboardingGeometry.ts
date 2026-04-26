import { getCurrentWindow, monitorFromPoint } from "@tauri-apps/api/window";
import type { DesktopOnboardingPlacement, DesktopOnboardingPresentation, DesktopOnboardingStep } from "./onboardingService";

export type DesktopOnboardingHighlight = {
  height: number;
  width: number;
  x: number;
  y: number;
};

/**
 * Resolves the current monitor frame in logical coordinates so the dedicated
 * onboarding window can align to the same screen as the active business window.
 *
 * @returns The monitor frame or `null` when the current monitor cannot be read.
 */
export async function resolveDesktopOnboardingMonitorFrame() {
  const currentWindow = getCurrentWindow();
  const outerPosition = await currentWindow.outerPosition();
  const outerSize = await currentWindow.outerSize();
  const monitor = await monitorFromPoint(
    Math.round(outerPosition.x + outerSize.width / 2),
    Math.round(outerPosition.y + outerSize.height / 2),
  );

  if (monitor === null) {
    return null;
  }

  const monitorPosition = monitor.position.toLogical(monitor.scaleFactor);
  const monitorSize = monitor.size.toLogical(monitor.scaleFactor);

  return {
    x: monitorPosition.x,
    y: monitorPosition.y,
    width: monitorSize.width,
    height: monitorSize.height,
  };
}

/**
 * Projects a DOM rect from the current business window into monitor-relative
 * coordinates used to place the onboarding card window near the target.
 *
 * @param rect Client rect measured inside the current business window.
 * @param monitorFrame The logical monitor frame used by the onboarding window.
 * @returns A monitor-relative highlight rectangle.
 */
export async function projectDesktopOnboardingHighlight(rect: DOMRect, monitorFrame: { x: number; y: number }) {
  const currentWindow = getCurrentWindow();
  const scaleFactor = await currentWindow.scaleFactor();
  const outerPosition = await currentWindow.outerPosition();
  const logicalPosition = outerPosition.toLogical(scaleFactor);

  return {
    x: logicalPosition.x - monitorFrame.x + rect.left,
    y: logicalPosition.y - monitorFrame.y + rect.top,
    width: rect.width,
    height: rect.height,
  };
}

/**
 * Chooses a card corner that stays away from the highlighted region so the
 * overlay prompt does not block the user's next real interaction.
 *
 * @param highlight The primary highlighted target.
 * @param monitorFrame The current monitor frame.
 * @returns A suggested floating-card placement.
 */
export function resolveDesktopOnboardingPlacementFromHighlight(
  highlight: DesktopOnboardingHighlight,
  monitorFrame: { height: number; width: number },
): DesktopOnboardingPlacement {
  const centerX = highlight.x + highlight.width / 2;
  const centerY = highlight.y + highlight.height / 2;
  const horizontal = centerX < monitorFrame.width / 2 ? "right" : "left";
  const vertical = centerY < monitorFrame.height / 2 ? "bottom" : "top";

  return `${vertical}-${horizontal}` as DesktopOnboardingPlacement;
}

/**
 * Builds a presentation payload for the current step from one or more business
 * window DOM anchors.
 *
 * @param input Step metadata and DOM anchors for the current window.
 * @returns A monitor-aligned onboarding presentation or `null` when unavailable.
 */
export async function buildDesktopOnboardingPresentation(input: {
  anchors: Array<Element | null | undefined>;
  placement?: DesktopOnboardingPlacement;
  step: DesktopOnboardingStep;
  windowLabel: DesktopOnboardingPresentation["windowLabel"];
}) {
  const monitorFrame = await resolveDesktopOnboardingMonitorFrame();

  if (monitorFrame === null) {
    return null;
  }

  const highlights = await Promise.all(
    input.anchors
      .filter((anchor): anchor is Element => anchor instanceof Element)
      .map((anchor) => projectDesktopOnboardingHighlight(anchor.getBoundingClientRect(), monitorFrame)),
  );

  const primaryHighlight = highlights[0] ?? null;

  return {
    highlights,
    monitorFrame,
    placement:
      input.placement ??
      (primaryHighlight
        ? resolveDesktopOnboardingPlacementFromHighlight(primaryHighlight, monitorFrame)
        : "bottom-right"),
    step: input.step,
    windowLabel: input.windowLabel,
  } satisfies DesktopOnboardingPresentation;
}
