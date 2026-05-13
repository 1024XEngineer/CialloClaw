/* global process, console */
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, unlinkSync } from "node:fs";
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

function killWindowsProcess(imageName) {
  const result = spawnSync("taskkill", ["/F", "/T", "/IM", imageName], {
    stdio: "pipe",
    encoding: "utf8",
  });

  // Windows reports a non-zero exit code when no matching process exists. That
  // is a clean state for this dev-time cleanup path, so only surface output
  // when callers still cannot delete the locked executable after retrying.
  return {
    details: result.stderr?.trim() || result.stdout?.trim() || "",
  };
}

function unlinkIfPresent(filePath) {
  if (!existsSync(filePath)) {
    return false;
  }

  unlinkSync(filePath);
  return true;
}

function removeCopiedSidecarTarget(srcTauriRoot) {
  const targetSidecarPaths = [
    resolve(srcTauriRoot, "target", "debug", "cialloclaw-service.exe"),
    resolve(srcTauriRoot, "target", "release", "cialloclaw-service.exe"),
  ];

  for (const targetSidecarPath of targetSidecarPaths) {
    try {
      unlinkIfPresent(targetSidecarPath);
    } catch (error) {
      if (process.platform !== "win32" || !["EACCES", "EPERM"].includes(error?.code)) {
        throw error;
      }

      const killResult = killWindowsProcess("cialloclaw-service.exe");

      try {
        unlinkIfPresent(targetSidecarPath);
      } catch (retryError) {
        const killDetails = killResult.details ? ` taskkill output: ${killResult.details}` : "";
        throw new Error(
          `Failed to remove stale Tauri sidecar target at ${targetSidecarPath}.`
            + ` Close any running CialloClaw dev sessions and retry.${killDetails}`,
          { cause: retryError },
        );
      }
    }
  }
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
  // Tauri copies `externalBin` into target/{debug,release} without the target
  // triple. Removing stale copies first avoids Windows file-lock panics when a
  // previous dev sidecar is still holding target/debug/cialloclaw-service.exe.
  removeCopiedSidecarTarget(srcTauriRoot);
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

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const sidecarPath = buildLocalServiceSidecar();
  console.log(`Built local-service sidecar at ${sidecarPath}`);
}
