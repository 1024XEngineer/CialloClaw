/**
 * Builds the temporary input label shown while shell-ball borrows the inline
 * composer for natural-language intent correction.
 *
 * @param currentIntentLabel Current inferred intent label.
 * @returns Borrowed-input label text.
 */
export function buildShellBallIntentCorrectionLabel(currentIntentLabel: string): string {
  return `当前意图：${currentIntentLabel}`;
}

/**
 * Builds the temporary input placeholder for intent correction mode.
 *
 * @param currentIntentLabel Current inferred intent label.
 * @returns Borrowed-input placeholder text.
 */
export function buildShellBallIntentCorrectionPlaceholder(currentIntentLabel: string): string {
  return `输入你真正想做的事；留空发送则按“${currentIntentLabel}”继续。`;
}
