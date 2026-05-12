import { isIP } from "node:net";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import { setTimeout as sleep } from "node:timers/promises";
import { fileURLToPath } from "node:url";

export const manifest = {
  worker_name: "playwright_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: [
    "page_read",
    "page_search",
    "web_search",
    "page_interact",
    "browser_attach_current",
    "browser_snapshot",
    "browser_navigate",
    "browser_tabs_list",
    "browser_tab_focus",
    "browser_interact",
  ],
};

const browserTimeoutMS = 30000;
const defaultCDPEndpointURL = "http://127.0.0.1:9222";
const defaultManagedCDPEndpointURL = "http://127.0.0.1:9333";
const defaultSearchEngineURL = "https://duckduckgo.com/html/";
const managedBrowserStartupTimeoutMS = 10000;
const managedBrowserRetryIntervalMS = 250;
const managedBrowserUserDataRoot = path.join(os.tmpdir(), "cialloclaw-playwright-browser");
const managedWorkerPageName = "cialloclaw-playwright-worker";
const supportedCDPBrowserKinds = new Set(["chrome", "edge"]);
const workerUserAgent = "CialloClawPlaywrightWorker/0.1";

const defaultDependencies = {
  connectToBrowser,
  launchBrowser: launchBundledBrowser,
  launchManagedBrowser,
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

function writeStream(stream, text) {
  return new Promise((resolve, reject) => {
    stream.write(text, (error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
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

async function launchBundledBrowser() {
  const { chromium } = await import("playwright");
  return chromium.launch({ headless: true });
}

async function connectToBrowser(endpointURL) {
  const { chromium } = await import("playwright");
  return chromium.connectOverCDP(endpointURL);
}

async function browserVersionString(browser) {
  if (!browser || typeof browser.version !== "function") {
    return "";
  }
  try {
    return await browser.version();
  } catch {
    return "";
  }
}

function detectWindowsBrowserCandidates(env = process.env) {
  const roots = [env["ProgramFiles(x86)"], env.ProgramFiles, env.LOCALAPPDATA].filter(Boolean);
  const joinWindowsPath = path.win32.join;
  return [
    {
      browserKind: "edge",
      executablePaths: roots.map((root) => joinWindowsPath(root, "Microsoft", "Edge", "Application", "msedge.exe")),
    },
    {
      browserKind: "chrome",
      executablePaths: roots.map((root) => joinWindowsPath(root, "Google", "Chrome", "Application", "chrome.exe")),
    },
  ];
}

function detectDarwinBrowserCandidates() {
  return [
    {
      browserKind: "chrome",
      executablePaths: ["/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"],
    },
    {
      browserKind: "edge",
      executablePaths: ["/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"],
    },
  ];
}

function detectLinuxBrowserCandidates() {
  return [
    {
      browserKind: "chrome",
      executablePaths: ["/usr/bin/google-chrome", "/usr/bin/google-chrome-stable", "/snap/bin/chromium"],
    },
    {
      browserKind: "edge",
      executablePaths: ["/usr/bin/microsoft-edge", "/usr/bin/microsoft-edge-stable"],
    },
  ];
}

function isFilePath(filePath, statSync = fs.statSync) {
  try {
    return statSync(filePath).isFile();
  } catch {
    return false;
  }
}

export function resolveLocalBrowserCandidates(options = {}) {
  const platformName = options.platform ?? process.platform;
  const statSync = options.statSync ?? fs.statSync;
  const env = options.env ?? process.env;
  let candidates;
  switch (platformName) {
    case "win32":
      candidates = detectWindowsBrowserCandidates(env);
      break;
    case "darwin":
      candidates = detectDarwinBrowserCandidates();
      break;
    default:
      candidates = detectLinuxBrowserCandidates();
      break;
  }

  return candidates
    .map((candidate) => {
      const executablePath = candidate.executablePaths.find((filePath) => isFilePath(filePath, statSync));
      if (!executablePath) {
        return null;
      }
      return {
        browserKind: candidate.browserKind,
        executablePath,
      };
    })
    .filter(Boolean);
}

function managedBrowserUserDataDir(browserKind) {
  return path.join(managedBrowserUserDataRoot, browserKind);
}

function managedBrowserArgs(endpointURL, browserKind) {
  const debugPort = new URL(endpointURL).port || "9333";
  return [
    `--remote-debugging-port=${debugPort}`,
    "--remote-debugging-address=127.0.0.1",
    `--user-data-dir=${managedBrowserUserDataDir(browserKind)}`,
    "--no-first-run",
    "--no-default-browser-check",
    "about:blank",
  ];
}

async function waitForManagedBrowserConnection(endpointURL, deps) {
  const deadline = Date.now() + managedBrowserStartupTimeoutMS;
  const sleeper = deps.sleep ?? sleep;
  let lastError;
  while (Date.now() < deadline) {
    try {
      return await deps.connectToBrowser(endpointURL);
    } catch (error) {
      lastError = error;
      await sleeper(managedBrowserRetryIntervalMS);
    }
  }
  if (lastError instanceof Error) {
    throw lastError;
  }
  throw new Error(`timed out waiting for browser endpoint ${endpointURL}`);
}

export async function launchManagedBrowser(deps = defaultDependencies) {
  const endpointURL = deps.endpointURL ?? defaultManagedCDPEndpointURL;
  const candidates = (deps.resolveLocalBrowserCandidates ?? resolveLocalBrowserCandidates)();
  const mkdirSync = deps.mkdirSync ?? fs.mkdirSync;
  const spawnBrowserProcess = deps.spawnBrowserProcess ?? ((executablePath, args) => spawn(executablePath, args, {
    detached: true,
    stdio: "ignore",
  }));
  if (candidates.length === 0) {
    throw new Error("no local Chrome or Edge browser executable was found");
  }

  for (let index = 0; index < candidates.length; index += 1) {
    const candidate = candidates[index];
    try {
      const browser = await deps.connectToBrowser(endpointURL);
      return {
        browser,
        browserKind: detectConnectedBrowserKind(await browserVersionString(browser)) ?? candidate.browserKind,
        browserTransport: "cdp",
        endpointURL,
        managed: true,
        source: "playwright_worker_local_browser",
      };
    } catch {
      // Fall through and try launching a managed browser process.
    }

    try {
      mkdirSync(managedBrowserUserDataDir(candidate.browserKind), { recursive: true });
      const child = spawnBrowserProcess(candidate.executablePath, managedBrowserArgs(endpointURL, candidate.browserKind));
      child.unref();
      const browser = await waitForManagedBrowserConnection(endpointURL, deps);
      return {
        browser,
        browserKind: detectConnectedBrowserKind(await browserVersionString(browser)) ?? candidate.browserKind,
        browserTransport: "cdp",
        endpointURL,
        managed: true,
        source: "playwright_worker_local_browser",
      };
    } catch (error) {
      if (index === candidates.length - 1) {
        throw error;
      }
    }
  }

  throw new Error("failed to launch a managed local browser");
}

async function acquireBrowserSession(deps = defaultDependencies) {
  let managedError;
  try {
    return await deps.launchManagedBrowser(deps);
  } catch (error) {
    managedError = error;
  }

  try {
    return {
      browser: await deps.launchBrowser(),
      browserKind: undefined,
      browserTransport: "launch",
      endpointURL: undefined,
      managed: false,
      source: "playwright_worker_browser",
    };
  } catch (launchError) {
    const details = [];
    if (managedError) {
      details.push(`local browser failed: ${managedError instanceof Error ? managedError.message : String(managedError)}`);
    }
    if (launchError) {
      details.push(`bundled browser failed: ${launchError instanceof Error ? launchError.message : String(launchError)}`);
    }
    throw new Error(details.join("; "));
  }
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

async function markManagedWorkerPage(page) {
  await page.evaluate((pageName) => {
    window.name = pageName;
  }, managedWorkerPageName);
}

async function isManagedWorkerPage(page) {
  try {
    return await page.evaluate(() => window.name === "cialloclaw-playwright-worker");
  } catch {
    return false;
  }
}

async function openSessionPage(session) {
  const browser = session?.browser;
  const contexts = typeof browser?.contexts === "function" ? browser.contexts() : [];
  if (session?.managed) {
    const context = contexts.find(Boolean);
    if (!context || typeof context.newPage !== "function") {
      throw new Error("managed browser did not expose a writable context");
    }
    const existingPages = typeof context.pages === "function" ? context.pages() : [];
    let page;
    for (let index = existingPages.length - 1; index >= 0; index -= 1) {
      if (await isManagedWorkerPage(existingPages[index])) {
        page = existingPages[index];
        break;
      }
    }
    if (!page) {
      page = await context.newPage();
      await markManagedWorkerPage(page);
    }
    return {
      context,
      page,
      async close() {
        await closeIfPossible(browser, "close");
      },
    };
  }

  const context = await browser.newContext({ userAgent: workerUserAgent });
  const page = await context.newPage();
  return {
    context,
    page,
    async close() {
      await closeResources(context, browser);
    },
  };
}

async function navigatePage(page, url) {
  const response = await page.goto(url, {
    // `load` avoids blocking on pages that keep background network traffic alive
    // after the primary document is already readable for page_read/page_search.
    waitUntil: "load",
    timeout: browserTimeoutMS,
  });
  if (!response) {
    throw new Error("navigation_failed");
  }
  if (!response.ok()) {
    throw new Error(`http_${response.status()}`);
  }
  return response;
}

async function openBrowserPage(url, deps, callback) {
  const session = await acquireBrowserSession(deps);
  let openedPage;
  try {
    openedPage = await openSessionPage(session);
    const page = openedPage.page;
    const response = await navigatePage(page, url);
    return await callback(page, response, {
      attached: false,
      browserKind: session.browserKind,
      browserTransport: session.browserTransport,
      endpointURL: session.endpointURL,
      source: session.source,
    });
  } finally {
    await openedPage?.close?.();
  }
}

function buildWebSearchURL(query, rawURL) {
  const normalizedURL = normalizeOptionalString(rawURL);
  if (normalizedURL) {
    return normalizedURL;
  }

  const normalizedQuery = String(query ?? "").trim();
  if (normalizedQuery === "") {
    throw createStructuredWorkerError("invalid_input", "query is required for web_search");
  }
  const searchURL = new URL(defaultSearchEngineURL);
  searchURL.searchParams.set("q", normalizedQuery);
  return searchURL.toString();
}

async function collectWebSearchResults(page, limit) {
  return page.evaluate((maxResults) => {
    const selectors = [
      "a.result__a",
      ".result__body a[href]",
      "article a[href]",
      ".links_main a[href]",
      "main a[href]",
    ];
    const seen = new Set();
    const results = [];
    const snippetFor = (node) => {
      const containers = [
        node.closest(".result"),
        node.closest("article"),
        node.parentElement,
      ].filter(Boolean);
      for (const container of containers) {
        const snippetNode = container.querySelector(".result__snippet, .snippet, p");
        const snippet = snippetNode?.textContent?.trim();
        if (snippet) {
          return snippet;
        }
      }
      return "";
    };

    for (const selector of selectors) {
      for (const anchor of Array.from(document.querySelectorAll(selector))) {
        const url = anchor.href?.trim();
        const title = anchor.textContent?.trim();
        if (!url || !title || seen.has(url)) {
          continue;
        }
        seen.add(url);
        results.push({
          title,
          url,
          snippet: snippetFor(anchor),
        });
        if (results.length >= maxResults) {
          return results;
        }
      }
    }
    return results;
  }, limit);
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

function createAttachedExecution(attachConfig) {
  return {
    attached: true,
    browserKind: attachConfig.browserKind,
    browserTransport: "cdp",
    endpointURL: attachConfig.endpointURL,
    source: "playwright_worker_cdp",
  };
}

function requireAttachConfig(request) {
  const attachConfig = resolveAttachConfig(request);
  if (!attachConfig) {
    throw createStructuredWorkerError("invalid_input", "attach is required for browser_* actions");
  }
  return attachConfig;
}

function serializeExecutionMetadata(execution) {
  return {
    attached: execution.attached,
    browser_kind: execution.browserKind,
    browser_transport: execution.browserTransport,
    endpoint_url: execution.endpointURL,
    source: execution.source,
  };
}

function detectConnectedBrowserKind(versionString) {
  const normalized = normalizeOptionalString(versionString)?.toLowerCase();
  if (!normalized) {
    return undefined;
  }
  if (normalized.includes("edge")) {
    return "edge";
  }
  if (normalized.includes("chrome")) {
    return "chrome";
  }
  return undefined;
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
    return candidates[attachConfig.pageIndex];
  }

  if (candidates.length === 1) {
    return candidates[0];
  }

  throw createStructuredWorkerError(
    "page_target_not_found",
    "could not resolve a unique attached browser page; provide a stricter url, title_contains, or page_index",
  );
}

async function connectAttachedBrowser(request, deps) {
  const attachConfig = requireAttachConfig(request);

  let browser;
  try {
    browser = await deps.connectToBrowser(attachConfig.endpointURL);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw createStructuredWorkerError("browser_attach_failed", message);
  }

  const connectedBrowserKind = detectConnectedBrowserKind(await browserVersionString(browser));
  if (attachConfig.browserKind && connectedBrowserKind && attachConfig.browserKind !== connectedBrowserKind) {
    throw createStructuredWorkerError(
      "browser_kind_mismatch",
      `attach.browser_kind '${attachConfig.browserKind}' did not match connected browser '${connectedBrowserKind}'`,
    );
  }

  return {
    attachConfig,
    browser,
    execution: createAttachedExecution(attachConfig),
    pageDescriptors: await describeAttachedPages(browser),
  };
}

async function withAttachedBrowser(request, deps, callback) {
  const session = await connectAttachedBrowser(request, deps);
  try {
    return await callback(session);
  } finally {
    await closeIfPossible(session?.browser, "close");
  }
}

async function withAttachedPage(request, deps, callback) {
  return withAttachedBrowser(request, deps, async (session) => {
    const descriptor = selectAttachedPage(session.pageDescriptors, session.attachConfig);
    return callback(descriptor, session.execution);
  });
}

async function openAttachedPage(request, deps, callback) {
  if (!request?.attach) {
    return openBrowserPage(String(request.url ?? ""), deps, callback);
  }

  return withAttachedPage(request, deps, async (descriptor, execution) => {
    // The real browser is user-owned; the short-lived worker process exits after
    // one request, so we intentionally leave the remote browser running and let
    // process teardown release the CDP connection.
    return callback(descriptor.page, undefined, {
      ...execution,
      pageIndex: descriptor.index,
    });
  });
}

function serializeAttachedPageSelection(descriptor, execution) {
  return {
    ...serializeExecutionMetadata(execution),
    page_index: descriptor.index,
    title: descriptor.title,
    url: descriptor.url,
  };
}

function requireTopLevelURL(request, actionName) {
  const url = normalizeOptionalString(request?.url);
  if (!url) {
    throw createStructuredWorkerError("invalid_input", `${actionName} requires a top-level url`);
  }
  return url;
}

async function summarizeLoadedPage(page, fallbackURL, execution, response) {
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
}

// healthResponse validates that the worker can load Playwright, start a browser,
// and create a fresh page before the Go runtime marks the sidecar as ready.
export async function healthResponse(deps = defaultDependencies) {
  const session = await acquireBrowserSession(deps);
  let openedPage;
  try {
    openedPage = await openSessionPage(session);
    return {
      ok: true,
      result: {
        status: "ok",
        worker_name: manifest.worker_name,
        capabilities: manifest.capabilities,
      },
    };
  } finally {
    await openedPage?.close?.();
  }
}

async function fetchPage(request, deps) {
  return openAttachedPage(request, deps, async (page, response, execution) => {
    return summarizeLoadedPage(page, String(request?.url ?? ""), execution, response);
  });
}

async function buildBrowserSnapshot(request, deps) {
  return withAttachedPage(request, deps, async (descriptor, execution) => {
    const page = descriptor.page;
    const html = await page.content();
    const bodyText = await page.locator("body").innerText().catch(() => html);
    const snapshot = await page.evaluate(() => ({
      headings: Array.from(document.querySelectorAll("h1, h2, h3")).map((node) => node.textContent?.trim()).filter(Boolean).slice(0, 20),
      links: Array.from(document.querySelectorAll("a[href]")).map((node) => node.textContent?.trim() || node.getAttribute("href") || "").filter(Boolean).slice(0, 20),
      buttons: Array.from(document.querySelectorAll("button, [role='button']")).map((node) => node.textContent?.trim()).filter(Boolean).slice(0, 20),
      inputs: Array.from(document.querySelectorAll("input, textarea, select")).map((node) => node.getAttribute("name") || node.getAttribute("aria-label") || node.getAttribute("placeholder") || node.tagName.toLowerCase()).filter(Boolean).slice(0, 20),
    }));
    return {
      ...serializeAttachedPageSelection(descriptor, execution),
      text_content: normalizeText(bodyText),
      ...snapshot,
    };
  });
}

async function attachCurrentBrowserPage(request, deps) {
  return withAttachedPage(request, deps, async (descriptor, execution) => ({
    ...serializeAttachedPageSelection(descriptor, execution),
  }));
}

async function listBrowserTabs(request, deps) {
  return withAttachedBrowser(request, deps, async ({ execution, pageDescriptors }) => ({
    ...serializeExecutionMetadata(execution),
    tab_count: pageDescriptors.length,
    tabs: pageDescriptors.map((descriptor) => ({
      page_index: descriptor.index,
      title: descriptor.title,
      url: descriptor.url,
    })),
  }));
}

async function focusBrowserTab(request, deps) {
  return withAttachedPage(request, deps, async (descriptor, execution) => {
    await descriptor.page.bringToFront();
    return serializeAttachedPageSelection(descriptor, execution);
  });
}

async function navigateBrowserPage(request, deps) {
  const destinationURL = requireTopLevelURL(request, "browser_navigate");
  return withAttachedPage(request, deps, async (descriptor, execution) => {
    const response = await navigatePage(descriptor.page, destinationURL);
    const summary = await summarizeLoadedPage(descriptor.page, destinationURL, execution, response);
    return {
      ...serializeAttachedPageSelection(descriptor, execution),
      url: summary.url,
      title: summary.title,
      text_content: summary.textContent,
      mime_type: summary.contentType,
      text_type: summary.contentType,
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

async function interactWithAttachedBrowserPage(request, actions, deps) {
  requireAttachConfig(request);
  return interactWithPage(request, actions, deps);
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
    case "web_search": {
      return executeBrowserRequest(request, deps, async () => {
        const normalizedQuery = String(request.query ?? "").trim();
        const rawLimit = Number(request.limit ?? 0);
        const limit = Number.isFinite(rawLimit) && rawLimit > 0 ? Math.floor(rawLimit) : 5;
        const explicitSearchURL = request.url_is_explicit === false
          ? ""
          : normalizeOptionalString(request.url);
        const searchURL = buildWebSearchURL(normalizedQuery, request.url);
        return openBrowserPage(searchURL, deps, async (page, _response, execution) => {
          const results = await collectWebSearchResults(page, limit);
          if (!explicitSearchURL && results.length === 0) {
            throw createStructuredWorkerError(
              "search_results_unavailable",
              "web_search could not extract results from the default search page",
            );
          }
          return {
            attached: execution.attached,
            browser_kind: execution.browserKind,
            browser_transport: execution.browserTransport,
            endpoint_url: execution.endpointURL,
            query: normalizedQuery,
            search_url: page.url(),
            result_count: results.length,
            results,
            source: execution.source,
          };
        });
      });
    }
    case "browser_attach_current": {
      return executeBrowserRequest(request, deps, async () => attachCurrentBrowserPage(request, deps));
    }
    case "browser_snapshot": {
      return executeBrowserRequest(request, deps, async () => buildBrowserSnapshot(request, deps));
    }
    case "browser_tabs_list": {
      return executeBrowserRequest(request, deps, async () => listBrowserTabs(request, deps));
    }
    case "browser_navigate": {
      return executeBrowserRequest(request, deps, async () => navigateBrowserPage(request, deps));
    }
    case "browser_tab_focus": {
      return executeBrowserRequest(request, deps, async () => focusBrowserTab(request, deps));
    }
    case "browser_interact": {
      const actions = Array.isArray(request.actions) ? request.actions : [];
      const validationError = validatePageActions(actions);
      if (validationError) {
        return validationError;
      }
      return executeBrowserRequest(request, deps, async () => interactWithAttachedBrowserPage(request, actions, deps));
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
    await writeStream(output, `${JSON.stringify(manifest)}\n`);
    process.exit(0);
    return;
  }

  const request = JSON.parse(trimmed);
  const response = await handleRequest(request);
  await writeStream(output, `${JSON.stringify(response)}\n`);
  process.exit(0);
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
    Promise.allSettled([
      writeStream(errorOutput, `${message}\n`),
      writeStream(output, `${JSON.stringify(response)}\n`),
    ]).finally(() => {
      process.exit(1);
    });
  });
}
