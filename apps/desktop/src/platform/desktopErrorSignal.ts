const DESKTOP_FORMAL_ERROR_SIGNAL_TOKENS = ["error", "failed", "exception"] as const;

function containsDesktopFormalErrorSignalToken(value: string) {
  const normalizedValue = value.toLowerCase();
  return DESKTOP_FORMAL_ERROR_SIGNAL_TOKENS.some((token) => normalizedValue.includes(token));
}

/**
 * Filters host-provided error candidates down to explicit failure evidence so
 * warning-only labels do not enter the formal desktop error path.
 *
 * @param value Host-provided error candidate text.
 * @returns Trimmed error text when it contains explicit failure evidence.
 */
export function normalizeDesktopErrorSignalText(value: string | null | undefined) {
  const normalizedValue = value?.trim();
  if (!normalizedValue) {
    return undefined;
  }

  return containsDesktopFormalErrorSignalToken(normalizedValue) ? normalizedValue : undefined;
}
