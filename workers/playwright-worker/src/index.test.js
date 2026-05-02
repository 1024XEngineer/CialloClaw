import assert from "node:assert/strict";
import test from "node:test";

import { handleRequest, healthResponse, normalizeText } from "./index.js";

function createResponse(overrides = {}) {
  return {
    headers: () => ({ "content-type": overrides.contentType ?? "text/html; charset=utf-8" }),
    ok: () => overrides.ok ?? true,
    status: () => overrides.status ?? 200,
  };
}

function createPage(overrides = {}) {
  const actionLog = overrides.actionLog ?? [];
  const navigationLog = overrides.navigationLog ?? [];
  const page = {
    currentURL: overrides.currentURL ?? "https://example.com/final",
    async content() {
      return overrides.html ?? "<html><head><title>Demo Page</title></head><body>Hello world. Search target. Another target.</body></html>";
    },
    async bringToFront() {
      actionLog.push({ action: "bringToFront" });
    },
    async evaluate() {
      return overrides.snapshot ?? {
        headings: ["Heading A"],
        links: ["Docs"],
        buttons: ["Submit"],
        inputs: ["email"],
      };
    },
    goto: async (url) => {
      navigationLog.push({ action: "goto", url });
      page.currentURL = overrides.gotoURL ?? url;
      return Object.prototype.hasOwnProperty.call(overrides, "response")
        ? overrides.response
        : createResponse();
    },
    locator: (selector) => ({
      async innerText() {
        return overrides.bodyText ?? "Hello world. Search target. Another target.";
      },
      first() {
        return {
          async check(options) {
            actionLog.push({ action: "check", options, selector });
          },
          async click(options) {
            actionLog.push({ action: "click", options, selector });
          },
          async fill(value, options) {
            actionLog.push({ action: "fill", options, selector, value });
          },
          async press(key, options) {
            actionLog.push({ action: "press", key, options, selector });
          },
          async uncheck(options) {
            actionLog.push({ action: "uncheck", options, selector });
          },
          async waitFor(options) {
            actionLog.push({ action: "waitFor", options, selector });
          },
        };
      },
    }),
    async title() {
      return overrides.title ?? "Demo Page";
    },
    url() {
      return page.currentURL;
    },
    async waitForTimeout(timeoutMS) {
      actionLog.push({ action: "waitForTimeout", timeoutMS });
    },
  };
  return page;
}

function createDeps(overrides = {}) {
  const page = overrides.page ?? createPage(overrides);
  const lifecycle = overrides.lifecycle ?? [];
  const connectedPages = overrides.connectedPages ?? [page];
  return {
    async connectToBrowser(endpointURL) {
      lifecycle.push(`connect:${endpointURL}`);
      if (overrides.connectError) {
        throw overrides.connectError;
      }
      return {
        contexts() {
          return overrides.connectedContexts ?? [{
            pages() {
              return connectedPages;
            },
          }];
        },
      };
    },
    async launchBrowser() {
      lifecycle.push("launch");
      return {
        async close() {
          lifecycle.push("browser.close");
          if (overrides.browserCloseError) {
            throw overrides.browserCloseError;
          }
        },
        async newContext() {
          lifecycle.push("newContext");
          return {
            async close() {
              lifecycle.push("context.close");
              if (overrides.contextCloseError) {
                throw overrides.contextCloseError;
              }
            },
            async newPage() {
              lifecycle.push("newPage");
              return page;
            },
          };
        },
      };
    },
  };
}

test("normalizeText removes markup noise", () => {
  assert.equal(normalizeText("<div>Hello&nbsp;<strong>world</strong></div>"), "Hello world");
});

test("health verifies browser startup and page creation", async () => {
  const lifecycle = [];
  const response = await healthResponse(createDeps({ lifecycle }));

  assert.equal(response.ok, true);
  assert.equal(response.result.status, "ok");
  assert.deepEqual(lifecycle, ["launch", "newContext", "newPage", "context.close", "browser.close"]);
});

test("health still closes browser when context cleanup fails", async () => {
  const lifecycle = [];
  await assert.rejects(
    () => healthResponse(createDeps({ lifecycle, contextCloseError: new Error("context close failed") })),
    /context close failed/,
  );

  assert.deepEqual(lifecycle, ["launch", "newContext", "newPage", "context.close", "browser.close"]);
});

test("handleRequest delegates health requests through the worker switch", async () => {
  const lifecycle = [];
  const response = await handleRequest({ action: "health" }, createDeps({ lifecycle }));

  assert.equal(response.ok, true);
  assert.equal(response.result.worker_name, "playwright_worker");
  assert.deepEqual(lifecycle, ["launch", "newContext", "newPage", "context.close", "browser.close"]);
});

test("page_read returns normalized page metadata", async () => {
  const response = await handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
    bodyText: "Hello world from browser",
    gotoURL: "https://example.com/article",
    title: "Example Article",
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.url, "https://example.com/article");
  assert.equal(response.result.title, "Example Article");
  assert.equal(response.result.text_content, "Hello world from browser");
  assert.equal(response.result.source, "playwright_worker_browser");
});

test("page_read uses the HTML title tag when Playwright title lookup is empty", async () => {
  const response = await handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
    bodyText: "Hello world from browser",
    html: "<html><head><title>Fallback Demo</title></head><body>Hello world</body></html>",
    title: "",
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.title, "Fallback Demo");
});

test("page_search returns bounded matches", async () => {
  const response = await handleRequest({ action: "page_search", url: "https://example.com", query: "target", limit: 1 }, createDeps({
    bodyText: "First target. Second target. Third miss.",
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.match_count, 2);
  assert.deepEqual(response.result.matches, ["First target"]);
});

test("page_read falls back to hostname and untitled titles when needed", async () => {
  const hostnameFallback = await handleRequest({ action: "page_read", url: "https://example.com/path" }, createDeps({
    bodyText: "No explicit title",
    html: "<html><body>No title tag</body></html>",
    title: "",
  }));
  assert.equal(hostnameFallback.ok, true);
  assert.equal(hostnameFallback.result.title, "example.com");

  const untitledFallback = await handleRequest({ action: "page_read", url: "notaurl" }, createDeps({
    bodyText: "No explicit title",
    currentURL: "notaurl",
    gotoURL: "notaurl",
    html: "<html><body>No title tag</body></html>",
    title: "",
  }));
  assert.equal(untitledFallback.ok, true);
  assert.equal(untitledFallback.result.title, "untitled page");
});

test("page_read keeps launch-path transport failures loud", async () => {
  await assert.rejects(
    () => handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({ response: null })),
    /navigation_failed/,
  );

  await assert.rejects(
    () => handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
      response: createResponse({ ok: false, status: 503 }),
    })),
    /http_503/,
  );
});

test("structured_dom returns page snapshot", async () => {
  const response = await handleRequest({ action: "structured_dom", url: "https://example.com" }, createDeps({
    snapshot: {
      headings: ["Heading A", "Heading B"],
      links: ["Docs"],
      buttons: ["Submit"],
      inputs: ["email"],
    },
  }));

  assert.equal(response.ok, true);
  assert.deepEqual(response.result.headings, ["Heading A", "Heading B"]);
  assert.deepEqual(response.result.links, ["Docs"]);
});

test("page_interact applies actions and returns updated content", async () => {
  const actionLog = [];
  const response = await handleRequest({
    action: "page_interact",
    url: "https://example.com",
    actions: [
      { type: "click", selector: "button.submit" },
      { type: "fill", selector: "input[name=email]", value: "demo@example.com" },
      { type: "wait_for", timeout_ms: 250 },
    ],
  }, createDeps({ actionLog, bodyText: "Interaction complete" }));

  assert.equal(response.ok, true);
  assert.equal(response.result.actions_applied, 3);
  assert.equal(response.result.text_content, "Interaction complete");
  assert.deepEqual(actionLog.map((entry) => entry.action), ["click", "fill", "waitForTimeout"]);
});

test("page_interact supports press, check, uncheck, and selector waits", async () => {
  const actionLog = [];
  const response = await handleRequest({
    action: "page_interact",
    url: "https://example.com/settings",
    actions: [
      { type: "press", selector: "input[name=email]", key: "Tab" },
      { type: "check", selector: "input[name=terms]" },
      { type: "uncheck", selector: "input[name=marketing]" },
      { type: "wait_for", selector: "div.ready" },
    ],
  }, createDeps({ actionLog, bodyText: "Interaction complete" }));

  assert.equal(response.ok, true);
  assert.equal(response.result.actions_applied, 4);
  assert.deepEqual(actionLog.map((entry) => entry.action), ["press", "check", "uncheck", "waitFor"]);
});

test("page_interact rejects selector actions without selectors", async () => {
  const response = await handleRequest({
    action: "page_interact",
    url: "https://example.com",
    actions: [
      { type: "click" },
    ],
  }, createDeps());

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "invalid_input");
  assert.match(response.error.message, /selector is required/);

  const waitForSelector = await handleRequest({
    action: "page_interact",
    url: "https://example.com",
    actions: [
      { type: "wait_for", selector: "" },
    ],
  }, createDeps());
  assert.equal(waitForSelector.ok, false);
  assert.equal(waitForSelector.error.code, "invalid_input");
});

test("page_read can attach to a real browser page over CDP", async () => {
  const lifecycle = [];
  const navigationLog = [];
  const response = await handleRequest({
    action: "page_read",
    url: "https://example.com/current",
    attach: {
      browser_kind: "chrome",
      target: {
        url: "https://example.com/current",
      },
    },
  }, createDeps({
    lifecycle,
    navigationLog,
    connectedPages: [createPage({
      navigationLog,
      currentURL: "https://example.com/current",
      title: "Connected Page",
      bodyText: "Attached browser content",
    })],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.browser_kind, "chrome");
  assert.equal(response.result.browser_transport, "cdp");
  assert.equal(response.result.endpoint_url, "http://127.0.0.1:9222");
  assert.equal(response.result.source, "playwright_worker_cdp");
  assert.equal(response.result.title, "Connected Page");
  assert.equal(response.result.text_content, "Attached browser content");
  assert.deepEqual(lifecycle, ["connect:http://127.0.0.1:9222"]);
  assert.deepEqual(navigationLog, []);
});

test("browser_attach_current returns the selected real browser tab", async () => {
  const response = await handleRequest({
    action: "browser_attach_current",
    attach: {
      browser_kind: "chrome",
      target: {
        title_contains: "connected page",
      },
    },
  }, createDeps({
    connectedPages: [
      createPage({ currentURL: "https://example.com/other", title: "Other Page" }),
      createPage({ currentURL: "https://example.com/current", title: "Connected Page" }),
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.page_index, 1);
  assert.equal(response.result.title, "Connected Page");
  assert.equal(response.result.url, "https://example.com/current");
});

test("browser_snapshot returns structured content for the attached tab", async () => {
  const response = await handleRequest({
    action: "browser_snapshot",
    attach: {
      browser_kind: "edge",
      target: {
        page_index: 0,
      },
    },
  }, createDeps({
    connectedPages: [createPage({
      currentURL: "https://example.com/current",
      title: "Snapshot Page",
      bodyText: "Snapshot body text",
      snapshot: {
        headings: ["Heading A"],
        links: ["Docs"],
        buttons: ["Submit"],
        inputs: ["email"],
      },
    })],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.page_index, 0);
  assert.equal(response.result.title, "Snapshot Page");
  assert.equal(response.result.text_content, "Snapshot body text");
  assert.deepEqual(response.result.headings, ["Heading A"]);
  assert.deepEqual(response.result.links, ["Docs"]);
});

test("browser_tabs_list reports attached browser tabs with stable indexes", async () => {
  const response = await handleRequest({
    action: "browser_tabs_list",
    attach: {
      browser_kind: "chrome",
    },
  }, createDeps({
    connectedPages: [
      createPage({ currentURL: "https://example.com/one", title: "One" }),
      createPage({ currentURL: "https://example.com/two", title: "Two" }),
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.tab_count, 2);
  assert.deepEqual(response.result.tabs, [
    { page_index: 0, title: "One", url: "https://example.com/one" },
    { page_index: 1, title: "Two", url: "https://example.com/two" },
  ]);
});

test("browser_tab_focus brings the selected tab to the front", async () => {
  const actionLog = [];
  const response = await handleRequest({
    action: "browser_tab_focus",
    attach: {
      browser_kind: "edge",
      target: {
        page_index: 1,
      },
    },
  }, createDeps({
    actionLog,
    connectedPages: [
      createPage({ actionLog, currentURL: "https://example.com/one", title: "One" }),
      createPage({ actionLog, currentURL: "https://example.com/two", title: "Two" }),
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.page_index, 1);
  assert.equal(response.result.title, "Two");
  assert.deepEqual(actionLog.map((entry) => entry.action), ["bringToFront"]);
});

test("browser_navigate drives the attached tab to a new url", async () => {
  const lifecycle = [];
  const navigationLog = [];
  const response = await handleRequest({
    action: "browser_navigate",
    url: "https://example.com/next",
    attach: {
      browser_kind: "chrome",
      target: {
        page_index: 0,
      },
    },
  }, createDeps({
    lifecycle,
    navigationLog,
    connectedPages: [createPage({
      navigationLog,
      currentURL: "https://example.com/current",
      gotoURL: "https://example.com/next",
      title: "Next Page",
      bodyText: "Navigation complete",
    })],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.page_index, 0);
  assert.equal(response.result.url, "https://example.com/next");
  assert.equal(response.result.title, "Next Page");
  assert.equal(response.result.text_content, "Navigation complete");
  assert.deepEqual(lifecycle, ["connect:http://127.0.0.1:9222"]);
  assert.deepEqual(navigationLog, [{ action: "goto", url: "https://example.com/next" }]);
});

test("browser_interact keeps real-browser actions on the attached tab", async () => {
  const actionLog = [];
  const navigationLog = [];
  const response = await handleRequest({
    action: "browser_interact",
    attach: {
      browser_kind: "edge",
      target: {
        page_index: 0,
      },
    },
    actions: [
      { type: "click", selector: "button.submit" },
      { type: "fill", selector: "input[name=email]", value: "demo@example.com" },
    ],
  }, createDeps({
    actionLog,
    navigationLog,
    connectedPages: [createPage({
      actionLog,
      navigationLog,
      currentURL: "https://example.com/form",
      title: "Connected Form",
      bodyText: "Interaction complete",
    })],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.actions_applied, 2);
  assert.equal(response.result.source, "playwright_worker_cdp");
  assert.deepEqual(actionLog.map((entry) => entry.action), ["click", "fill"]);
  assert.deepEqual(navigationLog, []);
});

test("page_read ignores top-level request url unless attach.target.url is explicit", async () => {
  const singlePage = await handleRequest({
    action: "page_read",
    url: "https://example.com/launch-shape",
    attach: {
      browser_kind: "chrome",
    },
  }, createDeps({
    connectedPages: [createPage({
      currentURL: "https://example.com/current-tab",
      title: "Current Tab",
      bodyText: "Attached browser content",
    })],
  }));

  assert.equal(singlePage.ok, true);
  assert.equal(singlePage.result.title, "Current Tab");
  assert.equal(singlePage.result.url, "https://example.com/current-tab");

  const ambiguousWithoutExplicitTarget = await handleRequest({
    action: "page_read",
    url: "https://example.com/current-tab",
    attach: {
      browser_kind: "chrome",
    },
  }, createDeps({
    connectedPages: [
      createPage({ currentURL: "https://example.com/current-tab", title: "Matching URL" }),
      createPage({ currentURL: "https://example.com/other-tab", title: "Other Tab" }),
    ],
  }));

  assert.equal(ambiguousWithoutExplicitTarget.ok, false);
  assert.equal(ambiguousWithoutExplicitTarget.error.code, "page_target_not_found");
});

test("page_read resolves attached pages by url and title narrowing", async () => {
  const response = await handleRequest({
    action: "page_read",
    url: "https://example.com/docs",
    attach: {
      endpoint_url: "http://127.0.0.1:9223",
      browser_kind: "edge",
      target: {
        url: "https://example.com/docs",
        title_contains: "target docs",
      },
    },
  }, createDeps({
    connectedPages: [
      createPage({ currentURL: "https://example.com/docs", title: "Background tab", bodyText: "Ignore me" }),
      createPage({ currentURL: "https://example.com/docs", title: "Target Docs", bodyText: "Selected tab" }),
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.title, "Target Docs");
  assert.equal(response.result.text_content, "Selected tab");
  assert.equal(response.result.endpoint_url, "http://127.0.0.1:9223");
});

test("page_read reports invalid attach modes and browser kinds without throwing", async () => {
  const invalidMode = await handleRequest({
    action: "page_read",
    attach: { mode: "launch" },
  }, createDeps());
  assert.equal(invalidMode.ok, false);
  assert.equal(invalidMode.error.code, "invalid_input");

  const unsupportedBrowser = await handleRequest({
    action: "page_read",
    attach: { browser_kind: "firefox" },
  }, createDeps());
  assert.equal(unsupportedBrowser.ok, false);
  assert.equal(unsupportedBrowser.error.code, "unsupported_browser_kind");

  const invalidPageIndex = await handleRequest({
    action: "page_read",
    attach: {
      browser_kind: "chrome",
      target: { page_index: -1 },
    },
  }, createDeps());
  assert.equal(invalidPageIndex.ok, false);
  assert.equal(invalidPageIndex.error.code, "invalid_input");

  const lifecycle = [];
  const externalEndpoint = await handleRequest({
    action: "page_read",
    attach: {
      browser_kind: "chrome",
      endpoint_url: "http://example.com:9222",
    },
  }, createDeps({ lifecycle }));
  assert.equal(externalEndpoint.ok, false);
  assert.equal(externalEndpoint.error.code, "invalid_input");
  assert.match(externalEndpoint.error.message, /loopback host/);
  assert.deepEqual(lifecycle, []);

  const invalidEndpoint = await handleRequest({
    action: "page_read",
    attach: {
      browser_kind: "chrome",
      endpoint_url: "not-a-url",
    },
  }, createDeps());
  assert.equal(invalidEndpoint.ok, false);
  assert.equal(invalidEndpoint.error.code, "invalid_input");

  const missingAttach = await handleRequest({
    action: "browser_attach_current",
  }, createDeps());
  assert.equal(missingAttach.ok, false);
  assert.equal(missingAttach.error.code, "invalid_input");

  const missingNavigationURL = await handleRequest({
    action: "browser_navigate",
    attach: { browser_kind: "chrome" },
  }, createDeps());
  assert.equal(missingNavigationURL.ok, false);
  assert.equal(missingNavigationURL.error.code, "invalid_input");
});

test("page_read reports attached browser resolution failures as structured errors", async () => {
  const attachFailure = await handleRequest({
    action: "page_read",
    attach: { browser_kind: "chrome" },
  }, createDeps({ connectError: new Error("connect ECONNREFUSED") }));
  assert.equal(attachFailure.ok, false);
  assert.equal(attachFailure.error.code, "browser_attach_failed");

  const noPageMatch = await handleRequest({
    action: "page_read",
    attach: {
      browser_kind: "chrome",
      target: { page_index: 2 },
    },
  }, createDeps({ connectedPages: [createPage({ currentURL: "https://example.com/current" })] }));
  assert.equal(noPageMatch.ok, false);
  assert.equal(noPageMatch.error.code, "page_target_not_found");

  const ambiguousMatch = await handleRequest({
    action: "page_read",
    attach: { browser_kind: "chrome" },
  }, createDeps({
    connectedPages: [
      createPage({ currentURL: "https://example.com/one", title: "One" }),
      createPage({ currentURL: "https://example.com/two", title: "Two" }),
    ],
  }));
  assert.equal(ambiguousMatch.ok, false);
  assert.equal(ambiguousMatch.error.code, "page_target_not_found");

  const malformedContext = await handleRequest({
    action: "page_read",
    attach: { browser_kind: "chrome" },
  }, createDeps({ connectedContexts: [{}] }));
  assert.equal(malformedContext.ok, false);
  assert.equal(malformedContext.error.code, "page_target_not_found");

  const missingURLMatch = await handleRequest({
    action: "page_read",
    attach: {
      browser_kind: "chrome",
      target: { url: "https://example.com/missing" },
    },
  }, createDeps({
    connectedPages: [createPage({ currentURL: "https://example.com/current", title: "Current" })],
  }));
  assert.equal(missingURLMatch.ok, false);
  assert.equal(missingURLMatch.error.code, "page_target_not_found");

  const missingTitleMatch = await handleRequest({
    action: "page_read",
    attach: {
      browser_kind: "chrome",
      target: {
        url: "https://example.com/current",
        title_contains: "missing title",
      },
    },
  }, createDeps({
    connectedPages: [createPage({ currentURL: "https://example.com/current", title: "Current" })],
  }));
  assert.equal(missingTitleMatch.ok, false);
  assert.equal(missingTitleMatch.error.code, "page_target_not_found");
});

test("page_interact uses attached tabs without forcing a new navigation", async () => {
  const actionLog = [];
  const navigationLog = [];
  const response = await handleRequest({
    action: "page_interact",
    url: "https://example.com/form",
    attach: {
      browser_kind: "edge",
      target: { page_index: 0 },
    },
    actions: [
      { type: "click", selector: "button.submit" },
      { type: "wait_for", timeout_ms: 100 },
    ],
  }, createDeps({
    actionLog,
    navigationLog,
    connectedPages: [createPage({
      actionLog,
      navigationLog,
      currentURL: "https://example.com/form",
      title: "Connected Form",
      bodyText: "Interaction complete",
    })],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.actions_applied, 2);
  assert.equal(response.result.source, "playwright_worker_cdp");
  assert.deepEqual(actionLog.map((entry) => entry.action), ["click", "waitForTimeout"]);
  assert.deepEqual(navigationLog, []);
});

test("structured_dom can read an attached page chosen by page index", async () => {
  const response = await handleRequest({
    action: "structured_dom",
    attach: {
      browser_kind: "edge",
      target: { page_index: 1 },
    },
  }, createDeps({
    connectedPages: [
      createPage({ currentURL: "https://example.com/one", title: "One", snapshot: { headings: ["Ignore"], links: [], buttons: [], inputs: [] } }),
      createPage({ currentURL: "https://example.com/two", title: "Two", snapshot: { headings: ["Chosen"], links: ["Docs"], buttons: ["Save"], inputs: ["email"] } }),
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.attached, true);
  assert.equal(response.result.title, "Two");
  assert.deepEqual(response.result.headings, ["Chosen"]);
});

test("page_interact rethrows unsupported interaction types", async () => {
  await assert.rejects(
    () => handleRequest({
      action: "page_interact",
      url: "https://example.com",
      actions: [{ type: "hover" }],
    }, createDeps()),
    /unsupported_interaction_hover/,
  );
});

test("unsupported action stays structured", async () => {
  const response = await handleRequest({ action: "unsupported" }, createDeps());

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "unsupported_action");
});
