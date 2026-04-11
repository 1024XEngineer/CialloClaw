import { useEffect, useState } from "react";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallInputBarMode } from "./shellBall.types";
import {
  emitShellBallInputDraft,
  emitShellBallInputFocus,
  emitShellBallInputHover,
  emitShellBallPrimaryAction,
  useShellBallHelperWindowSnapshot,
} from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import { ShellBallInputBar } from "./components/ShellBallInputBar";

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
    role: "input",
    visible: snapshot.visibility.input,
  });

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

    void emitShellBallPrimaryAction("attach_file", "input");
  }

  function handleSubmit() {
    if (onSubmit !== undefined) {
      onSubmit();
      return;
    }

    void emitShellBallPrimaryAction("submit", "input");
  }

  function handleFocusChange(focused: boolean) {
    if (onFocusChange !== undefined) {
      onFocusChange(focused);
      return;
    }

    void emitShellBallInputFocus(focused);
  }

  return (
    <div
      ref={rootRef}
      className="shell-ball-window shell-ball-window--input"
      aria-label="Shell-ball input window"
      onPointerEnter={() => {
        void emitShellBallInputHover(true);
      }}
      onPointerLeave={() => {
        void emitShellBallInputHover(false);
      }}
    >
      <ShellBallInputBar
        mode={resolvedMode}
        voicePreview={resolvedVoicePreview}
        value={resolvedValue}
        onValueChange={handleValueChange}
        onAttachFile={handleAttachFile}
        onSubmit={handleSubmit}
        onFocusChange={handleFocusChange}
      />
    </div>
  );
}
