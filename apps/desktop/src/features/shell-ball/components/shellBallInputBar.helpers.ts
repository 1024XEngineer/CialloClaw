export const SHELL_BALL_INPUT_MAX_RESIZE_WIDTH_FACTOR = 1.5;
export const SHELL_BALL_INPUT_MAX_VISIBLE_LINES = 3;

// clampShellBallInputResizeDimension keeps manual textarea resizing inside the
// bounded hover-input footprint so the shell-ball helper does not turn into a
// full chat editor or spill past the helper-window placement budget.
export function clampShellBallInputResizeDimension(value: number, min: number, max: number) {
  if (max <= min) {
    return Math.round(min);
  }

  return Math.round(Math.min(Math.max(value, min), max));
}

// resolveShellBallInputAutoWidth keeps width-driven autosize bounded between the
// resting helper width and the configured compact growth limit.
export function resolveShellBallInputAutoWidth(input: {
  contentWidth: number;
  minWidth: number;
  maxWidth: number;
}) {
  return clampShellBallInputResizeDimension(input.contentWidth, input.minWidth, input.maxWidth);
}

// resolveShellBallInputFieldWidth combines width-driven autosize with an
// optional manual resize override. Manual stretching should never disable
// content growth; the field keeps whichever width is larger inside the bounds.
export function resolveShellBallInputFieldWidth(input: {
  autoWidth: number;
  manualWidth: number | null;
  minWidth: number;
  maxWidth: number;
}) {
  const preferredWidth = Math.max(input.manualWidth ?? input.minWidth, input.autoWidth);
  return clampShellBallInputResizeDimension(preferredWidth, input.minWidth, input.maxWidth);
}

// resolveShellBallInputMaxWidth limits manual widening to one and a half times
// the initial hover-input width so the helper window stays compact.
export function resolveShellBallInputMaxWidth(initialWidth: number) {
  return Math.round(Math.max(initialWidth, initialWidth * SHELL_BALL_INPUT_MAX_RESIZE_WIDTH_FACTOR));
}

// resolveShellBallInputMaxHeight keeps the textarea near three visible lines.
// After that point the field should scroll internally rather than keep growing.
export function resolveShellBallInputMaxHeight(input: {
  lineHeight: number;
  paddingTop: number;
  paddingBottom: number;
  minHeight: number;
}) {
  const contentHeight = input.lineHeight * SHELL_BALL_INPUT_MAX_VISIBLE_LINES + input.paddingTop + input.paddingBottom;
  return Math.round(Math.max(input.minHeight, contentHeight));
}

// measureShellBallInputContentWidth estimates the widest visible line in the
// draft so autosize can expand the hover input horizontally before it starts to
// wrap into additional lines.
export function measureShellBallInputContentWidth(input: {
  value: string;
  font: string;
  letterSpacing: number;
  paddingLeft: number;
  paddingRight: number;
}) {
  if (typeof document === "undefined") {
    return Math.round(input.paddingLeft + input.paddingRight);
  }

  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (context === null) {
    return Math.round(input.paddingLeft + input.paddingRight);
  }

  context.font = input.font;
  const lines = input.value.length === 0 ? [""] : input.value.split(/\r?\n/u);
  const widestLine = lines.reduce((maxWidth, line) => {
    const measuredWidth = context.measureText(line).width;
    const spacingWidth = input.letterSpacing > 0 ? Math.max(0, line.length - 1) * input.letterSpacing : 0;
    return Math.max(maxWidth, measuredWidth + spacingWidth);
  }, 0);

  return Math.round(widestLine + input.paddingLeft + input.paddingRight + 2);
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
  const preferredHeight = Math.max(input.manualHeight ?? input.minHeight, input.contentHeight);
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
