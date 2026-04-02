export function escapeHtml(value = '') {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

export function groupBy(items, keyGetter) {
  return items.reduce((acc, item) => {
    const key = keyGetter(item);
    if (!acc[key]) acc[key] = [];
    acc[key].push(item);
    return acc;
  }, {});
}

export function runtimeCall(method, ...args) {
  const runtime = window.runtime;
  if (runtime && typeof runtime[method] === 'function') {
    return runtime[method](...args);
  }
  if (method === 'Hide' || method === 'Quit' || method === 'Show') {
    window.close();
  }
  return undefined;
}
