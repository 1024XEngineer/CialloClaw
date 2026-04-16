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
  const page = {
    currentURL: overrides.currentURL ?? "https://example.com/final",
    async content() {
      return overrides.html ?? "<html><head><title>Demo Page</title></head><body>Hello world. Search target. Another target.</body></html>";
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
      page.currentURL = overrides.gotoURL ?? url;
      return overrides.response ?? createResponse();
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
  return {
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

test("page_search returns bounded matches", async () => {
  const response = await handleRequest({ action: "page_search", url: "https://example.com", query: "target", limit: 1 }, createDeps({
    bodyText: "First target. Second target. Third miss.",
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.match_count, 2);
  assert.deepEqual(response.result.matches, ["First target"]);
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
});

test("unsupported action stays structured", async () => {
  const response = await handleRequest({ action: "unsupported" }, createDeps());

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "unsupported_action");
});
