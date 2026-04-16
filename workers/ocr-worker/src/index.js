import { mkdtemp, readFile, readdir, rm, stat } from "node:fs/promises";
import { spawn } from "node:child_process";
import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const manifest = {
  worker_name: "ocr_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["ocr_image", "ocr_pdf", "extract_text"],
};

const imageExtensions = new Set([".bmp", ".gif", ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp"]);
const htmlExtensions = new Set([".htm", ".html"]);

const defaultDependencies = {
  execFile,
  mkdtemp,
  readFile,
  readdir,
  rm,
  stat,
  tmpdir,
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

export function normalizeText(value) {
  return String(value ?? "").replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function execFile(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: ["ignore", "pipe", "pipe"] });
    let stdoutText = "";
    let stderrText = "";
    child.stdout.on("data", (chunk) => { stdoutText += chunk.toString(); });
    child.stderr.on("data", (chunk) => { stderrText += chunk.toString(); });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) {
        resolve({ stdout: stdoutText, stderr: stderrText });
        return;
      }
      reject(new Error(stderrText.trim() || `${command} exited with code ${code}`));
    });
  });
}

async function assertPathReadable(targetPath, deps) {
  if (typeof targetPath !== "string" || targetPath.trim() === "") {
    throw new Error("path_required");
  }
  await deps.stat(targetPath);
}

function normalizedExtension(targetPath) {
  return path.extname(String(targetPath ?? "")).toLowerCase();
}

function commandVersionArgs(command) {
  switch (command) {
    case "tesseract":
      return ["--version"];
    default:
      return ["-v"];
  }
}

async function checkCommand(command, deps) {
  try {
    await deps.execFile(command, commandVersionArgs(command));
    return true;
  } catch {
    return false;
  }
}

async function backendStatus(deps) {
  const [tesseract, pdftotext, pdftoppm] = await Promise.all([
    checkCommand("tesseract", deps),
    checkCommand("pdftotext", deps),
    checkCommand("pdftoppm", deps),
  ]);
  return { pdftoppm, pdftotext, tesseract };
}

function missingDependencies(backends) {
  return Object.entries(backends)
    .filter(([, ready]) => !ready)
    .map(([name]) => name)
    .sort();
}

export async function healthResponse(deps = defaultDependencies) {
  const backends = await backendStatus(deps);
  const missing = missingDependencies(backends);
  if (missing.length > 0) {
    return {
      ok: false,
      error: {
        code: "dependency_missing",
        message: `missing OCR dependencies: ${missing.join(", ")}`,
      },
      result: {
        status: "degraded",
        worker_name: manifest.worker_name,
        capabilities: manifest.capabilities,
        dependencies: backends,
      },
    };
  }
  return {
    ok: true,
    result: {
      status: "ok",
      worker_name: manifest.worker_name,
      capabilities: manifest.capabilities,
      dependencies: backends,
    },
  };
}

async function extractPlainText(targetPath, deps) {
  const extension = normalizedExtension(targetPath);
  const text = await deps.readFile(targetPath, "utf8");
  if (htmlExtensions.has(extension)) {
    return normalizeText(text);
  }
  return String(text).trim();
}

function pdfPageCount(rawText) {
  const count = String(rawText ?? "")
    .split("\f")
    .map((pageText) => normalizeText(pageText))
    .filter((pageText) => pageText !== "").length;
  return count;
}

async function extractPDFText(targetPath, deps) {
  const result = await deps.execFile("pdftotext", ["-layout", targetPath, "-"]);
  return {
    pageCount: pdfPageCount(result.stdout),
    text: normalizeText(result.stdout),
  };
}

async function runTesseract(targetPath, language, deps) {
  const args = [targetPath, "stdout"];
  if (typeof language === "string" && language.trim() !== "") {
    args.push("-l", language.trim());
  }
  const result = await deps.execFile("tesseract", args);
  return normalizeText(result.stdout);
}

async function renderPDFPages(targetPath, deps) {
  const tempDir = await deps.mkdtemp(path.join(deps.tmpdir(), "ocr-worker-"));
  const prefix = path.join(tempDir, "page");
  try {
    await deps.execFile("pdftoppm", ["-png", targetPath, prefix]);
    const entries = (await deps.readdir(tempDir))
      .filter((entry) => entry.startsWith("page-") && entry.endsWith(".png"))
      .sort();
    if (entries.length === 0) {
      throw new Error("pdf_pages_not_found");
    }
    return {
      imagePaths: entries.map((entry) => path.join(tempDir, entry)),
      tempDir,
    };
  } catch (error) {
    await deps.rm(tempDir, { force: true, recursive: true });
    throw error;
  }
}

export async function extractOCRPDF(targetPath, language, deps = defaultDependencies) {
  const pdfText = await extractPDFText(targetPath, deps);
  if (pdfText.text !== "") {
    return {
      path: targetPath,
      text: pdfText.text,
      language: "pdf_text",
      page_count: Math.max(pdfText.pageCount, 1),
      source: "ocr_worker_pdf_text",
    };
  }

  const rendered = await renderPDFPages(targetPath, deps);
  try {
    const pages = [];
    for (const imagePath of rendered.imagePaths) {
      const pageText = await runTesseract(imagePath, language, deps);
      if (pageText !== "") {
        pages.push(pageText);
      }
    }
    return {
      path: targetPath,
      text: pages.join("\n\n"),
      language: typeof language === "string" && language.trim() !== "" ? language.trim() : "eng",
      page_count: rendered.imagePaths.length,
      source: "ocr_worker_pdf_ocr",
    };
  } finally {
    await deps.rm(rendered.tempDir, { force: true, recursive: true });
  }
}

export async function extractTextResult(targetPath, language, deps = defaultDependencies) {
  const extension = normalizedExtension(targetPath);
  if (extension === ".pdf") {
    return extractOCRPDF(targetPath, language, deps);
  }
  if (imageExtensions.has(extension)) {
    return {
      path: targetPath,
      text: await runTesseract(targetPath, language, deps),
      language: typeof language === "string" && language.trim() !== "" ? language.trim() : "eng",
      page_count: 1,
      source: "ocr_worker_tesseract",
    };
  }
  return {
    path: targetPath,
    text: await extractPlainText(targetPath, deps),
    language: "plain_text",
    page_count: 1,
    source: "ocr_worker_text",
  };
}

export async function handleRequest(request, deps = defaultDependencies) {
  switch (request.action) {
    case "health":
      return healthResponse(deps);
    case "extract_text": {
      await assertPathReadable(request.path, deps);
      return {
        ok: true,
        result: await extractTextResult(request.path, request.language, deps),
      };
    }
    case "ocr_image": {
      await assertPathReadable(request.path, deps);
      return {
        ok: true,
        result: {
          path: request.path,
          text: await runTesseract(request.path, request.language, deps),
          language: request.language || "eng",
          page_count: 1,
          source: "ocr_worker_tesseract",
        },
      };
    }
    case "ocr_pdf": {
      await assertPathReadable(request.path, deps);
      return {
        ok: true,
        result: await extractOCRPDF(request.path, request.language, deps),
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
    errorOutput.write(`${message}\n`);
    output.write(`${JSON.stringify({ ok: false, error: { code: "worker_failed", message } })}\n`);
    process.exitCode = 1;
  });
}
