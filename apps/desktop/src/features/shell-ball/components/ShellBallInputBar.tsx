import { useEffect, useRef, useState } from "react";
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

const SHELL_BALL_INPUT_PLACEHOLDER = "和它说点什么…";
const SHELL_BALL_INPUT_COLLAPSED_HEIGHT_PX = 40;
const SHELL_BALL_INPUT_MIN_HEIGHT_PX = 48;

/**
 * Renders the floating shell-ball hover input as one continuous capsule shell.
 * A translucent hover surface upgrades into the filled focus surface by
 * animating the inner paper layer from bottom to top without moving the outer
 * frame or changing the submit behavior.
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
  const appliedFocusTokenRef = useRef(0);
  const [focusWithin, setFocusWithin] = useState(false);
  const trimmedValue = value.trim();
  const hasDraft = trimmedValue !== "";
  const isHidden = mode === "hidden";
  const isInteractive = mode === "interactive";
  const isReadonly = mode === "readonly";
  const isVoice = mode === "voice";
  const buttonsDisabled = isHidden || isReadonly || isVoice;
  const submitDisabled = !isInteractive || (trimmedValue === "" && !hasPendingFiles);
  const settledSurface = isReadonly || focusWithin;
  const collapsedSurface = isInteractive && !settledSurface;

  useEffect(() => {
    const field = inputRef.current;
    if (field === null) {
      return;
    }

    field.closest<HTMLElement>(".shell-ball-uiverse-inputbox")?.style.setProperty(
      "--shell-ball-input-height",
      `${collapsedSurface ? SHELL_BALL_INPUT_COLLAPSED_HEIGHT_PX : SHELL_BALL_INPUT_MIN_HEIGHT_PX}px`,
    );
  }, [collapsedSurface]);

  useEffect(() => {
    if (inputRef.current === null) {
      return;
    }

    if (!isInteractive) {
      if (inputRef.current === document.activeElement) {
        inputRef.current.blur();
        setFocusWithin(false);
        onFocusChange(false);
      }
      return;
    }

    if (focusToken !== 0 && focusToken !== appliedFocusTokenRef.current) {
      appliedFocusTokenRef.current = focusToken;
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

    if (event.key !== "Enter" || event.shiftKey || submitDisabled) {
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
      data-can-attach={isInteractive ? "true" : "false"}
      data-filled={hasDraft ? "true" : "false"}
      data-hidden={hiddenState ? "true" : "false"}
      data-mode={mode}
      data-collapsed={collapsedSurface ? "true" : "false"}
      data-settled={settledSurface ? "true" : "false"}
      data-voice-preview={voicePreview ?? undefined}
    >
      <div className="shell-ball-uiverse-inputbox">
        <div aria-hidden="true" className="shell-ball-uiverse-fill" />
        <div className="shell-ball-uiverse-content">
          <input
            ref={inputRef}
            data-shell-ball-interactive="true"
            required
            type="text"
            value={value}
            onChange={handleChange}
            onCompositionStart={handleCompositionStart}
            onCompositionEnd={handleCompositionEnd}
            onKeyDown={handleKeyDown}
            onFocus={() => {
              setFocusWithin(true);
              onFocusChange(true);
            }}
            onBlur={() => {
              if (compositionActiveRef.current) {
                return;
              }

              setFocusWithin(false);
              onFocusChange(false);
            }}
            readOnly={isHidden || isReadonly || isVoice}
            tabIndex={isInteractive ? 0 : -1}
            aria-label="Shell-ball input"
            placeholder=""
          />
          <span aria-hidden="true" className="shell-ball-uiverse-placeholder">
            {SHELL_BALL_INPUT_PLACEHOLDER}
          </span>
          <div className="shell-ball-uiverse-actions">
            <button
              type="button"
              className="shell-ball-uiverse-action shell-ball-uiverse-action--attach"
              data-shell-ball-interactive="true"
              onMouseDown={(event) => {
                event.preventDefault();
              }}
              onClick={() => {
                onAttachFile();
                restoreTextareaFocus();
              }}
              disabled={buttonsDisabled}
              aria-label="Attach file"
            >
              <Paperclip className="shell-ball-uiverse-action-icon" />
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
              aria-label={isReadonly ? "Send disabled" : isVoice ? "Send unavailable during voice capture" : "Send request"}
            >
              <ArrowUp className="shell-ball-uiverse-action-icon" />
            </button>
          </div>
        </div>
      </div>
    </StyledInputBar>
  );
}

const StyledInputBar = styled.div`
  --shell-ball-input-expanded-width: 224px;
  --shell-ball-input-shell-surface: linear-gradient(180deg, rgba(255, 255, 255, 0.48), rgba(246, 244, 240, 0.3));
  --shell-ball-input-shell-line: rgba(173, 188, 210, 0.46);
  --shell-ball-input-shell-line-active: rgba(149, 188, 232, 0.9);
  --shell-ball-input-fill-surface: linear-gradient(180deg, rgba(255, 255, 252, 0.98), rgba(252, 247, 240, 0.96) 76%, rgba(248, 243, 234, 0.96));
  --shell-ball-input-fill-edge: rgba(255, 255, 255, 0.92);
  --shell-ball-input-ink: rgba(43, 57, 74, 0.96);
  --shell-ball-input-copy: rgba(93, 108, 128, 0.88);
  --shell-ball-input-copy-active: rgba(132, 146, 164, 0.72);
  --shell-ball-input-shell-shadow: 0 16px 32px -28px rgba(86, 100, 126, 0.24);
  align-items: center;
  background: transparent;
  border: 0;
  display: inline-flex;
  flex-direction: column;
  padding: 0;
  width: fit-content;

  &[data-hidden="true"] {
    display: none;
  }

  .shell-ball-uiverse-inputbox {
    --shell-ball-input-height: 48px;
    background: var(--shell-ball-input-shell-surface);
    backdrop-filter: blur(16px) saturate(118%);
    -webkit-backdrop-filter: blur(16px) saturate(118%);
    border: 1px solid var(--shell-ball-input-shell-line);
    border-radius: 22px;
    box-shadow:
      0 0 0 1px rgba(255, 255, 255, 0.72) inset,
      var(--shell-ball-input-shell-shadow),
      0 1px 0 rgba(255, 255, 255, 0.42) inset;
    isolation: isolate;
    overflow: hidden;
    position: relative;
    transition:
      background 220ms ease,
      border-color 220ms cubic-bezier(0.22, 1, 0.36, 1),
      box-shadow 220ms cubic-bezier(0.22, 1, 0.36, 1),
      height 220ms cubic-bezier(0.22, 1, 0.36, 1);
    height: var(--shell-ball-input-height);
    width: var(--shell-ball-input-expanded-width);
  }

  .shell-ball-uiverse-inputbox::before {
    background: radial-gradient(
      circle at center,
      color-mix(in srgb, var(--shell-ball-input-shell-line-active) 72%, white) 0%,
      transparent 72%
    );
    border-radius: 28px;
    content: "";
    filter: blur(14px);
    inset: -8px;
    opacity: 0;
    pointer-events: none;
    position: absolute;
    transform: scale(1.04);
    transition:
      opacity 220ms cubic-bezier(0.22, 1, 0.36, 1),
      transform 220ms cubic-bezier(0.22, 1, 0.36, 1);
    z-index: -1;
  }

  .shell-ball-uiverse-fill {
    background: var(--shell-ball-input-fill-surface);
    border-radius: 20px;
    bottom: 2px;
    box-shadow:
      0 1px 0 rgba(255, 255, 255, 0.78) inset,
      0 -12px 20px -20px rgba(181, 206, 235, 0.34) inset;
    left: 2px;
    opacity: 0.96;
    pointer-events: none;
    position: absolute;
    right: 2px;
    top: 2px;
    transform: scaleY(0);
    transform-origin: center bottom;
    transition:
      background 220ms ease,
      opacity 220ms ease,
      transform 220ms cubic-bezier(0.22, 1, 0.36, 1);
    z-index: 0;
  }

  .shell-ball-uiverse-fill::after {
    background: linear-gradient(180deg, var(--shell-ball-input-fill-edge), rgba(255, 255, 255, 0));
    border-radius: 999px;
    content: "";
    filter: blur(6px);
    height: 20px;
    left: 14px;
    opacity: 0;
    position: absolute;
    right: 14px;
    top: -7px;
    transform: translateY(10px) scaleX(0.88);
    transform-origin: center top;
    transition:
      opacity 180ms ease 70ms,
      transform 220ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .shell-ball-uiverse-content {
    position: relative;
    z-index: 1;
  }

  .shell-ball-uiverse-content input {
    background: transparent;
    border: 0;
    box-shadow: none;
    caret-color: rgba(102, 147, 203, 0.96);
    color: var(--shell-ball-input-ink);
    font-size: 0.94rem;
    font-weight: 500;
    letter-spacing: 0.02em;
    line-height: 1.45;
    min-height: var(--shell-ball-input-height);
    outline: none;
    padding: 13px 76px 13px 16px;
    transition:
      color 160ms ease,
      opacity 160ms ease;
    width: 100%;
    z-index: 1;
  }

  .shell-ball-uiverse-content input::placeholder {
    color: transparent;
  }

  .shell-ball-uiverse-placeholder {
    color: var(--shell-ball-input-copy);
    font-size: 0.94rem;
    font-weight: 500;
    left: 16px;
    letter-spacing: 0.02em;
    line-height: 1.45;
    max-width: calc(100% - 92px);
    opacity: 0.94;
    overflow: hidden;
    pointer-events: none;
    position: absolute;
    text-overflow: ellipsis;
    top: 13px;
    transition:
      color 160ms ease 110ms,
      opacity 160ms ease 110ms,
      transform 220ms cubic-bezier(0.22, 1, 0.36, 1),
      left 220ms cubic-bezier(0.22, 1, 0.36, 1),
      top 220ms cubic-bezier(0.22, 1, 0.36, 1),
      font-size 220ms cubic-bezier(0.22, 1, 0.36, 1),
      max-width 220ms cubic-bezier(0.22, 1, 0.36, 1),
      width 220ms cubic-bezier(0.22, 1, 0.36, 1);
    white-space: nowrap;
    z-index: 2;
  }

  .shell-ball-uiverse-actions {
    align-items: center;
    display: inline-flex;
    gap: 0.35rem;
    pointer-events: none;
    position: absolute;
    right: 10px;
    top: 50%;
    transform: translateY(-50%);
    z-index: 3;
  }

  &[data-collapsed="true"] .shell-ball-uiverse-inputbox {
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.28), rgba(245, 248, 252, 0.16));
    border-color: rgba(149, 188, 232, 0.56);
    box-shadow:
      0 8px 24px rgba(90, 110, 130, 0.08),
      inset 0 1px 1px rgba(255, 255, 255, 0.9),
      0 0 0 1px rgba(255, 255, 255, 0.72) inset;
  }

  &[data-collapsed="true"] .shell-ball-uiverse-content input {
    color: transparent;
    min-height: ${SHELL_BALL_INPUT_COLLAPSED_HEIGHT_PX}px;
    padding-bottom: 2px;
    padding-top: 2px;
    padding-right: 16px;
  }

  &[data-collapsed="true"] .shell-ball-uiverse-placeholder {
    font-size: 0.82rem;
    left: 14px;
    max-width: calc(100% - 28px);
    text-align: left;
    top: 50%;
    line-height: 1;
    transform: translateY(-50%);
    width: calc(100% - 28px);
  }

  &[data-collapsed="true"] .shell-ball-uiverse-actions {
    opacity: 0;
    transform: translateY(-50%) translateX(8px);
    transition:
      opacity 140ms ease,
      transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  &[data-collapsed="true"] .shell-ball-uiverse-action--send {
    opacity: 0;
    pointer-events: none;
    transform: translateX(10px) scale(0.88);
  }

  .shell-ball-uiverse-action {
    align-items: center;
    background: rgba(244, 248, 253, 0.48);
    border: 1px solid rgba(173, 188, 210, 0.4);
    border-radius: 999px;
    box-shadow:
      0 10px 18px -18px rgba(88, 104, 129, 0.22),
      0 1px 0 rgba(255, 255, 255, 0.62) inset;
    color: rgba(90, 108, 133, 0.78);
    cursor: pointer;
    display: inline-flex;
    height: 1.9rem;
    justify-content: center;
    pointer-events: auto;
    transition:
      transform 160ms ease,
      background 160ms ease,
      border-color 160ms ease,
      box-shadow 160ms ease,
      color 160ms ease,
      opacity 160ms ease;
    width: 1.9rem;
  }

  .shell-ball-uiverse-action:hover:not(:disabled) {
    background: rgba(248, 251, 255, 0.92);
    border-color: rgba(146, 180, 220, 0.56);
    box-shadow:
      0 14px 24px -20px rgba(89, 109, 140, 0.3),
      0 1px 0 rgba(255, 255, 255, 0.78) inset;
    color: rgba(80, 114, 162, 0.94);
    transform: translateY(-1px);
  }

  .shell-ball-uiverse-action--attach {
    opacity: 0;
    pointer-events: none;
    transform: translateX(6px) scale(0.88);
  }

  .shell-ball-uiverse-action--send {
    background: linear-gradient(180deg, rgba(220, 233, 249, 0.82), rgba(188, 210, 237, 0.86));
    border-color: rgba(164, 195, 232, 0.72);
    box-shadow:
      0 12px 20px -18px rgba(115, 147, 191, 0.34),
      0 1px 0 rgba(255, 255, 255, 0.7) inset;
    color: rgba(71, 98, 135, 0.92);
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

  &:focus-within .shell-ball-uiverse-inputbox,
  &[data-settled="true"] .shell-ball-uiverse-inputbox {
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.6), rgba(250, 247, 242, 0.44));
    border-color: rgba(186, 216, 255, 0.92);
    box-shadow:
      0 0 0 1px rgba(186, 216, 255, 0.92),
      0 18px 32px -24px rgba(109, 141, 185, 0.2),
      0 0 0 5px rgba(142, 181, 232, 0.12),
      0 1px 0 rgba(255, 255, 255, 0.72) inset;
  }

  &:focus-within .shell-ball-uiverse-inputbox::before,
  &[data-settled="true"] .shell-ball-uiverse-inputbox::before {
    opacity: 0.92;
    transform: scale(1.08);
  }

  &:focus-within .shell-ball-uiverse-fill,
  &[data-settled="true"] .shell-ball-uiverse-fill {
    background: #fffcf8;
    opacity: 1;
    transform: scaleY(1);
  }

  &:focus-within .shell-ball-uiverse-fill::after,
  &[data-settled="true"] .shell-ball-uiverse-fill::after {
    opacity: 0.78;
    transform: translateY(0) scaleX(1);
  }

  &:focus-within .shell-ball-uiverse-placeholder,
  &[data-settled="true"] .shell-ball-uiverse-placeholder {
    color: var(--shell-ball-input-copy-active);
    font-size: 0.94rem;
    left: 16px;
    max-width: calc(100% - 92px);
    opacity: 0.74;
    top: 13px;
    transform: translateY(-1px);
    width: auto;
  }

  &[data-filled="true"] .shell-ball-uiverse-placeholder {
    opacity: 0;
    transform: translateY(2px);
    transition-delay: 0ms;
  }

  &:focus-within .shell-ball-uiverse-content input,
  &[data-settled="true"] .shell-ball-uiverse-content input {
    color: var(--shell-ball-input-ink);
    padding-right: 76px;
  }

  &:focus-within .shell-ball-uiverse-actions,
  &[data-settled="true"] .shell-ball-uiverse-actions {
    opacity: 1;
    transform: translateY(-50%) translateX(0);
  }

  &[data-can-attach="true"]:focus-within .shell-ball-uiverse-action--attach,
  &[data-can-attach="true"][data-settled="true"] .shell-ball-uiverse-action--attach {
    opacity: 1;
    pointer-events: auto;
    transform: translateX(0) scale(1);
  }

  &:focus-within .shell-ball-uiverse-action--send,
  &[data-settled="true"] .shell-ball-uiverse-action--send {
    background: linear-gradient(180deg, rgba(226, 238, 253, 0.98), rgba(187, 212, 241, 0.94));
    border-color: rgba(149, 188, 232, 0.88);
    box-shadow:
      0 14px 24px -18px rgba(115, 147, 191, 0.36),
      0 1px 0 rgba(255, 255, 255, 0.78) inset;
    color: rgba(62, 89, 127, 0.98);
    opacity: 1;
    pointer-events: auto;
    transform: translateX(0) scale(1);
  }

  .shell-ball-uiverse-action-icon {
    height: 0.8rem;
    width: 0.8rem;
  }
`;
