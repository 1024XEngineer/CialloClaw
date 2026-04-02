export const mockMaterials = [
  {
    id: 'material-selection',
    kind: 'text',
    label: '选中文本',
    title: '被高亮的一段句子',
    summary: '用户正在看一段需要提炼的内容。',
    content: '这里是一段适合提炼重点的中文文本，带有明显的任务线索和几个可收束的关键词。',
    intentHint: '提炼重点'
  },
  {
    id: 'material-copy',
    kind: 'text',
    label: '复制文本',
    title: '刚复制的段落',
    summary: '适合翻译或改写成更自然的表达。',
    content: 'Please help me translate this paragraph into a concise and natural Chinese reply.',
    intentHint: '翻译'
  },
  {
    id: 'material-clipboard',
    kind: 'text',
    label: '剪贴板文本',
    title: '剪贴板里的长句',
    summary: '可直接改写成更适合发送的草稿。',
    content: '我整理完这部分内容了，但还需要你帮我把口气调得更像日报语气。',
    intentHint: '生成草稿'
  },
  {
    id: 'material-file',
    kind: 'file',
    label: '文件卡片',
    title: 'daily-note-042.txt',
    summary: '一个可直接拖入的文件名卡片。',
    content: '文件名：daily-note-042.txt · 大小：18 KB · 已标记为今天需要处理。',
    intentHint: '生成草稿'
  },
  {
    id: 'material-error',
    kind: 'error',
    label: '报错片段',
    title: 'Runtime panic',
    summary: '适合解释报错和给出下一步排查。',
    content: 'panic: runtime error: invalid memory address or nil pointer dereference',
    intentHint: '解释报错'
  },
  {
    id: 'material-web',
    kind: 'web',
    label: '网页内容',
    title: '网页摘录',
    summary: '高密度页面，适合提炼或总结。',
    content: '页面中包含更新说明、注意事项和一条需要回写到工作台的改动摘要。',
    intentHint: '提炼重点'
  },
  {
    id: 'material-image',
    kind: 'image',
    label: '截图占位',
    title: '截图 / 图片',
    summary: '图片材料用占位卡片表达。',
    content: '这里是截图或图片的占位卡片，代表视觉类材料也能被承接。',
    intentHint: '提炼重点'
  }
];

export const intentPresets = [
  {
    id: 'translate',
    label: '翻译',
    description: '把内容保留原意后转成更自然的语言。',
    resultTitle: '翻译结果',
    reply: '我已经把内容翻译好了，保持了原意，也收了一点语气。',
    bullets: ['保留原意', '更自然', '可直接复用']
  },
  {
    id: 'summary',
    label: '提炼重点',
    description: '压缩为少量可回写的重点。',
    resultTitle: '提炼结果',
    reply: '我把它压成了 3 个重点，方便你继续推进。',
    bullets: ['一眼可读', '少量重点', '便于回写']
  },
  {
    id: 'debug',
    label: '解释报错',
    description: '解释错误含义并给出排查方向。',
    resultTitle: '报错解释',
    reply: '这类报错通常和空引用或上下文缺失有关，先补齐输入最稳。',
    bullets: ['定位原因', '给出排查方向', '不扩展成长文']
  },
  {
    id: 'draft',
    label: '生成草稿',
    description: '生成一版可发送、可继续修改的草稿。',
    resultTitle: '草稿初版',
    reply: '草稿已经收好，可以直接发，也可以继续压缩语气。',
    bullets: ['可直接发', '可继续压缩', '语气更稳']
  },
  {
    id: 'rewrite',
    label: '改写语气',
    description: '将表达改得更像日报、说明或回复。',
    resultTitle: '改写结果',
    reply: '我已经把口气收成更贴近你当前场景的版本。',
    bullets: ['语气更贴近', '更短更稳', '适合直接套用']
  }
];

export const workspaceConversations = [
  {
    id: 'conv-today-1',
    bucket: '今天',
    title: '把网页内容先提炼成重点',
    time: '09:42',
    summary: '先提炼，再决定要不要回写成待办。',
    tags: ['提炼', '回写', '网页'],
    messages: [
      { role: 'user', kind: 'user', title: '现场材料', body: '请先帮我提炼重点，最好能顺手给个下一步。', time: '09:41' },
      { role: 'assistant', kind: 'assistant', title: 'Agent 回复', body: '我先压成 3 点：目标、变化、下一步。', time: '09:42' },
      { role: 'result', kind: 'result', title: '结果卡', body: '1. 先看能回写的点；2. 不要铺太长；3. 先把今天能动的动作列出来。', time: '09:42' }
    ]
  },
  {
    id: 'conv-today-2',
    bucket: '今天',
    title: '解释一条运行时错误',
    time: '10:18',
    summary: '先解释，再给出排查方向。',
    tags: ['报错', '排查', '解释'],
    messages: [
      { role: 'user', kind: 'user', title: '报错片段', body: 'panic: runtime error: invalid memory address or nil pointer dereference', time: '10:18' },
      { role: 'assistant', kind: 'assistant', title: 'Agent 回复', body: '这通常是空引用或上下文没传到位。', time: '10:19' },
      { role: 'draft', kind: 'draft', title: '排查草稿', body: '建议先确认输入对象是否初始化，再看调用链是否提前退出。', time: '10:19' }
    ]
  },
  {
    id: 'conv-today-3',
    bucket: '今天',
    title: '改写一段回复口气',
    time: '11:03',
    summary: '把回复压成更稳、更短的版本。',
    tags: ['改写', '回复', '语气'],
    messages: [
      { role: 'user', kind: 'user', title: '原始内容', body: '我整理完这部分内容了，但还需要你帮我把口气调得更像日报语气。', time: '11:02' },
      { role: 'assistant', kind: 'assistant', title: 'Agent 回复', body: '我已经把表达收成更适合发送的版本。', time: '11:03' }
    ]
  },
  {
    id: 'conv-earlier-1',
    bucket: '更早',
    title: '把文件拖入后先判断意图',
    time: '昨天',
    summary: '先判断意图，再进入确认层。',
    tags: ['文件', '确认', '意图'],
    messages: [
      { role: 'user', kind: 'user', title: '文件卡', body: 'daily-note-042.txt', time: '昨天' },
      { role: 'assistant', kind: 'assistant', title: 'Agent 回复', body: '我先判断它更像草稿整理还是内容提炼。', time: '昨天' },
      { role: 'note', kind: 'note', title: '系统说明', body: '先提示，再确认，后展开。', time: '昨天' }
    ]
  },
  {
    id: 'conv-earlier-2',
    bucket: '更早',
    title: '小窗里只保留最近一来一回',
    time: '昨天',
    summary: '快速对话层保持轻量，不保留长历史。',
    tags: ['轻提示', '小窗', '单轮'],
    messages: [
      { role: 'user', kind: 'user', title: '最近输入', body: '我只需要看看当前这条内容能不能直接处理。', time: '昨天' },
      { role: 'assistant', kind: 'assistant', title: 'Agent 回复', body: '可以，先把最近一轮的协作结果收束在一个小窗里。', time: '昨天' }
    ]
  },
  {
    id: 'conv-earlier-3',
    bucket: '更早',
    title: '工作台里接入了一个结果卡',
    time: '3 天前',
    summary: '从结果窗平滑接入工作台继续推进。',
    tags: ['接入', '结果卡', '推进'],
    messages: [
      { role: 'user', kind: 'user', title: '结果窗', body: '请把这个结果接入工作台。', time: '3 天前' },
      { role: 'assistant', kind: 'assistant', title: 'Agent 回复', body: '我把它接入当前会话，并保留给你继续推进。', time: '3 天前' },
      { role: 'result', kind: 'result', title: '已接入内容', body: '结果卡已进入当前会话，等待你继续推进或转待办。', time: '3 天前' }
    ]
  }
];

export const statusSnapshots = {
  idle: {
    label: '空闲',
    title: '当前状态：空闲',
    detail: '悬浮球保持默认表情，等待下一次承接。',
    task: '暂无进行中的处理',
    failure: '最近一次失败：无',
    recent: ['默认入口常驻', '轻提示未打开', '工作台保持收起']
  },
  working: {
    label: '工作中',
    title: '当前状态：工作中',
    detail: '正在整理今日日报素材，或者在确认一条协作意图。',
    task: '正在总结当前选中的内容',
    failure: '最近一次失败：上下文不足',
    recent: ['正在收材料', '意图判断已进入确认', '结果窗可继续接入工作台']
  },
  error: {
    label: '故障',
    title: '当前状态：故障',
    detail: '静态 mock 展示一个明确的失败说明。',
    task: '最近一次失败动作被记录下来',
    failure: '最近一次失败：权限不足 / 网络中断 / 上下文不足',
    recent: ['失败会被留痕', '可回看最近成功项', '不会自动扩大为重平台']
  }
};

export const todoSeed = [
  { id: 'todo-1', title: '把提炼结果挂成今日待办', bucket: 'today', status: '进行中', source: '窗口3 / 提炼结果', agentGenerated: true },
  { id: 'todo-2', title: '确认一条报错是否需要发给同事', bucket: 'today', status: '待确认', source: '窗口2 / 报错片段', agentGenerated: true },
  { id: 'todo-3', title: '整理工作台默认会话的标题', bucket: 'later', status: '稍后', source: '会话历史', agentGenerated: false },
  { id: 'todo-4', title: '把“轻提示优先”加入长期偏好', bucket: 'later', status: '稍后', source: '设置 / 记忆', agentGenerated: true },
  { id: 'todo-5', title: '复核一次高风险动作的确认文案', bucket: 'done', status: '已完成', source: '状态页', agentGenerated: false },
  { id: 'todo-6', title: '把窗口2的拖入材料继续接入工作台', bucket: 'done', status: '已完成', source: '窗口2 / 材料卡', agentGenerated: true }
];

export const settingsSeed = {
  ballVisible: true,
  reminderEnabled: true,
  provider: 'Mock Provider',
  model: 'Static UX Mock',
  workspacePath: 'E:/work/cialloclaw/workspace',
  hotkey: 'Ctrl + Shift + Space',
  memoryEnabled: true,
  version: 'demo4 / 0.1.0'
};

export const memoryNotes = [
  '先提示，再确认，后展开。',
  '轻提示和小窗优先。',
  '记忆是长期协作能力，不是高频主舞台。',
  '高风险执行需要确认感，但 mock 以静态为主。'
];

export function getIntentByLabel(label) {
  return intentPresets.find((item) => item.label === label) || intentPresets[0];
}

export function guessIntent(materials) {
  const kinds = materials.map((item) => item.kind);
  if (kinds.includes('error')) return '解释报错';
  if (kinds.includes('file')) return '生成草稿';
  if (kinds.includes('web')) return '提炼重点';
  if (kinds.includes('image')) return '提炼重点';
  if (kinds.includes('text') && kinds.includes('text')) return '翻译';
  return '生成草稿';
}

export function summarizeMaterial(item) {
  if (!item) return '';
  if (item.kind === 'file') return `文件：${item.title}`;
  if (item.kind === 'error') return `报错：${item.title}`;
  if (item.kind === 'image') return `图片：${item.title}`;
  return item.content.slice(0, 36);
}

export function buildQuickReply(input) {
  const text = input.trim();
  const lower = text.toLowerCase();
  if (!text) {
    return '你可以继续补充，或者把内容拖进窗口2。';
  }
  if (lower.includes('翻译') || lower.includes('translate')) {
    return '我把这段内容翻成了更自然的表达，原意保持不变。';
  }
  if (lower.includes('提炼') || lower.includes('总结')) {
    return '我先压成了 3 个可回写重点：目标、变化、下一步。';
  }
  if (lower.includes('报错') || lower.includes('error') || lower.includes('panic')) {
    return '这类报错更像上下文不足或空引用，先补齐输入最稳。';
  }
  if (lower.includes('草稿') || lower.includes('改写') || lower.includes('回复')) {
    return '草稿已经收好，可以直接用，也可以再压短一点。';
  }
  return '我在，这条内容已经接住了。';
}

export function buildMockResult(intentLabel, materials, customIntent = '') {
  const label = customIntent.trim() || intentLabel;
  const preset = getIntentByLabel(label) || getIntentByLabel(intentLabel);
  const sourceNames = materials.map((item) => item.title).join(' · ');

  const result = {
    intentLabel: label,
    title: preset.resultTitle,
    body: preset.reply,
    bullets: preset.bullets,
    sourceNames,
    cardTitle: `${label} · 小型回复窗`,
    summary: preset.reply,
    followUp: '可以继续推进到工作台，或者再补一条材料。',
    todoHint: label === '解释报错' ? '可把这个报错加入待办以便稍后排查。' : '可把这个结果直接转成今日待办。'
  };

  if (label.includes('日报') || label.includes('语气')) {
    result.title = '草稿初版';
    result.body = '我已经把口气收成更像日报的版本。';
  }

  if (label.includes('翻译')) {
    result.title = '翻译结果';
    result.body = '我已经把内容翻译好了，尽量保留原意并收顺语气。';
  }

  if (label.includes('提炼')) {
    result.title = '提炼结果';
    result.body = '我把内容压成 3 个重点，方便你继续推进。';
  }

  if (label.includes('报错')) {
    result.title = '报错解释';
    result.body = '这个错误更像上下文或初始化问题，先补齐输入再排查最稳。';
  }

  result.sourceNames = sourceNames;
  return result;
}
