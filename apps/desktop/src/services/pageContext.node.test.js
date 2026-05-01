import assert from "node:assert/strict";
import test from "node:test";

import {
  compactPageContext,
  mapDesktopWindowSnapshotToPageContext,
  resolveTaskPageContext,
  sanitizePageContextUrl,
} from "./pageContext.ts";

test("sanitizePageContextUrl removes credentials, query, and fragments", () => {
  assert.equal(
    sanitizePageContextUrl("https://user:secret@example.com/build?ticket=1#summary"),
    "https://example.com/build",
  );
});

test("sanitizePageContextUrl falls back to a stable path for malformed inputs", () => {
  assert.equal(sanitizePageContextUrl("example.com/build?ticket=1#summary"), "example.com/build");
  assert.equal(sanitizePageContextUrl("   "), undefined);
});

test("mapDesktopWindowSnapshotToPageContext keeps Chromium attach hints", () => {
  assert.deepEqual(
    mapDesktopWindowSnapshotToPageContext({
      app_name: "Chrome",
      browser_kind: "chrome",
      process_id: 4412,
      process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
      title: "Build Dashboard",
      url: "https://example.com/build?ticket=secret#fragment",
    }),
    {
      app_name: "Chrome",
      browser_kind: "chrome",
      process_id: 4412,
      process_path: "C:/Program Files/Google/Chrome/Application/chrome.exe",
      title: "Build Dashboard",
      url: "https://example.com/build",
      window_title: "Build Dashboard",
    },
  );
});

test("mapDesktopWindowSnapshotToPageContext returns undefined when no snapshot exists", () => {
  assert.equal(mapDesktopWindowSnapshotToPageContext(null), undefined);
});

test("compactPageContext drops empty fields and invalid process ids", () => {
  assert.deepEqual(
    compactPageContext({
      app_name: " Chrome ",
      browser_kind: "chrome",
      process_id: 0,
      process_path: "  ",
      title: " Build Dashboard ",
      url: " https://example.com/build ",
      visible_text: "   ",
      hover_target: "Open docs",
      window_title: " Build Dashboard ",
    }),
    {
      app_name: "Chrome",
      browser_kind: "chrome",
      title: "Build Dashboard",
      url: "https://example.com/build",
      hover_target: "Open docs",
      window_title: "Build Dashboard",
    },
  );
});

test("compactPageContext returns undefined when no formal page hints remain", () => {
  assert.equal(compactPageContext(undefined), undefined);
  assert.equal(compactPageContext({}), undefined);
});

test("resolveTaskPageContext falls back only when no page hints remain", () => {
  const fallback = {
    app_name: "desktop",
    title: "Quick Intake",
    url: "local://shell-ball",
  };

  assert.deepEqual(resolveTaskPageContext(undefined, fallback), fallback);
  assert.deepEqual(
    resolveTaskPageContext(
      {
        browser_kind: "non_browser",
        process_id: 8844,
      },
      fallback,
    ),
    {
      browser_kind: "non_browser",
      process_id: 8844,
    },
  );
});
