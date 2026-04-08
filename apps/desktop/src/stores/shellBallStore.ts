// 该文件维护悬浮球交互状态。 
import { create } from "zustand";

type ShellBallState = {
  status: "idle" | "primed" | "confirming" | "running" | "waiting_auth";
  setStatus: (status: ShellBallState["status"]) => void;
};

export const useShellBallStore = create<ShellBallState>((set) => ({
  status: "primed",
  setStatus: (status) => set({ status }),
}));
