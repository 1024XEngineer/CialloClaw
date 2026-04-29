import { useEffect, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { desktopOnboardingEvents, desktopOnboardingLocalEvents } from "./onboarding.events";
import { loadDesktopOnboardingPresentation, type DesktopOnboardingPresentation } from "./onboardingService";

/**
 * Subscribes the current desktop window to onboarding presentation updates so
 * the dedicated card window can track the latest highlighted target.
 *
 * @returns The active onboarding presentation or `null` when no step is mapped.
 */
export function useDesktopOnboardingPresentation() {
  const [presentation, setPresentation] = useState<DesktopOnboardingPresentation | null>(() =>
    loadDesktopOnboardingPresentation(),
  );

  useEffect(() => {
    const currentWindow = getCurrentWindow();
    let disposeWindowListener: (() => void) | null = null;
    let disposed = false;

    const handleLocalPresentationChanged = (event: Event) => {
      const customEvent = event as CustomEvent<DesktopOnboardingPresentation | null>;
      setPresentation(customEvent.detail ?? null);
    };

    setPresentation(loadDesktopOnboardingPresentation());
    window.addEventListener(desktopOnboardingLocalEvents.presentationChanged, handleLocalPresentationChanged);

    void currentWindow
      .listen<DesktopOnboardingPresentation | null>(desktopOnboardingEvents.presentationChanged, ({ payload }) => {
        setPresentation(payload ?? null);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        disposeWindowListener = unlisten;
        setPresentation(loadDesktopOnboardingPresentation());
      });

    return () => {
      disposed = true;
      window.removeEventListener(desktopOnboardingLocalEvents.presentationChanged, handleLocalPresentationChanged);
      disposeWindowListener?.();
    };
  }, []);

  return presentation;
}
