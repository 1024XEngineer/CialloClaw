/* global process, console */
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const currentDirectory = dirname(fileURLToPath(import.meta.url));
const runtimeCacheManifestFileName = "playwright-runtime-manifest.json";

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

function writeRuntimeWorkerPackage(packagePath, playwrightVersion) {
  writeFileSync(
    packagePath,
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
}

function loadRuntimeCacheManifest(manifestPath) {
  if (!existsSync(manifestPath)) {
    return null;
  }
  try {
    return JSON.parse(readFileSync(manifestPath, "utf8"));
  } catch {
    return null;
  }
}

function createRuntimeCacheManifest(playwrightVersion) {
  return {
    node_version: process.version,
    platform: process.platform,
    arch: process.arch,
    playwright_version: playwrightVersion,
  };
}

function shouldRefreshRuntimeCache(options) {
  const { browsersRoot, cachedWorkerRoot, manifestPath, playwrightVersion } = options;
  const manifest = loadRuntimeCacheManifest(manifestPath);
  if (!manifest) {
    return true;
  }
  const expectedManifest = createRuntimeCacheManifest(playwrightVersion);
  if (JSON.stringify(manifest) !== JSON.stringify(expectedManifest)) {
    return true;
  }
  return !existsSync(resolve(cachedWorkerRoot, "node_modules")) || !existsSync(browsersRoot);
}

function installRuntimeCache(options) {
  const { browsersRoot, cachedWorkerRoot, cachedWorkerSourceRoot, manifestPath, playwrightVersion, sourceWorkerEntry } = options;
  rmSync(resolve(cachedWorkerRoot, "..", ".."), { recursive: true, force: true });
  mkdirSync(cachedWorkerSourceRoot, { recursive: true });
  cpSync(sourceWorkerEntry, resolve(cachedWorkerSourceRoot, "index.js"));
  writeRuntimeWorkerPackage(resolve(cachedWorkerRoot, "package.json"), playwrightVersion);

  runNpmInstall(["install", "--omit=dev"], { cwd: cachedWorkerRoot });
  run(process.execPath, [resolve(cachedWorkerRoot, "node_modules", "playwright", "cli.js"), "install", "chromium"], {
    cwd: cachedWorkerRoot,
    env: {
      ...process.env,
      PLAYWRIGHT_BROWSERS_PATH: browsersRoot,
      PLAYWRIGHT_SKIP_BROWSER_GC: "1",
    },
  });

  writeFileSync(manifestPath, JSON.stringify(createRuntimeCacheManifest(playwrightVersion), null, 2) + "\n", "utf8");
}

/**
 * Prepare the packaged Playwright runtime by reusing a local build cache for
 * the heavyweight browser/runtime install and recreating the final resource
 * directory from fresh sources on each build.
 */
export function preparePlaywrightRuntime() {
  const repoRoot = resolve(currentDirectory, "..", "..", "..", "..");
  const srcTauriRoot = resolve(currentDirectory, "..");
  const sourceWorkerRoot = resolve(repoRoot, "workers", "playwright-worker");
  const sourceWorkerEntry = resolve(sourceWorkerRoot, "src", "index.js");
  const sourceWorkerPackage = resolve(sourceWorkerRoot, "package.json");
  const sourceNodeExecutable = resolve(process.execPath);
  const runtimeCacheRoot = resolve(srcTauriRoot, ".cache", "playwright-runtime");
  const runtimeRoot = resolve(srcTauriRoot, "resources", "playwright-runtime");
  const packagedNodeRoot = resolve(runtimeRoot, "node");
  const packagedWorkerRoot = resolve(runtimeRoot, "workers", "playwright-worker");
  const packagedWorkerSourceRoot = resolve(packagedWorkerRoot, "src");
  const browsersRoot = resolve(runtimeRoot, "ms-playwright");
  const cachedWorkerRoot = resolve(runtimeCacheRoot, "workers", "playwright-worker");
  const cachedWorkerSourceRoot = resolve(cachedWorkerRoot, "src");
  const cachedBrowsersRoot = resolve(runtimeCacheRoot, "ms-playwright");
  const runtimeCacheManifestPath = resolve(runtimeCacheRoot, runtimeCacheManifestFileName);

  if (!existsSync(sourceWorkerEntry)) {
    throw new Error(`playwright worker entry is missing: ${sourceWorkerEntry}`);
  }
  if (!existsSync(sourceNodeExecutable)) {
    throw new Error(`node executable is missing: ${sourceNodeExecutable}`);
  }

  const playwrightVersion = loadPlaywrightVersion(sourceWorkerPackage);

  if (shouldRefreshRuntimeCache({
    browsersRoot: cachedBrowsersRoot,
    cachedWorkerRoot,
    manifestPath: runtimeCacheManifestPath,
    playwrightVersion,
  })) {
    installRuntimeCache({
      browsersRoot: cachedBrowsersRoot,
      cachedWorkerRoot,
      cachedWorkerSourceRoot,
      manifestPath: runtimeCacheManifestPath,
      playwrightVersion,
      sourceWorkerEntry,
    });
  }

  rmSync(runtimeRoot, { recursive: true, force: true });
  mkdirSync(packagedNodeRoot, { recursive: true });
  mkdirSync(packagedWorkerSourceRoot, { recursive: true });
  cpSync(sourceNodeExecutable, resolve(packagedNodeRoot, process.platform === "win32" ? "node.exe" : "node"));
  cpSync(sourceWorkerEntry, resolve(packagedWorkerSourceRoot, "index.js"));
  writeRuntimeWorkerPackage(resolve(packagedWorkerRoot, "package.json"), playwrightVersion);
  cpSync(resolve(cachedWorkerRoot, "node_modules"), resolve(packagedWorkerRoot, "node_modules"), { recursive: true });
  cpSync(cachedBrowsersRoot, browsersRoot, { recursive: true });

  return runtimeRoot;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const runtimeRoot = preparePlaywrightRuntime();
  console.log(`Prepared Playwright runtime at ${runtimeRoot}`);
}
