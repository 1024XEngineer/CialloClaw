import { useEffect, useMemo, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { shellBallWindowLabels } from "../../platform/shellBallWindowController";
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
  type ShellBallPrimaryAction,
  type ShellBallPrimaryActionPayload,
} from "./shellBall.windowSync";

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

export function applyShellBallBubbleAction(
  items: ShellBallBubbleItem[],
  payload: Pick<ShellBallBubbleActionPayload, "action" | "bubbleId">,
): ShellBallBubbleItem[] {
  if (payload.action === "delete") {
    return items.filter((item) => item.bubble.bubble_id !== payload.bubbleId);
  }

  return items.map((item) => {
    if (item.bubble.bubble_id !== payload.bubbleId) {
      return item;
    }

    return {
      ...item,
      bubble: {
        ...item.bubble,
        pinned: true,
      },
    };
  });
}

export function useShellBallCoordinator(input: ShellBallCoordinatorInput) {
  const [bubbleItems, setBubbleItems] = useState(() => cloneShellBallBubbleItems(SHELL_BALL_LOCAL_BUBBLE_ITEMS));
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

    async function emitSnapshotTo(role: ShellBallHelperWindowRole) {
      await currentWindow.emitTo(shellBallWindowLabels[role], shellBallWindowSyncEvents.snapshot, snapshotRef.current);
    }

    void Promise.all([emitSnapshotTo("bubble"), emitSnapshotTo("input")]);
  }, [snapshot]);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let disposed = false;
    let cleanupFns: Array<() => void> = [];

    async function emitSnapshotTo(role: ShellBallHelperWindowRole) {
      await currentWindow.emitTo(shellBallWindowLabels[role], shellBallWindowSyncEvents.snapshot, snapshotRef.current);
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
    }

    void Promise.all([
      currentWindow.listen<ShellBallHelperReadyPayload>(
        shellBallWindowSyncEvents.helperReady,
        ({ payload }) => {
          void emitSnapshotTo(payload.role);
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

    if (currentWindow.label !== shellBallWindowLabels[role]) {
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

export async function emitShellBallBubbleAction(action: ShellBallBubbleAction, bubbleId: string) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.bubbleAction, {
    action,
    bubbleId,
    source: "bubble",
  });
}
