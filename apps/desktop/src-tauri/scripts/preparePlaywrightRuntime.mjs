/* global process, console */
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

function resolveCommand(name) {
  if (process.platform !== "win32") {
    return name;
  }
  return name;
}

function runNpmInstall(args, options) {
  if (process.platform === "win32") {
    return run("cmd.exe", ["/d", "/s", "/c", `npm ${args.join(" ")}`], options);
  }
  return run("npm", args, options);
}

function run(command, args, options) {
  const resolvedCommand = resolveCommand(command);
  const result = spawnSync(resolvedCommand, args, {
    stdio: "pipe",
    encoding: "utf8",
    ...options,
  });

  if (result.status === 0) {
    return result;
  }

  const stderr = result.stderr?.trim();
  const stdout = result.stdout?.trim();
  const details = stderr || stdout || `exit code ${result.status ?? "unknown"}`;
  throw new Error(`${resolvedCommand} ${args.join(" ")} failed: ${details}`);
}

function loadPlaywrightVersion(workerPackagePath) {
  const packageJson = JSON.parse(readFileSync(workerPackagePath, "utf8"));
  const version = packageJson?.dependencies?.playwright;
  if (typeof version !== "string" || version.trim() === "") {
    throw new Error(`playwright dependency is missing from ${workerPackagePath}`);
  }
  return version.trim();
}

export function preparePlaywrightRuntime() {
  const repoRoot = resolve(currentDirectory, "..", "..", "..", "..");
  const srcTauriRoot = resolve(currentDirectory, "..");
  const sourceWorkerRoot = resolve(repoRoot, "workers", "playwright-worker");
  const sourceWorkerEntry = resolve(sourceWorkerRoot, "src", "index.js");
  const sourceWorkerPackage = resolve(sourceWorkerRoot, "package.json");
  const runtimeRoot = resolve(srcTauriRoot, "resources", "playwright-runtime");
  const packagedWorkerRoot = resolve(runtimeRoot, "workers", "playwright-worker");
  const packagedWorkerSourceRoot = resolve(packagedWorkerRoot, "src");
  const browsersRoot = resolve(runtimeRoot, "ms-playwright");

  if (!existsSync(sourceWorkerEntry)) {
    throw new Error(`playwright worker entry is missing: ${sourceWorkerEntry}`);
  }

  const playwrightVersion = loadPlaywrightVersion(sourceWorkerPackage);

  rmSync(runtimeRoot, { recursive: true, force: true });
  mkdirSync(packagedWorkerSourceRoot, { recursive: true });
  cpSync(sourceWorkerEntry, resolve(packagedWorkerSourceRoot, "index.js"));
  writeFileSync(
    resolve(packagedWorkerRoot, "package.json"),
    JSON.stringify(
      {
        name: "@cialloclaw/playwright-worker-runtime",
        private: true,
        type: "module",
        dependencies: {
          playwright: playwrightVersion,
        },
      },
      null,
      2,
    ) + "\n",
    "utf8",
  );

  runNpmInstall(["install", "--omit=dev"], { cwd: packagedWorkerRoot });
  run(process.execPath, [resolve(packagedWorkerRoot, "node_modules", "playwright", "cli.js"), "install", "chromium"], {
    cwd: packagedWorkerRoot,
    env: {
      ...process.env,
      PLAYWRIGHT_BROWSERS_PATH: browsersRoot,
      PLAYWRIGHT_SKIP_BROWSER_GC: "1",
    },
  });

  return runtimeRoot;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const runtimeRoot = preparePlaywrightRuntime();
  console.log(`Prepared Playwright runtime at ${runtimeRoot}`);
}
