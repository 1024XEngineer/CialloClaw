/* global process, console */
import { spawnSync } from "node:child_process";
import { copyFileSync, cpSync, existsSync, mkdirSync, readdirSync, rmSync, statSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

function run(command, args, options) {
  const result = spawnSync(command, args, {
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
  throw new Error(`${command} ${args.join(" ")} failed: ${details}`);
}

function resolveRustTargetTriple(repoRoot) {
  const requestedTarget = process.env.TAURI_ENV_TARGET_TRIPLE
    || process.env.CARGO_BUILD_TARGET
    || process.env.TARGET;

  if (requestedTarget) {
    return requestedTarget;
  }

  const result = run("rustc", ["-vV"], { cwd: repoRoot });
  const hostLine = result.stdout
    .split(/\r?\n/)
    .find((line) => line.startsWith("host: "));

  if (!hostLine) {
    throw new Error("Failed to determine Rust target triple from `rustc -vV`.");
  }

  return hostLine.slice("host: ".length).trim();
}

function resolveGoPlatform(targetTriple) {
  const normalizedTriple = targetTriple.toLowerCase();

  const goos = normalizedTriple.includes("windows")
    ? "windows"
    : normalizedTriple.includes("darwin") || normalizedTriple.includes("apple")
      ? "darwin"
      : normalizedTriple.includes("linux")
        ? "linux"
        : null;

  const goarch = normalizedTriple.startsWith("x86_64") || normalizedTriple.startsWith("amd64")
    ? "amd64"
    : normalizedTriple.startsWith("aarch64") || normalizedTriple.startsWith("arm64")
      ? "arm64"
      : normalizedTriple.startsWith("i686") || normalizedTriple.startsWith("i586") || normalizedTriple.startsWith("i386")
        ? "386"
        : normalizedTriple.startsWith("armv7") || normalizedTriple.startsWith("arm")
          ? "arm"
          : null;

  if (!goos || !goarch) {
    throw new Error(`Unsupported target triple for local-service sidecar: ${targetTriple}`);
  }

  return { goos, goarch };
}

export function buildLocalServiceSidecar() {
  const repoRoot = resolve(currentDirectory, "..", "..", "..", "..");
  const srcTauriRoot = resolve(currentDirectory, "..");
  const targetTriple = resolveRustTargetTriple(repoRoot);
  const { goarch, goos } = resolveGoPlatform(targetTriple);
  const sidecarDirectory = resolve(srcTauriRoot, "bin");
  // Keep the dev sidecar path aligned with `externalBin` so `tauri dev`
  // and packaged builds resolve the same executable name.
  const sidecarFileName = `cialloclaw-service-${targetTriple}${targetTriple.includes("windows") ? ".exe" : ""}`;
  const sidecarPath = resolve(sidecarDirectory, sidecarFileName);

  mkdirSync(sidecarDirectory, { recursive: true });
  run("go", ["build", "-trimpath", "-o", sidecarPath, "./services/local-service/cmd/server"], {
    cwd: repoRoot,
    env: {
      ...process.env,
      GOARCH: goarch,
      GOOS: goos,
    },
  });

  return sidecarPath;
}

function assertDirectoryExists(directoryPath, label) {
  if (!existsSync(directoryPath)) {
    throw new Error(`Missing ${label}: ${directoryPath}`);
  }
  if (!statSync(directoryPath).isDirectory()) {
    throw new Error(`${label} is not a directory: ${directoryPath}`);
  }
}

function assertFileExists(filePath, label) {
  if (!existsSync(filePath)) {
    throw new Error(`Missing ${label}: ${filePath}`);
  }
  if (!statSync(filePath).isFile()) {
    throw new Error(`${label} is not a file: ${filePath}`);
  }
}

function isDirectoryPath(candidatePath) {
  return existsSync(candidatePath) && statSync(candidatePath).isDirectory();
}

function isFilePath(candidatePath) {
  return existsSync(candidatePath) && statSync(candidatePath).isFile();
}

function copyRuntimePath(sourcePath, targetPath, label) {
  if (statSync(sourcePath).isDirectory()) {
    cpSync(sourcePath, targetPath, { recursive: true, force: true, dereference: true });
    return;
  }
  throw new Error(`${label} must be a directory: ${sourcePath}`);
}

function removeRuntimePathIfPresent(targetPath) {
  if (!existsSync(targetPath)) {
    return;
  }
  rmSync(targetPath, { recursive: true, force: true });
}

function resolveFirstExistingDirectory(candidates, label) {
  for (const candidate of candidates) {
    if (isDirectoryPath(candidate)) {
      return candidate;
    }
  }

  throw new Error(`Missing ${label}: ${candidates.join(", ")}`);
}

function stageBundledNodeRuntime(stagingRoot, runtimeSourceRoot) {
  const runtimeNodeDir = resolve(runtimeSourceRoot, "node");
  const targetNodeDir = resolve(stagingRoot, "node");
  removeRuntimePathIfPresent(targetNodeDir);
  mkdirSync(targetNodeDir, { recursive: true });

  if (isDirectoryPath(runtimeNodeDir)) {
    copyRuntimePath(runtimeNodeDir, targetNodeDir, "bundled Node runtime source");
    return targetNodeDir;
  }

  const currentNodeExecutable = process.execPath;
  if (!isFilePath(currentNodeExecutable)) {
    throw new Error(`Missing bundled Node runtime source: ${runtimeNodeDir}`);
  }

  copyFileSync(currentNodeExecutable, resolve(targetNodeDir, process.platform === "win32" ? "node.exe" : "node"));
  return targetNodeDir;
}

function resolveFallbackPlaywrightBrowserDirectory() {
  const candidates = [
    process.env.PLAYWRIGHT_BROWSERS_PATH,
    process.env.LOCALAPPDATA ? resolve(process.env.LOCALAPPDATA, "ms-playwright") : null,
    process.env.HOME ? resolve(process.env.HOME, ".cache", "ms-playwright") : null,
  ].filter(Boolean);

  for (const candidate of candidates) {
    if (isDirectoryPath(candidate)) {
      return candidate;
    }
  }

  return null;
}

function resolvePnpmHoistedPackageSource(nodeModulesPath, packageName) {
  const virtualStoreRoot = resolve(nodeModulesPath, ".pnpm");
  if (!existsSync(virtualStoreRoot) || !statSync(virtualStoreRoot).isDirectory()) {
    throw new Error(`Missing pnpm virtual store for ${packageName}: ${virtualStoreRoot}`);
  }

  for (const entryName of readdirSync(virtualStoreRoot)) {
    const candidate = resolve(virtualStoreRoot, entryName, "node_modules", packageName);
    if (existsSync(candidate) && statSync(candidate).isDirectory()) {
      return candidate;
    }
  }

  throw new Error(`Missing hoisted package ${packageName} in ${virtualStoreRoot}`);
}

function resolvePackageSourceFromNodeModulesRoots(nodeModulesRoots, packageName) {
  for (const nodeModulesRoot of nodeModulesRoots) {
    if (!isDirectoryPath(nodeModulesRoot)) {
      continue;
    }

    const directPackagePath = resolve(nodeModulesRoot, packageName);
    if (isDirectoryPath(directPackagePath)) {
      return directPackagePath;
    }

    try {
      return resolvePnpmHoistedPackageSource(nodeModulesRoot, packageName);
    } catch {
      // Fall through to the next candidate root.
    }
  }

  throw new Error(`Missing runtime package ${packageName} in ${nodeModulesRoots.join(", ")}`);
}

function installPlaywrightBrowserRuntime(targetBrowserRoot, playwrightPackageSource, repoRoot) {
  mkdirSync(targetBrowserRoot, { recursive: true });
  run(process.execPath, [resolve(playwrightPackageSource, "cli.js"), "install", "chromium"], {
    cwd: repoRoot,
    env: {
      ...process.env,
      PLAYWRIGHT_BROWSERS_PATH: targetBrowserRoot,
      PLAYWRIGHT_SKIP_BROWSER_GC: "1",
    },
  });
}

function preparePlaywrightBrowserRuntime(stagingRoot, runtimeSourceRoot, playwrightPackageSource, repoRoot) {
  const targetBrowserRoot = resolve(stagingRoot, "ms-playwright");
  removeRuntimePathIfPresent(targetBrowserRoot);

  const runtimeBrowserDir = resolve(runtimeSourceRoot, "ms-playwright");
  if (isDirectoryPath(runtimeBrowserDir)) {
    copyRuntimePath(runtimeBrowserDir, targetBrowserRoot, "bundled Playwright browser source");
    return targetBrowserRoot;
  }

  const cachedBrowserDir = resolveFallbackPlaywrightBrowserDirectory();
  if (cachedBrowserDir !== null) {
    copyRuntimePath(cachedBrowserDir, targetBrowserRoot, "bundled Playwright browser source");
    return targetBrowserRoot;
  }

  installPlaywrightBrowserRuntime(targetBrowserRoot, playwrightPackageSource, repoRoot);
  return targetBrowserRoot;
}

export function preparePlaywrightBundleRuntime() {
  const repoRoot = resolve(currentDirectory, "..", "..", "..", "..");
  const srcTauriRoot = resolve(currentDirectory, "..");
  const runtimeSourceRoot = resolve(srcTauriRoot, "runtime");
  const stagingRoot = resolve(srcTauriRoot, "generated-resources", "playwright-runtime");
  const workerSourceRoot = resolve(repoRoot, "workers", "playwright-worker");
  const bundledWorkerRuntimeRoot = resolve(runtimeSourceRoot, "workers", "playwright-worker");

  const workerNodeModulesRoots = [
    resolve(bundledWorkerRuntimeRoot, "node_modules"),
    resolve(workerSourceRoot, "node_modules"),
    resolve(repoRoot, "node_modules"),
  ];
  const workerScriptSource = resolve(workerSourceRoot, "src");
  const workerPackageSource = resolve(workerSourceRoot, "package.json");
  const playwrightPackageSource = resolvePackageSourceFromNodeModulesRoots(workerNodeModulesRoots, "playwright");
  const playwrightCorePackageSource = resolvePackageSourceFromNodeModulesRoots(workerNodeModulesRoots, "playwright-core");

  assertDirectoryExists(workerScriptSource, "Playwright worker source");
  assertFileExists(workerPackageSource, "Playwright worker package.json");

  mkdirSync(stagingRoot, { recursive: true });
  removeRuntimePathIfPresent(resolve(stagingRoot, "workers"));

  stageBundledNodeRuntime(stagingRoot, runtimeSourceRoot);
  preparePlaywrightBrowserRuntime(stagingRoot, runtimeSourceRoot, playwrightPackageSource, repoRoot);

  const stagedWorkerRoot = resolve(stagingRoot, "workers", "playwright-worker");
  mkdirSync(stagedWorkerRoot, { recursive: true });
  const stagedWorkerNodeModulesRoot = resolve(stagedWorkerRoot, "node_modules");
  mkdirSync(stagedWorkerNodeModulesRoot, { recursive: true });
  copyRuntimePath(playwrightPackageSource, resolve(stagedWorkerNodeModulesRoot, "playwright"), "bundled Playwright runtime package");
  copyRuntimePath(playwrightCorePackageSource, resolve(stagedWorkerNodeModulesRoot, "playwright-core"), "bundled Playwright core package");
  copyRuntimePath(workerScriptSource, resolve(stagedWorkerRoot, "src"), "Playwright worker source");
  cpSync(workerPackageSource, resolve(stagedWorkerRoot, "package.json"), { force: true });

  return stagingRoot;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const sidecarPath = buildLocalServiceSidecar();
  console.log(`Built local-service sidecar at ${sidecarPath}`);
}
