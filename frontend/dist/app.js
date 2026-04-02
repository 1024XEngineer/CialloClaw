const desktopCards = [
  { id: 'web-note', label: '网页片段', title: '今天的资料页里有一段可以快速提炼', text: '页面里夹着 3 段高密度内容：一个更新说明、一组对比点、以及一个需要回写的提醒。', footerLeft: '片段待提炼', footerRight: 'fragment / text', sourceType: 'fragment' },
  { id: 'doc-note', label: '文档卡片', title: '有一页合同草案正在等待差异确认', text: '这张卡模拟了一份被标注过的合同差异。它会在协作窗里变成更顺手的推进入口。', footerLeft: '合同差异', footerRight: 'file / diff', sourceType: 'file' },
  { id: 'task-note', label: '任务现场', title: '当前有几项待推进的工作在桌面上散开', text: '你可以把它当成一张碎片任务卡。它会把协作窗带回今天的推进节奏。', footerLeft: '待推进', footerRight: 'task / queue', sourceType: 'task' },
  { id: 'error-note', label: '报错窗口', title: '一个旧报错：反馈有轻微抖动', text: '这不是一个聊天线程，而是一段现场问题。它会把协作窗带到更靠近校正的视角。', footerLeft: 'error log', footerRight: 'stack / triage', sourceType: 'screenshot' },
];

const tasks = [
  { id: 'task-1', bucket: '当前最该推进', status: 'active', title: '把网页片段提炼成 3 条回写点', summary: '来自今天上午那页密度很高的资料页，需要先收束，再决定是否挂回今天。', why: '它会影响今天的表达节奏，而且相关记忆已经形成新的偏好。', lastStop: '停在第一段背景说明，尚未收束成可回写版本。', nextStep: '先提炼，再补一个最省力的回复草稿。', due: '今天 18:00', heat: 'hot', tags: ['提炼', '回写', '资料页'], materials: ['网页片段', '截图卡', '历史偏好'], memories: ['memory-1', 'memory-4'], focusMode: 'understand' },
  { id: 'task-2', bucket: '即将到期', status: 'due', title: '把合同差异整理成提醒', summary: '需要把条款变化拆成更容易发出去的提醒片段，避免漏看风险。', why: '它和“先看差异再看全局”的处理偏好相互呼应。', lastStop: '停在第 4 条条款，风险等级还没统一。', nextStep: '先用协作窗做差异查看，再落到推进建议。', due: '明天 10:30', heat: 'warm', tags: ['合同', '差异', '提醒'], materials: ['合同草案', '标红截图'], memories: ['memory-2', 'memory-7'], focusMode: 'advance' },
  { id: 'task-3', bucket: '已停滞', status: 'stalled', title: '旧报错：界面反馈有轻微抖动', summary: '这是一个可见但不刺眼的 bug，最适合先进入看懂层，再决定是否推进。', why: '它与潮汐核边缘反馈的冲突直接相关。', lastStop: '停在鼠标抖动与 hover ring 冲突的判断。', nextStep: '先看懂现场，再决定是否写回任务抽屉。', due: '无固定期限', heat: 'cold', tags: ['报错', '反馈', '现场'], materials: ['报错窗口', '截图卡'], memories: ['memory-3', 'memory-8'], focusMode: 'review' },
  { id: 'task-4', bucket: '等待中', status: 'waiting', title: '整理成待发回复', summary: '这件事不需要完整重写，只要把现有结果收成可发送版本。', why: '对方在等一句清楚、安静、可撤回的答复。', lastStop: '停在第二轮措辞整理。', nextStep: '把结果压成三行，再挂回今天。', due: '今天 16:00', heat: 'warm', tags: ['回复', '发送', '轻量'], materials: ['网页片段', '历史习惯'], memories: ['memory-5', 'memory-10'], focusMode: 'rewrite' },
  { id: 'task-5', bucket: '项目类任务', status: 'project', title: 'demo3 空间层级定稿', summary: '把桌面、潮汐核、协作窗、任务抽屉、记忆抽屉统一成一张场景图。', why: '这是整个原型的主骨架，决定后续所有工作层如何联动。', lastStop: '停在层级关系与视觉语言的统一。', nextStep: '把“看懂 / 改写 / 推进”三模式落成可操作结构。', due: '本周五', heat: 'hot', tags: ['项目', '层级', '空间'], materials: ['协作窗', '记忆抽屉'], memories: ['memory-6', 'memory-9'], focusMode: 'advance' },
  { id: 'task-6', bucket: '碎片类任务', status: 'fragment', title: '把记忆规则碎片收进长期偏好', summary: '一些观察已经开始稳定，可以逐步从观察卡升级成镜像卡。', why: '它能减少重复判断，并让系统更像“懂你”的工作层。', lastStop: '停在观察卡与镜像卡的边界。', nextStep: '把可复用的观察固化成偏好。', due: '随手', heat: 'cool', tags: ['记忆', '偏好', '固化'], materials: ['观察卡', '阶段折页'], memories: ['memory-1', 'memory-2', 'memory-10'], focusMode: 'review' },
];

const memories = [
  { id: 'memory-1', layer: '最近记忆', kind: 'observation', title: '你常先看局部，再回到全局', summary: '最近几次协作都先从片段、截图或单页内容开始，再把它们回收成结构。', why: '这会帮助系统优先显示可提炼片段，而不是一上来铺满全局。', linkedTasks: ['task-1', 'task-6'], focusMode: 'understand', confidence: 0.73, muted: false, corrected: false, state: '观察中' },
  { id: 'memory-2', layer: '最近记忆', kind: 'observation', title: '你对安静协作比弹窗提醒更敏感', summary: '过去的选择倾向于低打扰的提示方式，而不是高频通知。', why: '因此系统应该把主动提醒压成轨道里的状态，而不是跳出的消息。', linkedTasks: ['task-2', 'task-4'], focusMode: 'review', confidence: 0.68, muted: false, corrected: false, state: '观察中' },
  { id: 'memory-3', layer: '最近记忆', kind: 'observation', title: '右键菜单会把空间感压扁成选项列表', summary: '已经明确不适合把核心协作动作藏进传统弹出菜单。', why: '所以潮汐核必须用可感知的空间展开，而不是菜单弹窗。', linkedTasks: ['task-3'], focusMode: 'understand', confidence: 0.87, muted: false, corrected: false, state: '观察中' },
  { id: 'memory-4', layer: '阶段镜像', kind: 'mirror', title: '你更喜欢单场景展开，而不是页面跳转', summary: '协作现场会在同一桌面上展开多个工作层，而不是进入另一个总控平台。', why: '这让理解、推进和校正都能在同一空间里回流。', linkedTasks: ['task-1', 'task-5'], focusMode: 'review', confidence: 0.92, muted: false, corrected: false, state: '稳定镜像' },
  { id: 'memory-5', layer: '阶段镜像', kind: 'mirror', title: '你会保留多个待推进事项，但不想被淹没', summary: '任务应该有前后排、热度和状态，而不是一把平铺到网格里。', why: '任务抽屉需要“带”和“聚焦区”，而不是普通后台。', linkedTasks: ['task-4', 'task-5'], focusMode: 'rewrite', confidence: 0.89, muted: false, corrected: false, state: '稳定镜像' },
  { id: 'memory-6', layer: '阶段折页', kind: 'stage', title: '本周轨迹：理解 → 推进 → 校正', summary: '你曾在同一段协作里完成提炼、分派、回看和修正，说明层与层之间必须联动。', why: '这正好对应潮汐核、协作窗、任务抽屉和记忆抽屉之间的回路。', linkedTasks: ['task-5'], focusMode: 'advance', confidence: 0.84, muted: false, corrected: false, state: '阶段折页' },
  { id: 'memory-7', layer: '阶段折页', kind: 'stage', title: '上周：从文档提炼走向合同比对', summary: '理解层的结果越来越常直接进入推进层，不再停留在摘要本身。', why: '说明协作窗应该同时支持“看懂”和“继续推进”。', linkedTasks: ['task-2'], focusMode: 'advance', confidence: 0.8, muted: false, corrected: true, state: '阶段折页' },
  { id: 'memory-8', layer: '记忆管理', kind: 'correction', title: '不该把“系统轨道”误解成普通菜单', summary: '这条记忆已经被点过“这不对”，需要保持可回看、可校正。', why: '系统轨道必须像从球体背后抽出的细长轨，而不是右键菜单。', linkedTasks: ['task-3'], focusMode: 'review', confidence: 0.65, muted: false, corrected: true, state: '待校正' },
];

const modeViews = {
  understand: {
    label: '看懂',
    summary: ['已整理 3 个重点', '可继续推进', '记住了一条新偏好'],
    prompts: ['再浓缩一点', '用更白话一点解释', '为什么第三点重要'],
    placeholder: '例如：再浓缩一点 / 用更白话一点解释',
    variants: [
      { oneLine: '这页适合先提炼成一句话，再决定要不要继续往下做。', points: ['先抓出能回写的 1 个点', '别先铺满全局', '把信息压成轻提示'], diff: '与原文相比，重点被压缩成了 3 个最能回写的部分。', why: '我这样判断，是因为这页的密度高，但可以先得到一个更短的骨架。' },
      { oneLine: '先把它压成一句话：这页的核心是一个可以回写的变化点。', points: ['变化点优先', '细节稍后补', '先定方向再展开'], diff: '差异提炼成“变化点 + 回写点 + 下一步”三层。', why: '这是因为你通常先看局部，再回到全局。' },
      { oneLine: '这页不是要读完，而是先抓出能落到今天的那一条。', points: ['今天能做', '明天可补', '别被全文拖住'], diff: '差异被进一步收束成“今天就能动”的版本。', why: '因为当前现场更适合低打扰、低负担的进入方式。' },
    ],
  },
  rewrite: {
    label: '改写',
    summary: ['语气已切到更顺手的版本', '可以直接套用', '记住这次会更稳'],
    prompts: ['换成日报语气', '再正式一点', '再短一点'],
    placeholder: '例如：换成日报语气 / 再短一点',
    variants: [
      { oneLine: '改写结果已经更接近日报语气，能直接发出去。', tone: '日报语气', alternatives: ['更正式一版', '更短一版', '更像协作记录的一版'], replace: '推荐直接替换成这版，避免再做第二次解释。' },
      { oneLine: '这版更稳，适合拿去当一个安静的回写草稿。', tone: '安静 / 克制', alternatives: ['更正式一点', '更白话一点', '更像内部记录'], replace: '可以直接套用这次的口气与长度。' },
      { oneLine: '再短一点后，信息仍然完整，但没有多余语气。', tone: '更短更稳', alternatives: ['日报版', '待发送版', '内部备注版'], replace: '适合先记住这次写法，再继续收短。' },
    ],
  },
  advance: {
    label: '推进',
    summary: ['下一步建议已生成', '可稍后继续', '可以先放这儿'],
    prompts: ['下一步先做哪个', '给我一个能直接发出的版本', '帮我拆成动作'],
    placeholder: '例如：下一步先做哪个 / 帮我拆成动作',
    variants: [
      { oneLine: '下一步最顺手的是先生成一条可发出的草稿，再决定是否继续细化。', nextSteps: ['先做一个最小草稿', '把影响范围列出来', '只保留必要动作'], draft: '草稿可以先保留为“今天待推进”的最小版本。', ops: '先别扩展范围，避免把协作窗又变回工作台。' },
      { oneLine: '如果要推进，就先把来源打开，再确认要不要直接发。', nextSteps: ['打开来源', '确认目标对象', '再决定是否发送'], draft: '这份草稿更适合挂在旁边，晚点再继续。', ops: '推进不要抢过理解层，先让当前结果稳定。' },
      { oneLine: '先放这儿最稳：今天只推进最关键的那一步。', nextSteps: ['先放这儿', '晚点继续', '只保留一条关键动作'], draft: '下一步可以先收藏，不急着一次做完。', ops: '如果要撤回，当前动作可以直接退回微态。' },
    ],
  },
  review: {
    label: '回看',
    summary: ['关联任务已列出', '关联记忆已对齐', '这次判断可校正'],
    prompts: ['为什么你联想到这个历史任务', '这个偏好是怎么来的', '暂时不要用这个习惯'],
    placeholder: '例如：为什么你联想到这个历史任务 / 暂时不要用这个习惯',
    variants: [
      { oneLine: '这次判断关联到今天最该推进的任务与一条稳定偏好。', tasks: ['task-1', 'task-5'], memories: ['memory-4', 'memory-6'], history: '和过去几次“先看局部，再回到全局”的处理方式接近。', why: '系统会把这类回看放进同一条协作轨迹里。' },
      { oneLine: '这个偏好来自你多次把结果挂回今天，而不是直接归档。', tasks: ['task-2', 'task-4'], memories: ['memory-1', 'memory-5'], history: '类似历史处理方式更偏向低打扰、先局部后整体。', why: '所以回看时会优先提示“先放这儿”的动作。' },
      { oneLine: '这次回看触发了一个可校正项：有些记忆该暂时不要用。', tasks: ['task-3', 'task-6'], memories: ['memory-3', 'memory-8'], history: '历史上这类情况通常会先被标成“这不对”。', why: '这能避免把普通菜单式交互误当成长期偏好。' },
    ],
  },
};

const state = {
  shellOpen: false,
  shellState: 'hidden', // hidden | micro | half | collapsed
  shellDock: 'right', // left | right | bottom
  shellOffset: null,
  activeMode: 'understand',
  modeVariantIndex: { understand: 0, rewrite: 0, advance: 0, review: 0 },
  modeDetailOpen: { understand: false, rewrite: false, advance: false, review: false },
  selectedTaskId: 'task-1',
  selectedMemoryId: 'memory-4',
  sourceLayerOpen: false, // 左滑来源层：显示来源、关联任务/记忆
  followupPageOpen: false,
  followupInputOpen: false,
  followupHistory: [],
  sourceFocus: 'desk',
  sourceTitle: '等待承接',
  sourceId: '',
  quietMode: false,
  remindersPaused: false,
  coreRingVisible: false,
  coreSplitVisible: false,
  coreHotSlot: null,
  drag: null,
  shellDrag: null,
  ringTimer: null,
  longPressTimer: null,
  idleTimer: null,
  toasts: [],
};

const els = {};

function bootstrap() {
  bindElements();
  bindEvents();
  renderDesktopCards();
  renderAll();
  queueToast('桌面已就位', '双击潮汐核打开悬浮协作窗', 'quiet');
}

function bindElements() {
  for (const id of [
    'app', 'globalStatus', 'desktopWindows', 'desktopScene', 'tidalCore', 'tidalStatus', 'tidalSlots', 'systemTrack', 'systemSettings', 'collabTitle',
    'collabCluster', 'collabWindow', 'collabDragHandle', 'modeSwitchers', 'collabSummary', 'modeResults', 'followupPrompt',
    'followupHint', 'followupChips', 'followupInputRow', 'followupInput', 'followupSend', 'sourceLayer', 'sourceLayerBody',
    'followupPage', 'followupPageBody', 'toastStack',
  ]) {
    els[id] = document.getElementById(id);
  }
}

function bindEvents() {
  document.addEventListener('click', onDocumentClick);
  document.addEventListener('pointermove', onPointerMove);
  document.addEventListener('pointerup', onPointerUp);
  document.addEventListener('pointercancel', onPointerUp);
  document.addEventListener('keydown', onKeyDown);
  window.addEventListener('resize', onWindowResize);

  els.tidalCore.addEventListener('pointerdown', onCorePointerDown);
  els.tidalCore.addEventListener('dblclick', onCoreDoubleClick);
  els.collabDragHandle.addEventListener('dblclick', onShellHeaderDoubleClick);
  els.collabDragHandle.addEventListener('pointerdown', onShellPointerDown);
  els.followupPrompt.addEventListener('click', () => toggleFollowupInput(true));
  els.followupSend.addEventListener('click', submitFollowupFromInput);
  els.followupInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') submitFollowupFromInput();
  });
}

function renderAll() {
  renderCore();
  renderShell();
  renderToasts();
  updateStatus();
}

function renderDesktopCards() {
  els.desktopWindows.innerHTML = desktopCards.map((card, index) => `
    <article class="desktop-window desktop-window--${index + 1}" data-source-id="${card.id}">
      <div class="desktop-window__chrome">
        <div class="desktop-window__dots" aria-hidden="true"><span></span><span></span><span></span></div>
        <div class="desktop-window__label">${escapeHtml(card.label)}</div>
      </div>
      <div class="desktop-window__tagline"><b>${escapeHtml(card.footerLeft)}</b><span>${escapeHtml(card.footerRight)}</span></div>
      <h3 class="desktop-window__title">${escapeHtml(card.title)}</h3>
      <p class="desktop-window__text">${escapeHtml(card.text)}</p>
      <div class="desktop-window__footer">
        <span class="desktop-window__meta"><span class="desktop-window__dot"></span>${escapeHtml(card.footerLeft)}</span>
        <span>${escapeHtml(card.footerRight)}</span>
      </div>
    </article>
  `).join('');
}

function renderCore() {
  els.tidalCore.classList.toggle('is-ringed', state.coreRingVisible);
  els.tidalCore.classList.toggle('is-split', state.coreSplitVisible);
  els.tidalCore.classList.toggle('is-track-open', state.shellOpen && state.shellState !== 'hidden' && state.activeMode === 'review' && state.followupHistory.length > 0);
  els.tidalSlots.querySelectorAll('.tidal-slot').forEach((slot) => {
    slot.classList.toggle('is-hot', state.coreHotSlot === slot.dataset.slot);
  });
  // 单一轻量状态提示（取代原来的 3 个 ring items）
  const statusText = state.quietMode 
    ? '有 1 项待推进（静音中）' 
    : state.remindersPaused 
      ? '提醒已暂停' 
      : '有 2 项待推进';
  els.tidalStatus.querySelector('.tidal-status__text').textContent = statusText;
  els.systemSettings.innerHTML = state.activeMode === 'review' ? `
    <div class="track-settings">
      <div class="track-settings__title">系统轨道</div>
      <div class="track-settings__grid">
        <label class="track-settings__row mini-switch"><span>安静模式</span><input type="checkbox" data-system-toggle="quiet" ${state.quietMode ? 'checked' : ''} /><small>让空间更安静</small></label>
        <label class="track-settings__row mini-switch"><span>暂停提醒</span><input type="checkbox" data-system-toggle="pause" ${state.remindersPaused ? 'checked' : ''} /><small>减少打扰</small></label>
      </div>
    </div>
  ` : '';
}

function renderShell() {
  const visible = state.shellOpen && state.shellState !== 'hidden';
  els.collabCluster.classList.toggle('is-hidden', !visible);
  els.collabCluster.classList.remove('dock-left', 'dock-right', 'dock-bottom');
  els.collabCluster.classList.add(`dock-${state.shellDock}`);
  els.collabCluster.classList.toggle('is-dragging', !!state.shellDrag);

  if (state.shellOffset) {
    els.collabCluster.style.left = `${state.shellOffset.left}px`;
    els.collabCluster.style.top = `${state.shellOffset.top}px`;
    els.collabCluster.style.right = 'auto';
    els.collabCluster.style.bottom = 'auto';
    els.collabCluster.style.transform = 'none';
  } else {
    els.collabCluster.style.left = '';
    els.collabCluster.style.top = '';
    els.collabCluster.style.right = '';
    els.collabCluster.style.bottom = '';
    els.collabCluster.style.transform = state.shellDock === 'bottom' ? 'translateX(-50%)' : '';
  }

  els.collabWindow.classList.remove('state-hidden', 'state-micro', 'state-half', 'state-collapsed');
  els.collabWindow.classList.add(`state-${state.shellState}`);
  if (els.collabTitle) els.collabTitle.textContent = state.sourceTitle || '等待承接';

  renderModeSwitchers();
  renderSummary();
  renderModeResults();
  renderFollowupBar();
  renderSourceLayer();
  renderFollowupPage();

  els.sourceLayer.classList.toggle('is-open', state.sourceLayerOpen && state.shellState === 'half');
  els.followupPage.classList.toggle('is-open', state.followupPageOpen && state.shellState === 'half');
}

function renderModeSwitchers() {
  const visibleModes = ['understand', 'rewrite', 'advance'];
  els.modeSwitchers.innerHTML = visibleModes.map((key) => {
    const view = modeViews[key];
    return `
    <button class="mode-pill ${state.activeMode === key ? 'is-active' : ''}" type="button" data-mode-switch="${key}">${escapeHtml(view.label)}</button>
  `;}).join('');
}

function currentVariant(mode = state.activeMode) {
  const view = modeViews[mode];
  return view.variants[state.modeVariantIndex[mode] % view.variants.length];
}

function renderSummary() {
  const view = modeViews[state.activeMode];
  const variant = currentVariant();
  const status = state.shellState === 'micro' ? '微态' : state.shellState === 'collapsed' ? '折叠' : '半展开';
  if (state.shellState !== 'half') {
    els.collabSummary.innerHTML = `
      <div class="summary-strip summary-strip--compact">
        <span class="summary-strip__badge">${escapeHtml(status)}</span>
        <span class="summary-strip__mid">${escapeHtml(view.summary.join(' · '))}</span>
      </div>
    `;
    els.followupHint.textContent = '点击协作窗可恢复展开';
    return;
  }
  els.collabSummary.innerHTML = `
    <div class="summary-strip">
      <div class="summary-strip__left"><span class="summary-strip__badge">${escapeHtml(view.label)}</span><span class="summary-strip__text">${escapeHtml(view.summary[0])}</span></div>
      <div class="summary-strip__mid">${escapeHtml(variant.oneLine)}</div>
      <div class="summary-strip__right"><span class="summary-strip__badge summary-strip__badge--soft">${escapeHtml(status)}</span><span class="summary-strip__badge summary-strip__badge--soft">${escapeHtml(state.sourceTitle)}</span></div>
    </div>
  `;
  els.followupHint.textContent = state.followupHistory.length >= 2 ? `已经连续追问 ${state.followupHistory.length} 次` : `点击提示词，展开单行续写`;
}

function renderModeResults() {
  const variant = currentVariant();
  const mode = state.activeMode;
  const expanded = !!state.modeDetailOpen[mode];
  els.modeResults.classList.toggle('mode-expanded', expanded);
  const cards = [];

  if (mode === 'understand') {
    cards.push(resultCard('一句话摘要', `<p>${escapeHtml(variant.oneLine)}</p>`, [
      resultActionButton('留在今天', '留在今天'),
      resultActionButton('打开来源', '打开来源', true),
      resultActionButton(expanded ? '收起更多' : '展开更多', 'toggle-more', true),
    ]));
    cards.push(resultCard('三个重点', listMarkup(variant.points)));
    if (expanded) {
      cards.push(resultCard('差异提炼', `<p>${escapeHtml(variant.diff)}</p>`));
      cards.push(resultCard('原因解释', `<p>${escapeHtml(variant.why)}</p>`));
    }
  }
  if (mode === 'rewrite') {
    cards.push(resultCard('当前改写版本', `<p>${escapeHtml(variant.oneLine)}</p>`, [
      resultActionButton('记住这次', '记住这次'),
      resultActionButton('套用这版', '套用这版', true),
      resultActionButton(expanded ? '收起更多' : '展开更多', 'toggle-more', true),
    ]));
    cards.push(resultCard('当前语气标签', chipListMarkup([variant.tone])));
    if (expanded) {
      cards.push(resultCard('备选版本', listMarkup(variant.alternatives)));
      cards.push(resultCard('可直接替换', `<p>${escapeHtml(variant.replace)}</p>`));
    }
  }
  if (mode === 'advance') {
    cards.push(resultCard('下一步建议', listMarkup(variant.nextSteps), [
      resultActionButton('稍后继续', '稍后继续'),
      resultActionButton('先放这儿', '先放这儿', true),
      resultActionButton(expanded ? '收起更多' : '展开更多', 'toggle-more', true),
    ]));
    cards.push(resultCard('可直接采用的小草稿', `<p>${escapeHtml(variant.draft)}</p>`));
    if (expanded) {
      cards.push(resultCard('后续动作建议', `<p>${escapeHtml(variant.ops)}</p>`));
      cards.push(resultCard('当前目标', `<p>现在最适合处理：${escapeHtml(currentTask().title)}</p>`));
    }
  }
  if (mode === 'review') {
    cards.push(resultCard('当前判断', `<p>${escapeHtml(variant.oneLine)}</p>`, [
      resultActionButton('这不对', '这不对', true),
      resultActionButton('暂不采用', '暂不采用'),
      resultActionButton(expanded ? '收起更多' : '展开更多', 'toggle-more', true),
    ]));
    cards.push(resultCard('校正方向', `<p>${escapeHtml(variant.history)}</p>`));
    if (expanded) {
      cards.push(resultCard('为什么这样判断', `<p>${escapeHtml(variant.why)}</p>`));
      cards.push(resultCard('关联线索', `${taskChipList(variant.tasks)}${memoryChipList(variant.memories)}`));
    }
  }

  els.modeResults.innerHTML = cards.join('');
}

function renderFollowupBar() {
  const view = modeViews[state.activeMode];
  const variant = currentVariant();
  els.followupPrompt.textContent = promptLabelForMode(state.activeMode);
  els.followupChips.innerHTML = view.prompts.map((hint) => `<button class="followup-chip" type="button" data-followup-chip="${escapeHtml(hint)}">${escapeHtml(hint)}</button>`).join('');
  els.followupInput.placeholder = view.placeholder;
  els.followupInputRow.classList.toggle('is-hidden', !state.followupInputOpen);
  if (state.shellState === 'micro' || state.shellState === 'collapsed') {
    els.followupHint.textContent = variant.oneLine;
  }
}

function resolveSourceLayerContext() {
  const lastTurn = state.followupHistory[state.followupHistory.length - 1] || null;
  const currentTaskItem = findTask(state.selectedTaskId) || tasks[0];
  const currentMemoryItem = findMemory(state.selectedMemoryId) || memories[0];

  if (state.sourceFocus === 'task') {
    const task = findTask(state.sourceId) || currentTaskItem;
    return {
      title: task.title,
      meta: `${task.bucket} · ${task.due}`,
      body: task.summary,
      why: task.why,
      linkedTasks: [task],
      linkedMemories: task.memories.map((id) => findMemory(id)).filter(Boolean),
      previousOrigin: lastTurn ? `${modeViews[lastTurn.mode].label} · ${lastTurn.response}` : '还没有上一轮结果',
    };
  }

  if (state.sourceFocus === 'memory') {
    const memory = findMemory(state.sourceId) || currentMemoryItem;
    return {
      title: memory.title,
      meta: `${memory.layer} · ${memory.state}`,
      body: memory.summary,
      why: memory.why,
      linkedTasks: memory.linkedTasks.map((id) => findTask(id)).filter(Boolean),
      linkedMemories: [memory],
      previousOrigin: lastTurn ? `${modeViews[lastTurn.mode].label} · ${lastTurn.response}` : '还没有上一轮结果',
    };
  }

  if (state.sourceFocus === 'core') {
    const task = findTask(state.sourceId) || currentTaskItem;
    return {
      title: state.sourceTitle || task.title,
      meta: `核心承接 · ${modeViews[state.activeMode].label}`,
      body: task.summary,
      why: task.why,
      linkedTasks: [task],
      linkedMemories: task.memories.map((id) => findMemory(id)).filter(Boolean),
      previousOrigin: lastTurn ? `${modeViews[lastTurn.mode].label} · ${lastTurn.response}` : '还没有上一轮结果',
    };
  }

  const source = sourceForId(state.sourceId || 'web-note');
  return {
    title: source.title,
    meta: `${source.label} · ${source.footerRight}`,
    body: source.text,
    why: `当前更适合进入「${modeViews[modeForSourceType(source.sourceType)].label}」`,
    linkedTasks: [currentTaskItem],
    linkedMemories: [currentMemoryItem],
    previousOrigin: lastTurn ? `${modeViews[lastTurn.mode].label} · ${lastTurn.response}` : '还没有上一轮结果',
  };
}

// 左滑来源层：显示当前来源对象、为什么命中当前意图、关联的任务/记忆
function renderSourceLayer() {
  const context = resolveSourceLayerContext();

  els.sourceLayerBody.innerHTML = `
    <section class="source-context">
      <div class="source-context__kicker">当前来源</div>
      <article class="source-card">
        <div class="source-card__meta">${escapeHtml(context.meta)}</div>
        <div class="source-card__title">${escapeHtml(context.title)}</div>
        <div class="source-card__body">${escapeHtml(context.body)}</div>
      </article>
    </section>

    <section class="source-origin">
      <div class="source-context__kicker">为什么命中</div>
      <p class="source-origin__text">${escapeHtml(context.why)}</p>
    </section>

    ${context.linkedTasks.length > 0 ? `
    <section class="source-links">
      <div class="source-context__kicker">关联任务</div>
      <div class="source-links__list">
        ${context.linkedTasks.map((task) => `
          <div class="source-link-item" data-linked-task="${task.id}">
            <span class="source-link-item__badge">${escapeHtml(task.bucket)}</span>
            <span class="source-link-item__title">${escapeHtml(task.title)}</span>
          </div>
        `).join('')}
      </div>
    </section>
    ` : ''}

    ${context.linkedMemories.length > 0 ? `
    <section class="source-links">
      <div class="source-context__kicker">关联记忆</div>
      <div class="source-links__list">
        ${context.linkedMemories.map((memory) => `
          <div class="source-link-item" data-linked-memory="${memory.id}">
            <span class="source-link-item__badge">${escapeHtml(memory.layer)}</span>
            <span class="source-link-item__title">${escapeHtml(memory.title)}</span>
          </div>
        `).join('')}
      </div>
    </section>
    ` : ''}

    <section class="source-origin">
      <div class="source-context__kicker">前一轮结果来源</div>
      <p class="source-origin__text">${escapeHtml(context.previousOrigin)}</p>
    </section>
  `;
}

function renderFollowupPage() {
  const turns = state.followupHistory.slice(-5);
  els.followupPageBody.innerHTML = turns.length
    ? turns.map((turn, index) => `
      <article class="turn-card">
        <div class="turn-card__meta">第 ${index + 1} 轮 · ${escapeHtml(modeViews[turn.mode].label)}</div>
        <div class="turn-card__q"><span>你问</span><p>${escapeHtml(turn.input)}</p></div>
        <div class="turn-card__a"><span>回应</span><p>${escapeHtml(turn.response)}</p></div>
      </article>
    `).join('')
    : `<div class="empty-state">还没有形成连续追问。</div>`;
}

function renderToasts() {
  els.toastStack.innerHTML = state.toasts.map((toast) => `
    <div class="toast ${toast.variant ? `toast--${toast.variant}` : ''}">
      <strong>${escapeHtml(toast.title)}</strong>
      <div>${escapeHtml(toast.body)}</div>
    </div>
  `).join('');
}

function updateStatus() {
  if (!state.shellOpen || state.shellState === 'hidden') {
    els.globalStatus.textContent = '潮汐核常驻';
  } else if (state.shellState === 'micro') {
    els.globalStatus.textContent = '协作窗微态';
  } else if (state.shellState === 'collapsed') {
    els.globalStatus.textContent = '协作窗已折叠';
  } else {
    els.globalStatus.textContent = `${modeViews[state.activeMode].label} · ${currentVariant().oneLine}`;
  }
}

function onDocumentClick(event) {
  const target = event.target;

  const modeSwitch = target.closest('[data-mode-switch]');
  if (modeSwitch) return setMode(modeSwitch.dataset.modeSwitch);

  const shellAction = target.closest('[data-shell-action]');
  if (shellAction) return handleShellAction(shellAction.dataset.shellAction);

  const sideClose = target.closest('[data-side-close]');
  if (sideClose) return handleSideClose(sideClose.dataset.sideClose);

  const taskAction = target.closest('[data-task-action]');
  if (taskAction) return handleTaskAction(taskAction.dataset.taskAction, taskAction.dataset.taskId);

  const memoryAction = target.closest('[data-memory-action]');
  if (memoryAction) return handleMemoryPatternAction(memoryAction.dataset.memoryAction, memoryAction.dataset.memoryId);

  const followupChip = target.closest('[data-followup-chip]');
  if (followupChip) return openFollowupInput(followupChip.dataset.followupChip);

  const resultAction = target.closest('[data-result-action]');
  if (resultAction) return handleResultAction(resultAction.dataset.resultAction);

  if (target.closest('.collab-window') && state.shellState !== 'half') {
    state.shellState = 'half';
    state.followupPageOpen = state.followupHistory.length >= 2;
    renderAll();
  }
}

function handleShellAction(action) {
  touchActivity();
  if (action === 'micro') {
    state.shellOpen = true;
    state.shellState = 'micro';
    state.sourceLayerOpen = false;
    state.followupPageOpen = false;
    anchorShellToCore();
    renderAll();
    return;
  }
  if (action === 'collapse') {
    if (!state.shellOpen) return;
    state.shellState = state.shellState === 'collapsed' ? 'half' : 'collapsed';
    if (state.shellState === 'collapsed') {
      state.sourceLayerOpen = false;
      state.followupPageOpen = false;
      state.followupInputOpen = false;
      anchorShellToCore();
    } else {
      state.followupPageOpen = state.followupHistory.length >= 2;
      anchorShellToCore();
    }
    renderAll();
    return;
  }
  if (action === 'close') return closeShell();
}

function handleSideClose(side) {
  if (side === 'source') state.sourceLayerOpen = false;
  if (side === 'followup') state.followupPageOpen = false;
  renderAll();
}

function handleTaskAction(action, taskId) {
  const task = findTask(taskId);
  if (!task) return;
  selectTask(task.id);
  if (action === 'stay-today') {
    task.bucket = '今天';
    task.status = 'today';
    queueToast('已留在今天', task.title, 'quiet');
    return renderAll();
  }
  if (action === 'later') {
    task.bucket = '稍后';
    task.status = 'later';
    queueToast('稍后继续', task.title, 'quiet');
    return renderAll();
  }
  if (action === 'source') {
    openShell(task.focusMode === 'review' ? 'understand' : task.focusMode, 'task', task.id);
    queueToast('打开来源', task.title, 'quiet');
    return;
  }
}

function handleMemoryPatternAction(action, memoryId) {
  const memory = findMemory(memoryId);
  if (!memory) return;
  selectMemory(memory.id);
  if (action === 'remember') {
    memory.state = '稳定偏好';
    memory.muted = false;
    queueToast('记住这次', memory.title, 'quiet');
  }
  if (action === 'mute') {
    memory.state = '暂不采用';
    memory.muted = true;
    queueToast('暂不采用', memory.title, 'quiet');
  }
  if (action === 'correct') {
    memory.state = '待校正';
    memory.corrected = true;
    queueToast('这不对', memory.title, 'warn');
  }
  if (action === 'prefer') {
    memory.state = '以后先这样处理';
    queueToast('以后先这样处理', memory.title, 'quiet');
  }
  renderAll();
}

function handleResultAction(action) {
  touchActivity();
  if (state.activeMode === 'understand') {
    if (action === '留在今天') return markTaskInToday();
    if (action === '打开来源') return openSourceForCurrentMode();
    if (action === 'toggle-more') return toggleModeDetails('understand');
  }
  if (state.activeMode === 'rewrite') {
    if (action === '记住这次') return rememberThisRewrite();
    if (action === '套用这版') return applyThisRewrite();
    if (action === '再正式一点') return openFollowupInput('再正式一点');
    if (action === 'toggle-more') return toggleModeDetails('rewrite');
  }
  if (state.activeMode === 'advance') {
    if (action === '稍后继续' || action === '先放这儿') return collapseShellToMicro();
    if (action === '打开来源') return openSourceForCurrentMode();
    if (action === 'toggle-more') return toggleModeDetails('advance');
  }
  if (state.activeMode === 'review') {
    if (action === '这不对') return markMemoryCorrection();
    if (action === '暂不采用') return muteMemoryPattern();
    if (action === '以后先这样处理') return preferThisReview();
    if (action === 'toggle-more') return toggleModeDetails('review');
  }
}

function setMode(mode) {
  if (!modeViews[mode]) return;
  state.activeMode = mode;
  state.shellOpen = true;
  if (state.shellState === 'hidden' || state.shellState === 'micro') state.shellState = 'half';
  state.sourceLayerOpen = false;
  anchorShellToCore();
  touchActivity();
  renderAll();
}

function openFollowupInput(seed = '') {
  state.followupInputOpen = true;
  state.shellOpen = true;
  if (state.shellState !== 'half') state.shellState = 'half';
  els.followupInput.value = seed || els.followupInput.value || '';
  renderAll();
  els.followupInput.focus();
}

function toggleFollowupInput(force = !state.followupInputOpen, seed = '') {
  if (!force) {
    state.followupInputOpen = false;
    renderAll();
    return;
  }
  openFollowupInput(seed);
}

function submitFollowupFromInput() {
  const input = els.followupInput.value.trim();
  if (!input) return;
  const response = applyFollowupResponse(state.activeMode, input);
  state.followupHistory.push({ mode: state.activeMode, input, response, time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) });
  state.followupPageOpen = state.shellState === 'half' && state.followupHistory.length >= 2;
  state.followupInput.value = '';
  state.followupInputOpen = false;
  touchActivity();
  queueToast('已续写', response, 'quiet');
  renderAll();
}

function applyFollowupResponse(mode, input) {
  const index = chooseVariantIndex(mode, input);
  state.modeVariantIndex[mode] = index;
  return modeViews[mode].variants[index].oneLine;
}

function chooseVariantIndex(mode, input) {
  const text = input.toLowerCase();
  const current = state.modeVariantIndex[mode];
  if (mode === 'understand') {
    if (text.includes('白话')) return 1;
    if (text.includes('为什么')) return 2;
    return (current + 1) % 3;
  }
  if (mode === 'rewrite') {
    if (text.includes('日报')) return 0;
    if (text.includes('正式')) return 1;
    if (text.includes('短')) return 2;
    return (current + 1) % 3;
  }
  if (mode === 'advance') {
    if (text.includes('稍后') || text.includes('先放')) return 2;
    if (text.includes('待办') || text.includes('动作')) return 1;
    return (current + 1) % 3;
  }
  if (mode === 'review') {
    if (text.includes('不对')) return 2;
    if (text.includes('为什么')) return 1;
    return (current + 1) % 3;
  }
  return 0;
}

function markTaskInToday() {
  const task = currentTask();
  task.bucket = '今天';
  task.status = 'today';
  state.selectedTaskId = task.id;
  queueToast('已留在今天', task.title, 'quiet');
  renderAll();
}

function rememberThisRewrite() {
  const memory = currentMemory();
  memory.state = '稳定偏好';
  memory.muted = false;
  state.selectedMemoryId = memory.id;
  queueToast('记住这次', memory.title, 'quiet');
  renderAll();
}

function applyThisRewrite() {
  const task = currentTask();
  task.summary = `${task.summary}（已套用这次写法）`;
  queueToast('套用这版', task.title, 'quiet');
  renderAll();
}

function openSourceForCurrentMode() {
  openSourceLayer(true);
  if (state.activeMode === 'rewrite') {
    queueToast('打开来源', currentMemory().title, 'quiet');
    return;
  }
  queueToast('打开来源', currentTask().title, 'quiet');
}

function markMemoryCorrection() {
  const memory = currentMemory();
  memory.state = '待校正';
  memory.corrected = true;
  queueToast('这不对', memory.title, 'warn');
  renderAll();
}

function muteMemoryPattern() {
  const memory = currentMemory();
  memory.state = '暂不采用';
  memory.muted = true;
  queueToast('暂不采用', memory.title, 'quiet');
  renderAll();
}

function preferThisReview() {
  const memory = currentMemory();
  memory.state = '以后先这样处理';
  memory.muted = false;
  queueToast('以后先这样处理', memory.title, 'quiet');
  renderAll();
}

function selectTask(taskId) {
  const task = findTask(taskId);
  if (task) state.selectedTaskId = task.id;
}

function selectMemory(memoryId) {
  const memory = findMemory(memoryId);
  if (memory) state.selectedMemoryId = memory.id;
}

function currentTask() {
  return findTask(state.selectedTaskId) || tasks[0];
}

function currentMemory() {
  return findMemory(state.selectedMemoryId) || memories[0];
}

// 左滑打开来源层
function openSourceLayer(force = true) {
  state.shellOpen = true;
  if (state.shellState === 'hidden') state.shellState = 'half';
  state.sourceLayerOpen = force;
  state.followupPageOpen = false; // 来源层和追问页互斥
  anchorShellToCore();
  renderAll();
}

// 关闭来源层
function closeSourceLayer() {
  state.sourceLayerOpen = false;
  anchorShellToCore();
  renderAll();
}

function openShell(mode = state.activeMode, source = 'core', sourceId = '') {
  state.shellOpen = true;
  state.shellState = 'half';
  state.activeMode = mode;
  state.sourceFocus = source;
  state.sourceTitle = resolveShellSourceTitle(source, sourceId);
  state.sourceId = sourceId;
  if (sourceId) {
    if (source === 'task' || source === 'core') state.selectedTaskId = sourceId;
    if (source === 'memory') state.selectedMemoryId = sourceId;
  }
  if (source === 'task') state.sourceLayerOpen = true;
  if (source === 'memory') state.sourceLayerOpen = true;
  state.followupPageOpen = state.shellState === 'half' && state.followupHistory.length >= 2;
  anchorShellToCore();
  resetIdleTimer();
  renderAll();
}

function closeShell() {
  state.shellOpen = false;
  state.shellState = 'hidden';
  state.sourceLayerOpen = false;
  state.followupPageOpen = false;
  state.followupInputOpen = false;
  state.modeDetailOpen = { understand: false, rewrite: false, advance: false, review: false };
  state.shellOffset = null;
  state.shellDrag = null;
  state.sourceTitle = '等待承接';
  state.sourceFocus = 'desk';
  state.sourceId = '';
  clearTimeout(state.idleTimer);
  renderAll();
}

function collapseShellToMicro() {
  if (!state.shellOpen) return;
  state.shellState = 'micro';
  state.sourceLayerOpen = false;
  state.followupPageOpen = false;
  state.followupInputOpen = false;
  anchorShellToCore();
  renderAll();
}

function applyShellStateAfterOpen() {
  if (state.shellState === 'hidden') state.shellState = 'half';
}

function openShellFromSource(sourceId, slot) {
  const source = sourceForId(sourceId);
  if (slot === 'understand') {
    openShell('understand', 'desk', sourceId);
    return;
  }
  if (slot === 'advance') {
    openShell('advance', 'desk', sourceId);
    return;
  }
  if (slot === 'queue') {
    openShell('advance', 'desk', sourceId);
    const task = currentTask();
    task.bucket = '今天';
    task.summary = source.text;
    queueToast('对象已入列', task.title, 'quiet');
  }
}

function onDesktopPointerDown(event) {
  const source = event.target.closest('[data-source-id]');
  if (!source || event.button !== 0) return;
  startSourceDrag(source, event);
}

function onCoreDoubleClick(event) {
  event.preventDefault();
  openShell('understand', 'core', currentTask().id);
}

function onCorePointerDown(event) {
  if (event.button !== 0) return;
  clearTimeout(state.ringTimer);
  clearTimeout(state.longPressTimer);
  state.longPressTimer = setTimeout(() => {
    if (!state.drag && !state.shellDrag) {
      state.coreRingVisible = true;
      renderCore();
      queueToast('双击展开协作窗', '潮汐核保持收敛', 'quiet');
    }
  }, 800);
}

function onPointerMove(event) {
  if (state.drag) updateSourceDrag(event);
  if (state.shellDrag) updateShellDrag(event);
  if (!state.drag && !state.shellDrag) updateCoreProximity(event.clientX, event.clientY);
}

function onPointerUp(event) {
  if (state.drag) finishSourceDrag(event);
  if (state.shellDrag) finishShellDrag(event);
  clearTimeout(state.longPressTimer);
}

function onShellPointerDown(event) {
  if (event.button !== 0) return;
  state.shellDrag = { startX: event.clientX, startY: event.clientY, offsetLeft: state.shellOffset?.left ?? 0, offsetTop: state.shellOffset?.top ?? 0 };
}

function onShellHeaderDoubleClick(event) {
  if (event.target.closest('button')) return;
  if (!state.shellOpen || state.shellState === 'hidden') return;
  state.shellState = state.shellState === 'collapsed' ? 'half' : 'collapsed';
  if (state.shellState === 'collapsed') {
    state.sourceLayerOpen = false;
    state.followupPageOpen = false;
    state.followupInputOpen = false;
  } else {
    state.followupPageOpen = state.followupHistory.length >= 2;
  }
  anchorShellToCore();
  renderAll();
}

function onKeyDown(event) {
  if (event.key !== 'Escape') return;
  if (state.followupInputOpen) return toggleFollowupInput(false);
  if (state.followupPageOpen) { state.followupPageOpen = false; return renderAll(); }
  if (state.sourceLayerOpen) { return closeSourceLayer(); }
  if (state.shellState !== 'hidden') closeShell();
}

function startSourceDrag(sourceEl, event) {
  const source = sourceForId(sourceEl.dataset.sourceId);
  if (!source) return;
  const ghost = createDragGhost(source);
  state.drag = { source, pointerId: event.pointerId, sourceEl, ghost, lastX: event.clientX, lastY: event.clientY, overCore: false };
  sourceEl.setPointerCapture(event.pointerId);
  sourceEl.dataset.dragging = '1';
  updateDragGhost(event.clientX, event.clientY);
}

function updateSourceDrag(event) {
  state.drag.lastX = event.clientX;
  state.drag.lastY = event.clientY;
  updateDragGhost(event.clientX, event.clientY);
  const coreRect = els.tidalCore.getBoundingClientRect();
  const center = { x: coreRect.left + coreRect.width / 2, y: coreRect.top + coreRect.height / 2 };
  const dist = Math.hypot(event.clientX - center.x, event.clientY - center.y);
  const inside = dist < 150;
  if (inside !== state.drag.overCore) {
    state.drag.overCore = inside;
    state.coreSplitVisible = inside;
    state.coreRingVisible = !inside;
    renderCore();
  }
  if (inside) {
    state.coreHotSlot = determineHotSlot(event.clientX, event.clientY, coreRect);
    renderCore();
  } else if (state.coreHotSlot) {
    state.coreHotSlot = null;
    renderCore();
  }
}

function finishSourceDrag() {
  const drag = state.drag;
  if (!drag) return;
  drag.sourceEl.dataset.dragging = '0';
  try { drag.sourceEl.releasePointerCapture(drag.pointerId); } catch { /* noop */ }
  drag.ghost.remove();
  const coreRect = els.tidalCore.getBoundingClientRect();
  const center = { x: coreRect.left + coreRect.width / 2, y: coreRect.top + coreRect.height / 2 };
  const dist = Math.hypot(drag.lastX - center.x, drag.lastY - center.y);
  const slot = dist < 150 ? determineHotSlot(drag.lastX, drag.lastY, coreRect) : null;
  state.drag = null;
  state.coreSplitVisible = false;
  state.coreHotSlot = null;
  state.coreRingVisible = false;
  renderCore();
  if (slot) openShellFromSource(drag.source.id, slot);
}

function updateCoreProximity(x, y) {
  const rect = els.tidalCore.getBoundingClientRect();
  const center = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
  const dist = Math.hypot(x - center.x, y - center.y);
  const inside = dist < 150;
  if (inside) {
    if (!state.coreRingVisible && !state.ringTimer) {
      state.ringTimer = setTimeout(() => { state.coreRingVisible = true; state.ringTimer = null; renderCore(); }, 300);
    }
  } else {
    clearTimeout(state.ringTimer);
    state.ringTimer = null;
    state.coreRingVisible = false;
    renderCore();
  }
}

function updateShellDrag(event) {
  const drag = state.shellDrag;
  const dx = event.clientX - drag.startX;
  const dy = event.clientY - drag.startY;
  const left = clamp(drag.offsetLeft + dx, 8, window.innerWidth - 120);
  const top = clamp(drag.offsetTop + dy, 60, window.innerHeight - 60);
  state.shellOffset = { left, top };
  renderShell();
}

function finishShellDrag(event) {
  const drag = state.shellDrag;
  if (!drag) return;
  const x = event.clientX;
  const y = event.clientY;
  const w = window.innerWidth;
  const h = window.innerHeight;
  if (y > h * 0.75) state.shellDock = 'bottom';
  else if (x < w * 0.35) state.shellDock = 'left';
  else if (x > w * 0.65) state.shellDock = 'right';
  state.shellOffset = null;
  state.shellDrag = null;
  renderShell();
}

function onWindowResize() {
  if (!state.shellOpen || state.shellState === 'hidden') return;
  if (!state.shellOffset || state.shellDrag) return;
  const rect = els.collabWindow.getBoundingClientRect();
  state.shellOffset = {
    left: clamp(state.shellOffset.left, 16, Math.max(16, window.innerWidth - rect.width - 16)),
    top: clamp(state.shellOffset.top, 66, Math.max(66, window.innerHeight - rect.height - 24)),
  };
  renderShell();
}

function resetIdleTimer() {
  clearTimeout(state.idleTimer);
  if (!state.shellOpen || state.shellState !== 'half') return;
  state.idleTimer = setTimeout(() => {
    state.shellState = 'micro';
    state.sourceLayerOpen = false;
    state.followupPageOpen = state.followupHistory.length >= 2;
    renderAll();
  }, 12000);
}

function anchorShellToCore() {
  if (!els.tidalCore) return;
  const coreRect = els.tidalCore.getBoundingClientRect();
  const shellWidth = state.shellState === 'micro' ? 340 : state.shellState === 'collapsed' ? 280 : 500;
  const shellHeight = state.shellState === 'micro' ? 122 : state.shellState === 'collapsed' ? 92 : 460;
  const gap = 18;
  let left = coreRect.right + gap;
  let dock = 'right';
  if (coreRect.right + gap + shellWidth > window.innerWidth - 16) {
    dock = 'left';
    left = coreRect.left - gap - shellWidth;
  }
  left = clamp(left, 16, Math.max(16, window.innerWidth - shellWidth - 16));
  const top = clamp(coreRect.top + coreRect.height / 2 - shellHeight / 2, 66, Math.max(66, window.innerHeight - shellHeight - 24));
  state.shellOffset = { left, top };
  state.shellDock = dock;
}

function resolveShellSourceTitle(source, sourceId) {
  if (source === 'task' && sourceId) return titleForTask(sourceId);
  if (source === 'memory' && sourceId) return titleForMemory(sourceId);
  if (source === 'desk' && sourceId) return titleForSource(sourceId);
  if (source === 'core' && sourceId) return titleForTask(sourceId);
  return currentTask().title;
}

function toggleModeDetails(mode, force) {
  if (!state.modeDetailOpen[mode] && typeof force !== 'boolean') {
    state.modeDetailOpen[mode] = true;
  } else if (typeof force === 'boolean') {
    state.modeDetailOpen[mode] = force;
  } else {
    state.modeDetailOpen[mode] = !state.modeDetailOpen[mode];
  }
  renderAll();
}

function touchActivity() {
  if (!state.shellOpen) return;
  if (state.shellState === 'micro') state.shellState = 'half';
  resetIdleTimer();
}

function determineHotSlot(x, y, rect) {
  const slots = {
    understand: { x: rect.left + 38, y: rect.top + 84 },
    advance: { x: rect.right - 38, y: rect.top + 86 },
    queue: { x: rect.left + rect.width / 2, y: rect.bottom - 26 },
  };
  let winner = 'understand';
  let best = Infinity;
  for (const [key, point] of Object.entries(slots)) {
    const dist = Math.hypot(x - point.x, y - point.y);
    if (dist < best) { best = dist; winner = key; }
  }
  return winner;
}

function createDragGhost(source) {
  const ghost = document.createElement('div');
  ghost.className = 'drag-ghost';
  ghost.innerHTML = `<div class="drag-ghost__title">${escapeHtml(source.title)}</div><p class="drag-ghost__text">${escapeHtml(source.text)}</p>`;
  document.body.appendChild(ghost);
  return ghost;
}

function updateDragGhost(x, y) {
  if (!state.drag) return;
  state.drag.ghost.style.left = `${x}px`;
  state.drag.ghost.style.top = `${y}px`;
}

function queueToast(title, body, variant = '') {
  state.toasts.unshift({ id: `${Date.now()}-${Math.random()}`, title, body, variant });
  state.toasts = state.toasts.slice(0, 4);
  renderToasts();
  const id = state.toasts[0]?.id;
  setTimeout(() => {
    state.toasts = state.toasts.filter((toast) => toast.id !== id);
    renderToasts();
  }, 2400);
}

function listMarkup(items) {
  return `<ul class="bullet-list">${items.map((item) => `<li>${escapeHtml(item)}</li>`).join('')}</ul>`;
}

function chipListMarkup(items) {
  return `<div class="chip-cloud">${items.map((item) => `<span class="badge badge--accent">${escapeHtml(item)}</span>`).join('')}</div>`;
}

function resultActionButton(label, action, soft = false) {
  return `<button class="mini-action${soft ? ' mini-action--soft' : ''}" type="button" data-result-action="${escapeHtml(action)}">${escapeHtml(label)}</button>`;
}

function taskChipList(ids) {
  return `<div class="chip-cloud">${ids.map((id) => `<button class="badge badge--warm badge--button" type="button" data-task-action="source" data-task-id="${id}">${escapeHtml(taskTitle(id))}</button>`).join('')}</div>`;
}

function memoryChipList(ids) {
  return `<div class="chip-cloud">${ids.map((id) => `<button class="badge badge--button" type="button" data-memory-action="remember" data-memory-id="${id}">${escapeHtml((findMemory(id) || {}).title || id)}</button>`).join('')}</div>`;
}

function resultCard(title, body, actions = []) {
  return `<article class="result-card"><div class="result-card__title">${escapeHtml(title)}</div><div class="result-card__body">${body}</div>${actions.length ? `<div class="result-card__actions">${actions.join('')}</div>` : ''}</article>`;
}

function handleTaskMemoryActionFallback(action, id) {
  if (action === 'remember' || action === 'mute' || action === 'correct' || action === 'prefer') {
    handleMemoryPatternAction(action, id);
  }
}

function taskTitle(id) {
  return (findTask(id) || {}).title || id;
}

function titleForTask(id) { return taskTitle(id); }
function titleForMemory(id) { return (findMemory(id) || {}).title || id; }
function sourceForId(id) { return desktopCards.find((card) => card.id === id) || desktopCards[0]; }
function titleForSource(id) { return sourceForId(id).title; }
function modeForSource(sourceId) { return modeForSourceType(sourceForId(sourceId).sourceType); }
function modeForSourceType(sourceType) { return sourceType === 'file' ? 'rewrite' : sourceType === 'task' ? 'advance' : sourceType === 'screenshot' ? 'understand' : 'understand'; }

function promptLabelForMode(mode) {
  return mode === 'understand' ? '继续问这份结果' : mode === 'rewrite' ? '改一种写法' : mode === 'advance' ? '让我继续往下做' : '问它为什么这么判断';
}

function handleDocumentSourceOpen(sourceId) {
  openShell(modeForSource(sourceId), 'desk', sourceId);
}

function findTask(taskId) {
  return tasks.find((task) => task.id === taskId) || null;
}

function findMemory(memoryId) {
  return memories.find((memory) => memory.id === memoryId) || null;
}

function stateToString() { return state.shellState; }

function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }

function escapeHtml(value) {
  return String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&#39;');
}

window.addEventListener('DOMContentLoaded', bootstrap);
