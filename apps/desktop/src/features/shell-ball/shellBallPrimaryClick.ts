/**
 * Single-click recommendation fetches should never steal focus away from a
 * manual shell-ball draft or pending attachments that the user is already
 * curating locally.
 */
export function shouldFocusShellBallInlineInputBeforePrimaryClick(input: {
  inputValue: string;
  pendingFiles?: readonly string[];
}) {
  return input.inputValue.trim() !== "" || (input.pendingFiles?.length ?? 0) > 0;
}
