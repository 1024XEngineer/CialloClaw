const test = require('node:test');
const assert = require('node:assert/strict');

const { registerDesktopBridge } = require('../preload.js');

test('registerDesktopBridge exposes toggleMainPanel through CialloDesktop', async () => {
  const calls = [];
  let exposedName;
  let exposedApi;

  registerDesktopBridge(
    {
      exposeInMainWorld(name, api) {
        exposedName = name;
        exposedApi = api;
      }
    },
    {
      invoke(channelName) {
        calls.push(channelName);
        return Promise.resolve('ok');
      }
    }
  );

  assert.equal(exposedName, 'CialloDesktop');
  assert.equal(typeof exposedApi.toggleMainPanel, 'function');
  await assert.doesNotReject(exposedApi.toggleMainPanel());
  assert.deepEqual(calls, ['desktop:toggle-main-panel']);
});
