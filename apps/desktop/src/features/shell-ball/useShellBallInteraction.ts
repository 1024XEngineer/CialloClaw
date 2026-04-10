import { useEffect, useRef, useState } from "react";
import {
  createShellBallInteractionController,
  getShellBallInputBarMode,
  SHELL_BALL_CANCEL_DELTA_PX,
  SHELL_BALL_LOCK_DELTA_PX,
  SHELL_BALL_LONG_PRESS_MS,
} from "./shellBall.interaction";
import type { ShellBallInteractionEvent, ShellBallVisualState } from "./shellBall.types";
import { useShellBallStore } from "../../stores/shellBallStore";

type TimeoutHandle = ReturnType<typeof globalThis.setTimeout>;

type ShellBallInteractionController = ReturnType<typeof createShellBallInteractionController>;

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
  const regionActiveRef = useRef(false);
  const pressStartYRef = useRef<number | null>(null);
  const longPressHandleRef = useRef<TimeoutHandle | null>(null);
  const suppressNextPrimaryClickRef = useRef(false);
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

  function dispatch(event: ShellBallInteractionEvent, regionActive = regionActiveRef.current) {
    controllerRef.current?.dispatch(event, { regionActive });
    syncVisualState();
  }

  function handlePrimaryClick() {
    if (suppressNextPrimaryClickRef.current) {
      suppressNextPrimaryClickRef.current = false;
      return;
    }

    if (controllerRef.current?.getState() !== "voice_locked") {
      return;
    }

    dispatch("primary_click_locked_voice_end");
  }

  function handleRegionEnter() {
    regionActiveRef.current = true;
    dispatch("pointer_enter_hotspot", true);
  }

  function handleRegionLeave() {
    regionActiveRef.current = false;
    clearLongPressTimer();
    suppressNextPrimaryClickRef.current = false;
    dispatch("pointer_leave_region", false);
  }

  function handleSubmitText() {
    if (inputValue.trim() === "") {
      return;
    }

    dispatch("submit_text");
  }

  function handleAttachFile() {
    dispatch("attach_file");
  }

  function handlePressStart(clientY: number) {
    regionActiveRef.current = true;
    pressStartYRef.current = clientY;
    clearLongPressTimer();

    const currentState = controllerRef.current?.getState();
    if (currentState !== "idle" && currentState !== "hover_input") {
      return;
    }

    suppressNextPrimaryClickRef.current = true;

    longPressHandleRef.current = globalThis.setTimeout(() => {
      longPressHandleRef.current = null;
      dispatch("press_start");
    }, SHELL_BALL_LONG_PRESS_MS);
  }

  function handlePressMove(clientY: number) {
    if (pressStartYRef.current === null) {
      return;
    }

    const currentState = controllerRef.current?.getState();
    if (currentState !== "voice_listening") {
      return;
    }

    const deltaY = clientY - pressStartYRef.current;
    if (deltaY <= -SHELL_BALL_LOCK_DELTA_PX) {
      dispatch("voice_lock");
      return;
    }

    if (deltaY >= SHELL_BALL_CANCEL_DELTA_PX) {
      dispatch("voice_cancel");
      pressStartYRef.current = null;
    }
  }

  function handlePressEnd() {
    clearLongPressTimer();

    if (controllerRef.current?.getState() === "voice_listening") {
      dispatch("voice_finish");
    }

    pressStartYRef.current = null;
  }

  function handleForceState(state: ShellBallVisualState) {
    clearLongPressTimer();
    suppressNextPrimaryClickRef.current = false;
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
    return () => {
      clearLongPressTimer();
      suppressNextPrimaryClickRef.current = false;
      controllerRef.current?.dispose();
    };
  }, []);

  return {
    visualState,
    inputValue,
    setInputValue,
    inputBarMode: getShellBallInputBarMode(visualState),
    handlePrimaryClick,
    handleRegionEnter,
    handleRegionLeave,
    handleSubmitText,
    handleAttachFile,
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handleForceState,
  };
}
