import { useEffect, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow } from "@tauri-apps/api/window";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallInputBarMode } from "./shellBall.types";
import {
  emitShellBallInputDraft,
  emitShellBallInputFocus,
  emitShellBallInputHover,
  emitShellBallPendingFileAction,
  emitShellBallInputRequestFocus,
  emitShellBallPrimaryAction,
  useShellBallHelperWindowSnapshot,
} from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import { shellBallWindowSyncEvents } from "./shellBall.windowSync";
import { ShellBallAttachmentTray } from "./components/ShellBallAttachmentTray";
import { ShellBallInputBar } from "./components/ShellBallInputBar";

const SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS = 160;
const SHELL_BALL_INPUT_WINDOW_IME_GUARD_MS = 1400;

async function pickShellBallFiles(): Promise<string[]> {
  const result = await invoke<string[]>("pick_shell_ball_files");
  return Array.isArray(result) ? result : [];
}

type ShellBallInputWindowProps = {
  mode?: ShellBallInputBarMode;
  voicePreview?: ShellBallVoicePreview;
  value?: string;
  onValueChange?: (value: string) => void;
  onAttachFile?: () => void;
  onSubmit?: () => void;
  onFocusChange?: (focused: boolean) => void;
};

export function ShellBallInputWindow({
  mode,
  voicePreview,
  value,
  onValueChange,
  onAttachFile,
  onSubmit,
  onFocusChange,
}: ShellBallInputWindowProps) {
  const snapshot = useShellBallHelperWindowSnapshot({ role: "input" });
  const [draftValue, setDraftValue] = useState(value ?? snapshot.inputValue);
  const [focusToken, setFocusToken] = useState(0);
  const [isFocused, setIsFocused] = useState(false);
  const compositionActiveRef = useRef(false);
  const isResizingRef = useRef(false);
  const pendingWindowBlurRef = useRef(false);
  const windowFocusedRef = useRef(false);
  // Browser helper windows always use the DOM timer API here, so the timeout id
  // should stay typed as a numeric handle instead of the Node.js Timeout object.
  const pendingBlurTimeoutRef = useRef<number | null>(null);
  const imeBlurGuardUntilRef = useRef(0);

  const clearPendingBlurTimeout = () => {
    if (pendingBlurTimeoutRef.current === null) {
      return;
    }

    window.clearTimeout(pendingBlurTimeoutRef.current);
    pendingBlurTimeoutRef.current = null;
  };

  const commitWindowBlur = () => {
    clearPendingBlurTimeout();
    pendingWindowBlurRef.current = false;
    setIsFocused(false);
    void emitShellBallInputFocus(false);
    void emitShellBallInputHover(false);
  };

  const armImeBlurGuard = () => {
    imeBlurGuardUntilRef.current = Math.max(imeBlurGuardUntilRef.current, Date.now() + SHELL_BALL_INPUT_WINDOW_IME_GUARD_MS);
  };

  const getImeBlurGuardRemainingMs = () => {
    return Math.max(0, imeBlurGuardUntilRef.current - Date.now());
  };

  const isImeBlurGuardActive = () => {
    return getImeBlurGuardRemainingMs() > 0;
  };

  const scheduleDeferredBlur = (delayMs: number) => {
    clearPendingBlurTimeout();
    pendingBlurTimeoutRef.current = window.setTimeout(() => {
      if (windowFocusedRef.current) {
        return;
      }

      if (compositionActiveRef.current || isImeBlurGuardActive()) {
        scheduleDeferredBlur(
          compositionActiveRef.current
            ? SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS
            : Math.max(SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS, getImeBlurGuardRemainingMs()),
        );
        return;
      }

      commitWindowBlur();
    }, Math.max(SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS, delayMs));
  };

  useEffect(() => {
    if (value !== undefined) {
      setDraftValue(value);
      return;
    }

    setDraftValue(snapshot.inputValue);
  }, [snapshot.inputValue, value]);

  const resolvedMode = mode ?? snapshot.inputBarMode;
  const resolvedVoicePreview = voicePreview ?? snapshot.voicePreview;
  const resolvedValue = value ?? draftValue;
  const { rootRef } = useShellBallWindowMetrics({
    clickThrough: snapshot.inputInteraction.clickThrough,
    role: "input",
    visible: snapshot.visibility.input,
  });

  useEffect(() => {
    const currentWindow = getCurrentWindow();
    windowFocusedRef.current = document.hasFocus();

    let unlisten: (() => void) | null = null;
    let unlistenFocusRequest: (() => void) | null = null;
    void currentWindow.onFocusChanged(({ payload: focused }) => {
      windowFocusedRef.current = focused;
      if (focused) {
        clearPendingBlurTimeout();
        pendingWindowBlurRef.current = false;
        setIsFocused(true);
        void emitShellBallInputHover(true);
        return;
      }

      if (isResizingRef.current) {
        windowFocusedRef.current = true;
        return;
      }

      // Windows IME candidate popups can briefly blur the helper window.
      // Keep the hover input alive until composition finishes or the blur persists.
      if (compositionActiveRef.current) {
        pendingWindowBlurRef.current = true;
        scheduleDeferredBlur(SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS);
        return;
      }

      if (isImeBlurGuardActive()) {
        pendingWindowBlurRef.current = true;
        scheduleDeferredBlur(getImeBlurGuardRemainingMs());
        return;
      }

      scheduleDeferredBlur(SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS);
    }).then((dispose) => {
      unlisten = dispose;
    });

    void currentWindow.listen(shellBallWindowSyncEvents.inputRequestFocus, () => {
      windowFocusedRef.current = true;
      clearPendingBlurTimeout();
      pendingWindowBlurRef.current = false;
      setFocusToken((current) => current + 1);
      setIsFocused(true);
      void currentWindow.setFocus();
    }).then((dispose) => {
      unlistenFocusRequest = dispose;
    });

    return () => {
      clearPendingBlurTimeout();
      unlisten?.();
      unlistenFocusRequest?.();
    };
  }, []);

  function handleValueChange(nextValue: string) {
    if (onValueChange !== undefined) {
      onValueChange(nextValue);
      return;
    }

    setDraftValue(nextValue);
    void emitShellBallInputDraft(nextValue);
  }

  function handleAttachFile() {
    if (onAttachFile !== undefined) {
      onAttachFile();
      return;
    }

    void (async () => {
      try {
        const selectedPaths = await pickShellBallFiles();
        if (selectedPaths.length > 0) {
          await emitShellBallPendingFileAction({
            action: "append",
            paths: selectedPaths,
          });
        }
        await emitShellBallInputRequestFocus(Date.now());
      } catch (error) {
        console.warn("shell-ball file picker failed", error);
        await emitShellBallPrimaryAction("attach_file", "input");
      }
    })();
  }

  function handleSubmit() {
    if (onSubmit !== undefined) {
      onSubmit();
      return;
    }

    void emitShellBallPrimaryAction("submit", "input");
  }

  function handleRemovePendingFile(path: string) {
    void emitShellBallPendingFileAction({ action: "remove", path });
  }

  function handleFocusChange(focused: boolean) {
    if (focused) {
      windowFocusedRef.current = true;
      clearPendingBlurTimeout();
      pendingWindowBlurRef.current = false;
      setIsFocused(true);
    }

    if (onFocusChange !== undefined) {
      onFocusChange(focused);
      return;
    }

    if (!focused && compositionActiveRef.current) {
      pendingWindowBlurRef.current = true;
      scheduleDeferredBlur(SHELL_BALL_INPUT_WINDOW_BLUR_GRACE_MS);
      return;
    }

    if (!focused && isImeBlurGuardActive()) {
      pendingWindowBlurRef.current = true;
      scheduleDeferredBlur(getImeBlurGuardRemainingMs());
      return;
    }

    if (!focused) {
      pendingWindowBlurRef.current = false;
      setIsFocused(false);
    }

    void emitShellBallInputFocus(focused);
  }

  function handleCompositionStateChange(composing: boolean) {
    compositionActiveRef.current = composing;

    if (composing) {
      armImeBlurGuard();
      clearPendingBlurTimeout();
      return;
    }

    if (!pendingWindowBlurRef.current || windowFocusedRef.current) {
      return;
    }

    if (isImeBlurGuardActive()) {
      scheduleDeferredBlur(getImeBlurGuardRemainingMs());
      return;
    }

    // Flush the deferred blur once IME composition is done and the window never regained focus.
    commitWindowBlur();
  }

  function handleTransientInputActivity() {
    armImeBlurGuard();
    clearPendingBlurTimeout();
  }

  function handleResizeStateChange(resizing: boolean) {
    isResizingRef.current = resizing;

    if (resizing) {
      windowFocusedRef.current = true;
      clearPendingBlurTimeout();
      pendingWindowBlurRef.current = false;
      setIsFocused(true);
      void emitShellBallInputHover(true);
      return;
    }

    const rootHovered = rootRef.current?.matches(":hover") ?? false;
    const activeWithinRoot = rootRef.current?.contains(document.activeElement) ?? false;

    if (!rootHovered && !activeWithinRoot) {
      windowFocusedRef.current = false;
      pendingWindowBlurRef.current = false;
      setIsFocused(false);
      void emitShellBallInputHover(false);
      void emitShellBallInputFocus(false);
    }
  }

  function handlePointerLeave() {
    if (isResizingRef.current) {
      return;
    }

    void emitShellBallInputHover(false);
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    const target = event.target;
    if (target instanceof HTMLElement && target.closest('[data-shell-ball-input-resize-handle="true"]')) {
      return;
    }

    windowFocusedRef.current = true;
    clearPendingBlurTimeout();
    pendingWindowBlurRef.current = false;
    setIsFocused(true);
    void emitShellBallInputFocus(true);
    void getCurrentWindow().setFocus();
  }

  return (
    <div
      ref={rootRef}
      className="shell-ball-window shell-ball-window--input"
      data-shell-ball-input-window="true"
      onPointerDown={handlePointerDown}
      onPointerEnter={() => {
        void emitShellBallInputHover(true);
      }}
      onPointerLeave={handlePointerLeave}
    >
      <ShellBallAttachmentTray paths={snapshot.pendingFiles} onRemove={handleRemovePendingFile} />
      <ShellBallInputBar
        focusToken={focusToken}
        mode={resolvedMode}
        voicePreview={resolvedVoicePreview}
        value={resolvedValue}
        hasPendingFiles={snapshot.pendingFiles.length > 0}
        onValueChange={handleValueChange}
        onAttachFile={handleAttachFile}
        onSubmit={handleSubmit}
        onFocusChange={handleFocusChange}
        onResizeStateChange={handleResizeStateChange}
        onCompositionStateChange={handleCompositionStateChange}
        onTransientInputActivity={handleTransientInputActivity}
      />
    </div>
  );
}
