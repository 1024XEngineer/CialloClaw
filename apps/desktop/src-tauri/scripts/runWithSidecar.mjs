/* global process, console */
import { spawn, spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { buildLocalServiceSidecar } from "./ensureLocalServiceSidecar.mjs";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

function escapePowerShellLiteral(value) {
  return value.replace(/'/g, "''");
}

function buildStopStaleBundledSidecarsCommand(targetRoot) {
  const escapedTargetRoot = escapePowerShellLiteral(targetRoot);
  return [
    `$targetRoot = ([System.IO.Path]::GetFullPath('${escapedTargetRoot}')).TrimEnd('\\') + '\\'`,
    "Get-CimInstance Win32_Process | Where-Object {",
    "  $executablePath = $_.ExecutablePath",
    "  if (-not $executablePath) { return $false }",
    "  $fullPath = [System.IO.Path]::GetFullPath($executablePath)",
    "  $fileName = [System.IO.Path]::GetFileName($fullPath)",
    "  $fullPath.StartsWith($targetRoot, [System.StringComparison]::OrdinalIgnoreCase) -and $fileName.StartsWith('cialloclaw-service', [System.StringComparison]::OrdinalIgnoreCase)",
    "} | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }",
  ].join("; ");
}

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

  const staleSidecarTargetRoot = resolve(currentDirectory, "..", "target");

  // Tauri copies the bundled sidecar into `src-tauri/target/*` before booting
  // the app. On Windows, a stale child keeps that copied executable locked and
  // the next build panics with `PermissionDenied` while refreshing the bundle.
  // Only terminate copied sidecars for the current workspace target directory.
  const result = spawnSync(
    "powershell.exe",
    [
      "-NoProfile",
      "-NonInteractive",
      "-Command",
      buildStopStaleBundledSidecarsCommand(staleSidecarTargetRoot),
    ],
    {
      stdio: "pipe",
      encoding: "utf8",
    },
  );

  if (result.error) {
    console.warn("Failed to stop stale bundled sidecars before launching Tauri.");
    return;
  }

  if (result.status !== 0) {
    const details = result.stderr?.trim() || result.stdout?.trim();
    console.warn(`Failed to stop stale bundled sidecars before launching Tauri.${details ? ` ${details}` : ""}`);
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
