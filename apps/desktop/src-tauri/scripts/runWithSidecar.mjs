/* global process, console */
import { spawn, spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { buildLocalServiceSidecar } from "./ensureLocalServiceSidecar.mjs";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

function resolveCommand(name) {
  return process.platform === "win32" && name === "corepack" ? "corepack.cmd" : name;
}

function runFrontendCommand(commandName) {
  const desktopRoot = resolve(currentDirectory, "..", "..");
  const child = spawn(
    process.platform === "win32" ? "cmd.exe" : resolveCommand("corepack"),
    process.platform === "win32" ? ["/d", "/s", "/c", `corepack pnpm ${commandName}`] : ["pnpm", commandName],
    {
    cwd: desktopRoot,
    stdio: "inherit",
    },
  );

  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }

    process.exit(code ?? 0);
  });

  child.on("error", (error) => {
    console.error(error);
    process.exit(1);
  });
}

function stopStaleBundledSidecars() {
  if (process.platform !== "win32") {
    return;
  }

  // Tauri copies the bundled sidecar into `src-tauri/target/*` before booting
  // the app. On Windows, a stale child keeps that copied executable locked and
  // the next build panics with `PermissionDenied` while refreshing the bundle.
  const result = spawnSync(
    "powershell.exe",
    [
      "-NoProfile",
      "-NonInteractive",
      "-Command",
      "Stop-Process -Name cialloclaw-service -Force -ErrorAction SilentlyContinue",
    ],
    {
      stdio: "pipe",
      encoding: "utf8",
    },
  );

  if (result.error) {
    console.warn("Failed to stop stale bundled sidecars before launching Tauri.");
  }
}

const commandName = process.argv[2];

if (commandName !== "dev" && commandName !== "build") {
  console.error("Usage: node ./scripts/runWithSidecar.mjs <dev|build>");
  process.exit(1);
}

try {
  stopStaleBundledSidecars();
  const sidecarPath = buildLocalServiceSidecar();
  console.log(`Prepared local-service sidecar: ${sidecarPath}`);
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
}

runFrontendCommand(commandName);
