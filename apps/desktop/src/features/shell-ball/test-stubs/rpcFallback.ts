import { JsonRpcClientError } from "./rpcClient";

const RPC_UNAVAILABLE_MESSAGE_PARTS = [
  "transport is not wired",
  "failed to fetch",
  "fetch failed",
  "network request failed",
  "networkerror",
  "load failed",
  "request timed out",
  "timed out",
] as const;

export function isRpcChannelUnavailable(error: unknown) {
  if (error instanceof JsonRpcClientError) {
    return false;
  }

  if (!(error instanceof Error)) {
    return false;
  }

  const normalizedMessage = error.message.toLowerCase();
  return RPC_UNAVAILABLE_MESSAGE_PARTS.some((fragment) => normalizedMessage.includes(fragment));
}

export function logRpcMockFallback(scope: string, error: unknown) {
  console.warn(`${scope} RPC unavailable, using mock fallback.`, error);
}
