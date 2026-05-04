import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const NodeModule = require("node:module");

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const desktopRoot = resolve(__dirname, "..", "..");
const compiledRoot = resolve(desktopRoot, ".cache", "task-context-tests");
const compiledModulePaths = {
  conversationSessionService: resolve(compiledRoot, "services", "conversationSessionService.js"),
  pageContext: resolve(compiledRoot, "services", "pageContext.js"),
  taskService: resolve(compiledRoot, "services", "taskService.js"),
};

function resetCompiledModuleCache() {
  for (const modulePath of Object.values(compiledModulePaths)) {
    delete require.cache[modulePath];
  }
}

function withCompiledModuleRuntime(modulePath, mocks, callback) {
  resetCompiledModuleCache();

  const originalLoad = NodeModule._load;
  NodeModule._load = function loadCompiledModule(request, parent, isMain) {
    if (request in mocks) {
      return mocks[request];
    }

    return originalLoad(request, parent, isMain);
  };

  const restoreRuntime = () => {
    NodeModule._load = originalLoad;
  };

  try {
    const moduleExports = require(modulePath);
    const result = callback(moduleExports);

    if (result && typeof result.then === "function") {
      return result.finally(restoreRuntime);
    }

    restoreRuntime();
    return result;
  } catch (error) {
    restoreRuntime();
    throw error;
  }
}

function createStartTaskMocks() {
  const startTaskCalls = [];
  const mocks = {
    "@/rpc/methods": {
      startTask(params) {
        startTaskCalls.push(params);
        return Promise.resolve({
          bubble_message: null,
          delivery_result: null,
          task: {
            task_id: "task_task_context_runtime",
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
        });
      },
    },
    "@/stores/taskStore": {
      useTaskStore: {
        getState() {
          return { tasks: [] };
        },
      },
    },
    "@/platform/desktopWindowContext": {
      getActiveWindowContext() {
        return Promise.resolve(null);
      },
    },
    "./agentInputService": {
      submitTextInput() {
        return Promise.reject(new Error("transport is not wired"));
      },
    },
  };

  return {
    mocks,
    startTaskCalls,
  };
}

test("conversation session cache stores stable page anchors without page-content hints", () => {
  withCompiledModuleRuntime(compiledModulePaths.conversationSessionService, {}, (conversationSessionService) => {
    conversationSessionService.rememberConversationSessionFromTask({
      task_id: "task_shell_ball_anchor",
      session_id: "sess_shell_ball_anchor",
    });

    conversationSessionService.rememberConversationPageContextFromTask(
      { session_id: "sess_shell_ball_anchor" },
      {
        app_name: "Chrome",
        browser_kind: "chrome",
        process_id: 4412,
        process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
        title: "Build Dashboard",
        url: "https://example.com/build",
        visible_text: "Current build summary",
        hover_target: "Publish button",
        window_title: "Build Dashboard",
      },
    );

    assert.deepEqual(conversationSessionService.getConversationPageContextForSession("sess_shell_ball_anchor"), {
      app_name: "Chrome",
      title: "Build Dashboard",
      url: "https://example.com/build",
      window_title: "Build Dashboard",
    });
  });
});

test("task entry falls back to shell-ball context for hover-only remembered anchors", async () => {
  const { mocks, startTaskCalls } = createStartTaskMocks();
  mocks["@/platform/desktopWindowContext"] = {
    getActiveWindowContext() {
      return Promise.resolve({
        app_name: "Chrome",
        browser_kind: "chrome",
        process_id: 4412,
        process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
        title: "Build Dashboard",
        url: "https://example.com/build?ticket=secret#fragment",
        hover_target: "Save",
      });
    },
  };

  await withCompiledModuleRuntime(compiledModulePaths.taskService, mocks, async (taskService) => {
    const conversationSessionService = require(compiledModulePaths.conversationSessionService);
    conversationSessionService.rememberConversationSessionFromTask({
      task_id: "task_shell_ball_hover_anchor",
      session_id: "sess_shell_ball_hover_anchor",
    });
    conversationSessionService.rememberConversationPageContextFromTask(
      { session_id: "sess_shell_ball_hover_anchor" },
      {
        app_name: "Chrome",
        title: "Settings",
        hover_target: "Save",
        window_title: "Settings",
      },
    );

    await taskService.startTaskFromFiles(["C:\\workspace\\notes.md"]);

    assert.deepEqual(startTaskCalls[0]?.input, {
      type: "file",
      files: ["C:\\workspace\\notes.md"],
      page_context: {
        app_name: "desktop",
        title: "Quick Intake",
        url: "local://shell-ball",
      },
    });
  });
});

test("task entry strips stale page-content hints from legacy remembered anchors when foreground refresh is unavailable", async () => {
  const { mocks, startTaskCalls } = createStartTaskMocks();
  mocks["./conversationSessionService"] = {
    getCurrentConversationSessionId() {
      return "sess_shell_ball_same_url_anchor";
    },
    getConversationPageContextForSession(sessionId) {
      return sessionId === "sess_shell_ball_same_url_anchor"
        ? {
            app_name: "Chrome",
            title: "Build Dashboard",
            url: "https://example.com/build",
            visible_text: "Old validation warning",
            hover_target: "Old publish button",
            window_title: "Build Dashboard",
          }
        : undefined;
    },
    rememberConversationSessionFromTask() {},
    rememberConversationPageContextFromTask() {},
  };
  mocks["@/platform/desktopWindowContext"] = {
    getActiveWindowContext() {
      throw new Error("desktop host unavailable");
    },
  };

  await withCompiledModuleRuntime(compiledModulePaths.taskService, mocks, async (taskService) => {
    await taskService.startTaskFromFiles(["C:\\workspace\\notes.md"]);

    assert.deepEqual(startTaskCalls[0]?.input, {
      type: "file",
      files: ["C:\\workspace\\notes.md"],
      page_context: {
        app_name: "Chrome",
        title: "Build Dashboard",
        url: "https://example.com/build",
        window_title: "Build Dashboard",
      },
    });
  });
});

test("task entry rehydrates fresh attach hints for same-url remembered anchors without replaying stale page-content hints", async () => {
  const { mocks, startTaskCalls } = createStartTaskMocks();
  mocks["./conversationSessionService"] = {
    getCurrentConversationSessionId() {
      return "sess_shell_ball_same_url_anchor";
    },
    getConversationPageContextForSession(sessionId) {
      return sessionId === "sess_shell_ball_same_url_anchor"
        ? {
            app_name: "Chrome",
            title: "Build Dashboard",
            url: "https://example.com/build",
            visible_text: "Old validation warning",
            hover_target: "Old publish button",
          }
        : undefined;
    },
    rememberConversationSessionFromTask() {},
    rememberConversationPageContextFromTask() {},
  };
  mocks["@/platform/desktopWindowContext"] = {
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
  };

  await withCompiledModuleRuntime(compiledModulePaths.taskService, mocks, async (taskService) => {
    await taskService.startTaskFromFiles(["C:\\workspace\\notes.md"]);

    assert.deepEqual(startTaskCalls[0]?.input, {
      type: "file",
      files: ["C:\\workspace\\notes.md"],
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
  });
});
