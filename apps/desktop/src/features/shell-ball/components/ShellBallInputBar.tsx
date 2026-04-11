import { useEffect, useRef } from "react";
import type { ChangeEvent, KeyboardEvent } from "react";
import { ArrowUp, Paperclip } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallInputBarMode } from "../shellBall.types";

type ShellBallInputBarProps = {
  mode: ShellBallInputBarMode;
  voicePreview: ShellBallVoicePreview;
  value: string;
  focusToken?: number;
  onValueChange: (value: string) => void;
  onAttachFile: () => void;
  onSubmit: () => void;
  onFocusChange: (focused: boolean) => void;
};

export function ShellBallInputBar({
  mode,
  voicePreview,
  value,
  focusToken = 0,
  onValueChange,
  onAttachFile,
  onSubmit,
  onFocusChange,
}: ShellBallInputBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const trimmedValue = value.trim();
  const isInteractive = mode === "interactive";
  const isReadonly = mode === "readonly";
  const isVoice = mode === "voice";
  const buttonsDisabled = isReadonly || isVoice;
  const submitDisabled = !isInteractive || trimmedValue === "";

  useEffect(() => {
    if (inputRef.current === null) {
      return;
    }

    if (isInteractive) {
      return;
    }

    if (inputRef.current === document.activeElement) {
      inputRef.current.blur();
      onFocusChange(false);
    }
  }, [isInteractive, onFocusChange]);

  useEffect(() => {
    if (!isInteractive || focusToken === 0 || inputRef.current === null) {
      return;
    }

    inputRef.current.focus();
    inputRef.current.select();
  }, [focusToken, isInteractive]);

  function handleChange(event: ChangeEvent<HTMLInputElement>) {
    if (!isInteractive) {
      return;
    }

    onValueChange(event.target.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key !== "Enter" || submitDisabled) {
      return;
    }

    event.preventDefault();
    onSubmit();
  }

  if (mode === "hidden") {
    return null;
  }

  return (
    <div
      className={cn(
        "shell-ball-input-bar",
        `shell-ball-input-bar--${mode}`,
        voicePreview !== null && `shell-ball-input-bar--preview-${voicePreview}`,
      )}
      data-mode={mode}
      data-voice-preview={voicePreview ?? undefined}
    >
      <input
        ref={inputRef}
        type="text"
        className="shell-ball-input-bar__field"
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={() => onFocusChange(true)}
        onBlur={() => onFocusChange(false)}
        readOnly={isReadonly || isVoice}
        tabIndex={isVoice ? -1 : 0}
        aria-label="Shell-ball input"
        placeholder={isVoice ? "Voice capture is active" : ""}
      />
      <button
        type="button"
        className="shell-ball-input-bar__action"
        onClick={onAttachFile}
        disabled={buttonsDisabled}
        aria-label="Attach file"
      >
        <Paperclip className="shell-ball-input-bar__action-icon" />
      </button>
      <button
        type="button"
        className="shell-ball-input-bar__action shell-ball-input-bar__action--send"
        onClick={onSubmit}
        disabled={submitDisabled}
        aria-label={isReadonly ? "Send disabled" : isVoice ? "Send unavailable during voice capture" : "Send request"}
      >
        <ArrowUp className="shell-ball-input-bar__action-icon" />
      </button>
    </div>
  );
}
