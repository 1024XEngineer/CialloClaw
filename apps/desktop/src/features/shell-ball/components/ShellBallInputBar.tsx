import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ChangeEvent, CompositionEvent, KeyboardEvent, PointerEvent as ReactPointerEvent } from "react";
import { ArrowUp, Paperclip } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallInputBarMode } from "../shellBall.types";

type ShellBallInputManualSize = {
  width: number | null;
  height: number | null;
};

export const SHELL_BALL_INPUT_MAX_RESIZE_WIDTH_PX = 512;
export const SHELL_BALL_INPUT_MAX_RESIZE_HEIGHT_PX = 192;

const useShellBallInputLayoutEffect = typeof window === "undefined" ? useEffect : useLayoutEffect;

// clampShellBallInputResizeDimension keeps manual textarea resizing inside the
// bounded hover-input footprint so the shell-ball helper does not turn into a
// full chat editor or spill past the helper-window placement budget.
export function clampShellBallInputResizeDimension(value: number, min: number, max: number) {
  if (max <= min) {
    return Math.round(min);
  }

  return Math.round(Math.min(Math.max(value, min), max));
}

// resolveShellBallInputFieldHeight decides the visible textarea height after
// combining content-driven autosize with an optional manual resize override.
// Once the resolved height reaches the bounded maximum, the textarea should
// stop growing and rely on internal scrolling for additional content.
export function resolveShellBallInputFieldHeight(input: {
  contentHeight: number;
  manualHeight: number | null;
  minHeight: number;
  maxHeight: number;
}) {
  const preferredHeight = input.manualHeight ?? input.contentHeight;
  return clampShellBallInputResizeDimension(preferredHeight, input.minHeight, input.maxHeight);
}

// focusShellBallInputField restores keyboard focus without selecting the whole
// draft. The hover input should preserve the user's caret context when helper
// windows request focus during drag-drop or selected-text handoff.
export function focusShellBallInputField(field: Pick<HTMLTextAreaElement, "focus" | "setSelectionRange" | "value">) {
  field.focus();

  try {
    const cursorOffset = field.value.length;
    field.setSelectionRange(cursorOffset, cursorOffset);
  } catch {
    // Ignore selection-range errors from environments that do not expose a live selection API.
  }
}

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
  onCompositionStateChange?: (composing: boolean) => void;
  onTransientInputActivity?: () => void;
};

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
  onCompositionStateChange = () => {},
  onTransientInputActivity = () => {},
}: ShellBallInputBarProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const compositionActiveRef = useRef(false);
  const defaultFieldWidthRef = useRef(0);
  const resizeCleanupRef = useRef<(() => void) | null>(null);
  const [manualSize, setManualSize] = useState<ShellBallInputManualSize>({ width: null, height: null });
  const [resolvedFieldHeight, setResolvedFieldHeight] = useState<number | null>(null);
  const [contentOverflowing, setContentOverflowing] = useState(false);
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

    focusShellBallInputField(inputRef.current);
  }, [focusToken, isInteractive]);

  useShellBallInputLayoutEffect(() => {
    const field = inputRef.current;
    if (field === null) {
      return;
    }

    if (isHidden || isVoice) {
      if (resolvedFieldHeight !== null) {
        setResolvedFieldHeight(null);
      }
      if (contentOverflowing) {
        setContentOverflowing(false);
      }
      return;
    }

    if (manualSize.width === null) {
      defaultFieldWidthRef.current = field.getBoundingClientRect().width;
    }

    const computedStyle = window.getComputedStyle(field);
    const minHeight = parseFloat(computedStyle.minHeight) || field.getBoundingClientRect().height;
    const previousHeight = field.style.height;
    field.style.height = "0px";
    const contentHeight = field.scrollHeight;
    field.style.height = previousHeight;

    const nextHeight = resolveShellBallInputFieldHeight({
      contentHeight,
      manualHeight: manualSize.height,
      minHeight,
      maxHeight: SHELL_BALL_INPUT_MAX_RESIZE_HEIGHT_PX,
    });
    const nextOverflow = contentHeight > nextHeight + 1;

    if (resolvedFieldHeight !== nextHeight) {
      setResolvedFieldHeight(nextHeight);
    }

    if (contentOverflowing !== nextOverflow) {
      setContentOverflowing(nextOverflow);
    }
  }, [contentOverflowing, isHidden, isVoice, manualSize.height, manualSize.width, resolvedFieldHeight, value]);

  useEffect(() => {
    return () => {
      resizeCleanupRef.current?.();
    };
  }, []);

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

  const handleResizePointerDown = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    const field = inputRef.current;
    if (field === null || typeof window === "undefined") {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    const rect = field.getBoundingClientRect();
    const computedStyle = window.getComputedStyle(field);
    const minHeight = parseFloat(computedStyle.minHeight) || rect.height;
    const minWidth = defaultFieldWidthRef.current > 0 ? defaultFieldWidthRef.current : rect.width;
    const startWidth = manualSize.width ?? rect.width;
    const startHeight = manualSize.height ?? rect.height;
    const startX = event.clientX;
    const startY = event.clientY;

    resizeCleanupRef.current?.();

    const previousBodyCursor = document.body.style.cursor;
    const previousBodyUserSelect = document.body.style.userSelect;
    document.body.style.cursor = "nwse-resize";
    document.body.style.userSelect = "none";

    const cleanup = () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", cleanup);
      window.removeEventListener("pointercancel", cleanup);
      document.body.style.cursor = previousBodyCursor;
      document.body.style.userSelect = previousBodyUserSelect;
      resizeCleanupRef.current = null;
    };

    const handlePointerMove = (moveEvent: PointerEvent) => {
      const nextWidth = clampShellBallInputResizeDimension(
        startWidth + moveEvent.clientX - startX,
        minWidth,
        SHELL_BALL_INPUT_MAX_RESIZE_WIDTH_PX,
      );
      const nextHeight = clampShellBallInputResizeDimension(
        startHeight + moveEvent.clientY - startY,
        minHeight,
        SHELL_BALL_INPUT_MAX_RESIZE_HEIGHT_PX,
      );

      setManualSize((current) => {
        if (current.width === nextWidth && current.height === nextHeight) {
          return current;
        }

        return {
          width: nextWidth,
          height: nextHeight,
        };
      });
    };

    resizeCleanupRef.current = cleanup;
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", cleanup);
    window.addEventListener("pointercancel", cleanup);
  }, [manualSize.height, manualSize.width]);

  const textareaStyle: CSSProperties = {
    height: resolvedFieldHeight ?? undefined,
    overflowY: contentOverflowing ? "auto" : "hidden",
    width: manualSize.width ?? undefined,
  };

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
      <div className="shell-ball-input-bar__field-shell">
        <textarea
          ref={inputRef}
          className="shell-ball-input-bar__field"
          value={value}
          onChange={handleChange}
          onCompositionStart={handleCompositionStart}
          onCompositionEnd={handleCompositionEnd}
          onKeyDown={handleKeyDown}
          onFocus={() => onFocusChange(true)}
          onBlur={() => {
            // Let the window-level IME guard decide when a composing session really ended.
            if (compositionActiveRef.current) {
              return;
            }

            onFocusChange(false);
          }}
          readOnly={isHidden || isReadonly || isVoice}
          tabIndex={isHidden || isVoice ? -1 : 0}
          aria-label="Shell-ball input"
          placeholder={isVoice ? "Voice capture is active" : ""}
          rows={1}
          style={textareaStyle}
        />
        {isHidden || isVoice ? null : (
          <div
            aria-hidden="true"
            className="shell-ball-input-bar__resize-handle"
            data-shell-ball-input-resize-handle="true"
            onPointerDown={handleResizePointerDown}
          />
        )}
      </div>
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
