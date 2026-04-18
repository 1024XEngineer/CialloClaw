export class JsonRpcClientError extends Error {
  readonly code: number | null = null;

  readonly traceId: string | null = null;

  readonly detail: string | null = null;

  readonly rpcMessage: string;

  constructor(message = "stubbed json-rpc error") {
    super(message);
    this.name = "JsonRpcClientError";
    this.rpcMessage = message;
  }
}
