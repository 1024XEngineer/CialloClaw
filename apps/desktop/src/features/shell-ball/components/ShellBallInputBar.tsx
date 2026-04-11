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
  onValueChange: (value: string) => void;
  onAttachFile: () => void;
  onSubmit: () => void;
  onFocusChange: (focused: boolean) => void;
};

export function ShellBallInputBar({
  mode,
  voicePreview,
  value,
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
  const buttonsDisabled = !isInteractive;
  const submitDisabled = !isInteractive || trimmedValue === "";
  const previewLabel = voicePreview === null ? null : `Release to ${voicePreview}`;

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

  if (mode === "hidden") {
    return null;
  }

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
        readOnly={!isInteractive}
        tabIndex={isInteractive ? 0 : -1}
        aria-label="Shell-ball input"
        placeholder={isVoice ? "Voice capture is active" : "Type a request for shell-ball"}
      />
      {previewLabel === null ? null : (
        <span className="shell-ball-input-bar__preview" aria-live="polite">
          {previewLabel}
        </span>
      )}
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
