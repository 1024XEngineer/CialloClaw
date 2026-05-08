import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";
import {
  createShellBallInteractionController,
  getShellBallGestureAxisIntent,
  getShellBallInputBarMode,
  getShellBallProcessingReturnState,
  shouldPreviewShellBallVoiceGesture,
  getShellBallVoicePreview,
  getShellBallVoicePreviewForHintMode,
  resolveShellBallTransition,
  shouldRetainShellBallHoverInput,
  SHELL_BALL_CANCEL_DELTA_PX,
  SHELL_BALL_CONFIRMING_MS,
  SHELL_BALL_HOVER_INTENT_MS,
  SHELL_BALL_LOCKED_CANCEL_HOLD_MS,
  SHELL_BALL_LEAVE_GRACE_MS,
  SHELL_BALL_LOCK_DELTA_PX,
  SHELL_BALL_LONG_PRESS_MS,
  SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX,
  SHELL_BALL_PROCESSING_MS,
  SHELL_BALL_VERTICAL_PRIORITY_RATIO,
  SHELL_BALL_WAITING_AUTH_MS,
} from "./shellBall.interaction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { collectShellBallSpeechTranscript, composeShellBallSpeechDraft } from "./shellBall.speech";
import {
  compactPageContext,
  mapDesktopWindowSnapshotToPageContext,
  resolveTaskPageContext,
  sanitizePageContextUrl,
} from "../../services/pageContext";
import {
  isShellBallClipboardPromptActive,
  normalizeShellBallFloatingSize,
  resolveShellBallInlineInputMode,
  ShellBallApp,
  shouldRetainShellBallEdgeDockReveal,
  shouldArmShellBallTextDropTarget,
  shouldShowShellBallFileDropOverlay,
  shouldShowShellBallSelectionIndicator,
} from "./ShellBallApp";
import { ShellBallMascot } from "./components/ShellBallMascot";
import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";
import { FloatingPet } from "./components/floating-pet/FloatingPet";
import { getShellBallMascotHotspotGestureAction } from "./components/ShellBallMascot";
import { getShellBallMascotPetState } from "./components/ShellBallMascot";
import { getShellBallMascotPointerPhaseAction } from "./components/ShellBallMascot";
import { shouldSuppressShellBallMascotHotspotGestures } from "./components/ShellBallMascot";
import { extractShellBallDroppedText, resolveShellBallTextDropEffect, ShellBallSurface, shouldAcceptShellBallTextDrop } from "./ShellBallSurface";
import { shellBallWindowLabels, shellBallWindowPermissions } from "../../platform/shellBallWindowController";
import {
  ShellBallInputBar,
} from "./components/ShellBallInputBar";
import {
  clampShellBallInputResizeDimension,
  focusShellBallInputField,
  measureShellBallInputContentWidth,
  resolveShellBallInputAutoWidth,
  resolveShellBallInputFieldHeight,
  resolveShellBallInputFieldWidth,
  resolveShellBallInputMaxHeight,
  resolveShellBallInputMaxWidth,
} from "./components/shellBallInputBar.helpers";
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
  getShellBallInputInteractionState,
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
  clampShellBallHostFrameToVisibleBounds,
  createShellBallWindowGeometry,
  createShellBallWindowFrame,
  getShellBallDockAnimationConfig,
  getShellBallHelperWindowInteractionMode,
  getShellBallParkedDockInsetPx,
  getShellBallBubbleAnchor,
  getShellBallInputAnchor,
  getShellBallVoiceAnchor,
  measureShellBallContentSize,
  resolveShellBallReleaseSnapTarget,
  resolveShellBallDockedHostPosition,
} from "./useShellBallWindowMetrics";
import {
  applyShellBallBubbleAction,
  createShellBallAgentBubbleItem,
  createShellBallRuntimeObservationReply,
  shouldAutoOpenShellBallDeliveryResult,
  sortShellBallBubbleItemsByTimestamp,
} from "./useShellBallCoordinator";
import { respondSecurity } from "./test-stubs/rpcMethods";
import {
  appendShellBallDroppedText,
  createShellBallInputSubmitParams,
  createShellBallTaskStartParams,
  getShellBallPostSubmitInputReset,
  getShellBallDashboardOpenGesturePolicy,
  getShellBallVoiceRecognitionUnexpectedEndFallbackState,
  getShellBallPressCancelEvent,
  resolveShellBallVoiceRecognitionFinalState,
  getShellBallVoicePreviewFromEvent,
  mapShellBallInteractionConsumedEventToFlag,
  shouldLogShellBallSpeechRecognitionError,
  shouldRestoreShellBallSubmitFailureDraft,
  shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd,
  shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd,
  shouldKeepShellBallVoicePreviewOnRegionLeave,
  syncShellBallInteractionController,
  useShellBallInteraction,
} from "./useShellBallInteraction";
import { useShellBallStore } from "../../stores/shellBallStore";
import {
  areShellBallSelectionSnapshotsEqual,
} from "./selection/selection.provider";

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
    destroy?: () => Promise<void> | void;
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

function withSourceModuleRuntime<T>(
  modulePath: string,
  mocks: Record<string, unknown>,
  callback: (moduleExports: Record<string, unknown>) => T,
) {
  const NodeModule = require("node:module") as any;
  const originalLoad = NodeModule._load;
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

  NodeModule._load = function loadSourceModule(request: string, parent: unknown, isMain: boolean) {
    if (request in mocks) {
      return mocks[request];
    }

    return originalLoad(request, parent, isMain);
  };

  const restoreRuntime = () => {
    NodeModule._load = originalLoad;
  };

  try {
    moduleFactory(require, transpiledModule, transpiledModule.exports);
    const result = callback(transpiledModule.exports);
    const maybePromise = result as unknown as PromiseLike<T>;

    if (result && typeof maybePromise.then === "function") {
      return (result as unknown as Promise<T>).finally(restoreRuntime) as T;
    }

    restoreRuntime();
    return result;
  } catch (error) {
    restoreRuntime();
    throw error;
  }
}

function withTrayControllerRuntime<T>(
  invokeDesktopCommand: (command: string) => Promise<void>,
  callback: (mod: { openControlPanelFromTray: () => Promise<void>; calls: string[] }) => Promise<T> | T,
) {
  const NodeModule = require("node:module") as any;
  const originalLoad = NodeModule._load;
  const modulePath = resolve(desktopRoot, ".cache/shell-ball-tests/platform/trayController.js");
  const calls: string[] = [];

  delete require.cache[modulePath];

  NodeModule._load = function loadTrayController(request: string, parent: unknown, isMain: boolean) {
    if (request === "@tauri-apps/api/core") {
      return {
        invoke(command: string) {
          calls.push(command);
          return invokeDesktopCommand(command);
        },
      };
    }

    return originalLoad(request, parent, isMain);
  };

  const loaded = require(modulePath) as {
    openControlPanelFromTray: () => Promise<void>;
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

function renderDashboardRouteSurface(hash: string) {
  const originalWindow = globalThis.window;
  const originalDocument = globalThis.document;
  const originalSVGElement = globalThis.SVGElement;
  const fakeDocument = {
    location: null as unknown,
    querySelector(): Element | null {
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
      state: null as unknown,
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

async function flushAsyncEffects() {
  await new Promise<void>((resolve) => {
    setImmediate(resolve);
  });
}

function createImmediateShellBallReactRuntime(initialBubbleItems: ShellBallBubbleItem[] = []) {
  let bubbleItemsState = [...initialBubbleItems];
  let visualState: string | null = null;

  const shellBallVisualStateSet = new Set<string>(shellBallVisualStates);

  return {
    getBubbleItems() {
      return bubbleItemsState;
    },
    getVisualState() {
      return visualState;
    },
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
          resolvedValue.every((item) => item === undefined || item === null || (typeof item === "object" && "bubble" in item))
        ) {
          if (bubbleItemsState.length === 0) {
            bubbleItemsState = resolvedValue as unknown as ShellBallBubbleItem[];
          }

          return [bubbleItemsState as unknown as T, (nextValue: T | ((currentValue: T) => T)) => {
            bubbleItemsState = typeof nextValue === "function"
              ? (nextValue as (currentValue: T) => T)(bubbleItemsState as unknown as T) as unknown as ShellBallBubbleItem[]
              : nextValue as unknown as ShellBallBubbleItem[];
          }] as const;
        }

        if (typeof resolvedValue === "string" && shellBallVisualStateSet.has(resolvedValue)) {
          visualState = resolvedValue;

          return [resolvedValue, (nextValue: T | ((currentValue: T) => T)) => {
            const nextResolved = typeof nextValue === "function"
              ? (nextValue as (currentValue: T) => T)(resolvedValue)
              : nextValue;

            if (typeof nextResolved === "string" && shellBallVisualStateSet.has(nextResolved)) {
              visualState = nextResolved;
            }
          }] as const;
        }

        return [resolvedValue, () => {}] as const;
      },
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

void invalidTransitionResultMissingMs;
void invalidTransitionResultMissingTarget;

test("shell-ball visual states preserve the frozen seven-state contract", () => {
  assert.deepEqual(shellBallVisualStates, [
    "idle",
    "hover_input",
    "confirming_intent",
    "processing",
    "waiting_auth",
    "voice_listening",
    "voice_locked",
  ]);
});

test("shell-ball desktop host no longer creates bubble, input, and voice helper windows", () => {
  assert.equal(existsSync(resolve(desktopRoot, "shell-ball-bubble.html")), false);
  assert.equal(existsSync(resolve(desktopRoot, "shell-ball-input.html")), false);
  assert.equal(existsSync(resolve(desktopRoot, "shell-ball-voice.html")), false);
  assert.equal(existsSync(resolve(desktopRoot, "src/app/shell-ball-voice/main.tsx")), false);

  const viteConfig = readFileSync(resolve(desktopRoot, "vite.config.ts"), "utf8");
  const tauriConfig = readFileSync(resolve(desktopRoot, "src-tauri/tauri.conf.json"), "utf8");

  assert.doesNotMatch(viteConfig, /"shell-ball-bubble"/);
  assert.doesNotMatch(viteConfig, /"shell-ball-input"/);
  assert.doesNotMatch(viteConfig, /"shell-ball-voice"/);
  assert.doesNotMatch(tauriConfig, /"label": "shell-ball-bubble"/);
  assert.doesNotMatch(tauriConfig, /"label": "shell-ball-input"/);
  assert.doesNotMatch(tauriConfig, /"label": "shell-ball-voice"/);
  assert.doesNotMatch(tauriConfig, /"url": "shell-ball-bubble\.html"/);
  assert.doesNotMatch(tauriConfig, /"url": "shell-ball-input\.html"/);
  assert.doesNotMatch(tauriConfig, /"url": "shell-ball-voice\.html"/);
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
    voice: "shell-ball-voice",
  });

  assert.equal(shellBallWindowPermissions.includes("core:window:allow-set-position"), true);
  assert.equal(shellBallWindowPermissions.includes("core:window:allow-set-size"), true);
  assert.equal(shellBallWindowPermissions.includes("core:window:allow-start-dragging"), true);

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
    "shell-ball-bubble-pinned-*",
    "dashboard",
    "control-panel",
    "onboarding",
  ]);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-create"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-set-position"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-set-size"), true);
  assert.equal(parsedCapabilityConfig.permissions.includes("core:window:allow-start-dragging"), true);

  const generatedCapabilitySchema = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/gen/schemas/capabilities.json"), "utf8"),
  ) as {
    default: {
      windows: string[];
      permissions: string[];
    };
    "control-panel-destroy": {
      windows: string[];
      permissions: string[];
    };
  };

  assert.deepEqual(generatedCapabilitySchema.default.windows, parsedCapabilityConfig.windows);
  assert.deepEqual(generatedCapabilitySchema.default.permissions, parsedCapabilityConfig.permissions);
  assert.equal(generatedCapabilitySchema.default.permissions.includes("core:window:allow-create"), true);
  assert.equal(generatedCapabilitySchema.default.permissions.includes("core:window:allow-unminimize"), true);
});

test("desktop internal window classifiers share the same native allowlist", () => {
  const mainSource = readFileSync(resolve(desktopRoot, "src-tauri/src/main.rs"), "utf8");
  const internalWindowsSource = readFileSync(resolve(desktopRoot, "src-tauri/src/internal_windows.rs"), "utf8");
  const windowContextSource = readFileSync(resolve(desktopRoot, "src-tauri/src/window_context/windows.rs"), "utf8");
  const selectionWindowsSource = readFileSync(resolve(desktopRoot, "src-tauri/src/selection/windows.rs"), "utf8");

  assert.match(mainSource, /mod internal_windows;/);
  assert.match(internalWindowsSource, /pub const INTERNAL_WINDOW_LABELS: \[&str; 7\] = \[/);
  assert.match(internalWindowsSource, /"dashboard"/);
  assert.match(internalWindowsSource, /"control-panel"/);
  assert.match(internalWindowsSource, /pub const INTERNAL_PINNED_WINDOW_PREFIX: &str = "shell-ball-bubble-pinned-";/);
  assert.match(windowContextSource, /use crate::internal_windows::\{INTERNAL_PINNED_WINDOW_PREFIX, INTERNAL_WINDOW_LABELS\};/);
  assert.match(windowContextSource, /const INTERNAL_WINDOW_CONTEXT_REUSE_MAX_AGE_MS: u64 = 10_000;/);
  assert.match(windowContextSource, /fn read_fresh_cached_window_context\(\) -> Option<CachedWindowContext> \{/);
  assert.match(windowContextSource, /cached\.cached_at\.elapsed\(\)\s*>\s*Duration::from_millis\(INTERNAL_WINDOW_CONTEXT_REUSE_MAX_AGE_MS\)/);
  assert.match(windowContextSource, /\*cached_context = None;/);
  assert.match(selectionWindowsSource, /use crate::internal_windows::\{INTERNAL_PINNED_WINDOW_PREFIX, INTERNAL_WINDOW_LABELS\};/);
  assert.match(selectionWindowsSource, /for label in INTERNAL_WINDOW_LABELS/);
  assert.doesNotMatch(selectionWindowsSource, /const SHELL_BALL_WINDOW_LABELS/);
  assert.doesNotMatch(selectionWindowsSource, /const SHELL_BALL_PINNED_WINDOW_PREFIX/);
});

test("shell-ball tray hide and show paths target the merged shell-ball host", () => {
  const mainSource = readFileSync(
    resolve(desktopRoot, "src-tauri/src/main.rs"),
    "utf8",
  );

  assert.doesNotMatch(mainSource, /const SHELL_BALL_INPUT_WINDOW_LABEL: &str = "shell-ball-input";/);
  assert.doesNotMatch(mainSource, /const SHELL_BALL_VOICE_WINDOW_LABEL: &str = "shell-ball-voice";/);
  assert.match(mainSource, /if let Some\(window\) = app\.get_webview_window\(SHELL_BALL_WINDOW_LABEL\) \{/);
  assert.doesNotMatch(mainSource, /SHELL_BALL_BUBBLE_WINDOW_LABEL/);
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
  ) as Record<string, {
    windows: string[];
    permissions: string[];
  }> & {
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
  assert.deepEqual(generatedCapabilitySchema["control-panel-destroy"].windows, ["control-panel"]);
  assert.deepEqual(generatedCapabilitySchema["control-panel-destroy"].permissions, ["core:window:allow-destroy"]);
});

test("shell-ball window controller caches helper handles for drag-time updates", () => {
  const controllerSource = readFileSync(
    resolve(desktopRoot, "src/platform/shellBallWindowController.ts"),
    "utf8",
  );

  assert.match(controllerSource, /const shellBallWindowHandleCache = new Map<string, Window>\(\);/);
  assert.match(controllerSource, /shellBallWindowHandleCache\.set\(currentWindow\.label, currentWindow\);/);
  assert.match(controllerSource, /const cachedWindowHandle = shellBallWindowHandleCache\.get\(label\);/);
  assert.match(controllerSource, /if \(cachedWindowHandle !== undefined\) \{\s*return cachedWindowHandle;\s*\}/);
  assert.match(controllerSource, /shellBallWindowHandleCache\.set\(label, windowHandle\);/);
});

test("shell-ball drag sync keeps geometry publishes coalesced and visibility-aware", () => {
  const metricsSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallWindowMetrics.ts"),
    "utf8",
  );
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");

  assert.match(metricsSource, /const scheduleBallGeometryEmit = useCallback\(\(geometry: ShellBallWindowGeometry\) => \{/);
  assert.match(metricsSource, /const scheduleBallGeometryPublish = useCallback\(\(input\?: \{ snapToBounds\?: boolean \}\) => \{/);
  assert.match(metricsSource, /const pendingBallDragFrameRef = useRef<ShellBallWindowFrame \| null>\(null\);/);
  assert.match(metricsSource, /pendingBallDragFrameRef\.current = nextFrame;/);
  assert.match(metricsSource, /if \(ballDragPositionQueueRef\.current !== null\) \{\s*return ballDragPositionQueueRef\.current;\s*\}/);
  assert.match(metricsSource, /while \(pendingBallDragFrameRef\.current !== null\) \{/);
  assert.match(metricsSource, /scheduleBallGeometryEmit\(geometryRef\.current\);/);
  assert.match(metricsSource, /if \(ballDragSessionRef\.current !== null && !input\?\.snapToBounds\) \{/);
  assert.match(appSource, /helperVisibility: \{[\s\S]*bubble: false,[\s\S]*input: false,[\s\S]*voice: false,[\s\S]*\}/);
});

test("dashboard stays hidden on cold launch while control-panel is created on demand", () => {
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
  assert.equal(controlPanelWindow, undefined);
  assert.equal(dashboardWindow.visible, false);
  assert.equal(dashboardWindow.decorations, false);
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
  const shellBallSurfaceBlock = shellBallStyles.match(/\.shell-ball-surface\s*\{([\s\S]*?)\}/)?.[1] ?? "";
  const shellBallSurfaceBeforeBlock = shellBallStyles.match(/\.shell-ball-surface::before\s*\{([\s\S]*?)\}/)?.[1] ?? "";
  const mascotBlock = shellBallStyles.match(/\.shell-ball-mascot\s*\{([\s\S]*?)\}/)?.[1] ?? "";
  const mascotHotspotBlock = shellBallStyles.match(/\.shell-ball-mascot__hotspot\s*\{([\s\S]*?)\}/)?.[1] ?? "";

  assert.doesNotMatch(shellBallSurfaceBeforeBlock, /background:/);
  assert.doesNotMatch(shellBallSurfaceBlock, /overflow-x:\s*hidden/);
  assert.match(mascotBlock, /width:\s*clamp\(/);
  assert.match(mascotHotspotBlock, /inset:\s*0;/);
  assert.match(shellBallStyles, /data-floating-ball-size="small"/);
  assert.match(shellBallStyles, /data-floating-ball-size="medium"/);
  assert.match(shellBallStyles, /data-floating-ball-size="large"/);
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

  assert.doesNotMatch(tauriConfig, /"focusable": false/);
  assert.match(controllerSource, /setShellBallWindowFocusable\([^)]*focusable: boolean\)/);
  assert.match(controllerSource, /setShellBallWindowIgnoreCursorEvents\([^)]*ignore: boolean\)/);
  assert.match(metricsSource, /getShellBallHelperWindowInteractionMode/);
  assert.doesNotMatch(metricsSource, /setFocus\(\)/);
  assert.doesNotMatch(inputBarSource, /focus\(\{ preventScroll: true \}\)/);
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
  assert.match(dashboardHomeSource, /function getRouteForModule\(module: DashboardHomeModuleKey\)/);
  assert.match(dashboardHomeSource, /return resolveDashboardModuleRoutePath\(module\);/);
  assert.match(dashboardHomeSource, /const nextPath = getRouteForModule\(module\);/);
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
  const controlPanelCapabilityConfig = JSON.parse(
    readFileSync(resolve(desktopRoot, "src-tauri/capabilities/control-panel-destroy.json"), "utf8"),
  ) as { permissions: string[]; windows: string[] };

  assert.equal(capabilityConfig.permissions.includes("core:window:allow-unminimize"), true);
  assert.equal(capabilityConfig.permissions.includes("core:window:allow-minimize"), true);
  assert.equal(capabilityConfig.permissions.includes("core:window:allow-set-fullscreen"), true);
  assert.equal(capabilityConfig.permissions.includes("core:window:allow-destroy"), false);
  assert.deepEqual(controlPanelCapabilityConfig.windows, ["control-panel"]);
  assert.equal(controlPanelCapabilityConfig.permissions.includes("core:window:allow-destroy"), true);

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
      expectedCalls: ["onCloseRequested", "preventDefault", "destroy"],
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
      async destroy() {
        calls.push("destroy");
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

test("hide-on-close helper lets a destroy-triggered second close continue without prevention", async () => {
  const calls: string[] = [];
  let closeHandler: ((event: { preventDefault: () => void }) => Promise<void> | void) | null = null;

  await withHideOnCloseRequestRuntime({
    __calls__: calls,
    label: "control-panel",
    onCloseRequested(handler) {
      closeHandler = handler;
      return "unlisten";
    },
    async destroy() {
      calls.push("destroy");
      await closeHandler?.({
        preventDefault() {
          calls.push("preventDefault:second-close");
        },
      });
    },
    async hide() {
      calls.push("hide");
    },
  }, async ({ installHideOnCloseRequest }) => {
    installHideOnCloseRequest();
    await closeHandler?.({
      preventDefault() {
        calls.push("preventDefault:first-close");
      },
    });
  });

  assert.deepEqual(calls, ["preventDefault:first-close", "destroy"]);
});

test("hide-on-close helper still prevents repeated close requests for hide-on-close windows", async () => {
  const calls: string[] = [];
  let closeHandler: ((event: { preventDefault: () => void }) => Promise<void> | void) | null = null;

  await withHideOnCloseRequestRuntime({
    __calls__: calls,
    label: "dashboard",
    onCloseRequested(handler) {
      closeHandler = handler;
      return "unlisten";
    },
    async hide() {
      calls.push("hide");
      await closeHandler?.({
        preventDefault() {
          calls.push("preventDefault:second-close");
        },
      });
    },
  }, async ({ installHideOnCloseRequest }) => {
    installHideOnCloseRequest();
    await closeHandler?.({
      preventDefault() {
        calls.push("preventDefault:first-close");
      },
    });
  });

  assert.deepEqual(calls, ["preventDefault:first-close", "requestShellBallDashboardCloseTransition", "hide", "preventDefault:second-close"]);
});

test("dashboard and control-panel entrypoints install hide-on-close handling", () => {
  const dashboardMainSource = readFileSync(resolve(desktopRoot, "src/app/dashboard/main.tsx"), "utf8");
  const controlPanelMainSource = readFileSync(resolve(desktopRoot, "src/app/control-panel/main.tsx"), "utf8");

  assert.match(dashboardMainSource, /installHideOnCloseRequest/);
  assert.match(dashboardMainSource, /void installHideOnCloseRequest\(\)/);
  assert.match(controlPanelMainSource, /installHideOnCloseRequest/);
  assert.match(controlPanelMainSource, /void installHideOnCloseRequest\(\)/);
});

test("onboarding window is card-sized, interactive, and promoted natively", () => {
  const mainSource = readFileSync(resolve(desktopRoot, "src-tauri/src/main.rs"), "utf8");
  const onboardingControllerSource = readFileSync(resolve(desktopRoot, "src/platform/onboardingWindowController.ts"), "utf8");
  const onboardingWindowSource = readFileSync(resolve(desktopRoot, "src/features/onboarding/OnboardingWindow.tsx"), "utf8");
  const onboardingServiceSource = readFileSync(resolve(desktopRoot, "src/features/onboarding/onboardingService.ts"), "utf8");
  const controlPanelAppSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");

  assert.match(
    mainSource,
    /WebviewWindowBuilder::new\(\s*&handle,\s*ONBOARDING_WINDOW_LABEL,\s*WebviewUrl::App\("onboarding\.html"\.into\(\)\),[\s\S]*?\.inner_size\(460\.0, 340\.0\)[\s\S]*?\.visible\(false\)\s*[\s\S]*?\.focused\(false\)/,
  );
  assert.match(mainSource, /set_window_ignore_cursor_events\(hwnd, false\)/);
  assert.doesNotMatch(mainSource, /fn desktop_recreate_onboarding/);
  assert.match(mainSource, /fn desktop_promote_onboarding\(app: tauri::AppHandle\) -> Result<\(\), String> \{[\s\S]*?SetWindowPos\(/);
  assert.match(mainSource, /desktop_promote_onboarding,/);
  assert.match(onboardingControllerSource, /const ONBOARDING_CARD_WINDOW_WIDTH = 460/);
  assert.match(onboardingControllerSource, /const ONBOARDING_CARD_WINDOW_HEIGHT = 340/);
  assert.match(onboardingControllerSource, /function resolveOnboardingCardWindowFrame/);
  assert.match(onboardingControllerSource, /export async function getOnboardingWindow\(\)/);
  assert.match(onboardingControllerSource, /await setOnboardingIgnoreCursorEvents\(false\)/);
  assert.match(
    onboardingControllerSource,
    /export async function showOnboardingWindow\(\) \{[\s\S]*?await setOnboardingIgnoreCursorEvents\(false\);[\s\S]*?await onboardingWindow\.setFocusable\(true\);[\s\S]*?await onboardingWindow\.setAlwaysOnTop\(true\);[\s\S]*?await invoke\("desktop_promote_onboarding"\);[\s\S]*?\}/,
  );
  assert.doesNotMatch(onboardingWindowSource, /loadInitialOnboardingStateFromUrl/);
  assert.doesNotMatch(onboardingWindowSource, /setOnboardingInteractiveRegions/);
  assert.match(onboardingServiceSource, /let desktopOnboardingLaunchPromise: Promise<DesktopOnboardingSession \| null> \| null = null/);
  assert.match(onboardingServiceSource, /export async function startManualDesktopOnboardingReplay/);
  assert.match(onboardingServiceSource, /currentWindow\.emitTo\(label, desktopOnboardingEvents\.sessionChanged, session\)/);
  assert.match(onboardingServiceSource, /await showOnboardingWindow\(\)/);
  assert.match(controlPanelAppSource, /const \[isReplayingOnboarding, setIsReplayingOnboarding\] = useState\(false\)/);
  assert.match(controlPanelAppSource, /const onboardingReplayDisabled = isSaving \|\| isRunningInspection \|\| isReplayingOnboarding/);
  assert.match(controlPanelAppSource, /void ensureOnboardingWindow\(\)\.catch/);
  assert.doesNotMatch(controlPanelAppSource, /showShellBallWindow\("ball"\)/);
  assert.doesNotMatch(controlPanelAppSource, /5_000/);
  assert.doesNotMatch(controlPanelAppSource, /retriedSession/);
});

test("hide-on-close helper destroys onboarding before closing the control panel", () => {
  const hideOnCloseSource = readFileSync(resolve(desktopRoot, "src/platform/hideOnCloseRequest.ts"), "utf8");
  assert.match(hideOnCloseSource, /await destroyOnboardingWindow\(\)/);
});

test("control-panel entrypoint and view keep frameless window close and drag controls wired", () => {
  const controlPanelMainSource = readFileSync(resolve(desktopRoot, "src/app/control-panel/main.tsx"), "utf8");
  const controlPanelAppSource = readFileSync(resolve(desktopRoot, "src/features/control-panel/ControlPanelApp.tsx"), "utf8");
  const desktopWindowFrameSource = readFileSync(resolve(desktopRoot, "src/platform/desktopWindowFrame.ts"), "utf8");

  assert.match(controlPanelMainSource, /installDesktopEscapeClose/);
  assert.match(controlPanelMainSource, /installDesktopEscapeClose\(\)/);
  assert.match(controlPanelAppSource, /startCurrentDesktopWindowDragging/);
  assert.match(controlPanelAppSource, /requestCurrentDesktopWindowClose/);
  assert.match(controlPanelAppSource, /minimizeCurrentDesktopWindow/);
  assert.match(desktopWindowFrameSource, /export async function minimizeCurrentDesktopWindow\(\)/);
  assert.match(desktopWindowFrameSource, /export function installDesktopEscapeClose\(windowHandle\?: DesktopCloseHandle \| null\)/);
  assert.match(desktopWindowFrameSource, /const currentWindow = windowHandle \?\? getDesktopFrameWindow\(\)/);
  assert.match(controlPanelAppSource, /control-panel-shell__titlebar/);
  assert.match(controlPanelAppSource, /拖动控制面板窗口/);
  assert.match(controlPanelAppSource, /关闭控制面板/);
});

test("tray controller opens the control panel through the desktop host command", async () => {
  await withTrayControllerRuntime(async () => undefined, async ({ openControlPanelFromTray, calls }) => {
    await openControlPanelFromTray();

    assert.deepEqual(calls, ["desktop_open_or_focus_control_panel"]);
  });
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
    textSelectionState: "desktop-shell-ball:text-selection-state",
    selectionSnapshot: "desktop-shell-ball:selection-snapshot",
    pinnedWindowReady: "desktop-shell-ball:pinned-window-ready",
    pinnedWindowDetached: "desktop-shell-ball:pinned-window-detached",
    bubbleHover: "desktop-shell-ball:bubble-hover",
    inputHover: "desktop-shell-ball:input-hover",
    inputFocus: "desktop-shell-ball:input-focus",
    inputRequestFocus: "desktop-shell-ball:input-request-focus",
    inputDraft: "desktop-shell-ball:input-draft",
    primaryAction: "desktop-shell-ball:primary-action",
    pendingFileAction: "desktop-shell-ball:pending-file-action",
    intentDecision: "desktop-shell-ball:intent-decision",
    bubbleAction: "desktop-shell-ball:bubble-action",
  });

  assert.deepEqual(getShellBallHelperWindowVisibility("idle"), {
    bubble: false,
    input: true,
    voice: false,
  });

  assert.deepEqual(getShellBallHelperWindowVisibility("hover_input"), {
    bubble: false,
    input: true,
    voice: false,
  });

  assert.deepEqual(getShellBallHelperWindowVisibility("voice_locked"), {
    bubble: false,
    input: false,
    voice: false,
  });

  assert.deepEqual(getShellBallHelperWindowVisibility("voice_locked", true, "hidden", "cancel"), {
    bubble: false,
    input: false,
    voice: true,
  });

  assert.deepEqual(
    createShellBallWindowSnapshot({
      visualState: "voice_locked",
      voiceHintMode: "hidden",
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
      voiceHintMode: "hidden",
      inputBarMode: "hidden",
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
        clickThrough: true,
      },
      inputInteraction: {
        clickThrough: false,
      },
      visibility: {
        bubble: false,
        input: false,
        voice: false,
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
  assert.equal(SHELL_BALL_INPUT_GAP_PX, 4);
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
      y: 180,
    },
  );

  assert.equal(180 + 90 <= 300, true);

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
      y: 408,
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

  assert.deepEqual(
    createShellBallWindowGeometry({
      position: {
        x: 292,
        y: -16,
      },
      size: {
        width: 124,
        height: 104,
      },
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      scaleFactor: 1,
    }),
    {
      ballFrame: {
        x: 196,
        y: 0,
        width: 124,
        height: 104,
      },
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      scaleFactor: 1,
    },
  );

  const mascotFrame = {
    x: 20,
    y: 24,
    width: 100,
    height: 120,
  };

  assert.equal(
    getShellBallParkedDockInsetPx({
      side: "left",
      mascotFrame,
    }),
    50,
  );
  assert.ok(
    Math.abs(
      getShellBallParkedDockInsetPx({
        side: "top",
        mascotFrame,
      }) - 21.6,
    ) < 0.000001,
  );
  assert.ok(
    Math.abs(
      getShellBallParkedDockInsetPx({
        side: "bottom",
        mascotFrame,
      }) - 33.6,
    ) < 0.000001,
  );
  assert.ok(
    getShellBallParkedDockInsetPx({
      side: "top",
      mascotFrame,
    }) < getShellBallParkedDockInsetPx({
      side: "bottom",
      mascotFrame,
    }),
  );
  assert.deepEqual(
    resolveShellBallDockedHostPosition({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      currentPosition: { x: 144, y: 200 },
      edgeDockState: {
        side: "top",
        revealed: false,
      },
      mascotFrame,
    }),
    {
      x: 144,
      y: -46,
    },
  );
  assert.deepEqual(
    resolveShellBallDockedHostPosition({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      currentPosition: { x: 144, y: 200 },
      edgeDockState: {
        side: "left",
        revealed: false,
      },
      mascotFrame,
    }),
    {
      x: -70,
      y: 200,
    },
  );
  assert.deepEqual(
    resolveShellBallDockedHostPosition({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      currentPosition: { x: 144, y: 200 },
      edgeDockState: {
        side: "bottom",
        revealed: false,
      },
      mascotFrame,
    }),
    {
      x: 144,
      y: 410,
    },
  );
  assert.deepEqual(
    resolveShellBallDockedHostPosition({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      currentPosition: { x: 144, y: 200 },
      edgeDockState: {
        side: "top_left",
        revealed: false,
      },
      mascotFrame,
    }),
    {
      x: -70,
      y: -46,
    },
  );
  assert.deepEqual(
    resolveShellBallDockedHostPosition({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      currentPosition: { x: 144, y: 200 },
      edgeDockState: {
        side: "bottom_right",
        revealed: false,
      },
      mascotFrame,
    }),
    {
      x: 250,
      y: 410,
    },
  );
  assert.deepEqual(
    getShellBallDockAnimationConfig({
      side: "left",
      mode: "dock",
    }),
    {
      durationMs: 180,
      x: {
        direction: -1,
        overshootPx: 6,
      },
    },
  );
  assert.deepEqual(
    getShellBallDockAnimationConfig({
      side: "top",
      mode: "dock",
    }),
    {
      durationMs: 220,
      y: {
        direction: -1,
        overshootPx: 8,
      },
    },
  );
  assert.deepEqual(
    getShellBallDockAnimationConfig({
      side: "top_left",
      mode: "dock",
    }),
    {
      durationMs: 220,
      x: {
        direction: -1,
        overshootPx: 6,
      },
      y: {
        direction: -1,
        overshootPx: 8,
      },
    },
  );
  assert.deepEqual(
    getShellBallDockAnimationConfig({
      side: "bottom",
      mode: "reveal",
    }),
    {
      durationMs: 220,
      y: {
        direction: 1,
        overshootPx: 0,
      },
    },
  );
  assert.equal(
    resolveShellBallReleaseSnapTarget({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      hostFrame: {
        x: 10,
        y: 6,
        width: 124,
        height: 104,
      },
      mascotFrame,
    }),
    "top_left",
  );
  assert.equal(
    resolveShellBallReleaseSnapTarget({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      hostFrame: {
        x: 100,
        y: 200,
        width: 124,
        height: 104,
      },
      mascotFrame,
    }),
    null,
  );
  assert.equal(
    resolveShellBallReleaseSnapTarget({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      hostFrame: {
        x: 172,
        y: 368,
        width: 124,
        height: 104,
      },
      mascotFrame,
    }),
    "bottom_right",
  );
  assert.equal(
    resolveShellBallReleaseSnapTarget({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      hostFrame: {
        x: 154,
        y: 200,
        width: 124,
        height: 104,
      },
      mascotFrame,
    }),
    null,
  );
  assert.equal(
    resolveShellBallReleaseSnapTarget({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      hostFrame: {
        x: 172,
        y: 200,
        width: 124,
        height: 104,
      },
      mascotFrame,
      thresholdPx: 32,
    }),
    "right",
  );
  assert.equal(
    resolveShellBallReleaseSnapTarget({
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      hostFrame: {
        x: 130,
        y: 200,
        width: 124,
        height: 104,
      },
      mascotFrame,
      thresholdPx: 24,
    }),
    null,
  );
  assert.deepEqual(
    clampShellBallFrameToBounds(
      {
        x: 280,
        y: 470,
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
      x: 196,
      y: 416,
      width: 124,
      height: 104,
    },
  );
  assert.deepEqual(
    clampShellBallHostFrameToVisibleBounds({
      hostFrame: {
        x: -60,
        y: -48,
        width: 124,
        height: 104,
      },
      bounds: {
        minX: 0,
        minY: 0,
        maxX: 320,
        maxY: 520,
      },
      mascotFrame: {
        x: 20,
        y: 24,
        width: 100,
        height: 120,
      },
    }),
    {
      x: -20,
      y: -24,
      width: 124,
      height: 104,
    },
  );

  const metricsSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallWindowMetrics.ts"),
    "utf8",
  );
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");
  const mascotSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallMascot.tsx"), "utf8");
  const surfaceSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallSurface.tsx"), "utf8");

  assert.match(metricsSource, /const beginBallWindowPointerDrag = useCallback\(\(pointerStart: ShellBallPointerPosition\) => \{/);
  assert.match(metricsSource, /const updateBallWindowPointerDrag = useCallback\(\(pointer: ShellBallPointerPosition\) => \{/);
  assert.match(metricsSource, /const endBallWindowPointerDrag = useCallback\(async \(pointer\?: ShellBallPointerPosition\) => \{/);
  assert.match(metricsSource, /ballDragMoveAnimationFrameRef = useRef<number \| null>\(null\)/);
  assert.match(metricsSource, /const scheduleBallGeometryEmit = useCallback\(\(geometry: ShellBallWindowGeometry\) => \{/);
  assert.match(metricsSource, /const scheduleBallGeometryPublish = useCallback\(\(input\?: \{ snapToBounds\?: boolean \}\) => \{/);
  assert.match(metricsSource, /const pendingBallDragFrameRef = useRef<ShellBallWindowFrame \| null>\(null\);/);
  assert.match(metricsSource, /export function resolveShellBallReleaseSnapTarget\(/);
  assert.match(metricsSource, /const resolveManagedBallFrame = useCallback\(\(input: \{/);
  assert.match(metricsSource, /getShellBallParkedDockInsetPx\(/);
  assert.match(metricsSource, /getShellBallDockAnimationConfig\(/);
  assert.match(metricsSource, /const SHELL_BALL_EDGE_DOCK_SNAP_THRESHOLD_PX = 30;/);
  assert.match(metricsSource, /mode: "dock"/);
  assert.match(metricsSource, /mode: "reveal"/);
  assert.match(metricsSource, /const overshootFrame = animationConfig === null/);
  assert.match(metricsSource, /window\.requestAnimationFrame\(\(\) => \{/);
  assert.match(metricsSource, /await queueBallWindowDragPosition\(finalFrame\);/);
  assert.match(metricsSource, /while \(pendingBallDragFrameRef\.current !== null\) \{/);
  assert.match(metricsSource, /const nextDockSide = resolveShellBallReleaseSnapTarget\(/);
  assert.match(metricsSource, /export function clampShellBallHostFrameToVisibleBounds\(/);
  assert.match(metricsSource, /clampShellBallHostFrameToVisibleBounds\(\{/);
  assert.match(metricsSource, /type ShellBallBallDragSession = \{[\s\S]*originBounds: ShellBallWindowBounds;[\s\S]*\};/);
  assert.match(metricsSource, /const originBounds = geometryRef\.current\?\.bounds;/);
  assert.match(metricsSource, /ballDragSessionRef\.current = \{[\s\S]*pointerStart,[\s\S]*latestPointer: pointerStart,[\s\S]*frameStart,[\s\S]*originBounds,[\s\S]*\};/);
  assert.match(metricsSource, /const effectiveFrame = frameToApply;/);
  assert.doesNotMatch(metricsSource, /const effectiveFrame = bounds === undefined/);
  assert.match(metricsSource, /const releaseBounds = dragSession\?\.originBounds \?\? frameContext\.bounds;/);
  assert.match(metricsSource, /resolveShellBallReleaseSnapTarget\(\{[\s\S]*bounds: releaseBounds,/);
  assert.match(metricsSource, /commitEdgeDockState\(\{ side: null, revealed: false \}\);/);
  assert.match(metricsSource, /scheduleBallGeometryPublish\(\{ snapToBounds: true \}\);/);
  assert.match(metricsSource, /if \(ballDragSessionRef\.current !== null && !input\?\.snapToBounds\) \{/);
  assert.doesNotMatch(metricsSource, /snapToEdge/);
  assert.doesNotMatch(metricsSource, /SHELL_BALL_DRAG_RELEASE_POLL_MS/);
  assert.doesNotMatch(metricsSource, /armBallWindowBoundsSnapOnRelease/);
  assert.match(appSource, /beginBallWindowPointerDrag\(\{\s*x: event\.screenX,\s*y: event\.screenY,\s*\}\);/);
  assert.match(appSource, /updateBallWindowPointerDrag\(\{\s*x: event\.screenX,\s*y: event\.screenY,\s*\}\);/);
  assert.match(appSource, /void endBallWindowPointerDrag\(\{\s*x: event\.screenX,\s*y: event\.screenY,\s*\}\);/);
  assert.doesNotMatch(appSource, /startShellBallWindowDragging/);
  assert.match(interactionSource, /pressStartXRef\.current = event\.screenX;/);
  assert.match(interactionSource, /pressStartYRef\.current = event\.screenY;/);
  assert.match(interactionSource, /const driftDistance = Math\.hypot\(event\.screenX - pressStartXRef\.current, event\.screenY - pressStartYRef\.current\);/);
  assert.match(interactionSource, /driftDistance > SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX/);
  assert.match(mascotSource, /shouldSuppressShellBallMascotHotspotGestures/);
  assert.match(mascotSource, /dockTarget\?: ShellBallEdgeDockSide \| null;/);
  assert.match(mascotSource, /isDragging\?: boolean;/);
  assert.match(mascotSource, /isSettling\?: boolean;/);
  assert.match(surfaceSource, /onDragMove: \(event: PointerEvent<HTMLButtonElement>\) => void;/);
  assert.match(surfaceSource, /dockTarget\?: ShellBallEdgeDockSide \| null;/);
  assert.match(surfaceSource, /data-shell-ball-dragging=\{isDragging \? "true" : "false"\}/);
  assert.match(surfaceSource, /onHotspotDragMove=\{onDragMove\}/);
  assert.match(surfaceSource, /onHotspotDragEnd=\{onDragEnd\}/);
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

test("shell-ball interaction contract retains hover input while focused, hovered, or draft remains", () => {
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
      inputFocused: true,
      hasDraft: true,
    }),
    true,
  );

  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: false,
      inputHovered: true,
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
    true,
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
    sessionId: "sess_shell_ball_contract",
  });

  assert.ok(textParams);
  assert.equal(textParams.source, "floating_ball");
  assert.equal(textParams.trigger, "hover_text_input");
  assert.equal(textParams.session_id, "sess_shell_ball_contract");
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
    sessionId: "sess_shell_ball_contract",
  });

  assert.ok(voiceParams);
  assert.equal(voiceParams.trigger, "voice_commit");
  assert.equal(voiceParams.input.input_mode, "voice");
  assert.equal(voiceParams.session_id, "sess_shell_ball_contract");
  assert.equal(createShellBallInputSubmitParams({ text: "   ", trigger: "hover_text_input", inputMode: "text" }), null);
});

test("shell-ball file task params preserve attachment descriptions for agent.task.start", () => {
  const fileParams = createShellBallTaskStartParams({
    text: "  explain these files  ",
    files: ["  C:\\workspace\\notes.md  ", "C:\\workspace\\spec.md"],
  });

  assert.ok(fileParams);
  assert.equal(fileParams.trigger, "file_drop");
  assert.deepEqual(fileParams.input, {
    type: "file",
    text: "explain these files",
    files: ["C:\\workspace\\notes.md", "C:\\workspace\\spec.md"],
  });
  assert.deepEqual(fileParams.options, {
    confirm_required: false,
  });
  const fileParamsWithoutDescription = createShellBallTaskStartParams({
    text: "   ",
    files: ["C:\\workspace\\notes.md"],
  });
  assert.ok(fileParamsWithoutDescription);
  assert.deepEqual(fileParamsWithoutDescription.options, {
    confirm_required: false,
  });
  assert.equal(fileParamsWithoutDescription.input.text, undefined);
  assert.equal(createShellBallTaskStartParams({ text: "   ", files: [] }), null);
});

test("task-entry services keep rpc transport failures visible and forward file descriptions", async () => {
  const transportError = new Error("Named Pipe transport is not wired.");
  const mirrorCalls: string[] = [];

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/services/agentInputService.ts"),
    {
      "@/rpc/methods": {
        submitInput() {
          return Promise.reject(transportError);
        },
      },
      "./mirrorMemoryService": {
        recordMirrorConversationFailure() {
          mirrorCalls.push("failure");
        },
        recordMirrorConversationStart() {
          mirrorCalls.push("start");
        },
        recordMirrorConversationSuccess() {
          mirrorCalls.push("success");
        },
      },
      "./conversationSessionService": {
        getCurrentConversationSessionId(): string | undefined {
          return undefined;
        },
        rememberConversationSessionFromTask() {},
        rememberConversationPageContextFromTask() {},
      },
      "./pageContext": {
        compactPageContext,
        mapDesktopWindowSnapshotToPageContext,
        resolveTaskPageContext,
      },
    },
    async (moduleExports) => {
      const service = moduleExports as {
        submitTextInput: (input: {
          text: string;
          source: "floating_ball" | "dashboard" | "tray_panel";
          trigger: "voice_commit" | "hover_text_input";
          inputMode: "voice" | "text";
        }) => Promise<unknown>;
      };

      await assert.rejects(
        () =>
          service.submitTextInput({
            text: "submit through rpc",
            source: "floating_ball",
            trigger: "hover_text_input",
            inputMode: "text",
          }),
        /transport is not wired/i,
      );
    },
  );

  assert.deepEqual(mirrorCalls, ["start", "failure"]);

  const startTaskCalls: Array<Record<string, unknown>> = [];
  const bootstrapSubmitCalls: Array<Record<string, unknown>> = [];
  const rememberedPageContext = {
    app_name: "Chrome",
    title: "Build Dashboard",
    url: "https://example.com/build",
  };
  const taskResult: {
    bubble_message: null;
    delivery_result: null;
    task: {
      task_id: string;
      title: string;
      source_type: "dragged_file";
      status: "processing";
      intent: null;
      current_step: string;
      risk_level: "yellow";
      started_at: string;
      updated_at: string;
      finished_at: null;
    };
  } = {
    bubble_message: null,
    delivery_result: null,
    task: {
      task_id: "task_shell_ball_001",
      title: "Process files",
      source_type: "dragged_file",
      status: "processing",
      intent: null,
      current_step: "processing",
      risk_level: "yellow",
      started_at: "2026-04-18T10:00:00.000Z",
      updated_at: "2026-04-18T10:00:00.000Z",
      finished_at: null,
    },
  };

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/services/taskService.ts"),
    {
      "@/rpc/methods": {
        startTask(params: Record<string, unknown>) {
          startTaskCalls.push(params);
          return Promise.resolve(taskResult);
        },
      },
      "./conversationSessionService": {
        getCurrentConversationSessionId(): string | undefined {
          return "sess_shell_ball_files";
        },
        getConversationPageContextForSession(sessionId?: string) {
          return sessionId === "sess_shell_ball_files" ? rememberedPageContext : undefined;
        },
        rememberConversationSessionFromTask() {},
        rememberConversationPageContextFromTask() {},
      },
      "@/platform/desktopWindowContext": {
        getActiveWindowContext() {
          return Promise.resolve({
            app_name: "Chrome",
            browser_kind: "chrome",
            process_id: 4412,
            process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
            title: "Build Dashboard",
            url: "https://example.com/build?ticket=secret#fragment",
          });
        },
      },
      "./pageContext": {
        compactPageContext,
        mapDesktopWindowSnapshotToPageContext,
      },
      "./agentInputService": {
        submitTextInput(params: Record<string, unknown>) {
          bootstrapSubmitCalls.push(params);
          return Promise.resolve(taskResult);
        },
      },
    },
    async (moduleExports) => {
      const service = moduleExports as {
        bootstrapTask: (title: string) => Promise<unknown>;
        startTaskFromErrorSignal: (errorMessage: string, context?: Record<string, unknown>) => Promise<unknown>;
        startTaskFromFiles: (files: string[], context?: Record<string, unknown>, text?: string) => Promise<unknown>;
        startTaskFromSelectedText: (text: string, context?: Record<string, unknown>) => Promise<unknown>;
      };

      await service.startTaskFromFiles(
        ["  C:\\workspace\\notes.md  ", "C:\\workspace\\spec.md"],
        {
          sessionId: "sess_shell_ball_files",
          source: "floating_ball",
        },
        "  explain these files  ",
      );

      assert.equal(startTaskCalls[0]?.session_id, "sess_shell_ball_files");
      assert.equal(startTaskCalls[0]?.trigger, "file_drop");
      assert.deepEqual(startTaskCalls[0]?.input, {
        type: "file",
        text: "explain these files",
        files: ["C:\\workspace\\notes.md", "C:\\workspace\\spec.md"],
        page_context: {
          app_name: "Chrome",
          browser_kind: "chrome",
          process_id: 4412,
          process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
          title: "Build Dashboard",
          url: "https://example.com/build",
          window_title: "Build Dashboard",
        },
      });
      assert.deepEqual(startTaskCalls[0]?.options, {
        confirm_required: false,
      });

      await service.startTaskFromFiles(["C:\\workspace\\logs.txt"]);
      assert.equal(startTaskCalls[1]?.session_id, "sess_shell_ball_files");
      assert.deepEqual(startTaskCalls[1]?.options, {
        confirm_required: false,
      });
      assert.deepEqual(startTaskCalls[1]?.input, {
        type: "file",
        files: ["C:\\workspace\\logs.txt"],
        page_context: {
          app_name: "Chrome",
          browser_kind: "chrome",
          process_id: 4412,
          process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
          title: "Build Dashboard",
          url: "https://example.com/build",
          window_title: "Build Dashboard",
        },
      });

      await service.startTaskFromSelectedText("  selected text  ", {
        pageContext: {
          app_name: "notepad",
          browser_kind: "non_browser",
          process_id: 8844,
          process_path: "C:/Windows/System32/notepad.exe",
          title: "Notes",
          url: "native://windows-uia-selection",
        },
        sessionId: "sess_shell_ball_selection",
        source: "floating_ball",
      });

      await service.startTaskFromErrorSignal("  stack trace  ", {
        source: "floating_ball",
      });

      assert.equal(startTaskCalls[2]?.session_id, "sess_shell_ball_selection");
      assert.equal(startTaskCalls[2]?.trigger, "text_selected_click");
      assert.deepEqual(startTaskCalls[2]?.input, {
        type: "text_selection",
        text: "selected text",
        page_context: {
          app_name: "notepad",
          browser_kind: "non_browser",
          process_id: 8844,
          process_path: "C:/Windows/System32/notepad.exe",
          title: "Notes",
          url: "native://windows-uia-selection",
        },
      });

      assert.equal(startTaskCalls[2]?.session_id, undefined);
      assert.equal(startTaskCalls[2]?.trigger, "error_detected");
      assert.deepEqual(startTaskCalls[2]?.input, {
        type: "error",
        error_message: "stack trace",
        page_context: {
          app_name: "desktop",
          title: "Quick Intake",
          url: "local://shell-ball",
        },
      });

      await service.bootstrapTask("  summarize this  ");
    },
  );

  assert.equal(startTaskCalls.length, 3);
  assert.equal(bootstrapSubmitCalls.length, 1);
  assert.equal(bootstrapSubmitCalls[0]?.trigger, "hover_text_input");

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/services/taskService.ts"),
    {
      "@/rpc/methods": {
        startTask() {
          return Promise.reject(transportError);
        },
      },
      "./conversationSessionService": {
        getCurrentConversationSessionId(): string | undefined {
          return undefined;
        },
        getConversationPageContextForSession(): undefined {
          return undefined;
        },
        rememberConversationSessionFromTask() {},
        rememberConversationPageContextFromTask() {},
      },
      "@/platform/desktopWindowContext": {
        getActiveWindowContext() {
          return Promise.resolve(null);
        },
      },
      "./pageContext": {
        compactPageContext,
        mapDesktopWindowSnapshotToPageContext,
      },
      "./agentInputService": {
        submitTextInput() {
          return Promise.reject(transportError);
        },
      },
    },
    async (moduleExports) => {
      const service = moduleExports as {
        bootstrapTask: (title: string) => Promise<unknown>;
        startTaskFromErrorSignal: (errorMessage: string, context?: Record<string, unknown>) => Promise<unknown>;
        startTaskFromFiles: (files: string[], context?: Record<string, unknown>, text?: string) => Promise<unknown>;
        startTaskFromSelectedText: (text: string, context?: Record<string, unknown>) => Promise<unknown>;
      };

      await assert.rejects(() => service.startTaskFromSelectedText("selected text"), /transport is not wired/i);
      await assert.rejects(() => service.startTaskFromFiles(["C:\\workspace\\notes.md"], {}, "details"), /transport is not wired/i);
      await assert.rejects(() => service.startTaskFromErrorSignal("stack trace"), /transport is not wired/i);
      await assert.rejects(() => service.bootstrapTask("hover text"), /transport is not wired/i);
    },
  );
});

test("submitTextInput enriches formal context with desktop snapshots before rpc submit", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_001",
                session_id: null,
                title: "Inspect current screen",
                source_type: "screen_capture",
                status: "waiting_auth",
                intent: null,
                current_step: "waiting_auth",
                risk_level: "yellow",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return "sess_ctx_hidden";
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./pageContext": {
          compactPageContext,
          mapDesktopWindowSnapshotToPageContext,
          resolveTaskPageContext,
          sanitizePageContextUrl,
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            return Promise.resolve({
              app_name: "Chrome",
              browser_kind: "chrome",
              page_switch_count: 1,
              process_id: 4412,
              process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
              title: "Build Dashboard",
              url: "https://example.com/build?ticket=secret#fragment",
              window_switch_count: 2,
            });
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
            context?: Record<string, unknown>;
          }) => Promise<unknown>;
        };

        await service.submitTextInput({
          text: "帮我看看这个页面",
          source: "floating_ball",
          trigger: "hover_text_input",
          inputMode: "text",
          context: {
            screen: {
              summary: "release validation failed on current screen",
            },
          },
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    page: {
      app_name: "Chrome",
      browser_kind: "chrome",
      process_id: 4412,
      process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
      title: "Build Dashboard",
      url: "https://example.com/build",
      window_title: "Build Dashboard",
    },
    screen: {
      summary: "release validation failed on current screen",
      screen_summary: "Foreground Chrome page \"Build Dashboard\" is active at https://example.com/build.",
      window_title: "Build Dashboard",
    },
    behavior: {
      last_action: "hover_text_input",
      dwell_millis: 5000,
      window_switch_count: 2,
      page_switch_count: 1,
    },
  });
});

test("submitTextInput enriches floating-ball text submissions with foreground page attach hints", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  let windowContextCallCount = 0;
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_002",
                session_id: null,
                title: "Summarize note",
                source_type: "text_input",
                status: "processing",
                intent: null,
                current_step: "processing",
                risk_level: "green",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return undefined;
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./pageContext": {
          compactPageContext,
          mapDesktopWindowSnapshotToPageContext,
          resolveTaskPageContext,
          sanitizePageContextUrl,
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            windowContextCallCount += 1;
            return Promise.resolve({
              app_name: "Chrome",
              browser_kind: "chrome",
              page_switch_count: 1,
              process_id: 4412,
              process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
              title: "Build Dashboard",
              url: "https://example.com/build?ticket=secret#fragment",
              window_switch_count: 2,
            });
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
          }) => Promise<unknown>;
        };

        await service.submitTextInput({
          text: "Summarize this note",
          source: "floating_ball",
          trigger: "hover_text_input",
          inputMode: "text",
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.equal(submitCalls[0]?.session_id, undefined);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    page: {
      app_name: "Chrome",
      browser_kind: "chrome",
      process_id: 4412,
      process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
      title: "Build Dashboard",
      url: "https://example.com/build",
      window_title: "Build Dashboard",
    },
    behavior: {
      last_action: "hover_text_input",
      dwell_millis: 5000,
      window_switch_count: 2,
      page_switch_count: 1,
    },
  });
  assert.equal(windowContextCallCount, 1);
});

test("submitTextInput keeps dashboard voice submissions free of ambient page and screen snapshots", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  let windowContextCallCount = 0;
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_003",
                session_id: null,
                title: "Summarize note",
                source_type: "text_input",
                status: "processing",
                intent: null,
                current_step: "processing",
                risk_level: "green",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return undefined;
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./pageContext": {
          compactPageContext,
          mapDesktopWindowSnapshotToPageContext,
          resolveTaskPageContext,
          sanitizePageContextUrl,
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            windowContextCallCount += 1;
            return Promise.resolve({
              app_name: "Chrome",
              browser_kind: "chrome",
              page_switch_count: 1,
              process_id: 4412,
              process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
              title: "Build Dashboard",
              url: "https://example.com/build?ticket=secret#fragment",
              window_switch_count: 2,
            });
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
          }) => Promise<unknown>;
        };

        await service.submitTextInput({
          text: "Summarize this note",
          source: "dashboard",
          trigger: "voice_commit",
          inputMode: "voice",
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    behavior: {
      last_action: "voice_commit",
      dwell_millis: 5000,
    },
  });
  assert.equal(windowContextCallCount, 0);
});

test("submitTextInput can force foreground window snapshots when the caller opts in", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  let windowContextCallCount = 0;
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;
  let submitResult: unknown;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_003",
                session_id: null,
                title: "Open current site",
                source_type: "voice",
                status: "processing",
                intent: null,
                current_step: "processing",
                risk_level: "green",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return undefined;
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            windowContextCallCount += 1;
            return Promise.resolve({
              app_name: "Chrome",
              browser_kind: "chrome",
              page_switch_count: 1,
              process_id: 4412,
              process_path: null,
              title: "Build Dashboard",
              url: "https://example.com/build?ticket=secret#fragment",
              window_switch_count: 2,
            });
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
            includeForegroundWindowContext?: boolean;
          }) => Promise<unknown>;
        };

        submitResult = await service.submitTextInput({
          text: "Summarize this note",
          source: "dashboard",
          trigger: "voice_commit",
          inputMode: "voice",
          includeForegroundWindowContext: true,
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    page: {
      app_name: "Chrome",
      browser_kind: "chrome",
      process_id: 4412,
      title: "Build Dashboard",
      url: "https://example.com/build",
      window_title: "Build Dashboard",
    },
    screen: {
      summary: "Foreground Chrome page \"Build Dashboard\" is active at https://example.com/build.",
      screen_summary: "Foreground Chrome page \"Build Dashboard\" is active at https://example.com/build.",
      window_title: "Build Dashboard",
    },
    behavior: {
      last_action: "voice_commit",
      dwell_millis: 5000,
      window_switch_count: 2,
      page_switch_count: 1,
    },
  });
  assert.equal((submitResult as { clientContext?: unknown } | undefined)?.clientContext, undefined);
  assert.equal(windowContextCallCount, 1);
});

test("submitTextInput can restrict ambient foreground snapshots to browser pages with urls", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  let windowContextCallCount = 0;
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;
  let submitResult: unknown;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_004",
                session_id: null,
                title: "Summarize current page",
                source_type: "text_input",
                status: "processing",
                intent: null,
                current_step: "processing",
                risk_level: "green",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return undefined;
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./pageContext": {
          compactPageContext,
          mapDesktopWindowSnapshotToPageContext,
          resolveTaskPageContext,
          sanitizePageContextUrl,
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            windowContextCallCount += 1;
            return Promise.resolve({
              app_name: "Chrome",
              browser_kind: "chrome",
              page_switch_count: 1,
              process_id: 4412,
              process_path: null,
              title: "Build Dashboard",
              url: "https://example.com/build?ticket=secret#fragment",
              window_switch_count: 2,
            });
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
            includeForegroundBrowserPageContext?: boolean;
          }) => Promise<unknown>;
        };

        submitResult = await service.submitTextInput({
          text: "Summarize this page",
          source: "floating_ball",
          trigger: "hover_text_input",
          inputMode: "text",
          includeForegroundBrowserPageContext: true,
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    page: {
      app_name: "Chrome",
      browser_kind: "chrome",
      process_id: 4412,
      title: "Build Dashboard",
      url: "https://example.com/build",
      window_title: "Build Dashboard",
    },
    screen: {
      summary: "Foreground Chrome page \"Build Dashboard\" is active at https://example.com/build.",
      screen_summary: "Foreground Chrome page \"Build Dashboard\" is active at https://example.com/build.",
      window_title: "Build Dashboard",
    },
    behavior: {
      last_action: "hover_text_input",
      dwell_millis: 5000,
      window_switch_count: 2,
      page_switch_count: 1,
    },
  });
  assert.deepEqual((submitResult as { clientContext?: unknown } | undefined)?.clientContext, {
    detectedPage: {
      appName: "Chrome",
      title: "Build Dashboard",
      url: "https://example.com/build",
    },
    behavior: {
      last_action: "hover_text_input",
      dwell_millis: 5000,
      window_switch_count: 2,
      page_switch_count: 1,
    },
  });
  assert.deepEqual((submitResult as { clientContext?: unknown } | undefined)?.clientContext, {
    detectedPage: {
      appName: "Chrome",
      title: "Build Dashboard",
      url: "https://example.com/build",
    },
  });
  assert.equal(windowContextCallCount, 1);
});

test("submitTextInput sanitizes explicit page context urls before rpc submit", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  let windowContextCallCount = 0;
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_005",
                session_id: null,
                title: "Summarize note",
                source_type: "text_input",
                status: "processing",
                intent: null,
                current_step: "processing",
                risk_level: "green",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return undefined;
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            windowContextCallCount += 1;
            return Promise.resolve(null);
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
            pageContext?: Record<string, unknown>;
          }) => Promise<unknown>;
        };

        await service.submitTextInput({
          text: "Summarize this note",
          source: "dashboard",
          trigger: "voice_commit",
          inputMode: "voice",
          pageContext: {
            app_name: "Chrome",
            title: "Build Dashboard",
            url: "https://user:secret@example.com/build?ticket=secret#fragment",
          },
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    page: {
      app_name: "Chrome",
      title: "Build Dashboard",
      url: "https://example.com/build",
    },
    behavior: {
      last_action: "voice_commit",
      dwell_millis: 5000,
    },
  });
  assert.equal(windowContextCallCount, 1);
});

test("submitTextInput keeps behavior counters when browser-only ambient snapshots are suppressed", async () => {
  const submitCalls: Array<Record<string, unknown>> = [];
  let windowContextCallCount = 0;
  const originalDateNow = Date.now;
  Date.now = () => 1_713_864_005_000;
  let submitResult: unknown;

  try {
    await withSourceModuleRuntime(
      resolve(desktopRoot, "src/services/agentInputService.ts"),
      {
        "@/rpc/methods": {
          submitInput(params: Record<string, unknown>) {
            submitCalls.push(params);
            return Promise.resolve({
              bubble_message: null,
              delivery_result: null,
              task: {
                task_id: "task_ctx_005",
                session_id: null,
                title: "Summarize current page",
                source_type: "text_input",
                status: "processing",
                intent: null,
                current_step: "processing",
                risk_level: "green",
                started_at: "2026-04-23T10:00:00.000Z",
                updated_at: "2026-04-23T10:00:00.000Z",
                finished_at: null,
              },
            });
          },
        },
        "./conversationSessionService": {
          getCurrentConversationSessionId(): string | undefined {
            return undefined;
          },
          rememberConversationSessionFromTask() {},
          rememberConversationPageContextFromTask() {},
        },
        "./mirrorMemoryService": {
          recordMirrorConversationFailure() {},
          recordMirrorConversationStart() {},
          recordMirrorConversationSuccess() {},
        },
        "@/platform/desktopActivity": {
          getDesktopMouseActivitySnapshot() {
            return Promise.resolve({ updated_at: "1713864000000" });
          },
        },
        "@/platform/desktopWindowContext": {
          getActiveWindowContext() {
            windowContextCallCount += 1;
            return Promise.resolve({
              app_name: "Windows Terminal",
              browser_kind: "non_browser",
              page_switch_count: 1,
              process_path: null,
              title: "agent-log.txt",
              url: null,
              window_switch_count: 2,
            });
          },
        },
      },
      async (moduleExports) => {
        const service = moduleExports as {
          submitTextInput: (input: {
            text: string;
            source: "floating_ball" | "dashboard" | "tray_panel";
            trigger: "voice_commit" | "hover_text_input";
            inputMode: "voice" | "text";
            includeForegroundBrowserPageContext?: boolean;
          }) => Promise<unknown>;
        };

        submitResult = await service.submitTextInput({
          text: "Summarize this page",
          source: "floating_ball",
          trigger: "hover_text_input",
          inputMode: "text",
          includeForegroundBrowserPageContext: true,
        });
      },
    );
  } finally {
    Date.now = originalDateNow;
  }

  assert.equal(submitCalls.length, 1);
  assert.deepEqual(submitCalls[0]?.context, {
    files: [],
    behavior: {
      last_action: "hover_text_input",
      dwell_millis: 5000,
      window_switch_count: 2,
      page_switch_count: 1,
    },
  });
  assert.equal((submitResult as { clientContext?: unknown } | undefined)?.clientContext, undefined);
  assert.equal(windowContextCallCount, 1);
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

test("shell-ball text drop target only arms during eligible text drags", () => {
  assert.equal(
    shouldArmShellBallTextDropTarget({
      fileDropActive: false,
      textDragActive: true,
      visualState: "hover_input",
    }),
    true,
  );
  assert.equal(
    shouldArmShellBallTextDropTarget({
      fileDropActive: true,
      textDragActive: true,
      visualState: "hover_input",
    }),
    false,
  );
  assert.equal(
    shouldArmShellBallTextDropTarget({
      fileDropActive: false,
      textDragActive: true,
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

test("shell-ball voice preview mode filters lock and cancel gestures by stage", () => {
  assert.equal(
    getShellBallVoicePreviewForHintMode({
      hintMode: "lock",
      deltaX: 0,
      deltaY: -SHELL_BALL_LOCK_DELTA_PX,
    }),
    "lock",
  );
  assert.equal(
    getShellBallVoicePreviewForHintMode({
      hintMode: "lock",
      deltaX: 0,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    null,
  );
  assert.equal(
    getShellBallVoicePreviewForHintMode({
      hintMode: "cancel",
      deltaX: 0,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    "cancel",
  );
  assert.equal(
    getShellBallVoicePreviewForHintMode({
      hintMode: "cancel",
      deltaX: 0,
      deltaY: -SHELL_BALL_LOCK_DELTA_PX,
    }),
    null,
  );
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
  assert.match(markup, /shell-ball-mascot__pet-shell/);
  assert.match(markup, /shell-ball-mascot__pet/);
});

test("shell-ball mascot surfaces a microphone marker while voice capture is active", () => {
  const voiceMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "voice_listening",
      motionConfig: getShellBallMotionConfig("voice_listening"),
    }),
  );
  const idleMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );

  assert.match(voiceMarkup, /shell-ball-mascot__voice-marker/);
  assert.doesNotMatch(idleMarkup, /shell-ball-mascot__voice-marker/);
});

test("shell-ball mascot no longer renders the legacy selection marker when text selection is available", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      selectionIndicatorVisible: true,
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );

  assert.doesNotMatch(markup, /shell-ball-mascot__selection-marker/);
  assert.doesNotMatch(markup, /shell-ball-mascot__selection-marker-glyph/);
});

test("floating pet renders the open-eye and closed-beak layers by default", () => {
  const markup = renderToStaticMarkup(createElement(FloatingPet, { size: 128 }));

  assert.match(markup, /eye_open\.png/);
  assert.match(markup, /beak_closed\.png/);
  assert.doesNotMatch(markup, /bubble_safe\.png/);
});

test("floating pet swaps to the closed-eye layer when requested", () => {
  const markup = renderToStaticMarkup(createElement(FloatingPet, { eyesClosed: true, size: 128 }));

  assert.match(markup, /eye_closed\.png/);
  assert.match(markup, /beak_closed\.png/);
});

test("floating pet renders the matching effect layer for processing and auth modes", () => {
  const thinkingMarkup = renderToStaticMarkup(createElement(FloatingPet, { mode: "think", size: 128 }));
  const safeMarkup = renderToStaticMarkup(createElement(FloatingPet, { mode: "safe", size: 128 }));

  assert.match(thinkingMarkup, /bubble_thinking\.png/);
  assert.match(safeMarkup, /bubble_safe\.png/);
});

test("shell-ball mascot pet mapping keeps happy as a local override and voice lock explicit", () => {
  assert.deepEqual(getShellBallMascotPetState({ visualState: "processing", happyActive: false }), {
    listenLocked: false,
    mode: "think",
  });
  assert.deepEqual(getShellBallMascotPetState({ visualState: "voice_locked", happyActive: false }), {
    listenLocked: true,
    mode: "listen",
  });
  assert.deepEqual(getShellBallMascotPetState({ visualState: "waiting_auth", happyActive: true }), {
    listenLocked: false,
    mode: "happy",
  });
});

test("shell-ball mascot exposes edge and corner posture states", () => {
  const mascotSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallMascot.tsx"), "utf8");
  const shellBallStyles = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.css"), "utf8");
  const leftMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      edgeDockSide: "left",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const rightMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      edgeDockSide: "right",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const topMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      edgeDockSide: "top",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const bottomMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      edgeDockSide: "bottom",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const topRevealedMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      edgeDockSide: "top",
      edgeDockRevealed: true,
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const bottomRevealedMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      visualState: "idle",
      edgeDockSide: "bottom",
      edgeDockRevealed: true,
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const topLeftMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      dockTarget: "top_left",
      edgeDockSide: "top_left",
      isSettling: true,
      visualState: "idle",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );
  const bottomRightDraggingMarkup = renderToStaticMarkup(
    createElement(ShellBallMascot, {
      dockTarget: "bottom_right",
      edgeDockSide: "bottom_right",
      isDragging: true,
      visualState: "idle",
      motionConfig: getShellBallMotionConfig("idle"),
    }),
  );

  assert.match(leftMarkup, /rotate\(8deg\)/);
  assert.match(rightMarkup, /rotate\(-8deg\)/);
  assert.match(topMarkup, /data-edge-dock-side="top"/);
  assert.match(bottomMarkup, /data-edge-dock-side="bottom"/);
  assert.match(topLeftMarkup, /data-edge-dock-side="top_left"/);
  assert.match(topLeftMarkup, /data-dock-target="top_left"/);
  assert.match(topLeftMarkup, /data-shell-ball-settling="true"/);
  assert.match(bottomRightDraggingMarkup, /data-edge-dock-side="bottom_right"/);
  assert.match(bottomRightDraggingMarkup, /data-shell-ball-dragging="true"/);
  assert.match(topRevealedMarkup, /data-edge-dock-revealed="true"/);
  assert.match(bottomRevealedMarkup, /data-edge-dock-revealed="true"/);
  assert.match(mascotSource, /import \{ FloatingPet \} from "\.\/floating-pet\/FloatingPet";/);
  assert.match(mascotSource, /function resolveShellBallDockStyle\(input:/);
  assert.match(mascotSource, /case "top_left":/);
  assert.match(mascotSource, /case "top_right":/);
  assert.match(mascotSource, /case "bottom_left":/);
  assert.match(mascotSource, /case "bottom_right":/);
  assert.match(mascotSource, /<div className="shell-ball-mascot__pet-shell">/);
  assert.match(shellBallStyles, /\.shell-ball-mascot__visual \{/);
  assert.match(shellBallStyles, /\.shell-ball-mascot__pet-shell \{/);
  assert.match(shellBallStyles, /\.shell-ball-mascot__pet \{/);
});

test("shell-ball edge-dock reveal keeps the orb open while the pointer hugs the parked edge", () => {
  const bounds = {
    minX: 0,
    minY: 0,
    maxX: 1920,
    maxY: 1080,
  };

  assert.equal(
    shouldRetainShellBallEdgeDockReveal({
      bounds,
      edgeDockSide: "left",
      screenX: 8,
      screenY: 420,
    }),
    true,
  );
  assert.equal(
    shouldRetainShellBallEdgeDockReveal({
      bounds,
      edgeDockSide: "right",
      screenX: 1914,
      screenY: 420,
    }),
    true,
  );
  assert.equal(
    shouldRetainShellBallEdgeDockReveal({
      bounds,
      edgeDockSide: "left",
      screenX: 48,
      screenY: 420,
    }),
    false,
  );
});

test("shell-ball release preview recomputes from the final pointer position", () => {
  assert.equal(
    getShellBallVoicePreviewFromEvent({
      hintMode: "lock",
      startX: 100,
      startY: 100,
      pointerX: 100,
      pointerY: 52,
      fallbackPreview: null,
    }),
    "lock",
  );

  assert.equal(
    getShellBallVoicePreviewFromEvent({
      hintMode: "cancel",
      startX: 100,
      startY: 100,
      pointerX: 100,
      pointerY: 148,
      fallbackPreview: null,
    }),
    "cancel",
  );
});

test("shell-ball keeps voice preview alive on leave while voice listening is active", () => {
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("voice_listening"), true);
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("hover_input"), false);
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("voice_locked"), true);
});

test("shell-ball voice recognition resumes after unexpected end while listening or locked", () => {
  assert.equal(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd("voice_listening"), true);
  assert.equal(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd("voice_locked"), true);
  assert.equal(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd("hover_input"), false);
  assert.equal(shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd(null), true);
  assert.equal(shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd("network"), true);
  assert.equal(shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd("not-allowed"), false);

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

test("shell-ball helper metrics keep the voice overlay centered and always click-through", () => {
  assert.deepEqual(
    getShellBallVoiceAnchor({
      ballFrame: {
        x: 120,
        y: 240,
        width: 144,
        height: 156,
      },
      helperFrame: {
        width: 220,
        height: 280,
      },
    }),
    {
      x: 82,
      y: 178,
    },
  );
  assert.deepEqual(
    getShellBallHelperWindowInteractionMode({
      role: "voice",
      visible: true,
      clickThrough: true,
    }),
    {
      focusable: false,
      ignoreCursorEvents: true,
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
    nextFocused: true,
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

test("shell-ball submit failure recovery only restores untouched empty drafts", () => {
  assert.equal(
    shouldRestoreShellBallSubmitFailureDraft({
      currentInputValue: "",
      currentPendingFiles: [],
      currentDraftRevision: 4,
      submittedDraftRevision: 4,
    }),
    true,
  );

  assert.equal(
    shouldRestoreShellBallSubmitFailureDraft({
      currentInputValue: "new draft",
      currentPendingFiles: [],
      currentDraftRevision: 5,
      submittedDraftRevision: 4,
    }),
    false,
  );

  assert.equal(
    shouldRestoreShellBallSubmitFailureDraft({
      currentInputValue: "",
      currentPendingFiles: ["C:/draft.md"],
      currentDraftRevision: 4,
      submittedDraftRevision: 4,
    }),
    false,
  );

  assert.equal(
    shouldRestoreShellBallSubmitFailureDraft({
      currentInputValue: "",
      currentPendingFiles: [],
      currentDraftRevision: 5,
      submittedDraftRevision: 4,
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
  assert.doesNotMatch(readonlyMarkup, /shell-ball-input-bar__resize-handle/);
  assert.match(voiceMarkup, /tabindex="-1"/i);
});

test("shell-ball input bar renders the floating textarea shell", () => {
  const interactiveMarkup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "interactive",
      voicePreview: null,
      value: "Draft",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );
  const inputBarSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallInputBar.tsx"), "utf8");
  const shellBallStyles = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.css"), "utf8");

  assert.match(interactiveMarkup, /<textarea/);
  assert.match(interactiveMarkup, /shell-ball-uiverse-placeholder/);
  assert.match(inputBarSource, /if \(event\.key !== "Enter" \|\| event\.shiftKey \|\| submitDisabled\) \{/);
  assert.match(inputBarSource, /function restoreInputFocus\(\) \{/);
  assert.match(inputBarSource, /const visiblePlaceholder = placeholder\?\.trim\(\) \|\| SHELL_BALL_INPUT_DEFAULT_PLACEHOLDER;/);
  assert.match(shellBallStyles, /\.shell-ball-uiverse-inputbox::before \{/);
  assert.match(shellBallStyles, /\.shell-ball-uiverse-fill \{/);
  assert.match(shellBallStyles, /\.shell-ball-uiverse-placeholder \{/);
});

test("shell-ball input helpers clamp manual resize and autosize heights", () => {
  assert.equal(clampShellBallInputResizeDimension(96.4, 120, 240), 120);
  assert.equal(clampShellBallInputResizeDimension(188.2, 120, 240), 188);
  assert.equal(clampShellBallInputResizeDimension(312.7, 120, 240), 240);
  assert.equal(resolveShellBallInputAutoWidth({ contentWidth: 448, minWidth: 240, maxWidth: 360 }), 360);
  assert.equal(resolveShellBallInputFieldWidth({ autoWidth: 300, manualWidth: null, minWidth: 240, maxWidth: 360 }), 300);
  assert.equal(resolveShellBallInputFieldWidth({ autoWidth: 320, manualWidth: 280, minWidth: 240, maxWidth: 360 }), 320);
  assert.equal(resolveShellBallInputFieldWidth({ autoWidth: 260, manualWidth: 340, minWidth: 240, maxWidth: 360 }), 340);
  assert.equal(resolveShellBallInputMaxWidth(240), 360);
  assert.equal(
    resolveShellBallInputMaxHeight({
      lineHeight: 22,
      paddingTop: 2,
      paddingBottom: 4,
      minHeight: 44,
    }),
    72,
  );

  assert.equal(
    resolveShellBallInputFieldHeight({
      contentHeight: 96,
      manualHeight: null,
      minHeight: 44,
      maxHeight: 72,
    }),
    72,
  );
  assert.equal(
    resolveShellBallInputFieldHeight({
      contentHeight: 96,
      manualHeight: 58,
      minHeight: 44,
      maxHeight: 72,
    }),
    72,
  );
  assert.equal(
    resolveShellBallInputFieldHeight({
      contentHeight: 52,
      manualHeight: 58,
      minHeight: 44,
      maxHeight: 72,
    }),
    58,
  );
});

test("shell-ball input interaction state keeps every visible input helper directly interactive", () => {
  assert.deepEqual(
    getShellBallInputInteractionState({
      visualState: "hover_input",
      regionActive: false,
      inputFocused: false,
      inputHovered: false,
      hasDraft: true,
    }),
    { clickThrough: false },
  );

  assert.deepEqual(
    getShellBallInputInteractionState({
      visualState: "hover_input",
      regionActive: true,
      inputFocused: false,
      inputHovered: false,
      hasDraft: true,
    }),
    { clickThrough: false },
  );

  assert.deepEqual(
    getShellBallInputInteractionState({
      visualState: "hover_input",
      regionActive: false,
      inputFocused: false,
      inputHovered: false,
      hasDraft: false,
    }),
    { clickThrough: false },
  );

  assert.deepEqual(
    getShellBallInputInteractionState({
      visualState: "processing",
      regionActive: false,
      inputFocused: false,
      inputHovered: false,
      hasDraft: true,
    }),
    { clickThrough: false },
  );

  assert.deepEqual(
    getShellBallInputInteractionState({
      visualState: "idle",
      regionActive: false,
      inputFocused: false,
      inputHovered: false,
      hasDraft: false,
    }),
    { clickThrough: true },
  );
});

test("shell-ball input width helper measures the widest line plus padding", () => {
  const originalDocument = globalThis.document;
  const canvasContext = {
    font: "",
    measureText(value: string) {
      return {
        width: value.length * 10,
      };
    },
  };

  globalThis.document = {
    createElement(tagName: string) {
      assert.equal(tagName, "canvas");
      return {
        getContext(kind: string) {
          assert.equal(kind, "2d");
          return canvasContext;
        },
      };
    },
  } as unknown as Document;

  try {
    assert.equal(
      measureShellBallInputContentWidth({
        value: "short\nlonger",
        font: "16px serif",
        letterSpacing: 1,
        paddingLeft: 4,
        paddingRight: 6,
      }),
      69,
    );
  } finally {
    globalThis.document = originalDocument;
  }
});

test("shell-ball input focus helper keeps the caret at the end of the draft", () => {
  const calls: string[] = [];
  focusShellBallInputField({
    focus() {
      calls.push("focus");
    },
    setSelectionRange(start, end) {
      calls.push(`range:${String(start)}:${String(end)}`);
    },
    value: "Draft text",
  });

  assert.deepEqual(calls, ["focus", "range:10:10"]);
});

test("shell-ball bubble roles keep asymmetric softened bottom corners", () => {
  const shellBallStyles = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.css"), "utf8");

  assert.match(shellBallStyles, /\.shell-ball-bubble-message--agent \{[\s\S]*border-bottom-left-radius:\s*0\.3rem;/);
  assert.match(shellBallStyles, /\.shell-ball-bubble-message--user \{[\s\S]*border-bottom-right-radius:\s*0\.3rem;/);
});

test("shell-ball app drops page-shell copy while preserving the floating shell surface", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp));

  assert.doesNotMatch(markup, /shell-ball phase 1/i);
  assert.doesNotMatch(markup, /小胖啾近场承接/);
  assert.doesNotMatch(markup, /demo-only 第一阶段边界/);
  assert.doesNotMatch(markup, /<main/i);
  assert.match(markup, /shell-ball-surface/);
  assert.match(markup, /shell-ball-mascot/);
  assert.doesNotMatch(markup, /shell-ball-bubble-zone/);
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
    regionActive: false,
    inputValue: "draft",
    inputFocused: false,
    finalizedSpeechPayload: null,
    voicePreview: null,
    voiceHintMode: "hidden",
    setInputValue: () => {},
    onFinalizedSpeechHandled: () => {},
    onRegionEnter: () => {},
    onRegionLeave: () => {},
    onInputHoverChange: () => {},
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
      ShellBallBubbleMessage(): null {
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
      onDeleteBubble() {},
      onPinBubble() {},
    }),
  );

  assert.match(markup, /shell-ball-bubble-message__hover-controls/g);
  assert.match(markup, /shell-ball-bubble-message__hover-control-icon/g);
  assert.doesNotMatch(markup, /data-bubble-action="pin"/);
  assert.equal(markup.match(/data-bubble-action="delete"/g)?.length, 2);
  assert.doesNotMatch(markup, /data-tooltip=/);
});

test("shell-ball agent bubbles auto-link bare https urls without trailing punctuation", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleZone, {
      visualState: "processing",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "msg-agent-link-1",
            task_id: "task-agent-link-1",
            type: "result",
            text: "项目地址是 https://github.com/1024XEngineer/CialloClaw。",
            pinned: false,
            hidden: false,
            created_at: "2026-05-08T10:09:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
          },
        },
      ] satisfies ShellBallBubbleItem[],
    }),
  );

  assert.match(markup, /href="https:\/\/github\.com\/1024XEngineer\/CialloClaw"/);
  assert.doesNotMatch(markup, /href="https:\/\/github\.com\/1024XEngineer\/CialloClaw。"/);
  assert.match(markup, /target="_blank"/);
  assert.match(markup, /data-shell-ball-interactive="true"/);
});

test("shell-ball agent bubbles keep markdown links clickable", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleZone, {
      visualState: "processing",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "msg-agent-markdown-link-1",
            task_id: "task-agent-markdown-link-1",
            type: "result",
            text: "[项目地址](https://github.com/1024XEngineer/CialloClaw)",
            pinned: false,
            hidden: false,
            created_at: "2026-05-08T10:09:05.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
          },
        },
      ] satisfies ShellBallBubbleItem[],
    }),
  );

  assert.match(markup, />项目地址<\/a>/);
  assert.match(markup, /href="https:\/\/github\.com\/1024XEngineer\/CialloClaw"/);
  assert.match(markup, /data-shell-ball-interactive="true"/);
});

test("shell-ball markdown links open through the desktop external-url bridge", () => {
  const markdownSource = readFileSync(
    resolve(desktopRoot, "src/features/shell-ball/components/ShellBallMarkdown.tsx"),
    "utf8",
  );
  const externalUrlSource = readFileSync(
    resolve(desktopRoot, "src/platform/desktopExternalUrl.ts"),
    "utf8",
  );

  assert.match(markdownSource, /openDesktopExternalUrl\(href\)/);
  assert.doesNotMatch(markdownSource, /window\.open\(/);
  assert.match(externalUrlSource, /invoke<void>\("desktop_open_external_url", \{ url \}\)/);
});

test("shell-ball pending-approval bubbles render inline allow and deny controls", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallBubbleZone, {
      visualState: "waiting_auth",
      bubbleItems: [
        {
          bubble: {
            bubble_id: "msg-approval-1",
            task_id: "task-approval-1",
            type: "status",
            text: "Waiting for approval before the task can continue.",
            pinned: false,
            hidden: false,
            created_at: "2026-04-24T09:10:00.000Z",
          },
          role: "agent",
          desktop: {
            lifecycleState: "visible",
            inlineApproval: {
              approvalId: "approval-1",
              status: "idle",
            },
          },
        },
      ] satisfies ShellBallBubbleItem[],
      onAllowApprovalBubble() {},
      onDenyApprovalBubble() {},
      onDeleteBubble() {},
      onPinBubble() {},
    }),
  );

  assert.match(markup, /shell-ball-bubble-message__approval-actions/);
  assert.equal(markup.match(/data-bubble-action="allow_approval"/g)?.length, 1);
  assert.equal(markup.match(/data-bubble-action="deny_approval"/g)?.length, 1);
  assert.doesNotMatch(markup, /data-bubble-action="pin"/);
  assert.doesNotMatch(markup, /data-bubble-action="delete"/);
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

test("shell-ball coordinator prefers bubble_message text over non-empty delivery preview", () => {
  const createdItem = createShellBallAgentBubbleItem(
    {
      task: {
        task_id: "task-bubble-message-preview",
      },
      bubble_message: {
        bubble_id: "bubble-message-preview-1",
        task_id: "task-bubble-message-preview",
        type: "result",
        text: "完整的回复正文，不应该被预览文本替换。",
        pinned: false,
        hidden: true,
        created_at: "2026-04-11T10:10:30.000Z",
      },
      delivery_result: {
        type: "bubble",
        title: "Preview title",
        payload: {
          path: null,
          task_id: "task-bubble-message-preview",
          url: null,
        },
        preview_text: "被截断的预览…",
      },
    } as any,
    "2026-04-11T10:10:40.000Z",
  );

  assert.equal(createdItem.bubble.text, "完整的回复正文，不应该被预览文本替换。");
  assert.equal(createdItem.bubble.created_at, "2026-04-11T10:10:30.000Z");
  assert.equal(createdItem.role, "agent");
});

test("shell-ball auto-open helper only targets formal delivery types with native open flows", () => {
  assert.equal(shouldAutoOpenShellBallDeliveryResult(undefined), false);
  assert.equal(shouldAutoOpenShellBallDeliveryResult(null), false);
  assert.equal(shouldAutoOpenShellBallDeliveryResult({ type: "bubble" } as any), false);
  assert.equal(shouldAutoOpenShellBallDeliveryResult({ type: "task_detail" } as any), true);
  assert.equal(shouldAutoOpenShellBallDeliveryResult({ type: "workspace_document" } as any), true);
  assert.equal(shouldAutoOpenShellBallDeliveryResult({ type: "open_file" } as any), true);
  assert.equal(shouldAutoOpenShellBallDeliveryResult({ type: "reveal_in_folder" } as any), true);
  assert.equal(shouldAutoOpenShellBallDeliveryResult({ type: "result_page" } as any), true);
});

test("shell-ball runtime observation helper keeps runtime hints lightweight", () => {
  assert.equal(
    createShellBallRuntimeObservationReply({
      task_id: "task-runtime-observation",
      message: "Added another instruction.",
    }),
    null,
  );
  assert.equal(
    createShellBallRuntimeObservationReply({
      task_id: "task-runtime-observation",
      event: {
        event_id: "evt_loop_retry_1",
        run_id: "run-runtime",
        task_id: "task-runtime-observation",
        type: "loop.retrying",
        level: "warn",
        payload_json: "{}",
        created_at: "2026-04-27T08:00:00.000Z",
      },
      stop_reason: "network timeout",
    }),
    "Retrying the current task step after network timeout.",
  );
  assert.equal(
    createShellBallRuntimeObservationReply({
      task_id: "task-runtime-observation",
      event: {
        event_id: "evt_loop_failed_1",
        run_id: "run-runtime",
        task_id: "task-runtime-observation",
        type: "loop.failed",
        level: "error",
        payload_json: "{}",
        created_at: "2026-04-27T08:01:00.000Z",
      },
      stop_reason: "rate limit",
    }),
    "Task runtime failed: rate limit. Open task detail for more context.",
  );
  assert.equal(
    createShellBallRuntimeObservationReply({
      task_id: "task-runtime-observation",
      event: {
        event_id: "evt_loop_started_1",
        run_id: "run-runtime",
        task_id: "task-runtime-observation",
        type: "loop.started",
        level: "info",
        payload_json: "{}",
        created_at: "2026-04-27T08:02:00.000Z",
      },
    }),
    null,
  );
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

test("shell-ball bubble ordering keeps late agent replies attached to the originating user turn", () => {
  const sortedItems = sortShellBallBubbleItemsByTimestamp([
    {
      bubble: {
        bubble_id: "bubble-agent-1",
        task_id: "task-turn-1",
        type: "result",
        text: "First reply arrives late.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:05:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
        turnIndex: 1,
        turnPhase: 1,
      },
    },
    {
      bubble: {
        bubble_id: "bubble-user-2",
        task_id: "task-turn-2",
        type: "result",
        text: "Second user message.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:01:00.000Z",
      },
      role: "user",
      desktop: {
        lifecycleState: "visible",
        turnIndex: 2,
        turnPhase: 0,
      },
    },
    {
      bubble: {
        bubble_id: "bubble-user-1",
        task_id: "task-turn-1",
        type: "result",
        text: "First user message.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:00:00.000Z",
      },
      role: "user",
      desktop: {
        lifecycleState: "visible",
        turnIndex: 1,
        turnPhase: 0,
      },
    },
  ]);

  assert.deepEqual(
    sortedItems.map((item) => item.bubble.bubble_id),
    ["bubble-user-1", "bubble-agent-1", "bubble-user-2"],
  );
});

test("shell-ball selected-text prompt stays below an existing intent bubble even when timestamps skew", () => {
  const originalDate = globalThis.Date;
  let bubbleItemsState: ShellBallBubbleItem[] = [
    {
      bubble: {
        bubble_id: "bubble-intent-1",
        task_id: "task-intent-1",
        type: "intent_confirm",
        text: "Please confirm the intent.",
        pinned: false,
        hidden: false,
        created_at: "2026-04-11T10:05:00.000Z",
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
        turnIndex: 1,
        turnPhase: 1,
      },
    },
  ];
  let refCallCount = 0;

  class FakeDate extends Date {
    constructor(...args: any[]) {
      super(args.length === 0 ? "2026-04-11T10:00:00.000Z" : args[0]);
    }

    static now() {
      return originalDate.parse("2026-04-11T10:00:00.000Z");
    }
  }

  Object.defineProperty(globalThis, "Date", {
    configurable: true,
    value: FakeDate,
  });

  try {
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
          refCallCount += 1;

          if (refCallCount === 3) {
            return { current: 1 };
          }

          return { current: value };
        },
        useState<T>(value: T) {
          const resolvedValue = typeof value === "function" ? (value as () => T)() : value;

          if (Array.isArray(resolvedValue) && resolvedValue.every((item) => item && typeof item === "object" && "bubble" in item)) {
            return [bubbleItemsState as unknown as T, (nextValue: T | ((currentValue: T) => T)) => {
              bubbleItemsState = typeof nextValue === "function"
                ? (nextValue as (currentValue: T) => T)(bubbleItemsState as unknown as T) as unknown as ShellBallBubbleItem[]
                : nextValue as unknown as ShellBallBubbleItem[];
            }] as const;
          }

          return [resolvedValue, () => {}] as const;
        },
      },
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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

    const { handleSelectedTextPrompt } = useShellBallCoordinator({
      visualState: "hover_input",
      regionActive: false,
      inputValue: "",
      inputFocused: false,
      finalizedSpeechPayload: null,
      voicePreview: null,
      voiceHintMode: "hidden",
      setInputValue: () => {},
      onFinalizedSpeechHandled: () => {},
      onRegionEnter: () => {},
      onRegionLeave: () => {},
      onInputHoverChange: () => {},
      onInputFocusChange: () => {},
      onSubmitText: () => {},
      onAttachFile: () => {},
      onPrimaryClick: () => {},
    });

    handleSelectedTextPrompt("");

    assert.deepEqual(
      bubbleItemsState.map((item) => ({
        bubbleId: item.bubble.bubble_id,
        text: item.bubble.text,
        turnIndex: item.desktop.turnIndex,
        turnPhase: item.desktop.turnPhase,
      })),
      [
        {
          bubbleId: "bubble-intent-1",
          text: "Please confirm the intent.",
          turnIndex: 1,
          turnPhase: 1,
        },
        {
          bubbleId: bubbleItemsState[1]?.bubble.bubble_id,
          text: "识别到选中了文字",
          turnIndex: 2,
          turnPhase: 0,
        },
      ],
    );
  } finally {
    Object.defineProperty(globalThis, "Date", {
      configurable: true,
      value: originalDate,
    });
  }
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
      getShellBallPinnedBubbleIdFromLabel(): string | null {
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
    regionActive: false,
    inputValue: "",
    inputFocused: false,
    finalizedSpeechPayload: null,
    voicePreview: null,
    voiceHintMode: "hidden",
    setInputValue: () => {},
    onFinalizedSpeechHandled: () => {},
    onRegionEnter: () => {},
    onRegionLeave: () => {},
    onInputHoverChange: () => {},
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

test("shell-ball submit auto-opens formal delivery results through the shared desktop flow", async () => {
  const listeners = new Map<string, (event: { payload: unknown }) => void>();
  const openTaskDeliveryCalls: string[] = [];
  const openTaskDetailCalls: string[] = [];
  const executePlans: Array<{
    feedback: string;
    mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
    path: string | null;
    taskId: string | null;
    url: string | null;
  }> = [];
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
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
      "@/rpc/subscriptions": {
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask(taskId: string) {
          openTaskDeliveryCalls.push(taskId);
          return Promise.resolve({ task_id: taskId });
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "宸叉墦寮€鏈湴鏂囦欢銆?",
            mode: "open_local_path" as const,
            path: "C:\\output\\summary.docx",
            taskId: "task-auto-open",
            url: null,
          };
        },
        performTaskOpenExecution(plan: {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        }, options?: {
          onOpenTaskDetail?: (input: {
            plan: {
              feedback: string;
              mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
              path: string | null;
              taskId: string | null;
              url: string | null;
            };
            taskId: string;
          }) => Promise<string | void> | string | void;
        }) {
          executePlans.push(plan);
          if (plan.mode === "task_detail" && plan.taskId && options?.onOpenTaskDetail) {
            return Promise.resolve(options.onOpenTaskDetail({ plan, taskId: plan.taskId })).then(
              (feedback) => typeof feedback === "string" && feedback.trim() !== "" ? feedback : plan.feedback,
            );
          }
          return Promise.resolve(plan.feedback);
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen(taskId: string) {
          openTaskDetailCalls.push(taskId);
          return Promise.resolve();
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve(null);
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "璇锋€荤粨杩欎釜鏂囨。",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => ({
          task: {
            task_id: "task-auto-open",
          },
          bubble_message: null,
          delivery_result: {
            type: "workspace_document",
            title: "summary.docx",
            preview_text: "宸蹭负浣犵敓鎴愭€荤粨鏂囨。",
            payload: {
              path: null,
              task_id: "task-auto-open",
              url: null,
            },
          },
        }) as any,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      listeners.get(shellBallWindowSyncEvents.primaryAction)?.({
        payload: {
          source: "input",
          action: "submit",
        },
      });

      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  assert.deepEqual(openTaskDeliveryCalls, ["task-auto-open"]);
  assert.deepEqual(
    executePlans.map((plan) => ({
      mode: plan.mode,
      path: plan.path,
      taskId: plan.taskId,
    })),
    [
      {
        mode: "open_local_path",
        path: "C:\\output\\summary.docx",
        taskId: "task-auto-open",
      },
    ],
  );
  assert.deepEqual(openTaskDetailCalls, []);
});

test("shell-ball voice submit reuses task tracking and task-detail auto-open flow", async () => {
  const openTaskDeliveryCalls: string[] = [];
  const openTaskDetailCalls: string[] = [];
  const voiceSubmitCalls: string[] = [];
  let finalizedSpeechHandledCount = 0;
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask(taskId: string) {
          openTaskDeliveryCalls.push(taskId);
          return Promise.resolve({ task_id: taskId });
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: "task-voice-auto-open",
            url: null,
          };
        },
        performTaskOpenExecution(plan: {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        }, options?: {
          onOpenTaskDetail?: (input: {
            plan: {
              feedback: string;
              mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
              path: string | null;
              taskId: string | null;
              url: string | null;
            };
            taskId: string;
          }) => Promise<string | void> | string | void;
        }) {
          if (plan.mode === "task_detail" && plan.taskId && options?.onOpenTaskDetail) {
            return Promise.resolve(options.onOpenTaskDetail({ plan, taskId: plan.taskId })).then(
              (feedback) => typeof feedback === "string" && feedback.trim() !== "" ? feedback : plan.feedback,
            );
          }

          return Promise.resolve(plan.feedback);
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen(taskId: string) {
          openTaskDetailCalls.push(taskId);
          return Promise.resolve();
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve(null);
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "",
        inputFocused: false,
        finalizedSpeechPayload: "开始处理",
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {
          finalizedSpeechHandledCount += 1;
        },
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onSubmitVoiceText: async (text) => {
          voiceSubmitCalls.push(text);
          return {
            task: {
              task_id: "task-voice-auto-open",
            },
            bubble_message: null,
            delivery_result: {
              type: "task_detail",
              title: "Task detail",
              preview_text: "open task detail",
              payload: {
                path: null,
                task_id: "task-voice-auto-open",
                url: null,
              },
            },
          } as any;
        },
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  assert.deepEqual(voiceSubmitCalls, ["开始处理"]);
  assert.equal(finalizedSpeechHandledCount, 1);
  assert.deepEqual(openTaskDeliveryCalls, ["task-voice-auto-open"]);
  assert.deepEqual(openTaskDetailCalls, ["task-voice-auto-open"]);
});

test("shell-ball replays approval.pending notifications that arrive before submit resolves", async () => {
  let approvalPendingListener: ((payload: {
    task_id: string;
    approval_request: {
      approval_id: string;
      operation_name: string;
      target_object: string;
      reason: string;
    };
  }) => void) | null = null;
  let resolveSubmit: ((value: {
    task: {
      task_id: string;
    };
    bubble_message: null;
    delivery_result: null;
  }) => void) | null = null;
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeApprovalPending(callback: typeof approvalPendingListener) {
          approvalPendingListener = callback;
          return () => {};
        },
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeTaskUpdated() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return new Promise((resolve) => {
            resolveSubmit = resolve as typeof resolveSubmit;
          });
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask() {
          return Promise.resolve(null);
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: null,
            url: null,
          };
        },
        performTaskOpenExecution() {
          return Promise.resolve("open task detail");
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen() {
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handlePrimaryAction } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "截屏",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      const submitPromise = handlePrimaryAction("submit");
      await flushAsyncEffects();

      approvalPendingListener?.({
        task_id: "task-screen-approval",
        approval_request: {
          approval_id: "approval-screen-1",
          operation_name: "capture_screen",
          target_object: "foreground window",
          reason: "Needs permission before reading the current window.",
        },
      });

      resolveSubmit?.({
        task: {
          task_id: "task-screen-approval",
        },
        bubble_message: null,
        delivery_result: null,
      });

      await submitPromise;
      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  const approvalBubble = reactRuntime.getBubbleItems().find((item) => item.desktop.inlineApproval?.approvalId === "approval-screen-1");
  assert.ok(approvalBubble);
  assert.equal(approvalBubble?.bubble.task_id, "task-screen-approval");
  assert.match(approvalBubble?.bubble.text ?? "", /Waiting for approval/i);
});

test("shell-ball replays delivery.ready notifications that arrive before submit resolves", async () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  let deliveryReadyListener: ((payload: {
    task_id: string;
    delivery_result: {
      type: string;
      title: string;
      preview_text: string;
      payload: {
        path: string | null;
        task_id: string | null;
        url: string | null;
      };
    };
  }) => void) | null = null;
  let resolveSubmit: ((value: {
    task: {
      task_id: string;
    };
    bubble_message: null;
    delivery_result: null;
  }) => void) | null = null;
  const openTaskDeliveryCalls: string[] = [];
  const openTaskDetailCalls: string[] = [];
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeApprovalPending() {
          return () => {};
        },
        subscribeDeliveryReady(callback: typeof deliveryReadyListener) {
          deliveryReadyListener = callback;
          return () => {};
        },
        subscribeTaskUpdated() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return new Promise((resolve) => {
            resolveSubmit = resolve as typeof resolveSubmit;
          });
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask(taskId: string) {
          openTaskDeliveryCalls.push(taskId);
          return Promise.resolve({ task_id: taskId });
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: "task-screen-delivery-race",
            url: null,
          };
        },
        performTaskOpenExecution(plan: {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        }, options?: {
          onOpenTaskDetail?: (input: {
            plan: {
              feedback: string;
              mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
              path: string | null;
              taskId: string | null;
              url: string | null;
            };
            taskId: string;
          }) => Promise<string | void> | string | void;
        }) {
          if (plan.mode === "task_detail" && plan.taskId && options?.onOpenTaskDetail) {
            return Promise.resolve(options.onOpenTaskDetail({ plan, taskId: plan.taskId })).then(
              (feedback) => typeof feedback === "string" && feedback.trim() !== "" ? feedback : plan.feedback,
            );
          }

          return Promise.resolve(plan.feedback);
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen(taskId: string) {
          openTaskDetailCalls.push(taskId);
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handlePrimaryAction } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "截图",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      const submitPromise = handlePrimaryAction("submit");
      await flushAsyncEffects();

      deliveryReadyListener?.({
        task_id: "task-screen-delivery-race",
        delivery_result: {
          type: "workspace_document",
          title: "summary.docx",
          preview_text: "summary.docx",
          payload: {
            path: "C:\\output\\summary.docx",
            task_id: "task-screen-delivery-race",
            url: null,
          },
        },
      });

      resolveSubmit?.({
        task: {
          task_id: "task-screen-delivery-race",
        },
        bubble_message: null,
        delivery_result: null,
      });

      await submitPromise;
      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  assert.match(coordinatorSource, /const queuedDeliveryReadyNotificationsRef = useRef\(new Map<string, QueuedDeliveryReadyNotification\[]>\(\)\);/);
  assert.match(coordinatorSource, /queuedDeliveryNotifications\.forEach\(\(notification\) => \{\s*appendDeliveryReadyBubble\(notification\);/);
  assert.deepEqual(openTaskDeliveryCalls, ["task-screen-delivery-race"]);
  assert.deepEqual(openTaskDetailCalls, ["task-screen-delivery-race"]);
  const deliveryBubble = reactRuntime.getBubbleItems().find((item) => item.bubble.task_id === "task-screen-delivery-race" && item.bubble.type === "result");
  assert.ok(deliveryBubble);
  assert.match(deliveryBubble?.bubble.text ?? "", /summary\.docx/i);
});

test("shell-ball replays task.updated notifications that arrive before submit resolves", async () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  let taskUpdatedListener: ((payload: {
    task_id: string;
    session_id: string | null;
    status: "processing" | "waiting_auth";
  }) => void) | null = null;
  let resolveSubmit: ((value: {
    task: {
      task_id: string;
      status: "processing";
    };
    bubble_message: null;
    delivery_result: null;
  }) => void) | null = null;
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeApprovalPending() {
          return () => {};
        },
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeTaskUpdated(callback: typeof taskUpdatedListener) {
          taskUpdatedListener = callback;
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return new Promise((resolve) => {
            resolveSubmit = resolve as typeof resolveSubmit;
          });
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask() {
          return Promise.resolve(null);
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: null,
            url: null,
          };
        },
        performTaskOpenExecution() {
          return Promise.resolve("open task detail");
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen() {
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handlePrimaryAction } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "截屏",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      const submitPromise = handlePrimaryAction("submit");
      await flushAsyncEffects();

      taskUpdatedListener?.({
        task_id: "task-screen-status-race",
        session_id: null,
        status: "waiting_auth",
      });

      resolveSubmit?.({
        task: {
          task_id: "task-screen-status-race",
          status: "processing",
        },
        bubble_message: null,
        delivery_result: null,
      });

      await submitPromise;
      await flushAsyncEffects();
    },
  );

  assert.match(coordinatorSource, /const queuedTaskUpdatedNotificationsRef = useRef\(new Map<string, QueuedTaskUpdatedNotification>\(\)\);/);
  assert.equal(useShellBallStore.getState().visualState, "waiting_auth");
});

test("shell-ball replays runtime notifications that arrive before submit resolves", async () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  let runtimeListener: ((payload: {
    task_id: string;
    message: string;
  }) => void) | null = null;
  let resolveSubmit: ((value: {
    task: {
      task_id: string;
      status: "processing";
    };
    bubble_message: null;
    delivery_result: null;
  }) => void) | null = null;
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeAllTaskRuntime(callback: typeof runtimeListener) {
          runtimeListener = callback;
          return () => {};
        },
        subscribeApprovalPending() {
          return () => {};
        },
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeTaskUpdated() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return new Promise((resolve) => {
            resolveSubmit = resolve as typeof resolveSubmit;
          });
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask() {
          return Promise.resolve(null);
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: null,
            url: null,
          };
        },
        performTaskOpenExecution() {
          return Promise.resolve("open task detail");
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen() {
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handlePrimaryAction } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "截屏",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      const submitPromise = handlePrimaryAction("submit");
      await flushAsyncEffects();

      runtimeListener?.({
        task_id: "task-runtime-race",
        message: "Added another instruction.",
      });

      resolveSubmit?.({
        task: {
          task_id: "task-runtime-race",
          status: "processing",
        },
        bubble_message: null,
        delivery_result: null,
      });

      await submitPromise;
      await flushAsyncEffects();
    },
  );

  assert.match(coordinatorSource, /const queuedRuntimeNotificationsRef = useRef\(new Map<string, QueuedRuntimeNotification\[]>\(\)\);/);
  const runtimeBubble = reactRuntime.getBubbleItems().find((item) => item.bubble.task_id === "task-runtime-race" && item.bubble.text === "Added another instruction.");
  assert.ok(runtimeBubble);
});

test("shell-ball ignores untracked approval.pending notifications without a pending submit", async () => {
  let approvalPendingListener: ((payload: {
    task_id: string;
    approval_request: {
      approval_id: string;
      operation_name: string;
      target_object: string;
      reason: string;
    };
  }) => void) | null = null;
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeApprovalPending(callback: typeof approvalPendingListener) {
          approvalPendingListener = callback;
          return () => {};
        },
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeTaskUpdated() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve(null);
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask() {
          return Promise.resolve(null);
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: null,
            url: null,
          };
        },
        performTaskOpenExecution() {
          return Promise.resolve("open task detail");
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen() {
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "",
        inputFocused: false,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      await flushAsyncEffects();

      approvalPendingListener?.({
        task_id: "task-dashboard-approval",
        approval_request: {
          approval_id: "approval-dashboard-1",
          operation_name: "write_file",
          target_object: "workspace/report.md",
          reason: "Needs permission before editing the workspace.",
        },
      });

      await flushAsyncEffects();
    },
  );

  const approvalBubble = reactRuntime.getBubbleItems().find((item) => item.desktop.inlineApproval?.approvalId === "approval-dashboard-1");
  assert.equal(approvalBubble, undefined);
});

test("shell-ball approval responses do not overwrite newer task subscription state", async () => {
  const listeners = new Map<string, (event: { payload: unknown }) => void>();
  let approvalPendingListener: ((payload: {
    task_id: string;
    approval_request: {
      approval_id: string;
      operation_name: string;
      target_object: string;
      reason: string;
    };
  }) => void) | null = null;
  let taskUpdatedListener: ((payload: {
    task_id: string;
    session_id: string | null;
    status: "completed" | "processing" | "waiting_auth";
  }) => void) | null = null;
  let resolveApprovalResponse: ((value: {
    data: {
      task: {
        task_id: string;
        status: "processing";
      };
      bubble_message: null;
    };
  }) => void) | null = null;
  const reactRuntime = createImmediateShellBallReactRuntime();
  useShellBallStore.getState().setVisualState("hover_input");

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
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
      "@/rpc/methods": {
        respondSecurityDetailed() {
          return new Promise((resolve) => {
            resolveApprovalResponse = resolve as typeof resolveApprovalResponse;
          });
        },
      },
      "@/rpc/subscriptions": {
        subscribeApprovalPending(callback: typeof approvalPendingListener) {
          approvalPendingListener = callback;
          return () => {};
        },
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeTaskUpdated(callback: typeof taskUpdatedListener) {
          taskUpdatedListener = callback;
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve({
            task: {
              task_id: "task-approval-response-race",
              status: "waiting_auth",
            },
            bubble_message: null,
            delivery_result: null,
          });
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask() {
          return Promise.resolve(null);
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "open task detail",
            mode: "task_detail" as const,
            path: null,
            taskId: null,
            url: null,
          };
        },
        performTaskOpenExecution() {
          return Promise.resolve("open task detail");
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen() {
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handlePrimaryAction } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "截屏",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      await handlePrimaryAction("submit");
      await flushAsyncEffects();

      approvalPendingListener?.({
        task_id: "task-approval-response-race",
        approval_request: {
          approval_id: "approval-response-race-1",
          operation_name: "capture_screen",
          target_object: "foreground window",
          reason: "Needs permission before reading the current window.",
        },
      });
      await flushAsyncEffects();

      const approvalBubble = reactRuntime.getBubbleItems().find((item) => item.desktop.inlineApproval?.approvalId === "approval-response-race-1");
      assert.ok(approvalBubble);

      listeners.get(shellBallWindowSyncEvents.bubbleAction)?.({
        payload: {
          source: "bubble",
          action: "allow_approval",
          bubbleId: approvalBubble?.bubble.bubble_id,
        },
      });
      await flushAsyncEffects();

      taskUpdatedListener?.({
        task_id: "task-approval-response-race",
        session_id: null,
        status: "completed",
      });
      await flushAsyncEffects();

      resolveApprovalResponse?.({
        data: {
          task: {
            task_id: "task-approval-response-race",
            status: "processing",
          },
          bubble_message: null,
        },
      });
      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  assert.equal(useShellBallStore.getState().visualState, "idle");
});

test("shell-ball delivery.ready auto-opens tracked formal delivery results", async () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  let deliveryReadyListener: ((payload: {
    task_id: string;
    delivery_result: {
      type: string;
      title: string;
      preview_text: string;
      payload: {
        path: string | null;
        task_id: string | null;
        url: string | null;
      };
    };
  }) => void) | null = null;
  const openTaskDeliveryCalls: string[] = [];
  const openTaskDetailCalls: string[] = [];
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/subscriptions": {
        subscribeDeliveryReady(callback: typeof deliveryReadyListener) {
          deliveryReadyListener = callback;
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask(taskId: string) {
          openTaskDeliveryCalls.push(taskId);
          return Promise.resolve({ task_id: taskId });
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "宸插湪鏂囦欢澶逛腑瀹氫綅缁撴灉銆?",
            mode: "reveal_local_path" as const,
            path: "C:\\output\\summary.docx",
            taskId: "task-delivery-ready",
            url: null,
          };
        },
        performTaskOpenExecution(plan: {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        }, options?: {
          onOpenTaskDetail?: (input: {
            plan: {
              feedback: string;
              mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
              path: string | null;
              taskId: string | null;
              url: string | null;
            };
            taskId: string;
          }) => Promise<string | void> | string | void;
        }) {
          if (plan.mode === "task_detail" && plan.taskId && options?.onOpenTaskDetail) {
            return Promise.resolve(options.onOpenTaskDetail({ plan, taskId: plan.taskId })).then(
              (feedback) => typeof feedback === "string" && feedback.trim() !== "" ? feedback : plan.feedback,
            );
          }
          return Promise.resolve(plan.feedback);
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen(taskId: string) {
          openTaskDetailCalls.push(taskId);
          return Promise.resolve();
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve({
            task: {
              task_id: "task-delivery-ready",
            },
            bubble_message: null,
            delivery_result: null,
          });
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handleClipboardPrompt } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "",
        inputFocused: false,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: () => null,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      await handleClipboardPrompt("璇锋€荤粨杩欎釜鏂囨。");
      await flushAsyncEffects();

      deliveryReadyListener?.({
        task_id: "task-delivery-ready",
        delivery_result: {
          type: "reveal_in_folder",
          title: "summary.docx",
          preview_text: "宸插畬鎴愭€荤粨锛屾鍦ㄥ畾浣嶆枃浠躲€?",
          payload: {
            path: "C:\\output\\summary.docx",
            task_id: "task-delivery-ready",
            url: null,
          },
        },
      });

      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  assert.match(coordinatorSource, /void autoOpenShellBallDeliveryResult\(payload\.task_id, payload\.delivery_result\);/);
  assert.deepEqual(openTaskDeliveryCalls, ["task-delivery-ready"]);
  assert.deepEqual(openTaskDetailCalls, []);
});

test("shell-ball task-detail deliveries auto-open the dashboard detail view", async () => {
  const listeners = new Map<string, (event: { payload: unknown }) => void>();
  const openTaskDeliveryCalls: string[] = [];
  const openTaskDetailCalls: string[] = [];
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
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
      "@/rpc/subscriptions": {
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeAllTaskRuntime() {
          return () => {};
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask(taskId: string) {
          openTaskDeliveryCalls.push(taskId);
          return Promise.resolve({ task_id: taskId });
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "宸插畾浣嶅埌浠诲姟璇︽儏銆?",
            mode: "task_detail" as const,
            path: null,
            taskId: "task-task-detail",
            url: null,
          };
        },
        performTaskOpenExecution(plan: {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        }, options?: {
          onOpenTaskDetail?: (input: {
            plan: {
              feedback: string;
              mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
              path: string | null;
              taskId: string | null;
              url: string | null;
            };
            taskId: string;
          }) => Promise<string | void> | string | void;
        }) {
          return Promise.resolve(options?.onOpenTaskDetail?.({ plan, taskId: "task-task-detail" }) ?? plan.feedback).then(
            (feedback) => typeof feedback === "string" && feedback.trim() !== "" ? feedback : plan.feedback,
          );
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen(taskId: string) {
          openTaskDetailCalls.push(taskId);
          return Promise.resolve();
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve(null);
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "task detail",
        inputFocused: true,
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => ({
          task: {
            task_id: "task-task-detail",
          },
          bubble_message: null,
          delivery_result: {
            type: "task_detail",
            title: "Task detail",
            preview_text: "宸插畾浣嶅埌浠诲姟璇︽儏銆?",
            payload: {
              path: null,
              task_id: "task-task-detail",
              url: null,
            },
          },
        }) as any,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      listeners.get(shellBallWindowSyncEvents.primaryAction)?.({
        payload: {
          source: "input",
          action: "submit",
        },
      });

      await flushAsyncEffects();
      await flushAsyncEffects();
    },
  );

  assert.deepEqual(openTaskDeliveryCalls, ["task-task-detail"]);
  assert.deepEqual(openTaskDetailCalls, ["task-task-detail"]);
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
  assert.match(syncSource, /export type ShellBallBubbleAction = "pin" \| "unpin" \| "delete" \| "allow_approval" \| "deny_approval";/);
  assert.match(syncSource, /export type ShellBallBubbleActionSource = "bubble" \| "pinned_window";/);
  assert.match(coordinatorSource, /currentWindow\.listen<ShellBallBubbleActionPayload>\(shellBallWindowSyncEvents\.bubbleAction/);
  assert.match(coordinatorSource, /setBubbleItems\(\(currentItems\) => applyShellBallBubbleAction\(currentItems, payload\)\)/);
});

test("shell-ball approval bubble actions stay on the formal security respond path", () => {
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const bubbleWindowSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallBubbleWindow.tsx"), "utf8");
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(appSource, /onAllowApprovalBubble=\{\(bubbleId\) => \{\s*handleCoordinatorBubbleAction\(\{ action: "allow_approval", bubbleId, source: "bubble" \}\);/);
  assert.match(appSource, /onDenyApprovalBubble=\{\(bubbleId\) => \{\s*handleCoordinatorBubbleAction\(\{ action: "deny_approval", bubbleId, source: "bubble" \}\);/);
  assert.match(bubbleWindowSource, /emitShellBallBubbleAction\("allow_approval", bubbleId\)/);
  assert.match(bubbleWindowSource, /emitShellBallBubbleAction\("deny_approval", bubbleId\)/);
  assert.match(bubbleWindowSource, /clickThrough: snapshot\.bubbleRegion\.clickThrough,/);
  assert.match(coordinatorSource, /if \(payload\.action === "allow_approval" \|\| payload\.action === "deny_approval"\) \{/);
  assert.match(coordinatorSource, /const decision: ApprovalDecision = payload\.action === "allow_approval" \? "allow_once" : "deny_once";/);
  assert.match(coordinatorSource, /const response = await respondSecurityDetailed\(\{/);
  assert.match(coordinatorSource, /approval_id: inlineApproval\.approvalId,/);
  assert.match(coordinatorSource, /remember_rule: false,/);
  assert.match(coordinatorSource, /createShellBallApprovalPendingBubbleItem\(\{/);
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
          text: "Keep this pinned. https://github.com/1024XEngineer/CialloClaw",
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
        useShellBallPinnedBubbleSnapshot() {
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
  assert.match(markup, /href="https:\/\/github\.com\/1024XEngineer\/CialloClaw"/);
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
  assert.match(shellBallStyles, /\.shell-ball-input-bar,\s*\.shell-ball-input-bar--hidden\s*\{[\s\S]*min-width:\s*var\(--shell-ball-helper-width\);/);
  assert.match(shellBallStyles, /\.shell-ball-input-bar,\s*\.shell-ball-input-bar--hidden\s*\{[\s\S]*width:\s*fit-content;/);
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

test("shell-ball speech recognition treats no-speech as a silent retryable interruption", () => {
  assert.equal(shouldLogShellBallSpeechRecognitionError("no-speech"), false);
  assert.equal(shouldLogShellBallSpeechRecognitionError("network"), true);
});

test("shell-ball surface renders the mascot-only floating structure", () => {
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
      onDragMove: () => {},
      onDragEnd: () => {},
      onDragCancel: () => {},
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
});

test("shell-ball surface keeps drag and click on the mascot hotspot only", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallSurface, {
      floatingBallSize: "small",
      visualState: "hover_input",
      voicePreview: null,
      motionConfig: getShellBallMotionConfig("hover_input"),
      onPrimaryClick: () => {},
      onDoubleClick: () => {},
      onRegionEnter: () => {},
      onRegionLeave: () => {},
      onDragStart: () => {},
      onDragMove: () => {},
      onDragEnd: () => {},
      onDragCancel: () => {},
      onPressStart: () => {},
      onPressMove: () => {},
      onPressEnd: () => false,
      onPressCancel: () => {},
    }),
  );

  assert.match(markup, /data-shell-ball-zone="interaction"/);
  assert.match(markup, /data-shell-ball-zone="voice-hotspot"/);
  assert.match(markup, /data-floating-ball-size="small"/);
  assert.doesNotMatch(markup, /shell-ball-surface__host-drag-zone/);
  assert.match(markup, /shell-ball-surface__interaction-zone/);
});

test("shell-ball floating size normalization falls back to medium", () => {
  assert.equal(normalizeShellBallFloatingSize("small"), "small");
  assert.equal(normalizeShellBallFloatingSize("large"), "large");
  assert.equal(normalizeShellBallFloatingSize("unknown"), "medium");
  assert.equal(normalizeShellBallFloatingSize(undefined), "medium");
});

test("shell-ball mascot hotspot policy only opens primary click for selected-text prompts", () => {
  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "voice_locked",
      gesture: "single_click",
      suppressed: false,
    }),
    "noop",
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

  assert.equal(
    getShellBallMascotHotspotGestureAction({
      visualState: "idle",
      gesture: "single_click",
      suppressed: false,
      selectionIndicatorVisible: true,
    }),
    "primary_click",
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

test("shell-ball mascot drag policy suppresses click gestures only after meaningful pointer drift", () => {
  assert.equal(
    shouldSuppressShellBallMascotHotspotGestures({
      startX: 100,
      startY: 100,
      pointerX: 100 + Math.floor(SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX / 2),
      pointerY: 103,
    }),
    false,
  );

  assert.equal(
    shouldSuppressShellBallMascotHotspotGestures({
      startX: 100,
      startY: 100,
      pointerX: 100 + SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX + 1,
      pointerY: 100,
    }),
    true,
  );

  assert.equal(
    shouldSuppressShellBallMascotHotspotGestures({
      startX: null,
      startY: null,
      pointerX: 118,
      pointerY: 114,
    }),
    false,
  );
});

test("shell-ball voice swipe contract keeps upward lock and downward cancel explicit", () => {
  assert.equal(
    getShellBallVoicePreviewFromEvent({
      hintMode: "lock",
      startX: 100,
      startY: 100,
      pointerX: 100,
      pointerY: 100 - SHELL_BALL_LOCK_DELTA_PX,
      fallbackPreview: null,
    }),
    "lock",
  );

  assert.equal(
    getShellBallVoicePreviewFromEvent({
      hintMode: "cancel",
      startX: 100,
      startY: 100,
      pointerX: 100,
      pointerY: 100 + SHELL_BALL_CANCEL_DELTA_PX,
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

test("shell-ball hover timing stays driven by real hotspot enter and leave events", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(interactionSource, /function handleRegionEnter\(\) \{[\s\S]*dispatch\("pointer_enter_hotspot", \{[\s\S]*regionActive: true,[\s\S]*hoverRetained: getHoverRetained\(\),[\s\S]*\}\);[\s\S]*syncVisualState\(\);[\s\S]*\}/);
  assert.match(interactionSource, /function handleRegionLeave\(\) \{[\s\S]*dispatch\("pointer_leave_region", \{[\s\S]*regionActive: false,[\s\S]*hoverRetained: getHoverRetained\(\),[\s\S]*\}\);[\s\S]*syncVisualState\(\);[\s\S]*\}/);
});

test("shell-ball coordinator does not resurrect hover presence from hover_input state alone", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(coordinatorSource, /regionActiveRef\.current = input\.regionActive;/);
  assert.match(coordinatorSource, /inputFocusedRef\.current = input\.inputFocused;/);
  assert.match(coordinatorSource, /const voicePreviewActiveState =[\s\S]*input\.visualState === "voice_listening" \|\| input\.visualState === "voice_locked";/);
  assert.doesNotMatch(coordinatorSource, /input\.visualState === "hover_input" \|\| input\.visualState === "voice_listening" \|\| input\.visualState === "voice_locked"/);
});

test("shell-ball coordinator keeps thinking bubbles visible and re-arms hide timers for replacement replies", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(
    coordinatorSource,
    /function shouldKeepShellBallBubbleRegionVisibleForTaskState\(visualState: ShellBallVisualState\) \{[\s\S]*visualState === "confirming_intent" \|\| visualState === "processing" \|\| visualState === "waiting_auth";[\s\S]*\}/,
  );
  assert.match(
    coordinatorSource,
    /if \(shouldKeepShellBallBubbleRegionVisibleForTaskState\(visualStateRef\.current\)\) \{\s*applyBubbleVisibilityPhase\("visible"\);\s*return;\s*\}/,
  );
  assert.match(
    coordinatorSource,
    /const bubbleContentAdvanced =[\s\S]*visibleBubbleCount === previousVisibleBubbleCount[\s\S]*latestVisibleBubbleId !== null[\s\S]*latestVisibleBubbleId !== previousLatestVisibleBubbleId;/,
  );
  assert.match(
    coordinatorSource,
    /if \(visibleBubbleCount > previousVisibleBubbleCount \|\| bubbleContentAdvanced\) \{\s*revealBubbleRegion\(\);\s*scheduleBubbleRegionHide\(\);\s*\}/,
  );
});

test("shell-ball direct input starts fresh requests while explicit session reuse stays opt-in", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");
  const sessionServiceSource = readFileSync(resolve(desktopRoot, "src/services/conversationSessionService.ts"), "utf8");
  const agentInputServiceSource = readFileSync(resolve(desktopRoot, "src/services/agentInputService.ts"), "utf8");
  const taskServiceSource = readFileSync(resolve(desktopRoot, "src/services/taskService.ts"), "utf8");
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(sessionServiceSource, /export function getCurrentConversationSessionId\(\) \{/);
  assert.match(sessionServiceSource, /export function rememberConversationSessionFromTask\(task: Task \| null \| undefined\) \{/);
  assert.match(sessionServiceSource, /export function rememberConversationPageContextFromTask\(/);
  assert.match(sessionServiceSource, /export function getConversationPageContextForSession\(/);
  assert.doesNotMatch(interactionSource, /function ensureConversationSessionId\(\) \{/);
  assert.doesNotMatch(interactionSource, /createShellBallConversationSessionId/);
  assert.doesNotMatch(interactionSource, /startShellBallFileTask\(\{[\s\S]*sessionId: getCurrentConversationSessionId\(\),/);
  assert.doesNotMatch(interactionSource, /trigger: "hover_text_input",[\s\S]*includeForegroundBrowserPageContext: true,[\s\S]*sessionId: getCurrentConversationSessionId\(\),/);
  assert.doesNotMatch(interactionSource, /trigger: "voice_commit",[\s\S]*includeForegroundBrowserPageContext: true,[\s\S]*sessionId: getCurrentConversationSessionId\(\),/);
  assert.doesNotMatch(agentInputServiceSource, /getCurrentConversationSessionId/);
  assert.doesNotMatch(taskServiceSource, /getCurrentConversationSessionId/);
  assert.doesNotMatch(appSource, /getCurrentConversationSessionId,/);
  assert.doesNotMatch(coordinatorSource, /getCurrentConversationSessionId\?: \(\) => string \| undefined;/);
  assert.doesNotMatch(coordinatorSource, /sessionId: handlersRef\.current\.getCurrentConversationSessionId\?\.\(\),/);
});

test("shell-ball direct input does not expose task follow-up steering controls", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const inputBarSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallInputBar.tsx"), "utf8");
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  assert.doesNotMatch(coordinatorSource, /shellBallFollowUpTarget/);
  assert.doesNotMatch(coordinatorSource, /steerTask\(\{/);
  assert.doesNotMatch(coordinatorSource, /onPrepareTextSubmitDraft/);
  assert.doesNotMatch(coordinatorSource, /onRestoreTextSubmitDraft/);
  assert.doesNotMatch(appSource, /followUpTarget/);
  assert.doesNotMatch(appSource, /handleFollowUpToggle/);
  assert.doesNotMatch(appSource, /prepareTextSubmitDraft/);
  assert.doesNotMatch(appSource, /restorePreparedTextSubmitDraft/);
  assert.doesNotMatch(inputBarSource, /followUpArmed\?: boolean;/);
  assert.doesNotMatch(inputBarSource, /followUpLabel\?: string \| null;/);
  assert.doesNotMatch(inputBarSource, /onToggleFollowUp\?: \(\) => void;/);
  assert.doesNotMatch(inputBarSource, /发送到当前任务/);
  assert.doesNotMatch(inputBarSource, /补充当前任务/);
});

test("shell-ball direct submit shows a detected-page status bubble before the task reply", async () => {
  const reactRuntime = createImmediateShellBallReactRuntime();

  await withSourceModuleRuntime(
    resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"),
    {
      react: reactRuntime.react,
      "@tauri-apps/api/window": {
        getCurrentWindow() {
          return {
            label: shellBallWindowLabels.ball,
            listen() {
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
      "@/rpc/methods": {
        respondSecurityDetailed() {
          return Promise.resolve(null);
        },
      },
      "@/rpc/subscriptions": {
        subscribeApprovalPending() {
          return () => {};
        },
        subscribeDeliveryReady() {
          return () => {};
        },
        subscribeTaskUpdated() {
          return () => {};
        },
      },
      "@/services/agentInputService": {
        submitTextInput() {
          return Promise.resolve(null);
        },
      },
      "@/services/taskService": {
        startTaskFromSelectedText() {
          return Promise.resolve(null);
        },
      },
      "@/services/clipboardService": {
        readClipboardText() {
          return Promise.resolve("");
        },
      },
      "@/features/dashboard/tasks/taskOutput.service": {
        openTaskDeliveryForTask() {
          return Promise.resolve(null);
        },
        resolveTaskOpenExecutionPlan(): {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        } {
          return {
            feedback: "",
            mode: "task_detail" as const,
            path: null,
            taskId: null,
            url: null,
          };
        },
        performTaskOpenExecution() {
          return Promise.resolve("");
        },
      },
      "@/features/dashboard/shared/dashboardTaskDetailNavigation": {
        requestDashboardTaskDetailOpen() {
          return Promise.resolve();
        },
      },
      "../../platform/shellBallWindowController": {
        SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME: { width: 240, height: 140 },
        closeShellBallPinnedBubbleWindow() {
          return Promise.resolve();
        },
        emitToShellBallWindowLabel() {
          return Promise.resolve();
        },
        getShellBallPinnedBubbleIdFromLabel(): string | null {
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
      "./useShellBallWindowMetrics": {
        getShellBallBubbleAnchor() {
          return { x: 0, y: 0 };
        },
      },
    },
    async (moduleExports) => {
      const { useShellBallCoordinator } = moduleExports as {
        useShellBallCoordinator: typeof import("./useShellBallCoordinator").useShellBallCoordinator;
      };

      const { handlePrimaryAction } = useShellBallCoordinator({
        visualState: "hover_input",
        regionActive: false,
        inputValue: "帮我总结这个页面",
        inputFocused: true,
        pendingFiles: [],
        finalizedSpeechPayload: null,
        voicePreview: null,
        voiceHintMode: "hidden",
        setInputValue: () => {},
        onFinalizedSpeechHandled: () => {},
        onRegionEnter: () => {},
        onRegionLeave: () => {},
        onInputHoverChange: () => {},
        onInputFocusChange: () => {},
        onSubmitText: async () => ({
          task: {
            task_id: "task-url-context",
            status: "processing",
          },
          bubble_message: {
            bubble_id: "bubble-url-context",
            task_id: "task-url-context",
            type: "result",
            text: "我会先总结这个网页的重点内容。",
            pinned: false,
            hidden: false,
            created_at: "2026-04-26T08:00:03.000Z",
          },
          delivery_result: null,
          clientContext: {
            detectedPage: {
              appName: "Google Chrome",
              title: "OpenAI Docs",
              url: "https://platform.openai.com/docs/overview",
            },
          },
        }) as any,
        onAttachFile: () => {},
        onPrimaryClick: () => {},
      });

      await handlePrimaryAction("submit");
      await flushAsyncEffects();
    },
  );

  const taskBubbleItems = reactRuntime.getBubbleItems().filter((item) => item.bubble.task_id === "task-url-context");

  assert.deepEqual(
    taskBubbleItems.map((item) => ({
      role: item.role,
      text: item.bubble.text,
      turnPhase: item.desktop.turnPhase,
      type: item.bubble.type,
    })),
    [
      {
        role: "user",
        text: "帮我总结这个页面",
        turnPhase: 0,
        type: "result",
      },
      {
        role: "agent",
        text: "已识别当前网页：OpenAI Docs\nhttps://platform.openai.com/docs/overview",
        turnPhase: 1,
        type: "status",
      },
      {
        role: "agent",
        text: "我会先总结这个网页的重点内容。",
        turnPhase: 2,
        type: "result",
      },
    ],
  );
});

test("conversation session cache preserves real page anchors for later file continuations", () => {
  withSourceModuleRuntime(
    resolve(desktopRoot, "src/services/conversationSessionService.ts"),
    {
      "./pageContext": {
        compactPageContext,
      },
    },
    (moduleExports) => {
      const service = moduleExports as {
        getConversationPageContextForSession: (sessionId?: string) => unknown;
        rememberConversationPageContextFromTask: (task: Record<string, unknown>, pageContext: Record<string, unknown>) => unknown;
        rememberConversationSessionFromTask: (task: Record<string, unknown>) => unknown;
      };

      service.rememberConversationSessionFromTask({
        task_id: "task_shell_ball_anchor",
        session_id: "sess_shell_ball_anchor",
      });

      assert.equal(
        service.rememberConversationPageContextFromTask(
          { session_id: "sess_shell_ball_anchor" },
          { app_name: "desktop", title: "Quick Intake", url: "local://shell-ball" },
        ),
        null,
      );
      assert.equal(service.getConversationPageContextForSession("sess_shell_ball_anchor"), undefined);

      service.rememberConversationPageContextFromTask(
        { session_id: "sess_shell_ball_anchor" },
        {
          app_name: "Chrome",
          browser_kind: "chrome",
          process_id: 4412,
          process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
          title: "Build Dashboard",
          url: "https://example.com/build",
        },
      );

      assert.deepEqual(service.getConversationPageContextForSession("sess_shell_ball_anchor"), {
        app_name: "Chrome",
        title: "Build Dashboard",
        url: "https://example.com/build",
      });
      assert.deepEqual(service.getConversationPageContextForSession(), {
        app_name: "Chrome",
        title: "Build Dashboard",
        url: "https://example.com/build",
      });
    },
  );
});

test("shell-ball surface passes mascot double-click and drag wiring through the mascot only", () => {
  const surfaceSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallSurface.tsx"), "utf8");

  assert.match(surfaceSource, /onDoubleClick: \(\) => void;/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onDoubleClick=\{onDoubleClick\}/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onHotspotDragStart=\{onDragStart\}/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onHotspotDragMove=\{onDragMove\}/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onHotspotDragEnd=\{onDragEnd\}/);
  assert.match(surfaceSource, /<ShellBallMascot[\s\S]*onHotspotDragCancel=\{onDragCancel\}/);
  assert.doesNotMatch(surfaceSource, /data-shell-ball-zone="host-drag"/);
  assert.match(surfaceSource, /data-shell-ball-zone="interaction"/);
});

test("shell-ball file overlay only follows real file drags", () => {
  assert.equal(
    shouldShowShellBallFileDropOverlay({
      fileDropActive: false,
    }),
    false,
  );
  assert.equal(
    shouldShowShellBallFileDropOverlay({
      fileDropActive: true,
    }),
    true,
  );
});

test("shell-ball voice recognition source keeps locked sessions from auto-cancelling on unexpected end", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(interactionSource, /if \(recognitionStopReasonRef\.current !== "none"\) \{/);
  assert.match(interactionSource, /recognitionErrorRef\.current = event\.error;/);
  assert.match(interactionSource, /shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd\(recognitionError\)/);
  assert.match(interactionSource, /if \(shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd\(currentState\)\) \{/);
  assert.match(interactionSource, /const committedDraft = preserveUnexpectedVoiceTranscriptDraft\(\);/);
  assert.match(interactionSource, /if \(shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd\(recognitionError\) && startVoiceRecognition\(\)\) \{\s*return;\s*\}/);
});

test("shell-ball text drop populates and focuses the input instead of starting a task", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const surfaceSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallSurface.tsx"), "utf8");

  assert.match(interactionSource, /function handleDroppedText\(text: string\) \{/);
  assert.match(interactionSource, /setTrackedInputValue\(nextInputValue\);\s*handleInputFocusRequest\(\);/);
  assert.doesNotMatch(interactionSource, /startTaskFromSelectedText/);
  assert.match(appSource, /const handleSurfaceTextDrop = useCallback\(\(text: string\) => \{/);
  assert.match(appSource, /handleDroppedText\(text\);\s*window\.requestAnimationFrame\(\(\) => \{\s*focusInlineInputField\(false\);\s*\}\);/);
  assert.doesNotMatch(appSource, /emitShellBallInputRequestFocus/);
  assert.match(appSource, /useEventListener\("dragenter", handleWindowTextDrag, \{/);
  assert.match(appSource, /useEventListener\("dragover", handleWindowTextDrag, \{/);
  assert.match(appSource, /useEventListener\("dragleave", clearTextDragState, \{/);
  assert.match(appSource, /useEventListener\("drop", clearTextDragState, \{/);
  assert.match(appSource, /onTextDrop=\{handleSurfaceTextDrop\}/);
  assert.match(appSource, /textDropActive=\{shouldArmShellBallTextDropTarget\(/);
  assert.match(surfaceSource, /onDragEnterCapture=\{handleDragOver\}/);
  assert.match(surfaceSource, /onDropCapture=\{handleDrop\}/);
  assert.doesNotMatch(surfaceSource, /onDrop=\{handleDrop\}/);
  assert.match(surfaceSource, /className="shell-ball-surface__text-drop-target"/);
});

test("shell-ball clipboard command stays frontend-only and reads the desktop clipboard service", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const clipboardServiceSource = readFileSync(resolve(desktopRoot, "src/services/clipboardService.ts"), "utf8");
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(coordinatorSource, /const SHELL_BALL_CLIPBOARD_COMMAND = "粘贴板";/);
  assert.match(coordinatorSource, /shouldHandleShellBallClipboardCommand\(/);
  assert.match(coordinatorSource, /const clipboardText = await readClipboardText\(\);/);
  assert.match(coordinatorSource, /Clipboard is unavailable right now\./);
  assert.match(clipboardServiceSource, /import \{ readText \} from "@tauri-apps\/plugin-clipboard-manager";/);
  assert.match(clipboardServiceSource, /export async function readClipboardText\(\)/);
  assert.doesNotMatch(interactionSource, /SHELL_BALL_CLIPBOARD_COMMAND/);
});

test("shell-ball clipboard prompts stay active for 10 seconds after clipboard refresh", () => {
  assert.equal(
    isShellBallClipboardPromptActive({
      text: "clipboard text",
      expiresAt: 2_000,
    }, 1_500),
    true,
  );
  assert.equal(
    isShellBallClipboardPromptActive({
      text: "clipboard text",
      expiresAt: 2_000,
    }, 2_000),
    false,
  );
});

test("shell-ball app routes fresh clipboard prompts through the formal text submit path", () => {
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const submitSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBallSubmit.ts"), "utf8");
  const syncSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/shellBall.windowSync.ts"), "utf8");

  assert.match(appSource, /const SHELL_BALL_CLIPBOARD_PROMPT_WINDOW_MS = 10_000;/);
  assert.match(appSource, /const \[clipboardPrompt, setClipboardPrompt\] = useState<ShellBallClipboardPrompt \| null>\(null\);/);
  assert.match(appSource, /listen<ShellBallClipboardSnapshotPayload>\(shellBallWindowSyncEvents\.clipboardSnapshot/);
  assert.match(appSource, /if \(clipboardPrompt !== null\) \{/);
  assert.match(appSource, /void handleCoordinatorClipboardPrompt\(clipboardPrompt\.text\);/);
  assert.match(coordinatorSource, /const handleClipboardPrompt = useCallback\(async \(text: string\) => \{/);
  assert.match(coordinatorSource, /submitShellBallInput\(\{/);
  assert.match(submitSource, /export async function submitShellBallInput/);
  assert.match(submitSource, /trigger: input\.trigger/);
  assert.match(submitSource, /inputMode: input\.inputMode/);
  assert.match(submitSource, /includeForegroundBrowserPageContext: true/);
  assert.match(syncSource, /clipboardSnapshot: "desktop-shell-ball:clipboard-snapshot"/);
});

test("shell-ball routes active resumable text follow-ups through task steer", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(coordinatorSource, /import \{ respondSecurityDetailed, steerTask \} from "@\/rpc\/methods";/);
  assert.match(coordinatorSource, /const activeShellBallTaskIntentNameRef = useRef<string \| null>\(null\);/);
  assert.match(coordinatorSource, /const activeShellBallTaskStatusRef = useRef<TaskUpdatedNotification\["status"\] \| null>\(null\);/);
  assert.match(coordinatorSource, /function isShellBallActiveTaskSteerable\(/);
  assert.match(coordinatorSource, /shouldRouteShellBallSubmitToActiveSteering\(\{/);
  assert.match(coordinatorSource, /input\.activeTaskStatus === "processing"[\s\S]*input\.activeTaskIntentName === "agent_loop"/);
  assert.match(coordinatorSource, /input\.activeTaskStatus === "waiting_auth"/);
  assert.match(coordinatorSource, /input\.activeTaskStatus === "blocked"/);
  assert.match(coordinatorSource, /input\.files\.length === 0/);
  assert.match(coordinatorSource, /activeTaskIntentName: activeShellBallTaskIntentNameRef\.current/);
  assert.match(coordinatorSource, /activeTaskStatus: activeShellBallTaskStatusRef\.current/);
  assert.match(coordinatorSource, /const result = await steerTask\(\{/);
  assert.match(coordinatorSource, /request_meta: createShellBallRequestMeta\(\)/);
  assert.match(coordinatorSource, /task_id: activeShellBallTaskId/);
  assert.match(coordinatorSource, /message: submittedText/);
});

test("shell-ball falls back to regular submit when active steer status races", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(coordinatorSource, /import \{ JsonRpcClientError \} from "@\/rpc\/client";/);
  assert.match(coordinatorSource, /ERROR_CODES/);
  assert.match(coordinatorSource, /function isTaskStatusInvalidRpcError\(error: unknown\)/);
  assert.match(coordinatorSource, /error instanceof JsonRpcClientError && error\.code === ERROR_CODES\.TASK_STATUS_INVALID/);
  assert.match(coordinatorSource, /if \(isTaskStatusInvalidRpcError\(error\)\) \{/);
  assert.match(coordinatorSource, /const fallbackResult = await submitTextInput\(\{/);
  assert.match(coordinatorSource, /text: submittedText/);
  assert.match(coordinatorSource, /trigger: "hover_text_input"/);
  assert.match(coordinatorSource, /preferred_delivery: "bubble"/);
  assert.match(coordinatorSource, /task_id: fallbackResult\.task\.task_id/);
  assert.match(coordinatorSource, /autoOpenShellBallDeliveryResult\(fallbackResult\.task\.task_id, fallbackResult\.delivery_result\)/);
});

test("shell-ball screenshot command routes through the formal screen task path", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(coordinatorSource, /const SHELL_BALL_SCREENSHOT_COMMAND = "截屏";/);
  assert.match(coordinatorSource, /shouldHandleShellBallScreenshotCommand\(/);
  assert.match(coordinatorSource, /const submitShellBallScreenShortcut = useCallback\(async \(input: \{/);
  assert.match(coordinatorSource, /promptText: SHELL_BALL_SCREENSHOT_PROMPT_TEXT/);
  assert.match(coordinatorSource, /summary: SHELL_BALL_SCREENSHOT_SUMMARY/);
  assert.match(coordinatorSource, /last_action: "review_screen"/);
  assert.match(coordinatorSource, /void autoOpenShellBallDeliveryResult\(result\.task\.task_id, result\.delivery_result\);/);
  assert.doesNotMatch(coordinatorSource, /captureDesktopScreenshot/);
  assert.doesNotMatch(coordinatorSource, /Screenshot saved to/);
});

test("shell-ball window command routes through the formal screen task path", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(coordinatorSource, /const SHELL_BALL_WINDOW_COMMAND = "窗口";/);
  assert.match(coordinatorSource, /shouldHandleShellBallWindowCommand\(/);
  assert.match(coordinatorSource, /promptText: SHELL_BALL_WINDOW_PROMPT_TEXT/);
  assert.match(coordinatorSource, /summary: SHELL_BALL_WINDOW_SUMMARY/);
  assert.match(coordinatorSource, /last_action: "review_window"/);
  assert.match(coordinatorSource, /void autoOpenShellBallDeliveryResult\(result\.task\.task_id, result\.delivery_result\);/);
  assert.doesNotMatch(coordinatorSource, /getActiveWindowContext/);
  assert.doesNotMatch(coordinatorSource, /createShellBallWindowContextReply/);
});

test("shell-ball coordinator subscribes to formal task, approval, and runtime updates", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(coordinatorSource, /subscribeTaskUpdated\(\(payload\) => \{/);
  assert.match(coordinatorSource, /subscribeApprovalPending\(\(payload\) => \{/);
  assert.match(coordinatorSource, /subscribeAllTaskRuntime\(\(payload\) => \{/);
  assert.match(coordinatorSource, /const queuedRuntimeNotificationsRef = useRef\(new Map<string, QueuedRuntimeNotification\[]>\(\)\);/);
  assert.match(coordinatorSource, /queuedRuntimeNotifications\.forEach\(\(notification\) => \{\s*appendRuntimeObservationBubble\(notification\.taskId, notification\.payload\);/);
  assert.match(coordinatorSource, /syncShellBallVisualStateFromTaskStatus\(payload\.status\)/);
  assert.match(coordinatorSource, /activeShellBallTaskStatusRef\.current = "waiting_auth";\s*syncShellBallVisualStateFromTaskStatus\("waiting_auth"\);/);
  assert.match(coordinatorSource, /approvalRequest: payload\.approval_request/);
});

test("desktop tauri setup enables mouse activity tracking for dwell context", () => {
  const mainSource = readFileSync(resolve(desktopRoot, "src-tauri/src/main.rs"), "utf8");
  const activitySource = readFileSync(resolve(desktopRoot, "src-tauri/src/activity/windows.rs"), "utf8");

  assert.match(mainSource, /activity::install_mouse_activity_listener\(\)/);
  assert.match(activitySource, /SetWindowsHookExW\(WH_MOUSE_LL, Some\(mouse_activity_hook\), None, 0\)/);
  assert.doesNotMatch(activitySource, /println!\("mouse activity at /);
});

test("shell-ball file drops queue pending attachments instead of starting a task immediately", () => {
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(coordinatorSource, /const handleDroppedFiles = useCallback\(async \(paths: string\[\]\) => \{/);
  assert.match(coordinatorSource, /handlersRef\.current\.onAppendPendingFiles\(normalizedPaths\);/);
  assert.doesNotMatch(coordinatorSource, /emitShellBallInputRequestFocus/);
  assert.match(appSource, /const droppedPaths = event\.payload\.paths;\s*void \(async \(\) => \{\s*try \{\s*await dragDropHandlersRef\.current\.handleDroppedFiles\(droppedPaths\);\s*inputFocusRequestRef\.current\(\);/);
  assert.match(appSource, /console\.warn\("shell-ball file drop handling failed", error\);/);
  assert.doesNotMatch(coordinatorSource, /issue #187/);
  assert.match(interactionSource, /function handleDroppedFiles\(paths: string\[\]\) \{/);
  assert.match(interactionSource, /setPendingFilesState\(\(currentPaths\) => mergeShellBallPendingFiles\(currentPaths, normalizedPaths\)\);/);
  assert.match(interactionSource, /controllerRef\.current\?\.forceState\("hover_input", \{/);
});

test("shell-ball task entry sources keep rpc failures visible and forward attachment descriptions", () => {
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");
  const agentInputServiceSource = readFileSync(resolve(desktopRoot, "src/services/agentInputService.ts"), "utf8");
  const taskServiceSource = readFileSync(resolve(desktopRoot, "src/services/taskService.ts"), "utf8");

  assert.match(interactionSource, /return startTaskFromFiles\(normalizedFiles, \{[\s\S]*source: "floating_ball",[\s\S]*\}, input\.text\);/);
  assert.match(taskServiceSource, /\.\.\.\(normalizedText === undefined \? \{\} : \{ text: normalizedText \}\)/);
  assert.match(taskServiceSource, /const taskResult = await submitTextInput\(/);
  assert.doesNotMatch(taskServiceSource, /createMockTaskStartResult/);
  assert.doesNotMatch(taskServiceSource, /logRpcMockFallback/);
  assert.doesNotMatch(taskServiceSource, /isRpcChannelUnavailable/);
  assert.doesNotMatch(agentInputServiceSource, /createMockAgentInputSubmitResult/);
  assert.doesNotMatch(agentInputServiceSource, /logRpcMockFallback/);
  assert.doesNotMatch(agentInputServiceSource, /isRpcChannelUnavailable/);
  assert.match(coordinatorSource, /createShellBallTaskErrorBubbleItem/);
  assert.doesNotMatch(coordinatorSource, /createMockShellBallConfirmResult/);
  assert.doesNotMatch(coordinatorSource, /logRpcMockFallback/);
});

test("shell-ball selected-text prompt only surfaces in resting states", () => {
  assert.equal(
    shouldShowShellBallSelectionIndicator({
      selection: {
        text: "selected text",
        page_context: { title: "Dashboard", url: "local://dashboard", app_name: "dashboard" },
        source: "windows_uia",
        updated_at: "2026-04-16T10:00:00.000Z",
      },
      visualState: "idle",
    }),
    true,
  );
  assert.equal(
    shouldShowShellBallSelectionIndicator({
      selection: {
        text: "selected text",
        page_context: { title: "Dashboard", url: "local://dashboard", app_name: "dashboard" },
        source: "windows_uia",
        updated_at: "2026-04-16T10:00:00.000Z",
      },
      visualState: "processing",
    }),
    false,
  );
});

test("shell-ball selection snapshot equality includes browser attach hints", () => {
  const left = {
    text: "selected text",
    page_context: {
      title: "A",
      url: "native://windows-uia-selection",
      app_name: "notepad",
      browser_kind: "non_browser" as const,
      process_path: "C:/Windows/System32/notepad.exe",
      process_id: 8844,
    },
    source: "windows_uia" as const,
    updated_at: "1",
  };
  const right = {
    text: "selected text",
    page_context: {
      title: "A",
      url: "native://windows-uia-selection",
      app_name: "notepad",
      browser_kind: "non_browser" as const,
      process_path: "C:/Windows/System32/notepad.exe",
      process_id: 8844,
    },
    source: "windows_uia" as const,
    updated_at: "2",
  };

  assert.equal(areShellBallSelectionSnapshotsEqual(left, right), true);
  assert.equal(
    areShellBallSelectionSnapshotsEqual(left, {
      ...right,
      page_context: {
        ...right.page_context,
        browser_kind: "chrome" as const,
        process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
        process_id: 4412,
      },
    }),
    false,
  );
});

test("shell-ball app routes real selection snapshots into the formal selected-text task flow", () => {
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");
  const providersSource = readFileSync(resolve(desktopRoot, "src/features/shared/AppProviders.tsx"), "utf8");
  const selectionProviderSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/selection/selection.provider.tsx"), "utf8");
  const selectionHostSource = readFileSync(resolve(desktopRoot, "src-tauri/src/selection/windows.rs"), "utf8");

  assert.match(appSource, /listen<ShellBallSelectionSnapshotPayload>\(shellBallWindowSyncEvents\.selectionSnapshot/);
  assert.match(appSource, /const handleMascotPrimaryAction = useCallback\(\(\) => \{/);
  assert.match(appSource, /void handleCoordinatorSelectedTextPrompt\(selectionPrompt\);/);
  assert.match(coordinatorSource, /const handleSelectedTextPrompt = useCallback\(async \(selection: ShellBallSelectionSnapshot \| string\) => \{/);
  assert.match(coordinatorSource, /createShellBallSelectedTextPreview\(text\)/);
  assert.match(coordinatorSource, /startTaskFromSelectedText\(normalizedText, \{/);
  assert.match(coordinatorSource, /pageContext,/);
  assert.doesNotMatch(coordinatorSource, /sessionId: handlersRef\.current\.getCurrentConversationSessionId\?\.\(\),/);
  assert.match(providersSource, /<ShellBallSelectionProvider \/>/);
  assert.match(selectionProviderSource, /shellBallWindowSyncEvents\.selectionSnapshot/);
  assert.match(selectionHostSource, /w_param\.0 as u32 == WM_LBUTTONUP/);
  assert.doesNotMatch(selectionHostSource, /WM_RBUTTONUP/);
  assert.match(selectionHostSource, /vk_code == VK_BACK\.0 as u32 \|\| vk_code == VK_DELETE\.0 as u32/);
  assert.doesNotMatch(selectionProviderSource, /readShellBallSelectionSnapshot/);
  assert.doesNotMatch(selectionProviderSource, /useInterval\(/);
  assert.equal(
    areShellBallSelectionSnapshotsEqual(
      {
        text: "selected text",
        page_context: { title: "A", url: "native://windows-uia-selection", app_name: "notepad" },
        source: "windows_uia",
        updated_at: "1",
      },
      {
        text: "selected text",
        page_context: { title: "A", url: "native://windows-uia-selection", app_name: "notepad" },
        source: "windows_uia",
        updated_at: "2",
      },
    ),
    true,
  );
});

test("shell-ball input bar no longer relies on resize drag pointer capture", () => {
  const inputBarSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallInputBar.tsx"), "utf8");

  assert.doesNotMatch(inputBarSource, /onResizeStateChange\(true\);/);
  assert.doesNotMatch(inputBarSource, /handle\.setPointerCapture\(pointerId\);/);
  assert.doesNotMatch(inputBarSource, /handle\.addEventListener\("lostpointercapture", cleanup\);/);
  assert.doesNotMatch(inputBarSource, /window\.addEventListener\("blur", cleanup\);/);
  assert.doesNotMatch(inputBarSource, /onResizeStateChange\(false\);/);
});

test("shell-ball input bar restores textarea focus after attach and send actions", () => {
  const inputBarSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallInputBar.tsx"), "utf8");

  assert.match(inputBarSource, /function restoreInputFocus\(\) \{/);
  assert.match(inputBarSource, /field\.focus\(\);/);
  assert.match(inputBarSource, /field\.setSelectionRange\(selectionIndex, selectionIndex\);/);
  assert.match(inputBarSource, /onMouseDown=\{\(event\) => \{\s*event\.preventDefault\(\);/);
  assert.match(inputBarSource, /onAttachFile\(\);\s*restoreInputFocus\(\);/);
  assert.match(inputBarSource, /onSubmit\(\);\s*restoreInputFocus\(\);/);
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

test("shell-ball app keeps the reusable surface as the production structure", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp));

  assert.match(markup, /Shell-ball floating surface/);
  assert.match(markup, /shell-ball-surface__body/);
});

test("shell-ball inline input preserves readonly snapshots and only upgrades hidden idle input", () => {
  const appSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/ShellBallApp.tsx"), "utf8");

  assert.equal(
    resolveShellBallInlineInputMode({
      shouldRenderInlineInput: true,
      snapshotInputBarMode: "readonly",
    }),
    "readonly",
  );
  assert.equal(
    resolveShellBallInlineInputMode({
      shouldRenderInlineInput: true,
      snapshotInputBarMode: "interactive",
    }),
    "interactive",
  );
  assert.equal(
    resolveShellBallInlineInputMode({
      shouldRenderInlineInput: true,
      snapshotInputBarMode: "hidden",
    }),
    "interactive",
  );
  assert.equal(
    resolveShellBallInlineInputMode({
      shouldRenderInlineInput: false,
      snapshotInputBarMode: "readonly",
    }),
    "hidden",
  );
  assert.match(appSource, /const shouldRenderInlineInput = snapshot\.visibility\.input;/);
});

test("shell-ball input bar mode stays aligned with visual states", () => {
  assert.equal(getShellBallInputBarMode("idle"), "interactive");
  assert.equal(getShellBallInputBarMode("hover_input"), "interactive");
  assert.equal(getShellBallInputBarMode("confirming_intent"), "interactive");
  assert.equal(getShellBallInputBarMode("waiting_auth"), "readonly");
  assert.equal(getShellBallInputBarMode("processing"), "readonly");
  assert.equal(getShellBallInputBarMode("voice_listening"), "hidden");
  assert.equal(getShellBallInputBarMode("voice_locked"), "hidden");
});

test("intent confirm cancel action is wired to formal task cancellation", () => {
  const bubbleMessageSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/components/ShellBallBubbleMessage.tsx"), "utf8");
  const coordinatorSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallCoordinator.ts"), "utf8");

  assert.match(bubbleMessageSource, />取消任务</);
  assert.match(bubbleMessageSource, /data-bubble-action="cancel_task"/);
  assert.match(coordinatorSource, /const decisionText = payload\.decision === "confirm" \? "确认" : "取消任务";/);
  assert.match(
    coordinatorSource,
    /payload\.decision === "confirm"\s*\?\s*await confirmTask\([\s\S]*:\s*await controlTask\(\{[\s\S]*action:\s*"cancel"/,
  );
});

test("shell-ball text submit clears drafts before RPC completion and fully restores failed optimistic submits", () => {
  const interactionSource = readFileSync(resolve(desktopRoot, "src/features/shell-ball/useShellBallInteraction.ts"), "utf8");

  assert.match(interactionSource, /function prepareTextSubmitDraft\(\): ShellBallPreparedTextSubmitDraft \| null \{/);
  assert.match(
    interactionSource,
    /const submittedDraftRevision = draftRevisionRef\.current;\s*dispatch\("submit_text"\);\s*setInputValueState\(reset\.nextInputValue\);\s*setPendingFilesState\(reset\.nextPendingFiles\);/,
  );
  assert.match(interactionSource, /function restorePreparedTextSubmitDraft\(preparedDraft: ShellBallPreparedTextSubmitDraft\) \{/);
  assert.match(interactionSource, /if \(shouldRestoreShellBallSubmitFailureDraft\(\{/);
  assert.match(interactionSource, /setInputValueState\(preparedDraft\.currentInputValue\);\s*setPendingFilesState\(preparedDraft\.currentPendingFiles\);/);
  assert.match(
    interactionSource,
    /function restorePreparedTextSubmitDraft\(preparedDraft: ShellBallPreparedTextSubmitDraft\) \{[\s\S]*controllerRef\.current\?\.forceState\("hover_input", \{[\s\S]*regionActive: regionActiveRef\.current,[\s\S]*hoverRetained: true,[\s\S]*\}\);[\s\S]*syncVisualState\(\);[\s\S]*\}/,
  );
  assert.match(interactionSource, /const preparedDraft = prepareTextSubmitDraft\(\);\s*if \(preparedDraft === null\) \{\s*return null;\s*\}/);
  assert.match(interactionSource, /restorePreparedTextSubmitDraft\(preparedDraft\);/);
});

test("shell-ball interaction timing constants stay frozen", () => {
  assert.equal(SHELL_BALL_HOVER_INTENT_MS, 360);
  assert.equal(SHELL_BALL_LEAVE_GRACE_MS, 360);
  assert.equal(SHELL_BALL_LONG_PRESS_MS, 1000);
  assert.equal(SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX, 12);
  assert.equal(SHELL_BALL_LOCKED_CANCEL_HOLD_MS, 200);
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

test("shell-ball security respond stub exposes the restore union branch for restore approvals", async () => {
  const result = await respondSecurity({
    approval_id: "approval_restore_stub",
    task_id: "task_restore_stub",
  });

  assert.equal("authorization_record" in result, false);
  assert.equal("recovery_point" in result, true);

  if ("recovery_point" in result) {
    assert.equal(result.applied, true);
    assert.equal(result.task.task_id, "task_restore_stub");
    assert.equal(result.task.status, "completed");
    assert.equal(result.recovery_point.task_id, "task_restore_stub");
    assert.equal(result.audit_record?.task_id, "task_restore_stub");
    assert.equal(result.audit_record === null, false);
    assert.match(result.bubble_message?.text ?? "", /Restored the workspace state/);
  }
});
