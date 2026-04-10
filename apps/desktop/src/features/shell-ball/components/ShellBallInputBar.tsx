import type { ChangeEvent, KeyboardEvent } from "react";
import { ArrowUp, Paperclip } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallInputBarMode } from "../shellBall.types";

type ShellBallInputBarProps = {
  mode: ShellBallInputBarMode;
  value: string;
  onValueChange: (value: string) => void;
  onAttachFile: () => void;
  onSubmit: () => void;
};

export function ShellBallInputBar({ mode, value, onValueChange, onAttachFile, onSubmit }: ShellBallInputBarProps) {
  if (mode === "hidden") {
    return null;
  }

  const trimmedValue = value.trim();
  const isInteractive = mode === "interactive";
  const isReadonly = mode === "readonly";
  const isVoice = mode === "voice";
  const buttonsDisabled = !isInteractive;
  const submitDisabled = !isInteractive || trimmedValue === "";

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
    <div className={cn("shell-ball-input-bar", `shell-ball-input-bar--${mode}`)} data-mode={mode}>
      <input
        type="text"
        className="shell-ball-input-bar__field"
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        readOnly={!isInteractive}
        aria-label="Shell-ball input"
        placeholder={isVoice ? "Voice capture is active" : "Type a request for shell-ball"}
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
