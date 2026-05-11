import * as assert from "node:assert/strict";
import { test } from "node:test";

import { loadStoredValue } from "./storage";

test("loadStoredValue returns null when localStorage contains invalid JSON", () => {
  const originalWindow = globalThis.window;
  const warnCalls: unknown[][] = [];
  const originalWarn = console.warn;

  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage: {
        getItem: (key: string) => (key === "invalid-json" ? "{not:json}" : null),
      },
    },
  });

  console.warn = (...args: unknown[]) => {
    warnCalls.push(args);
  };

  try {
    assert.doesNotThrow(() => {
      const value = loadStoredValue("invalid-json");
      assert.equal(value, null);
    });
    assert.equal(warnCalls.length, 1);
    assert.equal(warnCalls[0]?.[0], "[storage] Failed to parse localStorage value");
    assert.equal((warnCalls[0]?.[1] as { key?: string }).key, "invalid-json");
    assert.ok((warnCalls[0]?.[1] as { error?: unknown }).error instanceof Error);
  } finally {
    console.warn = originalWarn;
    if (originalWindow === undefined) {
      Reflect.deleteProperty(globalThis, "window");
    } else {
      Object.defineProperty(globalThis, "window", {
        configurable: true,
        value: originalWindow,
      });
    }
  }
});
