// 该文件维护控制面板页面状态。 
import { create } from "zustand";

// ControlPanelState 描述当前模块状态。
type ControlPanelState = {
  currentSection: "general" | "memory" | "models";
  setCurrentSection: (section: ControlPanelState["currentSection"]) => void;
};

// useControlPanelStore 暴露当前模块的状态容器。
export const useControlPanelStore = create<ControlPanelState>((set) => ({
  currentSection: "general",
  setCurrentSection: (currentSection) => set({ currentSection }),
}));
