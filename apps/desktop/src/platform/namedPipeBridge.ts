import { Channel, invoke } from "@tauri-apps/api/core";

type JsonRpcRequest = {
  jsonrpc: "2.0";
  id: string;
  method: string;
  params?: object;
};

type JsonRpcEnvelope<T> = {
  jsonrpc?: "2.0";
  id?: string | number | null;
  result?: {
    data: T;
    meta?: { server_time: string };
    warnings?: string[];
  };
  error?: {
    code?: number;
    message: string;
    data?: {
      detail?: string;
      trace_id?: string;
    };
  };
};

type JsonRpcNotification = {
  jsonrpc?: "2.0";
  id?: string | number | null;
  method?: string;
  params?: unknown;
  [key: string]: unknown;
};

type NamedPipeSubscription = {
  id: number;
  unsubscribe: () => Promise<void>;
};

declare global {
  interface Window {
    __CIALLOCLAW_NAMED_PIPE__?: {
      request: <T>(payload: JsonRpcRequest) => Promise<JsonRpcEnvelope<T>>;
      subscribe: (topic: string, handler: (message: JsonRpcNotification) => void) => Promise<NamedPipeSubscription>;
    };
  }
}

const NAMED_PIPE_BRIDGE = Object.freeze({
  request<T>(payload: JsonRpcRequest) {
    return invoke<JsonRpcEnvelope<T>>("named_pipe_request", { payload });
  },

  async subscribe(topic: string, handler: (message: JsonRpcNotification) => void): Promise<NamedPipeSubscription> {
    const onEvent = new Channel<JsonRpcNotification>(handler);
    const subscriptionId = await invoke<number>("named_pipe_subscribe", { topic, onEvent });

    return {
      id: subscriptionId,
      unsubscribe: () => invoke("named_pipe_unsubscribe", { subscriptionId }),
    };
  },
});

function isTauriEnvironment() {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

if (isTauriEnvironment() && !window.__CIALLOCLAW_NAMED_PIPE__) {
  window.__CIALLOCLAW_NAMED_PIPE__ = NAMED_PIPE_BRIDGE;
}

export {};
