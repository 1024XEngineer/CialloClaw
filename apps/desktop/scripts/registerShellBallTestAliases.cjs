/* global process, require */
/* eslint-disable @typescript-eslint/no-require-imports */

const { existsSync } = require("node:fs");
const { dirname, resolve } = require("node:path");
const Module = require("node:module");

const desktopRoot = process.cwd();
const cacheRoot = resolve(desktopRoot, ".cache/shell-ball-tests");
const sourceRoot = resolve(desktopRoot, "src");
const originalResolveFilename = Module._resolveFilename;
const stubAliasMap = {
  "@/rpc/client": resolve(cacheRoot, "features/shell-ball/test-stubs/rpcClient.js"),
  "@/rpc/fallback": resolve(cacheRoot, "features/shell-ball/test-stubs/rpcFallback.js"),
  "@/rpc/methods": resolve(cacheRoot, "features/shell-ball/test-stubs/rpcMethods.js"),
  "@/rpc/subscriptions": resolve(cacheRoot, "features/shell-ball/test-stubs/rpcSubscriptions.js"),
  "@/features/dashboard/tasks/TasksPage": resolve(cacheRoot, "features/shell-ball/test-stubs/dashboardTasksPage.js"),
  "@/features/dashboard/notes/NotesPage": resolve(cacheRoot, "features/shell-ball/test-stubs/dashboardNotesPage.js"),
  "@/features/dashboard/memory/MemoryPage": resolve(cacheRoot, "features/shell-ball/test-stubs/dashboardMemoryPage.js"),
};

function resolveCacheModule(modulePath) {
  const emittedBasePath = resolve(cacheRoot, modulePath);
  const emittedCandidates = [`${emittedBasePath}.js`, resolve(emittedBasePath, "index.js")];

  for (const candidate of emittedCandidates) {
    if (existsSync(candidate)) {
      return candidate;
    }
  }

  return null;
}

function resolveSourceAsset(assetPath) {
  return resolve(sourceRoot, assetPath);
}

function resolveSiblingAsset(request, parentFilename) {
  if (typeof parentFilename !== "string" || !parentFilename.startsWith(cacheRoot)) {
    return null;
  }

  const compiledAssetPath = resolve(dirname(parentFilename), request);
  const relativeAssetPath = compiledAssetPath.slice(cacheRoot.length + 1);
  const sourceAssetPath = resolve(sourceRoot, relativeAssetPath);

  return existsSync(sourceAssetPath) ? sourceAssetPath : null;
}

require.extensions[".css"] = (module) => {
  module.exports = "";
};

require.extensions[".png"] = (module, filename) => {
  module.exports = filename;
};

Module._resolveFilename = function resolveShellBallTestAlias(request, parent, isMain, options) {
  const stubAlias = stubAliasMap[request];

  if (typeof stubAlias === "string" && existsSync(stubAlias)) {
    return stubAlias;
  }

  if (request.startsWith("@/")) {
    const modulePath = request.slice(2);

    if (modulePath.endsWith(".css") || modulePath.endsWith(".png")) {
      return resolveSourceAsset(modulePath);
    }

    const resolvedModule = resolveCacheModule(modulePath);

    if (resolvedModule !== null) {
      return resolvedModule;
    }
  }

  if (request === "@cialloclaw/protocol") {
    return resolve(cacheRoot, "features/shell-ball/test-stubs/protocol.js");
  }

  if (request === "@cialloclaw/ui") {
    return resolve(cacheRoot, "features/shell-ball/test-stubs/ui.js");
  }

  if (request.endsWith(".css") || request.endsWith(".png")) {
    const resolvedAsset = resolveSiblingAsset(request, parent?.filename);

    if (resolvedAsset !== null) {
      return resolvedAsset;
    }
  }

  return originalResolveFilename.call(this, request, parent, isMain, options);
};
