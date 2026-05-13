export type MockRecentMemory = {
  id: string;
  title: string;
  lastReferencedAt: string;
  sourceTask: string;
  referenceCount: number;
  source: string;
  summary: string;
  scope: string;
  highlights: string[];
  rawRecord: string;
  relatedPreference: string;
  historyLinks: string[];
};

export type MockSessionSummary = {
  id: string;
  date: string;
  title: string;
  timeRange: string;
  messageCount: number;
  durationLabel: string;
  sourceFile: string;
  summary: string;
  detailLines: string[];
};

export type MockProfileMetric = {
  label: string;
  value: string;
};

export const mockRecentMemories: MockRecentMemory[] = [
  {
    id: "memory-security-001",
    title: "补齐安全详情中的授权说明与恢复点解释",
    lastReferencedAt: "周四 13:24",
    sourceTask: "task_security_mock_001",
    referenceCount: 1,
    source: "2026.05.14 会话摘要",
    summary: "这条记忆来自今天唯一一次协作，核心是把安全详情页中的授权说明、恢复点含义和异常处理文案说清楚。",
    scope: "适用于补充安全说明、授权范围解释和恢复点展示文案时。",
    highlights: ["授权范围说明", "恢复点作用解释", "异常情况处理提示"],
    rawRecord: "用户要求在安全详情中补齐授权说明，并明确解释恢复点的作用，避免只给结论不说明边界。",
    relatedPreference: "当前样本显示，用户更关注说明是否清楚、恢复路径是否明确。",
    historyLinks: ["2026.05.14 会话摘要", "conversation_20260514_1300.json"],
  },
];

export const mockSessionSummaries: MockSessionSummary[] = [
  {
    id: "session-20260514-1300",
    date: "2026.05.14",
    title: "安全说明补充与恢复点解释",
    timeRange: "周四 13:00–13:30",
    messageCount: 12,
    durationLabel: "30 分钟",
    sourceFile: "conversation_20260514_1300.json",
    summary: "本次会话主要围绕安全详情页的授权说明、恢复点解释和展示文案进行补充，重点是让用户更清楚地理解授权范围、恢复点作用以及异常情况下的处理方式。",
    detailLines: [
      "讨论重点集中在授权说明是否清楚。",
      "恢复点需要解释用途，而不只是显示字段名。",
      "异常情况文案要强调如何回退和如何理解当前状态。",
    ],
  },
];

export const mockHistoryDates = ["2026.05.14", "2026.05.13", "2026.05.12"] as const;

export const mockProfileMetrics: MockProfileMetric[] = [
  { label: "活跃时间", value: "周四 13:00–13:30" },
  { label: "累计使用", value: "30 分钟" },
  { label: "活跃天数", value: "1 天" },
  { label: "使用状态", value: "初次观察中" },
];

export const mockProfileAxes = [
  { label: "表达偏好", value: 0.28 },
  { label: "结构敏感", value: 0.24 },
  { label: "安全关注", value: 0.32 },
  { label: "节奏稳定", value: 0.16 },
  { label: "样本量", value: 0.08 },
];

export const mockDailySummary = {
  title: "日报",
  lines: [
    "日期：2026.05.14",
    "完成任务：1 项",
    "使用时长：30 分钟",
    "活跃时间：13:00–13:30",
  ],
};

export const mockPhaseSummary = {
  title: "阶段总结",
  lines: [
    "统计周期：本周",
    "活跃天数：1/7 天",
    "累计使用：0.5h",
    "会话数量：1 个",
  ],
};

export const mockHeatmap = [
  [0, 0, 0, 0, 0, 0, 0],
  [0, 0, 0, 0, 0, 0, 0],
  [0, 0, 0, 0, 0, 0, 0],
  [0, 0, 0, 0.9, 0, 0, 0],
  [0, 0, 0, 0, 0, 0, 0],
  [0, 0, 0, 0, 0, 0, 0],
  [0, 0, 0, 0, 0, 0, 0],
];

export function findMockRecentMemory(memoryId: string) {
  return mockRecentMemories.find((memory) => memory.id === memoryId) ?? null;
}

export function findMockSessionSummary(sessionId: string) {
  return mockSessionSummaries.find((session) => session.id === sessionId) ?? null;
}

export function findMockSessionSummaryByDate(date: string) {
  return mockSessionSummaries.find((session) => session.date === date) ?? null;
}
