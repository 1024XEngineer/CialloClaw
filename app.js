(function () {
  const logic = window.CialloLogic;
  const isDesktopMode = /(?:^|[?&])shell=desktop(?:&|$)/.test(
    (window.location && window.location.search) || ''
  );

  if (!logic) {
    return;
  }

  const DEMO_TODAY = '2026-03-28';
  const demoTasks = [
    {
      id: 'task-status-update',
      title: 'Finish weekly status update',
      dueAt: DEMO_TODAY,
      status: 'pending',
      priority: 'high',
      suggestedAction: 'Generate a progress update draft'
    },
    {
      id: 'task-candidate-notes',
      title: 'Review candidate notes',
      dueAt: '2026-03-29',
      status: 'pending',
      priority: 'high',
      suggestedAction: 'Summarize follow-up items'
    },
    {
      id: 'task-archive-notes',
      title: 'Archive closed ticket notes',
      dueAt: '2026-03-27',
      status: 'done',
      priority: 'low',
      suggestedAction: 'Move notes to archive'
    }
  ];

  const state = {
    isPanelOpen: false,
    isDesktopMode: isDesktopMode,
    tasks: demoTasks.slice(),
    inspectionSummary: '',
    relevantTaskIds: [],
    inspectionHasRun: false,
    draft: '',
    awaitingDraftConfirmation: false,
    logEntries: [],
    forcedErrorAction: '',
    chatMessages: [
      {
        role: 'assistant',
        text: 'Assistant ready. Ask for a summary, tasks, or an explanation of the workflow.'
      }
    ]
  };

  let refs;

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function nl2br(value) {
    return escapeHtml(value).replace(/\n/g, '<br>');
  }

  function setAssistantMessage(message) {
    refs.assistantMessage.textContent = message;
  }

  function addLog(action, status, detail) {
    state.logEntries.push(logic.createLogEntry(action, status, detail));
    renderLog();
  }

  function maybeThrow(action) {
    if (state.forcedErrorAction === action) {
      state.forcedErrorAction = '';
      throw new Error('Forced error for testing.');
    }
  }

  function renderTasks() {
    refs.taskList.innerHTML = state.tasks.map(function (task) {
      const isRelevant = state.relevantTaskIds.indexOf(task.id) !== -1;
      const classes = ['task-card'];

      if (isRelevant) {
        classes.push('is-relevant');
      }

      return [
        '<article class="' + classes.join(' ') + '">',
        '<div class="task-card__meta">',
        '<span>' + escapeHtml(task.status) + '</span>',
        '<span>' + escapeHtml(task.priority) + ' priority</span>',
        '</div>',
        '<h3>' + escapeHtml(task.title) + '</h3>',
        '<p>Due ' + escapeHtml(task.dueAt) + '</p>',
        '</article>'
      ].join('');
    }).join('');
  }

  function renderSummary() {
    if (!state.inspectionSummary) {
      refs.inspectionSummary.innerHTML = '<p class="empty-state">Run inspection to identify follow-up tasks.</p>';
      return;
    }

    refs.inspectionSummary.innerHTML = [
      '<div class="result-block">',
      '<p class="result-label">Inspection summary</p>',
      '<p class="result-text">' + escapeHtml(state.inspectionSummary) + '</p>',
      '</div>'
    ].join('');
  }

  function renderDraft() {
    if (!state.draft) {
      refs.draftOutput.innerHTML = '<p class="empty-state">No draft generated yet.</p>';
      refs.undoLastAction.disabled = true;
      return;
    }

    refs.draftOutput.innerHTML = [
      '<div class="result-block result-block--draft">',
      '<p class="result-label">Generated draft</p>',
      '<p class="result-text">' + nl2br(state.draft) + '</p>',
      '</div>'
    ].join('');
    refs.undoLastAction.disabled = false;
  }

  function renderLog() {
    if (!state.logEntries.length) {
      refs.executionLog.innerHTML = '<p class="empty-state">No actions recorded yet.</p>';
      return;
    }

    refs.executionLog.innerHTML = state.logEntries.slice().reverse().map(function (entry) {
      return [
        '<article class="log-entry log-entry--' + escapeHtml(entry.status) + '">',
        '<div class="log-entry__row">',
        '<strong>' + escapeHtml(entry.action) + '</strong>',
        '<span>' + escapeHtml(entry.status) + '</span>',
        '</div>',
        '<p>' + escapeHtml(entry.detail) + '</p>',
        '</article>'
      ].join('');
    }).join('');
  }

  function renderChat() {
    refs.chatMessages.innerHTML = state.chatMessages.map(function (message) {
      return [
        '<article class="chat-message chat-message--' + escapeHtml(message.role) + '">',
        '<div class="chat-message__bubble">',
        '<p class="chat-message__role">' + escapeHtml(message.role) + '</p>',
        '<p class="chat-message__text">' + nl2br(message.text) + '</p>',
        '</div>',
        '</article>'
      ].join('');
    }).join('');
  }

  function renderPanel() {
    const isPanelVisible = state.isDesktopMode || state.isPanelOpen;

    refs.assistantPanel.classList.toggle('is-hidden', !isPanelVisible);
    refs.floatingBall.classList.toggle('is-hidden', state.isDesktopMode);
    refs.floatingBall.setAttribute('aria-expanded', String(isPanelVisible));
    refs.floatingBall.textContent = state.isPanelOpen ? 'Hide assistant' : 'Open assistant';
  }

  function render() {
    renderTasks();
    renderSummary();
    renderDraft();
    renderLog();
    renderChat();
    renderPanel();
  }

  function addChatMessage(role, text) {
    state.chatMessages.push({ role: role, text: text });
  }

  function getPendingTasks() {
    return state.tasks.filter(function (task) {
      return task.status === 'pending';
    });
  }

  function buildChatReply(prompt) {
    const normalizedPrompt = prompt.toLowerCase();

    if (normalizedPrompt.indexOf('summarize') !== -1) {
      return state.inspectionSummary || 'No inspection summary yet. Run inspection and I will summarize the current state.';
    }

    if (normalizedPrompt.indexOf('todo') !== -1 || normalizedPrompt.indexOf('task') !== -1) {
      const pendingTasks = getPendingTasks();

      if (!pendingTasks.length) {
        return 'There are no pending tasks right now.';
      }

      return 'Pending tasks:\n' + pendingTasks.map(function (task, index) {
        return (index + 1) + '. ' + task.title;
      }).join('\n');
    }

    if (normalizedPrompt.indexOf('explain') !== -1) {
      return 'The workflow stays local: run inspection to identify follow-up tasks, generate a draft with confirmation, review the execution log, and undo the draft without clearing the summary.';
    }

    return 'I can help with local summaries, pending tasks, and workflow explanations.';
  }

  function submitChatPrompt(prompt) {
    const trimmedPrompt = prompt.trim();

    if (!trimmedPrompt) {
      return;
    }

    addChatMessage('user', trimmedPrompt);
    addChatMessage('assistant', buildChatReply(trimmedPrompt));
    refs.chatInput.value = '';
    renderChat();
  }

  function withActionGuard(action, handler) {
    return function () {
      try {
        handler();
      } catch (error) {
        setAssistantMessage('Something went wrong. The current summary and log are still available.');
        addLog(action, 'error', 'Unexpected error: ' + error.message);
      }
    };
  }

  function runInspection() {
    maybeThrow('inspection');

    const result = logic.inspectTasks(state.tasks, DEMO_TODAY);

    state.inspectionHasRun = true;
    state.awaitingDraftConfirmation = false;
    state.inspectionSummary = result.summary;
    state.relevantTaskIds = result.relevantTasks.map(function (task) {
      return task.id;
    });

    setAssistantMessage(
      result.relevantTasks.length
        ? 'Inspection complete. Relevant tasks are highlighted and ready for draft confirmation.'
        : 'Inspection complete. Nothing needs follow-up right now.'
    );
    addLog(
      'inspection',
      result.relevantTasks.length ? 'success' : 'info',
      result.summary
    );
    render();
  }

  function generateDraft() {
    maybeThrow('generate-draft');

    if (!state.inspectionHasRun) {
      setAssistantMessage('Run inspection first so I can confirm what should go into the draft.');
      addLog('generate-draft', 'blocked', 'Draft generation was blocked until inspection runs.');
      render();
      return;
    }

    if (!state.awaitingDraftConfirmation) {
      state.awaitingDraftConfirmation = true;
      setAssistantMessage('Ready to generate the draft. Click Generate Draft again to confirm.');
      addLog('generate-draft', 'info', 'Draft generation is waiting for confirmation.');
      render();
      return;
    }

    const relevantTasks = state.tasks.filter(function (task) {
      return state.relevantTaskIds.indexOf(task.id) !== -1;
    });

    state.draft = logic.buildDraft(state.inspectionSummary, relevantTasks);
    state.awaitingDraftConfirmation = false;
    setAssistantMessage('Draft generated. You can undo the draft without clearing the inspection summary.');
    addLog('generate-draft', 'success', 'Draft created from the latest inspection summary.');
    render();
  }

  function undoLastAction() {
    maybeThrow('undo');

    if (!state.draft) {
      setAssistantMessage('There is no generated draft to undo yet.');
      addLog('undo', 'blocked', 'Undo was blocked because no generated draft exists.');
      render();
      return;
    }

    state.draft = '';
    state.awaitingDraftConfirmation = false;
    setAssistantMessage('Removed the generated draft. The inspection summary stays visible.');
    addLog('undo', 'success', 'Removed the generated draft block.');
    render();
  }

  function viewLog() {
    maybeThrow('view-log');

    refs.executionLog.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setAssistantMessage('Scrolled to the execution log.');
    addLog('view-log', 'info', 'Scrolled to the execution log section.');
    render();
  }

  function toggleAssistantPanel() {
    maybeThrow('toggle-panel');

    state.isPanelOpen = !state.isPanelOpen;
    renderPanel();
  }

  function sendChatMessage() {
    submitChatPrompt(refs.chatInput.value || '');
  }

  function runQuickAction(prompt) {
    return function () {
      submitChatPrompt(prompt);
    };
  }

  function bindEvents() {
    if (!state.isDesktopMode) {
      refs.floatingBall.addEventListener('click', withActionGuard('toggle-panel', toggleAssistantPanel));
    }

    refs.runInspection.addEventListener('click', withActionGuard('inspection', runInspection));
    refs.generateDraft.addEventListener('click', withActionGuard('generate-draft', generateDraft));
    refs.undoLastAction.addEventListener('click', withActionGuard('undo', undoLastAction));
    refs.viewLog.addEventListener('click', withActionGuard('view-log', viewLog));
    refs.chatSend.addEventListener('click', withActionGuard('chat-send', sendChatMessage));
    refs.chatActionSummarize.addEventListener('click', withActionGuard('chat-action-summarize', runQuickAction('Summarize the current work')));
    refs.chatActionExplain.addEventListener('click', withActionGuard('chat-action-explain', runQuickAction('Explain the current workflow')));
    refs.chatActionExtractTodos.addEventListener('click', withActionGuard('chat-action-extract-todos', runQuickAction('Extract todos')));
  }

  function collectRefs() {
    refs = {
      floatingBall: document.getElementById('floating-ball'),
      assistantPanel: document.getElementById('assistant-panel'),
      assistantMessage: document.getElementById('assistant-message'),
      taskList: document.getElementById('task-list'),
      inspectionSummary: document.getElementById('inspection-summary'),
      draftOutput: document.getElementById('draft-output'),
      executionLog: document.getElementById('execution-log'),
      runInspection: document.getElementById('run-inspection'),
      generateDraft: document.getElementById('generate-draft'),
      undoLastAction: document.getElementById('undo-last-action'),
      viewLog: document.getElementById('view-log'),
      chatMessages: document.getElementById('chat-messages'),
      chatActionSummarize: document.getElementById('chat-action-summarize'),
      chatActionExplain: document.getElementById('chat-action-explain'),
      chatActionExtractTodos: document.getElementById('chat-action-extract-todos'),
      chatInput: document.getElementById('chat-input'),
      chatSend: document.getElementById('chat-send')
    };
  }

  function init() {
    collectRefs();

    if (!refs.floatingBall || !refs.assistantPanel) {
      return;
    }

    if (state.isDesktopMode && document.body && document.body.classList) {
      document.body.classList.add('desktop-shell');
    }

    setAssistantMessage('Ready to inspect tasks and prepare a visible draft.');
    render();
    bindEvents();
  }

  window.CialloApp = {
    init: init,
    forceErrorForTesting: function (action) {
      state.forcedErrorAction = action;
    },
    getState: function () {
      return {
        inspectionSummary: state.inspectionSummary,
        relevantTaskIds: state.relevantTaskIds.slice(),
        draft: state.draft,
        inspectionHasRun: state.inspectionHasRun,
        awaitingDraftConfirmation: state.awaitingDraftConfirmation,
        logEntries: state.logEntries.slice()
      };
    }
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
