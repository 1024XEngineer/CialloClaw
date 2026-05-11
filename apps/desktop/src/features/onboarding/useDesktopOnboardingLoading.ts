import { useEffect, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { desktopOnboardingEvents, desktopOnboardingLocalEvents } from "./onboarding.events";
import { loadDesktopOnboardingLoadingState, type DesktopOnboardingLoadingState } from "./onboardingService";

/**
 * Subscribes the current window to onboarding launch loading updates so launch
 * points can show a lightweight "正在打开引导..." hint while the on-demand
 * onboarding window is still being prepared.
 */
export function useDesktopOnboardingLoading(windowLabel: DesktopOnboardingLoadingState["windowLabel"]) {
  const [loadingState, setLoadingState] = useState<DesktopOnboardingLoadingState | null>(() =>
    loadDesktopOnboardingLoadingState(),
  );

  useEffect(() => {
    const currentWindow = getCurrentWindow();
    let disposeWindowListener: (() => void) | null = null;
    let disposed = false;

    const handleLocalLoadingChanged = (event: Event) => {
      const customEvent = event as CustomEvent<DesktopOnboardingLoadingState | null>;
      setLoadingState(customEvent.detail ?? null);
    };

    setLoadingState(loadDesktopOnboardingLoadingState());
    window.addEventListener(desktopOnboardingLocalEvents.loadingChanged, handleLocalLoadingChanged);

    void currentWindow
      .listen<DesktopOnboardingLoadingState | null>(desktopOnboardingEvents.loadingChanged, ({ payload }) => {
        setLoadingState(payload ?? null);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        disposeWindowListener = unlisten;
        setLoadingState(loadDesktopOnboardingLoadingState());
      });

    return () => {
      disposed = true;
      window.removeEventListener(desktopOnboardingLocalEvents.loadingChanged, handleLocalLoadingChanged);
      disposeWindowListener?.();
    };
  }, []);

  return loadingState?.windowLabel === windowLabel ? loadingState : null;
}
