import { useEffect, useLayoutEffect, useRef } from "react";
import type { ChangeEvent, CompositionEvent, KeyboardEvent } from "react";
import styled from "styled-components";
import { ArrowUp, Paperclip, X } from "lucide-react";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallInputBarMode } from "../shellBall.types";

type ShellBallInputBarProps = {
  mode: ShellBallInputBarMode;
  voicePreview: ShellBallVoicePreview;
  value: string;
  hasPendingFiles?: boolean;
  focusToken?: number;
  label?: string;
  placeholder?: string;
  auxiliaryAction?: "attach" | "cancel";
  allowEmptySubmit?: boolean;
  onValueChange: (value: string) => void;
  onAttachFile: () => void;
  onCancel?: () => void;
  onSubmit: () => void;
  onFocusChange: (focused: boolean) => void;
  onResizeStateChange?: (resizing: boolean) => void;
  onCompositionStateChange?: (composing: boolean) => void;
  onTransientInputActivity?: () => void;
};

const SHELL_BALL_INPUT_LABEL = "输入";

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
  label = SHELL_BALL_INPUT_LABEL,
  placeholder,
  auxiliaryAction = "attach",
  allowEmptySubmit = false,
  onValueChange,
  onAttachFile,
  onCancel,
  onSubmit,
  onFocusChange,
  onResizeStateChange: _onResizeStateChange = () => {},
  onCompositionStateChange = () => {},
  onTransientInputActivity = () => {},
}: ShellBallInputBarProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const compositionActiveRef = useRef(false);
  const appliedFocusTokenRef = useRef(0);
  const multilineStateRef = useRef(false);
  const trimmedValue = value.trim();
  const isHidden = mode === "hidden";
  const isInteractive = mode === "interactive";
  const isReadonly = mode === "readonly";
  const isVoice = mode === "voice";
  const buttonsDisabled = isHidden || isReadonly || isVoice;
  const submitDisabled = !isInteractive || (!allowEmptySubmit && trimmedValue === "" && !hasPendingFiles);
  const auxiliaryActionLabel = auxiliaryAction === "cancel" ? "退出意图修改" : "添加文件";
  const visibleLabel = label?.trim() ?? "";

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

    if (focusToken !== 0 && focusToken !== appliedFocusTokenRef.current) {
      appliedFocusTokenRef.current = focusToken;
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

  function restoreTextareaFocus() {
    if (!isInteractive) {
      return;
    }

    window.requestAnimationFrame(() => {
      const field = inputRef.current;
      if (field === null) {
        return;
      }

      field.focus();
      const selectionIndex = field.value.length;
      field.setSelectionRange(selectionIndex, selectionIndex);
    });
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
          placeholder={isVoice ? "语音输入中" : placeholder ?? ""}
          rows={1}
        />
        {visibleLabel === "" ? null : <span>{visibleLabel}</span>}
        <i />
      </div>
      <div className="shell-ball-uiverse-actions">
        <button
          type="button"
          className="shell-ball-uiverse-action"
          data-shell-ball-interactive="true"
          onMouseDown={(event) => {
            event.preventDefault();
          }}
          onClick={() => {
            if (auxiliaryAction === "cancel") {
              onCancel?.();
              return;
            }

            onAttachFile();
            restoreTextareaFocus();
          }}
          disabled={buttonsDisabled}
          aria-label={auxiliaryActionLabel}
        >
          {auxiliaryAction === "cancel"
            ? <X className="shell-ball-uiverse-action-icon" />
            : <Paperclip className="shell-ball-uiverse-action-icon" />}
        </button>
        <button
          type="button"
          className="shell-ball-uiverse-action shell-ball-uiverse-action--send"
          data-shell-ball-interactive="true"
          onMouseDown={(event) => {
            event.preventDefault();
          }}
          onClick={() => {
            onSubmit();
            restoreTextareaFocus();
          }}
          disabled={submitDisabled}
          aria-label={isReadonly ? "发送不可用" : isVoice ? "语音输入期间不可发送" : "发送"}
        >
          <ArrowUp className="shell-ball-uiverse-action-icon" />
        </button>
      </div>
    </StyledInputBar>
  );
}

const StyledInputBar = styled.div`
  --shell-ball-input-paper: linear-gradient(180deg, rgba(255, 253, 249, 0.92), rgba(245, 236, 224, 0.86));
  --shell-ball-input-paper-soft: linear-gradient(180deg, rgba(255, 255, 255, 0.18), rgba(251, 246, 238, 0.14));
  --shell-ball-input-line: rgba(132, 118, 99, 0.2);
  --shell-ball-input-line-strong: rgba(106, 145, 200, 0.34);
  --shell-ball-input-ink: rgba(36, 49, 58, 0.96);
  --shell-ball-input-copy: rgba(96, 103, 113, 0.8);
  --shell-ball-input-shadow: 0 14px 26px -22px rgba(122, 106, 79, 0.34);
  align-items: flex-end;
  background: transparent;
  border: 0;
  display: inline-flex;
  flex-direction: column;
  gap: 0.46rem;
  padding: 0;
  width: max-content;

  &[data-hidden="true"] {
    display: none;
  }

  .shell-ball-uiverse-inputbox {
    --shell-ball-input-height: 44px;
    isolation: isolate;
    position: relative;
    width: 196px;
  }

  .shell-ball-uiverse-inputbox textarea {
    position: relative;
    top: 6px;
    width: 100%;
    padding: 10px;
    background: var(--shell-ball-input-paper-soft);
    outline: none;
    box-shadow:
      0 10px 20px -20px rgba(122, 106, 79, 0.22),
      0 1px 0 rgba(255, 255, 255, 0.34) inset;
    border: 1px solid transparent;
    border-radius: 1rem;
    caret-color: rgba(106, 145, 200, 0.94);
    color: var(--shell-ball-input-ink);
    font-size: 1em;
    font-weight: 500;
    letter-spacing: 0.05em;
    line-height: 1.4;
    max-height: calc(1.4em * 3 + 20px);
    min-height: 44px;
    overflow-y: auto;
    -ms-overflow-style: none;
    scrollbar-width: none;
    resize: none;
    transition:
      background 340ms ease,
      border-color 340ms ease,
      box-shadow 340ms ease,
      transform 340ms ease;
    z-index: 10;
  }

  .shell-ball-uiverse-inputbox textarea::placeholder {
    color: rgba(122, 132, 143, 0.72);
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
    color: var(--shell-ball-input-copy);
    font-weight: 600;
    letter-spacing: 0.05em;
    transform: translateY(calc(var(--shell-ball-input-height) - 24px));
    transition:
      color 360ms ease,
      transform 360ms ease,
      font-size 360ms ease;
    pointer-events: none;
    z-index: 11;
  }

  .shell-ball-uiverse-inputbox[data-multiline="true"] span {
    transition: none;
  }

  .shell-ball-uiverse-inputbox textarea:valid ~ span,
  .shell-ball-uiverse-inputbox textarea:focus ~ span,
  &[data-filled="true"] .shell-ball-uiverse-inputbox span {
    color: rgba(102, 138, 188, 0.92);
    transform: translateX(-10px) translateY(-14px);
    font-size: 0.75em;
  }

  .shell-ball-uiverse-inputbox i {
    position: absolute;
    left: 0;
    bottom: 0;
    width: 100%;
    height: 5px;
    background: linear-gradient(90deg, rgba(168, 194, 225, 0.82), rgba(255, 243, 230, 0.96));
    border: 1px solid rgba(255, 255, 255, 0.72);
    border-radius: 1rem;
    box-shadow:
      0 0 10px rgba(168, 194, 225, 0.18),
      0 8px 18px -18px rgba(122, 106, 79, 0.22);
    transform: none;
    transition:
      height 340ms ease,
      background 340ms ease,
      box-shadow 340ms ease,
      border-color 340ms ease;
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
    background: var(--shell-ball-input-paper);
    border-color: rgba(255, 255, 255, 0.84);
    box-shadow:
      0 16px 28px -22px rgba(122, 106, 79, 0.42),
      0 1px 0 rgba(255, 255, 255, 0.72) inset;
  }

  .shell-ball-uiverse-inputbox textarea:valid,
  .shell-ball-uiverse-inputbox textarea:focus,
  &[data-filled="true"] .shell-ball-uiverse-inputbox textarea {
    background: rgba(255, 253, 249, 0.98);
    border-color: rgba(171, 196, 229, 0.88);
    box-shadow:
      0 18px 30px -24px rgba(106, 145, 200, 0.24),
      0 12px 24px -22px rgba(122, 106, 79, 0.32),
      0 1px 0 rgba(255, 255, 255, 0.84) inset;
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
    background: linear-gradient(180deg, rgba(255, 253, 249, 0.9), rgba(244, 236, 225, 0.82));
    border: 1px solid rgba(255, 255, 255, 0.58);
    border-radius: 999px;
    box-shadow:
      0 12px 20px -18px rgba(122, 106, 79, 0.32),
      0 1px 0 rgba(255, 255, 255, 0.74) inset;
    color: rgba(90, 101, 113, 0.82);
    cursor: pointer;
    display: inline-flex;
    height: 1.88rem;
    justify-content: center;
    transition:
      transform 160ms ease,
      background 160ms ease,
      border-color 160ms ease,
      box-shadow 160ms ease,
      color 160ms ease,
      opacity 160ms ease;
    width: 1.88rem;
  }

  .shell-ball-uiverse-action:hover:not(:disabled) {
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(247, 239, 228, 0.88));
    border-color: rgba(106, 145, 200, 0.28);
    box-shadow:
      0 16px 26px -20px rgba(122, 106, 79, 0.36),
      0 1px 0 rgba(255, 255, 255, 0.82) inset;
    color: rgba(82, 114, 157, 0.92);
    transform: translateY(-1px);
  }

  .shell-ball-uiverse-action--send {
    background: linear-gradient(180deg, rgba(220, 234, 252, 0.96), rgba(178, 204, 234, 0.9));
    border-color: rgba(163, 196, 232, 0.72);
    color: rgba(72, 97, 134, 0.96);
  }

  .shell-ball-uiverse-action--send:hover:not(:disabled) {
    background: linear-gradient(180deg, rgba(230, 242, 255, 0.98), rgba(190, 214, 242, 0.94));
    border-color: rgba(106, 145, 200, 0.4);
    color: rgba(57, 84, 124, 0.98);
  }

  .shell-ball-uiverse-action:disabled {
    cursor: default;
    opacity: 0.52;
  }

  .shell-ball-uiverse-action-icon {
    height: 0.8rem;
    width: 0.8rem;
  }
`;
