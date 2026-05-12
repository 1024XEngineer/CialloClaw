/* global process, console */
import { spawn } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { buildLocalServiceSidecar } from "./ensureLocalServiceSidecar.mjs";
import { preparePlaywrightRuntime } from "./preparePlaywrightRuntime.mjs";

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

const commandName = process.argv[2];

if (commandName !== "dev" && commandName !== "build") {
  console.error("Usage: node ./scripts/runWithSidecar.mjs <dev|build>");
  process.exit(1);
}

try {
  const sidecarPath = buildLocalServiceSidecar();
  console.log(`Prepared local-service sidecar: ${sidecarPath}`);
  if (commandName === "build") {
    const runtimeRoot = preparePlaywrightRuntime();
    console.log(`Prepared Playwright runtime: ${runtimeRoot}`);
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
}

runFrontendCommand(commandName);
