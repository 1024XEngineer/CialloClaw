import { useCallback, useRef } from "react";
import { useInterval, useUnmount } from "ahooks";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { readShellBallSelectionSnapshot } from "@/platform/shellBallWindow";
import { shellBallWindowLabels } from "@/platform/shellBallWindowController";
import { shellBallWindowSyncEvents, type ShellBallSelectionSnapshotPayload } from "../shellBall.windowSync";
import type { ShellBallSelectionSnapshot } from "./selection.types";

const SHELL_BALL_SELECTION_POLL_MS = 250;

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
export function ShellBallSelectionProvider() {
  const currentWindow = typeof window !== "undefined" && "__TAURI_INTERNALS__" in window ? getCurrentWindow() : null;
  const windowLabel = currentWindow?.label ?? "browser";
  const pollEnabled = currentWindow !== null && shouldPollShellBallNativeSelection(windowLabel);
  const lastSnapshotRef = useRef<ShellBallSelectionSnapshot | null>(null);

  const emitSelectionSnapshot = useCallback(async (snapshot: ShellBallSelectionSnapshot | null) => {
    if (currentWindow === null) {
      return;
    }

    const payload: ShellBallSelectionSnapshotPayload = { snapshot };
    await currentWindow.emit(shellBallWindowSyncEvents.selectionSnapshot, payload);
  }, [currentWindow]);

  const publishLatestSelection = useCallback(async () => {
    if (!pollEnabled) {
      return;
    }

    const snapshot = await readShellBallSelectionSnapshot();
    if (areShellBallSelectionSnapshotsEqual(lastSnapshotRef.current, snapshot)) {
      return;
    }

    lastSnapshotRef.current = snapshot;
    await emitSelectionSnapshot(snapshot);
  }, [emitSelectionSnapshot, pollEnabled]);

  useInterval(() => {
    void publishLatestSelection();
  }, pollEnabled ? SHELL_BALL_SELECTION_POLL_MS : undefined, {
    immediate: true,
  });

  useUnmount(() => {
    lastSnapshotRef.current = null;
    void emitSelectionSnapshot(null);
  });

  return null;
}
