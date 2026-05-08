/**
 * Shell-ball interaction state owns the floating hover input, voice capture, and
 * lightweight submission gestures that sit on top of the task-centric backend.
 */
import type { AgentTaskStartParams, RequestMeta } from "@cialloclaw/protocol";
import { useLatest, useUnmount } from "ahooks";
import { useCallback, useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";
import type { SubmitTextInputClientContext } from "../../services/agentInputService";
import {
  createShellBallInteractionController,
  getShellBallInputBarMode,
  SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX,
  getShellBallVoicePreviewForHintMode,
  getShellBallVisualStateForTaskStatus,
  SHELL_BALL_LOCKED_CANCEL_HOLD_MS,
  SHELL_BALL_LONG_PRESS_MS,
  resolveShellBallVoiceReleaseEvent,
  shouldRetainShellBallHoverInput,
  type ShellBallVoicePreview,
} from "./shellBall.interaction";
import {
  collectShellBallSpeechTranscript,
  composeShellBallSpeechDraft,
  getShellBallSpeechRecognitionConstructor,
  getShellBallSpeechRecognitionLanguage,
  type ShellBallSpeechRecognition,
} from "./shellBall.speech";
import { submitShellBallInput } from "./shellBallSubmit";
import { startTaskFromFiles } from "@/services/taskService";
import type { ShellBallInteractionEvent, ShellBallVisualState, ShellBallVoiceHintMode } from "./shellBall.types";
import { useShellBallStore } from "../../stores/shellBallStore";

export { createShellBallInputSubmitParams } from "./shellBallSubmit";

type TimeoutHandle = ReturnType<typeof globalThis.setTimeout>;

type ShellBallInteractionController = ReturnType<typeof createShellBallInteractionController>;

type ShellBallDashboardOpenGesture = "single_click" | "double_click";

type ShellBallInteractionConsumedEvent =
  | "press_start"
  | "long_press_voice_entry"
  | "voice_flow_consumed"
  | "force_state_reset";

type ShellBallVoiceRecognitionStopReason = "none" | "finish" | "cancel";

function canStartShellBallVoiceEntry(state: ShellBallVisualState | undefined) {
  return state !== "confirming_intent" && state !== "voice_listening" && state !== "voice_locked";
}

const SHELL_BALL_NON_RECOVERABLE_VOICE_ERRORS = new Set([
  "audio-capture",
  "language-not-supported",
  "not-allowed",
  "service-not-allowed",
]);

/**
 * Describes the normalized submission result shape reused by shell-ball follow-up
 * UI such as local bubbles and delivery previews.
 */
export type ShellBallInputSubmitResult = (
  | NonNullable<Awaited<ReturnType<typeof submitShellBallInput>>>
  | Awaited<ReturnType<typeof startTaskFromFiles>>
) & {
  clientContext?: SubmitTextInputClientContext;
  delivery_result?: {
    type?: string;
    preview_text?: string | null;
    payload?: {
      task_id?: string | null;
    } | null;
  } | null;
};

export type ShellBallPreparedTextSubmitDraft = {
  currentDraft: string;
  currentInputValue: string;
  currentPendingFiles: string[];
  submittedDraftRevision: number;
};

type ShellBallPostSubmitReset = {
  nextInputValue: string;
  nextPendingFiles: string[];
  nextFocused: true;
};

function createShellBallRequestMeta(): RequestMeta {
  const now = new Date().toISOString();
  const traceId = typeof globalThis.crypto?.randomUUID === "function"
    ? globalThis.crypto.randomUUID()
    : `trace_${Date.now()}_${Math.random().toString(16).slice(2)}`;

  return {
    trace_id: traceId,
    client_time: now,
  };
}

function normalizeShellBallPendingFiles(filePaths: string[]) {
  const seenPaths = new Set<string>();
  const normalizedPaths: string[] = [];

  for (const filePath of filePaths) {
    const trimmedPath = filePath.trim();
    if (trimmedPath === "" || seenPaths.has(trimmedPath)) {
      continue;
    }

    seenPaths.add(trimmedPath);
    normalizedPaths.push(trimmedPath);
  }

  return normalizedPaths;
}

function mergeShellBallPendingFiles(currentPaths: string[], incomingPaths: string[]) {
  return normalizeShellBallPendingFiles([...currentPaths, ...incomingPaths]);
}

/**
 * Normalizes dropped text before it is merged into the hover input draft.
 *
 * @param text Raw drag-and-drop text payload.
 * @returns Trimmed text with canonical newlines.
 */
export function normalizeShellBallDroppedText(text: string) {
  return text.replace(/\r\n/g, "\n").trim();
}

/**
 * Appends dropped text to the current hover-input draft while preserving the
 * shell-ball formatting rules for empty and multiline drafts.
 *
 * @param input Current draft and the dropped text payload.
 * @returns The next draft value that should be rendered.
 */
export function appendShellBallDroppedText(input: {
  currentValue: string;
  droppedText: string;
}) {
  const normalizedDroppedText = normalizeShellBallDroppedText(input.droppedText);

  if (normalizedDroppedText === "") {
    return input.currentValue;
  }

  if (input.currentValue.trim() === "") {
    return normalizedDroppedText;
  }

  if (/\s$/.test(input.currentValue)) {
    return `${input.currentValue}${normalizedDroppedText}`;
  }

  return `${input.currentValue}\n${normalizedDroppedText}`;
}

/**
 * Builds the formal `agent.task.start` payload used when shell-ball submission
 * includes file attachments.
 *
 * @param input Draft text plus pending file attachments.
 * @returns The normalized task-start payload, or `null` when no files exist.
 */
export function createShellBallTaskStartParams(input: {
  text: string;
  files: string[];
}): AgentTaskStartParams | null {
  const normalizedFiles = normalizeShellBallPendingFiles(input.files);
  if (normalizedFiles.length === 0) {
    return null;
  }

  const normalizedText = input.text.trim();

  return {
    request_meta: createShellBallRequestMeta(),
    source: "floating_ball",
    trigger: "file_drop",
    input: {
      type: "file",
      text: normalizedText === "" ? undefined : normalizedText,
      files: normalizedFiles,
    },
    delivery: {
      preferred: "bubble",
    },
    options: {
      // File drops only carry a caller preference here. The backend owns the
      // effective confirmation decision for bare files versus pending evidence.
      confirm_required: false,
    },
  };
}

async function startShellBallFileTask(input: {
  text: string;
  files: string[];
  sessionId?: string;
}): Promise<ShellBallInputSubmitResult | null> {
  const normalizedFiles = normalizeShellBallPendingFiles(input.files);

  if (normalizedFiles.length === 0) {
    return null;
  }

  return startTaskFromFiles(normalizedFiles, {
    delivery: {
      preferred: "bubble",
      fallback: "task_detail",
    },
    sessionId: input.sessionId,
    source: "floating_ball",
  }, input.text);
}

export function mapShellBallInteractionConsumedEventToFlag(event: ShellBallInteractionConsumedEvent) {
  switch (event) {
    case "press_start":
    case "force_state_reset":
      return false;
    case "long_press_voice_entry":
    case "voice_flow_consumed":
      return true;
  }
}

export function getShellBallDashboardOpenGesturePolicy(input: {
  gesture: ShellBallDashboardOpenGesture;
  state: ShellBallVisualState;
  interactionConsumed: boolean;
}) {
  if (input.gesture === "single_click") {
    return false;
  }

  const canOpenFromState = input.state === "idle" || input.state === "hover_input";
  return canOpenFromState && !input.interactionConsumed;
}

/**
 * Recomputes the active voice gesture preview from the final pointer position.
 *
 * @param input Pointer coordinates and the currently armed voice hint mode.
 * @returns The lock or cancel preview that should remain highlighted.
 */
export function getShellBallVoicePreviewFromEvent(input: {
  hintMode: Exclude<ShellBallVoiceHintMode, "hidden">;
  startX: number | null;
  startY: number | null;
  pointerX: number;
  pointerY: number;
  fallbackPreview: ShellBallVoicePreview;
}) {
  if (input.startX === null || input.startY === null) {
    return input.fallbackPreview;
  }

  return getShellBallVoicePreviewForHintMode({
    hintMode: input.hintMode,
    deltaX: input.pointerX - input.startX,
    deltaY: input.pointerY - input.startY,
  });
}

export function shouldKeepShellBallVoicePreviewOnRegionLeave(state: ShellBallVisualState) {
  return state === "voice_listening" || state === "voice_locked";
}

export function shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd(state: ShellBallVisualState) {
  return state === "voice_listening" || state === "voice_locked";
}

export function shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd(error: string | null) {
  return error === null || !SHELL_BALL_NON_RECOVERABLE_VOICE_ERRORS.has(error);
}

export function shouldLogShellBallSpeechRecognitionError(error: string) {
  return error !== "no-speech";
}

export function getShellBallVoiceRecognitionUnexpectedEndFallbackState(input: {
  currentState: ShellBallVisualState;
  startState: ShellBallVisualState;
  committedDraft: string;
}) {
  if (input.currentState === "voice_listening" || input.currentState === "voice_locked") {
    return "hover_input" as const;
  }

  return input.startState === "hover_input" || input.committedDraft.trim() !== "" ? ("hover_input" as const) : ("idle" as const);
}

export function getShellBallPostSubmitInputReset(inputValue: string) {
  if (inputValue.trim() === "") {
    return null;
  }

  return {
    nextInputValue: "",
    nextFocused: true,
  };
}

function getShellBallPostSubmitReset(input: {
  inputValue: string;
  pendingFiles: string[];
}): ShellBallPostSubmitReset | null {
  if (input.inputValue.trim() === "" && input.pendingFiles.length === 0) {
    return null;
  }

  return {
    nextInputValue: "",
    nextPendingFiles: [],
    nextFocused: true,
  };
}

export function shouldRestoreShellBallSubmitFailureDraft(input: {
  currentInputValue: string;
  currentPendingFiles: string[];
  currentDraftRevision: number;
  submittedDraftRevision: number;
}) {
  return (
    input.currentInputValue.trim() === "" &&
    input.currentPendingFiles.length === 0 &&
    input.currentDraftRevision === input.submittedDraftRevision
  );
}

export function getShellBallPressCancelEvent(state: ShellBallVisualState): Extract<ShellBallInteractionEvent, "voice_cancel"> | null {
  return state === "voice_listening" ? "voice_cancel" : null;
}

export function syncShellBallInteractionController(input: {
  controller: ShellBallInteractionController;
  visualState: ShellBallVisualState;
  regionActive: boolean;
}) {
  if (input.controller.getState() === input.visualState) {
    return input.visualState;
  }

  return input.controller.forceState(input.visualState, {
    regionActive: input.regionActive,
    scheduleProcessingReturn: false,
  });
}

export function resolveShellBallVoiceRecognitionFinalState(input: {
  reason: Exclude<ShellBallVoiceRecognitionStopReason, "none">;
  transcript: string;
  baseDraft: string;
  startState: ShellBallVisualState;
}) {
  const normalizedTranscript = input.transcript.trim();
  const nextVisualState =
    input.startState === "hover_input" || input.baseDraft.trim() !== "" ? ("hover_input" as const) : ("idle" as const);

  if (input.reason === "finish" && normalizedTranscript !== "") {
    return {
      finalizedSpeechPayload: normalizedTranscript,
      nextInputValue: input.baseDraft,
      nextVisualState,
    };
  }

  return {
    finalizedSpeechPayload: null,
    nextInputValue: input.baseDraft,
    nextVisualState,
  };
}

/**
 * Owns shell-ball hover input, file intake, and voice capture state while the
 * mascot stays docked as the near-field desktop entry point.
 *
 * @returns The shell-ball interaction view model and event handlers.
 */
export function useShellBallInteraction() {
  const visualState = useShellBallStore((state) => state.visualState);
  const setVisualState = useShellBallStore((state) => state.setVisualState);
  const [inputValue, setInputValueState] = useState("");
  const [pendingFiles, setPendingFilesState] = useState<string[]>([]);
  const [finalizedSpeechPayload, setFinalizedSpeechPayload] = useState<string | null>(null);
  const [regionActive, setRegionActive] = useState(false);
  const [inputFocused, setInputFocused] = useState(false);
  const [voicePreview, setVoicePreview] = useState<ShellBallVoicePreview>(null);
  const [voiceHintMode, setVoiceHintMode] = useState<ShellBallVoiceHintMode>("hidden");
  const [voiceHoldProgress, setVoiceHoldProgress] = useState(0);
  const [interactionConsumed, setInteractionConsumed] = useState(false);
  const regionActiveRef = useRef(false);
  const inputFocusedRef = useRef(false);
  const inputHoveredRef = useRef(false);
  const pressStartXRef = useRef<number | null>(null);
  const pressStartYRef = useRef<number | null>(null);
  const voicePreviewRef = useRef<ShellBallVoicePreview>(null);
  const voiceHintModeRef = useRef<ShellBallVoiceHintMode>("hidden");
  const longPressHandleRef = useRef<TimeoutHandle | null>(null);
  const longPressProgressHandleRef = useRef<number | null>(null);
  const longPressStartAtRef = useRef<number | null>(null);
  const setVisualStateRef = useLatest<typeof setVisualState>(setVisualState);
  const controllerRef = useRef<ShellBallInteractionController | null>(null);
  const inputValueRef = useLatest<string>(inputValue);
  const pendingFilesRef = useLatest<string[]>(pendingFiles);
  const draftRevisionRef = useRef(0);
  const recognitionRef = useRef<ShellBallSpeechRecognition | null>(null);
  const recognitionSessionIdRef = useRef(0);
  const recognitionStopReasonRef = useRef<ShellBallVoiceRecognitionStopReason>("none");
  const recognitionErrorRef = useRef<string | null>(null);
  const voiceBaseDraftRef = useRef("");
  const voiceTranscriptRef = useRef("");
  const voiceStartStateRef = useRef<ShellBallVisualState>(visualState);
  if (controllerRef.current === null) {
    controllerRef.current = createShellBallInteractionController({
      initialState: visualState,
      schedule: (callback, ms) =>
        globalThis.setTimeout(() => {
          callback();
          setVisualStateRef.current?.(controllerRef.current?.getState() ?? visualState);
        }, ms),
      cancel: (handle) => {
        globalThis.clearTimeout(handle as TimeoutHandle);
      },
    });
  }

  function syncVisualState() {
    setVisualState(controllerRef.current?.getState() ?? visualState);
  }

  function setTrackedInputValue(nextValue: string) {
    if (nextValue !== inputValueRef.current) {
      draftRevisionRef.current += 1;
    }
    setInputValueState(nextValue);
  }

  function syncVisualStateFromTaskStatus(status: Parameters<typeof getShellBallVisualStateForTaskStatus>[0], fallbackState: ShellBallVisualState) {
    controllerRef.current?.forceState(getShellBallVisualStateForTaskStatus(status, fallbackState), {
      regionActive: regionActiveRef.current,
      scheduleProcessingReturn: false,
    });
    syncVisualState();
  }

  const clearLongPressTimer = useCallback(() => {
    if (longPressHandleRef.current === null) {
      if (longPressProgressHandleRef.current !== null) {
        cancelAnimationFrame(longPressProgressHandleRef.current);
        longPressProgressHandleRef.current = null;
      }
      longPressStartAtRef.current = null;
      setVoiceHoldProgress(0);
      return;
    }

    globalThis.clearTimeout(longPressHandleRef.current);
    longPressHandleRef.current = null;

    if (longPressProgressHandleRef.current !== null) {
      cancelAnimationFrame(longPressProgressHandleRef.current);
      longPressProgressHandleRef.current = null;
    }
    longPressStartAtRef.current = null;
    setVoiceHoldProgress(0);
  }, []);

  function resetInteractionConsumed() {
    setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("press_start"));
  }

  function consumeInteraction() {
    setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("voice_flow_consumed"));
  }

  function setCurrentVoicePreview(preview: ShellBallVoicePreview) {
    voicePreviewRef.current = preview;
    setVoicePreview(preview);
  }

  const setCurrentVoiceHintMode = useCallback((mode: ShellBallVoiceHintMode) => {
    voiceHintModeRef.current = mode;
    setVoiceHintMode(mode);
  }, []);

  function getHoverRetained() {
    return shouldRetainShellBallHoverInput({
      regionActive: regionActiveRef.current,
      inputFocused: inputFocusedRef.current,
      inputHovered: inputHoveredRef.current,
      hasDraft: inputValue.trim() !== "" || pendingFiles.length > 0,
    });
  }

  function dispatch(
    event: ShellBallInteractionEvent,
    options: { regionActive?: boolean; hoverRetained?: boolean } = {},
  ) {
    controllerRef.current?.dispatch(event, {
      regionActive: options.regionActive ?? regionActiveRef.current,
      hoverRetained: options.hoverRetained ?? getHoverRetained(),
    });
    syncVisualState();
  }

  function syncVoiceDraft(transcript: string) {
    voiceTranscriptRef.current = transcript;
    setTrackedInputValue(composeShellBallSpeechDraft(voiceBaseDraftRef.current, transcript));
  }

  function getVoicePreviewForPointer(input: {
    pointerX: number;
    pointerY: number;
    fallbackPreview: ShellBallVoicePreview;
  }) {
    const hintMode = voiceHintModeRef.current;

    if (hintMode === "hidden") {
      return input.fallbackPreview;
    }

    return getShellBallVoicePreviewFromEvent({
      hintMode,
      startX: pressStartXRef.current,
      startY: pressStartYRef.current,
      pointerX: input.pointerX,
      pointerY: input.pointerY,
      fallbackPreview: input.fallbackPreview,
    });
  }

  function preserveUnexpectedVoiceTranscriptDraft() {
    const committedDraft = composeShellBallSpeechDraft(voiceBaseDraftRef.current, voiceTranscriptRef.current);
    voiceBaseDraftRef.current = committedDraft;
    voiceTranscriptRef.current = "";
    setTrackedInputValue(committedDraft);
    return committedDraft;
  }

  async function finalizeVoiceRecognition(reason: Exclude<ShellBallVoiceRecognitionStopReason, "none">) {
    const resolution = resolveShellBallVoiceRecognitionFinalState({
      reason,
      transcript: voiceTranscriptRef.current,
      baseDraft: voiceBaseDraftRef.current,
      startState: voiceStartStateRef.current,
    });
    recognitionRef.current = null;
    recognitionStopReasonRef.current = "none";
    recognitionSessionIdRef.current += 1;

    setTrackedInputValue(resolution.nextInputValue);
    setCurrentVoiceHintMode("hidden");
    setCurrentVoicePreview(null);
    controllerRef.current?.forceState(resolution.nextVisualState, {
      regionActive: resolution.nextVisualState === "hover_input",
    });
    syncVisualState();
    voiceTranscriptRef.current = "";

    if (resolution.finalizedSpeechPayload === null) {
      return;
    }

    setFinalizedSpeechPayload(resolution.finalizedSpeechPayload);
  }

  function acknowledgeFinalizedSpeechPayload() {
    setFinalizedSpeechPayload(null);
  }

  const disposeVoiceRecognition = useCallback(() => {
    recognitionSessionIdRef.current += 1;
    recognitionStopReasonRef.current = "none";
    recognitionErrorRef.current = null;
    voiceTranscriptRef.current = "";
    const recognition = recognitionRef.current;
    recognitionRef.current = null;

    if (recognition === null) {
      return;
    }

    recognition.onresult = null;
    recognition.onerror = null;
    recognition.onend = null;

    try {
      recognition.abort();
    } catch {}
  }, []);

  function stopVoiceRecognition(reason: Exclude<ShellBallVoiceRecognitionStopReason, "none">) {
    recognitionStopReasonRef.current = reason;
    const recognition = recognitionRef.current;

    if (recognition === null) {
      finalizeVoiceRecognition(reason);
      return;
    }

    try {
      if (reason === "cancel") {
        recognition.abort();
        return;
      }

      recognition.stop();
    } catch {
      finalizeVoiceRecognition(reason);
    }
  }

  function startVoiceRecognition() {
    const Recognition = getShellBallSpeechRecognitionConstructor();

    if (Recognition === null) {
      return false;
    }

    disposeVoiceRecognition();
    recognitionSessionIdRef.current += 1;
    const sessionId = recognitionSessionIdRef.current;
    const recognition = new Recognition();
    recognitionRef.current = recognition;
    recognitionStopReasonRef.current = "none";
    recognitionErrorRef.current = null;
    voiceTranscriptRef.current = "";
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = getShellBallSpeechRecognitionLanguage();
    recognition.maxAlternatives = 1;

    recognition.onresult = (event) => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      syncVoiceDraft(collectShellBallSpeechTranscript(event.results));
    };

    recognition.onerror = (event) => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      if (recognitionStopReasonRef.current !== "none") {
        return;
      }

      recognitionErrorRef.current = event.error;
      if (shouldLogShellBallSpeechRecognitionError(event.error)) {
        console.warn("shell-ball speech recognition error", event.error);
      }
    };

    recognition.onend = () => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      const stopReason = recognitionStopReasonRef.current;
      const recognitionError = recognitionErrorRef.current;
      recognitionErrorRef.current = null;

      if (stopReason === "finish" || stopReason === "cancel") {
        void finalizeVoiceRecognition(stopReason);
        return;
      }

      const currentState = controllerRef.current?.getState() ?? visualState;

      if (shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd(currentState)) {
        const committedDraft = preserveUnexpectedVoiceTranscriptDraft();

        if (shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd(recognitionError) && startVoiceRecognition()) {
          return;
        }

        setCurrentVoicePreview(null);
        controllerRef.current?.forceState(
          getShellBallVoiceRecognitionUnexpectedEndFallbackState({
            currentState,
            startState: voiceStartStateRef.current,
            committedDraft,
          }),
          { regionActive: regionActiveRef.current, hoverRetained: false },
        );
        syncVisualState();
        return;
      }

      void finalizeVoiceRecognition("cancel");
    };

    try {
      recognition.start();
      return true;
    } catch (error) {
      console.warn("shell-ball speech recognition start failed", error);
      recognitionRef.current = null;
      recognitionStopReasonRef.current = "none";
      recognitionSessionIdRef.current += 1;
      return false;
    }
  }

  function handlePrimaryClick() {
    return;
  }

  function handleRegionEnter() {
    regionActiveRef.current = true;
    setRegionActive(true);
    dispatch("pointer_enter_hotspot", {
      regionActive: true,
      hoverRetained: getHoverRetained(),
    });
    syncVisualState();
  }

  function handleRegionLeave() {
    regionActiveRef.current = false;
    setRegionActive(false);
    clearLongPressTimer();

    if (!shouldKeepShellBallVoicePreviewOnRegionLeave(controllerRef.current?.getState() ?? visualState)) {
      setCurrentVoicePreview(null);
    }

    dispatch("pointer_leave_region", {
      regionActive: false,
      hoverRetained: getHoverRetained(),
    });
    syncVisualState();
  }

  function handleInputHoverChange(active: boolean) {
    inputHoveredRef.current = active;
  }

  /**
   * Applies the shell-ball optimistic text-submit reset without consuming the
   * user's draft revision. Callers can restore the captured draft later when
   * the submit fails and no new draft has claimed the field yet.
   */
  function prepareTextSubmitDraft(): ShellBallPreparedTextSubmitDraft | null {
    const currentInputValue = inputValueRef.current ?? "";
    const currentPendingFiles = pendingFilesRef.current ?? [];
    const currentDraft = currentInputValue.trim();
    const reset = getShellBallPostSubmitReset({
      inputValue: currentInputValue,
      pendingFiles: currentPendingFiles,
    });
    if (reset === null) {
      return null;
    }

    const submittedDraftRevision = draftRevisionRef.current;
    dispatch("submit_text");
    setInputValueState(reset.nextInputValue);
    setPendingFilesState(reset.nextPendingFiles);
    inputFocusedRef.current = reset.nextFocused;
    setInputFocused(reset.nextFocused);

    return {
      currentDraft,
      currentInputValue,
      currentPendingFiles,
      submittedDraftRevision,
    };
  }

  /**
   * Restores an optimistically-cleared shell-ball draft only when the field is
   * still untouched and no newer draft has started after the failed submit.
   * Failed submits must also restore the interactive hover state immediately so
   * the inline bar does not stay readonly during the controller cooldown.
   */
  function restorePreparedTextSubmitDraft(preparedDraft: ShellBallPreparedTextSubmitDraft) {
    if (shouldRestoreShellBallSubmitFailureDraft({
      currentInputValue: inputValueRef.current ?? "",
      currentPendingFiles: pendingFilesRef.current ?? [],
      currentDraftRevision: draftRevisionRef.current,
      submittedDraftRevision: preparedDraft.submittedDraftRevision,
    })) {
      setInputValueState(preparedDraft.currentInputValue);
      setPendingFilesState(preparedDraft.currentPendingFiles);
    }
    inputFocusedRef.current = true;
    setInputFocused(true);
    controllerRef.current?.forceState("hover_input", {
      regionActive: regionActiveRef.current,
      hoverRetained: true,
    });
    syncVisualState();
  }

  async function handleSubmitText() {
    const preparedDraft = prepareTextSubmitDraft();
    if (preparedDraft === null) {
      return null;
    }

    try {
      const result =
        preparedDraft.currentPendingFiles.length > 0
          ? await startShellBallFileTask({
              text: preparedDraft.currentDraft,
              files: preparedDraft.currentPendingFiles,
            })
          : await submitShellBallInput({
              text: preparedDraft.currentDraft,
              trigger: "hover_text_input",
              inputMode: "text",
            });
      if (result?.task) {
        syncVisualStateFromTaskStatus(result.task.status, controllerRef.current?.getState() ?? visualState);
      }
      return result;
    } catch (error) {
      console.warn("shell-ball text submit failed", error);
      restorePreparedTextSubmitDraft(preparedDraft);
      throw error;
    }
  }

  /**
   * Voice capture reuses the same formal `agent.input.submit` path as the hover
   * input, but the coordinator owns the shell-ball bubble timeline. This helper
   * keeps RPC submission and visual-state recovery inside interaction state
   * while allowing the coordinator to render the corresponding task bubbles.
   */
  async function handleSubmitVoiceText(text: string) {
    const normalizedText = text.trim();

    if (normalizedText === "") {
      return null;
    }

    try {
      const result = await submitShellBallInput({
        text: normalizedText,
        trigger: "voice_commit",
        inputMode: "voice",
      });

      if (result?.task) {
        syncVisualStateFromTaskStatus(result.task.status, controllerRef.current?.getState() ?? visualState);
      }

      return result;
    } catch (error) {
      console.warn("shell-ball voice submit failed", error);
      const restoredDraft = composeShellBallSpeechDraft(voiceBaseDraftRef.current, normalizedText);
      setInputValueState(restoredDraft);
      inputFocusedRef.current = true;
      setInputFocused(true);
      controllerRef.current?.forceState("hover_input", {
        regionActive: regionActiveRef.current,
        hoverRetained: true,
      });
      syncVisualState();
      throw error;
    }
  }

  function handleAttachFile() {
    dispatch("attach_file");
  }

  function handleDroppedFiles(paths: string[]) {
    const normalizedPaths = normalizeShellBallPendingFiles(paths);
    if (normalizedPaths.length === 0) {
      return;
    }

    draftRevisionRef.current += 1;
    setPendingFilesState((currentPaths) => mergeShellBallPendingFiles(currentPaths, normalizedPaths));
    inputFocusedRef.current = true;
    setInputFocused(true);
    controllerRef.current?.forceState("hover_input", {
      regionActive: regionActiveRef.current,
      hoverRetained: true,
    });
    syncVisualState();
  }

  function handleRemovePendingFile(path: string) {
    const normalizedPath = path.trim();
    if (normalizedPath === "") {
      return;
    }

    draftRevisionRef.current += 1;
    setPendingFilesState((currentPaths) => currentPaths.filter((currentPath) => currentPath !== normalizedPath));
  }

  function handlePressStart(event: PointerEvent<HTMLButtonElement>) {
    regionActiveRef.current = true;
    setRegionActive(true);
    resetInteractionConsumed();
    pressStartXRef.current = event.screenX;
    pressStartYRef.current = event.screenY;
    setCurrentVoicePreview(null);
    clearLongPressTimer();

    const currentState = controllerRef.current?.getState();
    if (currentState === "voice_locked") {
      longPressStartAtRef.current = performance.now();
      const tickProgress = () => {
        if (longPressStartAtRef.current === null) {
          return;
        }

        const elapsed = performance.now() - longPressStartAtRef.current;
        setVoiceHoldProgress(Math.min(elapsed / SHELL_BALL_LOCKED_CANCEL_HOLD_MS, 1));
        longPressProgressHandleRef.current = requestAnimationFrame(tickProgress);
      };
      longPressProgressHandleRef.current = requestAnimationFrame(tickProgress);

      longPressHandleRef.current = globalThis.setTimeout(() => {
        longPressHandleRef.current = null;
        if (longPressProgressHandleRef.current !== null) {
          cancelAnimationFrame(longPressProgressHandleRef.current);
          longPressProgressHandleRef.current = null;
        }
        longPressStartAtRef.current = null;
        setVoiceHoldProgress(0);
        setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("long_press_voice_entry"));
        setCurrentVoiceHintMode("cancel");
        setCurrentVoicePreview(null);
      }, SHELL_BALL_LOCKED_CANCEL_HOLD_MS);
      return;
    }

    if (!canStartShellBallVoiceEntry(currentState)) {
      return;
    }

    inputFocusedRef.current = false;
    setInputFocused(false);

    longPressStartAtRef.current = performance.now();
    const tickProgress = () => {
      if (longPressStartAtRef.current === null) {
        return;
      }

      const elapsed = performance.now() - longPressStartAtRef.current;
      setVoiceHoldProgress(Math.min(elapsed / SHELL_BALL_LONG_PRESS_MS, 1));
      longPressProgressHandleRef.current = requestAnimationFrame(tickProgress);
    };
    longPressProgressHandleRef.current = requestAnimationFrame(tickProgress);

    longPressHandleRef.current = globalThis.setTimeout(() => {
      longPressHandleRef.current = null;
      voiceStartStateRef.current = controllerRef.current?.getState() ?? visualState;
      voiceBaseDraftRef.current = inputValueRef.current ?? "";
      if (longPressProgressHandleRef.current !== null) {
        cancelAnimationFrame(longPressProgressHandleRef.current);
        longPressProgressHandleRef.current = null;
      }
      longPressStartAtRef.current = null;
      setVoiceHoldProgress(0);
      setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("long_press_voice_entry"));
      setCurrentVoiceHintMode("lock");
      dispatch("press_start");

      if (!startVoiceRecognition()) {
        setInputValueState(voiceBaseDraftRef.current);
        setCurrentVoiceHintMode("hidden");
        controllerRef.current?.forceState(
          voiceStartStateRef.current === "hover_input" || voiceBaseDraftRef.current.trim() !== "" ? "hover_input" : "idle",
          { regionActive: regionActiveRef.current },
        );
        syncVisualState();
      }
    }, SHELL_BALL_LONG_PRESS_MS);
  }

  function handlePressMove(event: PointerEvent<HTMLButtonElement>) {
    if (pressStartXRef.current === null || pressStartYRef.current === null) {
      return;
    }

    const currentState = controllerRef.current?.getState();
    if ((currentState === "idle" || currentState === "hover_input") && longPressHandleRef.current !== null) {
      const driftDistance = Math.hypot(event.screenX - pressStartXRef.current, event.screenY - pressStartYRef.current);

      if (driftDistance > SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX) {
        clearLongPressTimer();
      }

      return;
    }

    if (currentState !== "voice_listening" && !(currentState === "voice_locked" && voiceHintModeRef.current === "cancel")) {
      return;
    }

    setCurrentVoicePreview(getVoicePreviewForPointer({
      pointerX: event.screenX,
      pointerY: event.screenY,
      fallbackPreview: voicePreviewRef.current,
    }));
  }

  function handlePressEnd(event: PointerEvent<HTMLButtonElement>) {
    clearLongPressTimer();

    if (controllerRef.current?.getState() === "voice_listening") {
      consumeInteraction();
      const finalPreview = getVoicePreviewForPointer({
        pointerX: event.screenX,
        pointerY: event.screenY,
        fallbackPreview: voicePreviewRef.current,
      });

      if (finalPreview === "lock") {
        dispatch("voice_lock");
        pressStartXRef.current = null;
        pressStartYRef.current = null;
        setCurrentVoiceHintMode("hidden");
        setCurrentVoicePreview(null);
        return true;
      }

      setCurrentVoiceHintMode("hidden");
      stopVoiceRecognition("finish");
      dispatch(resolveShellBallVoiceReleaseEvent(finalPreview));
      inputFocusedRef.current = false;
      setInputFocused(false);
      pressStartXRef.current = null;
      pressStartYRef.current = null;
      setCurrentVoicePreview(null);
      return true;
    }

    if (controllerRef.current?.getState() === "voice_locked" && voiceHintModeRef.current === "cancel") {
      consumeInteraction();
      const finalPreview = getVoicePreviewForPointer({
        pointerX: event.screenX,
        pointerY: event.screenY,
        fallbackPreview: voicePreviewRef.current,
      });

      if (finalPreview === "cancel") {
        stopVoiceRecognition("cancel");
      }

      setCurrentVoiceHintMode("hidden");
      pressStartXRef.current = null;
      pressStartYRef.current = null;
      setCurrentVoicePreview(null);
      return true;
    }

    pressStartXRef.current = null;
    pressStartYRef.current = null;
    setCurrentVoicePreview(null);
    return false;
  }

  function handlePressCancel(_event: PointerEvent<HTMLButtonElement>) {
    clearLongPressTimer();

    const cancelEvent = getShellBallPressCancelEvent(controllerRef.current?.getState() ?? visualState);
    pressStartXRef.current = null;
    pressStartYRef.current = null;
    inputFocusedRef.current = false;
    setInputFocused(false);
    setCurrentVoiceHintMode("hidden");
    setCurrentVoicePreview(null);

    if (cancelEvent !== null) {
      stopVoiceRecognition("cancel");
      consumeInteraction();
      dispatch(cancelEvent);
    }
  }

  function handleInputFocusChange(focused: boolean) {
    inputFocusedRef.current = focused;
    setInputFocused(focused);
    if (focused) {
      controllerRef.current?.forceState("hover_input", {
        regionActive: regionActiveRef.current,
        hoverRetained: true,
      });
      syncVisualState();
      return;
    }

    if (!focused) {
      const currentState = controllerRef.current?.getState() ?? visualState;

      // Blur should fully retire the higher-level input-active state so later
      // mascot gestures do not inherit a stale input hover relationship. Once
      // formal task states take over, blur must not clobber them back to idle.
      inputHoveredRef.current = false;

      if (currentState === "hover_input") {
        controllerRef.current?.forceState("idle", {
          regionActive: false,
          hoverRetained: false,
        });
        syncVisualState();
      }
    }
  }

  function handleInputFocusRequest() {
    inputFocusedRef.current = true;
    setInputFocused(true);
    controllerRef.current?.forceState("hover_input", {
      regionActive: regionActiveRef.current,
      hoverRetained: true,
    });
    syncVisualState();
  }

  function handleDroppedText(text: string) {
    const nextInputValue = appendShellBallDroppedText({
      currentValue: inputValueRef.current ?? "",
      droppedText: text,
    });

    if (nextInputValue === (inputValueRef.current ?? "")) {
      return;
    }

    setTrackedInputValue(nextInputValue);
    handleInputFocusRequest();
  }

  function handleForceState(state: ShellBallVisualState) {
    clearLongPressTimer();
    disposeVoiceRecognition();
    setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("force_state_reset"));
    pressStartXRef.current = null;
    pressStartYRef.current = null;
    inputFocusedRef.current = false;
    setInputFocused(false);
    setCurrentVoiceHintMode("hidden");
    setCurrentVoicePreview(null);
    controllerRef.current?.forceState(state, { regionActive: regionActiveRef.current });
    syncVisualState();
  }

  useEffect(() => {
    if (getShellBallInputBarMode(visualState) !== "hidden") {
      return;
    }

    inputHoveredRef.current = false;

    if (!inputFocusedRef.current) {
      return;
    }

    // Hidden input modes should retire any stale textarea-focus bookkeeping so
    // follow-up hover transitions do not inherit a no-longer-rendered field.
    inputFocusedRef.current = false;
    setInputFocused(false);
  }, [visualState]);

  useEffect(() => {
    if (controllerRef.current === null) {
      return;
    }

    syncShellBallInteractionController({
      controller: controllerRef.current,
      visualState,
      regionActive: regionActiveRef.current,
    });
  }, [visualState]);

  useEffect(() => {
    if (visualState === "voice_listening" || visualState === "voice_locked") {
      return;
    }

    if (voiceHintModeRef.current !== "hidden") {
      setCurrentVoiceHintMode("hidden");
    }
  }, [setCurrentVoiceHintMode, visualState]);

  useUnmount(() => {
    clearLongPressTimer();
    disposeVoiceRecognition();
    pressStartXRef.current = null;
    pressStartYRef.current = null;
    voicePreviewRef.current = null;
    voiceHintModeRef.current = "hidden";
    controllerRef.current?.dispose();
  });

  return {
    visualState,
    inputValue,
    pendingFiles,
    setInputValue: setTrackedInputValue,
    prepareTextSubmitDraft,
    restorePreparedTextSubmitDraft,
    finalizedSpeechPayload,
    acknowledgeFinalizedSpeechPayload,
    regionActive,
    voicePreview,
    voiceHintMode,
    voiceHoldProgress,
    inputFocused,
    inputBarMode: getShellBallInputBarMode(visualState),
    interactionConsumed,
    shouldOpenDashboardFromDoubleClick: getShellBallDashboardOpenGesturePolicy({
      gesture: "double_click",
      state: visualState,
      interactionConsumed,
    }),
    handlePrimaryClick,
    handleRegionEnter,
    handleRegionLeave,
    handleSubmitText,
    handleSubmitVoiceText,
    handleAttachFile,
    handleDroppedFiles,
    handleRemovePendingFile,
    handleDroppedText,
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handlePressCancel,
    handleInputHoverChange,
    handleInputFocusChange,
    handleInputFocusRequest,
    handleForceState,
  };
}
