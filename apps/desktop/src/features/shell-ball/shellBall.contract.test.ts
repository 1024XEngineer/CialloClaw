import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";
import { getShellBallDemoViewModel } from "./shellBall.demo";
import {
  createShellBallInteractionController,
  getShellBallGestureAxisIntent,
  getShellBallInputBarMode,
  getShellBallProcessingReturnState,
  shouldPreviewShellBallVoiceGesture,
  getShellBallVoicePreview,
  resolveShellBallTransition,
  shouldRetainShellBallHoverInput,
  SHELL_BALL_CANCEL_DELTA_PX,
  SHELL_BALL_CONFIRMING_MS,
  SHELL_BALL_HOVER_INTENT_MS,
  SHELL_BALL_LEAVE_GRACE_MS,
  SHELL_BALL_LOCK_DELTA_PX,
  SHELL_BALL_LONG_PRESS_MS,
  SHELL_BALL_PROCESSING_MS,
  SHELL_BALL_VERTICAL_PRIORITY_RATIO,
  SHELL_BALL_WAITING_AUTH_MS,
} from "./shellBall.interaction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { collectShellBallSpeechTranscript, composeShellBallSpeechDraft } from "./shellBall.speech";
import { ShellBallApp, shouldArmShellBallTextDropTarget, shouldShowShellBallFileDropOverlay } from "./ShellBallApp";
import { ShellBallBubbleWindow } from "./ShellBallBubbleWindow";
import { ShellBallDevLayer } from "./ShellBallDevLayer";
import { ShellBallInputWindow } from "./ShellBallInputWindow";
import { ShellBallMascot } from "./components/ShellBallMascot";
import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";
import { getShellBallMascotHotspotGestureAction } from "./components/ShellBallMascot";
import { getShellBallMascotPointerPhaseAction } from "./components/ShellBallMascot";
import { shouldStartShellBallMascotWindowDrag } from "./components/ShellBallMascot";
import { extractShellBallDroppedText, resolveShellBallTextDropEffect, ShellBallSurface, shouldAcceptShellBallTextDrop } from "./ShellBallSurface";
import { shouldShowShellBallDemoSwitcher } from "./shellBall.dev";
import { shellBallWindowLabels, shellBallWindowPermissions } from "../../platform/shellBallWindowController";
import { ShellBallInputBar } from "./components/ShellBallInputBar";
import type { ShellBallTransitionResult } from "./shellBall.types";
import { shellBallVisualStates } from "./shellBall.types";
import {
  dashboardSafetyRoutePath,
  resolveDashboardModuleRoutePath,
  dashboardRoutePaths,
  resolveDashboardRouteHref,
  resolveDashboardRoutePath,
} from "../dashboard/shared/dashboardRouteTargets";
import {
  getShellBallBubbleRegionState,
  createShellBallWindowSnapshot,
  getShellBallHelperWindowVisibility,
  getShellBallVisibleBubbleItems,
  shellBallWindowSyncEvents,
} from "./shellBall.windowSync";
import type { ShellBallBubbleItem } from "./shellBall.bubble";
import { cloneShellBallBubbleItems } from "./shellBall.bubble";
import {
  SHELL_BALL_BUBBLE_GAP_PX,
  SHELL_BALL_INPUT_GAP_PX,
  SHELL_BALL_WINDOW_SAFE_MARGIN_PX,
  clampShellBallFrameToBounds,
  createShellBallWindowFrame,
  getShellBallHelperWindowInteractionMode,
  getShellBallBubbleAnchor,
  getShellBallInputAnchor,
  measureShellBallContentSize,
} from "./useShellBallWindowMetrics";
import { applyShellBallBubbleAction, createShellBallAgentBubbleItem } from "./useShellBallCoordinator";
import {
  appendShellBallDroppedText,
  createShellBallInputSubmitParams,
  getShellBallPostSubmitInputReset,
  getShellBallDashboardOpenGesturePolicy,
  getShellBallVoiceRecognitionUnexpectedEndFallbackState,
  getShellBallPressCancelEvent,
  resolveShellBallVoiceRecognitionFinalState,
  getShellBallVoicePreviewFromEvent,
  mapShellBallInteractionConsumedEventToFlag,
  shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd,
  shouldKeepShellBallVoicePreviewOnRegionLeave,
  syncShellBallInteractionController,
  useShellBallInteraction,
} from "./useShellBallInteraction";
import { useShellBallStore } from "../../stores/shellBallStore";

const desktopRoot = process.cwd();

function withDashboardRouteRuntime<T>(callback: (components: { DashboardRoot: unknown }) => T) {
  const NodeModule = require("node:module") as any;
  const createRequire = NodeModule.createRequire as (filename: string) => NodeRequire;
  const originalResolveFilename = NodeModule._resolveFilename;
  const originalLoad = NodeModule._load as undefined | ((request: string, parent: unknown, isMain: boolean) => unknown);
  const originalCssLoader = require.extensions[".css"];
  const originalPngLoader = require.extensions[".png"];

  require.extensions[".css"] = (module) => {
    module.exports = "";
  };

  require.extensions[".png"] = (module, filename) => {
    module.exports = filename;
  };

  NodeModule._resolveFilename = function resolveDashboardAlias(
    request: string,
    parent: unknown,
    isMain: boolean,
    options?: unknown,
  ) {
    if (request.startsWith("@/")) {
      const modulePath = request.slice(2);

      if (modulePath.endsWith(".css") || modulePath.endsWith(".png")) {
        return resolve(desktopRoot, "src", modulePath);
      }

      const emittedBasePath = resolve(desktopRoot, ".cache/shell-ball-tests", modulePath);
      const emittedCandidates = [`${emittedBasePath}.js`, resolve(emittedBasePath, "index.js")];

      for (const candidate of emittedCandidates) {
        if (existsSync(candidate)) {
          return candidate;
        }
      }
    }

    return originalResolveFilename.call(this, request, parent, isMain, options);
  };

  NodeModule._load = function loadDashboardRuntime(request: string, parent: unknown, isMain: boolean) {
    if (request === "./DashboardHome") {
      return require(resolve(desktopRoot, ".cache/shell-ball-tests/app/dashboard/DashboardHome.js"));
    }

    if (request === "./SecurityPageShell" || request.endsWith("/SecurityPageShell")) {
      return {
        SecurityPageShell() {
          return createElement("div", null, "security-shell-stub");
        },
      };
    }

    if (
      request === "@/features/dashboard/tasks/TasksPage" ||
      request === "@/features/dashboard/notes/NotesPage" ||
      request === "@/features/dashboard/memory/MemoryPage"
    ) {
      return {
        TasksPage() {
          return createElement("div", null, "tasks-page-stub");
        },
        NotesPage() {
          return createElement("div", null, "notes-page-stub");
        },
        MemoryPage() {
          return createElement("div", null, "memory-page-stub");
        },
      };
    }

    return originalLoad?.(request, parent, isMain);
  } as typeof NodeModule._load;

  try {
    const dashboardRootPath = resolve(desktopRoot, "src/app/dashboard/DashboardRoot.tsx");
    const dashboardRootModule = { exports: {} as Record<string, unknown> };
    const transpiledDashboardRoot = ts.transpileModule(readFileSync(dashboardRootPath, "utf8"), {
      compilerOptions: {
        jsx: ts.JsxEmit.ReactJSX,
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2020,
        esModuleInterop: true,
      },
      fileName: dashboardRootPath,
    });
    const moduleFactory = new Function("require", "module", "exports", transpiledDashboardRoot.outputText) as (
      require: NodeRequire,
      module: { exports: Record<string, unknown> },
      exports: Record<string, unknown>,
    ) => void;
    moduleFactory(createRequire(dashboardRootPath), dashboardRootModule, dashboardRootModule.exports);
    const { DashboardRoot } = dashboardRootModule.exports as { DashboardRoot: unknown };

    return callback({ DashboardRoot });
  } finally {
    NodeModule._resolveFilename = originalResolveFilename;
    if (originalLoad === undefined) {
      Reflect.deleteProperty(NodeModule, "_load");
    } else {
      NodeModule._load = originalLoad as typeof NodeModule._load;
    }

    if (originalCssLoader === undefined) {
      Reflect.deleteProperty(require.extensions, ".css");
    } else {
      require.extensions[".css"] = originalCssLoader;
    }

    if (originalPngLoader === undefined) {
      Reflect.deleteProperty(require.extensions, ".png");
    } else {
      require.extensions[".png"] = originalPngLoader;
    }
  }
}

function withWindowControllerRuntime<T>(runtime: {
  getByLabel: (label: string) => Promise<unknown> | unknown;
  createWindow?: (label: string, options: Record<string, unknown>) => unknown;
}, callback: (mod: {
  openOrFocusDesktopWindow: (label: "dashboard" | "control-panel") => Promise<string>;
}) => Promise<T> | T) {
  const NodeModule = require("node:module") as any;
  const originalLoad = NodeModule._load;
  const modulePath = resolve(desktopRoot, ".cache/shell-ball-tests/platform/windowController.js");

  delete require.cache[modulePath];

  NodeModule._load = function loadWindowController(request: string, parent: unknown, isMain: boolean) {
    if (request === "@tauri-apps/api/window") {
      function FakeWindow(this: unknown, label: string, options: Record<string, unknown>) {
        return runtime.createWindow?.(label, options);
      }

      FakeWindow.getByLabel = runtime.getByLabel;

      return {
        Window: FakeWindow,
      };
    }

    if (request === "./dashboardWindowTransition") {
      return {
        requestShellBallDashboardOpenTransition() {
          return Promise.resolve(true);
        },
      };
    }

    return originalLoad(request, parent, isMain);
  };

  const loaded = require(modulePath) as {
    openOrFocusDesktopWindow: (label: "dashboard" | "control-panel") => Promise<string>;
  };

  const finalize = () => {
    NodeModule._load = originalLoad;
    delete require.cache[modulePath];
  };

  try {
    return Promise.resolve(callback(loaded)).finally(finalize);
  } catch (error) {
    finalize();
    throw error;
  }
}

function withHideOnCloseRequestRuntime<T>(
  currentWindow: {
    __calls__?: string[];
    label?: string;
    hide: () => Promise<void> | void;
    onCloseRequested: (handler: (event: { preventDefault: () => void }) => Promise<void> | void) => unknown;
  },
  callback: (mod: {
    installHideOnCloseRequest: () => unknown;
  }) => Promise<T> | T,
) {
  const NodeModule = require("node:module") as any;
  const originalLoad = NodeModule._load;
  const modulePath = resolve(desktopRoot, "src/platform/hideOnCloseRequest.ts");
  const source = readFileSync(modulePath, "utf8");

  NodeModule._load = function loadHideOnCloseRequest(request: string, parent: unknown, isMain: boolean) {
    if (request === "@tauri-apps/api/window") {
      return {
        getCurrentWindow() {
          return currentWindow;
        },
      };
    }

    if (request === "./dashboardWindowTransition") {
      return {
        requestShellBallDashboardCloseTransition() {
          (currentWindow as { __calls__?: string[] }).__calls__?.push("requestShellBallDashboardCloseTransition");
          return Promise.resolve(true);
        },
      };
    }

    return originalLoad(request, parent, isMain);
  };

  const transpiledModule = { exports: {} as Record<string, unknown> };
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2020,
      esModuleInterop: true,
    },
    fileName: modulePath,
  });
  const moduleFactory = new Function("require", "module", "exports", transpiled.outputText) as (
    require: NodeRequire,
    module: { exports: Record<string, unknown> },
    exports: Record<string, unknown>,
  ) => void;

  moduleFactory(require, transpiledModule, transpiledModule.exports);

  const finalize = () => {
    NodeModule._load = originalLoad;
  };

  try {
    return Promise.resolve(callback(transpiledModule.exports as { installHideOnCloseRequest: () => unknown })).finally(finalize);
  } catch (error) {
    finalize();
    throw error;
  }
}

function withDesktopAliasRuntime<T>(callback: () => T) {
  const NodeModule = require("node:module") as any;
  const originalResolveFilename = NodeModule._resolveFilename;
  const originalCssLoader = require.extensions[".css"];
  const originalPngLoader = require.extensions[".png"];

  require.extensions[".css"] = (module) => {
    module.exports = "";
  };

  require.extensions[".png"] = (module, filename) => {
    module.exports = filename;
  };

  NodeModule._resolveFilename = function resolveDesktopAlias(
    request: string,
    parent: unknown,
    isMain: boolean,
    options?: unknown,
  ) {
    if (request.startsWith("@/")) {
      const modulePath = request.slice(2);

      if (modulePath.endsWith(".css") || modulePath.endsWith(".png")) {
        return resolve(desktopRoot, "src", modulePath);
      }

      const emittedBasePath = resolve(desktopRoot, ".cache/shell-ball-tests", modulePath);
      const emittedCandidates = [`${emittedBasePath}.js`, resolve(emittedBasePath, "index.js")];

      for (const candidate of emittedCandidates) {
        if (existsSync(candidate)) {
          return candidate;
        }
      }
    }

    if (request === "@cialloclaw/ui") {
      return resolve(desktopRoot, ".cache/shell-ball-tests/features/shell-ball/test-stubs/ui.js");
    }

    if (request === "@cialloclaw/protocol") {
      return resolve(desktopRoot, ".cache/shell-ball-tests/features/shell-ball/test-stubs/protocol.js");
    }

    return originalResolveFilename.call(this, request, parent, isMain, options);
  };

  try {
    return callback();
  } finally {
    NodeModule._resolveFilename = originalResolveFilename;

    if (originalCssLoader === undefined) {
      Reflect.deleteProperty(require.extensions, ".css");
    } else {
      require.extensions[".css"] = originalCssLoader;
    }

    if (originalPngLoader === undefined) {
      Reflect.deleteProperty(require.extensions, ".png");
    } else {
      require.extensions[".png"] = originalPngLoader;
    }
  }
}

function withShellBallModuleRuntime<T>(
  moduleRelativePath: string,
  mocks: Record<string, unknown>,
  callback: (moduleExports: Record<string, unknown>) => T,
) {
  const NodeModule = require("node:module") as any;
  const originalLoad = NodeModule._load;
  const modulePath = resolve(desktopRoot, "src/features/shell-ball", moduleRelativePath);
  const source = readFileSync(modulePath, "utf8");
  const transpiledModule = { exports: {} as Record<string, unknown> };
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      jsx: ts.JsxEmit.ReactJSX,
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2020,
      esModuleInterop: true,
    },
    fileName: modulePath,
  });
  const moduleFactory = new Function("require", "module", "exports", transpiled.outputText) as (
    require: NodeRequire,
    module: { exports: Record<string, unknown> },
    exports: Record<string, unknown>,
  ) => void;

  NodeModule._load = function loadShellBallModule(request: string, parent: unknown, isMain: boolean) {
    if (request in mocks) {
      return mocks[request];
    }

    return originalLoad(request, parent, isMain);
  };

  try {
    moduleFactory(require, transpiledModule, transpiledModule.exports);
    return callback(transpiledModule.exports);
  } finally {
    NodeModule._load = originalLoad;
  }
}

function withTrayControllerRuntime<T>(
  openOrFocusDesktopWindow: (label: "dashboard" | "control-panel") => Promise<string>,
  callback: (mod: { openControlPanelFromTray: () => Promise<string>; calls: Array<"dashboard" | "control-panel"> }) => Promise<T> | T,
) {
  const NodeModule = require("node:module") as any;
  const originalLoad = NodeModule._load;
  const modulePath = resolve(desktopRoot, ".cache/shell-ball-tests/platform/trayController.js");
  const calls: Array<"dashboard" | "control-panel"> = [];

  delete require.cache[modulePath];

  NodeModule._load = function loadTrayController(request: string, parent: unknown, isMain: boolean) {
    if (request === "@/platform/windowController") {
      return {
        openOrFocusDesktopWindow(label: "dashboard" | "control-panel") {
          calls.push(label);
          return openOrFocusDesktopWindow(label);
        },
      };
    }

    return originalLoad(request, parent, isMain);
  };

  const loaded = require(modulePath) as {
    openControlPanelFromTray: () => Promise<string>;
  };

  const finalize = () => {
    NodeModule._load = originalLoad;
    delete require.cache[modulePath];
  };

  try {
    return Promise.resolve(callback({ ...loaded, calls })).finally(finalize);
  } catch (error) {
    finalize();
    throw error;
  }
}

function renderDashboardAppMarkup() {
  return withDesktopAliasRuntime(() => {
    const modulePath = resolve(desktopRoot, ".cache/shell-ball-tests/features/dashboard/DashboardApp.js");

    delete require.cache[modulePath];

    try {
      const { DashboardApp } = require(modulePath) as { DashboardApp: unknown };

      return renderToStaticMarkup(createElement(DashboardApp as never));
    } finally {
      delete require.cache[modulePath];
    }
  });
}

function renderDashboardRouteSurface(hash: string) {
  const originalWindow = globalThis.window;
  const originalDocument = globalThis.document;
  const originalSVGElement = globalThis.SVGElement;
  const fakeDocument = {
    location: null as unknown,
    querySelector() {
      return null;
    },
    defaultView: null as unknown,
  };
  const fakeWindow = {
    location: {
      hash,
      href: `https://desktop.local/dashboard.html${hash}`,
      origin: "https://desktop.local",
      pathname: "/dashboard.html",
      search: "",
    },
    addEventListener() {},
    removeEventListener() {},
    history: {
      state: null,
      replaceState() {},
      pushState() {},
    },
    document: fakeDocument,
  };
  fakeDocument.location = fakeWindow.location;
  fakeDocument.defaultView = fakeWindow;

  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: fakeWindow,
  });

  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: fakeDocument,
  });

  Object.defineProperty(globalThis, "SVGElement", {
    configurable: true,
    value: function SVGElement() {},
  });

  try {
    return withDashboardRouteRuntime(({ DashboardRoot }) => renderToStaticMarkup(createElement(DashboardRoot as never)));
  } finally {
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.defineProperty(globalThis, "window", {
        configurable: true,
        value: originalWindow,
      });
    }

    if (originalDocument === undefined) {
      Reflect.deleteProperty(globalThis, "document");
    } else {
      Object.defineProperty(globalThis, "document", {
        configurable: true,
        value: originalDocument,
      });
    }

    if (originalSVGElement === undefined) {
      Reflect.deleteProperty(globalThis, "SVGElement");
    } else {
      Object.defineProperty(globalThis, "SVGElement", {
        configurable: true,
        value: originalSVGElement,
      });
    }
  }
}

function createFakeScheduler() {
  let nextId = 0;
  const queue = new Map<number, () => void>();

  return {
    schedule(callback: () => void, _ms: number) {
      const handle = ++nextId;
      queue.set(handle, callback);
      return handle;
    },
    cancel(handle: unknown) {
      if (typeof handle === "number") {
        queue.delete(handle);
      }
    },
    flush() {
      const currentHandles = [...queue.keys()];

      for (const handle of currentHandles) {
        const callback = queue.get(handle);
        if (callback === undefined) {
          continue;
        }

        queue.delete(handle);
        callback();
      }
    },
    get size() {
      return queue.size;
    },
  };
}

const validTransitionResult: ShellBallTransitionResult = {
  next: "processing",
  autoAdvanceTo: "idle",
  autoAdvanceMs: 1,
};

assert.equal(validTransitionResult.autoAdvanceTo, "idle");

// @ts-expect-error auto-advance fields must be defined together
const invalidTransitionResultMissingMs: ShellBallTransitionResult = {
  next: "processing",
  autoAdvanceTo: "idle",
};

// @ts-expect-error auto-advance fields must be defined together
const invalidTransitionResultMissingTarget: ShellBallTransitionResult = {
  next: "processing",
  autoAdvanceMs: 1,
};

test("shell-ball demo fixtures preserve the frozen seven-state contract", () => {
  assert.deepEqual(shellBallVisualStates, [
    "idle",
    "hover_input",
    "confirming_intent",
    "processing",
    "waiting_auth",
    "voice_listening",
    "voice_locked",
  ]);

  assert.deepEqual(getShellBallDemoViewModel("idle"), {
    badgeTone: "status",
    badgeLabel: "待机",
    title: "小胖啾正在桌面待命",
    subtitle: "轻量承接入口已就绪",
    helperText: "悬停后可进入输入承接态",
    panelMode: "hidden",
    showRiskBlock: false,
    showVoiceHint: false,
  });

  assert.deepEqual(getShellBallDemoViewModel("waiting_auth"), {
    badgeTone: "waiting_auth",
    badgeLabel: "等待授权",
    title: "此操作需要进一步确认",
    subtitle: "检测到潜在影响范围，正在等待授权",
    helperText: "确认后才会继续执行后续动作",
    panelMode: "full",
    showRiskBlock: true,
    riskTitle: "潜在影响范围",
    riskText: "本次操作可能修改当前工作区内容，需要你明确允许后继续。",
    showVoiceHint: false,
  });

  assert.deepEqual(getShellBallDemoViewModel("voice_locked"), {
    badgeTone: "processing",
    badgeLabel: "持续收音",
    title: "持续收音已锁定",
    subtitle: "语音输入会保持开启直到结束",
    helperText: "说完后可主动结束本次语音输入",
    panelMode: "compact",
    showRiskBlock: false,
    showVoiceHint: true,
    voiceHintText: "持续收音中，结束前不会自动退出。",
  });
});

test("shell-ball desktop host declares bubble and input helper windows", () => {
  assert.equal(existsSync(resolve(desktopRoot, "shell-ball-bubble.html")), true);
  assert.equal(existsSync(resolve(desktopRoot, "shell-ball-input.html")), true);

  const viteConfig = readFileSync(resolve(desktopRoot, "vite.config.ts"), "utf8");
  const tauriConfig = readFileSync(resolve(desktopRoot, "src-tauri/tauri.conf.json"), "utf8");

  assert.match(viteConfig, /"shell-ball-bubble"/);
  assert.match(viteConfig, /"shell-ball-input"/);
  assert.match(tauriConfig, /"label": "shell-ball-bubble"/);
  assert.match(tauriConfig, /"label": "shell-ball-input"/);
  assert.match(tauriConfig, /"url": "shell-ball-bubble\.html"/);
  assert.match(tauriConfig, /"url": "shell-ball-input\.html"/);
});

test("shell-ball desktop host declares detached pinned bubble windows", () => {
  assert.equal(existsSync(resolve(desktopRoot, "shell-ball-bubble-pinned.html")), true);
  assert.equal(existsSync(resolve(desktopRoot, "src/app/shell-ball-bubble-pinned/main.tsx")), true);

  const pinnedHtml = readFileSync(resolve(desktopRoot, "shell-ball-bubble-pinned.html"), "utf8");
  const pinnedEntry = readFileSync(resolve(desktopRoot, "src/app/shell-ball-bubble-pinned/main.tsx"), "utf8");
  const viteConfig = readFileSync(resolve(desktopRoot, "vite.config.ts"), "utf8");

  assert.match(pinnedHtml, /src="\/src\/app\/shell-ball-bubble-pinned\/main\.tsx"/);
  assert.match(pinnedEntry, /ShellBallPinnedBubbleWindow/);
  assert.match(pinnedEntry, /data-app-window/);
  assert.match(viteConfig, /"shell-ball-bubble-pinned"/);
});

test("shell-ball desktop window controller and capabilities stay aligned", () => {
  assert.deepEqual(shellBallWindowLabels, {
    ball: "shell-ball",
    bubble: "shell-ball-bubble",
    input: "shell-ball-input",
  });

  assert.equal(shellBallWindowPermissions.includes("core:window:allow-set-position"), true);
  assert.equal(shellBallWindowPermissions.includes("core:window:allow-set-size"), true);
  assert.equal(shellBallWindowPermissions.includes("core:window:allow-start-dragging"), true);
  assert.equal(shellBallWindowPermissions.includes("core:window:allow-set-ignore-cursor-events"), true);

  const capabilityConfig = readFileSync(
    resolve(desktopRoot, "src-tauri/capabilities/default.json"),
    "utf8",
  );
  const parsedCapabilityConfig = JSON.parse(capabilityConfig) as {
    windows: string[];
    permissions: string[];
  };

  assert.deepEqual(parsedCapabilityConfig.windows, [
    "shell-ball",
    "shell-ball-bubble",
    "shell-ball-input",
    "shell-ball-bubble-pinned-*",
    "dashboard",
    "control-panel",
  ]);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-create"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-set-position"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-set-size"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-start-dragging"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-set-ignore-cursor-events"), true);

  const generatedCapabilitySchema = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/gen/schemas/capabilities.json"), "utf8"),
  ) as {
    default: {
      windows: string[];
      permissions: string[];
    };
  };

  assert.deepEqual(generatedCapabilitySchema.default.windows, parsedCapabilityConfig.windows);
  assert.deepEqual(generatedCapabilitySchema.default.permissions, parsedCapabilityConfig.permissions);
  assert.equal(generatedCapabilitySchema.default.permissions.includes("core:window:allow-create"), true);
  assert.equal(generatedCapabilitySchema.default.permissions.includes("core:window:allow-unminimize"), true);
});

test("shell-ball pinned window labels and capabilities stay deterministic", () => {
  const controllerSource = readFileSync(
    resolve(desktopRoot, "src/platform/shellBallWindowController.ts"),
    "utf8",
  );
  const capabilityConfig = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/capabilities/default.json"), "utf8"),
  ) as {
    windows: string[];
    permissions: string[];
  };
  const generatedCapabilitySchema = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/gen/schemas/capabilities.json"), "utf8"),
  ) as {
    default: {
      windows: string[];
      permissions: string[];
    };
  };

  assert.match(controllerSource, /shellBallPinnedBubbleWindowLabelPrefix = "shell-ball-bubble-pinned-"/);
  assert.match(controllerSource, /return `\$\{shellBallPinnedBubbleWindowLabelPrefix\}\$\{bubbleId\}`/);
  assert.match(controllerSource, /shell-ball-bubble-pinned\.html/);
  assert.equal(capabilityConfig.windows.includes("shell-ball-bubble-pinned-*"), true);
  assert.deepEqual(generatedCapabilitySchema.default.windows, capabilityConfig.windows);
  assert.equal(generatedCapabilitySchema.default.windows.includes("shell-ball-bubble-pinned-*"), true);
});

test("dashboard and control-panel stay hidden on cold launch until explicitly opened", () => {
  const tauriConfig = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/tauri.conf.json"), "utf8"),
  ) as {
    app: {
      windows: Array<{
        label: string;
        decorations?: boolean;
        visible?: boolean;
      }>;
    };
  };

  const dashboardWindow = tauriConfig.app.windows.find((window) => window.label === "dashboard");
  const controlPanelWindow = tauriConfig.app.windows.find((window) => window.label === "control-panel");

  assert.ok(dashboardWindow);
  assert.ok(controlPanelWindow);
  assert.equal(dashboardWindow.visible, false);
  assert.equal(controlPanelWindow.visible, false);
  assert.equal(dashboardWindow.decorations, false);
  assert.equal(controlPanelWindow.decorations, false);
});

test("shell-ball entries opt into transparent window mode", () => {
  const ballEntry = readFileSync(resolve(desktopRoot, "src/app/shell-ball/main.tsx"), "utf8");
  const bubbleEntry = readFileSync(resolve(desktopRoot, "src/app/shell-ball-bubble/main.tsx"), "utf8");
  const inputEntry = readFileSync(resolve(desktopRoot, "src/app/shell-ball-input/main.tsx"), "utf8");
  const globalStyles = readFileSync(resolve(desktopRoot, "src/styles/globals.css"), "utf8");

  assert.match(ballEntry, /data-app-window/);
  assert.match(bubbleEntry, /data-app-window/);
  assert.match(inputEntry, /data-app-window/);
  assert.match(globalStyles, /\[data-app-window="shell-ball"\]/);
  assert.match(globalStyles, /overflow: hidden/);
});

test("shell-ball surface styles keep the shell transparent and fully draggable", () => {
  const shellBallStyles = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.css"), "utf8");
  const shellBallSurfaceBeforeBlock = shellBallStyles.match(/\.shell-ball-surface::before\s*\{([\s\S]*?)\}/)?.[1] ?? "";
  const mascotBlock = shellBallStyles.match(/\.shell-ball-mascot\s*\{([\s\S]*?)\}/)?.[1] ?? "";
  const mascotHotspotBlock = shellBallStyles.match(/\.shell-ball-mascot__hotspot\s*\{([\s\S]*?)\}/)?.[1] ?? "";

  assert.doesNotMatch(shellBallSurfaceBeforeBlock, /background:/);
  assert.doesNotMatch(shellBallStyles, /overflow-x:\s*hidden/);
  assert.match(mascotBlock, /width:\s*clamp\(/);
  assert.match(mascotHotspotBlock, /inset:\s*0;/);
});

test("shell-ball helper windows avoid auto-focus behavior", () => {
  const tauriConfig = readFileSync(resolve(desktopRoot, "src-tauri/tauri.conf.json"), "utf8");
  const controllerSource = readFileSync(
    resolve(desktopRoot, "src/platform/shellBallWindowController.ts"),
    "utf8",
  );
  const metricsSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallWindowMetrics.ts"),
    "utf8",
  );
  const inputBarSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/components/ShellBallInputBar.tsx"),
    "utf8",
  );
  const planSource = readFileSync(
    resolve(desktopRoot, "docs/2026-04-11-desktop-shell-ball-three-window-implementation-plan.md"),
    "utf8",
  );

  assert.doesNotMatch(tauriConfig, /"focusable": false/);
  assert.match(controllerSource, /setShellBallWindowFocusable\([^)]*focusable: boolean\)/);
  assert.match(controllerSource, /setShellBallWindowIgnoreCursorEvents\([^)]*ignore: boolean\)/);
  assert.match(metricsSource, /getShellBallHelperWindowInteractionMode/);
  assert.match(metricsSource, /setShellBallWindowFocusable\(role, interactionMode\.focusable\)/);
  assert.match(metricsSource, /setShellBallWindowIgnoreCursorEvents\(role, interactionMode\.ignoreCursorEvents\)/);
  assert.doesNotMatch(metricsSource, /setFocus\(\)/);
  assert.doesNotMatch(inputBarSource, /focus\(\{ preventScroll: true \}\)/);
  assert.doesNotMatch(planSource, /focusable: false/);
  assert.match(planSource, /setFocusable\(false\)/);
  assert.match(planSource, /setIgnoreCursorEvents\(true\)/);
});

test("shell-ball desktop navigation keeps route changes separate from desktop window focus", () => {
  const controllerSource = readFileSync(resolve(desktopRoot, "src/platform/windowController.ts"), "utf8");
  const dashboardRootSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardRoot.tsx"), "utf8");
  const dashboardHomeSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/DashboardHome.tsx"), "utf8");
  const dashboardBackHomeLinkSource = readFileSync(
    resolve(desktopRoot, "src/features/dashboard/shared/DashboardBackHomeLink.tsx"),
    "utf8",
  );
  const taskPageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/tasks/TaskPage.tsx"), "utf8");
  const notePageSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/notes/NotePage.tsx"), "utf8");
  const dashboardHomeConfigSource = readFileSync(
    resolve(desktopRoot, "src/features/dashboard/home/dashboardHome.config.ts"),
    "utf8",
  );
  const dashboardRoutesSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/shared/dashboardRoutes.ts"), "utf8");
  const dashboardEventPanelSource = readFileSync(
    resolve(desktopRoot, "src/features/dashboard/home/components/DashboardEventPanel.tsx"),
    "utf8",
  );
  const dashboardPlaceholderPageSource = readFileSync(
    resolve(desktopRoot, "src/features/dashboard/shared/DashboardPlaceholderPage.tsx"),
    "utf8",
  );
  const securityAppSource = readFileSync(resolve(desktopRoot, "src/features/dashboard/safety/SecurityApp.tsx"), "utf8");
  const dashboardRouteTargetsSource = readFileSync(
    resolve(desktopRoot, "src/features/dashboard/shared/dashboardRouteTargets.ts"),
    "utf8",
  );
  assert.deepEqual(dashboardRoutePaths, {
    home: "/",
    safety: "/safety",
  });
  assert.equal(dashboardSafetyRoutePath, "/safety");
  assert.equal(resolveDashboardRoutePath("home"), "/");
  assert.equal(resolveDashboardRoutePath("safety"), dashboardSafetyRoutePath);
  assert.equal(resolveDashboardRouteHref("home"), "./dashboard.html");
  assert.equal(resolveDashboardRouteHref("safety"), "./dashboard.html#/safety");
  assert.equal(resolveDashboardModuleRoutePath("tasks"), "/tasks");
  assert.equal(resolveDashboardModuleRoutePath("notes"), "/notes");
  assert.equal(resolveDashboardModuleRoutePath("memory"), "/memory");
  assert.equal(resolveDashboardModuleRoutePath("safety"), dashboardSafetyRoutePath);
  assert.equal(existsSync(resolve(desktopRoot, "src/features/dashboard/shared/dashboardRouteNavigation.ts")), false);
  assert.equal(existsSync(resolve(desktopRoot, ".cache/shell-ball-tests/app/dashboard/DashboardRoot.js")), true);
  assert.equal(existsSync(resolve(desktopRoot, ".cache/shell-ball-tests/features/dashboard/DashboardApp.js")), true);
  assert.equal(existsSync(resolve(desktopRoot, ".cache/shell-ball-tests/features/dashboard/safety/SafetyPage.js")), true);
  assert.equal(existsSync(resolve(desktopRoot, ".cache/shell-ball-tests/features/dashboard/safety/SecurityPageShell.js")), true);
  assert.equal(existsSync(resolve(desktopRoot, ".cache/shell-ball-tests/features/dashboard/safety/SecurityApp.js")), true);
  assert.equal(existsSync(resolve(desktopRoot, ".cache/shell-ball-tests/platform/trayController.js")), true);
  assert.match(dashboardRouteTargetsSource, /export const dashboardSafetyRoutePath = "\/safety"/);

  assert.match(controllerSource, /export type DesktopWindowLabel = "dashboard" \| "control-panel"/);
  assert.doesNotMatch(controllerSource, /resolveDashboardRouteHref/);
  assert.doesNotMatch(controllerSource, /openDashboardRoute/);
  assert.match(dashboardBackHomeLinkSource, /resolveDashboardRoutePath\("home"\)/);
  assert.doesNotMatch(dashboardBackHomeLinkSource, /to="\/"/);
  assert.match(dashboardRootSource, /resolveDashboardModuleRoutePath\("tasks"\)/);
  assert.match(dashboardRootSource, /resolveDashboardModuleRoutePath\("notes"\)/);
  assert.match(dashboardRootSource, /resolveDashboardModuleRoutePath\("memory"\)/);
  assert.match(dashboardRootSource, /resolveDashboardModuleRoutePath\("safety"\)/);
  assert.doesNotMatch(dashboardRootSource, /path="\/tasks\/\*"/);
  assert.doesNotMatch(dashboardRootSource, /path="\/notes\/\*"/);
  assert.doesNotMatch(dashboardRootSource, /path="\/memory\/\*"/);
  assert.match(dashboardHomeSource, /resolveDashboardModuleRoutePath\(module\)/);
  assert.match(dashboardHomeSource, /resolveDashboardModuleRoutePath\("tasks"\)/);
  assert.match(dashboardHomeSource, /resolveDashboardModuleRoutePath\("notes"\)/);
  assert.match(dashboardHomeSource, /resolveDashboardModuleRoutePath\("memory"\)/);
  assert.match(dashboardHomeSource, /resolveDashboardModuleRoutePath\("safety"\)/);
  assert.doesNotMatch(dashboardHomeSource, /"\/tasks"/);
  assert.doesNotMatch(dashboardHomeSource, /"\/notes"/);
  assert.doesNotMatch(dashboardHomeSource, /"\/memory"/);
  assert.doesNotMatch(dashboardHomeSource, /"\/safety"/);
  assert.match(taskPageSource, /resolveDashboardRoutePath\("home"\)/);
  assert.match(taskPageSource, /resolveDashboardRoutePath\("safety"\)/);
  assert.doesNotMatch(taskPageSource, /navigate\("\/safety"\)/);
  assert.doesNotMatch(taskPageSource, /to="\/"/);
  assert.match(notePageSource, /resolveDashboardRoutePath\("home"\)/);
  assert.match(notePageSource, /resolveDashboardModuleRoutePath\("tasks"\)/);
  assert.doesNotMatch(notePageSource, /navigate\("\/tasks"/);
  assert.doesNotMatch(notePageSource, /to="\/"/);
  assert.match(dashboardHomeConfigSource, /resolveDashboardModuleRoutePath\("tasks"\)/);
  assert.match(dashboardHomeConfigSource, /resolveDashboardModuleRoutePath\("notes"\)/);
  assert.match(dashboardHomeConfigSource, /resolveDashboardModuleRoutePath\("memory"\)/);
  assert.match(dashboardHomeConfigSource, /resolveDashboardModuleRoutePath\("safety"\)/);
  assert.doesNotMatch(dashboardHomeConfigSource, /route: "\/tasks"/);
  assert.doesNotMatch(dashboardHomeConfigSource, /route: "\/notes"/);
  assert.doesNotMatch(dashboardHomeConfigSource, /route: "\/memory"/);
  assert.doesNotMatch(dashboardHomeConfigSource, /route: "\/safety"/);
  assert.match(dashboardEventPanelSource, /resolveDashboardModuleRoutePath\(module\)/);
  assert.doesNotMatch(dashboardEventPanelSource, /navigate\(`\/\$\{module\}`\)/);
  assert.match(dashboardPlaceholderPageSource, /resolveDashboardRoutePath\("home"\)/);
  assert.doesNotMatch(dashboardPlaceholderPageSource, /to="\/"/);
  assert.match(dashboardRoutesSource, /resolveDashboardModuleRoutePath\("tasks"\)/);
  assert.match(dashboardRoutesSource, /resolveDashboardModuleRoutePath\("notes"\)/);
  assert.match(dashboardRoutesSource, /resolveDashboardModuleRoutePath\("memory"\)/);
  assert.match(dashboardRoutesSource, /resolveDashboardModuleRoutePath\("safety"\)/);
  assert.doesNotMatch(dashboardRoutesSource, /path: "\/tasks"/);
  assert.doesNotMatch(dashboardRoutesSource, /path: "\/notes"/);
  assert.doesNotMatch(dashboardRoutesSource, /path: "\/memory"/);
  assert.doesNotMatch(dashboardRoutesSource, /path: "\/safety"/);
  assert.match(securityAppSource, /useNavigate\(/);
  assert.match(securityAppSource, /navigate\(resolveDashboardRoutePath\("home"\)\)/);
  assert.doesNotMatch(securityAppSource, /openDashboardRoute/);
});

test("window controller focuses an existing labeled desktop window", async () => {
  const calls: string[] = [];
  const handle = {
    async unminimize() {
      calls.push("unminimize");
    },
    async setFullscreen(value: boolean) {
      calls.push(`setFullscreen:${String(value)}`);
    },
    async show() {
      calls.push("show");
    },
    async setFocus() {
      calls.push("setFocus");
    },
  };

  const capabilityConfig = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/capabilities/default.json"), "utf8"),
  ) as { permissions: string[] };

  assert.equal(capabilityConfig.permissions.includes("core:window:allow-unminimize"), true);
  assert.equal(capabilityConfig.permissions.includes("core:window:allow-set-fullscreen"), true);

  await withWindowControllerRuntime({
    getByLabel(label) {
      calls.push(`label:${label}`);
      return handle;
    },
  }, async ({ openOrFocusDesktopWindow }) => {
    await openOrFocusDesktopWindow("dashboard");
  });

  assert.deepEqual(calls, ["label:dashboard", "unminimize", "setFullscreen:true", "show", "setFocus"]);
});

test("window controller recreates missing known desktop windows before focusing them", async () => {
  const reopenScenarios = [
    {
      label: "dashboard",
      expectedOptions: {
        title: "CialloClaw Dashboard",
        width: 1280,
        height: 860,
        decorations: false,
        visible: false,
        url: "dashboard.html",
      },
    },
    {
      label: "control-panel",
      expectedOptions: {
        title: "CialloClaw Control Panel",
        width: 1080,
        height: 760,
        decorations: false,
        visible: false,
        url: "control-panel.html",
      },
    },
  ] as const;

  for (const scenario of reopenScenarios) {
    const calls: string[] = [];
    const recreatedHandle = {
      async unminimize() {
        calls.push("unminimize");
      },
      async setFullscreen(value: boolean) {
        calls.push(`setFullscreen:${String(value)}`);
      },
      async show() {
        calls.push("show");
      },
      async setFocus() {
        calls.push("setFocus");
      },
    };

    await withWindowControllerRuntime({
      getByLabel(label) {
        calls.push(`label:${label}`);
        return null;
      },
      createWindow(label, options) {
        calls.push(`create:${label}`);
        assert.equal(label, scenario.label);
        assert.deepEqual(options, scenario.expectedOptions);
        return recreatedHandle;
      },
    }, async ({ openOrFocusDesktopWindow }) => {
      await openOrFocusDesktopWindow(scenario.label);
    });

      assert.deepEqual(
        calls,
        scenario.label === "dashboard"
          ? [`label:${scenario.label}`, `create:${scenario.label}`, "unminimize", "setFullscreen:true", "show", "setFocus"]
          : [`label:${scenario.label}`, `create:${scenario.label}`, "unminimize", "show", "setFocus"],
      );
  }
});

test("hide-on-close helper prevents the close request and hides the current window", async () => {
  const calls: string[] = [];
  let closeHandler: ((event: { preventDefault: () => void }) => Promise<void> | void) | null = null;

  await withHideOnCloseRequestRuntime({
    onCloseRequested(handler) {
      calls.push("onCloseRequested");
      closeHandler = handler;
      return "unlisten";
    },
    async hide() {
      calls.push("hide");
    },
  }, async ({ installHideOnCloseRequest }) => {
    const result = installHideOnCloseRequest();

    assert.equal(result, "unlisten");
    assert.notEqual(closeHandler, null);

    await closeHandler?.({
      preventDefault() {
        calls.push("preventDefault");
      },
    });
  });

  assert.deepEqual(calls, ["onCloseRequested", "preventDefault", "hide"]);
});

test("hide-on-close helper waits for the dashboard close transition only in the dashboard window", async () => {
  for (const scenario of [
    {
      label: "dashboard",
      expectedCalls: ["onCloseRequested", "preventDefault", "requestShellBallDashboardCloseTransition", "hide"],
    },
    {
      label: "control-panel",
      expectedCalls: ["onCloseRequested", "preventDefault", "hide"],
    },
  ] as const) {
    const calls: string[] = [];
    let closeHandler: ((event: { preventDefault: () => void }) => Promise<void> | void) | null = null;

    await withHideOnCloseRequestRuntime({
      __calls__: calls,
      label: scenario.label,
      onCloseRequested(handler) {
        calls.push("onCloseRequested");
        closeHandler = handler;
        return "unlisten";
      },
      async hide() {
        calls.push("hide");
      },
    }, async ({ installHideOnCloseRequest }) => {
      installHideOnCloseRequest();
      await closeHandler?.({
        preventDefault() {
          calls.push("preventDefault");
        },
      });
    });

    assert.deepEqual(calls, scenario.expectedCalls);
  }
});

test("dashboard and control-panel entrypoints install hide-on-close handling", () => {
  const dashboardMainSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/main.tsx"), "utf8");
  const controlPanelMainSource = readFileSync(resolve(desktopRoot, "src/app/control-panel/main.tsx"), "utf8");

  assert.match(dashboardMainSource, /installHideOnCloseRequest/);
  assert.match(dashboardMainSource, /void installHideOnCloseRequest\(\)/);
  assert.match(controlPanelMainSource, /installHideOnCloseRequest/);
  assert.match(controlPanelMainSource, /void installHideOnCloseRequest\(\)/);
});

test("tray controller opens the control panel through the desktop window API", async () => {
  await withTrayControllerRuntime(async () => "control-panel", async ({ openControlPanelFromTray, calls }) => {
    await openControlPanelFromTray();

    assert.deepEqual(calls, ["control-panel"]);
  });
});

test("dashboard app safety CTA renders the shared safety href", () => {
  const markup = renderDashboardAppMarkup();

  assert.match(markup, /href="\.\/dashboard\.html#\/safety"/);
});

test("dashboard route surface renders the live home and safety routes", () => {
  const homeMarkup = renderDashboardRouteSurface("");
  const safetyMarkup = renderDashboardRouteSurface("#/safety");

  assert.match(homeMarkup, /Dashboard Orbit/);
  assert.doesNotMatch(homeMarkup, /security-shell-stub/);
  assert.doesNotMatch(homeMarkup, /返回首页/);
  assert.match(safetyMarkup, /返回首页/);
  assert.match(safetyMarkup, /security-shell-stub/);
  assert.doesNotMatch(safetyMarkup, /Dashboard Orbit/);
});

test("shell-ball input bar keeps hook order stable across hidden and visible states", () => {
  const inputBarSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/components/ShellBallInputBar.tsx"),
    "utf8",
  );

  assert.equal(
    inputBarSource.indexOf("useEffect(()") < inputBarSource.indexOf('if (mode === "hidden")'),
    true,
  );
});

test("shell-ball helper window sync maps visual states into visibility and snapshot payloads", () => {
  assert.deepEqual(shellBallWindowSyncEvents, {
    snapshot: "desktop-shell-ball:snapshot",
    geometry: "desktop-shell-ball:geometry",
    helperReady: "desktop-shell-ball:helper-ready",
    pinnedWindowReady: "desktop-shell-ball:pinned-window-ready",
    pinnedWindowDetached: "desktop-shell-ball:pinned-window-detached",
    inputHover: "desktop-shell-ball:input-hover",
    inputFocus: "desktop-shell-ball:input-focus",
    inputDraft: "desktop-shell-ball:input-draft",
    primaryAction: "desktop-shell-ball:primary-action",
    bubbleAction: "desktop-shell-ball:bubble-action",
  });

  assert.deepEqual(getShellBallHelperWindowVisibility("idle"), {
    bubble: true,
    input: false,
  });

  assert.deepEqual(getShellBallHelperWindowVisibility("hover_input"), {
    bubble: true,
    input: true,
  });

  assert.deepEqual(
    createShellBallWindowSnapshot({
      visualState: "voice_locked",
      inputValue: "draft",
      voicePreview: "lock",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "bubble-1",
            task_id: "task-1",
            type: "status",
            text: "Still listening.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:00:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
            freshnessHint: "fresh",
            motionHint: "settle",
          },
        },
      ],
    }),
    {
      visualState: "voice_locked",
      inputBarMode: "voice",
      inputValue: "draft",
      voicePreview: "lock",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "bubble-1",
            task_id: "task-1",
            type: "status",
            text: "Still listening.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:00:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
            freshnessHint: "fresh",
            motionHint: "settle",
          },
        },
      ],
      bubbleRegion: {
        strategy: "persistent",
        hasVisibleItems: true,
        clickThrough: false,
      },
      visibility: {
        bubble: true,
        input: true,
      },
    },
  );
});

test("shell-ball bubble region existence strategy is explicit and item-driven", () => {
  const bubbleItems: ShellBallBubbleItem[] = [
    {
      bubble: {
        bubble_id: "bubble-visible",
        task_id: "task-visible",
        type: "status",
        text: "Visible bubble",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:00:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "bubble-pinned",
        task_id: "task-pinned",
        type: "result",
        text: "Pinned bubble",
        pinned: true,
        hidden: false,
        created_at: "2026-04-11T10:01:00.000Z",
      },
      role: "user",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "bubble-hidden",
        task_id: "task-hidden",
        type: "status",
        text: "Hidden bubble",
        pinned: false,
        hidden: true,
        created_at: "2026-04-11T10:02:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "hidden",
      },
    },
  ];

  assert.deepEqual(getShellBallVisibleBubbleItems(bubbleItems).map((item) => item.bubble.bubble_id), ["bubble-visible"]);
  assert.deepEqual(getShellBallBubbleRegionState(bubbleItems), {
    strategy: "persistent",
    hasVisibleItems: true,
    clickThrough: false,
  });
  assert.deepEqual(getShellBallBubbleRegionState([]), {
    strategy: "persistent",
    hasVisibleItems: false,
    clickThrough: true,
  });
});

test("shell-ball bubble item contract wraps protocol payload and keeps desktop-only state local", () => {
  const bubbleContractSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.bubble.ts"), "utf8");
  const bubbleItem: ShellBallBubbleItem = {
    bubble: {
      bubble_id: "bubble-local-1",
      task_id: "task-local-1",
      type: "result",
      text: "Open the dashboard.",
      pinned: false,
      hidden: false,
      created_at: "2026-04-11T10:00:00.000Z",
    },
    role: "user",
    desktop: {
      lifecycleState: "hidden",
      freshnessHint: "stale",
      motionHint: "settle",
    },
  };

  assert.deepEqual(bubbleItem, {
    bubble: {
      bubble_id: "bubble-local-1",
      task_id: "task-local-1",
      type: "result",
      text: "Open the dashboard.",
      pinned: false,
      hidden: false,
      created_at: "2026-04-11T10:00:00.000Z",
    },
    role: "user",
    desktop: {
      lifecycleState: "hidden",
      freshnessHint: "stale",
      motionHint: "settle",
    },
  });
  assert.equal("role" in bubbleItem.bubble, false);
  assert.equal("desktop" in bubbleItem.bubble, false);
  assert.doesNotMatch(bubbleContractSource, /"pulse"/);
  assert.doesNotMatch(bubbleContractSource, /ShellBallBubbleMessage/);

  assert.deepEqual(createShellBallWindowSnapshot({
    visualState: "idle",
    inputValue: "",
    voicePreview: null,
    bubbleItems: [],
  }).bubbleItems, []);

  const minimalBubbleItem: ShellBallBubbleItem = {
    bubble: {
      bubble_id: "bubble-local-2",
      task_id: "task-local-2",
      type: "status",
      text: "On it.",
      pinned: false,
      hidden: false,
      created_at: "2026-04-11T10:01:00.000Z",
    },
    role: "agent",
    desktop: {
      lifecycleState: "visible",
    },
  };

  assert.deepEqual(minimalBubbleItem, {
    bubble: {
      bubble_id: "bubble-local-2",
      task_id: "task-local-2",
      type: "status",
      text: "On it.",
      pinned: false,
      hidden: false,
      created_at: "2026-04-11T10:01:00.000Z",
    },
    role: "agent",
    desktop: {
      lifecycleState: "visible",
    },
  });

  assert.deepEqual(
    cloneShellBallBubbleItems([minimalBubbleItem]),
    [minimalBubbleItem],
  );
});

test("shell-ball window snapshot copies bubble item arrays defensively", () => {
  const sourceItems: ShellBallBubbleItem[] = [
    {
      bubble: {
        bubble_id: "bubble-copy-1",
        task_id: "task-copy-1",
        type: "status",
        text: "Drafting update.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:02:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
  ];

  const snapshot = createShellBallWindowSnapshot({
    visualState: "hover_input",
    inputValue: "draft",
    voicePreview: null,
    bubbleItems: sourceItems,
  });

  assert.notEqual(snapshot.bubbleItems, sourceItems);
  assert.notEqual(snapshot.bubbleItems[0], sourceItems[0]);
  assert.deepEqual(snapshot.bubbleItems, sourceItems);

  sourceItems[0].bubble.text = "Changed after snapshot.";

  assert.deepEqual(snapshot.bubbleItems, [
    {
      bubble: {
        bubble_id: "bubble-copy-1",
        task_id: "task-copy-1",
        type: "status",
        text: "Drafting update.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:02:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
  ]);

  sourceItems.push({
    bubble: {
      bubble_id: "bubble-copy-2",
      task_id: "task-copy-2",
      type: "result",
      text: "Keep going.",
      pinned: false,
      hidden: false,
      created_at: "2026-04-11T10:03:00.000Z",
    },
    role: "user",
    desktop: {
      lifecycleState: "visible",
    },
  });

  assert.deepEqual(snapshot.bubbleItems, [
    {
      bubble: {
        bubble_id: "bubble-copy-1",
        task_id: "task-copy-1",
        type: "status",
        text: "Drafting update.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:02:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
  ]);
});

test("shell-ball window metrics compute safe frames and helper anchors", () => {
  assert.equal(SHELL_BALL_BUBBLE_GAP_PX, 6);
  assert.equal(SHELL_BALL_INPUT_GAP_PX, 12);
  assert.equal(SHELL_BALL_WINDOW_SAFE_MARGIN_PX, 12);

  const ballFrame = createShellBallWindowFrame({ width: 100, height: 80 });

  assert.deepEqual(ballFrame, {
    width: 124,
    height: 104,
  });

  assert.deepEqual(
    getShellBallBubbleAnchor({
      ballFrame: {
        x: 200,
        y: 300,
        ...ballFrame,
      },
      helperFrame: {
        width: 180,
        height: 90,
      },
    }),
    {
      x: 172,
      y: 204,
    },
  );

  assert.equal(204 + 90 <= 300, true);

  assert.deepEqual(
    getShellBallInputAnchor({
      ballFrame: {
        x: 200,
        y: 300,
        ...ballFrame,
      },
      helperFrame: {
        width: 220,
        height: 88,
      },
    }),
    {
      x: 152,
      y: 416,
    },
  );

  assert.deepEqual(
    clampShellBallFrameToBounds(
      {
        x: -24,
        y: 44,
        width: 124,
        height: 104,
      },
      {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
    ),
    {
      x: 0,
      y: 44,
      width: 124,
      height: 104,
    },
  );
});

test("shell-ball interaction contract auto-advances text submission into processing", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "submit_text",
      regionActive: true,
    }),
    {
      next: "confirming_intent",
      autoAdvanceTo: "processing",
      autoAdvanceMs: 600,
    },
  );
});

test("shell-ball interaction contract enters hover mode on hotspot entry", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "idle",
      event: "pointer_enter_hotspot",
      regionActive: true,
    }),
    {
      next: "idle",
      autoAdvanceTo: "hover_input",
      autoAdvanceMs: SHELL_BALL_HOVER_INTENT_MS,
    },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "pointer_enter_hotspot",
      regionActive: true,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract leaves the region only from hoverable resting states", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "pointer_leave_region",
      regionActive: false,
      hoverRetained: false,
    }),
    {
      next: "hover_input",
      autoAdvanceTo: "idle",
      autoAdvanceMs: SHELL_BALL_LEAVE_GRACE_MS,
    },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "pointer_leave_region",
      regionActive: false,
      hoverRetained: false,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract retains hover input only while focus is active", () => {
  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: true,
      hasDraft: false,
    }),
    true,
  );

  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: false,
      hasDraft: true,
    }),
    false,
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "pointer_leave_region",
      regionActive: false,
      hoverRetained: true,
    }),
    { next: "hover_input" },
  );
});

test("shell-ball interaction contract auto-advances file attach through auth waiting", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "attach_file",
      regionActive: true,
    }),
    {
      next: "waiting_auth",
      autoAdvanceTo: "processing",
      autoAdvanceMs: 700,
    },
  );
});

test("shell-ball interaction contract starts voice listening only from resting input states", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "idle",
      event: "press_start",
      regionActive: true,
    }),
    { next: "voice_listening" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "press_start",
      regionActive: true,
    }),
    { next: "voice_listening" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "press_start",
      regionActive: true,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract supports voice lock during long-press capture", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "voice_listening",
      event: "voice_lock",
      regionActive: true,
    }),
    { next: "voice_locked" },
  );
});

test("shell-ball interaction contract supports voice cancel", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "voice_listening",
      event: "voice_cancel",
      regionActive: true,
    }),
    { next: "idle" },
  );
});

test("shell-ball speech draft composition keeps English spacing and Chinese adjacency stable", () => {
  assert.equal(composeShellBallSpeechDraft("Draft", "ready now"), "Draft ready now");
  assert.equal(composeShellBallSpeechDraft("打开仪表盘", "然后开始处理"), "打开仪表盘然后开始处理");
  assert.equal(composeShellBallSpeechDraft("", "  hello   world  "), "hello world");
});

test("shell-ball speech transcript collection merges recognition chunks", () => {
  assert.equal(
    collectShellBallSpeechTranscript({
      0: { 0: { transcript: "hello" }, isFinal: true, length: 1 },
      1: { 0: { transcript: "dashboard" }, isFinal: false, length: 1 },
      length: 2,
    }),
    "hello dashboard",
  );
});

test("shell-ball voice recognition final state routes final transcript out of the input draft and restores draft on cancel", () => {
  assert.deepEqual(
    resolveShellBallVoiceRecognitionFinalState({
      reason: "finish",
      transcript: "开始处理",
      baseDraft: "打开仪表盘",
      startState: "idle",
    }),
    {
      finalizedSpeechPayload: "开始处理",
      nextInputValue: "打开仪表盘",
      nextVisualState: "hover_input",
    },
  );

  assert.deepEqual(
    resolveShellBallVoiceRecognitionFinalState({
      reason: "finish",
      transcript: "开始处理",
      baseDraft: "",
      startState: "idle",
    }),
    {
      finalizedSpeechPayload: "开始处理",
      nextInputValue: "",
      nextVisualState: "idle",
    },
  );

  assert.deepEqual(
    resolveShellBallVoiceRecognitionFinalState({
      reason: "cancel",
      transcript: "ignored",
      baseDraft: "保留原稿",
      startState: "hover_input",
    }),
    {
      finalizedSpeechPayload: null,
      nextInputValue: "保留原稿",
      nextVisualState: "hover_input",
    },
  );
});

test("shell-ball submit params route text and voice through the formal input contract", () => {
  const textParams = createShellBallInputSubmitParams({
    text: "  summarize this  ",
    trigger: "hover_text_input",
    inputMode: "text",
  });

  assert.ok(textParams);
  assert.equal(textParams.source, "floating_ball");
  assert.equal(textParams.trigger, "hover_text_input");
  assert.deepEqual(textParams.input, {
    type: "text",
    text: "summarize this",
    input_mode: "text",
  });
  assert.deepEqual(textParams.context, { files: [] });

  const voiceParams = createShellBallInputSubmitParams({
    text: "  打开仪表盘  ",
    trigger: "voice_commit",
    inputMode: "voice",
  });

  assert.ok(voiceParams);
  assert.equal(voiceParams.trigger, "voice_commit");
  assert.equal(voiceParams.input.input_mode, "voice");
  assert.equal(createShellBallInputSubmitParams({ text: "   ", trigger: "hover_text_input", inputMode: "text" }), null);
});

test("shell-ball text drop helpers only accept non-file drags and extract plain text", () => {
  assert.equal(
    shouldAcceptShellBallTextDrop({
      files: { length: 0 } as FileList,
    }),
    true,
  );
  assert.equal(
    shouldAcceptShellBallTextDrop({
      files: { length: 1 } as FileList,
    }),
    false,
  );
  assert.equal(
    extractShellBallDroppedText({
      files: { length: 0 } as FileList,
      effectAllowed: "copyMove",
      getData: (type: string) => (type === "text/plain" ? "  dragged summary text  " : ""),
    }),
    "dragged summary text",
  );
  assert.equal(resolveShellBallTextDropEffect("copyMove"), "copy");
  assert.equal(resolveShellBallTextDropEffect("move"), "move");
});

test("shell-ball dropped text appends into the input draft instead of submitting immediately", () => {
  assert.equal(
    appendShellBallDroppedText({
      currentValue: "",
      droppedText: "  dragged summary text  ",
    }),
    "dragged summary text",
  );
  assert.equal(
    appendShellBallDroppedText({
      currentValue: "Current note",
      droppedText: "Dragged text",
    }),
    "Current note\nDragged text",
  );
  assert.equal(
    appendShellBallDroppedText({
      currentValue: "Current note\n",
      droppedText: "Dragged text",
    }),
    "Current note\nDragged text",
  );
});

test("shell-ball text drop target only arms during eligible global drags", () => {
  assert.equal(
    shouldArmShellBallTextDropTarget({
      fileDropActive: false,
      globalDragActive: true,
      shellBallWindowDragActive: false,
      visualState: "hover_input",
    }),
    true,
  );
  assert.equal(
    shouldArmShellBallTextDropTarget({
      fileDropActive: true,
      globalDragActive: true,
      shellBallWindowDragActive: false,
      visualState: "hover_input",
    }),
    false,
  );
  assert.equal(
    shouldArmShellBallTextDropTarget({
      fileDropActive: false,
      globalDragActive: true,
      shellBallWindowDragActive: false,
      visualState: "voice_locked",
    }),
    false,
  );
});

test("shell-ball interaction contract auto-advances waiting auth and processing states", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "waiting_auth",
      event: "auto_advance",
      regionActive: true,
    }),
    { next: "processing" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "auto_advance",
      regionActive: true,
    }),
    { next: "hover_input" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "auto_advance",
      regionActive: false,
    }),
    { next: "idle" },
  );
});

test("shell-ball controller schedules confirm, auth, and processing auto-advances", () => {
  const hoverScheduler = createFakeScheduler();
  const hoverController = createShellBallInteractionController({
    initialState: "idle",
    schedule: hoverScheduler.schedule,
    cancel: hoverScheduler.cancel,
  });

  hoverController.dispatch("pointer_enter_hotspot", { regionActive: true });
  assert.equal(hoverController.getState(), "idle");
  assert.equal(hoverScheduler.size, 1);

  hoverScheduler.flush();
  assert.equal(hoverController.getState(), "hover_input");

  hoverController.dispatch("pointer_leave_region", { regionActive: false });
  assert.equal(hoverController.getState(), "hover_input");
  assert.equal(hoverScheduler.size, 1);

  hoverScheduler.flush();
  assert.equal(hoverController.getState(), "idle");
  hoverController.dispose();

  const confirmingScheduler = createFakeScheduler();
  const confirmingController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: confirmingScheduler.schedule,
    cancel: confirmingScheduler.cancel,
  });

  confirmingController.dispatch("submit_text", { regionActive: true });
  assert.equal(confirmingController.getState(), "confirming_intent");
  assert.equal(confirmingScheduler.size, 1);

  confirmingScheduler.flush();
  assert.equal(confirmingController.getState(), "processing");
  assert.equal(confirmingScheduler.size, 1);

  confirmingScheduler.flush();
  assert.equal(confirmingController.getState(), "hover_input");
  confirmingController.dispose();

  const authScheduler = createFakeScheduler();
  const authController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: authScheduler.schedule,
    cancel: authScheduler.cancel,
  });

  authController.dispatch("attach_file", { regionActive: false });
  assert.equal(authController.getState(), "waiting_auth");
  assert.equal(authScheduler.size, 1);

  authScheduler.flush();
  assert.equal(authController.getState(), "processing");
  assert.equal(authScheduler.size, 1);

  authScheduler.flush();
  assert.equal(authController.getState(), "idle");
  authController.dispose();
});

test("shell-ball controller cancels leave grace when the hotspot is re-entered", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("pointer_leave_region", { regionActive: false });
  assert.equal(scheduler.size, 1);

  controller.dispatch("pointer_enter_hotspot", { regionActive: true });
  assert.equal(controller.getState(), "hover_input");
  assert.equal(scheduler.size, 0);

  scheduler.flush();
  assert.equal(controller.getState(), "hover_input");
  controller.dispose();
});

test("shell-ball controller keeps hover input open while retained and closes after retention ends", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("pointer_leave_region", { regionActive: false, hoverRetained: true });
  assert.equal(controller.getState(), "hover_input");
  assert.equal(scheduler.size, 0);

  controller.dispatch("pointer_leave_region", { regionActive: false, hoverRetained: false });
  assert.equal(scheduler.size, 1);

  scheduler.flush();
  assert.equal(controller.getState(), "idle");
  controller.dispose();
});

test("shell-ball controller cancels stale auto-advance on forceState and replacement flows", () => {
  const forceScheduler = createFakeScheduler();
  const forceController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: forceScheduler.schedule,
    cancel: forceScheduler.cancel,
  });

  forceController.dispatch("submit_text", { regionActive: true });
  forceController.forceState("idle");
  forceScheduler.flush();
  assert.equal(forceController.getState(), "idle");
  forceController.dispose();

  const replacementScheduler = createFakeScheduler();
  const replacementController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: replacementScheduler.schedule,
    cancel: replacementScheduler.cancel,
  });

  replacementController.dispatch("submit_text", { regionActive: true });
  replacementController.forceState("hover_input");
  replacementController.dispatch("attach_file", { regionActive: false });
  replacementScheduler.flush();
  assert.equal(replacementController.getState(), "processing");
  replacementScheduler.flush();
  assert.equal(replacementController.getState(), "idle");
  replacementController.dispose();
});

test("shell-ball controller forceState applies processing entry side effects", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.forceState("processing", { regionActive: true });
  assert.equal(controller.getState(), "processing");
  assert.equal(scheduler.size, 1);

  scheduler.flush();
  assert.equal(controller.getState(), "hover_input");
  controller.dispose();
});

test("shell-ball controller keeps locked voice active without legacy finish events", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "voice_locked",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("pointer_leave_region", { regionActive: false });
  controller.dispatch("auto_advance", { regionActive: false });

  assert.equal(controller.getState(), "voice_locked");
  controller.dispose();
});

test("shell-ball processing return follows the latest region activity when the timer completes", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "idle",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.forceState("processing", { regionActive: true });
  controller.dispatch("pointer_leave_region", { regionActive: false });

  scheduler.flush();
  assert.equal(controller.getState(), "idle");
  controller.dispose();
});

test("shell-ball interaction sync helper re-aligns an externally changed visual state", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("submit_text", { regionActive: true });
  assert.equal(controller.getState(), "confirming_intent");

  syncShellBallInteractionController({
    controller,
    visualState: "voice_locked",
    regionActive: true,
  });

  scheduler.flush();
  assert.equal(controller.getState(), "voice_locked");
  controller.dispose();
});

test("shell-ball processing completion returns to the region-aware resting state", () => {
  assert.equal(getShellBallProcessingReturnState(true), "hover_input");
  assert.equal(getShellBallProcessingReturnState(false), "idle");
});

test("shell-ball voice preview helpers keep preview and release resolution pure", () => {
  assert.equal(getShellBallVoicePreview({ deltaX: 0, deltaY: -SHELL_BALL_LOCK_DELTA_PX }), "lock");
  assert.equal(getShellBallVoicePreview({ deltaX: 0, deltaY: SHELL_BALL_CANCEL_DELTA_PX }), "cancel");
  assert.equal(
    getShellBallVoicePreview({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    null,
  );
  assert.equal(getShellBallVoicePreview({ deltaX: SHELL_BALL_LOCK_DELTA_PX, deltaY: 0 }), null);

});

test("shell-ball gesture helpers classify vertical intent explicitly for drag-safe voice previews", () => {
  assert.equal(
    getShellBallGestureAxisIntent({
      deltaX: 8,
      deltaY: -SHELL_BALL_LOCK_DELTA_PX,
    }),
    "vertical",
  );

  assert.equal(
    getShellBallGestureAxisIntent({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    "horizontal",
  );

  assert.equal(
    getShellBallGestureAxisIntent({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: 12,
    }),
    "horizontal",
  );
});

test("shell-ball gesture helpers gate voice preview behind vertical-priority intent", () => {
  assert.equal(
    shouldPreviewShellBallVoiceGesture({
      deltaX: 0,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    true,
  );

  assert.equal(
    shouldPreviewShellBallVoiceGesture({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    false,
  );

  assert.equal(
    shouldPreviewShellBallVoiceGesture({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: 12,
    }),
    false,
  );
});

test("shell-ball input bar surfaces voice preview guidance to the UI", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "voice",
      voicePreview: "cancel",
      value: "",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  assert.match(markup, /data-voice-preview="cancel"/);
  assert.match(markup, /Listening has started — speak now/);
  assert.match(markup, /Release to cancel/);
});

test("shell-ball mascot supports passive rendering outside the floating ball host", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "processing",
      motionConfig: getShellBallMotionConfig("processing"),
    }),
  );

  assert.match(markup, /shell-ball-mascot/);
  assert.match(markup, /data-state="processing"/);
});

test("shell-ball release preview recomputes from the final pointer position", () => {
  assert.equal(
    getShellBallVoicePreviewFromEvent({
      startX: 100,
      startY: 100,
      clientX: 100,
      clientY: 52,
      fallbackPreview: null,
    }),
    "lock",
  );

  assert.equal(
    getShellBallVoicePreviewFromEvent({
      startX: 100,
      startY: 100,
      clientX: 100,
      clientY: 148,
      fallbackPreview: null,
    }),
    "cancel",
  );
});

test("shell-ball keeps voice preview alive on leave while voice listening is active", () => {
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("voice_listening"), true);
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("hover_input"), false);
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("voice_locked"), false);
});

test("shell-ball voice recognition resumes after unexpected end while listening or locked", () => {
  assert.equal(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd("voice_listening"), true);
  assert.equal(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd("voice_locked"), true);
  assert.equal(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd("hover_input"), false);

  assert.equal(
    getShellBallVoiceRecognitionUnexpectedEndFallbackState({
      currentState: "voice_locked",
      startState: "idle",
      committedDraft: "",
    }),
    "hover_input",
  );
  assert.equal(
    getShellBallVoiceRecognitionUnexpectedEndFallbackState({
      currentState: "idle",
      startState: "idle",
      committedDraft: "",
    }),
    "idle",
  );
});

test("shell-ball dashboard gesture policy stays task-2 explicit", () => {
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "single_click", state: "idle", interactionConsumed: false }),
    false,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "single_click", state: "hover_input", interactionConsumed: false }),
    false,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "idle", interactionConsumed: false }),
    true,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "hover_input", interactionConsumed: false }),
    true,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "hover_input", interactionConsumed: true }),
    false,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "voice_listening", interactionConsumed: false }),
    false,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "voice_locked", interactionConsumed: false }),
    false,
  );
});

test("shell-ball window measurement expands to overflowing mascot visuals", () => {
  assert.deepEqual(
    measureShellBallContentSize({
      getBoundingClientRect: () => ({ width: 100, height: 80 }),
      scrollWidth: 148,
      scrollHeight: 126,
    }),
    {
      width: 148,
      height: 126,
    },
  );
});

test("shell-ball interaction consumed reducer keeps pointer sequence scope explicit", () => {
  const afterPressStart = mapShellBallInteractionConsumedEventToFlag("press_start");
  assert.equal(afterPressStart, false);

  const afterLongPressVoiceEntry = mapShellBallInteractionConsumedEventToFlag("long_press_voice_entry");
  assert.equal(afterLongPressVoiceEntry, true);
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({
      gesture: "double_click",
      state: "hover_input",
      interactionConsumed: afterLongPressVoiceEntry,
    }),
    false,
  );

  const afterVoiceFlowConsumed = mapShellBallInteractionConsumedEventToFlag("voice_flow_consumed");
  assert.equal(afterVoiceFlowConsumed, true);

  const afterNextPressStart = mapShellBallInteractionConsumedEventToFlag("press_start");
  assert.equal(afterNextPressStart, false);
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({
      gesture: "double_click",
      state: "hover_input",
      interactionConsumed: afterNextPressStart,
    }),
    true,
  );

  const afterForceStateReset = mapShellBallInteractionConsumedEventToFlag("force_state_reset");
  assert.equal(afterForceStateReset, false);
});

test("shell-ball submit reset clears draft retention after submit", () => {
  assert.deepEqual(getShellBallPostSubmitInputReset("summarize this"), {
    nextInputValue: "",
    nextFocused: false,
  });

  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: false,
      hasDraft: false,
    }),
    false,
  );
});

test("shell-ball input bar removes keyboard focus stops outside interactive mode", () => {
  const readonlyMarkup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "readonly",
      voicePreview: null,
      value: "submitted",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  const voiceMarkup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "voice",
      voicePreview: null,
      value: "",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  assert.match(readonlyMarkup, /tabindex="-1"/i);
  assert.match(voiceMarkup, /tabindex="-1"/i);
});

test("shell-ball app drops page-shell copy while preserving the floating shell surface", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp, { isDev: false }));

  assert.doesNotMatch(markup, /shell-ball phase 1/i);
  assert.doesNotMatch(markup, /小胖啾近场承接/);
  assert.doesNotMatch(markup, /demo-only 第一阶段边界/);
  assert.doesNotMatch(markup, /<main/i);
  assert.match(markup, /shell-ball-surface/);
  assert.match(markup, /shell-ball-mascot/);
  assert.doesNotMatch(markup, /shell-ball-bubble-zone/);
  assert.doesNotMatch(markup, /shell-ball-input-bar/);
  assert.doesNotMatch(markup, /Shell-ball demo switcher/);
});

test("shell-ball bubble window owns the bubble zone rendering", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleWindow, {
      visualState: "hover_input",
    }),
  );

  assert.match(markup, /shell-ball-bubble-zone/);
  assert.doesNotMatch(markup, /shell-ball-input-bar/);
});

test("shell-ball coordinator snapshots carry shell-ball-local bubble messages", () => {
  const { useShellBallCoordinator } = withShellBallModuleRuntime("useShellBallCoordinator.ts", {
    react: {
      useEffect() {},
      useMemo<T>(factory: () => T) {
        return factory();
      },
      useRef<T>(value: T) {
        return { current: value };
      },
      useState<T>(value: T) {
        return [typeof value === "function" ? (value as () => T)() : value, () => {}] as const;
      },
    },
    "@tauri-apps/api/window": {
      getCurrentWindow() {
        return { label: shellBallWindowLabels.bubble };
      },
    },
    "../../platform/shellBallWindowController": {
      shellBallWindowLabels,
    },
    "./shellBall.windowSync": require(resolve(desktopRoot, ".cache/shell-ball-tests/features/shell-ball/shellBall.windowSync.js")),
  }, (moduleExports) => moduleExports as { useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator });

  const { snapshot } = useShellBallCoordinator({
    visualState: "hover_input",
    inputValue: "draft",
    finalizedSpeechPayload: null,
    voicePreview: null,
    setInputValue: () => {},
    onFinalizedSpeechHandled: () => {},
    onRegionEnter: () => {},
    onRegionLeave: () => {},
    onInputFocusChange: () => {},
    onSubmitText: () => {},
    onAttachFile: () => {},
    onPrimaryClick: () => {},
  });

  assert.ok(Array.isArray(snapshot.bubbleItems));
  assert.ok(snapshot.bubbleItems.length > 0);
  assert.equal(snapshot.bubbleRegion.strategy, "persistent");
  assert.equal(snapshot.bubbleRegion.hasVisibleItems, true);
  assert.equal(snapshot.bubbleRegion.clickThrough, false);
  assert.equal(snapshot.bubbleItems.at(-1)?.bubble.created_at, "2026-04-11T10:05:00.000Z");
  assert.equal(snapshot.bubbleItems.at(-1)?.desktop.freshnessHint, "fresh");
  assert.equal(snapshot.bubbleItems.at(-1)?.desktop.motionHint, "settle");
});

test("shell-ball bubble zone keeps the latest message visible on feed updates", () => {
  const effects: Array<() => void> = [];
  const scrollElement = {
    scrollHeight: 184,
    scrollTop: 0,
  };
  const refs = [
    { current: scrollElement },
  ];

  const { ShellBallBubbleZone: RuntimeShellBallBubbleZone } = withShellBallModuleRuntime("components/ShellBallBubbleZone.tsx", {
    react: {
      ...require("react"),
      useEffect(callback: () => void) {
        effects.push(callback);
      },
      useRef<T>() {
        return refs.shift() as { current: T };
      },
    },
    "./ShellBallBubbleMessage": {
      ShellBallBubbleMessage() {
        return null;
      },
    },
  }, (moduleExports) => moduleExports as { ShellBallBubbleZone: typeof import("./components/ShellBallBubbleZone").ShellBallBubbleZone });

  RuntimeShellBallBubbleZone({
    visualState: "processing",
    bubbleItems: [
      {
        bubble: {
          bubble_id: "msg-scroll-1",
          task_id: "task-scroll-1",
          type: "status",
          text: "Newest status.",
          pinned: false,
          hidden: false,
          created_at: "2026-04-11T10:08:00.000Z",
        },
        role: "agent",
        desktop: {
          lifecycleState: "visible",
        },
      },
    ],
  });

  assert.equal(effects.length, 1);
  effects[0]?.();
  assert.equal(scrollElement.scrollTop, scrollElement.scrollHeight);
});

test("shell-ball bubble window resolves bubble items from the helper-window snapshot", () => {
  const helperSnapshot = createShellBallWindowSnapshot({
    visualState: "processing",
    inputValue: "",
    voicePreview: null,
    bubbleItems: [
      {
        bubble: {
          bubble_id: "msg-helper-1",
          task_id: "task-helper-1",
          type: "status",
          text: "Drafting your update.",
          pinned: false,
          hidden: false,
          created_at: "2026-04-11T10:04:00.000Z",
        },
        role: "agent",
        desktop: {
          lifecycleState: "visible",
        },
      },
    ],
  });
  helperSnapshot.bubbleRegion = getShellBallBubbleRegionState(helperSnapshot.bubbleItems);
  let capturedProps: Record<string, unknown> | null = null;

  const { ShellBallBubbleWindow: RuntimeShellBallBubbleWindow } = withShellBallModuleRuntime("ShellBallBubbleWindow.tsx", {
    react: require("react"),
    "./useShellBallCoordinator": {
      useShellBallHelperWindowSnapshot() {
        return helperSnapshot;
      },
    },
    "./useShellBallWindowMetrics": {
      useShellBallWindowMetrics() {
        return { rootRef: null };
      },
    },
    "./components/ShellBallBubbleZone": {
      ShellBallBubbleZone(props: Record<string, unknown>) {
        capturedProps = { ...(capturedProps ?? {}), ...props };
        return createElement("section", { className: "shell-ball-bubble-zone-stub" });
      },
    },
  }, (moduleExports) => moduleExports as { ShellBallBubbleWindow: typeof import("./ShellBallBubbleWindow").ShellBallBubbleWindow });

  renderToStaticMarkup(createElement(RuntimeShellBallBubbleWindow, null));

  assert.notEqual(capturedProps, null);
  const resolvedProps = capturedProps as unknown as Record<string, unknown>;

  assert.deepEqual(resolvedProps.visualState, "processing");
  assert.deepEqual(resolvedProps.bubbleItems, getShellBallVisibleBubbleItems(helperSnapshot.bubbleItems));
  assert.equal(typeof resolvedProps.onDeleteBubble, "function");
  assert.equal(typeof resolvedProps.onPinBubble, "function");
});

test("shell-ball bubble window does not depend on only visualState to render its body", () => {
  const helperSnapshot = createShellBallWindowSnapshot({
    visualState: "idle",
    inputValue: "",
    voicePreview: null,
    bubbleItems: [
      {
        bubble: {
          bubble_id: "msg-helper-2",
          task_id: "task-helper-2",
          type: "result",
          text: "Open the dashboard.",
          pinned: false,
          hidden: false,
          created_at: "2026-04-11T10:05:00.000Z",
        },
        role: "user",
        desktop: {
          lifecycleState: "visible",
        },
      },
    ],
  });
  helperSnapshot.bubbleRegion = getShellBallBubbleRegionState(helperSnapshot.bubbleItems);
  let capturedProps: Record<string, unknown> | null = null;

  const { ShellBallBubbleWindow: RuntimeShellBallBubbleWindow } = withShellBallModuleRuntime("ShellBallBubbleWindow.tsx", {
    react: require("react"),
    "./useShellBallCoordinator": {
      useShellBallHelperWindowSnapshot() {
        return helperSnapshot;
      },
    },
    "./useShellBallWindowMetrics": {
      useShellBallWindowMetrics(input: Record<string, unknown>) {
        capturedProps = { ...(capturedProps ?? {}), metricsInput: input };
        return { rootRef: null };
      },
    },
    "./components/ShellBallBubbleZone": {
      ShellBallBubbleZone(props: Record<string, unknown>) {
        capturedProps = { ...(capturedProps ?? {}), ...props };
        return createElement("section", { className: "shell-ball-bubble-zone-stub" });
      },
    },
  }, (moduleExports) => moduleExports as { ShellBallBubbleWindow: typeof import("./ShellBallBubbleWindow").ShellBallBubbleWindow });

  renderToStaticMarkup(createElement(RuntimeShellBallBubbleWindow, { visualState: "voice_locked" }));

  assert.notEqual(capturedProps, null);
  const resolvedProps = capturedProps as unknown as Record<string, unknown>;

  assert.deepEqual(resolvedProps.visualState, "voice_locked");
  assert.deepEqual(resolvedProps.bubbleItems, getShellBallVisibleBubbleItems(helperSnapshot.bubbleItems));
  assert.deepEqual(resolvedProps.metricsInput, {
    role: "bubble",
    visible: true,
    clickThrough: helperSnapshot.bubbleRegion.clickThrough,
  });
  assert.equal(typeof resolvedProps.onDeleteBubble, "function");
  assert.equal(typeof resolvedProps.onPinBubble, "function");
});

test("shell-ball bubble zone renders a real message list without placeholder chrome", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleZone, {
      visualState: "processing",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "msg-agent-1",
            task_id: "task-agent-1",
            type: "status",
            text: "I found the latest dashboard status.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:06:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
          },
        },
        {
          bubble: {
            bubble_id: "msg-user-1",
            task_id: "task-user-1",
            type: "result",
            text: "Open it for me.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:06:05.000Z",
          },
          role: "user",
          desktop: {
            lifecycleState: "visible",
          },
        },
      ] satisfies ShellBallBubbleItem[],
    }),
  );

  assert.match(markup, /I found the latest dashboard status\./);
  assert.match(markup, /Open it for me\./);
  assert.match(markup, /shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--agent/);
  assert.match(markup, /shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--user/);
  assert.match(
    markup,
    /<section class="shell-ball-bubble-zone" data-state="processing"><div class="shell-ball-bubble-zone__scroll"><div class="shell-ball-bubble-zone__message-entry"/,
  );
  assert.doesNotMatch(markup, /shell-ball-bubble-zone__shell/);
  assert.doesNotMatch(markup, /shell-ball-bubble-zone__panel|shell-ball-bubble-zone__frame|shell-ball-bubble-zone__card/);
  assert.doesNotMatch(markup, /<header/);
  assert.doesNotMatch(markup, /<input/);
  assert.doesNotMatch(markup, /toolbar/i);
});

test("shell-ball bubble zone renders per-bubble pin and delete controls", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleZone, {
      visualState: "processing",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "msg-agent-pin-1",
            task_id: "task-agent-pin-1",
            type: "status",
            text: "Keep this handy.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:09:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
          },
        },
        {
          bubble: {
            bubble_id: "msg-user-pin-1",
            task_id: "task-user-pin-1",
            type: "result",
            text: "Delete this after review.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:09:05.000Z",
          },
          role: "user",
          desktop: {
            lifecycleState: "visible",
          },
        },
      ] satisfies ShellBallBubbleItem[],
    }),
  );

  assert.match(markup, /shell-ball-bubble-message__pin-control/g);
  assert.match(markup, /shell-ball-bubble-message__delete-control/g);
  assert.equal(markup.match(/data-bubble-action="pin"/g)?.length, 2);
  assert.equal(markup.match(/data-bubble-action="delete"/g)?.length, 2);
});

test("shell-ball coordinator bubble actions pin and delete local items", () => {
  const sourceItems: ShellBallBubbleItem[] = [
    {
      bubble: {
        bubble_id: "msg-action-1",
        task_id: "task-action-1",
        type: "status",
        text: "Pin this.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:10:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "msg-action-2",
        task_id: "task-action-2",
        type: "result",
        text: "Delete this.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:10:05.000Z",
      },
      role: "user",
      desktop: {
        lifecycleState: "visible",
      },
    },
  ];

  const pinnedItems = applyShellBallBubbleAction(sourceItems, {
    action: "pin",
    bubbleId: "msg-action-1",
  });

  assert.equal(pinnedItems[0]?.bubble.pinned, true);
  assert.equal(pinnedItems[1]?.bubble.pinned, false);
  assert.equal(sourceItems[0]?.bubble.pinned, false);

  const remainingItems = applyShellBallBubbleAction(pinnedItems, {
    action: "delete",
    bubbleId: "msg-action-2",
  });

  assert.deepEqual(remainingItems.map((item) => item.bubble.bubble_id), ["msg-action-1"]);

  const unpinnedItems = applyShellBallBubbleAction(pinnedItems, {
    action: "unpin",
    bubbleId: "msg-action-1",
  });

  assert.equal(unpinnedItems[0]?.bubble.pinned, false);
});

test("shell-ball coordinator prefers bubble_message text over empty delivery preview", () => {
  const createdItem = createShellBallAgentBubbleItem(
    {
      task: {
        task_id: "task-bubble-message",
      },
      bubble_message: {
        bubble_id: "bubble-message-1",
        task_id: "task-bubble-message",
        type: "result",
        text: "真实气泡回复",
        pinned: false,
        hidden: true,
        created_at: "2026-04-11T10:10:10.000Z",
      },
      delivery_result: {
        type: "bubble",
        preview_text: "   ",
      },
    } as any,
    "2026-04-11T10:10:20.000Z",
  );

  assert.equal(createdItem.bubble.text, "真实气泡回复");
  assert.equal(createdItem.bubble.hidden, false);
  assert.equal(createdItem.bubble.created_at, "2026-04-11T10:10:10.000Z");
  assert.equal(createdItem.role, "agent");
});

test("shell-ball coordinator bubble actions restore unpinned bubbles by timestamp then id", () => {
  const sourceItems: ShellBallBubbleItem[] = [
    {
      bubble: {
        bubble_id: "msg-order-2",
        task_id: "task-order-2",
        type: "status",
        text: "Pinned later twin.",
        pinned: true,
        hidden: false,
        created_at: "2026-04-11T10:10:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "msg-order-3",
        task_id: "task-order-3",
        type: "result",
        text: "Newest visible bubble.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:11:00.000Z",
      },
      role: "user",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "msg-order-1",
        task_id: "task-order-1",
        type: "status",
        text: "Oldest visible bubble.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:09:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "msg-order-0",
        task_id: "task-order-0",
        type: "status",
        text: "Pinned earlier twin.",
        pinned: true,
        hidden: false,
        created_at: "2026-04-11T10:10:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
  ];

  const unpinnedItems = applyShellBallBubbleAction(sourceItems, {
    action: "unpin",
    bubbleId: "msg-order-2",
  });

  assert.deepEqual(
    unpinnedItems.map((item) => ({
      bubbleId: item.bubble.bubble_id,
      pinned: item.bubble.pinned,
      createdAt: item.bubble.created_at,
    })),
    [
      {
        bubbleId: "msg-order-1",
        pinned: false,
        createdAt: "2026-04-11T10:09:00.000Z",
      },
      {
        bubbleId: "msg-order-0",
        pinned: true,
        createdAt: "2026-04-11T10:10:00.000Z",
      },
      {
        bubbleId: "msg-order-2",
        pinned: false,
        createdAt: "2026-04-11T10:10:00.000Z",
      },
      {
        bubbleId: "msg-order-3",
        pinned: false,
        createdAt: "2026-04-11T10:11:00.000Z",
      },
    ],
  );
});

test("shell-ball detached bubble actions close pinned windows and delete detached bubbles entirely", () => {
  const listeners = new Map<string, (event: { payload: unknown }) => void>();
  const closeCalls: string[] = [];
  let bubbleItemsState: ShellBallBubbleItem[] = [
    {
      bubble: {
        bubble_id: "msg-detached-1",
        task_id: "task-detached-1",
        type: "status",
        text: "Pinned bubble.",
        pinned: true,
        hidden: false,
        created_at: "2026-04-11T10:10:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
      },
    },
    {
      bubble: {
        bubble_id: "msg-detached-2",
        task_id: "task-detached-2",
        type: "result",
        text: "Keep me visible.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:11:00.000Z",
      },
      role: "user",
      desktop: {
        lifecycleState: "visible",
      },
    },
  ];

  const { useShellBallCoordinator } = withShellBallModuleRuntime("useShellBallCoordinator.ts", {
    react: {
      ...require("react"),
      useEffect(callback: () => void) {
        callback();
      },
      useMemo<T>(factory: () => T) {
        return factory();
      },
      useRef<T>(value: T) {
        return { current: value };
      },
      useState<T>(value: T) {
        const resolvedValue = typeof value === "function" ? (value as () => T)() : value;

        if (
          Array.isArray(resolvedValue) &&
          resolvedValue.every((item) => item && typeof item === "object" && "bubble" in item) &&
          bubbleItemsState.length === 0
        ) {
          bubbleItemsState = resolvedValue as ShellBallBubbleItem[];
        }

        return [bubbleItemsState as unknown as T || resolvedValue, (nextValue: T | ((currentValue: T) => T)) => {
          bubbleItemsState = typeof nextValue === "function"
            ? (nextValue as (currentValue: T) => T)(bubbleItemsState as unknown as T) as unknown as ShellBallBubbleItem[]
            : nextValue as unknown as ShellBallBubbleItem[];
        }] as const;
      },
    },
    "@tauri-apps/api/window": {
      getCurrentWindow() {
        return {
          label: shellBallWindowLabels.ball,
          listen(eventName: string, callback: (event: { payload: unknown }) => void) {
            listeners.set(eventName, callback);
            return Promise.resolve(() => {});
          },
          onMoved() {
            return Promise.resolve(() => {});
          },
          onResized() {
            return Promise.resolve(() => {});
          },
          outerPosition() {
            return Promise.resolve({ toLogical: () => ({ x: 0, y: 0 }) });
          },
          outerSize() {
            return Promise.resolve({ toLogical: () => ({ width: 124, height: 104 }) });
          },
          scaleFactor() {
            return Promise.resolve(1);
          },
        };
      },
    },
    "../../platform/shellBallWindowController": {
      SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
      closeShellBallPinnedBubbleWindow(bubbleId: string) {
        closeCalls.push(bubbleId);
        return Promise.resolve();
      },
      emitToShellBallWindowLabel() {
        return Promise.resolve();
      },
      getShellBallPinnedBubbleIdFromLabel() {
        return null;
      },
      getShellBallPinnedBubbleWindowAnchor() {
        return { x: 0, y: 0 };
      },
      getShellBallPinnedBubbleWindowLabel(bubbleId: string) {
        return `shell-ball-bubble-pinned-${bubbleId}`;
      },
      openShellBallPinnedBubbleWindow() {
        return Promise.resolve();
      },
      setShellBallPinnedBubbleWindowVisible() {
        return Promise.resolve();
      },
      shellBallWindowLabels,
    },
    "./shellBall.bubble": require(resolve(desktopRoot, ".cache/shell-ball-tests/features/shell-ball/shellBall.bubble.js")),
    "./shellBall.windowSync": require(resolve(desktopRoot, ".cache/shell-ball-tests/features/shell-ball/shellBall.windowSync.js")),
    "./useShellBallWindowMetrics": {
      getShellBallBubbleAnchor() {
        return { x: 0, y: 0 };
      },
    },
  }, (moduleExports) => moduleExports as { useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator });

  useShellBallCoordinator({
    visualState: "hover_input",
    inputValue: "",
    finalizedSpeechPayload: null,
    voicePreview: null,
    setInputValue: () => {},
    onFinalizedSpeechHandled: () => {},
    onRegionEnter: () => {},
    onRegionLeave: () => {},
    onInputFocusChange: () => {},
    onSubmitText: () => {},
    onAttachFile: () => {},
    onPrimaryClick: () => {},
  });

  listeners.get(shellBallWindowSyncEvents.pinnedWindowDetached)?.({
    payload: { bubbleId: "msg-detached-1" },
  });
  listeners.get(shellBallWindowSyncEvents.bubbleAction)?.({
    payload: { source: "pinned_window", action: "unpin", bubbleId: "msg-detached-1" },
  });

  assert.deepEqual(closeCalls, ["msg-detached-1"]);
  assert.deepEqual(bubbleItemsState.map((item) => ({ bubbleId: item.bubble.bubble_id, pinned: item.bubble.pinned })), [
    { bubbleId: "msg-detached-1", pinned: false },
    { bubbleId: "msg-detached-2", pinned: false },
  ]);

  listeners.get(shellBallWindowSyncEvents.pinnedWindowDetached)?.({
    payload: { bubbleId: "msg-detached-1" },
  });
  listeners.get(shellBallWindowSyncEvents.bubbleAction)?.({
    payload: { source: "pinned_window", action: "delete", bubbleId: "msg-detached-1" },
  });

  assert.deepEqual(closeCalls, ["msg-detached-1", "msg-detached-1"]);
  assert.deepEqual(bubbleItemsState.map((item) => item.bubble.bubble_id), ["msg-detached-2"]);
});

test("shell-ball bubble actions stay coordinator-owned and detached-position free", () => {
  const bubbleActionPayload = {
    source: "pinned_window",
    action: "unpin",
    bubbleId: "msg-action-1",
  } as const;
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const syncSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.windowSync.ts"), "utf8");

  assert.deepEqual(bubbleActionPayload, {
    source: "pinned_window",
    action: "unpin",
    bubbleId: "msg-action-1",
  });
  assert.equal("x" in bubbleActionPayload, false);
  assert.equal("y" in bubbleActionPayload, false);
  assert.equal("position" in bubbleActionPayload, false);
  assert.match(syncSource, /export type ShellBallBubbleAction = "pin" \| "unpin" \| "delete";/);
  assert.match(syncSource, /export type ShellBallBubbleActionSource = "bubble" \| "pinned_window";/);
  assert.match(coordinatorSource, /currentWindow\.listen<ShellBallBubbleActionPayload>\(shellBallWindowSyncEvents\.bubbleAction/);
  assert.match(coordinatorSource, /setBubbleItems\(\(currentItems\) => applyShellBallBubbleAction\(currentItems, payload\)\)/);
});

test("shell-ball pinned bubble windows render one coordinator-owned pinned item and emit detached actions", () => {
  const helperSnapshot = createShellBallWindowSnapshot({
    visualState: "processing",
    inputValue: "",
    voicePreview: null,
    bubbleItems: [
      {
        bubble: {
          bubble_id: "msg-pinned-1",
          task_id: "task-pinned-1",
          type: "status",
          text: "Keep this pinned.",
          pinned: true,
          hidden: false,
          created_at: "2026-04-11T10:12:00.000Z",
        },
        role: "agent",
        desktop: {
          lifecycleState: "visible",
        },
      },
      {
        bubble: {
          bubble_id: "msg-unpinned-1",
          task_id: "task-unpinned-1",
          type: "result",
          text: "Leave this in the region.",
          pinned: false,
          hidden: false,
          created_at: "2026-04-11T10:12:01.000Z",
        },
        role: "user",
        desktop: {
          lifecycleState: "visible",
        },
      },
    ],
  });
  const actions: Array<{ action: string; bubbleId: string; source: string | undefined }> = [];

  const { ShellBallPinnedBubbleWindow: RuntimeShellBallPinnedBubbleWindow } = withShellBallModuleRuntime(
    "ShellBallPinnedBubbleWindow.tsx",
    {
      react: require("react"),
      "./useShellBallCoordinator": {
        useShellBallHelperWindowSnapshot() {
          return helperSnapshot;
        },
        emitShellBallBubbleAction(action: string, bubbleId: string, source?: string) {
          actions.push({ action, bubbleId, source });
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        getShellBallPinnedBubbleIdFromLabel() {
          return "msg-pinned-1";
        },
        getShellBallCurrentWindow() {
          return { label: "shell-ball-bubble-pinned-msg-pinned-1" };
        },
        startShellBallWindowDragging() {
          actions.push({ action: "drag", bubbleId: "msg-pinned-1", source: "window" });
          return Promise.resolve();
        },
      },
    },
    (moduleExports) => moduleExports as {
      ShellBallPinnedBubbleWindow: (props: Record<string, unknown>) => ReturnType<typeof createElement>;
    },
  );

  const markup = renderToStaticMarkup(createElement(RuntimeShellBallPinnedBubbleWindow, null));

  assert.match(markup, /Keep this pinned\./);
  assert.doesNotMatch(markup, /Leave this in the region\./);
  assert.match(markup, /Unpin/);
  assert.match(markup, /Delete/);
});

test("shell-ball detached pinned window contract stays anchored before drag and detached after drag", () => {
  const pinnedWindowSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/ShellBallPinnedBubbleWindow.tsx"),
    "utf8",
  );
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const syncSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.windowSync.ts"), "utf8");

  assert.match(pinnedWindowSource, /startShellBallWindowDragging/);
  assert.match(pinnedWindowSource, /setFollowsShellBallGeometry\(false\)/);
  assert.match(syncSource, /pinnedWindowReady/);
  assert.match(coordinatorSource, /openShellBallPinnedBubbleWindow/);
  assert.match(coordinatorSource, /closeShellBallPinnedBubbleWindow/);
  assert.match(coordinatorSource, /shellBallWindowSyncEvents\.pinnedWindowReady/);
});

test("shell-ball bubble interaction mode stays clickable while visible unpinned bubbles remain", () => {
  assert.deepEqual(
    getShellBallHelperWindowInteractionMode({
      role: "bubble",
      visible: true,
      clickThrough: false,
    }),
    {
      focusable: false,
      ignoreCursorEvents: false,
    },
  );

  assert.deepEqual(
    getShellBallHelperWindowInteractionMode({
      role: "bubble",
      visible: true,
      clickThrough: true,
    }),
    {
      focusable: false,
      ignoreCursorEvents: true,
    },
  );
});

test("shell-ball bubble window styles stay transparent, faded, and motion-ready", () => {
  const shellBallStyles = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.css"), "utf8");
  const mobileBubbleZoneBlock = shellBallStyles.match(
    /@media \(max-width: 720px\)\s*\{[\s\S]*?(\.shell-ball-bubble-zone\s*\{[\s\S]*?\})/,
  )?.[1] ?? "";
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleZone, {
      visualState: "processing",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "msg-style-1",
            task_id: "task-style-1",
            type: "status",
            text: "Draft ready.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-11T10:07:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
            freshnessHint: "fresh",
            motionHint: "settle",
          },
        },
      ] satisfies ShellBallBubbleItem[],
    }),
  );

  assert.match(shellBallStyles, /\.shell-ball-window--bubble\s*\{[\s\S]*background:\s*transparent;/);
  assert.match(shellBallStyles, /\.shell-ball-window--bubble\s*\{[\s\S]*border:\s*0;/);
  assert.match(shellBallStyles, /\.shell-ball-window--bubble\s*\{[\s\S]*box-shadow:\s*none;/);
  assert.match(shellBallStyles, /--shell-ball-helper-width:\s*min\(22rem, calc\(100vw - 1rem\)\);/);
  assert.match(shellBallStyles, /@media \(max-width: 720px\)\s*\{[\s\S]*--shell-ball-helper-width:\s*min\(20rem, calc\(100vw - 0\.75rem\)\);/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone\s*\{[\s\S]*width:\s*var\(--shell-ball-helper-width\);/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone\s*\{[\s\S]*gap:\s*0\.4rem;/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone\s*\{[\s\S]*overflow:\s*hidden;/);
  assert.match(shellBallStyles, /\.shell-ball-input-bar,\s*\.shell-ball-input-bar--hidden\s*\{[\s\S]*width:\s*var\(--shell-ball-helper-width\);/);
  assert.match(mobileBubbleZoneBlock, /min-height:\s*4\.6rem;/);
  assert.match(mobileBubbleZoneBlock, /padding-inline:\s*0;/);
  assert.doesNotMatch(mobileBubbleZoneBlock, /width:/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone__scroll\s*\{[\s\S]*scrollbar-width:\s*none;/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone__scroll\s*\{[\s\S]*align-content:\s*end;/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone__scroll::-webkit-scrollbar\s*\{[\s\S]*display:\s*none;/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-zone__scroll\s*\{[\s\S]*mask-image:\s*linear-gradient\(/);
  assert.match(shellBallStyles, /@keyframes shell-ball-bubble-message-enter/);
  assert.match(
    shellBallStyles,
    /\.shell-ball-bubble-zone__message-entry\[data-freshness="fresh"\]\[data-motion="settle"\]\s*\{[\s\S]*animation:\s*shell-ball-bubble-message-enter/,
  );
  assert.match(markup, /data-freshness="fresh"/);
  assert.match(markup, /data-motion="settle"/);
  assert.match(markup, /shell-ball-bubble-zone__bottom-anchor/);
});

test("shell-ball input window owns the input rendering", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallInputWindow, {
      mode: "interactive",
      voicePreview: null,
      value: "draft",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  assert.match(markup, /shell-ball-input-bar/);
  assert.doesNotMatch(markup, /shell-ball-bubble-zone/);
});

test("shell-ball surface renders the mascot-only floating structure without the demo switcher", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallSurface, {
      visualState: "hover_input",
      voicePreview: null,
      motionConfig: getShellBallMotionConfig("hover_input"),
      onPrimaryClick: () => {},
      onDoubleClick: () => {},
      onRegionEnter: () => {},
      onRegionLeave: () => {},
      onDragStart: () => {},
      onPressStart: () => {},
      onPressMove: () => {},
      onPressEnd: () => false,
      onPressCancel: () => {},
    }),
  );

  assert.match(markup, /shell-ball-surface/);
  assert.match(markup, /shell-ball-mascot/);
  assert.doesNotMatch(markup, /shell-ball-bubble-zone/);
  assert.doesNotMatch(markup, /shell-ball-input-bar/);
  assert.doesNotMatch(markup, /Shell-ball demo switcher/);
  assert.doesNotMatch(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball surface keeps drag and click on the mascot hotspot only", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallSurface, {
      visualState: "hover_input",
      voicePreview: null,
      motionConfig: getShellBallMotionConfig("hover_input"),
      onPrimaryClick: () => {},
      onDoubleClick: () => {},
      onRegionEnter: () => {},
      onRegionLeave: () => {},
      onDragStart: () => {},
      onPressStart: () => {},
      onPressMove: () => {},
      onPressEnd: () => false,
      onPressCancel: () => {},
    }),
  );

  assert.match(markup, /data-shell-ball-zone="interaction"/);
  assert.match(markup, /data-shell-ball-zone="voice-hotspot"/);
  assert.doesNotMatch(markup, /shell-ball-surface__host-drag-zone/);
  assert.match(markup, /shell-ball-surface__interaction-zone/);
});

test("shell-ball mascot hotspot policy keeps single click inert outside locked voice", () => {
  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "voice_locked",
      gesture: "single_click",
      suppressed: false,
    }),
    "primary_click",
  );

  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "idle",
      gesture: "single_click",
      suppressed: false,
    }),
    "noop",
  );

  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "hover_input",
      gesture: "single_click",
      suppressed: false,
    }),
    "noop",
  );
});

test("shell-ball mascot hotspot policy opens dashboard only from resting double click", () => {
  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "idle",
      gesture: "double_click",
      suppressed: false,
    }),
    "double_click",
  );

  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "hover_input",
      gesture: "double_click",
      suppressed: false,
    }),
    "double_click",
  );

  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "voice_locked",
      gesture: "double_click",
      suppressed: false,
    }),
    "noop",
  );
});

test("shell-ball mascot hotspot policy drops suppressed sequences for both click kinds", () => {
  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "voice_locked",
      gesture: "single_click",
      suppressed: true,
    }),
    "noop",
  );

  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "hover_input",
      gesture: "double_click",
      suppressed: true,
    }),
    "noop",
  );
});

test("shell-ball mascot pointer policy accepts only primary-button press sequences", () => {
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_down", button: 0, isPrimary: true, pressHandled: false }),
    "start_press",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_up", button: 0, isPrimary: true, pressHandled: false }),
    "finish_press",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_down", button: 1, isPrimary: true, pressHandled: false }),
    "noop",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_up", button: 2, isPrimary: true, pressHandled: true }),
    "noop",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_down", button: 0, isPrimary: false, pressHandled: false }),
    "noop",
  );
});

test("shell-ball mascot pointer policy keeps cancellation separate from successful release", () => {
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_up", button: 0, isPrimary: true, pressHandled: true }),
    "suppress_gestures",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_cancel", button: 0, isPrimary: true, pressHandled: true }),
    "cleanup_only",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_cancel", button: 1, isPrimary: false, pressHandled: false }),
    "noop",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_cancel", button: 0, isPrimary: false, pressHandled: false }),
    "noop",
  );
  assert.equal(
    getShellBallMascotPointerPhaseAction({ phase: "pointer_cancel", button: -1, isPrimary: true, pressHandled: false }),
    "cleanup_only",
  );
});

test("shell-ball mascot drag policy lets the full hotspot start window dragging after movement", () => {
  assert.equal(
    shouldStartShellBallMascotWindowDrag({
      visualState: "hover_input",
      startX: 100,
      startY: 100,
      clientX: 104,
      clientY: 103,
    }),
    false,
  );

  assert.equal(
    shouldStartShellBallMascotWindowDrag({
      visualState: "idle",
      startX: 100,
      startY: 100,
      clientX: 118,
      clientY: 114,
    }),
    true,
  );

  assert.equal(
    shouldStartShellBallMascotWindowDrag({
      visualState: "voice_listening",
      startX: 100,
      startY: 100,
      clientX: 118,
      clientY: 114,
    }),
    false,
  );
});

test("shell-ball voice swipe contract keeps upward lock and downward cancel explicit", () => {
  assert.equal(
    getShellBallVoicePreviewFromEvent({
      startX: 100,
      startY: 100,
      clientX: 100,
      clientY: 100 - SHELL_BALL_LOCK_DELTA_PX,
      fallbackPreview: null,
    }),
    "lock",
  );

  assert.equal(
    getShellBallVoicePreviewFromEvent({
      startX: 100,
      startY: 100,
      clientX: 100,
      clientY: 100 + SHELL_BALL_CANCEL_DELTA_PX,
      fallbackPreview: null,
    }),
    "cancel",
  );
});

test("shell-ball press cancel policy clears pending press state and cancels active listening", () => {
  assert.equal(getShellBallPressCancelEvent("voice_listening"), "voice_cancel");
  assert.equal(getShellBallPressCancelEvent("hover_input"), null);
  assert.equal(getShellBallPressCancelEvent("voice_locked"), null);
});

test("shell-ball cancel callback path is wired from mascot through app interaction handlers", () => {
  const surfaceSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallSurface.tsx"), "utf8");
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(surfaceSource, /onPressCancel: \(event: PointerEvent<HTMLButtonElement>\) => void;/);
  assert.match(surfaceSource, /onPressCancel=\{onPressCancel\}/);
  assert.match(appSource, /handlePressCancel,/);
  assert.match(appSource, /onPressCancel=\{handlePressCancel\}/);
  assert.match(interactionSource, /function handlePressCancel\(event: PointerEvent<HTMLButtonElement>\)/);
  assert.match(interactionSource, /clearLongPressTimer\(\);/);
  assert.match(interactionSource, /pressStartXRef\.current = null;/);
  assert.match(interactionSource, /pressStartYRef\.current = null;/);
  assert.match(interactionSource, /setCurrentVoicePreview\(null\);/);
  assert.match(interactionSource, /const cancelEvent = getShellBallPressCancelEvent\(/);
  assert.match(interactionSource, /if \(cancelEvent !== null\) \{/);
  assert.match(interactionSource, /dispatch\(cancelEvent\);/);
});

test("shell-ball region leave keeps hover input visible while the text field is focused", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(interactionSource, /function handleRegionLeave\(\) \{[\s\S]*hoverRetained: getHoverRetained\(\),[\s\S]*\}/);
});

test("shell-ball surface passes mascot double-click and drag wiring through the mascot only", () => {
  const surfaceSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallSurface.tsx"), "utf8");

  assert.match(surfaceSource, /onDoubleClick: \(\) => void;/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onDoubleClick=\{onDoubleClick\}/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onHotspotDragStart=\{onDragStart\}/);
  assert.doesNotMatch(surfaceSource, /data-shell-ball-zone="host-drag"/);
  assert.match(surfaceSource, /data-shell-ball-zone="interaction"/);
});

test("shell-ball file overlay ignores global drag during mascot drag and voice gestures", () => {
  assert.equal(
    shouldShowShellBallFileDropOverlay({
      fileDropActive: false,
      globalDragActive: true,
      shellBallWindowDragActive: true,
      visualState: "hover_input",
    }),
    false,
  );
  assert.equal(
    shouldShowShellBallFileDropOverlay({
      fileDropActive: false,
      globalDragActive: true,
      shellBallWindowDragActive: false,
      visualState: "voice_listening",
    }),
    false,
  );
  assert.equal(
    shouldShowShellBallFileDropOverlay({
      fileDropActive: false,
      globalDragActive: true,
      shellBallWindowDragActive: false,
      visualState: "voice_locked",
    }),
    false,
  );
  assert.equal(
    shouldShowShellBallFileDropOverlay({
      fileDropActive: true,
      globalDragActive: false,
      shellBallWindowDragActive: false,
      visualState: "voice_locked",
    }),
    true,
  );
});

test("shell-ball voice recognition source keeps locked sessions from auto-cancelling on unexpected end", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(interactionSource, /if \(recognitionStopReasonRef\.current !== "none"\) \{/);
  assert.match(interactionSource, /if \(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd\(currentState\)\) \{/);
  assert.match(interactionSource, /const committedDraft = preserveUnexpectedVoiceTranscriptDraft\(\);/);
  assert.match(interactionSource, /if \(startVoiceRecognition\(\)\) \{\s*return;\s*\}/);
});

test("shell-ball text drop populates and focuses the input instead of starting a task", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const surfaceSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallSurface.tsx"), "utf8");

  assert.match(interactionSource, /function handleDroppedText\(text: string\) \{/);
  assert.match(interactionSource, /setInputValue\(nextInputValue\);\s*handleInputFocusRequest\(\);/);
  assert.doesNotMatch(interactionSource, /startTaskFromSelectedText/);
  assert.match(appSource, /const handleSurfaceTextDrop = useCallback\(\(text: string\) => \{/);
  assert.match(appSource, /handleDroppedText\(text\);\s*window\.requestAnimationFrame\(\(\) => \{\s*void emitShellBallInputRequestFocus\(Date\.now\(\)\);\s*\}\);/);
  assert.match(appSource, /window\.addEventListener\("dragenter", handleWindowTextDragOver\);/);
  assert.match(appSource, /window\.addEventListener\("dragover", handleWindowTextDragOver\);/);
  assert.match(appSource, /window\.addEventListener\("drop", handleWindowTextDrop\);/);
  assert.match(appSource, /onTextDrop=\{handleSurfaceTextDrop\}/);
  assert.match(appSource, /textDropActive=\{shouldArmShellBallTextDropTarget\(/);
  assert.match(surfaceSource, /onDragEnterCapture=\{handleDragOver\}/);
  assert.match(surfaceSource, /onDropCapture=\{handleDrop\}/);
  assert.match(surfaceSource, /className="shell-ball-surface__text-drop-target"/);
});

test("shell-ball app dashboard-open gate stays blocked for consumed or non-resting double clicks", () => {
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "idle", interactionConsumed: false }),
    true,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "hover_input", interactionConsumed: false }),
    true,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "hover_input", interactionConsumed: true }),
    false,
  );
  assert.equal(
    getShellBallDashboardOpenGesturePolicy({ gesture: "double_click", state: "voice_locked", interactionConsumed: false }),
    false,
  );
});

test("shell-ball demo switcher visibility stays dev-only", () => {
  assert.equal(shouldShowShellBallDemoSwitcher(true), true);
  assert.equal(shouldShowShellBallDemoSwitcher(false), false);
});

test("shell-ball dev layer isolates demo controls from the formal surface", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallDevLayer, {
      value: "idle",
      onChange: () => {},
    }),
  );

  assert.match(markup, /Shell-ball demo controls/);
  assert.match(markup, /Shell-ball demo switcher/);
  assert.match(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball app keeps the reusable surface as the production structure", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp, { isDev: false }));

  assert.match(markup, /Shell-ball floating surface/);
  assert.match(markup, /shell-ball-surface__body/);
  assert.doesNotMatch(markup, /Shell-ball demo switcher/);
  assert.doesNotMatch(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball app injects the demo switcher only in dev mode", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp, { isDev: true }));

  assert.match(markup, /Shell-ball floating surface/);
  assert.match(markup, /shell-ball-surface__body/);
  assert.match(markup, /Shell-ball demo switcher/);
  assert.match(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball input bar mode stays aligned with visual states", () => {
  assert.equal(getShellBallInputBarMode("idle"), "hidden");
  assert.equal(getShellBallInputBarMode("hover_input"), "interactive");
  assert.equal(getShellBallInputBarMode("confirming_intent"), "readonly");
  assert.equal(getShellBallInputBarMode("waiting_auth"), "readonly");
  assert.equal(getShellBallInputBarMode("processing"), "readonly");
  assert.equal(getShellBallInputBarMode("voice_listening"), "hidden");
  assert.equal(getShellBallInputBarMode("voice_locked"), "hidden");
});

test("shell-ball interaction timing constants stay frozen", () => {
  assert.equal(SHELL_BALL_HOVER_INTENT_MS, 360);
  assert.equal(SHELL_BALL_LEAVE_GRACE_MS, 180);
  assert.equal(SHELL_BALL_LONG_PRESS_MS, 1000);
  assert.equal(SHELL_BALL_LOCK_DELTA_PX, 48);
  assert.equal(SHELL_BALL_CANCEL_DELTA_PX, 48);
  assert.equal(SHELL_BALL_VERTICAL_PRIORITY_RATIO, 1.25);
  assert.equal(SHELL_BALL_CONFIRMING_MS, 600);
  assert.equal(SHELL_BALL_WAITING_AUTH_MS, 700);
  assert.equal(SHELL_BALL_PROCESSING_MS, 1200);
});

test("shell-ball motion mapping exposes state-specific accents and animations", () => {
  assert.equal(getShellBallMotionConfig("processing").wingMode, "flutter");
  assert.equal(getShellBallMotionConfig("waiting_auth").accentTone, "amber");
  assert.equal(getShellBallMotionConfig("voice_listening").ringMode, "listening");
  assert.equal(getShellBallMotionConfig("voice_locked").ringMode, "locked");
});

test("shell-ball store defaults to idle and only exposes the visual-state API", () => {
  useShellBallStore.setState({ visualState: "idle" });

  assert.equal(useShellBallStore.getState().visualState, "idle");

  useShellBallStore.getState().setVisualState("processing");

  assert.equal(useShellBallStore.getState().visualState, "processing");
  assert.deepEqual(Object.keys(useShellBallStore.getState()).sort(), ["setVisualState", "visualState"]);

  useShellBallStore.setState({ visualState: "idle" });
});

test("shell-ball interaction hook module exports the thin adapter", () => {
  assert.equal(typeof useShellBallInteraction, "function");
  assert.equal(typeof syncShellBallInteractionController, "function");
});
