export class JsonRpcClientError extends Error {
  readonly code = null;

  readonly traceId = null;

  readonly detail = null;

  readonly rpcMessage: string;

  constructor(message = "stubbed json-rpc error") {
    super(message);
    this.name = "JsonRpcClientError";
    this.rpcMessage = message;
  }
}
