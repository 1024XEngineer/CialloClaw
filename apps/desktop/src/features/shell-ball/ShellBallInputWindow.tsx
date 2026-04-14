import { useEffect, useState } from "react";
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
    clickThrough: resolvedMode === "interactive" && !isFocused,
    role: "input",
    visible: snapshot.visibility.input,
  });

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    let unlisten: (() => void) | null = null;
    let unlistenFocusRequest: (() => void) | null = null;
    void currentWindow.onFocusChanged(({ payload: focused }) => {
      if (focused) {
        setIsFocused(true);
        void emitShellBallInputHover(true);
        return;
      }

      setIsFocused(false);
      void emitShellBallInputFocus(false);
      void emitShellBallInputHover(false);
    }).then((dispose) => {
      unlisten = dispose;
    });

    void currentWindow.listen(shellBallWindowSyncEvents.inputRequestFocus, () => {
      setFocusToken((current) => current + 1);
      setIsFocused(true);
      void currentWindow.setFocus();
    }).then((dispose) => {
      unlistenFocusRequest = dispose;
    });

    return () => {
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
    setIsFocused(focused);
    if (onFocusChange !== undefined) {
      onFocusChange(focused);
      return;
    }

    void emitShellBallInputFocus(focused);
  }

  function handlePointerDown() {
    setIsFocused(true);
    void emitShellBallInputFocus(true);
    void getCurrentWindow().setFocus();
  }

  return (
    <div
      ref={rootRef}
      className="shell-ball-window shell-ball-window--input"
      onPointerDown={handlePointerDown}
      onPointerEnter={() => {
        void emitShellBallInputHover(true);
      }}
      onPointerLeave={() => {
        void emitShellBallInputHover(false);
      }}
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
      />
    </div>
  );
}
