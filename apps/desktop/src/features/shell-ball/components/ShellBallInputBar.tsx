import { useEffect, useLayoutEffect, useRef } from "react";
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
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const compositionActiveRef = useRef(false);
  const multilineStateRef = useRef(false);
  const trimmedValue = value.trim();
  const isHidden = mode === "hidden";
  const isInteractive = mode === "interactive";
  const isReadonly = mode === "readonly";
  const isVoice = mode === "voice";
  const buttonsDisabled = isHidden || isReadonly || isVoice;
  const submitDisabled = !isInteractive || (trimmedValue === "" && !hasPendingFiles);

  useLayoutEffect(() => {
    const field = inputRef.current;
    if (field === null) {
      return;
    }

    // Keep the decorative highlight layer aligned with the real textarea height
    // so multiline growth and shrink stay visually locked together.
    field.style.height = "auto";
    const computedStyle = window.getComputedStyle(field);
    const maxHeight = Number.parseFloat(window.getComputedStyle(field).maxHeight) || Number.POSITIVE_INFINITY;
    const nextHeight = Math.min(Math.max(44, field.scrollHeight), maxHeight);
    const lineHeight = Number.parseFloat(computedStyle.lineHeight) || 0;
    const verticalPadding = Number.parseFloat(computedStyle.paddingTop) + Number.parseFloat(computedStyle.paddingBottom);
    const multilineThreshold = lineHeight > 0 ? 44 + lineHeight * 0.5 : 52;
    const isMultiline = nextHeight > multilineThreshold && nextHeight > verticalPadding + lineHeight;
    const inputShell = field.parentElement;
    const isCollapsingToSingleLine = multilineStateRef.current && !isMultiline;

    field.style.height = `${nextHeight}px`;

    if (inputShell !== null) {
      inputShell.style.setProperty("--shell-ball-input-height", `${Math.max(44, nextHeight)}px`);
      inputShell.dataset.collapseToSingleLine = isCollapsingToSingleLine ? "true" : "false";
      inputShell.dataset.multiline = isMultiline ? "true" : "false";
    }

    multilineStateRef.current = isMultiline;
  }, [value]);

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

  function handleChange(event: ChangeEvent<HTMLTextAreaElement>) {
    if (!isInteractive) {
      return;
    }

    onValueChange(event.target.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (!event.ctrlKey && !event.metaKey && !event.altKey && (event.key.length === 1 || event.key === "Enter")) {
      onTransientInputActivity();
    }

    if (event.key !== "Enter" || event.shiftKey || submitDisabled) {
      return;
    }

    event.preventDefault();
    onSubmit();
  }

  function handleCompositionStart(_event: CompositionEvent<HTMLTextAreaElement>) {
    compositionActiveRef.current = true;
    onTransientInputActivity();
    onCompositionStateChange(true);
  }

  function handleCompositionEnd(_event: CompositionEvent<HTMLTextAreaElement>) {
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
        <textarea
          ref={inputRef}
          data-shell-ball-interactive="true"
          required
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
          tabIndex={isInteractive ? 0 : -1}
          aria-label="Shell-ball input"
          placeholder={isVoice ? "Voice capture is active" : ""}
          rows={1}
        />
        <span>{SHELL_BALL_INPUT_LABEL}</span>
        <i />
      </div>
      <div className="shell-ball-uiverse-actions">
        <button
          type="button"
          className="shell-ball-uiverse-action"
          data-shell-ball-interactive="true"
          onClick={onAttachFile}
          disabled={buttonsDisabled}
          aria-label="Attach file"
        >
          <Paperclip className="shell-ball-uiverse-action-icon" />
        </button>
        <button
          type="button"
          className="shell-ball-uiverse-action shell-ball-uiverse-action--send"
          data-shell-ball-interactive="true"
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
    --shell-ball-input-height: 44px;
    position: relative;
    width: 196px;
  }

  .shell-ball-uiverse-inputbox textarea {
    position: relative;
    top: 6px;
    width: 100%;
    padding: 10px;
    background: transparent;
    outline: none;
    box-shadow: none;
    border: none;
    caret-color: rgba(255, 255, 255, 0.96);
    color: rgba(255, 255, 255, 0.96);
    font-size: 1em;
    letter-spacing: 0.05em;
    line-height: 1.4;
    max-height: calc(1.4em * 3 + 20px);
    min-height: 44px;
    overflow-y: auto;
    -ms-overflow-style: none;
    scrollbar-width: none;
    resize: none;
    transition: 0.5s;
    z-index: 10;
  }

  .shell-ball-uiverse-inputbox textarea::-webkit-scrollbar {
    display: none;
  }

  .shell-ball-uiverse-inputbox span {
    position: absolute;
    left: 0;
    top: 4px;
    padding: 0 10px;
    font-size: 1em;
    color: rgba(143, 143, 143, 0.74);
    letter-spacing: 0.05em;
    transform: translateY(calc(var(--shell-ball-input-height) - 24px));
    transition: 0.5s;
    pointer-events: none;
  }

  .shell-ball-uiverse-inputbox[data-multiline="true"] span {
    transition: none;
  }

  .shell-ball-uiverse-inputbox textarea:valid ~ span,
  .shell-ball-uiverse-inputbox textarea:focus ~ span,
  &[data-filled="true"] .shell-ball-uiverse-inputbox span {
    color: rgba(128, 128, 128, 0.82);
    transform: translateX(-10px) translateY(-14px);
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
    transform: none;
    transition: 0.5s;
    pointer-events: none;
    z-index: 9;
  }

  .shell-ball-uiverse-inputbox[data-multiline="true"] i {
    transition: none;
  }

  .shell-ball-uiverse-inputbox[data-collapse-to-single-line="true"] i {
    transition: none;
  }

  .shell-ball-uiverse-inputbox textarea:valid ~ i,
  .shell-ball-uiverse-inputbox textarea:focus ~ i,
  &[data-filled="true"] .shell-ball-uiverse-inputbox i {
    height: var(--shell-ball-input-height);
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
