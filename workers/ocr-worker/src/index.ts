const manifest = {
  worker_name: "ocr_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["ocr_image", "ocr_pdf", "extract_text"],
};

console.log(JSON.stringify(manifest, null, 2));
