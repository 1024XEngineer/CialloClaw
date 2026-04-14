import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";

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
  const response = await fetch(url, {
    redirect: "follow",
    headers: {
      "user-agent": "CialloClawPlaywrightWorker/0.1",
      accept: "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8",
    },
  });
  if (!response.ok) {
    throw new Error(`http_${response.status}`);
  }
  const html = await response.text();
  const contentType = response.headers.get("content-type") ?? "text/html";
  return {
    url: response.url || url,
    html,
    contentType,
  };
}

async function handleRequest(request) {
  switch (request.action) {
    case "health":
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
      const textContent = normalizeText(page.html);
      return {
        ok: true,
        result: {
          url: page.url,
          title: extractTitle(page.html, page.url),
          text_content: textContent,
          mime_type: page.contentType,
          text_type: page.contentType,
          source: "playwright_worker_http",
        },
      };
    }
    case "page_search": {
      const page = await fetchPage(request.url);
      const textContent = normalizeText(page.html);
      const normalizedQuery = request.query.trim().toLowerCase();
      const rawLimit = Number(request.limit ?? 0);
      const limit = Number.isFinite(rawLimit) && rawLimit > 0 ? Math.floor(rawLimit) : 5;
      const segments = textContent
        .split(/[.!?。！？]\s*/)
        .map((segment) => segment.trim())
        .filter(Boolean);
      const matches = segments.filter((segment) => segment.toLowerCase().includes(normalizedQuery)).slice(0, limit);
      return {
        ok: true,
        result: {
          url: page.url,
          query: request.query,
          match_count: matches.length,
          matches,
          source: "playwright_worker_http",
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
