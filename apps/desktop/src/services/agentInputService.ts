import type { AgentInputSubmitParams, AgentInputSubmitResult } from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { createMockAgentInputSubmitResult, type MockAgentInputSubmitResult } from "./agentInputMock";
import {
  recordMirrorConversationFailure,
  recordMirrorConversationStart,
  recordMirrorConversationSuccess,
} from "./mirrorMemoryService";

type SubmitTextInputParams = {
  text: string;
  source: AgentInputSubmitParams["source"];
  trigger: AgentInputSubmitParams["trigger"];
  inputMode: AgentInputSubmitParams["input"]["input_mode"];
  options?: {
    confirm_required?: boolean;
    preferred_delivery?: "bubble" | "workspace_document" | "result_page" | "open_file" | "reveal_in_folder" | "task_detail";
  };
};

function createRequestMeta(): AgentInputSubmitParams["request_meta"] {
  const now = new Date().toISOString();
  const traceId = typeof globalThis.crypto?.randomUUID === "function"
    ? globalThis.crypto.randomUUID()
    : `trace_${Date.now()}_${Math.random().toString(16).slice(2)}`;

  return {
    trace_id: traceId,
    client_time: now,
  };
}

export function createTextInputSubmitParams(input: SubmitTextInputParams): AgentInputSubmitParams | null {
  const normalizedText = input.text.trim();

  if (normalizedText === "") {
    return null;
  }

  return {
    request_meta: createRequestMeta(),
    source: input.source,
    trigger: input.trigger,
    input: {
      type: "text",
      text: normalizedText,
      input_mode: input.inputMode,
    },
    context: {
      files: [],
    },
    ...(input.options ? { options: input.options } : {}),
  };
}

export type SubmitTextInputResult = AgentInputSubmitResult | MockAgentInputSubmitResult;

export async function submitTextInput(input: SubmitTextInputParams) {
  const params = createTextInputSubmitParams(input);

  if (params === null) {
    return null;
  }

  recordMirrorConversationStart(params);
  const rpcMethods = await import("../rpc/methods");

  try {
    const result = await rpcMethods.submitInput(params);
    recordMirrorConversationSuccess(params, result);
    return result;
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback(`agent.input.submit ${input.source}:${input.trigger}`, error);
      const fallbackResult = createMockAgentInputSubmitResult({
        inputMode: input.inputMode,
        text: input.text,
      });
      recordMirrorConversationFailure(params, error);
      return fallbackResult;
    }

    recordMirrorConversationFailure(params, error);
    throw error;
  }
}
