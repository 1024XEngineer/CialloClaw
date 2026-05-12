import assert from "node:assert/strict";
import test from "node:test";

import { handleRequest, healthResponse, launchManagedBrowser, normalizeText, resolveLocalBrowserCandidates } from "./index.js";

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
  let windowName = overrides.windowName ?? "";
  const page = {
    currentURL: overrides.currentURL ?? "https://example.com/final",
    async content() {
      return overrides.html ?? "<html><head><title>Demo Page</title></head><body>Hello world. Search target. Another target.</body></html>";
    },
    async close() {
      actionLog.push({ action: "page.close" });
    },
    async bringToFront() {
      actionLog.push({ action: "bringToFront" });
    },
    async evaluate(callback, ...args) {
      if (overrides.evaluateImpl) {
        return overrides.evaluateImpl(callback, ...args);
      }
      const callbackSource = typeof callback === "function" ? String(callback) : "";
      if (callbackSource.includes("window.name = pageName")) {
        windowName = args[0] ?? "";
        actionLog.push({ action: "markManagedWorkerPage", windowName });
        return undefined;
      }
      if (callbackSource.includes("window.name === \"cialloclaw-playwright-worker\"")) {
        actionLog.push({ action: "isManagedWorkerPage", windowName });
        return windowName === "cialloclaw-playwright-worker";
      }
      if (overrides.searchResults) {
        return overrides.searchResults;
      }
      return overrides.snapshot ?? {
        headings: ["Heading A"],
        links: ["Docs"],
        buttons: ["Submit"],
        inputs: ["email"],
      };
    },
    goto: async (url, options) => {
      navigationLog.push({ action: "goto", options, url });
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
        async version() {
          return overrides.browserVersion ?? "Chrome/125.0.0.0";
        },
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
      if (overrides.launchBrowser) {
        return overrides.launchBrowser();
      }
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
    async launchManagedBrowser() {
      lifecycle.push("launchManagedBrowser");
      if (overrides.launchManagedBrowser) {
        return overrides.launchManagedBrowser();
      }
      throw new Error("managed browser unavailable");
    },
  };
}

test("normalizeText removes markup noise", () => {
  assert.equal(normalizeText("<div>Hello&nbsp;<strong>world</strong></div>"), "Hello world");
});

test("resolveLocalBrowserCandidates keeps the first existing browser path per kind", () => {
  const candidates = resolveLocalBrowserCandidates({
    platform: "win32",
    env: {
      "ProgramFiles(x86)": "C:\\Program Files (x86)",
      ProgramFiles: "C:\\Program Files",
      LOCALAPPDATA: "C:\\Users\\demo\\AppData\\Local",
    },
    statSync(filePath) {
      return {
        isFile() {
          return filePath === "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe" ||
            filePath === "C:\\Users\\demo\\AppData\\Local\\Google\\Chrome\\Application\\chrome.exe";
        },
      };
    },
  });

  assert.deepEqual(candidates, [
    {
      browserKind: "edge",
      executablePath: "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
    },
    {
      browserKind: "chrome",
      executablePath: "C:\\Users\\demo\\AppData\\Local\\Google\\Chrome\\Application\\chrome.exe",
    },
  ]);
});

test("launchManagedBrowser reuses an existing endpoint before spawning", async () => {
  let spawnCalled = false;
  const browser = await launchManagedBrowser({
    resolveLocalBrowserCandidates() {
      return [{ browserKind: "edge", executablePath: "C:\\Edge\\msedge.exe" }];
    },
    async connectToBrowser(endpointURL) {
      assert.equal(endpointURL, "http://127.0.0.1:9555");
      return {
        async close() {},
        version() {
          return "Microsoft Edge/125.0.0.0";
        },
      };
    },
    endpointURL: "http://127.0.0.1:9555",
    spawnBrowserProcess() {
      spawnCalled = true;
      throw new Error("spawn should not be called");
    },
  });

  assert.equal(browser.browserKind, "edge");
  assert.equal(browser.browserTransport, "cdp");
  assert.equal(browser.endpointURL, "http://127.0.0.1:9555");
  assert.equal(browser.source, "playwright_worker_local_browser");
  assert.equal(spawnCalled, false);
});

test("launchManagedBrowser spawns a browser and waits for the endpoint", async () => {
  let attempts = 0;
  let mkdirPath = "";
  let spawned = null;
  const browser = await launchManagedBrowser({
    resolveLocalBrowserCandidates() {
      return [{ browserKind: "chrome", executablePath: "C:\\Chrome\\chrome.exe" }];
    },
    async connectToBrowser() {
      attempts += 1;
      if (attempts === 1) {
        throw new Error("ECONNREFUSED");
      }
      return {
        async close() {},
        async version() {
          return "Chrome/125.0.0.0";
        },
      };
    },
    mkdirSync(targetPath) {
      mkdirPath = targetPath;
    },
    sleep() {
      return Promise.resolve();
    },
    spawnBrowserProcess(executablePath, args) {
      spawned = { executablePath, args };
      return {
        unref() {},
      };
    },
  });

  assert.equal(browser.browserKind, "chrome");
  assert.equal(browser.endpointURL, "http://127.0.0.1:9333");
  assert.match(mkdirPath, /cialloclaw-playwright-browser/);
  assert.equal(spawned.executablePath, "C:\\Chrome\\chrome.exe");
  assert.ok(spawned.args.includes("--remote-debugging-port=9333"));
  assert.ok(spawned.args.includes("about:blank"));
  assert.equal(attempts, 2);
});

test("launchManagedBrowser rejects when no local browser candidate exists", async () => {
  await assert.rejects(
    () => launchManagedBrowser({
      resolveLocalBrowserCandidates() {
        return [];
      },
    }),
    /no local Chrome or Edge browser executable was found/,
  );
});

test("launchManagedBrowser reports endpoint timeouts after spawning", async () => {
  const originalNow = Date.now;
  let attempts = 0;
  Date.now = (() => {
    const values = [0, 0, 10001];
    return () => values.shift() ?? 10001;
  })();
  try {
    await assert.rejects(
      () => launchManagedBrowser({
        resolveLocalBrowserCandidates() {
          return [{ browserKind: "edge", executablePath: "C:\\Edge\\msedge.exe" }];
        },
        async connectToBrowser() {
          attempts += 1;
          throw "ECONNREFUSED";
        },
        mkdirSync() {},
        sleep() {
          return Promise.resolve();
        },
        spawnBrowserProcess() {
          return {
            unref() {},
          };
        },
      }),
      /timed out waiting for browser endpoint/,
    );
  } finally {
    Date.now = originalNow;
  }

  assert.equal(attempts, 2);
});

test("health verifies browser startup and page creation", async () => {
  const lifecycle = [];
  const response = await healthResponse(createDeps({ lifecycle }));

  assert.equal(response.ok, true);
  assert.equal(response.result.status, "ok");
  assert.deepEqual(lifecycle, ["launchManagedBrowser", "launch", "newContext", "newPage", "context.close", "browser.close"]);
});

test("health still closes browser when context cleanup fails", async () => {
  const lifecycle = [];
  await assert.rejects(
    () => healthResponse(createDeps({ lifecycle, contextCloseError: new Error("context close failed") })),
    /context close failed/,
  );

  assert.deepEqual(lifecycle, ["launchManagedBrowser", "launch", "newContext", "newPage", "context.close", "browser.close"]);
});

test("handleRequest delegates health requests through the worker switch", async () => {
  const lifecycle = [];
  const response = await handleRequest({ action: "health" }, createDeps({ lifecycle }));

  assert.equal(response.ok, true);
  assert.equal(response.result.worker_name, "playwright_worker");
  assert.deepEqual(lifecycle, ["launchManagedBrowser", "launch", "newContext", "newPage", "context.close", "browser.close"]);
});

test("health prefers a managed local browser session before bundled launch", async () => {
  const lifecycle = [];
  const existingPage = createPage({ title: "Managed Health Page", windowName: "cialloclaw-playwright-worker" });
  const response = await healthResponse(createDeps({
    lifecycle,
    launchManagedBrowser: async () => ({
      managed: true,
      browserKind: "edge",
      browserTransport: "cdp",
      endpointURL: "http://127.0.0.1:9333",
      source: "playwright_worker_local_browser",
      browser: {
        async close() {
          lifecycle.push("managed.browser.close");
        },
        contexts() {
          return [{
            pages() {
              return [existingPage];
            },
            async newPage() {
              lifecycle.push("managed.newPage");
              return existingPage;
            },
          }];
        },
      },
    }),
  }));

  assert.equal(response.ok, true);
  assert.deepEqual(lifecycle, ["launchManagedBrowser", "managed.browser.close"]);
});

test("page_read returns normalized page metadata", async () => {
  const navigationLog = [];
  const response = await handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
    bodyText: "Hello world from browser",
    gotoURL: "https://example.com/article",
    navigationLog,
    title: "Example Article",
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.url, "https://example.com/article");
  assert.equal(response.result.title, "Example Article");
  assert.equal(response.result.text_content, "Hello world from browser");
  assert.equal(response.result.source, "playwright_worker_browser");
  assert.deepEqual(navigationLog, [{
    action: "goto",
    options: {
      timeout: 30000,
      waitUntil: "load",
    },
    url: "https://example.com",
  }]);
});

test("page_read uses a managed local browser session when available", async () => {
  const lifecycle = [];
  const navigationLog = [];
  const actionLog = [];
  const page = createPage({
    actionLog,
    bodyText: "Hello from managed browser",
    currentURL: "https://example.com/managed",
    gotoURL: "https://example.com/managed",
    navigationLog,
    title: "Managed Article",
    windowName: "cialloclaw-playwright-worker",
  });
  const response = await handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
    lifecycle,
    launchManagedBrowser: async () => ({
      managed: true,
      browserKind: "edge",
      browserTransport: "cdp",
      endpointURL: "http://127.0.0.1:9333",
      source: "playwright_worker_local_browser",
      browser: {
        async close() {
          lifecycle.push("managed.browser.close");
        },
        contexts() {
          return [{
            pages() {
              return [page];
            },
            async newPage() {
              lifecycle.push("managed.newPage");
              return page;
            },
          }];
        },
      },
    }),
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.browser_kind, "edge");
  assert.equal(response.result.browser_transport, "cdp");
  assert.equal(response.result.endpoint_url, "http://127.0.0.1:9333");
  assert.equal(response.result.source, "playwright_worker_local_browser");
  assert.equal(response.result.title, "Managed Article");
  assert.deepEqual(lifecycle, ["launchManagedBrowser", "managed.browser.close"]);
  assert.deepEqual(navigationLog, [{
    action: "goto",
    options: {
      timeout: 30000,
      waitUntil: "load",
    },
    url: "https://example.com",
  }]);
  assert.deepEqual(actionLog.map((entry) => entry.action), ["isManagedWorkerPage"]);
});

test("page_read opens a managed tab when no reusable page is available", async () => {
  const lifecycle = [];
  const actionLog = [];
  const page = createPage({
    actionLog,
    bodyText: "Managed tab created on demand",
    currentURL: "https://example.com/new-managed",
    gotoURL: "https://example.com/new-managed",
    title: "Managed New Tab",
  });
  const response = await handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
    lifecycle,
    launchManagedBrowser: async () => ({
      managed: true,
      browserKind: "edge",
      browserTransport: "cdp",
      endpointURL: "http://127.0.0.1:9333",
      source: "playwright_worker_local_browser",
      browser: {
        async close() {
          lifecycle.push("managed.browser.close");
        },
        contexts() {
          return [{
            async newPage() {
              lifecycle.push("managed.newPage");
              return page;
            },
          }];
        },
      },
    }),
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.title, "Managed New Tab");
  assert.deepEqual(lifecycle, ["launchManagedBrowser", "managed.newPage", "managed.browser.close"]);
  assert.deepEqual(actionLog.map((entry) => entry.action), ["markManagedWorkerPage"]);
});

test("page_read does not reuse an unowned managed browser tab", async () => {
  const lifecycle = [];
  const actionLog = [];
  const userPage = createPage({
    actionLog,
    currentURL: "https://example.com/user-tab",
    title: "User Tab",
    windowName: "user-owned-page",
  });
  const workerPage = createPage({
    actionLog,
    bodyText: "Fresh worker-owned tab",
    currentURL: "https://example.com/worker-tab",
    gotoURL: "https://example.com/worker-tab",
    title: "Worker Tab",
  });
  const response = await handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
    lifecycle,
    launchManagedBrowser: async () => ({
      managed: true,
      browserKind: "edge",
      browserTransport: "cdp",
      endpointURL: "http://127.0.0.1:9333",
      source: "playwright_worker_local_browser",
      browser: {
        async close() {
          lifecycle.push("managed.browser.close");
        },
        contexts() {
          return [{
            pages() {
              return [userPage];
            },
            async newPage() {
              lifecycle.push("managed.newPage");
              return workerPage;
            },
          }];
        },
      },
    }),
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.title, "Worker Tab");
  assert.deepEqual(lifecycle, ["launchManagedBrowser", "managed.newPage", "managed.browser.close"]);
  assert.deepEqual(actionLog.map((entry) => entry.action), ["isManagedWorkerPage", "markManagedWorkerPage"]);
});

test("health fails fast when a managed browser has no writable context", async () => {
  await assert.rejects(
    () => healthResponse(createDeps({
      launchManagedBrowser: async () => ({
        managed: true,
        browserKind: "edge",
        browserTransport: "cdp",
        endpointURL: "http://127.0.0.1:9333",
        source: "playwright_worker_local_browser",
        browser: {
          contexts() {
            return [{}];
          },
        },
      }),
    })),
    /managed browser did not expose a writable context/,
  );
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

test("web_search returns structured search hits", async () => {
  const response = await handleRequest({ action: "web_search", query: "release notes", limit: 2 }, createDeps({
    gotoURL: "https://duckduckgo.com/html/?q=release+notes",
    searchResults: [
      { title: "Release Notes", url: "https://example.com/release-notes", snippet: "Latest release notes." },
      { title: "Docs", url: "https://example.com/docs", snippet: "Documentation home." },
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.query, "release notes");
  assert.equal(response.result.search_url, "https://duckduckgo.com/html/?q=release+notes");
  assert.equal(response.result.result_count, 2);
  assert.deepEqual(response.result.results[0], {
    title: "Release Notes",
    url: "https://example.com/release-notes",
    snippet: "Latest release notes.",
  });
});

test("web_search accepts an explicit url even when query is blank", async () => {
  const response = await handleRequest({
    action: "web_search",
    query: "   ",
    url: "https://example.com/search?q=codex",
  }, createDeps({
    gotoURL: "https://example.com/search?q=codex",
    searchResults: [
      { title: "Codex Result", url: "https://example.com/docs", snippet: "Docs home." },
    ],
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.query, "");
  assert.equal(response.result.search_url, "https://example.com/search?q=codex");
  assert.equal(response.result.result_count, 1);
});

test("web_search fails when the default search page yields no parseable results", async () => {
  const response = await handleRequest({
    action: "web_search",
    query: "release notes",
  }, createDeps({
    gotoURL: "https://duckduckgo.com/html/?q=release+notes",
    searchResults: [],
  }));

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "search_results_unavailable");
  assert.match(response.error.message, /could not extract results/i);
});

test("web_search still treats derived urls as default when the caller marks them implicit", async () => {
  const response = await handleRequest({
    action: "web_search",
    query: "release notes",
    url: "https://duckduckgo.com/html/?q=release+notes",
    url_is_explicit: false,
  }, createDeps({
    gotoURL: "https://duckduckgo.com/html/?q=release+notes",
    searchResults: [],
  }));

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "search_results_unavailable");
  assert.match(response.error.message, /could not extract results/i);
});

test("web_search reports missing queries as structured input errors", async () => {
  const response = await handleRequest({
    action: "web_search",
    query: "   ",
  }, createDeps());

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "invalid_input");
  assert.match(response.error.message, /query is required for web_search/);
});

test("web_search collects deduplicated snippets from the page DOM", async () => {
  const resultAnchor = {
    href: "https://example.com/result-1",
    textContent: "Result One",
    parentElement: null,
    closest(selector) {
      if (selector === ".result") {
        return {
          querySelector() {
            return { textContent: "Snippet One" };
          },
        };
      }
      return null;
    },
  };
  const duplicateAnchor = {
    href: "https://example.com/result-1",
    textContent: "Result One Duplicate",
    parentElement: null,
    closest() {
      return null;
    },
  };
  const articleAnchor = {
    href: "https://example.com/result-2",
    textContent: "Result Two",
    parentElement: {
      querySelector() {
        return { textContent: "Parent Snippet" };
      },
    },
    closest(selector) {
      if (selector === "article") {
        return {
          querySelector() {
            return null;
          },
        };
      }
      return null;
    },
  };
  const ignoredAnchor = {
    href: "https://example.com/ignored",
    textContent: "",
    parentElement: null,
    closest() {
      return null;
    },
  };
  const page = createPage({
    gotoURL: "https://duckduckgo.com/html/?q=codex",
  });
  page.evaluate = async (callback, limit) => {
    const originalDocument = globalThis.document;
    globalThis.document = {
      querySelectorAll(selector) {
        switch (selector) {
          case "a.result__a":
            return [resultAnchor];
          case ".result__body a[href]":
            return [duplicateAnchor];
          case "article a[href]":
            return [articleAnchor];
          case ".links_main a[href]":
            return [ignoredAnchor];
          default:
            return [];
        }
      },
    };
    try {
      return callback(limit);
    } finally {
      globalThis.document = originalDocument;
    }
  };

  const response = await handleRequest({
    action: "web_search",
    query: "codex",
    limit: 2,
  }, createDeps({ page }));

  assert.equal(response.ok, true);
  assert.deepEqual(response.result.results, [
    {
      title: "Result One",
      url: "https://example.com/result-1",
      snippet: "Snippet One",
    },
    {
      title: "Result Two",
      url: "https://example.com/result-2",
      snippet: "Parent Snippet",
    },
  ]);
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

test("page_read reports both managed and bundled launch failures", async () => {
  await assert.rejects(
    () => handleRequest({ action: "page_read", url: "https://example.com" }, createDeps({
      launchManagedBrowser: async () => {
        throw new Error("edge missing");
      },
      launchBrowser: async () => {
        throw new Error("bundled missing");
      },
    })),
    /local browser failed: edge missing; bundled browser failed: bundled missing/,
  );
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
    browserVersion: "Microsoft Edge/125.0.0.0",
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
    browserVersion: "Microsoft Edge/125.0.0.0",
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
  assert.deepEqual(navigationLog, [{
    action: "goto",
    options: {
      timeout: 30000,
      waitUntil: "load",
    },
    url: "https://example.com/next",
  }]);
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
    browserVersion: "Microsoft Edge/125.0.0.0",
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
    browserVersion: "Microsoft Edge/125.0.0.0",
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

  const mismatchedBrowser = await handleRequest({
    action: "browser_attach_current",
    attach: { browser_kind: "edge" },
  }, createDeps({
    browserVersion: "Chrome/125.0.0.0",
    connectedPages: [createPage({ currentURL: "https://example.com/current", title: "Current" })],
  }));
  assert.equal(mismatchedBrowser.ok, false);
  assert.equal(mismatchedBrowser.error.code, "browser_kind_mismatch");

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
    browserVersion: "Microsoft Edge/125.0.0.0",
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
