/* global process, console */
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync, statSync } from "node:fs";
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

function copyRuntimePath(sourcePath, targetPath, label) {
  if (statSync(sourcePath).isDirectory()) {
    cpSync(sourcePath, targetPath, { recursive: true, force: true });
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

export function preparePlaywrightBundleRuntime() {
  const repoRoot = resolve(currentDirectory, "..", "..", "..", "..");
  const srcTauriRoot = resolve(currentDirectory, "..");
  const runtimeSourceRoot = resolve(srcTauriRoot, "runtime");
  const stagingRoot = resolve(srcTauriRoot, "resources", "playwright-runtime");
  const workerSourceRoot = resolve(repoRoot, "workers", "playwright-worker");
  const bundledWorkerRuntimeRoot = resolve(runtimeSourceRoot, "workers", "playwright-worker");

  const nodeRuntimeSource = resolve(runtimeSourceRoot, "node");
  const browserRuntimeSource = resolve(runtimeSourceRoot, "ms-playwright");
  const workerModulesSource = resolve(bundledWorkerRuntimeRoot, "node_modules");
  const workerScriptSource = resolve(workerSourceRoot, "src");
  const workerPackageSource = resolve(workerSourceRoot, "package.json");

  assertDirectoryExists(nodeRuntimeSource, "bundled Node runtime source");
  assertDirectoryExists(browserRuntimeSource, "bundled Playwright browser source");
  assertDirectoryExists(workerModulesSource, "bundled Playwright worker node_modules source");
  assertDirectoryExists(workerScriptSource, "Playwright worker source");
  assertFileExists(workerPackageSource, "Playwright worker package.json");

  mkdirSync(stagingRoot, { recursive: true });
  removeRuntimePathIfPresent(resolve(stagingRoot, "node"));
  removeRuntimePathIfPresent(resolve(stagingRoot, "ms-playwright"));
  removeRuntimePathIfPresent(resolve(stagingRoot, "workers"));

  copyRuntimePath(nodeRuntimeSource, resolve(stagingRoot, "node"), "bundled Node runtime source");
  copyRuntimePath(browserRuntimeSource, resolve(stagingRoot, "ms-playwright"), "bundled Playwright browser source");

  const stagedWorkerRoot = resolve(stagingRoot, "workers", "playwright-worker");
  mkdirSync(stagedWorkerRoot, { recursive: true });
  copyRuntimePath(workerModulesSource, resolve(stagedWorkerRoot, "node_modules"), "bundled Playwright worker node_modules source");
  copyRuntimePath(workerScriptSource, resolve(stagedWorkerRoot, "src"), "Playwright worker source");
  cpSync(workerPackageSource, resolve(stagedWorkerRoot, "package.json"), { force: true });

  const stagedWorkerNodeModulesRoot = resolve(stagedWorkerRoot, "node_modules");
  const stagedPlaywrightCoreRoot = resolve(stagedWorkerNodeModulesRoot, "playwright-core");
  if (!existsSync(stagedPlaywrightCoreRoot)) {
    copyRuntimePath(
      resolvePnpmHoistedPackageSource(workerModulesSource, "playwright-core"),
      stagedPlaywrightCoreRoot,
      "bundled Playwright worker hoisted playwright-core package",
    );
  }

  return stagingRoot;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const sidecarPath = buildLocalServiceSidecar();
  console.log(`Built local-service sidecar at ${sidecarPath}`);
}
