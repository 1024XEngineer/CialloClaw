export type AgentStateKey =
  // Task states
  | 'standby'
  | 'idle_present'
  | 'working'
  | 'highlight'
  | 'completing'
  | 'done'
  | 'error_permission'
  | 'error_blocked'
  | 'error_missing_info'
  // Notepad / async collaboration
  | 'notepad_processing'
  | 'notepad_reminder'
  | 'scheduled_task'
  // Mirror / periodic summary
  | 'mirror_summary'
  | 'mirror_habit'
  // Sense / system awareness
  | 'sense_alert'
  | 'sense_suggestion';

export type ModuleKey = 'task' | 'notepad' | 'mirror' | 'sense';

export interface ContextItem {
  icon: string;
  text: string;
  time?: string;
  type?: 'normal' | 'warn' | 'error' | 'hint' | 'active';
}

export interface NotepadItem {
  id: string;
  text: string;
  status: 'pending' | 'processing' | 'done' | 'recurring';
  time?: string;
  tag?: string;
}

export interface MirrorInsight {
  icon: string;
  text: string;
  emphasis?: boolean;
}

export interface SenseSignal {
  label: string;
  value: string;
  level: 'normal' | 'warn' | 'critical';
  icon: string;
  translation?: string;
}

export interface AgentStateData {
  key: AgentStateKey;
  module: ModuleKey;
  label: string;
  orbColor: string;
  orbGlow: string;
  accentColor: string;
  tag: string;
  tagType: 'normal' | 'active' | 'highlight' | 'warn' | 'error' | 'done' | 'mirror' | 'sense';
  headline: string;
  subline: string;
  progress?: number;
  progressLabel?: string;
  progressSteps?: { label: string; status: 'done' | 'active' | 'pending' | 'error' }[];
  context: ContextItem[];
  notepadItems?: NotepadItem[];
  mirrorInsights?: MirrorInsight[];
  mirrorPeriod?: string;
  senseSignals?: SenseSignal[];
  senseAction?: string;
  anomaly?: {
    title: string;
    desc: string;
    actionLabel: string;
    dismissLabel: string;
    severity: 'warn' | 'error' | 'info';
  };
  breathSpeed: number;
}

export const agentStates: Record<AgentStateKey, AgentStateData> = {
  standby: {
    key: 'standby',
    module: 'task',
    label: '待机',
    orbColor: '#1e293b',
    orbGlow: 'rgba(51,65,85,0.35)',
    accentColor: '#475569',
    tag: '待命',
    tagType: 'normal',
    headline: '随时准备好了',
    subline: '没有进行中的任务。通过悬浮球发起新指令，或查看便签与提醒。',
    progress: 0,
    progressLabel: '等待指令',
    progressSteps: [
      { label: '准备', status: 'pending' },
      { label: '执行', status: 'pending' },
      { label: '完成', status: 'pending' },
    ],
    context: [],
    breathSpeed: 0.5,
  },
  idle_present: {
    key: 'idle_present',
    module: 'task',
    label: '空闲在场',
    orbColor: '#0f172a',
    orbGlow: 'rgba(99,102,241,0.3)',
    accentColor: '#818cf8',
    tag: '在场',
    tagType: 'normal',
    headline: '我在这里，持续关注中',
    subline: '上次任务已完成 23 分钟前。随时可以发起新的指令，我会持续监听。',
    progress: 0,
    progressLabel: '空闲中',
    progressSteps: [
      { label: '上次任务已完成', status: 'done' },
      { label: '等待新指令', status: 'active' },
    ],
    context: [
      { icon: 'ri-check-double-line', text: '竞品分析报告已完成', time: '23 分钟前', type: 'normal' },
      { icon: 'ri-eye-line', text: '持续监听新消息', time: '进行中', type: 'active' },
    ],
    breathSpeed: 0.65,
  },
  working: {
    key: 'working',
    module: 'task',
    label: '推进中',
    orbColor: '#052e16',
    orbGlow: 'rgba(52,211,153,0.35)',
    accentColor: '#34d399',
    tag: '正在推进',
    tagType: 'active',
    headline: '已完成资料检索，正在整理关键信息',
    subline: '从 47 个来源中筛选出 6 个高相关结果，正在提炼核心观点。',
    progress: 42,
    progressLabel: '执行中',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '资料检索', status: 'done' },
      { label: '信息整理', status: 'active' },
      { label: '生成结果', status: 'pending' },
    ],
    context: [
      { icon: 'ri-search-line', text: '已扫描 47 个来源', time: '2 分钟前', type: 'normal' },
      { icon: 'ri-filter-line', text: '筛选出 6 个高相关结果', time: '1 分钟前', type: 'normal' },
      { icon: 'ri-loader-4-line', text: '正在提炼核心观点...', time: '进行中', type: 'active' },
    ],
    breathSpeed: 1.8,
  },
  highlight: {
    key: 'highlight',
    module: 'task',
    label: '新进展',
    orbColor: '#451a03',
    orbGlow: 'rgba(251,191,36,0.45)',
    accentColor: '#fbbf24',
    tag: '值得关注',
    tagType: 'highlight',
    headline: '发现 2 个你可能最关心的风险点',
    subline: '竞品定价策略出现重大调整，用户留存数据出现异常波动，已标记为优先项。',
    progress: 58,
    progressLabel: '发现重点',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '资料检索', status: 'done' },
      { label: '分析中', status: 'active' },
      { label: '生成结果', status: 'pending' },
    ],
    context: [
      { icon: 'ri-alert-line', text: '竞品定价策略重大调整', time: '刚刚', type: 'warn' },
      { icon: 'ri-alert-line', text: '用户留存数据异常波动', time: '刚刚', type: 'warn' },
      { icon: 'ri-flag-line', text: '已标记为优先项', time: '', type: 'hint' },
    ],
    breathSpeed: 2.2,
  },
  completing: {
    key: 'completing',
    module: 'task',
    label: '接近完成',
    orbColor: '#042f2e',
    orbGlow: 'rgba(45,212,191,0.4)',
    accentColor: '#2dd4bf',
    tag: '接近完成',
    tagType: 'done',
    headline: '草稿已就绪，下一步只需你确认',
    subline: '报告初稿已生成，建议先看结论部分。确认发送对象后可立即发出。',
    progress: 85,
    progressLabel: '待确认',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '资料检索', status: 'done' },
      { label: '草稿生成', status: 'done' },
      { label: '待你确认', status: 'active' },
    ],
    context: [
      { icon: 'ri-file-text-line', text: '报告初稿已生成', time: '刚刚', type: 'normal' },
      { icon: 'ri-eye-line', text: '建议先看结论部分', time: '', type: 'hint' },
      { icon: 'ri-send-plane-line', text: '确认后可立即发送', time: '', type: 'hint' },
    ],
    anomaly: {
      title: '草稿已就绪，等待你确认',
      desc: '确认发送对象后，我可以立即帮你发出这份报告。',
      actionLabel: '确认发送',
      dismissLabel: '再看看',
      severity: 'info',
    },
    breathSpeed: 1.4,
  },
  done: {
    key: 'done',
    module: 'task',
    label: '已完成',
    orbColor: '#1e293b',
    orbGlow: 'rgba(226,232,240,0.2)',
    accentColor: '#94a3b8',
    tag: '已完成',
    tagType: 'done',
    headline: '会议纪要已整理完成',
    subline: '本次任务全部完成。共处理 3 个议题，生成摘要 1 份，待你确认发送对象。',
    progress: 100,
    progressLabel: '全部完成',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '资料检索', status: 'done' },
      { label: '草稿生成', status: 'done' },
      { label: '已完成', status: 'done' },
    ],
    context: [
      { icon: 'ri-check-double-line', text: '3 个议题全部处理完成', time: '刚刚', type: 'normal' },
      { icon: 'ri-file-text-line', text: '摘要已生成，共 1 份', time: '', type: 'normal' },
      { icon: 'ri-user-line', text: '待确认发送对象', time: '', type: 'hint' },
    ],
    breathSpeed: 0.7,
  },
  error_permission: {
    key: 'error_permission',
    module: 'task',
    label: '缺少权限',
    orbColor: '#4c0519',
    orbGlow: 'rgba(251,113,133,0.45)',
    accentColor: '#fb7185',
    tag: '需要介入',
    tagType: 'error',
    headline: '缺少访问权限，当前步骤无法继续',
    subline: '无法读取「Q3 财务数据」文件夹。请确认授权后，我可以立即继续推进。',
    progress: 55,
    progressLabel: '推进受阻',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '资料检索', status: 'done' },
      { label: '权限缺失', status: 'error' },
      { label: '待解决', status: 'pending' },
    ],
    context: [
      { icon: 'ri-check-line', text: '检索阶段已完成', time: '2 分钟前', type: 'normal' },
      { icon: 'ri-lock-line', text: '无法读取「Q3 财务数据」', time: '刚刚', type: 'error' },
      { icon: 'ri-question-line', text: '等待你授权后继续', time: '', type: 'hint' },
    ],
    anomaly: {
      title: '缺少访问权限',
      desc: '「Q3 财务数据」文件夹无法访问。授权后我可以立即继续。',
      actionLabel: '前往授权',
      dismissLabel: '稍后处理',
      severity: 'error',
    },
    breathSpeed: 2.5,
  },
  error_blocked: {
    key: 'error_blocked',
    module: 'task',
    label: '步骤阻塞',
    orbColor: '#4c0519',
    orbGlow: 'rgba(251,113,133,0.4)',
    accentColor: '#f87171',
    tag: '步骤阻塞',
    tagType: 'error',
    headline: '当前步骤被阻塞，无法自动继续',
    subline: '依赖的上游任务尚未完成，需要你手动确认是否跳过或等待。',
    progress: 38,
    progressLabel: '阻塞中',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '上游阻塞', status: 'error' },
      { label: '等待处理', status: 'pending' },
      { label: '继续推进', status: 'pending' },
    ],
    context: [
      { icon: 'ri-link', text: '依赖「数据清洗」任务', time: '未完成', type: 'error' },
      { icon: 'ri-pause-circle-line', text: '当前步骤已暂停', time: '5 分钟前', type: 'warn' },
      { icon: 'ri-question-line', text: '跳过 or 等待？需要你决定', time: '', type: 'hint' },
    ],
    anomaly: {
      title: '上游任务阻塞',
      desc: '「数据清洗」任务尚未完成，是否跳过并继续推进？',
      actionLabel: '跳过继续',
      dismissLabel: '等待完成',
      severity: 'error',
    },
    breathSpeed: 2.8,
  },
  error_missing_info: {
    key: 'error_missing_info',
    module: 'task',
    label: '缺少信息',
    orbColor: '#422006',
    orbGlow: 'rgba(251,146,60,0.4)',
    accentColor: '#fb923c',
    tag: '还差一项',
    tagType: 'warn',
    headline: '还差一项关键信息，无法继续',
    subline: '需要你提供目标受众的地区范围，才能完成受众分析这一步。',
    progress: 48,
    progressLabel: '信息不足',
    progressSteps: [
      { label: '任务启动', status: 'done' },
      { label: '初步分析', status: 'done' },
      { label: '信息缺失', status: 'error' },
      { label: '深度分析', status: 'pending' },
    ],
    context: [
      { icon: 'ri-check-line', text: '初步分析已完成', time: '1 分钟前', type: 'normal' },
      { icon: 'ri-information-line', text: '缺少：目标受众地区范围', time: '刚刚', type: 'warn' },
      { icon: 'ri-edit-line', text: '补充后可立即继续', time: '', type: 'hint' },
    ],
    anomaly: {
      title: '还差一项关键信息',
      desc: '请提供目标受众的地区范围（如：华东地区 / 全国 / 海外）。',
      actionLabel: '补充信息',
      dismissLabel: '跳过此步',
      severity: 'warn',
    },
    breathSpeed: 2.0,
  },

  // ── NOTEPAD MODULE ──
  notepad_processing: {
    key: 'notepad_processing',
    module: 'notepad',
    label: '便签处理中',
    orbColor: '#1a1040',
    orbGlow: 'rgba(167,139,250,0.35)',
    accentColor: '#a78bfa',
    tag: '异步处理',
    tagType: 'active',
    headline: '正在处理你昨晚留下的 3 条便签',
    subline: '已识别出 2 项可直接执行的任务，1 项需要你确认优先级后再推进。',
    context: [
      { icon: 'ri-sticky-note-line', text: '「整理 Q3 复盘要点」— 已开始', time: '进行中', type: 'active' },
      { icon: 'ri-sticky-note-line', text: '「联系设计师确认排期」— 已识别', time: '待执行', type: 'normal' },
      { icon: 'ri-question-line', text: '「下周汇报材料」— 需确认优先级', time: '', type: 'hint' },
    ],
    notepadItems: [
      { id: '1', text: '整理 Q3 复盘要点', status: 'processing', time: '进行中', tag: '文档' },
      { id: '2', text: '联系设计师确认排期', status: 'pending', time: '待执行', tag: '沟通' },
      { id: '3', text: '下周汇报材料准备', status: 'pending', time: '需确认', tag: '优先级?' },
    ],
    breathSpeed: 1.6,
  },
  notepad_reminder: {
    key: 'notepad_reminder',
    module: 'notepad',
    label: '重复任务提醒',
    orbColor: '#1a1040',
    orbGlow: 'rgba(167,139,250,0.4)',
    accentColor: '#c4b5fd',
    tag: '我记住了',
    tagType: 'highlight',
    headline: '你每周一都会整理周报，现在是周一上午',
    subline: '我已帮你准备好上周的数据摘要，可以直接开始写，也可以让我先起草一版。',
    context: [
      { icon: 'ri-repeat-line', text: '已连续识别 6 周的周报习惯', time: '', type: 'hint' },
      { icon: 'ri-bar-chart-line', text: '上周数据摘要已就绪', time: '刚刚', type: 'normal' },
      { icon: 'ri-edit-2-line', text: '可直接起草或让我先写', time: '', type: 'hint' },
    ],
    notepadItems: [
      { id: '1', text: '周报整理（每周一）', status: 'recurring', time: '今天', tag: '重复任务' },
      { id: '2', text: '上周数据摘要', status: 'done', time: '已就绪', tag: '已准备' },
    ],
    breathSpeed: 1.2,
  },
  scheduled_task: {
    key: 'scheduled_task',
    module: 'notepad',
    label: '定时任务',
    orbColor: '#0c1a2e',
    orbGlow: 'rgba(56,189,248,0.3)',
    accentColor: '#38bdf8',
    tag: '定时巡检',
    tagType: 'active',
    headline: '每日 9:00 巡检已完成，发现 1 项需关注',
    subline: '今日邮件中有 1 封来自重要客户的回复，已标记为优先处理。其余 12 封已分类归档。',
    context: [
      { icon: 'ri-mail-line', text: '重要客户回复 — 待处理', time: '刚刚', type: 'warn' },
      { icon: 'ri-archive-line', text: '12 封邮件已自动归档', time: '9:00', type: 'normal' },
      { icon: 'ri-calendar-check-line', text: '下次巡检：明日 9:00', time: '', type: 'hint' },
    ],
    notepadItems: [
      { id: '1', text: '重要客户邮件回复', status: 'pending', time: '待处理', tag: '优先' },
      { id: '2', text: '日常邮件归档', status: 'done', time: '已完成', tag: '自动' },
      { id: '3', text: '明日 9:00 巡检', status: 'pending', time: '已排期', tag: '定时' },
    ],
    breathSpeed: 1.0,
  },

  // ── MIRROR MODULE ──
  mirror_summary: {
    key: 'mirror_summary',
    module: 'mirror',
    label: '周期总结',
    orbColor: '#0f0a1e',
    orbGlow: 'rgba(196,181,253,0.3)',
    accentColor: '#c4b5fd',
    tag: '镜子',
    tagType: 'mirror',
    headline: '我帮你总结了最近两周的关注重点',
    subline: '这是我观察到的，不是报告，只是想让你知道我记住了这些。',
    context: [],
    mirrorPeriod: '2025 年 3 月 17 日 — 3 月 31 日',
    mirrorInsights: [
      { icon: 'ri-focus-3-line', text: '你最常处理的是「产品策略」和「竞品分析」类任务', emphasis: true },
      { icon: 'ri-time-line', text: '你的高效时段集中在上午 10:00 — 12:00' },
      { icon: 'ri-repeat-2-line', text: '「周报整理」已成为稳定的每周一习惯' },
      { icon: 'ri-chat-3-line', text: '你倾向于先看结论，再决定是否深入' },
    ],
    breathSpeed: 0.8,
  },
  mirror_habit: {
    key: 'mirror_habit',
    module: 'mirror',
    label: '习惯洞察',
    orbColor: '#0f0a1e',
    orbGlow: 'rgba(196,181,253,0.35)',
    accentColor: '#e9d5ff',
    tag: '我注意到',
    tagType: 'mirror',
    headline: '你似乎形成了一种新的工作节奏',
    subline: '最近 3 周，你在周四下午会集中处理需要深度思考的任务。我已为这个时段预留更安静的提醒策略。',
    context: [],
    mirrorInsights: [
      { icon: 'ri-brain-line', text: '周四下午是你的深度工作时段', emphasis: true },
      { icon: 'ri-notification-off-line', text: '已为该时段调整为低打扰模式' },
      { icon: 'ri-lightbulb-line', text: '建议将复杂任务优先排在周四下午' },
    ],
    breathSpeed: 0.75,
  },

  // ── SENSE MODULE ──
  sense_alert: {
    key: 'sense_alert',
    module: 'sense',
    label: '系统感知',
    orbColor: '#1a0a00',
    orbGlow: 'rgba(251,146,60,0.4)',
    accentColor: '#fb923c',
    tag: '感知到了',
    tagType: 'sense',
    headline: '你的电脑正在承受较高负荷，我建议暂缓大文件处理',
    subline: 'CPU 持续高负载已超过 8 分钟，内存余量不足 12%。当前不适合启动新的渲染任务。',
    context: [
      { icon: 'ri-cpu-line', text: 'CPU 负载 91%，持续 8 分钟', time: '现在', type: 'error' },
      { icon: 'ri-database-2-line', text: '内存余量 11.2%，接近临界', time: '现在', type: 'warn' },
      { icon: 'ri-wifi-line', text: '网络正常，延迟 18ms', time: '', type: 'normal' },
    ],
    senseSignals: [
      { label: 'CPU', value: '91%', level: 'critical', icon: 'ri-cpu-line', translation: '正在高强度运转' },
      { label: '内存', value: '11%', level: 'critical', icon: 'ri-database-2-line', translation: '余量接近临界' },
      { label: '网络', value: '18ms', level: 'normal', icon: 'ri-wifi-line', translation: '连接稳定' },
      { label: '磁盘', value: '34%', level: 'normal', icon: 'ri-hard-drive-2-line', translation: '读写正常' },
    ],
    senseAction: '暂缓大文件处理，等负载降低后继续',
    anomaly: {
      title: '系统负荷过高',
      desc: '建议暂缓启动新的渲染或大文件处理任务，等待负载恢复正常。',
      actionLabel: '暂缓任务',
      dismissLabel: '继续执行',
      severity: 'warn',
    },
    breathSpeed: 2.4,
  },
  sense_suggestion: {
    key: 'sense_suggestion',
    module: 'sense',
    label: '系统建议',
    orbColor: '#0a1628',
    orbGlow: 'rgba(56,189,248,0.3)',
    accentColor: '#38bdf8',
    tag: '系统感知',
    tagType: 'sense',
    headline: '系统状态良好，这是启动大任务的好时机',
    subline: '当前 CPU 空闲率 78%，内存充裕，网络延迟极低。如果有待处理的大型任务，现在是好时机。',
    context: [
      { icon: 'ri-cpu-line', text: 'CPU 空闲率 78%，状态极佳', time: '现在', type: 'normal' },
      { icon: 'ri-database-2-line', text: '内存余量 68%，充裕', time: '', type: 'normal' },
      { icon: 'ri-wifi-line', text: '网络延迟 6ms，极低', time: '', type: 'normal' },
    ],
    senseSignals: [
      { label: 'CPU', value: '22%', level: 'normal', icon: 'ri-cpu-line', translation: '空闲充裕' },
      { label: '内存', value: '68%', level: 'normal', icon: 'ri-database-2-line', translation: '余量充足' },
      { label: '网络', value: '6ms', level: 'normal', icon: 'ri-wifi-line', translation: '极低延迟' },
      { label: '磁盘', value: '12%', level: 'normal', icon: 'ri-hard-drive-2-line', translation: '读写流畅' },
    ],
    senseAction: '适合启动大型任务或批量处理',
    breathSpeed: 0.9,
  },
};

export const stateOrder: AgentStateKey[] = [
  'standby',
  'idle_present',
  'working',
  'highlight',
  'completing',
  'done',
  'error_permission',
  'error_blocked',
  'error_missing_info',
  'notepad_processing',
  'notepad_reminder',
  'scheduled_task',
  'mirror_summary',
  'mirror_habit',
  'sense_alert',
  'sense_suggestion',
];

export const moduleGroups: { key: ModuleKey; label: string; states: AgentStateKey[] }[] = [
  {
    key: 'task',
    label: '任务状态',
    states: ['standby', 'idle_present', 'working', 'highlight', 'completing', 'done', 'error_permission', 'error_blocked', 'error_missing_info'],
  },
  {
    key: 'notepad',
    label: '便签协作',
    states: ['notepad_processing', 'notepad_reminder', 'scheduled_task'],
  },
  {
    key: 'mirror',
    label: '镜子',
    states: ['mirror_summary', 'mirror_habit'],
  },
  {
    key: 'sense',
    label: '硬件感知',
    states: ['sense_alert', 'sense_suggestion'],
  },
];
