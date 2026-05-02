import { isIP } from "node:net";
import path from "node:path";
import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import { fileURLToPath } from "node:url";

export const manifest = {
  worker_name: "playwright_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["page_read", "page_search", "page_interact", "structured_dom"],
};

const browserTimeoutMS = 15000;
const defaultCDPEndpointURL = "http://127.0.0.1:9222";
const supportedCDPBrowserKinds = new Set(["chrome", "edge"]);
const workerUserAgent = "CialloClawPlaywrightWorker/0.1";

const defaultDependencies = {
  connectToBrowser,
  launchBrowser,
};

function readAllStdin() {
  return new Promise((resolve, reject) => {
    let data = "";
    input.setEncoding("utf8");
    input.on("data", (chunk) => {
      data += chunk;
    });
    input.on("end", () => resolve(data));
    input.on("error", reject);
  });
}

// normalizeText removes markup noise so page content is stable for summaries,
// searching, and contract tests across worker/runtime boundaries.
export function normalizeText(html) {
  return String(html ?? "")
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/\s+/g, " ")
    .trim();
}

function extractTitle(html, url) {
  const titleMatch = String(html ?? "").match(/<title[^>]*>([\s\S]*?)<\/title>/i);
  if (titleMatch?.[1]) {
    return normalizeText(titleMatch[1]);
  }
  try {
    return new URL(url).hostname;
  } catch {
    return "untitled page";
  }
}

async function launchBrowser() {
  const { chromium } = await import("playwright");
  return chromium.launch({ headless: true });
}

async function connectToBrowser(endpointURL) {
  const { chromium } = await import("playwright");
  return chromium.connectOverCDP(endpointURL);
}

async function closeIfPossible(target, methodName) {
  if (target && typeof target[methodName] === "function") {
    await target[methodName]();
  }
}

async function closeResources(context, browser) {
  try {
    await closeIfPossible(context, "close");
  } finally {
    await closeIfPossible(browser, "close");
  }
}

async function openBrowserPage(url, deps, callback) {
  const browser = await deps.launchBrowser();
  let context;
  try {
    context = await browser.newContext({ userAgent: workerUserAgent });
    const page = await context.newPage();
    const response = await page.goto(url, {
      waitUntil: "networkidle",
      timeout: browserTimeoutMS,
    });
    if (!response) {
      throw new Error("navigation_failed");
    }
    if (!response.ok()) {
      throw new Error(`http_${response.status()}`);
    }
    return await callback(page, response, {
      attached: false,
      browserKind: undefined,
      browserTransport: "launch",
      endpointURL: undefined,
      source: "playwright_worker_browser",
    });
  } finally {
    await closeResources(context, browser);
  }
}

function createStructuredWorkerError(code, message) {
  const error = new Error(message);
  error.code = code;
  return error;
}

function structuredWorkerErrorResponse(error) {
  if (!error || typeof error.code !== "string") {
    return null;
  }

  return {
    ok: false,
    error: {
      code: error.code,
      message: error.message,
    },
  };
}

function normalizeOptionalString(value) {
  const normalized = String(value ?? "").trim();
  return normalized === "" ? undefined : normalized;
}

function normalizeComparableURL(url) {
  const normalized = normalizeOptionalString(url);
  if (!normalized) {
    return undefined;
  }

  try {
    return new URL(normalized).toString();
  } catch {
    return normalized;
  }
}

function isLoopbackHostname(hostname) {
  const normalized = String(hostname ?? "").trim().toLowerCase().replace(/^\[/u, "").replace(/\]$/u, "");
  if (normalized === "localhost" || normalized === "::1") {
    return true;
  }
  return isIP(normalized) === 4 && normalized.startsWith("127.");
}

// normalizeAttachEndpointURL keeps the real-browser attach path bound to local
// debugging targets, so issue-2 does not become a generic outbound CDP dialer.
function normalizeAttachEndpointURL(rawEndpointURL) {
  const endpointURL = normalizeOptionalString(rawEndpointURL) ?? defaultCDPEndpointURL;
  let parsedURL;
  try {
    parsedURL = new URL(endpointURL);
  } catch {
    throw createStructuredWorkerError("invalid_input", "attach.endpoint_url must be a valid URL");
  }

  if (!["http:", "https:", "ws:", "wss:"].includes(parsedURL.protocol)) {
    throw createStructuredWorkerError("invalid_input", "attach.endpoint_url must use http, https, ws, or wss");
  }
  if (!isLoopbackHostname(parsedURL.hostname)) {
    throw createStructuredWorkerError("invalid_input", "attach.endpoint_url must target a loopback host");
  }
  return endpointURL;
}

// resolveAttachConfig keeps the worker-side attach contract additive so the Go
// runtime can keep using the legacy launch flow until issue-3 wiring lands.
function resolveAttachConfig(request) {
  const attach = request?.attach;
  if (!attach) {
    return null;
  }

  const mode = normalizeOptionalString(attach.mode) ?? "cdp";
  if (mode !== "cdp") {
    throw createStructuredWorkerError("invalid_input", "attach.mode must be 'cdp'");
  }

  const browserKind = normalizeOptionalString(attach.browser_kind)?.toLowerCase();
  if (browserKind && !supportedCDPBrowserKinds.has(browserKind)) {
    throw createStructuredWorkerError("unsupported_browser_kind", `unsupported browser kind '${browserKind}'`);
  }

  const target = typeof attach.target === "object" && attach.target !== null ? attach.target : {};
  const rawPageIndex = target.page_index;
  const pageIndex = rawPageIndex === undefined ? undefined : Number(rawPageIndex);
  if (rawPageIndex !== undefined && (!Number.isInteger(pageIndex) || pageIndex < 0)) {
    throw createStructuredWorkerError("invalid_input", "attach.target.page_index must be a non-negative integer");
  }

  return {
    browserKind,
    endpointURL: normalizeAttachEndpointURL(attach.endpoint_url),
    pageIndex,
    targetTitleContains: normalizeOptionalString(target.title_contains)?.toLowerCase(),
    targetURL: normalizeComparableURL(target.url),
  };
}

async function describeAttachedPages(browser) {
  const contexts = typeof browser?.contexts === "function" ? browser.contexts() : [];
  const pages = contexts.flatMap((context) => {
    if (!context || typeof context.pages !== "function") {
      return [];
    }
    return context.pages();
  });

  return Promise.all(pages.map(async (page, index) => ({
    index,
    page,
    title: normalizeOptionalString(await page.title().catch(() => "")) ?? "",
    url: normalizeComparableURL(typeof page.url === "function" ? page.url() : "") ?? "",
  })));
}

// selectAttachedPage avoids blind tab selection by progressively narrowing the
// candidate set and only falling back when the connected browser exposes a
// single usable page.
function selectAttachedPage(pageDescriptors, attachConfig) {
  if (pageDescriptors.length === 0) {
    throw createStructuredWorkerError("page_target_not_found", "no attached browser pages are available");
  }

  let candidates = pageDescriptors;
  if (attachConfig.targetURL) {
    const exactURLMatches = pageDescriptors.filter((descriptor) => descriptor.url === attachConfig.targetURL);
    if (exactURLMatches.length === 0) {
      throw createStructuredWorkerError("page_target_not_found", `no attached browser page matched url '${attachConfig.targetURL}'`);
    }
    candidates = exactURLMatches;
  }

  if (attachConfig.targetTitleContains) {
    const titleMatches = candidates.filter((descriptor) => descriptor.title.toLowerCase().includes(attachConfig.targetTitleContains));
    if (titleMatches.length === 0) {
      throw createStructuredWorkerError(
        "page_target_not_found",
        `no attached browser page matched title_contains '${attachConfig.targetTitleContains}'`,
      );
    }
    candidates = titleMatches;
  }

  if (attachConfig.pageIndex !== undefined) {
    if (attachConfig.pageIndex >= candidates.length) {
      throw createStructuredWorkerError("page_target_not_found", `attach.target.page_index ${attachConfig.pageIndex} is out of range`);
    }
    return candidates[attachConfig.pageIndex].page;
  }

  if (candidates.length === 1) {
    return candidates[0].page;
  }

  throw createStructuredWorkerError(
    "page_target_not_found",
    "could not resolve a unique attached browser page; provide a stricter url, title_contains, or page_index",
  );
}

async function openAttachedPage(request, deps, callback) {
  const attachConfig = resolveAttachConfig(request);
  if (!attachConfig) {
    return openBrowserPage(String(request.url ?? ""), deps, callback);
  }

  let browser;
  try {
    browser = await deps.connectToBrowser(attachConfig.endpointURL);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw createStructuredWorkerError("browser_attach_failed", message);
  }

  const pageDescriptors = await describeAttachedPages(browser);
  const page = selectAttachedPage(pageDescriptors, attachConfig);

  // The real browser is user-owned; the short-lived worker process exits after
  // one request, so we intentionally leave the remote browser running and let
  // process teardown release the CDP connection.
  return callback(page, undefined, {
    attached: true,
    browserKind: attachConfig.browserKind,
    browserTransport: "cdp",
    endpointURL: attachConfig.endpointURL,
    source: "playwright_worker_cdp",
  });
}

// healthResponse validates that the worker can load Playwright, start a browser,
// and create a fresh page before the Go runtime marks the sidecar as ready.
export async function healthResponse(deps = defaultDependencies) {
  const browser = await deps.launchBrowser();
  let context;
  try {
    context = await browser.newContext({ userAgent: workerUserAgent });
    await context.newPage();
    return {
      ok: true,
      result: {
        status: "ok",
        worker_name: manifest.worker_name,
        capabilities: manifest.capabilities,
      },
    };
  } finally {
    await closeResources(context, browser);
  }
}

async function fetchPage(request, deps) {
  return openAttachedPage(request, deps, async (page, response, execution) => {
    const fallbackURL = String(request?.url ?? "");
    const html = await page.content();
    const bodyText = await page.locator("body").innerText().catch(() => html);
    const contentType = response?.headers?.()["content-type"] ?? "text/html";
    return {
      attached: execution.attached,
      browserKind: execution.browserKind,
      browserTransport: execution.browserTransport,
      endpointURL: execution.endpointURL,
      url: page.url() || fallbackURL,
      html,
      source: execution.source,
      title: (await page.title()) || extractTitle(html, page.url()),
      textContent: normalizeText(bodyText),
      contentType,
    };
  });
}

async function buildStructuredDOM(request, deps) {
  return openAttachedPage(request, deps, async (page, _response, execution) => {
    const fallbackURL = String(request?.url ?? "");
    const snapshot = await page.evaluate(() => ({
      headings: Array.from(document.querySelectorAll("h1, h2, h3")).map((node) => node.textContent?.trim()).filter(Boolean).slice(0, 20),
      links: Array.from(document.querySelectorAll("a[href]")).map((node) => node.textContent?.trim() || node.getAttribute("href") || "").filter(Boolean).slice(0, 20),
      buttons: Array.from(document.querySelectorAll("button, [role='button']")).map((node) => node.textContent?.trim()).filter(Boolean).slice(0, 20),
      inputs: Array.from(document.querySelectorAll("input, textarea, select")).map((node) => node.getAttribute("name") || node.getAttribute("aria-label") || node.getAttribute("placeholder") || node.tagName.toLowerCase()).filter(Boolean).slice(0, 20),
    }));
    return {
      attached: execution.attached,
      browserKind: execution.browserKind,
      browserTransport: execution.browserTransport,
      endpointURL: execution.endpointURL,
      url: page.url() || fallbackURL,
      title: (await page.title()) || extractTitle(await page.content(), page.url()),
      source: execution.source,
      ...snapshot,
    };
  });
}

function pageActionTarget(page, selector) {
  return page.locator(selector).first();
}

function actionNeedsSelector(type) {
  switch (type) {
    case "click":
    case "fill":
    case "press":
    case "check":
    case "uncheck":
      return true;
    default:
      return false;
  }
}

function validatePageActions(actions) {
  for (const action of actions) {
    const type = String(action?.type ?? "").trim().toLowerCase();
    const selector = String(action?.selector ?? "").trim();
    const missingSelector = actionNeedsSelector(type) || (type === "wait_for" && selector === "" && Object.prototype.hasOwnProperty.call(action ?? {}, "selector"));
    if (missingSelector && selector === "") {
      return {
        ok: false,
        error: {
          code: "invalid_input",
          message: `selector is required for page_interact action type '${type}'`,
        },
      };
    }
  }
  return null;
}

async function interactWithPage(request, actions, deps) {
  return openAttachedPage(request, deps, async (page, _response, execution) => {
    const fallbackURL = String(request?.url ?? "");
    let applied = 0;
    for (const action of actions) {
      const type = String(action?.type ?? "").trim().toLowerCase();
      const selector = String(action?.selector ?? "").trim();
      switch (type) {
        case "click":
          await pageActionTarget(page, selector).click({ timeout: 10000 });
          applied += 1;
          break;
        case "fill":
          await pageActionTarget(page, selector).fill(String(action?.value ?? ""), { timeout: 10000 });
          applied += 1;
          break;
        case "press":
          await pageActionTarget(page, selector).press(String(action?.key ?? "Enter"), { timeout: 10000 });
          applied += 1;
          break;
        case "check":
          await pageActionTarget(page, selector).check({ timeout: 10000 });
          applied += 1;
          break;
        case "uncheck":
          await pageActionTarget(page, selector).uncheck({ timeout: 10000 });
          applied += 1;
          break;
        case "wait_for":
          if (selector) {
            await pageActionTarget(page, selector).waitFor({ timeout: 10000 });
          } else {
            await page.waitForTimeout(Number(action?.timeout_ms ?? 500));
          }
          applied += 1;
          break;
        default:
          throw new Error(`unsupported_interaction_${type}`);
      }
    }
    const html = await page.content();
    const bodyText = await page.locator("body").innerText().catch(() => html);
    return {
      attached: execution.attached,
      browserKind: execution.browserKind,
      browserTransport: execution.browserTransport,
      endpointURL: execution.endpointURL,
      url: page.url() || fallbackURL,
      title: (await page.title()) || extractTitle(html, page.url()),
      text_content: normalizeText(bodyText),
      actions_applied: applied,
      source: execution.source,
    };
  });
}

async function executeBrowserRequest(request, deps, callback) {
  try {
    return {
      ok: true,
      result: await callback(),
    };
  } catch (error) {
    const structured = structuredWorkerErrorResponse(error);
    if (structured) {
      return structured;
    }
    throw error;
  }
}

// handleRequest keeps the worker protocol stable for the Go sidecar runtime and
// is exported so worker-level tests can exercise real request/response shapes.
export async function handleRequest(request, deps = defaultDependencies) {
  switch (request.action) {
    case "health":
      return healthResponse(deps);
    case "page_read": {
      return executeBrowserRequest(request, deps, async () => {
        const page = await fetchPage(request, deps);
        return {
          attached: page.attached,
          browser_kind: page.browserKind,
          browser_transport: page.browserTransport,
          endpoint_url: page.endpointURL,
          url: page.url,
          title: page.title,
          text_content: page.textContent,
          mime_type: page.contentType,
          text_type: page.contentType,
          source: page.source,
        };
      });
    }
    case "page_search": {
      return executeBrowserRequest(request, deps, async () => {
        const page = await fetchPage(request, deps);
        const normalizedQuery = String(request.query ?? "").trim().toLowerCase();
        const rawLimit = Number(request.limit ?? 0);
        const limit = Number.isFinite(rawLimit) && rawLimit > 0 ? Math.floor(rawLimit) : 5;
        const segments = page.textContent
          .split(/[.!?。！？]\s*/)
          .map((segment) => segment.trim())
          .filter(Boolean);
        const allMatches = normalizedQuery === "" ? [] : segments.filter((segment) => segment.toLowerCase().includes(normalizedQuery));
        const matches = allMatches.slice(0, limit);
        return {
          attached: page.attached,
          browser_kind: page.browserKind,
          browser_transport: page.browserTransport,
          endpoint_url: page.endpointURL,
          url: page.url,
          query: String(request.query ?? ""),
          match_count: allMatches.length,
          matches,
          source: page.source,
        };
      });
    }
    case "structured_dom": {
      return executeBrowserRequest(request, deps, async () => buildStructuredDOM(request, deps));
    }
    case "page_interact": {
      const actions = Array.isArray(request.actions) ? request.actions : [];
      const validationError = validatePageActions(actions);
      if (validationError) {
        return validationError;
      }
      return executeBrowserRequest(request, deps, async () => interactWithPage(request, actions, deps));
    }
    default:
      return {
        ok: false,
        error: {
          code: "unsupported_action",
          message: "unsupported action",
        },
      };
  }
}

function isMainModule() {
  return process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
}

async function main() {
  const raw = await readAllStdin();
  const trimmed = raw.trim();
  if (trimmed === "" || trimmed === "--manifest") {
    output.write(`${JSON.stringify(manifest)}\n`);
    return;
  }

  const request = JSON.parse(trimmed);
  const response = await handleRequest(request);
  output.write(`${JSON.stringify(response)}\n`);
}

if (isMainModule()) {
  main().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    const response = {
      ok: false,
      error: {
        code: "worker_failed",
        message,
      },
    };
    errorOutput.write(`${message}\n`);
    output.write(`${JSON.stringify(response)}\n`);
    process.exitCode = 1;
  });
}
