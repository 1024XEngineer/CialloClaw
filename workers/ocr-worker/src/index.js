import { mkdtemp, readFile, readdir, rm, stat } from "node:fs/promises";
import { spawn } from "node:child_process";
import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { inflateRawSync } from "node:zlib";

export const manifest = {
  worker_name: "ocr_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["ocr_image", "ocr_pdf", "extract_text"],
};

const imageExtensions = new Set([".bmp", ".gif", ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp"]);
const htmlExtensions = new Set([".htm", ".html"]);
const docxExtensions = new Set([".docx"]);
const pptxExtensions = new Set([".pptx"]);
const xlsxExtensions = new Set([".xlsx"]);
const legacyWordExtensions = new Set([".doc"]);
const ooxmlArchiveSizeLimitBytes = 25 * 1024 * 1024;
const ooxmlEntrySizeLimitBytes = 8 * 1024 * 1024;
const ooxmlTotalXMLSizeLimitBytes = 16 * 1024 * 1024;
const docxWordEntryPattern = /^word\/(document|header\d+|footer\d+|footnotes|endnotes)\.xml$/i;
const pptxSlideEntryPattern = /^ppt\/slides\/slide\d+\.xml$/i;
const xlsxSheetEntryPattern = /^xl\/worksheets\/sheet\d+\.xml$/i;

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
    case "soffice":
      return ["--version"];
    case "tesseract":
      return ["--version"];
    case "antiword":
      return ["-h"];
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
  const [tesseract, pdftotext, pdftoppm, soffice, antiword, catdoc] = await Promise.all([
    checkCommand("tesseract", deps),
    checkCommand("pdftotext", deps),
    checkCommand("pdftoppm", deps),
    checkCommand("soffice", deps),
    checkCommand("antiword", deps),
    checkCommand("catdoc", deps),
  ]);
  return {
    legacy_doc: soffice || antiword || catdoc,
    pdftoppm,
    pdftotext,
    tesseract,
  };
}

function missingDependencies(backends) {
  return Object.entries(backends)
    .filter(([, ready]) => !ready)
    .map(([name]) => name)
    .sort();
}

export async function healthResponse(deps = defaultDependencies) {
  const backends = await backendStatus(deps);
  const requiredBackends = {
    pdftoppm: backends.pdftoppm,
    pdftotext: backends.pdftotext,
    tesseract: backends.tesseract,
  };
  const missing = missingDependencies(requiredBackends);
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
      // `legacy_doc` is an optional compatibility path: keep the worker healthy for
      // text/PDF/image extraction and surface `.doc` gaps only when that format is requested.
      status: backends.legacy_doc ? "ok" : "degraded",
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

async function readBuffer(targetPath, deps) {
  const value = await deps.readFile(targetPath);
  if (Buffer.isBuffer(value)) {
    return value;
  }
  if (value instanceof Uint8Array) {
    return Buffer.from(value);
  }
  if (typeof value === "string") {
    return Buffer.from(value);
  }
  throw new Error("unsupported_binary_payload");
}

function zipEndOfCentralDirectoryOffset(buffer) {
  const minimumRecordLength = 22;
  const lowerBound = Math.max(0, buffer.length - 0xFFFF - minimumRecordLength);
  for (let offset = buffer.length - minimumRecordLength; offset >= lowerBound; offset -= 1) {
    if (buffer.readUInt32LE(offset) === 0x06054B50) {
      return offset;
    }
  }
  throw new Error("zip_end_of_central_directory_not_found");
}

function readZipEntryContent(buffer, localHeaderOffset, compressionMethod, compressedSize, options = {}) {
  if (buffer.readUInt32LE(localHeaderOffset) !== 0x04034B50) {
    throw new Error("zip_local_header_not_found");
  }
  const fileNameLength = buffer.readUInt16LE(localHeaderOffset + 26);
  const extraLength = buffer.readUInt16LE(localHeaderOffset + 28);
  const dataOffset = localHeaderOffset + 30 + fileNameLength + extraLength;
  const compressed = buffer.subarray(dataOffset, dataOffset + compressedSize);
  const maxOutputLength = options.maxOutputLength;
  switch (compressionMethod) {
    case 0:
      if (typeof maxOutputLength === "number" && compressed.length > maxOutputLength) {
        throw new Error(`${options.entryTooLargeErrorCode ?? "zip_entry_too_large"}:${options.entryName ?? "unknown"}`);
      }
      return Buffer.from(compressed);
    case 8:
      try {
        return inflateRawSync(compressed, typeof maxOutputLength === "number" ? { maxOutputLength } : undefined);
      } catch (error) {
        if (typeof maxOutputLength === "number" && error?.code === "ERR_BUFFER_TOO_LARGE") {
          throw new Error(`${options.entryTooLargeErrorCode ?? "zip_entry_too_large"}:${options.entryName ?? "unknown"}`);
        }
        throw error;
      }
    default:
      throw new Error(`zip_compression_unsupported:${compressionMethod}`);
  }
}

function unzipEntries(buffer, options = {}) {
  const endRecordOffset = zipEndOfCentralDirectoryOffset(buffer);
  const entryCount = buffer.readUInt16LE(endRecordOffset + 10);
  let offset = buffer.readUInt32LE(endRecordOffset + 16);
  const entries = new Map();
  let totalSize = 0;
  for (let index = 0; index < entryCount; index += 1) {
    if (buffer.readUInt32LE(offset) !== 0x02014B50) {
      throw new Error("zip_central_directory_entry_not_found");
    }
    const compressionMethod = buffer.readUInt16LE(offset + 10);
    const compressedSize = buffer.readUInt32LE(offset + 20);
    const fileNameLength = buffer.readUInt16LE(offset + 28);
    const extraLength = buffer.readUInt16LE(offset + 30);
    const commentLength = buffer.readUInt16LE(offset + 32);
    const localHeaderOffset = buffer.readUInt32LE(offset + 42);
    const fileName = buffer.subarray(offset + 46, offset + 46 + fileNameLength).toString("utf8");
    if (!options.includeEntry || options.includeEntry(fileName)) {
      const entry = readZipEntryContent(buffer, localHeaderOffset, compressionMethod, compressedSize, {
        entryName: fileName,
        entryTooLargeErrorCode: options.entryTooLargeErrorCode,
        maxOutputLength: options.maxEntrySize,
      });
      totalSize += entry.length;
      if (typeof options.maxTotalSize === "number" && totalSize > options.maxTotalSize) {
        throw new Error(options.totalTooLargeError ?? "zip_text_too_large");
      }
      entries.set(fileName, entry);
    }
    offset += 46 + fileNameLength + extraLength + commentLength;
  }
  return entries;
}

async function readOOXMLArchive(targetPath, archiveKind, deps, options = {}) {
  const targetStats = await deps.stat(targetPath);
  if (typeof targetStats?.size === "number" && targetStats.size > ooxmlArchiveSizeLimitBytes) {
    throw new Error(`${archiveKind}_archive_too_large`);
  }
  return unzipEntries(await readBuffer(targetPath, deps), {
    ...options,
    entryTooLargeErrorCode: `${archiveKind}_entry_too_large`,
    maxEntrySize: ooxmlEntrySizeLimitBytes,
    maxTotalSize: ooxmlTotalXMLSizeLimitBytes,
    totalTooLargeError: `${archiveKind}_text_too_large`,
  });
}

function decodeXMLText(value) {
  return String(value ?? "").replace(/&#(\d+);/g, (_, digits) => String.fromCodePoint(Number.parseInt(digits, 10)))
    .replace(/&#x([0-9a-f]+);/gi, (_, hexDigits) => String.fromCodePoint(Number.parseInt(hexDigits, 16)))
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, "\"")
    .replace(/&apos;/g, "'");
}

function normalizeWordXML(value) {
  const text = String(value ?? "")
    .replace(/<w:tab[^>]*\/>/gi, "\t")
    .replace(/<w:br[^>]*\/>/gi, "\n")
    .replace(/<\/w:p>/gi, "\n")
    .replace(/<\/w:tr>/gi, "\n")
    .replace(/<[^>]+>/g, " ");
  return decodeXMLText(text)
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n[ \t]+/g, "\n")
    .replace(/[ \t]{2,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

async function extractDOCXText(targetPath, deps) {
  const archive = await readOOXMLArchive(targetPath, "docx", deps, {
    includeEntry(name) {
      return docxWordEntryPattern.test(name);
    },
  });
  const candidateNames = Array.from(archive.keys())
    .sort((left, right) => {
      if (left === "word/document.xml") {
        return -1;
      }
      if (right === "word/document.xml") {
        return 1;
      }
      return left.localeCompare(right);
    });
  const sections = [];
  for (const name of candidateNames) {
    const value = normalizeWordXML(archive.get(name)?.toString("utf8") ?? "");
    if (value !== "") {
      sections.push(value);
    }
  }
  if (sections.length === 0) {
    throw new Error("docx_text_not_found");
  }
  return {
    path: targetPath,
    text: sections.join("\n\n"),
    language: "docx_text",
    page_count: 1,
    source: "ocr_worker_docx",
  };
}

function normalizePresentationXML(value) {
  const text = String(value ?? "")
    .replace(/<(?:\w+:)?tab[^>]*\/>/gi, "\t")
    .replace(/<(?:\w+:)?br[^>]*\/>/gi, "\n")
    .replace(/<\/(?:\w+:)?p>/gi, "\n")
    .replace(/<[^>]+>/g, " ");
  return decodeXMLText(text)
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n[ \t]+/g, "\n")
    .replace(/[ \t]{2,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

function slideNumberFromEntryName(name) {
  const match = /slide(\d+)\.xml$/i.exec(name);
  return match ? Number.parseInt(match[1], 10) : Number.MAX_SAFE_INTEGER;
}

function worksheetNumberFromEntryName(name) {
  const match = /sheet(\d+)\.xml$/i.exec(name);
  return match ? Number.parseInt(match[1], 10) : Number.MAX_SAFE_INTEGER;
}

async function extractPPTXText(targetPath, deps) {
  const archive = await readOOXMLArchive(targetPath, "pptx", deps, {
    includeEntry(name) {
      return pptxSlideEntryPattern.test(name);
    },
  });
  const candidateNames = Array.from(archive.keys()).sort((left, right) => slideNumberFromEntryName(left) - slideNumberFromEntryName(right));
  const sections = [];
  for (const name of candidateNames) {
    const value = normalizePresentationXML(archive.get(name)?.toString("utf8") ?? "");
    if (value !== "") {
      sections.push(value);
    }
  }
  if (sections.length === 0) {
    throw new Error("pptx_text_not_found");
  }
  return {
    path: targetPath,
    text: sections.join("\n\n"),
    language: "pptx_text",
    page_count: Math.max(sections.length, 1),
    source: "ocr_worker_pptx",
  };
}

function normalizeSpreadsheetCellText(value) {
  return decodeXMLText(String(value ?? ""))
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n[ \t]+/g, "\n")
    .replace(/[ \t]{2,}/g, " ")
    .trim();
}

function extractXMLTextRuns(value) {
  const parts = [];
  for (const match of String(value ?? "").matchAll(/<(?:\w+:)?t\b[^>]*>([\s\S]*?)<\/(?:\w+:)?t>/gi)) {
    parts.push(decodeXMLText(match[1]));
  }
  return parts.join("");
}

function parseXLSXSharedStrings(value) {
  const items = [];
  for (const match of String(value ?? "").matchAll(/<si\b[^>]*>([\s\S]*?)<\/si>/gi)) {
    items.push(normalizeSpreadsheetCellText(extractXMLTextRuns(match[1])));
  }
  return items;
}

function extractXLSXCellText(attributes, body, sharedStrings) {
  const typeMatch = /\bt="([^"]+)"/i.exec(attributes);
  const cellType = typeMatch?.[1] ?? "";
  if (cellType === "inlineStr") {
    return normalizeSpreadsheetCellText(extractXMLTextRuns(body));
  }
  const valueMatch = /<v\b[^>]*>([\s\S]*?)<\/v>/i.exec(body);
  if (!valueMatch) {
    return "";
  }
  const rawValue = decodeXMLText(valueMatch[1]);
  if (cellType === "s") {
    const index = Number.parseInt(rawValue, 10);
    if (Number.isInteger(index) && index >= 0 && index < sharedStrings.length) {
      return sharedStrings[index] ?? "";
    }
    return "";
  }
  if (cellType === "b") {
    if (rawValue === "1") {
      return "TRUE";
    }
    if (rawValue === "0") {
      return "FALSE";
    }
  }
  return normalizeSpreadsheetCellText(rawValue);
}

function sheetLabelFromEntryName(name) {
  const match = /sheet(\d+)\.xml$/i.exec(name);
  if (!match) {
    return "Sheet";
  }
  return `Sheet ${match[1]}`;
}

function extractXLSXSheetText(value, sharedStrings, label) {
  const rows = [];
  for (const rowMatch of String(value ?? "").matchAll(/<row\b[^>]*>([\s\S]*?)<\/row>/gi)) {
    const cells = [];
    for (const cellMatch of rowMatch[1].matchAll(/<c\b([^>]*)>([\s\S]*?)<\/c>/gi)) {
      const cellText = extractXLSXCellText(cellMatch[1], cellMatch[2], sharedStrings);
      if (cellText !== "") {
        cells.push(cellText);
      }
    }
    if (cells.length > 0) {
      rows.push(cells.join("\t"));
    }
  }
  if (rows.length === 0) {
    return "";
  }
  return `${label}:\n${rows.join("\n")}`;
}

async function extractXLSXText(targetPath, deps) {
  const archive = await readOOXMLArchive(targetPath, "xlsx", deps, {
    includeEntry(name) {
      return name === "xl/sharedStrings.xml" || xlsxSheetEntryPattern.test(name);
    },
  });
  const sharedStrings = parseXLSXSharedStrings(archive.get("xl/sharedStrings.xml")?.toString("utf8") ?? "");
  const candidateNames = Array.from(archive.keys())
    .filter((name) => xlsxSheetEntryPattern.test(name))
    .sort((left, right) => worksheetNumberFromEntryName(left) - worksheetNumberFromEntryName(right));
  const sections = [];
  for (const name of candidateNames) {
    const value = extractXLSXSheetText(archive.get(name)?.toString("utf8") ?? "", sharedStrings, sheetLabelFromEntryName(name));
    if (value !== "") {
      sections.push(value);
    }
  }
  if (sections.length === 0) {
    throw new Error("xlsx_text_not_found");
  }
  return {
    path: targetPath,
    text: sections.join("\n\n"),
    language: "xlsx_text",
    page_count: Math.max(sections.length, 1),
    source: "ocr_worker_xlsx",
  };
}

async function extractLegacyWordViaSoffice(targetPath, deps) {
  const tempDir = await deps.mkdtemp(path.join(deps.tmpdir(), "ocr-worker-doc-"));
  const outputPath = path.join(tempDir, `${path.basename(targetPath, path.extname(targetPath))}.txt`);
  try {
    await deps.execFile("soffice", ["--headless", "--convert-to", "txt:Text", "--outdir", tempDir, targetPath]);
    const text = String(await deps.readFile(outputPath, "utf8")).trim();
    if (text === "") {
      throw new Error("legacy_doc_conversion_empty");
    }
    return {
      path: targetPath,
      text,
      language: "legacy_doc_text",
      page_count: 1,
      source: "ocr_worker_doc",
    };
  } finally {
    await deps.rm(tempDir, { force: true, recursive: true });
  }
}

async function extractLegacyWordViaStdout(command, targetPath, deps) {
  const result = await deps.execFile(command, [targetPath]);
  const text = normalizeText(result.stdout);
  if (text === "") {
    throw new Error(`${command}_empty_output`);
  }
  return {
    path: targetPath,
    text,
    language: "legacy_doc_text",
    page_count: 1,
    source: "ocr_worker_doc",
  };
}

async function extractLegacyDOCText(targetPath, deps) {
  const attempts = [
    async () => extractLegacyWordViaSoffice(targetPath, deps),
    async () => extractLegacyWordViaStdout("antiword", targetPath, deps),
    async () => extractLegacyWordViaStdout("catdoc", targetPath, deps),
  ];
  for (const attempt of attempts) {
    try {
      return await attempt();
    } catch {
      // Try the next converter. Legacy .doc files often need a real document
      // parser, so we avoid falling back to unsafe byte-to-text decoding here.
    }
  }
  throw new Error("legacy .doc extraction requires soffice, antiword, or catdoc; convert the file to .docx if those tools are unavailable");
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
  if (docxExtensions.has(extension)) {
    return extractDOCXText(targetPath, deps);
  }
  if (pptxExtensions.has(extension)) {
    return extractPPTXText(targetPath, deps);
  }
  if (xlsxExtensions.has(extension)) {
    return extractXLSXText(targetPath, deps);
  }
  if (legacyWordExtensions.has(extension)) {
    return extractLegacyDOCText(targetPath, deps);
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
