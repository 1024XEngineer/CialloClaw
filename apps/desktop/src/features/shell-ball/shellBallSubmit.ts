import type { AgentInputSubmitParams } from "@cialloclaw/protocol";
import { createTextInputSubmitParams, submitTextInput } from "../../services/agentInputService";

export type ShellBallTextSubmitInput = {
  text: string;
  trigger: "voice_commit" | "hover_text_input";
  inputMode: "voice" | "text";
  sessionId?: string;
};

/**
 * Builds the formal `agent.input.submit` payload used by shell-ball text and
 * voice submissions.
 *
 * @param input Trigger metadata together with the draft text to submit.
 * @returns The normalized RPC payload, or `null` when the draft is empty.
 */
export function createShellBallInputSubmitParams(input: ShellBallTextSubmitInput): AgentInputSubmitParams | null {
  return createTextInputSubmitParams({
    text: input.text,
    source: "floating_ball",
    trigger: input.trigger,
    inputMode: input.inputMode,
    sessionId: input.sessionId,
    options: {
      confirm_required: false,
      preferred_delivery: "bubble",
    },
  });
}

/**
 * Routes every shell-ball free-form submit through the same formal intake path
 * without reusing ambient browser-page context.
 *
 * @param input Trigger metadata together with the draft text to submit.
 * @returns The formal submit result, or `null` when the draft is empty.
 */
export async function submitShellBallInput(input: ShellBallTextSubmitInput) {
  return submitTextInput({
    text: input.text,
    source: "floating_ball",
    trigger: input.trigger,
    inputMode: input.inputMode,
    sessionId: input.sessionId,
    options: {
      confirm_required: false,
      preferred_delivery: "bubble",
    },
  });
}
