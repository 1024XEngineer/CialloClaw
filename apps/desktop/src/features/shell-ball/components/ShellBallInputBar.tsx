import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ChangeEvent, CompositionEvent, KeyboardEvent, PointerEvent as ReactPointerEvent } from "react";
import { ArrowUp, Paperclip } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallInputBarMode } from "../shellBall.types";
import {
  clampShellBallInputResizeDimension,
  focusShellBallInputField,
  measureShellBallInputContentWidth,
  resolveShellBallInputAutoWidth,
  resolveShellBallInputFieldHeight,
  resolveShellBallInputFieldWidth,
  resolveShellBallInputMaxHeight,
  resolveShellBallInputMaxWidth,
  SHELL_BALL_INPUT_MAX_VISIBLE_LINES,
} from "./shellBallInputBar.helpers";

type ShellBallInputManualSize = {
  width: number | null;
  height: number | null;
};

const useShellBallInputLayoutEffect = typeof window === "undefined" ? useEffect : useLayoutEffect;

function measureShellBallInputRestingWidth(field: HTMLTextAreaElement) {
  const previousInlineWidth = field.style.width;
  field.style.width = "";
  const restingWidth = field.getBoundingClientRect().width;
  field.style.width = previousInlineWidth;
  return restingWidth;
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
  onResizeStateChange?: (resizing: boolean) => void;
  onCompositionStateChange?: (composing: boolean) => void;
  onTransientInputActivity?: () => void;
};

const SHELL_BALL_INPUT_LABEL = "Message";

/**
 * Renders the floating shell-ball input bar while preserving the attachment,
 * resize, and submit controls that surround the draft field.
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
  onResizeStateChange = () => {},
  onCompositionStateChange = () => {},
  onTransientInputActivity = () => {},
}: ShellBallInputBarProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const compositionActiveRef = useRef(false);
  const resizeCleanupRef = useRef<(() => void) | null>(null);
  const [manualSize, setManualSize] = useState<ShellBallInputManualSize>({ width: null, height: null });
  const [resolvedFieldWidth, setResolvedFieldWidth] = useState<number | null>(null);
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
      if (resolvedFieldWidth !== null) {
        setResolvedFieldWidth(null);
      }
      if (resolvedFieldHeight !== null) {
        setResolvedFieldHeight(null);
      }
      if (contentOverflowing) {
        setContentOverflowing(false);
      }
      return;
    }

    const restingWidth = measureShellBallInputRestingWidth(field);
    const computedStyle = window.getComputedStyle(field);
    const minWidth = restingWidth;
    const maxWidth = resolveShellBallInputMaxWidth(minWidth);
    const minHeight = parseFloat(computedStyle.minHeight) || field.getBoundingClientRect().height;
    const paddingLeft = parseFloat(computedStyle.paddingLeft) || 0;
    const paddingRight = parseFloat(computedStyle.paddingRight) || 0;
    const maxHeight = resolveShellBallInputMaxHeight({
      lineHeight: parseFloat(computedStyle.lineHeight) || minHeight / SHELL_BALL_INPUT_MAX_VISIBLE_LINES,
      paddingTop: parseFloat(computedStyle.paddingTop) || 0,
      paddingBottom: parseFloat(computedStyle.paddingBottom) || 0,
      minHeight,
    });
    const font = computedStyle.font || `${computedStyle.fontStyle} ${computedStyle.fontWeight} ${computedStyle.fontSize} ${computedStyle.fontFamily}`;
    const autoWidth = resolveShellBallInputAutoWidth({
      contentWidth: measureShellBallInputContentWidth({
        value,
        font,
        letterSpacing: parseFloat(computedStyle.letterSpacing) || 0,
        paddingLeft,
        paddingRight,
      }),
      minWidth,
      maxWidth,
    });
    const nextWidth = resolveShellBallInputFieldWidth({
      autoWidth,
      manualWidth: manualSize.width,
      minWidth,
      maxWidth,
    });
    const previousWidth = field.style.width;
    const previousHeight = field.style.height;
    field.style.width = `${nextWidth}px`;
    field.style.height = "0px";
    const contentHeight = field.scrollHeight;
    field.style.width = previousWidth;
    field.style.height = previousHeight;

    const nextHeight = resolveShellBallInputFieldHeight({
      contentHeight,
      manualHeight: manualSize.height,
      minHeight,
      maxHeight,
    });
    const nextOverflow = contentHeight > nextHeight + 1;

    if (resolvedFieldWidth !== nextWidth) {
      setResolvedFieldWidth(nextWidth);
    }

    if (resolvedFieldHeight !== nextHeight) {
      setResolvedFieldHeight(nextHeight);
    }

    if (contentOverflowing !== nextOverflow) {
      setContentOverflowing(nextOverflow);
    }
  }, [contentOverflowing, isHidden, isVoice, manualSize.height, manualSize.width, resolvedFieldHeight, resolvedFieldWidth, value]);

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

    const handle = event.currentTarget;
    const pointerId = event.pointerId;

    const rect = field.getBoundingClientRect();
    const computedStyle = window.getComputedStyle(field);
    const restingWidth = measureShellBallInputRestingWidth(field);
    const minHeight = parseFloat(computedStyle.minHeight) || rect.height;
    const initialWidth = restingWidth;
    const minWidth = initialWidth;
    const maxWidth = resolveShellBallInputMaxWidth(initialWidth);
    const maxHeight = resolveShellBallInputMaxHeight({
      lineHeight: parseFloat(computedStyle.lineHeight) || minHeight / SHELL_BALL_INPUT_MAX_VISIBLE_LINES,
      paddingTop: parseFloat(computedStyle.paddingTop) || 0,
      paddingBottom: parseFloat(computedStyle.paddingBottom) || 0,
      minHeight,
    });
    const startWidth = manualSize.width ?? rect.width;
    const startHeight = manualSize.height ?? rect.height;
    const startX = event.clientX;
    const startY = event.clientY;

    resizeCleanupRef.current?.();

    const previousBodyCursor = document.body.style.cursor;
    const previousBodyUserSelect = document.body.style.userSelect;
    document.body.style.cursor = "nwse-resize";
    document.body.style.userSelect = "none";

    onResizeStateChange(true);

    try {
      handle.setPointerCapture(pointerId);
    } catch {
      // Ignore pointer-capture failures from environments that do not support captured dragging.
    }

    let cleanedUp = false;

    const cleanup = () => {
      if (cleanedUp) {
        return;
      }

      cleanedUp = true;

      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", cleanup);
      window.removeEventListener("pointercancel", cleanup);
      window.removeEventListener("blur", cleanup);
      handle.removeEventListener("lostpointercapture", cleanup);

      try {
        if (handle.hasPointerCapture(pointerId)) {
          handle.releasePointerCapture(pointerId);
        }
      } catch {
        // Ignore release failures when the browser already dropped pointer capture.
      }

      document.body.style.cursor = previousBodyCursor;
      document.body.style.userSelect = previousBodyUserSelect;
      resizeCleanupRef.current = null;
      onResizeStateChange(false);
    };

    const handlePointerMove = (moveEvent: PointerEvent) => {
      const nextWidth = clampShellBallInputResizeDimension(
        startWidth + moveEvent.clientX - startX,
        minWidth,
        maxWidth,
      );
      const nextHeight = clampShellBallInputResizeDimension(
        startHeight + moveEvent.clientY - startY,
        minHeight,
        maxHeight,
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
    window.addEventListener("blur", cleanup);
    handle.addEventListener("lostpointercapture", cleanup);
  }, [manualSize.height, manualSize.width, onResizeStateChange]);

  const textareaStyle: CSSProperties = {
    height: resolvedFieldHeight ?? undefined,
    overflowY: contentOverflowing ? "auto" : "hidden",
    width: resolvedFieldWidth ?? undefined,
  };

  const fieldShellStyle: CSSProperties = {
    height: resolvedFieldHeight ?? undefined,
    width: resolvedFieldWidth ?? undefined,
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
      <div
        className="shell-ball-input-bar__field-shell"
        data-filled={trimmedValue !== "" ? "true" : "false"}
        style={fieldShellStyle}
      >
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
        <span aria-hidden="true" className="shell-ball-input-bar__field-label">
          {SHELL_BALL_INPUT_LABEL}
        </span>
        <i aria-hidden="true" className="shell-ball-input-bar__field-line" />
        {!isInteractive ? null : (
          <div
            aria-hidden="true"
            className="shell-ball-input-bar__resize-handle"
            data-shell-ball-input-resize-handle="true"
            onPointerDown={handleResizePointerDown}
          />
        )}
      </div>
      <div className="shell-ball-input-bar__actions">
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
    </div>
  );
}
