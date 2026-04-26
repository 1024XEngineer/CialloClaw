import type { ReactNode } from "react";
import { Button, Heading, Text } from "@radix-ui/themes";
import "./onboarding.css";

type OnboardingOverlayProps = {
  body: string;
  endLabel?: string;
  footer?: ReactNode;
  onEnd?: () => void;
  onPrimary: () => void;
  onSecondary?: () => void;
  placement?: "modal" | "floating";
  primaryLabel: string;
  secondaryLabel?: string;
  stepLabel?: string;
  title: string;
};

/**
 * Renders the lightweight onboarding card shared by the floating ball,
 * dashboard, and control panel surfaces.
 *
 * @param props Overlay copy and action handlers for the current step.
 * @returns A compact onboarding card with consistent controls.
 */
export function OnboardingOverlay({
  body,
  endLabel = "结束引导",
  footer,
  onEnd,
  onPrimary,
  onSecondary,
  placement = "floating",
  primaryLabel,
  secondaryLabel,
  stepLabel,
  title,
}: OnboardingOverlayProps) {
  return (
    <div className="desktop-onboarding" data-placement={placement}>
      <div className="desktop-onboarding__backdrop" aria-hidden="true" />
      <section className="desktop-onboarding__card" aria-label={title}>
        {stepLabel ? (
          <Text as="p" size="1" className="desktop-onboarding__step-label">
            {stepLabel}
          </Text>
        ) : null}
        <Heading size={placement === "modal" ? "7" : "5"} className="desktop-onboarding__title">
          {title}
        </Heading>
        <Text as="p" size="2" className="desktop-onboarding__body">
          {body}
        </Text>
        {footer ? <div className="desktop-onboarding__footer">{footer}</div> : null}
        <div className="desktop-onboarding__actions">
          {onSecondary && secondaryLabel ? (
            <Button className="desktop-onboarding__button desktop-onboarding__button--secondary" variant="soft" onClick={onSecondary}>
              {secondaryLabel}
            </Button>
          ) : null}
          {onEnd ? (
            <Button className="desktop-onboarding__button desktop-onboarding__button--ghost" variant="soft" color="gray" onClick={onEnd}>
              {endLabel}
            </Button>
          ) : null}
          <Button className="desktop-onboarding__button desktop-onboarding__button--primary" onClick={onPrimary}>
            {primaryLabel}
          </Button>
        </div>
      </section>
    </div>
  );
}
