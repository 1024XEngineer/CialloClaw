const SHELL_BALL_INTENT_LABELS: Record<string, string> = {
  agent_loop: "执行任务",
  explain: "解释",
  rewrite: "改写",
  summarize: "总结",
  translate: "翻译",
  write_file: "写文档",
};

/**
 * Formats a formal intent name into a compact Chinese label for shell-ball.
 *
 * @param intentName Formal intent name returned by the backend task payload.
 * @returns Localized label for the confirmation bubble and correction input.
 */
export function formatShellBallIntentLabel(intentName: string): string {
  const normalizedIntentName = intentName.trim().toLowerCase();

  if (normalizedIntentName === "") {
    return "当前任务";
  }

  return SHELL_BALL_INTENT_LABELS[normalizedIntentName]
    ?? normalizedIntentName.replace(/[_-]+/gu, " ");
}

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
