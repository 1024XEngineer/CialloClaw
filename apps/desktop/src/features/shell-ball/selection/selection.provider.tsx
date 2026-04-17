import { useCallback, useEffect, useRef, useState } from "react";
import { useInterval, useUnmount } from "ahooks";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { readShellBallSelectionSnapshot } from "@/platform/shellBallWindow";
import type { ShellBallVisualState } from "../shellBall.types";
import { shellBallWindowLabels } from "@/platform/shellBallWindowController";
import {
  shellBallWindowSyncEvents,
  type ShellBallSelectionActivityPayload,
  type ShellBallSelectionSnapshotPayload,
} from "../shellBall.windowSync";
import type { ShellBallSelectionSnapshot } from "./selection.types";

const SHELL_BALL_SELECTION_POLL_MS = 1000;

/**
 * Determines whether the current desktop window should poll the native
 * selection adapter. The floating ball window owns global selection polling so
 * helper windows do not duplicate the host calls.
 *
 * @param label Current Tauri window label.
 * @returns Whether this window should poll native selection snapshots.
 */
export function shouldPollShellBallNativeSelection(label: string) {
  return label === shellBallWindowLabels.ball;
}

/**
 * Determines whether native selection sensing should stay armed for the current
 * shell-ball visual state.
 *
 * @param visualState Current shell-ball visual state.
 * @returns Whether selection activity should be tracked.
 */
export function shouldTrackShellBallSelectionInState(visualState: ShellBallVisualState) {
  return visualState === "idle" || visualState === "hover_input";
}

/**
 * Compares two selection snapshots while ignoring transient timestamp changes.
 *
 * @param left Previous selection snapshot.
 * @param right Latest selection snapshot.
 * @returns Whether the snapshots represent the same logical selection.
 */
export function areShellBallSelectionSnapshotsEqual(
  left: ShellBallSelectionSnapshot | null,
  right: ShellBallSelectionSnapshot | null,
) {
  if (left === right) {
    return true;
  }

  if (left === null || right === null) {
    return false;
  }

  return (
    left.text === right.text
    && left.source === right.source
    && left.page_context.title === right.page_context.title
    && left.page_context.url === right.page_context.url
    && left.page_context.app_name === right.page_context.app_name
  );
}

/**
 * Publishes native text selections from the host platform adapter so shell-ball
 * can react to real selections without encoding Windows-specific details in the
 * frontend.
 *
 * @returns `null`; this component only bridges selection state.
 */
export function ShellBallSelectionProvider({ visualState }: { visualState: ShellBallVisualState }) {
  const currentWindow = typeof window !== "undefined" && "__TAURI_INTERNALS__" in window ? getCurrentWindow() : null;
  const windowLabel = currentWindow?.label ?? "browser";
  const pollEnabled = currentWindow !== null && shouldPollShellBallNativeSelection(windowLabel);
  const sensingEnabled = pollEnabled && shouldTrackShellBallSelectionInState(visualState);
  const lastSnapshotRef = useRef<ShellBallSelectionSnapshot | null>(null);
  const [selectionDirty, setSelectionDirty] = useState(false);

  const emitSelectionSnapshot = useCallback(async (snapshot: ShellBallSelectionSnapshot | null) => {
    if (currentWindow === null) {
      return;
    }

    const payload: ShellBallSelectionSnapshotPayload = { snapshot };
    await currentWindow.emit(shellBallWindowSyncEvents.selectionSnapshot, payload);
  }, [currentWindow]);

  const publishLatestSelection = useCallback(async () => {
    if (!sensingEnabled) {
      return;
    }

    const snapshot = await readShellBallSelectionSnapshot();
    setSelectionDirty(false);

    if (areShellBallSelectionSnapshotsEqual(lastSnapshotRef.current, snapshot)) {
      return;
    }

    lastSnapshotRef.current = snapshot;
    await emitSelectionSnapshot(snapshot);
  }, [emitSelectionSnapshot, sensingEnabled]);

  useEffect(() => {
    if (!pollEnabled) {
      return;
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    void currentWindow
      ?.listen<ShellBallSelectionActivityPayload>(shellBallWindowSyncEvents.selectionActivity, () => {
        if (!shouldTrackShellBallSelectionInState(visualState)) {
          return;
        }

        setSelectionDirty(true);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, [currentWindow, pollEnabled, visualState]);

  useEffect(() => {
    if (sensingEnabled) {
      return;
    }

    setSelectionDirty(false);
    lastSnapshotRef.current = null;
    void emitSelectionSnapshot(null);
  }, [emitSelectionSnapshot, sensingEnabled]);

  useInterval(() => {
    void publishLatestSelection();
  }, sensingEnabled && selectionDirty ? SHELL_BALL_SELECTION_POLL_MS : undefined);

  useUnmount(() => {
    lastSnapshotRef.current = null;
    void emitSelectionSnapshot(null);
  });

  return null;
}
