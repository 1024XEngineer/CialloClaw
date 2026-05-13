import assert from "node:assert/strict";
import test from "node:test";
import path from "node:path";

import { extractOCRPDF, extractTextResult, handleRequest, healthResponse } from "./index.js";

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

function buildStoredZip(entries) {
  const localParts = [];
  const centralParts = [];
  let offset = 0;
  for (const [fileName, rawContent] of Object.entries(entries)) {
    const nameBuffer = Buffer.from(fileName, "utf8");
    const contentBuffer = Buffer.isBuffer(rawContent) ? rawContent : Buffer.from(String(rawContent), "utf8");
    const localHeader = Buffer.alloc(30);
    localHeader.writeUInt32LE(0x04034B50, 0);
    localHeader.writeUInt16LE(20, 4);
    localHeader.writeUInt16LE(0, 6);
    localHeader.writeUInt16LE(0, 8);
    localHeader.writeUInt16LE(0, 10);
    localHeader.writeUInt16LE(0, 12);
    localHeader.writeUInt32LE(0, 14);
    localHeader.writeUInt32LE(contentBuffer.length, 18);
    localHeader.writeUInt32LE(contentBuffer.length, 22);
    localHeader.writeUInt16LE(nameBuffer.length, 26);
    localHeader.writeUInt16LE(0, 28);
    localParts.push(localHeader, nameBuffer, contentBuffer);

    const centralHeader = Buffer.alloc(46);
    centralHeader.writeUInt32LE(0x02014B50, 0);
    centralHeader.writeUInt16LE(20, 4);
    centralHeader.writeUInt16LE(20, 6);
    centralHeader.writeUInt16LE(0, 8);
    centralHeader.writeUInt16LE(0, 10);
    centralHeader.writeUInt16LE(0, 12);
    centralHeader.writeUInt16LE(0, 14);
    centralHeader.writeUInt32LE(0, 16);
    centralHeader.writeUInt32LE(contentBuffer.length, 20);
    centralHeader.writeUInt32LE(contentBuffer.length, 24);
    centralHeader.writeUInt16LE(nameBuffer.length, 28);
    centralHeader.writeUInt16LE(0, 30);
    centralHeader.writeUInt16LE(0, 32);
    centralHeader.writeUInt16LE(0, 34);
    centralHeader.writeUInt16LE(0, 36);
    centralHeader.writeUInt32LE(0, 38);
    centralHeader.writeUInt32LE(offset, 42);
    centralParts.push(centralHeader, nameBuffer);

    offset += localHeader.length + nameBuffer.length + contentBuffer.length;
  }

  const centralDirectory = Buffer.concat(centralParts);
  const endRecord = Buffer.alloc(22);
  endRecord.writeUInt32LE(0x06054B50, 0);
  endRecord.writeUInt16LE(0, 4);
  endRecord.writeUInt16LE(0, 6);
  endRecord.writeUInt16LE(Object.keys(entries).length, 8);
  endRecord.writeUInt16LE(Object.keys(entries).length, 10);
  endRecord.writeUInt32LE(centralDirectory.length, 12);
  endRecord.writeUInt32LE(offset, 16);
  endRecord.writeUInt16LE(0, 20);
  return Buffer.concat([...localParts, centralDirectory, endRecord]);
}

test("health reports missing OCR dependencies", async () => {
  const response = await healthResponse(createDeps({
    execFile: async (command) => {
      if (command === "tesseract" || command === "soffice" || command === "antiword" || command === "catdoc") {
        throw new Error("missing tesseract");
      }
      return { stdout: "ok", stderr: "" };
    },
  }));

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "dependency_missing");
  assert.match(response.error.message, /tesseract/);
  assert.equal(response.result.status, "degraded");
  assert.equal(response.result.dependencies.legacy_doc, false);
});

test("health succeeds when OCR backends are installed", async () => {
  const response = await healthResponse(createDeps());

  assert.equal(response.ok, true);
  assert.equal(response.result.status, "ok");
  assert.deepEqual(response.result.dependencies, {
    legacy_doc: true,
    pdftoppm: true,
    pdftotext: true,
    tesseract: true,
  });
});

test("health succeeds when any legacy doc converter is available", async () => {
  const response = await healthResponse(createDeps({
    execFile: async (command) => {
      if (command === "soffice" || command === "catdoc") {
        throw new Error(`missing ${command}`);
      }
      return { stdout: "ok", stderr: "" };
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.dependencies.legacy_doc, true);
});

test("health stays available when only legacy doc conversion is missing", async () => {
  const response = await healthResponse(createDeps({
    execFile: async (command) => {
      if (command === "soffice" || command === "antiword" || command === "catdoc") {
        throw new Error(`missing ${command}`);
      }
      return { stdout: "ok", stderr: "" };
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.status, "degraded");
  assert.equal(response.result.dependencies.legacy_doc, false);
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

test("extract_text normalizes HTML files as plain text", async () => {
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.html" }, createDeps({
    readFile: async () => "<html><body><h1>Title</h1><script>ignored()</script><p>Body text</p></body></html>",
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_text");
  assert.equal(response.result.text, "Title Body text");
});

test("extract_text parses OOXML documents without external converters", async () => {
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.docx" }, createDeps({
    readFile: async (targetPath, encoding) => {
      if (targetPath === "workspace/demo.docx") {
        return buildStoredZip({
          "word/document.xml": "<?xml version=\"1.0\" encoding=\"UTF-8\"?><w:document><w:body><w:p><w:r><w:t>First paragraph</w:t></w:r></w:p><w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p></w:body></w:document>",
          "word/header1.xml": "<w:hdr><w:p><w:r><w:t>Header text</w:t></w:r></w:p></w:hdr>",
        });
      }
      throw new Error(`unexpected readFile target: ${targetPath} (${encoding ?? "buffer"})`);
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_docx");
  assert.equal(response.result.language, "docx_text");
  assert.match(response.result.text, /Header text/);
  assert.match(response.result.text, /First paragraph/);
  assert.match(response.result.text, /Second paragraph/);
});

test("extract_text parses PPTX slides without external converters", async () => {
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.pptx" }, createDeps({
    readFile: async (targetPath, encoding) => {
      if (targetPath === "workspace/demo.pptx") {
        return buildStoredZip({
          "ppt/slides/slide2.xml": "<p:sld><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Second slide body</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>",
          "ppt/slides/slide1.xml": "<p:sld><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Title slide</a:t></a:r></a:p><a:p><a:r><a:t>Agenda item</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>",
        });
      }
      throw new Error(`unexpected readFile target: ${targetPath} (${encoding ?? "buffer"})`);
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_pptx");
  assert.equal(response.result.language, "pptx_text");
  assert.match(response.result.text, /Title slide/);
  assert.match(response.result.text, /Agenda item/);
  assert.match(response.result.text, /Second slide body/);
});

test("extract_text parses XLSX worksheets without external converters", async () => {
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.xlsx" }, createDeps({
    readFile: async (targetPath, encoding) => {
      if (targetPath === "workspace/demo.xlsx") {
        return buildStoredZip({
          "xl/sharedStrings.xml": "<sst><si><t>Name</t></si><si><t>Status</t></si></sst>",
          "xl/worksheets/sheet1.xml": "<worksheet><sheetData><row r=\"1\"><c r=\"A1\" t=\"s\"><v>0</v></c><c r=\"B1\" t=\"s\"><v>1</v></c></row><row r=\"2\"><c r=\"A2\" t=\"inlineStr\"><is><t>Build</t></is></c><c r=\"B2\"><v>42</v></c></row></sheetData></worksheet>",
        });
      }
      throw new Error(`unexpected readFile target: ${targetPath} (${encoding ?? "buffer"})`);
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_xlsx");
  assert.equal(response.result.language, "xlsx_text");
  assert.match(response.result.text, /Sheet 1:/);
  assert.match(response.result.text, /Name\tStatus/);
  assert.match(response.result.text, /Build\t42/);
});

test("extract_text converts legacy doc files with soffice when available", async () => {
  const removed = [];
  const tempDir = path.join("/tmp", "ocr-worker-doc");
  const response = await handleRequest({ action: "extract_text", path: "workspace/demo.doc" }, createDeps({
    execFile: async (command, args) => {
      assert.equal(command, "soffice");
      assert.deepEqual(args, ["--headless", "--convert-to", "txt:Text", "--outdir", tempDir, "workspace/demo.doc"]);
      return { stdout: "", stderr: "" };
    },
    mkdtemp: async () => tempDir,
    readFile: async (targetPath, encoding) => {
      if (targetPath === path.join(tempDir, "demo.txt") && encoding === "utf8") {
        return "converted legacy doc text";
      }
      throw new Error(`unexpected readFile target: ${targetPath} (${encoding ?? "buffer"})`);
    },
    rm: async (targetPath) => {
      removed.push(targetPath);
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_doc");
  assert.equal(response.result.language, "legacy_doc_text");
  assert.equal(response.result.text, "converted legacy doc text");
  assert.deepEqual(removed, [tempDir]);
});

test("extract_text reports legacy doc converter requirements clearly", async () => {
  await assert.rejects(
    () => extractTextResult("workspace/demo.doc", undefined, createDeps({
      execFile: async () => {
        throw new Error("missing converter");
      },
    })),
    /convert the file to \.docx/i,
  );
});

test("extract_text falls back to antiword when soffice is unavailable", async () => {
  const calls = [];
  const result = await extractTextResult("workspace/fallback.doc", undefined, createDeps({
    execFile: async (command, args) => {
      calls.push({ command, args });
      if (command === "soffice") {
        throw new Error("missing soffice");
      }
      if (command === "antiword") {
        return { stdout: "converted by antiword", stderr: "" };
      }
      throw new Error(`unexpected command: ${command}`);
    },
  }));

  assert.equal(result.source, "ocr_worker_doc");
  assert.equal(result.text, "converted by antiword");
  assert.deepEqual(calls.map(({ command }) => command), ["soffice", "antiword"]);
});

test("extract_text rejects docx archives without readable word xml", async () => {
  await assert.rejects(
    () => extractTextResult("workspace/empty.docx", undefined, createDeps({
      readFile: async () => buildStoredZip({
        "word/document.xml": "<w:document><w:body></w:body></w:document>",
      }),
    })),
    /docx_text_not_found/,
  );
});

test("extract_text rejects oversized docx archives before reading them", async () => {
  await assert.rejects(
    () => extractTextResult("workspace/huge.docx", undefined, createDeps({
      readFile: async () => {
        throw new Error("readFile should not be called");
      },
      stat: async () => ({
        isFile: () => true,
        size: 26 * 1024 * 1024,
      }),
    })),
    /docx_archive_too_large/,
  );
});

test("extract_text rejects oversized docx xml entries", async () => {
  const oversizedEntry = "A".repeat((8 * 1024 * 1024) + 1);
  await assert.rejects(
    () => extractTextResult("workspace/large-entry.docx", undefined, createDeps({
      readFile: async () => buildStoredZip({
        "word/document.xml": oversizedEntry,
      }),
      stat: async () => ({
        isFile: () => true,
        size: 1024,
      }),
    })),
    /docx_entry_too_large:word\/document\.xml/,
  );
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

test("ocr_image action returns direct OCR payload", async () => {
  const response = await handleRequest({ action: "ocr_image", path: "workspace/direct.png", language: "eng" }, createDeps({
    execFile: async (command) => {
      assert.equal(command, "tesseract");
      return { stdout: "direct image text", stderr: "" };
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_tesseract");
  assert.equal(response.result.text, "direct image text");
});

test("ocr_pdf action returns OCR payload", async () => {
  const response = await handleRequest({ action: "ocr_pdf", path: "workspace/direct.pdf" }, createDeps({
    execFile: async (command) => {
      assert.equal(command, "pdftotext");
      return { stdout: "direct pdf text", stderr: "" };
    },
  }));

  assert.equal(response.ok, true);
  assert.equal(response.result.source, "ocr_worker_pdf_text");
  assert.equal(response.result.text, "direct pdf text");
});

test("unsupported actions stay structured", async () => {
  const response = await handleRequest({ action: "unknown_action" }, createDeps());

  assert.equal(response.ok, false);
  assert.equal(response.error.code, "unsupported_action");
});
