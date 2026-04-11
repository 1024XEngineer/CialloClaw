import { useEffect, useMemo, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import {
  SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
  closeShellBallPinnedBubbleWindow,
  emitToShellBallWindowLabel,
  getShellBallPinnedBubbleIdFromLabel,
  getShellBallPinnedBubbleWindowAnchor,
  getShellBallPinnedBubbleWindowLabel,
  openShellBallPinnedBubbleWindow,
  shellBallWindowLabels,
} from "../../platform/shellBallWindowController";
import { cloneShellBallBubbleItems, type ShellBallBubbleItem } from "./shellBall.bubble";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallInputBarMode, ShellBallVisualState } from "./shellBall.types";
import {
  createDefaultShellBallWindowSnapshot,
  createShellBallWindowSnapshot,
  type ShellBallBubbleAction,
  type ShellBallBubbleActionPayload,
  type ShellBallBubbleInteractionPayload,
  shellBallWindowSyncEvents,
  type ShellBallHelperReadyPayload,
  type ShellBallHelperWindowRole,
  type ShellBallInputDraftPayload,
  type ShellBallInputFocusPayload,
  type ShellBallInputHoverPayload,
  type ShellBallPinnedWindowDetachedPayload,
  type ShellBallPinnedWindowReadyPayload,
  type ShellBallPrimaryAction,
  type ShellBallPrimaryActionPayload,
} from "./shellBall.windowSync";
import { getShellBallBubbleAnchor } from "./useShellBallWindowMetrics";

type ShellBallCoordinatorInput = {
  visualState: ShellBallVisualState;
  inputValue: string;
  voicePreview: ShellBallVoicePreview;
  setInputValue: (value: string) => void;
  onRegionEnter: () => void;
  onRegionLeave: () => void;
  onInputFocusChange: (focused: boolean) => void;
  onSubmitText: () => void;
  onAttachFile: () => void;
  onPrimaryClick: () => void;
};

type ShellBallHelperSnapshotInput = {
  role: ShellBallHelperWindowRole;
  windowLabel?: string;
};

const SHELL_BALL_LOCAL_MOCK_BUBBLE_BASE_TIME_MS = Date.parse("2026-04-12T09:00:00.000Z");
const SHELL_BALL_LOCAL_MOCK_REPLY_DELAY_MS = 320;
const SHELL_BALL_LOCAL_BUBBLE_DISSIPATION_DELAY_MS = 4800;

const SHELL_BALL_LOCAL_BUBBLE_ITEMS: ShellBallBubbleItem[] = [
  {
    bubble: {
      bubble_id: "shell-ball-local-agent-1",
      task_id: "",
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
  {
    bubble: {
      bubble_id: "shell-ball-local-user-1",
      task_id: "",
      type: "result",
      text: "Open the dashboard.",
      pinned: false,
      hidden: false,
      created_at: "2026-04-11T10:05:00.000Z",
    },
    role: "user",
    desktop: {
      lifecycleState: "visible",
      freshnessHint: "fresh",
      motionHint: "settle",
    },
  },
];

export function compareShellBallBubbleItemsByTimestamp(left: ShellBallBubbleItem, right: ShellBallBubbleItem) {
  const createdAtOrder = left.bubble.created_at.localeCompare(right.bubble.created_at);

  if (createdAtOrder !== 0) {
    return createdAtOrder;
  }

  return left.bubble.bubble_id.localeCompare(right.bubble.bubble_id);
}

export function sortShellBallBubbleItemsByTimestamp(items: ShellBallBubbleItem[]) {
  return [...items].sort(compareShellBallBubbleItemsByTimestamp);
}

export function applyShellBallBubbleAction(
  items: ShellBallBubbleItem[],
  payload: Pick<ShellBallBubbleActionPayload, "action" | "bubbleId">,
): ShellBallBubbleItem[] {
  if (payload.action === "delete") {
    return sortShellBallBubbleItemsByTimestamp(items.filter((item) => item.bubble.bubble_id !== payload.bubbleId));
  }

  return sortShellBallBubbleItemsByTimestamp(
    items.map((item) => {
      if (item.bubble.bubble_id !== payload.bubbleId) {
        return item;
      }

      return {
        ...item,
        bubble: {
          ...item.bubble,
          pinned: payload.action === "pin",
        },
      };
    }),
  );
}

function createShellBallLocalMockBubbleItem(input: {
  sequence: number;
  role: ShellBallBubbleItem["role"];
  text: string;
  type: ShellBallBubbleItem["bubble"]["type"];
}): ShellBallBubbleItem {
  return {
    bubble: {
      bubble_id: `shell-ball-local-mock-${input.sequence}`,
      task_id: "",
      type: input.type,
      text: input.text,
      pinned: false,
      hidden: false,
      created_at: new Date(SHELL_BALL_LOCAL_MOCK_BUBBLE_BASE_TIME_MS + input.sequence * 1000).toISOString(),
    },
    role: input.role,
    desktop: {
      lifecycleState: "visible",
      freshnessHint: "fresh",
      motionHint: "settle",
    },
  };
}

function createShellBallLocalMockReplyText(submittedText: string) {
  return `Mock reply: I captured '${submittedText}'.`;
}

function shouldShellBallBubbleDissipate(item: ShellBallBubbleItem) {
  return item.bubble.hidden === false && item.bubble.pinned === false && item.desktop.lifecycleState === "visible";
}

function applyShellBallBubbleLifecycle(
  items: ShellBallBubbleItem[],
  input: {
    bubbleId: string;
    lifecycleState: ShellBallBubbleItem["desktop"]["lifecycleState"];
  },
) {
  return items.map((item) => {
    if (item.bubble.bubble_id !== input.bubbleId) {
      return item;
    }

    return {
      ...item,
      desktop: {
        ...item.desktop,
        freshnessHint: input.lifecycleState === "fading" ? "stale" : item.desktop.freshnessHint,
        lifecycleState: input.lifecycleState,
      },
    };
  });
}

export function useShellBallCoordinator(input: ShellBallCoordinatorInput) {
  const [bubbleItems, setBubbleItems] = useState(() => sortShellBallBubbleItemsByTimestamp(cloneShellBallBubbleItems(SHELL_BALL_LOCAL_BUBBLE_ITEMS)));
  const snapshot = useMemo(
    () =>
      createShellBallWindowSnapshot({
        visualState: input.visualState,
        inputValue: input.inputValue,
        voicePreview: input.voicePreview,
        bubbleItems,
      }),
    [bubbleItems, input.inputValue, input.visualState, input.voicePreview],
  );
  const snapshotRef = useRef(snapshot);
  const bubbleItemsRef = useRef(bubbleItems);
  const inputValueRef = useRef(input.inputValue);
  const localMockBubbleSequenceRef = useRef(0);
  const bubbleDissipationTimeoutsRef = useRef(new Map<string, unknown>());
  const bubbleInteractionActiveRef = useRef(false);
  const detachedPinnedBubbleIdsRef = useRef(new Set<string>());
  const handlersRef = useRef({
    setInputValue: input.setInputValue,
    onRegionEnter: input.onRegionEnter,
    onRegionLeave: input.onRegionLeave,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onAttachFile: input.onAttachFile,
    onPrimaryClick: input.onPrimaryClick,
  });

  snapshotRef.current = snapshot;
  bubbleItemsRef.current = bubbleItems;
  inputValueRef.current = input.inputValue;
  handlersRef.current = {
    setInputValue: input.setInputValue,
    onRegionEnter: input.onRegionEnter,
    onRegionLeave: input.onRegionLeave,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onAttachFile: input.onAttachFile,
    onPrimaryClick: input.onPrimaryClick,
  };

  function clearShellBallBubbleDissipationTimer(bubbleId: string) {
    const timeoutHandle = bubbleDissipationTimeoutsRef.current.get(bubbleId);
    if (timeoutHandle === undefined) {
      return;
    }

    globalThis.clearTimeout(timeoutHandle as ReturnType<typeof globalThis.setTimeout>);
    bubbleDissipationTimeoutsRef.current.delete(bubbleId);
  }

  function clearShellBallBubbleDissipationTimers() {
    for (const bubbleId of bubbleDissipationTimeoutsRef.current.keys()) {
      clearShellBallBubbleDissipationTimer(bubbleId);
    }
  }

  function syncShellBallBubbleDissipation(items: ShellBallBubbleItem[]) {
    const currentBubbleIds = new Set(items.map((item) => item.bubble.bubble_id));

    for (const bubbleId of bubbleDissipationTimeoutsRef.current.keys()) {
      const matchingItem = items.find((item) => item.bubble.bubble_id === bubbleId);

      if (matchingItem === undefined || !currentBubbleIds.has(bubbleId) || bubbleInteractionActiveRef.current || !shouldShellBallBubbleDissipate(matchingItem)) {
        clearShellBallBubbleDissipationTimer(bubbleId);
      }
    }

    if (bubbleInteractionActiveRef.current) {
      return;
    }

    for (const item of items) {
      if (!shouldShellBallBubbleDissipate(item) || bubbleDissipationTimeoutsRef.current.has(item.bubble.bubble_id)) {
        continue;
      }

      const bubbleId = item.bubble.bubble_id;
      const timeoutHandle = globalThis.setTimeout(() => {
        bubbleDissipationTimeoutsRef.current.delete(bubbleId);
        updateShellBallBubbleItems((currentItems) => applyShellBallBubbleLifecycle(currentItems, { bubbleId, lifecycleState: "fading" }));
      }, SHELL_BALL_LOCAL_BUBBLE_DISSIPATION_DELAY_MS);

      bubbleDissipationTimeoutsRef.current.set(bubbleId, timeoutHandle);
    }
  }

  function updateShellBallBubbleItems(updater: (currentItems: ShellBallBubbleItem[]) => ShellBallBubbleItem[]) {
    let nextItems = bubbleItemsRef.current;

    setBubbleItems((currentItems) => {
      nextItems = updater(currentItems);
      return nextItems;
    });

    bubbleItemsRef.current = nextItems;
    syncShellBallBubbleDissipation(nextItems);

    return nextItems;
  }

  function appendShellBallLocalMockBubble(item: Omit<Parameters<typeof createShellBallLocalMockBubbleItem>[0], "sequence">) {
    localMockBubbleSequenceRef.current += 1;

    updateShellBallBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallLocalMockBubbleItem({
          ...item,
          sequence: localMockBubbleSequenceRef.current,
        }),
      ]),
    );
  }

  function handleSubmitBubbleMock() {
    const submittedText = inputValueRef.current.trim();

    if (submittedText === "") {
      return;
    }

    appendShellBallLocalMockBubble({
      role: "user",
      text: submittedText,
      type: "result",
    });
    handlersRef.current.setInputValue("");
    handlersRef.current.onSubmitText();

    globalThis.setTimeout(() => {
      appendShellBallLocalMockBubble({
        role: "agent",
        text: createShellBallLocalMockReplyText(submittedText),
        type: "result",
      });
    }, SHELL_BALL_LOCAL_MOCK_REPLY_DELAY_MS);
  }

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    async function emitSnapshotToLabel(label: string) {
      await emitToShellBallWindowLabel(label, shellBallWindowSyncEvents.snapshot, snapshotRef.current);
    }

    const pinnedBubbleLabels = snapshotRef.current.bubbleItems
      .filter((item) => item.bubble.pinned)
      .map((item) => getShellBallPinnedBubbleWindowLabel(item.bubble.bubble_id));

    void Promise.all([
      emitSnapshotToLabel(shellBallWindowLabels.bubble),
      emitSnapshotToLabel(shellBallWindowLabels.input),
      ...pinnedBubbleLabels.map((label) => emitSnapshotToLabel(label)),
    ]);
  }, [snapshot]);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let disposed = false;
    let cleanupFns: Array<() => void> = [];

    async function emitSnapshotTo(role: Exclude<ShellBallHelperWindowRole, "pinned">) {
      await emitToShellBallWindowLabel(shellBallWindowLabels[role], shellBallWindowSyncEvents.snapshot, snapshotRef.current);
    }

    async function syncPinnedBubbleWindowAnchor(bubbleId: string, items = bubbleItemsRef.current) {
      if (detachedPinnedBubbleIdsRef.current.has(bubbleId)) {
        return;
      }

      const bubbleItem = items.find((item) => item.bubble.bubble_id === bubbleId && item.bubble.pinned);

      if (bubbleItem === undefined) {
        return;
      }

      const outerPosition = await currentWindow.outerPosition();
      const outerSize = await currentWindow.outerSize();
      const scaleFactor = await currentWindow.scaleFactor();
      const logicalPosition = outerPosition.toLogical(scaleFactor);
      const logicalSize = outerSize.toLogical(scaleFactor);
      const bubbleAnchor = getShellBallBubbleAnchor({
        ballFrame: {
          x: logicalPosition.x,
          y: logicalPosition.y,
          width: logicalSize.width,
          height: logicalSize.height,
        },
        helperFrame: SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
      });

      await openShellBallPinnedBubbleWindow({
        bubbleId,
        position: getShellBallPinnedBubbleWindowAnchor({ bubbleAnchor }),
        size: SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
      });
    }

    async function syncAnchoredPinnedBubbleWindows() {
      await Promise.all(
        bubbleItemsRef.current
          .filter((item) => item.bubble.pinned)
          .map((item) => syncPinnedBubbleWindowAnchor(item.bubble.bubble_id)),
      );
    }

    function handlePrimaryAction(action: ShellBallPrimaryAction) {
      switch (action) {
        case "attach_file":
          handlersRef.current.onAttachFile();
          break;
        case "submit":
          handleSubmitBubbleMock();
          break;
        case "primary_click":
          handlersRef.current.onPrimaryClick();
          break;
      }
    }

    function handleBubbleAction(payload: ShellBallBubbleActionPayload) {
      const nextItems = updateShellBallBubbleItems((currentItems) => applyShellBallBubbleAction(currentItems, payload));

      if (payload.action === "pin") {
        detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
        void syncPinnedBubbleWindowAnchor(payload.bubbleId, nextItems);
        return;
      }

      detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
      void closeShellBallPinnedBubbleWindow(payload.bubbleId);
    }

    void Promise.all([
      currentWindow.listen<ShellBallHelperReadyPayload>(
        shellBallWindowSyncEvents.helperReady,
        ({ payload }) => {
          void emitSnapshotTo(payload.role);
        },
      ),
      currentWindow.listen<ShellBallPinnedWindowReadyPayload>(
        shellBallWindowSyncEvents.pinnedWindowReady,
        ({ payload }) => {
          void emitToShellBallWindowLabel(payload.windowLabel, shellBallWindowSyncEvents.snapshot, snapshotRef.current);
          void syncPinnedBubbleWindowAnchor(payload.bubbleId);
        },
      ),
      currentWindow.listen<ShellBallPinnedWindowDetachedPayload>(
        shellBallWindowSyncEvents.pinnedWindowDetached,
        ({ payload }) => {
          detachedPinnedBubbleIdsRef.current.add(payload.bubbleId);
        },
      ),
      currentWindow.listen<ShellBallInputHoverPayload>(shellBallWindowSyncEvents.inputHover, ({ payload }) => {
        if (payload.active) {
          handlersRef.current.onRegionEnter();
          return;
        }

        handlersRef.current.onRegionLeave();
      }),
      currentWindow.listen<ShellBallInputFocusPayload>(shellBallWindowSyncEvents.inputFocus, ({ payload }) => {
        handlersRef.current.onInputFocusChange(payload.focused);
      }),
      currentWindow.listen<ShellBallInputDraftPayload>(shellBallWindowSyncEvents.inputDraft, ({ payload }) => {
        handlersRef.current.setInputValue(payload.value);
      }),
      currentWindow.listen<ShellBallPrimaryActionPayload>(
        shellBallWindowSyncEvents.primaryAction,
        ({ payload }) => {
          handlePrimaryAction(payload.action);
        },
      ),
      currentWindow.listen<ShellBallBubbleActionPayload>(shellBallWindowSyncEvents.bubbleAction, ({ payload }) => {
        handleBubbleAction(payload);
      }),
      currentWindow.listen<ShellBallBubbleInteractionPayload>(shellBallWindowSyncEvents.bubbleInteraction, ({ payload }) => {
        bubbleInteractionActiveRef.current = payload.active;

        if (payload.active) {
          clearShellBallBubbleDissipationTimers();
          return;
        }

        syncShellBallBubbleDissipation(bubbleItemsRef.current);
      }),
      currentWindow.onMoved(() => {
        void syncAnchoredPinnedBubbleWindows();
      }),
      currentWindow.onResized(() => {
        void syncAnchoredPinnedBubbleWindows();
      }),
    ]).then((unlisteners) => {
      if (disposed) {
        for (const unlisten of unlisteners) {
          unlisten();
        }
        return;
      }

      cleanupFns = unlisteners;
    });

    return () => {
      disposed = true;
      for (const cleanup of cleanupFns) {
        cleanup();
      }
    };
  }, []);

  useEffect(() => {
    syncShellBallBubbleDissipation(bubbleItems);
  }, [bubbleItems]);

  useEffect(() => {
    return () => {
      clearShellBallBubbleDissipationTimers();
    };
  }, []);

  return { snapshot };
}

export function useShellBallHelperWindowSnapshot({ role }: ShellBallHelperSnapshotInput) {
  const [snapshot, setSnapshot] = useState(createDefaultShellBallWindowSnapshot);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    const targetLabel = role === "pinned" ? currentWindow.label : shellBallWindowLabels[role];

    if (role === "pinned" && getShellBallPinnedBubbleIdFromLabel(targetLabel) === null) {
      return;
    }

    if (role !== "pinned" && currentWindow.label !== targetLabel) {
      return;
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    void currentWindow
      .listen(shellBallWindowSyncEvents.snapshot, ({ payload }) => {
        setSnapshot(payload as ReturnType<typeof createDefaultShellBallWindowSnapshot>);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;

        if (role === "pinned") {
          const bubbleId = getShellBallPinnedBubbleIdFromLabel(targetLabel);

          if (bubbleId !== null) {
            void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pinnedWindowReady, {
              windowLabel: targetLabel,
              bubbleId,
            });
          }

          return;
        }

        void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.helperReady, { role });
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, [role]);

  return snapshot;
}

export async function emitShellBallInputHover(active: boolean) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.inputHover, { active });
}

export async function emitShellBallInputFocus(focused: boolean) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.inputFocus, {
    focused,
  });
}

export async function emitShellBallInputDraft(value: string) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.inputDraft, { value });
}

export async function emitShellBallPrimaryAction(action: ShellBallPrimaryAction, source: ShellBallHelperWindowRole) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.primaryAction, {
    action,
    source,
  });
}

export async function emitShellBallBubbleAction(
  action: ShellBallBubbleAction,
  bubbleId: string,
  source: ShellBallBubbleActionPayload["source"] = "bubble",
) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.bubbleAction, {
    action,
    bubbleId,
    source,
  });
}

export async function emitShellBallBubbleInteraction(active: boolean) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.bubbleInteraction, {
    active,
  });
}

export async function emitShellBallPinnedWindowDetached(bubbleId: string) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pinnedWindowDetached, {
    bubbleId,
  });
}
