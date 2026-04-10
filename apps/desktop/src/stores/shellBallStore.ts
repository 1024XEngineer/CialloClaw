import { create } from "zustand";
import type { ShellBallVisualState } from "../features/shell-ball/shellBall.types";

type ShellBallState = {
  visualState: ShellBallVisualState;
  setVisualState: (visualState: ShellBallVisualState) => void;
};

export const useShellBallStore = create<ShellBallState>((set) => ({
  visualState: "idle",
  setVisualState: (visualState) => set({ visualState }),
}));
