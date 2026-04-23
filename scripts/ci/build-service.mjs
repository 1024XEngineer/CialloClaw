import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const outputPath = path.join(
  repoRoot,
  "apps",
  "desktop",
  "src-tauri",
  "bin",
  "cialloclaw-service-x86_64-pc-windows-msvc.exe",
);

mkdirSync(path.dirname(outputPath), { recursive: true });

const buildResult = spawnSync(
  "go",
  [
    "build",
    "-trimpath",
    "-o",
    outputPath,
    "./services/local-service/cmd/server",
  ],
  {
    cwd: repoRoot,
    stdio: "inherit",
    env: {
      ...process.env,
      GOOS: "windows",
      GOARCH: "amd64",
    },
  },
);

if (buildResult.error) {
  throw buildResult.error;
}

if (typeof buildResult.status === "number" && buildResult.status !== 0) {
  process.exit(buildResult.status);
}
