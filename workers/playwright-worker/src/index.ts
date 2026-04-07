const manifest = {
  worker_name: "playwright_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["page_read", "page_interact", "structured_dom"],
};

console.log(JSON.stringify(manifest, null, 2));
