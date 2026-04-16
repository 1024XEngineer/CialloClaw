import { mkdir, readdir, stat } from "node:fs/promises";
import { spawn } from "node:child_process";
import { stdin as input, stdout as output, stderr as errorOutput } from "node:process";
import path from "node:path";

const manifest = {
  worker_name: "media_worker",
  transport: ["stdio", "jsonrpc"],
  capabilities: ["transcode_media", "extract_frames", "normalize_recording"],
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

async function ensureInputReadable(targetPath) {
  if (typeof targetPath !== "string" || targetPath.trim() === "") {
    throw new Error("path_required");
  }
  await stat(targetPath);
}

async function ensureOutputDirectory(targetPath) {
  await mkdir(path.dirname(targetPath), { recursive: true });
}

async function ensureDirectory(targetDir) {
  if (typeof targetDir !== "string" || targetDir.trim() === "") {
    throw new Error("output_dir_required");
  }
  await mkdir(targetDir, { recursive: true });
}

async function healthCheck() {
  await execFile("ffmpeg", ["-version"]);
  await execFile("ffprobe", ["-version"]);
}

async function transcodeMedia(inputPath, outputPath, format) {
  await ensureOutputDirectory(outputPath);
  await execFile("ffmpeg", ["-y", "-i", inputPath, "-c:v", "libx264", "-c:a", "aac", outputPath]);
  return {
    input_path: inputPath,
    output_path: outputPath,
    format: format || path.extname(outputPath).replace(/^\./, "") || "mp4",
    source: "media_worker_ffmpeg",
  };
}

async function normalizeRecording(inputPath, outputPath) {
  await ensureOutputDirectory(outputPath);
  await execFile("ffmpeg", ["-y", "-i", inputPath, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-c:a", "aac", outputPath]);
  return {
    input_path: inputPath,
    output_path: outputPath,
    format: path.extname(outputPath).replace(/^\./, "") || "mp4",
    source: "media_worker_normalize",
  };
}

async function extractFrames(inputPath, outputDir, everySeconds, limit) {
  await ensureDirectory(outputDir);
  const safeEverySeconds = Number.isFinite(everySeconds) && everySeconds > 0 ? everySeconds : 1;
  const safeLimit = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : 5;
  const outputPattern = path.join(outputDir, "frame-%03d.jpg");
  await execFile("ffmpeg", ["-y", "-i", inputPath, "-vf", `fps=1/${safeEverySeconds}`, "-frames:v", String(safeLimit), outputPattern]);
  const framePaths = (await readdir(outputDir))
    .filter((entry) => entry.startsWith("frame-") && entry.endsWith(".jpg"))
    .sort()
    .map((entry) => path.join(outputDir, entry));
  return {
    input_path: inputPath,
    output_dir: outputDir,
    frame_paths: framePaths,
    frame_count: framePaths.length,
    source: "media_worker_frames",
  };
}

async function handleRequest(request) {
  switch (request.action) {
    case "health":
      await healthCheck();
      return { ok: true, result: { status: "ok", worker_name: manifest.worker_name, capabilities: manifest.capabilities } };
    case "transcode_media":
      await ensureInputReadable(request.path);
      return { ok: true, result: await transcodeMedia(request.path, request.output_path, request.format) };
    case "normalize_recording":
      await ensureInputReadable(request.path);
      return { ok: true, result: await normalizeRecording(request.path, request.output_path) };
    case "extract_frames":
      await ensureInputReadable(request.path);
      return { ok: true, result: await extractFrames(request.path, request.output_dir, Number(request.every_seconds ?? 1), Number(request.limit ?? 5)) };
    default:
      return { ok: false, error: { code: "unsupported_action", message: "unsupported action" } };
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
  errorOutput.write(`${message}\n`);
  output.write(`${JSON.stringify({ ok: false, error: { code: "worker_failed", message } })}\n`);
  process.exitCode = 1;
});
