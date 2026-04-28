import { invoke } from "@tauri-apps/api/core";

export type OnboardingInteractiveRect = {
  height: number;
  width: number;
  x: number;
  y: number;
};

/**
 * Updates the native hit-test map for the onboarding overlay window so only the
 * visible guide card consumes pointer input and the rest of the overlay stays
 * click-through.
 *
 * @param regions Interactive rectangles relative to the onboarding window.
 */
export async function setOnboardingInteractiveRegions(regions: OnboardingInteractiveRect[]) {
  await invoke("onboarding_set_interactive_regions", {
    regions,
  });
}

/**
 * Resets the native onboarding hit-test state so a recreated overlay starts in
 * a fully click-through mode until the first card layout registers new regions.
 */
export async function resetOnboardingInteractiveState() {
  await invoke("onboarding_reset_interactive_state");
}

/**
 * Forces the onboarding window back into a fully click-through state until the
 * frontend registers the first interactive guide card.
 */
export async function setOnboardingIgnoreCursorEvents(ignore = true) {
  await invoke("onboarding_set_ignore_cursor_events", {
    ignore,
  });
}
