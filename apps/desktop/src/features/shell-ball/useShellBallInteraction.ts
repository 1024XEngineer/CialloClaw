/**
 * Shell-ball interaction state owns the floating hover input, voice capture, and
 * lightweight submission gestures that sit on top of the task-centric backend.
 */
import type { AgentInputSubmitParams, AgentTaskStartParams, AgentTaskSteerParams, RequestMeta } from "@cialloclaw/protocol";
import { useLatest, useUnmount } from "ahooks";
import { useCallback, useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";
import { submitTextInput, createTextInputSubmitParams } from "../../services/agentInputService";
import { JsonRpcClientError } from "@/rpc/client";
import { steerTask } from "@/rpc/methods";
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
import { startTaskFromFiles } from "@/services/taskService";
import type { ShellBallInteractionEvent, ShellBallVisualState, ShellBallVoiceHintMode } from "./shellBall.types";
import { useShellBallStore } from "../../stores/shellBallStore";

type TimeoutHandle = ReturnType<typeof globalThis.setTimeout>;

type ShellBallInteractionController = ReturnType<typeof createShellBallInteractionController>;

type ShellBallDashboardOpenGesture = "single_click" | "double_click";

type ShellBallInteractionConsumedEvent =
  | "press_start"
  | "long_press_voice_entry"
  | "voice_flow_consumed"
  | "force_state_reset";

type ShellBallVoiceRecognitionStopReason = "none" | "finish" | "cancel" | "segment";

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
export type ShellBallInputSubmitResult = NonNullable<Awaited<ReturnType<typeof submitTextInput>>> & {
  delivery_result?: {
    type?: string;
    preview_text?: string | null;
    payload?: {
      task_id?: string | null;
    } | null;
  } | null;
};

export type ShellBallFinalizedSpeechPayload = {
  id: string;
  text: string;
  taskId: string | null;
};

export type ShellBallVoiceStatusPayload = {
  id: string;
  text: string;
  taskId: string | null;
};

type ShellBallVoiceTaskUpdateResult = {
  task: ShellBallInputSubmitResult["task"];
  bubble_message: ShellBallInputSubmitResult["bubble_message"];
  delivery_result?: ShellBallInputSubmitResult["delivery_result"] | null;
};

type ShellBallLockedVoiceSession = {
  activeTaskId: string | null;
  chain: Promise<void>;
  segmentSequence: number;
  sessionId: string;
  voiceSessionId: string;
};

type ShellBallLockedVoiceSegmentPlan = {
  id: string;
  session: ShellBallLockedVoiceSession;
  text: string;
};

type ShellBallPostSubmitReset = {
  nextInputValue: string;
  nextPendingFiles: string[];
  nextFocused: false;
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

function createShellBallLocalIdentifier(prefix: string) {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return `${prefix}_${globalThis.crypto.randomUUID()}`;
  }

  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function createShellBallLockedVoiceSession(): ShellBallLockedVoiceSession {
  return {
    activeTaskId: null,
    chain: Promise.resolve(),
    segmentSequence: 0,
    sessionId: createShellBallLocalIdentifier("shell_ball_voice_session"),
    voiceSessionId: createShellBallLocalIdentifier("shell_ball_locked_voice"),
  };
}

function isShellBallLockedVoiceSteerFallbackError(error: unknown) {
  if (!(error instanceof JsonRpcClientError)) {
    return false;
  }

  const rpcMessage = error.message;
  return (
    error.code === 1001001 ||
    error.code === 1001004 ||
    error.code === 1001005 ||
    rpcMessage.includes("TASK_NOT_FOUND") ||
    rpcMessage.includes("TASK_STATUS_INVALID") ||
    rpcMessage.includes("TASK_ALREADY_FINISHED") ||
    rpcMessage.includes("task not found") ||
    rpcMessage.includes("task status invalid")
  );
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
 * Builds the formal `agent.input.submit` payload used by shell-ball text and
 * voice submissions.
 *
 * @param input Trigger metadata together with the draft text to submit.
 * @returns The normalized RPC payload, or `null` when the draft is empty.
 */
export function createShellBallInputSubmitParams(input: {
  text: string;
  trigger: "voice_commit" | "hover_text_input";
  inputMode: "voice" | "text";
  sessionId?: AgentInputSubmitParams["session_id"];
  voiceMeta?: AgentInputSubmitParams["voice_meta"];
}): AgentInputSubmitParams | null {
  return createTextInputSubmitParams({
    text: input.text,
    sessionId: input.sessionId,
    source: "floating_ball",
    trigger: input.trigger,
    inputMode: input.inputMode,
    voiceMeta: input.voiceMeta,
    options: {
      confirm_required: false,
      preferred_delivery: "bubble",
    },
  });
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
  };
}

async function submitShellBallInput(input: {
  text: string;
  trigger: "voice_commit" | "hover_text_input";
  inputMode: "voice" | "text";
  sessionId?: AgentInputSubmitParams["session_id"];
  voiceMeta?: AgentInputSubmitParams["voice_meta"];
}): Promise<ShellBallInputSubmitResult | null> {
  return submitTextInput({
    text: input.text,
    sessionId: input.sessionId,
    source: "floating_ball",
    trigger: input.trigger,
    inputMode: input.inputMode,
    voiceMeta: input.voiceMeta,
    options: {
      confirm_required: false,
      preferred_delivery: "bubble",
    },
  });
}

async function startShellBallFileTask(input: {
  text: string;
  files: string[];
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
    nextFocused: false,
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
    nextFocused: false,
  };
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

  return input.controller.forceState(input.visualState, { regionActive: input.regionActive });
}

export function resolveShellBallVoiceRecognitionFinalState(input: {
  reason: Exclude<ShellBallVoiceRecognitionStopReason, "none">;
  transcript: string;
  baseDraft: string;
  startState: ShellBallVisualState;
  lockedSessionActive?: boolean;
}) {
  const normalizedTranscript = input.transcript.trim();
  const lockedSessionActive = input.lockedSessionActive ?? false;
  const nextVisualState =
    input.startState === "hover_input" || input.baseDraft.trim() !== "" ? ("hover_input" as const) : ("idle" as const);

  if (input.reason === "segment" && lockedSessionActive && normalizedTranscript !== "") {
    return {
      continueListening: true,
      finalizedSpeechText: normalizedTranscript,
      nextInputValue: input.baseDraft,
      nextVisualState: "voice_locked" as const,
    };
  }

  if (input.reason === "finish" && normalizedTranscript !== "") {
    return {
      continueListening: false,
      finalizedSpeechText: normalizedTranscript,
      nextInputValue: input.baseDraft,
      nextVisualState,
    };
  }

  return {
    continueListening: false,
    finalizedSpeechText: null,
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
  const [inputValue, setInputValue] = useState("");
  const [pendingFiles, setPendingFiles] = useState<string[]>([]);
  const [finalizedSpeechPayloads, setFinalizedSpeechPayloads] = useState<ShellBallFinalizedSpeechPayload[]>([]);
  const [voiceStatusPayloads, setVoiceStatusPayloads] = useState<ShellBallVoiceStatusPayload[]>([]);
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
  const recognitionRef = useRef<ShellBallSpeechRecognition | null>(null);
  const recognitionSessionIdRef = useRef(0);
  const recognitionStopReasonRef = useRef<ShellBallVoiceRecognitionStopReason>("none");
  const recognitionErrorRef = useRef<string | null>(null);
  const voiceBaseDraftRef = useRef("");
  const voiceTranscriptRef = useRef("");
  const voiceStartStateRef = useRef<ShellBallVisualState>(visualState);
  const lockedVoiceSessionRef = useRef<ShellBallLockedVoiceSession | null>(null);
  const finalizedSpeechPayload = finalizedSpeechPayloads[0] ?? null;
  const voiceStatusPayload = voiceStatusPayloads[0] ?? null;

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

  function syncVisualStateFromTaskStatus(status: Parameters<typeof getShellBallVisualStateForTaskStatus>[0], fallbackState: ShellBallVisualState) {
    controllerRef.current?.forceState(getShellBallVisualStateForTaskStatus(status, fallbackState), {
      regionActive: regionActiveRef.current,
    });
    syncVisualState();
  }

  function enqueueFinalizedSpeechPayload(payload: {
    id: string;
    text: string;
    taskId: string | null;
  }) {
    setFinalizedSpeechPayloads((currentPayloads) => [...currentPayloads, payload]);
  }

  function enqueueVoiceStatusPayload(payload: {
    id: string;
    text: string;
    taskId: string | null;
  }) {
    setVoiceStatusPayloads((currentPayloads) => [...currentPayloads, payload]);
  }

  function ensureLockedVoiceSession() {
    if (lockedVoiceSessionRef.current === null) {
      lockedVoiceSessionRef.current = createShellBallLockedVoiceSession();
    }

    return lockedVoiceSessionRef.current;
  }

  function clearLockedVoiceSession() {
    lockedVoiceSessionRef.current = null;
  }

  function createLockedVoiceSegmentPlan(text: string): ShellBallLockedVoiceSegmentPlan | null {
    const normalizedText = text.trim();
    if (normalizedText === "") {
      return null;
    }

    const session = ensureLockedVoiceSession();
    session.segmentSequence += 1;

    return {
      id: `${session.voiceSessionId}_segment_${session.segmentSequence}`,
      session,
      text: normalizedText,
    };
  }

  function enqueueLockedVoiceSessionWork<T>(plan: ShellBallLockedVoiceSegmentPlan, work: () => Promise<T>): Promise<T> {
    const run = plan.session.chain.catch((): void => undefined).then(work);
    plan.session.chain = run.then((): void => undefined, (): void => undefined);
    return run;
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

  async function steerLockedVoiceSegment(taskId: string, text: string): Promise<ShellBallVoiceTaskUpdateResult> {
    const params: AgentTaskSteerParams = {
      message: text,
      request_meta: createShellBallRequestMeta(),
      task_id: taskId,
    };
    const result = await steerTask(params);

    return {
      ...result,
      delivery_result: null,
    };
  }

  async function submitLockedVoiceSegment(plan: ShellBallLockedVoiceSegmentPlan): Promise<ShellBallVoiceTaskUpdateResult | null> {
    const currentTaskId = plan.session.activeTaskId;

    if (currentTaskId !== null) {
      try {
        const result = await steerLockedVoiceSegment(currentTaskId, plan.text);
        plan.session.activeTaskId = result.task.task_id;
        return result;
      } catch (error) {
        if (!isShellBallLockedVoiceSteerFallbackError(error)) {
          throw error;
        }
      }
    }

    const result = await submitShellBallInput({
      text: plan.text,
      trigger: "voice_commit",
      inputMode: "voice",
      sessionId: plan.session.sessionId,
      voiceMeta: {
        voice_session_id: plan.session.voiceSessionId,
        is_locked_session: true,
        asr_confidence: 1,
        segment_id: plan.id,
      },
    });

    if (result !== null) {
      plan.session.activeTaskId = result.task.task_id;
    }

    return result;
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
    setInputValue(composeShellBallSpeechDraft(voiceBaseDraftRef.current, transcript));
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
    setInputValue(committedDraft);
    return committedDraft;
  }

  async function finalizeVoiceRecognition(reason: Exclude<ShellBallVoiceRecognitionStopReason, "none">) {
    const lockedSession = lockedVoiceSessionRef.current;
    const resolution = resolveShellBallVoiceRecognitionFinalState({
      reason,
      transcript: voiceTranscriptRef.current,
      baseDraft: voiceBaseDraftRef.current,
      startState: voiceStartStateRef.current,
      lockedSessionActive: lockedSession !== null,
    });
    recognitionRef.current = null;
    recognitionStopReasonRef.current = "none";
    recognitionSessionIdRef.current += 1;

    setInputValue(resolution.nextInputValue);
    setCurrentVoiceHintMode("hidden");
    setCurrentVoicePreview(null);
    controllerRef.current?.forceState(resolution.nextVisualState, {
      regionActive: resolution.nextVisualState === "hover_input",
    });
    syncVisualState();
    voiceTranscriptRef.current = "";

    if (resolution.continueListening) {
      if (!startVoiceRecognition()) {
        clearLockedVoiceSession();
        controllerRef.current?.forceState("hover_input", {
          regionActive: regionActiveRef.current,
          hoverRetained: true,
        });
        syncVisualState();
      }
    }

    if (resolution.finalizedSpeechText === null) {
      if (reason === "finish" || reason === "cancel") {
        clearLockedVoiceSession();
      }
      return;
    }

    if (lockedSession !== null) {
      const segmentPlan = createLockedVoiceSegmentPlan(resolution.finalizedSpeechText);
      if (segmentPlan === null) {
        return;
      }

      const commitSegment = async () => {
        const result = await submitLockedVoiceSegment(segmentPlan);
        enqueueFinalizedSpeechPayload({
          id: segmentPlan.id,
          text: segmentPlan.text,
          taskId: result?.task.task_id ?? segmentPlan.session.activeTaskId ?? null,
        });
        return result;
      };

      if (resolution.continueListening) {
        void enqueueLockedVoiceSessionWork(segmentPlan, commitSegment).catch((error) => {
          console.warn("shell-ball locked voice segment submit failed", error);
          enqueueVoiceStatusPayload({
            id: `${segmentPlan.id}_failed`,
            text: "Voice segment failed to send. Still listening.",
            taskId: segmentPlan.session.activeTaskId,
          });
        });
        return;
      }

      // Ending a locked voice session should stop capture immediately and let
      // the last segment flush in the background without blocking the shell UI.
      clearLockedVoiceSession();
      void enqueueLockedVoiceSessionWork(segmentPlan, async () => {
        try {
          const result = await commitSegment();
          if (result !== null) {
            syncVisualStateFromTaskStatus(result.task.status, resolution.nextVisualState);
          }
        } catch (error) {
          console.warn("shell-ball locked voice finish submit failed", error);
          enqueueVoiceStatusPayload({
            id: `${segmentPlan.id}_failed`,
            text: "Voice segment failed to send after the call ended.",
            taskId: segmentPlan.session.activeTaskId,
          });
        }
      });
      return;
    }

    try {
      const result = await submitShellBallInput({
        text: resolution.finalizedSpeechText,
        trigger: "voice_commit",
        inputMode: "voice",
      });
      enqueueFinalizedSpeechPayload({
        id: createShellBallLocalIdentifier("shell_ball_voice_commit"),
        text: resolution.finalizedSpeechText,
        taskId: result?.task.task_id ?? null,
      });
      if (result !== null) {
        syncVisualStateFromTaskStatus(result.task.status, resolution.nextVisualState);
      }
    } catch (error) {
      console.warn("shell-ball voice submit failed", error);
      const restoredDraft = composeShellBallSpeechDraft(voiceBaseDraftRef.current, resolution.finalizedSpeechText);
      setInputValue(restoredDraft);
      inputFocusedRef.current = true;
      setInputFocused(true);
      controllerRef.current?.forceState("hover_input", {
        regionActive: regionActiveRef.current,
        hoverRetained: true,
      });
      syncVisualState();
    }
  }

  function acknowledgeFinalizedSpeechPayload() {
    setFinalizedSpeechPayloads((currentPayloads) => currentPayloads.slice(1));
  }

  function acknowledgeVoiceStatusPayload() {
    setVoiceStatusPayloads((currentPayloads) => currentPayloads.slice(1));
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
    if (reason === "finish" && (controllerRef.current?.getState() ?? visualState) === "voice_locked") {
      const nextVisualState =
        voiceStartStateRef.current === "hover_input" || voiceBaseDraftRef.current.trim() !== "" ? ("hover_input" as const) : ("idle" as const);
      inputFocusedRef.current = false;
      setInputFocused(false);
      setInputValue(voiceBaseDraftRef.current);
      setCurrentVoiceHintMode("hidden");
      setCurrentVoicePreview(null);
      controllerRef.current?.forceState(nextVisualState, {
        regionActive: nextVisualState === "hover_input",
      });
      syncVisualState();
    }
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

      const transcript = collectShellBallSpeechTranscript(event.results);
      syncVoiceDraft(transcript);

      const currentState = controllerRef.current?.getState() ?? visualState;
      const latestResult = event.results[event.results.length - 1];
      if (
        currentState === "voice_locked" &&
        voiceHintModeRef.current !== "finish" &&
        latestResult?.isFinal === true &&
        transcript.trim() !== ""
      ) {
        stopVoiceRecognition("segment");
      }
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

      if (stopReason === "finish" || stopReason === "cancel" || stopReason === "segment") {
        void finalizeVoiceRecognition(stopReason);
        return;
      }

      const currentState = controllerRef.current?.getState() ?? visualState;

      if (shouldResumeShellBallVoiceRecognitionAfterUnexpectedEnd(currentState)) {
        if (currentState === "voice_locked") {
          const lockedSession = lockedVoiceSessionRef.current;
          const recoveredTranscript = voiceTranscriptRef.current.trim();
          voiceTranscriptRef.current = "";
          setInputValue(voiceBaseDraftRef.current);

          if (lockedSession !== null && recoveredTranscript !== "") {
            const segmentPlan = createLockedVoiceSegmentPlan(recoveredTranscript);
            if (segmentPlan !== null) {
              void enqueueLockedVoiceSessionWork(segmentPlan, async () => {
                const result = await submitLockedVoiceSegment(segmentPlan);
                enqueueFinalizedSpeechPayload({
                  id: segmentPlan.id,
                  text: segmentPlan.text,
                  taskId: result?.task.task_id ?? segmentPlan.session.activeTaskId ?? null,
                });
                return result;
              }).catch((error) => {
                console.warn("shell-ball locked voice recovery submit failed", error);
                enqueueVoiceStatusPayload({
                  id: `${segmentPlan.id}_failed`,
                  text: "Voice segment failed to send. Still listening.",
                  taskId: segmentPlan.session.activeTaskId,
                });
              });
            }
          }

          if (shouldRetryShellBallVoiceRecognitionAfterUnexpectedEnd(recognitionError) && startVoiceRecognition()) {
            return;
          }

          setCurrentVoicePreview(null);
          controllerRef.current?.forceState(
            getShellBallVoiceRecognitionUnexpectedEndFallbackState({
              currentState,
              startState: voiceStartStateRef.current,
              committedDraft: voiceBaseDraftRef.current,
            }),
            { regionActive: regionActiveRef.current, hoverRetained: false },
          );
          syncVisualState();
          return;
        }

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

  function syncHoverRetention() {
    if (regionActiveRef.current) {
      return;
    }

    if (controllerRef.current?.getState() !== "hover_input") {
      return;
    }

    dispatch("pointer_leave_region", {
      regionActive: false,
      hoverRetained: getHoverRetained(),
    });
  }

  function handlePrimaryClick() {
    return;
  }

  function handleRegionEnter() {
    regionActiveRef.current = true;
    setRegionActive(true);
    dispatch("pointer_enter_hotspot", { regionActive: true, hoverRetained: false });
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
  }

  function handleInputHoverChange(active: boolean) {
    inputHoveredRef.current = active;

    if (active) {
      dispatch("pointer_enter_hotspot", {
        regionActive: regionActiveRef.current,
        hoverRetained: true,
      });
      return;
    }

    syncHoverRetention();
  }

  async function handleSubmitText() {
    const currentDraft = inputValue.trim();
    const reset = getShellBallPostSubmitReset({
      inputValue,
      pendingFiles,
    });
    if (reset === null) {
      return null;
    }

    try {
      const result =
        pendingFiles.length > 0
          ? await startShellBallFileTask({
              text: currentDraft,
              files: pendingFiles,
            })
          : await submitShellBallInput({
              text: currentDraft,
              trigger: "hover_text_input",
              inputMode: "text",
            });
      dispatch("submit_text");
      setInputValue(reset.nextInputValue);
      setPendingFiles(reset.nextPendingFiles);
      inputFocusedRef.current = reset.nextFocused;
      setInputFocused(reset.nextFocused);
      if (result !== null) {
        syncVisualStateFromTaskStatus(result.task.status, controllerRef.current?.getState() ?? visualState);
      }
      return result;
    } catch (error) {
      console.warn("shell-ball text submit failed", error);
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

    setPendingFiles((currentPaths) => mergeShellBallPendingFiles(currentPaths, normalizedPaths));
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

    setPendingFiles((currentPaths) => currentPaths.filter((currentPath) => currentPath !== normalizedPath));
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
        setCurrentVoiceHintMode("finish");
        setCurrentVoicePreview(null);
      }, SHELL_BALL_LOCKED_CANCEL_HOLD_MS);
      return;
    }

    if (currentState !== "idle" && currentState !== "hover_input") {
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
        setInputValue(voiceBaseDraftRef.current);
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

    if (currentState !== "voice_listening" && !(currentState === "voice_locked" && voiceHintModeRef.current === "finish")) {
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
        ensureLockedVoiceSession();
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

    if (controllerRef.current?.getState() === "voice_locked" && voiceHintModeRef.current === "finish") {
      consumeInteraction();
      const finalPreview = getVoicePreviewForPointer({
        pointerX: event.screenX,
        pointerY: event.screenY,
        fallbackPreview: voicePreviewRef.current,
      });

      if (finalPreview === "cancel") {
        stopVoiceRecognition("finish");
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

  function handlePressCancel(event: PointerEvent<HTMLButtonElement>) {
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
      syncHoverRetention();
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

    setInputValue(nextInputValue);
    handleInputFocusRequest();
  }

  function handleForceState(state: ShellBallVisualState) {
    clearLongPressTimer();
    disposeVoiceRecognition();
    clearLockedVoiceSession();
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
    clearLockedVoiceSession();
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
    setInputValue,
    finalizedSpeechPayload,
    acknowledgeFinalizedSpeechPayload,
    voiceStatusPayload,
    acknowledgeVoiceStatusPayload,
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
