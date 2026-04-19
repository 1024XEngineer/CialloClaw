import { useEffect, useRef } from "react";
import type { ChangeEvent, CompositionEvent, KeyboardEvent } from "react";
import styled from "styled-components";
import { ArrowUp, Paperclip } from "lucide-react";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallInputBarMode } from "../shellBall.types";

type ShellBallInputBarProps = {
  mode: ShellBallInputBarMode;
  voicePreview: ShellBallVoicePreview;
  value: string;
  hasPendingFiles?: boolean;
  focusToken?: number;
  onValueChange: (value: string) => void;
  onAttachFile: () => void;
  onSubmit: () => void;
  onFocusChange: (focused: boolean) => void;
  onResizeStateChange?: (resizing: boolean) => void;
  onCompositionStateChange?: (composing: boolean) => void;
  onTransientInputActivity?: () => void;
};

const SHELL_BALL_INPUT_LABEL = "Message";

/**
 * Renders the floating shell-ball input bar with the supplied Uiverse-inspired
 * field while preserving the attach and send buttons below the field.
 *
 * @param props Shell-ball input mode, draft state, and interaction callbacks.
 * @returns The shell-ball input bar UI.
 */
export function ShellBallInputBar({
  mode,
  voicePreview,
  value,
  hasPendingFiles = false,
  focusToken = 0,
  onValueChange,
  onAttachFile,
  onSubmit,
  onFocusChange,
  onResizeStateChange: _onResizeStateChange = () => {},
  onCompositionStateChange = () => {},
  onTransientInputActivity = () => {},
}: ShellBallInputBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const compositionActiveRef = useRef(false);
  const trimmedValue = value.trim();
  const isHidden = mode === "hidden";
  const isInteractive = mode === "interactive";
  const isReadonly = mode === "readonly";
  const isVoice = mode === "voice";
  const buttonsDisabled = isHidden || isReadonly || isVoice;
  const submitDisabled = !isInteractive || (trimmedValue === "" && !hasPendingFiles);

  useEffect(() => {
    if (inputRef.current === null) {
      return;
    }

    if (!isInteractive) {
      if (inputRef.current === document.activeElement) {
        inputRef.current.blur();
        onFocusChange(false);
      }
      return;
    }

    if (focusToken !== 0) {
      inputRef.current.focus();
    }
  }, [focusToken, isInteractive, onFocusChange]);

  function handleChange(event: ChangeEvent<HTMLInputElement>) {
    if (!isInteractive) {
      return;
    }

    onValueChange(event.target.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (!event.ctrlKey && !event.metaKey && !event.altKey && (event.key.length === 1 || event.key === "Enter")) {
      onTransientInputActivity();
    }

    if (event.key !== "Enter" || submitDisabled) {
      return;
    }

    event.preventDefault();
    onSubmit();
  }

  function handleCompositionStart(_event: CompositionEvent<HTMLInputElement>) {
    compositionActiveRef.current = true;
    onTransientInputActivity();
    onCompositionStateChange(true);
  }

  function handleCompositionEnd(_event: CompositionEvent<HTMLInputElement>) {
    compositionActiveRef.current = false;
    onCompositionStateChange(false);
  }

  const hiddenState = isHidden || isVoice;

  return (
    <StyledInputBar
      data-filled={trimmedValue !== "" ? "true" : "false"}
      data-hidden={hiddenState ? "true" : "false"}
      data-mode={mode}
      data-voice-preview={voicePreview ?? undefined}
    >
      <div className="shell-ball-uiverse-inputbox">
        <input
          ref={inputRef}
          required
          type="text"
          value={value}
          onChange={handleChange}
          onCompositionStart={handleCompositionStart}
          onCompositionEnd={handleCompositionEnd}
          onKeyDown={handleKeyDown}
          onFocus={() => onFocusChange(true)}
          onBlur={() => {
            if (compositionActiveRef.current) {
              return;
            }

            onFocusChange(false);
          }}
          readOnly={isHidden || isReadonly || isVoice}
          tabIndex={isHidden || isVoice ? -1 : 0}
          aria-label="Shell-ball input"
          placeholder={isVoice ? "Voice capture is active" : ""}
        />
        <span>{SHELL_BALL_INPUT_LABEL}</span>
        <i />
      </div>
      <div className="shell-ball-uiverse-actions">
        <button
          type="button"
          className="shell-ball-uiverse-action"
          onClick={onAttachFile}
          disabled={buttonsDisabled}
          aria-label="Attach file"
        >
          <Paperclip className="shell-ball-uiverse-action-icon" />
        </button>
        <button
          type="button"
          className="shell-ball-uiverse-action shell-ball-uiverse-action--send"
          onClick={onSubmit}
          disabled={submitDisabled}
          aria-label={isReadonly ? "Send disabled" : isVoice ? "Send unavailable during voice capture" : "Send request"}
        >
          <ArrowUp className="shell-ball-uiverse-action-icon" />
        </button>
      </div>
    </StyledInputBar>
  );
}

const StyledInputBar = styled.div`
  align-items: flex-end;
  background: transparent;
  border: 0;
  display: inline-flex;
  flex-direction: column;
  gap: 0.42rem;
  padding: 0;
  width: max-content;

  &[data-hidden="true"] {
    display: none;
  }

  .shell-ball-uiverse-inputbox {
    position: relative;
    width: 196px;
  }

  .shell-ball-uiverse-inputbox input {
    position: relative;
    width: 100%;
    padding: 20px 10px 10px;
    background: transparent;
    outline: none;
    box-shadow: none;
    border: none;
    color: rgba(35, 36, 42, 0.96);
    font-size: 1em;
    letter-spacing: 0.05em;
    transition: 0.5s;
    z-index: 10;
  }

  .shell-ball-uiverse-inputbox span {
    position: absolute;
    left: 0;
    padding: 20px 10px 10px;
    font-size: 1em;
    color: rgba(143, 143, 143, 0.74);
    letter-spacing: 0.05em;
    transition: 0.5s;
    pointer-events: none;
  }

  .shell-ball-uiverse-inputbox input:valid ~ span,
  .shell-ball-uiverse-inputbox input:focus ~ span,
  &[data-filled="true"] .shell-ball-uiverse-inputbox span {
    color: rgba(128, 128, 128, 0.82);
    transform: translateX(-10px) translateY(-34px);
    font-size: 0.75em;
  }

  .shell-ball-uiverse-inputbox i {
    position: absolute;
    left: 0;
    bottom: 0;
    width: 100%;
    height: 2px;
    background: rgba(128, 128, 128, 0.42);
    border-radius: 4px;
    transition: 0.5s;
    pointer-events: none;
    z-index: 9;
  }

  .shell-ball-uiverse-inputbox input:valid ~ i,
  .shell-ball-uiverse-inputbox input:focus ~ i,
  &[data-filled="true"] .shell-ball-uiverse-inputbox i {
    height: 44px;
    background: rgba(128, 128, 128, 0.24);
  }

  .shell-ball-uiverse-actions {
    align-items: center;
    display: inline-flex;
    gap: 0.35rem;
    justify-content: flex-end;
    width: fit-content;
  }

  .shell-ball-uiverse-action {
    align-items: center;
    background: transparent;
    border: 0;
    border-radius: 999px;
    color: rgba(108, 108, 108, 0.66);
    cursor: pointer;
    display: inline-flex;
    height: 1.72rem;
    justify-content: center;
    transition:
      transform 160ms ease,
      background 160ms ease,
      color 160ms ease,
      opacity 160ms ease;
    width: 1.72rem;
  }

  .shell-ball-uiverse-action:hover:not(:disabled) {
    color: rgba(88, 88, 88, 0.84);
    transform: translateY(-1px);
  }

  .shell-ball-uiverse-action:disabled {
    cursor: default;
    opacity: 0.58;
  }

  .shell-ball-uiverse-action-icon {
    height: 0.75rem;
    width: 0.75rem;
  }
`;
