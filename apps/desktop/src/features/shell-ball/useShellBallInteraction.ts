import { useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";
import {
  createShellBallInteractionController,
  getShellBallInputBarMode,
  getShellBallVoicePreview,
  resolveShellBallVoiceReleaseEvent,
  SHELL_BALL_LONG_PRESS_MS,
  shouldRetainShellBallHoverInput,
  type ShellBallVoicePreview,
} from "./shellBall.interaction";
import type { ShellBallInteractionEvent, ShellBallVisualState } from "./shellBall.types";
import { useShellBallStore } from "../../stores/shellBallStore";

type TimeoutHandle = ReturnType<typeof globalThis.setTimeout>;

type ShellBallInteractionController = ReturnType<typeof createShellBallInteractionController>;

export function canOpenShellBallDashboard(state: ShellBallVisualState) {
  return state === "idle" || state === "hover_input";
}

export function shouldOpenShellBallDashboardFromDoubleClick(input: {
  state: ShellBallVisualState;
  interactionConsumed: boolean;
}) {
  return !input.interactionConsumed && canOpenShellBallDashboard(input.state);
}

export function shouldOpenShellBallDashboardFromSingleClick(_state: ShellBallVisualState) {
  return false;
}

type ShellBallInteractionConsumedEvent =
  | "press_start"
  | "long_press_voice_entry"
  | "voice_flow_consumed"
  | "force_state_reset";

export function getShellBallInteractionConsumedForEvent(event: ShellBallInteractionConsumedEvent) {
  switch (event) {
    case "press_start":
    case "force_state_reset":
      return false;
    case "long_press_voice_entry":
    case "voice_flow_consumed":
      return true;
  }
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

export function useShellBallInteraction() {
  const visualState = useShellBallStore((state) => state.visualState);
  const setVisualState = useShellBallStore((state) => state.setVisualState);
  const [inputValue, setInputValue] = useState("");
  const [voicePreview, setVoicePreview] = useState<ShellBallVoicePreview>(null);
  const [interactionConsumed, setInteractionConsumed] = useState(false);
  const regionActiveRef = useRef(false);
  const inputFocusedRef = useRef(false);
  const pressStartXRef = useRef<number | null>(null);
  const pressStartYRef = useRef<number | null>(null);
  const voicePreviewRef = useRef<ShellBallVoicePreview>(null);
  const longPressHandleRef = useRef<TimeoutHandle | null>(null);
  const setVisualStateRef = useRef(setVisualState);
  const controllerRef = useRef<ShellBallInteractionController | null>(null);

  setVisualStateRef.current = setVisualState;

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
      return;
    }

    globalThis.clearTimeout(longPressHandleRef.current);
    longPressHandleRef.current = null;
  }

  function resetInteractionConsumed() {
    setInteractionConsumed(getShellBallInteractionConsumedForEvent("press_start"));
  }

  function consumeInteraction() {
    setInteractionConsumed(getShellBallInteractionConsumedForEvent("voice_flow_consumed"));
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

    consumeInteraction();
    dispatch("primary_click_locked_voice_end");
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
      hoverRetained: getHoverRetained(),
    });
  }

  function handleSubmitText() {
    const reset = getShellBallPostSubmitInputReset(inputValue);
    if (reset === null) {
      return;
    }

    dispatch("submit_text");
    setInputValue(reset.nextInputValue);
    inputFocusedRef.current = reset.nextFocused;
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

    longPressHandleRef.current = globalThis.setTimeout(() => {
      longPressHandleRef.current = null;
      setInteractionConsumed(getShellBallInteractionConsumedForEvent("long_press_voice_entry"));
      dispatch("press_start");
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

      dispatch(resolveShellBallVoiceReleaseEvent(finalPreview));
      pressStartXRef.current = null;
      pressStartYRef.current = null;
      setCurrentVoicePreview(null);
      return true;
    } else if (controllerRef.current?.getState() === "voice_locked") {
      consumeInteraction();
      dispatch("primary_click_locked_voice_end");
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

  function handleInputFocusChange(focused: boolean) {
    inputFocusedRef.current = focused;
    if (!focused) {
      syncHoverRetention();
    }
  }

  function handleForceState(state: ShellBallVisualState) {
    clearLongPressTimer();
    setInteractionConsumed(getShellBallInteractionConsumedForEvent("force_state_reset"));
    pressStartXRef.current = null;
    pressStartYRef.current = null;
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
    voicePreview,
    inputBarMode: getShellBallInputBarMode(visualState),
    interactionConsumed,
    shouldOpenDashboardFromDoubleClick: shouldOpenShellBallDashboardFromDoubleClick({
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
    handleInputFocusChange,
    handleForceState,
  };
}
