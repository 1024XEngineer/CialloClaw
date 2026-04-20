import type { AgentTaskDetailGetResult, ApprovalRequest, RecoveryPoint, Task, TaskRuntimeSummary, TaskStep } from "@cialloclaw/protocol";
import type { TaskControlOutcome, TaskDetailData, TaskExperience, TaskListItem } from "./taskPage.types";

const HOUR = 1000 * 60 * 60;
const DAY = HOUR * 24;
const now = Date.now();

function iso(offset: number) {
  return new Date(now + offset).toISOString();
}

function clone<T>(value: T) {
  return structuredClone(value);
}

function createRecoveryPoint(taskId: string, recoveryPointId: string, summary: string, createdAt: string, objects: string[]): RecoveryPoint {
  return {
    created_at: createdAt,
    objects,
    recovery_point_id: recoveryPointId,
    summary,
    task_id: taskId,
  };
}

function createApprovalRequest(taskId: string, approvalId: string, riskLevel: ApprovalRequest["risk_level"], operationName: string, targetObject: string, reason: string, createdAt: string): ApprovalRequest {
  return {
    approval_id: approvalId,
    created_at: createdAt,
    operation_name: operationName,
    reason,
    risk_level: riskLevel,
    status: "pending",
    target_object: targetObject,
    task_id: taskId,
  };
}

const baseTasks: Task[] = [
  {
    task_id: "task_focus_001",
    title: "把任务页整理成可爱又克制的云岛剧场",
    source_type: "dragged_file",
    status: "processing",
    intent: { name: "design_task_page", arguments: { mood: "soft_future" } },
    current_step: "polish_main_panel",
    risk_level: "green",
    started_at: iso(-6 * HOUR),
    updated_at: iso(-10 * 60 * 1000),
    finished_at: null,
  },
  {
    task_id: "task_focus_002",
    title: "补齐安全详情中的授权说明与恢复点解释",
    source_type: "selected_text",
    status: "waiting_auth",
    intent: { name: "explain_security_context", arguments: { focus: "approval" } },
    current_step: "authorization_pending",
    risk_level: "yellow",
    started_at: iso(-11 * HOUR),
    updated_at: iso(-35 * 60 * 1000),
    finished_at: null,
  },
  {
    task_id: "task_focus_003",
    title: "把最近便签整理成任务页所需的上下文提示",
    source_type: "todo",
    status: "paused",
    intent: { name: "organize_notes", arguments: { output: "context" } },
    current_step: "collect_notes",
    risk_level: "green",
    started_at: iso(-16 * HOUR),
    updated_at: iso(-2 * HOUR),
    finished_at: null,
  },
  {
    task_id: "task_done_001",
    title: "输出铃兰首页的交互构图和动效版本",
    source_type: "dragged_file",
    status: "completed",
    intent: { name: "compose_dashboard_home", arguments: { motif: "lily" } },
    current_step: "delivery_ready",
    risk_level: "green",
    started_at: iso(-2 * DAY),
    updated_at: iso(-26 * HOUR),
    finished_at: iso(-24 * HOUR),
  },
  {
    task_id: "task_done_002",
    title: "批量改写旧任务原型并整理回收项",
    source_type: "hover_input",
    status: "cancelled",
    intent: { name: "cleanup_legacy_task", arguments: { range: "prototype" } },
    current_step: "cancelled_by_user",
    risk_level: "yellow",
    started_at: iso(-3 * DAY),
    updated_at: iso(-46 * HOUR),
    finished_at: iso(-41 * HOUR),
  },
  {
    task_id: "task_done_003",
    title: "整理安全审批 mock 数据与展示口径",
    source_type: "selected_text",
    status: "completed",
    intent: { name: "prepare_security_mock", arguments: { source: "dashboard" } },
    current_step: "delivery_ready",
    risk_level: "green",
    started_at: iso(-6 * DAY),
    updated_at: iso(-5 * DAY + 3 * HOUR),
    finished_at: iso(-5 * DAY),
  },
  {
    task_id: "task_done_004",
    title: "梳理镜子页摘要卡片与漂浮信息层",
    source_type: "todo",
    status: "completed",
    intent: { name: "refine_mirror_cards", arguments: { density: "soft" } },
    current_step: "delivery_ready",
    risk_level: "green",
    started_at: iso(-8 * DAY),
    updated_at: iso(-6 * DAY + 5 * HOUR),
    finished_at: iso(-6 * DAY),
  },
  {
    task_id: "task_done_005",
    title: "导出悬浮球演示截图并归档素材索引",
    source_type: "dragged_file",
    status: "completed",
    intent: { name: "export_shell_ball_assets", arguments: { format: "png" } },
    current_step: "delivery_ready",
    risk_level: "green",
    started_at: iso(-13 * DAY),
    updated_at: iso(-10 * DAY + 4 * HOUR),
    finished_at: iso(-10 * DAY),
  },
];

const taskExperiences: Record<string, TaskExperience> = {
  task_focus_001: {
    priority: "critical",
    dueAt: iso(DAY),
    goal: "把当前任务页整理成一个聚焦推进单任务的云岛剧场，让人第一眼就知道正在做什么、下一步该怎么推进。",
    phase: "正在把详情、子任务、产出和笔记四个内容层整理成一组连贯舱室。",
    nextAction: "先把主任务区的呼吸感和 tabs 内容面板定型，再回头收细底部操作键与成果区。",
    progressHint: "当前正在打磨主任务区，离可交付版本还差产出区与动作区收口。",
    background: "这页不是后台，也不是看板，而是给当前任务提供一个沉浸式推进舱。所有信息都应围绕“此刻这一个任务”组织。",
    constraints: [
      "保持暖白 + 蓝灰冷调，不要出现赛博朋克式高饱和霓虹。",
      "布局必须优先照顾当前任务推进，而不是做任务管理总览。",
      "长内容只在局部滚动，不能让整页变成又长又硬的企业系统。",
    ],
    acceptance: [
      "顶部有可读性很强的任务头部胶囊。",
      "中间左主区能在同一视线里看到目标、tabs 和当前推进状态。",
      "右侧辅助区像陪伴脑区，而不是系统侧栏。",
    ],
    noteDraft:
      "- 已确认视觉关键词：柔软未来感、雾面玻璃、半透明、安静克制。\n- 当前决定把任务记录做成时间折叠舱，不使用传统列表表格。\n- 产出区要同时容纳草稿、结果和可继续编辑内容。",
    noteEntries: [
      "决定把“更多已结束任务”放在页面内展开，而不是新开文件夹或跳页。",
      "继续保留与悬浮球同源的暖白外壳和灰蓝边缘高光。",
      "对真实协议缺失的“修改任务”“打开产出”等动作，先用低打扰降级交互。",
    ],
    relatedFiles: [
      {
        id: "file_001",
        kind: "tsx",
        note: "任务页主容器与数据编排入口。",
        path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx",
        title: "TaskPage.tsx",
      },
      {
        id: "file_002",
        kind: "tsx",
        note: "任务页 tabs 与内容面板。",
        path: "apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx",
        title: "TaskTabsPanel.tsx",
      },
      {
        id: "file_003",
        kind: "json",
        note: "用于本地联调的任务页模拟数据。",
        path: "apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts",
        title: "taskPage.mock.ts",
      },
    ],
    quickContext: [
      { id: "ctx_001", label: "最近对话", content: "任务页必须像有呼吸感的云岛剧场，而不是企业后台。" },
      { id: "ctx_002", label: "用户约束", content: "可爱但不幼稚，炫酷但不赛博朋克。" },
      { id: "ctx_003", label: "关键提醒", content: "底部动作区要像软胶按键，不能做成普通工具栏。" },
    ],
    recentConversation: [
      "把未完成和已结束任务都收进页面，但重点只推进当前这一个。",
      "已结束任务只在页面内展开更多，不再打开独立文件夹。",
      "镜子和安全已经是 dashboard 子页，任务页也要维持同样的沉浸质感。",
    ],
    suggestedNext: "切到“产出内容”页签，把草稿和结果先并列看一眼，再决定是否继续生成。",
    assistantState: {
      label: "正在思考",
      hint: "我在整理当前任务的四个内容舱，并让底部动作条和右侧辅助区互相呼应。",
    },
    outputs: [
      {
        id: "output_001",
        label: "当前草稿",
        content: "任务页目前已经建立顶部任务胶囊、中部主辅双区和底部动作条三层骨架，还在打磨 tabs 与成果区。",
        tone: "draft",
      },
      {
        id: "output_002",
        label: "已生成结果",
        content: "已完成的结果应突出关键操作摘要、成果入口和重启动作，而不是继续展示执行中语气。",
        tone: "result",
      },
      {
        id: "output_003",
        label: "可继续编辑",
        content: "把子任务卡与右侧辅助卡建立轻联动，点击某个子任务时，让陪伴区产生一层柔和高亮。",
        tone: "editable",
      },
    ],
    stepTargets: {
      step_focus_001: "context",
      step_focus_002: "agent",
      step_focus_003: "files",
      step_focus_004: "context",
      step_focus_005: "agent",
    },
  },
  task_focus_002: {
    priority: "high",
    dueAt: iso(8 * HOUR),
    goal: "把安全详情中的授权原因、影响面和恢复点讲清楚，让用户知道为什么当前任务停在这里。",
    phase: "等待授权确认返回后继续推进说明卡与恢复点卡。",
    nextAction: "优先补充授权对象与原因摘要，再刷新风险提示语气。",
    progressHint: "现在卡在授权确认节点，核心问题不是布局，而是解释是否清楚。",
    background: "当前任务与安全模块直接关联，需要在主任务视角里高亮停顿原因。",
    constraints: ["不能越过授权节点直接执行。", "说明要简短但足够明确。", "恢复点必须保持可追踪。"],
    acceptance: ["用户能一眼看懂为什么等待授权。", "能快速跳去安全详情。", "恢复点有清晰摘要。"],
    noteDraft: "- 授权原因要放在比操作按钮更靠前的位置。\n- 风险说明保持短句，避免制造额外焦虑。",
    noteEntries: ["需要把等待授权卡做得更显眼。", "恢复点摘要要跟任务名绑在一起。"],
    relatedFiles: [
      {
        id: "file_004",
        kind: "tsx",
        note: "安全子页主题壳。",
        path: "apps/desktop/src/features/dashboard/safety/SecurityPageShell.tsx",
        title: "SecurityPageShell.tsx",
      },
      {
        id: "file_005",
        kind: "ts",
        note: "安全 mock 与 rpc fallback。",
        path: "apps/desktop/src/features/dashboard/safety/securityService.ts",
        title: "securityService.ts",
      },
      {
        id: "file_006",
        kind: "ts",
        note: "安全 mock 数据。",
        path: "apps/desktop/src/features/dashboard/safety/securityModuleMock.ts",
        title: "securityModuleMock.ts",
      },
    ],
    quickContext: [
      { id: "ctx_004", label: "等待原因", content: "这项任务正在等用户确认是否允许查看更多安全明细。" },
      { id: "ctx_005", label: "相关子页", content: "可从当前任务页直接跳去安全子页查看详细挂起项。" },
      { id: "ctx_006", label: "建议动作", content: "一旦授权通过，优先刷新风险摘要和恢复点呈现。" },
    ],
    recentConversation: ["优先让等待原因解释清楚。", "先高亮授权，再展示其它次要信息。", "恢复点必须可追踪。"],
    suggestedNext: "点开安全详情，确认待授权项是否已经返回最新状态。",
    assistantState: {
      label: "待命",
      hint: "我在等授权信号回来，一旦放行就继续补全说明层。",
    },
    outputs: [
      { id: "output_004", label: "当前草稿", content: "安全详情卡片已有骨架，但等待原因文案还需要再收束。", tone: "draft" },
      { id: "output_005", label: "已生成结果", content: "恢复点与授权摘要已经能独立显示。", tone: "result" },
      { id: "output_006", label: "可继续编辑", content: "把风险卡的提示语改成更克制的口吻。", tone: "editable" },
    ],
    stepTargets: {
      step_wait_001: "agent",
      step_wait_002: "context",
      step_wait_003: "files",
      step_wait_004: "context",
    },
    waitingReason: "当前任务已进入等待授权状态，停在“查看安全详情”这一步，需要用户明确放行后才能继续读取更多授权细节。",
  },
  task_focus_003: {
    priority: "steady",
    dueAt: iso(2 * DAY),
    goal: "把最近便签里可复用的约束、提醒与对话提炼成任务页右侧的轻上下文卡。",
    phase: "暂停在整理 note 与 context 的映射关系。",
    nextAction: "恢复后先筛掉噪声内容，再写成更短的提示句。",
    progressHint: "这项任务暂时停着，但一恢复就能很快补齐右侧陪伴区。",
    background: "目标是让右侧辅助区真正像陪伴脑区，而不是普通侧栏。",
    constraints: ["每条上下文必须短。", "不要重复主任务区已经表达的信息。", "保持语气柔和。"],
    acceptance: ["右侧提示不会太吵。", "最近对话能为当前任务提供推进线索。", "推荐下一步自然可信。"],
    noteDraft: "- 暂停原因：还没把便签来源统一映射。",
    noteEntries: ["继续时优先做内容去重。"],
    relatedFiles: [
      { id: "file_007", kind: "tsx", note: "任务页辅助区组件。", path: "apps/desktop/src/features/dashboard/tasks/components/TaskAssistantPanel.tsx", title: "TaskAssistantPanel.tsx" },
      { id: "file_008", kind: "tsx", note: "任务页 mock 数据。", path: "apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts", title: "taskPage.mock.ts" },
      { id: "file_009", kind: "tsx", note: "任务页底部操作条。", path: "apps/desktop/src/features/dashboard/tasks/components/TaskBottomActions.tsx", title: "TaskBottomActions.tsx" },
    ],
    quickContext: [
      { id: "ctx_007", label: "暂停原因", content: "上下文卡与便签的映射关系还没收敛。" },
      { id: "ctx_008", label: "关键提醒", content: "辅助区内容必须短，不要像第二个主区。" },
      { id: "ctx_009", label: "下一步", content: "恢复后优先整理最近对话与推荐动作。" },
    ],
    recentConversation: ["右侧卡片不要像系统侧栏。", "内容必须短句。", "推荐动作要可信。"],
    suggestedNext: "恢复任务后，先把右侧快速上下文卡里的三条提醒写成更短的版本。",
    assistantState: {
      label: "待命",
      hint: "暂停中的任务已经沉淀了上下文方向，只等下一次继续。",
    },
    outputs: [
      { id: "output_007", label: "当前草稿", content: "右侧辅助区正在重写，以便更贴近陪伴脑区而不是传统侧边栏。", tone: "draft" },
      { id: "output_008", label: "已生成结果", content: "Agent 状态卡和文件卡已确定为独立小舱。", tone: "result" },
      { id: "output_009", label: "可继续编辑", content: "把最近对话和关键提醒再压缩成两句以内。", tone: "editable" },
    ],
    stepTargets: {
      step_pause_001: "context",
      step_pause_002: "files",
      step_pause_003: "context",
    },
    waitingReason: "当前任务手动暂停，目的是先让主任务页骨架稳定，再回来整理右侧的轻上下文内容。",
  },
  task_done_001: {
    priority: "high",
    dueAt: null,
    goal: "完成铃兰首页的构图、拖拽与导航交互。",
    phase: "任务已收束，等待后续复用。",
    nextAction: "如需复用这套风格，可以把纸感背景和柔光层提炼成共享样式。",
    progressHint: "这项任务已经结束，可以回看成果或重启一轮。",
    background: "铃兰首页承担 dashboard 的入口视觉，需要兼顾自然感与可交互性。",
    constraints: ["保持克制。", "不能影响 dashboard 子页。", "拖拽只作用于枝干层。"],
    acceptance: ["首页跳转清晰。", "动效柔和。", "拖拽与 hover 都能正常工作。"],
    noteDraft: "铃兰入口已经稳定，可按需复用主题层。",
    noteEntries: ["产出内容建议整理进共享视觉片段。"],
    relatedFiles: [
      { id: "file_010", kind: "tsx", note: "铃兰首页。", path: "apps/desktop/src/app/dashboard/DashboardHome.tsx", title: "DashboardHome.tsx" },
      { id: "file_011", kind: "css", note: "铃兰首页样式。", path: "apps/desktop/src/app/dashboard/dashboard.css", title: "dashboard.css" },
      { id: "file_012", kind: "png", note: "铃兰素材。", path: "apps/desktop/src/assets/lily-of-the-valley", title: "lily-of-the-valley" },
    ],
    quickContext: [
      { id: "ctx_010", label: "结果摘要", content: "首页已支持花朵导航、枝干拖拽和 ClickSpark。" },
      { id: "ctx_011", label: "关键提醒", content: "保持枝干与叶片根部在屏幕外。" },
      { id: "ctx_012", label: "后续建议", content: "如果要继续演进，可给铃兰布局补更多窗口适配。" },
    ],
    recentConversation: ["铃兰首页现在是 dashboard 入口。", "所有子页都从这里进入。", "未来可以继续做窗口自适应。"],
    suggestedNext: "如果要复用这套风格，先拆分视觉 token 和素材定位配置。",
    assistantState: { label: "刚完成一步", hint: "这条任务已经完成，成果可直接回看。" },
    outputs: [
      { id: "output_010", label: "当前草稿", content: "无进行中草稿。", tone: "draft" },
      { id: "output_011", label: "已生成结果", content: "铃兰首页已具备完整的导航、拖拽与点击特效。", tone: "result" },
      { id: "output_012", label: "可继续编辑", content: "下一轮可继续收窗口适配和花朵位置细修。", tone: "editable" },
    ],
    stepTargets: {
      step_done_001: "files",
      step_done_002: "context",
      step_done_003: "agent",
      step_done_004: "files",
      step_done_005: "context",
    },
    endedSummary: "首页已经完成交付，重点是导航、拖拽和素材布局。",
  },
  task_done_002: {
    priority: "steady",
    dueAt: null,
    goal: "收束旧任务原型，避免它继续干扰当前 dashboard 结构。",
    phase: "已取消。",
    nextAction: "如需继续，可重新梳理哪些旧文件值得保留。",
    progressHint: "当前任务已取消，不再继续推进。",
    background: "旧原型残留较多，任务中途确认不应继续扩大重构范围。",
    constraints: ["不误删仍在使用的代码。"],
    acceptance: ["取消后仍可追溯。"],
    noteDraft: "取消原因：重构范围过大，不适合当前节奏。",
    noteEntries: ["如果重启，要先做引用审计。"],
    relatedFiles: [
      { id: "file_013", kind: "tsx", note: "旧版 dashboard 原型。", path: "apps/desktop/src/features/dashboard/DashboardApp.tsx", title: "DashboardApp.tsx" },
      { id: "file_014", kind: "ts", note: "旧任务 view model。", path: "apps/desktop/src/models/TaskDetailViewModel.ts", title: "TaskDetailViewModel.ts" },
      { id: "file_015", kind: "ts", note: "旧任务 store。", path: "apps/desktop/src/stores/taskStore.ts", title: "taskStore.ts" },
    ],
    quickContext: [
      { id: "ctx_013", label: "取消原因", content: "范围太大，当前阶段不适合继续做结构性清理。" },
      { id: "ctx_014", label: "后续建议", content: "重新开始前先做引用审计。" },
      { id: "ctx_015", label: "风险提示", content: "避免误删仍在使用的页面入口。" },
    ],
    recentConversation: ["这轮先做新增，不重构。", "旧 prototype 暂不扩大处理。"],
    suggestedNext: "如果要重启，先确认哪些旧文件真的无人使用。",
    assistantState: { label: "待命", hint: "取消任务后保持现状，等待新的整理范围。" },
    outputs: [
      { id: "output_013", label: "当前草稿", content: "无。", tone: "draft" },
      { id: "output_014", label: "已生成结果", content: "留下了一份可供后续重启使用的整理清单。", tone: "result" },
      { id: "output_015", label: "可继续编辑", content: "补一份引用关系图后再重启更稳。", tone: "editable" },
    ],
    stepTargets: {
      step_cancel_001: "context",
      step_cancel_002: "agent",
      step_cancel_003: "files",
    },
    endedSummary: "任务被取消，保留了一份后续重启时可复用的整理思路。",
  },
  task_done_003: {
    priority: "steady",
    dueAt: null,
    goal: "把安全 mock 数据整理成可用于 dashboard 子页展示的一套稳定口径。",
    phase: "已完成。",
    nextAction: "后续可以继续接入真实审批流。",
    progressHint: "当前任务已经稳定收束。",
    background: "安全页需要在无 RPC 时也能联调。",
    constraints: ["mock 结构必须贴近协议。"],
    acceptance: ["安全页无 RPC 时仍可展示。"],
    noteDraft: "安全 mock 已经可供子页直接使用。",
    noteEntries: ["后续接真实审批流时要保留同样的数据口径。"],
    relatedFiles: [
      { id: "file_016", kind: "ts", note: "安全 mock 数据。", path: "apps/desktop/src/features/dashboard/safety/securityModuleMock.ts", title: "securityModuleMock.ts" },
      { id: "file_017", kind: "ts", note: "安全 service。", path: "apps/desktop/src/features/dashboard/safety/securityService.ts", title: "securityService.ts" },
      { id: "file_018", kind: "tsx", note: "安全页入口壳。", path: "apps/desktop/src/features/dashboard/safety/SafetyPage.tsx", title: "SafetyPage.tsx" },
    ],
    quickContext: [
      { id: "ctx_016", label: "结果摘要", content: "安全 mock 已经贴近协议，并能支撑子页联调。" },
      { id: "ctx_017", label: "提醒", content: "后续接真实审批流时不要改坏现有 mock 结构。" },
      { id: "ctx_018", label: "推荐下一步", content: "继续补 task 页中的风险摘要联动。" },
    ],
    recentConversation: ["安全页已经并入 dashboard 子页。", "mock 仍要保留。"],
    suggestedNext: "接下来把 task 页中的风险摘要和安全页对齐。",
    assistantState: { label: "刚完成一步", hint: "安全子页的数据底座已经搭好。" },
    outputs: [
      { id: "output_016", label: "当前草稿", content: "无。", tone: "draft" },
      { id: "output_017", label: "已生成结果", content: "安全页可在无后端时展示审批、恢复点与预算摘要。", tone: "result" },
      { id: "output_018", label: "可继续编辑", content: "后续把 task 页的风险卡也改成同样口径。", tone: "editable" },
    ],
    stepTargets: {
      step_security_001: "files",
      step_security_002: "agent",
      step_security_003: "context",
    },
    endedSummary: "安全 mock 和 service 已稳定，可直接复用到 dashboard 子页。",
  },
  task_done_004: {
    priority: "steady",
    dueAt: null,
    goal: "把镜子页的卡片布局、摘要层和局部拖拽体验打磨到可演示状态。",
    phase: "已完成。",
    nextAction: "后续可以继续把镜子页里的气味提炼到任务页。",
    progressHint: "这条任务已结束。",
    background: "镜子页现在作为 dashboard 子页提供真实能力页面。",
    constraints: ["仍需保持轻质纸感。"],
    acceptance: ["镜子页在 dashboard 内显示正常。"],
    noteDraft: "镜子页交互已稳定。",
    noteEntries: ["未来可提炼更多共享视觉 token。"],
    relatedFiles: [
      { id: "file_019", kind: "tsx", note: "镜子页主体。", path: "apps/desktop/src/features/dashboard/memory/MirrorApp.tsx", title: "MirrorApp.tsx" },
      { id: "file_020", kind: "ts", note: "镜子页 service。", path: "apps/desktop/src/features/dashboard/memory/mirrorService.ts", title: "mirrorService.ts" },
      { id: "file_021", kind: "json", note: "镜子页 mock。", path: "apps/desktop/src/features/dashboard/memory/mirrorOverview.json", title: "mirrorOverview.json" },
    ],
    quickContext: [
      { id: "ctx_019", label: "结果摘要", content: "镜子页已经变成 dashboard 内的真实子页。" },
      { id: "ctx_020", label: "提醒", content: "保持纸感与未来感，不要把视觉做得过硬。" },
      { id: "ctx_021", label: "推荐下一步", content: "任务页可借用镜子页的轻质渐变与留白控制。" },
    ],
    recentConversation: ["镜子页已移入 dashboard/memory。", "独立入口已经删除。"],
    suggestedNext: "把镜子页的纸感和信息密度控制借给任务页。",
    assistantState: { label: "刚完成一步", hint: "镜子页已经作为真实子页稳定挂在 dashboard 下。" },
    outputs: [
      { id: "output_019", label: "当前草稿", content: "无。", tone: "draft" },
      { id: "output_020", label: "已生成结果", content: "镜子页的卡片布局和摘要层已经完成。", tone: "result" },
      { id: "output_021", label: "可继续编辑", content: "下一轮可以提取共享的纸感 token。", tone: "editable" },
    ],
    stepTargets: {
      step_mirror_001: "files",
      step_mirror_002: "context",
      step_mirror_003: "agent",
    },
    endedSummary: "镜子页已经稳定并入 dashboard，可继续作为任务页的风格参照。",
  },
  task_done_005: {
    priority: "steady",
    dueAt: null,
    goal: "把悬浮球的演示截图和说明资料整理出来，方便后续页面保持同一气质。",
    phase: "已完成。",
    nextAction: "如果还要沿用这套气质，可以继续提炼视觉关键词。",
    progressHint: "这条任务已经交付。",
    background: "悬浮球是当前整体风格的参考物。",
    constraints: ["保持可爱但不幼稚。"],
    acceptance: ["素材整理可复用。"],
    noteDraft: "悬浮球已成为任务页的视觉参照。",
    noteEntries: ["继续沿用暖白 + 蓝灰 + 少量暖色点缀。"],
    relatedFiles: [
      { id: "file_022", kind: "tsx", note: "悬浮球主页面。", path: "apps/desktop/src/features/shell-ball/ShellBallApp.tsx", title: "ShellBallApp.tsx" },
      { id: "file_023", kind: "css", note: "悬浮球样式。", path: "apps/desktop/src/features/shell-ball/shellBall.css", title: "shellBall.css" },
      { id: "file_024", kind: "tsx", note: "悬浮球主体组件。", path: "apps/desktop/src/features/shell-ball/components/ShellBallMascot.tsx", title: "ShellBallMascot.tsx" },
    ],
    quickContext: [
      { id: "ctx_022", label: "结果摘要", content: "悬浮球的暖白、柔和高光和蓝灰边缘是任务页的参考。" },
      { id: "ctx_023", label: "提醒", content: "不要把任务页做成硬质科技舱。" },
      { id: "ctx_024", label: "推荐下一步", content: "沿用软胶按键和雾面玻璃的轻质表达。" },
    ],
    recentConversation: ["任务页要和悬浮球属于同一世界观。", "可爱但要克制。"],
    suggestedNext: "把悬浮球的软质按钮感迁到任务页底部动作条。",
    assistantState: { label: "待命", hint: "视觉参考已经准备好，接下来重点是落地云岛剧场页面。" },
    outputs: [
      { id: "output_022", label: "当前草稿", content: "无。", tone: "draft" },
      { id: "output_023", label: "已生成结果", content: "悬浮球的世界观资料已经可直接参考。", tone: "result" },
      { id: "output_024", label: "可继续编辑", content: "可提炼更系统化的未来感软质 token。", tone: "editable" },
    ],
    stepTargets: {
      step_shell_001: "context",
      step_shell_002: "files",
      step_shell_003: "agent",
    },
    endedSummary: "悬浮球已经提供了足够清晰的风格锚点。",
  },
};

function createDetail(
  task: Task,
  timeline: TaskStep[],
  artifacts: AgentTaskDetailGetResult["artifacts"],
  securitySummary: AgentTaskDetailGetResult["security_summary"],
  mirrorReferences: AgentTaskDetailGetResult["mirror_references"],
  approvalRequest: AgentTaskDetailGetResult["approval_request"] = null,
  runtimeSummary: TaskRuntimeSummary = {
    active_steering_count: 0,
    events_count: timeline.length,
    latest_failure_code: null,
    latest_failure_category: null,
    latest_failure_summary: null,
    latest_event_type: timeline[timeline.length - 1]?.status === "completed" ? "step.completed" : "step.updated",
    loop_stop_reason: task.status === "completed" || task.status === "cancelled" || task.status === "ended_unfinished" ? "task_settled" : null,
    observation_signals: [],
  },
) {
  return {
    approval_request: approvalRequest,
    audit_record: null,
    artifacts,
    authorization_record: null,
    citations: [],
    delivery_result: null,
    mirror_references: mirrorReferences,
    runtime_summary: runtimeSummary,
    security_summary: securitySummary,
    task,
    timeline,
  } satisfies AgentTaskDetailGetResult;
}

const baseDetails: Record<string, AgentTaskDetailGetResult> = {
  task_focus_001: createDetail(
    baseTasks[0],
    [
      { step_id: "step_focus_001", task_id: "task_focus_001", name: "梳理页面目标与冻结边界", status: "completed", order_index: 1, input_summary: "整理页面目标、组件约束和整体气质。", output_summary: "确认任务页不走后台式布局。" },
      { step_id: "step_focus_002", task_id: "task_focus_001", name: "搭建顶部任务胶囊与主辅分区", status: "completed", order_index: 2, input_summary: "根据页面目标搭出固定布局框架。", output_summary: "顶部胶囊与左右分区已经形成。" },
      { step_id: "step_focus_003", task_id: "task_focus_001", name: "实现 tabs 与内容面板切换", status: "running", order_index: 3, input_summary: "开始接入详情、子任务、产出和笔记四个内容舱。", output_summary: "当前正在打磨 tabs 与内容层联动。" },
      { step_id: "step_focus_004", task_id: "task_focus_001", name: "补齐任务操作与信任摘要", status: "pending", order_index: 4, input_summary: "准备接上暂停、继续、取消与信任摘要。", output_summary: "等待主内容区稳定后继续。" },
      { step_id: "step_focus_005", task_id: "task_focus_001", name: "校验交互、构建并准备交付", status: "pending", order_index: 5, input_summary: "等待前四步完成后统一走验证。", output_summary: "尚未开始。" },
    ],
    [
      { artifact_id: "artifact_focus_001", task_id: "task_focus_001", artifact_type: "workspace_document", title: "TaskPage.tsx", path: "apps/desktop/src/features/dashboard/tasks/TaskPage.tsx", mime_type: "text/tsx" },
      { artifact_id: "artifact_focus_002", task_id: "task_focus_001", artifact_type: "workspace_document", title: "taskPage.mock.ts", path: "apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts", mime_type: "text/ts" },
      { artifact_id: "artifact_focus_003", task_id: "task_focus_001", artifact_type: "workspace_document", title: "TaskTabsPanel.tsx", path: "apps/desktop/src/features/dashboard/tasks/components/TaskTabsPanel.tsx", mime_type: "text/tsx" },
    ],
    { security_status: "normal", risk_level: "green", pending_authorizations: 0, latest_restore_point: null },
    [
      { memory_id: "mem_task_page_soft_future", reason: "任务页需要继承悬浮球的暖白与蓝灰冷调。", summary: "保持软胶质感、磨砂玻璃和低打扰留白。" },
      { memory_id: "mem_task_page_single_focus", reason: "任务页应聚焦推进当前一个任务，而不是管理一堆任务。", summary: "用中心舞台 + 漂浮任务岛组织信息，而不是后台式双栏表格。" },
      { memory_id: "mem_task_page_quiet_motion", reason: "动效必须克制，高级，不可出现夸张霓虹与过重 bounce。", summary: "主任务区轻呼吸、tabs 淡入位移、Sheet 像舱门滑开即可。" },
    ],
  ),
  task_focus_002: createDetail(
    baseTasks[1],
    [
      { step_id: "step_wait_001", task_id: "task_focus_002", name: "收集授权原因与目标对象", status: "completed", order_index: 1, input_summary: "从审批数据中提炼目标对象与原因。", output_summary: "已收集待展示字段。" },
      { step_id: "step_wait_002", task_id: "task_focus_002", name: "整理恢复点说明", status: "completed", order_index: 2, input_summary: "确认恢复点与任务的映射方式。", output_summary: "已准备恢复点摘要。" },
      { step_id: "step_wait_003", task_id: "task_focus_002", name: "等待授权返回", status: "running", order_index: 3, input_summary: "用户确认是否允许读取更多审批上下文。", output_summary: "当前停留在授权节点。" },
      { step_id: "step_wait_004", task_id: "task_focus_002", name: "回填安全说明卡", status: "pending", order_index: 4, input_summary: "等待授权通过后继续。", output_summary: "尚未开始。" },
    ],
    [
      { artifact_id: "artifact_wait_001", task_id: "task_focus_002", artifact_type: "workspace_document", title: "SecurityApp.tsx", path: "apps/desktop/src/features/dashboard/safety/SecurityApp.tsx", mime_type: "text/tsx" },
      { artifact_id: "artifact_wait_002", task_id: "task_focus_002", artifact_type: "workspace_document", title: "securityService.ts", path: "apps/desktop/src/features/dashboard/safety/securityService.ts", mime_type: "text/ts" },
    ],
    {
      security_status: "pending_confirmation",
      risk_level: "yellow",
      pending_authorizations: 1,
      latest_restore_point: createRecoveryPoint(
        "task_focus_002",
        "rp_task_focus_002",
        "授权前已保存安全详情回滚锚点。",
        iso(-50 * 60 * 1000),
        ["apps/desktop/src/features/dashboard/safety/securityService.ts"],
      ),
    },
    [
      { memory_id: "mem_security_authorization_first", reason: "等待授权态需要优先解释停顿原因。", summary: "先写明原因，再展示次要信息。" },
      { memory_id: "mem_security_restore_point", reason: "恢复点应与任务 ID 保持可追踪映射。", summary: "恢复点和任务摘要应出现在同一视野。" },
    ],
    createApprovalRequest(
      "task_focus_002",
      "approval_task_focus_002",
      "yellow",
      "read_security_context",
      "security detail context",
      "需要继续读取当前任务的安全详情与恢复点说明。",
      iso(-42 * 60 * 1000),
    ),
  ),
  task_focus_003: createDetail(
    baseTasks[2],
    [
      { step_id: "step_pause_001", task_id: "task_focus_003", name: "整理最近便签", status: "completed", order_index: 1, input_summary: "筛出和任务页相关的便签。", output_summary: "已初步完成筛选。" },
      { step_id: "step_pause_002", task_id: "task_focus_003", name: "映射为右侧上下文卡", status: "running", order_index: 2, input_summary: "准备把便签压缩成短句提示。", output_summary: "任务当前手动暂停。" },
      { step_id: "step_pause_003", task_id: "task_focus_003", name: "回填推荐下一步", status: "pending", order_index: 3, input_summary: "等待恢复后继续。", output_summary: "尚未开始。" },
    ],
    [
      { artifact_id: "artifact_pause_001", task_id: "task_focus_003", artifact_type: "workspace_document", title: "TaskAssistantPanel.tsx", path: "apps/desktop/src/features/dashboard/tasks/components/TaskAssistantPanel.tsx", mime_type: "text/tsx" },
    ],
    { security_status: "normal", risk_level: "green", pending_authorizations: 0, latest_restore_point: null },
    [
      { memory_id: "mem_context_short_copy", reason: "右侧辅助区需要短句、低干扰。", summary: "每条上下文都不要超过两句。" },
    ],
  ),
  task_done_001: createDetail(
    baseTasks[3],
    [
      { step_id: "step_done_001", task_id: "task_done_001", name: "整理铃兰视觉草图", status: "completed", order_index: 1, input_summary: "确认构图基准。", output_summary: "完成首页主构图。" },
      { step_id: "step_done_002", task_id: "task_done_001", name: "接入花朵导航与拖拽", status: "completed", order_index: 2, input_summary: "把 4 朵花接成导航入口。", output_summary: "交互已完成。" },
      { step_id: "step_done_003", task_id: "task_done_001", name: "补充 spark 与 hover 反馈", status: "completed", order_index: 3, input_summary: "增加轻量交互反馈。", output_summary: "spark 与 hover 已稳定。" },
      { step_id: "step_done_004", task_id: "task_done_001", name: "完成素材和位置微调", status: "completed", order_index: 4, input_summary: "把枝条、叶片、花朵定位到位。", output_summary: "素材布局已收口。" },
      { step_id: "step_done_005", task_id: "task_done_001", name: "构建与验证", status: "completed", order_index: 5, input_summary: "执行 typecheck/build。", output_summary: "验证通过。" },
    ],
    [
      { artifact_id: "artifact_done_001", task_id: "task_done_001", artifact_type: "workspace_document", title: "DashboardHome.tsx", path: "apps/desktop/src/app/dashboard/DashboardHome.tsx", mime_type: "text/tsx" },
      { artifact_id: "artifact_done_002", task_id: "task_done_001", artifact_type: "workspace_document", title: "dashboard.css", path: "apps/desktop/src/app/dashboard/dashboard.css", mime_type: "text/css" },
      { artifact_id: "artifact_done_003", task_id: "task_done_001", artifact_type: "reveal_in_folder", title: "铃兰素材目录", path: "apps/desktop/src/assets/lily-of-the-valley", mime_type: "inode/directory" },
    ],
    {
      security_status: "recovered",
      risk_level: "green",
      pending_authorizations: 0,
      latest_restore_point: createRecoveryPoint(
        "task_done_001",
        "rp_lily_home_001",
        "首页构图定版前的布局恢复点。",
        iso(-27 * HOUR),
        ["apps/desktop/src/app/dashboard/DashboardHome.tsx", "apps/desktop/src/app/dashboard/dashboard.css"],
      ),
    },
    [
      { memory_id: "mem_lily_entry_consistency", reason: "首页风格应与任务页保持同一气质。", summary: "自然、轻柔、留白、低打扰。" },
    ],
  ),
  task_done_002: createDetail(
    baseTasks[4],
    [
      { step_id: "step_cancel_001", task_id: "task_done_002", name: "收集旧文件引用", status: "completed", order_index: 1, input_summary: "先梳理旧 prototype 文件。", output_summary: "完成初步清点。" },
      { step_id: "step_cancel_002", task_id: "task_done_002", name: "评估删除边界", status: "cancelled", order_index: 2, input_summary: "确认整理范围。", output_summary: "任务在这里被取消。" },
      { step_id: "step_cancel_003", task_id: "task_done_002", name: "批量清理旧原型", status: "skipped", order_index: 3, input_summary: "未继续执行。", output_summary: "跳过。" },
    ],
    [
      { artifact_id: "artifact_cancel_001", task_id: "task_done_002", artifact_type: "workspace_document", title: "DashboardApp.tsx", path: "apps/desktop/src/features/dashboard/DashboardApp.tsx", mime_type: "text/tsx" },
    ],
    { security_status: "normal", risk_level: "yellow", pending_authorizations: 0, latest_restore_point: null },
    [],
  ),
  task_done_003: createDetail(
    baseTasks[5],
    [
      { step_id: "step_security_001", task_id: "task_done_003", name: "定义安全 mock 结构", status: "completed", order_index: 1, input_summary: "确定 summary/pending 结构。", output_summary: "结构与协议对齐。" },
      { step_id: "step_security_002", task_id: "task_done_003", name: "补充 service fallback", status: "completed", order_index: 2, input_summary: "让无 RPC 时仍有联调数据。", output_summary: "fallback 可用。" },
      { step_id: "step_security_003", task_id: "task_done_003", name: "接入 dashboard 子页", status: "completed", order_index: 3, input_summary: "把安全页收进 dashboard。", output_summary: "dashboard/safety 已完成。" },
    ],
    [
      { artifact_id: "artifact_security_001", task_id: "task_done_003", artifact_type: "workspace_document", title: "securityModuleMock.ts", path: "apps/desktop/src/features/dashboard/safety/securityModuleMock.ts", mime_type: "text/ts" },
      { artifact_id: "artifact_security_002", task_id: "task_done_003", artifact_type: "workspace_document", title: "securityService.ts", path: "apps/desktop/src/features/dashboard/safety/securityService.ts", mime_type: "text/ts" },
    ],
    {
      security_status: "normal",
      risk_level: "green",
      pending_authorizations: 0,
      latest_restore_point: createRecoveryPoint(
        "task_done_003",
        "rp_security_mock_001",
        "安全 mock 数据接入前的协议对齐快照。",
        iso(-5 * DAY + 2 * HOUR),
        ["apps/desktop/src/features/dashboard/safety/securityModuleMock.ts"],
      ),
    },
    [],
  ),
  task_done_004: createDetail(
    baseTasks[6],
    [
      { step_id: "step_mirror_001", task_id: "task_done_004", name: "收拢镜子页实现与 service", status: "completed", order_index: 1, input_summary: "移动页面与 service。", output_summary: "memory 子页归位。" },
      { step_id: "step_mirror_002", task_id: "task_done_004", name: "删除独立入口", status: "completed", order_index: 2, input_summary: "清理独立 mirror 入口。", output_summary: "只保留 dashboard 子页。" },
      { step_id: "step_mirror_003", task_id: "task_done_004", name: "验证 dashboard 内展示", status: "completed", order_index: 3, input_summary: "构建与浏览验证。", output_summary: "通过。" },
    ],
    [
      { artifact_id: "artifact_mirror_001", task_id: "task_done_004", artifact_type: "workspace_document", title: "MirrorApp.tsx", path: "apps/desktop/src/features/dashboard/memory/MirrorApp.tsx", mime_type: "text/tsx" },
      { artifact_id: "artifact_mirror_002", task_id: "task_done_004", artifact_type: "workspace_document", title: "mirrorService.ts", path: "apps/desktop/src/features/dashboard/memory/mirrorService.ts", mime_type: "text/ts" },
    ],
    { security_status: "recovered", risk_level: "green", pending_authorizations: 0, latest_restore_point: null },
    [],
  ),
  task_done_005: createDetail(
    baseTasks[7],
    [
      { step_id: "step_shell_001", task_id: "task_done_005", name: "导出演示截图", status: "completed", order_index: 1, input_summary: "整理悬浮球演示素材。", output_summary: "截图已导出。" },
      { step_id: "step_shell_002", task_id: "task_done_005", name: "归档视觉说明", status: "completed", order_index: 2, input_summary: "整理风格关键词。", output_summary: "风格说明已沉淀。" },
      { step_id: "step_shell_003", task_id: "task_done_005", name: "回传给任务页设计参考", status: "completed", order_index: 3, input_summary: "把悬浮球的气质作为任务页参考。", output_summary: "当前可直接复用。" },
    ],
    [
      { artifact_id: "artifact_shell_001", task_id: "task_done_005", artifact_type: "workspace_document", title: "shellBall.css", path: "apps/desktop/src/features/shell-ball/shellBall.css", mime_type: "text/css" },
      { artifact_id: "artifact_shell_002", task_id: "task_done_005", artifact_type: "workspace_document", title: "ShellBallApp.tsx", path: "apps/desktop/src/features/shell-ball/ShellBallApp.tsx", mime_type: "text/tsx" },
    ],
    { security_status: "normal", risk_level: "green", pending_authorizations: 0, latest_restore_point: null },
    [],
  ),
};

let mockTasksState = clone(baseTasks);
let mockDetailsState = clone(baseDetails);

export function getTaskExperience(taskId: string) {
  return taskExperiences[taskId];
}

/*
function createAdHocMockTask(taskId: string): Task {
  const nowIso = new Date().toISOString();

  return {
    current_step: "cross_module_focus",
    finished_at: null,
    intent: { name: "open_task_detail", arguments: { task_id: taskId } },
    risk_level: "green",
    source_type: "todo",
    started_at: nowIso,
    status: "processing",
    task_id: taskId,
    title: `鍏宠仈浠诲姟 ${taskId.slice(-4).toUpperCase()}`,
    updated_at: nowIso,
  };
}

function createAdHocMockExperience(task: Task): TaskExperience {
  return {
    acceptance: ["璺ㄦā鍧楄烦杞椂鍙互绋冲畾鎵撳紑浠诲姟璇︽儏銆?],
    assistantState: {
      hint: "褰撳墠浣跨敤鐨勬槸 mock 妯″紡涓嬬殑涓存椂浠诲姟璇︽儏锛岀敤鏉ユ壙鎺ヨ法妯″潡鐨勪换鍔¤烦杞€?,
      label: "detail focus",
    },
    background: "杩欐槸涓€鏉¤法妯″潡璺宠浆鍒颁换鍔￠〉鐨勪复鏃朵换鍔¤鍥撅紝鍏堜繚璇佽鎯呭彲鐢紝涓嶈鐣岄潰鍥炶惤鍒伴粯璁ょず渚嬨€?,
    constraints: ["涓嶆柊澧炲崗璁瓧娈点€?, "淇濇寔褰撳墠 task detail 鍜屽畨鍏ㄨ烦杞摼璺€?],
    dueAt: null,
    goal: task.title,
    nextAction: "鍏堟煡鐪嬪綋鍓嶄换鍔¤鎯咃紝鍐嶅喅瀹氭槸鍚︾户缁帹杩涖€?,
    noteDraft: "璇ヤ换鍔℃潵鑷叾浠栨ā鍧楃殑璺宠浆鑱氱劍锛屽綋鍓嶅厛浣跨敤鏈€灏忚鎯呰鏄庢壙鎺ャ€?,
    noteEntries: ["濡傛灉鍚庣画鎺ユ敹鍒版洿瀹屾暣鐨?task detail锛岄〉闈細鑷姩鍚屾銆?],
    outputs: [
      {
        id: `${task.task_id}_snapshot`,
        label: "褰撳墠蹇収",
        content: "杩欐潯浠诲姟鏆傛椂浠ユ渶灏忓彲鐢ㄨ鎯呭舰鎬佹壙鎺ヨ法妯″潡璺宠浆銆?,
        tone: "draft",
      },
    ],
    phase: "cross module detail focus",
    priority: "steady",
    progressHint: "璇ヤ换鍔¤繕娌℃湁鍥炲埌褰撳墠鍒楄〃锛屽厛浠ヨ鎯呮ā寮忔壙鎺ャ€?,
    quickContext: [
      { id: `${task.task_id}_ctx_source`, label: "鏉ユ簮", content: "褰撳墠浠诲姟鏄粠鍏朵粬 dashboard 妯″潡璺宠浆杩涙潵鐨勩€? },
    ],
    recentConversation: ["璇︽儏褰撳墠鐢?mock 妯″紡涓嬬殑涓存椂 task 鏁版嵁鎵挎帴銆?],
    relatedFiles: [],
    stepTargets: {},
    suggestedNext: "濡傛灉鍚庣画鍥炲埌浠诲姟鍒楄〃锛屽彲浠ユ巿鏉冩垨缁х画杩欐潯浠诲姟銆?,
  };
}

*/

function createAdHocMockTask(taskId: string): Task {
  const nowIso = new Date().toISOString();

  return {
    current_step: "cross_module_focus",
    finished_at: null,
    intent: { name: "open_task_detail", arguments: { task_id: taskId } },
    risk_level: "green",
    source_type: "todo",
    started_at: nowIso,
    status: "processing",
    task_id: taskId,
    title: `Linked task ${taskId.slice(-4).toUpperCase()}`,
    updated_at: nowIso,
  };
}

function createAdHocMockExperience(task: Task): TaskExperience {
  return {
    acceptance: ["Cross-module jumps should still land on a readable task detail."],
    assistantState: {
      hint: "This temporary mock detail keeps cross-module task links usable before the task appears in the stage buckets.",
      label: "detail focus",
    },
    background: "This is a temporary mock-stage detail used when another module asks the task page to focus a task that is not yet present in the current task buckets.",
    constraints: ["Do not add new protocol fields.", "Keep the task detail and safety deep-link behavior intact."],
    dueAt: null,
    goal: task.title,
    nextAction: "Inspect the current detail first, then decide whether to continue from the source module or from the task page.",
    noteDraft: "This task was opened from another dashboard module, so the detail page uses a minimal temporary summary until richer task data becomes available.",
    noteEntries: ["If the formal task detail arrives later, the page will refresh automatically."],
    outputs: [
      {
        id: `${task.task_id}_snapshot`,
        label: "Current snapshot",
        content: "This task is temporarily rendered with the smallest viable detail shape so cross-module navigation stays usable in mock mode.",
        tone: "draft",
      },
    ],
    phase: "cross module detail focus",
    priority: "steady",
    progressHint: "This task has not reached the visible stage buckets yet, so the page is rendering its detail directly.",
    quickContext: [{ id: `${task.task_id}_ctx_source`, label: "Source", content: "This task was opened from another dashboard module." }],
    recentConversation: ["Mock mode is temporarily supplying the detail payload for this task link."],
    relatedFiles: [],
    stepTargets: {},
    suggestedNext: "If the task later appears in the stage buckets, you can continue it from the regular task flow.",
  };
}

export function getMockTaskBuckets() {
  const items: TaskListItem[] = mockTasksState.map((task) => ({
    task,
    experience: taskExperiences[task.task_id],
  }));
  const unfinished = items.filter((item) => !item.task.finished_at);
  const finished = items.filter((item) => item.task.finished_at);

  return {
    finished: {
      items: finished,
      page: {
        has_more: false,
        limit: finished.length,
        offset: 0,
        total: finished.length,
      },
    },
    source: "mock" as const,
    unfinished: {
      items: unfinished,
      page: {
        has_more: false,
        limit: unfinished.length,
        offset: 0,
        total: unfinished.length,
      },
    },
  };
}

export function getMockTaskDetail(taskId: string): TaskDetailData {
  const detail = mockDetailsState[taskId];

  if (detail) {
    return {
      detail: clone(detail),
      experience: taskExperiences[detail.task.task_id],
      source: "mock",
      task: clone(detail.task),
    };
  }

  const task = createAdHocMockTask(taskId);

  return {
    detail: createDetail(task, [], [], { latest_restore_point: null, pending_authorizations: 0, risk_level: "green", security_status: "normal" }, []),
    experience: createAdHocMockExperience(task),
    source: "mock",
    task: clone(task),
  };
}

function syncTaskState(task: Task) {
  mockTasksState = mockTasksState.map((item) => (item.task_id === task.task_id ? clone(task) : item));
  mockDetailsState[task.task_id].task = clone(task);
}

export function runMockTaskControl(taskId: string, action: "pause" | "resume" | "cancel" | "restart"): TaskControlOutcome {
  const current = mockDetailsState[taskId] ?? mockDetailsState.task_focus_001;
  const nextTask = clone(current.task);
  const nextTimeline = clone(current.timeline);
  const updatedAt = new Date().toISOString();

  if (action === "pause") {
    nextTask.status = "paused";
  }

  if (action === "resume") {
    nextTask.status = "processing";
  }

  if (action === "cancel") {
    nextTask.status = "cancelled";
    nextTask.finished_at = updatedAt;
  }

  if (action === "restart") {
    nextTask.status = "processing";
    nextTask.finished_at = null;
    nextTimeline.forEach((step, index) => {
      step.status = index === 0 ? "running" : "pending";
    });
  }

  nextTask.updated_at = updatedAt;
  nextTask.current_step = nextTimeline.find((step) => step.status === "running")?.step_id ?? nextTask.current_step;

  syncTaskState(nextTask);
  mockDetailsState[taskId].timeline = nextTimeline;

  return {
    result: {
      bubble_message: {
        bubble_id: `bubble_${taskId}_${action}`,
        created_at: updatedAt,
        hidden: false,
        pinned: false,
        task_id: taskId,
        text: action === "restart" ? "已重新启动当前任务。" : `已执行${action}操作。`,
        type: "status",
      },
      task: clone(nextTask),
    },
    source: "mock",
  };
}
