import { useEffect, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { desktopOnboardingEvents, desktopOnboardingLocalEvents } from "./onboarding.events";
import { loadDesktopOnboardingSession, type DesktopOnboardingSession } from "./onboardingService";

/**
 * Subscribes the current desktop window to onboarding session changes so each
 * surface can render only the steps relevant to its own UI.
 *
 * @returns The active onboarding session, or `null` when onboarding is idle.
 */
export function useDesktopOnboardingSession() {
  const [session, setSession] = useState<DesktopOnboardingSession | null>(() => loadDesktopOnboardingSession());

  useEffect(() => {
    const currentWindow = getCurrentWindow();
    let disposeWindowListener: (() => void) | null = null;
    let disposed = false;

    const handleLocalSessionChanged = (event: Event) => {
      const customEvent = event as CustomEvent<DesktopOnboardingSession | null>;
      setSession(customEvent.detail ?? null);
    };

    setSession(loadDesktopOnboardingSession());
    window.addEventListener(desktopOnboardingLocalEvents.sessionChanged, handleLocalSessionChanged);

    void currentWindow
      .listen<DesktopOnboardingSession | null>(desktopOnboardingEvents.sessionChanged, ({ payload }) => {
        setSession(payload ?? null);
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
      window.removeEventListener(desktopOnboardingLocalEvents.sessionChanged, handleLocalSessionChanged);
      disposeWindowListener?.();
    };
  }, []);

  return session;
}
