import assert from "node:assert/strict";
import test from "node:test";
import type { AgentInputSubmitParams, AgentMirrorOverviewGetResult, ApprovalRequest, Task, TokenCostSummary } from "@cialloclaw/protocol";
import {
  buildMirrorConversationDateOptions,
  buildMirrorConversationTaskMoments,
  buildMirrorDailyDigest,
  buildMirrorProfileBaseItems,
  buildMirrorProfileView,
  filterMirrorConversationRecords,
} from "./mirrorViewModel";
import {
  loadMirrorConversationRecords,
  recordMirrorConversationFailure,
  recordMirrorConversationStart,
  recordMirrorConversationSuccess,
  upsertMirrorConversationRecord,
  type MirrorConversationRecord,
} from "../../../services/mirrorMemoryService";
import { loadSettings, saveSettings } from "../../../services/settingsService";

class MemoryStorage {
  #store = new Map<string, string>();

  clear() {
    this.#store.clear();
  }

  getItem(key: string) {
    return this.#store.get(key) ?? null;
  }

  key(index: number) {
    return Array.from(this.#store.keys())[index] ?? null;
  }

  removeItem(key: string) {
    this.#store.delete(key);
  }

  setItem(key: string, value: string) {
    this.#store.set(key, value);
  }

  get length() {
    return this.#store.size;
  }
}

function installWindowStorage() {
  const localStorage = new MemoryStorage();
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage,
    },
    writable: true,
  });
  return localStorage;
}

function createSubmitParams(traceId: string): AgentInputSubmitParams {
  return {
    request_meta: {
      trace_id: traceId,
      client_time: "2026-04-13T10:00:00+08:00",
    },
    source: "dashboard",
    trigger: "voice_commit",
    input: {
      type: "text",
      text: "把今天的镜子摘要整理得更清楚。",
      input_mode: "voice",
    },
    context: {
      files: [],
    },
  };
}

function createTask(taskId: string, status: Task["status"], input: Partial<Task> = {}): Task {
  return {
    task_id: taskId,
    session_id: input.session_id ?? null,
    title: input.title ?? `task ${taskId}`,
    source_type: input.source_type ?? "hover_input",
    status,
    intent: input.intent ?? null,
    current_step: input.current_step ?? "current_step",
    risk_level: input.risk_level ?? "green",
    started_at: input.started_at ?? "2026-04-13T09:00:00+08:00",
    updated_at: input.updated_at ?? "2026-04-13T10:00:00+08:00",
    finished_at: input.finished_at ?? null,
  };
}

function createConversationRecord(index: number): MirrorConversationRecord {
  const createdAt = new Date(2026, 3, 13, 0, 0, index).toISOString();
  const updatedAt = new Date(2026, 3, 13, 0, 1, index).toISOString();

  return {
    record_id: `record-${index}`,
    trace_id: `trace-${index}`,
    created_at: createdAt,
    updated_at: updatedAt,
    source: index % 2 === 0 ? "dashboard" : "floating_ball",
    trigger: index % 2 === 0 ? "voice_commit" : "hover_text_input",
    input_mode: index % 2 === 0 ? "voice" : "text",
    session_id: null,
    task_id: `task-${index}`,
    user_text: `user-${index}`,
    agent_text: `agent-${index}`,
    agent_bubble_type: "result",
    status: "responded",
    error_message: null,
  };
}

function createOverview(): AgentMirrorOverviewGetResult {
  return {
    history_summary: ["近期结果记录包含结构化结论与步骤。", "风险动作记录会先显示确认说明。"],
    daily_summary: {
      date: "2026-04-13",
      completed_tasks: 2,
      generated_outputs: 5,
    },
    profile: {
      work_style: "协议对齐优先，再进入实现。",
      preferred_output: "结构化结果页",
      active_hours: "09:30 - 19:00",
    },
    memory_references: [
      {
        memory_id: "mem_001",
        reason: "复用既有结果结构。",
        summary: "保留原有纸感母片和检片台细节。",
      },
    ],
  };
}

function createTokenCostSummary(): TokenCostSummary {
  return {
    current_task_tokens: 1200,
    current_task_cost: 0.24,
    today_tokens: 9200,
    today_cost: 1.62,
    single_task_limit: 50000,
    daily_limit: 300000,
    budget_auto_downgrade: true,
  };
}

test("upsertMirrorConversationRecord caps the list at one hundred items", () => {
  let records: MirrorConversationRecord[] = [];

  for (let index = 0; index < 105; index += 1) {
    records = upsertMirrorConversationRecord(records, createConversationRecord(index));
  }

  assert.equal(records.length, 100);
  assert.equal(records[0]?.trace_id, "trace-104");
  assert.equal(records.at(-1)?.trace_id, "trace-5");
});

test("mirror conversation lifecycle stores user and agent sides locally", () => {
  installWindowStorage();
  const params = createSubmitParams("trace-lifecycle");

  recordMirrorConversationStart(params);
  recordMirrorConversationSuccess(params, {
    task: createTask("task-lifecycle", "processing"),
    bubble_message: {
      bubble_id: "bubble-lifecycle",
      task_id: "task-lifecycle",
      type: "result",
      text: "镜子详情已经连上本地完整对话。",
      pinned: false,
      hidden: false,
      created_at: "2026-04-13T10:00:04+08:00",
    },
    delivery_result: null,
  });

  const records = loadMirrorConversationRecords();

  assert.equal(records.length, 1);
  assert.equal(records[0]?.user_text, params.input.text);
  assert.equal(records[0]?.agent_text, "镜子详情已经连上本地完整对话。");
  assert.equal(records[0]?.status, "responded");
  assert.equal(records[0]?.task_id, "task-lifecycle");
});

test("mirror conversation lifecycle preserves failures in local history", () => {
  installWindowStorage();
  const params = createSubmitParams("trace-failure");

  recordMirrorConversationStart(params);
  recordMirrorConversationFailure(params, new Error("network unavailable"));

  const records = loadMirrorConversationRecords();

  assert.equal(records[0]?.status, "failed");
  assert.equal(records[0]?.error_message, "network unavailable");
});

test("mirror conversation lifecycle stops persisting and clears local records when memory is disabled", () => {
  const storage = installWindowStorage();
  const params = createSubmitParams("trace-memory-disabled");

  recordMirrorConversationStart(params);
  assert.equal(loadMirrorConversationRecords().length, 1);

  const disabledSettings = loadSettings();
  saveSettings({
    ...disabledSettings,
    settings: {
      ...disabledSettings.settings,
      memory: {
        ...disabledSettings.settings.memory,
        enabled: false,
      },
    },
  });

  recordMirrorConversationStart(createSubmitParams("trace-memory-disabled-next"));

  assert.equal(loadMirrorConversationRecords().length, 0);
  assert.equal(storage.getItem("cialloclaw.mirror.conversations"), null);
});

test("buildMirrorDailyDigest surfaces stage and approval context", () => {
  const digest = buildMirrorDailyDigest({
    overview: createOverview(),
    unfinished_tasks: [
      createTask("task-processing", "processing", { title: "正在推进的任务" }),
      createTask("task-waiting", "waiting_auth", { title: "等待授权的任务", risk_level: "yellow" }),
    ],
    finished_tasks: [
      createTask("task-finished", "completed", {
        title: "今天完成的任务",
        finished_at: "2026-04-13T11:20:00+08:00",
      }),
    ],
    pending_approvals: [
      {
        approval_id: "approval-001",
        task_id: "task-waiting",
        operation_name: "批量改写文件",
        risk_level: "red",
        target_object: "workspace",
        reason: "涉及多个文件覆盖写入。",
        status: "pending",
        created_at: "2026-04-13T10:10:00+08:00",
      } satisfies ApprovalRequest,
    ],
    security_status: "pending_confirmation",
    latest_restore_point_summary: "改动前创建了恢复点。",
    token_cost_summary: createTokenCostSummary(),
    conversations: [createConversationRecord(1), createConversationRecord(2)],
  });

  assert.equal(digest.stage_snapshots[0]?.count, 1);
  assert.equal(digest.stage_snapshots[1]?.count, 1);
  assert.equal(digest.context_notes[0]?.value, "1 项待处理");
  assert.equal(digest.headline, "今日完成 2 条任务，未结束 2 条。");
  assert.equal(digest.lede, "当前待确认动作：批量改写文件。");
  assert.equal(digest.stats[3]?.detail, "仅统计最近 100 条本地输入与前端可见回应记录。");
});

test("buildMirrorProfileView keeps backend fields and local recent statistics separate", () => {
  const items = buildMirrorProfileBaseItems({
    profile: createOverview().profile,
    conversations: [createConversationRecord(1), createConversationRecord(2), createConversationRecord(3)],
  });
  const view = buildMirrorProfileView(items);

  const backendItem = view.backend_items.find((item) => item.id === "profile-work-style");
  const localStatItem = view.local_stat_items.find((item) => item.id === "local-stat-input-mode");

  assert.equal(view.backend_items.length, 3);
  assert.equal(view.local_stat_items.length, 3);
  assert.equal(view.total_items, 6);
  assert.equal(backendItem?.source_label, "后端画像字段");
  assert.equal(localStatItem?.source_label, "最近本地统计");
  assert.match(localStatItem?.hint ?? "", /本地记录机械统计/);
});

test("filterMirrorConversationRecords combines scope, source, input mode, and date filters", () => {
  const failedRecord = {
    ...createConversationRecord(4),
    status: "failed" as const,
    task_id: "task-special",
    source: "dashboard" as const,
    input_mode: "text" as const,
  };
  const earlierDayRecord = {
    ...createConversationRecord(5),
    source: "dashboard" as const,
    input_mode: "text" as const,
    updated_at: "2026-04-12T09:10:00+08:00",
  };
  const records = [
    createConversationRecord(1),
    createConversationRecord(2),
    {
      ...createConversationRecord(3),
      task_id: null,
      source: "tray_panel" as const,
      input_mode: "text" as const,
    },
    failedRecord,
    earlierDayRecord,
  ];

  assert.deepEqual(
    filterMirrorConversationRecords(records, {
      scope: "with_task",
      source: "dashboard",
      input_mode: "text",
      date_key: "2026-04-13",
    }).map((record) => record.record_id),
    [failedRecord.record_id],
  );

  assert.deepEqual(
    filterMirrorConversationRecords(records, {
      scope: "failed",
      source: "all",
      input_mode: "all",
      date_key: "all",
    }).map((record) => record.record_id),
    [failedRecord.record_id],
  );
});

test("buildMirrorConversationDateOptions keeps recent date filters ordered by newest activity day", () => {
  const records = [
    createConversationRecord(1),
    createConversationRecord(2),
    {
      ...createConversationRecord(3),
      updated_at: "2026-04-12T10:09:00+08:00",
    },
  ];

  const dateOptions = buildMirrorConversationDateOptions(records);

  assert.deepEqual(dateOptions.map((option) => option.date_key), ["2026-04-13", "2026-04-12"]);
  assert.equal(dateOptions[0]?.count, 2);
  assert.match(dateOptions[0]?.label ?? "", /4月13日/);
});

test("buildMirrorConversationTaskMoments keeps linked tasks ordered by time within the selected day", () => {
  const firstTaskRecord = createConversationRecord(1);
  const secondTaskRecord = createConversationRecord(2);
  const latestTaskRecord = {
    ...createConversationRecord(3),
    task_id: "task-1",
    updated_at: "2026-04-13T10:09:00+08:00",
  };
  const records = [firstTaskRecord, secondTaskRecord, latestTaskRecord];

  const taskMoments = buildMirrorConversationTaskMoments(records);

  assert.deepEqual(taskMoments.map((option) => option.task_id), ["task-2", "task-1"]);
  assert.equal(taskMoments[0]?.count, 1);
  assert.equal(taskMoments[0]?.latest_at, secondTaskRecord.updated_at);
  assert.equal(taskMoments[1]?.count, 2);
  assert.equal(taskMoments[1]?.latest_at, latestTaskRecord.updated_at);
});
