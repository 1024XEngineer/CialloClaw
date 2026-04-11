// JsonRpcRequest 描述当前模块请求结构。
type JsonRpcRequest = {
  jsonrpc: "2.0";
  id: string;
  method: string;
  params?: object;
};

// JsonRpcEnvelope 定义当前模块的数据结构。
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

export type JsonRpcResultMeta = {
  server_time: string;
};

export type JsonRpcResponsePayload<T> = {
  data: T;
  meta: JsonRpcResultMeta | null;
  warnings: string[];
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

// JsonRpcTransport 定义当前模块的接口约束。
interface JsonRpcTransport {
  send<T>(payload: JsonRpcRequest): Promise<JsonRpcEnvelope<T>>;
}

declare global {
  interface Window {
    __CIALLOCLAW_NAMED_PIPE__?: {
      request: <T>(payload: JsonRpcRequest) => Promise<JsonRpcEnvelope<T>>;
      subscribe: (
        topic: string,
        handler: (message: JsonRpcNotification) => void,
      ) => Promise<NamedPipeSubscription>;
    };
  }
}

// NamedPipeJsonRpcTransport 定义当前模块的数据结构。
class NamedPipeJsonRpcTransport implements JsonRpcTransport {
  async send<T>(payload: JsonRpcRequest): Promise<JsonRpcEnvelope<T>> {
    const bridge = window.__CIALLOCLAW_NAMED_PIPE__;

    if (!bridge) {
      throw new Error("Named Pipe transport is not wired. Set VITE_CIALLOCLAW_RPC_TRANSPORT=http to use the debug HTTP fallback.");
    }

    return bridge.request<T>(payload);
  }
}

// DebugHttpJsonRpcTransport 定义当前模块的数据结构。
class DebugHttpJsonRpcTransport implements JsonRpcTransport {
  constructor(private readonly endpoint: string) {}

  async send<T>(payload: JsonRpcRequest): Promise<JsonRpcEnvelope<T>> {
    const response = await fetch(this.endpoint, {
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      throw new Error(`rpc request failed: ${response.status}`);
    }

    return (await response.json()) as JsonRpcEnvelope<T>;
  }
}

export class JsonRpcClientError extends Error {
  readonly code: number | null;

  readonly traceId: string | null;

  readonly detail: string | null;

  readonly rpcMessage: string;

  constructor(error: JsonRpcEnvelope<never>["error"]) {
    const message = error?.data?.detail ?? error?.message ?? "Unknown JSON-RPC error";
    super(message);
    this.name = "JsonRpcClientError";
    this.code = error?.code ?? null;
    this.traceId = error?.data?.trace_id ?? null;
    this.detail = error?.data?.detail ?? null;
    this.rpcMessage = error?.message ?? message;
  }
}

// createTransport 处理当前模块的相关逻辑。
function createTransport(): JsonRpcTransport {
  const defaultTransport = import.meta.env.DEV ? "http" : "named_pipe";
  const transportMode = import.meta.env.VITE_CIALLOCLAW_RPC_TRANSPORT ?? defaultTransport;

  if (transportMode === "http") {
    return new DebugHttpJsonRpcTransport(import.meta.env.VITE_CIALLOCLAW_DEBUG_RPC_ENDPOINT ?? "/rpc");
  }

  return new NamedPipeJsonRpcTransport();
}

function createRequestId() {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }

  return `rpc_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

// JsonRpcClient 定义当前模块的数据结构。
export class JsonRpcClient {
  constructor(private readonly transport: JsonRpcTransport = createTransport()) {}

  async requestDetailed<T>(method: string, params?: object): Promise<JsonRpcResponsePayload<T>> {
    const payload: JsonRpcRequest = {
      jsonrpc: "2.0",
      id: createRequestId(),
      method,
      params,
    };

    const body = await this.transport.send<T>(payload);

    if (body.error) {
      throw new JsonRpcClientError(body.error);
    }

    if (!body.result) {
      throw new JsonRpcClientError({
        message: `JSON-RPC method ${method} returned no result payload.`,
      });
    }

    return {
      data: body.result.data,
      meta: body.result.meta ?? null,
      warnings: body.result.warnings ?? [],
    };
  }

  async request<T>(method: string, params?: object): Promise<T> {
    const response = await this.requestDetailed<T>(method, params);
    return response.data;
  }
}

// rpcClient 表示当前模块的客户端实例。
export const rpcClient = new JsonRpcClient();
