(function (root, factory) {
  const api = factory();

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  } else {
    root.CialloLogic = api;

    if (typeof globalThis !== 'undefined') {
      globalThis.CialloLogic = api;
    }

    if (typeof window !== 'undefined') {
      window.CialloLogic = api;
    }
  }
})(typeof globalThis !== 'undefined' ? globalThis : window, function () {
  let logEntryCount = 0;

  function inspectTasks(tasks, today) {
    const relevantTasks = tasks.filter((task) => {
      return task.status === 'pending' && (task.dueAt === today || task.priority === 'high');
    });

    const summary = relevantTasks.length
      ? `${relevantTasks.length} tasks need follow-up today.`
      : 'No tasks need follow-up right now.';

    return { relevantTasks, summary };
  }

  function buildDraft(summary, relevantTasks) {
    if (!relevantTasks.length) {
      return `${summary}\n\nNo action is needed right now.`;
    }

    const taskLines = relevantTasks.map((task, index) => {
      return `${index + 1}. ${task.title} - ${task.suggestedAction}`;
    });

    return [
      'Draft Ready',
      '',
      summary,
      '',
      'Recommended actions:',
      ...taskLines
    ].join('\n');
  }

  function createLogEntry(action, status, detail, timestamp) {
    const resolvedTimestamp = timestamp || new Date().toISOString();
    logEntryCount += 1;

    return {
      id: `${action}-${resolvedTimestamp}-${logEntryCount}`,
      action,
      status,
      detail,
      timestamp: resolvedTimestamp
    };
  }

  return {
    inspectTasks,
    buildDraft,
    createLogEntry
  };
});
