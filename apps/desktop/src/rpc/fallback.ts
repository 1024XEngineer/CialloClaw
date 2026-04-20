import { JsonRpcClientError } from "./client";

const RPC_UNAVAILABLE_MESSAGE_PARTS = [
  "transport is not wired",
  "failed to fetch",
  "fetch failed",
  "network request failed",
  "networkerror",
  "load failed",
  "failed to open named pipe",
  "named pipe bridge task failed",
  "named pipe response wait failed",
  "request timed out",
  "timed out",
] as const;

const loggedFallbackScopes = new Set<string>();

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
  if (loggedFallbackScopes.has(scope)) {
    return;
  }

  loggedFallbackScopes.add(scope);
  console.warn(`${scope} RPC unavailable, using mock fallback.`, error);
}
