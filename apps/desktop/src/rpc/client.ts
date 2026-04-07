type JsonRpcRequest = {
  jsonrpc: "2.0";
  id: string;
  method: string;
  params?: object;
};

export class JsonRpcClient {
  constructor(private readonly endpoint = "http://127.0.0.1:4317/rpc") {}

  async request<T>(method: string, params?: object): Promise<T> {
    const payload: JsonRpcRequest = {
      jsonrpc: "2.0",
      id: crypto.randomUUID(),
      method,
      params,
    };

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

    const body = (await response.json()) as {
      result?: {
        data: T;
        meta?: { server_time: string };
        warnings?: string[];
      };
      error?: { message: string };
    };

    if (body.error) {
      throw new Error(body.error.message);
    }

    return body.result?.data as T;
  }
}

export const rpcClient = new JsonRpcClient();
