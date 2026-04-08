// 媒体处理 worker 的最小入口。
const manifest = {
  worker_name: "media_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["transcode_media", "extract_frames", "normalize_recording"],
};

console.log(JSON.stringify(manifest, null, 2));
