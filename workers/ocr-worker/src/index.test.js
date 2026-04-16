import assert from "node:assert/strict";
import test from "node:test";
import path from "node:path";

import { extractOCRPDF, handleRequest, healthResponse } from "./index.js";

function createDeps(overrides = {}) {
  return {
    execFile: async () => ({ stdout: "", stderr: "" }),
    mkdtemp: async () => path.join("/tmp", "ocr-worker-test"),
    readFile: async () => "plain text",
    readdir: async () => [],
    rm: async () => {},
    stat: async () => ({ isFile: () => true }),
    tmpdir: () => "/tmp",
    ...overrides,
  };
}

test("health reports missing OCR dependencies", async () => {
  const response = await healthResponse(createDeps({
    execFile: async (command) => {
      if (command === "tesseract") {
        throw new Error("missing tesseract");
      }
      return { stdout: "ok", stderr: "" };
    },
  }));

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "dependency_missing");
  assert.match(response.error.message, /tesseract/);
  assert.equal(response.result.status, "degraded");
});

test("health succeeds when OCR backends are installed", async () => {
  const response = await healthResponse(createDeps());

  assert.equal(response.ok, true);
  assert.equal(response.result.status, "ok");
  assert.deepEqual(response.result.dependencies, {
    pdftoppm: true,
    pdftotext: true,
    tesseract: true,
  });
});

test("extract_text routes images through tesseract", async () => {
  const calls = [];
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.png", language: "eng" }, createDeps({
    execFile: async (command, args) => {
      calls.push({ args, command });
      assert.equal(command, "tesseract");
      return { stdout: "image text", stderr: "" };
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_tesseract");
  assert.equal(response.result.text, "image text");
  assert.equal(calls.length, 1);
});

test("extract_text routes PDFs through PDF extraction before OCR fallback", async () => {
  const calls = [];
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.pdf" }, createDeps({
    execFile: async (command, args) => {
      calls.push({ args, command });
      assert.equal(command, "pdftotext");
      return { stdout: "embedded pdf text", stderr: "" };
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_pdf_text");
  assert.equal(response.result.text, "embedded pdf text");
  assert.equal(calls.length, 1);
});

test("ocr_pdf falls back to page OCR when pdftotext returns no text", async () => {
  const commands = [];
  const removed = [];
  const result = await extractOCRPDF("workspace/scanned.pdf", "chi_sim", createDeps({
    execFile: async (command, args) => {
      commands.push({ args, command });
      if (command === "pdftotext") {
        return { stdout: "\f", stderr: "" };
      }
      if (command === "pdftoppm") {
        return { stdout: "", stderr: "" };
      }
      if (command === "tesseract") {
        return { stdout: `ocr:${path.basename(args[0])}`, stderr: "" };
      }
      throw new Error(`unexpected command: ${command}`);
    },
    mkdtemp: async () => "/tmp/ocr-worker-scan",
    readdir: async () => ["page-1.png", "page-2.png"],
    rm: async (target) => {
      removed.push(target);
    },
  }));

  assert.equal(result.source, "ocr_worker_pdf_ocr");
  assert.equal(result.language, "chi_sim");
  assert.equal(result.page_count, 2);
  assert.match(result.text, /page-1\.png/);
  assert.match(result.text, /page-2\.png/);
  assert.equal(commands.filter(({ command }) => command === "tesseract").length, 2);
  assert.deepEqual(removed, ["/tmp/ocr-worker-scan"]);
});
