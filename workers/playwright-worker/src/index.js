import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import { chromium } from "playwright";

const manifest = {
  worker_name: "playwright_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["page_read", "page_search", "page_interact", "structured_dom"],
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

function normalizeText(html) {
  return html
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
  const titleMatch = html.match(/<title[^>]*>([\s\S]*?)<\/title>/i);
  if (titleMatch?.[1]) {
    return normalizeText(titleMatch[1]);
  }
  try {
    return new URL(url).hostname;
  } catch {
    return "untitled page";
  }
}

async function fetchPage(url) {
  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({
      userAgent: "CialloClawPlaywrightWorker/0.1",
    });
    const page = await context.newPage();
    const response = await page.goto(url, {
      waitUntil: "networkidle",
      timeout: 15000,
    });
    if (!response) {
      throw new Error("navigation_failed");
    }
    if (!response.ok()) {
      throw new Error(`http_${response.status()}`);
    }
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
  } finally {
    await browser.close();
  }
}

async function verifyBrowserReady() {
  const browser = await chromium.launch({ headless: true });
  await browser.close();
}

async function handleRequest(request) {
  switch (request.action) {
    case "health":
      await verifyBrowserReady();
      return {
        ok: true,
        result: {
          status: "ok",
          worker_name: manifest.worker_name,
          capabilities: manifest.capabilities,
        },
      };
    case "page_read": {
      const page = await fetchPage(request.url);
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
      const page = await fetchPage(request.url);
      const normalizedQuery = request.query.trim().toLowerCase();
      const rawLimit = Number(request.limit ?? 0);
      const limit = Number.isFinite(rawLimit) && rawLimit > 0 ? Math.floor(rawLimit) : 5;
      const segments = page.textContent
        .split(/[.!?。！？]\s*/)
        .map((segment) => segment.trim())
        .filter(Boolean);
      const allMatches = segments.filter((segment) => segment.toLowerCase().includes(normalizedQuery));
      const matches = allMatches.slice(0, limit);
      return {
        ok: true,
        result: {
          url: page.url,
          query: request.query,
          match_count: allMatches.length,
          matches,
          source: "playwright_worker_browser",
        },
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
