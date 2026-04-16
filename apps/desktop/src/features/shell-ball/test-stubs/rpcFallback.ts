export function isRpcChannelUnavailable(error: unknown) {
  return error instanceof Error && /transport is not wired|timed out|fetch failed|network/i.test(error.message);
}

export function logRpcMockFallback(_scope: string, _error: unknown) {
  return;
}
