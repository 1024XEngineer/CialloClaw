function createDetailedResponse<T>(data: T) {
  return {
    data,
    meta: {
      server_time: new Date().toISOString(),
    },
    warnings: [] as string[],
  };
}

export async function listSecurityPending(_params?: unknown) {
  return { items: [], page: { limit: 20, offset: 0, total: 0, has_more: false } };
}

export async function respondSecurity(_params?: unknown) {
  return {};
}

export async function getSecuritySummary(_params?: unknown) {
  return {
    summary: {
      security_status: "normal" as const,
      pending_authorizations: 0,
    },
  };
}

export async function getSecuritySummaryDetailed(_params?: unknown) {
  return createDetailedResponse(await getSecuritySummary());
}

export async function listSecurityPendingDetailed(_params?: unknown) {
  return createDetailedResponse(await listSecurityPending());
}

export async function respondSecurityDetailed(_params?: unknown) {
  return createDetailedResponse({
    authorization_record: {
      decision: "allow_once",
      remember_rule: false,
    },
    impact_scope: {
      apps: [],
      files: [],
      webpages: [],
    },
    task: {
      task_id: "task_stub",
      status: "processing",
    },
  });
}
