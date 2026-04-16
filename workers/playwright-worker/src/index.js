import path from "node:path";
import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import { fileURLToPath } from "node:url";

export const manifest = {
  worker_name: "playwright_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["page_read", "page_search", "page_interact", "structured_dom"],
};

const browserTimeoutMS = 15000;
const workerUserAgent = "CialloClawPlaywrightWorker/0.1";

const defaultDependencies = {
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
    return await callback(page, response);
  } finally {
    await closeResources(context, browser);
  }
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

async function fetchPage(url, deps) {
  return openBrowserPage(url, deps, async (page, response) => {
    const html = await page.content();
    const bodyText = await page.locator("body").innerText().catch(() => html);
    const contentType = response.headers()["content-type"] ?? "text/html";
    return {
      url: page.url() || url,
      html,
      title: (await page.title()) || extractTitle(html, page.url()),
      textContent: normalizeText(bodyText),
      contentType,
    };
  });
}

async function buildStructuredDOM(url, deps) {
  return openBrowserPage(url, deps, async (page) => {
    const snapshot = await page.evaluate(() => ({
      headings: Array.from(document.querySelectorAll("h1, h2, h3")).map((node) => node.textContent?.trim()).filter(Boolean).slice(0, 20),
      links: Array.from(document.querySelectorAll("a[href]")).map((node) => node.textContent?.trim() || node.getAttribute("href") || "").filter(Boolean).slice(0, 20),
      buttons: Array.from(document.querySelectorAll("button, [role='button']")).map((node) => node.textContent?.trim()).filter(Boolean).slice(0, 20),
      inputs: Array.from(document.querySelectorAll("input, textarea, select")).map((node) => node.getAttribute("name") || node.getAttribute("aria-label") || node.getAttribute("placeholder") || node.tagName.toLowerCase()).filter(Boolean).slice(0, 20),
    }));
    return {
      url: page.url() || url,
      title: (await page.title()) || extractTitle(await page.content(), page.url()),
      source: "playwright_worker_browser",
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

async function interactWithPage(url, actions, deps) {
  return openBrowserPage(url, deps, async (page) => {
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
      url: page.url() || url,
      title: (await page.title()) || extractTitle(html, page.url()),
      text_content: normalizeText(bodyText),
      actions_applied: applied,
      source: "playwright_worker_browser",
    };
  });
}

// handleRequest keeps the worker protocol stable for the Go sidecar runtime and
// is exported so worker-level tests can exercise real request/response shapes.
export async function handleRequest(request, deps = defaultDependencies) {
  switch (request.action) {
    case "health":
      return healthResponse(deps);
    case "page_read": {
      const page = await fetchPage(String(request.url ?? ""), deps);
      return {
        ok: true,
        result: {
          url: page.url,
          title: page.title,
          text_content: page.textContent,
          mime_type: page.contentType,
          text_type: page.contentType,
          source: "playwright_worker_browser",
        },
      };
    }
    case "page_search": {
      const page = await fetchPage(String(request.url ?? ""), deps);
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
        ok: true,
        result: {
          url: page.url,
          query: String(request.query ?? ""),
          match_count: allMatches.length,
          matches,
          source: "playwright_worker_browser",
        },
      };
    }
    case "structured_dom": {
      return {
        ok: true,
        result: await buildStructuredDOM(String(request.url ?? ""), deps),
      };
    }
    case "page_interact": {
      const actions = Array.isArray(request.actions) ? request.actions : [];
      const validationError = validatePageActions(actions);
      if (validationError) {
        return validationError;
      }
      return {
        ok: true,
        result: await interactWithPage(
          String(request.url ?? ""),
          actions,
          deps,
        ),
      };
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
