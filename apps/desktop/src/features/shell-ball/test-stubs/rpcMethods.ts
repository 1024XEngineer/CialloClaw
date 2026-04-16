import { loadSettings } from "@/services/settingsService";

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

export async function submitInput(_params?: unknown) {
  return {
    task: {
      task_id: "task_stub",
      status: "processing",
    },
    bubble_message: null,
  };
}

export async function startTask(_params?: unknown) {
  return {
    task: {
      task_id: "task_stub",
      status: "processing",
    },
    bubble_message: null,
    delivery_result: null,
  };
}

export async function listTaskArtifacts(_params?: unknown) {
  return {
    items: [],
    page: {
      has_more: false,
      limit: 50,
      offset: 0,
      total: 0,
    },
  };
}

export async function openTaskArtifact(_params?: unknown) {
  return {
    artifact: {
      artifact_id: "artifact_stub",
      artifact_type: "workspace_document",
      mime_type: "text/plain",
      path: "workspace/stub.txt",
      task_id: "task_stub",
      title: "stub.txt",
    },
    delivery_result: {
      type: "task_detail" as const,
      title: "Task detail",
      preview_text: "回到任务详情",
      payload: { path: null, task_id: "task_stub", url: null },
    },
    open_action: "task_detail" as const,
    resolved_payload: { path: null, task_id: "task_stub", url: null },
  };
}

export async function openDelivery(_params?: unknown) {
  return {
    delivery_result: {
      type: "task_detail" as const,
      title: "Task detail",
      preview_text: "回到任务详情",
      payload: { path: null, task_id: "task_stub", url: null },
    },
    open_action: "task_detail" as const,
    resolved_payload: { path: null, task_id: "task_stub", url: null },
  };
}

export async function getSettingsDetailed(_params?: unknown) {
  // Dashboard contract tests only need a deterministic snapshot payload.
  return createDetailedResponse(loadSettings());
}

export async function updateSettings(_params?: unknown) {
  // The test stub mirrors the stable settings.update shape without simulating
  // backend-side policy changes, which is enough for mock-mode contract tests.
  const current = loadSettings();

  return {
    apply_mode: "immediate" as const,
    effective_settings: current.settings,
    need_restart: false,
    updated_keys: [],
  };
}
