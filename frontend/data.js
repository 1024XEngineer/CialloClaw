export const appData = {
  desktopWindows: [
    {
      id: 'window-focus',
      title: '创作工作台',
      tag: '进行中',
      lines: ['稿件结构已经铺开，等待下一轮取舍。', '右下角的桌面岛随时可以切到任务总览。']
    },
    {
      id: 'window-review',
      title: '巡检视窗',
      tag: '轻量提醒',
      lines: ['最近两条停滞任务都卡在“缺少明确下一步”。', '从悬浮球左拨可进入巡检，也可直接打开任务控制台。']
    },
    {
      id: 'window-memory',
      title: '镜像浮层',
      tag: '记忆同步',
      lines: ['偏好、阶段判断、最近观察会叠加成可读镜像。', '镜像不是数据库列表，而是可以回跳任务的工作材料。']
    }
  ],
  tasks: [
    {
      id: 'task-ux-route',
      title: '梳理桌面岛 → 任务控制台的首跳节奏',
      project: 'CialloClaw UX Mock',
      actionState: '现在要推进',
      timeBucket: '今天',
      importance: '高',
      whyNow: '任务总览是整个产品结构的主入口之一，如果首跳不顺，用户会继续把产品理解成聊天工具。',
      lastStop: '已经确定桌面岛负责轻量入口，但还没把“点一下就进总览”的路径做成明显主动作。',
      nextStep: '把桌面岛快捷卡片里的首个按钮固定为“打开任务控制台”，并在任务台默认落到现在要推进。',
      context: ['桌面常驻不是打断，而是随手抽出工作全貌', '首跳动作要比聊天输入更短', '需要与侧边信标形成同义入口'],
      relatedMemoryIds: ['mem-desk-entry', 'mem-mode-shift']
    },
    {
      id: 'task-memory-loop',
      title: '把记忆校正动作串回相关任务',
      project: '镜像联动',
      actionState: '现在要推进',
      timeBucket: '今天',
      importance: '高',
      whyNow: '如果记忆校正只停在记忆面，用户会觉得它只是标签管理；能回跳任务才像真正参与工作推进。',
      lastStop: '相关任务关联已经整理出来，但“校正后去哪”还没有被显式说明。',
      nextStep: '在记忆卡上提供“查看相关任务”，回到任务控制台后自动定位到对应任务卡。',
      context: ['用户想看到校正带来的下游影响', '双向跳转是核心体验证明', '任务详情要能回看相关镜像'],
      relatedMemoryIds: ['mem-correction-link', 'mem-user-voice']
    },
    {
      id: 'task-stall-recover',
      title: '复盘悬浮球四向入口的误触原因',
      project: '桌面交互',
      actionState: '已停滞',
      timeBucket: '本周',
      importance: '中',
      whyNow: '悬浮球是常驻入口，但方向感不清会让它像一个装饰按钮，影响桌面常驻价值。',
      lastStop: '已经记录“点击后不知道四向各自意味着什么”，但还没收敛到可执行的文案与排序。',
      nextStep: '把四向分别绑定理解、推进、巡检、记忆，并给出一眼可读的中文标签。',
      context: ['桌面态入口必须短、准、稳', '方向语义比图标花样更重要'],
      relatedMemoryIds: ['mem-orb-direction']
    },
    {
      id: 'task-understand-tone',
      title: '补齐理解面的阶段判断卡节奏',
      project: '理解面',
      actionState: '很快要处理',
      timeBucket: '本周',
      importance: '中',
      whyNow: '理解面不需要复杂，但必须像一个独立工作面，否则会被误读成聊天摘要。',
      lastStop: '目前只有结果句，没有把“为什么这样判断”拆成可浏览的卡片。',
      nextStep: '增加阶段判断、未决问题、建议观察点三类结果卡，形成独立工作板。',
      context: ['理解面强调把上下文压缩成判断', '不出现聊天气泡'],
      relatedMemoryIds: ['mem-stage-fold', 'mem-understand-style']
    },
    {
      id: 'task-permission-copy',
      title: '确认常驻桌面权限引导文案',
      project: '桌面常驻',
      actionState: '等待中',
      timeBucket: '稍后',
      importance: '中',
      whyNow: '权限引导决定用户是否愿意把产品留在桌面，但当前还依赖团队确认最终措辞。',
      lastStop: '已经收集到两版引导文案，等待法务/产品共同定稿。',
      nextStep: '把待确认文案放进理解面侧卡，待反馈回来后再接入首启流程。',
      context: ['桌面产品要先解释“为什么要常驻”', '文案要像陪伴而不是监控'],
      relatedMemoryIds: ['mem-dwell-tone']
    },
    {
      id: 'task-review-pack',
      title: '整理三月体验访谈为阶段镜像折页',
      project: '阶段镜像',
      actionState: '很快要处理',
      timeBucket: '本周',
      importance: '高',
      whyNow: '阶段镜像能解释“最近为什么会这样判断用户”，没有它，任务与记忆之间缺少时间纵深。',
      lastStop: '访谈摘要已经在文档里，但还没有整理成阶段折页，无法快速浏览。',
      nextStep: '把三月访谈合并成一张时间卡，挂到阶段镜像层级里。',
      context: ['镜像层级需要“最近—阶段—长期”递进', '时间卡要服务决策，不是归档'],
      relatedMemoryIds: ['mem-stage-fold']
    }
  ],
  memories: [
    {
      id: 'mem-desk-entry',
      level: 'recent',
      type: 'observation',
      title: '观察：用户会先找“总览”再决定聊不聊',
      summary: '最近几次演示里，大家下意识先寻找一个能总看任务与状态的位置。',
      detail: '这说明桌面岛和侧边信标应该承担“快速看到全局”的角色，而不是把用户先推到输入框。',
      relatedTaskId: 'task-ux-route',
      closable: true,
      correctable: true,
      updatedAt: '刚刚'
    },
    {
      id: 'mem-user-voice',
      level: 'recent',
      type: 'observation',
      title: '观察：校正记忆时，用户想知道会影响什么',
      summary: '用户不会满足于“已修正”提示，他们更想看到这条校正会回流到哪个任务或判断。',
      detail: '因此记忆面必须能跳回任务台，让“校正”变成工作推进动作，而不是元数据整理。',
      relatedTaskId: 'task-memory-loop',
      closable: true,
      correctable: true,
      updatedAt: '18 分钟前'
    },
    {
      id: 'mem-stage-fold',
      level: 'phase',
      type: 'timeline',
      title: '阶段折页：三月体验反馈的重心转移',
      summary: '从“能不能聊”转向“能不能帮我保持工作上下文”。',
      detail: '三月的集中反馈显示，用户更在意任务与理解是否能被保留和随时抽出，因此阶段镜像要把这次转移明确呈现出来。',
      relatedTaskId: 'task-review-pack',
      closable: true,
      correctable: true,
      updatedAt: '昨天'
    },
    {
      id: 'mem-mode-shift',
      level: 'phase',
      type: 'mirror',
      title: '镜像：CialloClaw 被期待成为桌面工作伴层',
      summary: '用户把它理解成工作伴层，而不是聊天容器。',
      detail: '产品结构需要围绕“桌面常驻、轻量抽出、随时回看任务与记忆”来组织，聊天入口只能是辅助手段。',
      relatedTaskId: 'task-ux-route',
      closable: true,
      correctable: true,
      updatedAt: '昨天'
    },
    {
      id: 'mem-orb-direction',
      level: 'phase',
      type: 'mirror',
      title: '镜像：悬浮球方向语义需要胜过造型新鲜感',
      summary: '方向入口只有在一眼看懂时才成立。',
      detail: '“上理解、右推进、左巡检、下记忆”比抽象图标更容易建立习惯，适合作为桌面常驻的短操作。',
      relatedTaskId: 'task-stall-recover',
      closable: true,
      correctable: true,
      updatedAt: '2 天前'
    },
    {
      id: 'mem-dwell-tone',
      level: 'preference',
      type: 'mirror',
      title: '长期偏好：常驻提示要像照看，不要像占用权限',
      summary: '用户愿意接受桌面常驻，但前提是语气温和、理由清楚。',
      detail: '任何关于常驻、通知、浮层的说明都应强调“帮你记住上下文”，而不是“持续监听”。',
      relatedTaskId: 'task-permission-copy',
      closable: true,
      correctable: true,
      updatedAt: '本周'
    },
    {
      id: 'mem-understand-style',
      level: 'preference',
      type: 'observation',
      title: '长期偏好：结果卡要先给判断，再给证据',
      summary: '用户浏览理解面时，更愿意先看结论，再决定是否展开证据。',
      detail: '理解面应以判断卡为主，再补少量依据和后续建议，避免像长对话一样让人失去重心。',
      relatedTaskId: 'task-understand-tone',
      closable: true,
      correctable: true,
      updatedAt: '本周'
    },
    {
      id: 'mem-correction-link',
      level: 'management',
      type: 'timeline',
      title: '记忆管理：校正动作需要留下回流说明',
      summary: '当记忆被修正时，要明确告诉用户：哪些任务、哪些判断会重新参考这条信息。',
      detail: '这条管理记忆用于约束镜像面交互，确保“校正”之后不会成为一条无后续的静态操作。',
      relatedTaskId: 'task-memory-loop',
      closable: true,
      correctable: true,
      updatedAt: '3 天前'
    },
    {
      id: 'mem-review-rhythm',
      level: 'management',
      type: 'timeline',
      title: '记忆管理：每次关闭都要留下一句原因',
      summary: '关闭记忆不是删除，而是告诉系统这条镜像暂时不再需要浮出。',
      detail: '因此关闭后的条目仍应出现在记忆管理层，以便用户回收、复看或重新打开。',
      relatedTaskId: 'task-review-pack',
      closable: true,
      correctable: true,
      updatedAt: '上周'
    }
  ],
  understandCards: [
    {
      title: '当前判断',
      body: '这版原型最应该证明的不是界面样式，而是“任务—记忆—理解—推进”之间能否自然来回。'
    },
    {
      title: '支撑依据',
      body: '高频需求集中在查看总览、回忆上下文、从记忆返回任务，而不是立即发起一段对话。'
    },
    {
      title: '仍需观察',
      body: '悬浮球四向入口的学习成本是否足够低；任务详情里的下一步是否足够明确。'
    },
    {
      title: '建议动作',
      body: '先用静态 mock 把跳转链路跑通，再根据使用反馈微调按钮层级与文案。'
    }
  ],
  advanceCards: [
    {
      title: '马上可推进',
      body: '把“现在要推进”中的两项做成明显主动作，确保桌面岛和记忆回跳都能一跳直达。'
    },
    {
      title: '下一批动作',
      body: '补齐阶段镜像折页，让用户能理解为什么近期判断发生变化。'
    },
    {
      title: '协同提醒',
      body: '等待中事项先挂在推进面，不必占据主视图焦点，但要保留存在感。'
    },
    {
      title: '风险提示',
      body: '如果任务详情仍然像字段堆砌，用户会看不出“为什么现在值得动手”。'
    }
  ]
};

export const taskBrowseOptions = [
  { id: 'status', label: '按行动状态看' },
  { id: 'time', label: '按时间看' },
  { id: 'project', label: '按项目看' }
];

export const taskFilterOptions = [
  { id: 'all', label: '全部任务' },
  { id: '现在要推进', label: '现在要推进' },
  { id: '很快要处理', label: '很快要处理' },
  { id: '已停滞', label: '已停滞' },
  { id: '等待中', label: '等待中' }
];

export const memoryLevelOptions = [
  { id: 'recent', label: '最近记忆' },
  { id: 'phase', label: '阶段镜像' },
  { id: 'preference', label: '长期偏好' },
  { id: 'management', label: '记忆管理' }
];

export function findTask(taskId) {
  return appData.tasks.find((task) => task.id === taskId);
}

export function findMemory(memoryId) {
  return appData.memories.find((memory) => memory.id === memoryId);
}

export function getPrimaryMemoryForTask(taskId) {
  const task = findTask(taskId);
  if (!task || task.relatedMemoryIds.length === 0) {
    return null;
  }
  return findMemory(task.relatedMemoryIds[0]) || null;
}

function createActionBar({
  title,
  impact,
  reversible = '可撤回',
  scopes = ['收束台', '任务控制台', '镜像面板'],
  confirmLabel = '确认',
  allowOnceLabel = '仅这次允许',
  resultNote,
  toastMessage
}) {
  return {
    kind: 'bar',
    bar: {
      title,
      impact,
      reversible,
      scopes,
      confirmLabel,
      allowOnceLabel,
      resultNote: resultNote || title,
      toastMessage: toastMessage || title
    }
  };
}

function createNavigateAction(label, targetView) {
  return {
    id: `${targetView}-${label}`,
    kind: 'navigate',
    label,
    targetView
  };
}

export const workbenchSceneOrder = ['doc-digest', 'contract-diff', 'daily-report'];

export const workbenchScenes = {
  'doc-digest': {
    id: 'doc-digest',
    label: '文档提炼',
    defaultMode: 'understand',
    objectLabel: '当前文档',
    objectTitle: '《桌面协作原型说明草案》',
    objectSummary: '先把当前页面压成一句话摘要，再提炼成可以继续推进的三点。',
    objectTags: ['文档提炼', '现场协作', '静态 mock'],
    relatedTaskId: 'task-ux-route',
    relatedMemoryId: 'mem-desk-entry',
    modes: {
      understand: {
        badge: '理解模式',
        summary: '先看懂，再决定要不要扩写、挂回任务，或写入镜像。',
        cards: [
          { title: '一句话摘要', body: '这是一个桌面常驻的协作原型，主打任务总览、记忆回跳和现场收束。' },
          { title: '三个重点', body: '桌面岛、悬浮球四向、任务与镜像之间的双向可跳。' },
          { title: '差异提炼', body: '它不是聊天入口，而是把行动、理解和记忆放在桌面边缘。' }
        ],
        actions: [
          {
            id: 'doc-refine',
            label: '继续提炼',
            ...createActionBar({
              title: '继续提炼当前文档',
              impact: '会影响收束台、任务控制台和镜像面板的展示方式。',
              reversible: '是，可随时撤回',
              scopes: ['收束台', '任务控制台', '镜像面板'],
              confirmLabel: '确认提炼',
              allowOnceLabel: '仅这次允许',
              resultNote: '已生成更短的文档提炼版。',
              toastMessage: '已提交继续提炼。'
            })
          },
          {
            id: 'doc-daily-tone',
            label: '换成日报语气',
            ...createActionBar({
              title: '把文档改成日报语气',
              impact: '会影响草稿表达和后续任务写法。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认改写',
              resultNote: '已转成日报语气草稿。',
              toastMessage: '已切换到日报语气。'
            })
          },
          {
            id: 'doc-mirror',
            label: '记入镜像',
            ...createActionBar({
              title: '把这条结论记入镜像',
              impact: '会影响镜像面板的最近记忆与长期偏好。',
              reversible: '是，可撤回',
              scopes: ['镜像面板', '收束台'],
              confirmLabel: '确认记入',
              resultNote: '已记入镜像。',
              toastMessage: '已准备写回镜像。'
            })
          },
          createNavigateAction('挂回任务控制台', 'tasks')
        ],
        followUps: ['再简短一点', '换成日报语气', '为什么你觉得这件事重要', '帮我按这个模式重写']
      },
      advance: {
        badge: '推进模式',
        summary: '把文档提炼成可直接推进的草稿、下一步与挂回任务的动作。',
        cards: [
          { title: '草稿骨架', body: '可以先写成“现在状态 / 下一步 / 需要确认”三段式草稿。' },
          { title: '下一步动作', body: '先把入口语义收束，再补执行确认和轻提示。' },
          { title: '回收提醒', body: '推进结果最好能回挂到任务控制台或镜像面板。' }
        ],
        actions: [
          {
            id: 'doc-draft',
            label: '生成草稿',
            ...createActionBar({
              title: '生成草稿并挂回任务',
              impact: '会影响收束台、任务控制台和今日任务。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台', '今日任务'],
              confirmLabel: '确认生成',
              resultNote: '已生成草稿，并准备挂回任务。',
              toastMessage: '已进入执行确认。'
            })
          },
          {
            id: 'doc-add-task',
            label: '加入今日任务',
            ...createActionBar({
              title: '把这一步加入今日任务',
              impact: '会影响今日任务列表与任务控制台的“现在要推进”。',
              reversible: '是，可撤回',
              scopes: ['任务控制台', '今日任务'],
              confirmLabel: '确认加入',
              resultNote: '已加入今日任务。',
              toastMessage: '已准备加入今日任务。'
            })
          },
          {
            id: 'doc-related-material',
            label: '打开相关资料',
            ...createActionBar({
              title: '打开相关资料并核对',
              impact: '会影响当前对象的参照材料与收束台内容。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认打开',
              resultNote: '已打开相关资料。',
              toastMessage: '已准备打开相关资料。'
            })
          },
          createNavigateAction('记入镜像面板', 'memory')
        ],
        followUps: ['生成更像草稿', '保留三段结构', '只留下一步', '挂回任务控制台']
      }
    }
  },
  'contract-diff': {
    id: 'contract-diff',
    label: '合同差异',
    defaultMode: 'understand',
    objectLabel: '当前对象',
    objectTitle: '合同 A / 合同 B 差异摘要',
    objectSummary: '先说差异，再说风险，最后决定要不要写回任务和镜像。',
    objectTags: ['合同差异', '风险摘要', '可回跳任务'],
    relatedTaskId: 'task-permission-copy',
    relatedMemoryId: 'mem-dwell-tone',
    modes: {
      understand: {
        badge: '理解模式',
        summary: '把差异解释清楚，再告诉用户最值得注意的两三处变化。',
        cards: [
          { title: '差异摘要', body: '两版内容主要差在范围描述、语气强度和可撤回说明。' },
          { title: '风险点', body: '如果只看条款本身，用户会忽略“是否会影响桌面常驻体验”的变化。' },
          { title: '影响范围', body: '会影响常驻说明、任务提示语和后续执行确认的文案。' }
        ],
        actions: [
          {
            id: 'contract-explain',
            label: '解释差异',
            ...createActionBar({
              title: '解释这份合同差异',
              impact: '会影响收束台的差异摘要与镜像解释。',
              reversible: '是，可撤回',
              scopes: ['收束台', '镜像面板'],
              confirmLabel: '确认解释',
              resultNote: '已生成差异解释。',
              toastMessage: '已提交差异解释。'
            })
          },
          {
            id: 'contract-law-tone',
            label: '按法务语气重写',
            ...createActionBar({
              title: '按法务语气重写',
              impact: '会影响条款语气与执行确认的描述。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认重写',
              resultNote: '已转成法务语气版本。',
              toastMessage: '已准备法务语气重写。'
            })
          },
          {
            id: 'contract-mirror',
            label: '记入镜像',
            ...createActionBar({
              title: '把差异记入镜像',
              impact: '会影响镜像面板中的长期偏好与阶段镜像。',
              reversible: '是，可撤回',
              scopes: ['镜像面板'],
              confirmLabel: '确认记入',
              resultNote: '已记入差异镜像。',
              toastMessage: '已准备记入镜像。'
            })
          },
          createNavigateAction('挂回任务控制台', 'tasks')
        ],
        followUps: ['差异再明确一点', '只说风险', '按法务语气重写', '讲给产品看']
      },
      advance: {
        badge: '推进模式',
        summary: '把差异变成可执行的修订草稿、核对项和可挂回的任务。',
        cards: [
          { title: '修订草稿', body: '先把变化写成一版更短的修订摘要，再交给任务控制台。' },
          { title: '核对项', body: '需要确认的是范围、回滚方式和常驻说明的措辞。' },
          { title: '执行提醒', body: '如果要推进，最好先记入镜像，再决定是否直接挂回任务。' }
        ],
        actions: [
          {
            id: 'contract-draft',
            label: '生成修订草稿',
            ...createActionBar({
              title: '生成修订草稿并挂回任务',
              impact: '会影响任务控制台和收束台中的修订结果。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认生成',
              resultNote: '已生成修订草稿。',
              toastMessage: '已准备修订草稿。'
            })
          },
          {
            id: 'contract-add-task',
            label: '加入今日任务',
            ...createActionBar({
              title: '把差异修订加入今日任务',
              impact: '会影响今日任务和任务控制台的“现在要推进”。',
              reversible: '是，可撤回',
              scopes: ['任务控制台', '今日任务'],
              confirmLabel: '确认加入',
              resultNote: '已加入今日任务。',
              toastMessage: '已准备加入今日任务。'
            })
          },
          {
            id: 'contract-open-material',
            label: '打开相关资料',
            ...createActionBar({
              title: '打开相关资料并核对',
              impact: '会影响当前对象的核对范围。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认打开',
              resultNote: '已打开相关资料。',
              toastMessage: '已准备打开相关资料。'
            })
          },
          createNavigateAction('记入镜像面板', 'memory')
        ],
        followUps: ['生成更短版本', '补上回滚说明', '换成产品语气', '只保留风险']
      }
    }
  },
  'daily-report': {
    id: 'daily-report',
    label: '日报骨架',
    defaultMode: 'advance',
    objectLabel: '当前对象',
    objectTitle: '今日日报骨架',
    objectSummary: '把今天的进展、阻塞和下一步收成一页，方便继续推进。',
    objectTags: ['日报生成', '推进动作', '今日任务'],
    relatedTaskId: 'task-review-pack',
    relatedMemoryId: 'mem-correction-link',
    modes: {
      understand: {
        badge: '理解模式',
        summary: '先把今天发生了什么说清楚，再决定是否继续推进。',
        cards: [
          { title: '今日进展', body: '桌面常驻、任务总览和镜像回跳已经跑通静态链路。' },
          { title: '阻塞点', body: '仍然需要把协作层和执行确认说得更清楚。' },
          { title: '需要确认', body: '哪些动作应该直接挂回任务，哪些只做确认。' }
        ],
        actions: [
          {
            id: 'daily-compact',
            label: '继续压缩',
            ...createActionBar({
              title: '继续压缩日报骨架',
              impact: '会影响收束台中的日报摘要和任务控制台的分区表达。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认压缩',
              resultNote: '已压缩日报骨架。',
              toastMessage: '已准备继续压缩。'
            })
          },
          {
            id: 'daily-morning-tone',
            label: '换成晨会语气',
            ...createActionBar({
              title: '把日报换成晨会语气',
              impact: '会影响草稿表达和任务控制台的提示语。',
              reversible: '是，可撤回',
              scopes: ['收束台', '今日任务'],
              confirmLabel: '确认改写',
              resultNote: '已改成晨会语气。',
              toastMessage: '已准备晨会语气改写。'
            })
          },
          {
            id: 'daily-mirror',
            label: '记入镜像',
            ...createActionBar({
              title: '把日报骨架记入镜像',
              impact: '会影响镜像面板中的阶段镜像与长期偏好。',
              reversible: '是，可撤回',
              scopes: ['镜像面板'],
              confirmLabel: '确认记入',
              resultNote: '已记入日报镜像。',
              toastMessage: '已准备记入镜像。'
            })
          },
          createNavigateAction('挂回任务控制台', 'tasks')
        ],
        followUps: ['再简短一点', '换成晨会语气', '为什么这件事重要', '只保留阻塞点']
      },
      advance: {
        badge: '推进模式',
        summary: '把当天内容收束成一版能直接挂回任务的日报草稿。',
        cards: [
          { title: '日报草稿', body: '建议直接写成“进展 / 阻塞 / 下一步”三段式。' },
          { title: '明日最先做', body: '把最顺的一步提到最前面，减少用户回看成本。' },
          { title: '回收提醒', body: '如果要同步到镜像，最好保留一句为什么这样判断。' }
        ],
        actions: [
          {
            id: 'daily-draft',
            label: '生成日报草稿',
            ...createActionBar({
              title: '生成日报草稿并挂回任务',
              impact: '会影响收束台、任务控制台和今日任务列表。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台', '今日任务'],
              confirmLabel: '确认生成',
              resultNote: '已生成日报草稿。',
              toastMessage: '已准备日报草稿。'
            })
          },
          {
            id: 'daily-add-task',
            label: '加入今日任务',
            ...createActionBar({
              title: '把日报里的动作加入今日任务',
              impact: '会影响任务控制台的“现在要推进”。',
              reversible: '是，可撤回',
              scopes: ['任务控制台', '今日任务'],
              confirmLabel: '确认加入',
              resultNote: '已加入今日任务。',
              toastMessage: '已准备加入今日任务。'
            })
          },
          {
            id: 'daily-open-material',
            label: '打开相关资料',
            ...createActionBar({
              title: '打开相关资料并核对',
              impact: '会影响当前对象的来源说明。',
              reversible: '是，可撤回',
              scopes: ['收束台', '任务控制台'],
              confirmLabel: '确认打开',
              resultNote: '已打开相关资料。',
              toastMessage: '已准备打开相关资料。'
            })
          },
          createNavigateAction('记入镜像面板', 'memory')
        ],
        followUps: ['日报更像骨架', '补上阻塞点', '只留下一步', '挂回任务控制台']
      }
    }
  }
};

export const hintCards = [
  {
    id: 'hint-doc-digest',
    title: '当前页面可提炼',
    body: '先把当前页面压成一句话摘要，再提炼成三个重点。',
    sceneId: 'doc-digest',
    mode: 'understand',
    sourceLabel: '创作工作台',
    triggerLabel: '可提炼',
    actionLabel: '打开收束台'
  },
  {
    id: 'hint-contract-diff',
    title: '这处差异可先解释',
    body: '两版内容已经足够分开，适合先做差异摘要。',
    sceneId: 'contract-diff',
    mode: 'understand',
    sourceLabel: '镜像浮层',
    triggerLabel: '可解释差异',
    actionLabel: '打开收束台'
  },
  {
    id: 'hint-daily-report',
    title: '这段可接成日报',
    body: '今天的进展与阻塞已经足够整理成日报骨架。',
    sceneId: 'daily-report',
    mode: 'advance',
    sourceLabel: '巡检视窗',
    triggerLabel: '可生成日报',
    actionLabel: '打开收束台'
  }
];

export const desktopSignalCards = [
  { windowId: 'window-focus', hintId: 'hint-doc-digest' },
  { windowId: 'window-review', hintId: 'hint-daily-report' },
  { windowId: 'window-memory', hintId: 'hint-contract-diff' }
];

export const taskWorkbenchBindings = {
  'task-ux-route': { sceneId: 'doc-digest', mode: 'advance' },
  'task-memory-loop': { sceneId: 'daily-report', mode: 'advance' },
  'task-stall-recover': { sceneId: 'contract-diff', mode: 'advance' },
  'task-understand-tone': { sceneId: 'doc-digest', mode: 'advance' },
  'task-permission-copy': { sceneId: 'contract-diff', mode: 'advance' },
  'task-review-pack': { sceneId: 'daily-report', mode: 'advance' }
};

export const memoryWorkbenchBindings = {
  'mem-desk-entry': { sceneId: 'doc-digest', mode: 'understand' },
  'mem-user-voice': { sceneId: 'daily-report', mode: 'understand' },
  'mem-stage-fold': { sceneId: 'daily-report', mode: 'advance' },
  'mem-mode-shift': { sceneId: 'doc-digest', mode: 'understand' },
  'mem-orb-direction': { sceneId: 'doc-digest', mode: 'understand' },
  'mem-dwell-tone': { sceneId: 'contract-diff', mode: 'understand' },
  'mem-understand-style': { sceneId: 'doc-digest', mode: 'understand' },
  'mem-correction-link': { sceneId: 'daily-report', mode: 'advance' },
  'mem-review-rhythm': { sceneId: 'contract-diff', mode: 'advance' }
};

export function findWorkbenchScene(sceneId) {
  return workbenchScenes[sceneId] || workbenchScenes['doc-digest'];
}

export function findHintCard(hintId) {
  return hintCards.find((hint) => hint.id === hintId) || null;
}

export function getTaskWorkbenchBinding(taskId) {
  return taskWorkbenchBindings[taskId] || { sceneId: 'doc-digest', mode: 'advance' };
}

export function getMemoryWorkbenchBinding(memoryId) {
  return memoryWorkbenchBindings[memoryId] || { sceneId: 'doc-digest', mode: 'understand' };
}
