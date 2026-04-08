// JsonRpcRequest 描述当前模块请求结构。
type JsonRpcRequest = {
  jsonrpc: "2.0";
  id: string;
  method: string;
  params?: object;
};

// JsonRpcEnvelope 定义当前模块的数据结构。
type JsonRpcEnvelope<T> = {
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

// JsonRpcTransport 定义当前模块的接口约束。
interface JsonRpcTransport {
  send<T>(payload: JsonRpcRequest): Promise<JsonRpcEnvelope<T>>;
}

declare global {
  interface Window {
    __CIALLOCLAW_NAMED_PIPE__?: {
      request: <T>(payload: JsonRpcRequest) => Promise<JsonRpcEnvelope<T>>;
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

// createTransport 处理当前模块的相关逻辑。
function createTransport(): JsonRpcTransport {
  const transportMode = import.meta.env.VITE_CIALLOCLAW_RPC_TRANSPORT ?? "named_pipe";

  if (transportMode === "http") {
    return new DebugHttpJsonRpcTransport(import.meta.env.VITE_CIALLOCLAW_DEBUG_RPC_ENDPOINT ?? "http://127.0.0.1:4317/rpc");
  }

  return new NamedPipeJsonRpcTransport();
}

// JsonRpcClient 定义当前模块的数据结构。
export class JsonRpcClient {
  constructor(private readonly transport: JsonRpcTransport = createTransport()) {}

  async request<T>(method: string, params?: object): Promise<T> {
    const payload: JsonRpcRequest = {
      jsonrpc: "2.0",
      id: crypto.randomUUID(),
      method,
      params,
    };

    const body = await this.transport.send<T>(payload);

    if (body.error) {
      throw new Error(body.error.data?.detail ?? body.error.message);
    }

    return body.result?.data as T;
  }
}

// rpcClient 表示当前模块的客户端实例。
export const rpcClient = new JsonRpcClient();
