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
  handlersRef.current = {
    setInputValue: input.setInputValue,
    onRegionEnter: input.onRegionEnter,
    onRegionLeave: input.onRegionLeave,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onAttachFile: input.onAttachFile,
    onPrimaryClick: input.onPrimaryClick,
  };

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

    async function syncPinnedBubbleWindowAnchor(bubbleId: string) {
      if (detachedPinnedBubbleIdsRef.current.has(bubbleId)) {
        return;
      }

      const bubbleItem = bubbleItemsRef.current.find((item) => item.bubble.bubble_id === bubbleId && item.bubble.pinned);

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
          handlersRef.current.onSubmitText();
          break;
        case "primary_click":
          handlersRef.current.onPrimaryClick();
          break;
      }
    }

    function handleBubbleAction(payload: ShellBallBubbleActionPayload) {
      setBubbleItems((currentItems) => applyShellBallBubbleAction(currentItems, payload));

      if (payload.action === "pin") {
        detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
        void syncPinnedBubbleWindowAnchor(payload.bubbleId);
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

export async function emitShellBallPinnedWindowDetached(bubbleId: string) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pinnedWindowDetached, {
    bubbleId,
  });
}
