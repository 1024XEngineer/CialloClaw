// 该文件维护悬浮球交互状态。 
import { create } from "zustand";

// ShellBallState 描述当前模块状态。
type ShellBallState = {
  status: "idle" | "primed" | "confirming" | "running" | "waiting_auth";
  setStatus: (status: ShellBallState["status"]) => void;
};

// useShellBallStore 暴露当前模块的状态容器。
export const useShellBallStore = create<ShellBallState>((set) => ({
  status: "primed",
  setStatus: (status) => set({ status }),
}));
