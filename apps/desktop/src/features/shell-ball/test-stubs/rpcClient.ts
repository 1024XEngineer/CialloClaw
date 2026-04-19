export class JsonRpcClientError extends Error {
  code: number;
  traceId: string | null;
  data?: Record<string, unknown>;

  constructor(input: { code: number; data?: Record<string, unknown>; message: string; traceId?: string | null }) {
    super(input.message);
    this.name = "JsonRpcClientError";
    this.code = input.code;
    this.traceId = input.traceId ?? null;
    this.data = input.data;
  }
}
