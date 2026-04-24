import { useEffect } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { desktopOnboardingEvents, desktopOnboardingLocalEvents } from "./onboarding.events";
import type { DesktopOnboardingActionRequest } from "./onboardingService";

/**
 * Subscribes the current window to onboarding action requests so the dedicated
 * onboarding overlay can ask a business window to perform a real operation
 * without handling the operation itself.
 *
 * @param targetWindow Window label handled by the current subscriber.
 * @param handler Action handler for matching requests.
 */
export function useDesktopOnboardingActions(
  targetWindow: DesktopOnboardingActionRequest["targetWindow"],
  handler: (action: DesktopOnboardingActionRequest) => void,
) {
  useEffect(() => {
    const currentWindow = getCurrentWindow();
    let disposeWindowListener: (() => void) | null = null;
    let disposed = false;

    const handleAction = (action: DesktopOnboardingActionRequest) => {
      if (action.targetWindow !== targetWindow) {
        return;
      }

      handler(action);
    };

    const handleLocalAction = (event: Event) => {
      const customEvent = event as CustomEvent<DesktopOnboardingActionRequest>;
      handleAction(customEvent.detail);
    };

    window.addEventListener(desktopOnboardingLocalEvents.actionRequested, handleLocalAction);

    void currentWindow
      .listen<DesktopOnboardingActionRequest>(desktopOnboardingEvents.actionRequested, ({ payload }) => {
        handleAction(payload);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        disposeWindowListener = unlisten;
      });

    return () => {
      disposed = true;
      window.removeEventListener(desktopOnboardingLocalEvents.actionRequested, handleLocalAction);
      disposeWindowListener?.();
    };
  }, [handler, targetWindow]);
}
