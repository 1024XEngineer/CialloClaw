import { create } from "zustand";

type ControlPanelState = {
  currentSection: "general" | "memory" | "models";
  setCurrentSection: (section: ControlPanelState["currentSection"]) => void;
};

export const useControlPanelStore = create<ControlPanelState>((set) => ({
  currentSection: "general",
  setCurrentSection: (currentSection) => set({ currentSection }),
}));
