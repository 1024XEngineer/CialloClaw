import type { AgentInputSubmitParams, RequestMeta } from "@cialloclaw/protocol";
import { useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";
import {
  createShellBallInteractionController,
  getShellBallInputBarMode,
  getShellBallVoicePreview,
  SHELL_BALL_LONG_PRESS_MS,
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
import type { ShellBallInteractionEvent, ShellBallVisualState } from "./shellBall.types";
import { useShellBallStore } from "../../stores/shellBallStore";

type TimeoutHandle = ReturnType<typeof globalThis.setTimeout>;

type ShellBallInteractionController = ReturnType<typeof createShellBallInteractionController>;

type ShellBallDashboardOpenGesture = "single_click" | "double_click";

type ShellBallInteractionConsumedEvent =
  | "press_start"
  | "long_press_voice_entry"
  | "voice_flow_consumed"
  | "force_state_reset";

type ShellBallVoiceRecognitionStopReason = "none" | "finish" | "cancel";

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

export function createShellBallInputSubmitParams(input: {
  text: string;
  trigger: "voice_commit" | "hover_text_input";
  inputMode: "voice" | "text";
}): AgentInputSubmitParams | null {
  const normalizedText = input.text.trim();

  if (normalizedText === "") {
    return null;
  }

  const requestMeta = createShellBallRequestMeta();

  return {
    request_meta: requestMeta,
    source: "floating_ball",
    trigger: input.trigger,
    input: {
      type: "text",
      text: normalizedText,
      input_mode: input.inputMode,
    },
    context: {
      files: [],
    },
  };
}

async function submitShellBallInput(input: {
  text: string;
  trigger: "voice_commit" | "hover_text_input";
  inputMode: "voice" | "text";
}) {
  const params = createShellBallInputSubmitParams(input);

  if (params === null) {
    return null;
  }

  const importRpcMethods = new Function("return import('../../rpc/methods')") as () => Promise<{
    submitInput: (request: AgentInputSubmitParams) => Promise<unknown>;
  }>;
  const rpcMethods = await importRpcMethods();
  return rpcMethods.submitInput(params);
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

export function getShellBallVoicePreviewFromEvent(input: {
  startX: number | null;
  startY: number | null;
  clientX: number;
  clientY: number;
  fallbackPreview: ShellBallVoicePreview;
}) {
  if (input.startX === null || input.startY === null) {
    return input.fallbackPreview;
  }

  return getShellBallVoicePreview({
    deltaX: input.clientX - input.startX,
    deltaY: input.clientY - input.startY,
  });
}

export function shouldKeepShellBallVoicePreviewOnRegionLeave(state: ShellBallVisualState) {
  return state === "voice_listening";
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

export function useShellBallInteraction() {
  const visualState = useShellBallStore((state) => state.visualState);
  const setVisualState = useShellBallStore((state) => state.setVisualState);
  const [inputValue, setInputValue] = useState("");
  const [finalizedSpeechPayload, setFinalizedSpeechPayload] = useState<string | null>(null);
  const [inputFocused, setInputFocused] = useState(false);
  const [voicePreview, setVoicePreview] = useState<ShellBallVoicePreview>(null);
  const [voiceHoldProgress, setVoiceHoldProgress] = useState(0);
  const [interactionConsumed, setInteractionConsumed] = useState(false);
  const regionActiveRef = useRef(false);
  const inputFocusedRef = useRef(false);
  const pressStartXRef = useRef<number | null>(null);
  const pressStartYRef = useRef<number | null>(null);
  const voicePreviewRef = useRef<ShellBallVoicePreview>(null);
  const longPressHandleRef = useRef<TimeoutHandle | null>(null);
  const longPressProgressHandleRef = useRef<number | null>(null);
  const longPressStartAtRef = useRef<number | null>(null);
  const setVisualStateRef = useRef(setVisualState);
  const controllerRef = useRef<ShellBallInteractionController | null>(null);
  const inputValueRef = useRef(inputValue);
  const recognitionRef = useRef<ShellBallSpeechRecognition | null>(null);
  const recognitionSessionIdRef = useRef(0);
  const recognitionStopReasonRef = useRef<ShellBallVoiceRecognitionStopReason>("none");
  const voiceBaseDraftRef = useRef("");
  const voiceTranscriptRef = useRef("");
  const voiceStartStateRef = useRef<ShellBallVisualState>(visualState);

  setVisualStateRef.current = setVisualState;
  inputValueRef.current = inputValue;

  if (controllerRef.current === null) {
    controllerRef.current = createShellBallInteractionController({
      initialState: visualState,
      schedule: (callback, ms) =>
        globalThis.setTimeout(() => {
          callback();
          setVisualStateRef.current(controllerRef.current?.getState() ?? visualState);
        }, ms),
      cancel: (handle) => {
        globalThis.clearTimeout(handle as TimeoutHandle);
      },
    });
  }

  function syncVisualState() {
    setVisualState(controllerRef.current?.getState() ?? visualState);
  }

  function clearLongPressTimer() {
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
  }

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

  function getHoverRetained() {
    return shouldRetainShellBallHoverInput({
      regionActive: regionActiveRef.current,
      inputFocused: inputFocusedRef.current,
      hasDraft: inputValue.trim() !== "",
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
    setInputValue(composeShellBallSpeechDraft(voiceBaseDraftRef.current, transcript));
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

    setInputValue(resolution.nextInputValue);
    controllerRef.current?.forceState(resolution.nextVisualState, {
      regionActive: resolution.nextVisualState === "hover_input",
    });
    syncVisualState();
    voiceTranscriptRef.current = "";

    if (resolution.finalizedSpeechPayload === null) {
      return;
    }

    try {
      await submitShellBallInput({
        text: resolution.finalizedSpeechPayload,
        trigger: "voice_commit",
        inputMode: "voice",
      });
      setFinalizedSpeechPayload(resolution.finalizedSpeechPayload);
    } catch (error) {
      console.warn("shell-ball voice submit failed", error);
    }
  }

  function acknowledgeFinalizedSpeechPayload() {
    setFinalizedSpeechPayload(null);
  }

  function disposeVoiceRecognition() {
    recognitionSessionIdRef.current += 1;
    recognitionStopReasonRef.current = "none";
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
  }

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

      if (recognitionStopReasonRef.current === "cancel" && event.error === "aborted") {
        return;
      }

      console.warn("shell-ball speech recognition error", event.error);
      recognitionStopReasonRef.current = "cancel";
    };

    recognition.onend = () => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      const stopReason = recognitionStopReasonRef.current;

      if (stopReason === "finish" || stopReason === "cancel") {
        void finalizeVoiceRecognition(stopReason);
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
    if (controllerRef.current?.getState() !== "voice_locked") {
      return;
    }

    stopVoiceRecognition("finish");
    consumeInteraction();
  }

  function handleRegionEnter() {
    regionActiveRef.current = true;
    dispatch("pointer_enter_hotspot", { regionActive: true, hoverRetained: false });
  }

  function handleRegionLeave() {
    regionActiveRef.current = false;
    clearLongPressTimer();

    if (!shouldKeepShellBallVoicePreviewOnRegionLeave(controllerRef.current?.getState() ?? visualState)) {
      setCurrentVoicePreview(null);
    }

    dispatch("pointer_leave_region", {
      regionActive: false,
      hoverRetained: false,
    });
  }

  function handleSubmitText() {
    const currentDraft = inputValue.trim();
    const reset = getShellBallPostSubmitInputReset(inputValue);
    if (reset === null) {
      return;
    }

    void (async () => {
      try {
        await submitShellBallInput({
          text: currentDraft,
          trigger: "hover_text_input",
          inputMode: "text",
        });
        dispatch("submit_text");
        setInputValue(reset.nextInputValue);
        inputFocusedRef.current = reset.nextFocused;
        setInputFocused(reset.nextFocused);
      } catch (error) {
        console.warn("shell-ball text submit failed", error);
      }
    })();
  }

  function handleAttachFile() {
    dispatch("attach_file");
  }

  function handlePressStart(event: PointerEvent<HTMLButtonElement>) {
    regionActiveRef.current = true;
    resetInteractionConsumed();
    pressStartXRef.current = event.clientX;
    pressStartYRef.current = event.clientY;
    setCurrentVoicePreview(null);
    clearLongPressTimer();

    const currentState = controllerRef.current?.getState();
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
      voiceBaseDraftRef.current = inputValueRef.current;
      if (longPressProgressHandleRef.current !== null) {
        cancelAnimationFrame(longPressProgressHandleRef.current);
        longPressProgressHandleRef.current = null;
      }
      longPressStartAtRef.current = null;
      setVoiceHoldProgress(0);
      setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("long_press_voice_entry"));
      dispatch("press_start");

      if (!startVoiceRecognition()) {
        setInputValue(voiceBaseDraftRef.current);
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
    if (currentState !== "voice_listening") {
      return;
    }

    setCurrentVoicePreview(
      getShellBallVoicePreviewFromEvent({
        startX: pressStartXRef.current,
        startY: pressStartYRef.current,
        clientX: event.clientX,
        clientY: event.clientY,
        fallbackPreview: voicePreviewRef.current,
      }),
    );
  }

  function handlePressEnd(event: PointerEvent<HTMLButtonElement>) {
    clearLongPressTimer();

    if (controllerRef.current?.getState() === "voice_listening") {
      consumeInteraction();
      const finalPreview = getShellBallVoicePreviewFromEvent({
        startX: pressStartXRef.current,
        startY: pressStartYRef.current,
        clientX: event.clientX,
        clientY: event.clientY,
        fallbackPreview: voicePreviewRef.current,
      });

      if (finalPreview === "lock") {
        dispatch("voice_lock");
        pressStartXRef.current = null;
        pressStartYRef.current = null;
        setCurrentVoicePreview(null);
        return true;
      }

      stopVoiceRecognition(finalPreview === "cancel" ? "cancel" : "finish");
      dispatch(resolveShellBallVoiceReleaseEvent(finalPreview));
      inputFocusedRef.current = false;
      setInputFocused(false);
      pressStartXRef.current = null;
      pressStartYRef.current = null;
      setCurrentVoicePreview(null);
      return true;
    }

    if (controllerRef.current?.getState() === "voice_locked") {
      stopVoiceRecognition("finish");
      consumeInteraction();
      dispatch("primary_click_locked_voice_end");
      inputFocusedRef.current = false;
      setInputFocused(false);
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
      regionActiveRef.current = true;
      controllerRef.current?.forceState("hover_input", { regionActive: true });
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
    regionActiveRef.current = true;
    controllerRef.current?.forceState("hover_input", { regionActive: true, hoverRetained: false });
    syncVisualState();
  }

  function handleForceState(state: ShellBallVisualState) {
    clearLongPressTimer();
    disposeVoiceRecognition();
    setInteractionConsumed(mapShellBallInteractionConsumedEventToFlag("force_state_reset"));
    pressStartXRef.current = null;
    pressStartYRef.current = null;
    inputFocusedRef.current = false;
    setInputFocused(false);
    setCurrentVoicePreview(null);
    controllerRef.current?.forceState(state, { regionActive: regionActiveRef.current });
    syncVisualState();
  }

  useEffect(() => {
    syncHoverRetention();
  }, [inputValue]);

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
    return () => {
      clearLongPressTimer();
      disposeVoiceRecognition();
      pressStartXRef.current = null;
      pressStartYRef.current = null;
      voicePreviewRef.current = null;
      controllerRef.current?.dispose();
    };
  }, []);

  return {
    visualState,
    inputValue,
    setInputValue,
    finalizedSpeechPayload,
    acknowledgeFinalizedSpeechPayload,
    voicePreview,
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
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handlePressCancel,
    handleInputFocusChange,
    handleInputFocusRequest,
    handleForceState,
  };
}
